package graph

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"github.com/furknozg/coldbru/internal/customer/graph/generated"
)

// NewHandler returns the GraphQL HTTP handler for POST /graphql. Only the POST
// transport is enabled: no GET, websocket, multipart, introspection extension
// or playground. Proposal builds its catalog from the build-time schema, so
// introspection is not needed (D-044). The handler does not authenticate:
// mount it behind auth.TokenSet.Middleware, which puts the caller into the
// context for the @hasScope directive (D-063). Without a caller every guarded
// field answers FORBIDDEN.
func NewHandler(r *Resolver) http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers:  r,
		Directives: generated.DirectiveRoot{OneOf: OneOf, HasScope: HasScope},
	}))
	srv.AddTransport(transport.POST{})
	return srv
}
