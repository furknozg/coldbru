-- The short link code of a delivery (D-106): Notification asks the redirect service for one link
-- per delivery while it writes the delivery rows of a recipient batch, and keeps the code here.
-- The sender builds the tracking link <TRACKING_BASE_URL>/r/<code> from it, so sending needs no
-- call to the redirect service. Nullable: deliveries written before this migration have no link
-- and are sent without one. The code is not personal data and survives ClearDeliveryPII.

-- +goose Up
ALTER TABLE delivery ADD COLUMN link_code text;

ALTER TABLE delivery ADD CONSTRAINT delivery_link_code_check
    CHECK (link_code IS NULL OR link_code ~ '^[0-9A-Za-z]{10}$');

-- +goose Down
ALTER TABLE delivery DROP CONSTRAINT delivery_link_code_check;

ALTER TABLE delivery DROP COLUMN link_code;
