-- name: InsertContextNote :one
INSERT INTO context_note (campaign_id, step_id, body) VALUES ($1, $2, $3) RETURNING *;

-- name: ListContextNotes :many
SELECT * FROM context_note WHERE campaign_id = $1 ORDER BY id;
