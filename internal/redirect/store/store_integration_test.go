//go:build integration

package store_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/redirect/rtest"
	"github.com/furknozg/coldbru/internal/redirect/store"
)

var pg *rtest.Postgres

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	var err error
	pg, err = rtest.Start(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	pg.Close()
	os.Exit(code)
}

func queryStrings(t *testing.T, pool *pgxpool.Pool, sql string) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(), sql)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestMigrations(t *testing.T) {
	pool := pg.NewMigratedPool(t)
	for _, tt := range []struct {
		name, sql string
		want      []string
	}{
		{"tables", "SELECT tablename FROM pg_tables WHERE schemaname = 'redirect' ORDER BY tablename",
			[]string{"click", "goose_db_version", "goose_platform_version", "link", "outbox", "processed_event"}},
		{"nothing in public", "SELECT tablename FROM pg_tables WHERE schemaname = 'public'", nil},
		{"click indexes", "SELECT indexname FROM pg_indexes WHERE schemaname = 'redirect' AND tablename = 'click' ORDER BY indexname",
			[]string{"click_code_idx", "click_pkey"}},
		{"link keys", "SELECT conname FROM pg_constraint WHERE conrelid = 'redirect.link'::regclass AND contype IN ('p', 'u') ORDER BY conname",
			[]string{"link_delivery_id_key", "link_pkey"}},
		{"foreign keys", "SELECT conname FROM pg_constraint WHERE connamespace = 'redirect'::regnamespace AND contype = 'f'", []string{"click_code_fkey"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryStrings(t, pool, tt.sql); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("down then up", func(t *testing.T) {
		sqlDB := stdlib.OpenDBFromPool(pool)
		defer sqlDB.Close()
		provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, store.Migrations())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := provider.DownTo(t.Context(), 0); err != nil {
			t.Fatalf("DownTo(0) error = %v", err)
		}
		if got := queryStrings(t, pool, "SELECT tablename FROM pg_tables WHERE schemaname = 'redirect' AND tablename IN ('link', 'click')"); len(got) != 0 {
			t.Errorf("tables after Down = %v", got)
		}
		if err := migrate.Up(t.Context(), pool, store.Migrations(), rtest.Discard()); err != nil {
			t.Fatalf("Up() after Down error = %v", err)
		}
	})
}

func TestConstraints(t *testing.T) {
	ctx := t.Context()
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	const code = "abcdef0123"
	insert := func(mutate func(*store.InsertLinkParams)) error {
		p := store.InsertLinkParams{
			Code: code, DeliveryID: uuid.New(), CampaignID: uuid.New(), Channel: "sms", Language: "tr",
			Timezone: "Europe/Istanbul", TargetUrl: "https://shop.example.com/?utm_campaign=c",
		}
		if mutate != nil {
			mutate(&p)
		}
		_, err := pool.Exec(ctx, `INSERT INTO link (code, delivery_id, campaign_id, channel, language, timezone, target_url) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			p.Code, p.DeliveryID, p.CampaignID, p.Channel, p.Language, p.Timezone, p.TargetUrl)
		return err
	}
	tests := []struct {
		name   string
		mutate func(*store.InsertLinkParams)
		want   string
	}{
		{"short code", func(p *store.InsertLinkParams) { p.Code = "abc" }, "link_code_check"},
		{"code with a symbol", func(p *store.InsertLinkParams) { p.Code = "abcdef012-" }, "link_code_check"},
		{"unknown channel", func(p *store.InsertLinkParams) { p.Code = "bbbbbbbbbb"; p.Channel = "push" }, "link_channel_check"},
		{"unknown language", func(p *store.InsertLinkParams) { p.Code = "cccccccccc"; p.Language = "de" }, "link_language_check"},
		{"blank timezone", func(p *store.InsertLinkParams) { p.Code = "dddddddddd"; p.Timezone = " " }, "link_timezone_check"},
		{"relative target", func(p *store.InsertLinkParams) { p.Code = "eeeeeeeeee"; p.TargetUrl = "/shop" }, "link_target_url_check"},
		{"first insert is fine", nil, ""},
		{"code is the primary key", func(p *store.InsertLinkParams) {}, "link_pkey"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := insert(tt.mutate)
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("error = %v, want success", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("error = %v, want %q", err, tt.want)
			}
		})
	}

	t.Run("one link per delivery", func(t *testing.T) {
		d := uuid.New()
		if err := insert(func(p *store.InsertLinkParams) { p.Code = "ffffffffff"; p.DeliveryID = d }); err != nil {
			t.Fatal(err)
		}
		if err := insert(func(p *store.InsertLinkParams) { p.Code = "gggggggggg"; p.DeliveryID = d }); err == nil || !strings.Contains(err.Error(), "link_delivery_id_key") {
			t.Errorf("second link for a delivery: error = %v, want link_delivery_id_key", err)
		}
	})

	t.Run("a click needs its link", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `INSERT INTO click (code, first) VALUES ('nolinkhere0', true)`); err == nil || !strings.Contains(err.Error(), "click_code_fkey") {
			t.Errorf("click without a link: error = %v, want click_code_fkey", err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		qt := q.WithTx(tx)
		if _, err := qt.LockLink(ctx, code); err != nil {
			t.Fatalf("LockLink: %v", err)
		}
		first, err := qt.InsertClick(ctx, code)
		if err != nil || !first.First {
			t.Fatalf("first InsertClick = %+v, %v; want first = true", first, err)
		}
		second, err := qt.InsertClick(ctx, code)
		if err != nil || second.First || second.ID == first.ID {
			t.Fatalf("second InsertClick = %+v, %v; want first = false and another id", second, err)
		}
		if first.ID.Version() != 7 {
			t.Errorf("click id version = %d, want UUIDv7 (D-030)", first.ID.Version())
		}
	})
}
