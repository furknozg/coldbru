-- Proposal core tables (D-022, D-033, D-034, D-068). IDs are UUIDv7 (D-030). Status and kind
-- columns are text + CHECK, not ENUM types. Foreign keys are plain: rows are never deleted.
-- Tables land in the proposal schema through the role's search_path (D-048).

-- +goose Up
CREATE TABLE campaign (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    brief      text NOT NULL,
    status     text NOT NULL DEFAULT 'drafting',
    created_at timestamptz NOT NULL DEFAULT now(),
    -- Set by the application on every change; there is no trigger (D-068).
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT campaign_brief_check CHECK (brief ~ '\S'),
    CONSTRAINT campaign_status_check CHECK (status IN ('drafting', 'scheduled', 'sending', 'completed', 'cancelled'))
);

CREATE TABLE proposal_step (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id uuid NOT NULL REFERENCES campaign (id),
    kind        text NOT NULL,
    version     int NOT NULL,
    status      text NOT NULL DEFAULT 'open',
    created_at  timestamptz NOT NULL DEFAULT now(),
    approved_at timestamptz,
    CONSTRAINT proposal_step_kind_check CHECK (kind IN ('audience', 'channel_plan', 'content', 'schedule')),
    CONSTRAINT proposal_step_version_check CHECK (version >= 1),
    CONSTRAINT proposal_step_status_check CHECK (status IN ('open', 'approved', 'superseded')),
    -- A superseded step keeps whatever stamp it had (history); open and approved are strict.
    CONSTRAINT proposal_step_approved_at_set_check CHECK (status <> 'approved' OR approved_at IS NOT NULL),
    CONSTRAINT proposal_step_open_approved_at_check CHECK (status <> 'open' OR approved_at IS NULL),
    -- Also serves as the index on campaign_id (foreign key).
    CONSTRAINT proposal_step_campaign_kind_version_key UNIQUE (campaign_id, kind, version),
    -- Target of proposal_variant's composite foreign key: a variant's kind equals its step's kind.
    CONSTRAINT proposal_step_id_kind_key UNIQUE (id, kind)
);

-- At most one live (not superseded) step per kind in a campaign.
CREATE UNIQUE INDEX proposal_step_live_kind_key ON proposal_step (campaign_id, kind) WHERE status <> 'superseded';

CREATE TABLE proposal_variant (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    step_id          uuid NOT NULL,
    kind             text NOT NULL,
    language         text,
    channel          text,
    status           text NOT NULL DEFAULT 'draft',
    content          jsonb NOT NULL,
    rejection_reason text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    approved_at      timestamptz,
    rejected_at      timestamptz,
    CONSTRAINT proposal_variant_step_kind_fkey FOREIGN KEY (step_id, kind) REFERENCES proposal_step (id, kind),
    CONSTRAINT proposal_variant_kind_check CHECK (kind IN ('audience', 'channel_plan', 'content', 'schedule')),
    CONSTRAINT proposal_variant_language_check CHECK (language IN ('tr', 'en')),
    CONSTRAINT proposal_variant_channel_check CHECK (channel IN ('sms', 'email')),
    CONSTRAINT proposal_variant_status_check CHECK (status IN ('draft', 'approved', 'rejected', 'superseded')),
    -- Variant shape per step kind (D-034).
    CONSTRAINT proposal_variant_shape_check CHECK (
        (kind IN ('audience', 'channel_plan') AND language IS NULL AND channel IS NULL)
        OR (kind = 'content' AND language IS NOT NULL AND channel IS NOT NULL)
        OR (kind = 'schedule' AND language IS NOT NULL AND channel IS NULL)
    ),
    -- A superseded variant keeps its earlier stamps and reason (history); the other states are strict.
    CONSTRAINT proposal_variant_approved_at_check CHECK (status <> 'approved' OR approved_at IS NOT NULL),
    CONSTRAINT proposal_variant_draft_check CHECK (
        status <> 'draft' OR (approved_at IS NULL AND rejected_at IS NULL AND rejection_reason IS NULL)
    ),
    CONSTRAINT proposal_variant_rejected_check CHECK (
        status <> 'rejected' OR (rejected_at IS NOT NULL AND rejection_reason IS NOT NULL AND rejection_reason ~ '\S')
    )
);

-- One draft or approved variant per (language, channel) slot in a step. NULLS NOT DISTINCT makes
-- the slot of audience and channel_plan steps (both NULL) unique too.
CREATE UNIQUE INDEX proposal_variant_live_slot_key ON proposal_variant (step_id, language, channel) NULLS NOT DISTINCT
    WHERE status IN ('draft', 'approved');

CREATE INDEX proposal_variant_step_id_idx ON proposal_variant (step_id);

-- +goose Down
DROP INDEX proposal_variant_step_id_idx;
DROP INDEX proposal_variant_live_slot_key;
DROP TABLE proposal_variant;
DROP INDEX proposal_step_live_kind_key;
DROP TABLE proposal_step;
DROP TABLE campaign;
