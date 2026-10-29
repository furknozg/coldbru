// Package boot wires the notification service together (D-056, D-078): the outbox relay that
// publishes what the intake writes, and the consumers of EVENTS, RECIPIENTS and SCHEDULES.
// cmd/notification passes Setup to the platform skeleton (internal/platform/app).
package boot

import (
	"context"
	"errors"
	"fmt"

	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

// Setup is the app.Options.Setup of the notification service. It needs the database and NATS. The
// relay and the consumers stop when the service stops, after the HTTP server and before the
// database pool and the NATS connection: the consumers first, then the relay, which keeps publishing
// until no consumer can write to the outbox any more.
func Setup(ctx context.Context, d *app.Deps) error {
	if d.Pool == nil || d.JetStream == nil {
		return errors.New("notification needs a database and a NATS connection")
	}
	// Hooks run in reverse order of registration: the relay is registered first, so it stops last.
	relay := &outbox.Relay{Pool: d.Pool, JS: d.JetStream, Logger: d.Logger}
	d.OnShutdown(relay.Start(ctx))

	stop, err := intake.Start(ctx, intake.Deps{JS: d.JetStream, Pool: d.Pool, Logger: d.Logger})
	if err != nil {
		return fmt.Errorf("intake: %w", err)
	}
	d.OnShutdown(stop)
	return nil
}
