package trigger

import "github.com/shubham-tomar/launchcrew/internal/github/comments"

// RollCallThreshold is the most ticked crew that still get per-PR comments.
const RollCallThreshold = 20

// Plan is what SendComments posts: per-PR bodies keyed by PR number, or one
// roll call for the release. Bodies carry no marker; the poster appends it.
type Plan struct {
	PerPR    map[int]string
	RollCall string
}

// PlanComments decides between per-PR comments and a roll call. links maps a
// GitHub user id to that member's share link; ticked members without a link
// are left out.
func PlanComments(spec LaunchSpec, links map[int64]string, settingsLink string) Plan {
	var ticked []CrewSuggestion
	for _, c := range spec.Crew {
		if c.Ticked {
			ticked = append(ticked, c)
		}
	}
	if len(ticked) > RollCallThreshold {
		in := comments.RollCallInput{Captain: spec.Captain, Project: spec.Project(), Version: spec.ReleaseTag, SettingsLink: settingsLink}
		for _, c := range ticked {
			if l := links[c.GitHubUserID]; l != "" {
				in.Entries = append(in.Entries, comments.Entry{Login: c.Login, Link: l})
			}
		}
		return Plan{RollCall: comments.RollCall(in)}
	}
	p := Plan{PerPR: map[int]string{}}
	for _, c := range ticked {
		if l := links[c.GitHubUserID]; l != "" {
			prs := make([]comments.PR, len(c.PRs))
			for i, r := range c.PRs {
				prs[i] = comments.PR{Number: r.Number, Title: r.Title}
			}
			if len(prs) == 0 {
				prs = []comments.PR{{Number: c.PRNumber, Title: c.PRTitle}}
			}
			// The comment goes on the most recent PR.
			p.PerPR[c.PRNumber] = comments.Body(spec.Captain, spec.Project(), spec.ReleaseTag, prs, l, settingsLink)
		}
	}
	return p
}
