// Package boot wires the proposal service together (D-056, D-072, D-080): the Customer client, the
// LLM chosen by LLM_MODE, the content generator, the flow, the event hub, the consumer of
// Notification's events and the REST and SSE routes.
// cmd/proposal passes Setup to the platform skeleton (internal/platform/app).
package boot

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/confirm"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/anthropic"
	"github.com/furknozg/coldbru/internal/proposal/llm/mock"
	"github.com/furknozg/coldbru/internal/proposal/progress"
)

// KeyLLMMode selects the model: auto, anthropic or mock (D-072).
const KeyLLMMode = "LLM_MODE"

// Mode is how the service obtains its model.
type Mode string

// Modes. Auto is a request, never an outcome: SelectLLM resolves it to Anthropic or Mock.
const (
	ModeAuto      Mode = "auto"
	ModeAnthropic Mode = "anthropic"
	ModeMock      Mode = "mock"
)

// ParseMode reads an LLM_MODE value; an empty value is auto.
func ParseMode(s string) (Mode, error) {
	switch m := Mode(strings.ToLower(strings.TrimSpace(s))); m {
	case "":
		return ModeAuto, nil
	case ModeAuto, ModeAnthropic, ModeMock:
		return m, nil
	default:
		return "", fmt.Errorf("%s: %q is not one of auto, anthropic, mock", KeyLLMMode, s)
	}
}

// SelectLLM returns the model for the configuration and the mode that was chosen, always Anthropic
// or Mock (D-072):
//
//	auto       the real model when ANTHROPIC_API_KEY is set and not blank, the mock otherwise;
//	anthropic  the real model; a missing key is an error that names the key;
//	mock       the mock, whatever the key.
//
// It makes no network call and never logs or returns the key.
func SelectLLM(v *viper.Viper) (llm.LLM, Mode, error) {
	requested, err := ParseMode(v.GetString(KeyLLMMode))
	if err != nil {
		return nil, "", err
	}
	if requested == ModeAuto {
		requested = ModeMock
		if strings.TrimSpace(v.GetString(anthropic.KeyAPIKey)) != "" {
			requested = ModeAnthropic
		}
	}
	if requested == ModeMock {
		return mock.New(), ModeMock, nil
	}
	cfg, err := anthropic.LoadConfig(v)
	if err != nil {
		return nil, "", fmt.Errorf("%s=%s: %w", KeyLLMMode, ModeAnthropic, err)
	}
	c, err := anthropic.New(cfg)
	if err != nil {
		return nil, "", err
	}
	return c, ModeAnthropic, nil
}

// Setup is the app.Options.Setup of the proposal service. Invalid configuration returns an error
// that names the key, which stops the service at startup. The flow is closed when the service
// stops, after the HTTP server and before the database pool.
func Setup(ctx context.Context, d *app.Deps) error {
	custCfg, err := customer.LoadConfig(d.Config)
	if err != nil {
		return fmt.Errorf("customer client: %w", err)
	}
	cust, err := customer.New(custCfg)
	if err != nil {
		return fmt.Errorf("customer client: %w", err)
	}
	contentCfg, err := content.LoadConfig(d.Config)
	if err != nil {
		return fmt.Errorf("content config: %w", err)
	}
	model, mode, err := SelectLLM(d.Config)
	if err != nil {
		return fmt.Errorf("llm: %w", err)
	}
	d.Logger.Info("llm", "mode", string(mode), "requested", strings.ToLower(strings.TrimSpace(d.Config.GetString(KeyLLMMode))))
	// The check passes in every mode and only reports the mode: nothing is called (D-072).
	d.Health.AddDetail("llm", func(context.Context) (string, error) { return "mode=" + string(mode), nil })

	// The confirmer publishes campaign_confirmed to the hub the flow's event streams read (D-074).
	hub := flow.NewHub(0)
	confirmer, err := confirm.New(confirm.Deps{
		Pool: d.Pool, Customer: cust, CTAURL: contentCfg.CTAURL, Hub: hub, Logger: d.Logger,
	})
	if err != nil {
		return fmt.Errorf("confirmer: %w", err)
	}
	// Registered before the flow's hook, so it runs after it: the relay keeps publishing until no
	// action can write to the outbox any more.
	startRelay(ctx, d)

	svc, err := flow.New(flow.Deps{
		Pool:     d.Pool,
		LLM:      model,
		Customer: cust,
		Content: contentgen.New(model, contentCfg.Prices, contentgen.WithSMSBudget(contentCfg.SMSBudget),
			contentgen.WithSMSLink(content.PlaceholderLink(contentCfg.TrackingBaseURL))),
		Confirmer: confirmer,
		Hub:       hub,
		Logger:    d.Logger,
		CTAURL:    contentCfg.CTAURL,
	})
	if err != nil {
		return fmt.Errorf("flow: %w", err)
	}
	d.OnShutdown(svc.Close)

	// Follows Notification: sending and completed (D-080). Registered after the flow's hook, so it
	// stops first; it only writes to the database.
	if d.JetStream == nil {
		d.Logger.Warn("notification events not consumed: no NATS connection")
	} else {
		stopProgress, err := progress.Start(ctx, progress.Deps{JS: d.JetStream, Pool: d.Pool, Hub: hub, Logger: d.Logger})
		if err != nil {
			return fmt.Errorf("notification events: %w", err)
		}
		d.OnShutdown(stopProgress)
	}

	api.New(api.Config{Flow: svc, Logger: d.Logger, Shutdown: ctx}).Mount(d.Router)
	d.Logger.Info("proposal routes mounted", "llm_mode", string(mode), "cta_url", contentCfg.CTAURL, "sms_text_budget", contentCfg.SMSBudget)
	return nil
}

// startRelay runs the outbox relay that publishes the confirmed and cancelled events and the
// recipient batches to NATS (D-006, D-054, D-074), and stops it on shutdown. It is a no-op, with a
// warning, when the service has no JetStream connection; cmd/proposal always has one.
func startRelay(ctx context.Context, d *app.Deps) {
	if d.JetStream == nil {
		d.Logger.Warn("outbox relay not started: no NATS connection")
		return
	}
	relayCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	relay := &outbox.Relay{Pool: d.Pool, JS: d.JetStream, Logger: d.Logger}
	go func() {
		defer close(done)
		_ = relay.Run(relayCtx) // Run returns nil when its context ends
	}()
	d.OnShutdown(func(ctx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}
