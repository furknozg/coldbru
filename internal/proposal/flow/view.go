package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

func ts(t pgtype.Timestamptz) time.Time { return t.Time }

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func campaignInfo(c store.Campaign) CampaignInfo {
	return CampaignInfo{ID: c.ID, Brief: c.Brief, Title: c.Title.String, Status: c.Status, CreatedAt: ts(c.CreatedAt), UpdatedAt: ts(c.UpdatedAt)}
}

// messageView converts a chat_message row. Options that do not decode (never, the table checks the
// shape) are left out.
func messageView(m store.ChatMessage) ChatMessageView {
	out := ChatMessageView{
		ID: m.ID, CampaignID: m.CampaignID, StepID: m.StepID, Kind: m.Kind, Body: m.Body, ReplyTo: m.ReplyTo, CreatedAt: ts(m.CreatedAt),
	}
	if len(m.Options) > 0 {
		_ = json.Unmarshal(m.Options, &out.Options)
	}
	return out
}

// pendingQuestions returns, per step, the latest question that has no answer (D-088).
func pendingQuestions(msgs []ChatMessageView) map[uuid.UUID]*ChatMessageView {
	answered := map[uuid.UUID]bool{}
	for _, m := range msgs {
		if m.Kind == MessageKindAnswer && m.ReplyTo != nil {
			answered[*m.ReplyTo] = true
		}
	}
	out := map[uuid.UUID]*ChatMessageView{}
	for i := range msgs {
		m := &msgs[i]
		if m.Kind == MessageKindQuestion && m.StepID != nil && !answered[m.ID] {
			out[*m.StepID] = m // messages are oldest first, so the last one wins
		}
	}
	return out
}

func stepInfo(st store.ProposalStep) StepInfo {
	return StepInfo{
		ID: st.ID, CampaignID: st.CampaignID, Kind: st.Kind, Version: int(st.Version), Status: st.Status,
		CreatedAt: ts(st.CreatedAt), ApprovedAt: tsPtr(st.ApprovedAt),
	}
}

func variantView(v store.ProposalVariant) VariantView {
	return VariantView{
		ID: v.ID, StepID: v.StepID, Kind: v.Kind, Language: v.Language.String, Channel: v.Channel.String, Timezone: v.Timezone.String, Status: v.Status,
		Content: json.RawMessage(v.Content), RejectionReason: v.RejectionReason.String,
		CreatedAt: ts(v.CreatedAt), ApprovedAt: tsPtr(v.ApprovedAt), RejectedAt: tsPtr(v.RejectedAt),
	}
}

func noteView(n store.ContextNote) NoteView {
	return NoteView{ID: n.ID, StepID: n.StepID, Body: n.Body, CreatedAt: ts(n.CreatedAt)}
}

// failureOfCalls derives from the newest llm_call rows of a step whether its last generation
// failed (D-070): the newest row is a refusal, an API error, or a validation failure that no later
// repair fixed. rows are newest first. The reason is the row's error text.
func failureOfCalls(rows []store.LlmCall) (failed bool, reason string) {
	if len(rows) == 0 {
		return false, ""
	}
	r := rows[0]
	switch llm.Outcome(r.Outcome) {
	case llm.OutcomeRefusal, llm.OutcomeAPIError, llm.OutcomeValidationFailed:
		reason = r.Error.String
		if reason == "" {
			reason = r.Outcome
		}
		return true, fmt.Sprintf("%s: %s: %s", r.Purpose, r.Outcome, strings.TrimSpace(reason))
	}
	return false, ""
}

// CampaignOfVariant returns the campaign a variant belongs to; ErrNotFound when there is no such
// variant.
func (s *Service) CampaignOfVariant(ctx context.Context, variantID uuid.UUID) (uuid.UUID, error) {
	_, st, err := s.lookup(ctx, variantID)
	if err != nil {
		return uuid.Nil, err
	}
	return st.CampaignID, nil
}

// CampaignOfStep returns the campaign a step belongs to; ErrNotFound when there is no such step.
func (s *Service) CampaignOfStep(ctx context.Context, stepID uuid.UUID) (uuid.UUID, error) {
	st, err := s.q.GetStep(ctx, stepID)
	if err != nil {
		return uuid.Nil, notFound(err)
	}
	return st.CampaignID, nil
}

// ListCampaigns returns up to limit campaigns, newest first, skipping offset (D-072). The caller
// caps limit; a negative offset or a limit below 1 is ErrInvalidInput.
func (s *Service) ListCampaigns(ctx context.Context, limit, offset int) ([]CampaignInfo, error) {
	if limit < 1 || offset < 0 || limit > math.MaxInt32 || offset > math.MaxInt32 {
		return nil, fmt.Errorf("%w: limit %d, offset %d", ErrInvalidInput, limit, offset)
	}
	rows, err := s.q.ListCampaigns(ctx, store.ListCampaignsParams{Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, err
	}
	out := make([]CampaignInfo, len(rows))
	for i, c := range rows {
		out[i] = campaignInfo(c)
	}
	return out, nil
}

// Get returns the read model of a campaign: the campaign, its current steps in step order with
// their live variants, all notes, and per open step whether it is being generated, whether the
// last generation failed and why, and which slots are still empty. The reads share one snapshot.
func (s *Service) Get(ctx context.Context, campaignID uuid.UUID) (*CampaignView, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := s.q.WithTx(tx)

	camp, err := q.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, notFound(err)
	}
	out := &CampaignView{CampaignInfo: campaignInfo(camp), Steps: []StepView{}, Notes: []NoteView{}, Messages: []ChatMessageView{}}
	notes, err := q.ListContextNotes(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		out.Notes = append(out.Notes, noteView(n))
	}
	msgs, err := q.ListChatMessages(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	for _, m := range msgs {
		out.Messages = append(out.Messages, messageView(m))
	}
	pending := pendingQuestions(out.Messages)
	steps, err := q.ListLiveSteps(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	// ListLiveSteps is by creation time; a reopened audience is newer than an old plan, but the
	// cascade supersedes everything after it, so kind order and creation order agree.
	var (
		plan     PlanContent
		aud      AudienceContent
		havePlan bool
		haveAud  bool
	)
	views := make([]StepView, 0, len(steps))
	for _, st := range steps {
		vs, err := q.ListLiveVariants(ctx, st.ID)
		if err != nil {
			return nil, err
		}
		sv := StepView{StepInfo: stepInfo(st), Variants: make([]VariantView, 0, len(vs))}
		for _, v := range vs {
			sv.Variants = append(sv.Variants, variantView(v))
			if st.Status != StepStatusApproved || v.Status != VariantStatusApproved {
				continue
			}
			switch st.Kind {
			case KindAudience:
				haveAud = json.Unmarshal(v.Content, &aud) == nil
			case KindChannelPlan:
				havePlan = json.Unmarshal(v.Content, &plan) == nil
			}
		}
		views = append(views, sv)
	}
	for i := range views {
		sv := &views[i]
		if sv.Status != StepStatusOpen {
			continue
		}
		var (
			a *AudienceContent
			p *PlanContent
		)
		if haveAud {
			a = &aud
		}
		if havePlan {
			p = &plan
		}
		if expected, err := expectedSlots(sv.Kind, a, p); err == nil {
			have := map[Slot]bool{}
			for _, v := range sv.Variants {
				if v.Status == VariantStatusDraft || v.Status == VariantStatusApproved {
					have[Slot{Language: v.Language, Channel: v.Channel, Timezone: v.Timezone}] = true
				}
			}
			sv.Missing = missingFrom(expected, have)
		}
		sv.Generating = s.isGenerating(sv.ID)
		if pq := pending[sv.ID]; pq != nil {
			cp := *pq
			sv.PendingQuestion = &cp
		}
		if reason := s.failureOf(sv.ID); reason != "" {
			sv.Failed, sv.Reason = true, reason
		} else if !sv.Generating {
			rows, err := q.ListLatestLLMCallsOfStep(ctx, store.ListLatestLLMCallsOfStepParams{StepID: &sv.ID, Limit: 3})
			if err != nil {
				return nil, err
			}
			if failed, reason := failureOfCalls(rows); failed {
				sv.Failed, sv.Reason = true, reason
			}
		}
	}
	out.Steps = orderSteps(views)
	out.Cost = costOf(out.Steps)
	return out, nil
}

// costOf sums the estimated_cost_usd of the draft and approved variants of the content step per
// channel (D-071). Amounts are added as integers in micro-dollars (content.Money), so no rounding
// error builds up; a variant whose content has no estimate, or a malformed one, counts as zero.
func costOf(steps []StepView) CostSummary {
	var sms, email content.Money
	for _, sv := range steps {
		if sv.Kind != KindContent {
			continue
		}
		for _, v := range sv.Variants {
			if v.Status != VariantStatusDraft && v.Status != VariantStatusApproved {
				continue
			}
			var c struct {
				Cost string `json:"estimated_cost_usd"`
			}
			if json.Unmarshal(v.Content, &c) != nil {
				continue
			}
			m, err := content.ParseMoney(c.Cost)
			if err != nil {
				continue
			}
			switch v.Channel {
			case string(llm.ChannelSMS):
				sms += m
			case string(llm.ChannelEmail):
				email += m
			}
		}
	}
	return CostSummary{SMSUSD: sms.String(), EmailUSD: email.String(), TotalUSD: (sms + email).String()}
}

// orderSteps sorts steps by kind order; at most one live step per kind exists (D-068).
func orderSteps(steps []StepView) []StepView {
	out := make([]StepView, 0, len(steps))
	for _, kind := range kindOrder {
		for _, sv := range steps {
			if sv.Kind == kind {
				out = append(out, sv)
			}
		}
	}
	return out
}
