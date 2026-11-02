package crew

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

// untouchedAfter is how long an unconnected open-link row lives.
const untouchedAfter = 24 * time.Hour

// SweepOpenedHandler handles jobs.SweepOpened: it deletes open-link rows
// still in opened with no Buffer connection 24 hours after opening, so
// the board counts people, not page views. A row left behind when a
// second device connected an account another row already had is one of
// these. GitHub rows are never swept: they carry a PR invite.
func SweepOpenedHandler(d Deps) jobs.Handler {
	return func(ctx context.Context, _ jobs.Job) error {
		n, err := d.Q.DeleteUntouchedOpened(ctx, pgtype.Timestamptz{Time: d.now().Add(-untouchedAfter), Valid: true})
		if err != nil {
			return fmt.Errorf("crew: sweep opened: %w", err)
		}
		if n > 0 {
			log.Printf("crew: swept %d untouched opened rows", n)
		}
		return nil
	}
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
