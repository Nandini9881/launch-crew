package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubham-tomar/launchcrew/internal/db"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

func testPool(t *testing.T) *pgxpool.Pool {
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
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// kindFor returns a test-private kind and removes its rows around the test.
func kindFor(t *testing.T, pool *pgxpool.Pool) jobs.Kind {
	t.Helper()
	k := jobs.Kind("test:" + t.Name())
	clean := func() { _, _ = pool.Exec(context.Background(), `DELETE FROM jobs WHERE kind = $1`, string(k)) }
	clean()
	t.Cleanup(clean)
	return k
}

func jobCount(t *testing.T, pool *pgxpool.Pool, id int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func runNow(t *testing.T, pool *pgxpool.Pool, id int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE jobs SET run_at = now() WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
}

func TestTwoWorkersOneJobRunsOnce(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	q := sqlc.New(pool)
	id, err := jobs.Enqueue(context.Background(), q, kind, map[string]int{"n": 1}, time.Time{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	handlers := map[jobs.Kind]jobs.Handler{kind: func(ctx context.Context, j jobs.Job) error {
		count.Add(1)
		time.Sleep(200 * time.Millisecond)
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, name := range []string{"w1", "w2"} {
		w := &jobs.Worker{Pool: pool, Handlers: handlers, ID: name, Concurrency: 2, Poll: 20 * time.Millisecond}
		wg.Add(1)
		go func() { defer wg.Done(); _ = w.Run(ctx) }()
	}
	time.Sleep(1500 * time.Millisecond)
	cancel()
	wg.Wait()
	if got := count.Load(); got != 1 {
		t.Fatalf("handler ran %d times, want 1", got)
	}
	if jobCount(t, pool, id) != 0 {
		t.Fatal("job row should be deleted")
	}
}

func failToDead(t *testing.T, pool *pgxpool.Pool, id int64, kind jobs.Kind, handlers map[jobs.Kind]jobs.Handler) {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(pool)
	for i := 0; i < 5; i++ {
		runNow(t, pool, id)
		handled, err := jobs.RunOnce(ctx, q, handlers, "t")
		if err != nil || !handled {
			t.Fatalf("run %d: handled=%v err=%v", i, handled, err)
		}
	}
}

func TestFiveFailuresDeadLetter(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	id, err := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { return errors.New("boom") }}
	failToDead(t, pool, id, kind, handlers)

	dead, err := jobs.DeadLetters(ctx, q, nil)
	if err != nil {
		t.Fatal(err)
	}
	var found *jobs.Job
	for i := range dead {
		if dead[i].ID == id {
			found = &dead[i]
		}
	}
	if found == nil || !found.Dead || found.Attempts != 5 || found.LastError != "boom" {
		t.Fatalf("dead letter = %+v", found)
	}
	runNow(t, pool, id)
	if handled, err := jobs.RunOnce(ctx, q, handlers, "t"); err != nil || handled {
		t.Fatalf("dead job claimed: handled=%v err=%v", handled, err)
	}
}

func TestBackoffDoublesPerAttempt(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	id, _ := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { return errors.New("x") }}
	for _, want := range []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second} {
		runNow(t, pool, id)
		if _, err := jobs.RunOnce(ctx, q, handlers, "t"); err != nil {
			t.Fatal(err)
		}
		var secs float64
		if err := pool.QueryRow(ctx, `SELECT extract(epoch FROM run_at - now()) FROM jobs WHERE id=$1`, id).Scan(&secs); err != nil {
			t.Fatal(err)
		}
		if d := time.Duration(secs * float64(time.Second)); d > want+time.Second || d < want-3*time.Second {
			t.Fatalf("backoff = %v, want about %v", d, want)
		}
	}
}

func TestFutureRunAtNotClaimed(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	if _, err := jobs.Enqueue(ctx, q, kind, nil, time.Now().Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	var ran atomic.Bool
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { ran.Store(true); return nil }}
	handled, err := jobs.RunOnce(ctx, q, handlers, "t")
	if err != nil || handled || ran.Load() {
		t.Fatalf("handled=%v ran=%v err=%v", handled, ran.Load(), err)
	}
}

func TestRetryResetsDeadJob(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	id, _ := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { return errors.New("boom") }}
	failToDead(t, pool, id, kind, handlers)
	if err := jobs.Retry(ctx, q, id); err != nil {
		t.Fatal(err)
	}
	handled, err := jobs.RunOnce(ctx, q, handlers, "t")
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}

func lastError(t *testing.T, pool *pgxpool.Pool, id int64) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT last_error FROM jobs WHERE id=$1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUnknownKindFails(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	id, _ := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	handled, err := jobs.RunOnce(ctx, q, map[jobs.Kind]jobs.Handler{}, "t")
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if got := lastError(t, pool, id); got != "no handler for kind" {
		t.Fatalf("last_error = %q", got)
	}
}

func TestPanicIsFailure(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	id, _ := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { panic("kaboom") }}
	handled, err := jobs.RunOnce(ctx, q, handlers, "t")
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	var attempts int
	_ = pool.QueryRow(ctx, `SELECT attempts FROM jobs WHERE id=$1`, id).Scan(&attempts)
	if attempts != 1 || lastError(t, pool, id) == "" {
		t.Fatalf("attempts=%d last_error=%q", attempts, lastError(t, pool, id))
	}
}

func TestCancelledWorkerReleasesWithoutAttempt(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	q := sqlc.New(pool)
	id, _ := jobs.Enqueue(context.Background(), q, kind, nil, time.Time{}, nil)
	started := make(chan struct{})
	handlers := map[jobs.Kind]jobs.Handler{kind: func(ctx context.Context, j jobs.Job) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = jobs.RunOnce(ctx, q, handlers, "t")
	}()
	<-started
	cancel()
	<-done
	var attempts int
	var locked bool
	var lastErr *string
	if err := pool.QueryRow(context.Background(),
		`SELECT attempts, locked_at IS NOT NULL, last_error FROM jobs WHERE id=$1`, id).Scan(&attempts, &locked, &lastErr); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || locked || lastErr != nil {
		t.Fatalf("attempts=%d locked=%v last_error=%v", attempts, locked, lastErr)
	}
	ok := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { return nil }}
	if handled, err := jobs.RunOnce(context.Background(), q, ok, "t"); err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}

func TestReapStaleUnlocksOnlyOldLocks(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	oldID, _ := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	newID, _ := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	for id, age := range map[int64]string{oldID: "20 minutes", newID: "1 minute"} {
		if _, err := pool.Exec(ctx, `UPDATE jobs SET locked_at = now() - $2::interval, locked_by='x' WHERE id=$1`, id, age); err != nil {
			t.Fatal(err)
		}
	}
	n, err := jobs.ReapStale(ctx, q, 12*time.Minute)
	if err != nil || n < 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	var oldLocked, newLocked bool
	_ = pool.QueryRow(ctx, `SELECT locked_at IS NOT NULL FROM jobs WHERE id=$1`, oldID).Scan(&oldLocked)
	_ = pool.QueryRow(ctx, `SELECT locked_at IS NOT NULL FROM jobs WHERE id=$1`, newID).Scan(&newLocked)
	if oldLocked || !newLocked {
		t.Fatalf("oldLocked=%v newLocked=%v", oldLocked, newLocked)
	}
}

func TestLongMultibyteErrorTruncatedSafely(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)
	id, _ := jobs.Enqueue(ctx, q, kind, nil, time.Time{}, nil)
	msg := strings.Repeat("é世\x00", 1000)
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { return errors.New(msg) }}
	if handled, err := jobs.RunOnce(ctx, q, handlers, "t"); err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	got := lastError(t, pool, id)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 2000 || strings.ContainsRune(got, 0) {
		t.Fatalf("bad last_error: %d runes", utf8.RuneCountInString(got))
	}
}

func TestFailedJobIsLoggedAndDeadLetterSaysSo(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	q := sqlc.New(pool)
	id, err := jobs.Enqueue(context.Background(), q, kind, map[string]int{}, time.Time{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	var mu sync.Mutex
	log.SetOutput(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) }))
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { return errors.New("boom") }}
	for i := 1; i <= 5; i++ {
		runNow(t, pool, id)
		if _, err := jobs.RunOnce(context.Background(), q, handlers, "w"); err != nil {
			t.Fatal(err)
		}
	}
	out := buf.String()
	if !strings.Contains(out, fmt.Sprintf("job failed kind=%s id=%d attempt=1 err=boom", kind, id)) ||
		!strings.Contains(out, "attempt=5 err=boom") {
		t.Fatalf("log lacks the failures: %q", out)
	}
	if strings.Count(out, "dead-lettered") != 1 || !strings.Contains(out, fmt.Sprintf("job dead-lettered kind=%s id=%d", kind, id)) {
		t.Fatalf("log lacks one dead-letter line: %q", out)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func staleJob(t *testing.T, pool *pgxpool.Pool, kind jobs.Kind) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := jobs.Enqueue(ctx, sqlc.New(pool), kind, nil, time.Time{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE jobs SET locked_at = now() - interval '20 minutes', locked_by = 'crashed' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func waitFor(t *testing.T, what string, d time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Reaping stale locks is one UPDATE the ready index cannot serve, so it runs
// once at start and then on its own ticker, never on every idle poll.
func TestReapRunsOnTickerNotOnIdlePolls(t *testing.T) {
	pool := testPool(t)
	kind := kindFor(t, pool)
	handlers := map[jobs.Kind]jobs.Handler{kind: func(context.Context, jobs.Job) error { return nil }}
	locked := func(id int64) bool {
		var l bool
		_ = pool.QueryRow(context.Background(), `SELECT locked_at IS NOT NULL FROM jobs WHERE id = $1`, id).Scan(&l)
		return l
	}
	run := func(w *jobs.Worker) (cancel func()) {
		ctx, stop := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); _ = w.Run(ctx) }()
		return func() { stop(); <-done }
	}

	first := staleJob(t, pool, kind)
	w := &jobs.Worker{Pool: pool, Handlers: handlers, ID: "w", Concurrency: 2, Poll: 20 * time.Millisecond, ReapEvery: time.Hour}
	stop := run(w)
	// The start-up reap frees the first stale job, which then runs and is deleted.
	waitFor(t, "start-up reap", 3*time.Second, func() bool { return jobCount(t, pool, first) == 0 })
	second := staleJob(t, pool, kind)
	time.Sleep(500 * time.Millisecond) // dozens of idle polls
	if !locked(second) || jobCount(t, pool, second) != 1 {
		t.Fatal("an idle poll reaped the stale lock")
	}
	stop()

	w.ReapEvery = 100 * time.Millisecond
	stop = run(w)
	defer stop()
	waitFor(t, "ticker reap", 3*time.Second, func() bool { return jobCount(t, pool, second) == 0 })
}
