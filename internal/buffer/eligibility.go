package buffer

import "strings"

var knownServices = map[string]bool{
	"twitter": true, "bluesky": true, "threads": true, "mastodon": true,
	"linkedin": true, "facebook": true, "instagram": true,
	"youtube": true, "tiktok": true, "pinterest": true,
}

// Eligible says whether a channel can take a post, with a short reason when not.
func Eligible(ch Channel, cfg Config, hasImage bool) (bool, string) {
	switch {
	case !ch.Connected:
		return false, "disconnected"
	case ch.Locked:
		return false, "locked"
	case ch.Paused:
		return false, "paused"
	}
	svc := strings.ToLower(ch.Service)
	if svc == "x" {
		svc = "twitter"
	}
	switch svc {
	case "youtube", "tiktok":
		return false, "video only"
	case "pinterest":
		return false, "needs a board"
	case "instagram":
		if !hasImage {
			return false, "needs an image"
		}
		return true, ""
	}
	if _, ok := cfg.Services[svc]; !ok && !knownServices[svc] {
		return false, "unsupported"
	}
	return true, ""
}
