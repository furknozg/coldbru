package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeSource struct {
	list          []CampaignListItem
	listErr       error
	gotLimit      int
	gotOffset     int
	report        CampaignReport
	reportErr     error
	versions      []PromptVersionUsage
	versionsErr   error
	versionsCalls int
	gotRange      Range
	overview      Overview
	overviewErr   error
	gotSeries     SeriesQuery
	seriesCalls   int
	llmSeries     LLMSeries
	delivSeries   OverviewSeries
}

func (f *fakeSource) ListCampaigns(_ context.Context, limit, offset int) ([]CampaignListItem, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.list, f.listErr
}

func (f *fakeSource) Campaign(_ context.Context, id uuid.UUID) (CampaignReport, error) {
	if f.reportErr != nil {
		return CampaignReport{}, f.reportErr
	}
	r := f.report
	r.CampaignID = id
	return r, nil
}

func (f *fakeSource) PromptVersions(_ context.Context, rg Range) ([]PromptVersionUsage, error) {
	f.versionsCalls++
	f.gotRange = rg
	return f.versions, f.versionsErr
}

func (f *fakeSource) Overview(_ context.Context, rg Range) (Overview, error) {
	f.gotRange = rg
	o := f.overview
	o.From, o.To = rg.From, rg.To
	return o, f.overviewErr
}

func (f *fakeSource) LLMSeries(_ context.Context, q SeriesQuery) (LLMSeries, error) {
	f.seriesCalls++
	f.gotSeries = q
	return f.llmSeries, nil
}

func (f *fakeSource) OverviewSeries(_ context.Context, q SeriesQuery) (OverviewSeries, error) {
	f.seriesCalls++
	f.gotSeries = q
	return f.delivSeries, nil
}

func serve(t *testing.T, src Source) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	New(Config{Source: src, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}).Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func decodeProblem(t *testing.T, resp *http.Response, body []byte, status int, code string) Problem {
	t.Helper()
	var p Problem
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}
	if resp.StatusCode != status || resp.Header.Get("Content-Type") != ProblemContentType || p.Code != code || p.Status != status ||
		p.Type != "urn:coldbru:analytics:problem:"+code || p.Title == "" {
		t.Errorf("status %d, content-type %q, problem %+v; want %d %s", resp.StatusCode, resp.Header.Get("Content-Type"), p, status, code)
	}
	return p
}

func TestListCampaigns(t *testing.T) {
	id := uuid.MustParse("0198a2c4-0000-7000-8000-000000000001")
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	src := &fakeSource{list: []CampaignListItem{{CampaignID: id, CompletedAt: at, SMSSegments: 6, DeliveryCostUSD: "0.242000", LLMCostUSD: "0.01234500"}}}
	srv := serve(t, src)

	resp, body := get(t, srv, "/campaigns")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content-type %q: %s", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	want := `{"items":[{"campaign_id":"0198a2c4-0000-7000-8000-000000000001","completed_at":"2026-10-29T09:00:00Z","sms_segments":6,"delivery_cost_usd":"0.242000","llm_cost_usd":"0.01234500"}],"limit":20,"offset":0}`
	if strings.TrimSpace(string(body)) != want {
		t.Errorf("body = %s\nwant   %s", body, want)
	}
	if src.gotLimit != 20 || src.gotOffset != 0 {
		t.Errorf("default page = %d/%d, want 20/0", src.gotLimit, src.gotOffset)
	}

	get(t, srv, "/campaigns?limit=100&offset=40")
	if src.gotLimit != 100 || src.gotOffset != 40 {
		t.Errorf("page = %d/%d, want 100/40", src.gotLimit, src.gotOffset)
	}
}

func TestListCampaignsEmptyPageIsAnEmptyArray(t *testing.T) {
	_, body := get(t, serve(t, &fakeSource{}), "/campaigns?limit=5&offset=10")
	if got := strings.TrimSpace(string(body)); got != `{"items":[],"limit":5,"offset":10}` {
		t.Errorf("body = %s", got)
	}
}

func TestListCampaignsRejectsBadPaging(t *testing.T) {
	srv := serve(t, &fakeSource{})
	for _, q := range []string{"limit=0", "limit=101", "limit=x", "limit=-1", "offset=-1", "offset=100001", "offset=1.5"} {
		t.Run(q, func(t *testing.T) {
			resp, body := get(t, srv, "/campaigns?"+q)
			p := decodeProblem(t, resp, body, 422, CodeInvalidRequest)
			if len(p.Problems) != 1 {
				t.Errorf("problems = %v", p.Problems)
			}
		})
	}
	resp, body := get(t, srv, "/campaigns?limit=0&offset=-1")
	if p := decodeProblem(t, resp, body, 422, CodeInvalidRequest); len(p.Problems) != 2 {
		t.Errorf("problems = %v, want both parameters named", p.Problems)
	}
}

func TestCampaignSummaryShapes(t *testing.T) {
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	id := "0198a2c4-0000-7000-8000-000000000001"
	llm := LLMSummary{
		Calls: 3, CostUSD: "0.01000000", Tokens: Tokens{Input: 10, Output: 20, CacheRead: 30, CacheCreation: 40},
		ByPurpose: []PurposeUsage{{Purpose: "content", Calls: 3, CostUSD: "0.01000000", InputTokens: 10, OutputTokens: 20}},
	}

	t.Run("completed", func(t *testing.T) {
		srv := serve(t, &fakeSource{report: CampaignReport{
			CompletedAt: &at, SMSSegments: 6, DeliveryCostUSD: "0.242000", LLM: llm,
			Deliveries: []DeliveryCount{{Channel: "email", Status: "sent", Count: 2}, {Channel: "sms", Status: "sent", Count: 3}},
			Clicks: CampaignClicks{
				Sent: 5, UniqueClicks: 2, TotalClicks: 3, CTR: 0.4,
				ByChannel: []ChannelClicks{{Channel: "email", Sent: 2, UniqueClicks: 1, TotalClicks: 1, CTR: 0.5}, {Channel: "sms", Sent: 3, UniqueClicks: 1, TotalClicks: 2, CTR: 0.3333}},
				ByLocale:  []LocaleClicks{{Channel: "email", Language: "tr", Timezone: "Europe/Istanbul", Sent: 2, UniqueClicks: 1, TotalClicks: 1, CTR: 0.5}},
			},
		}})
		resp, body := get(t, srv, "/campaigns/"+id+"/summary")
		want := `{"campaign_id":"` + id + `","completed_at":"2026-10-29T09:00:00Z","deliveries":[{"channel":"email","status":"sent","count":2},{"channel":"sms","status":"sent","count":3}],"sms_segments":6,"delivery_cost_usd":"0.242000","llm":{"calls":3,"cost_usd":"0.01000000","tokens":{"input":10,"output":20,"cache_read":30,"cache_creation":40},"by_purpose":[{"purpose":"content","calls":3,"cost_usd":"0.01000000","input_tokens":10,"output_tokens":20}]},"clicks":{"sent":5,"unique_clicks":2,"total_clicks":3,"ctr":0.4,"by_channel":[{"channel":"email","sent":2,"unique_clicks":1,"total_clicks":1,"ctr":0.5},{"channel":"sms","sent":3,"unique_clicks":1,"total_clicks":2,"ctr":0.3333}],"by_locale":[{"channel":"email","language":"tr","timezone":"Europe/Istanbul","sent":2,"unique_clicks":1,"total_clicks":1,"ctr":0.5}]}}`
		if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != want {
			t.Errorf("status %d, body %s\nwant %s", resp.StatusCode, body, want)
		}
	})

	t.Run("not completed yet", func(t *testing.T) {
		srv := serve(t, &fakeSource{report: CampaignReport{Deliveries: []DeliveryCount{}, DeliveryCostUSD: "0.000000", LLM: llm}})
		resp, body := get(t, srv, "/campaigns/"+id+"/summary")
		var got map[string]json.RawMessage
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 || string(got["completed_at"]) != "null" || string(got["deliveries"]) != "[]" || string(got["sms_segments"]) != "0" {
			t.Errorf("status %d, body %s: want completed_at null, deliveries [] and 0 segments", resp.StatusCode, body)
		}
	})

	t.Run("unknown campaign", func(t *testing.T) {
		resp, body := get(t, serve(t, &fakeSource{reportErr: ErrNotFound}), "/campaigns/"+id+"/summary")
		decodeProblem(t, resp, body, 404, CodeNotFound)
	})

	t.Run("bad id", func(t *testing.T) {
		resp, body := get(t, serve(t, &fakeSource{}), "/campaigns/not-a-uuid/summary")
		decodeProblem(t, resp, body, 400, CodeInvalidID)
	})

	t.Run("source failure says nothing of the cause", func(t *testing.T) {
		resp, body := get(t, serve(t, &fakeSource{reportErr: errors.New("password=hunter2 refused")}), "/campaigns/"+id+"/summary")
		p := decodeProblem(t, resp, body, 500, CodeInternal)
		if strings.Contains(string(body), "hunter2") || strings.Contains(p.Detail, "refused") {
			t.Errorf("the 500 leaks the cause: %s", body)
		}
	})
}

func TestLLMUsage(t *testing.T) {
	src := &fakeSource{versions: []PromptVersionUsage{{
		PromptName: "content", PromptVersion: "1.2.0", Calls: 16, Accepted: 14, ValidationFailed: 1, Refusal: 0, APIError: 1,
		RepairRate: 0.0625, AvgLatencyMs: 812.5, CostUSD: "0.01234500",
	}}}
	srv := serve(t, src)
	want := `[{"prompt_name":"content","prompt_version":"1.2.0","calls":16,"accepted":14,"validation_failed":1,"refusal":0,"api_error":1,"repair_rate":0.0625,"avg_latency_ms":812.5,"cost_usd":"0.01234500"}]`
	for _, q := range []string{"", "?group=prompt_version"} {
		resp, body := get(t, srv, "/llm-usage"+q)
		if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != want {
			t.Errorf("GET /llm-usage%s: status %d, body %s\nwant %s", q, resp.StatusCode, body, want)
		}
	}
	for _, g := range []string{"model", "purpose", "Prompt_Version"} {
		before := src.versionsCalls
		resp, body := get(t, srv, "/llm-usage?group="+g)
		decodeProblem(t, resp, body, 422, CodeInvalidRequest)
		if src.versionsCalls != before {
			t.Errorf("group=%s reached the source", g)
		}
	}
	if _, body := get(t, serve(t, &fakeSource{}), "/llm-usage"); strings.TrimSpace(string(body)) != "[]" {
		t.Errorf("empty usage = %s, want []", body)
	}
}

func TestRepairRate(t *testing.T) {
	for _, tt := range []struct {
		failed, calls int64
		want          float64
	}{
		{0, 0, 0}, {0, 10, 0}, {1, 16, 0.0625}, {1, 3, 0.3333}, {2, 3, 0.6667}, {5, 5, 1},
	} {
		if got := RepairRate(tt.failed, tt.calls); got != tt.want {
			t.Errorf("RepairRate(%d, %d) = %v, want %v", tt.failed, tt.calls, got, tt.want)
		}
	}
}

func TestRangeParsing(t *testing.T) {
	src := &fakeSource{}
	srv := serve(t, src)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)

	for _, path := range []string{"/overview", "/llm-usage"} {
		t.Run(path+" no range", func(t *testing.T) {
			src.gotRange = Range{From: &from, To: &to}
			get(t, srv, path)
			if src.gotRange.From != nil || src.gotRange.To != nil {
				t.Errorf("range = %+v, want open on both ends", src.gotRange)
			}
		})
		t.Run(path+" open ended", func(t *testing.T) {
			get(t, srv, path+"?from=2026-10-01T00:00:00Z")
			if src.gotRange.From == nil || !src.gotRange.From.Equal(from) || src.gotRange.To != nil {
				t.Errorf("from only: range = %+v", src.gotRange)
			}
			get(t, srv, path+"?to=2026-10-08T00:00:00Z")
			if src.gotRange.From != nil || src.gotRange.To == nil || !src.gotRange.To.Equal(to) {
				t.Errorf("to only: range = %+v", src.gotRange)
			}
		})
		t.Run(path+" offsets become UTC", func(t *testing.T) {
			// 03:00+03:00 is 00:00Z.
			get(t, srv, path+"?from=2026-10-01T03:00:00%2B03:00&to=2026-10-08T00:00:00.5Z")
			if src.gotRange.From == nil || !src.gotRange.From.Equal(from) || src.gotRange.From.Location() != time.UTC ||
				src.gotRange.To == nil || !src.gotRange.To.Equal(to.Add(500*time.Millisecond)) {
				t.Errorf("range = %+v", src.gotRange)
			}
		})
		for _, q := range []string{
			"from=yesterday", "to=2026-10-08", "from=2026-10-01T00:00:00", "from=1759276800",
			"from=2026-10-08T00:00:00Z&to=2026-10-01T00:00:00Z", "from=2026-10-01T00:00:00Z&to=2026-10-01T00:00:00Z",
		} {
			t.Run(path+" rejects "+q, func(t *testing.T) {
				before := src.versionsCalls
				resp, body := get(t, srv, path+"?"+q)
				p := decodeProblem(t, resp, body, 422, CodeInvalidRequest)
				if len(p.Problems) == 0 || src.versionsCalls != before {
					t.Errorf("problems = %v, source reached = %v", p.Problems, src.versionsCalls != before)
				}
			})
		}
	}
	t.Run("both malformed are both named", func(t *testing.T) {
		resp, body := get(t, srv, "/overview?from=a&to=b")
		if p := decodeProblem(t, resp, body, 422, CodeInvalidRequest); len(p.Problems) != 2 {
			t.Errorf("problems = %v", p.Problems)
		}
	})
}

func TestOverviewShape(t *testing.T) {
	src := &fakeSource{overview: Overview{
		CampaignsCompleted: 2, Deliveries: []DeliveryCount{{Channel: "sms", Status: "sent", Count: 3}}, SMSSegments: 6, DeliveryCostUSD: "0.242000",
		LLM: OverviewLLM{
			Calls: 3, CostUSD: "0.01000000", Tokens: Tokens{Input: 10, Output: 20, CacheRead: 30, CacheCreation: 40},
			ByPurpose: []PurposeUsage{{Purpose: "content", Calls: 3, CostUSD: "0.01000000", InputTokens: 10, OutputTokens: 20}},
			ByModel:   []ModelUsage{{Model: "claude-sonnet-5-5", Calls: 3, CostUSD: "0.01000000", InputTokens: 10, OutputTokens: 20}},
		},
		Clicks: OverviewClicks{Sent: 3, UniqueClicks: 1, TotalClicks: 2, CTR: 0.3333, ByChannel: []ChannelClicks{{Channel: "sms", Sent: 3, UniqueClicks: 1, TotalClicks: 2, CTR: 0.3333}}},
	}}
	srv := serve(t, src)
	resp, body := get(t, srv, "/overview?from=2026-10-01T00:00:00Z")
	want := `{"from":"2026-10-01T00:00:00Z","to":null,"campaigns_completed":2,"deliveries":[{"channel":"sms","status":"sent","count":3}],"sms_segments":6,"delivery_cost_usd":"0.242000","llm":{"calls":3,"cost_usd":"0.01000000","tokens":{"input":10,"output":20,"cache_read":30,"cache_creation":40},"by_purpose":[{"purpose":"content","calls":3,"cost_usd":"0.01000000","input_tokens":10,"output_tokens":20}],"by_model":[{"model":"claude-sonnet-5-5","calls":3,"cost_usd":"0.01000000","input_tokens":10,"output_tokens":20}]},"clicks":{"sent":3,"unique_clicks":1,"total_clicks":2,"ctr":0.3333,"by_channel":[{"channel":"sms","sent":3,"unique_clicks":1,"total_clicks":2,"ctr":0.3333}]}}`
	if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != want {
		t.Errorf("status %d, body %s\nwant %s", resp.StatusCode, body, want)
	}
	resp, body = get(t, serve(t, &fakeSource{overviewErr: errors.New("boom")}), "/overview")
	decodeProblem(t, resp, body, 500, CodeInternal)
}

func TestSeriesParsing(t *testing.T) {
	src := &fakeSource{}
	srv := serve(t, src)
	day := 24 * time.Hour
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	iso := func(t time.Time) string { return t.Format(time.RFC3339) }

	for _, path := range []string{"/llm-usage/series", "/overview/series"} {
		ok := func(name, query string, bucket string, d time.Duration) {
			t.Run(path+" accepts "+name, func(t *testing.T) {
				src.seriesCalls = 0
				resp, body := get(t, srv, path+"?"+query)
				if resp.StatusCode != 200 || src.seriesCalls != 1 || src.gotSeries.Bucket != bucket ||
					!src.gotSeries.From.Equal(at) || !src.gotSeries.To.Equal(at.Add(d)) || src.gotSeries.From.Location() != time.UTC {
					t.Errorf("status %d, series %+v, body %s", resp.StatusCode, src.gotSeries, body)
				}
			})
		}
		q := func(d time.Duration, bucket string) string {
			return "from=" + iso(at) + "&to=" + iso(at.Add(d)) + "&bucket=" + bucket
		}
		ok("hour at 7 days", q(7*day, "hour"), "hour", 7*day)
		ok("hour at 1 hour", q(time.Hour, "hour"), "hour", time.Hour)
		ok("day at 400 days", q(400*day, "day"), "day", 400*day)
		ok("day at 1 day", q(day, "day"), "day", day)

		for name, query := range map[string]string{
			"missing from":      "to=" + iso(at.Add(day)) + "&bucket=day",
			"missing to":        "from=" + iso(at) + "&bucket=day",
			"missing bucket":    "from=" + iso(at) + "&to=" + iso(at.Add(day)),
			"nothing":           "",
			"bad from":          "from=x&to=" + iso(at.Add(day)) + "&bucket=day",
			"bad to":            "from=" + iso(at) + "&to=2026-10-02&bucket=day",
			"bad bucket":        "from=" + iso(at) + "&to=" + iso(at.Add(day)) + "&bucket=week",
			"bucket case":       "from=" + iso(at) + "&to=" + iso(at.Add(day)) + "&bucket=Day",
			"from equals to":    "from=" + iso(at) + "&to=" + iso(at) + "&bucket=day",
			"from after to":     "from=" + iso(at.Add(day)) + "&to=" + iso(at) + "&bucket=day",
			"hour over 7 days":  q(7*day+time.Second, "hour"),
			"hour over 30 days": q(30*day, "hour"),
			"day over 400 days": q(400*day+time.Second, "day"),
		} {
			t.Run(path+" rejects "+name, func(t *testing.T) {
				src.seriesCalls = 0
				resp, body := get(t, srv, path+"?"+query)
				p := decodeProblem(t, resp, body, 422, CodeInvalidRequest)
				if len(p.Problems) == 0 || src.seriesCalls != 0 {
					t.Errorf("problems = %v, source reached = %v", p.Problems, src.seriesCalls != 0)
				}
			})
		}
	}
}

func TestSeriesShapes(t *testing.T) {
	d1 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	d2 := d1.Add(24 * time.Hour)
	srv := serve(t, &fakeSource{
		llmSeries: LLMSeries{
			From: d1, To: d2.Add(24 * time.Hour), Bucket: "day", Buckets: []time.Time{d1, d2},
			Points: []LLMSeriesPoint{{Start: d1, Purpose: "content", Model: "claude-sonnet-5-5", Calls: 2, CostUSD: "0.01000000", InputTokens: 5, OutputTokens: 6}},
		},
		delivSeries: OverviewSeries{
			From: d1, To: d2.Add(24 * time.Hour), Bucket: "day", Buckets: []time.Time{d1, d2},
			Points: []OverviewSeriesPoint{{Start: d2, Channel: "sms", Status: "sent", Count: 4}},
			Clicks: []ClickSeriesPoint{{Start: d1, Channel: "email", UniqueClicks: 2, TotalClicks: 3}},
		},
	})
	const rng = "?from=2026-10-01T00:00:00Z&to=2026-10-03T00:00:00Z&bucket=day"
	_, body := get(t, srv, "/llm-usage/series"+rng)
	want := `{"from":"2026-10-01T00:00:00Z","to":"2026-10-03T00:00:00Z","bucket":"day","buckets":["2026-10-01T00:00:00Z","2026-10-02T00:00:00Z"],"points":[{"start":"2026-10-01T00:00:00Z","purpose":"content","model":"claude-sonnet-5-5","calls":2,"cost_usd":"0.01000000","input_tokens":5,"output_tokens":6}]}`
	if strings.TrimSpace(string(body)) != want {
		t.Errorf("llm series = %s\nwant %s", body, want)
	}
	_, body = get(t, srv, "/overview/series"+rng)
	want = `{"from":"2026-10-01T00:00:00Z","to":"2026-10-03T00:00:00Z","bucket":"day","buckets":["2026-10-01T00:00:00Z","2026-10-02T00:00:00Z"],"points":[{"start":"2026-10-02T00:00:00Z","channel":"sms","status":"sent","count":4}],"clicks":[{"start":"2026-10-01T00:00:00Z","channel":"email","unique_clicks":2,"total_clicks":3}]}`
	if strings.TrimSpace(string(body)) != want {
		t.Errorf("overview series = %s\nwant %s", body, want)
	}
}
