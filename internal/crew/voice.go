package crew

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/drafting"
	"github.com/shubham-tomar/launchcrew/internal/jobs"
)

const maxExtraRunes = 80

var (
	lengths  = []string{"short", "medium", "long"}
	cases    = []string{"lowercase", "sentence"}
	hashtags = []string{"none", "few"}
)

type voiceErr struct{ msg string }

func (e voiceErr) Error() string   { return "crew: invalid voice: " + e.msg }
func (e voiceErr) Is(t error) bool { return t == ErrInvalidVoice }

// VoiceMessage is the person-facing sentence for an ErrInvalidVoice.
func VoiceMessage(err error) string {
	var v voiceErr
	if errors.As(err, &v) {
		return v.msg
	}
	return "check the voice options"
}

func oneOf(v string, ok []string) bool {
	for _, o := range ok {
		if v == o {
			return true
		}
	}
	return false
}

// UpdateVoice saves the three radios and the free-text note over the stored
// profile (other fields are kept), then queues one forced redraft of the
// member's unedited drafts. It returns the new voice line.
func UpdateVoice(ctx context.Context, d Deps, m sqlc.CrewMember, in drafting.Profile) (string, error) {
	extra := strings.TrimSpace(in.Extra)
	switch {
	case !oneOf(in.SentenceLength, lengths), !oneOf(in.Case, cases), !oneOf(in.Hashtags, hashtags):
		return "", voiceErr{"pick one option in each group"}
	case utf8.RuneCountInString(extra) > maxExtraRunes:
		return "", voiceErr{"keep it under 80 characters"}
	}
	if !m.BufferAccountID.Valid {
		return "", ErrNotConnected
	}
	acct := m.BufferAccountID.String
	p, ok := storedVoice(ctx, d, acct)
	if !ok {
		p = drafting.Neutral()
	}
	p.SentenceLength, p.Case, p.Hashtags, p.Extra = in.SentenceLength, in.Case, in.Hashtags, extra
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	if _, err := d.Q.UpsertVoiceProfile(ctx, sqlc.UpsertVoiceProfileParams{
		BufferAccountID: acct, Profile: raw, DerivedFrom: "manual",
	}); err != nil {
		return "", fmt.Errorf("crew: save voice: %w", err)
	}
	if d.Enqueue != nil && !forcedPending(ctx, d, m.ID) {
		lid := m.LaunchID
		if _, err := d.Enqueue(ctx, jobs.VoicedDrafts, VoicedPayload{MemberID: m.ID, Force: true}, time.Time{}, &lid); err != nil {
			return "", fmt.Errorf("crew: enqueue voiced drafts: %w", err)
		}
	}
	return drafting.Describe(p), nil
}

// forcedPending reports a live forced VoicedDrafts job; a plain one does not
// count, since it keeps drafts already from the model.
func forcedPending(ctx context.Context, d Deps, memberID uuid.UUID) bool {
	n, err := d.Q.CountLiveForcedMemberJobs(ctx, sqlc.CountLiveForcedMemberJobsParams{Kind: string(jobs.VoicedDrafts), MemberID: memberID.String()})
	return err == nil && n > 0
}
