// Package db opens the pgx connection pool and exposes its readiness check (D-020, D-050).
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/platform/health"
)

// KeyDatabaseURL is the configuration key holding the Postgres connection URL.
const KeyDatabaseURL = "DATABASE_URL"

// Open parses url, creates a pgx connection pool and pings the database.
//
// Pool settings come from the URL query, not from code: pgx reads
// pool_max_conns, pool_min_conns, pool_max_conn_lifetime and friends, and
// forwards other parameters (for example search_path) to the server as
// runtime parameters. If the ping fails the pool is closed and the error is
// returned wrapped. The caller owns the pool and must Close it.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Check returns a readiness check that pings the pool. Register it on the
// health registry behind /readyz (D-050).
func Check(pool *pgxpool.Pool) health.Check {
	return func(ctx context.Context) error {
		return pool.Ping(ctx)
	}
}
