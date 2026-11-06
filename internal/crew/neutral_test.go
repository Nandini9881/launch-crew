package crew_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/testfakes/anthropicfake"
)

// Two Share taps at once both ask the model, but only the first answer is
// kept; the other tap returns the stored text instead of overwriting it.
func TestNeutralDraftFirstWriteWins(t *testing.T) {
	s := newSvc(t)
	id, _ := s.member(t)
	s.ai.Script(
		anthropicfake.Response{Text: "Slow answer about profiling in prod at the meetup.", Delay: 300 * time.Millisecond},
		anthropicfake.Response{Text: "Fast answer about profiling in prod at the meetup."},
	)
	got, errs := make([]string, 2), make([]error, 2)
	var wg sync.WaitGroup
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = crew.NeutralDraft(s.ctx, s.deps, id)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("tap %d: %v", i, err)
		}
	}
	if got[0] != got[1] {
		t.Fatalf("taps disagree: %q vs %q", got[0], got[1])
	}
	if !strings.HasPrefix(got[0], "Fast answer") {
		t.Fatalf("text = %q, want the first write (the fast answer) to win", got[0])
	}
	if stored := s.row(t, id).NeutralDraft.String; stored != got[0] {
		t.Fatalf("stored %q, returned %q", stored, got[0])
	}
	// A later tap reads the stored draft and spends nothing.
	if text, err := crew.NeutralDraft(s.ctx, s.deps, id); err != nil || text != got[0] {
		t.Fatalf("third tap = %q, %v", text, err)
	}
	if n := len(s.ai.Requests()); n != 2 {
		t.Fatalf("model calls = %d, want 2", n)
	}
}
