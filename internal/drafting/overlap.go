package drafting

import (
	"regexp"
	"strings"
)

var wsRunRe = regexp.MustCompile(`\s+`)

func trigrams(s string) map[string]struct{} {
	s = urlRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(wsRunRe.ReplaceAllString(strings.ToLower(s), " "))
	r := []rune(s)
	set := map[string]struct{}{}
	for i := 0; i+3 <= len(r); i++ {
		set[string(r[i:i+3])] = struct{}{}
	}
	return set
}

// TrigramOverlap is the Jaccard similarity of the character trigrams of a and b.
func TrigramOverlap(a, b string) float64 {
	ta, tb := trigrams(a), trigrams(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for g := range ta {
		if _, ok := tb[g]; ok {
			inter++
		}
	}
	return float64(inter) / float64(len(ta)+len(tb)-inter)
}

// AssignHook returns the first hook not in used. When all are used it
// returns the least used one, ties broken by AllHooks order.
func AssignHook(used []HookType) HookType {
	return AssignHookFrom(AllHooks, used)
}

// AnnouncementHooks are the hooks for a project's own posts: every hook but
// the personal one, which speaks as a contributor ("I came along for...").
var AnnouncementHooks = []HookType{HookQuestion, HookDetail, HookReaction, HookLinkFirst}

// AssignHookFrom is AssignHook over the given hook set.
func AssignHookFrom(set, used []HookType) HookType {
	counts := map[HookType]int{}
	for _, h := range used {
		counts[h]++
	}
	best := set[0]
	for _, h := range set {
		if counts[h] == 0 {
			return h
		}
		if counts[h] < counts[best] {
			best = h
		}
	}
	return best
}
