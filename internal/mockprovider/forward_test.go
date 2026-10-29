package mockprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"
)

// smtpSink is a tiny SMTP server that keeps what it receives.
type smtpSink struct {
	ln       net.Listener
	mu       sync.Mutex
	from     []string
	rcpt     []string
	messages []string
	rejectTo string // RCPT TO for this address gets a 550
}

func newSMTPSink(t *testing.T) *smtpSink {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &smtpSink{ln: ln}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}

func (s *smtpSink) addr() string { return s.ln.Addr().String() }

func (s *smtpSink) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(line string) { _, _ = io.WriteString(c, line+"\r\n") }
	say("220 sink ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			say("250 sink")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			s.mu.Lock()
			s.from = append(s.from, line)
			s.mu.Unlock()
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if s.rejectTo != "" && strings.Contains(line, s.rejectTo) {
				say("550 no such user")
				continue
			}
			s.mu.Lock()
			s.rcpt = append(s.rcpt, line)
			s.mu.Unlock()
			say("250 ok")
		case cmd == "DATA":
			say("354 go ahead")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				l = strings.TrimPrefix(l, ".") // undo the dot-stuffing
				b.WriteString(l)
			}
			s.mu.Lock()
			s.messages = append(s.messages, b.String())
			s.mu.Unlock()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("500 what")
		}
	}
}

func TestInboxForwarderSendEmailOverSMTP(t *testing.T) {
	t.Parallel()
	sink := newSMTPSink(t)
	cfg := DefaultConfig()
	cfg.SMTPAddr = sink.addr()
	f := NewInboxForwarder(cfg)

	e := Email{
		From: "campaigns@coldbru.test", To: "ayse@example.com", Subject: "Sepetin seni bekliyor ✨",
		Text: "Merhaba\n.\nSon", HTML: "<p>Merhaba</p>", MessageID: "mid-1", IdempotencyKey: "key-1",
	}
	if err := f.SendEmail(t.Context(), e); err != nil {
		t.Fatalf("SendEmail() = %v", err)
	}
	if len(sink.messages) != 1 || len(sink.from) != 1 || len(sink.rcpt) != 1 {
		t.Fatalf("sink got %d messages, %d from, %d rcpt", len(sink.messages), len(sink.from), len(sink.rcpt))
	}
	if !strings.Contains(sink.from[0], "<campaigns@coldbru.test>") || !strings.Contains(sink.rcpt[0], "<ayse@example.com>") {
		t.Errorf("envelope = %q / %q", sink.from[0], sink.rcpt[0])
	}
	msg, err := mail.ReadMessage(strings.NewReader(sink.messages[0]))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Header.Get("X-Idempotency-Key") != "key-1" || msg.Header.Get("Date") == "" {
		t.Errorf("headers = %v", msg.Header)
	}
}

func TestInboxForwarderSMTPErrors(t *testing.T) {
	t.Parallel()
	sink := newSMTPSink(t)
	sink.rejectTo = "bad@example.com"
	cfg := DefaultConfig()
	cfg.SMTPAddr = sink.addr()
	e := Email{From: "campaigns@coldbru.test", To: "bad@example.com", Subject: "s", Text: "t", HTML: "h", MessageID: "m", IdempotencyKey: "k"}
	if err := NewInboxForwarder(cfg).SendEmail(t.Context(), e); err == nil || !strings.Contains(err.Error(), "RCPT") {
		t.Errorf("rejected recipient: err = %v", err)
	}

	// Nothing listens: a refused connection is an error, not a hang.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	cfg.SMTPAddr = addr
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := NewInboxForwarder(cfg).SendEmail(ctx, e); err == nil {
		t.Error("SendEmail() to a closed port succeeded")
	}
}

func TestInboxForwarderSendSMS(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var got SMS
	var gotPath, gotType string
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotPath, gotType = r.URL.Path, r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	cfg := DefaultConfig()
	cfg.MockServerURL = srv.URL + "/"
	f := NewInboxForwarder(cfg)

	want := SMS{To: "+905551112233", Text: "Merhaba Ayşe", MessageID: "mid", IdempotencyKey: "key"}
	if err := f.SendSMS(t.Context(), want); err != nil {
		t.Fatalf("SendSMS() = %v", err)
	}
	mu.Lock()
	if got != want || gotPath != "/sms" || gotType != "application/json" {
		t.Errorf("MockServer got %+v on %q (%q)", got, gotPath, gotType)
	}
	status = http.StatusNotFound
	mu.Unlock()
	if err := f.SendSMS(t.Context(), want); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a 404 from MockServer: err = %v", err)
	}
}
