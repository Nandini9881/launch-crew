package github

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v93/github"

	"github.com/shubham-tomar/launchcrew/internal/drafting"
)

const (
	maxCommits = 500 // newest commits walked per release
	maxPages   = 10
)

// prSuffix matches the "(#123)" GitHub appends to squash-merge subjects.
var prSuffix = regexp.MustCompile(`\(#(\d+)\)\s*$`)

// PRsBetween lists the merged PRs whose commits are in base...head, oldest first.
//
// Each commit is mapped to its PRs with the "pulls associated with a commit"
// endpoint; when that returns no merged PR (rebase merges, history rewritten
// after merge) the "(#N)" suffix of the commit subject is used instead. An
// empty base means a first release: the compare runs from the repository's
// root commit, so the whole history up to head counts (newest maxCommits).
func (c *Client) PRsBetween(ctx context.Context, installID int64, owner, repo, base, head string) ([]drafting.PR, error) {
	cl, err := c.installation(installID)
	if err != nil {
		return nil, err
	}
	var shas []commitRef
	if base == "" {
		root, err := rootCommit(ctx, cl, owner, repo, head)
		if err != nil {
			return nil, err
		}
		shas = append(shas, root)
		base = root.sha
	}
	between, err := compare(ctx, cl, owner, repo, base, head)
	if err != nil {
		return nil, err
	}
	shas = append(shas, between...)
	if len(shas) > maxCommits {
		shas = shas[len(shas)-maxCommits:]
	}

	var numbers []int
	seen := map[int]bool{}
	add := func(n int) {
		if !seen[n] {
			seen[n] = true
			numbers = append(numbers, n)
		}
	}
	for _, cm := range shas {
		prs, _, err := cl.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, cm.sha, nil)
		if err != nil {
			return nil, fmt.Errorf("pulls for %s: %w", cm.sha, err)
		}
		found := false
		for _, p := range prs {
			if p.MergedAt != nil && p.Number != nil {
				add(*p.Number)
				found = true
			}
		}
		if !found {
			subject, _, _ := strings.Cut(cm.message, "\n")
			if m := prSuffix.FindStringSubmatch(subject); m != nil {
				n, _ := strconv.Atoi(m[1])
				add(n)
			}
		}
	}

	out := make([]drafting.PR, 0, len(numbers))
	for _, n := range numbers {
		p, resp, err := cl.PullRequests.Get(ctx, owner, repo, n)
		if resp != nil && resp.StatusCode == 404 {
			continue // "(#N)" pointed at an issue or another repo's PR
		}
		if err != nil {
			return nil, fmt.Errorf("get PR #%d: %w", n, err)
		}
		if p.MergedAt == nil {
			continue
		}
		out = append(out, toPR(p))
	}
	return out, nil
}

func toPR(p *gh.PullRequest) drafting.PR {
	pr := drafting.PR{
		Number:  p.GetNumber(),
		Title:   p.GetTitle(),
		Body:    p.GetBody(),
		Added:   p.GetAdditions(),
		Deleted: p.GetDeletions(),
	}
	for _, l := range p.Labels {
		if l != nil {
			pr.Labels = append(pr.Labels, l.Name)
		}
	}
	if u := p.User; u != nil {
		pr.AuthorID, pr.AuthorLogin = u.GetID(), u.GetLogin()
		pr.AuthorType, pr.AuthorAvatarURL = u.GetType(), u.GetAvatarURL()
	}
	return pr
}

type commitRef struct{ sha, message string }

func refOf(rc *gh.RepositoryCommit) commitRef {
	r := commitRef{sha: rc.GetSHA()}
	if rc.Commit != nil {
		r.message = rc.Commit.GetMessage()
	}
	return r
}

// compare returns the commits in base...head, oldest first.
func compare(ctx context.Context, cl *gh.Client, owner, repo, base, head string) ([]commitRef, error) {
	var out []commitRef
	opts := &gh.ListOptions{PerPage: 100}
	for range maxPages {
		cmp, resp, err := cl.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
		if err != nil {
			return nil, fmt.Errorf("compare %s...%s: %w", base, head, err)
		}
		for _, rc := range cmp.Commits {
			out = append(out, refOf(rc))
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// rootCommit finds the oldest commit reachable from head: the last entry of
// the last page of the commit list.
func rootCommit(ctx context.Context, cl *gh.Client, owner, repo, head string) (commitRef, error) {
	opts := &gh.CommitsListOptions{SHA: head, ListOptions: gh.ListOptions{PerPage: 1}}
	list, resp, err := cl.Repositories.ListCommits(ctx, owner, repo, opts)
	if err != nil {
		return commitRef{}, fmt.Errorf("list commits: %w", err)
	}
	if resp.LastPage > 1 {
		opts.Page = resp.LastPage
		if list, _, err = cl.Repositories.ListCommits(ctx, owner, repo, opts); err != nil {
			return commitRef{}, fmt.Errorf("list commits: %w", err)
		}
	}
	if len(list) == 0 {
		return commitRef{}, fmt.Errorf("no commits reachable from %s", head)
	}
	return refOf(list[len(list)-1]), nil
}

// PreviousTag returns the tag released before head, or "" when head is the
// first (the caller then uses the full history).
//
// Releases are checked first, newest first, skipping drafts, and skipping
// prereleases when head is a full release. When head has no older release the
// tag list (newest first) decides.
func (c *Client) PreviousTag(ctx context.Context, installID int64, owner, repo, head string) (string, error) {
	cl, err := c.installation(installID)
	if err != nil {
		return "", err
	}
	var rels []*gh.RepositoryRelease
	opts := &gh.ListOptions{PerPage: 100}
	for range maxPages {
		page, resp, err := cl.Repositories.ListReleases(ctx, owner, repo, opts)
		if err != nil {
			return "", fmt.Errorf("list releases: %w", err)
		}
		rels = append(rels, page...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	for i, r := range rels {
		if r.TagName != head {
			continue
		}
		for _, older := range rels[i+1:] {
			if older.Draft || (older.Prerelease && !r.Prerelease) {
				continue
			}
			return older.TagName, nil
		}
		break
	}

	opts = &gh.ListOptions{PerPage: 100}
	var tags []string
	for range maxPages {
		page, resp, err := cl.Repositories.ListTags(ctx, owner, repo, opts)
		if err != nil {
			return "", fmt.Errorf("list tags: %w", err)
		}
		for _, t := range page {
			tags = append(tags, t.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	for i, t := range tags {
		if t == head && i+1 < len(tags) {
			return tags[i+1], nil
		}
	}
	return "", nil
}
