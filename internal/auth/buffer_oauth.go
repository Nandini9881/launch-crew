// Package auth implements sign-in with Buffer OAuth (authorization code + PKCE).
package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/session"
	"github.com/shubham-tomar/launchcrew/internal/users"
)

// Buffer endpoints, checked against developers.buffer.com on 2026-11-01 (see internal/buffer/SCHEMA.md).
const (
	DefaultAuthURL  = "https://auth.buffer.com/auth"
	DefaultTokenURL = "https://auth.buffer.com/token"
	DefaultAPIURL   = "https://api.buffer.com"
)

const (
	scopes       = "account:read posts:read posts:write offline_access"
	defaultAfter = "/captain"
	// httpTimeout bounds every call to Buffer made while signing in.
	httpTimeout = 15 * time.Second
)

// BufferOAuth serves the sign-in routes. AuthURL, TokenURL and APIURL
// default to Buffer's when empty; tests point them at a fake.
type BufferOAuth struct {
	ClientID, ClientSecret string
	BaseURL                string
	AuthURL, TokenURL      string
	APIURL                 string
	Q                      *sqlc.Queries
	Pool                   *pgxpool.Pool // for merging users
	Keyring                *crypto.Keyring
	Sessions               *session.Store
}

func (b *BufferOAuth) config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     b.ClientID,
		ClientSecret: b.ClientSecret,
		RedirectURL:  b.BaseURL + "/auth/buffer/callback",
		Scopes:       strings.Fields(scopes),
		Endpoint: oauth2.Endpoint{
			AuthURL:   orDefault(b.AuthURL, DefaultAuthURL),
			TokenURL:  orDefault(b.TokenURL, DefaultTokenURL),
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Start stores return_to, state and the PKCE verifier in the session, then redirects to Buffer.
func (b *BufferOAuth) Start(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier := oauth2.GenerateVerifier()
	// Keep the signed-in user (e.g. a verified contributor) across the flow.
	if err := b.Sessions.Set(w, session.Session{
		UserID:       b.Sessions.Get(r).UserID,
		ReturnTo:     safeReturnTo(r.URL.Query().Get("return_to")),
		OAuthState:   state,
		PKCEVerifier: verifier,
	}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// signup=1 is accepted and ignored for now.
	http.Redirect(w, r, b.config().AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "consent")), http.StatusFound)
}

// Callback completes the flow: verify state, exchange the code, upsert the connection, sign in.
func (b *BufferOAuth) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := b.Sessions.Get(r)
	// Consume the state up front so every callback attempt is single-use.
	spent := sess
	spent.OAuthState, spent.PKCEVerifier = "", ""
	if err := b.Sessions.Set(w, spent); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if r.URL.Query().Get("error") == "access_denied" {
		http.Error(w, "Buffer access was denied. Start again to retry.", http.StatusBadRequest)
		return
	}
	got := r.URL.Query().Get("state")
	if sess.OAuthState == "" || subtle.ConstantTimeCompare([]byte(got), []byte(sess.OAuthState)) != 1 {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	// Bound the exchange the same way fetchAccount is, so a stalled Buffer cannot hold the handler open.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: httpTimeout})
	tok, err := b.config().Exchange(ctx, code, oauth2.VerifierOption(sess.PKCEVerifier))
	if err != nil {
		log.Printf("buffer oauth: exchange: %v", err)
		http.Error(w, "sign-in failed", http.StatusBadGateway)
		return
	}
	accountID, orgID, err := b.fetchAccount(ctx, tok.AccessToken)
	if err != nil {
		log.Printf("buffer oauth: account: %v", err)
		http.Error(w, "sign-in failed", http.StatusBadGateway)
		return
	}
	conn, err := b.saveConnection(ctx, sess.UserID, accountID, orgID, tok)
	if err != nil {
		log.Printf("buffer oauth: save: %v", err)
		http.Error(w, "sign-in failed", http.StatusInternalServerError)
		return
	}
	// Signed in as A, and this Buffer account already belongs to B: the
	// browser proved both, so A merges into B.
	if sess.UserID != uuid.Nil && sess.UserID != conn.UserID {
		if err := users.Merge(ctx, b.Pool, sess.UserID, conn.UserID); err != nil {
			log.Printf("buffer oauth: %v", err)
			http.Error(w, "sign-in failed", http.StatusInternalServerError)
			return
		}
	}
	if err := b.Sessions.Set(w, session.Session{UserID: conn.UserID, ConnectionID: conn.ID}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	dest := safeReturnTo(sess.ReturnTo)
	if dest == "" {
		dest = defaultAfter
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// Logout clears the session.
func (b *BufferOAuth) Logout(w http.ResponseWriter, r *http.Request) {
	b.Sessions.Clear(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// userExists guards against a stale cookie naming a merged-away user.
func (b *BufferOAuth) userExists(ctx context.Context, id uuid.UUID) bool {
	_, err := b.Q.GetUser(ctx, id)
	return err == nil
}

// saveConnection upserts the account's connection. A new account belongs to
// the signed-in user when there is one, else to a new user.
func (b *BufferOAuth) saveConnection(ctx context.Context, sessionUser uuid.UUID, accountID, orgID string, tok *oauth2.Token) (sqlc.BufferConnection, error) {
	existing, err := b.Q.GetBufferConnectionByAccount(ctx, accountID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.BufferConnection{}, err
	}
	userID := existing.UserID
	if errors.Is(err, pgx.ErrNoRows) && sessionUser != uuid.Nil && b.userExists(ctx, sessionUser) {
		userID = sessionUser
	} else if errors.Is(err, pgx.ErrNoRows) {
		u, err := b.Q.CreateUser(ctx)
		if err != nil {
			return sqlc.BufferConnection{}, err
		}
		userID = u.ID
	}
	access, keyID, err := b.Keyring.Encrypt([]byte(tok.AccessToken))
	if err != nil {
		return sqlc.BufferConnection{}, err
	}
	refresh, _, err := b.Keyring.Encrypt([]byte(tok.RefreshToken))
	if err != nil {
		return sqlc.BufferConnection{}, err
	}
	return b.Q.UpsertBufferConnection(ctx, sqlc.UpsertBufferConnectionParams{
		UserID:          userID,
		BufferAccountID: accountID,
		OrgID:           pgtype.Text{String: orgID, Valid: orgID != ""},
		AccessTokenEnc:  access,
		RefreshTokenEnc: refresh,
		KeyID:           keyID,
		ExpiresAt:       pgtype.Timestamptz{Time: tok.Expiry, Valid: !tok.Expiry.IsZero()},
	})
}

const accountQuery = `query Account { account { id organizations { id } } }`

func (b *BufferOAuth) fetchAccount(ctx context.Context, accessToken string) (accountID, orgID string, err error) {
	body, _ := json.Marshal(map[string]string{"query": accountQuery})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, orDefault(b.APIURL, DefaultAPIURL), bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("api status %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			Account struct {
				ID            string `json:"id"`
				Organizations []struct {
					ID string `json:"id"`
				} `json:"organizations"`
			} `json:"account"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	if len(out.Errors) > 0 {
		return "", "", fmt.Errorf("graphql: %s", out.Errors[0].Message)
	}
	if out.Data.Account.ID == "" {
		return "", "", errors.New("no account id in response")
	}
	if orgs := out.Data.Account.Organizations; len(orgs) > 0 {
		orgID = orgs[0].ID
	}
	return out.Data.Account.ID, orgID, nil
}

func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// safeReturnTo keeps only same-site relative paths ("/x"). It rejects control
// characters (browsers strip tab/newline, turning "/\t/evil" into "//evil"),
// backslashes, "//host" and anything with a scheme or host.
func safeReturnTo(p string) string {
	for i := 0; i < len(p); i++ {
		if p[i] < 0x20 || p[i] == 0x7f || p[i] == '\\' {
			return ""
		}
	}
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return ""
	}
	if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return ""
	}
	return p
}
