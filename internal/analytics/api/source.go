package api

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/analytics/store"
)

// ErrNotFound: Analytics knows nothing of the campaign (no summary and no LLM usage).
var ErrNotFound = errors.New("api: campaign not found")

// Source reads the reports. *PoolSource implements it; tests use a fake.
type Source interface {
	// ListCampaigns returns one page of completed campaigns, the most recently completed first.
	ListCampaigns(ctx context.Context, limit, offset int) ([]CampaignListItem, error)
	// Campaign returns the report of one campaign, or ErrNotFound.
	Campaign(ctx context.Context, id uuid.UUID) (CampaignReport, error)
	// PromptVersions returns the LLM usage per (prompt name, prompt version).
	PromptVersions(ctx context.Context) ([]PromptVersionUsage, error)
}

// PoolSource reads the reports from analytics_db.
type PoolSource struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

// NewPoolSource returns a Source over pool.
func NewPoolSource(pool *pgxpool.Pool) *PoolSource {
	return &PoolSource{pool: pool, q: store.New(pool)}
}

// ListCampaigns implements Source.
func (s *PoolSource) ListCampaigns(ctx context.Context, limit, offset int) ([]CampaignListItem, error) {
	rows, err := s.q.ListCampaignSummaries(ctx, store.ListCampaignSummariesParams{Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, fmt.Errorf("list campaign summaries: %w", err)
	}
	out := make([]CampaignListItem, len(rows))
	for i, r := range rows {
		out[i] = CampaignListItem{
			CampaignID: r.CampaignID, CompletedAt: r.CompletedAt.Time.UTC(), SMSSegments: int64(r.SmsSegments),
			DeliveryCostUSD: r.DeliveryCostUsd, LLMCostUSD: r.LlmCostUsd,
		}
	}
	return out, nil
}

// zeroDeliveryCost is the delivery cost of a campaign that has not completed, at the scale of
// campaign_summary.delivery_cost_usd.
const zeroDeliveryCost = "0.000000"

// Campaign implements Source. The reads share one snapshot, so the summary, the counts and the LLM
// usage agree with each other.
func (s *PoolSource) Campaign(ctx context.Context, id uuid.UUID) (CampaignReport, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CampaignReport{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.q.WithTx(tx)

	out := CampaignReport{CampaignID: id, Deliveries: []DeliveryCount{}, DeliveryCostUSD: zeroDeliveryCost}
	completed := true
	sum, err := q.GetCampaignSummary(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		completed = false
	case err != nil:
		return CampaignReport{}, fmt.Errorf("read campaign summary: %w", err)
	default:
		at := sum.CompletedAt.Time.UTC()
		out.CompletedAt = &at
		out.SMSSegments = int64(sum.SmsSegments)
		out.DeliveryCostUSD = sum.DeliveryCostUsd
	}
	if completed {
		counts, err := q.ListDeliveryCounts(ctx, id)
		if err != nil {
			return CampaignReport{}, fmt.Errorf("read delivery counts: %w", err)
		}
		for _, c := range counts {
			out.Deliveries = append(out.Deliveries, DeliveryCount{Channel: c.Channel, Status: c.Status, Count: int64(c.Count)})
		}
	}

	tot, err := q.LLMTotalsOfCampaign(ctx, id)
	if err != nil {
		return CampaignReport{}, fmt.Errorf("read llm totals: %w", err)
	}
	if !completed && tot.Calls == 0 {
		return CampaignReport{}, ErrNotFound
	}
	out.LLM = LLMSummary{
		Calls: tot.Calls, CostUSD: tot.CostUsd, ByPurpose: []PurposeUsage{},
		Tokens: Tokens{Input: tot.InputTokens, Output: tot.OutputTokens, CacheRead: tot.CacheReadTokens, CacheCreation: tot.CacheCreationTokens},
	}
	byPurpose, err := q.LLMByPurposeOfCampaign(ctx, id)
	if err != nil {
		return CampaignReport{}, fmt.Errorf("read llm usage by purpose: %w", err)
	}
	for _, p := range byPurpose {
		out.LLM.ByPurpose = append(out.LLM.ByPurpose, PurposeUsage{
			Purpose: p.Purpose, Calls: p.Calls, CostUSD: p.CostUsd, InputTokens: p.InputTokens, OutputTokens: p.OutputTokens,
		})
	}
	return out, nil
}

// PromptVersions implements Source.
func (s *PoolSource) PromptVersions(ctx context.Context) ([]PromptVersionUsage, error) {
	rows, err := s.q.LLMUsageByPromptVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("read llm usage by prompt version: %w", err)
	}
	out := make([]PromptVersionUsage, len(rows))
	for i, r := range rows {
		out[i] = PromptVersionUsage{
			PromptName: r.PromptName, PromptVersion: r.PromptVersion, Calls: r.Calls,
			Accepted: r.Accepted, ValidationFailed: r.ValidationFailed, Refusal: r.Refusal, APIError: r.ApiError,
			RepairRate: RepairRate(r.ValidationFailed, r.Calls), AvgLatencyMs: round(r.AvgLatencyMs, 1), CostUSD: r.CostUsd,
		}
	}
	return out, nil
}

// RepairRate is validation_failed / calls rounded to four decimals; 0 when there are no calls.
func RepairRate(validationFailed, calls int64) float64 {
	if calls <= 0 {
		return 0
	}
	return round(float64(validationFailed)/float64(calls), 4)
}

func round(x float64, decimals int) float64 {
	p := math.Pow10(decimals)
	return math.Round(x*p) / p
}
