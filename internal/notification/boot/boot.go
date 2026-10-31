// Package boot wires the notification service together (D-056, D-078, D-079, D-080): the outbox
// relay that publishes what the intake writes, the consumers of EVENTS, RECIPIENTS and SCHEDULES,
// the send workers of SEND, the completion checker and the live delivery routes.
// cmd/notification passes Setup to the platform skeleton (internal/platform/app).
package boot

import (
	"context"
	"errors"
	"fmt"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/notification/api"
	"github.com/furknozg/coldbru/internal/notification/consent"
	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/notification/links"
	"github.com/furknozg/coldbru/internal/notification/progress"
	"github.com/furknozg/coldbru/internal/notification/provider"
	"github.com/furknozg/coldbru/internal/notification/sender"
	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

// Setup is the app.Options.Setup of the notification service. It needs the database and NATS, and
// PROVIDER_BASE_URL, CUSTOMER_GRAPHQL_URL, CUSTOMER_TOKEN and REDIRECT_URL; TRACKING_BASE_URL sets the tracking links and the SMS text budget (D-106, D-109); SMS_SEGMENT_PRICE_USD and
// EMAIL_PRICE_USD price the completed event and have demo defaults. The relay, the consumers, the
// send workers and the completion checker stop when the service stops, after the HTTP server and
// before the database pool and the NATS connection: the checker and the send workers first, then
// the consumers, then the relay, which keeps publishing until nothing can write to the outbox any
// more.
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

	contentCfg, err := content.LoadConfig(d.Config)
	if err != nil {
		return fmt.Errorf("content config: %w", err)
	}
	linksCfg, err := links.LoadConfig(d.Config)
	if err != nil {
		return err
	}
	linksClient, err := links.New(linksCfg)
	if err != nil {
		return err
	}
	checker, err := progress.New(progress.Config{Pool: d.Pool, Logger: d.Logger, Prices: contentCfg.Prices})
	if err != nil {
		return err
	}

	// Hooks run in reverse order of registration: the relay is registered first, so it stops last.
	relay := &outbox.Relay{Pool: d.Pool, JS: d.JetStream, Logger: d.Logger}
	d.OnShutdown(relay.Start(ctx))

	stop, err := intake.Start(ctx, intake.Deps{JS: d.JetStream, Pool: d.Pool, Logger: d.Logger, Links: linksClient})
	if err != nil {
		return fmt.Errorf("intake: %w", err)
	}
	d.OnShutdown(stop)

	stopSend, err := sender.Start(ctx, sender.Deps{
		JS: d.JetStream, Pool: d.Pool, Logger: d.Logger,
		Provider: providerClient, Consent: consentClient,
		Registerer: d.Metrics.Registry(), Nudge: checker.Nudge, TrackingBaseURL: contentCfg.TrackingBaseURL,
	})
	if err != nil {
		return fmt.Errorf("sender: %w", err)
	}
	d.OnShutdown(stopSend)

	// Registered after the send workers, so it stops before them: the workers keep nudging a
	// checker that is gone (Nudge never blocks) only for the moment between the two.
	d.OnShutdown(checker.Start(ctx))

	api.New(api.Config{Source: api.NewPoolSource(d.Pool), Logger: d.Logger, Shutdown: ctx}).Mount(d.Router)
	d.Logger.Info("send workers configured", "provider", providerCfg.BaseURL, "customer", consentCfg.String(),
		"sms_segment_price_usd", contentCfg.Prices.SMSSegment.String(), "email_price_usd", contentCfg.Prices.Email.String(),
		"redirect", linksCfg.URL, "tracking_base_url", contentCfg.TrackingBaseURL, "sms_text_budget", contentCfg.SMSBudget)
	return nil
}
