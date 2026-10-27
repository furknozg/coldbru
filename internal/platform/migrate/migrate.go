// Package migrate applies embedded goose migrations at service start (D-020, D-051).
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Up applies every pending migration in migrations to the database behind pool.
//
// migrations is the directory that directly holds the numbered .sql files;
// callers with an embed.FS pass fs.Sub(embedFS, "migrations"). A Postgres
// session-level advisory lock serialises concurrent callers, so several
// service instances may start at once: one applies, the others wait and then
// find nothing to do.
//
// goose creates its version table in the connection's current schema, which is
// the first schema of the role's search_path (D-048); no schema is hard-coded.
// Each applied migration is logged with its version, source and duration.
func Up(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS, logger *slog.Logger) error {
	return apply(ctx, pool, migrations, logger)
}

// platformTable is the goose version table of the platform migration set. It
// differs from goose's default so the platform set and a service's own set
// keep separate histories in the same schema (D-054).
const platformTable = "goose_platform_version"

//go:embed platform/*.sql
var platformFS embed.FS

// UpPlatform applies the platform migrations embedded in this package: the
// outbox and processed_event tables (D-054). It behaves like Up, but tracks
// its history in the goose_platform_version table.
//
// Services call UpPlatform before Up. The tables land in the first schema of
// the role's search_path (D-048). Calling it again is a no-op.
func UpPlatform(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	sub, err := fs.Sub(platformFS, "platform")
	if err != nil {
		return fmt.Errorf("open embedded platform migrations: %w", err)
	}
	return apply(ctx, pool, sub, logger, goose.WithTableName(platformTable))
}

// apply runs every pending migration in migrations; extra provider options
// (for example a custom version table) are passed through to goose.
func apply(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS, logger *slog.Logger, opts ...goose.ProviderOption) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration locker: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer closeDB(logger, sqlDB)

	opts = append(opts, goose.WithSessionLocker(locker))
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations, opts...)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	for _, r := range results {
		logResult(ctx, logger, r)
	}
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func logResult(ctx context.Context, logger *slog.Logger, r *goose.MigrationResult) {
	attrs := []any{slog.Duration("duration", r.Duration)}
	if r.Source != nil {
		attrs = append(attrs,
			slog.Int64("version", r.Source.Version),
			slog.String("source", filepath.Base(r.Source.Path)),
		)
	}
	logger.InfoContext(ctx, "migration applied", attrs...)
}

// closeDB releases the database/sql wrapper. Closing a wrapper made by
// stdlib.OpenDBFromPool does not close the pool, so a failure here only
// matters as a log line.
func closeDB(logger *slog.Logger, sqlDB *sql.DB) {
	if err := sqlDB.Close(); err != nil {
		logger.Warn("close migration connection", slog.Any("error", err))
	}
}
