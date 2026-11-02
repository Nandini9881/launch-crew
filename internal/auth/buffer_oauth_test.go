package auth_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubham-tomar/launchcrew/internal/auth"
	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/db"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/session"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/bufferfake"
	"github.com/shubham-tomar/launchcrew/internal/users"
)

const baseURL = "https://lc.test"

type env struct {
	oa   *auth.BufferOAuth
	q    *sqlc.Queries
	st   *session.Store
	kr   *crypto.Keyring
	fake *bufferfake.Fake
	acct string
	pool *pgxpool.Pool
}

func setup(t *testing.T) *env {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	kr, err := crypto.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	acct := "acct-" + uuid.NewString()
	fake := bufferfake.New(t, acct, "org-1")
	st := &session.Store{Keyring: kr}
	q := sqlc.New(pool)
	oa := &auth.BufferOAuth{
		ClientID: "cid", ClientSecret: "secret", BaseURL: baseURL,
		AuthURL: "https://auth.example/auth", TokenURL: fake.TokenURL(), APIURL: fake.APIURL(),
		Q: q, Pool: pool, Keyring: kr, Sessions: st,
	}
	return &env{oa, q, st, kr, fake, acct, pool}
}

// start runs /auth/buffer/start and returns the resulting cookie and redirect.
func (e *env) start(t *testing.T, query string) (*http.Cookie, *url.URL) {
	t.Helper()
	rr := httptest.NewRecorder()
	e.oa.Start(rr, httptest.NewRequest("GET", "/auth/buffer/start"+query, nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("start: %d", rr.Code)
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return rr.Result().Cookies()[0], loc
}

func (e *env) callback(cookie *http.Cookie, state string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/auth/buffer/callback?code=abc&state="+url.QueryEscape(state), nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	e.oa.Callback(rr, req)
	return rr
}

func TestStartRedirectsWithPKCE(t *testing.T) {
	e := setup(t)
	_, loc := e.start(t, "")
	q := loc.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("pkce params: %v", q)
	}
	if q.Get("state") == "" {
		t.Fatal("empty state")
	}
	if got, want := q.Get("redirect_uri"), baseURL+"/auth/buffer/callback"; got != want {
		t.Fatalf("redirect_uri %q want %q", got, want)
	}
	if q.Get("client_id") != "cid" || q.Get("response_type") != "code" || q.Get("scope") != "account:read posts:read posts:write offline_access" {
		t.Fatalf("params: %v", q)
	}
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	e := setup(t)
	cookie, _ := e.start(t, "")
	rr := e.callback(cookie, "wrong-state")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rr.Code)
	}
	if _, err := e.q.GetBufferConnectionByAccount(context.Background(), e.acct); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("connection row exists: %v", err)
	}
	if n := len(e.fake.TokenForms()); n != 0 {
		t.Fatalf("token endpoint called %d times", n)
	}
}

func TestCallbackExchangesAndUpserts(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	cookie, loc := e.start(t, "")
	rr := e.callback(cookie, loc.Query().Get("state"))
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/captain" {
		t.Fatalf("got %d %q", rr.Code, rr.Header().Get("Location"))
	}
	form := e.fake.TokenForms()[0]
	if form.Get("code_verifier") == "" || form.Get("code") != "abc" || form.Get("client_id") != "cid" || form.Get("client_secret") != "secret" || form.Get("redirect_uri") != baseURL+"/auth/buffer/callback" {
		t.Fatalf("token form: %v", form)
	}

	conn, err := e.q.GetBufferConnectionByAccount(ctx, e.acct)
	if err != nil {
		t.Fatal(err)
	}
	if conn.OrgID.String != "org-1" || conn.Status != "ok" || conn.KeyID != "k1" || !conn.ExpiresAt.Valid {
		t.Fatalf("conn: %+v", conn)
	}
	if plain, err := e.kr.Decrypt(conn.AccessTokenEnc); err != nil || string(plain) != "access-1" {
		t.Fatalf("access token: %q %v", plain, err)
	}
	if plain, err := e.kr.Decrypt(conn.RefreshTokenEnc); err != nil || string(plain) != "refresh-1" {
		t.Fatalf("refresh token: %q %v", plain, err)
	}

	// The session cookie now identifies the user and connection, with the OAuth state cleared.
	req := httptest.NewRequest("GET", "/", nil)
	cs := rr.Result().Cookies()
	req.AddCookie(cs[len(cs)-1]) // last Set-Cookie wins
	sess := e.st.Get(req)
	if sess.UserID != conn.UserID || sess.ConnectionID != conn.ID || sess.OAuthState != "" || sess.PKCEVerifier != "" {
		t.Fatalf("session: %+v", sess)
	}

	// A second login for the same account reuses the row and its user.
	cookie2, loc2 := e.start(t, "")
	rr = e.callback(cookie2, loc2.Query().Get("state"))
	if rr.Code != http.StatusFound {
		t.Fatalf("second callback: %d", rr.Code)
	}
	conn2, err := e.q.GetBufferConnectionByAccount(ctx, e.acct)
	if err != nil {
		t.Fatal(err)
	}
	if conn2.ID != conn.ID || conn2.UserID != conn.UserID {
		t.Fatalf("row not reused: %v/%v vs %v/%v", conn2.ID, conn2.UserID, conn.ID, conn.UserID)
	}
	if plain, _ := e.kr.Decrypt(conn2.AccessTokenEnc); string(plain) != "access-2" {
		t.Fatalf("tokens not refreshed: %q", plain)
	}
}

func TestReturnToSurvives(t *testing.T) {
	e := setup(t)
	cookie, loc := e.start(t, "?return_to=/s/abc")
	rr := e.callback(cookie, loc.Query().Get("state"))
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/s/abc" {
		t.Fatalf("got %d %q", rr.Code, rr.Header().Get("Location"))
	}

	// Absolute and protocol-relative targets are dropped.
	for _, bad := range []string{"https://evil.example/x", "//evil.example/x", `/\evil.example`, "/\t/evil.com", "/\n/evil.com"} {
		cookie, loc := e.start(t, "?return_to="+url.QueryEscape(bad))
		rr := e.callback(cookie, loc.Query().Get("state"))
		if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/captain" {
			t.Fatalf("return_to %q -> %q", bad, rr.Header().Get("Location"))
		}
	}
}

func TestCallbackStateIsSingleUse(t *testing.T) {
	e := setup(t)
	cookie, loc := e.start(t, "")
	state := loc.Query().Get("state")

	// First attempt fails at the exchange.
	good := e.oa.TokenURL
	e.oa.TokenURL = "http://127.0.0.1:1/token"
	rr := e.callback(cookie, state)
	e.oa.TokenURL = good
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("failed exchange: %d", rr.Code)
	}

	// Retrying with the cookie from that response and the same state is refused.
	rr = e.callback(rr.Result().Cookies()[0], state)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("retry: %d", rr.Code)
	}
	if _, err := e.q.GetBufferConnectionByAccount(context.Background(), e.acct); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("connection row exists: %v", err)
	}
}

func TestCallbackAccessDenied(t *testing.T) {
	e := setup(t)
	cookie, _ := e.start(t, "")
	req := httptest.NewRequest("GET", "/auth/buffer/callback?error=access_denied", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	e.oa.Callback(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "denied") {
		t.Fatalf("got %d %q", rr.Code, rr.Body)
	}
}

func TestNewAccountJoinsSignedInUser(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	u, _ := e.q.CreateUser(ctx) // e.g. a verified GitHub contributor
	rr := httptest.NewRecorder()
	if err := e.st.Set(rr, session.Session{UserID: u.ID}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/auth/buffer/start", nil)
	req.AddCookie(rr.Result().Cookies()[0])
	rr = httptest.NewRecorder()
	e.oa.Start(rr, req)
	loc, _ := url.Parse(rr.Header().Get("Location"))
	cb := e.callback(rr.Result().Cookies()[0], loc.Query().Get("state"))
	if cb.Code != http.StatusFound {
		t.Fatalf("callback: %d", cb.Code)
	}
	conn, err := e.q.GetBufferConnectionByAccount(ctx, e.acct)
	if err != nil || conn.UserID != u.ID {
		t.Fatalf("connection user = %v (%v), want %v", conn.UserID, err, u.ID)
	}
}

// signIn runs start and callback with an optional prior session user and
// returns the session the browser ends with.
func (e *env) signIn(t *testing.T, prior uuid.UUID) session.Session {
	t.Helper()
	req := httptest.NewRequest("GET", "/auth/buffer/start", nil)
	if prior != uuid.Nil {
		rr := httptest.NewRecorder()
		if err := e.st.Set(rr, session.Session{UserID: prior}); err != nil {
			t.Fatal(err)
		}
		req.AddCookie(rr.Result().Cookies()[0])
	}
	rr := httptest.NewRecorder()
	e.oa.Start(rr, req)
	loc, _ := url.Parse(rr.Header().Get("Location"))
	cb := e.callback(rr.Result().Cookies()[0], loc.Query().Get("state"))
	if cb.Code != http.StatusFound {
		t.Fatalf("callback: %d %s", cb.Code, cb.Body)
	}
	out := httptest.NewRequest("GET", "/", nil)
	cs := cb.Result().Cookies()
	out.AddCookie(cs[len(cs)-1])
	return e.st.Get(out)
}

func TestCaptainSignInWithoutSessionKeepsUser(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	first := e.signIn(t, uuid.Nil)
	again := e.signIn(t, uuid.Nil)
	if again.UserID != first.UserID {
		t.Fatalf("user changed: %v -> %v", first.UserID, again.UserID)
	}
	// A session already signed in as the owner takes the same path.
	if s := e.signIn(t, first.UserID); s.UserID != first.UserID {
		t.Fatalf("owner session user = %v", s.UserID)
	}
	if _, err := e.q.GetUser(ctx, first.UserID); err != nil {
		t.Fatalf("user gone: %v", err)
	}
}

func TestSignInMergesSessionUserIntoAccountOwner(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	owner := e.signIn(t, uuid.Nil).UserID // B: owns the Buffer account
	a, _ := e.q.CreateUser(ctx)           // A: e.g. made by GitHub verify
	ghID := 4_000_000_000 + int64(uuid.New().ID())
	if _, err := e.q.UpsertGitHubIdentity(ctx, sqlc.UpsertGitHubIdentityParams{GithubUserID: ghID, UserID: a.ID, Login: "octo"}); err != nil {
		t.Fatal(err)
	}
	s := e.signIn(t, a.ID)
	if s.UserID != owner {
		t.Fatalf("session user = %v, want %v", s.UserID, owner)
	}
	if ident, _ := e.q.GetGitHubIdentity(ctx, ghID); ident.UserID != owner {
		t.Fatalf("identity user = %v", ident.UserID)
	}
	if _, err := e.q.GetUser(ctx, a.ID); err == nil {
		t.Fatal("merged user still exists")
	}
}

func TestMergeUsersSameUserIsNoop(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	u, _ := e.q.CreateUser(ctx)
	if err := users.Merge(ctx, e.pool, u.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.GetUser(ctx, u.ID); err != nil {
		t.Fatalf("user deleted by self-merge: %v", err)
	}
}
