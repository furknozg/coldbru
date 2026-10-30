package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

const sampleBrief = "Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder."

func TestCreateCampaignAccepts(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	body, _ := json.Marshal(map[string]string{"brief": "  " + sampleBrief + "\n"})
	rec := call(t, h, http.MethodPost, "/campaigns", "application/json; charset=utf-8", string(body))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/campaigns/"+campID.String() {
		t.Errorf("Location = %q", loc)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q", ct)
	}
	info := decodeAs[flow.CampaignInfo](t, rec)
	if info.ID != campID || info.Brief != sampleBrief {
		t.Errorf("info = %+v: the brief must reach the flow trimmed", info)
	}
	if got := f.calls(); len(got) != 1 || got[0] != "create "+sampleBrief {
		t.Errorf("calls = %q", got)
	}
}

func TestRequestValidation(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("ö", api.MaxBriefChars+1)
	atLimit := strings.Repeat("ö", api.MaxBriefChars)
	huge := `{"brief":"` + strings.Repeat("a", api.MaxBodyBytes) + `"}`
	tests := []struct {
		name        string
		contentType string
		body        string
		status      int
		code        string
		problem     string // substring of problems[0], when set
	}{
		{"valid at the character cap", "application/json", `{"brief":"` + atLimit + `"}`, 202, "", ""},
		{"no content type", "", `{"brief":"x"}`, 415, api.CodeUnsupportedMedia, ""},
		{"form content type", "application/x-www-form-urlencoded", `brief=x`, 415, api.CodeUnsupportedMedia, ""},
		{"text content type", "text/plain", `{"brief":"x"}`, 415, api.CodeUnsupportedMedia, ""},
		{"empty body", "application/json", ``, 400, api.CodeMalformedJSON, ""},
		{"truncated JSON", "application/json", `{"brief":`, 400, api.CodeMalformedJSON, ""},
		{"not JSON", "application/json", `brief`, 400, api.CodeMalformedJSON, ""},
		{"two values", "application/json", `{"brief":"x"}{"brief":"y"}`, 400, api.CodeMalformedJSON, ""},
		{"trailing garbage", "application/json", `{"brief":"x"} nope`, 400, api.CodeMalformedJSON, ""},
		{"too large", "application/json", huge, 413, api.CodePayloadTooLarge, ""},
		{"unknown field", "application/json", `{"brief":"x","channel":"sms"}`, 422, api.CodeInvalidRequest, `unknown field "channel"`},
		{"wrong type", "application/json", `{"brief":12}`, 422, api.CodeInvalidRequest, "brief: expected string"},
		{"array body", "application/json", `["x"]`, 422, api.CodeInvalidRequest, ""},
		{"missing brief", "application/json", `{}`, 422, api.CodeInvalidRequest, "brief: must not be empty"},
		{"blank brief", "application/json", `{"brief":"  \n\t "}`, 422, api.CodeInvalidRequest, "brief: must not be empty"},
		{"null brief", "application/json", `{"brief":null}`, 422, api.CodeInvalidRequest, "brief: must not be empty"},
		{"brief one character too long", "application/json", `{"brief":"` + long + `"}`, 422, api.CodeInvalidRequest, "at most 2000 characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeFlow{}
			h, _ := newRouter(t, f)
			rec := call(t, h, http.MethodPost, "/campaigns", tt.contentType, tt.body)
			if tt.code == "" {
				if rec.Code != tt.status {
					t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
				}
				return
			}
			p := wantProblem(t, rec, tt.status, tt.code)
			if tt.problem != "" && (len(p.Problems) == 0 || !strings.Contains(p.Problems[0], tt.problem)) {
				t.Errorf("problems = %q, want one with %q", p.Problems, tt.problem)
			}
			if p.Instance != "/campaigns" {
				t.Errorf("instance = %q", p.Instance)
			}
			if calls := f.calls(); len(calls) != 0 {
				t.Errorf("the flow was called for a bad request: %q", calls)
			}
		})
	}
}

func TestCreateCampaignSurfacesFlowErrors(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{create: func(context.Context, string) (flow.CampaignInfo, error) {
		return flow.CampaignInfo{}, fmt.Errorf("%w: the brief is empty", flow.ErrInvalidInput)
	}}
	h, _ := newRouter(t, f)
	p := wantProblem(t, postJSON(t, h, "/campaigns", `{"brief":"x"}`), 422, api.CodeInvalidInput)
	if p.Detail != "invalid input: the brief is empty" {
		t.Errorf("detail = %q", p.Detail)
	}
}

func TestListCampaigns(t *testing.T) {
	t.Parallel()
	t.Run("defaults and an empty page", func(t *testing.T) {
		t.Parallel()
		f := &fakeFlow{}
		h, _ := newRouter(t, f)
		rec := call(t, h, http.MethodGet, "/campaigns", "", "")
		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Errorf("body = %s: an empty page must be []", rec.Body.String())
		}
		list := decodeAs[api.CampaignList](t, rec)
		if list.Limit != api.DefaultListLimit || list.Offset != 0 {
			t.Errorf("list = %+v", list)
		}
		if got := f.calls(); len(got) != 1 || got[0] != "list 20 0" {
			t.Errorf("calls = %q", got)
		}
	})
	t.Run("a page", func(t *testing.T) {
		t.Parallel()
		f := &fakeFlow{list: func(_ context.Context, limit, offset int) ([]flow.CampaignInfo, error) {
			return []flow.CampaignInfo{{ID: campID, Brief: "a"}, {ID: otherID, Brief: "b"}}, nil
		}}
		h, _ := newRouter(t, f)
		rec := call(t, h, http.MethodGet, "/campaigns?limit=100&offset=40", "", "")
		list := decodeAs[api.CampaignList](t, rec)
		if rec.Code != 200 || len(list.Items) != 2 || list.Items[0].ID != campID || list.Limit != 100 || list.Offset != 40 {
			t.Errorf("status %d list %+v", rec.Code, list)
		}
		if got := f.calls(); got[0] != "list 100 40" {
			t.Errorf("calls = %q", got)
		}
	})
	for _, q := range []string{"limit=0", "limit=101", "limit=-1", "limit=abc", "limit=1.5", "offset=-1", "offset=abc", "offset=100001", "limit=0&offset=-1"} {
		t.Run("rejects "+q, func(t *testing.T) {
			t.Parallel()
			f := &fakeFlow{}
			h, _ := newRouter(t, f)
			p := wantProblem(t, call(t, h, http.MethodGet, "/campaigns?"+q, "", ""), 422, api.CodeInvalidRequest)
			if len(p.Problems) == 0 {
				t.Error("no problems listed")
			}
			if len(f.calls()) != 0 {
				t.Error("the flow was called")
			}
		})
	}
}

func TestGetCampaign(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	rec := call(t, h, http.MethodGet, "/campaigns/"+campID.String(), "", "")
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if v := decodeAs[flow.CampaignView](t, rec); v.ID != campID || v.Steps == nil || v.Notes == nil {
		t.Errorf("view = %+v", v)
	}
	f.get = func(context.Context, uuid.UUID) (*flow.CampaignView, error) { return nil, flow.ErrNotFound }
	wantProblem(t, call(t, h, http.MethodGet, "/campaigns/"+otherID.String(), "", ""), 404, api.CodeNotFound)
}

func TestInvalidPathIDs(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	for _, tt := range []struct{ method, path, body string }{
		{"GET", "/campaigns/not-a-uuid", ""},
		{"GET", "/campaigns/not-a-uuid/events", ""},
		{"POST", "/campaigns/12345/context", `{"body":"x"}`},
		{"POST", "/campaigns/12345/reopen", `{"kind":"audience"}`},
		{"POST", "/campaigns/12345/regenerate", ""},
		{"POST", "/variants/x/approve", ""},
		{"POST", "/variants/x/reject", `{"reason":"x"}`},
		{"POST", "/variants/x/edit", `{"content":{}}`},
		{"POST", "/steps/x/regenerate", ""},
	} {
		ct := ""
		if tt.body != "" {
			ct = "application/json"
		}
		p := wantProblem(t, call(t, h, tt.method, tt.path, ct, tt.body), 400, api.CodeInvalidID)
		if p.Instance != tt.path {
			t.Errorf("%s %s: instance = %q", tt.method, tt.path, p.Instance)
		}
	}
	if len(f.calls()) != 0 {
		t.Errorf("the flow was called: %q", f.calls())
	}
}

func TestErrorMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", flow.ErrNotFound, 404, api.CodeNotFound},
		{"wrapped not found", fmt.Errorf("lookup: %w", flow.ErrNotFound), 404, api.CodeNotFound},
		{"not open", flow.ErrNotOpen, 409, api.CodeStepNotOpen},
		{"not draft", flow.ErrNotDraft, 409, api.CodeVariantNotDraft},
		{"not drafting", flow.ErrNotDrafting, 409, api.CodeCampaignNotDrafting},
		{"has steps", flow.ErrHasSteps, 409, api.CodeCampaignHasSteps},
		{"not approved", flow.ErrNotApproved, 409, api.CodeStepNotApproved},
		{"schedule not approved", flow.ErrScheduleNotApproved, 409, api.CodeScheduleNotApproved},
		{"no recipients", flow.ErrNoRecipients, 409, api.CodeNoRecipients},
		{"no recipients inside a failed confirmation", fmt.Errorf("%w: %w", flow.ErrConfirm, flow.ErrNoRecipients), 409, api.CodeNoRecipients},
		{"schedule not approved inside a failed confirmation", fmt.Errorf("%w: %w", flow.ErrConfirm, flow.ErrScheduleNotApproved), 409, api.CodeScheduleNotApproved},
		{"not drafting inside a failed confirmation", fmt.Errorf("%w: %w", flow.ErrConfirm, flow.ErrNotDrafting), 409, api.CodeCampaignNotDrafting},
		{"not cancellable", flow.ErrNotCancellable, 409, api.CodeNotCancellable},
		{"not a question", flow.ErrNotQuestion, 409, api.CodeNotQuestion},
		{"already answered", flow.ErrAlreadyAnswered, 409, api.CodeAlreadyAnswered},
		{"invalid input", fmt.Errorf("%w: a rejection needs a reason", flow.ErrInvalidInput), 422, api.CodeInvalidInput},
		{"confirm failed", fmt.Errorf("%w: %w", flow.ErrConfirm, errors.New("outbox: connection refused")), 502, api.CodeConfirmFailed},
		{"unknown", errors.New("pq: password authentication failed for user proposal_svc"), 500, api.CodeInternal},
		{"no content generator", flow.ErrNoContentGenerator, 500, api.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeFlow{approve: func(context.Context, uuid.UUID) error { return tt.err }}
			h, logs := newRouter(t, f)
			path := "/variants/" + otherID.String() + "/approve"
			rec := call(t, h, http.MethodPost, path, "", "")
			p := wantProblem(t, rec, tt.status, tt.code)
			if p.Instance != path {
				t.Errorf("instance = %q", p.Instance)
			}
			body := rec.Body.String()
			for _, secret := range []string{"password", "proposal_svc", "outbox", "connection refused", "flow: "} {
				if strings.Contains(body, secret) {
					t.Errorf("the body leaks %q: %s", secret, body)
				}
			}
			if tt.status >= 500 && !strings.Contains(logs.String(), "error=") {
				t.Errorf("a %d must be logged with its cause, log = %q", tt.status, logs.String())
			}
			if tt.status < 500 && strings.Contains(logs.String(), "level=ERROR") {
				t.Errorf("a %d is the client's problem and must not be logged as an error: %q", tt.status, logs.String())
			}
		})
	}
}

func TestInternalErrorLogsTheCause(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{get: func(context.Context, uuid.UUID) (*flow.CampaignView, error) {
		return nil, errors.New("pq: relation \"campaign\" is broken")
	}}
	h, logs := newRouter(t, f)
	p := wantProblem(t, call(t, h, http.MethodGet, "/campaigns/"+campID.String(), "", ""), 500, api.CodeInternal)
	if strings.Contains(p.Detail, "campaign") || strings.Contains(p.Detail, "pq") {
		t.Errorf("detail %q leaks the cause", p.Detail)
	}
	if !strings.Contains(logs.String(), `relation \"campaign\" is broken`) {
		t.Errorf("the cause is not logged: %q", logs.String())
	}
}

func TestApprove(t *testing.T) {
	t.Parallel()
	vid := otherID
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	rec := call(t, h, http.MethodPost, "/variants/"+vid.String()+"/approve", "", "")
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if v := decodeAs[flow.CampaignView](t, rec); v.ID != campID {
		t.Errorf("view = %+v: want the campaign of the variant", v)
	}
	if got := strings.Join(f.calls(), "|"); got != "approve "+vid.String()+"|get "+campID.String() {
		t.Errorf("calls = %s", got)
	}

	// An unknown variant is a 404 before anything is approved.
	f2 := &fakeFlow{ofVariant: func(context.Context, uuid.UUID) (uuid.UUID, error) { return uuid.Nil, flow.ErrNotFound }}
	h2, _ := newRouter(t, f2)
	wantProblem(t, call(t, h2, http.MethodPost, "/variants/"+vid.String()+"/approve", "", ""), 404, api.CodeNotFound)
	if len(f2.calls()) != 0 {
		t.Errorf("calls = %q", f2.calls())
	}
}

func TestApproveConfirmFailureIsABadGateway(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{approve: func(context.Context, uuid.UUID) error {
		return fmt.Errorf("%w: %w", flow.ErrConfirm, errors.New("nats down"))
	}}
	h, _ := newRouter(t, f)
	p := wantProblem(t, call(t, h, http.MethodPost, "/variants/"+otherID.String()+"/approve", "", ""), 502, api.CodeConfirmFailed)
	if !strings.Contains(p.Detail, "approval stands") {
		t.Errorf("detail = %q", p.Detail)
	}
}

func TestReject(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	path := "/variants/" + otherID.String() + "/reject"
	rec := postJSON(t, h, path, `{"reason":"  Çok resmi.  "}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if v := decodeAs[flow.CampaignView](t, rec); v.ID != campID {
		t.Errorf("view = %+v", v)
	}
	if got := f.calls()[0]; got != "reject "+otherID.String()+" Çok resmi." {
		t.Errorf("call = %q: the reason must arrive trimmed", got)
	}
	for body, want := range map[string]string{
		`{}`:            "reason: must not be empty",
		`{"reason":""}`: "reason: must not be empty",
		`{"reason":"` + strings.Repeat("x", api.MaxReasonChars+1) + `"}`: "at most 1000",
		`{"reason":"x","extra":1}`:                                       "unknown field",
	} {
		p := wantProblem(t, postJSON(t, h, path, body), 422, api.CodeInvalidRequest)
		if len(p.Problems) == 0 || !strings.Contains(p.Problems[0], want) {
			t.Errorf("body %.40s: problems = %q, want %q", body, p.Problems, want)
		}
	}
	// Conflicts from the flow.
	f.reject = func(context.Context, uuid.UUID, string) error { return flow.ErrNotDraft }
	wantProblem(t, postJSON(t, h, path, `{"reason":"x"}`), 409, api.CodeVariantNotDraft)
}

func TestEdit(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	path := "/variants/" + otherID.String() + "/edit"
	rec := postJSON(t, h, path, `{"content":{"text":"Merhaba {{first_name}}"}}`)
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := f.calls()[0]; got != `edit `+otherID.String()+` {"text":"Merhaba {{first_name}}"}` {
		t.Errorf("call = %q: the content must reach the flow untouched", got)
	}
	if v := decodeAs[flow.VariantView](t, rec); v.ID != otherID || v.Status != flow.VariantStatusDraft {
		t.Errorf("variant = %+v", v)
	}

	p := wantProblem(t, postJSON(t, h, path, `{}`), 422, api.CodeInvalidRequest)
	if len(p.Problems) != 1 || p.Problems[0] != "content: is required" {
		t.Errorf("problems = %q", p.Problems)
	}
	wantProblem(t, postJSON(t, h, path, `{"content":{},"x":1}`), 422, api.CodeInvalidRequest)

	// Invalid content lists every broken rule.
	f.edit = func(context.Context, uuid.UUID, json.RawMessage) (flow.VariantView, error) {
		return flow.VariantView{}, &flow.InvalidContentError{Problems: []string{"sms.text: 171 characters, the limit is 160", "sms.text: contains a URL"}}
	}
	p = wantProblem(t, postJSON(t, h, path, `{"content":{"text":"x"}}`), 422, api.CodeInvalidContent)
	if len(p.Problems) != 2 || p.Problems[0] != "sms.text: 171 characters, the limit is 160" {
		t.Errorf("problems = %q", p.Problems)
	}
	// A wrapped invalid-content error still maps.
	f.edit = func(context.Context, uuid.UUID, json.RawMessage) (flow.VariantView, error) {
		return flow.VariantView{}, fmt.Errorf("edit: %w", &flow.InvalidContentError{Problems: []string{"p"}})
	}
	wantProblem(t, postJSON(t, h, path, `{"content":{"text":"x"}}`), 422, api.CodeInvalidContent)
	f.edit = func(context.Context, uuid.UUID, json.RawMessage) (flow.VariantView, error) {
		return flow.VariantView{}, flow.ErrNotOpen
	}
	wantProblem(t, postJSON(t, h, path, `{"content":{"text":"x"}}`), 409, api.CodeStepNotOpen)
}

func TestAddContext(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	path := "/campaigns/" + campID.String() + "/context"
	rec := postJSON(t, h, path, `{"body":" Hafta sonu kampanyası. "}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if n := decodeAs[flow.NoteView](t, rec); n.Body != "Hafta sonu kampanyası." {
		t.Errorf("note = %+v", n)
	}
	wantProblem(t, postJSON(t, h, path, `{"body":" "}`), 422, api.CodeInvalidRequest)
	wantProblem(t, postJSON(t, h, path, `{"body":"`+strings.Repeat("x", api.MaxNoteChars+1)+`"}`), 422, api.CodeInvalidRequest)
	f.addContext = func(context.Context, uuid.UUID, string) (flow.NoteView, error) {
		return flow.NoteView{}, flow.ErrNotOpen
	}
	wantProblem(t, postJSON(t, h, path, `{"body":"x"}`), 409, api.CodeStepNotOpen)
	f.addContext = func(context.Context, uuid.UUID, string) (flow.NoteView, error) {
		return flow.NoteView{}, flow.ErrNotDrafting
	}
	wantProblem(t, postJSON(t, h, path, `{"body":"x"}`), 409, api.CodeCampaignNotDrafting)
}

func TestReopen(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	path := "/campaigns/" + campID.String() + "/reopen"
	rec := postJSON(t, h, path, `{"kind":"channel_plan"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if s := decodeAs[flow.StepInfo](t, rec); s.Kind != "channel_plan" || s.Version != 2 {
		t.Errorf("step = %+v", s)
	}
	f.reopen = func(context.Context, uuid.UUID, string) (flow.StepInfo, error) {
		return flow.StepInfo{}, flow.ErrNotApproved
	}
	wantProblem(t, postJSON(t, h, path, `{"kind":"audience"}`), 409, api.CodeStepNotApproved)
	f.reopen = func(_ context.Context, _ uuid.UUID, kind string) (flow.StepInfo, error) {
		return flow.StepInfo{}, fmt.Errorf("%w: unknown step kind %q", flow.ErrInvalidInput, kind)
	}
	p := wantProblem(t, postJSON(t, h, path, `{"kind":"nope"}`), 422, api.CodeInvalidInput)
	if !strings.Contains(p.Detail, `unknown step kind "nope"`) {
		t.Errorf("detail = %q", p.Detail)
	}
	wantProblem(t, postJSON(t, h, path, `{"kind":3}`), 422, api.CodeInvalidRequest)
}

func TestRegenerate(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	for _, path := range []string{"/campaigns/" + campID.String() + "/regenerate", "/steps/" + otherID.String() + "/regenerate"} {
		rec := call(t, h, http.MethodPost, path, "", "")
		if rec.Code != http.StatusAccepted {
			t.Fatalf("%s: status = %d: %s", path, rec.Code, rec.Body.String())
		}
		if v := decodeAs[flow.CampaignView](t, rec); v.ID != campID {
			t.Errorf("%s: view = %+v", path, v)
		}
	}
	if got := strings.Join(f.calls(), "|"); got != "regenerate-campaign "+campID.String()+"|get "+campID.String()+"|regenerate-step "+otherID.String()+"|get "+campID.String() {
		t.Errorf("calls = %s", got)
	}
	f.regenCampaign = func(context.Context, uuid.UUID) error { return flow.ErrHasSteps }
	wantProblem(t, call(t, h, http.MethodPost, "/campaigns/"+campID.String()+"/regenerate", "", ""), 409, api.CodeCampaignHasSteps)
	f.ofStep = func(context.Context, uuid.UUID) (uuid.UUID, error) { return uuid.Nil, flow.ErrNotFound }
	wantProblem(t, call(t, h, http.MethodPost, "/steps/"+otherID.String()+"/regenerate", "", ""), 404, api.CodeNotFound)
	f.ofStep = nil
	f.regenStep = func(context.Context, uuid.UUID) error { return flow.ErrNotOpen }
	wantProblem(t, call(t, h, http.MethodPost, "/steps/"+otherID.String()+"/regenerate", "", ""), 409, api.CodeStepNotOpen)
}

func TestResponsesAreNotCached(t *testing.T) {
	t.Parallel()
	h, _ := newRouter(t, &fakeFlow{})
	for _, rec := range []interface{ Header() http.Header }{
		call(t, h, http.MethodGet, "/campaigns/"+campID.String(), "", ""),
		call(t, h, http.MethodGet, "/campaigns/x", "", ""),
	} {
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control = %q", cc)
		}
	}
}

func TestConfirmEndpoint(t *testing.T) {
	t.Parallel()
	path := "/campaigns/" + campID.String() + "/confirm"
	t.Run("returns the campaign", func(t *testing.T) {
		t.Parallel()
		f := &fakeFlow{}
		h, _ := newRouter(t, f)
		rec := call(t, h, http.MethodPost, path, "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
		}
		if v := decodeAs[flow.CampaignView](t, rec); v.ID != campID {
			t.Errorf("view = %+v", v)
		}
		if got := f.calls(); len(got) != 2 || got[0] != "confirm "+campID.String() || got[1] != "get "+campID.String() {
			t.Errorf("calls = %q, want confirm then get", got)
		}
	})
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", flow.ErrNotFound, 404, api.CodeNotFound},
		{"already scheduled or cancelled", flow.ErrNotDrafting, 409, api.CodeCampaignNotDrafting},
		{"schedule not approved", fmt.Errorf("%w: %w", flow.ErrConfirm, flow.ErrScheduleNotApproved), 409, api.CodeScheduleNotApproved},
		{"no recipients", fmt.Errorf("%w: %w", flow.ErrConfirm, flow.ErrNoRecipients), 409, api.CodeNoRecipients},
		{"customer failed", fmt.Errorf("%w: %w", flow.ErrConfirm, errors.New("customer: unexpected HTTP status 500")), 502, api.CodeConfirmFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeFlow{confirm: func(context.Context, uuid.UUID) error { return tt.err }}
			h, _ := newRouter(t, f)
			p := wantProblem(t, call(t, h, http.MethodPost, path, "", ""), tt.status, tt.code)
			if strings.Contains(p.Detail, "customer") {
				t.Errorf("detail %q leaks the cause", p.Detail)
			}
		})
	}
	t.Run("bad id", func(t *testing.T) {
		t.Parallel()
		wantProblem(t, call(t, mustRouter(t), http.MethodPost, "/campaigns/nope/confirm", "", ""), 400, api.CodeInvalidID)
	})
}

func mustRouter(t *testing.T) http.Handler {
	t.Helper()
	h, _ := newRouter(t, &fakeFlow{})
	return h
}

func TestCancelEndpoint(t *testing.T) {
	t.Parallel()
	path := "/campaigns/" + campID.String() + "/cancel"
	tests := []struct {
		name        string
		contentType string
		body        string
		wantCall    string // the reason the flow receives; "-" when it is not called
		status      int
		code        string
	}{
		{"no body", "", "", "cancel " + campID.String() + " ", 200, ""},
		{"whitespace body without a media type", "", " \n", "cancel " + campID.String() + " ", 200, ""},
		{"empty object", "application/json", `{}`, "cancel " + campID.String() + " ", 200, ""},
		{"reason is trimmed", "application/json", `{"reason":"  wrong text \n"}`, "cancel " + campID.String() + " wrong text", 200, ""},
		{"reason at the cap", "application/json", `{"reason":"` + strings.Repeat("ö", api.MaxReasonChars) + `"}`, "cancel " + campID.String() + " " + strings.Repeat("ö", api.MaxReasonChars), 200, ""},
		{"reason too long", "application/json", `{"reason":"` + strings.Repeat("ö", api.MaxReasonChars+1) + `"}`, "-", 422, api.CodeInvalidRequest},
		{"body without a media type", "", `{"reason":"x"}`, "-", 415, api.CodeUnsupportedMedia},
		{"malformed", "application/json", `{"reason":`, "-", 400, api.CodeMalformedJSON},
		{"unknown field", "application/json", `{"why":"x"}`, "-", 422, api.CodeInvalidRequest},
		{"wrong type", "application/json", `{"reason":3}`, "-", 422, api.CodeInvalidRequest},
		{"too large", "application/json", `{"reason":"` + strings.Repeat("a", api.MaxBodyBytes) + `"}`, "-", 413, api.CodePayloadTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeFlow{}
			h, _ := newRouter(t, f)
			rec := call(t, h, http.MethodPost, path, tt.contentType, tt.body)
			calls := f.calls()
			if tt.code != "" {
				wantProblem(t, rec, tt.status, tt.code)
				if len(calls) != 0 {
					t.Errorf("the flow was called for a bad request: %q", calls)
				}
				return
			}
			if rec.Code != tt.status {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			if len(calls) != 2 || calls[0] != tt.wantCall || calls[1] != "get "+campID.String() {
				t.Errorf("calls = %q, want %q then get", calls, tt.wantCall)
			}
		})
	}
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", flow.ErrNotFound, 404, api.CodeNotFound},
		{"completed or already cancelled", flow.ErrNotCancellable, 409, api.CodeNotCancellable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeFlow{cancel: func(context.Context, uuid.UUID, string) error { return tt.err }}
			h, _ := newRouter(t, f)
			wantProblem(t, call(t, h, http.MethodPost, path, "", ""), tt.status, tt.code)
		})
	}
}

func TestAnswer(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	h, _ := newRouter(t, f)
	path := "/messages/" + otherID.String() + "/answer"
	rec := postJSON(t, h, path, `{"answer":"  Yalnızca son 30 günde alışveriş yapanlar  "}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	// The body is the campaign the answer's message named.
	if v := decodeAs[flow.CampaignView](t, rec); v.ID != campID {
		t.Errorf("view = %+v", v)
	}
	if got := f.calls(); len(got) != 2 || got[0] != "answer "+otherID.String()+" Yalnızca son 30 günde alışveriş yapanlar" || got[1] != "get "+campID.String() {
		t.Errorf("calls = %q: the answer must arrive trimmed, then the view is read", got)
	}
	for body, want := range map[string]string{
		`{}`:            "answer: must not be empty",
		`{"answer":""}`: "answer: must not be empty",
		`{"answer":"` + strings.Repeat("x", api.MaxAnswerChars+1) + `"}`: "at most 1000",
		`{"answer":"x","option":1}`:                                      "unknown field",
	} {
		p := wantProblem(t, postJSON(t, h, path, body), 422, api.CodeInvalidRequest)
		if len(p.Problems) == 0 || !strings.Contains(p.Problems[0], want) {
			t.Errorf("body %.40s: problems = %q, want %q", body, p.Problems, want)
		}
	}
	wantProblem(t, call(t, h, http.MethodPost, path, "", ""), 415, api.CodeUnsupportedMedia)
	wantProblem(t, postJSON(t, h, "/messages/not-a-uuid/answer", `{"answer":"x"}`), 400, api.CodeInvalidID)
	// Errors of the flow.
	for err, want := range map[error]struct {
		status int
		code   string
	}{
		flow.ErrNotFound:        {404, api.CodeNotFound},
		flow.ErrNotQuestion:     {409, api.CodeNotQuestion},
		flow.ErrAlreadyAnswered: {409, api.CodeAlreadyAnswered},
		flow.ErrNotOpen:         {409, api.CodeStepNotOpen},
		flow.ErrNotDrafting:     {409, api.CodeCampaignNotDrafting},
	} {
		f.answer = func(context.Context, uuid.UUID, string) (flow.ChatMessageView, error) {
			return flow.ChatMessageView{}, err
		}
		wantProblem(t, postJSON(t, h, path, `{"answer":"x"}`), want.status, want.code)
	}
}
