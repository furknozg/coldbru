-- name: CreateLocaleSchedule :one
INSERT INTO locale_schedule (campaign_id, language, timezone, send_at) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListLocaleSchedules :many
SELECT * FROM locale_schedule WHERE campaign_id = $1 ORDER BY language, timezone;

-- name: LockLocaleSchedule :one
-- Taken after LockSendCampaign by the firing, so a firing and a cancellation of one schedule never overlap (D-078).
SELECT * FROM locale_schedule WHERE id = $1 FOR UPDATE;

-- name: MarkScheduleFired :execrows
-- 0 rows when the schedule is no longer 'scheduled' (fired or cancelled).
UPDATE locale_schedule SET status = 'fired', fired_at = now() WHERE id = $1 AND status = 'scheduled';

-- name: CancelScheduledSchedules :execrows
-- Cancels the schedules of a campaign that have not fired (D-078).
UPDATE locale_schedule SET status = 'cancelled' WHERE campaign_id = $1 AND status = 'scheduled';
