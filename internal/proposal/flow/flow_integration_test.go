//go:build integration

package flow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

func langsOf(p *customer.Preview) []string {
	var out []string
	for _, l := range p.ByLanguage {
		out = append(out, l.Language)
	}
	sort.Strings(out)
	return out
}

func variantStatus(t *testing.T, h *harness, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := h.pool.QueryRow(t.Context(), `SELECT status FROM proposal_variant WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("variant status: %v", err)
	}
	return s
}

func stepStatus(t *testing.T, h *harness, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := h.pool.QueryRow(t.Context(), `SELECT status FROM proposal_step WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("step status: %v", err)
	}
	return s
}

func countRows(t *testing.T, h *harness, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// The sample brief of the case, start to finish (D-070): screen allow, audience of 46 (20/13/13/3),
// plan primary_fallback with email as primary (33 email, 13 sms), content only for the
// (language, channel) pairs that have recipients, one schedule per language, and the Confirmer
// once at the end. Every model call has its llm_call row.
func TestSampleBriefEndToEnd(t *testing.T) {
	h := newHarness(t)
	prev := h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	if prev.Count != 46 || prev.Coverage != (customer.Coverage{Both: 20, EmailOnly: 13, SmsOnly: 13, Unreachable: 3}) {
		t.Fatalf("the seed changed: the sample audience is %d with %+v, want 46 with 20/13/13/3", prev.Count, prev.Coverage)
	}
	langs := langsOf(prev)
	h.llm.ProposeSchedules(h.scheduleFor(langs...))

	camp, sub := h.startCampaign(sampleBrief)
	h.idle()

	// 1. Audience.
	v := h.get(camp.ID)
	if v.Status != flow.CampaignStatusDrafting || v.Brief != sampleBrief || len(v.Steps) != 1 {
		t.Fatalf("after screening: %+v", v.CampaignInfo)
	}
	audStep := v.Steps[0]
	if audStep.Kind != flow.KindAudience || audStep.Status != flow.StepStatusOpen || audStep.Version != 1 || audStep.Generating || audStep.Failed || len(audStep.Missing) != 0 || len(audStep.Variants) != 1 {
		t.Fatalf("audience step = %+v", audStep)
	}
	var aud flow.AudienceContent
	decodeInto(t, audStep.Variants[0].Content, &aud)
	if audStep.Variants[0].Status != flow.VariantStatusDraft || aud.DSL != sampleDSL || aud.Count != 46 ||
		aud.Coverage != (flow.Coverage{Both: 20, EmailOnly: 13, SMSOnly: 13, Unreachable: 3}) || len(aud.ByLanguage) != len(langs) ||
		!strings.HasPrefix(string(aud.Filter), "{") {
		t.Fatalf("audience content = %+v", aud)
	}
	if got := h.llm.ScreenBriefs(); len(got) != 1 || got[0] != sampleBrief {
		t.Errorf("screened %q", got)
	}
	h.mustApprove(audStep.Variants[0])

	// 2. Channel plan: generated from the approved audience's coverage table, counted in code.
	h.idle()
	planStep := h.step(camp.ID, flow.KindChannelPlan)
	if audStep = h.step(camp.ID, flow.KindAudience); audStep.Status != flow.StepStatusApproved || audStep.ApprovedAt == nil {
		t.Errorf("audience step after its only variant was approved = %+v", audStep.StepInfo)
	}
	if planStep.Status != flow.StepStatusOpen || len(planStep.Variants) != 1 {
		t.Fatalf("plan step = %+v", planStep)
	}
	var plan flow.PlanContent
	decodeInto(t, planStep.Variants[0].Content, &plan)
	if plan.Mode != llm.ModePrimaryFallback || plan.Primary != llm.ChannelEmail || plan.Recipients != (flow.Recipients{Email: 33, SMS: 13, Total: 46}) {
		t.Fatalf("plan content = %+v, want primary_fallback email with 33 email / 13 sms", plan)
	}
	in := h.llm.ChannelPlanInputs()
	if len(in) != 1 || in[0].Brief != sampleBrief || in[0].Coverage.Count != 46 || in[0].Coverage.Coverage.Both != 20 {
		t.Errorf("plan input = %+v", in)
	}
	h.mustApprove(planStep.Variants[0])

	// 3. Content: one variant per (language, channel) pair that has recipients (D-010).
	h.idle()
	contentStep := h.step(camp.ID, flow.KindContent)
	want := map[string]bool{}
	for _, l := range plan.ByLanguage {
		if l.Email > 0 {
			want[l.Language+"/email"] = true
		}
		if l.SMS > 0 {
			want[l.Language+"/sms"] = true
		}
	}
	got := map[string]bool{}
	for _, cv := range contentStep.Variants {
		got[slotKey(cv)] = true
	}
	if len(want) == 0 || len(got) != len(want) || len(contentStep.Variants) != len(want) {
		t.Fatalf("content slots = %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("no content variant for %s", k)
		}
	}
	for _, r := range h.content.Requests() {
		if r.Recipients <= 0 || r.Brief != sampleBrief || r.Plan.Mode != llm.ModePrimaryFallback || r.Audience.Count != 46 || r.RejectionReason != "" || r.StepID != contentStep.ID {
			t.Errorf("content request = %+v", r)
		}
	}
	// Approving all but the last variant leaves the step open; the last one closes it.
	for _, cv := range contentStep.Variants[:len(contentStep.Variants)-1] {
		h.mustApprove(cv)
	}
	if s := h.step(camp.ID, flow.KindContent); s.Status != flow.StepStatusOpen {
		t.Fatalf("content step is %s before its last variant was approved", s.Status)
	}
	if countStepsOf(h, camp.ID, flow.KindSchedule) != 0 {
		t.Fatal("the schedule step opened before every content variant was approved")
	}
	h.mustApprove(contentStep.Variants[len(contentStep.Variants)-1])

	// 4. Schedule: one variant per language.
	h.idle()
	schedStep := h.step(camp.ID, flow.KindSchedule)
	if len(schedStep.Variants) != len(langs) {
		t.Fatalf("schedule variants = %d, want one per language %v", len(schedStep.Variants), langs)
	}
	if in := h.llm.ScheduleInputs(); len(in) != 1 || strings.Join(in[0].Languages, ",") != strings.Join(langs, ",") || in[0].Brief != sampleBrief || !in[0].Now.Equal(h.clock.Now()) {
		t.Errorf("schedule input = %+v", in)
	}
	var withTime int
	for _, sv := range schedStep.Variants {
		var c flow.ScheduleContent
		decodeInto(t, sv.Content, &c)
		if sv.Language == "" || sv.Channel != "" {
			t.Errorf("schedule variant slot = %q/%q", sv.Language, sv.Channel)
		}
		if c.SendAt != nil {
			withTime++
		}
	}
	if withTime != 1 {
		t.Errorf("%d schedule variants carry a time, want 1", withTime)
	}
	for i, sv := range schedStep.Variants {
		if calls := h.confirmer.Calls(); len(calls) != 0 {
			t.Fatalf("Confirmer called early (%d calls before variant %d)", len(calls), i)
		}
		h.mustApprove(sv)
	}
	if calls := h.confirmer.Calls(); len(calls) != 1 || calls[0] != camp.ID {
		t.Errorf("Confirmer calls = %v, want exactly one for the campaign", calls)
	}
	final := h.get(camp.ID)
	if final.Status != flow.CampaignStatusDrafting || len(final.Steps) != 4 {
		t.Fatalf("final campaign = %+v with %d steps", final.CampaignInfo, len(final.Steps))
	}
	for _, s := range final.Steps {
		if s.Status != flow.StepStatusApproved || s.ApprovedAt == nil || s.Generating || s.Failed || len(s.Missing) != 0 {
			t.Errorf("step %s = %+v", s.Kind, s)
		}
	}

	// 5. llm_call: one row per model call, with its purpose, step and accepted variant.
	rows, err := h.q.ListLLMCalls(t.Context(), camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	byPurpose := map[string][]store.LlmCall{}
	for _, r := range rows {
		byPurpose[r.Purpose] = append(byPurpose[r.Purpose], r)
	}
	if len(byPurpose["screen"]) != 1 || len(byPurpose["segment"]) != 1 || len(byPurpose["channel_plan"]) != 1 ||
		len(byPurpose["content"]) != len(want) || len(byPurpose["schedule"]) != 1 || len(rows) != 4+len(want) {
		t.Fatalf("llm_call rows by purpose: screen %d, segment %d, channel_plan %d, content %d (want %d), schedule %d",
			len(byPurpose["screen"]), len(byPurpose["segment"]), len(byPurpose["channel_plan"]), len(byPurpose["content"]), len(want), len(byPurpose["schedule"]))
	}
	if r := byPurpose["screen"][0]; r.StepID != nil || r.VariantID != nil {
		t.Errorf("screen row = %+v: it belongs to no step", r)
	}
	for kind, purpose := range map[string]string{flow.KindAudience: "segment", flow.KindChannelPlan: "channel_plan"} {
		s := stepOf(final, kind)
		r := byPurpose[purpose][0]
		if r.StepID == nil || *r.StepID != s.ID || r.VariantID == nil || *r.VariantID != s.Variants[0].ID || r.Outcome != "accepted" {
			t.Errorf("%s row = step %v variant %v outcome %s, want step %s and its variant %s", purpose, r.StepID, r.VariantID, r.Outcome, s.ID, s.Variants[0].ID)
		}
	}
	cs := stepOf(final, flow.KindContent)
	ids := map[uuid.UUID]bool{}
	for _, r := range byPurpose["content"] {
		if r.StepID == nil || *r.StepID != cs.ID || r.VariantID == nil {
			t.Errorf("content row = %+v", r)
			continue
		}
		ids[*r.VariantID] = true
	}
	for _, cv := range cs.Variants {
		if !ids[cv.ID] {
			t.Errorf("content variant %s has no llm_call row", slotKey(cv))
		}
	}
	// The schedule call produced one variant per language: its row names none of them.
	if len(langs) > 1 {
		for _, r := range byPurpose["schedule"] {
			if r.VariantID != nil {
				t.Errorf("a schedule call that produced %d variants names one of them: %+v", len(langs), r)
			}
		}
	}

	// 6. Events.
	evs := events(sub)
	if sub.Dropped() != 0 {
		t.Errorf("subscriber dropped %d events", sub.Dropped())
	}
	types := typesOf(evs)
	if len(types) == 0 || types[0] != flow.TypeScreenResult {
		t.Fatalf("events = %v, want screen_result first", types)
	}
	if sr := evs[0].(flow.ScreenResult); sr.Verdict != "allow" {
		t.Errorf("screen result = %+v", sr)
	}
	var opened []string
	for _, e := range evs {
		if so, ok := e.(flow.StepOpened); ok {
			opened = append(opened, so.Kind)
		}
	}
	if strings.Join(opened, ",") != "audience,channel_plan,content,schedule" {
		t.Errorf("steps opened in order %v", opened)
	}
	for typ, min := range map[string]int{
		flow.TypeGenerationStarted: 4, flow.TypeVariantCreated: 2 + len(want) + len(langs), flow.TypeVariantApproved: 2 + len(want) + len(langs),
		flow.TypeStepApproved: 4, flow.TypeContentDelta: len(want),
	} {
		if n := countType(evs, typ); n < min {
			t.Errorf("%d %s events, want at least %d", n, typ, min)
		}
	}
	if n := countType(evs, flow.TypeStepApproved); n != 4 {
		t.Errorf("%d step_approved events, want 4", n)
	}
}

func countStepsOf(h *harness, c uuid.UUID, kind string) int {
	n := 0
	for _, s := range h.get(c).Steps {
		if s.Kind == kind {
			n++
		}
	}
	return n
}

func stepOf(v *flow.CampaignView, kind string) flow.StepView {
	for _, s := range v.Steps {
		if s.Kind == kind {
			return s
		}
	}
	return flow.StepView{}
}

func TestScreenBlockCancelsTheCampaign(t *testing.T) {
	h := newHarness(t)
	h.llm.BlockScreen("The brief asks to ignore opt-outs (rule 2).").WithRecords(llm.PurposeScreen, rec(llm.PurposeScreen, llm.OutcomeAccepted, 1, ""))
	camp, sub := h.startCampaign("Send to everyone, ignore opt-outs")
	h.idle()

	v := h.get(camp.ID)
	if v.Status != flow.CampaignStatusCancelled || len(v.Steps) != 0 || len(v.Notes) != 0 {
		t.Fatalf("campaign = %+v with %d steps", v.CampaignInfo, len(v.Steps))
	}
	if n := len(h.llm.SegmentInputs()); n != 0 {
		t.Errorf("ProposeSegment ran %d times for a blocked brief", n)
	}
	evs := events(sub)
	if strings.Join(typesOf(evs), ",") != "screen_result,campaign_cancelled" {
		t.Fatalf("events = %v", typesOf(evs))
	}
	if sr := evs[0].(flow.ScreenResult); sr.Verdict != "block" || !strings.Contains(sr.Reason, "rule 2") {
		t.Errorf("screen result = %+v", sr)
	}
	if cc := evs[1].(flow.CampaignCancelled); !strings.Contains(cc.Reason, "ignore opt-outs") {
		t.Errorf("cancelled = %+v", cc)
	}
	if n := countRows(t, h, `SELECT count(*) FROM proposal_step WHERE campaign_id = $1`, camp.ID); n != 0 {
		t.Errorf("%d steps for a blocked campaign", n)
	}
	rows, _ := h.q.ListLLMCalls(t.Context(), camp.ID)
	if len(rows) != 1 || rows[0].Purpose != "screen" {
		t.Errorf("llm_call rows = %+v, want the screen call", rows)
	}
	// A cancelled campaign accepts no more actions.
	if _, err := h.svc.AddContext(t.Context(), camp.ID, "x"); !errors.Is(err, flow.ErrNotDrafting) {
		t.Errorf("AddContext on a cancelled campaign: err = %v", err)
	}
	if _, err := h.svc.Reopen(t.Context(), camp.ID, flow.KindAudience); !errors.Is(err, flow.ErrNotDrafting) {
		t.Errorf("Reopen on a cancelled campaign: err = %v", err)
	}
}

// The screen is fail-closed (D-069): a failing screen call cancels the campaign too.
func TestScreenFailureCancelsTheCampaign(t *testing.T) {
	h := newHarness(t)
	h.llm.FailScreen(&llm.APIError{Purpose: llm.PurposeScreen, Status: 529, Type: "overloaded_error"})
	camp, sub := h.startCampaign(sampleBrief)
	h.idle()
	v := h.get(camp.ID)
	if v.Status != flow.CampaignStatusCancelled || len(v.Steps) != 0 {
		t.Fatalf("campaign = %+v with %d steps", v.CampaignInfo, len(v.Steps))
	}
	evs := events(sub)
	if len(evs) != 2 {
		t.Fatalf("events = %v", typesOf(evs))
	}
	if sr := evs[0].(flow.ScreenResult); sr.Verdict != "error" || !strings.Contains(sr.Reason, "could not be screened") || !strings.Contains(sr.Reason, "529") {
		t.Errorf("screen result = %+v", sr)
	}
}

func TestRejectRegeneratesOnlyThatSlot(t *testing.T) {
	h := newHarness(t)
	camp, sub, _ := h.toContent(llm.ModePrimaryFallback, llm.ChannelEmail)
	step := h.step(camp.ID, flow.KindContent)
	if len(step.Variants) < 2 {
		t.Fatalf("need at least two content slots, got %d", len(step.Variants))
	}
	target := step.Variants[0]
	others := map[uuid.UUID]string{}
	for _, v := range step.Variants[1:] {
		others[v.ID] = slotKey(v)
	}
	before := len(h.content.Requests())
	events(sub) // drop the history

	if err := h.svc.Reject(t.Context(), target.ID, "çok resmi, daha samimi olsun"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	h.idle()

	reqs := h.content.Requests()
	if len(reqs) != before+1 {
		t.Fatalf("%d generator calls after the rejection, want exactly one more (%d before)", len(reqs), before)
	}
	last := reqs[len(reqs)-1]
	if last.Language != target.Language || last.Channel != target.Channel || last.RejectionReason != "çok resmi, daha samimi olsun" {
		t.Errorf("replacement request = %+v, want the rejected slot with the reason", last)
	}
	if got := variantStatus(t, h, target.ID); got != flow.VariantStatusSuperseded {
		t.Errorf("rejected variant is %s, want superseded once its replacement exists", got)
	}
	var reason string
	if err := h.pool.QueryRow(t.Context(), `SELECT rejection_reason FROM proposal_variant WHERE id = $1`, target.ID).Scan(&reason); err != nil || reason != "çok resmi, daha samimi olsun" {
		t.Errorf("the superseded variant lost its rejection reason: %q, %v", reason, err)
	}
	after := h.step(camp.ID, flow.KindContent)
	if len(after.Variants) != len(step.Variants) {
		t.Fatalf("%d live variants, want %d", len(after.Variants), len(step.Variants))
	}
	var replaced int
	for _, v := range after.Variants {
		if slot, ok := others[v.ID]; ok {
			if slotKey(v) != slot || v.Status != flow.VariantStatusDraft {
				t.Errorf("untouched variant %s changed: %+v", slot, v)
			}
			continue
		}
		replaced++
		if slotKey(v) != slotKey(target) || v.Status != flow.VariantStatusDraft || v.ID == target.ID || !strings.HasSuffix(string(v.Content), `#2"}`) {
			t.Errorf("replacement = %+v", v)
		}
	}
	if replaced != 1 {
		t.Errorf("%d replacements, want 1", replaced)
	}
	evs := events(sub)
	if countType(evs, flow.TypeVariantRejected) != 1 || countType(evs, flow.TypeVariantSuperseded) != 1 || countType(evs, flow.TypeVariantCreated) != 1 || countType(evs, flow.TypeGenerationStarted) != 1 {
		t.Errorf("events = %v", typesOf(evs))
	}
	// The rejected variant had no say any more: the step is not approved by approving the others.
	for _, v := range after.Variants {
		h.mustApprove(v)
	}
	if h.step(camp.ID, flow.KindContent).Status != flow.StepStatusApproved {
		t.Error("content step did not close after every slot was approved")
	}
	// Errors.
	if err := h.svc.Reject(t.Context(), target.ID, "again"); !errors.Is(err, flow.ErrNotOpen) {
		t.Errorf("Reject on a superseded variant of an approved step: err = %v", err)
	}
	if err := h.svc.Reject(t.Context(), uuid.New(), "x"); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Reject(unknown) err = %v", err)
	}
}

func TestAddContextRegeneratesUnapprovedAndKeepsApproved(t *testing.T) {
	h := newHarness(t)
	camp, _, prev := h.toContent(llm.ModePrimaryFallback, llm.ChannelEmail)
	h.llm.ProposeSchedules(h.scheduleFor(langsOf(prev)...))
	step := h.step(camp.ID, flow.KindContent)
	if len(step.Variants) < 3 {
		t.Fatalf("need at least three content slots, got %d", len(step.Variants))
	}
	kept := step.Variants[0]
	h.mustApprove(kept)
	rejected := step.Variants[1]
	if err := h.svc.Reject(t.Context(), rejected.ID, "no emoji please"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	before := len(h.content.Requests())
	h.step(camp.ID, flow.KindContent) // snapshot after the rejection's regeneration
	draftsBefore := map[uuid.UUID]bool{}
	for _, v := range h.step(camp.ID, flow.KindContent).Variants {
		if v.Status == flow.VariantStatusDraft {
			draftsBefore[v.ID] = true
		}
	}

	note, err := h.svc.AddContext(t.Context(), camp.ID, "  mention the free shipping  ")
	if err != nil {
		t.Fatalf("AddContext: %v", err)
	}
	if note.Body != "mention the free shipping" || note.StepID == nil || *note.StepID != step.ID {
		t.Errorf("note = %+v, want the trimmed body on the open step", note)
	}
	h.idle()

	after := h.step(camp.ID, flow.KindContent)
	if len(after.Variants) != len(step.Variants) {
		t.Fatalf("%d live variants, want %d", len(after.Variants), len(step.Variants))
	}
	reqs := h.content.Requests()
	if len(reqs) != before+len(draftsBefore) {
		t.Errorf("%d new generator calls, want one per unapproved slot (%d)", len(reqs)-before, len(draftsBefore))
	}
	for _, r := range reqs[before:] {
		if !contains(r.ContextNotes, "mention the free shipping") {
			t.Errorf("a regenerated slot did not see the note: %q", r.ContextNotes)
		}
	}
	for _, v := range after.Variants {
		switch {
		case v.ID == kept.ID:
			if v.Status != flow.VariantStatusApproved {
				t.Errorf("the approved variant is %s", v.Status)
			}
		case draftsBefore[v.ID]:
			t.Errorf("draft %s was not replaced", slotKey(v))
		}
	}
	for id := range draftsBefore {
		if got := variantStatus(t, h, id); got != flow.VariantStatusSuperseded {
			t.Errorf("old draft is %s, want superseded", got)
		}
	}
	if got := variantStatus(t, h, rejected.ID); got != flow.VariantStatusSuperseded {
		t.Errorf("rejected variant is %s, want superseded", got)
	}
	if v := h.get(camp.ID); len(v.Notes) != 1 || v.Notes[0].StepID == nil || *v.Notes[0].StepID != step.ID {
		t.Errorf("notes = %+v", v.Notes)
	}

	// Later steps see the note too.
	h.approveAll(camp.ID, flow.KindContent)
	h.approveAll(camp.ID, flow.KindSchedule)
	in := h.llm.ScheduleInputs()
	if len(in) != 1 || !contains(in[0].ContextNotes, "mention the free shipping") {
		t.Errorf("schedule input notes = %+v", in)
	}
	if h.step(camp.ID, flow.KindSchedule).Status != flow.StepStatusApproved {
		t.Error("the campaign did not reach the end")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s || strings.Contains(x, s) {
			return true
		}
	}
	return false
}

func TestEditAudienceRecompilesAndRecounts(t *testing.T) {
	h := newHarness(t)
	h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	camp, sub := h.startCampaign(sampleBrief)
	h.idle()
	orig := h.step(camp.ID, flow.KindAudience).Variants[0]
	events(sub)

	// An invalid edit changes nothing and says what is wrong.
	for name, body := range map[string]string{
		"syntax":         `{"dsl": "city ==="}`,
		"unknown field":  `{"dsl": "citty === \"Ankara\""}`,
		"empty audience": `{"dsl": "loyaltyTier === \"Platinum\" && ageYears < 10"}`,
		"no dsl":         `{"dsl": "  "}`,
		"not an object":  `"city"`,
	} {
		_, err := h.svc.Edit(t.Context(), orig.ID, json.RawMessage(body))
		var ice *flow.InvalidContentError
		if !errors.Is(err, flow.ErrInvalidContent) || !errors.As(err, &ice) || len(ice.Problems) == 0 {
			t.Errorf("%s: err = %v, want ErrInvalidContent with problems", name, err)
			continue
		}
		if name == "empty audience" && !strings.Contains(ice.Problems[0], string(llm.CodeEmptyAudience)) {
			t.Errorf("empty audience problems = %q", ice.Problems)
		}
		if name == "unknown field" && !strings.Contains(strings.Join(ice.Problems, " "), "citty") {
			t.Errorf("unknown field problems = %q", ice.Problems)
		}
	}
	live := h.step(camp.ID, flow.KindAudience).Variants
	if len(live) != 1 || live[0].ID != orig.ID || live[0].Status != flow.VariantStatusDraft || len(events(sub)) != 0 {
		t.Fatalf("an invalid edit changed the step: %+v", live)
	}

	// A valid edit is recompiled and recounted by the real Customer service.
	const edited = `city === "Ankara"`
	want := h.preview(edited)
	nv, err := h.svc.Edit(t.Context(), orig.ID, json.RawMessage(`{"dsl": "`+strings.ReplaceAll(edited, `"`, `\"`)+`"}`))
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	var aud flow.AudienceContent
	decodeInto(t, nv.Content, &aud)
	if nv.ID == orig.ID || nv.Status != flow.VariantStatusDraft || aud.DSL != edited || aud.Count != want.Count || aud.Count == 46 ||
		aud.Coverage.Both != want.Coverage.Both || !strings.HasPrefix(string(aud.Filter), "{") {
		t.Fatalf("edited variant = %+v content %+v (preview %+v)", nv, aud, want)
	}
	if got := variantStatus(t, h, orig.ID); got != flow.VariantStatusSuperseded {
		t.Errorf("old variant is %s, want superseded", got)
	}
	cur := h.step(camp.ID, flow.KindAudience)
	if len(cur.Variants) != 1 || cur.Variants[0].ID != nv.ID || cur.Status != flow.StepStatusOpen {
		t.Errorf("step after the edit = %+v", cur)
	}
	evs := events(sub)
	if strings.Join(typesOf(evs), ",") != "variant_superseded,variant_created" {
		t.Errorf("events = %v", typesOf(evs))
	}
	// The old one cannot be edited, approved or rejected any more.
	if _, err := h.svc.Edit(t.Context(), orig.ID, json.RawMessage(`{"dsl":"city === \"Ankara\""}`)); !errors.Is(err, flow.ErrNotDraft) {
		t.Errorf("Edit(superseded) err = %v", err)
	}
	if err := h.svc.Approve(t.Context(), orig.ID); !errors.Is(err, flow.ErrNotDraft) {
		t.Errorf("Approve(superseded) err = %v", err)
	}
	// The plan is generated from the edited audience.
	h.mustApprove(nv)
	h.idle()
	in := h.llm.ChannelPlanInputs()
	if len(in) != 1 || in[0].Coverage.Count != want.Count {
		t.Errorf("plan input = %+v, want the coverage of the edited audience (%d)", in, want.Count)
	}
}

func TestEditPlanContentAndSchedule(t *testing.T) {
	h := newHarness(t)
	h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)

	// Plan: a plan that sends to nobody is refused; a valid one is recounted.
	plan := h.step(camp.ID, flow.KindChannelPlan).Variants[0]
	for name, body := range map[string]string{
		"unknown mode":    `{"mode":"all","primary":"sms"}`,
		"unknown channel": `{"mode":"both","primary":"push"}`,
		"no fields":       `{}`,
	} {
		if _, err := h.svc.Edit(t.Context(), plan.ID, json.RawMessage(body)); !errors.Is(err, flow.ErrInvalidContent) {
			t.Errorf("plan edit %s: err = %v", name, err)
		}
	}
	nv, err := h.svc.Edit(t.Context(), plan.ID, json.RawMessage(`{"mode":"both","primary":"email","rationale":"the marketer wants both"}`))
	if err != nil {
		t.Fatalf("Edit plan: %v", err)
	}
	var pc flow.PlanContent
	decodeInto(t, nv.Content, &pc)
	if pc.Mode != llm.ModeBoth || pc.Rationale != "the marketer wants both" || pc.Recipients != (flow.Recipients{Email: 33, SMS: 33, Total: 66}) {
		t.Errorf("edited plan = %+v, want both with 33 + 33 messages", pc)
	}
	h.mustApprove(nv)
	h.idle()

	// Content: the generator's validator decides.
	cstep := h.step(camp.ID, flow.KindContent)
	var email, sms int
	for _, v := range cstep.Variants {
		if v.Channel == "email" {
			email++
		} else {
			sms++
		}
	}
	if email == 0 || email != sms {
		t.Fatalf("mode both gave %d email and %d sms variants, want the same languages on both", email, sms)
	}
	cv := cstep.Variants[0]
	_, err = h.svc.Edit(t.Context(), cv.ID, json.RawMessage(`{"text":""}`))
	var ice *flow.InvalidContentError
	if !errors.As(err, &ice) || !strings.Contains(strings.Join(ice.Problems, ""), "non-empty") {
		t.Errorf("content edit with empty text: err = %v", err)
	}
	ncv, err := h.svc.Edit(t.Context(), cv.ID, json.RawMessage(`{"text":"Merhaba {{first_name}}, kendi metnim."}`))
	if err != nil || ncv.Language != cv.Language || ncv.Channel != cv.Channel || !strings.Contains(string(ncv.Content), "kendi metnim") {
		t.Fatalf("Edit content = %+v, %v", ncv, err)
	}
	if variantStatus(t, h, cv.ID) != flow.VariantStatusSuperseded {
		t.Error("the old content variant was not superseded")
	}
	if got := len(h.content.Requests()); got != len(cstep.Variants) {
		t.Errorf("an edit called the generator: %d requests for %d slots", got, len(cstep.Variants))
	}
	h.approveAll(camp.ID, flow.KindContent)
	if h.step(camp.ID, flow.KindContent).Status != flow.StepStatusApproved {
		t.Fatal("content step did not close")
	}
}

func TestEditScheduleValidatesTheWindow(t *testing.T) {
	h := newHarness(t)
	prev := h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	h.llm.ProposeSchedules(h.scheduleFor(langsOf(prev)...))
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	h.approveAll(camp.ID, flow.KindChannelPlan)
	h.approveAll(camp.ID, flow.KindContent)
	sv := h.step(camp.ID, flow.KindSchedule).Variants[0]
	now := h.clock.Now()
	at := func(d time.Duration) string { return now.Add(d).In(time.FixedZone("+03", 3*3600)).Format(time.RFC3339) }

	for name, body := range map[string]string{
		"in the past":        `{"send_at":"` + at(-time.Hour) + `"}`,
		"too soon":           `{"send_at":"` + at(4*time.Minute) + `"}`,
		"too far":            `{"send_at":"` + at(31*24*time.Hour) + `"}`,
		"no offset":          `{"send_at":"2026-11-02T10:00:00"}`,
		"not a time":         `{"send_at":"tomorrow"}`,
		"send_at is missing": `{"rationale":"x"}`,
	} {
		if _, err := h.svc.Edit(t.Context(), sv.ID, json.RawMessage(body)); !errors.Is(err, flow.ErrInvalidContent) {
			t.Errorf("%s: err = %v, want ErrInvalidContent", name, err)
		}
	}
	cur := sv
	for _, c := range []struct{ name, body string }{
		{"a good time", `{"send_at":"` + at(48*time.Hour) + `","rationale":"Friday evening"}`},
		{"send now", `{"send_at":null}`},
		{"the earliest", `{"send_at":"` + at(5*time.Minute) + `"}`},
	} {
		nv, err := h.svc.Edit(t.Context(), cur.ID, json.RawMessage(c.body))
		if err != nil || nv.Language != sv.Language || nv.Channel != "" || nv.ID == cur.ID {
			t.Errorf("%s: %+v, %v", c.name, nv, err)
			continue
		}
		cur = nv
	}
}

func TestReopenSupersedesTheStepAndEveryLaterOne(t *testing.T) {
	h := newHarness(t)
	camp, sub, _ := h.toContent(llm.ModePrimaryFallback, llm.ChannelEmail)
	// The second round: audience again, then a plan.
	h.llm.ProposeDSLs(sampleDSL).ProposePlans(llm.ChannelPlan{Mode: llm.ModeSingle, Primary: llm.ChannelSMS, Rationale: "second round"})
	v := h.get(camp.ID)
	var oldSteps []uuid.UUID
	for _, s := range v.Steps {
		oldSteps = append(oldSteps, s.ID)
	}
	if len(oldSteps) != 3 {
		t.Fatalf("expected audience, plan and content steps, got %d", len(oldSteps))
	}
	events(sub)

	// Only an approved step can be reopened; a kind with no step is not found.
	if _, err := h.svc.Reopen(t.Context(), camp.ID, flow.KindContent); !errors.Is(err, flow.ErrNotApproved) {
		t.Errorf("Reopen(open step) err = %v, want ErrNotApproved", err)
	}
	if _, err := h.svc.Reopen(t.Context(), camp.ID, flow.KindSchedule); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Reopen(no step) err = %v, want ErrNotFound", err)
	}

	step, err := h.svc.Reopen(t.Context(), camp.ID, flow.KindAudience)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if step.Kind != flow.KindAudience || step.Version != 2 || step.Status != flow.StepStatusOpen {
		t.Errorf("reopened step = %+v, want audience version 2, open", step)
	}
	h.idle()

	cur := h.get(camp.ID)
	if len(cur.Steps) != 1 || cur.Steps[0].ID != step.ID || cur.Steps[0].Version != 2 || len(cur.Steps[0].Variants) != 1 || cur.Steps[0].Variants[0].Status != flow.VariantStatusDraft {
		t.Fatalf("steps after the reopen = %+v", cur.Steps)
	}
	for _, id := range oldSteps {
		if got := stepStatus(t, h, id); got != flow.StepStatusSuperseded {
			t.Errorf("old step %s is %s, want superseded", id, got)
		}
		if n := countRows(t, h, `SELECT count(*) FROM proposal_variant WHERE step_id = $1 AND status <> 'superseded'`, id); n != 0 {
			t.Errorf("old step %s keeps %d live variants", id, n)
		}
	}
	// The old audience variant was approved and stays as history with its stamp.
	if n := countRows(t, h, `SELECT count(*) FROM proposal_variant WHERE step_id = $1 AND status = 'superseded' AND approved_at IS NOT NULL`, oldSteps[0]); n != 1 {
		t.Errorf("%d superseded audience variants kept their approval stamp, want 1", n)
	}
	evs := events(sub)
	if countType(evs, flow.TypeStepSuperseded) != 3 || countType(evs, flow.TypeStepOpened) != 1 {
		t.Errorf("events = %v", typesOf(evs))
	}
	// Going on creates version 2 of the plan.
	h.approveAll(camp.ID, flow.KindAudience)
	if p := h.step(camp.ID, flow.KindChannelPlan); p.Version != 2 || p.Status != flow.StepStatusOpen || len(p.Variants) != 1 {
		t.Errorf("plan after the reopen = %+v", p)
	}

	// Reopening the plan keeps the audience.
	h.approveAll(camp.ID, flow.KindChannelPlan)
	h.llm.ProposePlans(llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelSMS, Rationale: "third"})
	aud := h.step(camp.ID, flow.KindAudience)
	ps, err := h.svc.Reopen(t.Context(), camp.ID, flow.KindChannelPlan)
	if err != nil || ps.Version != 3 {
		t.Fatalf("Reopen(plan) = %+v, %v", ps, err)
	}
	h.idle()
	cur = h.get(camp.ID)
	if got := stepOf(cur, flow.KindAudience); got.ID != aud.ID || got.Status != flow.StepStatusApproved {
		t.Errorf("the audience was touched by reopening the plan: %+v", got)
	}
	if stepOf(cur, flow.KindContent).ID != uuid.Nil {
		t.Error("the content step survived the reopen of the plan")
	}
}

func TestSegmentRepairExhaustedIsReportedAndRegenerateRecovers(t *testing.T) {
	h := newHarness(t)
	rec3 := []llm.CallRecord{
		rec(llm.PurposeSegment, llm.OutcomeValidationFailed, 1, "propose_segment was rejected"),
		rec(llm.PurposeSegment, llm.OutcomeValidationFailed, 2, "propose_segment was rejected"),
		rec(llm.PurposeSegment, llm.OutcomeValidationFailed, 3, "propose_segment was rejected"),
	}
	h.llm.AllowScreen().WithRecords(llm.PurposeScreen, rec(llm.PurposeScreen, llm.OutcomeAccepted, 1, ""))
	h.llm.ProposeDSLs(`citty === 1`, `city ===`, `x`).WithRecords(llm.PurposeSegment, rec3...)
	camp, sub := h.startCampaign(sampleBrief)
	h.idle()

	step := h.step(camp.ID, flow.KindAudience)
	if !step.Failed || step.Generating || len(step.Variants) != 0 || len(step.Missing) != 1 || !strings.Contains(step.Reason, "no valid segment after 3 attempts") {
		t.Fatalf("audience step = %+v, want a failed step with the repair-exhausted reason", step)
	}
	evs := events(sub)
	var failed flow.GenerationFailed
	for _, e := range evs {
		if f, ok := e.(flow.GenerationFailed); ok {
			failed = f
		}
	}
	if failed.StepID != step.ID || failed.Kind != flow.KindAudience || !strings.Contains(failed.Reason, "no valid segment") {
		t.Errorf("GenerationFailed = %+v", failed)
	}
	rows, _ := h.q.ListLLMCalls(t.Context(), camp.ID)
	if len(rows) != 4 { // the screen and three failed turns
		t.Errorf("%d llm_call rows, want 4", len(rows))
	}

	// After a "restart" the reason comes from the llm_call rows alone.
	svc2, err := flow.New(flow.Deps{Pool: h.pool, LLM: h.llm, Tools: h.tools, Logger: discard()})
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close(t.Context())
	v2, err := svc2.Get(t.Context(), camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s := stepOf(v2, flow.KindAudience); !s.Failed || !strings.Contains(s.Reason, "segment: validation_failed: propose_segment was rejected") {
		t.Errorf("after a restart: %+v", s)
	}

	// Regenerate retries the step and succeeds.
	h.llm.ProposeDSLs(sampleDSL).WithRecords(llm.PurposeSegment, rec(llm.PurposeSegment, llm.OutcomeAccepted, 1, ""))
	if err := h.svc.Regenerate(t.Context(), step.ID); err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	h.idle()
	step = h.step(camp.ID, flow.KindAudience)
	if step.Failed || len(step.Variants) != 1 || step.Variants[0].Status != flow.VariantStatusDraft || len(step.Missing) != 0 {
		t.Errorf("audience step after Regenerate = %+v", step)
	}
	if err := h.svc.Regenerate(t.Context(), uuid.New()); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Regenerate(unknown) err = %v", err)
	}
	h.mustApprove(step.Variants[0])
	if err := h.svc.Regenerate(t.Context(), step.ID); !errors.Is(err, flow.ErrNotOpen) {
		t.Errorf("Regenerate(approved step) err = %v", err)
	}
}

// A refusal stops the step without a repair, and the reason says so.
func TestRefusalFailsThePlanStepAndRegenerateRecovers(t *testing.T) {
	h := newHarness(t)
	h.llm.AllowScreen().ProposeDSLs(sampleDSL)
	h.llm.FailChannelPlan(&llm.RefusalError{Purpose: llm.PurposeChannelPlan, Category: "general_harms"}).
		WithRecords(llm.PurposeChannelPlan, llm.CallRecord{
			Purpose: llm.PurposeChannelPlan, Model: "scripted", Attempt: 1, Outcome: llm.OutcomeRefusal, RefusalCategory: "general_harms",
			Prompt: rec(llm.PurposeChannelPlan, llm.OutcomeRefusal, 1, "").Prompt,
		})
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	plan := h.step(camp.ID, flow.KindChannelPlan)
	if !plan.Failed || !strings.Contains(plan.Reason, "refused") {
		t.Fatalf("plan step = %+v", plan)
	}
	h.llm.ProposePlans(llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "ok"})
	if err := h.svc.Regenerate(t.Context(), plan.ID); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if p := h.step(camp.ID, flow.KindChannelPlan); p.Failed || len(p.Variants) != 1 {
		t.Errorf("plan step after Regenerate = %+v", p)
	}
}

// The plan the model proposes is validated in code against the coverage: an invalid plan is
// repaired in the same conversation, and the repair loop is the fake's here.
func TestInvalidPlanIsRepairedAgainstTheCoverage(t *testing.T) {
	h := newHarness(t)
	h.llm.AllowScreen().ProposeDSLs(sampleDSL)
	h.llm.ProposePlans(
		llm.ChannelPlan{Mode: llm.ModeSingle, Primary: "push", Rationale: "bad channel"},
		llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "good"},
	)
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	plan := h.step(camp.ID, flow.KindChannelPlan)
	if plan.Failed || len(plan.Variants) != 1 || !strings.Contains(string(plan.Variants[0].Content), `"good"`) {
		t.Fatalf("plan step = %+v", plan)
	}
	rows, _ := h.q.ListLLMCalls(t.Context(), camp.ID)
	var outcomes []string
	for _, r := range rows {
		if r.Purpose == "channel_plan" {
			outcomes = append(outcomes, r.Outcome)
		}
	}
	if strings.Join(outcomes, ",") != "validation_failed,accepted" {
		t.Errorf("channel_plan outcomes = %v", outcomes)
	}
}

func TestContentFailureOfOneSlotLeavesTheOthers(t *testing.T) {
	h := newHarness(t)
	var once sync.Once
	var failedSlot string
	h.content.GenerateFunc = func(_ context.Context, req flow.ContentRequest, onDelta func(int, string)) (json.RawMessage, error) {
		fail := false
		once.Do(func() { fail, failedSlot = true, req.Language+"/"+req.Channel })
		if fail {
			return nil, &llm.RefusalError{Purpose: llm.PurposeContent, Category: "general_harms"}
		}
		return json.RawMessage(`{"text":"ok"}`), nil
	}
	camp, sub, _ := h.toContent(llm.ModePrimaryFallback, llm.ChannelEmail)
	step := h.step(camp.ID, flow.KindContent)
	if !step.Failed || !strings.Contains(step.Reason, failedSlot+": llm: content call refused") || len(step.Missing) != 1 {
		t.Fatalf("content step = failed %v reason %q missing %v (failed slot %s)", step.Failed, step.Reason, step.Missing, failedSlot)
	}
	if len(step.Variants) != len(h.content.Requests())-1 {
		t.Errorf("%d variants for %d requests, want all but the failed one", len(step.Variants), len(h.content.Requests()))
	}
	var gf *flow.GenerationFailed
	for _, e := range events(sub) {
		if f, ok := e.(flow.GenerationFailed); ok && f.Kind == flow.KindContent {
			gf = &f
		}
	}
	if gf == nil || gf.Slot.String() != failedSlot {
		t.Errorf("GenerationFailed = %+v, want slot %s", gf, failedSlot)
	}
	// Approving the others does not close the step while a slot is empty.
	for _, v := range step.Variants {
		h.mustApprove(v)
	}
	if s := h.step(camp.ID, flow.KindContent); s.Status != flow.StepStatusOpen {
		t.Fatalf("content step is %s with a slot still empty", s.Status)
	}
	if err := h.svc.Regenerate(t.Context(), step.ID); err != nil {
		t.Fatal(err)
	}
	h.idle()
	s := h.step(camp.ID, flow.KindContent)
	if s.Failed || len(s.Missing) != 0 || len(s.Variants) != len(step.Variants)+1 {
		t.Fatalf("content step after Regenerate = %+v", s)
	}
	reqs := h.content.Requests()
	if last := reqs[len(reqs)-1]; last.Language+"/"+last.Channel != failedSlot {
		t.Errorf("Regenerate generated %s/%s, want only %s", last.Language, last.Channel, failedSlot)
	}
}

func TestConcurrentApprovalsOpenTheNextStepOnce(t *testing.T) {
	t.Run("last variants of a step", func(t *testing.T) {
		h := newHarness(t)
		camp, sub, prev := h.toContent(llm.ModePrimaryFallback, llm.ChannelEmail)
		h.llm.ProposeSchedules(h.scheduleFor(langsOf(prev)...))
		step := h.step(camp.ID, flow.KindContent)
		events(sub)

		var wg sync.WaitGroup
		errs := make([]error, len(step.Variants))
		for i, v := range step.Variants {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = h.svc.Approve(t.Context(), v.ID)
			}()
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("Approve(%s): %v", slotKey(step.Variants[i]), err)
			}
		}
		h.idle()
		if n := countRows(t, h, `SELECT count(*) FROM proposal_step WHERE campaign_id = $1 AND kind = 'schedule'`, camp.ID); n != 1 {
			t.Errorf("%d schedule steps, want exactly one", n)
		}
		evs := events(sub)
		opened := 0
		for _, e := range evs {
			if so, ok := e.(flow.StepOpened); ok && so.Kind == flow.KindSchedule {
				opened++
			}
		}
		if opened != 1 || countType(evs, flow.TypeStepApproved) != 1 {
			t.Errorf("schedule opened %d times, content approved %d times", opened, countType(evs, flow.TypeStepApproved))
		}
		if n := len(h.llm.ScheduleInputs()); n != 1 {
			t.Errorf("the schedule was generated %d times", n)
		}
	})

	t.Run("the same variant", func(t *testing.T) {
		h := newHarness(t)
		h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
		camp, _ := h.startCampaign(sampleBrief)
		h.idle()
		v := h.step(camp.ID, flow.KindAudience).Variants[0]
		var wg sync.WaitGroup
		errs := make([]error, 8)
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = h.svc.Approve(t.Context(), v.ID)
			}()
		}
		wg.Wait()
		var ok int
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case !errors.Is(err, flow.ErrNotOpen) && !errors.Is(err, flow.ErrNotDraft):
				t.Errorf("err = %v, want nil, ErrNotOpen or ErrNotDraft", err)
			}
		}
		h.idle()
		if ok != 1 {
			t.Errorf("%d approvals succeeded, want exactly one", ok)
		}
		if n := countRows(t, h, `SELECT count(*) FROM proposal_step WHERE campaign_id = $1 AND kind = 'channel_plan'`, camp.ID); n != 1 {
			t.Errorf("%d plan steps, want exactly one", n)
		}
		if n := len(h.llm.ChannelPlanInputs()); n != 1 {
			t.Errorf("the plan was generated %d times", n)
		}
	})
}

// Generation of a campaign waits for the Postgres advisory lock, which another process may hold.
func TestGenerationWaitsForTheAdvisoryLock(t *testing.T) {
	h := newHarness(t)
	camp, err := h.q.CreateCampaign(t.Context(), sampleBrief)
	if err != nil {
		t.Fatal(err)
	}
	step, err := h.q.CreateStep(t.Context(), store.CreateStepParams{CampaignID: camp.ID, Kind: flow.KindAudience, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	h.llm.ProposeDSLs(sampleDSL)

	holder, err := h.pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(t.Context(), `SELECT pg_advisory_lock($1)`, flow.LockKey(camp.ID)); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Regenerate(t.Context(), step.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if n := len(h.llm.SegmentInputs()); n != 0 {
		t.Fatalf("the model was called %d times while another session held the lock", n)
	}
	if v := h.step(camp.ID, flow.KindAudience); !v.Generating || len(v.Variants) != 0 {
		t.Errorf("step while locked = %+v, want generating and empty", v)
	}
	// Another campaign is not blocked by this lock.
	other, _ := h.q.CreateCampaign(t.Context(), "other")
	ostep, _ := h.q.CreateStep(t.Context(), store.CreateStepParams{CampaignID: other.ID, Kind: flow.KindAudience, Version: 1})
	h.llm.ProposeDSLs(sampleDSL)
	if err := h.svc.Regenerate(t.Context(), ostep.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for len(h.step(other.ID, flow.KindAudience).Variants) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("a lock on one campaign blocked another")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if _, err := holder.Exec(t.Context(), `SELECT pg_advisory_unlock($1)`, flow.LockKey(camp.ID)); err != nil {
		t.Fatal(err)
	}
	holder.Release()
	h.idle()
	if v := h.step(camp.ID, flow.KindAudience); v.Generating || len(v.Variants) != 1 {
		t.Errorf("step after the lock was released = %+v", v)
	}
}

// A note that arrives while the model works makes the result stale: it is dropped, its calls are
// still recorded, and the regeneration with the note produces the variant.
func TestNoteDuringGenerationDropsTheStaleResult(t *testing.T) {
	h := newHarness(t)
	h.llm.AllowScreen().ProposeDSLs(`city === "Ankara"`).WithRecords(llm.PurposeSegment, rec(llm.PurposeSegment, llm.OutcomeAccepted, 1, "")).
		ProposeDSLs(sampleDSL).WithRecords(llm.PurposeSegment, rec(llm.PurposeSegment, llm.OutcomeAccepted, 1, ""))
	entered, release := make(chan struct{}), make(chan struct{})
	var first sync.Once
	camp, _ := h.startCampaignHooked(sampleBrief, func() {
		first.Do(func() {
			close(entered)
			<-release
		})
	})
	<-entered
	if _, err := h.svc.AddContext(t.Context(), camp.ID, "only people in Istanbul"); err != nil {
		t.Fatalf("AddContext: %v", err)
	}
	close(release)
	h.idle()

	step := h.step(camp.ID, flow.KindAudience)
	if len(step.Variants) != 1 || step.Failed {
		t.Fatalf("audience step = %+v, want one variant from the second call", step)
	}
	var aud flow.AudienceContent
	decodeInto(t, step.Variants[0].Content, &aud)
	if aud.DSL != sampleDSL || aud.Count != 46 {
		t.Errorf("the variant is from the stale call: %+v", aud)
	}
	in := h.llm.SegmentInputs()
	if len(in) != 2 || len(in[0].ContextNotes) != 0 || len(in[1].ContextNotes) != 1 || in[1].ContextNotes[0] != "only people in Istanbul" {
		t.Errorf("segment inputs = %+v, want the second call to carry the note", in)
	}
	rows, _ := h.q.ListLLMCalls(t.Context(), camp.ID)
	var seg, linked int
	for _, r := range rows {
		if r.Purpose == "segment" {
			seg++
			if r.VariantID != nil {
				linked++
			}
		}
	}
	if seg != 2 || linked != 1 {
		t.Errorf("%d segment rows, %d linked to a variant; want 2 and 1 (the dropped call is recorded without a variant)", seg, linked)
	}
	if n := countRows(t, h, `SELECT count(*) FROM proposal_variant WHERE step_id = $1`, step.ID); n != 1 {
		t.Errorf("%d variants were stored for the step, want 1", n)
	}
}

func TestErrorsOfTheActions(t *testing.T) {
	h := newHarness(t)
	camp, _, _ := h.toContent(llm.ModePrimaryFallback, llm.ChannelEmail)
	step := h.step(camp.ID, flow.KindContent)
	v := step.Variants[0]
	h.mustApprove(v)

	ctx := t.Context()
	if _, err := h.svc.Get(ctx, uuid.New()); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Get(unknown) err = %v", err)
	}
	if err := h.svc.Approve(ctx, uuid.New()); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Approve(unknown) err = %v", err)
	}
	if err := h.svc.Approve(ctx, v.ID); !errors.Is(err, flow.ErrNotDraft) {
		t.Errorf("Approve(approved) err = %v, want ErrNotDraft", err)
	}
	if err := h.svc.Reject(ctx, v.ID, "x"); !errors.Is(err, flow.ErrNotDraft) {
		t.Errorf("Reject(approved) err = %v, want ErrNotDraft", err)
	}
	if _, err := h.svc.Edit(ctx, v.ID, json.RawMessage(`{"text":"x"}`)); !errors.Is(err, flow.ErrNotDraft) {
		t.Errorf("Edit(approved) err = %v, want ErrNotDraft", err)
	}
	if _, err := h.svc.Edit(ctx, uuid.New(), json.RawMessage(`{}`)); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Edit(unknown) err = %v", err)
	}
	if _, err := h.svc.AddContext(ctx, uuid.New(), "x"); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("AddContext(unknown campaign) err = %v", err)
	}
	if _, err := h.svc.Reopen(ctx, uuid.New(), flow.KindAudience); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Reopen(unknown campaign) err = %v", err)
	}
	// An approved step is not open: a variant of the audience step cannot be touched any more.
	aud := h.step(camp.ID, flow.KindAudience)
	if err := h.svc.Reject(ctx, aud.Variants[0].ID, "late"); !errors.Is(err, flow.ErrNotOpen) {
		t.Errorf("Reject(variant of an approved step) err = %v, want ErrNotOpen", err)
	}
	if err := h.svc.Regenerate(ctx, aud.ID); !errors.Is(err, flow.ErrNotOpen) {
		t.Errorf("Regenerate(approved step) err = %v, want ErrNotOpen", err)
	}
}

// The Confirmer failing does not undo the approval; the caller is told.
func TestConfirmerFailureIsReportedAndTheApprovalStands(t *testing.T) {
	h := newHarness(t)
	camp, _, prev := h.toContent(llm.ModePrimaryFallback, llm.ChannelEmail)
	h.llm.ProposeSchedules(h.scheduleFor(langsOf(prev)...))
	h.approveAll(camp.ID, flow.KindContent)
	h.confirmer.Err = errors.New("outbox is down")
	svs := h.step(camp.ID, flow.KindSchedule).Variants
	for _, v := range svs[:len(svs)-1] {
		h.mustApprove(v)
	}
	err := h.svc.Approve(t.Context(), svs[len(svs)-1].ID)
	if !errors.Is(err, flow.ErrConfirm) || !strings.Contains(err.Error(), "outbox is down") {
		t.Fatalf("err = %v, want ErrConfirm", err)
	}
	if s := h.step(camp.ID, flow.KindSchedule); s.Status != flow.StepStatusApproved {
		t.Errorf("schedule step is %s: the approval must stand", s.Status)
	}
	if n := len(h.confirmer.Calls()); n != 1 {
		t.Errorf("Confirmer called %d times", n)
	}
}

// Get derives a brief overview without hiding anything: the rejected variant of a slot being
// regenerated is shown with its reason.
func TestGetShowsTheRejectedVariantWhileItIsRegenerated(t *testing.T) {
	h := newHarness(t)
	h.llm.AllowScreen().ProposeDSLs(sampleDSL)
	h.llm.ProposePlans(llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "first"})
	h.llm.ProposePlans(llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "second"})
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	plan := h.step(camp.ID, flow.KindChannelPlan).Variants[0]

	// Hold the regeneration open on the advisory lock so the intermediate state can be read.
	holder, err := h.pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	if _, err := holder.Exec(t.Context(), `SELECT pg_advisory_lock($1)`, flow.LockKey(camp.ID)); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Reject(t.Context(), plan.ID, "use sms first"); err != nil {
		t.Fatal(err)
	}
	s := h.step(camp.ID, flow.KindChannelPlan)
	if len(s.Variants) != 1 || s.Variants[0].Status != flow.VariantStatusRejected || s.Variants[0].RejectionReason != "use sms first" || !s.Generating || len(s.Missing) != 1 {
		t.Errorf("plan step during the regeneration = %+v", s)
	}
	if _, err := holder.Exec(t.Context(), `SELECT pg_advisory_unlock($1)`, flow.LockKey(camp.ID)); err != nil {
		t.Fatal(err)
	}
	h.idle()
	s = h.step(camp.ID, flow.KindChannelPlan)
	if len(s.Variants) != 1 || s.Variants[0].Status != flow.VariantStatusDraft || !strings.Contains(string(s.Variants[0].Content), `"second"`) {
		t.Errorf("plan step after the regeneration = %+v", s)
	}
	in := h.llm.ChannelPlanInputs()
	if len(in) != 2 || len(in[1].ContextNotes) != 1 || !strings.Contains(in[1].ContextNotes[0], "use sms first") || !strings.Contains(in[1].ContextNotes[0], "channel plan") {
		t.Errorf("the regeneration did not get the reason: %+v", in)
	}
}

// With a pool of two connections only one campaign is generated at a time: a worker holds one
// connection for its advisory lock and needs the other for its transactions. Several campaigns
// still all get through, and none deadlocks.
func TestWorkersNeverStarveTheirOwnTransactions(t *testing.T) {
	h := newHarnessWith(t, 2, nil)
	const n = 4
	for range n {
		h.llm.AllowScreen().ProposeDSLs(sampleDSL)
	}
	ids := make([]uuid.UUID, n)
	for i := range ids {
		c, err := h.svc.CreateCampaign(t.Context(), fmt.Sprintf("%s (%d)", sampleBrief, i))
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = c.ID
	}
	h.idle()
	for _, id := range ids {
		if s := h.step(id, flow.KindAudience); s.Failed || len(s.Variants) != 1 {
			t.Errorf("campaign %s: audience step = %+v", id, s)
		}
	}
}

// The content step needs a ContentGenerator; without one the step fails with a clear reason
// (T-024 is not wired yet).
func TestContentStepWithoutAGeneratorFails(t *testing.T) {
	h := newHarnessWith(t, 0, func(d *flow.Deps) { d.Content = nil })
	h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	h.approveAll(camp.ID, flow.KindChannelPlan)
	step := h.step(camp.ID, flow.KindContent)
	if !step.Failed || !strings.Contains(step.Reason, flow.ErrNoContentGenerator.Error()) || len(step.Variants) != 0 || len(step.Missing) == 0 {
		t.Errorf("content step = %+v", step)
	}
	v := stepOf(h.get(camp.ID), flow.KindChannelPlan).Variants[0]
	if _, err := h.svc.Edit(t.Context(), v.ID, json.RawMessage(`{}`)); !errors.Is(err, flow.ErrNotOpen) {
		t.Errorf("Edit on an approved step: err = %v", err)
	}
}

// A campaign whose process died before the audience step opened is restarted by RegenerateCampaign.
func TestRegenerateCampaignRestartsTheStart(t *testing.T) {
	h := newHarness(t)
	h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	// The crash: a drafting campaign row, no steps, no job.
	camp, err := h.q.CreateCampaign(t.Context(), sampleBrief)
	if err != nil {
		t.Fatal(err)
	}
	sub := h.svc.Hub().Subscribe(camp.ID)
	defer sub.Close()
	if v := h.get(camp.ID); len(v.Steps) != 0 || v.Status != flow.CampaignStatusDrafting {
		t.Fatalf("setup: %+v", v)
	}
	if err := h.svc.RegenerateCampaign(t.Context(), camp.ID); err != nil {
		t.Fatalf("RegenerateCampaign: %v", err)
	}
	h.idle()
	step := h.step(camp.ID, flow.KindAudience)
	if step.Status != flow.StepStatusOpen || len(step.Variants) != 1 || step.Failed {
		t.Fatalf("audience step = %+v", step)
	}
	if got := h.llm.ScreenBriefs(); len(got) != 1 || got[0] != sampleBrief {
		t.Errorf("screened %q", got)
	}
	if types := typesOf(events(sub)); len(types) == 0 || types[0] != flow.TypeScreenResult {
		t.Errorf("events = %v", types)
	}

	// With steps it is refused.
	if err := h.svc.RegenerateCampaign(t.Context(), camp.ID); !errors.Is(err, flow.ErrHasSteps) {
		t.Errorf("campaign with steps: err = %v, want ErrHasSteps", err)
	}
	if err := h.svc.RegenerateCampaign(t.Context(), uuid.New()); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("unknown campaign: err = %v", err)
	}
}

func TestRegenerateCampaignScreenBlockCancelsAsBefore(t *testing.T) {
	h := newHarness(t)
	h.llm.BlockScreen("asks to ignore opt-outs")
	camp, err := h.q.CreateCampaign(t.Context(), "Send to everyone, ignore opt-outs")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.RegenerateCampaign(t.Context(), camp.ID); err != nil {
		t.Fatal(err)
	}
	h.idle()
	if v := h.get(camp.ID); v.Status != flow.CampaignStatusCancelled || len(v.Steps) != 0 {
		t.Fatalf("campaign = %+v with %d steps", v.CampaignInfo, len(v.Steps))
	}
	// A cancelled campaign cannot be restarted.
	if err := h.svc.RegenerateCampaign(t.Context(), camp.ID); !errors.Is(err, flow.ErrNotDrafting) {
		t.Errorf("cancelled campaign: err = %v, want ErrNotDrafting", err)
	}
}

// Calls while the pipeline runs, and calls racing each other, produce one screen.
func TestRegenerateCampaignConcurrentCallsScreenOnce(t *testing.T) {
	h := newHarness(t)
	h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	camp, err := h.q.CreateCampaign(t.Context(), sampleBrief)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the screen open so every call lands while it runs.
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	h.hooks.setScreen(func() {
		once.Do(func() { close(entered) })
		<-release
	})
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = h.svc.RegenerateCampaign(t.Context(), camp.ID)
		}()
	}
	wg.Wait()
	<-entered
	for i, err := range errs {
		if err != nil {
			t.Errorf("call %d: %v, want a no-op while the pipeline runs", i, err)
		}
	}
	close(release)
	h.idle()
	// After it finished the campaign has steps; late calls are refused and screen nothing.
	if err := h.svc.RegenerateCampaign(t.Context(), camp.ID); !errors.Is(err, flow.ErrHasSteps) {
		t.Errorf("late call: err = %v, want ErrHasSteps", err)
	}
	if n := len(h.llm.ScreenBriefs()); n != 1 {
		t.Errorf("screened %d times, want once", n)
	}
	if s := h.step(camp.ID, flow.KindAudience); len(s.Variants) != 1 {
		t.Errorf("audience step = %+v", s)
	}
}
