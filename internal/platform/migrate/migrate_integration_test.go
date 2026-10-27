//go:build integration

package migrate_test

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/migrate"
)

const adminDB = "postgres"

var (
	// baseDSN points at the container's default database.
	baseDSN string
	// adminPool is used by tests only to create databases and schemas.
	adminPool *pgxpool.Pool
	dbSeq     int
	dbSeqMu   sync.Mutex
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctr, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.WithDatabase(adminDB), tcpostgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			fmt.Fprintf(os.Stderr, "terminate postgres container: %v\n", err)
		}
	}()

	baseDSN, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "container connection string: %v\n", err)
		return 1
	}
	adminPool, err = db.Open(ctx, baseDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open admin pool: %v\n", err)
		return 1
	}
	defer adminPool.Close()

	return m.Run()
}

// newDatabase creates an empty database and returns its DSN.
func newDatabase(t *testing.T) string {
	t.Helper()
	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("migrate_test_%d", dbSeq)
	dbSeqMu.Unlock()

	if _, err := adminPool.Exec(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	return withParams(t, baseDSN, "/"+name, nil)
}

// withParams returns dsn pointing at database path with extra query parameters
// (for example search_path=svc) applied.
func withParams(t *testing.T, dsn, path string, params map[string]string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if path != "" {
		u.Path = path
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func openPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := db.Open(t.Context(), dsn)
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testMigrations(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(os.DirFS("testdata"), "migrations")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	return sub
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// appliedVersions returns the versions goose recorded, resolved through the
// pool's own search_path.
func appliedVersions(t *testing.T, pool *pgxpool.Pool) []int64 {
	t.Helper()
	rows, err := pool.Query(t.Context(), "SELECT version_id FROM goose_db_version WHERE version_id > 0 ORDER BY version_id")
	if err != nil {
		t.Fatalf("query goose_db_version: %v", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func TestUp_AppliesMigrations(t *testing.T) {
	ctx := t.Context()
	pool := openPool(t, newDatabase(t))

	if err := migrate.Up(ctx, pool, testMigrations(t), discardLogger()); err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	if got, want := fmt.Sprint(appliedVersions(t, pool)), "[1 2]"; got != want {
		t.Errorf("applied versions = %s, want %s", got, want)
	}

	var id string
	var hasCreatedAt bool
	err := pool.QueryRow(ctx, "INSERT INTO probe (note) VALUES ('hi') RETURNING id::text, created_at IS NOT NULL").Scan(&id, &hasCreatedAt)
	if err != nil {
		t.Fatalf("insert into probe: %v", err)
	}
	if len(id) != 36 || id[14] != '7' {
		t.Errorf("id = %q, want a UUID version 7", id)
	}
	if !hasCreatedAt {
		t.Error("created_at is null, want default now()")
	}
}

func TestUp_Idempotent(t *testing.T) {
	ctx := t.Context()
	pool := openPool(t, newDatabase(t))
	migrations := testMigrations(t)

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	if err := migrate.Up(ctx, pool, migrations, logger); err != nil {
		t.Fatalf("first Up() error = %v", err)
	}
	if got := strings.Count(buf.String(), "migration applied"); got != 2 {
		t.Fatalf("first Up() logged %d applied migrations, want 2", got)
	}

	buf.Reset()
	if err := migrate.Up(ctx, pool, migrations, logger); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("second Up() logged %q, want nothing applied", buf.String())
	}
	if got, want := fmt.Sprint(appliedVersions(t, pool)), "[1 2]"; got != want {
		t.Errorf("applied versions = %s, want %s", got, want)
	}
}

func TestUp_UsesSearchPathSchema(t *testing.T) {
	ctx := t.Context()
	dsn := newDatabase(t)

	// The schema must exist before the role-scoped pool connects (D-048: initdb creates it).
	setup := openPool(t, dsn)
	if _, err := setup.Exec(ctx, "CREATE SCHEMA svc"); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	pool := openPool(t, withParams(t, dsn, "", map[string]string{"search_path": "svc,public"}))

	if err := migrate.Up(ctx, pool, testMigrations(t), discardLogger()); err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	tests := []struct {
		table  string
		schema string
		want   bool
	}{
		{table: "goose_db_version", schema: "svc", want: true},
		{table: "probe", schema: "svc", want: true},
		{table: "goose_db_version", schema: "public", want: false},
		{table: "probe", schema: "public", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.schema+"."+tt.table, func(t *testing.T) {
			var exists bool
			err := pool.QueryRow(ctx,
				"SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = $1 AND tablename = $2)",
				tt.schema, tt.table).Scan(&exists)
			if err != nil {
				t.Fatalf("query pg_tables: %v", err)
			}
			if exists != tt.want {
				t.Errorf("table %s.%s exists = %v, want %v", tt.schema, tt.table, exists, tt.want)
			}
		})
	}
}

func TestUp_ConcurrentCallers(t *testing.T) {
	ctx := t.Context()
	dsn := newDatabase(t)
	migrations := testMigrations(t)

	const callers = 2
	pools := make([]*pgxpool.Pool, callers)
	for i := range pools {
		pools[i] = openPool(t, dsn)
	}

	var buf syncBuilder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	errs := make([]error, callers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range pools {
		wg.Go(func() {
			<-start
			errs[i] = migrate.Up(ctx, pools[i], migrations, logger)
		})
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d: Up() error = %v", i, err)
		}
	}
	if got := strings.Count(buf.String(), "migration applied"); got != 2 {
		t.Errorf("applied migrations logged = %d, want 2 (each exactly once)", got)
	}
	if got, want := fmt.Sprint(appliedVersions(t, pools[0])), "[1 2]"; got != want {
		t.Errorf("applied versions = %s, want %s", got, want)
	}
}

// syncBuilder is a strings.Builder safe for concurrent slog writes.
type syncBuilder struct {
	mu sync.Mutex
	sb strings.Builder
}

func (s *syncBuilder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.Write(p)
}

func (s *syncBuilder) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.String()
}

func TestUpPlatform_ThenUp_UsesSearchPathSchema(t *testing.T) {
	ctx := t.Context()
	dsn := newDatabase(t)

	setup := openPool(t, dsn)
	if _, err := setup.Exec(ctx, "CREATE SCHEMA svc"); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	pool := openPool(t, withParams(t, dsn, "", map[string]string{"search_path": "svc,public"}))

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	if err := migrate.UpPlatform(ctx, pool, logger); err != nil {
		t.Fatalf("UpPlatform() error = %v", err)
	}
	if got := strings.Count(buf.String(), "migration applied"); got != 2 {
		t.Fatalf("UpPlatform() logged %d applied migrations, want 2", got)
	}
	if err := migrate.Up(ctx, pool, testMigrations(t), logger); err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	tests := []struct {
		table  string
		schema string
		want   bool
	}{
		{table: "goose_platform_version", schema: "svc", want: true},
		{table: "goose_db_version", schema: "svc", want: true},
		{table: "outbox", schema: "svc", want: true},
		{table: "processed_event", schema: "svc", want: true},
		{table: "probe", schema: "svc", want: true},
		{table: "goose_platform_version", schema: "public", want: false},
		{table: "outbox", schema: "public", want: false},
		{table: "processed_event", schema: "public", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.schema+"."+tt.table, func(t *testing.T) {
			var exists bool
			err := pool.QueryRow(ctx,
				"SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = $1 AND tablename = $2)",
				tt.schema, tt.table).Scan(&exists)
			if err != nil {
				t.Fatalf("query pg_tables: %v", err)
			}
			if exists != tt.want {
				t.Errorf("table %s.%s exists = %v, want %v", tt.schema, tt.table, exists, tt.want)
			}
		})
	}

	t.Run("separate version histories", func(t *testing.T) {
		if got, want := fmt.Sprint(appliedVersions(t, pool)), "[1 2]"; got != want {
			t.Errorf("goose_db_version = %s, want %s", got, want)
		}
		var n int
		err := pool.QueryRow(ctx, "SELECT count(*) FROM goose_platform_version WHERE version_id > 0").Scan(&n)
		if err != nil {
			t.Fatalf("query goose_platform_version: %v", err)
		}
		if n != 2 {
			t.Errorf("goose_platform_version rows = %d, want 2", n)
		}
	})

	t.Run("second UpPlatform is a no-op", func(t *testing.T) {
		buf.Reset()
		if err := migrate.UpPlatform(ctx, pool, logger); err != nil {
			t.Fatalf("second UpPlatform() error = %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("second UpPlatform() logged %q, want nothing applied", buf.String())
		}
	})

	t.Run("outbox schema", func(t *testing.T) {
		var id string
		err := pool.QueryRow(ctx,
			`INSERT INTO outbox (subject, payload) VALUES ('evt.a.b.v1', '{}') RETURNING id::text`).Scan(&id)
		if err != nil {
			t.Fatalf("insert outbox: %v", err)
		}
		if len(id) != 36 || id[14] != '7' {
			t.Errorf("id = %q, want a UUID version 7", id)
		}
		var idx bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'svc' AND indexname = 'outbox_unpublished_idx')").Scan(&idx)
		if err != nil || !idx {
			t.Errorf("outbox_unpublished_idx exists = %v, err = %v", idx, err)
		}
	})
}
