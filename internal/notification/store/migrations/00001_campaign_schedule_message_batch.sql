-- Notification send-side tables (D-077): the campaign Proposal confirmed, its (language, timezone)
-- send slots, its message texts and the bookkeeping of received recipient batches. IDs are UUIDv7
-- (D-030) except send_campaign.id, which is Proposal's campaign id and so has no default. Status
-- and kind columns are text + CHECK, not ENUM types. Foreign keys are plain: rows are never
-- deleted. Tables land in the notification schema through the role's search_path (D-048).

-- +goose Up
CREATE TABLE send_campaign (
    -- Proposal's campaign.id, copied from the confirm event (D-074); never generated here.
    id           uuid PRIMARY KEY,
    status       text NOT NULL DEFAULT 'scheduled',
    cta_url      text NOT NULL,
    -- How many RECIPIENTS batches the campaign has; a schedule fires only when all arrived (D-077).
    batch_count  int NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    -- Set by the application on every change; there is no trigger.
    updated_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    CONSTRAINT send_campaign_status_check CHECK (status IN ('scheduled', 'sending', 'completed', 'cancelled')),
    CONSTRAINT send_campaign_cta_url_check CHECK (cta_url ~ '\S'),
    CONSTRAINT send_campaign_batch_count_check CHECK (batch_count >= 1),
    -- Cancellation time is not required: only a completed campaign must carry completed_at.
    CONSTRAINT send_campaign_completed_at_check CHECK (status <> 'completed' OR completed_at IS NOT NULL)
);

CREATE TABLE locale_schedule (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id uuid NOT NULL REFERENCES send_campaign (id),
    language    text NOT NULL,
    timezone    text NOT NULL,
    -- NULL means send when the schedule is processed (D-077).
    send_at     timestamptz,
    status      text NOT NULL DEFAULT 'scheduled',
    fired_at    timestamptz,
    CONSTRAINT locale_schedule_language_check CHECK (language IN ('tr', 'en')),
    CONSTRAINT locale_schedule_timezone_check CHECK (timezone ~ '\S'),
    CONSTRAINT locale_schedule_status_check CHECK (status IN ('scheduled', 'fired', 'cancelled')),
    CONSTRAINT locale_schedule_fired_at_check CHECK (status <> 'fired' OR fired_at IS NOT NULL),
    -- One slot per (language, timezone) (D-076). Also serves as the index on campaign_id (foreign key).
    CONSTRAINT locale_schedule_campaign_language_timezone_key UNIQUE (campaign_id, language, timezone),
    -- Target of delivery's composite foreign key: a delivery's schedule belongs to its campaign.
    CONSTRAINT locale_schedule_id_campaign_id_key UNIQUE (id, campaign_id)
);

CREATE TABLE message (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id uuid NOT NULL REFERENCES send_campaign (id),
    language    text NOT NULL,
    channel     text NOT NULL,
    content     jsonb NOT NULL,
    CONSTRAINT message_language_check CHECK (language IN ('tr', 'en')),
    CONSTRAINT message_channel_check CHECK (channel IN ('sms', 'email')),
    -- Also serves as the index on campaign_id (foreign key).
    CONSTRAINT message_campaign_language_channel_key UNIQUE (campaign_id, language, channel),
    -- Target of delivery's composite foreign key: a delivery's message belongs to its campaign
    -- and has the delivery's channel.
    CONSTRAINT message_id_campaign_id_channel_key UNIQUE (id, campaign_id, channel)
);

CREATE TABLE recipient_batch (
    campaign_id uuid NOT NULL REFERENCES send_campaign (id),
    batch_no    int NOT NULL,
    recipients  int NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT recipient_batch_batch_no_check CHECK (batch_no >= 1),
    CONSTRAINT recipient_batch_recipients_check CHECK (recipients >= 0),
    -- Also serves as the index on campaign_id (foreign key).
    CONSTRAINT recipient_batch_pkey PRIMARY KEY (campaign_id, batch_no)
);

-- +goose Down
DROP TABLE recipient_batch;
DROP TABLE message;
DROP TABLE locale_schedule;
DROP TABLE send_campaign;
