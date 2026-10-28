package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// frame is one SSE frame.
type frame struct {
	event string
	data  string
}

// stream is an open event stream read frame by frame.
type stream struct {
	t      *testing.T
	resp   *http.Response
	frames chan frame
	cancel context.CancelFunc
}

// openStream connects to the event stream of campaign id on srv and reads its frames in the background.
func openStream(t *testing.T, srv *httptest.Server, id uuid.UUID) *stream {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/campaigns/"+id.String()+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	s := &stream{t: t, resp: resp, frames: make(chan frame, 64), cancel: cancel}
	t.Cleanup(func() { cancel(); resp.Body.Close() })
	go func() {
		defer close(s.frames)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		var cur frame
		for sc.Scan() {
			line := sc.Text()
			switch {
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

// next returns the next frame or fails after a second.
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

// ended waits for the server to end the stream.
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

// ticker is a ping ticker the test drives.
type ticker struct {
	c       chan time.Time
	stopped chan struct{}
}

func newTicker() *ticker { return &ticker{c: make(chan time.Time), stopped: make(chan struct{})} }

func (k *ticker) config(c *api.Config) {
	c.Ticker = func(time.Duration) (<-chan time.Time, func()) {
		return k.c, func() { close(k.stopped) }
	}
}

func serve(t *testing.T, f *fakeFlow, mods ...func(*api.Config)) *httptest.Server {
	t.Helper()
	h, _ := newRouter(t, f, mods...)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// waitSubscribers waits until the hub holds n subscriptions of id.
func waitSubscribers(t *testing.T, hub *flow.Hub, id uuid.UUID, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for hub.Subscribers(id) != n {
		if time.Now().After(deadline) {
			t.Fatalf("%d subscribers of %s, want %d", hub.Subscribers(id), id, n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func meta() flow.Meta {
	return flow.Meta{CampaignID: campID, At: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)}
}

func TestEventsStartWithASnapshotAndForwardEvents(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	srv := serve(t, f)
	s := openStream(t, srv, campID)

	if ct := s.resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := s.resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if xa := s.resp.Header.Get("X-Accel-Buffering"); xa != "no" {
		t.Errorf("X-Accel-Buffering = %q", xa)
	}
	first := s.next()
	if first.event != api.EventSnapshot {
		t.Fatalf("first event = %q, want snapshot", first.event)
	}
	var view flow.CampaignView
	if err := json.Unmarshal([]byte(first.data), &view); err != nil || view.ID != campID {
		t.Errorf("snapshot = %s (%v)", first.data, err)
	}
	waitSubscribers(t, f.Hub(), campID, 1)

	step, variant := uuid.New(), uuid.New()
	f.Hub().Publish(flow.StepOpened{Meta: meta(), StepID: step, Kind: flow.KindAudience, Version: 1})
	f.Hub().Publish(flow.ContentDelta{Meta: meta(), StepID: step, Slot: flow.Slot{Language: "tr", Channel: "sms"}, Text: "Merhaba", Attempt: 2})
	f.Hub().Publish(flow.ContentReset{Meta: meta(), StepID: step, Slot: flow.Slot{Language: "tr", Channel: "sms"}, Attempt: 3})
	f.Hub().Publish(flow.VariantCreated{Meta: meta(), StepID: step, VariantID: variant, Kind: flow.KindAudience, Content: json.RawMessage(`{"count":46}`)})
	// An event of another campaign must not arrive.
	f.Hub().Publish(flow.StepOpened{Meta: flow.Meta{CampaignID: otherID, At: meta().At}, StepID: step, Kind: flow.KindAudience})
	f.Hub().Publish(flow.StepApproved{Meta: meta(), StepID: step, Kind: flow.KindAudience, Version: 1})

	want := []struct {
		event string
		check func(map[string]any) bool
	}{
		{flow.TypeStepOpened, func(m map[string]any) bool {
			return m["kind"] == "audience" && m["step_id"] == step.String() && m["campaign_id"] == campID.String()
		}},
		{flow.TypeContentDelta, func(m map[string]any) bool { return m["text"] == "Merhaba" && m["attempt"] == float64(2) }},
		{flow.TypeContentReset, func(m map[string]any) bool { return m["attempt"] == float64(3) }},
		{flow.TypeVariantCreated, func(m map[string]any) bool {
			c, _ := m["content"].(map[string]any)
			return m["variant_id"] == variant.String() && c["count"] == float64(46)
		}},
		{flow.TypeStepApproved, func(m map[string]any) bool { return m["version"] == float64(1) }},
	}
	for _, w := range want {
		fr := s.next()
		var m map[string]any
		if err := json.Unmarshal([]byte(fr.data), &m); err != nil {
			t.Fatalf("%s: data %q: %v", fr.event, fr.data, err)
		}
		if fr.event != w.event || !w.check(m) {
			t.Errorf("frame = %s %s, want a %s event", fr.event, fr.data, w.event)
		}
	}
}

func TestEventsUnknownCampaignIs404BeforeStreaming(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{get: func(context.Context, uuid.UUID) (*flow.CampaignView, error) { return nil, flow.ErrNotFound }}
	srv := serve(t, f)
	resp, err := http.Get(srv.URL + "/campaigns/" + campID.String() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 || resp.Header.Get("Content-Type") != api.ProblemContentType {
		t.Errorf("status %d, content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var p api.Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil || p.Code != api.CodeNotFound {
		t.Errorf("problem = %+v (%v)", p, err)
	}
	waitSubscribers(t, f.Hub(), campID, 0)
}

func TestEventsSubscribeBeforeTheSnapshot(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	// An event published while the snapshot is being read must still reach the client.
	f.get = func(_ context.Context, id uuid.UUID) (*flow.CampaignView, error) {
		f.Hub().Publish(flow.StepOpened{Meta: meta(), StepID: uuid.New(), Kind: flow.KindAudience, Version: 1})
		return viewOf(id), nil
	}
	s := openStream(t, serve(t, f), campID)
	if fr := s.next(); fr.event != api.EventSnapshot {
		t.Fatalf("first = %+v", fr)
	}
	if fr := s.next(); fr.event != flow.TypeStepOpened {
		t.Errorf("second = %+v: the event published during the snapshot was lost", fr)
	}
}

func TestEventsPing(t *testing.T) {
	t.Parallel()
	k := newTicker()
	f := &fakeFlow{}
	s := openStream(t, serve(t, f, k.config, func(c *api.Config) { c.PingEvery = 15 * time.Second }), campID)
	s.next() // snapshot
	k.c <- time.Now()
	fr := s.next()
	if fr.event != api.EventPing {
		t.Fatalf("frame = %+v, want ping", fr)
	}
	var p api.PingEvent
	if err := json.Unmarshal([]byte(fr.data), &p); err != nil || p.At.IsZero() {
		t.Errorf("ping data = %s (%v)", fr.data, err)
	}
	// Events and pings interleave in order.
	f.Hub().Publish(flow.VariantApproved{Meta: meta(), StepID: uuid.New(), VariantID: uuid.New()})
	if fr := s.next(); fr.event != flow.TypeVariantApproved {
		t.Errorf("frame = %+v", fr)
	}
	k.c <- time.Now()
	if fr := s.next(); fr.event != api.EventPing {
		t.Errorf("frame = %+v", fr)
	}
}

func TestEventsDefaultPingIntervalIs15Seconds(t *testing.T) {
	t.Parallel()
	if api.DefaultPingEvery != 15*time.Second {
		t.Errorf("DefaultPingEvery = %s", api.DefaultPingEvery)
	}
	got := make(chan time.Duration, 1)
	k := newTicker()
	f := &fakeFlow{}
	srv := serve(t, f, func(c *api.Config) {
		c.Ticker = func(d time.Duration) (<-chan time.Time, func()) {
			got <- d
			return k.c, func() {}
		}
	})
	openStream(t, srv, campID).next()
	if d := <-got; d != 15*time.Second {
		t.Errorf("ticker interval = %s", d)
	}
}

func TestEventsResnapshotAfterDroppedEvents(t *testing.T) {
	t.Parallel()
	var snapshots int
	f := &fakeFlow{hub: flow.NewHub(1)}
	f.get = func(_ context.Context, id uuid.UUID) (*flow.CampaignView, error) {
		snapshots++
		v := viewOf(id)
		v.Brief = "snapshot " + string(rune('0'+snapshots))
		if snapshots == 1 {
			// The buffer of the subscription holds one event: the other two are dropped.
			for range 3 {
				f.Hub().Publish(flow.StepOpened{Meta: meta(), StepID: uuid.New(), Kind: flow.KindAudience, Version: 1})
			}
		}
		return v, nil
	}
	s := openStream(t, serve(t, f), campID)

	briefOf := func(fr frame) string {
		var v flow.CampaignView
		if err := json.Unmarshal([]byte(fr.data), &v); err != nil {
			t.Fatal(err)
		}
		return v.Brief
	}
	first := s.next()
	if first.event != api.EventSnapshot || briefOf(first) != "snapshot 1" {
		t.Fatalf("first = %+v", first)
	}
	second := s.next()
	if second.event != api.EventSnapshot || briefOf(second) != "snapshot 2" {
		t.Fatalf("second = %+v: want a fresh snapshot after the drop", second)
	}
	if third := s.next(); third.event != flow.TypeStepOpened {
		t.Errorf("third = %+v: the buffered event follows the snapshot", third)
	}
	// No further drops, no further snapshots: the next event comes through alone.
	f.Hub().Publish(flow.StepApproved{Meta: meta(), StepID: uuid.New(), Kind: flow.KindAudience, Version: 1})
	if fr := s.next(); fr.event != flow.TypeStepApproved {
		t.Errorf("frame = %+v, want step_approved without a snapshot", fr)
	}
}

func TestEventsClientDisconnectUnsubscribes(t *testing.T) {
	t.Parallel()
	k := newTicker()
	f := &fakeFlow{}
	srv := serve(t, f, k.config)
	s := openStream(t, srv, campID)
	s.next()
	waitSubscribers(t, f.Hub(), campID, 1)
	s.cancel()
	waitSubscribers(t, f.Hub(), campID, 0)
	select {
	case <-k.stopped:
	case <-time.After(5 * time.Second):
		t.Error("the ping ticker was not stopped")
	}
}

func TestEventsEndOnShutdown(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	ctx, cancel := context.WithCancel(t.Context())
	srv := serve(t, f, func(c *api.Config) { c.Shutdown = ctx })
	s := openStream(t, srv, campID)
	s.next()
	waitSubscribers(t, f.Hub(), campID, 1)
	cancel()
	s.ended()
	waitSubscribers(t, f.Hub(), campID, 0)
}

func TestEventsManyClientsEachGetTheirOwnStream(t *testing.T) {
	t.Parallel()
	f := &fakeFlow{}
	srv := serve(t, f)
	a, b := openStream(t, srv, campID), openStream(t, srv, campID)
	a.next()
	b.next()
	waitSubscribers(t, f.Hub(), campID, 2)
	f.Hub().Publish(flow.StepOpened{Meta: meta(), StepID: uuid.New(), Kind: flow.KindContent, Version: 1})
	for _, s := range []*stream{a, b} {
		if fr := s.next(); fr.event != flow.TypeStepOpened {
			t.Errorf("frame = %+v", fr)
		}
	}
	a.cancel()
	waitSubscribers(t, f.Hub(), campID, 1)
}
