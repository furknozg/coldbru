package graph

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// CodeNotImplemented is the extensions.code of the placeholder resolvers.
const CodeNotImplemented = "NOT_IMPLEMENTED"

// CodeBadUserInput is the extensions.code of input errors found at execution
// time, after the GraphQL validator has passed the document.
const CodeBadUserInput = "BAD_USER_INPUT"

// errNotImplemented is returned by every resolver until T-015 (audience) and
// T-017 (consent) implement them. The GraphQL error carries the path of the
// field and extensions.code NOT_IMPLEMENTED.
func errNotImplemented(ctx context.Context) error {
	return &gqlerror.Error{
		Path:       graphql.GetPath(ctx),
		Message:    "not implemented",
		Extensions: map[string]any{"code": CodeNotImplemented},
	}
}
