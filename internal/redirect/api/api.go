package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/redirect/code"
	"github.com/furknozg/coldbru/internal/redirect/store"
)

const (
	// maxBodyBytes bounds a POST /links body: 1000 links of at most ~2.4 kB each is far below it.
	maxBodyBytes = 4 << 20
	// maxCodeRounds is how many times a request draws codes for the deliveries that still have
	// none. A round is needed only when a code collides, which has a chance of about 1 in 56
	// billion per code, so a second round is already rare and a fifth never happens.
	maxCodeRounds = 5
	// clickTimeout bounds the transaction of one click.
	clickTimeout = 5 * time.Second
)

// Click outcomes, the label of redirect_clicks_total.
const (
	resultFirst   = "first"
	resultRepeat  = "repeat"
	resultInvalid = "invalid_code"
	resultUnknown = "unknown_code"
	resultError   = "error"
)

// Config configures a Handler.
type Config struct {
	// Pool is the redirect_db pool.
	Pool *pgxpool.Pool
	// Signer makes and verifies the codes.
	Signer *code.Signer
	// FallbackURL is where a request for an unknown or badly signed code is sent (CAMPAIGN_CTA_URL).
	FallbackURL string
	// Logger defaults to slog.Default.
	Logger *slog.Logger
	// Registerer receives the metrics; nil registers them nowhere.
	Registerer prometheus.Registerer
	// NewCode makes a code for a new link; nil means Signer.New. Tests set it to force collisions.
	NewCode func() (string, error)
}

// Handler serves POST /links and GET /r/{code}.
type Handler struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	signer   *code.Signer
	fallback string
	log      *slog.Logger
	clicks   *prometheus.CounterVec
	newCode  func() (string, error)
}

// New returns a Handler for cfg.
func New(cfg Config) (*Handler, error) {
	if cfg.Pool == nil || cfg.Signer == nil {
		return nil, errors.New("redirect api: a database pool and a signer are required")
	}
	if err := checkTargetURL(cfg.FallbackURL); err != nil {
		return nil, fmt.Errorf("redirect api: fallback URL: %w", err)
	}
	h := &Handler{
		pool: cfg.Pool, q: store.New(cfg.Pool), signer: cfg.Signer, fallback: cfg.FallbackURL, log: cfg.Logger, newCode: cfg.NewCode,
		clicks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "redirect_clicks_total",
			Help: "Requests to /r/{code} by outcome: first or repeat click, invalid (bad tag) or unknown code, error.",
		}, []string{"result"}),
	}
	if h.log == nil {
		h.log = slog.Default()
	}
	if h.newCode == nil {
		h.newCode = cfg.Signer.New
	}
	if cfg.Registerer != nil {
		cfg.Registerer.MustRegister(h.clicks)
	}
	for _, r := range []string{resultFirst, resultRepeat, resultInvalid, resultUnknown, resultError} {
		h.clicks.WithLabelValues(r)
	}
	return h, nil
}

// Mount registers the endpoints on r.
func (h *Handler) Mount(r chi.Router) {
	r.Post("/links", h.createLinks)
	r.Get("/r/{code}", h.redirect)
}

// createLinks godoc
//
//	@Summary		Create the short links of a recipient batch
//	@Description	Returns one 10 character code per delivery (D-106). The delivery id is the idempotency key: a delivery that has a link gets the same code again and nothing else changes, so Notification can retry a batch safely. Up to 1000 links per request. The link recipients click is `<TRACKING_BASE_URL>/r/<code>`. Compose-internal: nginx does not route this endpoint.
//	@Tags			links
//	@Accept			json
//	@Produce		json
//	@Param			body	body		api.CreateLinksRequest	true	"The deliveries to link"
//	@Success		200		{object}	api.CreateLinksResponse
//	@Failure		400		{object}	api.Problem	"the body is not valid JSON"
//	@Failure		422		{object}	api.Problem	"a field is missing or invalid, or more than 1000 links"
//	@Failure		500		{object}	api.Problem
//	@Router			/links [post]
func (h *Handler) createLinks(w http.ResponseWriter, r *http.Request) {
	var req CreateLinksRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		fail(w, r, CodeInvalidJSON, "The request body is not valid JSON for this endpoint.", err.Error())
		return
	}
	if problems := validateLinks(req.Links); len(problems) > 0 {
		fail(w, r, CodeInvalidRequest, "The request is invalid.", problems...)
		return
	}
	codes, err := h.ensureLinks(r.Context(), req.Links)
	if err != nil {
		h.log.Error("redirect api: create links failed", "links", len(req.Links), "request_id", middleware.GetReqID(r.Context()), "error", err)
		fail(w, r, CodeInternal, "Something went wrong; try again later.")
		return
	}
	out := CreateLinksResponse{Links: make([]LinkResult, len(req.Links))}
	for i, l := range req.Links {
		out.Links[i] = LinkResult{DeliveryID: l.DeliveryID, Code: codes[l.DeliveryID]}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

// validateLinks returns one message per problem, prefixed with the index of the link.
func validateLinks(links []LinkRequest) []string {
	var problems []string
	switch n := len(links); {
	case n == 0:
		return []string{"links: at least one link is required"}
	case n > MaxLinksPerRequest:
		return []string{fmt.Sprintf("links: %d links, at most %d per request", n, MaxLinksPerRequest)}
	}
	add := func(i int, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("links[%d]: ", i)+fmt.Sprintf(format, args...))
	}
	for i, l := range links {
		switch {
		case l.DeliveryID == uuid.Nil:
			add(i, "delivery_id is missing")
		case l.CampaignID == uuid.Nil:
			add(i, "campaign_id is missing")
		case l.Channel != "sms" && l.Channel != "email":
			add(i, "channel %q is not sms or email", l.Channel)
		case l.Language != "tr" && l.Language != "en":
			add(i, "language %q is not tr or en", l.Language)
		case strings.TrimSpace(l.Timezone) == "" || len(l.Timezone) > maxTimezoneLen:
			add(i, "timezone must be 1 to %d characters", maxTimezoneLen)
		default:
			if err := checkTargetURL(l.TargetURL); err != nil {
				add(i, "target_url: %v", err)
			}
		}
	}
	return problems
}

// checkTargetURL accepts an absolute http or https URL with a host and no credentials.
func checkTargetURL(raw string) error {
	if len(raw) > maxTargetURLLen {
		return fmt.Errorf("longer than %d characters", maxTargetURLLen)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || strings.ContainsAny(raw, " \t\r\n") {
		return errors.New("must be an absolute http or https URL without credentials")
	}
	return nil
}

// ensureLinks makes sure every delivery of links has a link and returns the code of each. A
// delivery that has one keeps its code (first request wins). The others get fresh codes; if the
// database refuses a code because it is taken, the delivery is still without one after the insert
// and is given a new code in the next round.
func (h *Handler) ensureLinks(ctx context.Context, links []LinkRequest) (map[uuid.UUID]string, error) {
	byDelivery := make(map[uuid.UUID]LinkRequest, len(links))
	ids := make([]uuid.UUID, 0, len(links))
	for _, l := range links {
		if _, dup := byDelivery[l.DeliveryID]; !dup {
			byDelivery[l.DeliveryID] = l
			ids = append(ids, l.DeliveryID)
		}
	}
	codes := make(map[uuid.UUID]string, len(ids))
	for round := 0; round < maxCodeRounds; round++ {
		existing, err := h.q.ListLinksByDelivery(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("list links: %w", err)
		}
		for _, e := range existing {
			codes[e.DeliveryID] = e.Code
		}
		if len(codes) == len(ids) {
			return codes, nil
		}
		var rows []store.InsertLinkParams
		for _, id := range ids {
			if _, has := codes[id]; has {
				continue
			}
			c, err := h.newCode()
			if err != nil {
				return nil, err
			}
			l := byDelivery[id]
			rows = append(rows, store.InsertLinkParams{
				Code: c, DeliveryID: id, CampaignID: l.CampaignID, Channel: l.Channel,
				Language: l.Language, Timezone: l.Timezone, TargetUrl: l.TargetURL,
			})
		}
		var batchErr error
		h.q.InsertLink(ctx, rows).Exec(func(_ int, err error) {
			if err != nil && batchErr == nil {
				batchErr = err
			}
		})
		if batchErr != nil {
			return nil, fmt.Errorf("insert links: %w", batchErr)
		}
	}
	// The loop checks before it inserts, so the last round's inserts are read once more here.
	existing, err := h.q.ListLinksByDelivery(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	for _, e := range existing {
		codes[e.DeliveryID] = e.Code
	}
	if len(codes) != len(ids) {
		return nil, fmt.Errorf("%d of %d deliveries still have no link after %d rounds", len(ids)-len(codes), len(ids), maxCodeRounds)
	}
	return codes, nil
}

// redirect godoc
//
//	@Summary		Follow a short link
//	@Description	Records the click and redirects (302) to the target stored with the link (D-106, D-107). The target comes from the link's record, never from the request, so the endpoint cannot be used as an open redirect. A code whose tag does not verify, or that no link has, is not counted and is redirected to the default campaign page (CAMPAIGN_CTA_URL). Every GET counts; bots and link previews are not filtered (D-108). No IP address or User-Agent is stored.
//	@Tags			links
//	@Param			code	path	string	true	"The 10 character code"
//	@Success		302		"Redirect to the stored target, or to the default page for a bad code"
//	@Header			302		{string}	Location	"The target URL"
//	@Router			/r/{code} [get]
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	c := chi.URLParam(r, "code")
	if !h.signer.Verify(c) {
		h.clicks.WithLabelValues(resultInvalid).Inc()
		h.found(w, r, h.fallback)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), clickTimeout)
	defer cancel()
	target, result, err := h.recordClick(ctx, c)
	h.clicks.WithLabelValues(result).Inc()
	if err != nil {
		// The recipient still gets somewhere; the click is lost and the error is counted.
		h.log.Error("redirect api: click not recorded", "code", c, "request_id", middleware.GetReqID(r.Context()), "error", err)
		h.found(w, r, h.fallback)
		return
	}
	if target == "" {
		h.found(w, r, h.fallback)
		return
	}
	h.found(w, r, target)
}

// found answers with a 302 that no cache may keep, so every click reaches the service.
func (h *Handler) found(w http.ResponseWriter, r *http.Request, target string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, target, http.StatusFound)
}

// recordClick inserts the click and its event in one transaction and returns the link's target.
// An empty target with a nil error means no link has the code.
func (h *Handler) recordClick(ctx context.Context, c string) (target, result string, err error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return "", resultError, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // no-op after Commit
	q := h.q.WithTx(tx)

	link, err := q.LockLink(ctx, c)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", resultUnknown, nil
	}
	if err != nil {
		return "", resultError, fmt.Errorf("lock link: %w", err)
	}
	click, err := q.InsertClick(ctx, c)
	if err != nil {
		return "", resultError, fmt.Errorf("insert click: %w", err)
	}
	env, err := events.New(EventTypeClicked, EventClickVersion, EventProducer, link.CampaignID.String(), ClickedData{
		ClickID: click.ID.String(), DeliveryID: link.DeliveryID.String(), CampaignID: link.CampaignID.String(),
		Channel: link.Channel, Language: link.Language, Timezone: link.Timezone,
		ClickedAt: click.ClickedAt.Time.UTC(), First: click.First,
	})
	if err != nil {
		return "", resultError, err
	}
	if _, err := outbox.Write(ctx, tx, env); err != nil {
		return "", resultError, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", resultError, fmt.Errorf("commit: %w", err)
	}
	if click.First {
		return link.TargetUrl, resultFirst, nil
	}
	return link.TargetUrl, resultRepeat, nil
}
