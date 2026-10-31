//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/furknozg/coldbru/internal/redirect/api"
	"github.com/furknozg/coldbru/internal/redirect/code"
	"github.com/furknozg/coldbru/internal/redirect/rtest"
)

const (
	signingKey = "integration-signing-key-123456"
	fallback   = "https://shop.example.com/default"
	target     = "https://shop.example.com/?utm_campaign=0198a2c4-0000-7000-8000-000000000001"
)

var pg *rtest.Postgres

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	var err error
	pg, err = rtest.Start(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	exit := m.Run()
	pg.Close()
	os.Exit(exit)
}

var noFollow = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type fixture struct {
	t      *testing.T
	pool   *pgxpool.Pool
	srv    *httptest.Server
	signer *code.Signer
	reg    *prometheus.Registry
	logs   *bytes.Buffer
}

func newFixture(t *testing.T, newCode func() (string, error)) *fixture {
	t.Helper()
	pool := pg.NewMigratedPool(t)
	signer, err := code.NewSigner(signingKey)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, pool: pool, signer: signer, reg: prometheus.NewRegistry(), logs: &bytes.Buffer{}}
	h, err := api.New(api.Config{
		Pool: pool, Signer: signer, FallbackURL: fallback, Logger: slog.New(slog.NewTextHandler(f.logs, nil)),
		Registerer: f.reg, NewCode: newCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	h.Mount(r)
	f.srv = httptest.NewServer(r)
	t.Cleanup(f.srv.Close)
	return f
}

func link(delivery uuid.UUID, mutate func(*api.LinkRequest)) api.LinkRequest {
	l := api.LinkRequest{
		DeliveryID: delivery, CampaignID: uuid.MustParse("0198a2c4-0000-7000-8000-000000000001"), Channel: "sms",
		Language: "tr", Timezone: "Europe/Istanbul", TargetURL: target,
	}
	if mutate != nil {
		mutate(&l)
	}
	return l
}

func (f *fixture) create(links ...api.LinkRequest) (int, api.CreateLinksResponse) {
	f.t.Helper()
	body, err := json.Marshal(api.CreateLinksRequest{Links: links})
	if err != nil {
		f.t.Fatal(err)
	}
	resp, err := http.Post(f.srv.URL+"/links", "application/json", bytes.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out api.CreateLinksResponse
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			f.t.Fatal(err)
		}
	}
	return resp.StatusCode, out
}

func (f *fixture) follow(c string) *http.Response {
	f.t.Helper()
	resp, err := noFollow.Get(f.srv.URL + "/r/" + c)
	if err != nil {
		f.t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.t.Context(), sql, args...).Scan(&n); err != nil {
		f.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func TestCreateLinksIsIdempotentOnTheDelivery(t *testing.T) {
	f := newFixture(t, nil)
	d1, d2, d3 := uuid.New(), uuid.New(), uuid.New()

	status, first := f.create(link(d1, nil), link(d2, func(l *api.LinkRequest) { l.Channel = "email"; l.Language = "en"; l.Timezone = "Europe/London" }))
	if status != 200 || len(first.Links) != 2 {
		t.Fatalf("status %d, %+v", status, first)
	}
	for i, d := range []uuid.UUID{d1, d2} {
		l := first.Links[i]
		if l.DeliveryID != d || len(l.Code) != code.Length || !f.signer.Verify(l.Code) {
			t.Errorf("result %d = %+v: want delivery %s and a code that verifies", i, l, d)
		}
	}
	if first.Links[0].Code == first.Links[1].Code {
		t.Error("two deliveries got the same code")
	}

	// A retry of the same batch, now with a third delivery and in another order, returns the codes
	// it gave and creates one new link.
	status, second := f.create(link(d3, nil), link(d2, nil), link(d1, nil))
	if status != 200 || len(second.Links) != 3 {
		t.Fatalf("status %d, %+v", status, second)
	}
	if second.Links[1].Code != first.Links[1].Code || second.Links[2].Code != first.Links[0].Code {
		t.Errorf("a retry changed the codes: first %+v, second %+v", first.Links, second.Links)
	}
	if n := f.count(`SELECT count(*) FROM link`); n != 3 {
		t.Errorf("%d links, want 3", n)
	}

	// The record keeps what the first request said: a retry cannot change the target or the slot.
	status, _ = f.create(link(d1, func(l *api.LinkRequest) { l.TargetURL = "https://evil.example.com"; l.Channel = "email" }))
	var stored struct{ channel, target string }
	if err := f.pool.QueryRow(t.Context(), `SELECT channel, target_url FROM link WHERE delivery_id = $1`, d1).Scan(&stored.channel, &stored.target); err != nil {
		t.Fatal(err)
	}
	if status != 200 || stored.channel != "sms" || stored.target != target {
		t.Errorf("after a conflicting retry: status %d, stored %+v; the first request must stand", status, stored)
	}

	// The same delivery twice in one request is one link.
	d4 := uuid.New()
	_, dup := f.create(link(d4, nil), link(d4, nil))
	if len(dup.Links) != 2 || dup.Links[0].Code != dup.Links[1].Code || f.count(`SELECT count(*) FROM link WHERE delivery_id = $1`, d4) != 1 {
		t.Errorf("duplicate delivery in a request: %+v", dup.Links)
	}
}

func TestCreateLinksDrawsANewCodeOnACollision(t *testing.T) {
	// The first code the generator hands out is taken by another delivery; the delivery that lost
	// it gets the next one in a second round.
	real := func() func() (string, error) {
		s, _ := code.NewSigner(signingKey)
		return s.New
	}()
	var calls atomic.Int32
	var taken string
	f := newFixture(t, func() (string, error) {
		calls.Add(1)
		if taken == "" {
			c, err := real()
			taken = c
			return c, err
		}
		if calls.Load() <= 3 {
			return taken, nil // the second and third request for a code collide
		}
		return real()
	})
	_, a := f.create(link(uuid.New(), nil)) // takes `taken`
	if a.Links[0].Code != taken {
		t.Fatalf("setup: %+v, taken %s", a.Links, taken)
	}
	d := uuid.New()
	status, b := f.create(link(d, nil))
	if status != 200 || len(b.Links) != 1 || b.Links[0].Code == taken || !f.signer.Verify(b.Links[0].Code) {
		t.Fatalf("after a collision: status %d, %+v", status, b)
	}
	if n := f.count(`SELECT count(*) FROM link`); n != 2 {
		t.Errorf("%d links, want 2", n)
	}
}

func TestCreateLinksFailsWhenCodesKeepColliding(t *testing.T) {
	var fixed string
	f := newFixture(t, func() (string, error) { return fixed, nil })
	s, _ := code.NewSigner(signingKey)
	fixed, _ = s.New()
	if status, _ := f.create(link(uuid.New(), nil)); status != 200 {
		t.Fatalf("first link: status %d", status)
	}
	if status, _ := f.create(link(uuid.New(), nil)); status != 500 {
		t.Errorf("a generator that only returns a taken code: status %d, want 500", status)
	}
}

func TestClickRedirectsAndRecordsOnceAsFirst(t *testing.T) {
	f := newFixture(t, nil)
	delivery := uuid.New()
	_, res := f.create(link(delivery, nil))
	c := res.Links[0].Code

	resp := f.follow(c)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != target || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("GET /r/%s = %d, Location %q, Cache-Control %q; want 302 to the stored target, no-store", c, resp.StatusCode, resp.Header.Get("Location"), resp.Header.Get("Cache-Control"))
	}
	f.follow(c)
	f.follow(c)

	rows, err := f.pool.Query(t.Context(), `SELECT first FROM click WHERE code = $1 ORDER BY clicked_at, id`, c)
	if err != nil {
		t.Fatal(err)
	}
	var firsts []bool
	for rows.Next() {
		var first bool
		if err := rows.Scan(&first); err != nil {
			t.Fatal(err)
		}
		firsts = append(firsts, first)
	}
	rows.Close()
	if len(firsts) != 3 || !firsts[0] || firsts[1] || firsts[2] {
		t.Errorf("first flags = %v, want [true false false]", firsts)
	}

	// One event per click, written in the click's transaction, carrying what Analytics needs.
	evRows, err := f.pool.Query(t.Context(), `SELECT subject, payload FROM outbox ORDER BY created_at, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer evRows.Close()
	var seen []api.ClickedData
	for evRows.Next() {
		var subject string
		var payload []byte
		if err := evRows.Scan(&subject, &payload); err != nil {
			t.Fatal(err)
		}
		var env struct {
			Type          string          `json:"type"`
			Producer      string          `json:"producer"`
			CorrelationID string          `json:"correlation_id"`
			Version       int             `json:"version"`
			Data          api.ClickedData `json:"data"`
		}
		if err := json.Unmarshal(payload, &env); err != nil {
			t.Fatal(err)
		}
		if subject != "evt.redirect.clicked.v1" || env.Type != "redirect.clicked" || env.Version != 1 || env.Producer != "redirect" ||
			env.CorrelationID != "0198a2c4-0000-7000-8000-000000000001" {
			t.Errorf("outbox row = %s %+v", subject, env)
		}
		seen = append(seen, env.Data)
	}
	if len(seen) != 3 {
		t.Fatalf("%d events, want 3", len(seen))
	}
	for i, d := range seen {
		if d.DeliveryID != delivery.String() || d.CampaignID != "0198a2c4-0000-7000-8000-000000000001" || d.Channel != "sms" || d.Language != "tr" ||
			d.Timezone != "Europe/Istanbul" || d.ClickedAt.IsZero() || d.First != (i == 0) {
			t.Errorf("event %d = %+v", i, d)
		}
		if _, err := uuid.Parse(d.ClickID); err != nil {
			t.Errorf("event %d click_id %q is not a uuid", i, d.ClickID)
		}
	}
	var clickIDs int
	_ = f.pool.QueryRow(t.Context(), `SELECT count(DISTINCT id) FROM click`).Scan(&clickIDs)
	if clickIDs != 3 {
		t.Errorf("%d distinct click ids, want 3", clickIDs)
	}
}

func TestFirstClickIsUniqueUnderConcurrency(t *testing.T) {
	f := newFixture(t, nil)
	_, res := f.create(link(uuid.New(), nil))
	c := res.Links[0].Code
	const clicks = 25
	var wg sync.WaitGroup
	for range clicks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.follow(c)
		}()
	}
	wg.Wait()
	if n := f.count(`SELECT count(*) FROM click WHERE code = $1`, c); n != clicks {
		t.Errorf("%d clicks stored, want %d", n, clicks)
	}
	if n := f.count(`SELECT count(*) FROM click WHERE code = $1 AND first`, c); n != 1 {
		t.Errorf("%d first clicks, want exactly 1", n)
	}
	if n := f.count(`SELECT count(*) FROM outbox`); n != clicks {
		t.Errorf("%d events, want %d", n, clicks)
	}
}

func TestUnknownAndBadCodesAreNotCounted(t *testing.T) {
	f := newFixture(t, nil)
	_, res := f.create(link(uuid.New(), nil))
	good := res.Links[0].Code
	unknown, err := f.signer.New() // signed, but no link has it
	if err != nil {
		t.Fatal(err)
	}
	badTag := good[:code.IDLen] + "0000"
	if f.signer.Verify(badTag) {
		badTag = good[:code.IDLen] + "1111"
	}
	for name, c := range map[string]string{"unknown code with a valid tag": unknown, "bad tag of a real link": badTag, "garbage": "x"} {
		resp := f.follow(c)
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != fallback {
			t.Errorf("%s: GET /r/%s = %d, Location %q; want 302 to %s", name, c, resp.StatusCode, resp.Header.Get("Location"), fallback)
		}
	}
	if n := f.count(`SELECT count(*) FROM click`); n != 0 {
		t.Errorf("%d clicks stored for bad codes", n)
	}
	if n := f.count(`SELECT count(*) FROM outbox`); n != 0 {
		t.Errorf("%d events written for bad codes", n)
	}
}

func TestNoPersonalDataIsStored(t *testing.T) {
	f := newFixture(t, nil)
	rows, err := f.pool.Query(t.Context(), `SELECT table_name || '.' || column_name FROM information_schema.columns
		WHERE table_schema = 'redirect' AND table_name IN ('link', 'click') ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, c)
	}
	want := []string{"click.clicked_at", "click.code", "click.first", "click.id",
		"link.campaign_id", "link.channel", "link.code", "link.created_at", "link.delivery_id", "link.language", "link.target_url", "link.timezone"}
	if fmt.Sprint(cols) != fmt.Sprint(want) {
		t.Errorf("columns = %v, want %v: no IP address, no User-Agent, no contact", cols, want)
	}
}

func TestClickMetricsCountOutcomes(t *testing.T) {
	f := newFixture(t, nil)
	_, res := f.create(link(uuid.New(), nil))
	c := res.Links[0].Code
	f.follow(c)
	f.follow(c)
	f.follow("short")
	unknown, _ := f.signer.New()
	f.follow(unknown)

	body := scrape(t, f.reg)
	for result, want := range map[string]float64{"first": 1, "repeat": 1, "invalid_code": 1, "unknown_code": 1, "error": 0} {
		got, ok := body[result]
		if !ok || got != want {
			t.Errorf("redirect_clicks_total{result=%q} = %v (present %v), want %v", result, got, ok, want)
		}
	}
}

func scrape(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, mf := range mfs {
		if mf.GetName() != "redirect_clicks_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "result" {
					out[l.GetValue()] = m.GetCounter().GetValue()
				}
			}
		}
	}
	return out
}

func TestClosedDatabaseStillRedirects(t *testing.T) {
	f := newFixture(t, nil)
	_, res := f.create(link(uuid.New(), nil))
	f.pool.Close()
	resp := f.follow(res.Links[0].Code)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != fallback {
		t.Errorf("with the database closed: %d, Location %q; want 302 to the fallback", resp.StatusCode, resp.Header.Get("Location"))
	}
	if got := scrape(t, f.reg)["error"]; got != 1 {
		t.Errorf("error count = %v, want 1", got)
	}
}
