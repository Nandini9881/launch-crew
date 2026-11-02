package main

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/db"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

func ring(t *testing.T, id string) *crypto.Keyring {
	t.Helper()
	kr, err := crypto.ParseKeyring(id + ":" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return kr
}

// Boot enqueues one re-encrypt job when any of the three encrypted places
// is off the current key, never twice, and not when all rows are current.
// It runs in a transaction on emptied tables, rolled back at the end.
func TestEnsureReencryptEnqueuesOncePerPlace(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, stmt := range []string{
		`DELETE FROM buffer_connections`,
		`UPDATE launches SET open_token_enc = NULL`,
		`UPDATE crew_members SET invite_token_enc = NULL`,
		`DELETE FROM jobs WHERE kind = 'reencrypt_tokens'`,
	} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	q := sqlc.New(tx)
	oldKR, newKR := ring(t, "old"), ring(t, "new")
	enc := func(kr *crypto.Keyring) []byte {
		b, _, err := kr.Encrypt([]byte("tok"))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	u, err := q.CreateUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	l, err := q.CreateLaunch(ctx, sqlc.CreateLaunchParams{
		OwnerUserID: u.ID, Source: "manual", Title: "t", Summary: "s",
		HeadlineChanges: []byte(`[]`), Roles: []byte(`[]`),
		LaunchAt:      pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
		WindowMinutes: 90, CrewCap: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := q.InsertCrewMember(ctx, sqlc.InsertCrewMemberParams{
		LaunchID: l.ID, Role: "contributor", State: "ticked", InviteTokenHash: []byte(uuid.NewString()),
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := q.UpsertBufferConnection(ctx, sqlc.UpsertBufferConnectionParams{
		UserID: u.ID, BufferAccountID: "acct-" + uuid.NewString(), AccessTokenEnc: enc(newKR), KeyID: "new",
	})
	if err != nil {
		t.Fatal(err)
	}
	setOpen := func(b []byte) {
		if _, err := tx.Exec(ctx, `UPDATE launches SET open_token_enc = $2 WHERE id = $1`, l.ID, b); err != nil {
			t.Fatal(err)
		}
	}
	setInvite := func(b []byte) {
		if _, err := tx.Exec(ctx, `UPDATE crew_members SET invite_token_enc = $2 WHERE id = $1`, m.ID, b); err != nil {
			t.Fatal(err)
		}
	}
	setConn := func(b []byte, key string) {
		if _, err := tx.Exec(ctx, `UPDATE buffer_connections SET access_token_enc = $2, key_id = $3 WHERE id = $1`, conn.ID, b, key); err != nil {
			t.Fatal(err)
		}
	}
	setOpen(enc(newKR))
	setInvite(enc(newKR))

	jobCount := func() int64 {
		n, err := q.CountLiveJobsOfKind(ctx, string(jobs.ReencryptTokens))
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	clear := func() {
		if _, err := tx.Exec(ctx, `DELETE FROM jobs WHERE kind = 'reencrypt_tokens'`); err != nil {
			t.Fatal(err)
		}
	}

	if err := ensureReencrypt(ctx, q, newKR); err != nil || jobCount() != 0 {
		t.Fatalf("all current: jobs = %d, err %v; want 0", jobCount(), err)
	}
	for name, off := range map[string]func(){
		"connection":   func() { setConn(enc(oldKR), "old") },
		"open token":   func() { setOpen(enc(oldKR)) },
		"invite token": func() { setInvite(enc(oldKR)) },
	} {
		off()
		for i := 0; i < 3; i++ {
			if err := ensureReencrypt(ctx, q, newKR); err != nil {
				t.Fatal(err)
			}
		}
		if n := jobCount(); n != 1 {
			t.Fatalf("%s off-key: jobs = %d, want 1", name, n)
		}
		clear()
		setConn(enc(newKR), "new")
		setOpen(enc(newKR))
		setInvite(enc(newKR))
	}
}
