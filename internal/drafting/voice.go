package drafting

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/shubham-tomar/launchcrew/internal/buffer"
)

// minPostsForBuffer is how many sent posts are needed to trust them as a voice.
const minPostsForBuffer = 10

var (
	voiceURLRe        = regexp.MustCompile(`https?://\S+`)
	voiceURLEndRe     = regexp.MustCompile(`https?://\S+$`)
	voiceHashtagRe    = regexp.MustCompile(`#\p{L}[\p{L}\p{N}_]*`)
	voiceHashLetterRe = regexp.MustCompile(`#\p{L}`)
	voiceSentenceRe   = regexp.MustCompile(`[.!?]+(?:\s+|$)`)
)

// Neutral is the profile used when there is nothing to learn a voice from.
func Neutral() Profile {
	return Profile{SentenceLength: "medium", Case: "sentence", Hashtags: "none", Emoji: "none"}
}

// DeriveVoice builds a style profile from sent Buffer posts, falling back to
// PR text when there are fewer than 10 posts. source is buffer, github or neutral.
func DeriveVoice(posts []buffer.Post, prText []string) (Profile, string) {
	if len(posts) >= minPostsForBuffer {
		texts := make([]string, len(posts))
		for i, p := range posts {
			texts[i] = p.Text
		}
		return profileOf(texts), "buffer"
	}
	var texts []string
	for _, s := range prText {
		if strings.TrimSpace(s) != "" {
			texts = append(texts, s)
		}
	}
	if len(texts) == 0 {
		return Neutral(), "neutral"
	}
	return profileOf(texts), "github"
}

func profileOf(texts []string) Profile {
	p := Profile{SentenceLength: sentenceLength(texts), Case: "sentence", Hashtags: "none", Emoji: "none"}

	var letterStarts, lower, hashtags, emoji, urls, urlsAtEnd int
	for _, t := range texts {
		t = strings.TrimSpace(t)
		if first := []rune(t); len(first) > 0 && unicode.IsLetter(first[0]) {
			letterStarts++
			if unicode.IsLower(first[0]) {
				lower++
			}
		}
		if voiceHashLetterRe.MatchString(t) {
			hashtags++
		}
		if hasEmoji(t) {
			emoji++
		}
		if voiceURLRe.MatchString(t) {
			urls++
		}
		if voiceURLEndRe.MatchString(t) {
			urlsAtEnd++
		}
	}
	n := len(texts)
	if letterStarts > 0 && lower*100 > letterStarts*60 {
		p.Case = "lowercase"
	}
	if hashtags*100 > n*30 {
		p.Hashtags = "few"
	}
	if emoji*100 > n*30 {
		p.Emoji = "some"
	}
	// Link is the one inference beyond the length/case/hashtag/emoji rules:
	// a URL at the very end of more than half the posts means the link sits
	// outside the copy ("first comment"); any other URL use is "inline".
	switch {
	case urlsAtEnd*2 > n:
		p.Link = "first comment"
	case urls > 0:
		p.Link = "inline"
	}
	return p
}

// sentenceLength classifies the median sentence length in words, ignoring
// URLs and hashtags.
func sentenceLength(texts []string) string {
	var counts []int
	for _, t := range texts {
		t = voiceHashtagRe.ReplaceAllString(voiceURLRe.ReplaceAllString(t, " "), " ")
		for _, s := range voiceSentenceRe.Split(t, -1) {
			if w := len(strings.Fields(s)); w > 0 {
				counts = append(counts, w)
			}
		}
	}
	if len(counts) == 0 {
		return "medium"
	}
	sort.Ints(counts)
	mid := len(counts) / 2
	median := float64(counts[mid])
	if len(counts)%2 == 0 {
		median = float64(counts[mid-1]+counts[mid]) / 2
	}
	switch {
	case median < 12:
		return "short"
	case median > 22:
		return "long"
	}
	return "medium"
}

// hasEmoji reports whether s has a rune in a common emoji block.
func hasEmoji(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x1F300 && r <= 0x1FAFF, // pictographs, emoticons, transport, supplemental
			r >= 0x1F1E6 && r <= 0x1F1FF, // regional indicators (flags)
			r >= 0x2600 && r <= 0x27BF:   // misc symbols, dingbats
			return true
		}
	}
	return false
}

// Describe renders a profile as the voice line given to the drafter.
func Describe(p Profile) string {
	parts := []string{p.SentenceLength + " sentences"}
	if p.Case == "lowercase" {
		parts = append(parts, "lowercase")
	} else {
		parts = append(parts, "sentence case")
	}
	if p.Hashtags == "few" {
		parts = append(parts, "a hashtag or two")
	} else {
		parts = append(parts, "no hashtags")
	}
	if p.Emoji == "some" {
		parts = append(parts, "some emoji")
	}
	if p.Extra != "" {
		parts = append(parts, p.Extra)
	}
	return strings.Join(parts, ", ")
}
