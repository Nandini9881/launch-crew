package launch

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

// Repo is a repository the captain installed the App on.
type Repo struct {
	ID              uuid.UUID `json:"id"`
	FullName        string    `json:"fullName"`
	Enabled         bool      `json:"enabled"`
	AllowPrerelease bool      `json:"allowPrerelease"`
}

// RepoPatch changes a repo's flags; nil fields are left alone.
type RepoPatch struct {
	Enabled         *bool `json:"enabled"`
	AllowPrerelease *bool `json:"allowPrerelease"`
}

func repoOf(r sqlc.Repo) Repo {
	return Repo{ID: r.ID, FullName: r.FullName, Enabled: r.Enabled, AllowPrerelease: r.AllowPrerelease}
}

// Repos lists the captain's repos by name.
func Repos(ctx context.Context, d Deps, userID uuid.UUID) ([]Repo, error) {
	rs, err := d.Q.ListReposByOwner(ctx, uuid.NullUUID{UUID: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("launch: list repos: %w", err)
	}
	out := make([]Repo, 0, len(rs))
	for _, r := range rs {
		out = append(out, repoOf(r))
	}
	return out, nil
}

// UpdateRepo sets a repo's flags. A repo the user does not own is ErrNotFound.
func UpdateRepo(ctx context.Context, d Deps, userID, repoID uuid.UUID, p RepoPatch) (Repo, error) {
	r, err := d.Q.GetRepo(ctx, repoID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!r.OwnerUserID.Valid || r.OwnerUserID.UUID != userID)) {
		return Repo{}, ErrNotFound
	}
	if err != nil {
		return Repo{}, fmt.Errorf("launch: load repo: %w", err)
	}
	arg := sqlc.UpdateRepoFlagsParams{ID: r.ID, Enabled: r.Enabled, AllowPrerelease: r.AllowPrerelease}
	if p.Enabled != nil {
		arg.Enabled = *p.Enabled
	}
	if p.AllowPrerelease != nil {
		arg.AllowPrerelease = *p.AllowPrerelease
	}
	r, err = d.Q.UpdateRepoFlags(ctx, arg)
	if err != nil {
		return Repo{}, fmt.Errorf("launch: update repo: %w", err)
	}
	return repoOf(r), nil
}
