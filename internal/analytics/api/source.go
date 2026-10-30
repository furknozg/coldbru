package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	// PromptVersions returns the LLM usage per (prompt name, prompt version) of the calls in rg.
	PromptVersions(ctx context.Context, rg Range) ([]PromptVersionUsage, error)
	// Overview returns the headline figures over rg.
	Overview(ctx context.Context, rg Range) (Overview, error)
	// LLMSeries returns the LLM usage per bucket over q's range.
	LLMSeries(ctx context.Context, q SeriesQuery) (LLMSeries, error)
	// OverviewSeries returns the delivery counts per bucket over q's range.
	OverviewSeries(ctx context.Context, q SeriesQuery) (OverviewSeries, error)
}

// Range is a half-open time range [From, To). A nil bound is open-ended.
type Range struct {
	From, To *time.Time
}

// SeriesQuery is a validated series request: UTC bounds, From < To, and a Bucket of BucketHour or
// BucketDay within the limits of that bucket.
type SeriesQuery struct {
	From, To time.Time
	Bucket   string
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
func (s *PoolSource) PromptVersions(ctx context.Context, rg Range) ([]PromptVersionUsage, error) {
	rows, err := s.q.LLMUsageByPromptVersion(ctx, store.LLMUsageByPromptVersionParams{FromTs: tstz(rg.From), ToTs: tstz(rg.To)})
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

// tstz is t as a nullable timestamptz parameter: nil is NULL, which the queries read as open-ended.
func tstz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// Overview implements Source. The reads share one snapshot, so the figures agree with each other.
func (s *PoolSource) Overview(ctx context.Context, rg Range) (Overview, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Overview{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.q.WithTx(tx)
	from, to := tstz(rg.From), tstz(rg.To)

	out := Overview{From: rg.From, To: rg.To, Deliveries: []DeliveryCount{}}
	camp, err := q.DeliveryOverviewTotals(ctx, store.DeliveryOverviewTotalsParams{FromTs: from, ToTs: to})
	if err != nil {
		return Overview{}, fmt.Errorf("read delivery totals: %w", err)
	}
	out.CampaignsCompleted, out.SMSSegments, out.DeliveryCostUSD = camp.CampaignsCompleted, camp.SmsSegments, camp.DeliveryCostUsd
	counts, err := q.DeliveryOverviewCounts(ctx, store.DeliveryOverviewCountsParams{FromTs: from, ToTs: to})
	if err != nil {
		return Overview{}, fmt.Errorf("read delivery counts: %w", err)
	}
	for _, c := range counts {
		out.Deliveries = append(out.Deliveries, DeliveryCount{Channel: c.Channel, Status: c.Status, Count: c.Count})
	}

	tot, err := q.LLMOverviewTotals(ctx, store.LLMOverviewTotalsParams{FromTs: from, ToTs: to})
	if err != nil {
		return Overview{}, fmt.Errorf("read llm totals: %w", err)
	}
	out.LLM = OverviewLLM{
		Calls: tot.Calls, CostUSD: tot.CostUsd, ByPurpose: []PurposeUsage{}, ByModel: []ModelUsage{},
		Tokens: Tokens{Input: tot.InputTokens, Output: tot.OutputTokens, CacheRead: tot.CacheReadTokens, CacheCreation: tot.CacheCreationTokens},
	}
	byPurpose, err := q.LLMOverviewByPurpose(ctx, store.LLMOverviewByPurposeParams{FromTs: from, ToTs: to})
	if err != nil {
		return Overview{}, fmt.Errorf("read llm usage by purpose: %w", err)
	}
	for _, p := range byPurpose {
		out.LLM.ByPurpose = append(out.LLM.ByPurpose, PurposeUsage{
			Purpose: p.Purpose, Calls: p.Calls, CostUSD: p.CostUsd, InputTokens: p.InputTokens, OutputTokens: p.OutputTokens,
		})
	}
	byModel, err := q.LLMOverviewByModel(ctx, store.LLMOverviewByModelParams{FromTs: from, ToTs: to})
	if err != nil {
		return Overview{}, fmt.Errorf("read llm usage by model: %w", err)
	}
	for _, m := range byModel {
		out.LLM.ByModel = append(out.LLM.ByModel, ModelUsage{
			Model: m.Model, Calls: m.Calls, CostUSD: m.CostUsd, InputTokens: m.InputTokens, OutputTokens: m.OutputTokens,
		})
	}
	return out, nil
}

// bucketStarts lists the bucket starts of q's range, in UTC.
func bucketStarts(ctx context.Context, q *store.Queries, sq SeriesQuery) ([]time.Time, error) {
	rows, err := q.SeriesBuckets(ctx, store.SeriesBucketsParams{Bucket: sq.Bucket, FromTs: tstz(&sq.From), ToTs: tstz(&sq.To)})
	if err != nil {
		return nil, fmt.Errorf("read series buckets: %w", err)
	}
	out := make([]time.Time, len(rows))
	for i, r := range rows {
		out[i] = r.Time.UTC()
	}
	return out, nil
}

// LLMSeries implements Source.
func (s *PoolSource) LLMSeries(ctx context.Context, sq SeriesQuery) (LLMSeries, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return LLMSeries{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.q.WithTx(tx)

	out := LLMSeries{From: sq.From, To: sq.To, Bucket: sq.Bucket, Points: []LLMSeriesPoint{}}
	if out.Buckets, err = bucketStarts(ctx, q, sq); err != nil {
		return LLMSeries{}, err
	}
	rows, err := q.LLMSeries(ctx, store.LLMSeriesParams{Bucket: sq.Bucket, FromTs: tstz(&sq.From), ToTs: tstz(&sq.To)})
	if err != nil {
		return LLMSeries{}, fmt.Errorf("read llm series: %w", err)
	}
	for _, r := range rows {
		out.Points = append(out.Points, LLMSeriesPoint{
			Start: r.BucketStart.Time.UTC(), Purpose: r.Purpose, Model: r.Model, Calls: r.Calls, CostUSD: r.CostUsd,
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
		})
	}
	return out, nil
}

// OverviewSeries implements Source.
func (s *PoolSource) OverviewSeries(ctx context.Context, sq SeriesQuery) (OverviewSeries, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return OverviewSeries{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.q.WithTx(tx)

	out := OverviewSeries{From: sq.From, To: sq.To, Bucket: sq.Bucket, Points: []OverviewSeriesPoint{}}
	if out.Buckets, err = bucketStarts(ctx, q, sq); err != nil {
		return OverviewSeries{}, err
	}
	rows, err := q.DeliverySeries(ctx, store.DeliverySeriesParams{Bucket: sq.Bucket, FromTs: tstz(&sq.From), ToTs: tstz(&sq.To)})
	if err != nil {
		return OverviewSeries{}, fmt.Errorf("read delivery series: %w", err)
	}
	for _, r := range rows {
		out.Points = append(out.Points, OverviewSeriesPoint{Start: r.BucketStart.Time.UTC(), Channel: r.Channel, Status: r.Status, Count: r.Count})
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
