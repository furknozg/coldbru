-- name: DeleteSentLocales :exec
DELETE FROM campaign_sent_locale WHERE campaign_id = $1;

-- name: InsertSentLocale :exec
-- A completion has at most 2 channels x 2 languages x the campaign's time zones rows, inserted one by one.
INSERT INTO campaign_sent_locale (campaign_id, channel, language, timezone, sent)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertClickEvent :execrows
-- A repeated click_id inserts nothing (D-108).
INSERT INTO click_event (click_id, campaign_id, channel, language, timezone, clicked_at, first)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (click_id) DO NOTHING;

-- name: AddCampaignClick :exec
-- One click: a unique click when first, a total click always.
INSERT INTO campaign_click (campaign_id, channel, language, timezone, unique_clicks, total_clicks)
VALUES ($1, $2, $3, $4, CASE WHEN sqlc.arg(first)::boolean THEN 1 ELSE 0 END, 1)
ON CONFLICT (campaign_id, channel, language, timezone) DO UPDATE
SET unique_clicks = campaign_click.unique_clicks + EXCLUDED.unique_clicks,
    total_clicks = campaign_click.total_clicks + 1;

-- name: CampaignLocaleClicks :many
-- Sent and clicks of one campaign per (channel, language, timezone). A slot that has clicks but no
-- sent row (the campaign has not completed yet) shows sent 0; one with sent and no clicks shows 0 clicks.
SELECT COALESCE(s.channel, c.channel)::text AS channel,
       COALESCE(s.language, c.language)::text AS language,
       COALESCE(s.timezone, c.timezone)::text AS timezone,
       COALESCE(s.sent, 0)::bigint AS sent,
       COALESCE(c.unique_clicks, 0)::bigint AS unique_clicks,
       COALESCE(c.total_clicks, 0)::bigint AS total_clicks
FROM (SELECT l.channel, l.language, l.timezone, l.sent FROM campaign_sent_locale l WHERE l.campaign_id = $1) s
FULL JOIN (SELECT k.channel, k.language, k.timezone, k.unique_clicks, k.total_clicks FROM campaign_click k WHERE k.campaign_id = $1) c
  ON c.channel = s.channel AND c.language = s.language AND c.timezone = s.timezone
ORDER BY 1, 2, 3;

-- name: CampaignHasClicks :one
SELECT EXISTS (SELECT 1 FROM campaign_click WHERE campaign_id = $1);

-- name: ClickOverviewByChannel :many
-- Sent and clicks per channel over the campaigns completed in [from_ts, to_ts) of completed_at: the
-- same campaigns as the delivery figures of the overview, so the click-through rate compares clicks
-- and sends of one cohort (D-108). A NULL bound is open-ended.
WITH cohort AS (
    SELECT campaign_id FROM campaign_summary
    WHERE completed_at >= COALESCE(sqlc.narg('from_ts')::timestamptz, '-infinity')
      AND completed_at <  COALESCE(sqlc.narg('to_ts')::timestamptz, 'infinity')
), sent AS (
    SELECT channel, sum(sent) AS sent FROM campaign_sent_locale
    WHERE campaign_id IN (SELECT campaign_id FROM cohort) GROUP BY channel
), clicks AS (
    SELECT channel, sum(unique_clicks) AS unique_clicks, sum(total_clicks) AS total_clicks FROM campaign_click
    WHERE campaign_id IN (SELECT campaign_id FROM cohort) GROUP BY channel
)
SELECT COALESCE(s.channel, c.channel)::text AS channel,
       COALESCE(s.sent, 0)::bigint AS sent,
       COALESCE(c.unique_clicks, 0)::bigint AS unique_clicks,
       COALESCE(c.total_clicks, 0)::bigint AS total_clicks
FROM sent s FULL JOIN clicks c ON c.channel = s.channel
ORDER BY 1;

-- name: ClickSeries :many
-- Clicks per (UTC bucket of clicked_at, channel) over [from_ts, to_ts); unique_clicks counts the
-- first clicks of a delivery. Buckets without a click have no row (see SeriesBuckets). Served by
-- click_event_clicked_at_idx.
SELECT date_trunc(sqlc.arg('bucket')::text, clicked_at, 'UTC')::timestamptz AS bucket_start,
       channel,
       (count(*) FILTER (WHERE first))::bigint AS unique_clicks,
       count(*)::bigint AS total_clicks
FROM click_event
WHERE clicked_at >= sqlc.arg('from_ts')::timestamptz
  AND clicked_at <  sqlc.arg('to_ts')::timestamptz
GROUP BY 1, channel
ORDER BY 1, channel;
