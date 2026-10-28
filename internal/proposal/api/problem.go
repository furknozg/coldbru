package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// ProblemContentType is the media type of every error body (RFC 9457).
const ProblemContentType = "application/problem+json"

// Problem codes: the stable machine-readable names the UI branches on. Each also forms the problem
// type, urn:coldbru:proposal:problem:<code>.
const (
	CodeNotFound            = "not_found"
	CodeStepNotOpen         = "step_not_open"
	CodeVariantNotDraft     = "variant_not_draft"
	CodeCampaignNotDrafting = "campaign_not_drafting"
	CodeCampaignHasSteps    = "campaign_has_steps"
	CodeStepNotApproved     = "step_not_approved"
	CodeScheduleNotApproved = "schedule_not_approved"
	CodeNoRecipients        = "no_recipients"
	CodeNotCancellable      = "campaign_not_cancellable"
	CodeInvalidContent      = "invalid_content"
	CodeInvalidInput        = "invalid_input"
	CodeInvalidRequest      = "invalid_request"
	CodeMalformedJSON       = "malformed_json"
	CodeUnsupportedMedia    = "unsupported_media_type"
	CodePayloadTooLarge     = "payload_too_large"
	CodeInvalidID           = "invalid_id"
	CodeConfirmFailed       = "confirm_failed"
	CodeInternal            = "internal"
)

// Problem is an RFC 9457 problem details object, the body of every error response, sent with the
// media type application/problem+json (the OpenAPI document lists the error bodies under
// application/json because swag cannot name another media type per response). It has two extension
// members: Code, the stable machine-readable name of the problem, and Problems, the individual
// messages of a validation failure (for invalid content: one per broken rule).
type Problem struct {
	// Type identifies the problem type: urn:coldbru:proposal:problem:<code>.
	Type string `json:"type" example:"urn:coldbru:proposal:problem:step_not_open" validate:"required"`
	// Title is a short, fixed summary of the problem type.
	Title string `json:"title" example:"The step is not open" validate:"required"`
	// Status is the HTTP status code.
	Status int `json:"status" example:"409" validate:"required"`
	// Detail explains this occurrence. It never contains internal details.
	Detail string `json:"detail,omitempty" example:"The step is approved; reopen it to change it."`
	// Instance is the path of the request.
	Instance string `json:"instance,omitempty" example:"/variants/019a0e6e-0000-7000-8000-000000000001/approve"`
	// Code is the stable machine-readable name of the problem.
	Code string `json:"code" example:"step_not_open" validate:"required"`
	// Problems lists the individual validation messages of an invalid request or invalid content.
	Problems []string `json:"problems,omitempty"`
}

// problemSpec is the fixed part of a problem.
type problemSpec struct {
	status int
	title  string
}

var specs = map[string]problemSpec{
	CodeNotFound:            {http.StatusNotFound, "Not found"},
	CodeStepNotOpen:         {http.StatusConflict, "The step is not open"},
	CodeVariantNotDraft:     {http.StatusConflict, "The variant is not a draft"},
	CodeCampaignNotDrafting: {http.StatusConflict, "The campaign is not a draft"},
	CodeCampaignHasSteps:    {http.StatusConflict, "The campaign already has steps"},
	CodeStepNotApproved:     {http.StatusConflict, "The step is not approved"},
	CodeScheduleNotApproved: {http.StatusConflict, "The schedule is not approved"},
	CodeNoRecipients:        {http.StatusConflict, "The plan reaches no recipients"},
	CodeNotCancellable:      {http.StatusConflict, "The campaign cannot be cancelled"},
	CodeInvalidContent:      {http.StatusUnprocessableEntity, "The content is invalid"},
	CodeInvalidInput:        {http.StatusUnprocessableEntity, "The input is invalid"},
	CodeInvalidRequest:      {http.StatusUnprocessableEntity, "The request body is invalid"},
	CodeMalformedJSON:       {http.StatusBadRequest, "Malformed JSON"},
	CodeUnsupportedMedia:    {http.StatusUnsupportedMediaType, "Unsupported media type"},
	CodePayloadTooLarge:     {http.StatusRequestEntityTooLarge, "The request body is too large"},
	CodeInvalidID:           {http.StatusBadRequest, "The identifier is not a UUID"},
	CodeConfirmFailed:       {http.StatusBadGateway, "Confirming the campaign failed"},
	CodeInternal:            {http.StatusInternalServerError, "Internal server error"},
}

func newProblem(r *http.Request, code, detail string, problems ...string) Problem {
	sp := specs[code]
	return Problem{
		Type: "urn:coldbru:proposal:problem:" + code, Title: sp.title, Status: sp.status,
		Detail: detail, Instance: r.URL.Path, Code: code, Problems: problems,
	}
}

// writeProblem writes p as application/problem+json.
func writeProblem(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", ProblemContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	// The status line is gone already; a failed write only means the client left.
	_ = json.NewEncoder(w).Encode(p)
}

// fail answers a request with the problem code.
func fail(w http.ResponseWriter, r *http.Request, code, detail string, problems ...string) {
	writeProblem(w, newProblem(r, code, detail, problems...))
}

// writeError maps an error of the flow to a problem (D-072). Anything it does not know is a 500
// that says nothing about the cause; the cause is logged with the request id.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var ice *flow.InvalidContentError
	switch {
	case errors.As(err, &ice):
		fail(w, r, CodeInvalidContent, "The content did not pass validation.", ice.Problems...)
	case errors.Is(err, flow.ErrNotFound):
		fail(w, r, CodeNotFound, "No such campaign, step or variant.")
	case errors.Is(err, flow.ErrNotOpen):
		fail(w, r, CodeStepNotOpen, "The step is not open: it is approved, replaced, or not opened yet.")
	case errors.Is(err, flow.ErrNotDraft):
		fail(w, r, CodeVariantNotDraft, "The variant is not a draft: it is already approved, rejected or replaced.")
	case errors.Is(err, flow.ErrNotDrafting):
		fail(w, r, CodeCampaignNotDrafting, "The campaign is no longer a draft: it was cancelled or confirmed.")
	case errors.Is(err, flow.ErrHasSteps):
		fail(w, r, CodeCampaignHasSteps, "The campaign already has steps; regenerate its open step instead.")
	case errors.Is(err, flow.ErrNotApproved):
		fail(w, r, CodeStepNotApproved, "Only an approved step can be reopened.")
	case errors.Is(err, flow.ErrScheduleNotApproved):
		fail(w, r, CodeScheduleNotApproved, "The schedule step is not approved; approve every language's send time first.")
	case errors.Is(err, flow.ErrNoRecipients):
		fail(w, r, CodeNoRecipients, "The approved plan reaches nobody right now; reopen the audience or the channel plan.")
	case errors.Is(err, flow.ErrNotCancellable):
		fail(w, r, CodeNotCancellable, "The campaign is completed or already cancelled.")
	case errors.Is(err, flow.ErrInvalidInput):
		fail(w, r, CodeInvalidInput, strings.TrimPrefix(err.Error(), "flow: "))
	case errors.Is(err, flow.ErrConfirm):
		log.Error("api: confirming the campaign failed", "path", r.URL.Path, "request_id", middleware.GetReqID(r.Context()), "error", err)
		fail(w, r, CodeConfirmFailed, "The schedule was approved but confirming the campaign failed; the approval stands.")
	default:
		log.Error("api: request failed", "path", r.URL.Path, "request_id", middleware.GetReqID(r.Context()), "error", err)
		fail(w, r, CodeInternal, "Something went wrong; try again later.")
	}
}
