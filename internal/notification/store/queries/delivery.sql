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
-- Run in the transaction that completes or cancels the campaign (D-077, D-078). Only deliveries that
-- are final lose their personal data: a pending or sending one still needs it. The cancellation
-- cancels the pending ones first, so they are cleared too; a delivery that is still sending is
-- cleared by the send worker when it settles it (the settle statements do that for a cancelled
-- campaign). At the end of a campaign every delivery is final, so all of them are cleared.
UPDATE delivery SET first_name = NULL, contact = NULL, updated_at = now()
WHERE campaign_id = $1 AND status NOT IN ('pending', 'sending') AND (first_name IS NOT NULL OR contact IS NOT NULL);

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
