// Package boot wires the analytics service together (D-056, D-083): the consumer of the EVENTS
// stream that keeps campaign summaries and LLM usage, and the report routes.
// cmd/analytics passes Setup to the platform skeleton (internal/platform/app).
package boot

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/furknozg/coldbru/internal/analytics/api"
	"github.com/furknozg/coldbru/internal/analytics/intake"
	"github.com/furknozg/coldbru/internal/analytics/pricing"
	"github.com/furknozg/coldbru/internal/platform/app"
)

// Setup is the app.Options.Setup of the analytics service. It needs the database and NATS;
// LLM_PRICES optionally overrides the model price table (see pricing.KeyLLMPrices). The consumer
// stops when the service stops, after the HTTP server and before the database pool and the NATS
// connection.
func Setup(ctx context.Context, d *app.Deps) error {
	if d.Pool == nil || d.JetStream == nil {
		return errors.New("analytics needs a database and a NATS connection")
	}
	prices, err := pricing.LoadConfig(d.Config)
	if err != nil {
		return err
	}
	stop, err := intake.Start(ctx, intake.Deps{JS: d.JetStream, Pool: d.Pool, Logger: d.Logger, Prices: prices})
	if err != nil {
		return fmt.Errorf("intake: %w", err)
	}
	d.OnShutdown(stop)

	api.New(api.Config{Source: api.NewPoolSource(d.Pool), Logger: d.Logger}).Mount(d.Router)

	models := make([]string, 0, len(prices))
	for m := range prices {
		models = append(models, m)
	}
	sort.Strings(models)
	d.Logger.Info("analytics configured", "priced_models", models)
	return nil
}
