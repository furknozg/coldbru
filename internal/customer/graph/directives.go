package graph

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
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
