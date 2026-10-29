-- name: InsertLLMUsage :execrows
-- A redelivered or repeated llm_call_id inserts nothing (D-083).
INSERT INTO llm_usage (
    llm_call_id, campaign_id, purpose, model, prompt_name, prompt_version,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
    outcome, latency_ms, cost_usd, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14
)
ON CONFLICT (llm_call_id) DO NOTHING;

-- name: LLMTotalsOfCampaign :one
SELECT count(*)::bigint AS calls,
       COALESCE(sum(cost_usd), 0)::numeric(14,8)::text AS cost_usd,
       COALESCE(sum(input_tokens), 0)::bigint AS input_tokens,
       COALESCE(sum(output_tokens), 0)::bigint AS output_tokens,
       COALESCE(sum(cache_read_tokens), 0)::bigint AS cache_read_tokens,
       COALESCE(sum(cache_creation_tokens), 0)::bigint AS cache_creation_tokens
FROM llm_usage
WHERE campaign_id = $1;

-- name: LLMByPurposeOfCampaign :many
SELECT purpose,
       count(*)::bigint AS calls,
       COALESCE(sum(cost_usd), 0)::numeric(14,8)::text AS cost_usd,
       COALESCE(sum(input_tokens), 0)::bigint AS input_tokens,
       COALESCE(sum(output_tokens), 0)::bigint AS output_tokens
FROM llm_usage
WHERE campaign_id = $1
GROUP BY purpose
ORDER BY purpose;

-- name: LLMUsageByPromptVersion :many
-- One row per (prompt_name, prompt_version), served by llm_usage_prompt_name_version_idx.
SELECT prompt_name, prompt_version,
       count(*)::bigint AS calls,
       (count(*) FILTER (WHERE outcome = 'accepted'))::bigint AS accepted,
       (count(*) FILTER (WHERE outcome = 'validation_failed'))::bigint AS validation_failed,
       (count(*) FILTER (WHERE outcome = 'refusal'))::bigint AS refusal,
       (count(*) FILTER (WHERE outcome = 'api_error'))::bigint AS api_error,
       COALESCE(avg(latency_ms), 0)::float8 AS avg_latency_ms,
       COALESCE(sum(cost_usd), 0)::numeric(14,8)::text AS cost_usd
FROM llm_usage
GROUP BY prompt_name, prompt_version
ORDER BY prompt_name, prompt_version;
