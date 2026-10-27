// Command customer is the customer service: sync GraphQL over customer_db, no NATS (D-011, D-043, D-056).
package main

import (
	"context"

	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/platform/app"
)

func main() {
	app.Main(app.Options{Name: "customer", Database: true, Setup: setup})
}

// setup mounts the GraphQL endpoint (D-058). Auth is added by T-016 (D-043).
func setup(_ context.Context, d *app.Deps) error {
	d.Router.Method("POST", "/graphql", graph.NewHandler(graph.NewResolver(d.Pool, d.Logger)))
	return nil
}
