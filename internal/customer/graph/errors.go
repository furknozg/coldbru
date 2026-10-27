package graph

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/furknozg/coldbru/internal/customer/audience"
)

// CodeForbidden is the extensions.code of a field the caller's token has no
// scope for (D-063).
const CodeForbidden = "FORBIDDEN"

// CodeBadUserInput is the extensions.code of input errors found at execution
// time, after the GraphQL validator has passed the document.
const CodeBadUserInput = audience.CodeBadUserInput

// Codes of the audience errors (D-062).
const (
	CodeUnknownValue    = audience.CodeUnknownValue
	CodeComplexityLimit = audience.CodeComplexityLimit
	// CodeNotFound is an unknown customer id in updateConsent (D-064).
	CodeNotFound = audience.CodeNotFound
	// CodeQueryTimeout means the query ran into the 5 second
	// statement timeout; narrow the filter.
	CodeQueryTimeout = "QUERY_TIMEOUT"
	// CodeInternal is any other failure; the details stay in the server log.
	CodeInternal = "INTERNAL"
)

// serviceError converts an error of the audience or consent package into the
// GraphQL error clients see; what names the operation in the log. Client errors
// keep their code and details, and filter errors add extensions.filterPath, the
// location inside the filter argument; anything else is logged and answered
// with a generic message.
func (r *Resolver) serviceError(ctx context.Context, what string, err error) error {
	var ae *audience.Error
	if errors.As(err, &ae) {
		ext := map[string]any{"code": ae.Code}
		for k, v := range ae.Extensions {
			ext[k] = v
		}
		if len(ae.FilterPath) > 0 {
			ext["filterPath"] = ae.FilterPath
		}
		return &gqlerror.Error{Path: graphql.GetPath(ctx), Message: ae.Message, Extensions: ext}
	}
	r.Logger.ErrorContext(ctx, what+" failed", "error", err)
	if audience.IsTimeout(err) {
		return &gqlerror.Error{
			Path:       graphql.GetPath(ctx),
			Message:    "query timed out; narrow the request",
			Extensions: map[string]any{"code": CodeQueryTimeout},
		}
	}
	return &gqlerror.Error{
		Path:       graphql.GetPath(ctx),
		Message:    "internal error",
		Extensions: map[string]any{"code": CodeInternal},
	}
}
