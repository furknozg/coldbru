// Package boot wires the notification service together (D-056, D-078, D-079): the outbox relay that
// publishes what the intake writes, the consumers of EVENTS, RECIPIENTS and SCHEDULES, and the
// send workers of SEND. cmd/notification passes Setup to the platform skeleton (internal/platform/app).
package boot

import (
	"context"
	"errors"
	"fmt"

	"github.com/furknozg/coldbru/internal/notification/consent"
	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/notification/provider"
	"github.com/furknozg/coldbru/internal/notification/sender"
	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

// Setup is the app.Options.Setup of the notification service. It needs the database and NATS, and
// PROVIDER_BASE_URL, CUSTOMER_GRAPHQL_URL and CUSTOMER_TOKEN. The relay, the consumers and the send
// workers stop when the service stops, after the HTTP server and before the database pool and the
// NATS connection: the send workers and the consumers first, then the relay, which keeps publishing
// until nothing can write to the outbox any more.
func Setup(ctx context.Context, d *app.Deps) error {
	if d.Pool == nil || d.JetStream == nil {
		return errors.New("notification needs a database and a NATS connection")
	}
	providerCfg, err := provider.LoadConfig(d.Config)
	if err != nil {
		return err
	}
	providerClient, err := provider.New(providerCfg)
	if err != nil {
		return err
	}
	consentCfg, err := consent.LoadConfig(d.Config)
	if err != nil {
		return err
	}
	consentCfg.Logger = d.Logger
	consentClient, err := consent.New(consentCfg)
	if err != nil {
		return err
	}

	// Hooks run in reverse order of registration: the relay is registered first, so it stops last.
	relay := &outbox.Relay{Pool: d.Pool, JS: d.JetStream, Logger: d.Logger}
	d.OnShutdown(relay.Start(ctx))

	stop, err := intake.Start(ctx, intake.Deps{JS: d.JetStream, Pool: d.Pool, Logger: d.Logger})
	if err != nil {
		return fmt.Errorf("intake: %w", err)
	}
	d.OnShutdown(stop)

	stopSend, err := sender.Start(ctx, sender.Deps{
		JS: d.JetStream, Pool: d.Pool, Logger: d.Logger,
		Provider: providerClient, Consent: consentClient,
		Registerer: d.Metrics.Registry(),
	})
	if err != nil {
		return fmt.Errorf("sender: %w", err)
	}
	d.OnShutdown(stopSend)
	d.Logger.Info("send workers configured", "provider", providerCfg.BaseURL, "customer", consentCfg.String())
	return nil
}
