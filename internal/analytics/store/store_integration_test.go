//go:build integration

package store_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/furknozg/coldbru/internal/analytics/atest"
	"github.com/furknozg/coldbru/internal/analytics/store"
	"github.com/furknozg/coldbru/internal/platform/migrate"
)

var pg *atest.Postgres

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	var err error
	pg, err = atest.Start(ctx)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	pg.Close()
	os.Exit(code)
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
	pool := pg.NewMigratedPool(t)
	for _, tt := range []struct {
		name string
		sql  string
		want []string
	}{
		{"tables", "SELECT tablename FROM pg_tables WHERE schemaname = 'analytics' ORDER BY tablename",
			[]string{"campaign_delivery_count", "campaign_summary", "goose_db_version", "goose_platform_version", "llm_usage", "outbox", "processed_event"}},
		{"nothing in public", "SELECT tablename FROM pg_tables WHERE schemaname = 'public'", nil},
		{"service versions", "SELECT version_id::text FROM goose_db_version WHERE version_id > 0 ORDER BY version_id", []string{"1", "2"}},
		{"llm_usage indexes", "SELECT indexname FROM pg_indexes WHERE schemaname = 'analytics' AND tablename = 'llm_usage' ORDER BY indexname",
			[]string{"llm_usage_campaign_id_idx", "llm_usage_created_at_idx", "llm_usage_pkey", "llm_usage_prompt_name_version_idx"}},
		{"campaign_summary indexes", "SELECT indexname FROM pg_indexes WHERE schemaname = 'analytics' AND tablename = 'campaign_summary' ORDER BY indexname",
			[]string{"campaign_summary_completed_at_idx", "campaign_summary_pkey"}},
		{"index columns", `SELECT indexdef FROM pg_indexes WHERE schemaname = 'analytics' AND indexname IN ('llm_usage_campaign_id_idx', 'llm_usage_prompt_name_version_idx', 'llm_usage_created_at_idx', 'campaign_summary_completed_at_idx') ORDER BY indexname`,
			[]string{
				"CREATE INDEX campaign_summary_completed_at_idx ON analytics.campaign_summary USING btree (completed_at)",
				"CREATE INDEX llm_usage_campaign_id_idx ON analytics.llm_usage USING btree (campaign_id)",
				"CREATE INDEX llm_usage_created_at_idx ON analytics.llm_usage USING btree (created_at)",
				"CREATE INDEX llm_usage_prompt_name_version_idx ON analytics.llm_usage USING btree (prompt_name, prompt_version)",
			}},
		{"no foreign keys", "SELECT conname FROM pg_constraint WHERE connamespace = 'analytics'::regnamespace AND contype = 'f'", nil},
		{"campaign_summary has no llm cost column", `SELECT column_name FROM information_schema.columns
			WHERE table_schema = 'analytics' AND table_name = 'campaign_summary' ORDER BY ordinal_position`,
			[]string{"campaign_id", "completed_at", "sms_segments", "delivery_cost_usd", "updated_at"}},
		{"money columns", `SELECT table_name || '.' || column_name || ' ' || data_type || '(' || numeric_precision || ',' || numeric_scale || ')'
			FROM information_schema.columns WHERE table_schema = 'analytics' AND data_type = 'numeric' ORDER BY 1`,
			[]string{"campaign_summary.delivery_cost_usd numeric(14,6)", "llm_usage.cost_usd numeric(14,8)"}},
		{"delivery count key", `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'campaign_delivery_count_pkey'`,
			[]string{"PRIMARY KEY (campaign_id, channel, status)"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryStrings(t, pool, tt.sql); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("second Up applies nothing", func(t *testing.T) {
		var buf strings.Builder
		if err := migrate.Up(t.Context(), pool, store.Migrations(), slog.New(slog.NewTextHandler(&buf, nil))); err != nil {
			t.Fatal(err)
		}
		if buf.Len() != 0 {
			t.Errorf("second Up() logged %q", buf.String())
		}
	})
}

func TestMigrations_DownThenUp(t *testing.T) {
	pool := pg.NewMigratedPool(t)
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, store.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	// Down of 00002 drops only the time indexes (D-097); the tables and the older indexes stay.
	if _, err := provider.DownTo(t.Context(), 1); err != nil {
		t.Fatalf("DownTo(1) error = %v", err)
	}
	if got := queryStrings(t, pool, "SELECT indexname FROM pg_indexes WHERE schemaname = 'analytics' AND indexname IN ('llm_usage_created_at_idx', 'campaign_summary_completed_at_idx')"); len(got) != 0 {
		t.Errorf("time indexes after Down of 00002 = %v", got)
	}
	if got := queryStrings(t, pool, "SELECT indexname FROM pg_indexes WHERE schemaname = 'analytics' AND tablename = 'llm_usage' ORDER BY indexname"); !slices.Equal(got, []string{"llm_usage_campaign_id_idx", "llm_usage_pkey", "llm_usage_prompt_name_version_idx"}) {
		t.Errorf("llm_usage indexes after Down of 00002 = %v", got)
	}
	if _, err := provider.DownTo(t.Context(), 0); err != nil {
		t.Fatalf("DownTo(0) error = %v", err)
	}
	left := queryStrings(t, pool, "SELECT tablename FROM pg_tables WHERE schemaname = 'analytics' ORDER BY tablename")
	if want := []string{"goose_db_version", "goose_platform_version", "outbox", "processed_event"}; !slices.Equal(left, want) {
		t.Errorf("tables after Down = %v, want %v", left, want)
	}
	if got := queryStrings(t, pool, "SELECT indexname FROM pg_indexes WHERE schemaname = 'analytics' AND indexname LIKE 'llm\\_usage%'"); len(got) != 0 {
		t.Errorf("indexes after Down = %v", got)
	}
	if err := migrate.Up(t.Context(), pool, store.Migrations(), atest.Discard()); err != nil {
		t.Fatalf("Up() after Down error = %v", err)
	}
}

func num(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatal(err)
	}
	return n
}

func usage(mutate func(*store.InsertLLMUsageParams)) store.InsertLLMUsageParams {
	p := store.InsertLLMUsageParams{
		LlmCallID: uuid.New(), CampaignID: uuid.New(), Purpose: "content", Model: "claude-sonnet-5-5",
		PromptName: "content", PromptVersion: "1.0.0", InputTokens: 10, OutputTokens: 5, CacheReadTokens: 2, CacheCreationTokens: 1,
		Outcome: "accepted", LatencyMs: 300, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

func TestConstraints(t *testing.T) {
	ctx := t.Context()
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	at := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	summary := func(mutate func(*store.UpsertCampaignSummaryParams)) func() error {
		return func() error {
			p := store.UpsertCampaignSummaryParams{CampaignID: uuid.New(), CompletedAt: at, SmsSegments: 3, DeliveryCostUsd: num(t, "1.5")}
			if mutate != nil {
				mutate(&p)
			}
			return q.UpsertCampaignSummary(ctx, p)
		}
	}
	count := func(mutate func(*store.InsertDeliveryCountParams)) func() error {
		return func() error {
			p := store.InsertDeliveryCountParams{CampaignID: uuid.New(), Channel: "sms", Status: "sent", Count: 1}
			if mutate != nil {
				mutate(&p)
			}
			return q.InsertDeliveryCount(ctx, p)
		}
	}
	llm := func(mutate func(*store.InsertLLMUsageParams)) func() error {
		return func() error {
			p := usage(mutate)
			p.CostUsd = num(t, "0.001")
			if mutate != nil {
				mutate(&p)
			}
			_, err := q.InsertLLMUsage(ctx, p)
			return err
		}
	}

	tests := []struct {
		name string
		run  func() error
		want string // constraint name; empty means success
	}{
		{"summary ok", summary(nil), ""},
		{"summary negative segments", summary(func(p *store.UpsertCampaignSummaryParams) { p.SmsSegments = -1 }), "campaign_summary_sms_segments_check"},
		{"summary negative cost", summary(func(p *store.UpsertCampaignSummaryParams) { p.DeliveryCostUsd = num(t, "-0.01") }), "campaign_summary_delivery_cost_usd_check"},
		{"summary cost overflows numeric(14,6)", summary(func(p *store.UpsertCampaignSummaryParams) { p.DeliveryCostUsd = num(t, "123456789.0") }), "numeric field overflow"},
		{"count ok", count(nil), ""},
		{"count email pending ok", count(func(p *store.InsertDeliveryCountParams) { p.Channel, p.Status = "email", "pending" }), ""},
		{"count unknown channel", count(func(p *store.InsertDeliveryCountParams) { p.Channel = "push" }), "campaign_delivery_count_channel_check"},
		{"count unknown status", count(func(p *store.InsertDeliveryCountParams) { p.Status = "delivered" }), "campaign_delivery_count_status_check"},
		{"count negative", count(func(p *store.InsertDeliveryCountParams) { p.Count = -1 }), "campaign_delivery_count_count_check"},
		{"count zero ok", count(func(p *store.InsertDeliveryCountParams) { p.Count = 0 }), ""},
		{"usage ok", llm(nil), ""},
		{"usage every purpose ok", func() error {
			for _, pu := range []string{"screen", "segment", "content", "channel_plan", "schedule"} {
				if err := llm(func(p *store.InsertLLMUsageParams) { p.Purpose = pu })(); err != nil {
					return err
				}
			}
			return nil
		}, ""},
		{"usage every outcome ok", func() error {
			for _, o := range []string{"accepted", "validation_failed", "refusal", "api_error"} {
				if err := llm(func(p *store.InsertLLMUsageParams) { p.Outcome = o })(); err != nil {
					return err
				}
			}
			return nil
		}, ""},
		{"usage unknown purpose", llm(func(p *store.InsertLLMUsageParams) { p.Purpose = "translate" }), "llm_usage_purpose_check"},
		{"usage unknown outcome", llm(func(p *store.InsertLLMUsageParams) { p.Outcome = "timeout" }), "llm_usage_outcome_check"},
		{"usage blank model", llm(func(p *store.InsertLLMUsageParams) { p.Model = " " }), "llm_usage_model_check"},
		{"usage blank prompt name", llm(func(p *store.InsertLLMUsageParams) { p.PromptName = "" }), "llm_usage_prompt_name_check"},
		{"usage blank prompt version", llm(func(p *store.InsertLLMUsageParams) { p.PromptVersion = "\n" }), "llm_usage_prompt_version_check"},
		{"usage negative input", llm(func(p *store.InsertLLMUsageParams) { p.InputTokens = -1 }), "llm_usage_input_tokens_check"},
		{"usage negative output", llm(func(p *store.InsertLLMUsageParams) { p.OutputTokens = -1 }), "llm_usage_output_tokens_check"},
		{"usage negative cache read", llm(func(p *store.InsertLLMUsageParams) { p.CacheReadTokens = -1 }), "llm_usage_cache_read_tokens_check"},
		{"usage negative cache creation", llm(func(p *store.InsertLLMUsageParams) { p.CacheCreationTokens = -1 }), "llm_usage_cache_creation_tokens_check"},
		{"usage negative latency", llm(func(p *store.InsertLLMUsageParams) { p.LatencyMs = -1 }), "llm_usage_latency_ms_check"},
		{"usage zero latency ok", llm(func(p *store.InsertLLMUsageParams) { p.LatencyMs = 0 }), ""},
		{"usage negative cost", llm(func(p *store.InsertLLMUsageParams) { p.CostUsd = num(t, "-0.00000001") }), "llm_usage_cost_usd_check"},
		{"usage for a campaign that never completes ok", llm(func(p *store.InsertLLMUsageParams) { p.CampaignID = uuid.New() }), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("error = %v, want success", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	ctx := t.Context()
	q := store.New(pg.NewMigratedPool(t))

	t.Run("llm_call_id is unique, a repeat inserts nothing", func(t *testing.T) {
		p := usage(nil)
		p.CostUsd = num(t, "0.5")
		n, err := q.InsertLLMUsage(ctx, p)
		if err != nil || n != 1 {
			t.Fatalf("first insert = %d, %v", n, err)
		}
		p.CostUsd = num(t, "9")
		p.InputTokens = 999
		if n, err = q.InsertLLMUsage(ctx, p); err != nil || n != 0 {
			t.Fatalf("repeat insert = %d, %v, want 0 rows", n, err)
		}
		tot, err := q.LLMTotalsOfCampaign(ctx, p.CampaignID)
		if err != nil || tot.Calls != 1 || tot.CostUsd != "0.50000000" || tot.InputTokens != 10 {
			t.Errorf("totals = %+v, %v: the first row must stand", tot, err)
		}
	})

	t.Run("delivery count key", func(t *testing.T) {
		id := uuid.New()
		p := store.InsertDeliveryCountParams{CampaignID: id, Channel: "sms", Status: "sent", Count: 1}
		if err := q.InsertDeliveryCount(ctx, p); err != nil {
			t.Fatal(err)
		}
		if err := q.InsertDeliveryCount(ctx, p); err == nil || !strings.Contains(err.Error(), "campaign_delivery_count_pkey") {
			t.Errorf("duplicate (campaign, channel, status): error = %v", err)
		}
		p.Channel = "email"
		if err := q.InsertDeliveryCount(ctx, p); err != nil {
			t.Errorf("same status on another channel: %v", err)
		}
	})

	t.Run("a repeated summary replaces the first", func(t *testing.T) {
		id := uuid.New()
		at := pgtype.Timestamptz{Time: time.Now().UTC().Truncate(time.Microsecond), Valid: true}
		for _, cost := range []string{"1.0", "2.5"} {
			if err := q.UpsertCampaignSummary(ctx, store.UpsertCampaignSummaryParams{CampaignID: id, CompletedAt: at, SmsSegments: 4, DeliveryCostUsd: num(t, cost)}); err != nil {
				t.Fatal(err)
			}
		}
		got, err := q.GetCampaignSummary(ctx, id)
		if err != nil || got.DeliveryCostUsd != "2.500000" || got.SmsSegments != 4 {
			t.Errorf("summary = %+v, %v", got, err)
		}
	})
}
