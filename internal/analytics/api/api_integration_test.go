//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
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

// day0 is the anchor of the range fixtures: midnight UTC.
var day0 = time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func rfc(t time.Time) string { return url.QueryEscape(t.Format(time.RFC3339)) }

// rangeFixture inserts LLM calls and completed campaigns around day0 and returns the pool:
//
//	llm   day0-30m  screen   haiku   0.1     before the range [day0, day0+3d)
//	      day0      content  sonnet  1.0     exactly at from: inside
//	      day0+59m  content  sonnet  0.5
//	      day0+2h   segment  haiku   0.25
//	      day0+1d5h content  haiku   0.125
//	      day0+3d   screen   sonnet  2.0     exactly at to: outside
//	camps day0-1h  sms 100 sent                                   outside
//	      day0+1h  c1: 4 segments 0.16; sms sent 10, sms failed 2, email sent 5
//	      day0+37h c2: 2 segments 0.08; sms sent 3, email skipped_no_consent 1
//	      day0+3d  sms 7 sent                                      exactly at to: outside
func rangeFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := t.Context()
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	call := func(at time.Time, purpose, model, cost string, in, out int32) {
		t.Helper()
		if _, err := q.InsertLLMUsage(ctx, store.InsertLLMUsageParams{
			LlmCallID: uuid.New(), CampaignID: uuid.New(), Purpose: purpose, Model: model, PromptName: purpose, PromptVersion: "1.0.0",
			InputTokens: in, OutputTokens: out, CacheReadTokens: 1, CacheCreationTokens: 2, Outcome: "accepted", LatencyMs: 10,
			CostUsd: numeric(t, cost), CreatedAt: ts(at),
		}); err != nil {
			t.Fatal(err)
		}
	}
	call(day0.Add(-30*time.Minute), "screen", "claude-haiku-5-5", "0.1", 10, 1)
	call(day0, "content", "claude-sonnet-5-5", "1.0", 100, 10)
	call(day0.Add(59*time.Minute), "content", "claude-sonnet-5-5", "0.5", 50, 5)
	call(day0.Add(2*time.Hour), "segment", "claude-haiku-5-5", "0.25", 20, 2)
	call(day0.Add(29*time.Hour), "content", "claude-haiku-5-5", "0.125", 30, 3)
	call(day0.Add(72*time.Hour), "screen", "claude-sonnet-5-5", "2.0", 7, 7)

	campaign := func(at time.Time, segments int32, cost string, counts ...store.InsertDeliveryCountParams) {
		t.Helper()
		id := uuid.New()
		if err := q.UpsertCampaignSummary(ctx, store.UpsertCampaignSummaryParams{
			CampaignID: id, CompletedAt: ts(at), SmsSegments: segments, DeliveryCostUsd: numeric(t, cost),
		}); err != nil {
			t.Fatal(err)
		}
		for _, c := range counts {
			c.CampaignID = id
			if err := q.InsertDeliveryCount(ctx, c); err != nil {
				t.Fatal(err)
			}
		}
	}
	dc := func(channel, status string, n int32) store.InsertDeliveryCountParams {
		return store.InsertDeliveryCountParams{Channel: channel, Status: status, Count: n}
	}
	campaign(day0.Add(-time.Hour), 1, "0.04", dc("sms", "sent", 100))
	campaign(day0.Add(time.Hour), 4, "0.16", dc("sms", "sent", 10), dc("sms", "failed", 2), dc("email", "sent", 5))
	campaign(day0.Add(37*time.Hour), 2, "0.08", dc("sms", "sent", 3), dc("email", "skipped_no_consent", 1))
	campaign(day0.Add(72*time.Hour), 9, "0.5", dc("sms", "sent", 7))
	return pool
}

func TestOverviewRange(t *testing.T) {
	srv := server(t, rangeFixture(t))
	end := day0.Add(72 * time.Hour)

	var in api.Overview
	if code := getInto(t, srv, "/overview?from="+rfc(day0)+"&to="+rfc(end), &in); code != 200 {
		t.Fatalf("status %d", code)
	}
	if in.From == nil || !in.From.Equal(day0) || in.To == nil || !in.To.Equal(end) {
		t.Errorf("echoed range = %v .. %v", in.From, in.To)
	}
	wantDeliveries := []api.DeliveryCount{
		{Channel: "email", Status: "sent", Count: 5}, {Channel: "email", Status: "skipped_no_consent", Count: 1},
		{Channel: "sms", Status: "failed", Count: 2}, {Channel: "sms", Status: "sent", Count: 13},
	}
	if in.CampaignsCompleted != 2 || in.SMSSegments != 6 || in.DeliveryCostUSD != "0.240000" || !slices.Equal(in.Deliveries, wantDeliveries) {
		t.Errorf("deliveries part = %+v", in)
	}
	wantLLM := api.OverviewLLM{
		Calls: 4, CostUSD: "1.87500000", Tokens: api.Tokens{Input: 200, Output: 20, CacheRead: 4, CacheCreation: 8},
		ByPurpose: []api.PurposeUsage{
			{Purpose: "content", Calls: 3, CostUSD: "1.62500000", InputTokens: 180, OutputTokens: 18},
			{Purpose: "segment", Calls: 1, CostUSD: "0.25000000", InputTokens: 20, OutputTokens: 2},
		},
		ByModel: []api.ModelUsage{
			{Model: "claude-haiku-5-5", Calls: 2, CostUSD: "0.37500000", InputTokens: 50, OutputTokens: 5},
			{Model: "claude-sonnet-5-5", Calls: 2, CostUSD: "1.50000000", InputTokens: 150, OutputTokens: 15},
		},
	}
	if !reflect.DeepEqual(in.LLM, wantLLM) {
		t.Errorf("llm = %+v\nwant %+v", in.LLM, wantLLM)
	}

	t.Run("open ended", func(t *testing.T) {
		var all, fromOnly, toOnly api.Overview
		getInto(t, srv, "/overview", &all)
		getInto(t, srv, "/overview?from="+rfc(day0), &fromOnly)
		getInto(t, srv, "/overview?to="+rfc(day0), &toOnly)
		if all.From != nil || all.To != nil || all.CampaignsCompleted != 4 || all.LLM.Calls != 6 || all.SMSSegments != 16 || all.DeliveryCostUSD != "0.780000" || all.LLM.CostUSD != "3.97500000" {
			t.Errorf("no range = %+v", all)
		}
		// from day0 on: campaigns day0+1h, +37h and day0+3d; calls day0 .. day0+3d.
		if fromOnly.To != nil || fromOnly.CampaignsCompleted != 3 || fromOnly.SMSSegments != 15 || fromOnly.LLM.Calls != 5 || fromOnly.LLM.CostUSD != "3.87500000" {
			t.Errorf("from only = %+v", fromOnly)
		}
		// before day0: one campaign and one call.
		if toOnly.From != nil || toOnly.CampaignsCompleted != 1 || toOnly.SMSSegments != 1 || toOnly.LLM.Calls != 1 || toOnly.LLM.CostUSD != "0.10000000" ||
			len(toOnly.Deliveries) != 1 || toOnly.Deliveries[0].Count != 100 {
			t.Errorf("to only = %+v", toOnly)
		}
	})

	t.Run("empty range is zeros and empty arrays", func(t *testing.T) {
		var none api.Overview
		far := day0.AddDate(1, 0, 0)
		if code := getInto(t, srv, "/overview?from="+rfc(far)+"&to="+rfc(far.Add(time.Hour)), &none); code != 200 {
			t.Fatalf("status %d", code)
		}
		if none.CampaignsCompleted != 0 || none.SMSSegments != 0 || none.DeliveryCostUSD != "0.000000" || none.Deliveries == nil || len(none.Deliveries) != 0 ||
			none.LLM.Calls != 0 || none.LLM.CostUSD != "0.00000000" || none.LLM.ByPurpose == nil || len(none.LLM.ByPurpose) != 0 || none.LLM.ByModel == nil || len(none.LLM.ByModel) != 0 {
			t.Errorf("empty overview = %+v", none)
		}
	})

	t.Run("bad range is 422", func(t *testing.T) {
		if code := getInto(t, srv, "/overview?from="+rfc(end)+"&to="+rfc(day0), nil); code != 422 {
			t.Errorf("status %d, want 422", code)
		}
	})
}

func TestLLMUsageRange(t *testing.T) {
	srv := server(t, rangeFixture(t))
	var all, in []api.PromptVersionUsage
	if code := getInto(t, srv, "/llm-usage", &all); code != 200 || len(all) != 3 {
		t.Fatalf("without a range: status %d, %+v", code, all)
	}
	// No parameters count every call: screen 2 (0.1 + 2.0), content 3, segment 1.
	byName := map[string]api.PromptVersionUsage{}
	for _, v := range all {
		byName[v.PromptName] = v
	}
	if byName["screen"].Calls != 2 || byName["screen"].CostUSD != "2.10000000" || byName["content"].Calls != 3 || byName["segment"].Calls != 1 {
		t.Errorf("without a range = %+v", all)
	}
	if code := getInto(t, srv, "/llm-usage?from="+rfc(day0)+"&to="+rfc(day0.Add(72*time.Hour)), &in); code != 200 || len(in) != 2 {
		t.Fatalf("with a range: status %d, %+v", code, in)
	}
	if in[0].PromptName != "content" || in[0].Calls != 3 || in[0].CostUSD != "1.62500000" || in[1].PromptName != "segment" || in[1].Calls != 1 {
		t.Errorf("with a range = %+v", in)
	}
}

func TestLLMSeries(t *testing.T) {
	srv := server(t, rangeFixture(t))
	h := func(n int) time.Time { return day0.Add(time.Duration(n) * time.Hour) }

	t.Run("hour buckets, empty ones listed", func(t *testing.T) {
		var s api.LLMSeries
		if code := getInto(t, srv, "/llm-usage/series?bucket=hour&from="+rfc(day0)+"&to="+rfc(h(4)), &s); code != 200 {
			t.Fatalf("status %d", code)
		}
		if !slices.EqualFunc(s.Buckets, []time.Time{h(0), h(1), h(2), h(3)}, time.Time.Equal) || s.Bucket != "hour" || !s.From.Equal(day0) || !s.To.Equal(h(4)) {
			t.Errorf("series head = %+v", s)
		}
		want := []api.LLMSeriesPoint{
			{Start: h(0), Purpose: "content", Model: "claude-sonnet-5-5", Calls: 2, CostUSD: "1.50000000", InputTokens: 150, OutputTokens: 15},
			{Start: h(2), Purpose: "segment", Model: "claude-haiku-5-5", Calls: 1, CostUSD: "0.25000000", InputTokens: 20, OutputTokens: 2},
		}
		if !reflect.DeepEqual(s.Points, want) {
			t.Errorf("points = %+v\nwant %+v", s.Points, want)
		}
	})

	t.Run("day buckets align to midnight UTC from an unaligned range", func(t *testing.T) {
		from, to := day0.Add(-12*time.Hour), day0.Add(72*time.Hour+12*time.Hour)
		var s api.LLMSeries
		if code := getInto(t, srv, "/llm-usage/series?bucket=day&from="+rfc(from)+"&to="+rfc(to), &s); code != 200 {
			t.Fatalf("status %d", code)
		}
		d := func(n int) time.Time { return day0.AddDate(0, 0, n) }
		if !slices.EqualFunc(s.Buckets, []time.Time{d(-1), d(0), d(1), d(2), d(3)}, time.Time.Equal) {
			t.Errorf("buckets = %v", s.Buckets)
		}
		type key struct {
			start          time.Time
			purpose, model string
		}
		got := map[key]int64{}
		for _, p := range s.Points {
			got[key{p.Start, p.Purpose, p.Model}] = p.Calls
		}
		want := map[key]int64{
			{d(-1), "screen", "claude-haiku-5-5"}: 1, {d(0), "content", "claude-sonnet-5-5"}: 2, {d(0), "segment", "claude-haiku-5-5"}: 1,
			{d(1), "content", "claude-haiku-5-5"}: 1, {d(3), "screen", "claude-sonnet-5-5"}: 1,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("points = %v\nwant %v", got, want)
		}
		for i := 1; i < len(s.Points); i++ { // ordered by bucket
			if s.Points[i].Start.Before(s.Points[i-1].Start) {
				t.Errorf("points not ordered by bucket: %+v", s.Points)
			}
		}
	})

	t.Run("to on a bucket boundary adds no bucket and excludes its rows", func(t *testing.T) {
		var s api.LLMSeries
		getInto(t, srv, "/llm-usage/series?bucket=day&from="+rfc(day0)+"&to="+rfc(day0.Add(72*time.Hour)), &s)
		if len(s.Buckets) != 3 || !s.Buckets[2].Equal(day0.AddDate(0, 0, 2)) {
			t.Errorf("buckets = %v", s.Buckets)
		}
		var total int64
		for _, p := range s.Points {
			total += p.Calls
		}
		if total != 4 {
			t.Errorf("calls in points = %d, want 4", total)
		}
	})

	t.Run("a range without data is all empty buckets", func(t *testing.T) {
		far := day0.AddDate(1, 0, 0)
		var s api.LLMSeries
		getInto(t, srv, "/llm-usage/series?bucket=hour&from="+rfc(far)+"&to="+rfc(far.Add(2*time.Hour)), &s)
		if len(s.Buckets) != 2 || s.Points == nil || len(s.Points) != 0 {
			t.Errorf("series = %+v", s)
		}
	})
}

func TestOverviewSeries(t *testing.T) {
	srv := server(t, rangeFixture(t))
	h := func(n int) time.Time { return day0.Add(time.Duration(n) * time.Hour) }

	t.Run("hour buckets", func(t *testing.T) {
		var s api.OverviewSeries
		if code := getInto(t, srv, "/overview/series?bucket=hour&from="+rfc(day0)+"&to="+rfc(h(3)), &s); code != 200 {
			t.Fatalf("status %d", code)
		}
		if !slices.EqualFunc(s.Buckets, []time.Time{h(0), h(1), h(2)}, time.Time.Equal) {
			t.Errorf("buckets = %v", s.Buckets)
		}
		want := []api.OverviewSeriesPoint{
			{Start: h(1), Channel: "email", Status: "sent", Count: 5},
			{Start: h(1), Channel: "sms", Status: "failed", Count: 2},
			{Start: h(1), Channel: "sms", Status: "sent", Count: 10},
		}
		if !reflect.DeepEqual(s.Points, want) {
			t.Errorf("points = %+v\nwant %+v", s.Points, want)
		}
	})

	t.Run("day buckets sum the campaigns of a day", func(t *testing.T) {
		var s api.OverviewSeries
		getInto(t, srv, "/overview/series?bucket=day&from="+rfc(day0.Add(-24*time.Hour))+"&to="+rfc(day0.Add(72*time.Hour)), &s)
		d := func(n int) time.Time { return day0.AddDate(0, 0, n) }
		if !slices.EqualFunc(s.Buckets, []time.Time{d(-1), d(0), d(1), d(2)}, time.Time.Equal) {
			t.Errorf("buckets = %v", s.Buckets)
		}
		want := []api.OverviewSeriesPoint{
			{Start: d(-1), Channel: "sms", Status: "sent", Count: 100},
			{Start: d(0), Channel: "email", Status: "sent", Count: 5},
			{Start: d(0), Channel: "sms", Status: "failed", Count: 2},
			{Start: d(0), Channel: "sms", Status: "sent", Count: 10},
			{Start: d(1), Channel: "email", Status: "skipped_no_consent", Count: 1},
			{Start: d(1), Channel: "sms", Status: "sent", Count: 3},
		}
		if !reflect.DeepEqual(s.Points, want) {
			t.Errorf("points = %+v\nwant %+v", s.Points, want)
		}
	})
}

// TestSeriesBucketsAreUTCUnderOtherSessionZones: the buckets are UTC date_trunc boundaries and 24h
// apart even when the database session runs in a zone with DST, across the day it changes.
func TestSeriesBucketsAreUTCUnderOtherSessionZones(t *testing.T) {
	ctx := t.Context()
	pool := rangeFixture(t)
	if _, err := pool.Exec(ctx, `DO $$ BEGIN EXECUTE format('ALTER DATABASE %I SET timezone = %L', current_database(), 'America/New_York'); END $$`); err != nil {
		t.Fatal(err)
	}
	pool.Reset() // new connections pick the zone up
	var zone string
	if err := pool.QueryRow(ctx, "SHOW timezone").Scan(&zone); err != nil || zone != "America/New_York" {
		t.Fatalf("session zone = %q, %v", zone, err)
	}
	srv := server(t, pool)
	from := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC) // New York leaves DST on 2026-11-01
	to := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	for _, path := range []string{"/llm-usage/series", "/overview/series"} {
		var s struct {
			Buckets []time.Time `json:"buckets"`
		}
		if code := getInto(t, srv, path+"?bucket=day&from="+rfc(from)+"&to="+rfc(to), &s); code != 200 {
			t.Fatalf("%s: status %d", path, code)
		}
		if len(s.Buckets) != 5 {
			t.Fatalf("%s: buckets = %v", path, s.Buckets)
		}
		for i, b := range s.Buckets {
			if want := from.AddDate(0, 0, i); !b.Equal(want) {
				t.Errorf("%s: bucket %d = %v, want %v", path, i, b, want)
			}
		}
	}
	// The fixture's calls still land in the UTC day buckets.
	var s api.LLMSeries
	getInto(t, srv, "/llm-usage/series?bucket=day&from="+rfc(day0)+"&to="+rfc(day0.AddDate(0, 0, 2)), &s)
	if len(s.Buckets) != 2 || !s.Buckets[0].Equal(day0) || len(s.Points) != 3 || !s.Points[0].Start.Equal(day0) {
		t.Errorf("series under New York = %+v", s)
	}
}

// seedCompleted stores a completed campaign with its sent counts per slot.
func seedCompleted(t *testing.T, q *store.Queries, id uuid.UUID, at time.Time, sent ...store.InsertSentLocaleParams) {
	t.Helper()
	ctx := t.Context()
	if err := q.UpsertCampaignSummary(ctx, store.UpsertCampaignSummaryParams{
		CampaignID: id, CompletedAt: pgtype.Timestamptz{Time: at, Valid: true}, SmsSegments: 0, DeliveryCostUsd: numeric(t, "0"),
	}); err != nil {
		t.Fatal(err)
	}
	for _, s := range sent {
		s.CampaignID = id
		if err := q.InsertSentLocale(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
}

// seedClick stores one click the way the intake does: the event row and the counters.
func seedClick(t *testing.T, q *store.Queries, id uuid.UUID, channel, language, timezone string, first bool, at time.Time) {
	t.Helper()
	ctx := t.Context()
	if _, err := q.InsertClickEvent(ctx, store.InsertClickEventParams{
		ClickID: uuid.New(), CampaignID: id, Channel: channel, Language: language, Timezone: timezone,
		ClickedAt: pgtype.Timestamptz{Time: at, Valid: true}, First: first,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.AddCampaignClick(ctx, store.AddCampaignClickParams{CampaignID: id, Channel: channel, Language: language, Timezone: timezone, First: first}); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignClickThroughRate(t *testing.T) {
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	srv := server(t, pool)
	id := uuid.New()
	at := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

	// Clicks before the completed event: visible and live, but there is no denominator yet, so CTR is 0.
	seedClick(t, q, id, "sms", "tr", "Europe/Istanbul", true, at)
	var rep api.CampaignReport
	if code := getInto(t, srv, "/campaigns/"+id.String()+"/summary", &rep); code != 200 {
		t.Fatalf("a campaign with clicks only: status %d, want 200", code)
	}
	if rep.CompletedAt != nil || rep.Clicks.UniqueClicks != 1 || rep.Clicks.TotalClicks != 1 || rep.Clicks.Sent != 0 || rep.Clicks.CTR != 0 ||
		len(rep.Clicks.ByLocale) != 1 || rep.Clicks.ByLocale[0].Sent != 0 {
		t.Errorf("clicks before completion = %+v", rep.Clicks)
	}

	// Completed: 40 SMS to tr/Istanbul, 10 SMS to en/London, 100 emails to tr/Istanbul and 5 to en/London.
	seedCompleted(t, q, id, at.Add(time.Hour),
		store.InsertSentLocaleParams{Channel: "sms", Language: "tr", Timezone: "Europe/Istanbul", Sent: 40},
		store.InsertSentLocaleParams{Channel: "sms", Language: "en", Timezone: "Europe/London", Sent: 10},
		store.InsertSentLocaleParams{Channel: "email", Language: "tr", Timezone: "Europe/Istanbul", Sent: 100},
		store.InsertSentLocaleParams{Channel: "email", Language: "en", Timezone: "Europe/London", Sent: 5})
	for range 9 { // sms/tr: 10 unique clickers in all (one above), 3 more clicks by the same people
		seedClick(t, q, id, "sms", "tr", "Europe/Istanbul", true, at)
	}
	for range 3 {
		seedClick(t, q, id, "sms", "tr", "Europe/Istanbul", false, at)
	}
	for range 25 { // email/tr: 25 unique of 100
		seedClick(t, q, id, "email", "tr", "Europe/Istanbul", true, at)
	}
	seedClick(t, q, id, "email", "en", "Europe/London", true, at) // 1 of 5

	if code := getInto(t, srv, "/campaigns/"+id.String()+"/summary", &rep); code != 200 {
		t.Fatalf("status %d", code)
	}
	c := rep.Clicks
	if c.Sent != 155 || c.UniqueClicks != 36 || c.TotalClicks != 39 || c.CTR != 0.2323 {
		t.Errorf("campaign clicks = sent %d, unique %d, total %d, ctr %v; want 155, 36, 39, 0.2323", c.Sent, c.UniqueClicks, c.TotalClicks, c.CTR)
	}
	wantChannels := []api.ChannelClicks{
		{Channel: "email", Sent: 105, UniqueClicks: 26, TotalClicks: 26, CTR: 0.2476},
		{Channel: "sms", Sent: 50, UniqueClicks: 10, TotalClicks: 13, CTR: 0.2},
	}
	if !reflect.DeepEqual(c.ByChannel, wantChannels) {
		t.Errorf("by channel = %+v\nwant %+v", c.ByChannel, wantChannels)
	}
	wantLocales := []api.LocaleClicks{
		{Channel: "email", Language: "en", Timezone: "Europe/London", Sent: 5, UniqueClicks: 1, TotalClicks: 1, CTR: 0.2},
		{Channel: "email", Language: "tr", Timezone: "Europe/Istanbul", Sent: 100, UniqueClicks: 25, TotalClicks: 25, CTR: 0.25},
		{Channel: "sms", Language: "en", Timezone: "Europe/London", Sent: 10, UniqueClicks: 0, TotalClicks: 0, CTR: 0},
		{Channel: "sms", Language: "tr", Timezone: "Europe/Istanbul", Sent: 40, UniqueClicks: 10, TotalClicks: 13, CTR: 0.25},
	}
	if !reflect.DeepEqual(c.ByLocale, wantLocales) {
		t.Errorf("by locale = %+v\nwant %+v", c.ByLocale, wantLocales)
	}

	// A campaign nobody has heard of is still a 404, and a completed one with nothing sent has an empty report.
	if code := getInto(t, srv, "/campaigns/"+uuid.NewString()+"/summary", nil); code != 404 {
		t.Errorf("unknown campaign: status %d, want 404", code)
	}
	quiet := uuid.New()
	seedCompleted(t, q, quiet, at)
	var empty api.CampaignReport
	if code := getInto(t, srv, "/campaigns/"+quiet.String()+"/summary", &empty); code != 200 ||
		empty.Clicks.ByChannel == nil || empty.Clicks.ByLocale == nil || len(empty.Clicks.ByLocale) != 0 || empty.Clicks.CTR != 0 {
		t.Errorf("completed with nothing sent: status %d, %+v", code, empty.Clicks)
	}
}

func TestOverviewClicksAndSeries(t *testing.T) {
	pool := pg.NewMigratedPool(t)
	q := store.New(pool)
	srv := server(t, pool)
	d1 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	d2 := d1.Add(24 * time.Hour)
	d3 := d2.Add(24 * time.Hour)

	// Campaign A completed on d1, campaign B on d3. A: 10 SMS sent, 2 unique clickers (one clicked
	// twice); 20 emails, 5 clickers. B: 10 SMS sent, 1 clicker.
	a, b := uuid.New(), uuid.New()
	seedCompleted(t, q, a, d1.Add(time.Hour),
		store.InsertSentLocaleParams{Channel: "sms", Language: "tr", Timezone: "Europe/Istanbul", Sent: 10},
		store.InsertSentLocaleParams{Channel: "email", Language: "tr", Timezone: "Europe/Istanbul", Sent: 20})
	seedCompleted(t, q, b, d3.Add(time.Hour), store.InsertSentLocaleParams{Channel: "sms", Language: "tr", Timezone: "Europe/Istanbul", Sent: 10})
	seedClick(t, q, a, "sms", "tr", "Europe/Istanbul", true, d1.Add(2*time.Hour))
	seedClick(t, q, a, "sms", "tr", "Europe/Istanbul", true, d2.Add(2*time.Hour))
	seedClick(t, q, a, "sms", "tr", "Europe/Istanbul", false, d2.Add(3*time.Hour))
	for range 5 {
		seedClick(t, q, a, "email", "tr", "Europe/Istanbul", true, d2.Add(4*time.Hour))
	}
	seedClick(t, q, b, "sms", "tr", "Europe/Istanbul", true, d3.Add(2*time.Hour))

	var all api.Overview
	if code := getInto(t, srv, "/overview", &all); code != 200 {
		t.Fatalf("status %d", code)
	}
	wantAll := api.OverviewClicks{
		Sent: 40, UniqueClicks: 8, TotalClicks: 9, CTR: 0.2,
		ByChannel: []api.ChannelClicks{
			{Channel: "email", Sent: 20, UniqueClicks: 5, TotalClicks: 5, CTR: 0.25},
			{Channel: "sms", Sent: 20, UniqueClicks: 3, TotalClicks: 4, CTR: 0.15},
		},
	}
	if !reflect.DeepEqual(all.Clicks, wantAll) {
		t.Errorf("overview clicks = %+v\nwant %+v", all.Clicks, wantAll)
	}

	// The range picks campaigns by completion: only A completed in [d1, d2+24h) ... [d1, d3).
	var rng api.Overview
	if code := getInto(t, srv, "/overview?from="+d1.Format(time.RFC3339)+"&to="+d3.Format(time.RFC3339), &rng); code != 200 {
		t.Fatalf("status %d", code)
	}
	if rng.Clicks.Sent != 30 || rng.Clicks.UniqueClicks != 7 || rng.Clicks.TotalClicks != 8 || rng.Clicks.CTR != 0.2333 {
		t.Errorf("overview clicks of campaign A only = %+v", rng.Clicks)
	}
	var none api.Overview
	if code := getInto(t, srv, "/overview?from=2027-01-01T00:00:00Z", &none); code != 200 || none.Clicks.ByChannel == nil || len(none.Clicks.ByChannel) != 0 || none.Clicks.CTR != 0 {
		t.Errorf("empty range clicks: status %d, %+v", code, none.Clicks)
	}

	// The series is by click time.
	var series api.OverviewSeries
	if code := getInto(t, srv, "/overview/series?bucket=day&from="+d1.Format(time.RFC3339)+"&to="+d3.Add(24*time.Hour).Format(time.RFC3339), &series); code != 200 {
		t.Fatalf("status %d", code)
	}
	wantSeries := []api.ClickSeriesPoint{
		{Start: d1, Channel: "sms", UniqueClicks: 1, TotalClicks: 1},
		{Start: d2, Channel: "email", UniqueClicks: 5, TotalClicks: 5},
		{Start: d2, Channel: "sms", UniqueClicks: 1, TotalClicks: 2},
		{Start: d3, Channel: "sms", UniqueClicks: 1, TotalClicks: 1},
	}
	if !reflect.DeepEqual(series.Clicks, wantSeries) {
		t.Errorf("click series = %+v\nwant %+v", series.Clicks, wantSeries)
	}
	var quiet api.OverviewSeries
	if code := getInto(t, srv, "/overview/series?bucket=day&from=2027-01-01T00:00:00Z&to=2027-01-03T00:00:00Z", &quiet); code != 200 || quiet.Clicks == nil || len(quiet.Clicks) != 0 {
		t.Errorf("empty click series: status %d, %+v", code, quiet.Clicks)
	}
}
