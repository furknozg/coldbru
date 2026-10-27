-- +goose Up
CREATE TABLE processed_event (
    event_id     uuid PRIMARY KEY,
    processed_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_event;
