-- Append-only audit tables (D-022, D-060, D-068): context_note holds the marketer's extra context,
-- llm_call one row per model attempt. A trigger rejects UPDATE and DELETE on both.

-- +goose Up
CREATE TABLE context_note (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id uuid NOT NULL REFERENCES campaign (id),
    -- The step during which the note was given.
    step_id     uuid REFERENCES proposal_step (id),
    body        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT context_note_body_check CHECK (body ~ '\S')
);

CREATE INDEX context_note_campaign_id_idx ON context_note (campaign_id);
CREATE INDEX context_note_step_id_idx ON context_note (step_id);

CREATE TABLE llm_call (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id           uuid NOT NULL REFERENCES campaign (id),
    step_id               uuid REFERENCES proposal_step (id),
    -- The variant this call produced, when it produced one; the outcome says whether it was accepted.
    variant_id            uuid REFERENCES proposal_variant (id),
    purpose               text NOT NULL,
    model                 text NOT NULL,
    attempt               int NOT NULL,
    input_tokens          int NOT NULL DEFAULT 0,
    output_tokens         int NOT NULL DEFAULT 0,
    cache_read_tokens     int NOT NULL DEFAULT 0,
    cache_creation_tokens int NOT NULL DEFAULT 0,
    stop_reason           text,
    latency_ms            int NOT NULL,
    prompt_name           text NOT NULL,
    prompt_version        text NOT NULL,
    prompt_sha256         text NOT NULL,
    outcome               text NOT NULL,
    -- The model's structured output, kept even when validation rejected it.
    response              jsonb,
    error                 text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT llm_call_purpose_check CHECK (purpose IN ('screen', 'segment', 'content')),
    CONSTRAINT llm_call_attempt_check CHECK (attempt >= 1),
    CONSTRAINT llm_call_input_tokens_check CHECK (input_tokens >= 0),
    CONSTRAINT llm_call_output_tokens_check CHECK (output_tokens >= 0),
    CONSTRAINT llm_call_cache_read_tokens_check CHECK (cache_read_tokens >= 0),
    CONSTRAINT llm_call_cache_creation_tokens_check CHECK (cache_creation_tokens >= 0),
    CONSTRAINT llm_call_latency_ms_check CHECK (latency_ms >= 0),
    CONSTRAINT llm_call_prompt_sha256_check CHECK (prompt_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT llm_call_outcome_check CHECK (outcome IN ('accepted', 'validation_failed', 'refusal', 'api_error')),
    CONSTRAINT llm_call_api_error_check CHECK (outcome <> 'api_error' OR error IS NOT NULL)
);

CREATE INDEX llm_call_campaign_id_idx ON llm_call (campaign_id);
CREATE INDEX llm_call_step_id_idx ON llm_call (step_id);
CREATE INDEX llm_call_variant_id_idx ON llm_call (variant_id);

-- +goose StatementBegin
CREATE FUNCTION reject_row_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only: % is not allowed', TG_TABLE_NAME, TG_OP;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER context_note_append_only
    BEFORE UPDATE OR DELETE ON context_note
    FOR EACH ROW EXECUTE FUNCTION reject_row_change();

CREATE TRIGGER llm_call_append_only
    BEFORE UPDATE OR DELETE ON llm_call
    FOR EACH ROW EXECUTE FUNCTION reject_row_change();

-- +goose Down
DROP TRIGGER llm_call_append_only ON llm_call;
DROP TRIGGER context_note_append_only ON context_note;
DROP FUNCTION reject_row_change();
DROP INDEX llm_call_variant_id_idx;
DROP INDEX llm_call_step_id_idx;
DROP INDEX llm_call_campaign_id_idx;
DROP TABLE llm_call;
DROP INDEX context_note_step_id_idx;
DROP INDEX context_note_campaign_id_idx;
DROP TABLE context_note;
