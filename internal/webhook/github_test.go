package webhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/shubham-tomar/launchcrew/internal/db"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
	"github.com/shubham-tomar/launchcrew/internal/webhook"
)

var secret = []byte("whsec_test")

const publishedBody = `{"action":"published","release":{"tag_name":"v1.0.0"}}`

func setup(t *testing.T, deliveryID string) (*sqlc.Queries, *atomic.Int32, *webhook.GitHub) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	clean := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM webhook_deliveries WHERE delivery_id = $1`, deliveryID)
	}
	clean()
	t.Cleanup(clean)
	q := sqlc.New(pool)
	calls := new(atomic.Int32)
	h := &webhook.GitHub{Secret: secret, Q: q, Enqueue: func(context.Context, jobs.Kind, any) error {
		calls.Add(1)
		return nil
	}}
	return q, calls, h
}

func sign(body string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func req(body, event, delivery, sig string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader(body))
	if sig != "" {
		r.Header.Set("X-Hub-Signature-256", sig)
	}
	if event != "" {
		r.Header.Set("X-GitHub-Event", event)
	}
	if delivery != "" {
		r.Header.Set("X-GitHub-Delivery", delivery)
	}
	return r
}

func TestBadHMAC401WritesNothing(t *testing.T) {
	id := "test-badhmac-" + t.Name()
	q, calls, h := setup(t, id)
	for _, sig := range []string{"", "sha256=" + strings.Repeat("0", 64), "nonsense", "sha256=zz"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req(publishedBody, "release", id, sig))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("sig %q: code = %d, want 401", sig, w.Code)
		}
	}
	if _, err := q.GetWebhookDelivery(context.Background(), id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delivery row exists after bad signature: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("job enqueued after bad signature")
	}
}

func TestValidDelivery202Under50ms(t *testing.T) {
	id := "test-valid-" + t.Name()
	q, _, h := setup(t, id)
	h.Enqueue = func(context.Context, jobs.Kind, any) error {
		go time.Sleep(2 * time.Second)
		return nil
	}
	w := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(w, req(publishedBody, "release", id, sign(publishedBody)))
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("handler took %v", d)
	}
	if w.Code != http.StatusAccepted || w.Body.Len() != 0 {
		t.Fatalf("code = %d body = %q", w.Code, w.Body.String())
	}
	row, err := q.GetWebhookDelivery(context.Background(), id)
	if err != nil || row.Event != "release" {
		t.Fatalf("row = %+v err = %v", row, err)
	}
}

func TestDuplicateDelivery202NoSecondJob(t *testing.T) {
	id := "test-dup-" + t.Name()
	q, calls, h := setup(t, id)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req(publishedBody, "release", id, sign(publishedBody)))
		if w.Code != http.StatusAccepted {
			t.Fatalf("request %d: code = %d", i, w.Code)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("enqueue calls = %d, want 1", n)
	}
	if _, err := q.GetWebhookDelivery(context.Background(), id); err != nil {
		t.Fatalf("first row missing: %v", err)
	}
}

func TestNonReleaseEventIgnored202(t *testing.T) {
	id := "test-nonrelease-" + t.Name()
	_, calls, h := setup(t, id)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req(`{"zen":"hi"}`, "ping", id, sign(`{"zen":"hi"}`)))
	if w.Code != http.StatusAccepted {
		t.Fatalf("ping: code = %d", w.Code)
	}
	id2 := id + "-created"
	body := `{"action":"created"}`
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req(body, "release", id2, sign(body)))
	t.Cleanup(func() { _ = deleteRow(t, h, id2) })
	if w.Code != http.StatusAccepted {
		t.Fatalf("created: code = %d", w.Code)
	}
	if calls.Load() != 0 {
		t.Fatal("job enqueued for non-published event")
	}
}

func deleteRow(t *testing.T, h *webhook.GitHub, id string) error {
	return h.Q.DeleteWebhookDelivery(context.Background(), id)
}

func TestOversizeBody413(t *testing.T) {
	id := "test-oversize-" + t.Name()
	_, calls, h := setup(t, id)
	body := `{"pad":"` + strings.Repeat("a", 1<<20) + `"}`
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req(body, "release", id, sign(body)))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", w.Code)
	}
	if calls.Load() != 0 {
		t.Fatal("job enqueued")
	}
}

func TestMissingDeliveryID400(t *testing.T) {
	_, calls, h := setup(t, "unused-"+t.Name())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req(publishedBody, "release", "", sign(publishedBody)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
	if calls.Load() != 0 {
		t.Fatal("job enqueued")
	}
}

func TestEnqueueFailureDeletesRowAnd500(t *testing.T) {
	id := "test-enqfail-" + t.Name()
	q, _, h := setup(t, id)
	h.Enqueue = func(context.Context, jobs.Kind, any) error { return errors.New("boom") }
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req(publishedBody, "release", id, sign(publishedBody)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500", w.Code)
	}
	if _, err := q.GetWebhookDelivery(context.Background(), id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("row should be deleted for retry: %v", err)
	}
}

func TestEmptySecretRejects(t *testing.T) {
	id := "test-emptysecret-" + t.Name()
	q, calls, h := setup(t, id)
	h.Secret = nil
	m := hmac.New(sha256.New, nil)
	m.Write([]byte(publishedBody))
	sig := "sha256=" + hex.EncodeToString(m.Sum(nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req(publishedBody, "release", id, sig))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
	if _, err := q.GetWebhookDelivery(context.Background(), id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("row exists: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("job enqueued")
	}
}

func TestRetryAfterEnqueueFailureSucceeds(t *testing.T) {
	id := "test-retry-" + t.Name()
	q, calls, h := setup(t, id)
	h.Enqueue = func(context.Context, jobs.Kind, any) error { return errors.New("boom") }
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req(publishedBody, "release", id, sign(publishedBody)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("first code = %d", w.Code)
	}
	h.Enqueue = func(context.Context, jobs.Kind, any) error { calls.Add(1); return nil }
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req(publishedBody, "release", id, sign(publishedBody)))
	if w.Code != http.StatusAccepted {
		t.Fatalf("retry code = %d", w.Code)
	}
	if calls.Load() != 1 {
		t.Fatalf("enqueue calls = %d, want 1", calls.Load())
	}
	if _, err := q.GetWebhookDelivery(context.Background(), id); err != nil {
		t.Fatalf("row missing: %v", err)
	}
}

func TestSignedNonJSON400(t *testing.T) {
	id := "test-nonjson-" + t.Name()
	q, calls, h := setup(t, id)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req("not json", "release", id, sign("not json")))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", w.Code)
	}
	if _, err := q.GetWebhookDelivery(context.Background(), id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("row exists: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("job enqueued")
	}
}
