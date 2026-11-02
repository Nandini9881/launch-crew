package crew_test

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

// Twenty people connect inside one 15-minute window. Buffer allows
// 100 + 10 per user per window; the connect path must stay under it, and
// the voice query must run once per account.
func TestTwentyConnectsStayUnderBudget(t *testing.T) {
	const accounts = 20
	s := newSvc(t)
	s.channels(ch("x1", "twitter"), ch("bs1", "bluesky"))
	start := s.now
	voiced := func(memberID uuid.UUID, force bool) {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"MemberID": memberID, "Force": force})
		if err := crew.VoicedDraftsHandler(s.deps)(s.ctx, jobs.Job{Kind: jobs.VoicedDrafts, Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}

	var first uuid.UUID
	var firstConn uuid.UUID
	for i := 0; i < accounts; i++ {
		s.now = start.Add(time.Duration(i) * 40 * time.Second) // 13 minutes in all
		id, _, err := crew.ChooseRole(s.ctx, s.deps, s.launch.ID, "Speaker", "", "Talk", net.ParseIP(fmt.Sprintf("10.0.1.%d", i+1)))
		if err != nil {
			t.Fatal(err)
		}
		c := s.conn(t)
		if _, err := crew.Connect(s.ctx, s.deps, id, c.ID, uuid.Nil); err != nil {
			t.Fatal(err)
		}
		voiced(id, false)
		if i == 0 {
			first, firstConn = id, c.ID
		}
	}

	total := len(s.fake.Requests())
	limit := 100 + 10*accounts
	t.Logf("calls: total=%d (limit %d) channels=%d configuration=%d sentPosts=%d",
		total, limit, s.fake.Count("GetChannels"), s.fake.Count("GetConfiguration"), s.fake.Count("GetSentPosts"))
	if total > limit {
		t.Fatalf("%d Buffer calls in one window, budget is %d", total, limit)
	}
	// Tight bound: channels, configuration and voice once per account.
	if total > 4*accounts {
		t.Fatalf("%d Buffer calls for %d connects, want at most %d", total, accounts, 4*accounts)
	}
	for _, op := range []string{"GetChannels", "GetConfiguration"} {
		if n := s.fake.Count(op); n != accounts {
			t.Fatalf("%s called %d times for %d accounts", op, n, accounts)
		}
	}
	if n := s.fake.Count("GetSentPosts"); n != accounts {
		t.Fatalf("sentPosts queried %d times for %d accounts", n, accounts)
	}

	// Connecting the same account again, and voicing it again, stays cached.
	other, _, err := crew.ChooseRole(s.ctx, s.deps, s.launch.ID, "Attendee", "", "", net.ParseIP("10.0.2.1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := crew.Connect(s.ctx, s.deps, other, firstConn, uuid.Nil); err != nil {
		t.Fatal(err)
	}
	voiced(first, true)
	if n := s.fake.Count("GetSentPosts"); n != accounts {
		t.Fatalf("sentPosts queried %d times after reconnect, want %d", n, accounts)
	}
}
