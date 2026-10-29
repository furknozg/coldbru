//go:build integration

package confirm_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// batchesOf reads every RECIPIENTS message of subject, in stream order.
func (s *system) batchesOf(subject string) []flow.RecipientBatch {
	s.t.Helper()
	info, err := s.recips.Info(s.t.Context(), jetstream.WithSubjectFilter(subject))
	if err != nil {
		s.t.Fatal(err)
	}
	var out []flow.RecipientBatch
	for seq := info.State.FirstSeq; seq <= info.State.LastSeq; seq++ {
		m, err := s.recips.GetMsg(s.t.Context(), seq)
		if err != nil || m.Subject != subject {
			continue
		}
		var b flow.RecipientBatch
		if err := json.Unmarshal(m.Data, &b); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, b)
	}
	return out
}

// An audience in both time zones is confirmed with four schedules, and every recipient carries the
// zone Customer has for the person (D-076). The mock sends immediately, so send_at is null.
func TestConfirmCarriesTheTimezoneOfEverySchedule(t *testing.T) {
	s := newSystem(t)
	// The mock answers a brief it does not know with `totalOrders >= 1`: both languages, both zones.
	w := s.walkBrief("Daha önce sipariş vermiş herkese teşekkür mesajı gönder.")
	if w.aud.DSL != "totalOrders >= 1" || len(w.aud.ByLocale) != 4 || w.slots != 4 || len(w.schedules) != 4 {
		t.Fatalf("audience %q with %d pairs and %d schedule slots, want totalOrders >= 1 with 4 and 4", w.aud.DSL, len(w.aud.ByLocale), w.slots)
	}
	for _, v := range w.schedules {
		if v.Timezone == "" || v.Language == "" {
			t.Errorf("schedule variant %+v has no (language, timezone)", v)
		}
	}
	if code, raw := s.approveSchedules(w); code != 200 {
		t.Fatalf("the last schedule approval = %d: %s", code, raw)
	}

	// evt.proposal.confirmed.v1: schedules[{language, timezone, send_at}] for the four pairs.
	s.waitSubject(s.events, "evt.proposal.confirmed.v1", 1)
	var env events.Envelope
	if err := json.Unmarshal(s.lastMsg(s.events, "evt.proposal.confirmed.v1").Data, &env); err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Schedules []map[string]any `json:"schedules"`
	}
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatal(err)
	}
	var data flow.ConfirmedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	var pairs []string
	for i, sc := range data.Schedules {
		pairs = append(pairs, sc.Language+"@"+sc.Timezone)
		if sc.SendAt != nil {
			t.Errorf("schedule %s@%s send_at = %v, want null (the mock sends immediately)", sc.Language, sc.Timezone, sc.SendAt)
		}
		if _, ok := raw.Schedules[i]["timezone"]; !ok || len(raw.Schedules[i]) != 3 {
			t.Errorf("schedule %d keys = %v, want language, timezone, send_at", i, raw.Schedules[i])
		}
	}
	want := []string{"en@Europe/Istanbul", "en@Europe/London", "tr@Europe/Istanbul", "tr@Europe/London"}
	if !slices.Equal(pairs, want) {
		t.Errorf("schedules = %v, want %v (ordered by language, then timezone)", pairs, want)
	}

	// RECIPIENTS: every recipient has a timezone with a schedule, and the people per pair are the
	// messages the plan sends to that pair.
	subject := "recipients." + w.id.String()
	s.waitSubject(s.recips, subject, data.RecipientBatches)
	got := map[string]int{}
	total := 0
	for _, b := range s.batchesOf(subject) {
		for _, r := range b.Recipients {
			total++
			got[r.Language+"@"+r.Timezone]++
			if !slices.Contains(want, r.Language+"@"+r.Timezone) {
				t.Errorf("recipient %+v has no schedule", r)
			}
		}
	}
	if total != data.RecipientCounts.Total {
		t.Errorf("batches hold %d recipients, the event says %d", total, data.RecipientCounts.Total)
	}
	msgs, err := flow.MessagesByLocale(w.plan, w.aud)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if k := m.Language + "@" + m.Timezone; got[k] != m.Total() {
			t.Errorf("%s: %d recipients in the batches, the plan sends %d messages", k, got[k], m.Total())
		}
	}
}
