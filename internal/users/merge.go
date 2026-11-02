// Package users holds operations on whole users.
package users

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

// Merge moves everything owned by from to into and deletes from, in
// one transaction. from == into is a no-op.
func Merge(ctx context.Context, pool *pgxpool.Pool, from, into uuid.UUID) error {
	if from == into || from == uuid.Nil || into == uuid.Nil {
		return nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := sqlc.New(tx)
	n := func(id uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: id, Valid: true} }
	steps := []struct {
		name string
		run  func() error
	}{
		{"connections", func() error {
			return q.MergeUserConnections(ctx, sqlc.MergeUserConnectionsParams{IntoID: into, FromID: from})
		}},
		{"identities", func() error {
			return q.MergeUserIdentities(ctx, sqlc.MergeUserIdentitiesParams{IntoID: into, FromID: from})
		}},
		{"repos", func() error {
			return q.MergeUserRepos(ctx, sqlc.MergeUserReposParams{IntoID: n(into), FromID: n(from)})
		}},
		{"launches", func() error {
			return q.MergeUserLaunches(ctx, sqlc.MergeUserLaunchesParams{IntoID: n(into), FromID: n(from)})
		}},
		{"crew", func() error { return q.MergeUserCrew(ctx, sqlc.MergeUserCrewParams{IntoID: n(into), FromID: n(from)}) }},
		{"user", func() error { return q.DeleteUser(ctx, from) }},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			return fmt.Errorf("merge users: %s: %w", s.name, err)
		}
	}
	return tx.Commit(ctx)
}
