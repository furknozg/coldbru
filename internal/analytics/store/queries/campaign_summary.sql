-- name: UpsertCampaignSummary :exec
-- A repeated completed event for the same campaign replaces the summary (D-083).
INSERT INTO campaign_summary (campaign_id, completed_at, sms_segments, delivery_cost_usd)
VALUES ($1, $2, $3, $4)
ON CONFLICT (campaign_id) DO UPDATE
SET completed_at = EXCLUDED.completed_at,
    sms_segments = EXCLUDED.sms_segments,
    delivery_cost_usd = EXCLUDED.delivery_cost_usd,
    updated_at = now();

-- name: GetCampaignSummary :one
SELECT campaign_id, completed_at, sms_segments, delivery_cost_usd::text AS delivery_cost_usd
FROM campaign_summary
WHERE campaign_id = $1;

-- name: ListCampaignSummaries :many
-- Newest completion first; the campaign id breaks ties so pages are stable. The LLM cost is the
-- sum over llm_usage, served by llm_usage_campaign_id_idx.
SELECT s.campaign_id, s.completed_at, s.sms_segments,
       s.delivery_cost_usd::text AS delivery_cost_usd,
       COALESCE((SELECT sum(u.cost_usd) FROM llm_usage u WHERE u.campaign_id = s.campaign_id), 0)::numeric(14,8)::text AS llm_cost_usd
FROM campaign_summary s
ORDER BY s.completed_at DESC, s.campaign_id DESC
LIMIT $1 OFFSET $2;
