// Package sender is the SEND worker of Notification (D-024, D-079): it takes the delivery ids that
// the schedule firing queued on send.sms and send.email (D-078), checks consent, renders the
// message for the recipient, calls the provider and settles the delivery.
//
// Per channel there is one durable pull consumer, one fetching goroutine and a fixed pool of sender
// goroutines:
//
//	SEND ──fetch ≤100──▶ batch: load rows, drop final and cancelled ones,
//	                     one consentStatus call for the batch (fail-closed, behind a breaker)
//	     ──jobs────────▶ sender pool: rate limit (100/s per channel, shared), claim with a lease,
//	                     render, provider call, settle: sent | pending again | failed
//
// Guarantees:
//   - A delivery goes to the provider only after it was claimed in the database (pending to sending
//     with a lease). A duplicate SEND message, a worker that crashed (its lease runs out and the
//     next delivery of the message takes the row over) and a cancellation are all decided by that
//     one conditional UPDATE. The provider also gets the delivery id as Idempotency-Key.
//   - A message is acked only after the database says what became of the delivery. When the
//     database cannot be reached after the provider answered, the message is left unsettled; it is
//     redelivered after AckWait, which is longer than the lease.
//   - A delivery that is not eligible on the channel is never sent (D-011): it becomes
//     skipped_no_consent. If consent cannot be checked, nothing is sent.
//   - At most 5 deliveries per message (MaxDeliver). On the last one a provider failure that would
//     be retried ends the delivery as failed. If consent could not be checked, the delivery is not
//     failed: a new message is written to the outbox and the old one acked, so the delivery waits
//     for Customer however long it takes.
//
// The campaign is not completed here, nor are its personal data cleared: that is T-030. The one
// exception is a campaign cancelled while a delivery is being sent; the delivery then loses its
// personal data when it is settled (D-078).
package sender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/notification/provider"
	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

// Stream and consumer settings (D-024, D-079).
const (
	// Stream is the work-queue stream of deliveries to send.
	Stream = "SEND"
	// ConsumerPrefix plus the channel is the durable consumer name: notification-send-sms.
	ConsumerPrefix = "notification-send-"

	// MaxDeliver is how many times JetStream delivers a message (D-024).
	MaxDeliver = 5
	// AckWait is how long JetStream waits for an ack before it redelivers. It is longer than the
	// lease, so a message that comes back after a crash finds the lease run out.
	AckWait = 45 * time.Second
	// LeaseDuration is how long a claimed delivery belongs to its worker (D-079). A claim covers
	// one provider call (10 s limit), not the wait for a rate-limit token.
	LeaseDuration = 30 * time.Second
	// BatchSize is the most messages one fetch asks for, and the most customers one consent query covers.
	BatchSize = 100
	// WorkersPerChannel is the size of the sender pool of a channel. Enough to keep 100 requests a
	// second going at a few hundred milliseconds each; the rate limiter, not this, sets the speed.
	WorkersPerChannel = 16
	// RatePerSecond and Burst are the token bucket of a channel (D-024, D-079).
	RatePerSecond = 100
	Burst         = 10
	// DefaultPause is how long a channel stops for a 429 that has no Retry-After.
	DefaultPause = time.Second

	// fetchWait is how long a fetch waits for a full batch before it returns what it has.
	fetchWait = 500 * time.Millisecond
	// maxAckPending leaves room for a full batch in the sender queue, one in the senders' hands and one being fetched.
	maxAckPending = 3 * BatchSize
	// settleAttempts is how often a database write after a provider call is tried before the
	// message is left to expire.
	settleAttempts = 3
	// maxErrorLen bounds last_error.
	maxErrorLen = 500
)

// DefaultBackoff is the delay before a retry: 2 s, doubling to 1 minute, +-20% (D-079).
var DefaultBackoff = consumer.Backoff{Base: 2 * time.Second, Factor: 2, Max: time.Minute, Jitter: 0.2}

// Provider sends messages. *provider.Client implements it.
type Provider interface {
	SendSMS(ctx context.Context, idempotencyKey string, m provider.SMS) (provider.Result, error)
	SendEmail(ctx context.Context, idempotencyKey string, m provider.Email) (provider.Result, error)
}

// Consent says which customers may be written to. *consent.Client implements it. An error means
// that nobody may be written to.
type Consent interface {
	Eligible(ctx context.Context, channel string, customerIDs []string) (map[string]bool, error)
}

// Deps are what the worker needs.
type Deps struct {
	JS       jetstream.JetStream
	Pool     *pgxpool.Pool
	Logger   *slog.Logger
	Provider Provider
	Consent  Consent
	// Registerer receives the metrics; nil registers them nowhere.
	Registerer prometheus.Registerer

	// The fields below default to the production values; tests shorten them.

	// Backoff is the delay before a retry; the zero value means DefaultBackoff.
	Backoff consumer.Backoff
	// Workers is the size of the sender pool per channel; zero means WorkersPerChannel.
	Workers int
	// Limit and Burst are the token bucket per channel; zero means RatePerSecond and Burst.
	Limit rate.Limit
	Burst int
	// Lease is the lease of a claim; zero means LeaseDuration.
	Lease time.Duration
}

// Channels the worker serves.
var workerChannels = []string{"sms", "email"}

// ConsumerConfig returns the durable pull consumer of a channel: filter send.<channel>, explicit
// acks, MaxDeliver 5, AckWait 45 s.
func ConsumerConfig(channel string) jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{
		Durable:       ConsumerPrefix + channel,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    MaxDeliver,
		AckWait:       AckWait,
		FilterSubject: intake.SendSubject(channel),
		MaxAckPending: maxAckPending,
	}
}

// worker is one channel's fetcher and sender pool.
type worker struct {
	channel  string
	cons     jetstream.Consumer
	pool     *pgxpool.Pool
	q        *store.Queries
	log      *slog.Logger
	provider Provider
	consent  Consent
	gate     *gate
	m        *metrics
	backoff  consumer.Backoff
	lease    time.Duration
	workers  int
	jobs     chan job
}

// job is a delivery cleared for sending and the message that carries it.
type job struct {
	msg jetstream.Msg
	row store.ListSendDeliveriesRow
	// n is how many times JetStream has delivered the message, this time included.
	n int
}

// Start creates the durable consumers on the SEND stream (which must exist) and starts the workers.
// It returns the function that stops them: no new messages are fetched, the batch in hand is
// finished, and the function waits for the senders until ctx ends; after that the senders are
// aborted and their messages are nak'd. Database writes are never cut off by ctx's cancellation
// before Start's, so a delivery claimed is settled.
func Start(ctx context.Context, d Deps) (stop func(context.Context) error, err error) {
	if d.JS == nil || d.Pool == nil || d.Provider == nil || d.Consent == nil {
		return nil, errors.New("sender: JetStream, a database pool, a provider and a consent client are required")
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	reg := d.Registerer
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	m := newMetrics(reg)
	backoff := d.Backoff
	if backoff == (consumer.Backoff{}) {
		backoff = DefaultBackoff
	}
	limit, burst := d.Limit, d.Burst
	if limit == 0 {
		limit = RatePerSecond
	}
	if burst == 0 {
		burst = Burst
	}
	lease := d.Lease
	if lease <= 0 {
		lease = LeaseDuration
	}
	senders := d.Workers
	if senders <= 0 {
		senders = WorkersPerChannel
	}

	var workers []*worker
	for _, ch := range workerChannels {
		cfg := ConsumerConfig(ch)
		cons, err := d.JS.CreateOrUpdateConsumer(ctx, Stream, cfg)
		if err != nil {
			return nil, fmt.Errorf("create consumer %s on %s: %w", cfg.Durable, Stream, err)
		}
		workers = append(workers, &worker{
			channel: ch, cons: cons, pool: d.Pool, q: store.New(d.Pool), log: logger.With("channel", ch),
			provider: d.Provider, consent: d.Consent, gate: newGate(limit, burst), m: m,
			backoff: backoff, lease: lease, workers: senders, jobs: make(chan job, BatchSize),
		})
		logger.Info("consumer started", "consumer", cfg.Durable, "stream", Stream, "filter", cfg.FilterSubject,
			"max_ack_pending", cfg.MaxAckPending, "ack_wait", cfg.AckWait.String(), "max_deliver", cfg.MaxDeliver,
			"senders", senders, "rate_per_second", float64(limit), "burst", burst)
	}

	// procCtx is what database writes and provider calls run under: ctx's cancellation does not
	// reach it, so work in flight finishes when the service is told to stop. It is cancelled only
	// when the stop function runs out of time.
	procCtx, cancelProc := context.WithCancel(context.WithoutCancel(ctx))
	fetchCtx, cancelFetch := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, w := range workers {
		var senderWG sync.WaitGroup
		for range w.workers {
			senderWG.Add(1)
			go func() {
				defer senderWG.Done()
				for j := range w.jobs {
					w.send(procCtx, j)
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.fetchLoop(fetchCtx, procCtx)
			close(w.jobs) // the senders finish what is queued and exit
			senderWG.Wait()
		}()
	}

	return func(stopCtx context.Context) error {
		defer cancelProc()
		cancelFetch()
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			cancelProc() // aborts rate-limit waits and provider calls; queued messages are nak'd
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
			return fmt.Errorf("wait for send workers: %w", stopCtx.Err())
		}
	}, nil
}

// fetchLoop fetches batches until ctx ends. fetchCtx only decides when to stop fetching; the batch
// in hand is handled under procCtx.
func (w *worker) fetchLoop(fetchCtx, procCtx context.Context) {
	for fetchCtx.Err() == nil {
		batch, err := w.cons.Fetch(BatchSize, jetstream.FetchMaxWait(fetchWait))
		if err != nil {
			if fetchCtx.Err() == nil {
				w.log.Warn("fetch failed", "error", err)
				_ = sleep(fetchCtx, time.Second)
			}
			continue
		}
		var msgs []jetstream.Msg
		for msg := range batch.Messages() {
			msgs = append(msgs, msg)
		}
		if err := batch.Error(); err != nil && fetchCtx.Err() == nil {
			w.log.Warn("fetch ended with an error", "error", err, "messages", len(msgs))
		}
		if len(msgs) > 0 {
			w.handleBatch(procCtx, msgs)
		}
	}
}

// entry is one decoded message of a batch.
type entry struct {
	msg jetstream.Msg
	id  uuid.UUID
	n   int
}

// handleBatch settles what can be settled without the provider (malformed, final, cancelled, no
// consent) and queues the rest for the senders.
func (w *worker) handleBatch(ctx context.Context, msgs []jetstream.Msg) {
	entries := make([]entry, 0, len(msgs))
	ids := make([]uuid.UUID, 0, len(msgs))
	for _, msg := range msgs {
		var p intake.SendPayload
		if err := json.Unmarshal(msg.Data(), &p); err != nil {
			w.terminate(msg, "invalid json", err)
			continue
		}
		id, err := uuid.Parse(p.DeliveryID)
		if err != nil {
			w.terminate(msg, "delivery_id is not a uuid", err)
			continue
		}
		entries = append(entries, entry{msg: msg, id: id, n: deliveries(msg)})
		ids = append(ids, id)
	}
	if len(entries) == 0 {
		return
	}

	rows, err := w.q.ListSendDeliveries(ctx, ids)
	if err != nil {
		w.log.Warn("load deliveries failed", "messages", len(entries), "error", err)
		for _, e := range entries {
			w.nakOrTerm(e.msg, e.n, e.id, "load deliveries", err)
		}
		return
	}
	byID := make(map[uuid.UUID]store.ListSendDeliveriesRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	var todo []job
	for _, e := range entries {
		row, ok := byID[e.id]
		switch {
		case !ok:
			w.terminate(e.msg, "delivery not found", errors.New(e.id.String()))
		case row.Channel != w.channel:
			w.terminate(e.msg, "delivery belongs to another channel", fmt.Errorf("%s is %s, queued on %s", e.id, row.Channel, w.channel))
		case row.Status != "pending" && row.Status != "sending":
			// Sent, failed, skipped or cancelled already: a duplicate message.
			w.log.Debug("delivery already final, message dropped", "delivery_id", e.id, "status", row.Status)
			w.ack(e.msg, e.id)
		case row.CampaignStatus == "cancelled":
			// Nothing is sent for a cancelled campaign. A pending row was cancelled by the cancellation
			// itself; one that is still pending here is cancelled now and loses its personal data.
			n, err := w.q.CancelPendingDelivery(ctx, e.id)
			if err != nil {
				w.nakOrTerm(e.msg, e.n, e.id, "cancel delivery", err)
				continue
			}
			if n == 1 {
				w.m.final(w.channel, "cancelled")
			}
			w.log.Info("delivery of a cancelled campaign dropped", "delivery_id", e.id, "campaign_id", row.CampaignID)
			w.ack(e.msg, e.id)
		default:
			todo = append(todo, job{msg: e.msg, row: row, n: e.n})
		}
	}
	if len(todo) == 0 {
		return
	}

	// One consent query for the whole batch (D-011, D-079).
	seen := make(map[string]bool, len(todo))
	customers := make([]string, 0, len(todo))
	for _, j := range todo {
		if !seen[j.row.CustomerID] {
			seen[j.row.CustomerID] = true
			customers = append(customers, j.row.CustomerID)
		}
	}
	eligible, err := w.consent.Eligible(ctx, w.channel, customers)
	if err != nil {
		// Fail-closed: nobody in the batch is sent anything, and nobody is blamed on the provider.
		w.log.Warn("consent check failed, batch deferred", "messages", len(todo), "error", err)
		for _, j := range todo {
			w.consentUnavailable(ctx, j)
		}
		return
	}
	for _, j := range todo {
		if !eligible[j.row.CustomerID] {
			n, err := w.q.SkipDeliveryNoConsent(ctx, j.row.ID)
			if err != nil {
				w.nakOrTerm(j.msg, j.n, j.row.ID, "skip delivery without consent", err)
				continue
			}
			if n == 1 {
				w.m.final(w.channel, "skipped_no_consent")
				w.log.Info("delivery skipped, no consent", "delivery_id", j.row.ID, "campaign_id", j.row.CampaignID)
			}
			w.ack(j.msg, j.row.ID)
			continue
		}
		select {
		case w.jobs <- j:
		case <-ctx.Done():
			_ = j.msg.Nak() // forced stop: back to the stream at once
		}
	}
}

// consentUnavailable defers a message because consent could not be checked: nothing is sent and
// the delivery stays pending (D-011). While deliveries remain it is nak'd with backoff. JetStream
// delivers a message at most MaxDeliver times, so on the last delivery a fresh message for the same
// delivery is written to the outbox and the old one is acked; the new message starts counting from
// one and the cycle repeats until Customer answers. The delivery is never failed for this.
func (w *worker) consentUnavailable(ctx context.Context, j job) {
	if j.n < MaxDeliver {
		w.nak(j.msg, j.row.ID, w.backoff.Delay(j.n))
		return
	}
	if err := w.requeue(ctx, j.row.ID); err != nil {
		w.terminate(j.msg, "delivery stuck pending: could not requeue after the last delivery", err)
		return
	}
	w.log.Warn("consent still unavailable on the last delivery, message requeued",
		"delivery_id", j.row.ID, "campaign_id", j.row.CampaignID)
	w.ack(j.msg, j.row.ID)
}

// requeue writes a new send.<channel> message for a delivery to the outbox, in its own transaction.
// The relay publishes it; until the old message is acked both exist, which a duplicate-safe claim
// tolerates.
func (w *worker) requeue(ctx context.Context, id uuid.UUID) error {
	payload, err := json.Marshal(intake.SendPayload{DeliveryID: id.String()})
	if err != nil {
		return err
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // no-op after Commit
	if _, err := outbox.WriteMessage(ctx, tx, intake.SendSubject(w.channel), payload); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// send takes one queued delivery through rate limit, claim, render, provider call and settlement.
func (w *worker) send(ctx context.Context, j job) {
	id := j.row.ID
	log := w.log.With("delivery_id", id, "campaign_id", j.row.CampaignID)

	// Wait for the channel before claiming, so that the lease covers the provider call only.
	if err := w.gate.Wait(ctx); err != nil {
		_ = j.msg.Nak()
		return
	}
	claim, err := w.q.ClaimDelivery(ctx, store.ClaimDeliveryParams{ID: id, LeaseSeconds: w.lease.Seconds()})
	if errors.Is(err, pgx.ErrNoRows) {
		log.Debug("delivery not claimed: final, leased by another worker or campaign cancelled")
		w.ack(j.msg, id)
		return
	}
	if err != nil {
		log.Warn("claim failed", "error", err)
		w.nakOrTerm(j.msg, j.n, id, "claim delivery", err)
		return
	}

	call, err := w.prepare(log, j.row, claim.FirstName.String, claim.Contact.String)
	if err != nil {
		// The stored content cannot be rendered; the same would happen on every attempt.
		w.fail(ctx, log, j, claim.Attempts, "cannot render message: "+err.Error())
		return
	}

	start := time.Now()
	res, err := call(ctx)
	status := res.Status
	if err != nil {
		status = provider.StatusOf(err)
	}
	w.m.providerCall(w.channel, status, time.Since(start).Seconds())

	switch provider.Classify(err) {
	case provider.ClassOK:
		w.sent(ctx, log, j, claim.Attempts, res.MessageID)
	case provider.ClassPermanent:
		w.fail(ctx, log, j, claim.Attempts, err.Error())
	case provider.ClassRateLimited:
		var rl *provider.RateLimitedError
		pause := DefaultPause
		if errors.As(err, &rl) && rl.RetryAfter > 0 {
			pause = rl.RetryAfter
		}
		w.gate.PauseFor(pause)
		log.Warn("provider rate limited the channel", "pause", pause.String())
		w.retry(ctx, log, j, claim.Attempts, err, pause)
	default:
		w.retry(ctx, log, j, claim.Attempts, err, w.backoff.Delay(j.n))
	}
}

// sent records the provider's acceptance. Personal data stay until the campaign ends (D-077).
func (w *worker) sent(ctx context.Context, log *slog.Logger, j job, attempts int32, messageID string) {
	n, err := settle(ctx, func() (int64, error) {
		return w.q.MarkDeliverySent(ctx, store.MarkDeliverySentParams{ID: j.row.ID, Attempts: attempts, ProviderMessageID: text(messageID)})
	})
	if err != nil {
		// The provider has the message. The row stays sending; once its lease has run out the
		// redelivery claims it again and the Idempotency-Key makes the provider answer the same.
		log.Error("message sent but the delivery could not be marked sent, left to be redelivered", "error", err)
		return
	}
	if n == 1 {
		w.m.final(w.channel, "sent")
	} else {
		log.Warn("delivery no longer ours when marking it sent", "attempts", attempts)
	}
	w.ack(j.msg, j.row.ID)
}

// fail ends a claimed delivery as failed and drops the message.
func (w *worker) fail(ctx context.Context, log *slog.Logger, j job, attempts int32, reason string) {
	reason = truncate(reason)
	n, err := settle(ctx, func() (int64, error) {
		return w.q.MarkDeliveryFailed(ctx, store.MarkDeliveryFailedParams{ID: j.row.ID, Attempts: attempts, LastError: text(reason)})
	})
	if err != nil {
		log.Error("delivery failed but could not be marked failed, left to be redelivered", "error", err)
		return
	}
	if n == 1 {
		w.m.final(w.channel, "failed")
		log.Warn("delivery failed", "attempts", attempts, "reason", reason)
	}
	w.ack(j.msg, j.row.ID)
}

// retry puts a claimed delivery back to pending and asks JetStream to redeliver the message after
// delay. On the last delivery the delivery fails instead.
func (w *worker) retry(ctx context.Context, log *slog.Logger, j job, attempts int32, cause error, delay time.Duration) {
	if j.n >= MaxDeliver {
		w.fail(ctx, log, j, attempts, fmt.Sprintf("gave up after %d deliveries: %v", MaxDeliver, cause))
		return
	}
	var status string
	_, err := settle(ctx, func() (int64, error) {
		var err error
		status, err = w.q.ReleaseDelivery(ctx, store.ReleaseDeliveryParams{ID: j.row.ID, Attempts: attempts, LastError: text(truncate(cause.Error()))})
		if errors.Is(err, pgx.ErrNoRows) {
			status = ""
			return 0, nil
		}
		return 1, err
	})
	switch {
	case err != nil:
		log.Error("delivery could not be put back to pending, left to be redelivered", "error", err)
	case status == "":
		// Taken over by another worker, or released by an earlier try of this write whose answer was
		// lost. A redelivery is harmless in both cases: it finds the delivery claimed or pending.
		log.Warn("delivery no longer ours when releasing it", "attempts", attempts)
		w.nak(j.msg, j.row.ID, delay)
	case status == "cancelled":
		// The campaign was cancelled while the message was out: the delivery ended, with its personal data cleared.
		w.m.final(w.channel, "cancelled")
		w.ack(j.msg, j.row.ID)
	default:
		log.Info("delivery will be retried", "delivery", j.n, "delay", delay.String(), "reason", cause.Error())
		w.nak(j.msg, j.row.ID, delay)
	}
}

// prepare renders the message of a claimed delivery and returns the provider call that sends it.
// An error means the stored content cannot be turned into a message.
func (w *worker) prepare(log *slog.Logger, row store.ListSendDeliveriesRow, firstName, contact string) (func(context.Context) (provider.Result, error), error) {
	if strings.TrimSpace(contact) == "" {
		return nil, errors.New("delivery has no contact")
	}
	key := row.ID.String()
	switch row.Channel {
	case "sms":
		var c content.SMSContent
		if err := json.Unmarshal(row.Content, &c); err != nil || strings.TrimSpace(c.Text) == "" {
			return nil, fmt.Errorf("sms content: %v", err)
		}
		body, fit := fitSMS(c.Text, firstName)
		if n := utf8.RuneCountInString(body); n > content.SMSMaxChars {
			return nil, fmt.Errorf("sms text has %d characters without the first name, the limit is %d", n, content.SMSMaxChars)
		}
		if fit != nameFull {
			log.Info("first name did not fit the sms", "fit", fit.String())
		}
		info := content.MeasureSMS(body)
		log.Debug("sms rendered", "encoding", string(info.Encoding), "segments", info.Segments, "length", info.Length)
		return func(ctx context.Context) (provider.Result, error) {
			return w.provider.SendSMS(ctx, key, provider.SMS{To: contact, Text: body})
		}, nil
	case "email":
		var c content.EmailContent
		if err := json.Unmarshal(row.Content, &c); err != nil {
			return nil, fmt.Errorf("email content: %w", err)
		}
		r, err := content.RenderEmail(c.Email(), content.RenderOptions{
			CTAURL: row.CtaUrl, CampaignID: row.CampaignID.String(), Language: row.Language,
		}, firstName)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context) (provider.Result, error) {
			return w.provider.SendEmail(ctx, key, provider.Email{To: contact, Subject: r.Subject, HTML: r.HTML, Text: r.Text})
		}, nil
	}
	return nil, fmt.Errorf("unknown channel %q", row.Channel)
}

// settle runs a database write that follows a provider call, a few times if it fails: the message
// has been sent, so losing the record of it is the worst outcome.
func settle(ctx context.Context, fn func() (int64, error)) (int64, error) {
	var err error
	for i := range settleAttempts {
		var n int64
		if n, err = fn(); err == nil {
			return n, nil
		}
		if i < settleAttempts-1 {
			if sleep(ctx, time.Duration(i+1)*200*time.Millisecond) != nil {
				break
			}
		}
	}
	return 0, err
}

// nakOrTerm defers a message after a database failure that left the delivery untouched. On the
// last delivery there is nothing more JetStream will do, and the database could not be written to
// record a failure: the message is terminated and the problem logged.
func (w *worker) nakOrTerm(msg jetstream.Msg, n int, id uuid.UUID, what string, cause error) {
	if n < MaxDeliver {
		w.nak(msg, id, w.backoff.Delay(n))
		return
	}
	w.terminate(msg, "delivery stuck pending after the last delivery: "+what, cause)
}

func (w *worker) nak(msg jetstream.Msg, id uuid.UUID, delay time.Duration) {
	if err := msg.NakWithDelay(delay); err != nil {
		w.log.Warn("nak failed, the message will come back after the ack wait", "delivery_id", id, "error", err)
	}
}

func (w *worker) ack(msg jetstream.Msg, id uuid.UUID) {
	if err := msg.Ack(); err != nil {
		// The delivery is settled in the database; a redelivery finds it final or claimed.
		w.log.Warn("ack failed, the message will be redelivered and dropped", "delivery_id", id, "error", err)
	}
}

// terminate drops a message that can never be processed, logging why.
func (w *worker) terminate(msg jetstream.Msg, reason string, cause error) {
	w.log.Error("send message terminated", "reason", reason, "subject", msg.Subject(), "error", cause)
	if err := msg.Term(); err != nil {
		w.log.Warn("terminate failed", "error", err)
	}
}

// deliveries returns how many times JetStream has delivered msg, this time included.
func deliveries(msg jetstream.Msg) int {
	meta, err := msg.Metadata()
	if err != nil {
		return 1
	}
	return int(meta.NumDelivered)
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// truncate bounds an error text for last_error, cutting on a character boundary.
func truncate(s string) string {
	if len(s) <= maxErrorLen {
		return s
	}
	s = s[:maxErrorLen]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}
