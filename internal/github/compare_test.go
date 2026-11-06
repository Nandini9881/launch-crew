package github_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/github"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/githubfake"
)

func sha(i int) string { return fmt.Sprintf("%040x", i) }

func merged() *string { s := "2026-10-20T10:00:00Z"; return &s }

func mergedPR(n int, mergeSHA string) githubfake.PR {
	return githubfake.PR{Number: n, Title: fmt.Sprintf("PR %d", n), State: "closed", Merged: true, MergedAt: merged(), MergeCommitSHA: mergeSHA}
}

func client(t *testing.T, f *githubfake.Server) *github.Client {
	t.Helper()
	c, err := github.New(1, githubfake.PrivateKeyPEM(), github.WithBaseURL(f.URL))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// commitPullCalls counts GET /commits/{sha}/pulls requests.
func commitPullCalls(f *githubfake.Server) int {
	n := 0
	for _, r := range f.Requests("GET", "/commits/") {
		if strings.HasSuffix(r.Path, "/pulls") {
			n++
		}
	}
	return n
}

func TestPRsBetweenReadsSubjectSuffixFirst(t *testing.T) {
	f := githubfake.New("acme", "rocket")
	t.Cleanup(f.Close)
	f.Commits = []githubfake.Commit{
		githubfake.NewCommit(sha(0), "Initial commit"),
		githubfake.NewCommit(sha(1), "Release v1"),
		githubfake.NewCommit(sha(2), "Squash merge (#1)\n\nbody"),
		githubfake.NewCommit(sha(3), "Rebase merge, no suffix"),
		githubfake.NewCommit(sha(4), "Second squash (#2)"),
		githubfake.NewCommit(sha(5), "Still open (#5)"),
		githubfake.NewCommit(sha(6), "Points at an issue (#6)"),
		githubfake.NewCommit(sha(7), "Follow-up for the same PR (#2)"),
	}
	f.PRs = []githubfake.PR{
		mergedPR(1, sha(2)),
		mergedPR(7, sha(3)), // only the commit endpoint knows this one
		mergedPR(2, sha(4)),
		{Number: 5, Title: "PR 5", State: "open"},
	}
	f.Tags = []githubfake.Tag{{Name: "v2", SHA: sha(7)}, {Name: "v1", SHA: sha(1)}}

	prs, err := client(t, f).PRsBetween(context.Background(), 1, "acme", "rocket", "v1", "v2")
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, p := range prs {
		got = append(got, p.Number)
	}
	if fmt.Sprint(got) != "[1 7 2]" {
		t.Fatalf("PRs = %v, want [1 7 2] (oldest first, unmerged and missing skipped)", got)
	}
	if n := commitPullCalls(f); n != 1 {
		t.Fatalf("%d commit->pulls calls, want 1 (only the commit without a suffix)", n)
	}
	if n := len(f.Requests("GET", "/pulls/")); n != 5 {
		t.Fatalf("%d PR fetches, want 5 (1, 7, 2, 5, 6; #2 fetched once)", n)
	}
}

func TestPRsBetweenWalksNewestFiveHundredCommits(t *testing.T) {
	f := githubfake.New("acme", "rocket")
	t.Cleanup(f.Close)
	const total = 505
	f.Commits = []githubfake.Commit{githubfake.NewCommit(sha(0), "Initial commit"), githubfake.NewCommit(sha(1), "Release v1")}
	for i := 1; i <= total; i++ {
		s := sha(100 + i)
		f.Commits = append(f.Commits, githubfake.NewCommit(s, fmt.Sprintf("Change %d (#%d)", i, i)))
		f.PRs = append(f.PRs, mergedPR(i, s))
	}
	f.Tags = []githubfake.Tag{{Name: "v2", SHA: sha(100 + total)}, {Name: "v1", SHA: sha(1)}}

	prs, err := client(t, f).PRsBetween(context.Background(), 1, "acme", "rocket", "v1", "v2")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 500 || prs[0].Number != 6 || prs[499].Number != total {
		t.Fatalf("got %d PRs, first #%d last #%d; want the newest 500 (#6..#%d)", len(prs), prs[0].Number, prs[len(prs)-1].Number, total)
	}
	if n := commitPullCalls(f); n != 0 {
		t.Fatalf("%d commit->pulls calls, want 0 when every subject carries its PR", n)
	}
	if n := len(f.Requests("GET", "/pulls/")); n != 500 {
		t.Fatalf("%d PR fetches, want 500", n)
	}
}
