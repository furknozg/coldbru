package mockprovider

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

// Email is one message to be handed to the inbox.
type Email struct {
	From, To, Subject, HTML, Text string
	// MessageID is the id the mock gave the message; it becomes the Message-ID header.
	MessageID string
	// IdempotencyKey is carried in the X-Idempotency-Key header, so the inbox shows which delivery
	// a message belongs to.
	IdempotencyKey string
	Date           time.Time
}

// buildMIME renders e as a multipart/alternative message: a text part and then an HTML part (the
// last part is the preferred one), both UTF-8 and quoted-printable, with an RFC 2047 encoded subject.
func buildMIME(e Email) ([]byte, error) {
	from, err := mail.ParseAddress(e.From)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	hdr("From", e.From)
	hdr("To", e.To)
	hdr("Subject", mime.QEncoding.Encode("utf-8", e.Subject))
	hdr("Date", e.Date.Format(time.RFC1123Z))
	hdr("Message-ID", "<"+e.MessageID+"@"+domain+">")
	hdr("X-Idempotency-Key", e.IdempotencyKey)
	hdr("MIME-Version", "1.0")
	hdr("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	buf.WriteString("\r\n")

	for _, part := range []struct{ ctype, body string }{
		{"text/plain; charset=UTF-8", e.Text},
		{"text/html; charset=UTF-8", e.HTML},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(pw)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
