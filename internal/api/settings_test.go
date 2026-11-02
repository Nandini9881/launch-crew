package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/api"
	"github.com/shubham-tomar/launchcrew/internal/cards"
	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/drafting"
	"github.com/shubham-tomar/launchcrew/internal/github"
	"github.com/shubham-tomar/launchcrew/internal/httpserver"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
	"github.com/shubham-tomar/launchcrew/internal/session"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/githubfake"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/r2fake"
	"github.com/shubham-tomar/launchcrew/internal/trigger"
	"github.com/shubham-tomar/launchcrew/internal/webhook"
)

// settingsEnv is the crew env served together with the settings routes.
func settingsEnv(t *testing.T) *env {
	t.Helper()
	e := setup(t)
	e.h = httpserver.New(httpserver.Deps{
		Crew:     api.NewCrew(e.deps, e.st),
		Settings: api.NewSettings(e.deps, e.st, "help@lc.test"),
	})
	return e
}

// person is a user with a linked GitHub id; everything it owns is removed after the test.
func (e *env) person(t *testing.T, gh int64, login string) sqlc.User {
	t.Helper()
	u, err := e.q.CreateUser(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.UpsertGitHubIdentity(e.ctx, sqlc.UpsertGitHubIdentityParams{GithubUserID: gh, UserID: u.ID, Login: login}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = e.deps.Pool.Exec(bg, `DELETE FROM opt_outs WHERE github_user_id = $1`, gh)
		_, _ = e.deps.Pool.Exec(bg, `DELETE FROM github_identities WHERE github_user_id = $1`, gh)
		_, _ = e.deps.Pool.Exec(bg, `DELETE FROM users WHERE id = $1`, u.ID)
	})
	return u
}

func (e *env) crewRow(t *testing.T, launch uuid.UUID, user *sqlc.User, gh int64, state string) (sqlc.CrewMember, string) {
	t.Helper()
	tok, hash := crew.NewToken()
	p := sqlc.InsertCrewMemberParams{
		LaunchID: launch, Role: "contributor", State: state, InviteTokenHash: hash,
		TokenExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}, // expired on purpose
	}
	if gh != 0 {
		p.GithubUserID = pgtype.Int8{Int64: gh, Valid: true}
	}
	if user != nil {
		p.UserID = uuid.NullUUID{UUID: user.ID, Valid: true}
	}
	m, err := e.q.InsertCrewMember(e.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	return m, tok
}

func (e *env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.deps.Pool.QueryRow(e.ctx, query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSettingsMeShapes(t *testing.T) {
	e := settingsEnv(t)
	rr := e.do("GET", "/api/me", "", nil, nil)
	v := decode(t, rr)
	if rr.Code != 200 || v["signedIn"] != false || v["supportEmail"] != "help@lc.test" || len(v["githubLogins"].([]any)) != 0 {
		t.Fatalf("signed out: %d %s", rr.Code, rr.Body)
	}
	gh := 5_000_000_000 + int64(uuid.New().ID())
	u := e.person(t, gh, "dana")
	e.crewRow(t, e.launch.ID, &u, gh, "ticked")
	c := e.cookie(t, session.Session{UserID: u.ID})
	rr = e.do("GET", "/api/me", "", nil, c)
	v = decode(t, rr)
	if rr.Code != 200 || v["signedIn"] != true || v["bufferConnected"] != false || v["optedOut"] != false ||
		v["launches"] != float64(0) || v["crewRows"] != float64(1) || v["githubLogins"].([]any)[0] != "dana" {
		t.Fatalf("signed in: %d %s", rr.Code, rr.Body)
	}
	for _, p := range []string{"/api/me/opt-out"} {
		if rr := e.do("POST", p, "", nil, nil); rr.Code != http.StatusUnauthorized || decode(t, rr)["error"] != "sign_in" {
			t.Fatalf("POST %s signed out: %d", p, rr.Code)
		}
	}
	// A session with no linked GitHub id records nothing and says so.
	lone, _ := e.q.CreateUser(e.ctx)
	t.Cleanup(func() { _, _ = e.deps.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, lone.ID) })
	before := e.count(t, `SELECT count(*) FROM opt_outs`)
	rr = e.do("POST", "/api/me/opt-out", "", nil, e.cookie(t, session.Session{UserID: lone.ID}))
	if rr.Code != http.StatusConflict || decode(t, rr)["error"] != "no_github" {
		t.Fatalf("no linked github: %d %s", rr.Code, rr.Body)
	}
	if e.count(t, `SELECT count(*) FROM opt_outs`) != before {
		t.Fatal("opt_outs changed")
	}
	if rr := e.do("DELETE", "/api/me", "", nil, nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("DELETE signed out: %d", rr.Code)
	}
	// The crew routes still answer next to the settings routes.
	if rr := e.do("GET", "/api/s/unknown", "", nil, nil); rr.Code != http.StatusNotFound {
		t.Fatalf("crew route: %d", rr.Code)
	}
}

func TestOptOutByTokenNeedsNoSession(t *testing.T) {
	e := settingsEnv(t)
	gh := 5_000_000_000 + int64(uuid.New().ID())
	t.Cleanup(func() { _, _ = e.deps.Pool.Exec(e.ctx, `DELETE FROM opt_outs WHERE github_user_id = $1`, gh) })
	m, tok := e.crewRow(t, e.launch.ID, nil, gh, "invited")

	if rr := e.do("POST", "/api/opt-out?token=nope", "", nil, nil); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown token: %d", rr.Code)
	}
	if rr := e.do("POST", "/api/opt-out", "", nil, nil); rr.Code != http.StatusNotFound {
		t.Fatalf("no token: %d", rr.Code)
	}
	// The same person has a ticked row on a second launch.
	l2, err := e.q.CreateLaunch(e.ctx, sqlc.CreateLaunchParams{
		OwnerUserID: e.launch.OwnerUserID.UUID, Source: "manual", Title: "Second", HeadlineChanges: []byte(`[]`), Roles: []byte(`[]`), WindowMinutes: 90, CrewCap: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, _ := e.crewRow(t, l2.ID, nil, gh, "ticked")
	rr := e.do("POST", "/api/opt-out?token="+tok, "", nil, nil) // the token is expired
	if rr.Code != 200 || decode(t, rr)["optedOut"] != true {
		t.Fatalf("opt out: %d %s", rr.Code, rr.Body)
	}
	if out, _ := e.q.IsOptedOut(e.ctx, gh); !out {
		t.Fatal("github id not opted out")
	}
	if got, _ := e.q.GetCrewMember(e.ctx, m.ID); got.State != "opted_out" {
		t.Fatalf("state %s", got.State)
	}

	if got, _ := e.q.GetCrewMember(e.ctx, second.ID); got.State != "opted_out" {
		t.Fatalf("row on the second launch is %s, want opted_out", got.State)
	}

	// A queued row keeps its state, and a row with no GitHub id adds nothing.
	q, qtok := e.crewRow(t, e.launch.ID, nil, gh+1, "queued")
	if rr := e.do("POST", "/api/opt-out?token="+qtok, "", nil, nil); rr.Code != 200 {
		t.Fatalf("queued: %d", rr.Code)
	}
	if got, _ := e.q.GetCrewMember(e.ctx, q.ID); got.State != "queued" {
		t.Fatalf("queued row became %s", got.State)
	}
	_, _ = e.deps.Pool.Exec(e.ctx, `DELETE FROM opt_outs WHERE github_user_id = $1`, gh+1)
	before := e.count(t, `SELECT count(*) FROM opt_outs`)
	_, ntok := e.crewRow(t, e.launch.ID, nil, 0, "invited")
	if rr := e.do("POST", "/api/opt-out?token="+ntok, "", nil, nil); rr.Code != 200 {
		t.Fatalf("no github id: %d", rr.Code)
	}
	if after := e.count(t, `SELECT count(*) FROM opt_outs`); after != before {
		t.Fatalf("opt_outs grew from %d to %d", before, after)
	}
}

func TestOptOutRateLimited(t *testing.T) {
	e := settingsEnv(t)
	_, tok := e.crewRow(t, e.launch.ID, nil, 0, "invited")
	hdr := map[string]string{"Fly-Client-IP": "203.0.113.77"}
	var last int
	for range 12 {
		last = e.do("POST", "/api/opt-out?token="+tok, "", hdr, nil).Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("12th try: %d, want 429", last)
	}
}

// releaseJob runs the release trigger for acme/rocket, whose merged PRs are
// by users 91001..91004, against a fresh enabled repo.
func releaseJob(t *testing.T, e *env, owner uuid.UUID) (sqlc.Repo, jobs.Handler) {
	t.Helper()
	readJSON := func(name string, v any) {
		b, err := os.ReadFile("../trigger/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	var cmp struct {
		BaseCommit githubfake.Commit   `json:"base_commit"`
		Commits    []githubfake.Commit `json:"commits"`
	}
	readJSON("compare.json", &cmp)
	f := githubfake.New("acme", "rocket")
	t.Cleanup(f.Close)
	readJSON("prs.json", &f.PRs)
	f.Commits = append([]githubfake.Commit{githubfake.NewCommit("a000000000000000000000000000000000000001", "Initial commit"), cmp.BaseCommit}, cmp.Commits...)
	head := cmp.Commits[len(cmp.Commits)-1].SHA
	f.Tags = []githubfake.Tag{{Name: "v1.2.0", SHA: head}, {Name: "v1.1.0", SHA: cmp.BaseCommit.SHA}}
	f.Releases = []githubfake.Release{{ID: 1001, TagName: "v1.2.0", Name: "Rocket 1.2"}, {ID: 1000, TagName: "v1.1.0", Name: "Rocket 1.1"}}
	gc, err := github.New(1, githubfake.PrivateKeyPEM(), github.WithBaseURL(f.URL))
	if err != nil {
		t.Fatal(err)
	}
	r2 := r2fake.New("cards")
	t.Cleanup(r2.Close)

	install := 7_000_000_000 + int64(uuid.New().ID())
	repo, err := e.q.UpsertRepo(e.ctx, sqlc.UpsertRepoParams{
		InstallationID: install, OwnerUserID: uuid.NullUUID{UUID: owner, Valid: true}, FullName: "acme/rocket",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = e.deps.Pool.Exec(context.Background(), `DELETE FROM repos WHERE id = $1`, repo.ID) })

	var ev map[string]any
	readJSON("release.json", &ev)
	ev["installation"].(map[string]any)["id"] = install
	payload, _ := json.Marshal(ev)
	id := "settings-" + uuid.NewString()
	if _, err := e.q.InsertWebhookDelivery(e.ctx, sqlc.InsertWebhookDeliveryParams{DeliveryID: id, Event: "release", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.q.DeleteWebhookDelivery(context.Background(), id) })

	h := trigger.ReleaseJob(trigger.ReleaseDeps{
		Pool: e.deps.Pool,
		Trigger: trigger.GitHubRelease{GH: gc, Headlines: func(_ context.Context, _ string, prs []drafting.PR) (h, c []drafting.Change, err error) {
			for _, p := range prs {
				c = append(c, drafting.Change{PRNumber: p.Number, Title: p.Title, AuthorID: p.AuthorID, AuthorLogin: p.AuthorLogin})
			}
			return nil, c, nil
		}},
		Cards: cards.NewUploader(r2.URL, "cards", "ak", "sk", "https://cdn.example"),
	})
	body, _ := json.Marshal(webhook.ProcessReleasePayload{DeliveryID: id})
	return repo, func(ctx context.Context, j jobs.Job) error { j.Payload = body; return h(ctx, j) }
}

func TestOptOutExcludesFromFutureTicks(t *testing.T) {
	e := settingsEnv(t)
	const carol = 91003
	u := e.person(t, carol, "carol")
	other := e.person(t, 5_000_000_000+int64(uuid.New().ID()), "erin")

	// Carol has rows on two launches: one still invited, one already queued.
	queued, err := e.q.CreateLaunch(e.ctx, sqlc.CreateLaunchParams{
		OwnerUserID: other.ID, Source: "manual", Title: "Old", HeadlineChanges: []byte(`[]`), Roles: []byte(`[]`), WindowMinutes: 90, CrewCap: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	invited, _ := e.crewRow(t, e.launch.ID, &u, carol, "invited")
	done, _ := e.crewRow(t, queued.ID, &u, carol, "queued")

	rr := e.do("POST", "/api/me/opt-out", "", nil, e.cookie(t, session.Session{UserID: u.ID}))
	if rr.Code != 200 || decode(t, rr)["optedOut"] != true {
		t.Fatalf("opt out: %d %s", rr.Code, rr.Body)
	}
	if out, _ := e.q.IsOptedOut(e.ctx, carol); !out {
		t.Fatal("carol not in opt_outs")
	}
	if m, _ := e.q.GetCrewMember(e.ctx, invited.ID); m.State != "opted_out" {
		t.Fatalf("invited row is %s", m.State)
	}
	if m, _ := e.q.GetCrewMember(e.ctx, done.ID); m.State != "queued" {
		t.Fatalf("queued row is %s", m.State)
	}
	if v := decode(t, e.do("GET", "/api/me", "", nil, e.cookie(t, session.Session{UserID: u.ID}))); v["optedOut"] != true {
		t.Fatalf("me after opt-out: %v", v)
	}

	repo, run := releaseJob(t, e, other.ID)
	if err := run(e.ctx, jobs.Job{Kind: jobs.ProcessRelease}); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	rows, err := e.deps.Pool.Query(e.ctx, `SELECT m.github_user_id FROM crew_members m JOIN launches l ON l.id = m.launch_id WHERE l.repo_id = $1`, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		t.Fatal("the release suggested nobody, so the check proves nothing")
	}
	for _, id := range ids {
		if id == carol {
			t.Fatalf("opted-out carol was suggested: %v", ids)
		}
	}
}

func TestDeleteRemovesTokensKeepsOptOut(t *testing.T) {
	e := settingsEnv(t)
	gh := 5_000_000_000 + int64(uuid.New().ID())
	u := e.person(t, gh, "frank")
	c := e.cookie(t, session.Session{UserID: u.ID})

	conn := e.conn(t) // a Buffer connection belonging to a stand-in user, re-pointed at frank
	if _, err := e.deps.Pool.Exec(e.ctx, `UPDATE buffer_connections SET user_id = $1 WHERE id = $2`, u.ID, conn.ID); err != nil {
		t.Fatal(err)
	}
	m, tok := e.crewRow(t, e.launch.ID, &u, gh, "queued")
	// Voice profiles keyed by the user's Buffer account and by their crew row's account.
	crewAcct := "acct-" + uuid.NewString()
	if _, err := e.deps.Pool.Exec(e.ctx, `UPDATE crew_members SET buffer_account_id = $1 WHERE id = $2`, crewAcct, m.ID); err != nil {
		t.Fatal(err)
	}
	for _, acct := range []string{conn.BufferAccountID, crewAcct} {
		if _, err := e.q.UpsertVoiceProfile(e.ctx, sqlc.UpsertVoiceProfileParams{BufferAccountID: acct, Profile: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = e.deps.Pool.Exec(context.Background(), `DELETE FROM voice_profiles WHERE buffer_account_id = ANY($1)`, []string{conn.BufferAccountID, crewAcct})
	})
	if _, err := e.q.UpsertCrewDraft(e.ctx, sqlc.UpsertCrewDraftParams{CrewMemberID: m.ID, ChannelID: "c1", Network: "twitter", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	own, err := e.q.CreateLaunch(e.ctx, sqlc.CreateLaunchParams{
		OwnerUserID: u.ID, Source: "manual", Title: "Mine", HeadlineChanges: []byte(`[]`), Roles: []byte(`[]`), WindowMinutes: 90, CrewCap: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.q.InsertCaptainDraftIfAbsent(e.ctx, sqlc.InsertCaptainDraftIfAbsentParams{LaunchID: own.ID, ChannelID: "cap", Text: "x"}); err != nil {
		t.Fatal(err)
	}
	// Someone else's data must survive.
	bystander := e.person(t, gh+1, "gina")
	bm, _ := e.crewRow(t, e.launch.ID, &bystander, gh+1, "ticked")
	// A contributor invited on the captain's own launch keeps their row and opt-out link.
	gh3 := gh + 3
	t.Cleanup(func() {
		_, _ = e.deps.Pool.Exec(context.Background(), `DELETE FROM opt_outs WHERE github_user_id = $1`, gh3)
	})
	contrib, contribTok := e.crewRow(t, own.ID, nil, gh3, "invited")

	if rr := e.do("POST", "/api/me/opt-out", "", nil, c); rr.Code != 200 {
		t.Fatalf("opt out: %d", rr.Code)
	}
	rr := e.do("DELETE", "/api/me", "", nil, c)
	if rr.Code != 200 || decode(t, rr)["deleted"] != true {
		t.Fatalf("delete: %d %s", rr.Code, rr.Body)
	}
	if cs := rr.Result().Cookies(); len(cs) != 1 || cs[0].MaxAge >= 0 {
		t.Fatalf("session cookie not cleared: %v", cs)
	}

	for what, q := range map[string]string{
		"user":       `SELECT count(*) FROM users WHERE id = $1`,
		"connection": `SELECT count(*) FROM buffer_connections WHERE user_id = $1`,
		"identity":   `SELECT count(*) FROM github_identities WHERE user_id = $1`,
		"crew row":   `SELECT count(*) FROM crew_members WHERE user_id = $1`,
		"launch":     `SELECT count(*) FROM launches WHERE owner_user_id = $1`,
	} {
		if n := e.count(t, q, u.ID); n != 0 {
			t.Errorf("%s: %d left", what, n)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM voice_profiles WHERE buffer_account_id = ANY($1)`, []string{conn.BufferAccountID, crewAcct}); n != 0 {
		t.Errorf("voice profiles: %d left", n)
	}
	if n := e.count(t, `SELECT count(*) FROM crew_drafts WHERE crew_member_id = $1`, m.ID); n != 0 {
		t.Errorf("crew drafts: %d left", n)
	}
	if n := e.count(t, `SELECT count(*) FROM captain_drafts WHERE launch_id = $1`, own.ID); n != 0 {
		t.Errorf("captain drafts: %d left", n)
	}
	if out, _ := e.q.IsOptedOut(e.ctx, gh); !out {
		t.Fatal("deletion removed the opt-out")
	}
	if _, err := e.q.GetCrewMember(e.ctx, bm.ID); err != nil {
		t.Fatalf("bystander row gone: %v", err)
	}
	var status string
	var owner pgtype.UUID
	if err := e.deps.Pool.QueryRow(e.ctx, `SELECT status, owner_user_id FROM launches WHERE id = $1`, own.ID).Scan(&status, &owner); err != nil {
		t.Fatalf("the captain's launch is gone: %v", err)
	}
	if status != "deleted" || owner.Valid {
		t.Fatalf("launch status %q, owner valid %v; want deleted and no owner", status, owner.Valid)
	}
	if _, err := e.q.GetCrewMember(e.ctx, contrib.ID); err != nil {
		t.Fatalf("contributor row on the deleted captain's launch gone: %v", err)
	}
	rr = e.do("POST", "/api/opt-out?token="+contribTok, "", nil, nil)
	if rr.Code != 200 {
		t.Fatalf("contributor opt-out link after captain deletion: %d %s", rr.Code, rr.Body)
	}
	if out, _ := e.q.IsOptedOut(e.ctx, gh3); !out {
		t.Fatal("contributor opt-out not recorded")
	}
	// The deleted person's old invite link no longer resolves.
	if rr := e.do("GET", "/api/s/"+tok, "", nil, nil); rr.Code != http.StatusNotFound {
		t.Fatalf("old invite link: %d", rr.Code)
	}

	// Without an opt-out, deletion leaves no trace of the GitHub id.
	gh2 := gh + 2
	u2 := e.person(t, gh2, "hana")
	if rr := e.do("DELETE", "/api/me", "", nil, e.cookie(t, session.Session{UserID: u2.ID})); rr.Code != 200 {
		t.Fatalf("second delete: %d", rr.Code)
	}
	if out, _ := e.q.IsOptedOut(e.ctx, gh2); out {
		t.Fatal("deleting alone opted the person out")
	}
}
