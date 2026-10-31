-- name: ListCompletableCampaigns :many
-- Sending campaigns that look finished: every schedule has fired (none is still scheduled) and no
-- delivery is pending or sending. Only a pre-filter: the checker locks the campaign and asks again
-- (GetCampaignProgress) before it completes it. The delivery probe is served by
-- delivery_campaign_channel_status_idx (D-077).
SELECT c.id FROM send_campaign c
WHERE c.status = 'sending'
  AND NOT EXISTS (SELECT 1 FROM locale_schedule s WHERE s.campaign_id = c.id AND s.status = 'scheduled')
  AND NOT EXISTS (SELECT 1 FROM delivery d WHERE d.campaign_id = c.id AND d.status IN ('pending', 'sending'))
ORDER BY c.id;

-- name: GetCampaignProgress :one
-- The two numbers the completion predicate needs, read in the transaction that holds the campaign lock.
SELECT c.status,
       (SELECT count(*) FROM locale_schedule s WHERE s.campaign_id = c.id AND s.status = 'scheduled') AS scheduled_schedules,
       (SELECT count(*) FROM delivery d WHERE d.campaign_id = c.id AND d.status IN ('pending', 'sending')) AS active_deliveries
FROM send_campaign c WHERE c.id = $1;

-- name: CompleteSendCampaign :one
-- sending -> completed (D-080). No row means the campaign is not sending any more (completed
-- already, or cancelled), so a repeat is a no-op.
UPDATE send_campaign SET status = 'completed', completed_at = now(), updated_at = now()
WHERE id = $1 AND status = 'sending'
RETURNING completed_at;

-- name: SumSentSMSSegments :one
-- The segments of the SMS that were sent: for each sent SMS delivery the segment count measured and
-- stored with the approved message (content.SMSContent.segments, D-071). A message without a numeric
-- segments value counts as zero. It is an estimate when the real first name changes the encoding or
-- the length (D-080).
SELECT coalesce(sum(CASE WHEN jsonb_typeof(m.content -> 'segments') = 'number' THEN (m.content ->> 'segments')::numeric ELSE 0 END), 0)::bigint AS segments
FROM delivery d
JOIN message m ON m.id = d.message_id
WHERE d.campaign_id = $1 AND d.channel = 'sms' AND d.status = 'sent';

-- name: CountSentByLocale :many
-- The deliveries that were sent, per channel and the language and time zone of their send slot
-- (D-108): the denominators of the click-through rate, written into the completed event. The language
-- is that of the delivery's message, the time zone that of its schedule. Served by
-- delivery_campaign_channel_status_idx.
SELECT d.channel, m.language, s.timezone, count(*)::bigint AS sent
FROM delivery d
JOIN message m ON m.id = d.message_id
JOIN locale_schedule s ON s.id = d.schedule_id
WHERE d.campaign_id = $1 AND d.status = 'sent'
GROUP BY d.channel, m.language, s.timezone
ORDER BY d.channel, m.language, s.timezone;
