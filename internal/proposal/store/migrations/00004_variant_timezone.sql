-- Schedule slots are (language, timezone) pairs (D-076, which replaces the schedule slot of
-- D-034): proposal_variant gains the IANA timezone of the slot. It is set on schedule variants
-- and NULL on every other kind, and it joins the live slot unique index so one language can hold
-- one schedule per time zone.

-- +goose Up
ALTER TABLE proposal_variant ADD COLUMN timezone text;

-- Schedule rows written before this migration (development databases) have no time zone. Fill them
-- with the language mapping of D-070 that D-076 retires: tr -> Europe/Istanbul, en -> Europe/London.
UPDATE proposal_variant SET timezone = CASE language WHEN 'tr' THEN 'Europe/Istanbul' ELSE 'Europe/London' END
WHERE kind = 'schedule' AND timezone IS NULL;

ALTER TABLE proposal_variant DROP CONSTRAINT proposal_variant_shape_check;
ALTER TABLE proposal_variant ADD CONSTRAINT proposal_variant_shape_check CHECK (
    (kind IN ('audience', 'channel_plan') AND language IS NULL AND channel IS NULL AND timezone IS NULL)
    OR (kind = 'content' AND language IS NOT NULL AND channel IS NOT NULL AND timezone IS NULL)
    OR (kind = 'schedule' AND language IS NOT NULL AND channel IS NULL AND timezone IS NOT NULL)
);
ALTER TABLE proposal_variant ADD CONSTRAINT proposal_variant_timezone_check CHECK (timezone ~ '\S');

DROP INDEX proposal_variant_live_slot_key;
CREATE UNIQUE INDEX proposal_variant_live_slot_key ON proposal_variant (step_id, language, channel, timezone) NULLS NOT DISTINCT
    WHERE status IN ('draft', 'approved');

-- +goose Down
-- Fails (and rolls back) when one step holds live schedule variants of the same language in
-- different time zones: the old slot index cannot represent them. Existing time zones are dropped.
DROP INDEX proposal_variant_live_slot_key;
ALTER TABLE proposal_variant DROP CONSTRAINT proposal_variant_timezone_check;
ALTER TABLE proposal_variant DROP CONSTRAINT proposal_variant_shape_check;
ALTER TABLE proposal_variant DROP COLUMN timezone;
ALTER TABLE proposal_variant ADD CONSTRAINT proposal_variant_shape_check CHECK (
    (kind IN ('audience', 'channel_plan') AND language IS NULL AND channel IS NULL)
    OR (kind = 'content' AND language IS NOT NULL AND channel IS NOT NULL)
    OR (kind = 'schedule' AND language IS NOT NULL AND channel IS NULL)
);
CREATE UNIQUE INDEX proposal_variant_live_slot_key ON proposal_variant (step_id, language, channel) NULLS NOT DISTINCT
    WHERE status IN ('draft', 'approved');
