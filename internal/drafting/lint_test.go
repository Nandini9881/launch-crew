package drafting

import (
	"reflect"
	"testing"
)

func TestLintFlagsTells(t *testing.T) {
	cases := map[string]string{
		"em dash":          "It ships today — finally.",
		"not X but Y":      "This is not a fix but a rewrite.",
		"triad":            "It is fast, simple, and free.",
		"stock emoji":      "Out now \U0001F680",
		"excited to share": "Excited to share the new release.",
	}
	for want, text := range cases {
		if got := Lint(text); !reflect.DeepEqual(got, []string{want}) {
			t.Errorf("Lint(%q) = %v, want [%s]", text, got, want)
		}
	}
	if got := Lint("I fixed the retry bug in the sync worker. It was a small change."); got != nil {
		t.Errorf("clean text flagged: %v", got)
	}
}
