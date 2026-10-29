// Package outbox implements the transactional outbox (D-006, D-054): services
// write events into the outbox table inside their business transaction, and a
// Relay publishes them to NATS JetStream.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

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

// EnvelopeSubjectPrefix is the subject namespace of event envelopes (D-035). The Relay tells the
// two kinds of outbox rows apart by it, so no table column is needed: a row whose subject starts
// with it is an envelope and gets its id added when it is published; every other row is a raw
// message and is published as stored.
const EnvelopeSubjectPrefix = "evt."

// MaxSubjectLength bounds a raw message subject; NATS allows more, a service needs far less.
const MaxSubjectLength = 255

// MessageOption customises WriteMessage.
type MessageOption func(*messageOptions)

type messageOptions struct {
	headers map[string]string
}

// WithHeaders sets NATS headers the Relay publishes with the message (D-078), for example the
// Nats-Schedule headers of a scheduled message. Calling it again adds to the headers already set;
// a name given twice keeps the last value. Names and values are checked by ValidateHeaders and
// Nats-Msg-Id is rejected: the Relay sets it from the row id. No headers (or an empty map) stores
// NULL, which publishes exactly as before.
func WithHeaders(headers map[string]string) MessageOption {
	return func(o *messageOptions) {
		if len(headers) == 0 {
			return
		}
		if o.headers == nil {
			o.headers = make(map[string]string, len(headers))
		}
		for k, v := range headers {
			o.headers[k] = v
		}
	}
}

// WriteMessage stores a raw, non-envelope message in the outbox through tx and returns the outbox
// row id (D-074). The Relay publishes payload to subject unchanged in meaning and sends the row id
// as Nats-Msg-Id, so a re-publish after a crash is dropped by JetStream's dedup window. Like Write,
// the message becomes visible to the Relay only when the caller's transaction commits.
//
// subject must be a plain NATS subject: dot-separated tokens of letters, digits, '_' and '-', no
// wildcards, no empty token, at most MaxSubjectLength bytes, and outside the "evt." namespace, which
// belongs to Write. Whether a JetStream stream captures the subject is not checked here: a subject
// without a stream makes the Relay's publish fail until a stream exists, and rows behind it wait.
//
// payload must be valid JSON. The outbox column is jsonb, so the Relay publishes the semantically
// equal normalized form: whitespace removed, object keys reordered, duplicate keys collapsed.
// Messages are not ordered between rows of one transaction beyond the outbox's created order.
//
// Options add NATS headers (WithHeaders, D-078).
func WriteMessage(ctx context.Context, tx pgx.Tx, subject string, payload []byte, opts ...MessageOption) (string, error) {
	if err := ValidateRawSubject(subject); err != nil {
		return "", fmt.Errorf("outbox subject: %w", err)
	}
	if !json.Valid(payload) {
		return "", errors.New("outbox payload: not valid JSON")
	}
	var o messageOptions
	for _, opt := range opts {
		opt(&o)
	}
	if err := ValidateHeaders(o.headers); err != nil {
		return "", fmt.Errorf("outbox headers: %w", err)
	}
	var headers []byte // nil stores NULL
	if len(o.headers) > 0 {
		var err error
		if headers, err = json.Marshal(o.headers); err != nil {
			return "", fmt.Errorf("marshal outbox headers: %w", err)
		}
	}
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO outbox (subject, payload, headers) VALUES ($1, $2, $3) RETURNING id::text`,
		subject, payload, headers).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert outbox row: %w", err)
	}
	return id, nil
}

// MsgIDHeader is the header the Relay sets from the outbox row id; WriteMessage refuses to set it.
const MsgIDHeader = "Nats-Msg-Id"

// ValidateHeaders checks the headers of WriteMessage: a name must not be empty or contain ':' or
// whitespace or control characters, a value must not contain a line break (it would end the
// header on the wire), and MsgIDHeader (any letter case) may not be set. nil is valid.
func ValidateHeaders(headers map[string]string) error {
	for name, value := range headers {
		switch {
		case name == "":
			return errors.New("header name is empty")
		case strings.EqualFold(name, MsgIDHeader):
			return fmt.Errorf("header %q is set by the relay from the row id", name)
		}
		for _, r := range name {
			if r == ':' || unicode.IsSpace(r) || unicode.IsControl(r) {
				return fmt.Errorf("header name %q has %q; ':', whitespace and control characters are not allowed", name, r)
			}
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("header %q: value has a line break", name)
		}
	}
	return nil
}

// ValidateRawSubject checks a subject for WriteMessage.
func ValidateRawSubject(subject string) error {
	switch {
	case subject == "":
		return errors.New("subject is empty")
	case len(subject) > MaxSubjectLength:
		return fmt.Errorf("subject is longer than %d bytes", MaxSubjectLength)
	case strings.HasPrefix(subject, EnvelopeSubjectPrefix):
		return fmt.Errorf("subject %q is in the %q namespace of event envelopes; use Write", subject, EnvelopeSubjectPrefix)
	}
	for _, token := range strings.Split(subject, ".") {
		if token == "" {
			return fmt.Errorf("subject %q has an empty token", subject)
		}
		for _, r := range token {
			ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-'
			if !ok {
				return fmt.Errorf("subject %q: token %q has %q; allowed are letters, digits, '_' and '-' (no wildcards or spaces)", subject, token, r)
			}
		}
	}
	return nil
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
