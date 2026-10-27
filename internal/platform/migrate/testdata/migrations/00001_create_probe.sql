-- +goose Up
CREATE TABLE probe (
    id   uuid PRIMARY KEY DEFAULT uuidv7(),
    note text NOT NULL
);

-- +goose Down
DROP TABLE probe;
