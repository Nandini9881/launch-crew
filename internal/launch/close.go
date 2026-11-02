package launch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

// CloseWindowHandler handles jobs.CloseWindow: a launched launch becomes
// closed, so "land in the window" is no longer offered. Other statuses are
// left alone.
func CloseWindowHandler(d Deps) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		var p LaunchPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		n, err := d.Q.CloseLaunch(ctx, p.LaunchID)
		if err != nil {
			return fmt.Errorf("close launch: %w", err)
		}
		if n == 0 {
			log.Printf("launch %s: not launched, nothing to close", p.LaunchID)
		}
		return nil
	}
}

func pgInt8(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }
