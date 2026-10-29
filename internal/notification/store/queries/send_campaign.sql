-- name: CreateSendCampaign :one
-- id is Proposal's campaign id (D-074).
INSERT INTO send_campaign (id, cta_url, batch_count) VALUES ($1, $2, $3) RETURNING *;

-- name: GetSendCampaign :one
SELECT * FROM send_campaign WHERE id = $1;
