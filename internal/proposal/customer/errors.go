package customer

import (
	"errors"
	"fmt"

	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// Codes of Customer's GraphQL errors (extensions.code) this client treats specially.
const (
	CodeForbidden       = "FORBIDDEN"
	CodeQueryTimeout    = "QUERY_TIMEOUT"
	CodeInternal        = "INTERNAL"
	CodeBadUserInput    = "BAD_USER_INPUT"
	CodeUnknownValue    = "UNKNOWN_VALUE"
	CodeComplexityLimit = "COMPLEXITY_LIMIT"
)

// UnauthorizedError is HTTP 401: Customer does not know the token (D-063). It is a configuration
// problem, not a retry or repair case.
type UnauthorizedError struct{}

// Error implements error.
func (*UnauthorizedError) Error() string {
	return "customer: unauthorized: the service token was rejected (check " + KeyToken + ")"
}

// StatusError is an HTTP response that is neither a success nor a GraphQL error body: a 4xx
// other than 401, or a 5xx other than 502, 503 and 504, and 502/503/504 once the attempts are
// used up.
type StatusError struct {
	Status int
	// Body is the start of the response body, for diagnostics.
	Body string
}

// Error implements error.
func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("customer: unexpected HTTP status %d", e.Status)
	}
	return fmt.Sprintf("customer: unexpected HTTP status %d: %s", e.Status, e.Body)
}

// Error is a GraphQL error of Customer that is not a filter-value problem the model can repair:
// FORBIDDEN, QUERY_TIMEOUT, INTERNAL, an error without a filter path, or a filter error of another
// code (for example BAD_USER_INPUT, which then also carries the DSL span when it could be found).
type Error struct {
	Code    string
	Message string
	// Path is the response path of the failed field ("path" of the GraphQL error).
	Path []any
	// FilterPath is extensions.filterPath: where in the filter variable the problem is.
	FilterPath []any
	// Span is the DSL range FilterPath maps to; nil when there is no filter path or no mapping.
	Span *dsl.Span
	// Extensions is the error's extensions object, including code.
	Extensions map[string]any
}

// Error implements error.
func (e *Error) Error() string {
	if e.Code == "" {
		return "customer: " + e.Message
	}
	return "customer: " + e.Code + ": " + e.Message
}

// DSLErrors returns the *dsl.Error values inside err, however it is wrapped or joined, in order.
// The repair loop feeds them back to the model; it is empty when err holds none.
func DSLErrors(err error) []*dsl.Error {
	var out []*dsl.Error
	var walk func(error)
	walk = func(err error) {
		switch x := err.(type) {
		case nil:
			return
		case *dsl.Error:
			out = append(out, x)
			return
		case interface{ Unwrap() []error }:
			for _, e := range x.Unwrap() {
				walk(e)
			}
		case interface{ Unwrap() error }:
			walk(x.Unwrap())
		}
	}
	walk(err)
	return out
}

// IsUnauthorized reports whether err is a rejected token.
func IsUnauthorized(err error) bool {
	var u *UnauthorizedError
	return errors.As(err, &u)
}
