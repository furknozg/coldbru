-- +goose Up
ALTER TABLE probe ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE probe DROP COLUMN created_at;
