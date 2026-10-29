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

func (f *fakeSource) PromptVersions(context.Context) ([]PromptVersionUsage, error) {
	f.versionsCalls++
	return f.versions, f.versionsErr
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
		}})
		resp, body := get(t, srv, "/campaigns/"+id+"/summary")
		want := `{"campaign_id":"` + id + `","completed_at":"2026-10-29T09:00:00Z","deliveries":[{"channel":"email","status":"sent","count":2},{"channel":"sms","status":"sent","count":3}],"sms_segments":6,"delivery_cost_usd":"0.242000","llm":{"calls":3,"cost_usd":"0.01000000","tokens":{"input":10,"output":20,"cache_read":30,"cache_creation":40},"by_purpose":[{"purpose":"content","calls":3,"cost_usd":"0.01000000","input_tokens":10,"output_tokens":20}]}}`
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
