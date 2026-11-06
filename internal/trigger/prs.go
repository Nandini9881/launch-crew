package trigger

import (
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// rowQuerier is satisfied by a pgx pool or transaction.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// MemberPRs returns every PR credited to a crew member, newest first. A row
// without a stored list (made before several PRs were credited) is the single
// PR in pr_number and pr_title; with no PR at all it returns nil.
func MemberPRs(ctx context.Context, q rowQuerier, memberID uuid.UUID, number int32, title string) []PRRef {
	single := func() []PRRef {
		if number == 0 && title == "" {
			return nil
		}
		return []PRRef{{Number: int(number), Title: title}}
	}
	if q == nil {
		return single()
	}
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT prs FROM crew_members WHERE id = $1`, memberID).Scan(&raw); err != nil || len(raw) == 0 {
		if err != nil && err != pgx.ErrNoRows {
			log.Printf("trigger: read prs of %s: %v", memberID, err)
		}
		return single()
	}
	var prs []PRRef
	if err := json.Unmarshal(raw, &prs); err != nil || len(prs) == 0 {
		return single()
	}
	return prs
}
