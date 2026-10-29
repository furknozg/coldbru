-- Optional NATS headers of a raw outbox message (D-078): a JSON object, header name to value. NULL
-- means none. The relay publishes them next to Nats-Msg-Id; Nats-Msg-Id itself cannot be set here.
-- +goose Up
ALTER TABLE outbox ADD COLUMN headers jsonb;

-- +goose Down
ALTER TABLE outbox DROP COLUMN headers;
