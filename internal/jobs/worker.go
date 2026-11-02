package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

const (
	jobTimeout   = 10 * time.Minute
	maxErrorLen  = 2000
	backoffBase  = 10 * time.Second
	defaultPoll  = time.Second
	defaultConcs = 2
	settleTries  = 3
	settleGap    = 100 * time.Millisecond
	staleAfter   = jobTimeout + 2*time.Minute
	maxAttempts  = 5 // matches FailJob's dead-letter threshold
)

// Worker polls the queue and dispatches jobs to Handlers.
type Worker struct {
	Pool        *pgxpool.Pool
	Handlers    map[Kind]Handler
	ID          string
	Concurrency int
	Poll        time.Duration
}

// Run processes jobs until ctx is cancelled, then waits for in-flight jobs.
func (w *Worker) Run(ctx context.Context) error {
	n := w.Concurrency
	if n <= 0 {
		n = defaultConcs
	}
	poll := w.Poll
	if poll <= 0 {
		poll = defaultPoll
	}
	q := sqlc.New(w.Pool)
	var wg sync.WaitGroup
	for i := 1; i <= n; i++ {
		lockedBy := fmt.Sprintf("%s-%d", w.ID, i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				handled, err := RunOnce(ctx, q, w.Handlers, lockedBy)
				if err != nil && ctx.Err() == nil {
					log.Printf("jobs: %s: %v", lockedBy, err)
				}
				if handled && err == nil {
					continue
				}
				if !handled && err == nil {
					if _, rerr := ReapStale(ctx, q, staleAfter); rerr != nil && ctx.Err() == nil {
						log.Printf("jobs: reap: %v", rerr)
					}
				}
				select {
				case <-ctx.Done():
				case <-time.After(poll):
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

// RunOnce claims and runs at most one job with the given handlers.
func RunOnce(ctx context.Context, q *sqlc.Queries, handlers map[Kind]Handler, lockedBy string) (bool, error) {
	j, ok, err := claim(ctx, q, lockedBy)
	if err != nil || !ok {
		return false, err
	}
	herr := dispatch(ctx, handlers, j)
	if herr != nil && ctx.Err() == nil {
		log.Printf("job failed kind=%s id=%d attempt=%d err=%v", j.Kind, j.ID, j.Attempts+1, herr)
		if j.Attempts+1 >= maxAttempts {
			log.Printf("job dead-lettered kind=%s id=%d", j.Kind, j.ID)
		}
	}
	// Settle even if ctx was cancelled mid-job, or the row stays locked.
	sctx := context.WithoutCancel(ctx)
	var serr error
	for i := 0; i < settleTries; i++ {
		if i > 0 {
			time.Sleep(settleGap)
		}
		if serr = settle(sctx, ctx, q, j, herr); serr == nil {
			break
		}
	}
	return true, serr
}

func settle(sctx, runCtx context.Context, q *sqlc.Queries, j Job, herr error) error {
	switch {
	case herr == nil:
		return q.CompleteJob(sctx, j.ID)
	case runCtx.Err() != nil:
		// Shutdown interrupted the job: give it back without burning an attempt.
		return q.ReleaseJob(sctx, j.ID)
	}
	delay := backoffBase << min(j.Attempts, 20)
	return q.FailJob(sctx, sqlc.FailJobParams{
		ID:        j.ID,
		LastError: pgtype.Text{String: truncateErr(herr.Error()), Valid: true},
		RunAt:     pgtype.Timestamptz{Time: time.Now().Add(delay), Valid: true},
	})
}

// truncateErr cuts on a rune boundary and drops bytes Postgres rejects.
func truncateErr(msg string) string {
	msg = strings.ToValidUTF8(strings.ReplaceAll(msg, "\x00", ""), "?")
	if r := []rune(msg); len(r) > maxErrorLen {
		msg = string(r[:maxErrorLen])
	}
	return msg
}

// ReapStale unlocks jobs held longer than olderThan, e.g. by a crashed worker.
func ReapStale(ctx context.Context, q *sqlc.Queries, olderThan time.Duration) (int64, error) {
	return q.ReapStaleLocks(ctx, pgtype.Interval{Microseconds: olderThan.Microseconds(), Valid: true})
}

func dispatch(ctx context.Context, handlers map[Kind]Handler, j Job) (err error) {
	h, ok := handlers[j.Kind]
	if !ok {
		return errors.New("no handler for kind")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	jctx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()
	return h(jctx, j)
}
