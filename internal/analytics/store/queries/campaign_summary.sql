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

-- name: DeliveryOverviewTotals :one
-- Campaigns completed in [from_ts, to_ts) of completed_at, with their SMS segments and delivery
-- cost (D-097); a NULL bound is open-ended. COALESCE to +-infinity keeps the predicate usable by
-- campaign_summary_completed_at_idx.
SELECT count(*)::bigint AS campaigns_completed,
       COALESCE(sum(sms_segments), 0)::bigint AS sms_segments,
       COALESCE(sum(delivery_cost_usd), 0)::numeric(20,6)::text AS delivery_cost_usd
FROM campaign_summary
WHERE completed_at >= COALESCE(sqlc.narg('from_ts')::timestamptz, '-infinity')
  AND completed_at <  COALESCE(sqlc.narg('to_ts')::timestamptz, 'infinity');

-- name: DeliveryOverviewCounts :many
-- Delivery counts per (channel, status) summed over the campaigns completed in the range.
SELECT c.channel, c.status, sum(c.count)::bigint AS count
FROM campaign_delivery_count c
JOIN campaign_summary s ON s.campaign_id = c.campaign_id
WHERE s.completed_at >= COALESCE(sqlc.narg('from_ts')::timestamptz, '-infinity')
  AND s.completed_at <  COALESCE(sqlc.narg('to_ts')::timestamptz, 'infinity')
GROUP BY c.channel, c.status
ORDER BY c.channel, c.status;

-- name: DeliverySeries :many
-- Delivery counts per (UTC bucket of completed_at, channel, status) over [from_ts, to_ts). Buckets
-- without a completed campaign have no row (see SeriesBuckets).
SELECT date_trunc(sqlc.arg('bucket')::text, s.completed_at, 'UTC')::timestamptz AS bucket_start,
       c.channel, c.status, sum(c.count)::bigint AS count
FROM campaign_summary s
JOIN campaign_delivery_count c ON c.campaign_id = s.campaign_id
WHERE s.completed_at >= sqlc.arg('from_ts')::timestamptz
  AND s.completed_at <  sqlc.arg('to_ts')::timestamptz
GROUP BY 1, c.channel, c.status
ORDER BY 1, c.channel, c.status;
