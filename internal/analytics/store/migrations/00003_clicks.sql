-- Click metrics (D-108): the denominators of the click-through rate, the click counters and the click
-- events behind the time series. Channel, language and timezone are those of the delivery's send slot.
-- Channel and language columns are text + CHECK, not ENUM types. There is no foreign key to
-- campaign_summary: clicks arrive while the campaign is still sending, before its completed event.
-- Tables land in the analytics schema through the role's search_path (D-048).

-- +goose Up
-- Sent deliveries per (campaign, channel, language, timezone), from the completed event
-- (sent_by_locale). A repeated completed event replaces the campaign's rows.
CREATE TABLE campaign_sent_locale (
    campaign_id uuid NOT NULL,
    channel     text NOT NULL,
    language    text NOT NULL,
    timezone    text NOT NULL,
    sent        int NOT NULL,
    CONSTRAINT campaign_sent_locale_pkey PRIMARY KEY (campaign_id, channel, language, timezone),
    CONSTRAINT campaign_sent_locale_channel_check CHECK (channel IN ('sms', 'email')),
    CONSTRAINT campaign_sent_locale_language_check CHECK (language IN ('tr', 'en')),
    CONSTRAINT campaign_sent_locale_timezone_check CHECK (timezone ~ '\S'),
    CONSTRAINT campaign_sent_locale_sent_check CHECK (sent >= 1)
);

-- Clicks per (campaign, channel, language, timezone): unique_clicks counts deliveries that were
-- clicked at least once (the clicks with first = true), total_clicks every click. Bumped in the
-- transaction that inserts the click_event, so the counters and the events agree.
CREATE TABLE campaign_click (
    campaign_id   uuid NOT NULL,
    channel       text NOT NULL,
    language      text NOT NULL,
    timezone      text NOT NULL,
    unique_clicks int NOT NULL,
    total_clicks  int NOT NULL,
    CONSTRAINT campaign_click_pkey PRIMARY KEY (campaign_id, channel, language, timezone),
    CONSTRAINT campaign_click_channel_check CHECK (channel IN ('sms', 'email')),
    CONSTRAINT campaign_click_language_check CHECK (language IN ('tr', 'en')),
    CONSTRAINT campaign_click_timezone_check CHECK (timezone ~ '\S'),
    CONSTRAINT campaign_click_unique_clicks_check CHECK (unique_clicks >= 0),
    CONSTRAINT campaign_click_total_clicks_check CHECK (total_clicks >= unique_clicks)
);

-- One row per click, keyed by Redirect's click id: a redelivered event inserts nothing and so
-- bumps no counter. The source of the click time series.
CREATE TABLE click_event (
    click_id    uuid PRIMARY KEY,
    campaign_id uuid NOT NULL,
    channel     text NOT NULL,
    language    text NOT NULL,
    timezone    text NOT NULL,
    clicked_at  timestamptz NOT NULL,
    first       boolean NOT NULL,
    CONSTRAINT click_event_channel_check CHECK (channel IN ('sms', 'email')),
    CONSTRAINT click_event_language_check CHECK (language IN ('tr', 'en')),
    CONSTRAINT click_event_timezone_check CHECK (timezone ~ '\S')
);

-- The click series reads a range of click times and groups by date_trunc, like the other series (D-097).
CREATE INDEX click_event_clicked_at_idx ON click_event (clicked_at);

-- +goose Down
DROP INDEX click_event_clicked_at_idx;
DROP TABLE click_event;
DROP TABLE campaign_click;
DROP TABLE campaign_sent_locale;
