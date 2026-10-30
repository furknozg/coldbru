//go:build integration

package flow_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
)

// The agent's questions, the marketer's answers and the summaries (D-088).

var cityQuestion = llm.Question{Text: "Hangi şehirdeki müşteriler?", Options: []string{"İstanbul", "Ankara"}}

// messagesOf returns the chat messages of a step, oldest first.
func messagesOf(v *flow.CampaignView, step uuid.UUID) []flow.ChatMessageView {
	var out []flow.ChatMessageView
	for _, m := range v.Messages {
		if m.StepID != nil && *m.StepID == step {
			out = append(out, m)
		}
	}
	return out
}

func TestAudienceQuestionWaitsForTheAnswer(t *testing.T) {
	h := newHarness(t)
	h.llm.AllowScreen().WithRecords(llm.PurposeScreen, rec(llm.PurposeScreen, llm.OutcomeAccepted, 1, ""))
	h.llm.AskSegment(cityQuestion).WithRecords(llm.PurposeSegment, rec(llm.PurposeSegment, llm.OutcomeAccepted, 1, ""))
	h.llm.ProposeDSLs(sampleDSL).WithRecords(llm.PurposeSegment, rec(llm.PurposeSegment, llm.OutcomeAccepted, 1, "")).WithSummary(llm.PurposeSegment, "**Kitle**: İstanbul, 46 kişi.")
	camp, sub := h.startCampaign("Sepetini terk edenlere hatırlatma gönder. Hangi şehir?")
	h.idle()

	// 1. The step is open and waits: no variant, no failure, the question pending.
	v := h.get(camp.ID)
	if len(v.Steps) != 1 {
		t.Fatalf("steps = %d", len(v.Steps))
	}
	step := v.Steps[0]
	if step.Status != flow.StepStatusOpen || step.Generating || step.Failed || len(step.Variants) != 0 || len(step.Missing) != 1 {
		t.Fatalf("audience step = %+v", step)
	}
	if step.PendingQuestion == nil || step.PendingQuestion.Kind != flow.MessageKindQuestion || step.PendingQuestion.Body != cityQuestion.Text ||
		strings.Join(step.PendingQuestion.Options, ",") != "İstanbul,Ankara" || step.PendingQuestion.StepID == nil || *step.PendingQuestion.StepID != step.ID ||
		step.PendingQuestion.CampaignID != camp.ID {
		t.Fatalf("pending question = %+v", step.PendingQuestion)
	}
	if len(v.Messages) != 1 || v.Messages[0].ID != step.PendingQuestion.ID {
		t.Errorf("messages = %+v", v.Messages)
	}
	if in := h.llm.SegmentInputs(); len(in) != 1 || in[0].QuestionsLeft != llm.MaxQuestionsPerStep || len(in[0].Clarifications) != 0 {
		t.Errorf("first segment input = %+v", in)
	}
	evs := events(sub)
	var asked *flow.QuestionAsked
	for _, e := range evs {
		if q, ok := e.(flow.QuestionAsked); ok {
			asked = &q
		}
	}
	if asked == nil || asked.StepID != step.ID || asked.MessageID != step.PendingQuestion.ID || asked.Question != cityQuestion.Text || len(asked.Options) != 2 {
		t.Fatalf("question_asked = %+v (events %v)", asked, typesOf(evs))
	}
	if countType(evs, flow.TypeVariantCreated) != 0 || countType(evs, flow.TypeGenerationFailed) != 0 {
		t.Errorf("events = %v", typesOf(evs))
	}
	// The question turn has its llm_call row, with the step.
	rows, err := h.q.ListLLMCalls(t.Context(), camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1].Purpose != "segment" || rows[1].StepID == nil || *rows[1].StepID != step.ID || rows[1].VariantID != nil {
		t.Errorf("llm_call rows = %+v", rows)
	}

	// 2. The answer: stored as a reply, given to the model as a clarification, the step filled.
	ans, err := h.svc.Answer(t.Context(), step.PendingQuestion.ID, "  Ankara  ")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if ans.Kind != flow.MessageKindAnswer || ans.Body != "Ankara" || ans.ReplyTo == nil || *ans.ReplyTo != step.PendingQuestion.ID || ans.CampaignID != camp.ID {
		t.Errorf("answer = %+v", ans)
	}
	h.idle()
	v = h.get(camp.ID)
	step = v.Steps[0]
	if step.PendingQuestion != nil || len(step.Variants) != 1 || step.Failed || len(step.Missing) != 0 {
		t.Fatalf("audience step after the answer = %+v", step)
	}
	in := h.llm.SegmentInputs()
	if len(in) != 2 || in[1].QuestionsLeft != llm.MaxQuestionsPerStep-1 || len(in[1].Clarifications) != 1 ||
		in[1].Clarifications[0] != (llm.Clarification{Question: cityQuestion.Text, Answer: "Ankara"}) {
		t.Errorf("second segment input = %+v", in)
	}
	msgs := messagesOf(v, step.ID)
	if len(msgs) != 3 || msgs[0].Kind != flow.MessageKindQuestion || msgs[1].Kind != flow.MessageKindAnswer || msgs[2].Kind != flow.MessageKindSummary ||
		msgs[2].Body != "**Kitle**: İstanbul, 46 kişi." || msgs[1].ID != ans.ID {
		t.Errorf("messages = %+v", msgs)
	}
	evs = events(sub)
	var given *flow.AnswerGiven
	var posted *flow.SummaryPosted
	for _, e := range evs {
		switch x := e.(type) {
		case flow.AnswerGiven:
			given = &x
		case flow.SummaryPosted:
			posted = &x
		}
	}
	if given == nil || given.MessageID != ans.ID || given.QuestionID != msgs[0].ID || given.Answer != "Ankara" || given.StepID != step.ID {
		t.Errorf("answer_given = %+v", given)
	}
	if posted == nil || posted.MessageID != msgs[2].ID || posted.Markdown != msgs[2].Body || posted.Slot != (flow.Slot{}) {
		t.Errorf("summary_posted = %+v", posted)
	}
	types := typesOf(evs)
	if i, j := indexOf(types, flow.TypeAnswerGiven), indexOf(types, flow.TypeVariantCreated); i < 0 || j < 0 || i > j {
		t.Errorf("events = %v, want answer_given before variant_created", types)
	}

	// 3. A second answer to the same question is refused, so is answering anything but a question.
	if _, err := h.svc.Answer(t.Context(), msgs[0].ID, "İstanbul"); !errors.Is(err, flow.ErrAlreadyAnswered) {
		t.Errorf("second Answer: err = %v, want ErrAlreadyAnswered", err)
	}
	if _, err := h.svc.Answer(t.Context(), msgs[2].ID, "x"); !errors.Is(err, flow.ErrNotQuestion) {
		t.Errorf("Answer(summary): err = %v, want ErrNotQuestion", err)
	}
	if _, err := h.svc.Answer(t.Context(), ans.ID, "x"); !errors.Is(err, flow.ErrNotQuestion) {
		t.Errorf("Answer(answer): err = %v, want ErrNotQuestion", err)
	}
	if _, err := h.svc.Answer(t.Context(), uuid.New(), "x"); !errors.Is(err, flow.ErrNotFound) {
		t.Errorf("Answer(unknown): err = %v, want ErrNotFound", err)
	}
	if _, err := h.svc.Answer(t.Context(), msgs[0].ID, "  "); !errors.Is(err, flow.ErrInvalidInput) {
		t.Errorf("Answer(blank): err = %v, want ErrInvalidInput", err)
	}
	if n := len(h.llm.SegmentInputs()); n != 2 {
		t.Errorf("refused answers triggered %d extra generations", n-2)
	}
}

func indexOf(ss []string, s string) int {
	for i, x := range ss {
		if x == s {
			return i
		}
	}
	return -1
}

// A third question in a step is not stored: the generation fails with a visible reason and the
// marketer can regenerate (the adapters refuse it earlier; the flow is the backstop).
func TestThirdQuestionIsBlocked(t *testing.T) {
	h := newHarness(t)
	h.llm.AllowScreen()
	q2 := llm.Question{Text: "Hangi dönem?"}
	h.llm.AskSegment(cityQuestion).AskSegment(q2).AskSegment(llm.Question{Text: "Bir soru daha?"}).ProposeDSLs(sampleDSL)
	camp, sub := h.startCampaign("Kim?")
	h.idle()
	step := h.get(camp.ID).Steps[0]
	if step.PendingQuestion == nil || step.PendingQuestion.Body != cityQuestion.Text {
		t.Fatalf("step = %+v", step)
	}
	if _, err := h.svc.Answer(t.Context(), step.PendingQuestion.ID, "İstanbul"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	step = h.get(camp.ID).Steps[0]
	if step.PendingQuestion == nil || step.PendingQuestion.Body != q2.Text || len(step.PendingQuestion.Options) != 0 {
		t.Fatalf("second question = %+v", step.PendingQuestion)
	}
	if in := h.llm.SegmentInputs(); len(in) != 2 || in[1].QuestionsLeft != 1 {
		t.Errorf("inputs = %+v", in)
	}
	if _, err := h.svc.Answer(t.Context(), step.PendingQuestion.ID, "Son 30 gün"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	v := h.get(camp.ID)
	step = v.Steps[0]
	if step.PendingQuestion != nil || !step.Failed || !strings.Contains(step.Reason, "after the 2 allowed") || len(step.Variants) != 0 {
		t.Fatalf("step after the third question = %+v", step)
	}
	if n := len(messagesOf(v, step.ID)); n != 4 {
		t.Errorf("%d messages, want 2 questions and 2 answers (the third question is not stored)", n)
	}
	if in := h.llm.SegmentInputs(); len(in) != 3 || in[2].QuestionsLeft != 0 || len(in[2].Clarifications) != 2 || in[2].Clarifications[1].Answer != "Son 30 gün" {
		t.Errorf("third input = %+v", in)
	}
	evs := events(sub)
	if countType(evs, flow.TypeQuestionAsked) != 2 || countType(evs, flow.TypeGenerationFailed) != 1 {
		t.Errorf("events = %v", typesOf(evs))
	}
	// Regenerate carries on with both clarifications and no question left.
	if err := h.svc.Regenerate(t.Context(), step.ID); err != nil {
		t.Fatal(err)
	}
	h.idle()
	step = h.get(camp.ID).Steps[0]
	if step.Failed || len(step.Variants) != 1 {
		t.Errorf("step after Regenerate = %+v", step)
	}
	if in := h.llm.SegmentInputs(); len(in) != 4 || in[3].QuestionsLeft != 0 || len(in[3].Clarifications) != 2 {
		t.Errorf("fourth input = %+v", in)
	}
}

// The plan and the schedule steps ask too, through the question of their structured output.
func TestPlanAndScheduleQuestions(t *testing.T) {
	h := newHarness(t)
	prev := h.scriptAudienceAndPlan(llm.ModePrimaryFallback, llm.ChannelEmail)
	locales := localesOf(prev)
	// The fake pops in queue order: the first plan call plans (scriptAudienceAndPlan), the second
	// one, after a rejection, asks; the first schedule call schedules, the second, after a note, asks.
	h.llm.AskChannelPlan(llm.Question{Text: "SMS de gönderilsin mi?", Options: []string{"Evet", "Hayır"}})
	h.llm.ProposeSchedules(h.scheduleFor(locales...))
	camp, _ := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)

	plan := h.step(camp.ID, flow.KindChannelPlan)
	if plan.PendingQuestion != nil {
		t.Fatalf("the plan asked before planning: %+v", plan.PendingQuestion)
	}
	if len(plan.Variants) != 1 {
		t.Fatalf("plan step = %+v", plan)
	}
	// Reject the plan: the regeneration is the question call.
	if err := h.svc.Reject(t.Context(), plan.Variants[0].ID, "Fazla e-posta."); err != nil {
		t.Fatal(err)
	}
	h.idle()
	plan = h.step(camp.ID, flow.KindChannelPlan)
	if plan.PendingQuestion == nil || plan.PendingQuestion.Body != "SMS de gönderilsin mi?" || plan.Failed || len(plan.Missing) != 1 {
		t.Fatalf("plan step after the question = %+v", plan)
	}
	if in := h.llm.ChannelPlanInputs(); len(in) != 2 || in[1].QuestionsLeft != 2 || len(in[1].Clarifications) != 0 {
		t.Errorf("plan inputs = %+v", in)
	}
	h.llm.ProposePlans(llm.ChannelPlan{Mode: llm.ModeBoth, Primary: llm.ChannelEmail, Rationale: "both, as answered"}).WithSummary(llm.PurposeChannelPlan, "- her iki kanal")
	if _, err := h.svc.Answer(t.Context(), plan.PendingQuestion.ID, "Evet"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	plan = h.step(camp.ID, flow.KindChannelPlan)
	if plan.PendingQuestion != nil || len(plan.Variants) != 1 || plan.Variants[0].Status != flow.VariantStatusDraft {
		t.Fatalf("plan step after the answer = %+v", plan)
	}
	if in := h.llm.ChannelPlanInputs(); len(in) != 3 || in[2].QuestionsLeft != 1 || len(in[2].Clarifications) != 1 || in[2].Clarifications[0].Answer != "Evet" ||
		!strings.Contains(strings.Join(in[2].ContextNotes, " "), "Fazla e-posta") {
		t.Errorf("plan inputs = %+v", in)
	}
	// The first plan's summary, the question, the answer and the second plan's summary.
	msgs := messagesOf(h.get(camp.ID), plan.ID)
	if len(msgs) != 4 || msgs[0].Kind != flow.MessageKindSummary || msgs[1].Kind != flow.MessageKindQuestion || msgs[2].Kind != flow.MessageKindAnswer ||
		msgs[3].Kind != flow.MessageKindSummary || msgs[3].Body != "- her iki kanal" {
		t.Errorf("plan messages = %+v", msgs)
	}
	h.approveAll(camp.ID, flow.KindChannelPlan)
	h.approveAll(camp.ID, flow.KindContent)

	// The schedule: the scripted schedule is consumed first; a note regenerates with a question.
	sched := h.step(camp.ID, flow.KindSchedule)
	if len(sched.Variants) != len(locales) {
		t.Fatalf("schedule step = %+v", sched)
	}
	h.llm.AskSchedule(llm.Question{Text: "Hangi Cuma?"})
	if _, err := h.svc.AddContext(t.Context(), camp.ID, "Cuma günü gönder."); err != nil {
		t.Fatal(err)
	}
	h.idle()
	sched = h.step(camp.ID, flow.KindSchedule)
	if sched.PendingQuestion == nil || sched.PendingQuestion.Body != "Hangi Cuma?" || len(sched.Missing) != len(locales) {
		t.Fatalf("schedule step after the question = %+v", sched)
	}
	h.llm.ProposeSchedules(h.scheduleFor(locales...))
	schedQuestion := sched.PendingQuestion.ID
	if _, err := h.svc.Answer(t.Context(), schedQuestion, "Bu Cuma"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	sched = h.step(camp.ID, flow.KindSchedule)
	if sched.PendingQuestion != nil || len(sched.Variants) != len(locales) {
		t.Fatalf("schedule step after the answer = %+v", sched)
	}
	if in := h.llm.ScheduleInputs(); len(in) != 3 || in[2].QuestionsLeft != 1 || len(in[2].Clarifications) != 1 || in[2].Clarifications[0].Answer != "Bu Cuma" {
		t.Errorf("schedule inputs = %+v", in)
	}
	// One schedule summary, posted without a slot.
	msgs = messagesOf(h.get(camp.ID), sched.ID)
	n := 0
	for _, m := range msgs {
		if m.Kind == flow.MessageKindSummary {
			n++
		}
	}
	if n != 2 { // the first generation and the regeneration after the answer
		t.Errorf("%d schedule summaries, want 2: %+v", n, msgs)
	}
	// An approved step takes no more answers.
	h.approveAll(camp.ID, flow.KindSchedule)
	if _, err := h.svc.Answer(t.Context(), schedQuestion, "x"); !errors.Is(err, flow.ErrNotOpen) {
		t.Errorf("Answer on a question of a closed step: %v, want ErrNotOpen (checked before the answer)", err)
	}
}

// A title that breaks the checks is dropped, never a reason to fail the screen; a model that
// gives no summary leaves the step without one; a summary that breaks the rules is dropped.
func TestTitleAndSummaryChecks(t *testing.T) {
	h := newHarness(t)
	long := strings.Repeat("ş", llm.MaxTitleChars+1)
	h.llm.AllowScreenTitled(long)
	h.llm.ProposeDSLs(sampleDSL).WithSummary(llm.PurposeSegment, "")
	camp, sub := h.startCampaign(sampleBrief)
	h.idle()
	v := h.get(camp.ID)
	if v.Title != "" || len(v.Steps) != 1 || len(v.Steps[0].Variants) != 1 || len(v.Messages) != 0 {
		t.Fatalf("campaign = %+v, steps %d, messages %d", v.CampaignInfo, len(v.Steps), len(v.Messages))
	}
	evs := events(sub)
	if countType(evs, flow.TypeCampaignTitled) != 0 || countType(evs, flow.TypeSummaryPosted) != 0 {
		t.Errorf("events = %v", typesOf(evs))
	}

	h2 := newHarness(t)
	h2.llm.AllowScreenTitled("  Kısa başlık  ")
	h2.llm.ProposeDSLs(sampleDSL).WithSummary(llm.PurposeSegment, "see <a href='x'>here</a>")
	camp2, _ := h2.startCampaign(sampleBrief)
	h2.idle()
	v2 := h2.get(camp2.ID)
	if v2.Title != "Kısa başlık" || len(v2.Steps[0].Variants) != 1 || len(v2.Messages) != 0 {
		t.Errorf("campaign = %+v, variants %d, messages %+v", v2.CampaignInfo, len(v2.Steps[0].Variants), v2.Messages)
	}
	if !strings.Contains(h2.get(camp2.ID).Title, "Kısa") {
		t.Error("title lost")
	}
	_ = llmtest.DefaultSummary
}

// An answer that arrives while the model is working makes the running result stale, like a note
// (D-070): the regeneration the answer queued is the one that counts.
func TestAnswerDuringGenerationDropsTheStaleResult(t *testing.T) {
	h := newHarness(t)
	h.llm.AllowScreen()
	h.llm.AskSegment(cityQuestion).AskSegment(llm.Question{Text: "Hangi dönem?"}).ProposeDSLs(sampleDSL).ProposeDSLs(sampleDSL)
	camp, _ := h.startCampaign("Kim?")
	h.idle()
	q1 := h.get(camp.ID).Steps[0].PendingQuestion
	if q1 == nil {
		t.Fatal("no pending question")
	}
	if _, err := h.svc.Answer(t.Context(), q1.ID, "İstanbul"); err != nil {
		t.Fatal(err)
	}
	h.idle()
	step := h.get(camp.ID).Steps[0]
	q2 := step.PendingQuestion
	if q2 == nil || q2.ID == q1.ID {
		t.Fatalf("step = %+v", step)
	}
	// Regenerate while the second question is pending; the model proposes, but the marketer
	// answers the second question before the proposal is stored.
	started := make(chan struct{})
	release := make(chan struct{})
	h.hooks.setSegment(func() {
		close(started)
		<-release
	})
	if err := h.svc.Regenerate(t.Context(), step.ID); err != nil {
		t.Fatal(err)
	}
	<-started
	h.hooks.setSegment(nil)
	if _, err := h.svc.Answer(t.Context(), q2.ID, "Son 30 gün"); err != nil {
		t.Fatal(err)
	}
	close(release)
	h.idle()
	step = h.get(camp.ID).Steps[0]
	if len(step.Variants) != 1 || step.PendingQuestion != nil || step.Failed {
		t.Fatalf("step = %+v", step)
	}
	in := h.llm.SegmentInputs()
	if len(in) != 4 || len(in[2].Clarifications) != 1 || len(in[3].Clarifications) != 2 {
		t.Fatalf("inputs = %+v: the stale proposal of the third call must be dropped and the fourth call must see both answers", in)
	}
	if n := countRows(t, h, `SELECT count(*) FROM proposal_variant WHERE step_id = $1`, step.ID); n != 1 {
		t.Errorf("%d variants stored, want 1 (the stale one dropped)", n)
	}
}
