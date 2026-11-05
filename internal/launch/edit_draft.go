package launch

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/drafting"
)

// EditCaptainDraft replaces the text of one of the captain's own posts. Only a
// draft launch can be edited, and only a post that has not been sent. The text
// must be non-empty and fit the channel's character limit.
func EditCaptainDraft(ctx context.Context, d Deps, launchID, draftID uuid.UUID, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return invalid("a post cannot be empty")
	}
	s, err := load(ctx, d, launchID)
	if err != nil {
		return err
	}
	if s.l.Status != statusDraft {
		return ErrNotDraft
	}
	all, err := d.Q.ListCaptainDrafts(ctx, launchID)
	if err != nil {
		return fmt.Errorf("launch: load captain drafts: %w", err)
	}
	i := slices.IndexFunc(all, func(c sqlc.CaptainDraft) bool { return c.ID == draftID })
	if i < 0 {
		return ErrNotFound
	}
	cd := all[i]
	if cd.Status != "draft" || cd.BufferPostID.Valid {
		return invalid("this post has already been sent to Buffer")
	}
	if ch, ok := s.channels[cd.ChannelID]; ok {
		max := drafting.RulesFor(drafting.Network(ch.Service), s.cfg).MaxChars
		if n := utf8.RuneCountInString(text); max > 0 && n > max {
			return invalid("that post is %d characters and %s allows %d", n, ch.Service, max)
		}
	}
	if _, err := d.Pool.Exec(ctx,
		`UPDATE captain_drafts SET text = $3 WHERE id = $2 AND launch_id = $1 AND status = 'draft' AND buffer_post_id IS NULL`,
		launchID, draftID, text); err != nil {
		return fmt.Errorf("launch: edit captain draft: %w", err)
	}
	return nil
}
