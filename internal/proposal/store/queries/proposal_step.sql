-- name: CreateStep :one
INSERT INTO proposal_step (campaign_id, kind, version) VALUES ($1, $2, $3) RETURNING *;

-- name: ListSteps :many
-- UUIDv7 ids sort by creation time (D-030).
SELECT * FROM proposal_step WHERE campaign_id = $1 ORDER BY id;
