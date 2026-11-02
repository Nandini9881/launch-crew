package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shubham-tomar/launchcrew/internal/buffer"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

const (
	draftPosted = "posted"
	draftFailed = "failed_in_buffer"
	pageMax     = 100 // PostsCreatedBetween returns at most this many
)

// PollStatusHandler handles jobs.PollStatus, once after the window: each
// queued crew draft is looked up in its member's Buffer and marked posted
// (sent) or failed_in_buffer (errored, or gone). Drafts still scheduled
// stay queued. A member with a posted draft becomes posted.
func PollStatusHandler(d Deps) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		var p LaunchPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		l, err := getLaunch(ctx, d.Q, p.LaunchID)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		rows, err := d.Q.ListQueuedCrewDraftsByLaunch(ctx, l.ID)
		if err != nil {
			return fmt.Errorf("list queued drafts: %w", err)
		}
		type key struct{ account, channel string }
		groups := map[key][]sqlc.ListQueuedCrewDraftsByLaunchRow{}
		var order []key
		for _, r := range rows {
			k := key{r.BufferAccountID.String, r.ChannelID}
			if _, ok := groups[k]; !ok {
				order = append(order, k)
			}
			groups[k] = append(groups[k], r)
		}
		var errs []error
		posted := map[uuid.UUID]bool{}
		from, to := l.CreatedAt.Time, d.now().Add(lookupAhead)
		for _, k := range order {
			conn, err := d.Q.GetBufferConnectionByAccount(ctx, k.account)
			if errors.Is(err, pgx.ErrNoRows) {
				continue // disconnected since; nothing to ask
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}
			posts, err := d.Buffer.PostsCreatedBetween(buffer.WithLaunch(ctx, l.ID), conn.ID, k.channel, from, to)
			if err != nil {
				errs = append(errs, fmt.Errorf("posts for %s: %w", k.channel, err))
				continue
			}
			byID := map[string]buffer.Post{}
			for _, p := range posts {
				byID[p.ID] = p
			}
			for _, r := range groups[k] {
				post, found := byID[r.BufferPostID.String]
				var status string
				switch {
				case found && post.Status == "sent":
					status = draftPosted
				case found && post.Status == "error":
					status = draftFailed
				case !found && len(posts) < pageMax:
					status = draftFailed // deleted in Buffer
				default:
					continue // still scheduled, or beyond the page
				}
				if err := d.Q.SetCrewDraftStatus(ctx, sqlc.SetCrewDraftStatusParams{ID: r.ID, Status: status}); err != nil {
					errs = append(errs, err)
					continue
				}
				if status == draftPosted {
					posted[r.CrewMemberID] = true
				}
			}
		}
		for id := range posted {
			if err := d.Q.MarkCrewMemberPosted(ctx, id); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			log.Printf("launch %s: poll: %d errors", l.ID, len(errs))
		}
		return errors.Join(errs...)
	}
}
