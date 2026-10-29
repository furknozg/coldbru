package mockprovider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

// maxBody caps a request body.
const maxBody = 1 << 20

// maxKeyLen caps an Idempotency-Key.
const maxKeyLen = 128

// statusClientClosed is recorded when the client left while its answer was held back (nginx's 499).
const statusClientClosed = 499

// Channels.
const (
	ChannelSMS   = "sms"
	ChannelEmail = "email"
)

// Outcomes of a request, the label of mock_requests_total.
const (
	OutcomeAccepted      = "accepted"
	OutcomeReplayed      = "replayed"
	OutcomeRateLimited   = "rate_limited"
	OutcomeInjected5xx   = "injected_5xx"
	OutcomeInjected429   = "injected_429"
	OutcomeTimeout       = "timeout"
	OutcomeInvalid       = "invalid"
	OutcomeForwardFailed = "forward_failed"
)

var (
	channels = []string{ChannelSMS, ChannelEmail}
	outcomes = []string{OutcomeAccepted, OutcomeReplayed, OutcomeRateLimited, OutcomeInjected5xx,
		OutcomeInjected429, OutcomeTimeout, OutcomeInvalid, OutcomeForwardFailed}

	e164 = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
)

// SMSRequest is the body of POST /v1/sms.
type SMSRequest struct {
	// To is the recipient phone number in E.164 format.
	To string `json:"to" example:"+905551112233" validate:"required"`
	// Text is the message.
	Text string `json:"text" example:"Merhaba Ayşe, sepetinde ürün kaldı: %30 indirim seni bekliyor." validate:"required"`
}

// EmailRequest is the body of POST /v1/email.
type EmailRequest struct {
	// To is the recipient email address.
	To string `json:"to" example:"ayse@example.com" validate:"required"`
	// Subject is the subject line.
	Subject string `json:"subject" example:"Sepetin seni bekliyor" validate:"required"`
	// HTML is the HTML part of the message.
	HTML string `json:"html" example:"<p>Merhaba Ayşe</p>" validate:"required"`
	// Text is the plain-text alternative of the message.
	Text string `json:"text" example:"Merhaba Ayşe" validate:"required"`
}

// Accepted is the body of a 202 answer.
type Accepted struct {
	// MessageID is the provider's id of the message. A repeated Idempotency-Key gets the same id.
	MessageID string `json:"message_id" example:"019a0e6e-0000-7000-8000-000000000001" validate:"required"`
}

// HandlerConfig configures a Handler.
type HandlerConfig struct {
	Config Config
	// Forwarder is the inbox. Required.
	Forwarder Forwarder
	// Registerer receives mock_requests_total. Nil registers nothing.
	Registerer prometheus.Registerer
	// Logger defaults to slog.Default.
	Logger *slog.Logger
	// Now replaces time.Now: the clock of the TTL, of the token buckets and of the email Date.
	Now func() time.Time
	// Sleep replaces the wait of the latency and of the held answer; it returns early with the
	// context's error when ctx is done. Tests inject a recorder.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Handler serves the mock provider's endpoints.
type Handler struct {
	cfg      Config
	fwd      Forwarder
	log      *slog.Logger
	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
	store    *store
	flight   *flight
	faults   *injector
	limiters map[string]*rate.Limiter
	requests *prometheus.CounterVec
}

// NewHandler returns a Handler. It validates the configuration.
func NewHandler(hc HandlerConfig) (*Handler, error) {
	if err := hc.Config.Validate(); err != nil {
		return nil, err
	}
	if hc.Forwarder == nil {
		return nil, errors.New("mockprovider: a Forwarder is required")
	}
	h := &Handler{
		cfg: hc.Config, fwd: hc.Forwarder, log: hc.Logger, now: hc.Now, sleep: hc.Sleep,
		flight: newFlight(), faults: newInjector(hc.Config), limiters: make(map[string]*rate.Limiter),
	}
	if h.log == nil {
		h.log = slog.Default()
	}
	if h.now == nil {
		h.now = time.Now
	}
	if h.sleep == nil {
		h.sleep = sleep
	}
	h.store = newStore(hc.Config.IdempotencyTTL, h.now)
	for _, ch := range channels {
		h.limiters[ch] = rate.NewLimiter(rate.Limit(hc.Config.RateLimit), hc.Config.Burst)
	}
	h.requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mock_requests_total",
		Help: "Requests to the mock provider by channel and outcome.",
	}, []string{"channel", "outcome"})
	for _, ch := range channels {
		for _, o := range outcomes {
			h.requests.WithLabelValues(ch, o)
		}
	}
	if hc.Registerer != nil {
		if err := hc.Registerer.Register(h.requests); err != nil {
			return nil, err
		}
	}
	return h, nil
}

// Mount registers the endpoints on r.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/v1/sms", h.sms)
	r.Post("/v1/email", h.email)
}

// Sweep removes the expired idempotency records and returns how many it removed.
func (h *Handler) Sweep() int { return h.store.sweep() }

// Stored returns the number of idempotency records held.
func (h *Handler) Stored() int { return h.store.len() }

// RunSweeper sweeps every interval until ctx is done.
func (h *Handler) RunSweeper(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n := h.Sweep(); n > 0 {
				h.log.Debug("swept idempotency records", "removed", n)
			}
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// job is a decoded request of one channel.
type job struct {
	channel string
	// canon is the canonical JSON of the decoded body, the input of the body hash.
	canon []byte
	// validate returns the first problem of the fields, empty when they are valid.
	validate func() string
	// forward hands the accepted message to the inbox.
	forward func(ctx context.Context, messageID, key string) error
}

// sms godoc
//
//	@Summary		Send an SMS
//	@Description	Accepts one SMS and hands it to MockServer, where it shows on the dashboard. The Idempotency-Key header is required: a repeated key with the same body gets the stored answer (same `message_id`) and is forwarded no second time; the same key with another body is 422. Order of the checks: idempotency lookup, validation, the per-channel rate limit (110/s, 429 with Retry-After: 1), injected faults (5% 502/503, 2% 429 with Retry-After 1-3, 1% accepted but the 202 is held back 12 s), 50-300 ms latency, forward, store. Faults and rate-limit answers are not stored.
//	@Tags			provider
//	@Accept			json
//	@Produce		json
//	@Param			Idempotency-Key	header		string				true	"Delivery id; at most 128 printable ASCII characters"
//	@Param			body			body		mockprovider.SMSRequest	true	"The message"
//	@Success		202				{object}	mockprovider.Accepted
//	@Failure		400				{object}	mockprovider.Problem	"Missing or invalid Idempotency-Key, or the body is not JSON"
//	@Failure		413				{object}	mockprovider.Problem	"Body above 1 MiB"
//	@Failure		422				{object}	mockprovider.Problem	"Invalid fields, or the key was used with another body"
//	@Failure		429				{object}	mockprovider.Problem	"Rate limit or injected 429; Retry-After in seconds"
//	@Failure		502				{object}	mockprovider.Problem	"Injected failure"
//	@Failure		503				{object}	mockprovider.Problem	"Injected failure, or the inbox is unreachable"
//	@Router			/v1/sms [post]
func (h *Handler) sms(w http.ResponseWriter, r *http.Request) {
	var req SMSRequest
	canon, ok := h.decode(w, r, ChannelSMS, &req)
	if !ok {
		return
	}
	h.process(w, r, job{
		channel: ChannelSMS, canon: canon,
		validate: func() string {
			switch {
			case !e164.MatchString(req.To):
				return "to must be an E.164 phone number such as +905551112233."
			case strings.TrimSpace(req.Text) == "":
				return "text must not be empty."
			}
			return ""
		},
		forward: func(ctx context.Context, id, key string) error {
			return h.fwd.SendSMS(ctx, SMS{To: req.To, Text: req.Text, MessageID: id, IdempotencyKey: key})
		},
	})
}

// email godoc
//
//	@Summary		Send an email
//	@Description	Accepts one email and hands it to Mailpit over SMTP as a multipart/alternative message (text and HTML, UTF-8, RFC 2047 subject, `X-Idempotency-Key` header). Idempotency, rate limit, fault injection and latency are as for POST /v1/sms.
//	@Tags			provider
//	@Accept			json
//	@Produce		json
//	@Param			Idempotency-Key	header		string					true	"Delivery id; at most 128 printable ASCII characters"
//	@Param			body			body		mockprovider.EmailRequest	true	"The message"
//	@Success		202				{object}	mockprovider.Accepted
//	@Failure		400				{object}	mockprovider.Problem	"Missing or invalid Idempotency-Key, or the body is not JSON"
//	@Failure		413				{object}	mockprovider.Problem	"Body above 1 MiB"
//	@Failure		422				{object}	mockprovider.Problem	"Invalid fields, or the key was used with another body"
//	@Failure		429				{object}	mockprovider.Problem	"Rate limit or injected 429; Retry-After in seconds"
//	@Failure		502				{object}	mockprovider.Problem	"Injected failure"
//	@Failure		503				{object}	mockprovider.Problem	"Injected failure, or the inbox is unreachable"
//	@Router			/v1/email [post]
func (h *Handler) email(w http.ResponseWriter, r *http.Request) {
	var req EmailRequest
	canon, ok := h.decode(w, r, ChannelEmail, &req)
	if !ok {
		return
	}
	h.process(w, r, job{
		channel: ChannelEmail, canon: canon,
		validate: func() string {
			if a, err := mail.ParseAddress(req.To); err != nil || a.Address != req.To || a.Name != "" || strings.ContainsAny(req.To, "\r\n") {
				return "to must be an email address such as ayse@example.com."
			}
			switch {
			case strings.TrimSpace(req.Subject) == "":
				return "subject must not be empty."
			case strings.ContainsAny(req.Subject, "\r\n"):
				return "subject must be a single line."
			case strings.TrimSpace(req.HTML) == "":
				return "html must not be empty."
			case strings.TrimSpace(req.Text) == "":
				return "text must not be empty."
			}
			return ""
		},
		forward: func(ctx context.Context, id, key string) error {
			return h.fwd.SendEmail(ctx, Email{
				From: h.cfg.EmailFrom, To: req.To, Subject: req.Subject, HTML: req.HTML, Text: req.Text,
				MessageID: id, IdempotencyKey: key, Date: h.now(),
			})
		},
	})
}

// decode reads the body into into and returns its canonical JSON. A problem is answered here and ok
// is false.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, channel string, into any) (canon []byte, ok bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.invalid(w, r, channel, http.StatusRequestEntityTooLarge, CodeTooLarge, "The body is above 1 MiB.")
		} else {
			h.invalid(w, r, channel, http.StatusBadRequest, CodeInvalidJSON, "The body could not be read.")
		}
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil || dec.More() {
		h.invalid(w, r, channel, http.StatusBadRequest, CodeInvalidJSON, "The body must be one JSON object with the fields of the endpoint.")
		return nil, false
	}
	canon, err = json.Marshal(into)
	if err != nil {
		h.invalid(w, r, channel, http.StatusBadRequest, CodeInvalidJSON, "The body could not be read.")
		return nil, false
	}
	return canon, true
}

func (h *Handler) invalid(w http.ResponseWriter, r *http.Request, channel string, status int, code, detail string) {
	h.requests.WithLabelValues(channel, OutcomeInvalid).Inc()
	writeProblem(w, r, status, code, detail, nil)
}

// validKey reports whether key is 1 to 128 printable ASCII characters.
func validKey(key string) bool {
	if key == "" || len(key) > maxKeyLen {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x21 || key[i] > 0x7e {
			return false
		}
	}
	return true
}

// process runs the steps of D-081 for a decoded request.
func (h *Handler) process(w http.ResponseWriter, r *http.Request, j job) {
	ctx := r.Context()
	key := r.Header.Get("Idempotency-Key")
	switch {
	case key == "":
		h.invalid(w, r, j.channel, http.StatusBadRequest, CodeMissingKey, "Send the delivery id in the Idempotency-Key header.")
		return
	case !validKey(key):
		h.invalid(w, r, j.channel, http.StatusBadRequest, CodeInvalidKey, "The key must be 1 to 128 printable ASCII characters.")
		return
	}
	scope := j.channel + ":" + key
	hash := sha256.Sum256(j.canon)

	release, ok := h.flight.acquire(scope, ctx.Done())
	if !ok {
		w.WriteHeader(statusClientClosed)
		return
	}
	released := false
	releaseOnce := func() {
		if !released {
			released = true
			release()
		}
	}
	defer releaseOnce()

	// (1) Idempotency: a known key is answered from the store, with no forward, fault or rate limit.
	if rec, ok := h.store.get(scope); ok {
		if rec.hash != hash {
			h.invalid(w, r, j.channel, http.StatusUnprocessableEntity, CodeKeyConflict, "This Idempotency-Key was used with another body.")
			return
		}
		h.requests.WithLabelValues(j.channel, OutcomeReplayed).Inc()
		writeAccepted(w, rec.status, rec.body)
		return
	}
	if msg := j.validate(); msg != "" {
		h.invalid(w, r, j.channel, http.StatusUnprocessableEntity, CodeValidation, msg)
		return
	}

	// (2) The token bucket of the channel.
	if !h.limiters[j.channel].AllowN(h.now(), 1) {
		h.requests.WithLabelValues(j.channel, OutcomeRateLimited).Inc()
		writeProblem(w, r, http.StatusTooManyRequests, CodeRateLimited, "More than "+strconv.FormatFloat(h.cfg.RateLimit, 'f', -1, 64)+" requests per second on this channel.", http.Header{"Retry-After": {"1"}})
		return
	}

	// (3) Fault injection. Nothing of a failed request is stored or forwarded.
	fault := h.faults.next()
	switch fault {
	case Fault5xx:
		h.requests.WithLabelValues(j.channel, OutcomeInjected5xx).Inc()
		writeProblem(w, r, h.faults.status5xx(), CodeInjectedFailure, "Injected failure; repeat the request with the same Idempotency-Key.", nil)
		return
	case Fault429:
		h.requests.WithLabelValues(j.channel, OutcomeInjected429).Inc()
		writeProblem(w, r, http.StatusTooManyRequests, CodeRateLimited, "Injected rate limit.", http.Header{"Retry-After": {strconv.Itoa(h.faults.retryAfter())}})
		return
	}

	// (4) Latency.
	if err := h.sleep(ctx, h.faults.latency()); err != nil {
		w.WriteHeader(statusClientClosed)
		return
	}

	// (5) Forward and (6) store. Both run to the end even if the client leaves meanwhile: a message
	// that reached the inbox must be stored, or the retry would forward it again.
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*forwardTimeout)
	defer cancel()
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.New()
	}
	if err := j.forward(work, id.String(), key); err != nil {
		h.log.Warn("forward failed", "channel", j.channel, "error", err)
		h.requests.WithLabelValues(j.channel, OutcomeForwardFailed).Inc()
		writeProblem(w, r, http.StatusServiceUnavailable, CodeForwardFailed, "The inbox did not take the message; repeat the request with the same Idempotency-Key.", nil)
		return
	}
	body, _ := json.Marshal(Accepted{MessageID: id.String()})
	body = append(body, '\n')
	h.store.put(scope, hash, http.StatusAccepted, body)

	if fault == FaultTimeout {
		// The message is in the inbox and stored. Let go of the key so the client's retry gets the
		// stored answer, then hold this answer back until the client has given up.
		releaseOnce()
		h.requests.WithLabelValues(j.channel, OutcomeTimeout).Inc()
		if err := h.sleep(ctx, h.cfg.TimeoutDelay); err != nil {
			w.WriteHeader(statusClientClosed)
			return
		}
		writeAccepted(w, http.StatusAccepted, body)
		return
	}
	h.requests.WithLabelValues(j.channel, OutcomeAccepted).Inc()
	writeAccepted(w, http.StatusAccepted, body)
}

func writeAccepted(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The status line is gone already; a failed write only means the client left.
	_, _ = w.Write(body)
}
