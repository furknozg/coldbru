-- name: CreateLocaleSchedule :one
INSERT INTO locale_schedule (campaign_id, language, timezone, send_at) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListLocaleSchedules :many
SELECT * FROM locale_schedule WHERE campaign_id = $1 ORDER BY language, timezone;
