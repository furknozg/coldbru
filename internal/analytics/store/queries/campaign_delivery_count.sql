-- name: DeleteDeliveryCounts :exec
DELETE FROM campaign_delivery_count WHERE campaign_id = $1;

-- name: InsertDeliveryCount :exec
-- At most 12 (channel, status) pairs exist, so a completion inserts them one by one.
INSERT INTO campaign_delivery_count (campaign_id, channel, status, count)
VALUES ($1, $2, $3, $4);

-- name: ListDeliveryCounts :many
SELECT channel, status, count FROM campaign_delivery_count
WHERE campaign_id = $1
ORDER BY channel, status;
