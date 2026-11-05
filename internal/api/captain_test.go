package api_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubham-tomar/launchcrew/internal/api"
	"github.com/shubham-tomar/launchcrew/internal/buffer"
	"github.com/shubham-tomar/launchcrew/internal/cards"
	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/db"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/httpserver"
	"github.com/shubham-tomar/launchcrew/internal/launch"
	"github.com/shubham-tomar/launchcrew/internal/preflight"
	"github.com/shubham-tomar/launchcrew/internal/session"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/bufferfake"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/r2fake"
)

// captainEnv is a draft manual launch owned by owner, with a repo, served
// through the full handler.
type captainEnv struct {
	*env
	pool         *pgxpool.Pool
	owner, other sqlc.User
	launch       sqlc.Launch
	repo         sqlc.Repo
}

func setupCaptain(t *testing.T) *captainEnv {
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
	q := sqlc.New(pool)
	kr, err := crypto.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	owner, _ := q.CreateUser(ctx)
	other, _ := q.CreateUser(ctx)
	repo, err := q.UpsertRepo(ctx, sqlc.UpsertRepoParams{
		InstallationID: 9_000_000_000 + int64(uuid.New().ID()), OwnerUserID: uuid.NullUUID{UUID: owner.ID, Valid: true}, FullName: "acme/rocket",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM repos WHERE id = $1`, repo.ID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = ANY($1)`, []uuid.UUID{owner.ID, other.ID})
	})
	l, err := q.CreateLaunch(ctx, sqlc.CreateLaunchParams{
		OwnerUserID: owner.ID, Source: "manual", Title: "Meetup", HeadlineChanges: []byte(`[]`), Roles: []byte(`[]`),
		LaunchAt:      pgtype.Timestamptz{Time: time.Now().Add(2 * time.Hour), Valid: true},
		WindowMinutes: 90, CrewCap: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	account := "acct-" + uuid.NewString()
	access, keyID, _ := kr.Encrypt([]byte("tok"))
	if _, err := q.UpsertBufferConnection(ctx, sqlc.UpsertBufferConnectionParams{
		UserID: owner.ID, BufferAccountID: account, OrgID: pgtype.Text{String: "org-1", Valid: true},
		AccessTokenEnc: access, KeyID: keyID, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	fake := bufferfake.New(t, account, "org-1")
	fake.Handle("GetChannels", map[string]any{"channels": []any{}})
	fake.Handle("GetConfiguration", map[string]any{"configuration": map[string]any{"services": []any{}}})
	buf := buffer.New(&http.Client{Timeout: 5 * time.Second}, q, kr, buffer.WithEndpoints(fake.APIURL(), fake.TokenURL()))
	st := &session.Store{Keyring: kr}
	r2 := r2fake.New("cards")
	t.Cleanup(r2.Close)
	deps := launch.Deps{
		Q: q, Pool: pool, Buffer: buf, Keyring: kr, BaseURL: "https://lc.test",
		Cards:  cards.NewUploader(r2.URL, "cards", "ak", "sk", "https://cdn.example"),
		Render: func(any, cards.Preset) ([]byte, error) { return []byte("png"), nil },
		Preflight: preflight.Deps{
			Fetch:        func(context.Context, string) (int, string, error) { return 200, "image/png", nil },
			Limits:       buf.DailyPostingLimits,
			Config:       buf.Configuration,
			RateHeadroom: func(context.Context, uuid.UUID) (int, error) { return 100, nil },
		},
	}
	h := httpserver.New(httpserver.Deps{Captain: api.NewCaptain(deps, st)})
	return &captainEnv{pool: pool, env: &env{ctx: ctx, q: q, st: st, kr: kr, h: h}, owner: owner, other: other, launch: l, repo: repo}
}

func TestCaptainRoutesNeedTheOwner(t *testing.T) {
	e := setupCaptain(t)
	mine := e.cookie(t, session.Session{UserID: e.owner.ID})
	theirs := e.cookie(t, session.Session{UserID: e.other.ID})
	base := "/api/captain/launches/" + e.launch.ID.String()

	rr := e.do("GET", "/api/captain/launches", "", nil, nil)
	if rr.Code != http.StatusUnauthorized || decode(t, rr)["error"] != "sign_in" {
		t.Fatalf("signed out: %d %s", rr.Code, rr.Body)
	}
	for _, c := range []struct{ method, path, body string }{
		{"GET", base, ""},
		{"PATCH", base, `{"title":"mine now"}`},
		{"POST", base + "/launch", ""},
		{"POST", base + "/retry/1", ""},
		{"PATCH", "/api/captain/repos/" + e.repo.ID.String(), `{"enabled":false}`},
	} {
		if rr := e.do(c.method, c.path, c.body, nil, theirs); rr.Code != http.StatusNotFound {
			t.Fatalf("%s %s by another user: %d, want 404", c.method, c.path, rr.Code)
		}
	}
	if rr := e.do("GET", "/api/captain/launches/"+uuid.NewString(), "", nil, mine); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown launch: %d", rr.Code)
	}

	rr = e.do("GET", "/api/captain/launches", "", nil, theirs)
	if rr.Code != 200 || rr.Body.String() != "[]\n" {
		t.Fatalf("another user's list: %d %s", rr.Code, rr.Body)
	}
	rr = e.do("GET", "/api/captain/launches", "", nil, mine)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), e.launch.ID.String()) {
		t.Fatalf("own list: %d %s", rr.Code, rr.Body)
	}

	rr = e.do("GET", base, "", nil, mine)
	v := decode(t, rr)
	if rr.Code != 200 || v["id"] != e.launch.ID.String() || v["status"] != "draft" || v["windowMinutes"] != float64(90) {
		t.Fatalf("detail: %d %s", rr.Code, rr.Body)
	}
	for _, k := range []string{"headlines", "chores", "captainDrafts", "crew", "checks", "deadLetters", "counts"} {
		if _, ok := v[k]; !ok {
			t.Fatalf("detail lacks %q: %s", k, rr.Body)
		}
	}

	rr = e.do("PATCH", base, `{"windowMinutes":2}`, nil, mine)
	if rr.Code != http.StatusBadRequest || decode(t, rr)["error"] != "invalid" {
		t.Fatalf("bad patch: %d %s", rr.Code, rr.Body)
	}
	rr = e.do("PATCH", base, `{"title":"Go meetup","windowMinutes":45}`, nil, mine)
	if rr.Code != 200 || decode(t, rr)["title"] != "Go meetup" {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body)
	}

	if rr := e.do("POST", base+"/launch", "", nil, mine); rr.Code != http.StatusAccepted {
		t.Fatalf("launch: %d %s", rr.Code, rr.Body)
	}
	rr = e.do("POST", base+"/launch", "", nil, mine)
	if rr.Code != http.StatusConflict || decode(t, rr)["error"] != "already_launched" {
		t.Fatalf("second launch: %d %s", rr.Code, rr.Body)
	}

	// Retry: only a dead job of this launch.
	row, err := e.q.EnqueueJob(e.ctx, sqlc.EnqueueJobParams{Kind: "launch", Payload: []byte(`{}`), LaunchID: uuid.NullUUID{UUID: e.launch.ID, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	jobID := row.ID
	retry := fmt.Sprintf("%s/retry/%d", base, jobID)
	if rr := e.do("POST", retry, "", nil, mine); rr.Code != http.StatusNotFound {
		t.Fatalf("retry of a live job: %d", rr.Code)
	}
	if _, err := e.pool.Exec(e.ctx, `UPDATE jobs SET dead = true, attempts = 5 WHERE id = $1`, jobID); err != nil {
		t.Fatal(err)
	}
	if rr := e.do("POST", retry, "", nil, mine); rr.Code != http.StatusAccepted {
		t.Fatalf("retry: %d %s", rr.Code, rr.Body)
	}

	rr = e.do("GET", "/api/captain/repos", "", nil, mine)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"fullName":"acme/rocket"`) {
		t.Fatalf("repos: %d %s", rr.Code, rr.Body)
	}
	rr = e.do("PATCH", "/api/captain/repos/"+e.repo.ID.String(), `{"allowPrerelease":true}`, nil, mine)
	if m := decode(t, rr); rr.Code != 200 || m["allowPrerelease"] != true || m["enabled"] != true {
		t.Fatalf("repo patch: %d %s", rr.Code, rr.Body)
	}
}

func TestCaptainCreateManual(t *testing.T) {
	e := setupCaptain(t)
	mine := e.cookie(t, session.Session{UserID: e.owner.ID})
	at := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Minute).Format(time.RFC3339)
	rr := e.do("POST", "/api/captain/launches", `{"title":"Go night","summary":"Talks","roles":[{"name":"Speaker","hint":"spoke"}],"launchAt":"`+at+`","windowMinutes":60}`, nil, mine)
	v := decode(t, rr)
	if rr.Code != http.StatusCreated || v["title"] != "Go night" || v["source"] != "manual" || v["windowMinutes"] != float64(60) || v["launchAt"] != at {
		t.Fatalf("create: %d %s", rr.Code, rr.Body)
	}
	if v["openUrl"] != "" || v["status"] != "draft" {
		t.Fatalf("draft openUrl %q, want none until launch", v["openUrl"])
	}
	rr = e.do("POST", "/api/captain/launches", `{"title":"  "}`, nil, mine)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty title: %d %s", rr.Code, rr.Body)
	}
}

func TestCaptainSessionForDeletedUserSignsOut(t *testing.T) {
	e := setupCaptain(t)
	gone, err := e.q.CreateUser(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := e.cookie(t, session.Session{UserID: gone.ID})
	if _, err := e.pool.Exec(e.ctx, `DELETE FROM users WHERE id = $1`, gone.ID); err != nil {
		t.Fatal(err)
	}
	rr := e.do("GET", "/api/captain/launches", "", nil, c)
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), `"sign_in"`) {
		t.Fatalf("deleted user: %d %s", rr.Code, rr.Body)
	}
	cleared := false
	for _, ck := range rr.Result().Cookies() {
		if ck.Name == c.Name && ck.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("cookie not cleared: %v", rr.Result().Cookies())
	}
}

func TestCaptainEditDraft(t *testing.T) {
	e := setupCaptain(t)
	mine := e.cookie(t, session.Session{UserID: e.owner.ID})
	var id uuid.UUID
	if err := e.pool.QueryRow(e.ctx,
		`INSERT INTO captain_drafts (launch_id, channel_id, text) VALUES ($1, 'ch-1', 'old') RETURNING id`, e.launch.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	url := "/api/captain/launches/" + e.launch.ID.String() + "/drafts/" + id.String()

	if rr := e.do("PATCH", url, `{"text":"  A better post  "}`, nil, mine); rr.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rr.Code, rr.Body)
	}
	var got string
	if err := e.pool.QueryRow(e.ctx, `SELECT text FROM captain_drafts WHERE id = $1`, id).Scan(&got); err != nil || got != "A better post" {
		t.Fatalf("stored text %q, err %v", got, err)
	}
	if rr := e.do("PATCH", url, `{"text":"   "}`, nil, mine); rr.Code != http.StatusBadRequest {
		t.Fatalf("empty text: %d %s", rr.Code, rr.Body)
	}
	if rr := e.do("PATCH", "/api/captain/launches/"+e.launch.ID.String()+"/drafts/"+uuid.NewString(), `{"text":"x"}`, nil, mine); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown draft: %d %s", rr.Code, rr.Body)
	}
	theirs := e.cookie(t, session.Session{UserID: e.other.ID})
	if rr := e.do("PATCH", url, `{"text":"hijack"}`, nil, theirs); rr.Code != http.StatusNotFound {
		t.Fatalf("someone else's launch: %d %s", rr.Code, rr.Body)
	}
	if _, err := e.pool.Exec(e.ctx, `UPDATE captain_drafts SET buffer_post_id = 'p1' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if rr := e.do("PATCH", url, `{"text":"too late"}`, nil, mine); rr.Code != http.StatusBadRequest {
		t.Fatalf("sent draft: %d %s", rr.Code, rr.Body)
	}
}
