package confirm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// fakeCustomer serves fixed pages.
type fakeCustomer struct {
	pages [][]customer.Recipient
	err   error
	calls int
}

func (f *fakeCustomer) EachRecipientPage(_ context.Context, _ *compile.Filter, fn func([]customer.Recipient) error) error {
	f.calls++
	for _, p := range f.pages {
		if err := fn(p); err != nil {
			return err
		}
	}
	return f.err
}

const (
	istanbul = "Europe/Istanbul"
	london   = "Europe/London"
)

// rec is a recipient living in Istanbul; recIn puts one in another zone.
func rec(id, lang string, email, sms bool) customer.Recipient {
	return recIn(id, lang, istanbul, email, sms)
}

func recIn(id, lang, tz string, email, sms bool) customer.Recipient {
	r := customer.Recipient{CustomerID: id, FirstName: "N" + id, Language: lang, Timezone: tz, EmailEligible: email, SMSEligible: sms}
	if email {
		r.Email = id + "@example.com"
	}
	if sms {
		r.Phone = "+90" + id
	}
	return r
}

func approvedFor(mode llm.PlanMode, primary llm.Channel) *approved {
	return &approved{
		audience: flow.AudienceContent{DSL: `totalOrders >= 1`},
		plan:     flow.PlanContent{Mode: mode, Primary: primary},
		messages: []flow.ConfirmedMessage{
			{Language: "tr", Channel: "email"}, {Language: "tr", Channel: "sms"}, {Language: "en", Channel: "email"},
		},
		schedules: []flow.ConfirmedSchedule{{Language: "tr", Timezone: istanbul}, {Language: "en", Timezone: istanbul}},
	}
}

func TestPrepare(t *testing.T) {
	t.Parallel()
	c := &Confirmer{cust: &fakeCustomer{pages: [][]customer.Recipient{
		{rec("1", "tr", true, true), rec("2", "tr", true, false)},
		{rec("3", "tr", false, true), rec("4", "en", true, false), rec("5", "tr", false, false)},
	}}}
	p, err := c.prepare(t.Context(), approvedFor(llm.ModePrimaryFallback, llm.ChannelEmail))
	if err != nil {
		t.Fatal(err)
	}
	// 1 -> email, 2 -> email, 3 -> sms (fallback), 4 -> email (en), 5 -> nobody.
	if want := (flow.Recipients{Email: 3, SMS: 1, Total: 4}); p.counts != want || len(p.recipients) != 4 || p.dropped != 0 {
		t.Errorf("counts = %+v recipients = %d dropped = %d, want %+v / 4 / 0", p.counts, len(p.recipients), p.dropped, want)
	}
	if p.recipients[0].CustomerID != "1" || p.recipients[3].CustomerID != "4" {
		t.Errorf("order not kept across pages: %+v", p.recipients)
	}
	for _, r := range p.recipients {
		if r.Timezone != istanbul {
			t.Errorf("recipient %s has timezone %q, want the one Customer reported (D-076)", r.CustomerID, r.Timezone)
		}
	}
}

// A person whose (language, timezone) has no approved schedule is dropped and the pair is logged
// with its head count, even when the language and the channel are covered (D-076, D-074).
func TestPrepareDropsPeopleWhoseLocaleHasNoSchedule(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	c := &Confirmer{
		log: slog.New(slog.NewTextHandler(&logs, nil)),
		cust: &fakeCustomer{pages: [][]customer.Recipient{{
			recIn("1", "tr", istanbul, true, false),
			recIn("2", "tr", london, true, false), // a Turkish speaker in London: no (tr, London) schedule
			recIn("3", "en", london, true, false), // English in London: no schedule either
			recIn("4", "en", istanbul, true, false),
			recIn("5", "tr", london, true, false),
		}}},
	}
	p, err := c.prepare(t.Context(), approvedFor(llm.ModePrimaryFallback, llm.ChannelEmail))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range p.recipients {
		ids = append(ids, r.CustomerID)
	}
	if !slices.Equal(ids, []string{"1", "4"}) || p.dropped != 3 || p.counts.Total != 2 {
		t.Errorf("recipients = %v dropped = %d counts = %+v, want 1 and 4 kept and 3 dropped", ids, p.dropped, p.counts)
	}
	got := logs.String()
	for _, want := range []string{"level=WARN", "locale_channel=tr/Europe/London/email", "recipients=2", "locale_channel=en/Europe/London/email", "recipients=1"} {
		if !strings.Contains(got, want) {
			t.Errorf("log lacks %q:\n%s", want, got)
		}
	}
}

// With a schedule for both zones nobody is dropped and every recipient keeps its own zone.
func TestPrepareKeepsEveryPairWithASchedule(t *testing.T) {
	t.Parallel()
	a := approvedFor(llm.ModePrimaryFallback, llm.ChannelEmail)
	a.schedules = []flow.ConfirmedSchedule{
		{Language: "tr", Timezone: istanbul}, {Language: "tr", Timezone: london},
		{Language: "en", Timezone: istanbul}, {Language: "en", Timezone: london},
	}
	c := &Confirmer{cust: &fakeCustomer{pages: [][]customer.Recipient{{
		recIn("1", "tr", london, true, false), recIn("2", "en", london, true, false), recIn("3", "tr", istanbul, true, false),
	}}}}
	p, err := c.prepare(t.Context(), a)
	if err != nil || p.dropped != 0 || len(p.recipients) != 3 {
		t.Fatalf("prepare = %+v, %v", p, err)
	}
	for i, tz := range []string{london, london, istanbul} {
		if p.recipients[i].Timezone != tz {
			t.Errorf("recipient %d timezone = %q, want %q", i, p.recipients[i].Timezone, tz)
		}
	}
}

func TestPrepareBothModeCountsAPersonPerChannel(t *testing.T) {
	t.Parallel()
	c := &Confirmer{cust: &fakeCustomer{pages: [][]customer.Recipient{{rec("1", "tr", true, true)}}}}
	p, err := c.prepare(t.Context(), approvedFor(llm.ModeBoth, llm.ChannelEmail))
	if err != nil {
		t.Fatal(err)
	}
	if want := (flow.Recipients{Email: 1, SMS: 1, Total: 2}); p.counts != want {
		t.Errorf("counts = %+v, want %+v", p.counts, want)
	}
}

// The audience can change between the preview and the confirmation (ago(), D-039): a person whose
// language or channel has no approved text cannot be sent anything and is left out.
func TestPrepareDropsPeopleTheTextsDoNotCover(t *testing.T) {
	t.Parallel()
	c := &Confirmer{cust: &fakeCustomer{pages: [][]customer.Recipient{{
		rec("1", "tr", true, false),
		rec("2", "de", true, false), // no German text or schedule
		rec("3", "en", false, true), // English has an email text only
	}}}}
	p, err := c.prepare(t.Context(), approvedFor(llm.ModePrimaryFallback, llm.ChannelEmail))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.recipients) != 1 || p.recipients[0].CustomerID != "1" || p.dropped != 2 || p.counts.Total != 1 {
		t.Errorf("recipients = %+v dropped = %d counts = %+v, want only person 1 and 2 dropped", p.recipients, p.dropped, p.counts)
	}
}

func TestPrepareWithoutRecipientsIsAnError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		pages [][]customer.Recipient
		mode  llm.PlanMode
	}{
		{"empty audience", nil, llm.ModeBoth},
		{"nobody eligible", [][]customer.Recipient{{rec("1", "tr", false, false)}}, llm.ModeBoth},
		{"single email but only sms people", [][]customer.Recipient{{rec("1", "tr", false, true)}}, llm.ModeSingle},
		{"everybody dropped", [][]customer.Recipient{{rec("1", "de", true, true)}}, llm.ModeBoth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Confirmer{cust: &fakeCustomer{pages: tt.pages}}
			_, err := c.prepare(t.Context(), approvedFor(tt.mode, llm.ChannelEmail))
			if !errors.Is(err, flow.ErrNoRecipients) {
				t.Errorf("err = %v, want flow.ErrNoRecipients", err)
			}
		})
	}
}

func TestPrepareReportsACustomerFailure(t *testing.T) {
	t.Parallel()
	boom := &customer.StatusError{Status: 500}
	c := &Confirmer{cust: &fakeCustomer{pages: [][]customer.Recipient{{rec("1", "tr", true, true)}}, err: boom}}
	_, err := c.prepare(t.Context(), approvedFor(llm.ModeBoth, llm.ChannelEmail))
	var se *customer.StatusError
	if !errors.As(err, &se) || errors.Is(err, flow.ErrNoRecipients) {
		t.Errorf("err = %v, want the Customer error and not ErrNoRecipients", err)
	}
}

func TestPrepareRejectsAFilterThatNoLongerCompiles(t *testing.T) {
	t.Parallel()
	fc := &fakeCustomer{}
	c := &Confirmer{cust: fc}
	a := approvedFor(llm.ModeBoth, llm.ChannelEmail)
	a.audience.DSL = `this is not a filter (`
	if _, err := c.prepare(t.Context(), a); err == nil || fc.calls != 0 {
		t.Errorf("err = %v, customer calls = %d, want an error before Customer is called", err, fc.calls)
	}
}

func TestNewValidatesItsDependencies(t *testing.T) {
	t.Parallel()
	if _, err := New(Deps{}); err == nil {
		t.Error("New without a pool succeeded")
	}
	if _, err := New(Deps{Customer: &fakeCustomer{}}); err == nil {
		t.Error("New without a pool succeeded")
	}
}
