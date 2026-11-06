package drafting

import (
	"context"
	"strings"
	"testing"
)

func longSubject() Subject {
	return Subject{
		Project: "Launch Crew", Version: "v1.2.0", URL: "https://example.com/releases/v1.2.0",
		Headline: strings.Repeat("The new release adds a much faster sync path for large workspaces. ", 12),
		PRTitle:  strings.Repeat("Rework the queue so retries never block fresh work ", 8),
	}
}

func draft(t *testing.T, s Subject, h HookType, n Network) Draft {
	t.Helper()
	d, err := Template{}.Draft(context.Background(), DraftRequest{Subject: s, Hook: h, Network: n})
	if err != nil {
		t.Fatal(err)
	}
	return FitToRules(d, RulesFor(n, nil))
}

func TestTemplateWithinLimitsEveryNetwork(t *testing.T) {
	for _, n := range AllNetworks {
		r := RulesFor(n, nil)
		for _, h := range AllHooks {
			d := draft(t, longSubject(), h, n)
			if runeLen(d.Text) > r.MaxChars {
				t.Errorf("%s/%s: text %d > %d", n, h, runeLen(d.Text), r.MaxChars)
			}
			for _, item := range d.Thread {
				if runeLen(item) > r.MaxChars {
					t.Errorf("%s/%s: thread item %d > %d", n, h, runeLen(item), r.MaxChars)
				}
			}
			if r.Threads && (len(d.Thread) == 0 || d.Thread[0] != d.Text) {
				t.Errorf("%s/%s: Thread[0] != Text", n, h)
			}
			if again := FitToRules(d, r); again.Text != d.Text || len(again.Thread) != len(d.Thread) || again.FirstComment != d.FirstComment {
				t.Errorf("%s/%s: FitToRules not idempotent", n, h)
			}
		}
	}
}

func TestLinkedInLinkInFirstComment(t *testing.T) {
	s := longSubject()
	d := draft(t, s, HookDetail, LinkedIn)
	if strings.Contains(d.Text, s.URL) || d.FirstComment != s.URL {
		t.Errorf("text=%q firstComment=%q", d.Text, d.FirstComment)
	}
}

func TestXThreadFirstItemEqualsText(t *testing.T) {
	d := draft(t, longSubject(), HookReaction, Twitter)
	if len(d.Thread) < 2 || d.Thread[0] != d.Text {
		t.Errorf("thread=%d first==text:%v", len(d.Thread), len(d.Thread) > 0 && d.Thread[0] == d.Text)
	}
}

func TestTemplateDiffersByHook(t *testing.T) {
	s := Subject{Project: "Launch Crew", Version: "v1.2.0", URL: "https://example.com/r", Headline: "Posts now keep each person's voice.", PRTitle: "Add voice profiles"}
	seen := map[string]HookType{}
	for _, h := range AllHooks {
		d, _ := Template{}.Draft(context.Background(), DraftRequest{Subject: s, Hook: h})
		if prev, dup := seen[d.Text]; dup {
			t.Errorf("%s and %s give the same text", prev, h)
		}
		seen[d.Text] = h
		if tells := Lint(d.Text); tells != nil {
			t.Errorf("%s: lint tells %v", h, tells)
		}
	}
	manual := Subject{Project: "Launch Crew", Version: "v1.2.0", Role: "a guest", RoleHint: "Talked to people at the booth"}
	d, _ := Template{}.Draft(context.Background(), DraftRequest{Subject: manual, Hook: HookPersonal})
	if !strings.Contains(d.Text, "came along") {
		t.Errorf("manual personal hook: %q", d.Text)
	}
}

func TestTemplateCreditsEveryPR(t *testing.T) {
	s := Subject{Project: "Launch Crew", Version: "v1.2.0", Headline: "Posts keep voice.", PRTitle: "C", PRTitles: []string{"C", "B", "A"}}
	if got := templateText(s, HookDetail); !strings.Contains(got, "A few changes in it: C, B and A.") {
		t.Errorf("detail: %s", got)
	}
	if got := templateText(s, HookPersonal); !strings.Contains(got, "I had a hand in Launch Crew v1.2.0: C, B and A.") {
		t.Errorf("personal: %s", got)
	}
	s.PRTitles = []string{"1", "2", "3", "4", "5"}
	if got := titleList(s.PRTitles); got != "1, 2, 3 and 2 more" {
		t.Errorf("titleList = %q", got)
	}
}
