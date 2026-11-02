package drafting

import (
	"regexp"
	"strings"
)

var (
	notXButYRe = regexp.MustCompile(`(?i)\bnot (?:an? |just |only )?[^.,;!?]{1,40}?,? but\b|\b(?:isn['’]t|is not|aren['’]t|are not|wasn['’]t) [^.,;!?]{1,40}, (?:it['’]s|it is|they['’]re|it was)\b`)
	triadRe    = regexp.MustCompile(`(?i)\b[\w-]+, [\w-]+,? (?:and|or) [\w-]+`)
	hookPhrase = regexp.MustCompile(`(?i)\b(?:excited|thrilled|proud|pleased|happy) to (?:share|announce)\b`)
)

var stockEmoji = []string{"🚀", "✨", "🎉", "💡", "🔥", "🙌", "✅"}

// Lint returns the writing tells found in text, in a fixed order.
func Lint(text string) []string {
	var tells []string
	if strings.ContainsAny(text, "—–") {
		tells = append(tells, "em dash")
	}
	if notXButYRe.MatchString(text) {
		tells = append(tells, "not X but Y")
	}
	if triadRe.MatchString(text) {
		tells = append(tells, "triad")
	}
	for _, e := range stockEmoji {
		if strings.Contains(text, e) {
			tells = append(tells, "stock emoji")
			break
		}
	}
	if hookPhrase.MatchString(text) {
		tells = append(tells, "excited to share")
	}
	return tells
}
