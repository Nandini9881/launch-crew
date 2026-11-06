package crew

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

const (
	// untouchedAfter is how long an unconnected open-link row lives.
	untouchedAfter = 24 * time.Hour
	// auditRetention is how long webhook payloads and Buffer call rows are
	// kept; both only serve dedup and the 15-minute rate headroom.
	auditRetention = 30 * 24 * time.Hour
	// openRateRetention covers the per-minute open-link buckets.
	openRateRetention = 24 * time.Hour
)

// SweepOpenedHandler handles jobs.SweepOpened: it deletes open-link rows
// still in opened with no Buffer connection 24 hours after opening, so
// the board counts people, not page views. A row left behind when a
// second device connected an account another row already had is one of
// these. GitHub rows are never swept: they carry a PR invite.
//
// The same hourly job prunes the audit tables, which nothing else
// trims: webhook_deliveries and buffer_calls past auditRetention and
// open_rate past openRateRetention.
func SweepOpenedHandler(d Deps) jobs.Handler {
	return func(ctx context.Context, _ jobs.Job) error {
		var errs []error
		n, err := d.Q.DeleteUntouchedOpened(ctx, pgtype.Timestamptz{Time: d.now().Add(-untouchedAfter), Valid: true})
		if err != nil {
			errs = append(errs, fmt.Errorf("crew: sweep opened: %w", err))
		} else if n > 0 {
			log.Printf("crew: swept %d untouched opened rows", n)
		}
		errs = append(errs, prune(ctx, d)...)
		return errors.Join(errs...)
	}
}

// prune deletes audit rows past their retention; every table is tried.
func prune(ctx context.Context, d Deps) []error {
	cutoff := func(keep time.Duration) pgtype.Timestamptz {
		return pgtype.Timestamptz{Time: d.now().Add(-keep), Valid: true}
	}
	var errs []error
	for _, p := range []struct {
		table string
		del   func() (int64, error)
	}{
		{"webhook_deliveries", func() (int64, error) { return d.Q.DeleteWebhookDeliveriesBefore(ctx, cutoff(auditRetention)) }},
		{"buffer_calls", func() (int64, error) { return d.Q.DeleteBufferCallsBefore(ctx, cutoff(auditRetention)) }},
		{"open_rate", func() (int64, error) { return d.Q.DeleteOpenRateBefore(ctx, cutoff(openRateRetention)) }},
	} {
		n, err := p.del()
		if err != nil {
			errs = append(errs, fmt.Errorf("crew: prune %s: %w", p.table, err))
			continue
		}
		if n > 0 {
			log.Printf("crew: pruned %d %s rows", n, p.table)
		}
	}
	return errs
}

// EnsureSweep enqueues a SweepOpened job unless one is already waiting.
func EnsureSweep(ctx context.Context, d Deps) error {
	n, err := d.Q.CountLiveJobsOfKind(ctx, string(jobs.SweepOpened))
	if err != nil {
		return fmt.Errorf("crew: count sweep jobs: %w", err)
	}
	if n > 0 || d.Enqueue == nil {
		return nil
	}
	_, err = d.Enqueue(ctx, jobs.SweepOpened, struct{}{}, time.Time{}, nil)
	return err
}
