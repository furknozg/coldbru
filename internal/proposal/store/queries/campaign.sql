-- name: CreateCampaign :one
INSERT INTO campaign (brief) VALUES ($1) RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaign WHERE id = $1;

-- name: GetCampaignForUpdate :one
-- Row lock that serializes every state change of one campaign (D-070).
SELECT * FROM campaign WHERE id = $1 FOR UPDATE;

-- name: SetCampaignStatus :one
UPDATE campaign SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: ListCampaigns :many
-- Newest first, for the campaign list of the UI (D-072). Not indexed on purpose: the list is small
-- and a campaign table of this service stays small.
SELECT * FROM campaign ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2;

-- name: SetCampaignTitle :one
UPDATE campaign SET title = $2, updated_at = now() WHERE id = $1 RETURNING *;
