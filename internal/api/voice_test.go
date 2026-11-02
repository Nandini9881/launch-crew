package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/api"
	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/httpserver"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
	"github.com/shubham-tomar/launchcrew/internal/session"
)

// voiceEnv binds a member to a fresh connection and returns a handler whose
// deps enqueue into the real jobs table.
func voiceEnv(t *testing.T) (e *env, h http.Handler, id uuid.UUID, base string, owner *http.Cookie, acct string) {
	t.Helper()
	e = setup(t)
	id, _, err := crew.ChooseRole(e.ctx, e.deps, e.launch.ID, "Speaker", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	c := e.conn(t)
	if _, err := crew.Connect(e.ctx, e.deps, id, c.ID, uuid.Nil); err != nil {
		t.Fatal(err)
	}
	d := e.deps
	d.Enqueue = func(ctx context.Context, k jobs.Kind, p any, at time.Time, l *uuid.UUID) (int64, error) {
		return jobs.Enqueue(ctx, e.q, k, p, at, l)
	}
	e.deps = d
	h = httpserver.New(httpserver.Deps{Crew: api.NewCrew(d, e.st)})
	return e, h, id, "/api/crew/" + id.String(), e.cookie(t, session.Session{UserID: c.UserID, ConnectionID: c.ID}), c.BufferAccountID
}

func put(h http.Handler, target, body string, c *http.Cookie, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("PUT", target, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if c != nil {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func liveVoiced(t *testing.T, e *env, id uuid.UUID) []sqlc.Job {
	t.Helper()
	rows, err := e.deps.Pool.Query(e.ctx, `SELECT payload FROM jobs WHERE kind = $1 AND dead = false AND payload->>'MemberID' = $2`, string(jobs.VoicedDrafts), id.String())
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []sqlc.Job
	for rows.Next() {
		var j sqlc.Job
		if err := rows.Scan(&j.Payload); err != nil {
			t.Fatal(err)
		}
		out = append(out, j)
	}
	return out
}

func TestVoiceUpdatePersistsAndEnqueuesOnce(t *testing.T) {
	e, h, id, base, owner, acct := voiceEnv(t)
	edited, err := e.q.InsertCrewDraftIfAbsent(e.ctx, sqlc.InsertCrewDraftIfAbsentParams{
		CrewMemberID: id, ChannelID: "x1", Network: "twitter", Text: "template", Thread: []byte(`[]`),
		FirstComment: pgtype.Text{}, HookType: pgtype.Text{}, OverlapScore: pgtype.Float4{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := crew.SaveEdit(e.ctx, e.deps, edited.ID, "my own words"); err != nil {
		t.Fatal(err)
	}
	// A token alone cannot mutate a bound row.
	if rr := put(h, base+"/voice", `{}`, nil, map[string]string{"X-Crew-Token": "whatever"}); rr.Code != 404 {
		t.Fatalf("token-only voice PUT: %d", rr.Code)
	}

	// A plain job from connect is live; it must not suppress the forced rewrite.
	if _, err := jobs.Enqueue(e.ctx, e.q, jobs.VoicedDrafts, crew.VoicedPayload{MemberID: id}, time.Time{}, nil); err != nil {
		t.Fatal(err)
	}

	body := `{"sentenceLength":"short","case":"lowercase","hashtags":"few","extra":"never say excited"}`
	rr := put(h, base+"/voice", body, owner, nil)
	if rr.Code != 200 || decode(t, rr)["voiceLine"] == "" {
		t.Fatalf("first PUT: %d %s", rr.Code, rr.Body)
	}
	body2 := strings.Replace(body, "short", "long", 1)
	if rr := put(h, base+"/voice", body2, owner, nil); rr.Code != 200 {
		t.Fatalf("second PUT: %d %s", rr.Code, rr.Body)
	}

	vp, err := e.q.GetVoiceProfile(e.ctx, acct)
	if err != nil || vp.DerivedFrom != "manual" {
		t.Fatalf("voice row = %+v, %v", vp, err)
	}
	var m map[string]string
	_ = json.Unmarshal(vp.Profile, &m)
	if m["sentenceLength"] != "long" || m["case"] != "lowercase" || m["hashtags"] != "few" || m["extra"] != "never say excited" {
		t.Fatalf("stored profile = %s", vp.Profile)
	}
	js := liveVoiced(t, e, id)
	forced := 0
	for _, j := range js {
		var pl crew.VoicedPayload
		if json.Unmarshal(j.Payload, &pl) == nil && pl.Force {
			forced++
		}
	}
	if len(js) != 2 || forced != 1 {
		t.Fatalf("live jobs = %d, forced = %d, want 2 and 1", len(js), forced)
	}

	// Run it: the edited draft is untouched.
	for i := 0; i < 30 && len(liveVoiced(t, e, id)) > 0; i++ {
		if _, err := jobs.RunOnce(e.ctx, e.q, map[jobs.Kind]jobs.Handler{jobs.VoicedDrafts: crew.VoicedDraftsHandler(e.deps)}, "test"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := e.q.GetCrewDraft(e.ctx, edited.ID)
	if err != nil || got.Text != "my own words" || !got.Edited {
		t.Fatalf("edited draft = %+v, %v", got, err)
	}
}

func TestExtraOver80Rejected(t *testing.T) {
	e, h, id, base, owner, acct := voiceEnv(t)
	extra := strings.Repeat("é", 81)
	rr := put(h, base+"/voice", `{"sentenceLength":"short","case":"lowercase","hashtags":"none","extra":"`+extra+`"}`, owner, nil)
	if rr.Code != 400 || decode(t, rr)["error"] != "invalid_voice" {
		t.Fatalf("81 runes: %d %s", rr.Code, rr.Body)
	}
	if _, err := e.q.GetVoiceProfile(e.ctx, acct); err == nil {
		t.Fatal("voice stored")
	}
	if n := len(liveVoiced(t, e, id)); n != 0 {
		t.Fatalf("enqueued %d", n)
	}
	if rr := put(h, base+"/voice", `{"sentenceLength":"huge","case":"lowercase","hashtags":"none"}`, owner, nil); rr.Code != 400 {
		t.Fatalf("bad enum: %d", rr.Code)
	}
}
