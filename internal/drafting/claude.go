package drafting

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

const (
	claudeModel     = "claude-opus-5-5"
	claudeMaxTokens = 1024
	// defaultTimeout caps a call whose context has no deadline.
	defaultTimeout = 30 * time.Second
)

//go:embed prompts/draft_v1.txt
var draftPrompt string

var draftTmpl = template.Must(template.New("draft_v1").Parse(draftPrompt))

// ErrRefused is returned when the model declines to write the post.
var ErrRefused = errors.New("drafting: model refused")

// Claude drafts posts with the Anthropic API.
type Claude struct {
	client anthropic.Client
}

var _ Drafter = (*Claude)(nil)

// NewClaude returns a Claude drafter. SDK retries are off: the caller owns fallback.
func NewClaude(apiKey string, opts ...option.RequestOption) *Claude {
	all := append([]option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(0)}, opts...)
	return &Claude{client: anthropic.NewClient(all...)}
}

// requestTimeout bounds one request by the context deadline.
func requestTimeout(ctx context.Context) option.RequestOption {
	d := defaultTimeout
	if dl, ok := ctx.Deadline(); ok {
		d = time.Until(dl)
	}
	return option.WithRequestTimeout(d)
}

type draftView struct {
	Profile
	Subject
	Network  Network
	Hook     HookType
	MaxChars int
	Avoid    []string
}

func renderDraftPrompt(req DraftRequest) (string, error) {
	v := draftView{Subject: req.Subject, Network: req.Network, Hook: req.Hook, MaxChars: req.Rules.MaxChars, Avoid: req.Avoid}
	if req.Voice != nil {
		v.Profile = *req.Voice
	}
	var b bytes.Buffer
	if err := draftTmpl.Execute(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

// Draft asks the model for one post.
func (c *Claude) Draft(ctx context.Context, req DraftRequest) (Draft, error) {
	system, err := renderDraftPrompt(req)
	if err != nil {
		return Draft{}, fmt.Errorf("render draft prompt: %w", err)
	}
	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:        claudeModel,
		MaxTokens:    claudeMaxTokens,
		System:       []anthropic.TextBlockParam{{Text: system}},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortLow},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Write the post.")),
		},
	}, requestTimeout(ctx))
	if err != nil {
		return Draft{}, fmt.Errorf("claude draft: %w", err)
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return Draft{}, ErrRefused
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	out := unquote(strings.TrimSpace(text.String()))
	if out == "" {
		return Draft{}, errors.New("claude draft: empty response")
	}
	return Draft{Text: out, Hook: req.Hook, FromModel: true}, nil
}

// unquote strips one pair of quotes wrapped around the whole post.
func unquote(s string) string {
	pairs := [][2]string{{`"`, `"`}, {"“", "”"}, {"'", "'"}}
	for _, p := range pairs {
		if len(s) > len(p[0])+len(p[1]) && strings.HasPrefix(s, p[0]) && strings.HasSuffix(s, p[1]) {
			inner := s[len(p[0]) : len(s)-len(p[1])]
			if !strings.Contains(inner, p[0]) {
				return strings.TrimSpace(inner)
			}
		}
	}
	return s
}
