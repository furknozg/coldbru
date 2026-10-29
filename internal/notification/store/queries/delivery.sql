-- name: InsertDelivery :execrows
-- Returns 1 when the delivery was new and 0 when (campaign, customer, channel) already had one (D-023).
INSERT INTO delivery (campaign_id, schedule_id, message_id, customer_id, channel, first_name, contact)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (campaign_id, customer_id, channel) DO NOTHING;

-- name: ListPendingDeliveries :many
-- Served by delivery_schedule_id_pending_idx. Ordered by id (UUIDv7, so by creation) for stable paging.
SELECT * FROM delivery WHERE schedule_id = $1 AND status = 'pending' ORDER BY id LIMIT $2;

-- name: CountDeliveriesByStatus :many
-- Served by delivery_campaign_channel_status_idx (D-036).
SELECT channel, status, count(*) AS total FROM delivery WHERE campaign_id = $1
GROUP BY channel, status ORDER BY channel, status;

-- name: ClearDeliveryPII :execrows
-- Run in the transaction that completes or cancels the campaign (D-077). A delivery that is still
-- pending or sending is rejected by delivery_pii_check and aborts the statement, on purpose.
UPDATE delivery SET first_name = NULL, contact = NULL, updated_at = now()
WHERE campaign_id = $1 AND (first_name IS NOT NULL OR contact IS NOT NULL);

-- name: InsertDeliveryBatch :batchexec
-- InsertDelivery for a whole recipient batch in one round trip (D-077): one queued statement per
-- recipient, same ON CONFLICT rule as InsertDelivery (D-023).
INSERT INTO delivery (campaign_id, schedule_id, message_id, customer_id, channel, first_name, contact)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (campaign_id, customer_id, channel) DO NOTHING;

-- name: CancelPendingDeliveries :execrows
-- Run before ClearDeliveryPII in the cancellation transaction (D-078): a pending delivery cannot lose
-- its personal data (delivery_pii_check). Deliveries that are sending, sent, failed or skipped keep their state.
UPDATE delivery SET status = 'cancelled', updated_at = now() WHERE campaign_id = $1 AND status = 'pending';
