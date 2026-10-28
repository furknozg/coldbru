package confirm_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/confirm"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

func person(id string, email, sms bool) customer.Recipient {
	r := customer.Recipient{CustomerID: id, FirstName: "Ada", Language: "tr", EmailEligible: email, SMSEligible: sms}
	if email {
		r.Email = id + "@example.com"
	}
	if sms {
		r.Phone = "+90555" + id
	}
	return r
}

func entry(id, channel, contact string) flow.BatchRecipient {
	return flow.BatchRecipient{CustomerID: id, FirstName: "Ada", Language: "tr", Channel: channel, Contact: contact}
}

func TestAssign(t *testing.T) {
	t.Parallel()
	both, emailOnly, smsOnly, neither := person("1", true, true), person("2", true, false), person("3", false, true), person("4", false, false)
	emailEntry := func(id string) flow.BatchRecipient { return entry(id, "email", id+"@example.com") }
	smsEntry := func(id string) flow.BatchRecipient { return entry(id, "sms", "+90555"+id) }
	tests := []struct {
		name    string
		mode    llm.PlanMode
		primary llm.Channel
		who     customer.Recipient
		want    []flow.BatchRecipient
	}{
		{"single email: both", llm.ModeSingle, llm.ChannelEmail, both, []flow.BatchRecipient{emailEntry("1")}},
		{"single email: email only", llm.ModeSingle, llm.ChannelEmail, emailOnly, []flow.BatchRecipient{emailEntry("2")}},
		{"single email: sms only gets nothing", llm.ModeSingle, llm.ChannelEmail, smsOnly, []flow.BatchRecipient{}},
		{"single email: neither", llm.ModeSingle, llm.ChannelEmail, neither, []flow.BatchRecipient{}},
		{"single sms: both", llm.ModeSingle, llm.ChannelSMS, both, []flow.BatchRecipient{smsEntry("1")}},
		{"single sms: email only gets nothing", llm.ModeSingle, llm.ChannelSMS, emailOnly, []flow.BatchRecipient{}},
		{"single sms: sms only", llm.ModeSingle, llm.ChannelSMS, smsOnly, []flow.BatchRecipient{smsEntry("3")}},
		{"single sms: neither", llm.ModeSingle, llm.ChannelSMS, neither, []flow.BatchRecipient{}},

		{"fallback email: both gets the primary", llm.ModePrimaryFallback, llm.ChannelEmail, both, []flow.BatchRecipient{emailEntry("1")}},
		{"fallback email: email only", llm.ModePrimaryFallback, llm.ChannelEmail, emailOnly, []flow.BatchRecipient{emailEntry("2")}},
		{"fallback email: sms only falls back", llm.ModePrimaryFallback, llm.ChannelEmail, smsOnly, []flow.BatchRecipient{smsEntry("3")}},
		{"fallback email: neither", llm.ModePrimaryFallback, llm.ChannelEmail, neither, []flow.BatchRecipient{}},
		{"fallback sms: both gets the primary", llm.ModePrimaryFallback, llm.ChannelSMS, both, []flow.BatchRecipient{smsEntry("1")}},
		{"fallback sms: email only falls back", llm.ModePrimaryFallback, llm.ChannelSMS, emailOnly, []flow.BatchRecipient{emailEntry("2")}},
		{"fallback sms: sms only", llm.ModePrimaryFallback, llm.ChannelSMS, smsOnly, []flow.BatchRecipient{smsEntry("3")}},
		{"fallback sms: neither", llm.ModePrimaryFallback, llm.ChannelSMS, neither, []flow.BatchRecipient{}},

		{"both: both channels", llm.ModeBoth, llm.ChannelEmail, both, []flow.BatchRecipient{emailEntry("1"), smsEntry("1")}},
		{"both: the primary does not matter", llm.ModeBoth, llm.ChannelSMS, both, []flow.BatchRecipient{emailEntry("1"), smsEntry("1")}},
		{"both: email only", llm.ModeBoth, llm.ChannelEmail, emailOnly, []flow.BatchRecipient{emailEntry("2")}},
		{"both: sms only", llm.ModeBoth, llm.ChannelEmail, smsOnly, []flow.BatchRecipient{smsEntry("3")}},
		{"both: neither", llm.ModeBoth, llm.ChannelEmail, neither, []flow.BatchRecipient{}},

		{"unknown mode sends nothing", llm.PlanMode("fanout"), llm.ChannelEmail, both, []flow.BatchRecipient{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := confirm.Assign(tt.mode, tt.primary, tt.who)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Assign() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// A flag without a contact would give the receiver nothing to send to: the channel does not count.
func TestAssignTreatsAnEmptyContactAsNotEligible(t *testing.T) {
	t.Parallel()
	r := person("1", true, true)
	r.Email = ""
	got := confirm.Assign(llm.ModePrimaryFallback, llm.ChannelEmail, r)
	want := []flow.BatchRecipient{entry("1", "sms", "+905551")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Assign() = %+v, want the sms fallback %+v", got, want)
	}
	if got := confirm.Assign(llm.ModeSingle, llm.ChannelEmail, r); len(got) != 0 {
		t.Errorf("single email without an address = %+v, want nothing", got)
	}
}

func TestAssignCarriesTheRecipientFields(t *testing.T) {
	t.Parallel()
	r := customer.Recipient{CustomerID: "c-9", FirstName: "Zeynep", Language: "en", EmailEligible: true, Email: "z@example.com"}
	got := confirm.Assign(llm.ModeSingle, llm.ChannelEmail, r)
	want := flow.BatchRecipient{CustomerID: "c-9", FirstName: "Zeynep", Language: "en", Channel: "email", Contact: "z@example.com"}
	if len(got) != 1 || got[0] != want {
		t.Errorf("Assign() = %+v, want %+v", got, want)
	}
}

func TestBatches(t *testing.T) {
	t.Parallel()
	mk := func(n int) []flow.BatchRecipient {
		out := make([]flow.BatchRecipient, n)
		for i := range out {
			out[i] = entry(fmt.Sprint(i), "email", fmt.Sprintf("%d@example.com", i))
		}
		return out
	}
	tests := []struct {
		n    int
		want []int // recipients per batch
	}{
		{0, nil},
		{1, []int{1}},
		{499, []int{499}},
		{500, []int{500}},
		{501, []int{500, 1}},
		{1000, []int{500, 500}},
		{1001, []int{500, 500, 1}},
		{46, []int{46}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.n), func(t *testing.T) {
			t.Parallel()
			in := mk(tt.n)
			got := confirm.Batches("camp-1", in)
			if len(got) != len(tt.want) {
				t.Fatalf("%d recipients: %d batches, want %d", tt.n, len(got), len(tt.want))
			}
			next := 0
			for i, b := range got {
				if b.CampaignID != "camp-1" || b.BatchNo != i+1 || b.BatchCount != len(tt.want) {
					t.Errorf("batch %d header = %s %d/%d, want camp-1 %d/%d", i, b.CampaignID, b.BatchNo, b.BatchCount, i+1, len(tt.want))
				}
				if len(b.Recipients) != tt.want[i] || len(b.Recipients) > flow.MaxBatchRecipients {
					t.Errorf("batch %d has %d recipients, want %d", i+1, len(b.Recipients), tt.want[i])
				}
				for _, r := range b.Recipients { // order is kept, nobody twice, nobody lost
					if r != in[next] {
						t.Fatalf("batch %d: recipient %+v out of order, want %+v", i+1, r, in[next])
					}
					next++
				}
			}
			if next != tt.n {
				t.Errorf("batches hold %d recipients, want %d", next, tt.n)
			}
		})
	}
}

// Appending to a batch must not write into the next one.
func TestBatchesDoNotShareCapacity(t *testing.T) {
	t.Parallel()
	in := make([]flow.BatchRecipient, 501)
	got := confirm.Batches("c", in)
	if cap(got[0].Recipients) != flow.MaxBatchRecipients {
		t.Errorf("cap of the first batch = %d, want %d", cap(got[0].Recipients), flow.MaxBatchRecipients)
	}
}

func TestCount(t *testing.T) {
	t.Parallel()
	in := []flow.BatchRecipient{entry("1", "email", "a"), entry("1", "sms", "b"), entry("2", "email", "c")}
	want := flow.Recipients{Email: 2, SMS: 1, Total: 3}
	if got := confirm.Count(in); got != want {
		t.Errorf("Count() = %+v, want %+v (a person on two channels counts twice)", got, want)
	}
	if got := confirm.Count(nil); got != (flow.Recipients{}) {
		t.Errorf("Count(nil) = %+v, want zero", got)
	}
}
