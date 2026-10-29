package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// ProblemContentType is the media type of every error body (RFC 9457).
const ProblemContentType = "application/problem+json"

// Problem codes: the stable machine-readable names the UI branches on. Each also forms the problem
// type, urn:coldbru:notification:problem:<code>.
const (
	CodeNotFound  = "not_found"
	CodeInvalidID = "invalid_id"
	CodeInternal  = "internal"
)

// Problem is an RFC 9457 problem details object, the body of every error response, sent with the
// media type application/problem+json (the OpenAPI document lists the error bodies under
// application/json because swag cannot name another media type per response). Code is an extension
// member: the stable machine-readable name of the problem.
type Problem struct {
	// Type identifies the problem type: urn:coldbru:notification:problem:<code>.
	Type string `json:"type" example:"urn:coldbru:notification:problem:not_found" validate:"required"`
	// Title is a short, fixed summary of the problem type.
	Title string `json:"title" example:"Not found" validate:"required"`
	// Status is the HTTP status code.
	Status int `json:"status" example:"404" validate:"required"`
	// Detail explains this occurrence. It never contains internal details.
	Detail string `json:"detail,omitempty" example:"No such campaign."`
	// Instance is the path of the request.
	Instance string `json:"instance,omitempty" example:"/campaigns/019a0e6e-0000-7000-8000-000000000001/deliveries"`
	// Code is the stable machine-readable name of the problem.
	Code string `json:"code" example:"not_found" validate:"required"`
}

type problemSpec struct {
	status int
	title  string
}

var specs = map[string]problemSpec{
	CodeNotFound:  {http.StatusNotFound, "Not found"},
	CodeInvalidID: {http.StatusBadRequest, "The identifier is not a UUID"},
	CodeInternal:  {http.StatusInternalServerError, "Internal server error"},
}

func newProblem(r *http.Request, code, detail string) Problem {
	sp := specs[code]
	return Problem{
		Type: "urn:coldbru:notification:problem:" + code, Title: sp.title, Status: sp.status,
		Detail: detail, Instance: r.URL.Path, Code: code,
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
func fail(w http.ResponseWriter, r *http.Request, code, detail string) {
	writeProblem(w, newProblem(r, code, detail))
}

// writeError maps an error of the Source to a problem. Anything it does not know is a 500 that says
// nothing about the cause; the cause is logged with the request id.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	if errors.Is(err, ErrNotFound) {
		fail(w, r, CodeNotFound, "No such campaign.")
		return
	}
	log.Error("api: request failed", "path", r.URL.Path, "request_id", middleware.GetReqID(r.Context()), "error", err)
	fail(w, r, CodeInternal, "Something went wrong; try again later.")
}
