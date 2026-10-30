// Package flow is Proposal's agent flow (D-009, D-010, D-022, D-033, D-070): a campaign moves
// through the steps audience, channel plan, content and schedule; the model proposes, the
// marketer approves, rejects, edits or adds context, and code validates everything the model says.
//
// Every action returns quickly. Generation runs in a goroutine per campaign, one job at a time,
// ordered by a Postgres session advisory lock that is also held across replicas. State lives in
// the database and changes in transactions that first lock the campaign row; progress goes to an
// in-memory Hub as typed events. Only Cancel writes to the outbox (the cancelled event of a
// scheduled campaign, D-074); approving the schedule step calls a Confirmer, which writes the rest.
package flow

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// Clock tells the time; tests replace it.
type Clock interface{ Now() time.Time }

// ClockFunc adapts a function to Clock.
type ClockFunc func() time.Time

// Now calls f.
func (f ClockFunc) Now() time.Time { return f() }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Deps are what a Service needs.
type Deps struct {
	// Pool is the proposal_db pool. Transactions and the advisory lock use it.
	Pool *pgxpool.Pool
	// LLM is the model port.
	LLM llm.LLM
	// Tools validates and counts the filters the segment loop writes, and the audience edits. When
	// nil, Customer must be set and the default tools are built from it.
	Tools llm.SegmentTools
	// Customer is the Customer client for the default tools (*customer.Client).
	Customer llm.Previewer
	// Content writes and checks the content step. It may be nil until T-024; the content step then
	// fails with ErrNoContentGenerator.
	Content ContentGenerator
	// Confirmer is called when the schedule step is approved and by Confirm; nil means NopConfirmer.
	Confirmer Confirmer
	// Clock defaults to the system clock.
	Clock Clock
	// Hub receives the events; nil means a new hub (see Service.Hub).
	Hub *Hub
	// Logger defaults to slog.Default.
	Logger *slog.Logger
	// CTAURL is the link of the email button in previews (content.Config.CTAURL, D-090);
	// content.DefaultCTAURL when empty.
	CTAURL string
}

// Service is the proposal flow. It is safe for concurrent use. Close it when done.
type Service struct {
	pool    *pgxpool.Pool
	q       *store.Queries
	llm     llm.LLM
	tools   llm.SegmentTools
	content ContentGenerator
	confirm Confirmer
	clock   Clock
	hub     *Hub
	log     *slog.Logger
	cta     string

	baseCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	// slots bounds the campaign workers. A worker holds one pool connection for its advisory lock
	// and needs a second one for every transaction, so with more workers than half of the pool
	// every worker could hold a lock connection and wait for a transaction connection forever.
	slots chan struct{}

	mu       sync.Mutex
	closed   bool
	runners  map[uuid.UUID]*runner
	pending  int                           // jobs queued or running
	starting map[uuid.UUID]bool            // campaigns with a start job queued or running
	stepJobs map[uuid.UUID]int             // jobs queued or running per step
	failures map[uuid.UUID]map[Slot]string // in-memory reasons of failed generations, per step
}

// New returns a Service. It starts no goroutine until an action needs one.
func New(d Deps) (*Service, error) {
	switch {
	case d.Pool == nil:
		return nil, errors.New("flow: Deps.Pool is required")
	case d.LLM == nil:
		return nil, errors.New("flow: Deps.LLM is required")
	case d.Tools == nil && d.Customer == nil:
		return nil, errors.New("flow: Deps.Tools or Deps.Customer is required")
	}
	maxConns := int(d.Pool.Config().MaxConns)
	if maxConns < 2 {
		return nil, errors.New("flow: the pool needs at least 2 connections: a worker holds one for its advisory lock and needs another for its transactions")
	}
	s := &Service{
		pool: d.Pool, slots: make(chan struct{}, maxConns/2), q: store.New(d.Pool), llm: d.LLM, tools: d.Tools, content: d.Content,
		confirm: d.Confirmer, clock: d.Clock, hub: d.Hub, log: d.Logger, cta: d.CTAURL,
		runners: map[uuid.UUID]*runner{}, starting: map[uuid.UUID]bool{}, stepJobs: map[uuid.UUID]int{}, failures: map[uuid.UUID]map[Slot]string{},
	}
	if s.tools == nil {
		s.tools = llm.NewCustomerTools(d.Customer)
	}
	if s.confirm == nil {
		s.confirm = NopConfirmer{}
	}
	if s.clock == nil {
		s.clock = systemClock{}
	}
	if s.hub == nil {
		s.hub = NewHub(0)
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.cta == "" {
		s.cta = content.DefaultCTAURL
	}
	s.baseCtx, s.cancel = context.WithCancel(context.Background())
	return s, nil
}

// Hub returns the event hub; T-025 subscribes to it.
func (s *Service) Hub() *Hub { return s.hub }

// Close stops accepting generation jobs, cancels the running ones and waits for them (until ctx
// ends). Jobs that were queued are dropped; the steps they belonged to can be regenerated.
func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitIdle blocks until no generation job is queued or running, or ctx ends. It exists for tests
// and for graceful shutdown.
func (s *Service) WaitIdle(ctx context.Context) error {
	for {
		s.mu.Lock()
		n := s.pending
		s.mu.Unlock()
		if n == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// ---- jobs ----

type jobKind int

const (
	// jobStart screens the brief and opens the audience step.
	jobStart jobKind = iota
	// jobFill generates the slots of a step that have no draft or approved variant.
	jobFill
)

type job struct {
	kind     jobKind
	campaign uuid.UUID
	step     uuid.UUID // jobFill only
}

type runner struct{ queue []job }

// enqueue queues j for its campaign's worker, starting the worker if it is not running. A fill
// job for a step that already has one waiting is dropped: the waiting one will see the newest state.
func (s *Service) enqueue(j job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	r, running := s.runners[j.campaign]
	if !running {
		r = &runner{}
		s.runners[j.campaign] = r
	}
	switch j.kind {
	case jobFill:
		for _, w := range r.queue {
			if w == j {
				return
			}
		}
		s.stepJobs[j.step]++
	case jobStart:
		if s.starting[j.campaign] {
			return // screening or generating already
		}
		s.starting[j.campaign] = true
	}
	r.queue = append(r.queue, j)
	s.pending++
	if !running {
		s.wg.Add(1)
		go s.drain(j.campaign)
	}
}

// pop takes the next job of a campaign; it removes the runner when the queue is empty, under the
// same lock that enqueue uses, so no job is stranded.
func (s *Service) pop(campaign uuid.UUID) (job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runners[campaign]
	if r == nil || len(r.queue) == 0 {
		delete(s.runners, campaign)
		return job{}, false
	}
	j := r.queue[0]
	r.queue = r.queue[1:]
	return j, true
}

func (s *Service) jobDone(j job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending--
	if j.kind == jobStart {
		delete(s.starting, j.campaign)
	}
	if j.kind == jobFill {
		if s.stepJobs[j.step]--; s.stepJobs[j.step] <= 0 {
			delete(s.stepJobs, j.step)
		}
	}
}

// drain is the worker of one campaign: it takes the advisory lock, runs the queued jobs one by
// one, and releases the lock when the queue is empty.
func (s *Service) drain(campaign uuid.UUID) {
	defer s.wg.Done()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-s.baseCtx.Done():
		s.failQueued(campaign, s.baseCtx.Err())
		return
	}
	release, err := s.lockCampaign(s.baseCtx, campaign)
	if err != nil {
		s.log.Error("flow: cannot lock the campaign for generation", "campaign", campaign, "error", err)
		s.failQueued(campaign, err)
		return
	}
	defer release()
	for {
		j, ok := s.pop(campaign)
		if !ok {
			return
		}
		s.run(j)
		s.jobDone(j)
	}
}

// failQueued drops the queued jobs of a campaign because its worker cannot run, and tells the
// steps why (nothing is said during a shutdown).
func (s *Service) failQueued(campaign uuid.UUID, cause error) {
	for {
		j, ok := s.pop(campaign)
		if !ok {
			return
		}
		if j.kind == jobFill && s.baseCtx.Err() == nil {
			s.recordFailure(campaign, j.step, "", Slot{}, fmt.Errorf("could not lock the campaign: %w", cause))
		}
		s.jobDone(j)
	}
}

func (s *Service) run(j job) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("flow: generation job panicked", "campaign", j.campaign, "step", j.step, "panic", r)
			if j.kind == jobFill {
				s.recordFailure(j.campaign, j.step, "", Slot{}, fmt.Errorf("internal error: %v", r))
			}
		}
	}()
	switch j.kind {
	case jobStart:
		s.runStart(s.baseCtx, j.campaign)
	case jobFill:
		s.runFill(s.baseCtx, j.campaign, j.step)
	}
}

// LockKey is the 64-bit key of the Postgres advisory lock that orders the generation of one
// campaign across processes (D-070). It is exported so that operators and tests can see the lock.
func LockKey(campaign uuid.UUID) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("proposal.campaign.generation."))
	_, _ = h.Write(campaign[:])
	return int64(h.Sum64())
}

// lockCampaign takes the session advisory lock of the campaign on a dedicated connection, blocking
// until it is free, and returns the function that releases it. A connection that cannot prove the
// unlock is closed instead of returned to the pool, which ends the session and so the lock.
func (s *Service) lockCampaign(ctx context.Context, campaign uuid.UUID) (func(), error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key := LockKey(campaign)
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		// An interrupted lock request may still be granted later: do not reuse the connection.
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = conn.Hijack().Close(closeCtx)
		return nil, err
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var ok bool
		if err := conn.QueryRow(ctx, `SELECT pg_advisory_unlock($1)`, key).Scan(&ok); err != nil || !ok {
			s.log.Warn("flow: advisory unlock failed; closing the connection", "campaign", campaign, "error", err, "unlocked", ok)
			_ = conn.Hijack().Close(ctx)
			return
		}
		conn.Release()
	}, nil
}

// ---- failures (in memory) ----

func (s *Service) isGenerating(step uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stepJobs[step] > 0
}

func (s *Service) setFailure(step uuid.UUID, slot Slot, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.failures[step]
	if m == nil {
		m = map[Slot]string{}
		s.failures[step] = m
	}
	m[slot] = reason
}

func (s *Service) clearFailures(step uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, step)
}

// failureOf returns the reason of a failed generation of the step in this process, "" when none.
// Several failed slots are joined.
func (s *Service) failureOf(step uuid.UUID) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.failures[step]
	if len(m) == 0 {
		return ""
	}
	slots := make([]Slot, 0, len(m))
	for k := range m {
		slots = append(slots, k)
	}
	sortSlots(slots)
	out := ""
	for i, k := range slots {
		if i > 0 {
			out += "; "
		}
		if len(slots) > 1 || k != (Slot{}) {
			out += k.String() + ": "
		}
		out += m[k]
	}
	return out
}

// ---- transactions ----

// txn is the scope of one transaction: its queries and the events to publish after commit.
type txn struct {
	tx     pgx.Tx
	q      *store.Queries
	events []Event
}

func (t *txn) emit(ev Event) { t.events = append(t.events, ev) }

func (s *Service) meta(campaign uuid.UUID) Meta {
	return Meta{CampaignID: campaign, At: s.clock.Now()}
}

// inTx runs fn in a transaction that starts by locking the campaign row, which serializes every
// state change of one campaign (D-070). Events emitted by fn are published after the commit.
func (s *Service) inTx(ctx context.Context, campaign uuid.UUID, fn func(t *txn, camp store.Campaign) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	t := &txn{tx: tx, q: s.q.WithTx(tx)}
	camp, err := t.q.GetCampaignForUpdate(ctx, campaign)
	if err != nil {
		return notFound(err)
	}
	if err := fn(t, camp); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, ev := range t.events {
		s.hub.Publish(ev)
	}
	return nil
}

// notFound maps "no rows" to ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
