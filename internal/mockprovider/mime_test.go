package mockprovider

import (
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestBuildMIME(t *testing.T) {
	t.Parallel()
	e := Email{
		From: "campaigns@coldbru.test", To: "ayse@example.com",
		Subject:   "Çağrı: sepetinde ürün kaldı, %30 indirim seni bekliyor ✨",
		Text:      "Merhaba Ayşe,\nsepetin seni bekliyor. İyi alışverişler!\n.\nSon satır",
		HTML:      "<p>Merhaba <b>Ayşe</b>, ğüşiöç İĞÜŞÖÇ</p>",
		MessageID: "019a0e6e-0000-7000-8000-000000000001", IdempotencyKey: "delivery-1",
		Date: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
	}
	raw, err := buildMIME(e)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.SplitN(string(raw), "\r\n\r\n", 2)[0], "Çağrı") {
		t.Error("the subject header carries raw non-ASCII, want RFC 2047 words")
	}

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("not a valid message: %v", err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || subject != e.Subject {
		t.Errorf("subject = %q (%v), want %q", subject, err, e.Subject)
	}
	if !strings.Contains(msg.Header.Get("Subject"), "=?utf-8?q?") {
		t.Errorf("subject header = %q, want RFC 2047 encoded words", msg.Header.Get("Subject"))
	}
	for k, want := range map[string]string{
		"From": e.From, "To": e.To, "X-Idempotency-Key": "delivery-1",
		"Message-Id": "<019a0e6e-0000-7000-8000-000000000001@coldbru.test>", "Mime-Version": "1.0",
	} {
		if got := msg.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if d, err := msg.Header.Date(); err != nil || !d.Equal(e.Date) {
		t.Errorf("Date = %v (%v), want %v", d, err, e.Date)
	}

	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content type = %q (%v)", mt, err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types, bodies []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		// multipart.Reader decodes quoted-printable by itself and drops the header.
		b, err := io.ReadAll(p)
		if err != nil {
			t.Fatal(err)
		}
		types = append(types, p.Header.Get("Content-Type"))
		bodies = append(bodies, strings.ReplaceAll(string(b), "\r\n", "\n"))
	}
	if n := strings.Count(string(raw), "Content-Transfer-Encoding: quoted-printable"); n != 2 {
		t.Errorf("%d quoted-printable parts, want 2", n)
	}
	if len(types) != 2 || types[0] != "text/plain; charset=UTF-8" || types[1] != "text/html; charset=UTF-8" {
		t.Fatalf("parts = %v, want text then html, both UTF-8", types)
	}
	if bodies[0] != e.Text || bodies[1] != e.HTML {
		t.Errorf("bodies = %q, want %q and %q", bodies, e.Text, e.HTML)
	}
}

func TestBuildMIMELongSubjectStaysWithinLineLimits(t *testing.T) {
	t.Parallel()
	e := Email{
		From: "campaigns@coldbru.test", To: "a@example.com",
		Subject: strings.Repeat("Şimdi sepetini tamamla ", 4), Text: "t", HTML: "h",
		MessageID: "id", IdempotencyKey: "k", Date: time.Now(),
	}
	raw, err := buildMIME(e)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.SplitN(string(raw), "\r\n\r\n", 2)[0]
	for _, line := range strings.Split(head, "\r\n") {
		if len(line) > 998 {
			t.Errorf("header line of %d bytes", len(line))
		}
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); got != e.Subject {
		t.Errorf("subject = %q, want %q", got, e.Subject)
	}
}

func TestBuildMIMEBadFrom(t *testing.T) {
	t.Parallel()
	if _, err := buildMIME(Email{From: "nope"}); err == nil {
		t.Error("buildMIME() accepted a bad From")
	}
}
