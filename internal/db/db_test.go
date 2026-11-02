package db_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubham-tomar/launchcrew/internal/db"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestMigrateAndQuery(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	q := sqlc.New(pool)
	u, err := q.CreateUser(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	got, err := q.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("got %v, want %v", got.ID, u.ID)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t)
	for i := 0; i < 2; i++ {
		if err := db.Migrate(ctx, pool); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}
}

func TestJobQueueClaimAndFail(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(pool)
	if _, err := pool.Exec(ctx, "TRUNCATE jobs"); err != nil {
		t.Fatal(err)
	}
	job, err := q.EnqueueJob(ctx, sqlc.EnqueueJobParams{Kind: "t", Payload: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := q.ClaimJob(ctx, pgtype.Text{String: "w1", Valid: true})
	if err != nil || claimed.ID != job.ID {
		t.Fatalf("claim: %v %v", claimed.ID, err)
	}
	if _, err := q.ClaimJob(ctx, pgtype.Text{String: "w2", Valid: true}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second claim should find nothing, got %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := q.FailJob(ctx, sqlc.FailJobParams{ID: job.ID, LastError: pgtype.Text{String: "boom", Valid: true},
			RunAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Second), Valid: true}}); err != nil {
			t.Fatal(err)
		}
	}
	dead, err := q.ListDeadLetters(ctx, uuid.NullUUID{})
	if err != nil || len(dead) != 1 || !dead[0].Dead || dead[0].Attempts != 5 {
		t.Fatalf("dead letters: %+v %v", dead, err)
	}
	if _, err := q.ClaimJob(ctx, pgtype.Text{String: "w1", Valid: true}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("dead job must not be claimable, got %v", err)
	}
}

func TestMarkLaunchedOnlyFromDraftAndWebhookDedupe(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(pool)
	u, _ := q.CreateUser(ctx)
	l, err := q.CreateLaunch(ctx, sqlc.CreateLaunchParams{OwnerUserID: u.ID, Source: "manual",
		HeadlineChanges: []byte(`[]`), Roles: []byte(`[]`), WindowMinutes: 90, CrewCap: 200})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.MarkLaunched(ctx, l.ID); err != nil {
		t.Fatalf("first mark: %v", err)
	}
	if _, err := q.MarkLaunched(ctx, l.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second mark should match no rows, got %v", err)
	}
	id := "d-" + l.ID.String()
	arg := sqlc.InsertWebhookDeliveryParams{DeliveryID: id, Event: "release", Payload: []byte(`{}`)}
	if _, err := q.InsertWebhookDelivery(ctx, arg); err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertWebhookDelivery(ctx, arg); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("duplicate delivery should return no rows, got %v", err)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	boom := errors.New("boom")
	err := db.WithTx(ctx, pool, func(q *sqlc.Queries) error {
		u, err := q.CreateUser(ctx)
		id = u.ID
		if err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if _, err := sqlc.New(pool).GetUser(ctx, id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("user should have been rolled back, got %v", err)
	}
}

func TestListRecentQueuedTextsNewestFirst(t *testing.T) {
	ctx := context.Background()
	pool := openTestPool(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	q := sqlc.New(pool)
	u, _ := q.CreateUser(ctx)
	l, err := q.CreateLaunch(ctx, sqlc.CreateLaunchParams{OwnerUserID: u.ID, Source: "manual",
		HeadlineChanges: []byte(`[]`), Roles: []byte(`[]`), WindowMinutes: 90, CrewCap: 200})
	if err != nil {
		t.Fatal(err)
	}
	m, err := q.InsertCrewMember(ctx, sqlc.InsertCrewMemberParams{LaunchID: l.ID, State: "connected"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"third", "second", "first"}
	for _, text := range []string{"first", "second", "third"} {
		d, err := q.UpsertCrewDraft(ctx, sqlc.UpsertCrewDraftParams{CrewMemberID: m.ID, ChannelID: text, Network: "x", Text: text})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.UpdateCrewDraftPosting(ctx, sqlc.UpdateCrewDraftPostingParams{ID: d.ID, Status: "queued"}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	got, err := q.ListRecentQueuedTexts(ctx, sqlc.ListRecentQueuedTextsParams{LaunchID: l.ID, Network: "x", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want[:2])
	}
}
