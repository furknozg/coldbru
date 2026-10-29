package mockprovider

import (
	"encoding/json"
	"net/http"
)

// ProblemContentType is the media type of every error body (RFC 9457).
const ProblemContentType = "application/problem+json"

// Problem codes: stable machine-readable names. Each also forms the problem type,
// urn:coldbru:mockprovider:problem:<code>.
const (
	CodeMissingKey      = "missing_idempotency_key"
	CodeInvalidKey      = "invalid_idempotency_key"
	CodeInvalidJSON     = "invalid_json"
	CodeTooLarge        = "request_too_large"
	CodeValidation      = "validation_failed"
	CodeKeyConflict     = "idempotency_key_reused"
	CodeRateLimited     = "rate_limited"
	CodeInjectedFailure = "provider_unavailable"
	CodeForwardFailed   = "inbox_unavailable"
)

// Problem is an RFC 9457 problem details object, the body of every error response, sent with the
// media type application/problem+json (the OpenAPI document lists the error bodies under
// application/json because swag cannot name another media type per response). Code is an extension
// member: the stable machine-readable name of the problem.
type Problem struct {
	// Type identifies the problem type: urn:coldbru:mockprovider:problem:<code>.
	Type string `json:"type" example:"urn:coldbru:mockprovider:problem:validation_failed" validate:"required"`
	// Title is a short, fixed summary of the problem type.
	Title string `json:"title" example:"Validation failed" validate:"required"`
	// Status is the HTTP status code.
	Status int `json:"status" example:"422" validate:"required"`
	// Detail explains this occurrence.
	Detail string `json:"detail,omitempty" example:"to must be an E.164 phone number such as +905551112233."`
	// Instance is the path of the request.
	Instance string `json:"instance,omitempty" example:"/v1/sms"`
	// Code is the stable machine-readable name of the problem.
	Code string `json:"code" example:"validation_failed" validate:"required"`
}

var titles = map[string]string{
	CodeMissingKey:      "Idempotency-Key header is required",
	CodeInvalidKey:      "Idempotency-Key header is invalid",
	CodeInvalidJSON:     "The body is not the expected JSON",
	CodeTooLarge:        "The request body is too large",
	CodeValidation:      "Validation failed",
	CodeKeyConflict:     "The Idempotency-Key was used with another body",
	CodeRateLimited:     "Rate limit exceeded",
	CodeInjectedFailure: "The provider is unavailable",
	CodeForwardFailed:   "The inbox is unavailable",
}

// writeProblem answers with an RFC 9457 body; header values of extra are set before the status.
func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string, extra http.Header) {
	h := w.Header()
	for k, v := range extra {
		h[k] = v
	}
	h.Set("Content-Type", ProblemContentType)
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The status line is gone already; a failed write only means the client left.
	_ = json.NewEncoder(w).Encode(Problem{
		Type: "urn:coldbru:mockprovider:problem:" + code, Title: titles[code], Status: status,
		Detail: detail, Instance: r.URL.Path, Code: code,
	})
}
