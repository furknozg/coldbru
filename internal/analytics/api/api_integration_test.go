//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/analytics/api"
	"github.com/furknozg/coldbru/internal/analytics/atest"
	"github.com/furknozg/coldbru/internal/analytics/store"
)

var pg *atest.Postgres

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	var err error
	pg, err = atest.Start(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	pg.Close()
	os.Exit(code)
}

func numeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatal(err)
	}
	return n
}

func server(t *testing.T, pool *pgxpool.Pool) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	api.New(api.Config{Source: api.NewPoolSource(pool), Logger: atest.Discard()}).Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func getInto(t *testing.T, srv *httptest.Server, path string, into any) int {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if into != nil && resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func TestEmptyDatabase(t *testing.T) {
	srv := server(t, pg.NewMigratedPool(t))
	var list api.CampaignList
	if code := getInto(t, srv, "/campaigns", &list); code != 200 || list.Items == nil || len(list.Items) != 0 || list.Limit != 20 || list.Offset != 0 {
		t.Errorf("list: status %d, %+v", code, list)
	}
	var versions []api.PromptVersionUsage
	if code := getInto(t, srv, "/llm-usage", &versions); code != 200 || versions == nil || len(versions) != 0 {
		t.Errorf("llm-usage: status %d, %+v", code, versions)
	}
	if code := getInto(t, srv, "/campaigns/"+uuid.NewString()+"/summary", nil); code != 404 {
		t.Errorf("summary of nothing: status %d, want 404", code)
	}
}

func TestListIsNewestFirstAndPaged(t *testing.T) {
	ctx := t.Context()
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
		at := base.Add(time.Duration(i) * time.Hour)
		if i == 4 {
			at = base.Add(3 * time.Hour) // ties with ids[3]: the larger campaign id comes first
		}
		if err := q.UpsertCampaignSummary(ctx, store.UpsertCampaignSummaryParams{
			CampaignID: ids[i], CompletedAt: pgtype.Timestamptz{Time: at, Valid: true}, SmsSegments: int32(i), DeliveryCostUsd: numeric(t, fmt.Sprint(i)+".5"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	// ids[0] has LLM usage; the others none.
	if _, err := q.InsertLLMUsage(ctx, store.InsertLLMUsageParams{
		LlmCallID: uuid.New(), CampaignID: ids[0], Purpose: "screen", Model: "claude-haiku-5-5", PromptName: "screen", PromptVersion: "1.0.0",
		Outcome: "accepted", CostUsd: numeric(t, "0.00000007"), CreatedAt: pgtype.Timestamptz{Time: base, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	srv := server(t, pool)

	hi, lo := ids[3], ids[4]
	if lo.String() > hi.String() {
		hi, lo = lo, hi
	}
	want := []uuid.UUID{hi, lo, ids[2], ids[1], ids[0]}

	var all api.CampaignList
	if code := getInto(t, srv, "/campaigns?limit=100", &all); code != 200 || len(all.Items) != 5 {
		t.Fatalf("status %d, %+v", code, all)
	}
	for i, it := range all.Items {
		if it.CampaignID != want[i] {
			t.Errorf("item %d = %s, want %s", i, it.CampaignID, want[i])
		}
	}
	last := all.Items[4]
	if last.LLMCostUSD != "0.00000007" || last.DeliveryCostUSD != "0.500000" || last.SMSSegments != 0 || !last.CompletedAt.Equal(base) {
		t.Errorf("oldest item = %+v", last)
	}
	if all.Items[0].LLMCostUSD != "0.00000000" {
		t.Errorf("a campaign without usage has llm_cost_usd %s, want 0.00000000", all.Items[0].LLMCostUSD)
	}

	var page api.CampaignList
	if code := getInto(t, srv, "/campaigns?limit=2&offset=1", &page); code != 200 || len(page.Items) != 2 || page.Limit != 2 || page.Offset != 1 ||
		page.Items[0].CampaignID != want[1] || page.Items[1].CampaignID != want[2] {
		t.Errorf("page = status %d, %+v", code, page)
	}
	if code := getInto(t, srv, "/campaigns?offset=5", &page); code != 200 || page.Items == nil || len(page.Items) != 0 {
		t.Errorf("past the end = status %d, %+v", code, page)
	}
}

func TestCampaignWithUsageButNoCompletion(t *testing.T) {
	ctx := t.Context()
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	id := uuid.New()
	for i, purpose := range []string{"screen", "segment", "segment"} {
		if _, err := q.InsertLLMUsage(ctx, store.InsertLLMUsageParams{
			LlmCallID: uuid.New(), CampaignID: id, Purpose: purpose, Model: "claude-haiku-5-5", PromptName: purpose, PromptVersion: "1.0.0",
			InputTokens: int32(10 * (i + 1)), OutputTokens: 1, Outcome: "accepted", LatencyMs: 5, CostUsd: numeric(t, "0.25"),
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
	}
	var rep api.CampaignReport
	if code := getInto(t, server(t, pool), "/campaigns/"+id.String()+"/summary", &rep); code != 200 {
		t.Fatalf("status %d", code)
	}
	if rep.CampaignID != id || rep.CompletedAt != nil || rep.Deliveries == nil || len(rep.Deliveries) != 0 || rep.SMSSegments != 0 || rep.DeliveryCostUSD != "0.000000" ||
		rep.LLM.Calls != 3 || rep.LLM.CostUSD != "0.75000000" || rep.LLM.Tokens.Input != 60 || rep.LLM.Tokens.Output != 3 ||
		len(rep.LLM.ByPurpose) != 2 || rep.LLM.ByPurpose[0].Purpose != "screen" || rep.LLM.ByPurpose[1].Purpose != "segment" ||
		rep.LLM.ByPurpose[1].Calls != 2 || rep.LLM.ByPurpose[1].InputTokens != 50 || rep.LLM.ByPurpose[1].CostUSD != "0.50000000" {
		t.Errorf("report = %+v", rep)
	}
}

func TestCompletedCampaignWithoutUsage(t *testing.T) {
	ctx := t.Context()
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	id := uuid.New()
	if err := q.UpsertCampaignSummary(ctx, store.UpsertCampaignSummaryParams{
		CampaignID: id, CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, SmsSegments: 1, DeliveryCostUsd: numeric(t, "0.04"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.InsertDeliveryCount(ctx, store.InsertDeliveryCountParams{CampaignID: id, Channel: "sms", Status: "sent", Count: 1}); err != nil {
		t.Fatal(err)
	}
	var rep api.CampaignReport
	if code := getInto(t, server(t, pool), "/campaigns/"+id.String()+"/summary", &rep); code != 200 {
		t.Fatalf("status %d", code)
	}
	if rep.CompletedAt == nil || len(rep.Deliveries) != 1 || rep.DeliveryCostUSD != "0.040000" ||
		rep.LLM.Calls != 0 || rep.LLM.CostUSD != "0.00000000" || rep.LLM.ByPurpose == nil || len(rep.LLM.ByPurpose) != 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestPromptVersionsRepairRate(t *testing.T) {
	ctx := t.Context()
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	call := func(name, version, outcome string, latency int32, cost string) {
		t.Helper()
		if _, err := q.InsertLLMUsage(ctx, store.InsertLLMUsageParams{
			LlmCallID: uuid.New(), CampaignID: uuid.New(), Purpose: "content", Model: "claude-sonnet-5-5", PromptName: name, PromptVersion: version,
			Outcome: outcome, LatencyMs: latency, CostUsd: numeric(t, cost), CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
	}
	for range 13 {
		call("content", "1.0.0", "accepted", 100, "0.1")
	}
	call("content", "1.0.0", "validation_failed", 400, "0.2")
	call("content", "1.0.0", "refusal", 100, "0")
	call("content", "1.0.0", "api_error", 100, "0")
	call("content", "1.1.0", "accepted", 50, "0.05")
	call("content", "1.1.0", "validation_failed", 51, "0.05")
	call("content", "1.1.0", "validation_failed", 52, "0.05")

	var got []api.PromptVersionUsage
	if code := getInto(t, server(t, pool), "/llm-usage?group=prompt_version", &got); code != 200 || len(got) != 2 {
		t.Fatalf("status %d, %+v", code, got)
	}
	// 1.0.0: mean latency (13*100 + 400 + 100 + 100) / 16 = 118.75, shown as 118.8.
	a, b := got[0], got[1]
	if a.PromptName != "content" || a.PromptVersion != "1.0.0" || a.Calls != 16 || a.Accepted != 13 || a.ValidationFailed != 1 || a.Refusal != 1 || a.APIError != 1 ||
		a.RepairRate != 0.0625 || a.AvgLatencyMs != 118.8 || a.CostUSD != "1.50000000" {
		t.Errorf("1.0.0 = %+v", a)
	}
	// 2 of 3 calls were validation_failed: 0.6667; mean latency (50+51+52)/3 = 51.
	if b.PromptVersion != "1.1.0" || b.Calls != 3 || b.RepairRate != 0.6667 || b.AvgLatencyMs != 51 || b.CostUSD != "0.15000000" {
		t.Errorf("1.1.0 = %+v", b)
	}
}
