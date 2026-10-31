package api

import (
	"encoding/json"
	"net/http"
)

// ProblemContentType is the media type of every error body (RFC 9457).
const ProblemContentType = "application/problem+json"

// Problem codes; each also forms the problem type, urn:coldbru:redirect:problem:<code>.
const (
	CodeInvalidJSON    = "invalid_json"
	CodeInvalidRequest = "invalid_request"
	CodeInternal       = "internal"
)

// Problem is an RFC 9457 problem details object, the body of every error response of POST /links.
type Problem struct {
	// Type identifies the problem type: urn:coldbru:redirect:problem:<code>.
	Type string `json:"type" example:"urn:coldbru:redirect:problem:invalid_request" validate:"required"`
	// Title is a short, fixed summary of the problem type.
	Title string `json:"title" example:"The request is invalid" validate:"required"`
	// Status is the HTTP status code.
	Status int `json:"status" example:"422" validate:"required"`
	// Detail explains this occurrence. It never contains internal details.
	Detail string `json:"detail,omitempty"`
	// Instance is the path of the request.
	Instance string `json:"instance,omitempty" example:"/links"`
	// Code is the stable machine-readable name of the problem.
	Code string `json:"code" example:"invalid_request" validate:"required"`
	// Problems lists the individual validation messages of an invalid request.
	Problems []string `json:"problems,omitempty"`
}

var problemSpecs = map[string]struct {
	status int
	title  string
}{
	CodeInvalidJSON:    {http.StatusBadRequest, "The body is not valid JSON"},
	CodeInvalidRequest: {http.StatusUnprocessableEntity, "The request is invalid"},
	CodeInternal:       {http.StatusInternalServerError, "Internal server error"},
}

func fail(w http.ResponseWriter, r *http.Request, code, detail string, problems ...string) {
	sp := problemSpecs[code]
	p := Problem{
		Type: "urn:coldbru:redirect:problem:" + code, Title: sp.title, Status: sp.status,
		Detail: detail, Instance: r.URL.Path, Code: code, Problems: problems,
	}
	w.Header().Set("Content-Type", ProblemContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	// The status line is gone already; a failed write only means the client left.
	_ = json.NewEncoder(w).Encode(p)
}
