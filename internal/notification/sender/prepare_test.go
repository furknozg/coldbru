package sender

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/notification/provider"
	"github.com/furknozg/coldbru/internal/notification/store"
)

const (
	trackBase = "http://localhost:8080"
	trackCode = "k3JdQ9aB7x"
	trackLink = "http://localhost:8080/r/k3JdQ9aB7x"
)

// capture is a Provider that keeps what it was asked to send.
type capture struct {
	sms   []provider.SMS
	email []provider.Email
}

func (c *capture) SendSMS(_ context.Context, _ string, m provider.SMS) (provider.Result, error) {
	c.sms = append(c.sms, m)
	return provider.Result{MessageID: "m"}, nil
}

func (c *capture) SendEmail(_ context.Context, _ string, m provider.Email) (provider.Result, error) {
	c.email = append(c.email, m)
	return provider.Result{MessageID: "m"}, nil
}

type logBuf struct{ strings.Builder }

func (b *logBuf) logger() *slog.Logger { return slog.New(slog.NewTextHandler(b, nil)) }

func prepWorker(base string) (*worker, *capture) {
	c := &capture{}
	return &worker{provider: c, trackingBase: base}, c
}

func smsRow(t *testing.T, text, code string) store.ListSendDeliveriesRow {
	t.Helper()
	raw, err := json.Marshal(content.SMSContent{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	row := store.ListSendDeliveriesRow{ID: uuid.New(), CampaignID: uuid.New(), Channel: "sms", Language: "tr", Content: raw, CtaUrl: "https://shop.example.com"}
	if code != "" {
		row.LinkCode = pgtype.Text{String: code, Valid: true}
	}
	return row
}

func sendSMS(t *testing.T, w *worker, row store.ListSendDeliveriesRow, first string) (string, string) {
	t.Helper()
	var logs logBuf
	call, err := w.prepare(logs.logger(), row, first, "+905550000000")
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	if _, err := call(t.Context()); err != nil {
		t.Fatal(err)
	}
	return w.provider.(*capture).sms[0].Text, logs.String()
}

func TestSMSIsTheTextASpaceAndTheLink(t *testing.T) {
	t.Parallel()
	w, _ := prepWorker(trackBase)
	got, _ := sendSMS(t, w, smsRow(t, "Merhaba {{first_name}}, sepetin seni bekliyor.", trackCode), "Ayşe")
	if want := "Merhaba Ayşe, sepetin seni bekliyor. " + trackLink; got != want {
		t.Errorf("sms = %q, want %q", got, want)
	}
}

func TestSMSTextAtTheBudgetFitsTheLinkIn160(t *testing.T) {
	t.Parallel()
	budget := content.SMSBudget(trackBase)
	w, _ := prepWorker(trackBase)
	got, logs := sendSMS(t, w, smsRow(t, strings.Repeat("a", budget), trackCode), "")
	if n := runes(got); n != content.SMSMaxChars || !strings.HasSuffix(got, " "+trackLink) {
		t.Errorf("sms has %d characters, suffix ok = %v; want 160 ending in the link", n, strings.HasSuffix(got, " "+trackLink))
	}
	if strings.Contains(logs, "no room") {
		t.Errorf("a text at the budget lost its link: %s", logs)
	}
}

func TestSMSNameIsShortenedToTheBudgetNotTo160(t *testing.T) {
	t.Parallel()
	budget := content.SMSBudget(trackBase)
	w, _ := prepWorker(trackBase)
	// The text without the name takes budget-4 characters, so only 4 characters of the name fit.
	tpl := "{{first_name}} " + strings.Repeat("a", budget-5)
	got, _ := sendSMS(t, w, smsRow(t, tpl, trackCode), "Abdurrahman")
	if !strings.HasPrefix(got, "Abdu ") || runes(got) != content.SMSMaxChars || !strings.HasSuffix(got, " "+trackLink) {
		t.Errorf("sms = %q (%d characters); want the name cut to 4 characters and 160 in all", got, runes(got))
	}
}

func TestSMSTextOverTheBudgetIsSentWithoutTheLink(t *testing.T) {
	t.Parallel()
	budget := content.SMSBudget(trackBase)
	w, _ := prepWorker(trackBase)
	text := strings.Repeat("a", budget+10) // approved under the old 160 limit
	got, logs := sendSMS(t, w, smsRow(t, text, trackCode), "")
	if got != text {
		t.Errorf("sms = %q, want the text alone", got)
	}
	if !strings.Contains(logs, "leaves no room for the tracking link") {
		t.Errorf("the dropped link was not logged: %s", logs)
	}
}

func TestSMSWithoutACodeOrABaseIsSentAsBefore(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("a", content.SMSMaxChars)
	for name, tt := range map[string]struct{ base, code string }{"no code": {trackBase, ""}, "no base url": {"", trackCode}} {
		t.Run(name, func(t *testing.T) {
			w, _ := prepWorker(tt.base)
			if got, _ := sendSMS(t, w, smsRow(t, text, tt.code), ""); got != text {
				t.Errorf("sms = %q, want the 160 character text unchanged", got)
			}
		})
	}
}

func TestSMSTextThatCannotFitEvenWithoutALinkFails(t *testing.T) {
	t.Parallel()
	w, _ := prepWorker(trackBase)
	var logs logBuf
	if _, err := w.prepare(logs.logger(), smsRow(t, strings.Repeat("a", 161), trackCode), "", "+905550000000"); err == nil {
		t.Error("prepare() accepted a 161 character text")
	}
}

func TestSMSIsMeasuredOnTheFinalText(t *testing.T) {
	t.Parallel()
	w, _ := prepWorker(trackBase)
	// 40 characters with a Turkish ş: UCS-2. Alone it is one segment (70 units); with the 34
	// character link and a space it is 75 units and two segments.
	text := "Şimdi sepete dön, " + strings.Repeat("b", 22)
	got, _ := sendSMS(t, w, smsRow(t, text, trackCode), "")
	alone, final := content.MeasureSMS(text), content.MeasureSMS(got)
	if alone.Encoding != content.UCS2 || alone.Segments != 1 || final.Encoding != content.UCS2 || final.Segments != 2 || final.Length != runes(text)+35 {
		t.Errorf("text alone %+v, final %+v: the segments must be those of the text with its link", alone, final)
	}
}

func emailRow(t *testing.T, code string) store.ListSendDeliveriesRow {
	t.Helper()
	raw, err := json.Marshal(content.EmailContent{
		Subject: "Hi {{first_name}}", Preheader: "p", Headline: "h", Paragraphs: []string{"body"}, CTALabel: "Go",
	})
	if err != nil {
		t.Fatal(err)
	}
	row := store.ListSendDeliveriesRow{ID: uuid.New(), CampaignID: uuid.New(), Channel: "email", Language: "en", Content: raw, CtaUrl: "https://shop.example.com/?utm_campaign=c"}
	if code != "" {
		row.LinkCode = pgtype.Text{String: code, Valid: true}
	}
	return row
}

func TestEmailButtonPointsAtTheTrackingLink(t *testing.T) {
	t.Parallel()
	w, c := prepWorker(trackBase)
	call, err := w.prepare(slog.New(slog.NewTextHandler(io.Discard, nil)), emailRow(t, trackCode), "Ada", "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := call(t.Context()); err != nil {
		t.Fatal(err)
	}
	m := c.email[0]
	if !strings.Contains(m.HTML, `href="`+trackLink+`"`) || !strings.Contains(m.Text, "Go: "+trackLink) || strings.Contains(m.HTML, "shop.example.com") {
		t.Errorf("email links: html %q, text %q; want only the tracking link", m.HTML, m.Text)
	}
}

func TestEmailWithoutACodeKeepsTheCampaignLink(t *testing.T) {
	t.Parallel()
	w, c := prepWorker(trackBase)
	row := emailRow(t, "")
	call, err := w.prepare(slog.New(slog.NewTextHandler(io.Discard, nil)), row, "Ada", "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := call(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.email[0].HTML, "https://shop.example.com/?utm_campaign="+row.CampaignID.String()) {
		t.Errorf("html = %q, want the campaign link", c.email[0].HTML)
	}
}
