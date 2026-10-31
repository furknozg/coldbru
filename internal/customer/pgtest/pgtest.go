//go:build integration

// Package pgtest starts a throwaway Postgres seeded exactly like the compose
// stack: the repo's infra/postgres/initdb scripts and the stub CSVs run on first
// start, so integration tests see the real customer_db (D-017). It exists only
// under the integration build tag.
package pgtest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/db"
)

// Image matches docker-compose.yml.
const Image = "postgres:18-alpine"

// customerPassword is the dev default of CUSTOMER_DB_PASSWORD in compose.
const customerPassword = "customer_svc_dev"

// Env is a running seeded Postgres and a pool connected as customer_svc to customer_db.
type Env struct {
	// Pool is the customer_svc pool, the same role the service uses.
	Pool *pgxpool.Pool
	ctr  *tcpostgres.PostgresContainer
}

// Start boots the container and waits until the seed is committed. Call Close when done.
func Start(ctx context.Context) (*Env, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	ctr, err := tcpostgres.Run(ctx, Image,
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.WithDatabase("postgres"),
		testcontainers.WithEnv(map[string]string{
			"CUSTOMER_DB_PASSWORD":     customerPassword,
			"PROPOSAL_DB_PASSWORD":     "proposal_svc_dev",
			"NOTIFICATION_DB_PASSWORD": "notification_svc_dev",
			"ANALYTICS_DB_PASSWORD":    "analytics_svc_dev",
			"REDIRECT_DB_PASSWORD":     "redirect_svc_dev",
			"TZ":                       "UTC",
			"PGTZ":                     "UTC",
		}),
		testcontainers.WithMounts(
			testcontainers.ContainerMount{
				Source:   testcontainers.GenericBindMountSource{HostPath: filepath.Join(root, "infra", "postgres", "initdb")},
				Target:   "/docker-entrypoint-initdb.d",
				ReadOnly: true,
			},
			testcontainers.ContainerMount{
				Source:   testcontainers.GenericBindMountSource{HostPath: filepath.Join(root, "stub")},
				Target:   "/seed/stub",
				ReadOnly: true,
			},
		),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		if ctr != nil {
			_ = testcontainers.TerminateContainer(ctr)
		}
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	env := &Env{ctr: ctr}

	host, err := ctr.Host(ctx)
	if err != nil {
		env.Close()
		return nil, fmt.Errorf("container host: %w", err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		env.Close()
		return nil, fmt.Errorf("container port: %w", err)
	}
	url := fmt.Sprintf("postgres://customer_svc:%s@%s/customer_db?sslmode=disable",
		customerPassword, net.JoinHostPort(host, port.Port()))

	// BasicWaitStrategies already waits for the post-init restart; polling the
	// seeded table makes it certain (same idea as the compose healthcheck).
	deadline := time.Now().Add(90 * time.Second)
	for {
		pool, err := db.Open(ctx, url)
		if err == nil {
			var n int
			err = pool.QueryRow(ctx, `SELECT count(*) FROM dwh.dim_customer`).Scan(&n)
			if err == nil && n > 0 {
				env.Pool = pool
				return env, nil
			}
			pool.Close()
			if err == nil {
				err = errors.New("dim_customer is empty")
			}
		}
		if time.Now().After(deadline) {
			env.Close()
			return nil, fmt.Errorf("wait for seed: %w", err)
		}
		select {
		case <-ctx.Done():
			env.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Close closes the pool and removes the container.
func (e *Env) Close() {
	if e.Pool != nil {
		e.Pool.Close()
	}
	if e.ctr != nil {
		_ = testcontainers.TerminateContainer(e.ctr)
	}
}

// repoRoot walks up from the working directory to the directory holding go.mod.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above the working directory")
		}
		dir = parent
	}
}
