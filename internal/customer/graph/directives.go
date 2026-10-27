package graph

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/furknozg/coldbru/internal/customer/auth"
)

// OneOf implements the @oneOf input directive (D-058). The GraphQL validator
// enforces @oneOf for literal values only; a TimeValue sent through a variable
// (how Proposal sends filters) is not checked, so gqlgen calls this after
// unmarshaling every @oneOf input. obj is the raw input map: exactly one key
// must be present and its value must not be null.
func OneOf(ctx context.Context, obj any, next graphql.Resolver) (any, error) {
	fields, ok := obj.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("@oneOf: unexpected input type %T", obj)
	}
	set := 0
	for _, v := range fields {
		if v != nil {
			set++
		}
	}
	if set != 1 {
		return nil, &gqlerror.Error{
			Path:       graphql.GetPath(ctx),
			Message:    "OneOf Input Object must specify exactly one non-null field",
			Extensions: map[string]any{"code": CodeBadUserInput},
		}
	}
	return next(ctx)
}

// HasScope implements the @hasScope field directive (D-063). The caller was put
// into the context by auth.TokenSet.Middleware; if it lacks scope, or there is
// no caller at all (handler used without the middleware), only this field fails
// with FORBIDDEN and the resolver does not run.
func HasScope(ctx context.Context, _ any, next graphql.Resolver, scope string) (any, error) {
	if caller, ok := auth.FromContext(ctx); !ok || !caller.HasScope(scope) {
		return nil, &gqlerror.Error{
			Path:       graphql.GetPath(ctx),
			Message:    "forbidden",
			Extensions: map[string]any{"code": CodeForbidden},
		}
	}
	return next(ctx)
}
