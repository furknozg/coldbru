-- name: InsertLLMCall :one
INSERT INTO llm_call (
    campaign_id, step_id, variant_id, purpose, model, attempt,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
    stop_reason, latency_ms, prompt_name, prompt_version, prompt_sha256,
    outcome, response, error
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, $17, $18
)
RETURNING *;

-- name: ListLLMCalls :many
SELECT * FROM llm_call WHERE campaign_id = $1 ORDER BY id;
