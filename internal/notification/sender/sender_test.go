package sender

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"golang.org/x/time/rate"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/platform/consumer"
)

// runes is the length the 160 limit applies to.
func runes(s string) int { return utf8.RuneCountInString(s) }

// pad returns a template of exactly n characters in which {{first_name}} stands for nothing yet:
// the placeholder is placed first, followed by filler, so that expanding it adds the name's length.
func padTemplate(prefix string, n int) string {
	filler := n - runes(prefix)
	return prefix + strings.Repeat("a", filler)
}

func TestFitSMS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		template string
		first    string
		want     string
		wantFit  nameFit
	}{
		{"short text keeps the name", "Merhaba {{first_name}}, sepetin seni bekliyor", "Ayşe", "Merhaba Ayşe, sepetin seni bekliyor", nameFull},
		{"placeholder with spaces", "Hi {{ first_name }}!", "Bob", "Hi Bob!", nameFull},
		{"no placeholder", "Sepetin seni bekliyor", "Ayşe", "Sepetin seni bekliyor", nameFull},
		{"exactly 160 characters is allowed", padTemplate("{{first_name}}", 160-len("Ada")+len("{{first_name}}")), "Ada", "", nameFull},
		{"one over 160 shortens the name by one", padTemplate("{{first_name}}", 161-len("Ada")+len("{{first_name}}")), "Ada", "", nameShortened},
		{"two placeholders share the room", "{{first_name}} {{first_name}} " + strings.Repeat("a", 143), "Abcdefgh", "", nameShortened},
		{"no room at all drops the name", "Merhaba {{first_name}}! " + strings.Repeat("a", 160-len("Merhaba ! ")), "Ada", "", nameDropped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, fit := fitSMS(tt.template, tt.first)
			if fit != tt.wantFit {
				t.Fatalf("fit = %s, want %s (text %q)", fit, tt.wantFit, got)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
			if n := runes(got); n > content.SMSMaxChars {
				t.Errorf("text has %d characters, want at most 160", n)
			}
		})
	}
}

func TestFitSMSBoundaryWithTheRealName(t *testing.T) {
	t.Parallel()
	// 160 characters with the name in: untouched.
	tpl := "{{first_name}}" + strings.Repeat("x", 157)
	text, fit := fitSMS(tpl, "Ada")
	if fit != nameFull || runes(text) != 160 || !strings.HasPrefix(text, "Ada") {
		t.Fatalf("160 characters: fit = %s, length = %d", fit, runes(text))
	}
	// 161: the name loses its last character, the text is exactly 160.
	text, fit = fitSMS("{{first_name}}"+strings.Repeat("x", 158), "Ada")
	if fit != nameShortened || runes(text) != 160 || !strings.HasPrefix(text, "Ad"+"x") {
		t.Fatalf("161 characters: fit = %s, length = %d, text = %.8q", fit, runes(text), text)
	}
	// Room for no character of the name: dropped, and the text is the template without it.
	text, fit = fitSMS("{{first_name}}"+strings.Repeat("x", 160), "Ada")
	if fit != nameDropped || text != strings.Repeat("x", 160) {
		t.Fatalf("no room: fit = %s, length = %d", fit, runes(text))
	}
}

func TestFitSMSTurkishCharactersCountAsCharacters(t *testing.T) {
	t.Parallel()
	// Ç, ş, ğ, ı and İ are one character each (two bytes in UTF-8, one UTF-16 unit). The limit counts
	// characters, so a name of such letters is cut at a character boundary and stays valid UTF-8.
	name := "İşığıÇağlar"
	tpl := "{{first_name}}" + strings.Repeat("x", 154) // room for 6 characters of the name
	text, fit := fitSMS(tpl, name)
	if fit != nameShortened {
		t.Fatalf("fit = %s, want shortened", fit)
	}
	if !utf8.ValidString(text) || runes(text) != 160 {
		t.Fatalf("length = %d, valid = %v, want 160 valid characters", runes(text), utf8.ValidString(text))
	}
	if !strings.HasPrefix(text, "İşığıÇ"+"x") {
		t.Errorf("text starts with %.20q, want the first 6 characters of the name", text)
	}
	// UCS-2 text of 160 characters is still 160 characters: the limit is not in UTF-16 units.
	if info := content.MeasureSMS(text); info.Encoding != content.UCS2 || info.Length != 160 {
		t.Errorf("measure = %+v, want UCS-2 and length 160", info)
	}
}

func TestFitSMSEmojiNameIsOneCharacterEach(t *testing.T) {
	t.Parallel()
	// An emoji is one character for the limit (two UTF-16 units) and is never cut in half.
	tpl := "{{first_name}}" + strings.Repeat("x", 158)
	text, fit := fitSMS(tpl, "😀😀😀")
	if fit != nameShortened || !utf8.ValidString(text) || runes(text) != 160 || !strings.HasPrefix(text, "😀😀x") {
		t.Errorf("fit = %s, valid = %v, length = %d", fit, utf8.ValidString(text), runes(text))
	}
}

func TestFitSMSTrimsSpaceAfterTheCut(t *testing.T) {
	t.Parallel()
	// "Ali Veli" cut to 4 characters is "Ali " and must not leave a dangling space before the text.
	tpl := "{{first_name}}!" + strings.Repeat("x", 155) // 160 - base(156) = room for 4
	text, fit := fitSMS(tpl, "Ali Veli")
	if fit != nameShortened || !strings.HasPrefix(text, "Ali!") {
		t.Errorf("fit = %s, text = %.10q, want shortened with Ali!", fit, text)
	}
}

func TestFitSMSDroppedNameLeavesCleanText(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ tpl, want string }{
		{"Merhaba {{first_name}}, " + strings.Repeat("a", 150), "Merhaba, " + strings.Repeat("a", 150)},
		{"{{first_name}}, " + strings.Repeat("a", 160), strings.Repeat("a", 160)},
		{"Merhaba {{first_name}}! " + strings.Repeat("a", 150), "Merhaba! " + strings.Repeat("a", 150)},
		{strings.Repeat("a", 158) + " {{first_name}}.", strings.Repeat("a", 158) + "."},
	} {
		text, fit := fitSMS(tt.tpl, "Abdurrahman Cemalettin")
		if fit != nameDropped || text != tt.want {
			t.Errorf("fitSMS(%.30q...) = %.40q (%s), want %.40q dropped", tt.tpl, text, fit, tt.want)
		}
	}
}

func TestFitSMSTemplateTooLongStaysTooLong(t *testing.T) {
	t.Parallel()
	// Text that is over 160 without the name is not repaired; the caller refuses to send it.
	text, fit := fitSMS("Merhaba {{first_name}} "+strings.Repeat("a", 200), "Ada")
	if fit != nameDropped || runes(text) <= content.SMSMaxChars {
		t.Errorf("fit = %s, length = %d, want dropped and still too long", fit, runes(text))
	}
}

func TestGatePausesAllCallers(t *testing.T) {
	t.Parallel()
	g := newGate(rate.Inf, 1)
	g.PauseFor(150 * time.Millisecond)
	if d := g.Paused(); d <= 0 || d > 150*time.Millisecond {
		t.Fatalf("Paused() = %s, want within (0, 150ms]", d)
	}
	start := time.Now()
	done := make(chan time.Duration, 3)
	for range 3 {
		go func() {
			if err := g.Wait(t.Context()); err != nil {
				t.Errorf("Wait() error = %v", err)
			}
			done <- time.Since(start)
		}()
	}
	for range 3 {
		if d := <-done; d < 120*time.Millisecond {
			t.Errorf("a caller got through after %s, want it held for the pause", d)
		}
	}
	if g.Paused() != 0 {
		t.Errorf("Paused() = %s after the pause, want 0", g.Paused())
	}
}

func TestGatePauseNeverShortens(t *testing.T) {
	t.Parallel()
	g := newGate(rate.Inf, 1)
	g.PauseFor(time.Minute)
	g.PauseFor(time.Millisecond)
	g.PauseFor(0)
	if g.Paused() < 50*time.Second {
		t.Errorf("Paused() = %s, want the longer pause to stand", g.Paused())
	}
}

func TestGateWaitEndsWithContext(t *testing.T) {
	t.Parallel()
	g := newGate(rate.Inf, 1)
	g.PauseFor(time.Minute)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := g.Wait(ctx); err == nil {
		t.Fatal("Wait() returned nil during a pause with a cancelled context")
	}
	if time.Since(start) > time.Second {
		t.Error("Wait() did not return promptly when the context ended")
	}
}

func TestGateLimitsRate(t *testing.T) {
	t.Parallel()
	// 10 tokens at once, then 100 a second: 30 waits take about 200 ms.
	g := newGate(RatePerSecond, Burst)
	start := time.Now()
	for range 30 {
		if err := g.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 170*time.Millisecond || d > 600*time.Millisecond {
		t.Errorf("30 waits took %s, want about 200ms", d)
	}
}

func TestProductionConstants(t *testing.T) {
	t.Parallel()
	if RatePerSecond != 100 || Burst != 10 || MaxDeliver != 5 || BatchSize != 100 {
		t.Errorf("rate = %d, burst = %d, max deliver = %d, batch = %d; want 100, 10, 5, 100", RatePerSecond, Burst, MaxDeliver, BatchSize)
	}
	if LeaseDuration != 30*time.Second {
		t.Errorf("lease = %s, want 30s", LeaseDuration)
	}
	if AckWait <= LeaseDuration {
		t.Errorf("ack wait %s must be longer than the lease %s", AckWait, LeaseDuration)
	}
}

func TestConsumerConfig(t *testing.T) {
	t.Parallel()
	for ch, want := range map[string]string{"sms": "send.sms", "email": "send.email"} {
		cfg := ConsumerConfig(ch)
		if cfg.Durable != "notification-send-"+ch || cfg.FilterSubject != want {
			t.Errorf("%s: durable = %q, filter = %q", ch, cfg.Durable, cfg.FilterSubject)
		}
		if cfg.MaxDeliver != 5 || cfg.AckPolicy != jetstream.AckExplicitPolicy || cfg.AckWait != AckWait {
			t.Errorf("%s: %+v", ch, cfg)
		}
	}
}

func TestDefaultBackoffValues(t *testing.T) {
	t.Parallel()
	// 2 s, doubling, capped at 1 minute, within 20% jitter.
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, time.Minute, time.Minute}
	for i, base := range want {
		for range 50 {
			got := DefaultBackoff.Delay(i + 1)
			lo, hi := time.Duration(float64(base)*0.8), time.Duration(float64(base)*1.2)
			if got < lo || got > hi {
				t.Fatalf("Delay(%d) = %s, want within %s..%s", i+1, got, lo, hi)
			}
		}
	}
	exact := DefaultBackoff
	exact.Jitter = 0
	if exact.Delay(1) != 2*time.Second || exact.Delay(3) != 8*time.Second || exact.Delay(10) != time.Minute {
		t.Error("backoff without jitter is not 2s, 4s, 8s ... 60s")
	}
}

func TestMetricsSeries(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	m.final("sms", "sent")
	m.final("sms", "sent")
	m.final("email", "skipped_no_consent")
	m.providerCall("sms", 202, 0.01)
	m.providerCall("sms", 429, 0.02)
	m.providerCall("sms", 503, 0.02)
	m.providerCall("email", 0, 0.5)

	check := func(name string, want int) {
		t.Helper()
		if got := testutil.CollectAndCount(reg, name); got != want {
			t.Errorf("%s has %d series, want %d", name, got, want)
		}
	}
	check("deliveries_total", 8)         // 2 channels x 4 statuses
	check("provider_requests_total", 10) // 2 channels x 5 classes
	check("provider_429_total", 2)
	check("send_latency_seconds", 2)

	if got := testutil.ToFloat64(m.deliveries.WithLabelValues("sms", "sent")); got != 2 {
		t.Errorf("deliveries_total{sms,sent} = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("sms", "4xx")); got != 1 {
		t.Errorf("provider_requests_total{sms,4xx} = %v, want 1 (the 429)", got)
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("email", "error")); got != 1 {
		t.Errorf("provider_requests_total{email,error} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.rateLimits.WithLabelValues("sms")); got != 1 {
		t.Errorf("provider_429_total{sms} = %v, want 1", got)
	}
}

// fakeMsg is a jetstream.Msg that records how it was settled.
type fakeMsg struct {
	jetstream.Msg
	subject   string
	data      string
	delivered uint64
	acked     atomic.Int32
	naked     atomic.Int32
	termed    atomic.Int32
	delay     atomic.Int64
}

func (m *fakeMsg) Data() []byte    { return []byte(m.data) }
func (m *fakeMsg) Subject() string { return m.subject }
func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{NumDelivered: m.delivered}, nil
}
func (m *fakeMsg) Ack() error  { m.acked.Add(1); return nil }
func (m *fakeMsg) Term() error { m.termed.Add(1); return nil }
func (m *fakeMsg) NakWithDelay(d time.Duration) error {
	m.naked.Add(1)
	m.delay.Store(int64(d))
	return nil
}

func TestMalformedMessagesAreTerminated(t *testing.T) {
	t.Parallel()
	w := &worker{channel: "sms", log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	msgs := []*fakeMsg{
		{subject: "send.sms", data: `not json`, delivered: 1},
		{subject: "send.sms", data: `{"delivery_id":"nope"}`, delivered: 1},
		{subject: "send.sms", data: `{}`, delivered: 1},
	}
	var in []jetstream.Msg
	for _, m := range msgs {
		in = append(in, m)
	}
	w.handleBatch(t.Context(), in) // nothing decodes, so no database is touched
	for i, m := range msgs {
		if m.termed.Load() != 1 || m.acked.Load() != 0 || m.naked.Load() != 0 {
			t.Errorf("message %d: term = %d, ack = %d, nak = %d; want one Term", i, m.termed.Load(), m.acked.Load(), m.naked.Load())
		}
	}
}

func TestNakOrTermOnTheLastDelivery(t *testing.T) {
	t.Parallel()
	w := &worker{
		channel: "sms", log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		backoff: consumer.Backoff{Base: time.Second, Factor: 2, Max: time.Minute},
	}
	early := &fakeMsg{subject: "send.sms", delivered: 2}
	w.nakOrTerm(early, 2, [16]byte{}, "load", io.ErrUnexpectedEOF)
	if early.naked.Load() != 1 || time.Duration(early.delay.Load()) != 2*time.Second || early.termed.Load() != 0 {
		t.Errorf("delivery 2: nak = %d (%s), term = %d; want a 2s nak", early.naked.Load(), time.Duration(early.delay.Load()), early.termed.Load())
	}
	last := &fakeMsg{subject: "send.sms", delivered: 5}
	w.nakOrTerm(last, 5, [16]byte{}, "load", io.ErrUnexpectedEOF)
	if last.termed.Load() != 1 || last.naked.Load() != 0 {
		t.Errorf("delivery 5: nak = %d, term = %d; want Term", last.naked.Load(), last.termed.Load())
	}
}

func TestDeliveriesDefaultsToOne(t *testing.T) {
	t.Parallel()
	if got := deliveries(&fakeMsg{delivered: 3}); got != 3 {
		t.Errorf("deliveries() = %d, want 3", got)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	if got := truncate("short"); got != "short" {
		t.Errorf("truncate(short) = %q", got)
	}
	long := strings.Repeat("ş", 400) // 800 bytes
	got := truncate(long)
	if !utf8.ValidString(got) || len(got) > maxErrorLen+len("…") {
		t.Errorf("truncate() = %d bytes, valid = %v", len(got), utf8.ValidString(got))
	}
}

func TestFinalCountsTheDeliveryAndNudges(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	var nudges atomic.Int32
	w := &worker{channel: "sms", m: newMetrics(reg), nudge: func() { nudges.Add(1) }}
	w.final("sent")
	w.final("failed")
	if got := nudges.Load(); got != 2 {
		t.Errorf("nudges = %d, want 2", got)
	}
	if got := testutil.ToFloat64(w.m.deliveries.WithLabelValues("sms", "sent")); got != 1 {
		t.Errorf("sent counter = %v, want 1", got)
	}
	// Without a nudge function a settled delivery is still counted.
	w = &worker{channel: "email", m: newMetrics(prometheus.NewRegistry())}
	w.final("sent")
	if got := testutil.ToFloat64(w.m.deliveries.WithLabelValues("email", "sent")); got != 1 {
		t.Errorf("sent counter without nudge = %v, want 1", got)
	}
}
