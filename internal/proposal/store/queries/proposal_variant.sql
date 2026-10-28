-- name: CreateVariant :one
INSERT INTO proposal_variant (step_id, kind, language, channel, content)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListVariants :many
SELECT * FROM proposal_variant WHERE step_id = $1 ORDER BY id;

-- name: GetVariant :one
SELECT * FROM proposal_variant WHERE id = $1;

-- name: ListLiveVariants :many
-- The variants a marketer still sees in a step: everything except superseded (D-022).
SELECT * FROM proposal_variant WHERE step_id = $1 AND status <> 'superseded' ORDER BY id;

-- name: ApproveVariant :one
UPDATE proposal_variant SET status = 'approved', approved_at = now() WHERE id = $1 RETURNING *;

-- name: RejectVariant :one
UPDATE proposal_variant SET status = 'rejected', rejected_at = now(), rejection_reason = $2 WHERE id = $1 RETURNING *;

-- name: SupersedeVariant :one
UPDATE proposal_variant SET status = 'superseded' WHERE id = $1 RETURNING *;
