-- name: CreateStep :one
INSERT INTO proposal_step (campaign_id, kind, version) VALUES ($1, $2, $3) RETURNING *;

-- name: ListSteps :many
-- UUIDv7 ids sort by creation time (D-030).
SELECT * FROM proposal_step WHERE campaign_id = $1 ORDER BY id;

-- name: GetStep :one
SELECT * FROM proposal_step WHERE id = $1;

-- name: ListLiveSteps :many
-- The current steps of a campaign: everything that is not superseded (D-033).
SELECT * FROM proposal_step WHERE campaign_id = $1 AND status <> 'superseded' ORDER BY id;

-- name: MaxStepVersion :one
SELECT COALESCE(max(version), 0)::int AS version FROM proposal_step WHERE campaign_id = $1 AND kind = $2;

-- name: ApproveStep :one
UPDATE proposal_step SET status = 'approved', approved_at = now() WHERE id = $1 RETURNING *;

-- name: SupersedeStep :one
UPDATE proposal_step SET status = 'superseded' WHERE id = $1 RETURNING *;
