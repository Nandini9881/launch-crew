package crew_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

// The hourly sweep also prunes the audit tables that otherwise grow for
// ever: webhook payloads and Buffer calls after 30 days, open-rate buckets
// after a day. Younger rows stay.
func TestSweepPrunesOldAuditRows(t *testing.T) {
	s := newSvc(t)
	ctx := s.ctx
	conn := s.conn(t)
	oldDelivery, newDelivery := "old-"+uuid.NewString(), "new-"+uuid.NewString()
	for _, id := range []string{oldDelivery, newDelivery} {
		if _, err := s.q.InsertWebhookDelivery(ctx, sqlc.InsertWebhookDeliveryParams{DeliveryID: id, Event: "release", Payload: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `UPDATE webhook_deliveries SET received_at = now() - interval '31 days' WHERE delivery_id = $1`, oldDelivery); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.q.InsertBufferCall(ctx, sqlc.InsertBufferCallParams{ConnectionID: conn.ID, Op: "Ping", Status: 200}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `UPDATE buffer_calls SET at = now() - interval '31 days' WHERE id = (SELECT min(id) FROM buffer_calls WHERE connection_id = $1)`, conn.ID); err != nil {
		t.Fatal(err)
	}
	ip := netip.MustParseAddr("10.9.9.9")
	for _, minute := range []time.Time{s.now.Add(-2 * 24 * time.Hour), s.now} {
		if _, err := s.q.UpsertOpenRate(ctx, sqlc.UpsertOpenRateParams{LaunchID: s.launch.ID, Ip: ip, Minute: pgtype.Timestamptz{Time: minute, Valid: true}}); err != nil {
			t.Fatal(err)
		}
	}

	if err := crew.SweepOpenedHandler(s.deps)(ctx, jobs.Job{Kind: jobs.SweepOpened, Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.q.GetWebhookDelivery(ctx, oldDelivery); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("31-day-old delivery: err = %v, want gone", err)
	}
	if _, err := s.q.GetWebhookDelivery(ctx, newDelivery); err != nil {
		t.Fatalf("fresh delivery swept: %v", err)
	}
	count := func(query string, args ...any) int {
		var n int
		if err := s.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM buffer_calls WHERE connection_id = $1`, conn.ID); n != 1 {
		t.Fatalf("buffer_calls = %d, want the fresh one only", n)
	}
	if n := count(`SELECT count(*) FROM open_rate WHERE launch_id = $1 AND ip = $2`, s.launch.ID, ip); n != 1 {
		t.Fatalf("open_rate = %d, want the fresh bucket only", n)
	}
}
