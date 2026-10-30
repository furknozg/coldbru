package api

import (
	"time"

	"github.com/google/uuid"
)

// DeliveryCount is the number of deliveries of one channel in one status, as the completed event
// reported them.
type DeliveryCount struct {
	Channel string `json:"channel" validate:"required" enums:"sms,email"`
	Status  string `json:"status" validate:"required" enums:"pending,sending,sent,failed,skipped_no_consent,cancelled"`
	Count   int64  `json:"count" validate:"required" example:"1200"`
}

// CampaignListItem is one completed campaign in GET /campaigns.
type CampaignListItem struct {
	CampaignID  uuid.UUID `json:"campaign_id" validate:"required" format:"uuid"`
	CompletedAt time.Time `json:"completed_at" validate:"required" format:"date-time"`
	// SMSSegments is the estimated number of SMS segments sent (D-071).
	SMSSegments int64 `json:"sms_segments" validate:"required" example:"2400"`
	// DeliveryCostUSD is the estimated cost of the deliveries in US dollars, six decimals.
	DeliveryCostUSD string `json:"delivery_cost_usd" validate:"required" example:"96.000000"`
	// LLMCostUSD is the cost of the model calls of the campaign in US dollars, eight decimals.
	LLMCostUSD string `json:"llm_cost_usd" validate:"required" example:"0.01234500"`
}

// CampaignList is the response of GET /campaigns: one page, the most recently completed first.
type CampaignList struct {
	Items []CampaignListItem `json:"items" validate:"required"`
	// Limit and Offset echo the page that was asked for. A page with fewer than Limit items is the last.
	Limit  int `json:"limit" example:"20" validate:"required"`
	Offset int `json:"offset" example:"0" validate:"required"`
}

// Tokens are summed token counts.
type Tokens struct {
	Input         int64 `json:"input" validate:"required"`
	Output        int64 `json:"output" validate:"required"`
	CacheRead     int64 `json:"cache_read" validate:"required"`
	CacheCreation int64 `json:"cache_creation" validate:"required"`
}

// PurposeUsage is the LLM usage of one purpose within a campaign.
type PurposeUsage struct {
	Purpose      string `json:"purpose" validate:"required" enums:"screen,segment,content,channel_plan,schedule"`
	Calls        int64  `json:"calls" validate:"required"`
	CostUSD      string `json:"cost_usd" validate:"required" example:"0.00420000"`
	InputTokens  int64  `json:"input_tokens" validate:"required"`
	OutputTokens int64  `json:"output_tokens" validate:"required"`
}

// LLMSummary is the LLM usage of a campaign. Every call counts, accepted or not.
type LLMSummary struct {
	Calls int64 `json:"calls" validate:"required"`
	// CostUSD is the sum of the cost of the calls in US dollars, eight decimals.
	CostUSD   string         `json:"cost_usd" validate:"required" example:"0.01234500"`
	Tokens    Tokens         `json:"tokens" validate:"required"`
	ByPurpose []PurposeUsage `json:"by_purpose" validate:"required"`
}

// CampaignReport is the response of GET /campaigns/{id}/summary. A campaign whose completed event
// has not arrived yet (or that never completes) has completed_at null, no deliveries, zero SMS
// segments and a zero delivery cost; its LLM usage is reported all the same.
type CampaignReport struct {
	CampaignID uuid.UUID `json:"campaign_id" validate:"required" format:"uuid"`
	// CompletedAt is null until the campaign is completed.
	CompletedAt *time.Time `json:"completed_at" validate:"required" format:"date-time" extensions:"x-nullable"`
	// Deliveries has one entry per (channel, status) with at least one delivery, ordered by channel and status.
	Deliveries      []DeliveryCount `json:"deliveries" validate:"required"`
	SMSSegments     int64           `json:"sms_segments" validate:"required" example:"2400"`
	DeliveryCostUSD string          `json:"delivery_cost_usd" validate:"required" example:"96.000000"`
	LLM             LLMSummary      `json:"llm" validate:"required"`
}

// PromptVersionUsage is the LLM usage of one prompt version across all campaigns.
type PromptVersionUsage struct {
	PromptName    string `json:"prompt_name" validate:"required" example:"content"`
	PromptVersion string `json:"prompt_version" validate:"required" example:"1.2.0"`
	Calls         int64  `json:"calls" validate:"required"`
	// Accepted, ValidationFailed, Refusal and APIError count the calls by outcome; they add up to Calls.
	Accepted         int64 `json:"accepted" validate:"required"`
	ValidationFailed int64 `json:"validation_failed" validate:"required"`
	Refusal          int64 `json:"refusal" validate:"required"`
	APIError         int64 `json:"api_error" validate:"required"`
	// RepairRate is validation_failed divided by calls, 0 to 1, four decimals: the share of calls
	// whose output had to be repaired or was dropped.
	RepairRate float64 `json:"repair_rate" validate:"required" example:"0.0625"`
	// AvgLatencyMs is the mean latency of the calls, one decimal.
	AvgLatencyMs float64 `json:"avg_latency_ms" validate:"required" example:"812.5"`
	// CostUSD is the cost of the calls in US dollars, eight decimals.
	CostUSD string `json:"cost_usd" validate:"required" example:"0.01234500"`
}

// ModelUsage is the LLM usage of one model.
type ModelUsage struct {
	Model        string `json:"model" validate:"required" example:"claude-sonnet-5-5"`
	Calls        int64  `json:"calls" validate:"required"`
	CostUSD      string `json:"cost_usd" validate:"required" example:"0.00420000"`
	InputTokens  int64  `json:"input_tokens" validate:"required"`
	OutputTokens int64  `json:"output_tokens" validate:"required"`
}

// OverviewLLM is the LLM usage of an Overview: every call whose created_at is in the range,
// accepted or not.
type OverviewLLM struct {
	Calls int64 `json:"calls" validate:"required"`
	// CostUSD is the sum of the cost of the calls in US dollars, eight decimals.
	CostUSD string `json:"cost_usd" validate:"required" example:"0.01234500"`
	Tokens  Tokens `json:"tokens" validate:"required"`
	// ByPurpose has one entry per purpose with calls, ordered by purpose.
	ByPurpose []PurposeUsage `json:"by_purpose" validate:"required"`
	// ByModel has one entry per model with calls, ordered by model.
	ByModel []ModelUsage `json:"by_model" validate:"required"`
}

// Overview is the response of GET /overview: the dashboard's headline figures over a time range
// (D-097). Delivery figures cover the campaigns whose completed_at is in the range, LLM figures
// the calls whose created_at is in the range. T-044 adds the success metrics as further fields.
type Overview struct {
	// From and To echo the range of the request (RFC 3339, UTC); null when that bound was omitted.
	From *time.Time `json:"from" validate:"required" format:"date-time" extensions:"x-nullable"`
	To   *time.Time `json:"to" validate:"required" format:"date-time" extensions:"x-nullable"`
	// CampaignsCompleted is the number of campaigns completed in the range.
	CampaignsCompleted int64 `json:"campaigns_completed" validate:"required"`
	// Deliveries has one entry per (channel, status) with at least one delivery, ordered by channel and status.
	Deliveries      []DeliveryCount `json:"deliveries" validate:"required"`
	SMSSegments     int64           `json:"sms_segments" validate:"required" example:"2400"`
	DeliveryCostUSD string          `json:"delivery_cost_usd" validate:"required" example:"96.000000"`
	LLM             OverviewLLM     `json:"llm" validate:"required"`
}

// LLMSeriesPoint is the LLM usage of one purpose and model within one bucket.
type LLMSeriesPoint struct {
	// Start is the start of the bucket (UTC); one of the bucket starts of the response.
	Start        time.Time `json:"start" validate:"required" format:"date-time"`
	Purpose      string    `json:"purpose" validate:"required" enums:"screen,segment,content,channel_plan,schedule"`
	Model        string    `json:"model" validate:"required" example:"claude-sonnet-5-5"`
	Calls        int64     `json:"calls" validate:"required"`
	CostUSD      string    `json:"cost_usd" validate:"required" example:"0.00420000"`
	InputTokens  int64     `json:"input_tokens" validate:"required"`
	OutputTokens int64     `json:"output_tokens" validate:"required"`
}

// LLMSeries is the response of GET /llm-usage/series.
type LLMSeries struct {
	From   time.Time `json:"from" validate:"required" format:"date-time"`
	To     time.Time `json:"to" validate:"required" format:"date-time"`
	Bucket string    `json:"bucket" validate:"required" enums:"hour,day"`
	// Buckets lists the start of every bucket of the range, ascending, empty ones included.
	Buckets []time.Time `json:"buckets" validate:"required" format:"date-time"`
	// Points has one row per (bucket, purpose, model) with at least one call, ordered by bucket,
	// purpose, model. A bucket without a row has no calls: the client zero-fills it from Buckets.
	Points []LLMSeriesPoint `json:"points" validate:"required"`
}

// OverviewSeriesPoint is the delivery count of one channel and status within one bucket.
type OverviewSeriesPoint struct {
	// Start is the start of the bucket (UTC); one of the bucket starts of the response.
	Start   time.Time `json:"start" validate:"required" format:"date-time"`
	Channel string    `json:"channel" validate:"required" enums:"sms,email"`
	Status  string    `json:"status" validate:"required" enums:"pending,sending,sent,failed,skipped_no_consent,cancelled"`
	Count   int64     `json:"count" validate:"required" example:"1200"`
}

// OverviewSeries is the response of GET /overview/series.
type OverviewSeries struct {
	From   time.Time `json:"from" validate:"required" format:"date-time"`
	To     time.Time `json:"to" validate:"required" format:"date-time"`
	Bucket string    `json:"bucket" validate:"required" enums:"hour,day"`
	// Buckets lists the start of every bucket of the range, ascending, empty ones included.
	Buckets []time.Time `json:"buckets" validate:"required" format:"date-time"`
	// Points has one row per (bucket, channel, status) with at least one delivery, ordered by
	// bucket, channel, status. A bucket without a row has no deliveries: the client zero-fills it
	// from Buckets.
	Points []OverviewSeriesPoint `json:"points" validate:"required"`
}
