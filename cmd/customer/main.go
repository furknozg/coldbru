// Command customer is the customer service: sync GraphQL over customer_db, no NATS (D-011, D-043, D-056).
package main

import (
	"context"
	"fmt"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/platform/app"
)

func main() {
	app.Main(app.Options{Name: "customer", Database: true, Setup: setup})
}

// setup loads the city dictionary and mounts the GraphQL endpoint (D-058,
// D-062). Startup fails if the cities cannot be loaded: without them every
// city filter would be rejected. Auth is added by T-016 (D-043).
func setup(ctx context.Context, d *app.Deps) error {
	cities, err := audience.LoadCities(ctx, d.Pool)
	if err != nil {
		return fmt.Errorf("city dictionary: %w", err)
	}
	d.Logger.Info("city dictionary loaded", "cities", len(cities.Names()))
	svc := audience.NewService(d.Pool, cities)
	d.Router.Method("POST", "/graphql", graph.NewHandler(graph.NewResolver(svc, d.Logger)))
	return nil
}
