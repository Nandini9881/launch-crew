package jobs_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

// rotation returns a keyring with only the old key, and one where a new key
// leads and the old key still decrypts. Ids are random per test.
func rotation(t *testing.T) (old, rotated *crypto.Keyring, newID string) {
	t.Helper()
	entry := func(id string) string {
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			t.Fatal(err)
		}
		return id + ":" + base64.StdEncoding.EncodeToString(k)
	}
	oldEnv := entry("old-" + uuid.NewString()[:8])
	newEnv := entry("new-" + uuid.NewString()[:8])
	var err error
	if old, err = crypto.ParseKeyring(oldEnv); err != nil {
		t.Fatal(err)
	}
	if rotated, err = crypto.ParseKeyring(newEnv + "," + oldEnv); err != nil {
		t.Fatal(err)
	}
	return old, rotated, rotated.CurrentKeyID()
}

type encRows struct {
	connID, launchID, memberID uuid.UUID
}

// seedRows stores one token of each kind, encrypted under kr.
func seedRows(t *testing.T, q *sqlc.Queries, kr *crypto.Keyring) encRows {
	t.Helper()
	ctx := context.Background()
	enc := func(s string) []byte {
		b, _, err := kr.Encrypt([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	u, err := q.CreateUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	access, keyID, _ := kr.Encrypt([]byte("access-1"))
	conn, err := q.UpsertBufferConnection(ctx, sqlc.UpsertBufferConnectionParams{
		UserID: u.ID, BufferAccountID: "acct-" + uuid.NewString(),
		AccessTokenEnc: access, RefreshTokenEnc: enc("refresh-1"), KeyID: keyID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
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
	if err := q.SetLaunchOpenTokens(ctx, sqlc.SetLaunchOpenTokensParams{
		ID: l.ID, OpenTokenHash: []byte(uuid.NewString()), OpenTokenEnc: enc("open-1"),
	}); err != nil {
		t.Fatal(err)
	}
	m, err := q.InsertCrewMember(ctx, sqlc.InsertCrewMemberParams{
		LaunchID: l.ID, Role: "contributor", State: "ticked", InviteTokenHash: []byte(uuid.NewString()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := q.InviteCrewMember(ctx, sqlc.InviteCrewMemberParams{
		ID: m.ID, InviteTokenHash: []byte(uuid.NewString()), InviteTokenEnc: enc("invite-1"),
		TokenExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}); err != nil || n != 1 {
		t.Fatalf("invite: %d %v", n, err)
	}
	return encRows{conn.ID, l.ID, m.ID}
}

func TestReencryptMovesEveryRowToCurrentKey(t *testing.T) {
	pool := testPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	old, rotated, newID := rotation(t)
	rows := seedRows(t, q, old)
	res, err := jobs.Reencrypt(ctx, q, rotated)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rotated < 3 {
		t.Fatalf("rotated = %d, want at least 3", res.Rotated)
	}

	conn, _ := q.GetBufferConnection(ctx, rows.connID)
	l, _ := q.GetLaunch(ctx, rows.launchID)
	m, _ := q.GetCrewMember(ctx, rows.memberID)
	if conn.KeyID != newID {
		t.Fatalf("key_id = %q", conn.KeyID)
	}
	for _, c := range []struct {
		name string
		enc  []byte
		want string
	}{
		{"access", conn.AccessTokenEnc, "access-1"}, {"refresh", conn.RefreshTokenEnc, "refresh-1"},
		{"open", l.OpenTokenEnc, "open-1"}, {"invite", m.InviteTokenEnc, "invite-1"},
	} {
		if !bytes.HasPrefix(c.enc, []byte(newID+":")) {
			t.Errorf("%s not on the new key", c.name)
		}
		got, err := rotated.Decrypt(c.enc)
		if err != nil || string(got) != c.want {
			t.Errorf("%s = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestReencryptIdempotent(t *testing.T) {
	pool := testPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	old, rotated, _ := rotation(t)
	rows := seedRows(t, q, old)
	first, err := jobs.Reencrypt(ctx, q, rotated)
	if err != nil || first.Rotated < 3 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	before, _ := q.GetCrewMember(ctx, rows.memberID)
	second, err := jobs.Reencrypt(ctx, q, rotated)
	if err != nil {
		t.Fatal(err)
	}
	if second.Rotated != 0 {
		t.Fatalf("second run rotated %d rows, want 0", second.Rotated)
	}
	after, _ := q.GetCrewMember(ctx, rows.memberID)
	if !bytes.Equal(before.InviteTokenEnc, after.InviteTokenEnc) {
		t.Fatal("second run rewrote a row")
	}

	// The handler wraps the same pass.
	if err := jobs.ReencryptHandler(q, rotated)(ctx, jobs.Job{Kind: jobs.ReencryptTokens}); err != nil {
		t.Fatal(err)
	}
}

// A refresh that already rotated the row must not be overwritten by a
// re-encryption holding the stale ciphertext.
func TestRotateConnectionTokensSkipsRefreshedRow(t *testing.T) {
	pool := testPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	old, rotated, newID := rotation(t)
	rows := seedRows(t, q, old)
	stale, _ := q.GetBufferConnection(ctx, rows.connID)

	fresh, _, _ := rotated.Encrypt([]byte("access-refreshed"))
	if _, err := pool.Exec(ctx, `UPDATE buffer_connections SET access_token_enc = $2 WHERE id = $1`, rows.connID, fresh); err != nil {
		t.Fatal(err)
	}
	mine, _, _ := rotated.Encrypt([]byte("access-1"))
	n, err := q.RotateConnectionTokens(ctx, sqlc.RotateConnectionTokensParams{
		ID: rows.connID, AccessOld: stale.AccessTokenEnc, AccessNew: mine, RefreshNew: stale.RefreshTokenEnc, KeyID: newID,
	})
	if err != nil || n != 0 {
		t.Fatalf("rotate = %d, %v; want 0 rows", n, err)
	}
	got, _ := q.GetBufferConnection(ctx, rows.connID)
	if !bytes.Equal(got.AccessTokenEnc, fresh) || got.KeyID != stale.KeyID || !bytes.Equal(got.RefreshTokenEnc, stale.RefreshTokenEnc) {
		t.Fatal("row changed by a stale rotation")
	}
}
