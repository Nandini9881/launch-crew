package buffer

import "testing"

func TestEligibilityTable(t *testing.T) {
	cfg := Config{Services: map[string]ServiceRules{"twitter": {MaxChars: 280}}}
	cases := []struct {
		name     string
		ch       Channel
		hasImage bool
		ok       bool
		reason   string
	}{
		{"youtube", Channel{Service: "youtube", Connected: true}, true, false, "video only"},
		{"tiktok", Channel{Service: "tiktok", Connected: true}, true, false, "video only"},
		{"pinterest", Channel{Service: "pinterest", Connected: true}, true, false, "needs a board"},
		{"instagram no image", Channel{Service: "instagram", Connected: true}, false, false, "needs an image"},
		{"instagram image", Channel{Service: "instagram", Connected: true}, true, true, ""},
		{"disconnected twitter", Channel{Service: "twitter"}, true, false, "disconnected"},
		{"locked linkedin", Channel{Service: "linkedin", Connected: true, Locked: true}, true, false, "locked"},
		{"paused mastodon", Channel{Service: "mastodon", Connected: true, Paused: true}, true, false, "paused"},
		{"unsupported", Channel{Service: "myspace", Connected: true}, true, false, "unsupported"},
		{"x alias", Channel{Service: "X", Connected: true}, false, true, ""},
		{"connected bluesky", Channel{Service: "bluesky", Connected: true}, false, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, reason := Eligible(c.ch, cfg, c.hasImage)
			if ok != c.ok || reason != c.reason {
				t.Fatalf("got (%v, %q), want (%v, %q)", ok, reason, c.ok, c.reason)
			}
		})
	}
}
