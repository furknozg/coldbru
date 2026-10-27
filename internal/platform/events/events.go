// Package events defines the JSON envelope every service-to-service event
// travels in and the NATS subject it is published on (D-035).
package events

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Envelope is the wire format of one event. ID is a UUIDv7 assigned by the
// outbox at write time (D-030); Data holds the event specific payload.
type Envelope struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Producer      string          `json:"producer"`
	CorrelationID string          `json:"correlation_id"`
	Data          json.RawMessage `json:"data"`
}

var namePart = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Subject returns the NATS subject of e: evt.<service>.<event>.v<version>.
//
// Type must be "<service>.<event>" with both parts lowercase
// [a-z][a-z0-9_]* and exactly one dot; Version must be at least 1.
func Subject(e Envelope) (string, error) {
	service, event, ok := strings.Cut(e.Type, ".")
	if !ok {
		return "", fmt.Errorf("event type %q: want <service>.<event>", e.Type)
	}
	if strings.Contains(event, ".") {
		return "", fmt.Errorf("event type %q: want exactly one dot", e.Type)
	}
	if !namePart.MatchString(service) {
		return "", fmt.Errorf("event type %q: service %q must match %s", e.Type, service, namePart)
	}
	if !namePart.MatchString(event) {
		return "", fmt.Errorf("event type %q: event %q must match %s", e.Type, event, namePart)
	}
	if e.Version < 1 {
		return "", fmt.Errorf("event type %q: version %d must be at least 1", e.Type, e.Version)
	}
	return fmt.Sprintf("evt.%s.%s.v%d", service, event, e.Version), nil
}

// New builds an envelope for typ and version with data marshalled to JSON.
// OccurredAt is the current UTC time. ID stays empty: the outbox assigns it.
func New(typ string, version int, producer, correlationID string, data any) (Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal %s data: %w", typ, err)
	}
	return Envelope{
		Type:          typ,
		Version:       version,
		OccurredAt:    time.Now().UTC(),
		Producer:      producer,
		CorrelationID: correlationID,
		Data:          raw,
	}, nil
}
