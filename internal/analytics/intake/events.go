package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furknozg/coldbru/internal/analytics/pricing"
	"github.com/furknozg/coldbru/internal/analytics/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/events"
)

// Values the tables accept; the CHECK constraints repeat them. An event outside them can never be
// stored, so it is rejected as malformed instead of being retried until the delivery limit.
var (
	channels  = []string{"sms", "email"}
	statuses  = []string{"pending", "sending", "sent", "failed", "skipped_no_consent", "cancelled"}
	purposes  = []string{"screen", "segment", "content", "channel_plan", "schedule"}
	outcomes  = []string{"accepted", "validation_failed", "refusal", "api_error"}
	languages = []string{"tr", "en"}

	// deliveryCostRE fits numeric(14,6): up to 8 integer and 6 fraction digits.
	deliveryCostRE = regexp.MustCompile(`^\d{1,8}(\.\d{1,6})?$`)
)

// Events handles the envelopes on the EVENTS stream (D-083). Use Handle as the consumer.Handler of
// a consumer.Processor; the Processor records processed_event.
type Events struct {
	Logger *slog.Logger
	Prices pricing.Table
}

// Handle applies one envelope in the Processor's transaction. Types and versions it does not know
// are ignored.
func (h *Events) Handle(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	if env.Version != EventVersion {
		h.Logger.DebugContext(ctx, "event version ignored", slog.String("type", env.Type), slog.Int("version", env.Version))
		return nil
	}
	switch env.Type {
	case EventTypeCompleted:
		return h.completed(ctx, tx, env)
	case EventTypeTokenCompleted:
		return h.tokenCompleted(ctx, tx, env)
	case EventTypeClicked:
		return h.clicked(ctx, tx, env)
	default:
		h.Logger.DebugContext(ctx, "event type ignored", slog.String("type", env.Type))
		return nil
	}
}

// completed replaces the summary, the delivery counts and the sent-by-locale counts of the campaign.
func (h *Events) completed(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	var d CompletedData
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return consumer.Malformed(fmt.Errorf("decode %s data: %w", env.Type, err))
	}
	p, err := d.validate()
	if err != nil {
		return consumer.Malformed(fmt.Errorf("%s event %s: %w", env.Type, env.ID, err))
	}

	q := store.New(tx)
	if err := q.UpsertCampaignSummary(ctx, store.UpsertCampaignSummaryParams{
		CampaignID: p.campaign, CompletedAt: pgtype.Timestamptz{Time: d.CompletedAt, Valid: true},
		SmsSegments: int32(d.SMSSegments), DeliveryCostUsd: p.cost,
	}); err != nil {
		return fmt.Errorf("upsert campaign summary: %w", err)
	}
	if err := q.DeleteDeliveryCounts(ctx, p.campaign); err != nil {
		return fmt.Errorf("delete delivery counts: %w", err)
	}
	for _, c := range d.Counts {
		if err := q.InsertDeliveryCount(ctx, store.InsertDeliveryCountParams{
			CampaignID: p.campaign, Channel: c.Channel, Status: c.Status, Count: int32(c.Count),
		}); err != nil {
			return fmt.Errorf("insert delivery count: %w", err)
		}
	}
	if err := q.DeleteSentLocales(ctx, p.campaign); err != nil {
		return fmt.Errorf("delete sent locales: %w", err)
	}
	for _, l := range d.SentByLocale {
		if err := q.InsertSentLocale(ctx, store.InsertSentLocaleParams{
			CampaignID: p.campaign, Channel: l.Channel, Language: l.Language, Timezone: l.Timezone, Sent: int32(l.Sent),
		}); err != nil {
			return fmt.Errorf("insert sent locale: %w", err)
		}
	}
	h.Logger.InfoContext(ctx, "campaign summary stored",
		slog.String("campaign_id", d.CampaignID), slog.Int("counts", len(d.Counts)), slog.Int("sent_locales", len(d.SentByLocale)),
		slog.Int64("sms_segments", d.SMSSegments))
	return nil
}

type completedParsed struct {
	campaign uuid.UUID
	cost     pgtype.Numeric
}

// validate checks the event against what the tables accept.
func (d CompletedData) validate() (completedParsed, error) {
	var p completedParsed
	var err error
	if p.campaign, err = uuid.Parse(d.CampaignID); err != nil {
		return p, fmt.Errorf("campaign_id %q is not a uuid", d.CampaignID)
	}
	if d.CompletedAt.IsZero() {
		return p, fmt.Errorf("completed_at is missing")
	}
	if d.SMSSegments < 0 || d.SMSSegments > math.MaxInt32 {
		return p, fmt.Errorf("sms_segments %d is out of range", d.SMSSegments)
	}
	if !deliveryCostRE.MatchString(d.EstimatedCostUSD) {
		return p, fmt.Errorf("estimated_cost_usd %q is not a non-negative decimal with at most 6 decimals", d.EstimatedCostUSD)
	}
	if err := p.cost.Scan(d.EstimatedCostUSD); err != nil {
		return p, fmt.Errorf("estimated_cost_usd %q: %w", d.EstimatedCostUSD, err)
	}
	seen := map[[2]string]bool{}
	for _, c := range d.Counts {
		switch {
		case !slices.Contains(channels, c.Channel):
			return p, fmt.Errorf("count channel %q is unknown", c.Channel)
		case !slices.Contains(statuses, c.Status):
			return p, fmt.Errorf("count status %q is unknown", c.Status)
		case c.Count < 0 || c.Count > math.MaxInt32:
			return p, fmt.Errorf("count %d of %s/%s is out of range", c.Count, c.Channel, c.Status)
		case seen[[2]string{c.Channel, c.Status}]:
			return p, fmt.Errorf("count %s/%s appears twice", c.Channel, c.Status)
		}
		seen[[2]string{c.Channel, c.Status}] = true
	}
	seenLocale := map[[3]string]bool{}
	for _, l := range d.SentByLocale {
		if err := checkLocale(l.Channel, l.Language, l.Timezone); err != nil {
			return p, fmt.Errorf("sent_by_locale: %w", err)
		}
		key := [3]string{l.Channel, l.Language, l.Timezone}
		switch {
		case l.Sent < 1 || l.Sent > math.MaxInt32:
			return p, fmt.Errorf("sent_by_locale %v: sent %d is out of range", key, l.Sent)
		case seenLocale[key]:
			return p, fmt.Errorf("sent_by_locale %v appears twice", key)
		}
		seenLocale[key] = true
	}
	return p, nil
}

// checkLocale checks the (channel, language, timezone) of a count or a click against the tables.
func checkLocale(channel, language, timezone string) error {
	switch {
	case !slices.Contains(channels, channel):
		return fmt.Errorf("channel %q is unknown", channel)
	case !slices.Contains(languages, language):
		return fmt.Errorf("language %q is unknown", language)
	case !nonBlank(timezone):
		return fmt.Errorf("timezone is blank")
	}
	return nil
}

// clicked stores one click of a short link and bumps the campaign's click counters (D-108). A click
// whose click_id is stored already inserts nothing and bumps nothing, so a repeat is harmless even
// without the Processor's processed_event row.
func (h *Events) clicked(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	var d ClickedData
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return consumer.Malformed(fmt.Errorf("decode %s data: %w", env.Type, err))
	}
	click, campaign, err := d.validate()
	if err != nil {
		return consumer.Malformed(fmt.Errorf("%s event %s: %w", env.Type, env.ID, err))
	}
	q := store.New(tx)
	n, err := q.InsertClickEvent(ctx, store.InsertClickEventParams{
		ClickID: click, CampaignID: campaign, Channel: d.Channel, Language: d.Language, Timezone: d.Timezone,
		ClickedAt: pgtype.Timestamptz{Time: d.ClickedAt, Valid: true}, First: d.First,
	})
	if err != nil {
		return fmt.Errorf("insert click event: %w", err)
	}
	if n == 0 {
		h.Logger.InfoContext(ctx, "click already stored, event ignored", slog.String("click_id", d.ClickID))
		return nil
	}
	if err := q.AddCampaignClick(ctx, store.AddCampaignClickParams{
		CampaignID: campaign, Channel: d.Channel, Language: d.Language, Timezone: d.Timezone, First: d.First,
	}); err != nil {
		return fmt.Errorf("add campaign click: %w", err)
	}
	return nil
}

// validate checks the event against what click_event accepts and returns the click and campaign ids.
func (d ClickedData) validate() (click, campaign uuid.UUID, err error) {
	if click, err = uuid.Parse(d.ClickID); err != nil {
		return click, campaign, fmt.Errorf("click_id %q is not a uuid", d.ClickID)
	}
	if campaign, err = uuid.Parse(d.CampaignID); err != nil {
		return click, campaign, fmt.Errorf("campaign_id %q is not a uuid", d.CampaignID)
	}
	if _, err = uuid.Parse(d.DeliveryID); err != nil {
		return click, campaign, fmt.Errorf("delivery_id %q is not a uuid", d.DeliveryID)
	}
	if err = checkLocale(d.Channel, d.Language, d.Timezone); err != nil {
		return click, campaign, err
	}
	if d.ClickedAt.IsZero() {
		return click, campaign, fmt.Errorf("clicked_at is missing")
	}
	return click, campaign, nil
}

// tokenCompleted stores the usage of one model call, priced by its model. A call already stored
// inserts nothing.
func (h *Events) tokenCompleted(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	var d TokenCompletedData
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return consumer.Malformed(fmt.Errorf("decode %s data: %w", env.Type, err))
	}
	call, campaign, err := d.validate()
	if err != nil {
		return consumer.Malformed(fmt.Errorf("%s event %s: %w", env.Type, env.ID, err))
	}

	costText, known := h.Prices.Cost(d.Model, pricing.Usage{
		Input: d.Tokens.Input, Output: d.Tokens.Output, CacheRead: d.Tokens.CacheRead, CacheCreation: d.Tokens.CacheCreation,
	})
	if !known {
		h.Logger.WarnContext(ctx, "no price for the model, cost recorded as 0",
			slog.String("model", d.Model), slog.String("llm_call_id", d.LLMCallID), slog.String("campaign_id", d.CampaignID))
	}
	var cost pgtype.Numeric
	if err := cost.Scan(costText); err != nil {
		return consumer.Malformed(fmt.Errorf("cost %q: %w", costText, err))
	}

	n, err := store.New(tx).InsertLLMUsage(ctx, store.InsertLLMUsageParams{
		LlmCallID: call, CampaignID: campaign, Purpose: d.Purpose, Model: d.Model,
		PromptName: d.PromptName, PromptVersion: d.PromptVersion,
		InputTokens: int32(d.Tokens.Input), OutputTokens: int32(d.Tokens.Output),
		CacheReadTokens: int32(d.Tokens.CacheRead), CacheCreationTokens: int32(d.Tokens.CacheCreation),
		Outcome: d.Outcome, LatencyMs: int32(d.LatencyMs), CostUsd: cost,
		CreatedAt: pgtype.Timestamptz{Time: d.CreatedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert llm usage: %w", err)
	}
	if n == 0 {
		h.Logger.InfoContext(ctx, "llm usage already stored, event ignored", slog.String("llm_call_id", d.LLMCallID))
	}
	return nil
}

// validate checks the event against what llm_usage accepts and returns the two ids.
func (d TokenCompletedData) validate() (call, campaign uuid.UUID, err error) {
	if call, err = uuid.Parse(d.LLMCallID); err != nil {
		return call, campaign, fmt.Errorf("llm_call_id %q is not a uuid", d.LLMCallID)
	}
	if campaign, err = uuid.Parse(d.CampaignID); err != nil {
		return call, campaign, fmt.Errorf("campaign_id %q is not a uuid", d.CampaignID)
	}
	for _, f := range []struct{ name, value string }{{"model", d.Model}, {"prompt_name", d.PromptName}, {"prompt_version", d.PromptVersion}} {
		if !nonBlank(f.value) {
			return call, campaign, fmt.Errorf("%s is blank", f.name)
		}
	}
	if !slices.Contains(purposes, d.Purpose) {
		return call, campaign, fmt.Errorf("purpose %q is unknown", d.Purpose)
	}
	if !slices.Contains(outcomes, d.Outcome) {
		return call, campaign, fmt.Errorf("outcome %q is unknown", d.Outcome)
	}
	for _, f := range []struct {
		name string
		n    int64
	}{{"tokens.input", d.Tokens.Input}, {"tokens.output", d.Tokens.Output}, {"tokens.cache_read", d.Tokens.CacheRead},
		{"tokens.cache_creation", d.Tokens.CacheCreation}, {"latency_ms", d.LatencyMs}} {
		if f.n < 0 || f.n > math.MaxInt32 {
			return call, campaign, fmt.Errorf("%s %d is out of range", f.name, f.n)
		}
	}
	if d.CreatedAt.IsZero() {
		return call, campaign, fmt.Errorf("created_at is missing")
	}
	return call, campaign, nil
}

func nonBlank(s string) bool { return strings.TrimSpace(s) != "" }
