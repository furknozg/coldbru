//go:build integration

// Package rtest starts a throwaway Postgres for the redirect service's integration tests and hands out one
// empty database per test, with the redirect schema and a pool whose search_path is
// "redirect,public" like the service role's (D-048). It exists only under the integration build
// tag.
package rtest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/redirect/store"
)

// Schema is the schema initdb gives the redirect role (D-048).
const Schema = "redirect"

// Postgres is a running container.
type Postgres struct {
	// DSN points at the maintenance database; BaseURL derives the others from it.
	DSN   string
	admin *pgxpool.Pool
	ctr   *tcpostgres.PostgresContainer

	mu  sync.Mutex
	seq int
}

// Start boots postgres:18-alpine, the image of docker-compose.yml. Call Close when done.
func Start(ctx context.Context) (*Postgres, error) {
	ctr, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.WithDatabase("postgres"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		if ctr != nil {
			_ = testcontainers.TerminateContainer(ctr)
		}
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	p := &Postgres{ctr: ctr}
	if p.DSN, err = ctr.ConnectionString(ctx, "sslmode=disable"); err != nil {
		p.Close()
		return nil, fmt.Errorf("connection string: %w", err)
	}
	if p.admin, err = db.Open(ctx, p.DSN); err != nil {
		p.Close()
		return nil, fmt.Errorf("open admin pool: %w", err)
	}
	return p, nil
}

// Close stops the container.
func (p *Postgres) Close() {
	if p.admin != nil {
		p.admin.Close()
	}
	if p.ctr != nil {
		_ = testcontainers.TerminateContainer(p.ctr)
	}
}

// Discard is a logger that drops everything.
func Discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// NewPool creates an empty database with an redirect schema and returns a pool on it. Migrations
// are not applied.
func (p *Postgres) NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p.mu.Lock()
	p.seq++
	name := fmt.Sprintf("redirect_test_%d", p.seq)
	p.mu.Unlock()
	if _, err := p.admin.Exec(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	u, err := url.Parse(p.DSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.Path = "/" + name
	setup, err := db.Open(t.Context(), u.String())
	if err != nil {
		t.Fatalf("open setup pool: %v", err)
	}
	if _, err := setup.Exec(t.Context(), "CREATE SCHEMA "+Schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	setup.Close()
	q := u.Query()
	q.Set("search_path", Schema+",public")
	u.RawQuery = q.Encode()
	pool, err := db.Open(t.Context(), u.String())
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// NewMigratedPool is NewPool plus the platform and redirect migration sets, in startup order.
func (p *Postgres) NewMigratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := p.NewPool(t)
	if err := migrate.UpPlatform(t.Context(), pool, Discard()); err != nil {
		t.Fatalf("UpPlatform() error = %v", err)
	}
	if err := migrate.Up(t.Context(), pool, store.Migrations(), Discard()); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	return pool
}
