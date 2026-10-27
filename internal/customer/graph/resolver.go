// Package graph holds the Customer GraphQL resolvers (D-007, D-058). The schema
// is schema.graphqls; gqlgen generates the executable schema into generated/
// and the models into model/. This file is not regenerated.
package graph

import (
	"log/slog"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/consent"
)

// Resolver holds the dependencies of every resolver.
type Resolver struct {
	// Audience compiles filters and runs the audience queries (D-062).
	Audience *audience.Service
	// Consent reads and changes per-channel consent (D-064).
	Consent *consent.Service
	// Logger is the service logger.
	Logger *slog.Logger
}

// NewResolver returns a Resolver over the audience and consent services.
func NewResolver(aud *audience.Service, cons *consent.Service, logger *slog.Logger) *Resolver {
	return &Resolver{Audience: aud, Consent: cons, Logger: logger}
}
