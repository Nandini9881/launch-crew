package buffer

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

// Buffer sends one RateLimit and one RateLimit-Policy entry per window
// (15 min, 1 day, 30 days), e.g.
//
//	RateLimit:        "150-in-15min";r=98;t=897
//	RateLimit-Policy: "150-in-15min";q=150;w=900;pk=:ZjJj:
//
// The budget tracks the 15-minute window, matched by w=900.
const (
	budgetWindowSeconds = 900
	defaultWindow       = 100 // app client base quota per 15 min
)

type rate struct {
	remaining int
	quota     int // 0 when no policy header matched
	reset     time.Time
}

// parseRate reads the 15-minute policy from the response headers.
func parseRate(h http.Header, now time.Time) (rate, bool) {
	policies := map[string]map[string]string{}
	for _, e := range headerEntries(h, "RateLimit-Policy") {
		policies[e.name] = e.params
	}
	var pick *entry
	entries := headerEntries(h, "RateLimit")
	for i, e := range entries {
		if p, ok := policies[e.name]; ok && p["w"] == strconv.Itoa(budgetWindowSeconds) {
			pick = &entries[i]
			break
		}
	}
	if pick == nil && len(entries) > 0 {
		pick = &entries[0]
	}
	if pick == nil {
		return rate{}, false
	}
	r, err := strconv.Atoi(pick.params["r"])
	if err != nil {
		return rate{}, false
	}
	t, _ := strconv.Atoi(pick.params["t"])
	q, _ := strconv.Atoi(policies[pick.name]["q"])
	return rate{remaining: r, quota: q, reset: now.Add(time.Duration(t) * time.Second)}, true
}

type entry struct {
	name   string
	params map[string]string
}

// headerEntries splits repeated or comma-joined structured header values
// into `"name";k=v;k=v` entries.
func headerEntries(h http.Header, key string) []entry {
	var out []entry
	for _, v := range h.Values(key) {
		for _, item := range strings.Split(v, ",") {
			parts := strings.Split(item, ";")
			e := entry{name: strings.Trim(strings.TrimSpace(parts[0]), `"`), params: map[string]string{}}
			for _, p := range parts[1:] {
				k, val, _ := strings.Cut(strings.TrimSpace(p), "=")
				e.params[k] = val
			}
			out = append(out, e)
		}
	}
	return out
}

// noteWindow remembers the largest quota seen for a connection.
func (c *Client) noteWindow(connID uuid.UUID, r rate) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n := max(r.quota, r.remaining); n > c.windows[connID] {
		c.windows[connID] = n
	}
}

func (c *Client) window(connID uuid.UUID) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if w := c.windows[connID]; w > 0 {
		return w
	}
	return defaultWindow
}

// waitBudget sleeps until the window resets when less than 20% of it is left.
func (c *Client) waitBudget(ctx context.Context, conn sqlc.BufferConnection) error {
	if !conn.RateRemaining.Valid || !conn.RateResetAt.Valid {
		return nil
	}
	if int(conn.RateRemaining.Int32)*5 >= c.window(conn.ID) {
		return nil
	}
	d := conn.RateResetAt.Time.Sub(c.now())
	if d <= 0 {
		return nil
	}
	if d > maxWait {
		return fmt.Errorf("buffer: rate budget low; window resets in %s, longer than the %s cap", d, maxWait)
	}
	return c.wait(ctx, d)
}
