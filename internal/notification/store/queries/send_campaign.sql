-- name: CreateSendCampaign :one
-- id is Proposal's campaign id (D-074).
INSERT INTO send_campaign (id, cta_url, batch_count) VALUES ($1, $2, $3) RETURNING *;

-- name: GetSendCampaign :one
SELECT * FROM send_campaign WHERE id = $1;

-- name: InsertSendCampaign :execrows
-- Returns 1 when the campaign was new and 0 when Proposal's confirmed event for it was already
-- applied (D-078). Unlike CreateSendCampaign it does not fail on a repeat.
INSERT INTO send_campaign (id, cta_url, batch_count) VALUES ($1, $2, $3)
ON CONFLICT (id) DO NOTHING;

-- name: LockSendCampaign :one
-- Serialises the tx with every other that locks the campaign: batch intake, a schedule firing and
-- the cancellation all take this lock first, so none sees a half-applied state of another (D-078).
SELECT * FROM send_campaign WHERE id = $1 FOR UPDATE;

-- name: StartSendCampaign :execrows
-- scheduled -> sending when the first schedule fires; 0 rows when it is already past that.
UPDATE send_campaign SET status = 'sending', updated_at = now() WHERE id = $1 AND status = 'scheduled';

-- name: CancelSendCampaign :execrows
-- Cancels a campaign that has not finished (D-078); 0 rows for completed or already cancelled.
UPDATE send_campaign SET status = 'cancelled', updated_at = now()
WHERE id = $1 AND status IN ('scheduled', 'sending');
