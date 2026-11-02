package drafting

import "testing"

func TestTrigramIdenticalIsOne(t *testing.T) {
	if got := TrigramOverlap("Shipped v2 today", "Shipped v2 today"); got != 1 {
		t.Fatalf("got %v", got)
	}
	if got := TrigramOverlap("", "anything"); got != 0 {
		t.Fatalf("empty: got %v", got)
	}
}

func TestTrigramUnrelatedBelowPointOne(t *testing.T) {
	got := TrigramOverlap("Caching layer rewrite landed, p99 dropped a lot", "Our garden tomatoes finally ripened this week")
	if got >= 0.1 {
		t.Fatalf("got %v", got)
	}
}

func TestTrigramNearDuplicateAbove035(t *testing.T) {
	a := "Tuios 1.4 is out. Window tiling got a rewrite. https://x.dev/a"
	b := "tuios 1.4 is out.  Window tiling got a rewrite! https://y.dev/b"
	if got := TrigramOverlap(a, b); got <= 0.35 {
		t.Fatalf("got %v", got)
	}
}

func TestAssignHookNeverRepeatsWithinSetSize(t *testing.T) {
	var used []HookType
	seen := map[HookType]bool{}
	for range AllHooks {
		h := AssignHook(used)
		if seen[h] {
			t.Fatalf("repeat %s", h)
		}
		seen[h] = true
		used = append(used, h)
	}
}

func TestAssignHookWrapsToLeastUsed(t *testing.T) {
	used := append(append([]HookType{}, AllHooks...), HookPersonal, HookQuestion)
	if got := AssignHook(used); got != HookDetail {
		t.Fatalf("got %s", got)
	}
}
