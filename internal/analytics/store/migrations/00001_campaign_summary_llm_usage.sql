-- Analytics read-side tables (D-083): one summary per completed campaign with its delivery counts,
-- and one row per Anthropic call (llm_usage) fed by evt.proposal.token_completed.v1. Status,
-- channel, purpose and outcome columns are text + CHECK, not ENUM types. Money is numeric, never
-- float, and always USD. There is no foreign key between the tables: a call's usage arrives before
-- its campaign completes, and a campaign may never complete. Tables land in the analytics schema
-- through the role's search_path (D-048).

-- +goose Up
CREATE TABLE campaign_summary (
    -- Proposal's and Notification's campaign id, from the completed event; never generated here.
    campaign_id       uuid PRIMARY KEY,
    completed_at      timestamptz NOT NULL,
    sms_segments      int NOT NULL,
    -- The estimate of the completed event (four decimals there, six here). The campaign's LLM cost
    -- is not stored: reports sum llm_usage.cost_usd by campaign_id (D-083).
    delivery_cost_usd numeric(14,6) NOT NULL,
    -- Set by the application on every change; there is no trigger.
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT campaign_summary_sms_segments_check CHECK (sms_segments >= 0),
    CONSTRAINT campaign_summary_delivery_cost_usd_check CHECK (delivery_cost_usd >= 0)
);

CREATE TABLE campaign_delivery_count (
    campaign_id uuid NOT NULL,
    channel     text NOT NULL,
    status      text NOT NULL,
    count       int NOT NULL,
    CONSTRAINT campaign_delivery_count_pkey PRIMARY KEY (campaign_id, channel, status),
    CONSTRAINT campaign_delivery_count_channel_check CHECK (channel IN ('sms', 'email')),
    CONSTRAINT campaign_delivery_count_status_check CHECK (
        status IN ('pending', 'sending', 'sent', 'failed', 'skipped_no_consent', 'cancelled')
    ),
    CONSTRAINT campaign_delivery_count_count_check CHECK (count >= 0)
);

CREATE TABLE llm_usage (
    -- Proposal's llm_call.id; the key that makes a redelivered event a no-op.
    llm_call_id           uuid PRIMARY KEY,
    campaign_id           uuid NOT NULL,
    purpose               text NOT NULL,
    model                 text NOT NULL,
    prompt_name           text NOT NULL,
    prompt_version        text NOT NULL,
    input_tokens          int NOT NULL,
    output_tokens         int NOT NULL,
    cache_read_tokens     int NOT NULL,
    cache_creation_tokens int NOT NULL,
    outcome               text NOT NULL,
    latency_ms            int NOT NULL,
    -- Priced from the model price table when the event arrived (D-083); 0 for an unknown model.
    cost_usd              numeric(14,8) NOT NULL,
    created_at            timestamptz NOT NULL,
    CONSTRAINT llm_usage_purpose_check CHECK (purpose IN ('screen', 'segment', 'content', 'channel_plan', 'schedule')),
    CONSTRAINT llm_usage_model_check CHECK (model ~ '\S'),
    CONSTRAINT llm_usage_prompt_name_check CHECK (prompt_name ~ '\S'),
    CONSTRAINT llm_usage_prompt_version_check CHECK (prompt_version ~ '\S'),
    CONSTRAINT llm_usage_input_tokens_check CHECK (input_tokens >= 0),
    CONSTRAINT llm_usage_output_tokens_check CHECK (output_tokens >= 0),
    CONSTRAINT llm_usage_cache_read_tokens_check CHECK (cache_read_tokens >= 0),
    CONSTRAINT llm_usage_cache_creation_tokens_check CHECK (cache_creation_tokens >= 0),
    CONSTRAINT llm_usage_outcome_check CHECK (outcome IN ('accepted', 'validation_failed', 'refusal', 'api_error')),
    CONSTRAINT llm_usage_latency_ms_check CHECK (latency_ms >= 0),
    CONSTRAINT llm_usage_cost_usd_check CHECK (cost_usd >= 0)
);

-- One campaign's calls: the per-campaign report and the cost sum.
CREATE INDEX llm_usage_campaign_id_idx ON llm_usage (campaign_id);

-- One prompt version's calls: the repair rate and cost per version report (D-060).
CREATE INDEX llm_usage_prompt_name_version_idx ON llm_usage (prompt_name, prompt_version);

-- +goose Down
DROP INDEX llm_usage_prompt_name_version_idx;
DROP INDEX llm_usage_campaign_id_idx;
DROP TABLE llm_usage;
DROP TABLE campaign_delivery_count;
DROP TABLE campaign_summary;
