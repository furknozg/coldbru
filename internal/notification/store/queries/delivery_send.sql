-- Queries of the send worker (D-079). Every state change names the state it expects in WHERE, so a
-- stale worker (a lease that ran out, a duplicate SEND message, a cancellation that won) changes
-- nothing and learns it from the row count.

-- name: ListSendDeliveries :many
-- The rows a batch of SEND messages points at: the delivery, the approved content of its message and
-- the campaign's status and button link. Ids that do not exist are simply absent from the result.
SELECT d.id, d.campaign_id, d.customer_id, d.channel, d.first_name, d.contact, d.status, d.attempts, d.lease_until,
       m.language, m.content, c.status AS campaign_status, c.cta_url
FROM delivery d
JOIN message m ON m.id = d.message_id
JOIN send_campaign c ON c.id = d.campaign_id
WHERE d.id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ClaimDelivery :one
-- pending -> sending with a lease, or takes over a sending delivery whose lease ran out (a worker
-- that crashed). No row means somebody else has it or it is final or its campaign is cancelled.
-- attempts is the fencing token of the claim: the statements that finish it must present it.
UPDATE delivery SET status = 'sending', attempts = attempts + 1, updated_at = now(),
    lease_until = now() + make_interval(secs => sqlc.arg(lease_seconds)::float8)
WHERE delivery.id = sqlc.arg(id)
  AND (status = 'pending' OR (status = 'sending' AND lease_until < now()))
  AND NOT EXISTS (SELECT 1 FROM send_campaign c WHERE c.id = delivery.campaign_id AND c.status = 'cancelled')
RETURNING attempts, first_name, contact;

-- The three statements that settle a claimed delivery read the campaign status under FOR SHARE: the
-- cancellation holds the campaign row FOR UPDATE until it commits, so a settle either sees
-- 'cancelled' (and clears the personal data) or finishes first, and the cancellation then clears
-- the final row itself. Without the lock a delivery settled concurrently would keep its data.

-- name: MarkDeliverySent :execrows
-- sending -> sent. Personal data is cleared here only when the campaign was cancelled meanwhile
-- (D-078); otherwise it stays until the campaign ends (D-077).
WITH c AS (SELECT status FROM send_campaign WHERE id = (SELECT campaign_id FROM delivery WHERE delivery.id = sqlc.arg(id)) FOR SHARE)
UPDATE delivery d SET status = 'sent', sent_at = now(), provider_message_id = sqlc.arg(provider_message_id),
    lease_until = NULL, last_error = NULL, updated_at = now(),
    first_name = CASE WHEN c.status = 'cancelled' THEN NULL ELSE d.first_name END,
    contact = CASE WHEN c.status = 'cancelled' THEN NULL ELSE d.contact END
FROM c
WHERE d.id = sqlc.arg(id) AND d.status = 'sending' AND d.attempts = sqlc.arg(attempts);

-- name: MarkDeliveryFailed :execrows
-- sending -> failed (a permanent error or the last delivery). Clears personal data when the
-- campaign was cancelled meanwhile.
WITH c AS (SELECT status FROM send_campaign WHERE id = (SELECT campaign_id FROM delivery WHERE delivery.id = sqlc.arg(id)) FOR SHARE)
UPDATE delivery d SET status = 'failed', last_error = sqlc.arg(last_error), lease_until = NULL, updated_at = now(),
    first_name = CASE WHEN c.status = 'cancelled' THEN NULL ELSE d.first_name END,
    contact = CASE WHEN c.status = 'cancelled' THEN NULL ELSE d.contact END
FROM c
WHERE d.id = sqlc.arg(id) AND d.status = 'sending' AND d.attempts = sqlc.arg(attempts);

-- name: ReleaseDelivery :one
-- sending -> pending after a failure that will be retried. If the campaign was cancelled meanwhile
-- the delivery becomes cancelled and loses its personal data instead; the returned status says which.
WITH c AS (SELECT status FROM send_campaign WHERE id = (SELECT campaign_id FROM delivery WHERE delivery.id = sqlc.arg(id)) FOR SHARE)
UPDATE delivery d SET lease_until = NULL, last_error = sqlc.arg(last_error), updated_at = now(),
    status = CASE WHEN c.status = 'cancelled' THEN 'cancelled' ELSE 'pending' END,
    first_name = CASE WHEN c.status = 'cancelled' THEN NULL ELSE d.first_name END,
    contact = CASE WHEN c.status = 'cancelled' THEN NULL ELSE d.contact END
FROM c
WHERE d.id = sqlc.arg(id) AND d.status = 'sending' AND d.attempts = sqlc.arg(attempts)
RETURNING d.status;

-- name: SkipDeliveryNoConsent :execrows
-- pending (or a sending delivery whose lease ran out) -> skipped_no_consent (D-011). Personal data
-- stays until the campaign ends (D-077).
UPDATE delivery SET status = 'skipped_no_consent', lease_until = NULL, updated_at = now()
WHERE id = $1 AND (status = 'pending' OR (status = 'sending' AND lease_until < now()));

-- name: CancelPendingDelivery :execrows
-- A SEND message that arrives for a delivery of a cancelled campaign which is still pending (the
-- cancellation clears personal data for finished rows only, D-078): cancelled, personal data cleared.
UPDATE delivery SET status = 'cancelled', first_name = NULL, contact = NULL, lease_until = NULL, updated_at = now()
WHERE id = $1 AND status = 'pending';
