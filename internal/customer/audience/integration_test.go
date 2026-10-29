//go:build integration

package audience

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
)

// pool is the customer_svc pool of the seeded container shared by the package.
var pool *pgxpool.Pool

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	env, err := pgtest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start seeded postgres: %v\n", err)
		return 1
	}
	defer env.Close()
	pool = env.Pool
	return m.Run()
}

func liveCompiler(t *testing.T) *Compiler {
	t.Helper()
	cities, err := LoadCities(t.Context(), pool)
	if err != nil {
		t.Fatal(err)
	}
	return &Compiler{Cities: cities}
}

func count(t *testing.T, where string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM dwh.dim_customer c WHERE "+where, args...).Scan(&n); err != nil {
		t.Fatalf("WHERE %s: %v", where, err)
	}
	return n
}

// TestEveryGoldenFilterRunsOnPostgres executes the SQL of every golden case, so
// each operator, cast and argument type is accepted by the real server and
// pgx's encoder. The city cases use the live dictionary.
func TestEveryGoldenFilterRunsOnPostgres(t *testing.T) {
	c := liveCompiler(t)
	for _, tt := range goldenCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Compile(tt.filter)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			count(t, got.Where, got.Args...)
		})
	}
}

// TestCompiledFiltersMatchHandWrittenSQL compares the compiler's result with an
// independently written query for operators where the cast or null rules matter.
func TestCompiledFiltersMatchHandWrittenSQL(t *testing.T) {
	c := liveCompiler(t)
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	instant := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		filter *model.CustomerFilter
		sql    string
	}{
		{"int in", parse(t, `{"ageYears":{"in":[18,19,20]}}`), `c.age_years IN (18,19,20)`},
		{"decimal in", parse(t, `{"totalSpendGbp":{"in":["0","10.50"]}}`), `c.total_spend_gbp IN (0, 10.50)`},
		{"decimal gte", parse(t, `{"totalSpendGbp":{"gte":"1000.5"}}`), `c.total_spend_gbp >= 1000.5`},
		{"date ago", parse(t, `{"signupDate":{"gte":{"ago":"90d"}}}`), `c.signup_date >= (now() - interval '90 days')::date`},
		{"datetime ago hours", parse(t, `{"lastCampaignSentAt":{"gte":{"ago":"72h"}}}`), `c.last_campaign_sent_at >= now() - interval '72 hours'`},
		{"datetime date", &model.CustomerFilter{LastCampaignSentAt: &model.DateTimeFilter{Lt: &model.TimeValue{Date: &model.Date{Time: day}}}},
			`c.last_campaign_sent_at < '2026-01-01'::date::timestamptz`},
		{"date at", &model.CustomerFilter{SignupDate: &model.DateFilter{Lte: &model.TimeValue{At: &instant}}},
			`c.signup_date <= '2026-06-01T12:00:00Z'::timestamptz::date`},
		{"bool ne", parse(t, `{"retired":{"ne":true}}`), `c.retired IS DISTINCT FROM true`},
		{"null safe ne", parse(t, `{"vehicleMake":{"ne":"Ford"}}`), `c.vehicle_make IS DISTINCT FROM 'Ford'`},
		{"not over null", parse(t, `{"not":{"vehicleMake":{"eq":"Ford"}}}`), `c.vehicle_make IS DISTINCT FROM 'Ford'`},
		{"or", parse(t, `{"or":[{"sex":{"eq":"Male"}},{"ageYears":{"lt":30}}]}`), `(c.sex = 'Male' OR c.age_years < 30)`},
		{"orders every", parse(t, `{"orders":{"every":{"orderStatus":{"in":["delivered","shipped"]}}}}`),
			`NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND o.order_status NOT IN ('delivered','shipped'))`},
		{"events some", parse(t, `{"events":{"some":{"eventType":{"eq":"purchase"},"valueGbp":{"gt":"20"}}}}`),
			`EXISTS (SELECT 1 FROM dwh.fact_event e WHERE e.customer_id = c.customer_id AND e.event_type = 'purchase' AND e.value_gbp > 20)`},
		{"city canonical", parse(t, `{"city":{"in":["ESKISEHIR","diyarbakir"]}}`), `c.city IN ('Eskişehir','Diyarbakır')`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Compile(tt.filter)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if have, want := count(t, got.Where, got.Args...), count(t, tt.sql); have != want {
				t.Errorf("compiled filter matches %d customers, hand-written SQL %d\nWHERE %s", have, want, got.Where)
			}
		})
	}
}

var quoted = regexp.MustCompile(`'([^']*)'::(?:text|bpchar)`)

// TestClosedListsMatchSchema keeps the hard-coded value lists in sync with the
// CHECK constraints of 02_customer_schema.sql (D-062).
func TestClosedListsMatchSchema(t *testing.T) {
	for _, ent := range []*Entity{customerEntity, orderEntity, cartItemEntity, eventEntity} {
		for _, f := range ent.Fields {
			if f.Closed == nil {
				continue
			}
			t.Run(ent.Alias+"."+f.Name, func(t *testing.T) {
				rows, err := pool.Query(t.Context(), `
					SELECT pg_get_constraintdef(oid) FROM pg_constraint
					WHERE conrelid = $1::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE $2`,
					ent.Table, "CHECK (("+f.Column+" = ANY (ARRAY[%")
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				var defs []string
				for rows.Next() {
					var d string
					if err := rows.Scan(&d); err != nil {
						t.Fatal(err)
					}
					defs = append(defs, d)
				}
				if len(defs) != 1 {
					t.Fatalf("found %d CHECK constraints on %s.%s, want 1: %v", len(defs), ent.Table, f.Column, defs)
				}
				var live []string
				for _, m := range quoted.FindAllStringSubmatch(defs[0], -1) {
					live = append(live, m[1])
				}
				want := slices.Sorted(slices.Values(f.Closed))
				if !slices.Equal(slices.Sorted(slices.Values(live)), want) {
					t.Errorf("catalog %v differs from the constraint %v", f.Closed, live)
				}
			})
		}
	}
}

func TestLoadCitiesFindsEveryStoredCity(t *testing.T) {
	cities, err := LoadCities(t.Context(), pool)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(t.Context(), `SELECT DISTINCT city FROM dwh.dim_customer`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var city string
		if err := rows.Scan(&city); err != nil {
			t.Fatal(err)
		}
		n++
		// The stored spelling must come back unchanged, whatever its case or dots.
		for _, typed := range []string{city, Key(city)} {
			if got, ok := cities.Lookup(typed); !ok || got != city {
				t.Errorf("Lookup(%q) = %q, %v; want %q", typed, got, ok, city)
			}
		}
	}
	if len(cities.Names()) != n {
		t.Errorf("%d cities loaded, %d stored", len(cities.Names()), n)
	}
}

func TestPreviewAndRecipientsAgreeWithEachOther(t *testing.T) {
	svc := NewService(pool, liveCompiler(t).Cities)
	filters := []*model.CustomerFilter{
		{},
		parse(t, `{"city":{"eq":"istanbul"}}`),
		parse(t, `{"loyaltyTier":{"in":["Gold","Platinum"]},"orders":{"some":{}}}`),
	}
	for i, f := range filters {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			pv, err := svc.Preview(t.Context(), f)
			if err != nil {
				t.Fatal(err)
			}
			total, email, sms, maxName := 0, 0, 0, 0
			perPair := map[[2]string]int{}
			var after *string
			for {
				page, err := svc.Recipients(t.Context(), f, 250, after)
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range page.Nodes {
					total++
					perPair[[2]string{r.Language, r.Timezone}]++
					maxName = max(maxName, utf8.RuneCountInString(r.FirstName))
					if r.EmailEligible {
						email++
					}
					if r.SmsEligible {
						sms++
					}
				}
				if !page.PageInfo.HasNextPage {
					break
				}
				after = page.PageInfo.EndCursor
			}
			if total != pv.Count || email != pv.Coverage.Both+pv.Coverage.EmailOnly || sms != pv.Coverage.Both+pv.Coverage.SmsOnly {
				t.Errorf("recipients: %d total, %d email, %d sms; preview: %+v", total, email, sms, pv)
			}
			for _, l := range pv.ByLocale {
				if got := perPair[[2]string{l.Language, l.Timezone}]; got != l.Count {
					t.Errorf("pair %s/%s: %d recipients, byLocale count %d", l.Language, l.Timezone, got, l.Count)
				}
				delete(perPair, [2]string{l.Language, l.Timezone})
			}
			if len(perPair) != 0 {
				t.Errorf("recipients in pairs missing from byLocale: %v", perPair)
			}
			if maxName != pv.MaxFirstNameLength {
				t.Errorf("longest recipient first name = %d, preview maxFirstNameLength = %d", maxName, pv.MaxFirstNameLength)
			}
		})
	}
}
