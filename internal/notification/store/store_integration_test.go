//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/migrate"
)

const (
	adminDB = "postgres"
	// pgNotNullViolation is the SQLSTATE of a NOT NULL violation, which names no constraint.
	pgNotNullViolation = "23502"
	// serviceSchema is the schema initdb gives the notification role (D-048).
	serviceSchema = "notification"
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

// newPool creates an empty database with a notification schema, opens a pool whose search_path is
// "notification,public" like the service role's (D-048), and returns it. Migrations are not applied.
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

// newMigratedPool is newPool plus the platform and notification migration sets, in startup order.
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
			name: "tables in schema notification",
			sql:  "SELECT tablename FROM pg_tables WHERE schemaname = 'notification' ORDER BY tablename",
			want: []string{
				"delivery", "goose_db_version", "goose_platform_version", "locale_schedule", "message",
				"outbox", "processed_event", "recipient_batch", "send_campaign",
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
			want: []string{"1", "2"},
		},
		{
			name: "platform versions",
			sql:  "SELECT version_id::text FROM goose_platform_version WHERE version_id > 0 ORDER BY version_id",
			want: []string{"1", "2"},
		},
		{
			name: "delivery indexes",
			sql:  "SELECT indexname FROM pg_indexes WHERE schemaname = 'notification' AND tablename = 'delivery' ORDER BY indexname",
			want: []string{
				"delivery_campaign_channel_status_idx", "delivery_campaign_customer_channel_key",
				"delivery_message_id_idx", "delivery_pkey", "delivery_schedule_id_idx", "delivery_schedule_id_pending_idx",
			},
		},
		{
			name: "partial pending index predicate",
			sql: `SELECT pg_get_expr(i.indpred, i.indrelid) FROM pg_index i
				JOIN pg_class c ON c.oid = i.indexrelid WHERE c.relname = 'delivery_schedule_id_pending_idx'`,
			want: []string{"(status = 'pending'::text)"},
		},
		{
			name: "composite foreign key targets",
			sql: `SELECT conname FROM pg_constraint WHERE connamespace = 'notification'::regnamespace AND contype = 'u'
				AND conname IN ('locale_schedule_id_campaign_id_key', 'message_id_campaign_id_channel_key') ORDER BY conname`,
			want: []string{"locale_schedule_id_campaign_id_key", "message_id_campaign_id_channel_key"},
		},
		{
			name: "no triggers on service tables",
			sql: `SELECT tgname FROM pg_trigger t JOIN pg_class c ON c.oid = t.tgrelid
				WHERE NOT tgisinternal AND c.relname IN ('send_campaign', 'locale_schedule', 'message', 'recipient_batch', 'delivery')`,
			want: nil,
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

	t.Run("id defaults", func(t *testing.T) {
		got := queryStrings(t, pool, `SELECT table_name || '.' || column_name FROM information_schema.columns
			WHERE table_schema = 'notification' AND table_name <> 'outbox' AND column_name = 'id' AND column_default LIKE 'uuidv7()%' ORDER BY 1`)
		want := []string{"delivery.id", "locale_schedule.id", "message.id"}
		if !slices.Equal(got, want) {
			t.Errorf("uuidv7 defaults on %v, want %v (send_campaign.id has none)", got, want)
		}
	})

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

	left := queryStrings(t, pool, "SELECT tablename FROM pg_tables WHERE schemaname = 'notification' ORDER BY tablename")
	want := []string{"goose_db_version", "goose_platform_version", "outbox", "processed_event"}
	if !slices.Equal(left, want) {
		t.Errorf("tables after Down = %v, want %v (service tables gone, platform set untouched)", left, want)
	}
	if got := queryStrings(t, pool, "SELECT indexname FROM pg_indexes WHERE schemaname = 'notification' AND indexname LIKE ANY (ARRAY['delivery\\_%', 'locale\\_schedule\\_%', 'message\\_%', 'recipient\\_batch\\_%', 'send\\_campaign\\_%'])"); len(got) != 0 {
		t.Errorf("indexes after Down = %v, want none", got)
	}

	if err := migrate.Up(ctx, pool, store.Migrations(), discardLogger()); err != nil {
		t.Fatalf("Up() after Down error = %v", err)
	}
	if _, err := store.New(pool).CreateSendCampaign(ctx, store.CreateSendCampaignParams{ID: uuid.New(), CtaUrl: "https://shop.example/cart", BatchCount: 1}); err != nil {
		t.Errorf("CreateSendCampaign() after re-up error = %v", err)
	}
}

// fixture is one campaign with a tr schedule and tr sms and email messages, what a delivery needs.
type fixture struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	campaign uuid.UUID
	schedule uuid.UUID
	message  uuid.UUID // the sms message
	email    uuid.UUID
}

func newFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := t.Context()
	q := store.New(pool)
	c, err := q.CreateSendCampaign(ctx, store.CreateSendCampaignParams{ID: uuid.New(), CtaUrl: "https://shop.example/cart", BatchCount: 3})
	if err != nil {
		t.Fatalf("CreateSendCampaign() error = %v", err)
	}
	s, err := q.CreateLocaleSchedule(ctx, store.CreateLocaleScheduleParams{CampaignID: c.ID, Language: "tr", Timezone: "Europe/Istanbul"})
	if err != nil {
		t.Fatalf("CreateLocaleSchedule() error = %v", err)
	}
	m, err := q.CreateMessage(ctx, store.CreateMessageParams{CampaignID: c.ID, Language: "tr", Channel: "sms", Content: []byte(`{"text":"Merhaba {{first_name}}"}`)})
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}
	e, err := q.CreateMessage(ctx, store.CreateMessageParams{CampaignID: c.ID, Language: "tr", Channel: "email", Content: []byte(`{"subject":"Merhaba"}`)})
	if err != nil {
		t.Fatalf("CreateMessage(email) error = %v", err)
	}
	return fixture{pool: pool, q: q, campaign: c.ID, schedule: s.ID, message: m.ID, email: e.ID}
}

// otherCampaign creates a second campaign with its own schedule and sms message.
func (f fixture) otherCampaign(t *testing.T, ctx context.Context) fixture {
	t.Helper()
	return newFixture(t, f.pool)
}

func (f fixture) exec(ctx context.Context, sql string, args ...any) error {
	_, err := f.pool.Exec(ctx, sql, args...)
	return err
}

// deliveryRow is a delivery insert; nil fields are NULL and zero strings take the shown defaults.
type deliveryRow struct {
	campaign any
	schedule any
	message  any
	customer string
	channel  string
	status   string
	attempts int
	first    any
	contact  any
	lease    any
	sentAt   any
	lastErr  any
}

// delivery inserts a row built from d; unset campaign/schedule/message point at the fixture,
// unset customer is unique per call, unset channel is sms, unset message is the fixture message
// of that channel and unset status is pending. first_name and contact stay NULL unless set; pii
// sets them.
func (f fixture) delivery(ctx context.Context, d deliveryRow) error {
	if d.campaign == nil {
		d.campaign = f.campaign
	}
	if d.schedule == nil {
		d.schedule = f.schedule
	}
	if d.channel == "" {
		d.channel = "sms"
	}
	if d.message == nil {
		d.message = f.message
		if d.channel == "email" {
			d.message = f.email
		}
	}
	if d.customer == "" {
		d.customer = "CUS-" + uuid.NewString()
	}
	if d.status == "" {
		d.status = "pending"
	}
	return f.exec(ctx, `INSERT INTO delivery
		(campaign_id, schedule_id, message_id, customer_id, channel, status, attempts, first_name, contact, lease_until, sent_at, last_error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		d.campaign, d.schedule, d.message, d.customer, d.channel, d.status, d.attempts, d.first, d.contact, d.lease, d.sentAt, d.lastErr)
}

// pii is a deliveryRow with personal data set.
func pii(d deliveryRow) deliveryRow {
	d.first, d.contact = "Ayse", "+905550000000"
	return d
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
	// want is the violated constraint or unique index; empty means the row is accepted.
	want string
}

func runConstraintCases(t *testing.T, tests []constraintCase) {
	t.Helper()
	pool := newMigratedPool(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, pool)
			wantConstraint(t, tt.run(t, t.Context(), f), tt.want)
		})
	}
}

func mustExec(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup statement error = %v", err)
	}
}

func TestConstraints_SendCampaign(t *testing.T) {
	insert := func(cta string, batches int, status string, completedAt any) func(*testing.T, context.Context, fixture) error {
		return func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO send_campaign (id, cta_url, batch_count, status, completed_at) VALUES ($1, $2, $3, $4, $5)",
				uuid.New(), cta, batches, status, completedAt)
		}
	}
	now := time.Now()
	runConstraintCases(t, []constraintCase{
		{name: "valid", run: insert("https://shop.example", 1, "scheduled", nil)},
		{name: "scheduled and sending without completed_at", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, insert("https://a", 1, "scheduled", nil)(t, ctx, f))
			return insert("https://a", 2, "sending", nil)(t, ctx, f)
		}},
		{name: "completed with completed_at", run: insert("https://a", 1, "completed", now)},
		{name: "completed without completed_at", run: insert("https://a", 1, "completed", nil), want: "send_campaign_completed_at_check"},
		{name: "cancelled without completed_at", run: insert("https://a", 1, "cancelled", nil)},
		{name: "cancelled with completed_at", run: insert("https://a", 1, "cancelled", now)},
		{name: "empty cta_url", run: insert("", 1, "scheduled", nil), want: "send_campaign_cta_url_check"},
		{name: "blank cta_url", run: insert(" \t", 1, "scheduled", nil), want: "send_campaign_cta_url_check"},
		{name: "batch_count zero", run: insert("https://a", 0, "scheduled", nil), want: "send_campaign_batch_count_check"},
		{name: "batch_count negative", run: insert("https://a", -1, "scheduled", nil), want: "send_campaign_batch_count_check"},
		{name: "unknown status", run: insert("https://a", 1, "paused", nil), want: "send_campaign_status_check"},
		{name: "duplicate id", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO send_campaign (id, cta_url, batch_count) VALUES ($1, 'https://a', 1)", f.campaign)
		}, want: "send_campaign_pkey"},
		{name: "defaults", run: func(t *testing.T, ctx context.Context, f fixture) error {
			c, err := f.q.GetSendCampaign(ctx, f.campaign)
			if err != nil {
				return err
			}
			if c.Status != "scheduled" || c.CompletedAt.Valid || !c.CreatedAt.Valid || !c.UpdatedAt.Valid {
				t.Errorf("defaults = %+v, want scheduled, no completed_at, both timestamps set", c)
			}
			return nil
		}},
	})
}

func TestConstraints_LocaleSchedule(t *testing.T) {
	insert := func(lang, tz, status string, fired any) func(*testing.T, context.Context, fixture) error {
		return func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO locale_schedule (campaign_id, language, timezone, status, fired_at) VALUES ($1, $2, $3, $4, $5)",
				f.campaign, lang, tz, status, fired)
		}
	}
	runConstraintCases(t, []constraintCase{
		{name: "valid en", run: insert("en", "Europe/London", "scheduled", nil)},
		{name: "same language, other timezone", run: insert("tr", "Europe/Berlin", "scheduled", nil)},
		{name: "send_at NULL means now", run: func(t *testing.T, ctx context.Context, f fixture) error {
			ss, err := f.q.ListLocaleSchedules(ctx, f.campaign)
			if err != nil {
				return err
			}
			if len(ss) != 1 || ss[0].SendAt.Valid || ss[0].Status != "scheduled" || ss[0].FiredAt.Valid {
				t.Errorf("fixture schedule = %+v, want send_at NULL, scheduled, not fired", ss)
			}
			return nil
		}},
		{name: "unknown language", run: insert("de", "Europe/Berlin", "scheduled", nil), want: "locale_schedule_language_check"},
		{name: "empty timezone", run: insert("en", "", "scheduled", nil), want: "locale_schedule_timezone_check"},
		{name: "blank timezone", run: insert("en", "  ", "scheduled", nil), want: "locale_schedule_timezone_check"},
		{name: "unknown status", run: insert("en", "Europe/London", "paused", nil), want: "locale_schedule_status_check"},
		{name: "fired with fired_at", run: insert("en", "Europe/London", "fired", time.Now())},
		{name: "fired without fired_at", run: insert("en", "Europe/London", "fired", nil), want: "locale_schedule_fired_at_check"},
		{name: "cancelled without fired_at", run: insert("en", "Europe/London", "cancelled", nil)},
		{name: "duplicate slot", run: insert("tr", "Europe/Istanbul", "scheduled", nil), want: "locale_schedule_campaign_language_timezone_key"},
		{name: "unknown campaign", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO locale_schedule (campaign_id, language, timezone) VALUES ($1, 'en', 'UTC')", uuid.New())
		}, want: "locale_schedule_campaign_id_fkey"},
	})
}

func TestConstraints_Message(t *testing.T) {
	insert := func(lang, channel, content string) func(*testing.T, context.Context, fixture) error {
		return func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO message (campaign_id, language, channel, content) VALUES ($1, $2, $3, $4::jsonb)", f.campaign, lang, channel, content)
		}
	}
	runConstraintCases(t, []constraintCase{
		{name: "valid", run: insert("en", "email", `{"subject":"s","html":"h"}`)},
		{name: "same language, other channel", run: insert("en", "sms", `{}`)},
		{name: "same channel, other language", run: insert("en", "email", `{}`)},
		{name: "unknown language", run: insert("de", "sms", `{}`), want: "message_language_check"},
		{name: "unknown channel", run: insert("tr", "push", `{}`), want: "message_channel_check"},
		{name: "duplicate slot", run: insert("tr", "sms", `{}`), want: "message_campaign_language_channel_key"},
		{name: "unknown campaign", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO message (campaign_id, language, channel, content) VALUES ($1, 'en', 'sms', '{}')", uuid.New())
		}, want: "message_campaign_id_fkey"},
	})
}

func TestConstraints_RecipientBatch(t *testing.T) {
	insert := func(batchNo, recipients int) func(*testing.T, context.Context, fixture) error {
		return func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO recipient_batch (campaign_id, batch_no, recipients) VALUES ($1, $2, $3)", f.campaign, batchNo, recipients)
		}
	}
	runConstraintCases(t, []constraintCase{
		{name: "valid", run: insert(1, 500)},
		{name: "empty batch", run: insert(1, 0)},
		{name: "batch_no zero", run: insert(0, 10), want: "recipient_batch_batch_no_check"},
		{name: "batch_no negative", run: insert(-1, 10), want: "recipient_batch_batch_no_check"},
		{name: "recipients negative", run: insert(1, -1), want: "recipient_batch_recipients_check"},
		{name: "duplicate batch", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, insert(2, 5)(t, ctx, f))
			return insert(2, 6)(t, ctx, f)
		}, want: "recipient_batch_pkey"},
		{name: "unknown campaign", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.exec(ctx, "INSERT INTO recipient_batch (campaign_id, batch_no, recipients) VALUES ($1, 1, 1)", uuid.New())
		}, want: "recipient_batch_campaign_id_fkey"},
	})
}

func TestConstraints_Delivery(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Minute)
	runConstraintCases(t, []constraintCase{
		{name: "valid pending", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{}))
		}},
		{name: "defaults", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.delivery(ctx, pii(deliveryRow{customer: "CUS-1"})))
			ds, err := f.q.ListPendingDeliveries(ctx, store.ListPendingDeliveriesParams{ScheduleID: f.schedule, Limit: 10})
			if err != nil {
				return err
			}
			if len(ds) != 1 || ds[0].Status != "pending" || ds[0].Attempts != 0 || ds[0].LeaseUntil.Valid ||
				ds[0].ProviderMessageID.Valid || ds[0].LastError.Valid || ds[0].SentAt.Valid || !ds[0].UpdatedAt.Valid {
				t.Errorf("defaults = %+v, want pending, 0 attempts, nothing else set", ds)
			}
			if id := ds[0].ID.String(); id[14] != '7' {
				t.Errorf("delivery id = %s, want a UUID version 7", id)
			}
			return nil
		}},
		{name: "email channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{channel: "email"}))
		}},
		{name: "unknown channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{channel: "push"}))
		}, want: "delivery_channel_check"},
		{name: "unknown status", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{status: "queued"}))
		}, want: "delivery_status_check"},
		{name: "attempts negative", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{attempts: -1}))
		}, want: "delivery_attempts_check"},
		{name: "attempts positive", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{attempts: 5}))
		}},
		// sent => sent_at
		{name: "sent with sent_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{status: "sent", sentAt: now}))
		}},
		{name: "sent without sent_at", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{status: "sent"}))
		}, want: "delivery_sent_at_check"},
		// failed => last_error
		{name: "failed with last_error", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{status: "failed", lastErr: "provider 500"}))
		}},
		{name: "failed without last_error", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{status: "failed"}))
		}, want: "delivery_last_error_check"},
		// sending => lease_until
		{name: "sending with lease_until", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{status: "sending", lease: later}))
		}},
		{name: "sending without lease_until", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{status: "sending"}))
		}, want: "delivery_lease_until_check"},
		// PII: required while pending or sending, optional once final.
		{name: "pending without contact", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{first: "Ayse"})
		}, want: "delivery_pii_check"},
		{name: "pending without first_name", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{contact: "+905550000000"})
		}, want: "delivery_pii_check"},
		{name: "pending without either", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{})
		}, want: "delivery_pii_check"},
		{name: "sending without contact", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{status: "sending", lease: later, first: "Ayse"})
		}, want: "delivery_pii_check"},
		{name: "sent with NULL contact and first_name", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{status: "sent", sentAt: now})
		}},
		{name: "failed with NULL personal data", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{status: "failed", lastErr: "boom"})
		}},
		{name: "skipped_no_consent with NULL personal data", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{status: "skipped_no_consent"})
		}},
		{name: "cancelled with NULL personal data", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, deliveryRow{status: "cancelled"})
		}},
		// dedup key (D-023)
		{name: "same customer, other channel", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.delivery(ctx, pii(deliveryRow{customer: "CUS-1", channel: "sms"})))
			return f.delivery(ctx, pii(deliveryRow{customer: "CUS-1", channel: "email"}))
		}},
		{name: "same customer and channel", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.delivery(ctx, pii(deliveryRow{customer: "CUS-1"})))
			return f.delivery(ctx, pii(deliveryRow{customer: "CUS-1"}))
		}, want: "delivery_campaign_customer_channel_key"},
		{name: "same customer in another campaign", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.delivery(ctx, pii(deliveryRow{customer: "CUS-1"})))
			other, err := f.q.CreateSendCampaign(ctx, store.CreateSendCampaignParams{ID: uuid.New(), CtaUrl: "https://a", BatchCount: 1})
			mustExec(t, err)
			sch, err := f.q.CreateLocaleSchedule(ctx, store.CreateLocaleScheduleParams{CampaignID: other.ID, Language: "tr", Timezone: "Europe/Istanbul"})
			mustExec(t, err)
			msg, err := f.q.CreateMessage(ctx, store.CreateMessageParams{CampaignID: other.ID, Language: "tr", Channel: "sms", Content: []byte(`{}`)})
			mustExec(t, err)
			return f.delivery(ctx, pii(deliveryRow{customer: "CUS-1", campaign: other.ID, schedule: sch.ID, message: msg.ID}))
		}},
		// foreign keys
		{name: "unknown campaign", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{campaign: uuid.New()}))
		}, want: "delivery_campaign_id_fkey"},
		{name: "unknown schedule", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{schedule: uuid.New()}))
		}, want: "delivery_schedule_campaign_fkey"},
		{name: "unknown message", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{message: uuid.New()}))
		}, want: "delivery_message_campaign_channel_fkey"},
		{name: "schedule of another campaign", run: func(t *testing.T, ctx context.Context, f fixture) error {
			o := f.otherCampaign(t, ctx)
			return f.delivery(ctx, pii(deliveryRow{schedule: o.schedule}))
		}, want: "delivery_schedule_campaign_fkey"},
		{name: "message of another campaign", run: func(t *testing.T, ctx context.Context, f fixture) error {
			o := f.otherCampaign(t, ctx)
			return f.delivery(ctx, pii(deliveryRow{message: o.message}))
		}, want: "delivery_message_campaign_channel_fkey"},
		{name: "message channel differs from delivery channel", run: func(_ *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{channel: "email", message: f.message}))
		}, want: "delivery_message_campaign_channel_fkey"},
		{name: "message channel matches delivery channel", run: func(t *testing.T, ctx context.Context, f fixture) error {
			return f.delivery(ctx, pii(deliveryRow{channel: "email", message: f.email}))
		}},
		{name: "cannot delete a referenced schedule", run: func(t *testing.T, ctx context.Context, f fixture) error {
			mustExec(t, f.delivery(ctx, pii(deliveryRow{})))
			return f.exec(ctx, "DELETE FROM locale_schedule WHERE id = $1", f.schedule)
		}, want: "delivery_schedule_campaign_fkey"},
	})
}

// TestConstraints_NotNull covers the NOT NULL columns; a NOT NULL violation names no constraint.
func TestConstraints_NotNull(t *testing.T) {
	pool := newMigratedPool(t)
	f := newFixture(t, pool)
	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{"send_campaign.id", "INSERT INTO send_campaign (cta_url, batch_count) VALUES ('https://a', 1)", nil},
		{"message.content", "INSERT INTO message (campaign_id, language, channel, content) VALUES ($1, 'en', 'sms', NULL)", []any{f.campaign}},
		{"locale_schedule.timezone", "INSERT INTO locale_schedule (campaign_id, language, timezone) VALUES ($1, 'en', NULL)", []any{f.campaign}},
		{"delivery.customer_id", "INSERT INTO delivery (campaign_id, schedule_id, message_id, channel, first_name, contact) VALUES ($1, $2, $3, 'sms', 'a', 'b')",
			[]any{f.campaign, f.schedule, f.message}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(t.Context(), tt.sql, tt.args...)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != pgNotNullViolation {
				t.Errorf("error = %v, want SQLSTATE %s", err, pgNotNullViolation)
			}
		})
	}
}

func TestQueries(t *testing.T) {
	ctx := t.Context()
	pool := newMigratedPool(t)
	q := store.New(pool)

	campID := uuid.New()
	var sched, schedEN store.LocaleSchedule
	var msgSMS, msgEmail store.Message

	t.Run("send campaign round trip", func(t *testing.T) {
		c, err := q.CreateSendCampaign(ctx, store.CreateSendCampaignParams{ID: campID, CtaUrl: "https://shop.example/cart?c=1", BatchCount: 2})
		mustExec(t, err)
		if c.ID != campID || c.Status != "scheduled" || c.CtaUrl != "https://shop.example/cart?c=1" || c.BatchCount != 2 ||
			!c.CreatedAt.Valid || !c.UpdatedAt.Valid || c.CompletedAt.Valid {
			t.Errorf("CreateSendCampaign() = %+v", c)
		}
		got, err := q.GetSendCampaign(ctx, campID)
		if err != nil {
			t.Fatalf("GetSendCampaign() error = %v", err)
		}
		if got != c {
			t.Errorf("GetSendCampaign() = %+v, want %+v", got, c)
		}
		if _, err := q.GetSendCampaign(ctx, uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("GetSendCampaign(unknown) error = %v, want pgx.ErrNoRows", err)
		}
		if _, err := q.CreateSendCampaign(ctx, store.CreateSendCampaignParams{ID: campID, CtaUrl: "https://x", BatchCount: 1}); err == nil {
			t.Error("CreateSendCampaign(duplicate id) error = nil, want unique violation")
		}
	})

	t.Run("locale schedules", func(t *testing.T) {
		at := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
		var err error
		sched, err = q.CreateLocaleSchedule(ctx, store.CreateLocaleScheduleParams{CampaignID: campID, Language: "tr", Timezone: "Europe/Istanbul"})
		mustExec(t, err)
		schedEN, err = q.CreateLocaleSchedule(ctx, store.CreateLocaleScheduleParams{
			CampaignID: campID, Language: "en", Timezone: "Europe/London", SendAt: pgtype.Timestamptz{Time: at, Valid: true},
		})
		mustExec(t, err)
		if sched.Status != "scheduled" || sched.SendAt.Valid || sched.FiredAt.Valid || sched.ID.String()[14] != '7' {
			t.Errorf("CreateLocaleSchedule(now) = %+v, want scheduled, NULL send_at, UUIDv7", sched)
		}
		if !schedEN.SendAt.Valid || !schedEN.SendAt.Time.Equal(at) {
			t.Errorf("send_at = %v, want %v", schedEN.SendAt, at)
		}
		_, err = q.CreateLocaleSchedule(ctx, store.CreateLocaleScheduleParams{CampaignID: uuid.New(), Language: "en", Timezone: "UTC"})
		if err == nil {
			t.Error("CreateLocaleSchedule(unknown campaign) error = nil, want FK violation")
		}
		list, err := q.ListLocaleSchedules(ctx, campID)
		if err != nil {
			t.Fatalf("ListLocaleSchedules() error = %v", err)
		}
		if len(list) != 2 || list[0].ID != schedEN.ID || list[1].ID != sched.ID {
			t.Errorf("ListLocaleSchedules() = %+v, want [en, tr]", list)
		}
		if list, err := q.ListLocaleSchedules(ctx, uuid.New()); err != nil || len(list) != 0 {
			t.Errorf("ListLocaleSchedules(unknown) = %v, %v, want empty", list, err)
		}
	})

	t.Run("messages", func(t *testing.T) {
		var err error
		msgSMS, err = q.CreateMessage(ctx, store.CreateMessageParams{CampaignID: campID, Language: "tr", Channel: "sms", Content: []byte(`{"text":"Merhaba {{first_name}}"}`)})
		mustExec(t, err)
		msgEmail, err = q.CreateMessage(ctx, store.CreateMessageParams{CampaignID: campID, Language: "tr", Channel: "email", Content: []byte(`{"subject":"Sepetin seni bekliyor"}`)})
		mustExec(t, err)
		if msgSMS.ID.String()[14] != '7' || msgSMS.Language != "tr" || msgSMS.Channel != "sms" {
			t.Errorf("CreateMessage() = %+v", msgSMS)
		}
		list, err := q.ListMessages(ctx, campID)
		if err != nil {
			t.Fatalf("ListMessages() error = %v", err)
		}
		if len(list) != 2 || list[0].ID != msgEmail.ID || list[1].ID != msgSMS.ID {
			t.Fatalf("ListMessages() = %+v, want [email, sms]", list)
		}
		var content map[string]string
		if err := json.Unmarshal(list[1].Content, &content); err != nil || content["text"] != "Merhaba {{first_name}}" {
			t.Errorf("sms content = %s (%v), want the text round-tripped", list[1].Content, err)
		}
		if _, err := q.CreateMessage(ctx, store.CreateMessageParams{CampaignID: campID, Language: "tr", Channel: "sms", Content: []byte(`{}`)}); err == nil {
			t.Error("CreateMessage(duplicate slot) error = nil, want unique violation")
		}
	})

	t.Run("recipient batches", func(t *testing.T) {
		if n, err := q.CountRecipientBatches(ctx, campID); err != nil || n != 0 {
			t.Fatalf("CountRecipientBatches(none) = %d, %v, want 0", n, err)
		}
		first := store.InsertRecipientBatchParams{CampaignID: campID, BatchNo: 1, Recipients: 500}
		if n, err := q.InsertRecipientBatch(ctx, first); err != nil || n != 1 {
			t.Fatalf("InsertRecipientBatch(new) = %d, %v, want 1 row", n, err)
		}
		// A redelivered batch, even with a different count, is ignored and does not change the first.
		again := first
		again.Recipients = 7
		if n, err := q.InsertRecipientBatch(ctx, again); err != nil || n != 0 {
			t.Errorf("InsertRecipientBatch(duplicate) = %d, %v, want 0 rows", n, err)
		}
		if n, err := q.InsertRecipientBatch(ctx, store.InsertRecipientBatchParams{CampaignID: campID, BatchNo: 2, Recipients: 120}); err != nil || n != 1 {
			t.Fatalf("InsertRecipientBatch(batch 2) = %d, %v, want 1 row", n, err)
		}
		if n, err := q.CountRecipientBatches(ctx, campID); err != nil || n != 2 {
			t.Errorf("CountRecipientBatches() = %d, %v, want 2", n, err)
		}
		var kept int32
		if err := pool.QueryRow(ctx, "SELECT recipients FROM recipient_batch WHERE campaign_id = $1 AND batch_no = 1", campID).Scan(&kept); err != nil || kept != 500 {
			t.Errorf("recipients of batch 1 = %d, %v, want the first write (500)", kept, err)
		}
		if _, err := q.InsertRecipientBatch(ctx, store.InsertRecipientBatchParams{CampaignID: uuid.New(), BatchNo: 1, Recipients: 1}); err == nil {
			t.Error("InsertRecipientBatch(unknown campaign) error = nil, want FK violation")
		}
	})

	deliver := func(customer, channel string, schedule, message uuid.UUID) store.InsertDeliveryParams {
		return store.InsertDeliveryParams{
			CampaignID: campID, ScheduleID: schedule, MessageID: message, CustomerID: customer, Channel: channel,
			FirstName: pgtype.Text{String: "Ayse", Valid: true}, Contact: pgtype.Text{String: "ayse@example.com", Valid: true},
		}
	}

	t.Run("deliveries dedup on campaign, customer, channel", func(t *testing.T) {
		for _, p := range []store.InsertDeliveryParams{
			deliver("CUS-1", "sms", sched.ID, msgSMS.ID),
			deliver("CUS-2", "sms", sched.ID, msgSMS.ID),
			deliver("CUS-1", "email", sched.ID, msgEmail.ID),
		} {
			if n, err := q.InsertDelivery(ctx, p); err != nil || n != 1 {
				t.Fatalf("InsertDelivery(%s/%s) = %d, %v, want 1 row", p.CustomerID, p.Channel, n, err)
			}
		}
		// The same recipient again, from a redelivered batch, even with other text and slot, is a no-op.
		dup := deliver("CUS-1", "sms", schedEN.ID, msgSMS.ID)
		dup.FirstName = pgtype.Text{String: "Changed", Valid: true}
		if n, err := q.InsertDelivery(ctx, dup); err != nil || n != 0 {
			t.Errorf("InsertDelivery(duplicate) = %d, %v, want 0 rows and no error", n, err)
		}
		var total int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM delivery WHERE campaign_id = $1", campID).Scan(&total); err != nil || total != 3 {
			t.Errorf("delivery rows = %d, %v, want 3", total, err)
		}
		var name string
		if err := pool.QueryRow(ctx, "SELECT first_name FROM delivery WHERE customer_id = 'CUS-1' AND channel = 'sms'").Scan(&name); err != nil || name != "Ayse" {
			t.Errorf("first_name = %q, %v, want the first write (Ayse)", name, err)
		}
		bad := deliver("CUS-9", "sms", uuid.New(), msgSMS.ID)
		if _, err := q.InsertDelivery(ctx, bad); err == nil {
			t.Error("InsertDelivery(unknown schedule) error = nil, want FK violation")
		}
		bad = deliver("CUS-9", "sms", sched.ID, uuid.New())
		if _, err := q.InsertDelivery(ctx, bad); err == nil {
			t.Error("InsertDelivery(unknown message) error = nil, want FK violation")
		}
		noPII := deliver("CUS-9", "sms", sched.ID, msgSMS.ID)
		noPII.Contact = pgtype.Text{}
		if _, err := q.InsertDelivery(ctx, noPII); err == nil {
			t.Error("InsertDelivery(pending without contact) error = nil, want delivery_pii_check")
		}
	})

	t.Run("list pending deliveries", func(t *testing.T) {
		ds, err := q.ListPendingDeliveries(ctx, store.ListPendingDeliveriesParams{ScheduleID: sched.ID, Limit: 10})
		if err != nil {
			t.Fatalf("ListPendingDeliveries() error = %v", err)
		}
		if len(ds) != 3 {
			t.Fatalf("ListPendingDeliveries() = %d rows, want 3", len(ds))
		}
		for i := 1; i < len(ds); i++ {
			if ds[i-1].ID.String() >= ds[i].ID.String() {
				t.Errorf("rows not ordered by id: %s then %s", ds[i-1].ID, ds[i].ID)
			}
		}
		if ds[0].Status != "pending" || ds[0].FirstName.String != "Ayse" || ds[0].Contact.String != "ayse@example.com" {
			t.Errorf("first row = %+v, want pending with personal data", ds[0])
		}
		limited, err := q.ListPendingDeliveries(ctx, store.ListPendingDeliveriesParams{ScheduleID: sched.ID, Limit: 2})
		if err != nil || len(limited) != 2 || limited[0].ID != ds[0].ID {
			t.Errorf("ListPendingDeliveries(limit 2) = %d rows, %v, want the first 2", len(limited), err)
		}
		other, err := q.ListPendingDeliveries(ctx, store.ListPendingDeliveriesParams{ScheduleID: schedEN.ID, Limit: 10})
		if err != nil || len(other) != 0 {
			t.Errorf("ListPendingDeliveries(other schedule) = %d rows, %v, want none", len(other), err)
		}
		// A delivery that left pending is not listed.
		mustExec(t, execErr(pool.Exec(ctx, "UPDATE delivery SET status = 'cancelled' WHERE id = $1", ds[0].ID)))
		after, err := q.ListPendingDeliveries(ctx, store.ListPendingDeliveriesParams{ScheduleID: sched.ID, Limit: 10})
		if err != nil || len(after) != 2 {
			t.Errorf("ListPendingDeliveries() after cancelling one = %d rows, %v, want 2", len(after), err)
		}
	})

	t.Run("count deliveries by status", func(t *testing.T) {
		mustExec(t, execErr(pool.Exec(ctx, "UPDATE delivery SET status = 'sent', sent_at = now() WHERE customer_id = 'CUS-2' AND channel = 'sms'")))
		rows, err := q.CountDeliveriesByStatus(ctx, campID)
		if err != nil {
			t.Fatalf("CountDeliveriesByStatus() error = %v", err)
		}
		got := map[string]int64{}
		for _, r := range rows {
			got[r.Channel+"/"+r.Status] = r.Total
		}
		want := map[string]int64{"email/pending": 1, "sms/cancelled": 1, "sms/sent": 1}
		if !maps.Equal(got, want) {
			t.Errorf("CountDeliveriesByStatus() = %v, want %v", got, want)
		}
		if rows, err := q.CountDeliveriesByStatus(ctx, uuid.New()); err != nil || len(rows) != 0 {
			t.Errorf("CountDeliveriesByStatus(unknown) = %v, %v, want empty", rows, err)
		}
	})

	t.Run("clear personal data", func(t *testing.T) {
		// The email delivery is still pending: clearing is refused and nothing changes.
		if _, err := q.ClearDeliveryPII(ctx, campID); err == nil {
			t.Fatal("ClearDeliveryPII() with a pending delivery error = nil, want delivery_pii_check")
		} else {
			wantConstraint(t, err, "delivery_pii_check")
		}
		var kept int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM delivery WHERE campaign_id = $1 AND contact IS NOT NULL", campID).Scan(&kept); err != nil || kept != 3 {
			t.Fatalf("rows with contact after refused clear = %d, %v, want 3 (statement rolled back)", kept, err)
		}

		// Finish the campaign: every delivery final, then clear in the same transaction.
		mustExec(t, execErr(pool.Exec(ctx, "UPDATE delivery SET status = 'failed', last_error = 'boom' WHERE status = 'pending'")))
		tx, err := pool.Begin(ctx)
		mustExec(t, err)
		defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
		n, err := q.WithTx(tx).ClearDeliveryPII(ctx, campID)
		if err != nil || n != 3 {
			t.Fatalf("ClearDeliveryPII() = %d, %v, want 3 rows", n, err)
		}
		mustExec(t, tx.Commit(ctx))

		var withPII int
		var statuses []string
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM delivery WHERE first_name IS NOT NULL OR contact IS NOT NULL").Scan(&withPII); err != nil || withPII != 0 {
			t.Errorf("rows with personal data = %d, %v, want 0", withPII, err)
		}
		statuses = queryStrings(t, pool, "SELECT status || '/' || customer_id FROM delivery ORDER BY 1")
		if want := []string{"cancelled/CUS-1", "failed/CUS-1", "sent/CUS-2"}; !slices.Equal(statuses, want) {
			t.Errorf("statuses and customer_ids after clear = %v, want %v (kept)", statuses, want)
		}
		// Second run touches nothing.
		if n, err := q.ClearDeliveryPII(ctx, campID); err != nil || n != 0 {
			t.Errorf("ClearDeliveryPII() again = %d, %v, want 0 rows", n, err)
		}
		// Another campaign is untouched by the clear above (nothing to clear there either).
		if n, err := q.ClearDeliveryPII(ctx, uuid.New()); err != nil || n != 0 {
			t.Errorf("ClearDeliveryPII(unknown) = %d, %v, want 0 rows", n, err)
		}
	})
}

// execErr drops the command tag of Exec so it can feed mustExec.
func execErr(_ pgconn.CommandTag, err error) error { return err }
