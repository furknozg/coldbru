package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// Limits of the requests (D-072).
const (
	// MaxBodyBytes is the size limit of a request body.
	MaxBodyBytes = 64 << 10
	// MaxBriefChars is the longest brief, in Unicode characters. The sample brief has about 140.
	MaxBriefChars = 2000
	// MaxNoteChars is the longest context note.
	MaxNoteChars = 2000
	// MaxReasonChars is the longest rejection reason.
	MaxReasonChars = 1000
	// MaxAnswerChars is the longest answer to a question of the agent (D-088).
	MaxAnswerChars = 1000
	// DefaultListLimit and MaxListLimit bound GET /campaigns.
	DefaultListLimit = 20
	MaxListLimit     = 100
	// MaxListOffset keeps a list offset sane.
	MaxListOffset = 100000
)

// DefaultPingEvery is the interval of the ping event of the event stream.
const DefaultPingEvery = 15 * time.Second

// Flow is what the handlers need from the proposal flow. *flow.Service implements it; tests use a
// fake.
type Flow interface {
	CreateCampaign(ctx context.Context, brief string) (flow.CampaignInfo, error)
	ListCampaigns(ctx context.Context, limit, offset int) ([]flow.CampaignInfo, error)
	Get(ctx context.Context, campaignID uuid.UUID) (*flow.CampaignView, error)
	Approve(ctx context.Context, variantID uuid.UUID) error
	Reject(ctx context.Context, variantID uuid.UUID, reason string) error
	Edit(ctx context.Context, variantID uuid.UUID, content json.RawMessage) (flow.VariantView, error)
	AddContext(ctx context.Context, campaignID uuid.UUID, body string) (flow.NoteView, error)
	Reopen(ctx context.Context, campaignID uuid.UUID, kind string) (flow.StepInfo, error)
	RegenerateCampaign(ctx context.Context, campaignID uuid.UUID) error
	Regenerate(ctx context.Context, stepID uuid.UUID) error
	Confirm(ctx context.Context, campaignID uuid.UUID) error
	Cancel(ctx context.Context, campaignID uuid.UUID, reason string) error
	Answer(ctx context.Context, questionID uuid.UUID, answer string) (flow.ChatMessageView, error)
	PreviewEmail(ctx context.Context, variantID uuid.UUID, edited json.RawMessage) (flow.EmailPreview, error)
	CampaignOfVariant(ctx context.Context, variantID uuid.UUID) (uuid.UUID, error)
	CampaignOfStep(ctx context.Context, stepID uuid.UUID) (uuid.UUID, error)
	// Hub is the event hub the event stream subscribes to.
	Hub() *flow.Hub
}

var _ Flow = (*flow.Service)(nil)

// Config configures a Handler.
type Config struct {
	// Flow is required.
	Flow Flow
	// Logger defaults to slog.Default.
	Logger *slog.Logger
	// Shutdown ends the open event streams when it is done (the service passes the context of its
	// Setup, which a SIGTERM cancels); nil means never.
	Shutdown context.Context
	// PingEvery is the interval of the ping event; DefaultPingEvery when zero.
	PingEvery time.Duration
	// Ticker replaces time.NewTicker for the ping, in tests: it returns the tick channel and a stop
	// function.
	Ticker func(d time.Duration) (<-chan time.Time, func())
}

// Handler serves the REST endpoints and the event stream.
type Handler struct {
	flow      Flow
	log       *slog.Logger
	shutdown  <-chan struct{}
	pingEvery time.Duration
	ticker    func(d time.Duration) (<-chan time.Time, func())
}

// New returns a Handler for cfg.
func New(cfg Config) *Handler {
	h := &Handler{flow: cfg.Flow, log: cfg.Logger, pingEvery: cfg.PingEvery, ticker: cfg.Ticker}
	if h.log == nil {
		h.log = slog.Default()
	}
	if cfg.Shutdown != nil {
		h.shutdown = cfg.Shutdown.Done()
	}
	if h.pingEvery <= 0 {
		h.pingEvery = DefaultPingEvery
	}
	if h.ticker == nil {
		h.ticker = func(d time.Duration) (<-chan time.Time, func()) {
			t := time.NewTicker(d)
			return t.C, t.Stop
		}
	}
	return h
}

// Mount registers the endpoints on r.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/campaigns", h.listCampaigns)
	r.Post("/campaigns", h.createCampaign)
	r.Get("/campaigns/{id}", h.getCampaign)
	r.Get("/campaigns/{id}/events", h.events)
	r.Post("/campaigns/{id}/context", h.addContext)
	r.Post("/campaigns/{id}/reopen", h.reopen)
	r.Post("/campaigns/{id}/regenerate", h.regenerateCampaign)
	r.Post("/campaigns/{id}/confirm", h.confirm)
	r.Post("/campaigns/{id}/cancel", h.cancel)
	r.Post("/variants/{id}/approve", h.approve)
	r.Post("/variants/{id}/reject", h.reject)
	r.Post("/variants/{id}/edit", h.edit)
	r.Post("/variants/{id}/preview", h.preview)
	r.Post("/steps/{id}/regenerate", h.regenerateStep)
	r.Post("/messages/{id}/answer", h.answer)
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The status line is gone already; a failed write only means the client left.
	_ = json.NewEncoder(w).Encode(v)
}

// pathID reads the {id} path parameter. A malformed one is answered here and ok is false.
func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, CodeInvalidID, "The identifier in the path is not a UUID.")
		return uuid.Nil, false
	}
	return id, true
}
