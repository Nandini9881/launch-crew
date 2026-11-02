package drafting

import (
	"context"
	"strings"
)

// Template drafts deterministically from fixed sentence patterns.
type Template struct{}

// Draft builds the post text for req.Hook from req.Subject.
func (Template) Draft(_ context.Context, req DraftRequest) (Draft, error) {
	hook := req.Hook
	if hook == "" {
		hook = HookPersonal
	}
	return Draft{Text: templateText(req.Subject, hook), Hook: hook}, nil
}

func release(s Subject) string {
	return strings.TrimSpace(s.Project + " " + s.Version)
}

func templateText(s Subject, h HookType) string {
	rel := release(s)
	var parts []string
	switch h {
	case HookQuestion:
		q := "What's new in " + rel + "?"
		if s.Version == "" {
			q = "What's new at " + s.Project + "?"
		}
		parts = []string{q}
		if hl := sentence(s.Headline); hl != "" {
			parts = append(parts, hl)
		}
	case HookDetail:
		parts = []string{rel + " is out.", orElse(sentence(s.Headline), "Release notes are linked below.")}
		if s.PRTitle != "" && !sameLine(s.PRTitle, s.Headline) {
			parts = append(parts, "One change in it: "+stripEnd(s.PRTitle)+".")
		}
	case HookReaction:
		parts = []string{"Just shipped " + rel + ".", orElse(sentence(s.Headline), "It has been a while in the making.")}
	case HookLinkFirst:
		parts = []string{rel + " is out."}
		if hl := sentence(s.Headline); hl != "" {
			parts = append(parts, hl)
		}
		if s.URL != "" {
			parts = append([]string{s.URL}, parts...)
		}
		return strings.Join(parts, " ")
	default:
		parts = []string{personal(s, rel)}
		// The headline is skipped when it is this person's own PR title.
		if hl := sentence(s.Headline); hl != "" && !sameLine(s.Headline, s.PRTitle) {
			parts = append(parts, hl)
		}
	}
	if s.URL != "" {
		parts = append(parts, s.URL)
	}
	return strings.Join(parts, " ")
}

func personal(s Subject, rel string) string {
	switch {
	case s.PRTitle != "":
		return "I had a hand in " + rel + ": " + stripEnd(s.PRTitle) + "."
	case s.TalkTitle != "":
		return "I spoke about " + stripEnd(s.TalkTitle) + " around " + rel + "."
	case s.Role != "" && s.RoleHint != "":
		return "I came along for " + rel + " as " + s.Role + ". " + stripEnd(s.RoleHint) + "."
	case s.Role != "":
		return "I came along for " + rel + " as " + s.Role + "."
	default:
		return "I came along for " + rel + "."
	}
}

func orElse(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if len(r) > 1 && r[1] >= 'A' && r[1] <= 'Z' {
		return s
	}
	r[0] = []rune(strings.ToLower(string(r[0])))[0]
	return string(r)
}

// sameLine reports whether two fields read as the same sentence.
func sameLine(a, b string) bool {
	return strings.EqualFold(stripEnd(a), stripEnd(b))
}

// stripEnd drops trailing sentence punctuation so a field can be framed.
func stripEnd(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), ".!?,;: ")
}

// sentence returns s with one terminal period, or "" when s is empty.
func sentence(s string) string {
	if s = stripEnd(s); s == "" {
		return ""
	}
	return s + "."
}
