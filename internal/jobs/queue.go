package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

// Job mirrors a row of the jobs table.
type Job struct {
	ID        int64
	Kind      Kind
	Payload   json.RawMessage
	RunAt     time.Time
	Attempts  int
	LaunchID  *uuid.UUID
	LastError string
	Dead      bool
}

// Handler runs one job; a returned error schedules a retry.
type Handler func(ctx context.Context, j Job) error

func fromRow(r sqlc.Job) Job {
	j := Job{
		ID:        r.ID,
		Kind:      Kind(r.Kind),
		Payload:   json.RawMessage(r.Payload),
		RunAt:     r.RunAt.Time,
		Attempts:  int(r.Attempts),
		LastError: r.LastError.String,
		Dead:      r.Dead,
	}
	if r.LaunchID.Valid {
		id := r.LaunchID.UUID
		j.LaunchID = &id
	}
	return j
}

// Enqueue adds a job; a zero runAt means now.
func Enqueue(ctx context.Context, q *sqlc.Queries, kind Kind, payload any, runAt time.Time, launchID *uuid.UUID) (int64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("encode payload: %w", err)
	}
	p := sqlc.EnqueueJobParams{Kind: string(kind), Payload: body}
	if !runAt.IsZero() {
		p.RunAt = pgtype.Timestamptz{Time: runAt, Valid: true}
	}
	if launchID != nil {
		p.LaunchID = uuid.NullUUID{UUID: *launchID, Valid: true}
	}
	row, err := q.EnqueueJob(ctx, p)
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}

// DeadLetters lists dead jobs, optionally for one launch.
func DeadLetters(ctx context.Context, q *sqlc.Queries, launchID *uuid.UUID) ([]Job, error) {
	var arg uuid.NullUUID
	if launchID != nil {
		arg = uuid.NullUUID{UUID: *launchID, Valid: true}
	}
	rows, err := q.ListDeadLetters(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]Job, len(rows))
	for i, r := range rows {
		out[i] = fromRow(r)
	}
	return out, nil
}

// Retry revives a dead job with a fresh attempt count.
func Retry(ctx context.Context, q *sqlc.Queries, id int64) error {
	return q.RetryJob(ctx, id)
}

// claim takes the next runnable job, or reports false when there is none.
func claim(ctx context.Context, q *sqlc.Queries, lockedBy string) (Job, bool, error) {
	row, err := q.ClaimJob(ctx, pgtype.Text{String: lockedBy, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return fromRow(row), true, nil
}
