-- +goose Up
CREATE TABLE outbox (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    subject      text NOT NULL,
    payload      jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);

CREATE INDEX outbox_unpublished_idx ON outbox (created_at) WHERE published_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION outbox_notify() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_notify('outbox', '');
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER outbox_notify
    AFTER INSERT ON outbox
    FOR EACH STATEMENT EXECUTE FUNCTION outbox_notify();

-- +goose Down
DROP TRIGGER outbox_notify ON outbox;
DROP FUNCTION outbox_notify();
DROP TABLE outbox;
