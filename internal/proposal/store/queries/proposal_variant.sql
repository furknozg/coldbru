-- name: CreateVariant :one
INSERT INTO proposal_variant (step_id, kind, language, channel, content)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListVariants :many
SELECT * FROM proposal_variant WHERE step_id = $1 ORDER BY id;
