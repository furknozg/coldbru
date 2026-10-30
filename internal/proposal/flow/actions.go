package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// CreateCampaign stores a campaign in the drafting state and returns at once; the brief is
// screened and the audience proposed in the background (D-069, D-070). A blocked brief cancels the
// campaign and publishes why; the marketer edits the brief by creating a new campaign.
func (s *Service) CreateCampaign(ctx context.Context, brief string) (CampaignInfo, error) {
	if strings.TrimSpace(brief) == "" {
		return CampaignInfo{}, fmt.Errorf("%w: the brief is empty", ErrInvalidInput)
	}
	c, err := s.q.CreateCampaign(ctx, brief)
	if err != nil {
		return CampaignInfo{}, err
	}
	s.enqueue(job{kind: jobStart, campaign: c.ID})
	return campaignInfo(c), nil
}

// RegenerateCampaign restarts the screen and audience pipeline of a campaign that has none: one
// that was created but whose process stopped before the audience step opened. It does what
// CreateCampaign does after the insert and returns at once; a block or a screen failure cancels the
// campaign as before. A call while the pipeline is already running is a no-op. Errors: ErrNotFound,
// ErrNotDrafting (cancelled or confirmed), ErrHasSteps (use Regenerate on the open step).
func (s *Service) RegenerateCampaign(ctx context.Context, campaignID uuid.UUID) error {
	camp, err := s.q.GetCampaign(ctx, campaignID)
	if err != nil {
		return notFound(err)
	}
	if camp.Status != CampaignStatusDrafting {
		return ErrNotDrafting
	}
	steps, err := s.q.ListSteps(ctx, campaignID)
	if err != nil {
		return err
	}
	if len(steps) > 0 {
		return ErrHasSteps
	}
	s.enqueue(job{kind: jobStart, campaign: campaignID})
	return nil
}

// lookup finds a variant and its step without a lock, for the checks that precede a transaction.
func (s *Service) lookup(ctx context.Context, variantID uuid.UUID) (store.ProposalVariant, store.ProposalStep, error) {
	v, err := s.q.GetVariant(ctx, variantID)
	if err != nil {
		return store.ProposalVariant{}, store.ProposalStep{}, notFound(err)
	}
	st, err := s.q.GetStep(ctx, v.StepID)
	if err != nil {
		return store.ProposalVariant{}, store.ProposalStep{}, notFound(err)
	}
	return v, st, nil
}

// checkDraft re-reads the variant and its step inside the transaction and returns them when the
// step is open and the variant is a draft.
func checkDraft(ctx context.Context, t *txn, camp store.Campaign, variantID uuid.UUID) (store.ProposalVariant, store.ProposalStep, error) {
	v, err := t.q.GetVariant(ctx, variantID)
	if err != nil {
		return v, store.ProposalStep{}, notFound(err)
	}
	st, err := t.q.GetStep(ctx, v.StepID)
	if err != nil {
		return v, st, notFound(err)
	}
	if camp.Status != CampaignStatusDrafting {
		return v, st, ErrNotDrafting
	}
	if st.Status != StepStatusOpen {
		return v, st, ErrNotOpen
	}
	if v.Status != VariantStatusDraft {
		return v, st, ErrNotDraft
	}
	return v, st, nil
}

// Approve approves a draft variant. When it was the last slot of its step the step is approved
// (D-009), the next step opens and its generation starts; approving the schedule step calls the
// Confirmer once. Errors: ErrNotFound, ErrNotOpen, ErrNotDraft, ErrNotDrafting, and ErrConfirm
// when only the Confirmer failed (the approval stands).
func (s *Service) Approve(ctx context.Context, variantID uuid.UUID) error {
	_, st0, err := s.lookup(ctx, variantID)
	if err != nil {
		return err
	}
	var (
		next    *store.ProposalStep
		confirm bool
	)
	err = s.inTx(ctx, st0.CampaignID, func(t *txn, camp store.Campaign) error {
		v, st, err := checkDraft(ctx, t, camp, variantID)
		if err != nil {
			return err
		}
		if _, err := t.q.ApproveVariant(ctx, v.ID); err != nil {
			return err
		}
		t.emit(VariantApproved{Meta: s.meta(camp.ID), StepID: st.ID, VariantID: v.ID})

		aud, plan, err := priorContent(ctx, t.q, camp.ID, st.Kind)
		if err != nil {
			return err
		}
		expected, err := expectedSlots(st.Kind, aud, plan)
		if err != nil {
			return err
		}
		live, err := t.q.ListLiveVariants(ctx, st.ID)
		if err != nil {
			return err
		}
		if !stepComplete(expected, live) {
			return nil
		}
		done, err := t.q.ApproveStep(ctx, st.ID)
		if err != nil {
			return err
		}
		t.emit(StepApproved{Meta: s.meta(camp.ID), StepID: done.ID, Kind: done.Kind, Version: int(done.Version)})
		if idx := kindIndex(done.Kind); idx+1 < len(kindOrder) {
			ns, err := t.openStep(ctx, s, camp.ID, kindOrder[idx+1])
			if err != nil {
				return err
			}
			next = &ns
		} else {
			confirm = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if next != nil {
		s.enqueue(job{kind: jobFill, campaign: next.CampaignID, step: next.ID})
	}
	if confirm {
		if err := s.confirm.Confirm(ctx, st0.CampaignID); err != nil {
			return fmt.Errorf("%w: %w", ErrConfirm, err)
		}
	}
	return nil
}

// Reject rejects a draft variant with a reason, which is required. Only its slot is regenerated,
// with the reason given to the model; the rejected variant becomes superseded when its
// replacement exists (D-009, D-022).
func (s *Service) Reject(ctx context.Context, variantID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("%w: a rejection needs a reason", ErrInvalidInput)
	}
	_, st0, err := s.lookup(ctx, variantID)
	if err != nil {
		return err
	}
	err = s.inTx(ctx, st0.CampaignID, func(t *txn, camp store.Campaign) error {
		v, st, err := checkDraft(ctx, t, camp, variantID)
		if err != nil {
			return err
		}
		if _, err := t.q.RejectVariant(ctx, store.RejectVariantParams{ID: v.ID, RejectionReason: text(reason)}); err != nil {
			return err
		}
		t.emit(VariantRejected{Meta: s.meta(camp.ID), StepID: st.ID, VariantID: v.ID, Reason: reason})
		return nil
	})
	if err != nil {
		return err
	}
	s.enqueue(job{kind: jobFill, campaign: st0.CampaignID, step: st0.ID})
	return nil
}

// Edit replaces a draft variant by hand (D-070). The new content passes the validators the
// generated content passes: an audience edit is a new DSL that is recompiled and recounted, a plan
// edit a mode and primary checked against the audience, a schedule edit a send time inside the
// window with the right offset for the slot's time zone, and a content edit whatever the ContentGenerator accepts. It becomes a new draft variant
// in the same slot and the old one is superseded. content is a JSON object: {"dsl": ...} for the
// audience, {"mode", "primary", "rationale"} for the plan, {"send_at", "rationale"} for the
// schedule (the send time is read in the slot's time zone, D-076), the generator's own shape for
// the content. Invalid content returns *InvalidContentError
// (errors.Is ErrInvalidContent) and nothing changes.
func (s *Service) Edit(ctx context.Context, variantID uuid.UUID, content json.RawMessage) (VariantView, error) {
	v0, st0, err := s.lookup(ctx, variantID)
	if err != nil {
		return VariantView{}, err
	}
	if st0.Status != StepStatusOpen {
		return VariantView{}, ErrNotOpen
	}
	if v0.Status != VariantStatusDraft {
		return VariantView{}, ErrNotDraft
	}
	built, err := s.buildEdit(ctx, v0, st0, content)
	if err != nil {
		return VariantView{}, err
	}
	var out store.ProposalVariant
	err = s.inTx(ctx, st0.CampaignID, func(t *txn, camp store.Campaign) error {
		old, st, err := checkDraft(ctx, t, camp, variantID)
		if err != nil {
			return err
		}
		if _, err := t.q.SupersedeVariant(ctx, old.ID); err != nil {
			return err
		}
		t.emit(VariantSuperseded{Meta: s.meta(camp.ID), StepID: st.ID, VariantID: old.ID})
		out, err = t.q.CreateVariant(ctx, store.CreateVariantParams{
			StepID: st.ID, Kind: st.Kind, Language: old.Language, Channel: old.Channel, Timezone: old.Timezone, Content: built,
		})
		if err != nil {
			return err
		}
		t.emit(VariantCreated{Meta: s.meta(camp.ID), StepID: st.ID, VariantID: out.ID, Kind: st.Kind, Slot: slotOf(out), Content: out.Content})
		return nil
	})
	if err != nil {
		return VariantView{}, err
	}
	return variantView(out), nil
}

// buildEdit validates edited content and returns the variant content to store.
func (s *Service) buildEdit(ctx context.Context, v store.ProposalVariant, st store.ProposalStep, content json.RawMessage) (json.RawMessage, error) {
	invalid := func(problems ...string) error { return &InvalidContentError{Problems: problems} }
	if err := checkObject(content); err != nil {
		return nil, invalid("the content must be a JSON object")
	}
	aud, plan, err := priorContent(ctx, s.q, st.CampaignID, st.Kind)
	if err != nil {
		return nil, err
	}
	switch st.Kind {
	case KindAudience:
		var in struct {
			DSL string `json:"dsl"`
		}
		if err := json.Unmarshal(content, &in); err != nil || strings.TrimSpace(in.DSL) == "" {
			return nil, invalid(`the audience needs a non-empty "dsl" string`)
		}
		f, p, errs, err := s.tools.Validate(ctx, in.DSL)
		if err != nil {
			return nil, fmt.Errorf("flow: validate the edited filter: %w", err)
		}
		if len(errs) > 0 {
			problems := make([]string, len(errs))
			for i, e := range errs {
				problems[i] = e.Error()
			}
			return nil, invalid(problems...)
		}
		c, err := NewAudienceContent(in.DSL, f, p)
		if err != nil {
			return nil, err
		}
		return json.Marshal(c)
	case KindChannelPlan:
		var in struct {
			Mode      llm.PlanMode `json:"mode"`
			Primary   llm.Channel  `json:"primary"`
			Rationale string       `json:"rationale"`
		}
		if err := json.Unmarshal(content, &in); err != nil {
			return nil, invalid(`the plan needs "mode" and "primary" strings`)
		}
		p := llm.ChannelPlan{Mode: in.Mode, Primary: in.Primary, Rationale: in.Rationale}
		if problems := PlanProblems(p, *aud); len(problems) > 0 {
			return nil, invalid(problems...)
		}
		c, err := NewPlanContent(p, *aud)
		if err != nil {
			return nil, err
		}
		return json.Marshal(c)
	case KindContent:
		if s.content == nil {
			return nil, ErrNoContentGenerator
		}
		slot := slotOf(v)
		built, err := s.content.Validate(ContentRequest{
			CampaignID: st.CampaignID, StepID: st.ID, Language: slot.Language, Channel: slot.Channel,
			Audience: *aud, Plan: *plan, Recipients: plan.recipientsOf(slot),
		}, content)
		if err != nil {
			return nil, invalid(llm.Problems(err)...)
		}
		if err := checkObject(built); err != nil {
			return nil, fmt.Errorf("flow: validated edit: %w", err)
		}
		return built, nil
	case KindSchedule:
		var in struct {
			SendAt    json.RawMessage `json:"send_at"`
			Rationale string          `json:"rationale"`
		}
		if err := json.Unmarshal(content, &in); err != nil || len(in.SendAt) == 0 {
			return nil, invalid(`the schedule needs "send_at": an RFC 3339 time with an offset, or null`)
		}
		var sc ScheduleContent
		if string(in.SendAt) != "null" {
			if err := json.Unmarshal(in.SendAt, &sc.SendAt); err != nil {
				return nil, invalid(`send_at must be an RFC 3339 time with an offset, or null`)
			}
		}
		sc.Rationale = in.Rationale
		entry := llm.ScheduleEntry{Language: v.Language.String, Timezone: v.Timezone.String, SendAt: sc.SendAt}
		locale := llm.ScheduleLocale{Language: entry.Language, Timezone: entry.Timezone}
		if problems := ScheduleProblems([]llm.ScheduleEntry{entry}, []llm.ScheduleLocale{locale}, s.clock.Now()); len(problems) > 0 {
			return nil, invalid(problems...)
		}
		return json.Marshal(sc)
	}
	return nil, fmt.Errorf("flow: unknown step kind %q", st.Kind)
}

// AddContext records a note on the open step (D-009). Its draft and rejected variants are
// superseded and regenerated with the note; approved variants are kept; later steps see the note
// too. ErrNotOpen when the campaign has no open step.
func (s *Service) AddContext(ctx context.Context, campaignID uuid.UUID, body string) (NoteView, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return NoteView{}, fmt.Errorf("%w: the note is empty", ErrInvalidInput)
	}
	var (
		note store.ContextNote
		step store.ProposalStep
	)
	err := s.inTx(ctx, campaignID, func(t *txn, camp store.Campaign) error {
		if camp.Status != CampaignStatusDrafting {
			return ErrNotDrafting
		}
		open, err := openStepOf(ctx, t.q, camp.ID)
		if err != nil {
			return err
		}
		step = open
		note, err = t.q.InsertContextNote(ctx, store.InsertContextNoteParams{CampaignID: camp.ID, StepID: &step.ID, Body: body})
		if err != nil {
			return err
		}
		t.emit(ContextAdded{Meta: s.meta(camp.ID), StepID: step.ID, NoteID: note.ID, Body: body})
		live, err := t.q.ListLiveVariants(ctx, step.ID)
		if err != nil {
			return err
		}
		for _, v := range live {
			if v.Status == VariantStatusApproved {
				continue
			}
			if _, err := t.q.SupersedeVariant(ctx, v.ID); err != nil {
				return err
			}
			t.emit(VariantSuperseded{Meta: s.meta(camp.ID), StepID: step.ID, VariantID: v.ID})
		}
		return nil
	})
	if err != nil {
		return NoteView{}, err
	}
	s.enqueue(job{kind: jobFill, campaign: campaignID, step: step.ID})
	return noteView(note), nil
}

// Answer records the marketer's answer to a question of the agent and generates the step again
// with it (D-088): the answer becomes a chat message that replies to the question, the question
// and the answer go to the model as a clarification, and the step's empty slots are filled like
// Regenerate does. It returns at once. Errors: ErrInvalidInput (empty answer), ErrNotFound,
// ErrNotQuestion, ErrAlreadyAnswered, ErrNotOpen (the step of the question is not open),
// ErrNotDrafting.
func (s *Service) Answer(ctx context.Context, questionID uuid.UUID, answer string) (ChatMessageView, error) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return ChatMessageView{}, fmt.Errorf("%w: the answer is empty", ErrInvalidInput)
	}
	q, err := s.q.GetChatMessage(ctx, questionID)
	if err != nil {
		return ChatMessageView{}, notFound(err)
	}
	if q.Kind != MessageKindQuestion || q.StepID == nil {
		return ChatMessageView{}, ErrNotQuestion
	}
	var out store.ChatMessage
	err = s.inTx(ctx, q.CampaignID, func(t *txn, camp store.Campaign) error {
		if camp.Status != CampaignStatusDrafting {
			return ErrNotDrafting
		}
		st, err := t.q.GetStep(ctx, *q.StepID)
		if err != nil {
			return notFound(err)
		}
		if st.Status != StepStatusOpen {
			return ErrNotOpen
		}
		if _, err := t.q.GetAnswerOfQuestion(ctx, &q.ID); err == nil {
			return ErrAlreadyAnswered
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		out, err = t.q.InsertChatMessage(ctx, store.InsertChatMessageParams{
			CampaignID: camp.ID, StepID: &st.ID, Kind: MessageKindAnswer, Body: answer, ReplyTo: &q.ID,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrAlreadyAnswered // a concurrent answer won the row lock race
			}
			return err
		}
		t.emit(AnswerGiven{Meta: s.meta(camp.ID), StepID: st.ID, MessageID: out.ID, QuestionID: q.ID, Answer: answer})
		return nil
	})
	if err != nil {
		return ChatMessageView{}, err
	}
	s.enqueue(job{kind: jobFill, campaign: q.CampaignID, step: *q.StepID})
	return messageView(out), nil
}

// openStepOf returns the open step of a campaign, ErrNotOpen when there is none.
func openStepOf(ctx context.Context, q *store.Queries, campaignID uuid.UUID) (store.ProposalStep, error) {
	steps, err := q.ListLiveSteps(ctx, campaignID)
	if err != nil {
		return store.ProposalStep{}, err
	}
	for _, st := range steps {
		if st.Status == StepStatusOpen {
			return st, nil
		}
	}
	return store.ProposalStep{}, ErrNotOpen
}

// Regenerate retries the generation of an open step: after a failure, or after a restart that
// interrupted it. Slots that have a draft or approved variant are left alone. It returns at once.
func (s *Service) Regenerate(ctx context.Context, stepID uuid.UUID) error {
	st, err := s.q.GetStep(ctx, stepID)
	if err != nil {
		return notFound(err)
	}
	if st.Status != StepStatusOpen {
		return ErrNotOpen
	}
	camp, err := s.q.GetCampaign(ctx, st.CampaignID)
	if err != nil {
		return notFound(err)
	}
	if camp.Status != CampaignStatusDrafting {
		return ErrNotDrafting
	}
	s.enqueue(job{kind: jobFill, campaign: st.CampaignID, step: st.ID})
	return nil
}

// Reopen revises an approved step (D-010): the step and every later one are superseded, with their
// variants, a new version of the step opens and its generation starts. ErrNotFound when the
// campaign has no step of kind, ErrNotApproved when that step is not approved.
func (s *Service) Reopen(ctx context.Context, campaignID uuid.UUID, kind string) (StepInfo, error) {
	idx := kindIndex(kind)
	if idx < 0 {
		return StepInfo{}, fmt.Errorf("%w: unknown step kind %q", ErrInvalidInput, kind)
	}
	var opened store.ProposalStep
	err := s.inTx(ctx, campaignID, func(t *txn, camp store.Campaign) error {
		if camp.Status != CampaignStatusDrafting {
			return ErrNotDrafting
		}
		steps, err := t.q.ListLiveSteps(ctx, camp.ID)
		if err != nil {
			return err
		}
		var target *store.ProposalStep
		for i := range steps {
			if steps[i].Kind == kind {
				target = &steps[i]
			}
		}
		if target == nil {
			return ErrNotFound
		}
		if target.Status != StepStatusApproved {
			return ErrNotApproved
		}
		for _, st := range steps {
			if kindIndex(st.Kind) < idx {
				continue
			}
			vs, err := t.q.ListLiveVariants(ctx, st.ID)
			if err != nil {
				return err
			}
			for _, v := range vs {
				if _, err := t.q.SupersedeVariant(ctx, v.ID); err != nil {
					return err
				}
				t.emit(VariantSuperseded{Meta: s.meta(camp.ID), StepID: st.ID, VariantID: v.ID})
			}
			if _, err := t.q.SupersedeStep(ctx, st.ID); err != nil {
				return err
			}
			t.emit(StepSuperseded{Meta: s.meta(camp.ID), StepID: st.ID, Kind: st.Kind, Version: int(st.Version)})
		}
		opened, err = t.openStep(ctx, s, camp.ID, kind)
		return err
	})
	if err != nil {
		return StepInfo{}, err
	}
	s.enqueue(job{kind: jobFill, campaign: campaignID, step: opened.ID})
	return stepInfo(opened), nil
}

// Confirm runs the Confirmer again for a campaign that is still `drafting`: the retry path after
// Approve returned ErrConfirm (D-070, D-074). The Confirmer checks that the schedule step is
// approved (ErrScheduleNotApproved) and that the plan reaches somebody (ErrNoRecipients). Errors:
// ErrNotFound, ErrNotDrafting (also for a campaign that is already scheduled), and ErrConfirm
// wrapping the Confirmer's own error.
func (s *Service) Confirm(ctx context.Context, campaignID uuid.UUID) error {
	camp, err := s.q.GetCampaign(ctx, campaignID)
	if err != nil {
		return notFound(err)
	}
	if camp.Status != CampaignStatusDrafting {
		return ErrNotDrafting
	}
	if err := s.confirm.Confirm(ctx, campaignID); err != nil {
		return fmt.Errorf("%w: %w", ErrConfirm, err)
	}
	return nil
}

// DefaultCancelReason is the reason recorded when the marketer gives none.
const DefaultCancelReason = "cancelled by the marketer"

// Cancel cancels a campaign (D-033, D-074). A `drafting` campaign only changes status: no event
// leaves the service, and a generation that is still running finds the campaign no longer a draft
// and drops its result. A `scheduled` or `sending` campaign also writes evt.proposal.cancelled.v1
// to the outbox in the same transaction, so that Notification stops sending. A `completed` or
// already `cancelled` campaign is ErrNotCancellable. reason may be empty (DefaultCancelReason);
// the caller limits its length. Subscribers get a campaign_cancelled event.
func (s *Service) Cancel(ctx context.Context, campaignID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = DefaultCancelReason
	}
	return s.inTx(ctx, campaignID, func(t *txn, camp store.Campaign) error {
		publish := false
		switch camp.Status {
		case CampaignStatusDrafting:
		case CampaignStatusScheduled, CampaignStatusSending:
			publish = true
		default:
			return ErrNotCancellable
		}
		if _, err := t.q.SetCampaignStatus(ctx, store.SetCampaignStatusParams{ID: camp.ID, Status: CampaignStatusCancelled}); err != nil {
			return err
		}
		if publish {
			env, err := events.New(EventTypeCancelled, EventVersion, EventProducer, camp.ID.String(),
				CancelledData{CampaignID: camp.ID.String(), Reason: reason})
			if err != nil {
				return err
			}
			if _, err := outbox.Write(ctx, t.tx, env); err != nil {
				return err
			}
		}
		t.emit(CampaignCancelled{Meta: s.meta(camp.ID), Reason: reason})
		return nil
	})
}
