-- name: CreateMessage :one
INSERT INTO message (campaign_id, language, channel, content) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListMessages :many
SELECT * FROM message WHERE campaign_id = $1 ORDER BY language, channel;
