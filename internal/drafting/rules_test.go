package drafting

import (
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/buffer"
)

func TestRulesForMaxImages(t *testing.T) {
	for n, want := range map[Network]int{Twitter: 4, Bluesky: 4, Mastodon: 4, Threads: 20, LinkedIn: 20, Facebook: 10, Instagram: 20} {
		if got := RulesFor(n, nil).MaxImages; got != want {
			t.Errorf("%s: MaxImages = %d, want %d", n, got, want)
		}
	}
	// Buffer's configuration knows nothing of image counts; the built-in
	// number fills in, and a network without images gets 0.
	cfg := &buffer.Config{Services: map[string]buffer.ServiceRules{
		"twitter": {MaxChars: 280, ImageOK: true},
		"youtube": {MaxChars: 5000},
	}}
	if got := RulesFor(Twitter, cfg).MaxImages; got != 4 {
		t.Errorf("twitter from config: MaxImages = %d, want 4", got)
	}
	if got := RulesFor("youtube", cfg).MaxImages; got != 0 {
		t.Errorf("youtube: MaxImages = %d, want 0", got)
	}
}
