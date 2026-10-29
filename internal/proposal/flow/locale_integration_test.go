//go:build integration

package flow_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// allZonesDSL matches customers in both time zones and both languages of the seed (D-076).
const allZonesDSL = `totalOrders >= 1`

// scriptWith queues the screen, a segment answer for dsl and a plan, and returns the audience.
func (h *harness) scriptWith(dsl string, mode llm.PlanMode, primary llm.Channel) *customer.Preview {
	h.llm.AllowScreen().WithRecords(llm.PurposeScreen, rec(llm.PurposeScreen, llm.OutcomeAccepted, 1, ""))
	h.llm.ProposeDSLs(dsl).WithRecords(llm.PurposeSegment, rec(llm.PurposeSegment, llm.OutcomeAccepted, 1, ""))
	h.llm.ProposePlans(llm.ChannelPlan{Mode: mode, Primary: primary, Rationale: "scripted"})
	return h.preview(dsl)
}

// toSchedule runs a campaign with the scripted schedule attempts up to its schedule step, the
// audience, plan and content approved.
func (h *harness) toSchedule(attempts ...[]llm.ScheduleEntry) (flow.CampaignInfo, *flow.Subscription, flow.StepView) {
	h.t.Helper()
	h.llm.ProposeSchedules(attempts...)
	camp, sub := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	h.approveAll(camp.ID, flow.KindChannelPlan)
	h.approveAll(camp.ID, flow.KindContent)
	return camp, sub, h.step(camp.ID, flow.KindSchedule)
}

func slotTZ(v flow.VariantView) string { return v.Language + "@" + v.Timezone }

// tomorrowMorning answers "yarın sabah" for the pairs: 09:00 of 2026-10-30, the day after the
// harness clock, on each pair's own wall clock. wrong lists pairs written with +01:00 instead.
func tomorrowMorning(t *testing.T, locales []llm.ScheduleLocale, wrong ...string) []llm.ScheduleEntry {
	t.Helper()
	var out []llm.ScheduleEntry
	for _, l := range locales {
		loc, err := time.LoadLocation(l.Timezone)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 10, 30, 9, 0, 0, 0, loc)
		if slices.Contains(wrong, l.Language+"@"+l.Timezone) {
			at = time.Date(2026, 10, 30, 9, 0, 0, 0, time.FixedZone("", 3600))
		}
		out = append(out, llm.ScheduleEntry{Language: l.Language, Timezone: l.Timezone, SendAt: &at, Rationale: "yarın sabah"})
	}
	return out
}

// The sample brief's audience is all in Istanbul: the schedule step has one slot for each language
// in Europe/Istanbul and none for London, although both zones exist in the seed. A scripted
// "yarın sabah" answer is 09:00+03:00 for both (D-076).
func TestSampleBriefScheduleSlotsAreIstanbulOnly(t *testing.T) {
	h := newHarness(t)
	prev := h.scriptWith(sampleDSL, llm.ModePrimaryFallback, llm.ChannelEmail)
	locales := localesOf(prev)
	if len(locales) < 2 {
		t.Fatalf("the sample audience has %d pairs, want tr and en", len(locales))
	}
	camp, sub, step := h.toSchedule(tomorrowMorning(t, locales))

	var got, want []string
	for _, l := range locales {
		if l.Timezone != "Europe/Istanbul" {
			t.Errorf("pair %+v: the sample audience is all in Istanbul", l)
		}
		want = append(want, l.Language+"@Europe/Istanbul")
	}
	for _, v := range step.Variants {
		got = append(got, slotTZ(v))
		var c flow.ScheduleContent
		decodeInto(t, v.Content, &c)
		if c.SendAt == nil || c.SendAt.Format(time.RFC3339) != "2026-10-30T09:00:00+03:00" {
			t.Errorf("%s: send_at = %v, want 2026-10-30T09:00:00+03:00", slotTZ(v), c.SendAt)
		}
		if strings.Contains(string(v.Content), "timezone") {
			t.Errorf("the timezone belongs to the slot, not to the content: %s", v.Content)
		}
		if raw, _ := json.Marshal(v); !strings.Contains(string(raw), `"timezone":"Europe/Istanbul"`) {
			t.Errorf("the variant view lacks its timezone: %s", raw)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("schedule slots = %v, want %v (every language in Europe/Istanbul only)", got, want)
	}
	if n := countRows(t, h, `SELECT count(*) FROM proposal_variant WHERE step_id = $1 AND timezone = 'Europe/London'`, step.ID); n != 0 {
		t.Errorf("%d London schedule variants for an Istanbul audience", n)
	}

	// The model is asked about exactly these pairs, each with the messages the plan sends to it.
	in := h.llm.ScheduleInputs()
	if len(in) != 1 || len(in[0].Locales) != len(locales) {
		t.Fatalf("schedule input = %+v", in)
	}
	total := 0
	for _, l := range in[0].Locales {
		total += l.Recipients
	}
	if total != 46 {
		t.Errorf("recipients of the schedule input add up to %d, want the plan's 46 messages", total)
	}

	// Events carry the timezone on the schedule slots.
	var created, started int
	for _, e := range events(sub) {
		switch ev := e.(type) {
		case flow.VariantCreated:
			if ev.Kind == flow.KindSchedule {
				created++
				if ev.Slot.Timezone != "Europe/Istanbul" || ev.Slot.Channel != "" || ev.Slot.Language == "" {
					t.Errorf("variant_created slot = %+v", ev.Slot)
				}
			}
		case flow.GenerationStarted:
			if ev.Kind == flow.KindSchedule {
				started++
				if len(ev.Slots) != len(locales) {
					t.Errorf("generation_started slots = %+v", ev.Slots)
				}
				for _, s := range ev.Slots {
					if s.Timezone != "Europe/Istanbul" {
						t.Errorf("generation_started slot = %+v", s)
					}
				}
			}
		}
	}
	if created != len(locales) || started != 1 {
		t.Errorf("%d schedule variant_created and %d generation_started events, want %d and 1", created, started, len(locales))
	}

	// The step closes when every slot is approved, then the Confirmer runs once.
	h.approveAll(camp.ID, flow.KindSchedule)
	if calls := h.confirmer.Calls(); len(calls) != 1 || calls[0] != camp.ID {
		t.Errorf("Confirmer calls = %v", calls)
	}
}

// A brief whose audience lives in both zones gets four slots. London is +00:00 in late October:
// a first answer that writes London's morning with +01:00 is sent back (offset check), the repair
// is accepted, and the stored times are 09:00 on each wall clock (D-076).
func TestAudienceInBothZonesGetsFourSlots(t *testing.T) {
	h := newHarness(t)
	prev := h.scriptWith(allZonesDSL, llm.ModePrimaryFallback, llm.ChannelEmail)
	locales := localesOf(prev)
	if len(locales) != 4 {
		t.Fatalf("pairs = %+v, want tr and en in both zones", locales)
	}
	camp, _, step := h.toSchedule(
		tomorrowMorning(t, locales, "tr@Europe/London", "en@Europe/London"),
		tomorrowMorning(t, locales),
	)

	if len(step.Variants) != 4 {
		t.Fatalf("schedule variants = %d, want 4", len(step.Variants))
	}
	wantInstant := map[string]string{
		"en@Europe/Istanbul": "2026-10-30T06:00:00Z", "tr@Europe/Istanbul": "2026-10-30T06:00:00Z",
		"en@Europe/London": "2026-10-30T09:00:00Z", "tr@Europe/London": "2026-10-30T09:00:00Z",
	}
	for _, v := range step.Variants {
		var c flow.ScheduleContent
		decodeInto(t, v.Content, &c)
		want, ok := wantInstant[slotTZ(v)]
		if !ok || c.SendAt == nil || c.SendAt.UTC().Format(time.RFC3339) != want {
			t.Errorf("%s: send_at = %v, want the instant %s", slotTZ(v), c.SendAt, want)
		}
	}
	if len(h.step(camp.ID, flow.KindSchedule).Missing) != 0 {
		t.Error("slots are missing")
	}

	// The repair loop ran in one conversation: one rejected attempt (the offset), one accepted.
	rows, err := h.q.ListLLMCalls(t.Context(), camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	var outcomes []string
	for _, r := range rows {
		if r.Purpose == "schedule" {
			outcomes = append(outcomes, r.Outcome)
			if r.Outcome == "validation_failed" && !strings.Contains(r.Error.String, "Europe/London") {
				t.Errorf("the repair message does not name the zone: %q", r.Error.String)
			}
		}
	}
	if !slices.Equal(outcomes, []string{"validation_failed", "accepted"}) {
		t.Errorf("schedule llm_call outcomes = %v", outcomes)
	}

	// Editing checks the offset of the slot's own zone: London takes +00:00 here, Istanbul +03:00.
	var london, istanbul flow.VariantView
	for _, v := range step.Variants {
		switch slotTZ(v) {
		case "en@Europe/London":
			london = v
		case "en@Europe/Istanbul":
			istanbul = v
		}
	}
	for name, c := range map[string]struct {
		v    flow.VariantView
		body string
	}{
		"london with summer time":      {london, `{"send_at":"2026-10-30T09:00:00+01:00"}`},
		"istanbul with london offset":  {istanbul, `{"send_at":"2026-10-30T09:00:00Z"}`},
		"istanbul with another offset": {istanbul, `{"send_at":"2026-10-30T09:00:00+02:00"}`},
	} {
		_, err := h.svc.Edit(t.Context(), c.v.ID, json.RawMessage(c.body))
		var ice *flow.InvalidContentError
		if !errors.Is(err, flow.ErrInvalidContent) || !errors.As(err, &ice) || !strings.Contains(strings.Join(ice.Problems, ";"), "offset") {
			t.Errorf("%s: err = %v, want an offset problem", name, err)
		}
	}
	nl, err := h.svc.Edit(t.Context(), london.ID, json.RawMessage(`{"send_at":"2026-10-30T18:00:00+00:00","rationale":"evening"}`))
	if err != nil || nl.Language != "en" || nl.Timezone != "Europe/London" || nl.ID == london.ID {
		t.Fatalf("Edit london = %+v, %v; want a new draft in the same slot", nl, err)
	}
	if got := variantStatus(t, h, london.ID); got != flow.VariantStatusSuperseded {
		t.Errorf("the edited variant is %s, want superseded", got)
	}

	// Approving every slot closes the step and calls the Confirmer.
	h.approveAll(camp.ID, flow.KindSchedule)
	if s := h.step(camp.ID, flow.KindSchedule); s.Status != flow.StepStatusApproved {
		t.Errorf("schedule step = %s", s.Status)
	}
	if calls := h.confirmer.Calls(); len(calls) != 1 {
		t.Errorf("Confirmer calls = %v", calls)
	}
}

// A failed schedule generation leaves its (language, timezone) slots missing, and Regenerate fills
// them.
func TestScheduleMissingSlotsCarryTheTimezone(t *testing.T) {
	h := newHarness(t)
	prev := h.scriptWith(allZonesDSL, llm.ModePrimaryFallback, llm.ChannelEmail)
	locales := localesOf(prev)
	h.llm.FailSchedule(&llm.APIError{Purpose: llm.PurposeSchedule, Status: 529})
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	h.approveAll(camp.ID, flow.KindChannelPlan)
	h.approveAll(camp.ID, flow.KindContent)

	step := h.step(camp.ID, flow.KindSchedule)
	if !step.Failed || len(step.Variants) != 0 || len(step.Missing) != 4 {
		t.Fatalf("schedule step = %+v, want failed with four missing slots", step)
	}
	for i, m := range step.Missing {
		if m.Language != locales[i].Language || m.Timezone != locales[i].Timezone || m.Channel != "" {
			t.Errorf("missing slot %d = %+v, want %+v", i, m, locales[i])
		}
	}
	h.llm.ProposeSchedules(tomorrowMorning(t, locales))
	if err := h.svc.Regenerate(t.Context(), step.ID); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if step = h.step(camp.ID, flow.KindSchedule); len(step.Variants) != 4 || len(step.Missing) != 0 || step.Failed {
		t.Errorf("after Regenerate: %+v", step)
	}
}
