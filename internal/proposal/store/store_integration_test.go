//go:build integration

package store_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

const (
	adminDB = "postgres"
	// serviceSchema is the schema initdb gives the proposal role (D-048).
	serviceSchema = "proposal"
	// raiseException is the SQLSTATE of RAISE EXCEPTION without a USING ERRCODE.
	raiseException = "P0001"
	// pgNotNullViolation is the SQLSTATE of a NOT NULL violation, which names no constraint.
	pgNotNullViolation = "23502"
)

var (
	baseDSN   string
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

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newPool creates an empty database with a proposal schema, opens a pool whose search_path is
// "proposal,public" like the service role's (D-048), and returns it. Migrations are not applied.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("store_test_%d", dbSeq)
	dbSeqMu.Unlock()

	if _, err := adminPool.Exec(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	u, err := url.Parse(baseDSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.Path = "/" + name

	setup, err := db.Open(t.Context(), u.String())
	if err != nil {
		t.Fatalf("open setup pool: %v", err)
	}
	if _, err := setup.Exec(t.Context(), "CREATE SCHEMA "+serviceSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	setup.Close()

	q := u.Query()
	q.Set("search_path", serviceSchema+",public")
	u.RawQuery = q.Encode()
	pool, err := db.Open(t.Context(), u.String())
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// newMigratedPool is newPool plus the platform and proposal migration sets, in startup order.
func newMigratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := newPool(t)
	if err := migrate.UpPlatform(t.Context(), pool, discardLogger()); err != nil {
		t.Fatalf("UpPlatform() error = %v", err)
	}
	if err := migrate.Up(t.Context(), pool, store.Migrations(), discardLogger()); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	return pool
}

func queryStrings(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(), sql, args...)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func TestMigrations_Applied(t *testing.T) {
	pool := newMigratedPool(t)

	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "tables in schema proposal",
			sql:  "SELECT tablename FROM pg_tables WHERE schemaname = 'proposal' ORDER BY tablename",
			want: []string{
				"campaign", "context_note", "goose_db_version", "goose_platform_version", "llm_call",
				"outbox", "processed_event", "proposal_step", "proposal_variant",
			},
		},
		{
			name: "nothing in public",
			sql:  "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename",
			want: nil,
		},
		{
			name: "service versions",
			sql:  "SELECT version_id::text FROM goose_db_version WHERE version_id > 0 ORDER BY version_id",
			want: []string{"1", "2", "3", "4"},
		},
		{
			name: "platform versions",
			sql:  "SELECT version_id::text FROM goose_platform_version WHERE version_id > 0 ORDER BY version_id",
			want: []string{"1", "2"},
		},
		{
			name: "indexes",
			sql: `SELECT indexname FROM pg_indexes WHERE schemaname = 'proposal'
				AND indexname IN ('proposal_step_live_kind_key', 'proposal_variant_live_slot_key', 'proposal_variant_step_id_idx',
					'context_note_campaign_id_idx', 'context_note_step_id_idx',
					'llm_call_campaign_id_idx', 'llm_call_step_id_idx', 'llm_call_variant_id_idx') ORDER BY indexname`,
			want: []string{
				"context_note_campaign_id_idx", "context_note_step_id_idx",
				"llm_call_campaign_id_idx", "llm_call_step_id_idx", "llm_call_variant_id_idx",
				"proposal_step_live_kind_key", "proposal_variant_live_slot_key", "proposal_variant_step_id_idx",
			},
		},
		{
			name: "append-only triggers",
			sql:  "SELECT tgname FROM pg_trigger WHERE NOT tgisinternal AND tgname LIKE '%append_only' ORDER BY tgname",
			want: []string{"context_note_append_only", "llm_call_append_only"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queryStrings(t, pool, tt.sql)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("second Up applies nothing", func(t *testing.T) {
		var buf strings.Builder
		if err := migrate.Up(t.Context(), pool, store.Migrations(), slog.New(slog.NewTextHandler(&buf, nil))); err != nil {
			t.Fatalf("Up() error = %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("second Up() logged %q, want nothing applied", buf.String())
		}
	})
}

func TestMigrations_DownThenUp(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, store.Migrations())
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("DownTo(0) error = %v", err)
	}

	left := queryStrings(t, pool, "SELECT tablename FROM pg_tables WHERE schemaname = 'proposal' ORDER BY tablename")
	want := []string{"goose_db_version", "goose_platform_version", "outbox", "processed_event"}
	if !slices.Equal(left, want) {
		t.Errorf("tables after Down = %v, want %v (service tables gone, platform set untouched)", left, want)
	}
	if got := queryStrings(t, pool, "SELECT proname FROM pg_proc WHERE proname = 'reject_row_change'"); len(got) != 0 {
		t.Errorf("functions after Down = %v, want none", got)
	}
	if got := queryStrings(t, pool, "SELECT indexname FROM pg_indexes WHERE schemaname = 'proposal' AND indexname LIKE ANY (ARRAY['proposal\\_%', 'context\\_note\\_%', 'llm\\_call\\_%'])"); len(got) != 0 {
		t.Errorf("indexes after Down = %v, want none", got)
	}

	if err := migrate.Up(ctx, pool, store.Migrations(), discardLogger()); err != nil {
		t.Fatalf("Up() after Down error = %v", err)
	}
	if _, err := store.New(pool).CreateCampaign(ctx, "brief after re-up"); err != nil {
		t.Errorf("CreateCampaign() after re-up error = %v", err)
	}
}

// fixture is one campaign with an open audience, content and schedule step.
type fixture struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	campaign uuid.UUID
	audience uuid.UUID
	content  uuid.UUID
	schedule uuid.UUID
}

func newFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := t.Context()
	q := store.New(pool)
	c, err := q.CreateCampaign(ctx, "Remind cart abandoners in Istanbul, 30% off")
	if err != nil {
		t.Fatalf("CreateCampaign() error = %v", err)
	}
	f := fixture{pool: pool, q: q, campaign: c.ID}
	for kind, dst := range map[string]*uuid.UUID{"audience": &f.audience, "content": &f.content, "schedule": &f.schedule} {
		s, err := q.CreateStep(ctx, store.CreateStepParams{CampaignID: c.ID, Kind: kind, Version: 1})
		if err != nil {
			t.Fatalf("CreateStep(%s) error = %v", kind, err)
		}
		*dst = s.ID
	}
	return f
}

func (f fixture) exec(ctx context.Context, sql string, args ...any) error {
	_, err := f.pool.Exec(ctx, sql, args...)
	return err
}

// step inserts a step row with explicit status and approved_at (nil for NULL).
func (f fixture) step(ctx context.Context, kind string, version int, status string, approvedAt any) error {
	return f.exec(ctx,
		"INSERT INTO proposal_step (campaign_id, kind, version, status, approved_at) VALUES ($1, $2, $3, $4, $5)",
		f.campaign, kind, version, status, approvedAt)
}

// variantRow is a proposal_variant insert; nil fields are NULL.
type variantRow struct {
	step     uuid.UUID
	kind     string
	language any
	channel  any
	timezone any
	status   string
	reason   any
	approved any
	rejected any
}

func (f fixture) variant(ctx context.Context, v variantRow) error {
	return f.exec(ctx, `INSERT INTO proposal_variant
		(step_id, kind, language, channel, timezone, status, content, rejection_reason, approved_at, rejected_at)
		VALUES ($1, $2, $3, $4, $5, $6, '{}'::jsonb, $7, $8, $9)`,
		v.step, v.kind, v.language, v.channel, v.timezone, v.status, v.reason, v.approved, v.rejected)
}

// wantConstraint asserts that err is nil when want is empty, otherwise a Postgres error that
// names the constraint (or unique index) want.
func wantConstraint(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Errorf("error = %v, want accepted", err)
		}
		return
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Errorf("error = %v, want a Postgres error for constraint %q", err, want)
		return
	}
	if pgErr.ConstraintName != want {
		t.Errorf("constraint = %q (%s), want %q", pgErr.ConstraintName, pgErr.Message, want)
	}
}

type constraintCase struct {
	name string
	run  func(t *testing.T, ctx context.Context, f fixture) error
	// want is the violated constraint or unique index; empty means the row is accepted
	// unless wantCode is set.
	want string
	// wantCode is the SQLSTATE expected when the violation carries no constraint name
	// (NOT NULL, trigger exceptions).
	wantCode string
}

func runConstraintCases(t *testing.T, tests []constraintCase) {
	t.Helper()
	pool := newMigratedPool(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, pool)
			err := tt.run(t, t.Context(), f)
			if tt.wantCode != "" {
				assertPgCode(t, err, tt.wantCode)
				return
			}
			wantConstraint(t, err, tt.want)
		})
	}
}

func mustExec(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup statement error = %v", err)
	}
}

func TestConstraints_Campaign(t *testing.T) {
	insert := func(brief, status string) func(*testing.T, context.Context, fixture) error {
		return func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO campaign (brief, status) VALUES ($1, $2)", brief, status)
		}
	}
	runConstraintCases(t, []constraintCase{
		{name: "valid", run: insert("a brief", "drafting")},
		{name: "every status", run: func(t *testing.T, ctx context.Context, f fixture) error {
			for _, s := range []string{"drafting", "scheduled", "sending", "completed", "cancelled"} {
				mustExec(t, f.exec(ctx, "INSERT INTO campaign (brief, status) VALUES ('b', $1)", s))
			}
			return nil
		}},
		{name: "empty brief", run: insert("", "drafting"), want: "campaign_brief_check"},
		{name: "blank brief", run: insert("  \t", "drafting"), want: "campaign_brief_check"},
		{name: "unknown status", run: insert("b", "paused"), want: "campaign_status_check"},
		{name: "default status is drafting", run: func(t *testing.T, ctx context.Context, f fixture) error {
			c, err := f.q.GetCampaign(ctx, f.campaign)
			if err != nil {
				return err
			}
			if c.Status != "drafting" {
				t.Errorf("default status = %q, want drafting", c.Status)
			}
			return nil
		}},
	})
}

func TestConstraints_ProposalStep(t *testing.T) {
	now := time.Now()
	runConstraintCases(t, []constraintCase{
		{name: "valid open", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 1, "open", nil)
		}},
		{name: "valid approved", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.exec(ctx, "UPDATE proposal_step SET status = 'approved', approved_at = now() WHERE id = $1", f.audience))
			return f.step(ctx, "channel_plan", 1, "approved", now)
		}},
		{name: "unknown kind", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "review", 1, "open", nil)
		}, want: "proposal_step_kind_check"},
		{name: "version zero", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 0, "open", nil)
		}, want: "proposal_step_version_check"},
		{name: "unknown status", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 1, "done", nil)
		}, want: "proposal_step_status_check"},
		{name: "approved without approved_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 1, "approved", nil)
		}, want: "proposal_step_approved_at_set_check"},
		{name: "open with approved_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 1, "open", now)
		}, want: "proposal_step_open_approved_at_check"},
		{name: "superseded without approved_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 1, "superseded", nil)
		}},
		{name: "superseded keeps approved_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 1, "superseded", now)
		}},
		{name: "unknown campaign", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO proposal_step (campaign_id, kind, version) VALUES ($1, 'audience', 1)", uuid.New())
		}, want: "proposal_step_campaign_id_fkey"},
		{name: "same version twice", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.exec(ctx, "UPDATE proposal_step SET status = 'superseded' WHERE id = $1", f.audience))
			return f.step(ctx, "audience", 1, "open", nil)
		}, want: "proposal_step_campaign_kind_version_key"},
		{name: "second open step of a kind", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "audience", 2, "open", nil)
		}, want: "proposal_step_live_kind_key"},
		{name: "approved step still blocks a second live step", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.exec(ctx, "UPDATE proposal_step SET status = 'approved', approved_at = now() WHERE id = $1", f.audience))
			return f.step(ctx, "audience", 2, "open", nil)
		}, want: "proposal_step_live_kind_key"},
		{name: "superseded allows a new open step", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.exec(ctx, "UPDATE proposal_step SET status = 'superseded' WHERE id = $1", f.audience))
			return f.step(ctx, "audience", 2, "open", nil)
		}},
		{name: "another kind is independent", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.step(ctx, "channel_plan", 1, "open", nil)
		}},
		{name: "another campaign is independent", run: func(t *testing.T, ctx context.Context, f fixture) error {
			c, err := f.q.CreateCampaign(ctx, "other brief")
			mustExec(t, err)
			return f.exec(ctx, "INSERT INTO proposal_step (campaign_id, kind, version) VALUES ($1, 'audience', 1)", c.ID)
		}},
	})
}

func TestConstraints_ProposalVariant(t *testing.T) {
	now := time.Now()
	runConstraintCases(t, []constraintCase{
		// Shape per step kind (D-034).
		{name: "audience without language and channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "draft"})
		}},
		{name: "audience with language", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", language: "tr", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "audience with channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", channel: "sms", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "channel_plan with language", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.step(ctx, "channel_plan", 1, "open", nil))
			var id uuid.UUID
			mustExec(t, f.pool.QueryRow(ctx, "SELECT id FROM proposal_step WHERE campaign_id = $1 AND kind = 'channel_plan'", f.campaign).Scan(&id))
			return f.variant(ctx, variantRow{step: id, kind: "channel_plan", language: "en", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "content with language and channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"})
		}},
		{name: "content without channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "content without language", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.content, kind: "content", channel: "email", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "schedule with language and timezone (D-076)", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "en", timezone: "Europe/London", status: "draft"})
		}},
		{name: "schedule with language only", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "en", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "schedule with channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "en", channel: "sms", timezone: "Europe/London", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "schedule without language", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", timezone: "Europe/London", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "schedule with a blank timezone", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "en", timezone: " ", status: "draft"})
		}, want: "proposal_variant_timezone_check"},
		{name: "audience with a timezone", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", timezone: "Europe/London", status: "draft"})
		}, want: "proposal_variant_shape_check"},
		{name: "content with a timezone", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", timezone: "Europe/Istanbul", status: "draft"})
		}, want: "proposal_variant_shape_check"},

		// Value lists.
		{name: "unknown kind", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "review", status: "draft"})
		}, want: "proposal_variant_kind_check"},
		{name: "unknown language", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "de", channel: "sms", status: "draft"})
		}, want: "proposal_variant_language_check"},
		{name: "unknown channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "push", status: "draft"})
		}, want: "proposal_variant_channel_check"},
		{name: "unknown status", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "open"})
		}, want: "proposal_variant_status_check"},

		// Composite foreign key: the variant's kind must equal its step's kind.
		{name: "kind differs from the step's kind", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "content", language: "tr", channel: "sms", status: "draft"})
		}, want: "proposal_variant_step_kind_fkey"},
		{name: "unknown step", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: uuid.New(), kind: "audience", status: "draft"})
		}, want: "proposal_variant_step_kind_fkey"},

		// Status stamps.
		{name: "approved with approved_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "approved", approved: now})
		}},
		{name: "approved without approved_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "approved"})
		}, want: "proposal_variant_approved_at_check"},
		{name: "draft with approved_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "draft", approved: now})
		}, want: "proposal_variant_draft_check"},
		{name: "draft with rejected_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "draft", rejected: now})
		}, want: "proposal_variant_draft_check"},
		{name: "draft with rejection_reason", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "draft", reason: "too long"})
		}, want: "proposal_variant_draft_check"},
		{name: "rejected with reason and rejected_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "rejected", reason: "too long", rejected: now})
		}},
		{name: "rejected without reason", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "rejected", rejected: now})
		}, want: "proposal_variant_rejected_check"},
		{name: "rejected with empty reason", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "rejected", reason: " ", rejected: now})
		}, want: "proposal_variant_rejected_check"},
		{name: "rejected without rejected_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "rejected", reason: "too long"})
		}, want: "proposal_variant_rejected_check"},
		{name: "superseded keeps earlier stamps", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "superseded", reason: "old", approved: now, rejected: now})
		}},
		{name: "superseded without stamps", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.variant(ctx, variantRow{step: f.audience, kind: "audience", status: "superseded"})
		}},
		{name: "content is required", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO proposal_variant (step_id, kind, content) VALUES ($1, 'audience', NULL)", f.audience)
		}, wantCode: pgNotNullViolation},
	})
}

func TestConstraints_ProposalVariantSlots(t *testing.T) {
	now := time.Now()
	runConstraintCases(t, []constraintCase{
		// Partial unique: one draft or approved variant per (language, channel) slot.
		{name: "second draft in the same slot", run: func(t *testing.T, ctx context.Context, f fixture) error {
			row := variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"}
			mustExec(t, f.variant(ctx, row))
			return f.variant(ctx, row)
		}, want: "proposal_variant_live_slot_key"},
		{name: "second audience draft (NULLS NOT DISTINCT)", run: func(t *testing.T, ctx context.Context, f fixture) error {
			row := variantRow{step: f.audience, kind: "audience", status: "draft"}
			mustExec(t, f.variant(ctx, row))
			return f.variant(ctx, row)
		}, want: "proposal_variant_live_slot_key"},
		{name: "draft next to an approved variant in the same slot", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "approved", approved: now}))
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"})
		}, want: "proposal_variant_live_slot_key"},
		{name: "new draft after a rejected one", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "rejected", reason: "too long", rejected: now}))
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"})
		}},
		{name: "new draft after a superseded one", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "superseded"}))
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"})
		}},
		{name: "different channel, same language", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"}))
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "email", status: "draft"})
		}},
		{name: "different language, same channel", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"}))
			return f.variant(ctx, variantRow{step: f.content, kind: "content", language: "en", channel: "sms", status: "draft"})
		}},
		// A schedule slot is (language, timezone) (D-076).
		{name: "schedule: same language, other timezone", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "tr", timezone: "Europe/Istanbul", status: "draft"}))
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "tr", timezone: "Europe/London", status: "draft"})
		}},
		{name: "schedule: same timezone, other language", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "tr", timezone: "Europe/Istanbul", status: "draft"}))
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "en", timezone: "Europe/Istanbul", status: "draft"})
		}},
		{name: "schedule: second draft in the same (language, timezone)", run: func(t *testing.T, ctx context.Context, f fixture) error {
			row := variantRow{step: f.schedule, kind: "schedule", language: "tr", timezone: "Europe/Istanbul", status: "draft"}
			mustExec(t, f.variant(ctx, row))
			return f.variant(ctx, row)
		}, want: "proposal_variant_live_slot_key"},
		{name: "schedule: new draft after a superseded one", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "tr", timezone: "Europe/Istanbul", status: "superseded"}))
			return f.variant(ctx, variantRow{step: f.schedule, kind: "schedule", language: "tr", timezone: "Europe/Istanbul", status: "draft"})
		}},
		{name: "reject then re-draft by update", run: func(t *testing.T, ctx context.Context, f fixture) error {
			row := variantRow{step: f.content, kind: "content", language: "tr", channel: "sms", status: "draft"}
			mustExec(t, f.variant(ctx, row))
			mustExec(t, f.exec(ctx, "UPDATE proposal_variant SET status = 'rejected', rejected_at = now(), rejection_reason = 'bad tone' WHERE step_id = $1", f.content))
			return f.variant(ctx, row)
		}},
	})
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want a Postgres error with code %s", err, code)
	}
	if pgErr.Code != code {
		t.Errorf("code = %s (%s), want %s", pgErr.Code, pgErr.Message, code)
	}
}

func TestConstraints_ContextNote(t *testing.T) {
	runConstraintCases(t, []constraintCase{
		{name: "note without step", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			_, err := f.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: f.campaign, Body: "mention the free shipping"})
			return err
		}},
		{name: "note with step", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			_, err := f.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: f.campaign, StepID: &f.content, Body: "shorter"})
			return err
		}},
		{name: "empty body", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			_, err := f.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: f.campaign, Body: ""})
			return err
		}, want: "context_note_body_check"},
		{name: "blank body", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			_, err := f.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: f.campaign, Body: "   "})
			return err
		}, want: "context_note_body_check"},
		{name: "unknown campaign", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			_, err := f.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: uuid.New(), Body: "x"})
			return err
		}, want: "context_note_campaign_id_fkey"},
		{name: "unknown step", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			unknown := uuid.New()
			_, err := f.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: f.campaign, StepID: &unknown, Body: "x"})
			return err
		}, want: "context_note_step_id_fkey"},
	})
}

const testSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// llmCall returns a valid accepted call for f; mutate adjusts it before insert.
func (f fixture) llmCall(ctx context.Context, mutate func(*store.InsertLLMCallParams)) error {
	p := store.InsertLLMCallParams{
		CampaignID:          f.campaign,
		StepID:              &f.audience,
		Purpose:             "segment",
		Model:               "claude-test",
		Attempt:             1,
		InputTokens:         120,
		OutputTokens:        45,
		CacheReadTokens:     0,
		CacheCreationTokens: 80,
		StopReason:          pgtype.Text{String: "end_turn", Valid: true},
		LatencyMs:           850,
		PromptName:          "segment",
		PromptVersion:       "1",
		PromptSha256:        testSHA256,
		Outcome:             "accepted",
		Response:            []byte(`{"dsl":"city = \"Istanbul\""}`),
	}
	if mutate != nil {
		mutate(&p)
	}
	_, err := f.q.InsertLLMCall(ctx, p)
	return err
}

func TestConstraints_LLMCall(t *testing.T) {
	call := func(mutate func(*store.InsertLLMCallParams)) func(*testing.T, context.Context, fixture) error {
		return func(_ *testing.T, ctx context.Context, f fixture) error { return f.llmCall(ctx, mutate) }
	}
	runConstraintCases(t, []constraintCase{
		{name: "valid accepted", run: call(nil)},
		{name: "every purpose", run: func(t *testing.T, ctx context.Context, f fixture) error {
			for _, p := range []string{"screen", "segment", "content", "channel_plan", "schedule"} {
				mustExec(t, f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.Purpose = p }))
			}
			return nil
		}},
		{name: "without step and response", run: call(func(c *store.InsertLLMCallParams) {
			c.StepID, c.Response, c.StopReason = nil, nil, pgtype.Text{}
			c.Purpose = "screen"
		})},
		{name: "unknown purpose", run: call(func(c *store.InsertLLMCallParams) { c.Purpose = "translate" }), want: "llm_call_purpose_check"},
		{name: "attempt zero", run: call(func(c *store.InsertLLMCallParams) { c.Attempt = 0 }), want: "llm_call_attempt_check"},
		{name: "negative input tokens", run: call(func(c *store.InsertLLMCallParams) { c.InputTokens = -1 }), want: "llm_call_input_tokens_check"},
		{name: "negative output tokens", run: call(func(c *store.InsertLLMCallParams) { c.OutputTokens = -1 }), want: "llm_call_output_tokens_check"},
		{name: "negative cache read tokens", run: call(func(c *store.InsertLLMCallParams) { c.CacheReadTokens = -1 }), want: "llm_call_cache_read_tokens_check"},
		{name: "negative cache creation tokens", run: call(func(c *store.InsertLLMCallParams) { c.CacheCreationTokens = -1 }), want: "llm_call_cache_creation_tokens_check"},
		{name: "zero latency", run: call(func(c *store.InsertLLMCallParams) { c.LatencyMs = 0 })},
		{name: "negative latency", run: call(func(c *store.InsertLLMCallParams) { c.LatencyMs = -1 }), want: "llm_call_latency_ms_check"},
		{name: "sha256 too short", run: call(func(c *store.InsertLLMCallParams) { c.PromptSha256 = testSHA256[:63] }), want: "llm_call_prompt_sha256_check"},
		{name: "sha256 upper case", run: call(func(c *store.InsertLLMCallParams) { c.PromptSha256 = strings.ToUpper(testSHA256) }), want: "llm_call_prompt_sha256_check"},
		{name: "sha256 not hex", run: call(func(c *store.InsertLLMCallParams) { c.PromptSha256 = strings.Repeat("g", 64) }), want: "llm_call_prompt_sha256_check"},
		{name: "sha256 with trailing newline", run: call(func(c *store.InsertLLMCallParams) { c.PromptSha256 = testSHA256 + "\n" }), want: "llm_call_prompt_sha256_check"},
		{name: "unknown outcome", run: call(func(c *store.InsertLLMCallParams) { c.Outcome = "timeout" }), want: "llm_call_outcome_check"},
		{name: "validation_failed keeps the response", run: call(func(c *store.InsertLLMCallParams) {
			c.Outcome, c.Error = "validation_failed", pgtype.Text{String: "unknown field city", Valid: true}
		})},
		{name: "refusal without error", run: call(func(c *store.InsertLLMCallParams) {
			c.Outcome, c.Response, c.StopReason = "refusal", nil, pgtype.Text{String: "refusal", Valid: true}
		})},
		{name: "api_error with error", run: call(func(c *store.InsertLLMCallParams) {
			c.Outcome, c.Response = "api_error", nil
			c.Error = pgtype.Text{String: "529 overloaded", Valid: true}
		})},
		{name: "api_error without error", run: call(func(c *store.InsertLLMCallParams) {
			c.Outcome, c.Response = "api_error", nil
		}), want: "llm_call_api_error_check"},
		{name: "unknown campaign", run: call(func(c *store.InsertLLMCallParams) { c.CampaignID = uuid.New() }), want: "llm_call_campaign_id_fkey"},
		{name: "unknown step", run: call(func(c *store.InsertLLMCallParams) {
			unknown := uuid.New()
			c.StepID = &unknown
		}), want: "llm_call_step_id_fkey"},
		{name: "unknown variant", run: call(func(c *store.InsertLLMCallParams) {
			unknown := uuid.New()
			c.VariantID = &unknown
		}), want: "llm_call_variant_id_fkey"},
		{name: "with a real variant", run: func(t *testing.T, ctx context.Context, f fixture) error {
			v, err := f.q.CreateVariant(ctx, store.CreateVariantParams{StepID: f.audience, Kind: "audience", Content: []byte(`{}`)})
			mustExec(t, err)
			return f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.VariantID = &v.ID })
		}},
	})
}

func TestAppendOnly(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)
	f := newFixture(t, pool)

	note, err := f.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: f.campaign, Body: "original"})
	mustExec(t, err)
	mustExec(t, f.llmCall(ctx, nil))
	calls, err := f.q.ListLLMCalls(ctx, f.campaign)
	mustExec(t, err)
	if len(calls) != 1 {
		t.Fatalf("ListLLMCalls() = %d rows, want 1", len(calls))
	}

	tests := []struct {
		name  string
		table string
		sql   string
		id    uuid.UUID
	}{
		{name: "update context_note", table: "context_note", sql: "UPDATE context_note SET body = 'edited' WHERE id = $1", id: note.ID},
		{name: "delete context_note", table: "context_note", sql: "DELETE FROM context_note WHERE id = $1", id: note.ID},
		{name: "update llm_call", table: "llm_call", sql: "UPDATE llm_call SET attempt = 2 WHERE id = $1", id: calls[0].ID},
		{name: "delete llm_call", table: "llm_call", sql: "DELETE FROM llm_call WHERE id = $1", id: calls[0].ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := f.exec(ctx, tt.sql, tt.id)
			assertPgCode(t, err, raiseException)
			if !strings.Contains(err.Error(), tt.table+" is append-only") {
				t.Errorf("error = %v, want it to name %s as append-only", err, tt.table)
			}
		})
	}

	t.Run("rows unchanged", func(t *testing.T) {
		gotNotes, err := f.q.ListContextNotes(ctx, f.campaign)
		mustExec(t, err)
		if len(gotNotes) != 1 || gotNotes[0].Body != "original" {
			t.Errorf("notes = %+v, want the one original note", gotNotes)
		}
		gotCalls, err := f.q.ListLLMCalls(ctx, f.campaign)
		mustExec(t, err)
		if len(gotCalls) != 1 || gotCalls[0].Attempt != 1 {
			t.Errorf("calls = %+v, want the one original call", gotCalls)
		}
	})
}

func TestQueries(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)
	q := store.New(pool)

	camp, err := q.CreateCampaign(ctx, "Remind cart abandoners in Istanbul")
	mustExec(t, err)
	other, err := q.CreateCampaign(ctx, "Another brief")
	mustExec(t, err)

	t.Run("campaign round trip", func(t *testing.T) {
		if camp.Status != "drafting" || camp.Brief != "Remind cart abandoners in Istanbul" {
			t.Errorf("CreateCampaign() = %+v, want drafting status and the brief", camp)
		}
		if id := camp.ID.String(); id[14] != '7' {
			t.Errorf("campaign id = %s, want a UUID version 7", id)
		}
		if !camp.CreatedAt.Valid || !camp.UpdatedAt.Valid {
			t.Errorf("timestamps = %v, %v, want both set by default", camp.CreatedAt, camp.UpdatedAt)
		}
		got, err := q.GetCampaign(ctx, camp.ID)
		if err != nil {
			t.Fatalf("GetCampaign() error = %v", err)
		}
		if got.ID != camp.ID || got.Brief != camp.Brief {
			t.Errorf("GetCampaign() = %+v, want %+v", got, camp)
		}
		if _, err := q.GetCampaign(ctx, uuid.New()); err == nil {
			t.Error("GetCampaign(unknown) error = nil, want no rows")
		}
	})

	var audience, content store.ProposalStep
	t.Run("steps", func(t *testing.T) {
		audience, err = q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "audience", Version: 1})
		mustExec(t, err)
		content, err = q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "content", Version: 1})
		mustExec(t, err)
		_, err = q.CreateStep(ctx, store.CreateStepParams{CampaignID: other.ID, Kind: "audience", Version: 1})
		mustExec(t, err)

		if audience.Status != "open" || audience.ApprovedAt.Valid {
			t.Errorf("CreateStep() = %+v, want open without approved_at", audience)
		}
		steps, err := q.ListSteps(ctx, camp.ID)
		if err != nil {
			t.Fatalf("ListSteps() error = %v", err)
		}
		var kinds []string
		for _, s := range steps {
			kinds = append(kinds, s.Kind)
		}
		if want := []string{"audience", "content"}; !slices.Equal(kinds, want) {
			t.Errorf("ListSteps() kinds = %v, want %v (this campaign only, creation order)", kinds, want)
		}
	})

	t.Run("variants", func(t *testing.T) {
		a, err := q.CreateVariant(ctx, store.CreateVariantParams{StepID: audience.ID, Kind: "audience", Content: []byte(`{"count": 12}`)})
		if err != nil {
			t.Fatalf("CreateVariant(audience) error = %v", err)
		}
		if a.Language.Valid || a.Channel.Valid || a.Status != "draft" {
			t.Errorf("audience variant = %+v, want NULL language and channel, draft", a)
		}
		_, err = q.CreateVariant(ctx, store.CreateVariantParams{
			StepID:   content.ID,
			Kind:     "content",
			Language: pgtype.Text{String: "tr", Valid: true},
			Channel:  pgtype.Text{String: "sms", Valid: true},
			Content:  []byte(`{"text": "Merhaba {{first_name}}"}`),
		})
		if err != nil {
			t.Fatalf("CreateVariant(content) error = %v", err)
		}
		got, err := q.ListVariants(ctx, content.ID)
		if err != nil {
			t.Fatalf("ListVariants() error = %v", err)
		}
		if len(got) != 1 || got[0].Language.String != "tr" || got[0].Channel.String != "sms" {
			t.Fatalf("ListVariants() = %+v, want the one tr/sms variant", got)
		}
		if !strings.Contains(string(got[0].Content), "first_name") {
			t.Errorf("content = %s, want the stored JSON back", got[0].Content)
		}
	})

	t.Run("context notes", func(t *testing.T) {
		if _, err := q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: camp.ID, Body: "no emojis"}); err != nil {
			t.Fatalf("InsertContextNote() error = %v", err)
		}
		if _, err := q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: camp.ID, StepID: &content.ID, Body: "shorter"}); err != nil {
			t.Fatalf("InsertContextNote(step) error = %v", err)
		}
		if _, err := q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: other.ID, Body: "elsewhere"}); err != nil {
			t.Fatalf("InsertContextNote(other) error = %v", err)
		}
		notes, err := q.ListContextNotes(ctx, camp.ID)
		if err != nil {
			t.Fatalf("ListContextNotes() error = %v", err)
		}
		if len(notes) != 2 || notes[0].Body != "no emojis" || notes[0].StepID != nil || notes[1].StepID == nil || *notes[1].StepID != content.ID {
			t.Errorf("ListContextNotes() = %+v, want the two notes of this campaign in order", notes)
		}
	})

	t.Run("llm calls", func(t *testing.T) {
		f := fixture{pool: pool, q: q, campaign: camp.ID, audience: audience.ID, content: content.ID}
		mustExec(t, f.llmCall(ctx, nil))
		mustExec(t, f.llmCall(ctx, func(c *store.InsertLLMCallParams) {
			c.Attempt, c.Outcome, c.Response = 2, "api_error", nil
			c.Error = pgtype.Text{String: "429 rate limited", Valid: true}
		}))
		mustExec(t, f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.CampaignID = other.ID; c.StepID = nil }))

		calls, err := q.ListLLMCalls(ctx, camp.ID)
		if err != nil {
			t.Fatalf("ListLLMCalls() error = %v", err)
		}
		if len(calls) != 2 {
			t.Fatalf("ListLLMCalls() = %d rows, want 2 (this campaign only)", len(calls))
		}
		first, second := calls[0], calls[1]
		if first.Attempt != 1 || first.Outcome != "accepted" || first.CacheCreationTokens != 80 || first.PromptSha256 != testSHA256 {
			t.Errorf("first call = %+v, want the accepted attempt with its token counts", first)
		}
		if !strings.Contains(string(first.Response), "Istanbul") {
			t.Errorf("response = %s, want the stored JSON back", first.Response)
		}
		if second.Attempt != 2 || second.Outcome != "api_error" || second.Error.String != "429 rate limited" || second.Response != nil {
			t.Errorf("second call = %+v, want the api_error attempt without a response", second)
		}
	})
}

// Migration 00003 widens llm_call.purpose (D-070); Down restores the list of 00002 and is refused
// while a row with a new purpose exists, because llm_call is append-only.
func TestMigration00003_PurposeList(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)
	q := store.New(pool)
	camp, err := q.CreateCampaign(ctx, "brief")
	mustExec(t, err)
	f := fixture{pool: pool, q: q, campaign: camp.ID}
	step, err := q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "audience", Version: 1})
	mustExec(t, err)
	f.audience = step.ID

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, store.Migrations())
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	insert := func(purpose string) error {
		return f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.Purpose = purpose })
	}

	if err := insert("channel_plan"); err != nil {
		t.Fatalf("channel_plan after Up: %v", err)
	}
	// 00004 is on top of 00003; roll it back first.
	if _, err := provider.DownTo(ctx, 3); err != nil {
		t.Fatalf("DownTo(3) error = %v", err)
	}
	// Down is refused while the table holds a purpose the old list does not know.
	if _, err := provider.Down(ctx); err == nil || !strings.Contains(err.Error(), "llm_call_purpose_check") {
		t.Fatalf("Down() with a channel_plan row: err = %v, want the constraint to refuse it", err)
	}
	if got := queryStrings(t, pool, "SELECT version_id::text FROM goose_db_version WHERE version_id > 0 AND is_applied ORDER BY version_id"); !slices.Equal(got, []string{"1", "2", "3"}) {
		t.Fatalf("versions after the refused Down = %v: the failed migration must roll back", got)
	}
	if err := insert("schedule"); err != nil {
		t.Fatalf("the constraint was lost by the refused Down: %v", err)
	}
}

func TestMigration00003_DownRestoresTheOldList(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)
	camp, err := store.New(pool).CreateCampaign(ctx, "brief")
	mustExec(t, err)
	step, err := store.New(pool).CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "audience", Version: 1})
	mustExec(t, err)
	f := fixture{pool: pool, q: store.New(pool), campaign: camp.ID, audience: step.ID}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, store.Migrations())
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.DownTo(ctx, 3); err != nil { // 00004 is on top of 00003
		t.Fatalf("DownTo(3) error = %v", err)
	}
	res, err := provider.Down(ctx)
	if err != nil || res.Source.Version != 3 {
		t.Fatalf("Down() = %+v, %v; want to roll back 00003", res, err)
	}
	for _, p := range []string{"screen", "segment", "content"} {
		if err := f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.Purpose = p }); err != nil {
			t.Errorf("%s after Down: %v", p, err)
		}
	}
	for _, p := range []string{"channel_plan", "schedule"} {
		if err := f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.Purpose = p }); err == nil || !strings.Contains(err.Error(), "llm_call_purpose_check") {
			t.Errorf("%s after Down: err = %v, want llm_call_purpose_check", p, err)
		}
	}
	if _, err := provider.UpByOne(ctx); err != nil {
		t.Fatalf("UpByOne() error = %v", err)
	}
	for _, p := range []string{"channel_plan", "schedule"} {
		if err := f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.Purpose = p }); err != nil {
			t.Errorf("%s after re-Up: %v", p, err)
		}
	}
}

// The queries of the proposal flow (T-023): locks, status changes and lookups.
func TestQueries_Flow(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)
	q := store.New(pool)
	camp, err := q.CreateCampaign(ctx, "brief")
	mustExec(t, err)

	t.Run("campaign lock and status", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		mustExec(t, err)
		defer tx.Rollback(ctx)
		locked, err := q.WithTx(tx).GetCampaignForUpdate(ctx, camp.ID)
		if err != nil || locked.ID != camp.ID {
			t.Fatalf("GetCampaignForUpdate() = %+v, %v", locked, err)
		}
		// A second locker waits until the first transaction ends.
		waited := make(chan struct{})
		go func() {
			defer close(waited)
			_, _ = q.GetCampaignForUpdate(ctx, camp.ID)
		}()
		select {
		case <-waited:
			t.Fatal("GetCampaignForUpdate did not wait for the lock")
		case <-time.After(200 * time.Millisecond):
		}
		mustExec(t, tx.Rollback(ctx))
		<-waited

		got, err := q.SetCampaignStatus(ctx, store.SetCampaignStatusParams{ID: camp.ID, Status: "cancelled"})
		if err != nil || got.Status != "cancelled" || !got.UpdatedAt.Time.After(camp.UpdatedAt.Time) {
			t.Errorf("SetCampaignStatus() = %+v, %v; want cancelled with a newer updated_at", got, err)
		}
		if _, err := q.SetCampaignStatus(ctx, store.SetCampaignStatusParams{ID: camp.ID, Status: "paused"}); err == nil || !strings.Contains(err.Error(), "campaign_status_check") {
			t.Errorf("SetCampaignStatus(paused) err = %v, want campaign_status_check", err)
		}
	})

	var aud1, aud2, plan store.ProposalStep
	t.Run("steps", func(t *testing.T) {
		if v, err := q.MaxStepVersion(ctx, store.MaxStepVersionParams{CampaignID: camp.ID, Kind: "audience"}); err != nil || v != 0 {
			t.Fatalf("MaxStepVersion() = %d, %v; want 0 before any step", v, err)
		}
		aud1, err = q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "audience", Version: 1})
		mustExec(t, err)
		approved, err := q.ApproveStep(ctx, aud1.ID)
		if err != nil || approved.Status != "approved" || !approved.ApprovedAt.Valid {
			t.Fatalf("ApproveStep() = %+v, %v", approved, err)
		}
		plan, err = q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "channel_plan", Version: 1})
		mustExec(t, err)
		if got, err := q.GetStep(ctx, plan.ID); err != nil || got.Kind != "channel_plan" {
			t.Errorf("GetStep() = %+v, %v", got, err)
		}
		if _, err := q.GetStep(ctx, uuid.New()); err == nil {
			t.Error("GetStep(unknown) error = nil")
		}

		// Superseding the audience keeps its approved_at (history) and frees the live slot.
		sup, err := q.SupersedeStep(ctx, aud1.ID)
		if err != nil || sup.Status != "superseded" || !sup.ApprovedAt.Valid {
			t.Fatalf("SupersedeStep() = %+v, %v", sup, err)
		}
		aud2, err = q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "audience", Version: 2})
		mustExec(t, err)
		if v, err := q.MaxStepVersion(ctx, store.MaxStepVersionParams{CampaignID: camp.ID, Kind: "audience"}); err != nil || v != 2 {
			t.Errorf("MaxStepVersion() = %d, %v; want 2 (superseded versions count)", v, err)
		}
		live, err := q.ListLiveSteps(ctx, camp.ID)
		mustExec(t, err)
		var ids []uuid.UUID
		for _, s := range live {
			ids = append(ids, s.ID)
		}
		if !slices.Equal(ids, []uuid.UUID{plan.ID, aud2.ID}) {
			t.Errorf("ListLiveSteps() = %v, want the plan and audience v2 in creation order, without the superseded v1", ids)
		}
	})

	t.Run("variants", func(t *testing.T) {
		mk := func() store.ProposalVariant {
			v, err := q.CreateVariant(ctx, store.CreateVariantParams{StepID: aud2.ID, Kind: "audience", Content: []byte(`{}`)})
			mustExec(t, err)
			return v
		}
		a := mk()
		if got, err := q.GetVariant(ctx, a.ID); err != nil || got.Status != "draft" {
			t.Fatalf("GetVariant() = %+v, %v", got, err)
		}
		rej, err := q.RejectVariant(ctx, store.RejectVariantParams{ID: a.ID, RejectionReason: pgtype.Text{String: "too wide", Valid: true}})
		if err != nil || rej.Status != "rejected" || !rej.RejectedAt.Valid || rej.RejectionReason.String != "too wide" {
			t.Fatalf("RejectVariant() = %+v, %v", rej, err)
		}
		if _, err := q.RejectVariant(ctx, store.RejectVariantParams{ID: a.ID}); err == nil || !strings.Contains(err.Error(), "proposal_variant_rejected_check") {
			t.Errorf("RejectVariant(no reason) err = %v, want proposal_variant_rejected_check", err)
		}
		// A rejected variant does not hold its slot: the replacement can be created before it is superseded.
		b := mk()
		app, err := q.ApproveVariant(ctx, b.ID)
		if err != nil || app.Status != "approved" || !app.ApprovedAt.Valid {
			t.Fatalf("ApproveVariant() = %+v, %v", app, err)
		}
		live, err := q.ListLiveVariants(ctx, aud2.ID)
		mustExec(t, err)
		if len(live) != 2 || live[0].Status != "rejected" || live[1].Status != "approved" {
			t.Errorf("ListLiveVariants() = %d rows %+v, want the rejected and the approved", len(live), live)
		}
		sup, err := q.SupersedeVariant(ctx, a.ID)
		if err != nil || sup.Status != "superseded" || !sup.RejectedAt.Valid || sup.RejectionReason.String != "too wide" {
			t.Fatalf("SupersedeVariant() = %+v, %v; want the rejection kept as history", sup, err)
		}
		if live, err := q.ListLiveVariants(ctx, aud2.ID); err != nil || len(live) != 1 || live[0].ID != b.ID {
			t.Errorf("ListLiveVariants() after supersede = %+v, %v", live, err)
		}
	})

	t.Run("latest llm calls of a step", func(t *testing.T) {
		f := fixture{pool: pool, q: q, campaign: camp.ID, audience: aud2.ID}
		for _, outcome := range []string{"validation_failed", "validation_failed", "accepted"} {
			mustExec(t, f.llmCall(ctx, func(c *store.InsertLLMCallParams) {
				c.Outcome = outcome
				c.Purpose = "segment"
			}))
		}
		mustExec(t, f.llmCall(ctx, func(c *store.InsertLLMCallParams) { c.StepID = &plan.ID; c.Purpose = "channel_plan" }))
		rows, err := q.ListLatestLLMCallsOfStep(ctx, store.ListLatestLLMCallsOfStepParams{StepID: &aud2.ID, Limit: 2})
		mustExec(t, err)
		if len(rows) != 2 || rows[0].Outcome != "accepted" || rows[1].Outcome != "validation_failed" {
			t.Errorf("ListLatestLLMCallsOfStep(limit 2) = %d rows, outcomes %v; want the newest two, newest first", len(rows), []string{rows[0].Outcome, rows[1].Outcome})
		}
		for _, r := range rows {
			if r.StepID == nil || *r.StepID != aud2.ID {
				t.Errorf("a row of another step: %+v", r)
			}
		}
	})
}

// provider00004 opens a goose provider over the proposal migrations of pool.
func provider00004(t *testing.T, pool *pgxpool.Pool) *goose.Provider {
	t.Helper()
	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, store.Migrations())
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	return provider
}

// Migration 00004 adds proposal_variant.timezone (D-076). Up backfills the schedule rows an older
// database holds with the language mapping of D-070 before the CHECK is added; Down drops the
// column and restores the old slot index and shape check.
func TestMigration00004_BackfillAndRoundTrip(t *testing.T) {
	ctx := t.Context()
	pool := newPool(t)
	if err := migrate.UpPlatform(ctx, pool, discardLogger()); err != nil {
		t.Fatalf("UpPlatform() error = %v", err)
	}
	provider := provider00004(t, pool)

	// A database at 00003, with schedule variants written by the old code: one per language.
	if _, err := provider.UpTo(ctx, 3); err != nil {
		t.Fatalf("UpTo(3) error = %v", err)
	}
	if got := queryStrings(t, pool, "SELECT column_name FROM information_schema.columns WHERE table_schema = 'proposal' AND table_name = 'proposal_variant' AND column_name = 'timezone'"); len(got) != 0 {
		t.Fatalf("timezone exists before 00004: %v", got)
	}
	q := store.New(pool)
	camp, err := q.CreateCampaign(ctx, "brief")
	mustExec(t, err)
	sched, err := q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "schedule", Version: 1})
	mustExec(t, err)
	content, err := q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "content", Version: 1})
	mustExec(t, err)
	insert := func(step uuid.UUID, kind, lang, channel, status string) {
		t.Helper()
		var ch any
		if channel != "" {
			ch = channel
		}
		_, err := pool.Exec(ctx, `INSERT INTO proposal_variant (step_id, kind, language, channel, status, content, approved_at)
			VALUES ($1, $2, $3, $4, $5, '{"send_at":null}'::jsonb, CASE WHEN $5 = 'approved' THEN now() END)`, step, kind, lang, ch, status)
		mustExec(t, err)
	}
	insert(sched.ID, "schedule", "tr", "", "approved")
	insert(sched.ID, "schedule", "en", "", "draft")
	insert(sched.ID, "schedule", "en", "", "superseded") // history is backfilled too
	insert(content.ID, "content", "tr", "sms", "draft")

	if _, err := provider.UpByOne(ctx); err != nil {
		t.Fatalf("Up 00004 error = %v", err)
	}
	got := queryStrings(t, pool, `SELECT kind || ':' || language || ':' || coalesce(timezone, 'NULL') || ':' || status
		FROM proposal_variant ORDER BY kind, language, status`)
	want := []string{
		"content:tr:NULL:draft",
		"schedule:en:Europe/London:draft", "schedule:en:Europe/London:superseded",
		"schedule:tr:Europe/Istanbul:approved",
	}
	if !slices.Equal(got, want) {
		t.Errorf("variants after Up = %v, want %v (tr -> Europe/Istanbul, en -> Europe/London; non-schedule stays NULL)", got, want)
	}
	// The new live slot index lets a language hold a schedule per time zone.
	insertTZ := func(lang, tz string) error {
		_, err := pool.Exec(ctx, `INSERT INTO proposal_variant (step_id, kind, language, timezone, status, content)
			VALUES ($1, 'schedule', $2, $3, 'draft', '{}'::jsonb)`, sched.ID, lang, tz)
		return err
	}
	if err := insertTZ("tr", "Europe/London"); err != nil {
		t.Errorf("a second zone of the same language: %v", err)
	}
	wantConstraint(t, insertTZ("tr", "Europe/London"), "proposal_variant_live_slot_key")

	// Down while two zones of one language are live: the old index cannot hold them, so Down is
	// refused and rolls back.
	if _, err := provider.Down(ctx); err == nil || !strings.Contains(err.Error(), "proposal_variant_live_slot_key") {
		t.Fatalf("Down() with two live zones of one language: err = %v, want the old slot index to refuse it", err)
	}
	if got := queryStrings(t, pool, "SELECT version_id::text FROM goose_db_version WHERE version_id > 0 AND is_applied ORDER BY version_id"); !slices.Equal(got, []string{"1", "2", "3", "4"}) {
		t.Fatalf("versions after the refused Down = %v: the failed migration must roll back", got)
	}
	mustExec(t, func() error {
		_, err := pool.Exec(ctx, `UPDATE proposal_variant SET status = 'superseded' WHERE timezone = 'Europe/London' AND language = 'tr'`)
		return err
	}())

	// Down: the column and the new constraints are gone, the rows stay.
	if res, err := provider.Down(ctx); err != nil || res.Source.Version != 4 {
		t.Fatalf("Down() = %+v, %v; want to roll back 00004", res, err)
	}
	if got := queryStrings(t, pool, "SELECT column_name FROM information_schema.columns WHERE table_schema = 'proposal' AND table_name = 'proposal_variant' AND column_name = 'timezone'"); len(got) != 0 {
		t.Errorf("timezone survived Down: %v", got)
	}
	if got := queryStrings(t, pool, "SELECT conname FROM pg_constraint WHERE conname = 'proposal_variant_timezone_check'"); len(got) != 0 {
		t.Errorf("timezone check survived Down: %v", got)
	}
	if n := queryStrings(t, pool, "SELECT count(*)::text FROM proposal_variant"); n[0] != "5" {
		t.Errorf("rows after Down = %s, want all 5 kept", n[0])
	}
	// The old shape (language only) and the old slot index are back.
	oldInsert := func(lang string) error {
		_, err := pool.Exec(ctx, `INSERT INTO proposal_variant (step_id, kind, language, status, content)
			VALUES ($1, 'schedule', $2, 'draft', '{}'::jsonb)`, sched.ID, lang)
		return err
	}
	wantConstraint(t, oldInsert("en"), "proposal_variant_live_slot_key") // en draft exists

	// Up again: the CHECK is back and rows written without a timezone are refused.
	if _, err := provider.UpByOne(ctx); err != nil {
		t.Fatalf("Up 00004 again error = %v", err)
	}
	wantConstraint(t, oldInsert("tr"), "proposal_variant_shape_check")
}

// A schedule variant written through the generated query carries its timezone; every other kind
// has none (D-076).
func TestQueries_CreateVariantTimezone(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)
	q := store.New(pool)
	camp, err := q.CreateCampaign(ctx, "brief")
	mustExec(t, err)
	sched, err := q.CreateStep(ctx, store.CreateStepParams{CampaignID: camp.ID, Kind: "schedule", Version: 1})
	mustExec(t, err)
	v, err := q.CreateVariant(ctx, store.CreateVariantParams{
		StepID: sched.ID, Kind: "schedule", Language: pgtype.Text{String: "tr", Valid: true},
		Timezone: pgtype.Text{String: "Europe/Istanbul", Valid: true}, Content: []byte(`{"send_at":null,"rationale":"now"}`),
	})
	if err != nil || v.Timezone.String != "Europe/Istanbul" || !v.Timezone.Valid {
		t.Fatalf("CreateVariant(schedule) = %+v, %v", v, err)
	}
	if _, err := q.CreateVariant(ctx, store.CreateVariantParams{
		StepID: sched.ID, Kind: "schedule", Language: pgtype.Text{String: "en", Valid: true}, Content: []byte(`{}`),
	}); err == nil {
		t.Error("a schedule variant without a timezone was accepted")
	}
	got, err := q.ListVariants(ctx, sched.ID)
	if err != nil || len(got) != 1 || got[0].Timezone.String != "Europe/Istanbul" {
		t.Errorf("ListVariants = %+v, %v", got, err)
	}
}
