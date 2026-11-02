package crew

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

// The approve page reads thread[0], so a draft without a thread sends [].
func TestDraftViewThreadNeverNull(t *testing.T) {
	b, err := json.Marshal(draftViews([]sqlc.CrewDraft{{Network: "twitter", Text: "hi"}}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"thread":[]`) {
		t.Fatalf("got %s", b)
	}
}
