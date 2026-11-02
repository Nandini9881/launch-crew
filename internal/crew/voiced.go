package crew

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shubham-tomar/launchcrew/internal/buffer"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/drafting"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

const sentPostsForVoice = 100

// VoicedDraftsHandler handles jobs.VoicedDrafts: it derives the member's
// voice (cached 30 days per Buffer account) and redrafts each draft that
// is unedited and not yet in Buffer. Without Force, drafts already from
// the model are kept.
func VoicedDraftsHandler(d Deps) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		var p VoicedPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return fmt.Errorf("crew: voiced payload: %w", err)
		}
		m, err := getMember(ctx, d, p.MemberID)
		if errors.Is(err, ErrNotFound) {
			return nil // swept or deleted
		}
		if err != nil {
			return err
		}
		if !m.BufferAccountID.Valid {
			return nil // never connected
		}
		conn, err := d.Q.GetBufferConnectionByAccount(ctx, m.BufferAccountID.String)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("crew: load connection: %w", err)
		}
		if conn.Status == "needs_reconnect" {
			return nil // disconnected or revoked; voiced on the next connect
		}
		info, err := loadLaunch(ctx, d, m.LaunchID)
		if err != nil {
			return err
		}
		voice, vpID, err := voiceFor(ctx, d, m, conn)
		if err != nil {
			return err
		}

		ds, err := d.Q.ListCrewDrafts(ctx, m.ID)
		if err != nil {
			return fmt.Errorf("crew: list drafts: %w", err)
		}
		var hooks []drafting.HookType
		var todo []sqlc.CrewDraft
		for _, dr := range ds {
			if redraft(dr, p.Force) {
				todo = append(todo, dr)
			} else if dr.HookType.Valid {
				hooks = append(hooks, drafting.HookType(dr.HookType.String))
			}
		}
		var cfg *buffer.Config
		if len(todo) > 0 {
			if c, err := d.Buffer.Configuration(ctx, conn.ID); err == nil {
				cfg = &c
			}
		}
		recent := map[string][]string{}
		for _, dr := range todo {
			r, ok := recent[dr.Network]
			if !ok {
				if r, err = d.Q.ListRecentQueuedTexts(ctx, sqlc.ListRecentQueuedTextsParams{
					LaunchID: info.l.ID, Network: dr.Network, Limit: recentLimit,
				}); err != nil {
					return fmt.Errorf("crew: recent texts: %w", err)
				}
				recent[dr.Network] = r
			}
			n := drafting.Network(dr.Network)
			res, err := d.drafter().DraftFor(ctx, drafting.DraftRequest{
				Subject: subject(info, m), Voice: &voice, Hook: drafting.AssignHook(hooks),
				Network: n, Rules: drafting.RulesFor(n, cfg), AllowModel: true, Avoid: head(r, avoidInPrompt),
			}, r)
			if err != nil {
				return fmt.Errorf("crew: voiced draft: %w", err)
			}
			dd := res.Draft
			hooks = append(hooks, dd.Hook)
			// The query skips rows edited or queued since they were listed.
			if _, err := d.Q.UpdateCrewDraftVoiced(ctx, sqlc.UpdateCrewDraftVoicedParams{
				ID: dr.ID, Text: dd.Text, Thread: threadJSON(dd.Thread),
				FirstComment: pgtype.Text{String: dd.FirstComment, Valid: dd.FirstComment != ""},
				HookType:     pgtype.Text{String: string(dd.Hook), Valid: dd.Hook != ""},
				OverlapScore: pgtype.Float4{Float32: float32(res.Overlap), Valid: true},
				Flagged:      res.Flagged, FromModel: dd.FromModel && !res.UsedTemplate,
			}); err != nil {
				return fmt.Errorf("crew: save voiced draft: %w", err)
			}
		}
		if err := d.Q.SetCrewMemberVoiced(ctx, sqlc.SetCrewMemberVoicedParams{
			ID: m.ID, VoicedAt: pgtype.Timestamptz{Time: d.now(), Valid: true},
			VoiceProfileID: uuid.NullUUID{UUID: vpID, Valid: vpID != uuid.Nil},
		}); err != nil {
			return fmt.Errorf("crew: mark voiced: %w", err)
		}
		return nil
	}
}

func redraft(dr sqlc.CrewDraft, force bool) bool {
	if dr.Edited || dr.Status != statusDraft || dr.BufferPostID.Valid {
		return false
	}
	return force || !dr.FromModel
}

// voiceFor returns the account's voice: the stored profile when updated
// within 30 days, else one derived from sent posts (or the PR title).
func voiceFor(ctx context.Context, d Deps, m sqlc.CrewMember, conn sqlc.BufferConnection) (drafting.Profile, uuid.UUID, error) {
	vp, err := d.Q.GetVoiceProfile(ctx, conn.BufferAccountID)
	if err == nil && vp.UpdatedAt.Time.After(d.now().Add(-voiceFresh)) {
		var p drafting.Profile
		if err := json.Unmarshal(vp.Profile, &p); err == nil {
			return p, vp.ID, nil
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return drafting.Profile{}, uuid.Nil, fmt.Errorf("crew: load voice: %w", err)
	}
	posts, err := d.Buffer.SentPosts(ctx, conn.ID, sentPostsForVoice)
	if err != nil {
		log.Printf("crew: sent posts for voice of %s: %v", m.ID, err)
		posts = nil
	}
	var prText []string
	if m.PrTitle.Valid {
		prText = []string{m.PrTitle.String}
	}
	p, source := drafting.DeriveVoice(posts, prText)
	raw, err := json.Marshal(p)
	if err != nil {
		return drafting.Profile{}, uuid.Nil, err
	}
	vp, err = d.Q.UpsertVoiceProfile(ctx, sqlc.UpsertVoiceProfileParams{
		BufferAccountID: conn.BufferAccountID, Profile: raw, DerivedFrom: source,
	})
	if err != nil {
		return drafting.Profile{}, uuid.Nil, fmt.Errorf("crew: save voice: %w", err)
	}
	return p, vp.ID, nil
}
