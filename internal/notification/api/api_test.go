package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/notification/api"
)

var (
	campID  = uuid.MustParse("0199f3a1-0000-7000-8000-000000000001")
	fixedAt = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
)

// fakeSource answers Summary from a value the test changes.
type fakeSource struct {
	mu    sync.Mutex
	sum   api.DeliverySummary
	err   error
	calls int
}

func (f *fakeSource) set(status string, counts ...api.ChannelCount) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sum = summary(status, counts...)
}

func (f *fakeSource) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeSource) Summary(_ context.Context, id uuid.UUID) (api.DeliverySummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return api.DeliverySummary{}, f.err
	}
	if id != campID {
		return api.DeliverySummary{}, api.ErrNotFound
	}
	s := f.sum
	s.UpdatedAt = time.Now().UTC() // the read time changes on every call, the state does not
	return s, nil
}

func summary(status string, counts ...api.ChannelCount) api.DeliverySummary {
	s := api.DeliverySummary{CampaignID: campID, Status: status, Counts: append([]api.ChannelCount{}, counts...), UpdatedAt: fixedAt}
	for _, c := range counts {
		s.Total += c.Count
	}
	return s
}

func cc(channel, status string, n int64) api.ChannelCount {
	return api.ChannelCount{Channel: channel, Status: status, Count: n}
}

// ticks drives the poll and the ping separately: the handler asks for a ticker per interval.
type ticks struct {
	mu      sync.Mutex
	byEvery map[time.Duration]chan time.Time
	stopped int
}

func newTicks() *ticks { return &ticks{byEvery: map[time.Duration]chan time.Time{}} }

func (k *ticks) config(c *api.Config) {
	c.Ticker = func(d time.Duration) (<-chan time.Time, func()) {
		k.mu.Lock()
		defer k.mu.Unlock()
		ch := make(chan time.Time)
		k.byEvery[d] = ch
		return ch, func() { k.mu.Lock(); k.stopped++; k.mu.Unlock() }
	}
}

// fire delivers one tick of the ticker of interval d, waiting for the handler to have asked for it.
func (k *ticks) fire(t *testing.T, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		k.mu.Lock()
		ch := k.byEvery[d]
		k.mu.Unlock()
		if ch != nil {
			select {
			case ch <- time.Now():
				return
			case <-time.After(5 * time.Second):
				t.Fatalf("the %s ticker was not read", d)
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s ticker was created", d)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (k *ticks) stoppedCount() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.stopped
}

func serve(t *testing.T, src api.Source, mods ...func(*api.Config)) *httptest.Server {
	t.Helper()
	cfg := api.Config{Source: src, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return fixedAt }}
	for _, m := range mods {
		m(&cfg)
	}
	r := chi.NewRouter()
	api.New(cfg).Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

type frame struct{ event, data string }

type stream struct {
	t      *testing.T
	resp   *http.Response
	frames chan frame
}

func openStream(t *testing.T, srv *httptest.Server, id string) *stream {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/campaigns/"+id+"/deliveries/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	s := &stream{t: t, resp: resp, frames: make(chan frame, 64)}
	go func() {
		defer close(s.frames)
		sc := bufio.NewScanner(resp.Body)
		var cur frame
		for sc.Scan() {
			switch line := sc.Text(); {
			case line == "":
				if cur.event != "" {
					s.frames <- cur
				}
				cur = frame{}
			case strings.HasPrefix(line, "event: "):
				cur.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				cur.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return s
}

func (s *stream) next() frame {
	s.t.Helper()
	select {
	case f, ok := <-s.frames:
		if !ok {
			s.t.Fatal("the stream ended, want another frame")
		}
		return f
	case <-time.After(5 * time.Second):
		s.t.Fatal("no frame within 5s")
		return frame{}
	}
}

func (s *stream) ended() {
	s.t.Helper()
	select {
	case f, ok := <-s.frames:
		if ok {
			s.t.Fatalf("got frame %+v, want the end of the stream", f)
		}
	case <-time.After(5 * time.Second):
		s.t.Fatal("the stream did not end within 5s")
	}
}

// quiet fails if a frame arrives within a short time.
func (s *stream) quiet() {
	s.t.Helper()
	select {
	case f, ok := <-s.frames:
		if ok {
			s.t.Fatalf("unexpected frame %+v", f)
		}
		s.t.Fatal("the stream ended unexpectedly")
	case <-time.After(60 * time.Millisecond):
	}
}

func snapshotOf(t *testing.T, f frame) api.DeliverySummary {
	t.Helper()
	if f.event != api.EventSnapshot {
		t.Fatalf("event = %q (%s), want snapshot", f.event, f.data)
	}
	var s api.DeliverySummary
	if err := json.Unmarshal([]byte(f.data), &s); err != nil {
		t.Fatalf("snapshot data %q: %v", f.data, err)
	}
	return s
}

func TestDeliveriesReturnsTheSummary(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("sending", cc("email", "sent", 2), cc("sms", "pending", 3), cc("sms", "sent", 4))
	srv := serve(t, src)
	resp, err := http.Get(srv.URL + "/campaigns/" + campID.String() + "/deliveries")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"campaign_id", "status", "counts", "total", "updated_at"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("response has no %q", k)
		}
	}
	if len(raw) != 5 {
		t.Errorf("response has %d members, want 5: %v", len(raw), raw)
	}
	var counts []api.ChannelCount
	if err := json.Unmarshal(raw["counts"], &counts); err != nil || len(counts) != 3 || counts[0] != cc("email", "sent", 2) {
		t.Errorf("counts = %v (%v)", counts, err)
	}
	if string(raw["total"]) != "9" || string(raw["status"]) != `"sending"` {
		t.Errorf("total = %s, status = %s", raw["total"], raw["status"])
	}
}

func TestDeliveriesEmptyCountsIsAnArray(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("scheduled")
	srv := serve(t, src)
	resp, err := http.Get(srv.URL + "/campaigns/" + campID.String() + "/deliveries")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), `"counts":[]`) || !strings.Contains(string(b), `"total":0`) {
		t.Errorf("body = %s", b)
	}
}

func TestProblems(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("sending")
	srv := serve(t, src)
	tests := []struct {
		name, path string
		status     int
		code       string
	}{
		{"unknown campaign", "/campaigns/" + uuid.NewString() + "/deliveries", 404, api.CodeNotFound},
		{"unknown campaign stream", "/campaigns/" + uuid.NewString() + "/deliveries/events", 404, api.CodeNotFound},
		{"not a uuid", "/campaigns/nope/deliveries", 400, api.CodeInvalidID},
		{"not a uuid stream", "/campaigns/nope/deliveries/events", 400, api.CodeInvalidID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.status || resp.Header.Get("Content-Type") != api.ProblemContentType {
				t.Errorf("status %d, content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
			}
			var p api.Problem
			if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
				t.Fatal(err)
			}
			if p.Code != tt.code || p.Status != tt.status || p.Type != "urn:coldbru:notification:problem:"+tt.code || p.Instance != tt.path || p.Title == "" {
				t.Errorf("problem = %+v", p)
			}
		})
	}
}

func TestInternalErrorsHideTheCause(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.fail(errors.New("connection to 10.1.2.3 refused"))
	srv := serve(t, src)
	for _, path := range []string{"/deliveries", "/deliveries/events"} {
		resp, err := http.Get(srv.URL + "/campaigns/" + campID.String() + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var p api.Problem
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatalf("%s: %v (%s)", path, err, b)
		}
		if resp.StatusCode != 500 || p.Code != api.CodeInternal || strings.Contains(string(b), "10.1.2.3") {
			t.Errorf("%s: status %d, body %s", path, resp.StatusCode, b)
		}
	}
}

func TestEventsStartWithASnapshotAndSendOnlyChanges(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("sending", cc("sms", "pending", 3))
	k := newTicks()
	srv := serve(t, src, k.config)
	s := openStream(t, srv, campID.String())

	if ct := s.resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := s.resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if xa := s.resp.Header.Get("X-Accel-Buffering"); xa != "no" {
		t.Errorf("X-Accel-Buffering = %q", xa)
	}
	first := snapshotOf(t, s.next())
	if first.Status != "sending" || first.Total != 3 || len(first.Counts) != 1 || first.CampaignID != campID {
		t.Errorf("first snapshot = %+v", first)
	}

	// Nothing changed: the poll sends nothing, although the read time moved.
	k.fire(t, api.DefaultPollEvery)
	s.quiet()
	k.fire(t, api.DefaultPollEvery)
	s.quiet()

	// A count changed: one new snapshot.
	src.set("sending", cc("sms", "pending", 1), cc("sms", "sent", 2))
	k.fire(t, api.DefaultPollEvery)
	second := snapshotOf(t, s.next())
	if second.Total != 3 || len(second.Counts) != 2 || second.Counts[1] != cc("sms", "sent", 2) {
		t.Errorf("second snapshot = %+v", second)
	}
	k.fire(t, api.DefaultPollEvery)
	s.quiet()
}

func TestEventsPing(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("sending")
	k := newTicks()
	srv := serve(t, src, k.config)
	s := openStream(t, srv, campID.String())
	snapshotOf(t, s.next())

	k.fire(t, api.DefaultPingEvery)
	fr := s.next()
	var p api.PingEvent
	if fr.event != api.EventPing || json.Unmarshal([]byte(fr.data), &p) != nil || !p.At.Equal(fixedAt) {
		t.Errorf("frame = %+v", fr)
	}
}

func TestEventsEndWithDoneWhenTheCampaignCompletes(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"completed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			src := &fakeSource{}
			src.set("sending", cc("sms", "sent", 2), cc("sms", "pending", 1))
			k := newTicks()
			srv := serve(t, src, k.config)
			s := openStream(t, srv, campID.String())
			snapshotOf(t, s.next())

			src.set(status, cc("sms", "sent", 3))
			k.fire(t, api.DefaultPollEvery)
			final := snapshotOf(t, s.next())
			if final.Status != status || final.Counts[0] != cc("sms", "sent", 3) {
				t.Errorf("final snapshot = %+v", final)
			}
			done := s.next()
			var d api.DoneEvent
			if done.event != api.EventDone || json.Unmarshal([]byte(done.data), &d) != nil || d.Status != status {
				t.Errorf("frame = %+v, want done %s", done, status)
			}
			s.ended()
			if k.stoppedCount() != 2 {
				t.Errorf("stopped tickers = %d, want 2", k.stoppedCount())
			}
		})
	}
}

func TestEventsOfAFinishedCampaignSendSnapshotAndDoneAtOnce(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("completed", cc("email", "sent", 5))
	srv := serve(t, src)
	s := openStream(t, srv, campID.String())
	if got := snapshotOf(t, s.next()); got.Status != "completed" {
		t.Errorf("snapshot = %+v", got)
	}
	if fr := s.next(); fr.event != api.EventDone || fr.data != `{"status":"completed"}` {
		t.Errorf("frame = %+v", fr)
	}
	s.ended()
}

func TestEventsSurviveAFailedPoll(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("sending", cc("sms", "pending", 1))
	k := newTicks()
	srv := serve(t, src, k.config)
	s := openStream(t, srv, campID.String())
	snapshotOf(t, s.next())

	src.fail(errors.New("database is down"))
	k.fire(t, api.DefaultPollEvery)
	s.quiet()
	src.fail(nil)
	src.set("sending", cc("sms", "sent", 1))
	k.fire(t, api.DefaultPollEvery)
	if got := snapshotOf(t, s.next()); got.Counts[0] != cc("sms", "sent", 1) {
		t.Errorf("snapshot after the failed poll = %+v", got)
	}
}

func TestEventsEndOnShutdown(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("sending")
	ctx, cancel := context.WithCancel(t.Context())
	srv := serve(t, src, func(c *api.Config) { c.Shutdown = ctx })
	s := openStream(t, srv, campID.String())
	snapshotOf(t, s.next())
	cancel()
	s.ended()
}

func TestEventsStopWhenTheClientLeaves(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	src.set("sending")
	k := newTicks()
	srv := serve(t, src, k.config)
	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/campaigns/"+campID.String()+"/deliveries/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for k.stoppedCount() != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("stopped tickers = %d, want 2: the handler kept running", k.stoppedCount())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
