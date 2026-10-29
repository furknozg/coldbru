-- name: InsertRecipientBatch :execrows
-- Returns 1 when the batch was new and 0 when it was already recorded; the event arrives at least once (D-077).
INSERT INTO recipient_batch (campaign_id, batch_no, recipients) VALUES ($1, $2, $3)
ON CONFLICT (campaign_id, batch_no) DO NOTHING;

-- name: CountRecipientBatches :one
SELECT count(*) FROM recipient_batch WHERE campaign_id = $1;
