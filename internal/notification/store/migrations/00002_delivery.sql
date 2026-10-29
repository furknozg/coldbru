-- One delivery per (campaign, customer, channel) (D-023, D-077). id is the UUIDv7 sent to the
-- provider as Idempotency-Key. first_name and contact are personal data: required while a
-- delivery is pending or sending, cleared by the application when the campaign finishes (D-077).
-- Foreign keys are plain (no cascade). schedule_id and message_id use composite keys, like
-- proposal_variant_step_kind_fkey (D-068): the schedule must belong to the delivery's campaign,
-- the message to its campaign and channel.

-- +goose Up
CREATE TABLE delivery (
    id                  uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id         uuid NOT NULL REFERENCES send_campaign (id),
    schedule_id         uuid NOT NULL,
    message_id          uuid NOT NULL,
    customer_id         text NOT NULL,
    channel             text NOT NULL,
    first_name          text,
    contact             text,
    status              text NOT NULL DEFAULT 'pending',
    attempts            int NOT NULL DEFAULT 0,
    lease_until         timestamptz,
    provider_message_id text,
    last_error          text,
    sent_at             timestamptz,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT delivery_schedule_campaign_fkey FOREIGN KEY (schedule_id, campaign_id)
        REFERENCES locale_schedule (id, campaign_id),
    CONSTRAINT delivery_message_campaign_channel_fkey FOREIGN KEY (message_id, campaign_id, channel)
        REFERENCES message (id, campaign_id, channel),
    CONSTRAINT delivery_channel_check CHECK (channel IN ('sms', 'email')),
    CONSTRAINT delivery_status_check CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped_no_consent', 'cancelled')),
    CONSTRAINT delivery_attempts_check CHECK (attempts >= 0),
    CONSTRAINT delivery_sent_at_check CHECK (status <> 'sent' OR sent_at IS NOT NULL),
    CONSTRAINT delivery_last_error_check CHECK (status <> 'failed' OR last_error IS NOT NULL),
    CONSTRAINT delivery_lease_until_check CHECK (status <> 'sending' OR lease_until IS NOT NULL),
    -- Personal data may be cleared only once a delivery is final (D-077).
    CONSTRAINT delivery_pii_check CHECK (
        status IN ('sent', 'failed', 'skipped_no_consent', 'cancelled')
        OR (first_name IS NOT NULL AND contact IS NOT NULL)
    ),
    -- The dedup key (D-023); also serves as the index on campaign_id (foreign key).
    CONSTRAINT delivery_campaign_customer_channel_key UNIQUE (campaign_id, customer_id, channel)
);

-- Dashboard counts per channel and status (D-036).
CREATE INDEX delivery_campaign_channel_status_idx ON delivery (campaign_id, channel, status);

-- Pending deliveries of one schedule, read when the schedule fires (D-077).
CREATE INDEX delivery_schedule_id_pending_idx ON delivery (schedule_id) WHERE status = 'pending';

-- Foreign key indexes not covered above (their leading column is the parent's id). The partial
-- index cannot serve schedule_id lookups of non-pending rows, so schedule_id gets a full index too.
CREATE INDEX delivery_schedule_id_idx ON delivery (schedule_id);
CREATE INDEX delivery_message_id_idx ON delivery (message_id);

-- +goose Down
DROP INDEX delivery_message_id_idx;
DROP INDEX delivery_schedule_id_idx;
DROP INDEX delivery_schedule_id_pending_idx;
DROP INDEX delivery_campaign_channel_status_idx;
DROP TABLE delivery;
