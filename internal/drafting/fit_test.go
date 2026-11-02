package drafting

import (
	"strings"
	"testing"
)

func TestFitTruncatesNonThreadedAtSentence(t *testing.T) {
	url := "https://example.com/r"
	text := "First sentence is here. Second sentence follows it. Third sentence runs on and on. " + url
	d := FitToRules(Draft{Text: text}, Rules{MaxChars: 60})
	if d.Text != "First sentence is here. Second sentence follows it." {
		t.Errorf("text=%q", d.Text)
	}
	if d.FirstComment != url || len(d.Thread) != 0 {
		t.Errorf("firstComment=%q thread=%v", d.FirstComment, d.Thread)
	}
}

func TestFitCountsRunes(t *testing.T) {
	cjk := strings.Repeat("日本語", 30) + "。" // 91 runes, 274 bytes
	if d := FitToRules(Draft{Text: cjk}, Rules{MaxChars: 100, Threads: true}); d.Text != cjk || len(d.Thread) != 0 {
		t.Errorf("91 runes under 100 should be untouched: %d thread", len(d.Thread))
	}
	emoji := strings.Repeat("👍", 10) + " ok. " + strings.Repeat("🙂", 10) + " fine."
	d := FitToRules(Draft{Text: emoji}, Rules{MaxChars: 20, Threads: true})
	if len(d.Thread) != 2 || d.Thread[0] != strings.Repeat("👍", 10)+" ok." {
		t.Errorf("thread=%q", d.Thread)
	}
	for _, it := range d.Thread {
		if runeLen(it) > 20 {
			t.Errorf("item too long: %d", runeLen(it))
		}
	}
}

func TestFitHardWrapsLongSentenceKeepingWords(t *testing.T) {
	var words []string
	for i := 0; runeLen(strings.Join(words, " ")) < 300; i++ {
		words = append(words, "word"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	text := strings.Join(words, " ") + "."
	d := FitToRules(Draft{Text: text}, Rules{MaxChars: 280, Threads: true})
	if len(d.Thread) < 2 || d.Thread[0] != d.Text {
		t.Fatalf("thread=%d", len(d.Thread))
	}
	for _, it := range d.Thread {
		if runeLen(it) > 280 {
			t.Errorf("item %d runes", runeLen(it))
		}
	}
	if got := strings.Join(d.Thread, " "); got != text {
		t.Errorf("content lost or altered:\n%q\n%q", got, text)
	}
}

func TestFitURLTrailingPunctuation(t *testing.T) {
	d := FitToRules(Draft{Text: "Notes are up, see https://x.com/r."}, Rules{MaxChars: 100, LinkInFirstComment: true})
	if d.FirstComment != "https://x.com/r" || d.Text != "Notes are up, see." {
		t.Errorf("text=%q first=%q", d.Text, d.FirstComment)
	}
}

func TestFitLinkedInKeepsParagraphs(t *testing.T) {
	text := "First paragraph here.\n\nRead more at https://x.com/r today.\n\nLast line."
	d := FitToRules(Draft{Text: text}, Rules{MaxChars: 3000, LinkInFirstComment: true})
	want := "First paragraph here.\n\nRead more at today.\n\nLast line."
	if d.Text != want || d.FirstComment != "https://x.com/r" {
		t.Errorf("text=%q first=%q", d.Text, d.FirstComment)
	}
}

func TestFitEmptyOverLimitDoesNotPanic(t *testing.T) {
	for _, th := range []bool{true, false} {
		d := FitToRules(Draft{Text: strings.Repeat(" ", 50)}, Rules{MaxChars: 10, Threads: th})
		if d.Text != "" {
			t.Errorf("text=%q", d.Text)
		}
	}
}

func TestLintTriadIsWordLevelAndCaseInsensitive(t *testing.T) {
	if got := Lint("Thanks to Ann, Bob and Cy."); len(got) != 1 || got[0] != "triad" {
		t.Errorf("name list is flagged on purpose: %v", got)
	}
	if got := Lint("THRILLED TO ANNOUNCE it"); len(got) != 1 || got[0] != "excited to share" {
		t.Errorf("got %v", got)
	}
}

func TestTemplateExactStrings(t *testing.T) {
	s := Subject{Project: "Launch Crew", Version: "v1.2.0", URL: "https://example.com/r", Headline: "Posts now keep each person's voice.", PRTitle: "Add voice profiles."}
	tpl := func(s Subject, h HookType) string { return templateText(s, h) }
	if got, want := tpl(s, HookQuestion), "What's new in Launch Crew v1.2.0? Posts now keep each person's voice. https://example.com/r"; got != want {
		t.Errorf("question: %q", got)
	}
	if got, want := tpl(s, HookPersonal), "I had a hand in Launch Crew v1.2.0: Add voice profiles. Posts now keep each person's voice. https://example.com/r"; got != want {
		t.Errorf("personal: %q", got)
	}
	own := Subject{Project: "Launch Crew", Version: "v1.2.0", URL: "https://example.com/r", Headline: "Add voice profiles", PRTitle: "Add voice profiles."}
	if got, want := tpl(own, HookPersonal), "I had a hand in Launch Crew v1.2.0: Add voice profiles. https://example.com/r"; got != want {
		t.Errorf("personal, own PR is the headline: %q", got)
	}
	if got, want := tpl(own, HookDetail), "Launch Crew v1.2.0 is out. Add voice profiles. https://example.com/r"; got != want {
		t.Errorf("detail, own PR is the headline: %q", got)
	}
	q := Subject{Project: "Launch Crew", Headline: "Can posts keep a voice?"}
	if got, want := tpl(q, HookQuestion), "What's new at Launch Crew? Can posts keep a voice."; got != want {
		t.Errorf("question punctuation: %q", got)
	}
	m := Subject{Project: "Launch Crew", Version: "v1.2.0", TalkTitle: "Shipping small."}
	if got, want := tpl(m, HookPersonal), "I spoke about Shipping small around Launch Crew v1.2.0."; got != want {
		t.Errorf("talk: %q", got)
	}
}
