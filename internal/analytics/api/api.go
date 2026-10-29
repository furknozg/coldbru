package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Limits of the list request.
const (
	// DefaultListLimit and MaxListLimit bound GET /campaigns.
	DefaultListLimit = 20
	MaxListLimit     = 100
	// MaxListOffset keeps a list offset sane.
	MaxListOffset = 100000
)

// GroupPromptVersion is the only supported value of the group parameter of GET /llm-usage.
const GroupPromptVersion = "prompt_version"

// Config configures a Handler.
type Config struct {
	// Source is required.
	Source Source
	// Logger defaults to slog.Default.
	Logger *slog.Logger
}

// Handler serves the report endpoints.
type Handler struct {
	src Source
	log *slog.Logger
}

// New returns a Handler for cfg.
func New(cfg Config) *Handler {
	h := &Handler{src: cfg.Source, log: cfg.Logger}
	if h.log == nil {
		h.log = slog.Default()
	}
	return h
}

// Mount registers the endpoints on r.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/campaigns", h.listCampaigns)
	r.Get("/campaigns/{id}/summary", h.campaignSummary)
	r.Get("/llm-usage", h.llmUsage)
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The status line is gone already; a failed write only means the client left.
	_ = json.NewEncoder(w).Encode(v)
}

// listCampaigns godoc
//
//	@Summary		List completed campaigns
//	@Description	Returns one page of completed campaign summaries, the most recently completed first (D-083). A campaign appears once its completed event has been processed.
//	@Tags			campaigns
//	@Produce		json
//	@Param			limit	query		int	false	"Page size, 1 to 100"	default(20)	minimum(1)	maximum(100)
//	@Param			offset	query		int	false	"Campaigns to skip"		default(0)	minimum(0)	maximum(100000)
//	@Success		200		{object}	api.CampaignList
//	@Failure		422		{object}	api.Problem	"limit or offset out of range"
//	@Failure		500		{object}	api.Problem
//	@Router			/campaigns [get]
func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset := DefaultListLimit, 0
	var problems []string
	if s := q.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err != nil || n < 1 || n > MaxListLimit {
			problems = append(problems, "limit: must be a whole number from 1 to "+strconv.Itoa(MaxListLimit))
		} else {
			limit = n
		}
	}
	if s := q.Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err != nil || n < 0 || n > MaxListOffset {
			problems = append(problems, "offset: must be a whole number from 0 to "+strconv.Itoa(MaxListOffset))
		} else {
			offset = n
		}
	}
	if len(problems) > 0 {
		fail(w, r, CodeInvalidRequest, "The query parameters are out of range.", problems...)
		return
	}
	items, err := h.src.ListCampaigns(r.Context(), limit, offset)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if items == nil {
		items = []CampaignListItem{} // an empty page is [], never null
	}
	writeJSON(w, http.StatusOK, CampaignList{Items: items, Limit: limit, Offset: offset})
}

// campaignSummary godoc
//
//	@Summary		Summary of a campaign
//	@Description	Delivery counts per channel and status, SMS segments, the delivery cost estimate and the LLM usage of the campaign (D-083). A campaign with LLM usage whose completed event has not arrived has `completed_at` null and no deliveries. Money is a decimal string in US dollars.
//	@Tags			campaigns
//	@Produce		json
//	@Param			id	path		string	true	"Campaign id"	format(uuid)
//	@Success		200	{object}	api.CampaignReport
//	@Failure		400	{object}	api.Problem	"id is not a UUID"
//	@Failure		404	{object}	api.Problem	"Analytics knows nothing of the campaign"
//	@Failure		500	{object}	api.Problem
//	@Router			/campaigns/{id}/summary [get]
func (h *Handler) campaignSummary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, CodeInvalidID, "The identifier in the path is not a UUID.")
		return
	}
	rep, err := h.src.Campaign(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// llmUsage godoc
//
//	@Summary		LLM usage per prompt version
//	@Description	One entry per prompt name and version: the number of calls by outcome, the repair rate (validation_failed divided by calls), the mean latency and the cost in US dollars (D-060, D-083). Only `group=prompt_version` is supported. Ordered by prompt name, then version (as text).
//	@Tags			llm-usage
//	@Produce		json
//	@Param			group	query		string	false	"How to group the calls"	Enums(prompt_version)	default(prompt_version)
//	@Success		200		{array}		api.PromptVersionUsage
//	@Failure		422		{object}	api.Problem	"unsupported group"
//	@Failure		500		{object}	api.Problem
//	@Router			/llm-usage [get]
func (h *Handler) llmUsage(w http.ResponseWriter, r *http.Request) {
	if g := r.URL.Query().Get("group"); g != "" && g != GroupPromptVersion {
		fail(w, r, CodeInvalidRequest, "The group is not supported.", "group: only "+GroupPromptVersion+" is supported")
		return
	}
	items, err := h.src.PromptVersions(r.Context())
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if items == nil {
		items = []PromptVersionUsage{}
	}
	writeJSON(w, http.StatusOK, items)
}
