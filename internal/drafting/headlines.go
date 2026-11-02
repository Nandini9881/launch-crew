package drafting

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

//go:embed prompts/headlines_v1.txt
var headlinesPrompt string

// picksSchema is the JSON schema of the model's answer.
var picksSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"headlines": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pr":    map[string]any{"type": "integer"},
					"title": map[string]any{"type": "string"},
				},
				"required":             []string{"pr", "title"},
				"additionalProperties": false,
			},
		},
		"chores": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
	},
	"required":             []string{"headlines", "chores"},
	"additionalProperties": false,
}

// PR is one merged pull request in a release.
type PR struct {
	Number      int
	Title, Body string
	Added       int
	Deleted     int
	Labels      []string
	AuthorID    int64
	AuthorLogin string
	// AuthorType is GitHub's account type ("User", "Bot"); AuthorAvatarURL its avatar.
	AuthorType      string
	AuthorAvatarURL string
}

type pickResult struct {
	Headlines []struct {
		PR    int    `json:"pr"`
		Title string `json:"title"`
	} `json:"headlines"`
	Chores []int `json:"chores"`
}

// PickHeadlines picks the 2 to 3 headline changes of a release; the rest are chores.
func PickHeadlines(ctx context.Context, c *Claude, notes string, prs []PR) (headlines []Change, chores []Change, err error) {
	if len(prs) == 0 {
		return nil, nil, nil
	}
	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     claudeModel,
		MaxTokens: claudeMaxTokens,
		System:    []anthropic.TextBlockParam{{Text: headlinesPrompt}},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: picksSchema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(renderPRs(notes, prs))),
		},
	}, requestTimeout(ctx))
	if err != nil {
		return nil, nil, fmt.Errorf("claude headlines: %w", err)
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return nil, nil, ErrRefused
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	var got pickResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(text.String())), &got); err != nil {
		return nil, nil, fmt.Errorf("claude headlines: bad JSON output: %w", err)
	}
	headlines, chores = mapPicks(got, prs)
	return headlines, chores, nil
}

func mapPicks(got pickResult, prs []PR) (headlines, chores []Change) {
	byNum := make(map[int]PR, len(prs))
	for _, p := range prs {
		byNum[p.Number] = p
	}
	change := func(p PR, title string) Change {
		if title == "" {
			title = p.Title
		}
		return Change{PRNumber: p.Number, Title: title, AuthorID: p.AuthorID, AuthorLogin: p.AuthorLogin}
	}
	chore := map[int]bool{}
	for _, n := range got.Chores {
		if _, ok := byNum[n]; ok {
			chore[n] = true
		}
	}
	picked := map[int]bool{}
	for _, h := range got.Headlines {
		p, ok := byNum[h.PR]
		if !ok || picked[h.PR] || chore[h.PR] {
			continue
		}
		picked[h.PR] = true
		headlines = append(headlines, change(p, strings.TrimSpace(h.Title)))
	}
	if len(headlines) > 3 {
		for _, h := range headlines[3:] {
			picked[h.PRNumber] = false
		}
		headlines = headlines[:3]
	}
	for _, p := range prs { // fill up to 2 from non-chore PRs in input order
		if len(headlines) >= 2 {
			break
		}
		if !picked[p.Number] && !chore[p.Number] {
			picked[p.Number] = true
			headlines = append(headlines, change(p, ""))
		}
	}
	for _, p := range prs {
		if !picked[p.Number] {
			chores = append(chores, change(p, ""))
		}
	}
	return headlines, chores
}

func renderPRs(notes string, prs []PR) string {
	var b strings.Builder
	if strings.TrimSpace(notes) != "" {
		fmt.Fprintf(&b, "Release notes:\n%s\n\n", notes)
	}
	b.WriteString("Merged pull requests:\n")
	for _, p := range prs {
		body := strings.TrimSpace(p.Body)
		if len(body) > 400 {
			body = body[:400]
		}
		fmt.Fprintf(&b, "#%d %s (+%d -%d", p.Number, p.Title, p.Added, p.Deleted)
		if len(p.Labels) > 0 {
			fmt.Fprintf(&b, ", labels: %s", strings.Join(p.Labels, ", "))
		}
		b.WriteString(")\n")
		if body != "" {
			fmt.Fprintf(&b, "%s\n", body)
		}
	}
	return b.String()
}
