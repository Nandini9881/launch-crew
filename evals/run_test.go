//go:build eval

// Package evals runs the drafter against the real API. Run with
// ANTHROPIC_API_KEY=... go test -tags eval ./evals/
package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/shubham-tomar/launchcrew/internal/drafting"
)

type fixturePR struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Added       int      `json:"added"`
	Deleted     int      `json:"deleted"`
	Labels      []string `json:"labels"`
	AuthorID    int64    `json:"author_id"`
	AuthorLogin string   `json:"author_login"`
}

type fixture struct {
	Project  string      `json:"project"`
	Version  string      `json:"version"`
	Notes    string      `json:"notes"`
	PRs      []fixturePR `json:"prs"`
	ChorePRs []int       `json:"chore_prs"`
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	files, err := filepath.Glob("fixtures/releases/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	var out []fixture
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var fx fixture
		if err := json.Unmarshal(raw, &fx); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out = append(out, fx)
	}
	return out
}

func TestEvalDraftsAndHeadlines(t *testing.T) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	c := drafting.NewClaude(key)
	fixtures := loadFixtures(t)
	voice := &drafting.Profile{SentenceLength: "short", Case: "lowercase", Hashtags: "none", Emoji: "none"}
	for i := 0; i < 20; i++ {
		fx := fixtures[i%len(fixtures)]
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		prs := make([]drafting.PR, len(fx.PRs))
		for j, p := range fx.PRs {
			prs[j] = drafting.PR{Number: p.Number, Title: p.Title, Body: p.Body, Added: p.Added, Deleted: p.Deleted, Labels: p.Labels, AuthorID: p.AuthorID, AuthorLogin: p.AuthorLogin}
		}
		heads, _, err := drafting.PickHeadlines(ctx, c, fx.Notes, prs)
		if err != nil {
			cancel()
			t.Fatalf("run %d %s: headlines: %v", i, fx.Project, err)
		}
		if len(heads) < 2 || len(heads) > 3 {
			t.Errorf("run %d %s: %d headlines", i, fx.Project, len(heads))
		}
		for _, h := range heads {
			if slices.Contains(fx.ChorePRs, h.PRNumber) {
				t.Errorf("run %d %s: chore #%d picked as headline", i, fx.Project, h.PRNumber)
			}
		}
		if len(heads) == 0 {
			cancel()
			continue
		}
		d, err := c.Draft(ctx, drafting.DraftRequest{
			Subject: drafting.Subject{Project: fx.Project, Version: fx.Version, Headline: heads[0].Title},
			Voice:   voice,
			Hook:    drafting.HookDetail,
			Network: drafting.Bluesky,
			Rules:   drafting.Rules{MaxChars: 300},
		})
		cancel()
		if err != nil {
			t.Fatalf("run %d %s: draft: %v", i, fx.Project, err)
		}
		if tells := drafting.Lint(d.Text); tells != nil {
			t.Errorf("run %d %s: lint tells %v in %q", i, fx.Project, tells, d.Text)
		}
	}
}
