-- name: CreateCampaign :one
INSERT INTO campaign (brief) VALUES ($1) RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaign WHERE id = $1;
