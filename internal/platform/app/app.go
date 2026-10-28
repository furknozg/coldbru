// Package app is the shared service skeleton (D-056): configuration, logging,
// health, metrics, optional Postgres and NATS, and graceful shutdown. Each
// cmd/<service>/main.go only declares which dependencies it needs.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/apidocs"
	"github.com/furknozg/coldbru/internal/platform/config"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/health"
	"github.com/furknozg/coldbru/internal/platform/httpserver"
	"github.com/furknozg/coldbru/internal/platform/logging"
	"github.com/furknozg/coldbru/internal/platform/metrics"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/platform/natsconn"
)

// Options declares what a service needs from the platform.
type Options struct {
	// Name identifies the service in logs and the NATS connection.
	Name string
	// Database opens the Postgres pool from DATABASE_URL and registers a
	// "postgres" readiness check.
	Database bool
	// PlatformMigrations applies the platform migration set (outbox,
	// processed_event) at startup. It requires Database.
	PlatformMigrations bool
	// Migrations is the service's own goose migration set: the directory that directly
	// holds the numbered .sql files (callers with an embed.FS pass fs.Sub). It is applied
	// at startup after the platform set and before Setup, with goose's default version
	// table; an error aborts startup. A nil Migrations applies nothing. It requires Database.
	Migrations fs.FS
	// NATS connects to NATS_URL and registers a "nats" readiness check.
	NATS bool
	// OpenAPISpec is the service's embedded OpenAPI JSON. With API_DOCS_ENABLED
	// true and a non-empty spec, GET /docs (Scalar) and GET /openapi.json are
	// mounted; otherwise neither is (D-057).
	OpenAPISpec []byte
	// Setup, if set, runs after the dependencies are ready and before the HTTP
	// server starts. Services mount routes and start workers here; workers must
	// stop when ctx is cancelled. A Setup error aborts startup.
	Setup func(ctx context.Context, d *Deps) error
}

// Deps is what Setup receives. Pool is nil unless Options.Database is set;
// NATS and JetStream are nil unless Options.NATS is set.
type Deps struct {
	Router    chi.Router
	Config    *viper.Viper
	Logger    *slog.Logger
	Metrics   *metrics.Metrics
	Health    *health.Registry
	Pool      *pgxpool.Pool
	NATS      *nats.Conn
	JetStream jetstream.JetStream

	shutdown []func(ctx context.Context) error
}

// OnShutdown registers fn to run when the service stops: after the HTTP server has shut down, and
// before NATS is drained and the database pool is closed, so that a worker still using them can
// finish. Functions run most recently registered first, each with the shutdown timeout as its
// deadline; an error is logged and does not stop the others. Call it from Setup.
func (d *Deps) OnShutdown(fn func(ctx context.Context) error) {
	d.shutdown = append(d.shutdown, fn)
}

// runShutdown runs the OnShutdown functions in reverse order of registration.
func (d *Deps) runShutdown(timeout time.Duration) {
	for i := len(d.shutdown) - 1; i >= 0; i-- {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		if err := d.shutdown[i](ctx); err != nil {
			d.Logger.Warn("shutdown hook failed", "error", err)
		}
		cancel()
	}
}

// Main is the whole body of a service's main function. It never returns.
func Main(opts Options) {
	healthcheck := flag.Bool("healthcheck", false, "probe /readyz on this service and exit 0 (ready) or 1")
	flag.Parse()

	if *healthcheck {
		addr := os.Getenv(config.KeyHTTPAddr)
		if addr == "" {
			addr = config.PlatformDefaults()[config.KeyHTTPAddr].(string)
		}
		if err := Healthcheck(addr); err != nil {
			fmt.Fprintln(os.Stderr, "healthcheck:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	os.Exit(runMain(opts))
}

// runMain returns the process exit code.
func runMain(opts Options) int {
	v, err := config.New(config.PlatformDefaults())
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}
	logger, err := logging.New(os.Stdout, v.GetString(config.KeyLogLevel))
	if err != nil {
		fmt.Fprintln(os.Stderr, "logging:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := Run(ctx, opts, v, logger); err != nil {
		logger.Error("service failed", "service", opts.Name, "error", err)
		return 1
	}
	return 0
}

// Run builds the service and blocks until ctx is cancelled or the server fails.
// A clean shutdown returns nil.
func Run(ctx context.Context, opts Options, v *viper.Viper, logger *slog.Logger) error {
	if opts.PlatformMigrations && !opts.Database {
		return errors.New("platform migrations require a database")
	}
	if opts.Migrations != nil && !opts.Database {
		return errors.New("service migrations require a database")
	}

	reg := health.NewRegistry()
	m := metrics.New()
	d := &Deps{
		Router:  httpserver.NewRouter(logger, m, reg),
		Config:  v,
		Logger:  logger,
		Metrics: m,
		Health:  reg,
	}

	logger.Info("service starting",
		"service", opts.Name,
		"database", opts.Database,
		"platform_migrations", opts.PlatformMigrations,
		"service_migrations", opts.Migrations != nil,
		"nats", opts.NATS,
	)

	if opts.Database {
		if err := config.Require(v, db.KeyDatabaseURL); err != nil {
			return err
		}
		pool, err := db.Open(ctx, v.GetString(db.KeyDatabaseURL))
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer pool.Close()
		d.Pool = pool
		reg.Add("postgres", db.Check(pool))

		if opts.PlatformMigrations {
			if err := migrate.UpPlatform(ctx, pool, logger); err != nil {
				return fmt.Errorf("platform migrations: %w", err)
			}
		}
		if opts.Migrations != nil {
			if err := migrate.Up(ctx, pool, opts.Migrations, logger); err != nil {
				return fmt.Errorf("service migrations: %w", err)
			}
		}
	}

	shutdownTimeout := v.GetDuration(config.KeyShutdownTimeout)

	if opts.NATS {
		if err := config.Require(v, natsconn.KeyNATSURL); err != nil {
			return err
		}
		nc, js, err := natsconn.Connect(v.GetString(natsconn.KeyNATSURL), opts.Name, logger)
		if err != nil {
			return err
		}
		defer drain(nc, shutdownTimeout, logger)
		d.NATS, d.JetStream = nc, js
		reg.Add("nats", natsconn.Check(nc))
	}

	if err := mountAPIDocs(d.Router, v, opts, logger); err != nil {
		return err
	}

	// Deferred after the database and NATS cleanups, so it runs before them.
	defer d.runShutdown(shutdownTimeout)
	if opts.Setup != nil {
		if err := opts.Setup(ctx, d); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
	}

	srv := httpserver.NewServer(v.GetString(config.KeyHTTPAddr), d.Router)
	return httpserver.Run(ctx, srv, shutdownTimeout, logger)
}

// mountAPIDocs mounts /docs and /openapi.json when API_DOCS_ENABLED is true and
// the service embeds a spec, and logs whether API docs are enabled (D-057).
func mountAPIDocs(r chi.Router, v *viper.Viper, opts Options, logger *slog.Logger) error {
	enabled := v.GetBool(config.KeyAPIDocsEnabled) && len(opts.OpenAPISpec) > 0
	if enabled {
		if err := apidocs.Mount(r, opts.OpenAPISpec, "Coldbru "+opts.Name+" API"); err != nil {
			return fmt.Errorf("mount api docs: %w", err)
		}
	}
	logger.Info("api docs", "service", opts.Name, "enabled", enabled)
	return nil
}

// drain flushes pending NATS messages and closes the connection, waiting at
// most timeout for the drain to finish.
func drain(nc *nats.Conn, timeout time.Duration, logger *slog.Logger) {
	if err := nc.Drain(); err != nil {
		logger.Warn("nats drain", "error", err)
		nc.Close()
		return
	}
	deadline := time.Now().Add(timeout)
	for !nc.IsClosed() {
		if time.Now().After(deadline) {
			logger.Warn("nats drain timed out")
			nc.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

const healthcheckTimeout = 3 * time.Second

// Healthcheck GETs /readyz on this host at the port of addr (a HTTP_ADDR value
// such as ":8080") and returns nil only on a 200 response.
func Healthcheck(addr string) error {
	port, err := portOf(addr)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", port)+"/readyz", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("probe readyz: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz returned %d", resp.StatusCode)
	}
	return nil
}

// portOf extracts the port from a listen address.
func portOf(addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse HTTP_ADDR %q: %w", addr, err)
	}
	if port == "" {
		return "", fmt.Errorf("parse HTTP_ADDR %q: no port", addr)
	}
	return port, nil
}
