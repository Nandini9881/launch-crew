package drafting

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shubham-tomar/launchcrew/internal/buffer"
)

func loadPosts(t *testing.T) []buffer.Post {
	t.Helper()
	raw, err := os.ReadFile("testdata/sent_posts_100.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Text   string    `json:"text"`
		SentAt time.Time `json:"sentAt"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	posts := make([]buffer.Post, len(rows))
	for i, r := range rows {
		posts[i] = buffer.Post{Text: r.Text, SentAt: r.SentAt}
	}
	return posts
}

func loadPRText(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("testdata/pr_text.json")
	if err != nil {
		t.Fatal(err)
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVoiceFromHundredPosts(t *testing.T) {
	posts := loadPosts(t)
	if len(posts) != 100 {
		t.Fatalf("fixture has %d posts", len(posts))
	}
	got, source := DeriveVoice(posts, nil)
	want := Profile{SentenceLength: "short", Case: "lowercase", Hashtags: "none", Emoji: "none"}
	if got != want {
		t.Errorf("profile = %+v, want %+v", got, want)
	}
	if source != "buffer" {
		t.Errorf("source = %q, want buffer", source)
	}
}

func TestVoiceFallsBackToPRText(t *testing.T) {
	got, source := DeriveVoice(loadPosts(t)[:7], loadPRText(t))
	if source != "github" {
		t.Errorf("source = %q, want github", source)
	}
	if got.Case != "sentence" || got.SentenceLength != "long" {
		t.Errorf("profile = %+v, want sentence case and long sentences", got)
	}
}

func TestVoiceNeutral(t *testing.T) {
	got, source := DeriveVoice(nil, nil)
	if got != Neutral() || source != "neutral" {
		t.Errorf("got %+v, %q", got, source)
	}
	got, source = DeriveVoice(loadPosts(t)[:3], []string{"", ""})
	if got != Neutral() || source != "neutral" {
		t.Errorf("thin input: got %+v, %q", got, source)
	}
	want := Profile{SentenceLength: "medium", Case: "sentence", Hashtags: "none", Emoji: "none"}
	if Neutral() != want {
		t.Errorf("Neutral() = %+v", Neutral())
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		p    Profile
		want string
	}{
		{Profile{SentenceLength: "short", Case: "lowercase", Hashtags: "none", Emoji: "none"}, "short sentences, lowercase, no hashtags"},
		{Profile{SentenceLength: "long", Case: "sentence", Hashtags: "few", Emoji: "some"}, "long sentences, sentence case, a hashtag or two, some emoji"},
		{Profile{SentenceLength: "medium", Case: "sentence", Hashtags: "none", Emoji: "none", Extra: "dry humour"}, "medium sentences, sentence case, no hashtags, dry humour"},
	}
	for _, tt := range tests {
		if got := Describe(tt.p); got != tt.want {
			t.Errorf("Describe = %q, want %q", got, tt.want)
		}
	}
}

func mkPosts(n int, text func(i int) string) []buffer.Post {
	posts := make([]buffer.Post, n)
	for i := range posts {
		posts[i] = buffer.Post{Text: text(i)}
	}
	return posts
}

func TestVoiceThresholds(t *testing.T) {
	// first k of 100 posts get the variant text, the rest the base text.
	frac := func(k int, variant, base string) []buffer.Post {
		return mkPosts(100, func(i int) string {
			if i < k {
				return variant
			}
			return base
		})
	}
	tests := []struct {
		name  string
		posts []buffer.Post
		check func(Profile) string
		want  string
	}{
		{"60% lowercase is sentence", frac(60, "hello there", "Hello there"), func(p Profile) string { return p.Case }, "sentence"},
		{"61% lowercase is lowercase", frac(61, "hello there", "Hello there"), func(p Profile) string { return p.Case }, "lowercase"},
		{"30% hashtags is none", frac(30, "hi #go", "hi"), func(p Profile) string { return p.Hashtags }, "none"},
		{"31% hashtags is few", frac(31, "hi #go", "hi"), func(p Profile) string { return p.Hashtags }, "few"},
		{"hash before digit is not a hashtag", frac(100, "issue #12", "hi"), func(p Profile) string { return p.Hashtags }, "none"},
		{"30% emoji is none", frac(30, "hi 🚀", "hi"), func(p Profile) string { return p.Emoji }, "none"},
		{"31% emoji is some", frac(31, "hi 🚀", "hi"), func(p Profile) string { return p.Emoji }, "some"},
		{"plain punctuation is not emoji", frac(100, "hi — ok…", "hi"), func(p Profile) string { return p.Emoji }, "none"},
		{"non-letter starts ignored for case", append(frac(0, "", "hello there")[:10], mkPosts(90, func(int) string { return "1. Hello" })...), func(p Profile) string { return p.Case }, "lowercase"},
		{"11 words is short", frac(100, "one two three four five six seven eight nine ten eleven.", ""), func(p Profile) string { return p.SentenceLength }, "short"},
		{"12 words is medium", frac(100, "one two three four five six seven eight nine ten eleven twelve.", ""), func(p Profile) string { return p.SentenceLength }, "medium"},
		{"22 words is medium", frac(100, strings.Repeat("w ", 21)+"w.", ""), func(p Profile) string { return p.SentenceLength }, "medium"},
		{"23 words is long", frac(100, strings.Repeat("w ", 22)+"w.", ""), func(p Profile) string { return p.SentenceLength }, "long"},
		{"urls and hashtags are not words", frac(100, "short one https://example.com/a/very/long/path #hashtag #another", ""), func(p Profile) string { return p.SentenceLength }, "short"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := DeriveVoice(tt.posts, nil)
			if v := tt.check(got); v != tt.want {
				t.Errorf("got %q, want %q", v, tt.want)
			}
		})
	}
}

func TestVoiceSourceSwitch(t *testing.T) {
	pr := []string{"Add a thing."}
	for _, tt := range []struct {
		n    int
		want string
	}{{10, "buffer"}, {9, "github"}} {
		t.Run(fmt.Sprintf("%d posts", tt.n), func(t *testing.T) {
			_, source := DeriveVoice(mkPosts(tt.n, func(int) string { return "hi there" }), pr)
			if source != tt.want {
				t.Errorf("source = %q, want %q", source, tt.want)
			}
		})
	}
}

func TestVoiceLink(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"url at end", "new release https://x.io/r", "first comment"},
		{"url inline", "see https://x.io/r for the release", "inline"},
		{"no url", "new release", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := DeriveVoice(mkPosts(10, func(int) string { return tt.text }), nil)
			if got.Link != tt.want {
				t.Errorf("Link = %q, want %q", got.Link, tt.want)
			}
		})
	}
}
