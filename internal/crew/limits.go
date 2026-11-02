package crew

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

const (
	openRatePerMinute = 10
	defaultTokenTTL   = 30 * 24 * time.Hour
)

// NewToken returns a fresh link token and the hash that is stored.
// Only the hash is ever written; the plain token goes into the link.
func NewToken() (plain string, hash []byte) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crew: random token: %v", err)) // crypto/rand never fails on supported platforms
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	return plain, HashToken(plain)
}

// HashToken is the SHA-256 of the token bytes, as stored in
// invite_token_hash and open_token_hash.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func (d Deps) tokenTTL() time.Duration {
	if d.TokenTTL > 0 {
		return d.TokenTTL
	}
	return defaultTokenTTL
}

// rateLimit counts one open of the launch from ip in the current minute.
func rateLimit(ctx context.Context, d Deps, launchID uuid.UUID, ip net.IP) error {
	addr := addrOf(ip)
	n, err := d.Q.UpsertOpenRate(ctx, sqlc.UpsertOpenRateParams{
		LaunchID: launchID, Ip: addr,
		Minute: pgtype.Timestamptz{Time: d.now().UTC().Truncate(time.Minute), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("crew: open rate: %w", err)
	}
	if n > openRatePerMinute {
		return ErrRateLimited
	}
	return nil
}

// checkCap returns ErrCrewFull when the launch has crew_cap rows.
func checkCap(ctx context.Context, d Deps, l sqlc.Launch) error {
	n, err := d.Q.CountCrewByLaunch(ctx, l.ID)
	if err != nil {
		return fmt.Errorf("crew: count crew: %w", err)
	}
	if n >= int64(l.CrewCap) {
		return ErrCrewFull
	}
	return nil
}

// addrOf converts ip for the inet column; an unknown address counts as 0.0.0.0.
func addrOf(ip net.IP) netip.Addr {
	if a, ok := netip.AddrFromSlice(ip); ok {
		return a.Unmap()
	}
	return netip.IPv4Unspecified()
}

// RateLimit counts one open of the launch from ip against the shared
// per-minute budget; the settings opt-out link spends it too.
func RateLimit(ctx context.Context, d Deps, launchID uuid.UUID, ip net.IP) error {
	return rateLimit(ctx, d, launchID, ip)
}
