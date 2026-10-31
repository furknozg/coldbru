-- Redirect service tables (D-106, D-107): the short links Notification asks for, one per delivery,
-- and one row per click. link.code is the public identifier (10 base62 characters, the last four an
-- HMAC tag, package code); delivery_id is Notification's delivery id and the idempotency key of
-- POST /links. channel, language and timezone are copied from the request so a click event can be
-- written without a call back to Notification. No IP address and no User-Agent are stored (D-107).
-- Status-like columns are text + CHECK, not ENUM types. Tables land in the redirect schema through
-- the role's search_path (D-048).

-- +goose Up
CREATE TABLE link (
    code        text PRIMARY KEY,
    delivery_id uuid NOT NULL,
    campaign_id uuid NOT NULL,
    channel     text NOT NULL,
    language    text NOT NULL,
    timezone    text NOT NULL,
    target_url  text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT link_code_check CHECK (code ~ '^[0-9A-Za-z]{10}$'),
    CONSTRAINT link_channel_check CHECK (channel IN ('sms', 'email')),
    CONSTRAINT link_language_check CHECK (language IN ('tr', 'en')),
    CONSTRAINT link_timezone_check CHECK (timezone ~ '\S'),
    CONSTRAINT link_target_url_check CHECK (target_url ~ '^https?://\S+$'),
    -- One link per delivery: makes POST /links idempotent and serves the lookup by delivery.
    CONSTRAINT link_delivery_id_key UNIQUE (delivery_id)
);

CREATE TABLE click (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    code       text NOT NULL REFERENCES link (code),
    clicked_at timestamptz NOT NULL DEFAULT now(),
    -- True for the first click of the delivery (its link); the others only add to the total.
    first      boolean NOT NULL
);

-- Foreign key index; also what the first-click check reads.
CREATE INDEX click_code_idx ON click (code);

-- +goose Down
DROP INDEX click_code_idx;
DROP TABLE click;
DROP TABLE link;
