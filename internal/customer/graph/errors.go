package graph

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/furknozg/coldbru/internal/customer/audience"
)

// CodeNotImplemented is the extensions.code of the placeholder resolvers.
const CodeNotImplemented = "NOT_IMPLEMENTED"

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
	// CodeQueryTimeout means the audience query ran into the 5 second
	// statement timeout; narrow the filter.
	CodeQueryTimeout = "QUERY_TIMEOUT"
	// CodeInternal is any other failure; the details stay in the server log.
	CodeInternal = "INTERNAL"
)

// errNotImplemented is returned by the consent resolvers until T-017 implements
// them. The GraphQL error carries the path of the field and extensions.code
// NOT_IMPLEMENTED.
func errNotImplemented(ctx context.Context) error {
	return &gqlerror.Error{
		Path:       graphql.GetPath(ctx),
		Message:    "not implemented",
		Extensions: map[string]any{"code": CodeNotImplemented},
	}
}

// audienceError converts an error of the audience package into the GraphQL
// error clients see. Filter errors keep their code and details and add
// extensions.filterPath, the location inside the filter argument; anything
// else is logged and answered with a generic message.
func (r *Resolver) audienceError(ctx context.Context, err error) error {
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
	r.Logger.ErrorContext(ctx, "audience query failed", "error", err)
	if audience.IsTimeout(err) {
		return &gqlerror.Error{
			Path:       graphql.GetPath(ctx),
			Message:    "audience query timed out; narrow the filter",
			Extensions: map[string]any{"code": CodeQueryTimeout},
		}
	}
	return &gqlerror.Error{
		Path:       graphql.GetPath(ctx),
		Message:    "internal error",
		Extensions: map[string]any{"code": CodeInternal},
	}
}
