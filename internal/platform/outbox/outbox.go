// Package outbox implements the transactional outbox (D-006, D-054): services
// write events into the outbox table inside their business transaction, and a
// Relay publishes them to NATS JetStream.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/furknozg/coldbru/internal/platform/events"
)

// Write stores e in the outbox through tx and returns the event id assigned by
// Postgres (UUIDv7, D-030). The row becomes visible to the Relay only when the
// caller's transaction commits; a rollback discards the event with the
// business change.
//
// The stored payload is the envelope without id. The Relay adds the id when it
// publishes, so the id has a single source: the outbox row.
func Write(ctx context.Context, tx pgx.Tx, e events.Envelope) (string, error) {
	subject, err := events.Subject(e)
	if err != nil {
		return "", fmt.Errorf("outbox subject: %w", err)
	}
	e.ID = ""
	payload, err := marshalWithoutID(e)
	if err != nil {
		return "", fmt.Errorf("marshal outbox payload: %w", err)
	}

	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO outbox (subject, payload) VALUES ($1, $2) RETURNING id::text`,
		subject, payload).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert outbox row: %w", err)
	}
	return id, nil
}

// marshalWithoutID encodes e as JSON with the id field left out.
func marshalWithoutID(e events.Envelope) ([]byte, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, "id")
	return json.Marshal(m)
}
