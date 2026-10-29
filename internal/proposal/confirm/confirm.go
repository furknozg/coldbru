package confirm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// Recipients is the part of the Customer client the Confirmer uses; *customer.Client implements it.
type Recipients interface {
	// EachRecipientPage calls fn with every page of the audience of f (customer.MaxPageSize at a time).
	EachRecipientPage(ctx context.Context, f *compile.Filter, fn func(page []customer.Recipient) error) error
}

var _ Recipients = (*customer.Client)(nil)

// Deps are what a Confirmer needs.
type Deps struct {
	// Pool is the proposal_db pool, with the outbox table in the search path.
	Pool *pgxpool.Pool
	// Customer supplies the recipients.
	Customer Recipients
	// CTAURL is the base link of the email button (content.Config.CTAURL); the event carries it with
	// utm_campaign merged in.
	CTAURL string
	// Hub receives the campaign_confirmed event after the commit; nil means none is published. Use
	// the hub of the flow.Service so that the event reaches the event streams.
	Hub *flow.Hub
	// Clock defaults to the system clock.
	Clock flow.Clock
	// Logger defaults to slog.Default.
	Logger *slog.Logger
}

// Confirmer implements flow.Confirmer. It is safe for concurrent use: the campaign row lock and the
// status check inside the transaction make a second concurrent or repeated Confirm a no-op.
type Confirmer struct {
	pool  *pgxpool.Pool
	q     *store.Queries
	cust  Recipients
	cta   string
	hub   *flow.Hub
	clock flow.Clock
	log   *slog.Logger
}

var _ flow.Confirmer = (*Confirmer)(nil)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// New returns a Confirmer. The CTA link is validated here so that a bad one fails at startup.
func New(d Deps) (*Confirmer, error) {
	switch {
	case d.Pool == nil:
		return nil, errors.New("confirm: Deps.Pool is required")
	case d.Customer == nil:
		return nil, errors.New("confirm: Deps.Customer is required")
	}
	if _, err := content.CTAURL(d.CTAURL, ""); err != nil {
		return nil, fmt.Errorf("confirm: CTA URL: %w", err)
	}
	c := &Confirmer{pool: d.Pool, q: store.New(d.Pool), cust: d.Customer, cta: d.CTAURL, hub: d.Hub, clock: d.Clock, log: d.Logger}
	if c.clock == nil {
		c.clock = systemClock{}
	}
	if c.log == nil {
		c.log = slog.Default()
	}
	return c, nil
}

// approved is what the approved steps of a campaign say, read in one consistent pass.
type approved struct {
	campaignID   uuid.UUID
	scheduleStep uuid.UUID
	audience     flow.AudienceContent
	plan         flow.PlanContent
	messages     []flow.ConfirmedMessage
	schedules    []flow.ConfirmedSchedule
}

// load reads the approved audience, plan, texts and schedules of a campaign. It returns
// flow.ErrScheduleNotApproved when the campaign has no approved schedule step.
func load(ctx context.Context, q *store.Queries, campaignID uuid.UUID) (*approved, error) {
	steps, err := q.ListLiveSteps(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	byKind := map[string]store.ProposalStep{}
	for _, st := range steps {
		byKind[st.Kind] = st
	}
	sched, ok := byKind[flow.KindSchedule]
	if !ok || sched.Status != flow.StepStatusApproved {
		return nil, flow.ErrScheduleNotApproved
	}
	out := &approved{campaignID: campaignID, scheduleStep: sched.ID}
	for _, kind := range []string{flow.KindAudience, flow.KindChannelPlan, flow.KindContent, flow.KindSchedule} {
		st, ok := byKind[kind]
		if !ok || st.Status != flow.StepStatusApproved {
			return nil, fmt.Errorf("confirm: the schedule step is approved but the %s step is not", kind)
		}
		vs, err := q.ListLiveVariants(ctx, st.ID)
		if err != nil {
			return nil, err
		}
		for _, v := range vs {
			if v.Status != flow.VariantStatusApproved {
				continue
			}
			switch kind {
			case flow.KindAudience:
				err = json.Unmarshal(v.Content, &out.audience)
			case flow.KindChannelPlan:
				err = json.Unmarshal(v.Content, &out.plan)
			case flow.KindContent:
				out.messages = append(out.messages, flow.ConfirmedMessage{
					Language: v.Language.String, Channel: v.Channel.String, Content: json.RawMessage(v.Content),
				})
			case flow.KindSchedule:
				var sc flow.ScheduleContent
				err = json.Unmarshal(v.Content, &sc)
				out.schedules = append(out.schedules, flow.ConfirmedSchedule{Language: v.Language.String, Timezone: v.Timezone.String, SendAt: sc.SendAt})
			}
			if err != nil {
				return nil, fmt.Errorf("confirm: decode the approved %s variant: %w", kind, err)
			}
		}
	}
	sort.Slice(out.messages, func(i, j int) bool {
		a, b := out.messages[i], out.messages[j]
		return a.Language < b.Language || a.Language == b.Language && a.Channel < b.Channel
	})
	sort.Slice(out.schedules, func(i, j int) bool {
		a, b := out.schedules[i], out.schedules[j]
		return a.Language < b.Language || a.Language == b.Language && a.Timezone < b.Timezone
	})
	if len(out.messages) == 0 || len(out.schedules) == 0 {
		return nil, errors.New("confirm: the approved steps hold no texts or no schedules")
	}
	return out, nil
}

// prepared is everything the transaction writes.
type prepared struct {
	recipients []flow.BatchRecipient
	counts     flow.Recipients
	dropped    int
}

// prepare fetches the audience from Customer and assigns channels under the approved plan. A
// person is dropped when the approved texts do not cover their language and assigned channel, or the
// approved schedules their (language, timezone) pair (D-076), which happens when the audience
// changed since the preview (D-039): the receiver could not send them anything. Every dropped
// pair is logged once with the number of people. No recipient left is flow.ErrNoRecipients.
func (c *Confirmer) prepare(ctx context.Context, a *approved) (*prepared, error) {
	f, errs := compile.Compile(a.audience.DSL)
	if len(errs) > 0 || f == nil {
		return nil, fmt.Errorf("confirm: the approved audience filter no longer compiles: %v", errs)
	}
	haveMessage := map[[2]string]bool{}
	for _, m := range a.messages {
		haveMessage[[2]string{m.Language, m.Channel}] = true
	}
	haveSchedule := map[[2]string]bool{}
	for _, s := range a.schedules {
		haveSchedule[[2]string{s.Language, s.Timezone}] = true
	}
	out := &prepared{}
	droppedBy := map[string]int{}
	err := c.cust.EachRecipientPage(ctx, f, func(page []customer.Recipient) error {
		for _, r := range page {
			for _, e := range Assign(a.plan.Mode, a.plan.Primary, r) {
				if !haveMessage[[2]string{e.Language, e.Channel}] || !haveSchedule[[2]string{e.Language, e.Timezone}] {
					out.dropped++
					droppedBy[e.Language+"/"+e.Timezone+"/"+e.Channel]++
					continue
				}
				out.recipients = append(out.recipients, e)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("confirm: fetch the recipients: %w", err)
	}
	for _, k := range slices.Sorted(maps.Keys(droppedBy)) {
		if c.log == nil {
			break
		}
		c.log.WarnContext(ctx, "recipients dropped: no approved text or schedule covers them", "campaign", a.campaignID, "locale_channel", k, "recipients", droppedBy[k])
	}
	out.counts = Count(out.recipients)
	if out.counts.Total == 0 {
		return nil, flow.ErrNoRecipients
	}
	return out, nil
}

// errChanged: the approved steps changed between reading them and the transaction, which only a
// reopen followed by new approvals does. Nothing was written; the caller may confirm again.
var errChanged = errors.New("confirm: the campaign changed while it was being confirmed; try again")

// Confirm implements flow.Confirmer (D-074). It returns nil, writing nothing, when the campaign is
// already `scheduled`. Errors: flow.ErrNotFound, flow.ErrNotDrafting (cancelled, sending, completed),
// flow.ErrScheduleNotApproved, flow.ErrNoRecipients, and any other error from Customer or the
// database, none of which changed anything.
func (c *Confirmer) Confirm(ctx context.Context, campaignID uuid.UUID) error {
	camp, err := c.q.GetCampaign(ctx, campaignID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return flow.ErrNotFound
		}
		return err
	}
	switch camp.Status {
	case flow.CampaignStatusScheduled:
		return nil
	case flow.CampaignStatusDrafting:
	default:
		return flow.ErrNotDrafting
	}
	a, err := load(ctx, c.q, campaignID)
	if err != nil {
		return err
	}
	// The recipient fetch is slow and is done without a lock; the transaction below checks that
	// nothing changed.
	p, err := c.prepare(ctx, a)
	if err != nil {
		return err
	}
	cta, err := content.CTAURL(c.cta, campaignID.String())
	if err != nil {
		return fmt.Errorf("confirm: %w", err)
	}
	batches := Batches(campaignID.String(), p.recipients)

	err = c.write(ctx, campaignID, a, p, cta, batches)
	switch {
	case err == nil:
	case errors.Is(err, errAlreadyScheduled):
		return nil
	default:
		return err
	}
	c.log.InfoContext(ctx, "campaign confirmed", "campaign", campaignID,
		"email", p.counts.Email, "sms", p.counts.SMS, "batches", len(batches), "dropped", p.dropped)
	if c.hub != nil {
		c.hub.Publish(flow.CampaignConfirmed{
			Meta:            flow.Meta{CampaignID: campaignID, At: c.clock.Now()},
			RecipientCounts: p.counts, RecipientBatches: len(batches),
		})
	}
	return nil
}

var errAlreadyScheduled = errors.New("confirm: already scheduled")

// write is the transaction: lock the campaign, re-check, set the status and write the event and the
// batches to the outbox.
func (c *Confirmer) write(ctx context.Context, campaignID uuid.UUID, a *approved, p *prepared, cta string, batches []flow.RecipientBatch) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := c.q.WithTx(tx)

	camp, err := q.GetCampaignForUpdate(ctx, campaignID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return flow.ErrNotFound
		}
		return err
	}
	switch camp.Status {
	case flow.CampaignStatusScheduled:
		return errAlreadyScheduled
	case flow.CampaignStatusDrafting:
	default:
		return flow.ErrNotDrafting
	}
	// Any change to the audience, plan or texts supersedes the schedule step (a reopen cascades), so
	// the same approved schedule step proves that what was read before is still what is approved.
	now, err := load(ctx, q, campaignID)
	if err != nil {
		return err
	}
	if now.scheduleStep != a.scheduleStep {
		return errChanged
	}

	if _, err := q.SetCampaignStatus(ctx, store.SetCampaignStatusParams{ID: campaignID, Status: flow.CampaignStatusScheduled}); err != nil {
		return err
	}
	env, err := events.New(flow.EventTypeConfirmed, flow.EventVersion, flow.EventProducer, campaignID.String(), flow.ConfirmedData{
		CampaignID: campaignID.String(), Schedules: a.schedules, Messages: a.messages, CTAURL: cta,
		RecipientBatches: len(batches), RecipientCounts: p.counts,
	})
	if err != nil {
		return err
	}
	if _, err := outbox.Write(ctx, tx, env); err != nil {
		return err
	}
	subject := flow.RecipientsSubject(campaignID.String())
	for _, b := range batches {
		payload, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("confirm: encode batch %d: %w", b.BatchNo, err)
		}
		if _, err := outbox.WriteMessage(ctx, tx, subject, payload); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
