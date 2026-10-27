// Package graph holds the Customer GraphQL resolvers (D-007, D-058). The schema
// is schema.graphqls; gqlgen generates the executable schema into generated/
// and the models into model/. This file is not regenerated.
package graph

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Resolver holds the dependencies of every resolver.
type Resolver struct {
	// Pool is the customer_db pool; nil until a resolver needs it (T-015).
	Pool *pgxpool.Pool
	// Logger is the service logger.
	Logger *slog.Logger
}

// NewResolver returns a Resolver. pool may be nil in tests.
func NewResolver(pool *pgxpool.Pool, logger *slog.Logger) *Resolver {
	return &Resolver{Pool: pool, Logger: logger}
}
