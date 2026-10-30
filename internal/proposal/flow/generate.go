package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// persistTimeout bounds the writes that follow a finished model call. They use a context that
// ignores the cancellation of a shutdown: the call has been paid for.
const persistTimeout = 15 * time.Second

// collect returns a context whose model calls are collected in memory, to be stored when the
// outcome is known (llm_call is append-only and the accepted record names its variant, D-060).
func collect(ctx context.Context) (context.Context, *llm.Collector) {
	col := &llm.Collector{}
	return llm.WithRecorder(ctx, col), col
}

// runStart screens the brief and opens the audience step (D-069, D-070). A block, a refusal or any
// failure of the screen cancels the campaign: the marketer writes a new one.
func (s *Service) runStart(ctx context.Context, campaignID uuid.UUID) {
	camp, err := s.q.GetCampaign(ctx, campaignID)
	if err != nil || camp.Status != CampaignStatusDrafting {
		return
	}
	// A start that arrives after another one finished (RegenerateCampaign) must not screen again.
	if steps, err := s.q.ListSteps(ctx, campaignID); err != nil || len(steps) > 0 {
		return
	}
	cctx, col := collect(ctx)
	res, err := s.llm.Screen(cctx, camp.Brief)
	s.persistLoose(ctx, campaignID, nil, col.Records())
	if ctx.Err() == nil {
		// The title is stored for an allowed and a blocked brief alike (the list shows both); a
		// title that fails the checks is dropped, never a reason to fail the screen (D-088).
		s.setTitle(ctx, campaignID, res.Title)
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		verdict, reason := "error", "the brief could not be screened: "+err.Error()
		var blocked *llm.ScreenBlockedError
		if errors.As(err, &blocked) {
			verdict, reason = "block", blocked.Reason
		}
		s.cancelCampaign(ctx, campaignID, verdict, reason)
		return
	}
	s.hub.Publish(ScreenResult{Meta: s.meta(campaignID), Verdict: string(res.Verdict), Reason: res.Reason})

	var step store.ProposalStep
	err = s.inTx(ctx, campaignID, func(t *txn, camp store.Campaign) error {
		if camp.Status != CampaignStatusDrafting {
			return ErrNotDrafting
		}
		live, err := t.q.ListLiveSteps(ctx, campaignID)
		if err != nil {
			return err
		}
		if len(live) > 0 {
			return ErrNotOpen // already started
		}
		step, err = t.openStep(ctx, s, campaignID, KindAudience)
		return err
	})
	if err != nil {
		if !errors.Is(err, ErrNotDrafting) && !errors.Is(err, ErrNotOpen) && ctx.Err() == nil {
			s.log.Error("flow: cannot open the audience step", "campaign", campaignID, "error", err)
		}
		return
	}
	done := s.markGenerating(step.ID)
	defer done()
	s.runFill(ctx, campaignID, step.ID)
}

// setTitle stores the title the screen gave the campaign and publishes CampaignTitled (D-088). An
// empty or invalid title is logged and skipped.
func (s *Service) setTitle(ctx context.Context, campaignID uuid.UUID, title string) {
	if strings.TrimSpace(title) == "" {
		return
	}
	title, err := llm.CheckTitle(title)
	if err != nil {
		s.log.Warn("flow: dropping the campaign title the screen gave", "campaign", campaignID, "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	err = s.inTx(ctx, campaignID, func(t *txn, camp store.Campaign) error {
		if _, err := t.q.SetCampaignTitle(ctx, store.SetCampaignTitleParams{ID: camp.ID, Title: text(title)}); err != nil {
			return err
		}
		t.emit(CampaignTitled{Meta: s.meta(camp.ID), Title: title})
		return nil
	})
	if err != nil {
		s.log.Error("flow: cannot store the campaign title", "campaign", campaignID, "error", err)
	}
}

// cancelCampaign moves a drafting campaign to cancelled and publishes why (D-033, D-069).
func (s *Service) cancelCampaign(ctx context.Context, campaignID uuid.UUID, verdict, reason string) {
	ctx = context.WithoutCancel(ctx)
	err := s.inTx(ctx, campaignID, func(t *txn, camp store.Campaign) error {
		if camp.Status != CampaignStatusDrafting {
			return nil
		}
		if _, err := t.q.SetCampaignStatus(ctx, store.SetCampaignStatusParams{ID: campaignID, Status: CampaignStatusCancelled}); err != nil {
			return err
		}
		t.emit(ScreenResult{Meta: s.meta(campaignID), Verdict: verdict, Reason: reason})
		t.emit(CampaignCancelled{Meta: s.meta(campaignID), Reason: reason})
		return nil
	})
	if err != nil {
		s.log.Error("flow: cannot cancel the campaign", "campaign", campaignID, "error", err)
	}
}

// markGenerating counts a generation of step that is not a queued job (the one runStart starts).
func (s *Service) markGenerating(step uuid.UUID) func() {
	s.mu.Lock()
	s.stepJobs[step]++
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.stepJobs[step]--; s.stepJobs[step] <= 0 {
			delete(s.stepJobs, step)
		}
	}
}

// openStep creates the next version of a step of kind and emits StepOpened.
func (t *txn) openStep(ctx context.Context, s *Service, campaignID uuid.UUID, kind string) (store.ProposalStep, error) {
	v, err := t.q.MaxStepVersion(ctx, store.MaxStepVersionParams{CampaignID: campaignID, Kind: kind})
	if err != nil {
		return store.ProposalStep{}, err
	}
	st, err := t.q.CreateStep(ctx, store.CreateStepParams{CampaignID: campaignID, Kind: kind, Version: v + 1})
	if err != nil {
		return store.ProposalStep{}, err
	}
	t.emit(StepOpened{Meta: s.meta(campaignID), StepID: st.ID, Kind: st.Kind, Version: int(st.Version)})
	return st, nil
}

// fillCtx is everything a generation of one step works from, read at its start.
type fillCtx struct {
	campaign store.Campaign
	step     store.ProposalStep
	// notes are the bodies of all context notes; noteCount is how many there were, to notice a
	// note added while the model was working.
	notes     []string
	noteCount int
	live      []store.ProposalVariant
	audience  *AudienceContent
	plan      *PlanContent
	expected  []Slot
	missing   []Slot
	// rejected maps a slot to its latest rejected variant, whose reason goes to the model.
	rejected map[Slot]store.ProposalVariant
	// clarifications are the answered questions of the step, oldest first (D-088); questions is
	// how many questions the step asked, answered or not; answers is how many answers the step
	// had, to notice one that arrived while the model was working (summaries posted by this very
	// generation do not count).
	clarifications []llm.Clarification
	questions      int
	answers        int
}

// questionsLeft is how many questions the step may still ask (D-088).
func (fc *fillCtx) questionsLeft() int {
	return max(llm.MaxQuestionsPerStep-fc.questions, 0)
}

// chatOf reads the chat of a step: the answered questions as clarifications, the number of
// questions and of answers.
func chatOf(ctx context.Context, q *store.Queries, stepID uuid.UUID) (clar []llm.Clarification, questions, answers int, err error) {
	msgs, err := q.ListChatMessagesOfStep(ctx, &stepID)
	if err != nil {
		return nil, 0, 0, err
	}
	byQuestion := map[uuid.UUID]string{}
	for _, m := range msgs {
		if m.Kind == MessageKindAnswer && m.ReplyTo != nil {
			byQuestion[*m.ReplyTo] = m.Body
		}
	}
	for _, m := range msgs {
		if m.Kind != MessageKindQuestion {
			continue
		}
		questions++
		if a, ok := byQuestion[m.ID]; ok {
			clar = append(clar, llm.Clarification{Question: m.Body, Answer: a})
		}
	}
	return clar, questions, len(byQuestion), nil
}

// countAnswers counts the answers of a step.
func countAnswers(ctx context.Context, q *store.Queries, stepID uuid.UUID) (int, error) {
	msgs, err := q.ListChatMessagesOfStep(ctx, &stepID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range msgs {
		if m.Kind == MessageKindAnswer {
			n++
		}
	}
	return n, nil
}

// approvedContent decodes the content of the approved variant of the live step of kind into out.
// It reports false when there is no approved step of that kind.
func approvedContent(ctx context.Context, q *store.Queries, campaignID uuid.UUID, kind string, out any) (bool, error) {
	steps, err := q.ListLiveSteps(ctx, campaignID)
	if err != nil {
		return false, err
	}
	for _, st := range steps {
		if st.Kind != kind || st.Status != StepStatusApproved {
			continue
		}
		vs, err := q.ListLiveVariants(ctx, st.ID)
		if err != nil {
			return false, err
		}
		for _, v := range vs {
			if v.Status == VariantStatusApproved {
				if err := json.Unmarshal(v.Content, out); err != nil {
					return false, fmt.Errorf("flow: decode the approved %s variant: %w", kind, err)
				}
				return true, nil
			}
		}
	}
	return false, nil
}

// priorContent loads the approved audience and plan a step of kind builds on.
func priorContent(ctx context.Context, q *store.Queries, campaignID uuid.UUID, kind string) (aud *AudienceContent, plan *PlanContent, err error) {
	idx := kindIndex(kind)
	if idx >= 1 {
		var a AudienceContent
		ok, err := approvedContent(ctx, q, campaignID, KindAudience, &a)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("flow: the %s step needs an approved audience", kind)
		}
		aud = &a
	}
	if idx >= 2 {
		var p PlanContent
		ok, err := approvedContent(ctx, q, campaignID, KindChannelPlan, &p)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("flow: the %s step needs an approved channel plan", kind)
		}
		plan = &p
	}
	return aud, plan, nil
}

// loadFill reads the state a generation of stepID needs. It returns nil when there is nothing to
// do: the step is not open, or the campaign is not a draft.
func (s *Service) loadFill(ctx context.Context, stepID uuid.UUID) (*fillCtx, error) {
	step, err := s.q.GetStep(ctx, stepID)
	if err != nil {
		return nil, notFound(err)
	}
	if step.Status != StepStatusOpen {
		return nil, nil
	}
	camp, err := s.q.GetCampaign(ctx, step.CampaignID)
	if err != nil {
		return nil, notFound(err)
	}
	if camp.Status != CampaignStatusDrafting {
		return nil, nil
	}
	notes, err := s.q.ListContextNotes(ctx, camp.ID)
	if err != nil {
		return nil, err
	}
	live, err := s.q.ListLiveVariants(ctx, step.ID)
	if err != nil {
		return nil, err
	}
	aud, plan, err := priorContent(ctx, s.q, camp.ID, step.Kind)
	if err != nil {
		return nil, err
	}
	expected, err := expectedSlots(step.Kind, aud, plan)
	if err != nil {
		return nil, err
	}
	clar, questions, answers, err := chatOf(ctx, s.q, step.ID)
	if err != nil {
		return nil, err
	}
	fc := &fillCtx{
		campaign: camp, step: step, noteCount: len(notes), live: live, audience: aud, plan: plan,
		expected: expected, missing: missingSlots(expected, live), rejected: map[Slot]store.ProposalVariant{},
		clarifications: clar, questions: questions, answers: answers,
	}
	for _, n := range notes {
		fc.notes = append(fc.notes, n.Body)
	}
	for _, v := range live {
		if v.Status == VariantStatusRejected {
			fc.rejected[slotOf(v)] = v
		}
	}
	return fc, nil
}

// modelNotes are the notes the model sees for slots: every context note, then the reason of each
// rejected variant these slots replace, as the marketer's remark (D-009). The prompts expect
// notes such as "why a previous proposal was rejected".
func (fc *fillCtx) modelNotes(slots ...Slot) []string {
	out := append([]string(nil), fc.notes...)
	for _, sl := range slots {
		v, ok := fc.rejected[sl]
		if !ok || !v.RejectionReason.Valid {
			continue
		}
		what := strings.ReplaceAll(fc.step.Kind, "_", " ")
		if sl != (Slot{}) {
			what += " (" + sl.String() + ")"
		}
		out = append(out, fmt.Sprintf("The marketer rejected the previous %s proposal. Reason: %s", what, v.RejectionReason.String))
	}
	return out
}

// runFill generates the slots of a step that have no draft or approved variant: the first
// generation, the replacement of a rejected variant, the regeneration after a note, or a retry.
func (s *Service) runFill(ctx context.Context, campaignID, stepID uuid.UUID) {
	fc, err := s.loadFill(ctx, stepID)
	if err != nil {
		if ctx.Err() == nil {
			s.recordFailure(campaignID, stepID, "", Slot{}, err)
		}
		return
	}
	if fc == nil || len(fc.missing) == 0 {
		return
	}
	s.clearFailures(stepID)
	s.hub.Publish(GenerationStarted{Meta: s.meta(campaignID), StepID: stepID, Kind: fc.step.Kind, Slots: fc.missing})
	switch fc.step.Kind {
	case KindAudience:
		s.genAudience(ctx, fc)
	case KindChannelPlan:
		s.genPlan(ctx, fc)
	case KindContent:
		s.genContent(ctx, fc)
	case KindSchedule:
		s.genSchedule(ctx, fc)
	}
}

func (s *Service) genAudience(ctx context.Context, fc *fillCtx) {
	slot := Slot{}
	cctx, col := collect(ctx)
	res, err := s.llm.ProposeSegment(cctx, llm.SegmentInput{
		Brief: fc.campaign.Brief, ContextNotes: fc.modelNotes(slot), Clarifications: fc.clarifications, QuestionsLeft: fc.questionsLeft(),
	}, s.tools)
	if err != nil {
		s.failed(ctx, fc, slot, err, col.Records())
		return
	}
	if res.Question != nil {
		s.askQuestion(ctx, fc, slot, res.Question, col)
		return
	}
	content, err := NewAudienceContent(res.DSL, res.Filter, res.Preview)
	if err == nil {
		err = s.storeVariants(ctx, fc, []newVariant{{slot: slot, summary: res.Summary}}, col, content)
	}
	if err != nil {
		s.failed(ctx, fc, slot, err, col.Records())
	}
}

func (s *Service) genPlan(ctx context.Context, fc *fillCtx) {
	slot := Slot{}
	aud := *fc.audience
	cctx, col := collect(ctx)
	res, err := s.llm.ProposeChannelPlan(cctx, llm.ChannelPlanInput{
		Brief: fc.campaign.Brief, ContextNotes: fc.modelNotes(slot), Coverage: aud.Preview(),
		Clarifications: fc.clarifications, QuestionsLeft: fc.questionsLeft(),
	}, func(p llm.ChannelPlan) error { return llm.NewProblemsError(PlanProblems(p, aud)) })
	if err != nil {
		s.failed(ctx, fc, slot, err, col.Records())
		return
	}
	if res.Question != nil {
		s.askQuestion(ctx, fc, slot, res.Question, col)
		return
	}
	content, err := NewPlanContent(res.Plan, aud)
	if err == nil {
		err = s.storeVariants(ctx, fc, []newVariant{{slot: slot, summary: res.Summary}}, col, content)
	}
	if err != nil {
		s.failed(ctx, fc, slot, err, col.Records())
	}
}

func (s *Service) genSchedule(ctx context.Context, fc *fillCtx) {
	locales, err := scheduleLocales(fc.missing, *fc.audience, *fc.plan)
	if err != nil {
		s.failed(ctx, fc, Slot{}, err, nil)
		return
	}
	cctx, col := collect(ctx)
	res, err := s.llm.ProposeSchedule(cctx, llm.ScheduleInput{
		Brief: fc.campaign.Brief, ContextNotes: fc.modelNotes(fc.missing...), Locales: locales, Now: s.clock.Now(),
		Clarifications: fc.clarifications, QuestionsLeft: fc.questionsLeft(),
	}, func(e []llm.ScheduleEntry) error {
		return llm.NewProblemsError(ScheduleProblems(e, locales, s.clock.Now()))
	})
	if err != nil {
		s.failed(ctx, fc, Slot{}, err, col.Records())
		return
	}
	if res.Question != nil {
		s.askQuestion(ctx, fc, Slot{}, res.Question, col)
		return
	}
	byLocale := map[Slot]llm.ScheduleEntry{}
	for _, e := range res.Entries {
		byLocale[Slot{Language: e.Language, Timezone: e.Timezone}] = e
	}
	items := make([]newVariant, 0, len(fc.missing))
	contents := make([]any, 0, len(fc.missing))
	for i, sl := range fc.missing {
		e, ok := byLocale[sl]
		if !ok {
			s.failed(ctx, fc, sl, fmt.Errorf("the model returned no entry for %s", sl), col.Records())
			return
		}
		item := newVariant{slot: sl}
		if i == 0 {
			// One summary covers every locale; it is posted without a slot.
			item.summary, item.summarySlot = res.Summary, &Slot{}
		}
		items = append(items, item)
		contents = append(contents, ScheduleContent{SendAt: e.SendAt, Rationale: e.Rationale})
	}
	if err := s.storeVariants(ctx, fc, items, col, contents...); err != nil {
		s.failed(ctx, fc, Slot{}, err, col.Records())
	}
}

func (s *Service) genContent(ctx context.Context, fc *fillCtx) {
	for _, slot := range fc.missing {
		if s.content == nil {
			s.recordFailure(fc.campaign.ID, fc.step.ID, fc.step.Kind, slot, ErrNoContentGenerator)
			continue
		}
		if ctx.Err() != nil {
			return
		}
		req := ContentRequest{
			CampaignID: fc.campaign.ID, StepID: fc.step.ID, Language: slot.Language, Channel: slot.Channel,
			Brief: fc.campaign.Brief, ContextNotes: fc.modelNotes(slot), Audience: *fc.audience, Plan: *fc.plan,
			Recipients: fc.plan.recipientsOf(slot),
		}
		if v, ok := fc.rejected[slot]; ok {
			req.RejectionReason = v.RejectionReason.String
		}
		cctx, col := collect(ctx)
		gen, err := s.content.Generate(cctx, req, s.deltaSink(fc.campaign.ID, fc.step.ID, slot))
		if err == nil {
			err = checkObject(gen.Content)
		}
		if err == nil {
			err = s.storeVariants(ctx, fc, []newVariant{{slot: slot, summary: gen.Summary}}, col, json.RawMessage(gen.Content))
			if errors.Is(err, errStale) {
				return // the step changed under us; the job that changed it regenerates
			}
		}
		if err != nil {
			s.failed(ctx, fc, slot, err, col.Records())
		}
	}
}

// deltaSink returns the onDelta callback of a content generation: it publishes every non-empty
// fragment as ContentDelta with its attempt, and ContentReset before the first sign of an attempt
// after the first (D-072). An empty fragment only announces the start of its attempt. The
// generator calls it from one goroutine at a time.
func (s *Service) deltaSink(campaign, step uuid.UUID, slot Slot) func(attempt int, text string) {
	last := 0
	return func(attempt int, text string) {
		if attempt > 1 && attempt > last {
			s.hub.Publish(ContentReset{Meta: s.meta(campaign), StepID: step, Slot: slot, Attempt: attempt})
		}
		if attempt > last {
			last = attempt
		}
		if text == "" {
			return
		}
		s.hub.Publish(ContentDelta{Meta: s.meta(campaign), StepID: step, Slot: slot, Text: text, Attempt: attempt})
	}
}

// recipientsOf returns the messages the plan sends in the slot's language on its channel.
func (p PlanContent) recipientsOf(sl Slot) int {
	for _, l := range p.ByLanguage {
		if l.Language == sl.Language {
			if sl.Channel == string(llm.ChannelSMS) {
				return l.SMS
			}
			return l.Email
		}
	}
	return 0
}

// checkObject accepts only a JSON object as the content of a variant.
func checkObject(raw json.RawMessage) error {
	t := bytes.TrimSpace(raw)
	if !json.Valid(t) || len(t) == 0 || t[0] != '{' {
		return errors.New("the content generator returned something that is not a JSON object")
	}
	return nil
}

// newVariant is one variant to store, with the agent's summary of it when it gave one (D-088): the
// summary is posted in the chat under summarySlot, or the variant's slot when summarySlot is nil.
type newVariant struct {
	slot        Slot
	summary     string
	summarySlot *Slot
}

// errStale reports that the result of a generation was dropped because the state it was made for
// changed: the step is no longer open, a note or an answer arrived, or the slot is filled.
var errStale = errors.New("flow: generation result is stale")

// stale reports whether the state a generation worked from changed under it.
func (fc *fillCtx) stale(ctx context.Context, q *store.Queries, camp store.Campaign, step store.ProposalStep) (bool, error) {
	if camp.Status != CampaignStatusDrafting || step.Status != StepStatusOpen {
		return true, nil
	}
	notes, err := q.ListContextNotes(ctx, camp.ID)
	if err != nil {
		return false, err
	}
	answers, err := countAnswers(ctx, q, step.ID)
	if err != nil {
		return false, err
	}
	return len(notes) != fc.noteCount || answers != fc.answers, nil
}

// askQuestion stores the agent's question as a chat message, publishes QuestionAsked and leaves
// the step open without variants: it waits for Answer (D-088). The call's records are stored in
// the same transaction. A question beyond llm.MaxQuestionsPerStep is not stored: the generation is
// reported failed instead (the adapters refuse it earlier, this is the backstop). A stale state
// drops the question like storeVariants drops a variant.
func (s *Service) askQuestion(ctx context.Context, fc *fillCtx, slot Slot, q *llm.Question, col *llm.Collector) {
	if fc.questions >= llm.MaxQuestionsPerStep {
		s.failed(ctx, fc, slot, fmt.Errorf("the model asked a question after the %d allowed for this step", llm.MaxQuestionsPerStep), col.Records())
		return
	}
	if problems := llm.CheckQuestion(q); len(problems) > 0 {
		s.failed(ctx, fc, slot, fmt.Errorf("the model asked an invalid question: %s", strings.Join(problems, "; ")), col.Records())
		return
	}
	var options []byte
	if q.Options != nil {
		b, err := json.Marshal(q.Options)
		if err != nil {
			s.failed(ctx, fc, slot, fmt.Errorf("encode the question options: %w", err), col.Records())
			return
		}
		options = b
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	recs := col.Records()
	stale := false
	err := s.inTx(ctx, fc.campaign.ID, func(t *txn, camp store.Campaign) error {
		step, err := t.q.GetStep(ctx, fc.step.ID)
		if err != nil {
			return err
		}
		if stale, err = fc.stale(ctx, t.q, camp, step); err != nil {
			return err
		}
		if !stale {
			m, err := t.q.InsertChatMessage(ctx, store.InsertChatMessageParams{
				CampaignID: camp.ID, StepID: &step.ID, Kind: MessageKindQuestion, Body: q.Text, Options: options,
			})
			if err != nil {
				return err
			}
			t.emit(QuestionAsked{Meta: s.meta(camp.ID), StepID: step.ID, MessageID: m.ID, Question: q.Text, Options: q.Options})
		}
		return t.persistCalls(ctx, s, camp.ID, &step.ID, recs, nil)
	})
	if err != nil {
		s.recordFailure(fc.campaign.ID, fc.step.ID, fc.step.Kind, slot, err)
		return
	}
	if stale {
		s.log.Info("flow: dropped a stale question", "campaign", fc.campaign.ID, "step", fc.step.ID)
	}
}

// storeVariants stores the generated variants of one model call together with the call's records
// in one transaction (D-060, D-070). contents has one value per item, or one for a single item.
// When the state changed while the model worked the variants are dropped and errStale is returned;
// the records are stored anyway.
func (s *Service) storeVariants(ctx context.Context, fc *fillCtx, items []newVariant, col *llm.Collector, contents ...any) error {
	if len(contents) != len(items) {
		return fmt.Errorf("flow: %d variants but %d contents", len(items), len(contents))
	}
	encoded := make([][]byte, len(contents))
	for i, c := range contents {
		switch v := c.(type) {
		case json.RawMessage:
			encoded[i] = v
		default:
			b, err := json.Marshal(c)
			if err != nil {
				return fmt.Errorf("flow: encode %s content: %w", fc.step.Kind, err)
			}
			encoded[i] = b
		}
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	recs := col.Records()
	stale := false
	err := s.inTx(ctx, fc.campaign.ID, func(t *txn, camp store.Campaign) error {
		step, err := t.q.GetStep(ctx, fc.step.ID)
		if err != nil {
			return err
		}
		if stale, err = fc.stale(ctx, t.q, camp, step); err != nil {
			return err
		}
		var made []store.ProposalVariant
		if !stale {
			live, err := t.q.ListLiveVariants(ctx, step.ID)
			if err != nil {
				return err
			}
			have := filled(live)
			for i, it := range items {
				if have[it.slot] {
					stale = true
					continue
				}
				for _, v := range live {
					if v.Status == VariantStatusRejected && slotOf(v) == it.slot {
						if _, err := t.q.SupersedeVariant(ctx, v.ID); err != nil {
							return err
						}
						t.emit(VariantSuperseded{Meta: s.meta(camp.ID), StepID: step.ID, VariantID: v.ID})
					}
				}
				v, err := t.q.CreateVariant(ctx, store.CreateVariantParams{
					StepID: step.ID, Kind: step.Kind, Language: text(it.slot.Language), Channel: text(it.slot.Channel), Timezone: text(it.slot.Timezone), Content: encoded[i],
				})
				if err != nil {
					return err
				}
				made = append(made, v)
				t.emit(VariantCreated{
					Meta: s.meta(camp.ID), StepID: step.ID, VariantID: v.ID, Kind: step.Kind, Slot: it.slot, Content: v.Content,
				})
				if it.summary != "" {
					sl := it.slot
					if it.summarySlot != nil {
						sl = *it.summarySlot
					}
					if err := t.postSummary(ctx, s, camp.ID, step.ID, sl, it.summary); err != nil {
						return err
					}
				}
			}
		}
		var accepted *uuid.UUID
		if len(made) == 1 {
			accepted = &made[0].ID
		}
		return t.persistCalls(ctx, s, camp.ID, &step.ID, recs, accepted)
	})
	if err != nil {
		return err
	}
	if stale {
		s.log.Info("flow: dropped a stale generation result", "campaign", fc.campaign.ID, "step", fc.step.ID)
		return errStale
	}
	return nil
}

// postSummary stores the agent's summary of a generated variant as a chat message and emits
// SummaryPosted (D-088). A summary that fails llm.CheckSummary is logged and dropped; the variant
// stands.
func (t *txn) postSummary(ctx context.Context, s *Service, campaign, step uuid.UUID, slot Slot, markdown string) error {
	md, problems := llm.CheckSummary(markdown)
	if len(problems) > 0 {
		s.log.Warn("flow: dropping the summary the model gave", "campaign", campaign, "step", step, "problems", strings.Join(problems, "; "))
		return nil
	}
	m, err := t.q.InsertChatMessage(ctx, store.InsertChatMessageParams{CampaignID: campaign, StepID: &step, Kind: MessageKindSummary, Body: md})
	if err != nil {
		return err
	}
	t.emit(SummaryPosted{Meta: s.meta(campaign), StepID: step, MessageID: m.ID, Slot: slot, Markdown: md})
	return nil
}

// text returns a nullable text value.
func text(v string) pgtype.Text { return pgtype.Text{String: v, Valid: v != ""} }

// persistCalls inserts the records of a model call. The last record of an accepted call names the
// variant it produced. Each insert runs in a savepoint: a record the table refuses is logged and
// skipped instead of losing the variant.
func (t *txn) persistCalls(ctx context.Context, s *Service, campaign uuid.UUID, step *uuid.UUID, recs []llm.CallRecord, variant *uuid.UUID) error {
	for i, r := range recs {
		var vid *uuid.UUID
		if variant != nil && i == len(recs)-1 && r.Outcome == llm.OutcomeAccepted {
			vid = variant
		}
		sp, err := t.tx.Begin(ctx)
		if err != nil {
			return err
		}
		if err := insertCall(ctx, sp, r.StoreParams(campaign, step, vid)); err != nil {
			s.log.Error("flow: cannot store an llm_call record", "campaign", campaign, "purpose", r.Purpose, "error", err)
			if rerr := sp.Rollback(ctx); rerr != nil {
				return rerr
			}
			continue
		}
		if err := sp.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

// persistLoose stores records that belong to no variant: a screen call, a failed generation.
func (s *Service) persistLoose(ctx context.Context, campaign uuid.UUID, step *uuid.UUID, recs []llm.CallRecord) {
	if len(recs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	for _, r := range recs {
		if err := s.insertLooseCall(ctx, r.StoreParams(campaign, step, nil)); err != nil {
			s.log.Error("flow: cannot store an llm_call record", "campaign", campaign, "purpose", r.Purpose, "error", err)
		}
	}
}

// insertLooseCall stores one record and its token_completed event in a transaction of their own.
func (s *Service) insertLooseCall(ctx context.Context, p store.InsertLLMCallParams) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := insertCall(ctx, tx, p); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertCall inserts the llm_call row through tx and, in the same transaction, the
// evt.proposal.token_completed.v1 envelope that tells Analytics about it (D-083). When either
// write fails the caller rolls tx back, so a row never exists without its event or the reverse.
func insertCall(ctx context.Context, tx pgx.Tx, p store.InsertLLMCallParams) error {
	row, err := store.New(tx).InsertLLMCall(ctx, p)
	if err != nil {
		return err
	}
	data := TokenCompletedData{
		LLMCallID: row.ID.String(), CampaignID: row.CampaignID.String(),
		Purpose: row.Purpose, Model: row.Model, PromptName: row.PromptName, PromptVersion: row.PromptVersion,
		Tokens: TokenCounts{
			Input: int64(row.InputTokens), Output: int64(row.OutputTokens),
			CacheRead: int64(row.CacheReadTokens), CacheCreation: int64(row.CacheCreationTokens),
		},
		Outcome: row.Outcome, LatencyMs: int64(row.LatencyMs), CreatedAt: row.CreatedAt.Time.UTC(),
	}
	if row.StepID != nil {
		id := row.StepID.String()
		data.StepID = &id
	}
	env, err := events.New(EventTypeTokenCompleted, EventVersion, EventProducer, row.CampaignID.String(), data)
	if err != nil {
		return err
	}
	_, err = outbox.Write(ctx, tx, env)
	return err
}

// failed stores the records of a failed generation and reports the failure. A cancelled context
// (shutdown) is not a failure of the step.
func (s *Service) failed(ctx context.Context, fc *fillCtx, slot Slot, err error, recs []llm.CallRecord) {
	if errors.Is(err, errStale) {
		return // storeVariants stored the records already
	}
	s.persistLoose(ctx, fc.campaign.ID, &fc.step.ID, recs)
	if ctx.Err() != nil {
		return
	}
	s.recordFailure(fc.campaign.ID, fc.step.ID, fc.step.Kind, slot, err)
}

// recordFailure remembers why a generation failed and publishes GenerationFailed. kind may be
// empty; it is then looked up.
func (s *Service) recordFailure(campaign, step uuid.UUID, kind string, slot Slot, err error) {
	if kind == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if st, gerr := s.q.GetStep(ctx, step); gerr == nil {
			kind = st.Kind
		}
		cancel()
	}
	reason := err.Error()
	s.setFailure(step, slot, reason)
	s.log.Warn("flow: generation failed", "campaign", campaign, "step", step, "kind", kind, "slot", slot.String(), "reason", reason)
	s.hub.Publish(GenerationFailed{Meta: s.meta(campaign), StepID: step, Kind: kind, Slot: slot, Reason: reason})
}
