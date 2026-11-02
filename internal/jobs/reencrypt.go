package jobs

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

const reencryptBatch = 200

// ReencryptResult counts a pass: Rotated rows moved to the current key,
// Failed rows left alone (undecryptable, or changed by a concurrent writer
// that already holds a newer ciphertext).
type ReencryptResult struct {
	Rotated int
	Failed  int
}

// ReencryptHandler handles jobs.ReencryptTokens.
func ReencryptHandler(q *sqlc.Queries, kr *crypto.Keyring) Handler {
	return func(ctx context.Context, _ Job) error {
		res, err := Reencrypt(ctx, q, kr)
		log.Printf("reencrypt: rotated %d rows, skipped %d", res.Rotated, res.Failed)
		return err
	}
}

// Reencrypt moves every stored token off old keys onto the current one:
// Buffer access and refresh tokens, launch open tokens and crew invite
// tokens. Rows go in batches and each write is guarded on the ciphertext
// that was read, so a concurrent refresh is never overwritten. A second
// pass finds nothing to do.
func Reencrypt(ctx context.Context, q *sqlc.Queries, kr *crypto.Keyring) (ReencryptResult, error) {
	var res ReencryptResult
	for _, step := range []func(context.Context, *sqlc.Queries, *crypto.Keyring, *ReencryptResult) error{
		reencryptConnections, reencryptOpenTokens, reencryptInviteTokens,
	} {
		if err := step(ctx, q, kr, &res); err != nil {
			return res, err
		}
	}
	return res, nil
}

func prefix(kr *crypto.Keyring) []byte { return []byte(kr.CurrentKeyID() + ":") }

// reseal re-encrypts one ciphertext under the current key.
func reseal(kr *crypto.Keyring, enc []byte) ([]byte, error) {
	plain, err := kr.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	out, _, err := kr.Encrypt(plain)
	return out, err
}

// record tallies a guarded write; zero rows means another writer got there.
func record(res *ReencryptResult, n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		res.Failed++
		return nil
	}
	res.Rotated++
	return nil
}

func reencryptConnections(ctx context.Context, q *sqlc.Queries, kr *crypto.Keyring, res *ReencryptResult) error {
	after := uuid.Nil
	for {
		rows, err := q.ListConnectionsNotOnKey(ctx, sqlc.ListConnectionsNotOnKeyParams{
			KeyID: kr.CurrentKeyID(), AfterID: after, Batch: reencryptBatch,
		})
		if err != nil {
			return fmt.Errorf("reencrypt: list connections: %w", err)
		}
		for _, r := range rows {
			after = r.ID
			access, err := reseal(kr, r.AccessTokenEnc)
			if err != nil {
				log.Printf("reencrypt: connection %s access token: %v", r.ID, err)
				res.Failed++
				continue
			}
			var refresh []byte
			if r.RefreshTokenEnc != nil {
				if refresh, err = reseal(kr, r.RefreshTokenEnc); err != nil {
					log.Printf("reencrypt: connection %s refresh token: %v", r.ID, err)
					res.Failed++
					continue
				}
			}
			n, err := q.RotateConnectionTokens(ctx, sqlc.RotateConnectionTokensParams{
				ID: r.ID, AccessOld: r.AccessTokenEnc, AccessNew: access, RefreshNew: refresh, KeyID: kr.CurrentKeyID(),
			})
			if err := record(res, n, err); err != nil {
				return fmt.Errorf("reencrypt: connection %s: %w", r.ID, err)
			}
		}
		if len(rows) < reencryptBatch {
			return nil
		}
	}
}

func reencryptOpenTokens(ctx context.Context, q *sqlc.Queries, kr *crypto.Keyring, res *ReencryptResult) error {
	after := uuid.Nil
	for {
		rows, err := q.ListLaunchOpenTokensNotOnKey(ctx, sqlc.ListLaunchOpenTokensNotOnKeyParams{
			Prefix: prefix(kr), AfterID: after, Batch: reencryptBatch,
		})
		if err != nil {
			return fmt.Errorf("reencrypt: list open tokens: %w", err)
		}
		for _, r := range rows {
			after = r.ID
			enc, err := reseal(kr, r.OpenTokenEnc)
			if err != nil {
				log.Printf("reencrypt: launch %s open token: %v", r.ID, err)
				res.Failed++
				continue
			}
			n, err := q.RotateLaunchOpenToken(ctx, sqlc.RotateLaunchOpenTokenParams{ID: r.ID, EncOld: r.OpenTokenEnc, EncNew: enc})
			if err := record(res, n, err); err != nil {
				return fmt.Errorf("reencrypt: launch %s: %w", r.ID, err)
			}
		}
		if len(rows) < reencryptBatch {
			return nil
		}
	}
}

func reencryptInviteTokens(ctx context.Context, q *sqlc.Queries, kr *crypto.Keyring, res *ReencryptResult) error {
	after := uuid.Nil
	for {
		rows, err := q.ListInviteTokensNotOnKey(ctx, sqlc.ListInviteTokensNotOnKeyParams{
			Prefix: prefix(kr), AfterID: after, Batch: reencryptBatch,
		})
		if err != nil {
			return fmt.Errorf("reencrypt: list invite tokens: %w", err)
		}
		for _, r := range rows {
			after = r.ID
			enc, err := reseal(kr, r.InviteTokenEnc)
			if err != nil {
				log.Printf("reencrypt: crew member %s invite token: %v", r.ID, err)
				res.Failed++
				continue
			}
			n, err := q.RotateInviteToken(ctx, sqlc.RotateInviteTokenParams{ID: r.ID, EncOld: r.InviteTokenEnc, EncNew: enc})
			if err := record(res, n, err); err != nil {
				return fmt.Errorf("reencrypt: crew member %s: %w", r.ID, err)
			}
		}
		if len(rows) < reencryptBatch {
			return nil
		}
	}
}
