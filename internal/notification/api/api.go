package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	// DefaultPollEvery is how often a stream reads the counts of its campaign (D-080).
	DefaultPollEvery = time.Second
	// DefaultPingEvery is the interval of the ping event of the stream.
	DefaultPingEvery = 15 * time.Second
)

// Config configures a Handler.
type Config struct {
	// Source is required.
	Source Source
	// Logger defaults to slog.Default.
	Logger *slog.Logger
	// Shutdown ends the open streams when it is done (the service passes the context of its Setup,
	// which a SIGTERM cancels); nil means never.
	Shutdown context.Context
	// PollEvery and PingEvery default to DefaultPollEvery and DefaultPingEvery.
	PollEvery time.Duration
	PingEvery time.Duration
	// Ticker replaces time.NewTicker for the poll and the ping, in tests: it returns the tick channel
	// and a stop function.
	Ticker func(d time.Duration) (<-chan time.Time, func())
	// Now replaces time.Now for the ping event, in tests.
	Now func() time.Time
}

// Handler serves the live delivery endpoints.
type Handler struct {
	src       Source
	log       *slog.Logger
	shutdown  <-chan struct{}
	pollEvery time.Duration
	pingEvery time.Duration
	ticker    func(d time.Duration) (<-chan time.Time, func())
	now       func() time.Time
}

// New returns a Handler for cfg.
func New(cfg Config) *Handler {
	h := &Handler{src: cfg.Source, log: cfg.Logger, pollEvery: cfg.PollEvery, pingEvery: cfg.PingEvery, ticker: cfg.Ticker, now: cfg.Now}
	if h.log == nil {
		h.log = slog.Default()
	}
	if cfg.Shutdown != nil {
		h.shutdown = cfg.Shutdown.Done()
	}
	if h.pollEvery <= 0 {
		h.pollEvery = DefaultPollEvery
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
	if h.now == nil {
		h.now = time.Now
	}
	return h
}

// Mount registers the endpoints on r.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/campaigns/{id}/deliveries", h.deliveries)
	r.Get("/campaigns/{id}/deliveries/events", h.events)
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

// deliveries godoc
//
//	@Summary		Delivery counts of a campaign
//	@Description	The current number of deliveries of the campaign per channel and status, read from the database at the moment of the request (D-036, D-080). `status` is the campaign status in Notification. Use GET /campaigns/{id}/deliveries/events to follow the counts live.
//	@Tags			deliveries
//	@Produce		json
//	@Param			id	path		string	true	"Campaign id"	format(uuid)
//	@Success		200	{object}	api.DeliverySummary
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem	"No such campaign"
//	@Failure		500	{object}	api.Problem
//	@Router			/campaigns/{id}/deliveries [get]
func (h *Handler) deliveries(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s, err := h.src.Summary(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}
