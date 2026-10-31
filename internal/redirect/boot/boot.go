// Package boot wires the redirect service together (D-056, D-106, D-107): the outbox relay that
// publishes the click events, and the routes POST /links and GET /r/{code}.
// cmd/redirect passes Setup to the platform skeleton (internal/platform/app).
package boot

import (
	"context"
	"errors"
	"fmt"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/platform/config"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/redirect/api"
	"github.com/furknozg/coldbru/internal/redirect/code"
)

// KeySigningKey is the HMAC key of the link codes (D-106). It is required and has no default in
// code; docker-compose.yml and .env.example carry a development placeholder.
const KeySigningKey = "REDIRECT_SIGNING_KEY"

// Setup is the app.Options.Setup of the redirect service. It needs the database and NATS and
// REDIRECT_SIGNING_KEY; CAMPAIGN_CTA_URL is where an unknown or badly signed code is sent. The
// relay stops when the service stops, after the HTTP server, so a click recorded just before
// shutdown is still published.
func Setup(ctx context.Context, d *app.Deps) error {
	if d.Pool == nil || d.JetStream == nil {
		return errors.New("redirect needs a database and a NATS connection")
	}
	if err := config.Require(d.Config, KeySigningKey); err != nil {
		return err
	}
	signer, err := code.NewSigner(d.Config.GetString(KeySigningKey))
	if err != nil {
		return fmt.Errorf("%s: %w", KeySigningKey, err)
	}
	contentCfg, err := content.LoadConfig(d.Config)
	if err != nil {
		return fmt.Errorf("content config: %w", err)
	}

	relay := &outbox.Relay{Pool: d.Pool, JS: d.JetStream, Logger: d.Logger}
	d.OnShutdown(relay.Start(ctx))

	h, err := api.New(api.Config{
		Pool: d.Pool, Signer: signer, FallbackURL: contentCfg.CTAURL, Logger: d.Logger, Registerer: d.Metrics.Registry(),
	})
	if err != nil {
		return err
	}
	h.Mount(d.Router)
	d.Logger.Info("redirect configured", "fallback_url", contentCfg.CTAURL)
	return nil
}
