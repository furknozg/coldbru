-- name: InsertLink :batchexec
-- One queued statement per link of a request, sent in one round trip (D-106). A delivery that has a
-- link already (a retry) and a code that is taken (a one in 56 billion collision) insert nothing;
-- the caller reads the result with ListLinksByDelivery and draws new codes for the deliveries that
-- still have none.
INSERT INTO link (code, delivery_id, campaign_id, channel, language, timezone, target_url)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING;

-- name: ListLinksByDelivery :many
-- Served by link_delivery_id_key.
SELECT delivery_id, code FROM link WHERE delivery_id = ANY(sqlc.arg(delivery_ids)::uuid[]);

-- name: LockLink :one
-- The link of a code, locked until the transaction ends: concurrent clicks of one delivery take
-- turns, so exactly one of them is the first (D-107).
SELECT * FROM link WHERE code = $1 FOR UPDATE;
