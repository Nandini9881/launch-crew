// Package db opens the Postgres pool, runs embedded migrations and wraps transactions.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects a pool and verifies it with a ping.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrate applies all pending embedded migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sdb := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sdb.Close() }()
	return migrate(ctx, sdb)
}

func migrate(ctx context.Context, sdb *sql.DB) error {
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, sdb, "migrations"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// WithTx runs fn in a transaction, rolling back on error and committing otherwise.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *sqlc.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(sqlc.New(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
