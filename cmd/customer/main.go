// Command customer is the customer service: sync GraphQL over customer_db, no NATS (D-011, D-043, D-056).
package main

import (
	"context"
	"fmt"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/platform/app"
)

func main() {
	app.Main(app.Options{Name: "customer", Database: true, Setup: setup})
}

// setup loads the city dictionary and mounts the GraphQL endpoint (D-058,
// D-062). Startup fails if the cities cannot be loaded: without them every
// city filter would be rejected. It also loads the caller tokens and puts the
// bearer-token middleware in front of /graphql only; startup fails if a token is
// missing or two callers share one (D-063). /healthz, /readyz and /metrics stay
// open (D-056).
func setup(ctx context.Context, d *app.Deps) error {
	tokens, err := auth.LoadTokenSet(d.Config)
	if err != nil {
		return fmt.Errorf("caller tokens: %w", err)
	}
	cities, err := audience.LoadCities(ctx, d.Pool)
	if err != nil {
		return fmt.Errorf("city dictionary: %w", err)
	}
	d.Logger.Info("city dictionary loaded", "cities", len(cities.Names()))
	svc := audience.NewService(d.Pool, cities)
	d.Router.Method("POST", "/graphql", tokens.Middleware(graph.NewHandler(graph.NewResolver(svc, d.Logger))))
	return nil
}
