package mockprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// forwardTimeout bounds one forward to the inbox.
const forwardTimeout = 5 * time.Second

// SMS is an SMS handed to the inbox.
type SMS struct {
	To             string `json:"to"`
	Text           string `json:"text"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// Forwarder hands an accepted message to the inbox where a person can see it: Mailpit for email,
// MockServer for SMS. A returned error means the message was not delivered to the inbox.
type Forwarder interface {
	SendEmail(ctx context.Context, e Email) error
	SendSMS(ctx context.Context, s SMS) error
}

// InboxForwarder is the Forwarder of the compose stack: email over SMTP to Mailpit, SMS over HTTP to
// MockServer. It is safe for concurrent use.
type InboxForwarder struct {
	smtpAddr      string
	mockServerURL string
	http          *http.Client
	now           func() time.Time
}

// NewInboxForwarder returns a forwarder for the configured Mailpit and MockServer.
func NewInboxForwarder(cfg Config) *InboxForwarder {
	return &InboxForwarder{
		smtpAddr:      cfg.SMTPAddr,
		mockServerURL: strings.TrimRight(cfg.MockServerURL, "/"),
		http:          &http.Client{},
		now:           time.Now,
	}
}

// SendEmail builds the MIME message and sends it with net/smtp, without authentication and without
// TLS: the inbox is a development tool on the compose network.
func (f *InboxForwarder) SendEmail(ctx context.Context, e Email) error {
	if e.Date.IsZero() {
		e.Date = f.now()
	}
	msg, err := buildMIME(e)
	if err != nil {
		return fmt.Errorf("build message: %w", err)
	}
	from, err := mail.ParseAddress(e.From)
	if err != nil {
		return fmt.Errorf("from: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, forwardTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", f.smtpAddr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	host, _, _ := net.SplitHostPort(f.smtpAddr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer c.Close()
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(e.To); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp end of data: %w", err)
	}
	// The message is accepted here; a failed QUIT changes nothing.
	_ = c.Quit()
	return nil
}

// SendSMS posts the message as JSON to MockServer's /sms.
func (f *InboxForwarder) SendSMS(ctx context.Context, s SMS) error {
	body, err := json.Marshal(s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, forwardTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.mockServerURL+"/sms", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := f.http.Do(req)
	if err != nil {
		return fmt.Errorf("post sms: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<16))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("mockserver answered %d", res.StatusCode)
	}
	return nil
}
