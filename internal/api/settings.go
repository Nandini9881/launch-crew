package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shubham-tomar/launchcrew/internal/crew"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/session"
)

// Settings serves the privacy routes: what is stored, global opt-out and
// deletion. Paths are relative to /api.
type Settings struct {
	deps         crew.Deps
	sessions     *session.Store
	supportEmail string
}

// NewSettings returns the router for /me, /me/opt-out and /opt-out.
func NewSettings(deps crew.Deps, sessions *session.Store, supportEmail string) http.Handler {
	s := &Settings{deps: deps, sessions: sessions, supportEmail: supportEmail}
	r := chi.NewRouter()
	r.NotFound(notFound)
	r.MethodNotAllowed(methodNotAllowed)
	r.Get("/me", s.me)
	r.Post("/me/opt-out", s.optOut)
	r.Delete("/me", s.delete)
	r.Post("/opt-out", s.tokenOptOut)
	return r
}

type meView struct {
	SignedIn        bool     `json:"signedIn"`
	GitHubLogins    []string `json:"githubLogins"`
	BufferConnected bool     `json:"bufferConnected"`
	OptedOut        bool     `json:"optedOut"`
	Launches        int64    `json:"launches"`
	CrewRows        int64    `json:"crewRows"`
	SupportEmail    string   `json:"supportEmail"`
}

func (s *Settings) fail(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("api: %s %s: %v", r.Method, r.URL.Path, err)
	writeError(w, http.StatusInternalServerError, "internal", "something went wrong on our side, try again")
}

func (s *Settings) me(w http.ResponseWriter, r *http.Request) {
	v := meView{GitHubLogins: []string{}, SupportEmail: s.supportEmail}
	uid := s.sessions.Get(r).UserID
	if uid == uuid.Nil {
		writeJSON(w, http.StatusOK, v)
		return
	}
	q := s.deps.Q
	ctx := r.Context()
	ids, err := q.ListGitHubIdentitiesByUser(ctx, uid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, id := range ids {
		v.GitHubLogins = append(v.GitHubLogins, id.Login)
	}
	v.SignedIn = true
	nullUID := uuid.NullUUID{UUID: uid, Valid: true}
	if v.BufferConnected, err = q.UserHasBufferConnection(ctx, uid); err == nil {
		if v.OptedOut, err = q.UserIsOptedOut(ctx, uid); err == nil {
			if v.Launches, err = q.CountUserLaunches(ctx, nullUID); err == nil {
				v.CrewRows, err = q.CountUserCrewRows(ctx, nullUID)
			}
		}
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// optOut records every GitHub id linked to the user. Rows already queued or
// posted keep their state; their drafts are in the person's own Buffer.
func (s *Settings) optOut(w http.ResponseWriter, r *http.Request) {
	uid := s.sessions.Get(r).UserID
	if uid == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "sign_in", "sign in with Buffer first")
		return
	}
	n, err := s.deps.Q.CountUserGitHubIDs(r.Context(), uid)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusConflict, "no_github", "no GitHub account is linked to this session, use the link from a PR comment")
		return
	}
	err = s.inTx(r, func(q *sqlc.Queries) error {
		if err := q.OptOutUser(r.Context(), uid); err != nil {
			return err
		}
		return q.MarkUserCrewOptedOut(r.Context(), uuid.NullUUID{UUID: uid, Valid: true})
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"optedOut": true})
}

// tokenOptOut is the PR comment footer link: the invite token alone opts
// that contributor out, signed in or not. It never checks expiry.
func (s *Settings) tokenOptOut(w http.ResponseWriter, r *http.Request) {
	tok := r.URL.Query().Get("token")
	if tok == "" {
		notFound(w, r)
		return
	}
	ctx := r.Context()
	m, err := s.deps.Q.GetCrewMemberByTokenHash(ctx, crew.HashToken(tok))
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w, r)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := crew.RateLimit(ctx, s.deps, m.LaunchID, clientIP(r)); errors.Is(err, crew.ErrRateLimited) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many tries from here, wait a minute")
		return
	} else if err != nil {
		s.fail(w, r, err)
		return
	}
	err = s.inTx(r, func(q *sqlc.Queries) error {
		if m.GithubUserID.Valid {
			if err := q.InsertOptOut(ctx, m.GithubUserID.Int64); err != nil {
				return err
			}
			return q.MarkGitHubCrewOptedOut(ctx, m.GithubUserID)
		}
		return q.MarkCrewMemberOptedOut(ctx, m.ID)
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"optedOut": true})
}

// delete removes the user's tokens, drafts and crew rows in one transaction.
// opt_outs stays, so a deleted person is still never commented on.
func (s *Settings) delete(w http.ResponseWriter, r *http.Request) {
	uid := s.sessions.Get(r).UserID
	if uid == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "sign_in", "sign in with Buffer first")
		return
	}
	ctx := r.Context()
	nullUID := uuid.NullUUID{UUID: uid, Valid: true}
	err := s.inTx(r, func(q *sqlc.Queries) error {
		steps := []struct {
			name string
			run  func() error
		}{
			{"voice profiles", func() error { return q.DeleteUserVoiceProfiles(ctx, uid) }},
			{"crew drafts", func() error { return q.DeleteUserCrewDrafts(ctx, nullUID) }},
			{"crew rows", func() error { return q.DeleteUserCrewMembers(ctx, nullUID) }},
			{"buffer connections", func() error { return q.DeleteUserBufferConnections(ctx, uid) }},
			{"captain drafts", func() error { return q.DeleteUserCaptainDrafts(ctx, nullUID) }},
			{"launches", func() error { return q.MarkUserLaunchesDeleted(ctx, nullUID) }},
			{"identities", func() error { return q.DeleteUserIdentities(ctx, uid) }},
			{"user", func() error { return q.DeleteUser(ctx, uid) }},
		}
		for _, st := range steps {
			if err := st.run(); err != nil {
				return fmt.Errorf("delete %s: %w", st.name, err)
			}
		}
		return nil
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.sessions.Clear(w)
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Settings) inTx(r *http.Request, fn func(*sqlc.Queries) error) error {
	ctx := r.Context()
	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.deps.Q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
