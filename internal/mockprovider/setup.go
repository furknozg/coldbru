package mockprovider

import (
	"context"
	"time"

	"github.com/furknozg/coldbru/internal/platform/app"
)

// SweepInterval is how often expired idempotency records are removed.
const SweepInterval = time.Minute

// Setup is the app.Options.Setup of the mockprovider service: it reads the configuration, logs it,
// mounts the endpoints and starts the sweeper of the idempotency store, which stops with ctx.
func Setup(ctx context.Context, d *app.Deps) error {
	cfg, err := LoadConfig(d.Config)
	if err != nil {
		return err
	}
	h, err := NewHandler(HandlerConfig{
		Config: cfg, Forwarder: NewInboxForwarder(cfg),
		Registerer: d.Metrics.Registry(), Logger: d.Logger,
	})
	if err != nil {
		return err
	}
	h.Mount(d.Router)
	go h.RunSweeper(ctx, SweepInterval)
	d.Logger.Info("mock provider configured", cfg.LogAttrs()...)
	return nil
}
