-- name: CreateCampaign :one
INSERT INTO campaign (brief) VALUES ($1) RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaign WHERE id = $1;

-- name: GetCampaignForUpdate :one
-- Row lock that serializes every state change of one campaign (D-070).
SELECT * FROM campaign WHERE id = $1 FOR UPDATE;

-- name: SetCampaignStatus :one
UPDATE campaign SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;
