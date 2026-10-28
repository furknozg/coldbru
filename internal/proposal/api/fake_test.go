package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// fakeFlow is an api.Flow whose every method is a settable function and records its arguments.
type fakeFlow struct {
	mu  sync.Mutex
	log []string

	hub *flow.Hub

	create        func(ctx context.Context, brief string) (flow.CampaignInfo, error)
	list          func(ctx context.Context, limit, offset int) ([]flow.CampaignInfo, error)
	get           func(ctx context.Context, id uuid.UUID) (*flow.CampaignView, error)
	approve       func(ctx context.Context, id uuid.UUID) error
	reject        func(ctx context.Context, id uuid.UUID, reason string) error
	edit          func(ctx context.Context, id uuid.UUID, content json.RawMessage) (flow.VariantView, error)
	addContext    func(ctx context.Context, id uuid.UUID, body string) (flow.NoteView, error)
	reopen        func(ctx context.Context, id uuid.UUID, kind string) (flow.StepInfo, error)
	regenCampaign func(ctx context.Context, id uuid.UUID) error
	regenStep     func(ctx context.Context, id uuid.UUID) error
	confirm       func(ctx context.Context, id uuid.UUID) error
	cancel        func(ctx context.Context, id uuid.UUID, reason string) error
	ofVariant     func(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	ofStep        func(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

var _ api.Flow = (*fakeFlow)(nil)

// note records a call, "name arg ..." .
func (f *fakeFlow) note(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, s)
}

func (f *fakeFlow) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.log...)
}

func (f *fakeFlow) Confirm(ctx context.Context, id uuid.UUID) error {
	f.note("confirm " + id.String())
	if f.confirm != nil {
		return f.confirm(ctx, id)
	}
	return nil
}

func (f *fakeFlow) Cancel(ctx context.Context, id uuid.UUID, reason string) error {
	f.note("cancel " + id.String() + " " + reason)
	if f.cancel != nil {
		return f.cancel(ctx, id, reason)
	}
	return nil
}

func (f *fakeFlow) Hub() *flow.Hub {
	if f.hub == nil {
		f.hub = flow.NewHub(0)
	}
	return f.hub
}

var (
	campID  = uuid.MustParse("019a0e6e-0000-7000-8000-00000000c0de")
	otherID = uuid.MustParse("019a0e6e-0000-7000-8000-0000000000aa")
)

func viewOf(id uuid.UUID) *flow.CampaignView {
	return &flow.CampaignView{
		CampaignInfo: flow.CampaignInfo{ID: id, Brief: "brief", Status: flow.CampaignStatusDrafting},
		Steps:        []flow.StepView{}, Notes: []flow.NoteView{},
	}
}

func (f *fakeFlow) CreateCampaign(ctx context.Context, brief string) (flow.CampaignInfo, error) {
	f.note("create " + brief)
	if f.create != nil {
		return f.create(ctx, brief)
	}
	return flow.CampaignInfo{ID: campID, Brief: brief, Status: flow.CampaignStatusDrafting}, nil
}

func (f *fakeFlow) ListCampaigns(ctx context.Context, limit, offset int) ([]flow.CampaignInfo, error) {
	f.note("list " + itoa(limit) + " " + itoa(offset))
	if f.list != nil {
		return f.list(ctx, limit, offset)
	}
	return nil, nil
}

func (f *fakeFlow) Get(ctx context.Context, id uuid.UUID) (*flow.CampaignView, error) {
	f.note("get " + id.String())
	if f.get != nil {
		return f.get(ctx, id)
	}
	return viewOf(id), nil
}

func (f *fakeFlow) Approve(ctx context.Context, id uuid.UUID) error {
	f.note("approve " + id.String())
	if f.approve != nil {
		return f.approve(ctx, id)
	}
	return nil
}

func (f *fakeFlow) Reject(ctx context.Context, id uuid.UUID, reason string) error {
	f.note("reject " + id.String() + " " + reason)
	if f.reject != nil {
		return f.reject(ctx, id, reason)
	}
	return nil
}

func (f *fakeFlow) Edit(ctx context.Context, id uuid.UUID, content json.RawMessage) (flow.VariantView, error) {
	f.note("edit " + id.String() + " " + string(content))
	if f.edit != nil {
		return f.edit(ctx, id, content)
	}
	return flow.VariantView{ID: otherID, Status: flow.VariantStatusDraft, Content: content}, nil
}

func (f *fakeFlow) AddContext(ctx context.Context, id uuid.UUID, body string) (flow.NoteView, error) {
	f.note("context " + id.String() + " " + body)
	if f.addContext != nil {
		return f.addContext(ctx, id, body)
	}
	return flow.NoteView{ID: otherID, Body: body}, nil
}

func (f *fakeFlow) Reopen(ctx context.Context, id uuid.UUID, kind string) (flow.StepInfo, error) {
	f.note("reopen " + id.String() + " " + kind)
	if f.reopen != nil {
		return f.reopen(ctx, id, kind)
	}
	return flow.StepInfo{ID: otherID, CampaignID: id, Kind: kind, Version: 2, Status: flow.StepStatusOpen}, nil
}

func (f *fakeFlow) RegenerateCampaign(ctx context.Context, id uuid.UUID) error {
	f.note("regenerate-campaign " + id.String())
	if f.regenCampaign != nil {
		return f.regenCampaign(ctx, id)
	}
	return nil
}

func (f *fakeFlow) Regenerate(ctx context.Context, id uuid.UUID) error {
	f.note("regenerate-step " + id.String())
	if f.regenStep != nil {
		return f.regenStep(ctx, id)
	}
	return nil
}

func (f *fakeFlow) CampaignOfVariant(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if f.ofVariant != nil {
		return f.ofVariant(ctx, id)
	}
	return campID, nil
}

func (f *fakeFlow) CampaignOfStep(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if f.ofStep != nil {
		return f.ofStep(ctx, id)
	}
	return campID, nil
}

func itoa(n int) string { return strconv.Itoa(n) }

// ---- helpers ----

// lockedBuffer is a bytes.Buffer safe for the logger of concurrent handlers.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// newRouter mounts a handler over f on a chi router and returns it with the log it writes.
func newRouter(t *testing.T, f *fakeFlow, mods ...func(*api.Config)) (http.Handler, *lockedBuffer) {
	t.Helper()
	logs := &lockedBuffer{}
	cfg := api.Config{Flow: f, Logger: slog.New(slog.NewTextHandler(logs, nil))}
	for _, m := range mods {
		m(&cfg)
	}
	r := chi.NewRouter()
	api.New(cfg).Mount(r)
	return r, logs
}

// call sends one request to h and returns the recorder.
func call(t *testing.T, h http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func postJSON(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return call(t, h, http.MethodPost, path, "application/json", body)
}

// decodeAs decodes the recorder body into T.
func decodeAs[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

// wantProblem checks the status, content type and code of an error response and returns it.
func wantProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) api.Problem {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != api.ProblemContentType {
		t.Errorf("content type = %q, want %q", ct, api.ProblemContentType)
	}
	p := decodeAs[api.Problem](t, rec)
	if p.Code != code || p.Status != status || p.Title == "" || p.Type != "urn:coldbru:proposal:problem:"+code {
		t.Errorf("problem = %+v, want code %s status %d", p, code, status)
	}
	return p
}
