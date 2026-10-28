// Package boot wires the proposal service together (D-056, D-072): the Customer client, the LLM
// chosen by LLM_MODE, the content generator, the flow, the event hub and the REST and SSE routes.
// cmd/proposal passes Setup to the platform skeleton (internal/platform/app).
package boot

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/anthropic"
	"github.com/furknozg/coldbru/internal/proposal/llm/mock"
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

	svc, err := flow.New(flow.Deps{
		Pool:     d.Pool,
		LLM:      model,
		Customer: cust,
		Content:  contentgen.New(model, contentCfg.Prices),
		// T-026 replaces the no-op with the real confirmer.
		Confirmer: flow.NopConfirmer{},
		Logger:    d.Logger,
	})
	if err != nil {
		return fmt.Errorf("flow: %w", err)
	}
	d.OnShutdown(svc.Close)

	api.New(api.Config{Flow: svc, Logger: d.Logger, Shutdown: ctx}).Mount(d.Router)
	d.Logger.Info("proposal routes mounted", "llm_mode", string(mode), "cta_url", contentCfg.CTAURL)
	return nil
}
