//go:build integration

package sender_test

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/notification/api"
	"github.com/furknozg/coldbru/internal/notification/progress"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

const (
	startedSubject   = "evt.notification.started.v1"
	completedSubject = "evt.notification.completed.v1"
)

// eventCount returns how many messages of the subject the EVENTS stream holds.
func (e *env) eventCount(subject string) int {
	e.t.Helper()
	info, err := e.streams["EVENTS"].Info(e.t.Context(), jetstream.WithSubjectFilter(subject))
	if err != nil {
		e.t.Fatalf("EVENTS info: %v", err)
	}
	return int(info.State.Subjects[subject])
}

// lastEvent returns the newest envelope of the subject.
func (e *env) lastEvent(subject string) events.Envelope {
	e.t.Helper()
	msg, err := e.streams["EVENTS"].GetLastMsgForSubject(e.t.Context(), subject)
	if err != nil {
		e.t.Fatalf("last message of %s: %v", subject, err)
	}
	var env events.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		e.t.Fatalf("decode envelope: %v", err)
	}
	return env
}

func (e *env) campaignStatus(id string) string {
	e.t.Helper()
	return e.scalar(`SELECT status FROM send_campaign WHERE id = $1::uuid`, id)
}

// piiRows is how many deliveries of the campaign still hold a first name or a contact.
func (e *env) piiRows(id string) int {
	e.t.Helper()
	return e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id)
}

// settleTime is how long the tests wait for the checker: the production interval of 2 s plus slack.
const settleTime = 4 * time.Second

func TestCompletionEndToEnd(t *testing.T) {
	e := newEnv(t, withChecker())
	id := newCampaignID()
	e.campaignWith(id, smsCustomers[:3], emailCustomers[:2], 2)

	eventually(t, 20*time.Second, "the started event", func() bool { return e.eventCount(startedSubject) == 1 })
	started := e.lastEvent(startedSubject)
	var sd progress.StartedData
	if err := json.Unmarshal(started.Data, &sd); err != nil {
		t.Fatal(err)
	}
	if started.Producer != "notification" || started.CorrelationID != id || started.Version != 1 || started.Type != "notification.started" ||
		sd.CampaignID != id || sd.Language != "tr" || sd.Timezone != istanbul || sd.StartedAt.IsZero() || sd.ScheduleID == "" {
		t.Errorf("started envelope = %+v, data = %+v", started, sd)
	}
	if got := e.scalar(`SELECT id::text FROM locale_schedule WHERE campaign_id = $1::uuid`, id); got != sd.ScheduleID {
		t.Errorf("started schedule_id = %s, want %s", sd.ScheduleID, got)
	}

	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 5})
	sentAt := time.Now()
	eventually(t, settleTime, "the campaign to complete", func() bool { return e.campaignStatus(id) == "completed" })
	t.Logf("completed %s after the last delivery was sent", time.Since(sentAt).Round(10*time.Millisecond))

	if got := e.scalar(`SELECT completed_at::text FROM send_campaign WHERE id = $1::uuid`, id); got == "<null>" {
		t.Error("completed_at is not set")
	}
	if n := e.piiRows(id); n != 0 {
		t.Errorf("%d deliveries still hold personal data after completion", n)
	}
	if got := e.statusCounts(id); got["sent"] != 5 || len(got) != 1 {
		t.Errorf("statuses after completion = %v", got)
	}

	eventually(t, 10*time.Second, "the completed event", func() bool { return e.eventCount(completedSubject) == 1 })
	env := e.lastEvent(completedSubject)
	var cd progress.CompletedData
	if err := json.Unmarshal(env.Data, &cd); err != nil {
		t.Fatal(err)
	}
	wantCounts := []progress.Count{{Channel: "email", Status: "sent", Count: 2}, {Channel: "sms", Status: "sent", Count: 3}}
	if env.Producer != "notification" || env.CorrelationID != id || env.Type != "notification.completed" || env.Version != 1 ||
		cd.CampaignID != id || cd.CompletedAt.IsZero() || cd.SMSSegments != 6 || cd.EstimatedCostUSD != "0.2420" ||
		len(cd.Counts) != 2 || cd.Counts[0] != wantCounts[0] || cd.Counts[1] != wantCounts[1] {
		t.Errorf("completed envelope = %+v, data = %+v", env, cd)
	}

	// Later checks leave the completed campaign alone: still exactly one event.
	never(t, 2500*time.Millisecond, "a second completed event was written", func() bool { return e.eventCount(completedSubject) != 1 })
	if got := e.campaignStatus(id); got != "completed" {
		t.Errorf("status = %s, want completed", got)
	}
}

func TestCompletionCountsEveryFinalStatus(t *testing.T) {
	e := newEnv(t, withChecker())
	// One sms recipient is rejected for good (a 4xx), the others are sent.
	e.prov.setRespond(func(r provRequest) provResponse {
		if r.Body["to"] == smsCustomers[0].Contact {
			return provResponse{Status: http.StatusBadRequest, Body: `{"error":"invalid number"}`}
		}
		return provResponse{Status: http.StatusAccepted, Body: `{"message_id":"prov-` + r.Key + `"}`}
	})
	id := newCampaignID()
	e.campaignWith(id, smsCustomers[:3], emailCustomers[:1], 1)

	eventually(t, 20*time.Second, "the campaign to complete", func() bool { return e.campaignStatus(id) == "completed" })
	eventually(t, 10*time.Second, "the completed event", func() bool { return e.eventCount(completedSubject) == 1 })
	var cd progress.CompletedData
	if err := json.Unmarshal(e.lastEvent(completedSubject).Data, &cd); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, c := range cd.Counts {
		got[c.Channel+"/"+c.Status] = c.Count
	}
	want := map[string]int64{"email/sent": 1, "sms/sent": 2, "sms/failed": 1}
	if len(got) != len(want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("counts = %v, want %v", got, want)
		}
	}
	// Only the sent SMS count as segments and only the sent email is priced: 2 x 1 x 0.04 + 1 x 0.001.
	if cd.SMSSegments != 2 || cd.EstimatedCostUSD != "0.0810" {
		t.Errorf("sms_segments = %d, estimated_cost_usd = %s", cd.SMSSegments, cd.EstimatedCostUSD)
	}
	if n := e.piiRows(id); n != 0 {
		t.Errorf("%d deliveries still hold personal data", n)
	}
}

func TestCompletionWaitsForPendingDeliveries(t *testing.T) {
	e := newEnv(t, withChecker(), withDeferredSender())
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:3], nil)
	e.waitFired(id, 3)

	never(t, 2500*time.Millisecond, "the campaign completed with deliveries pending", func() bool {
		return e.campaignStatus(id) == "completed" || e.eventCount(completedSubject) != 0
	})
	if got := e.campaignStatus(id); got != "sending" {
		t.Fatalf("status = %s, want sending", got)
	}
	if n := e.piiRows(id); n != 3 {
		t.Errorf("deliveries holding personal data = %d, want 3", n)
	}

	e.startSender()
	eventually(t, 20*time.Second, "the campaign to complete once the deliveries are sent", func() bool { return e.campaignStatus(id) == "completed" })
	if n := e.piiRows(id); n != 0 {
		t.Errorf("%d deliveries still hold personal data", n)
	}
}

func TestCompletionWaitsForUnfiredSchedules(t *testing.T) {
	e := newEnv(t, withChecker())
	id := newCampaignID()
	smsContent, _ := json.Marshal(map[string]any{"text": smsTemplate, "segments": 1})
	emailContent, _ := json.Marshal(emailBlocks)
	later := time.Now().Add(time.Hour).UTC()
	// Turkish goes now; the English slot waits an hour and has nobody to send to.
	e.publishEvent(flow.EventTypeConfirmed, id, flow.ConfirmedData{
		CampaignID: id,
		Schedules: []flow.ConfirmedSchedule{
			{Language: "tr", Timezone: istanbul},
			{Language: "en", Timezone: "Europe/London", SendAt: &later},
		},
		Messages: []flow.ConfirmedMessage{
			{Language: "tr", Channel: "sms", Content: smsContent}, {Language: "tr", Channel: "email", Content: emailContent},
			{Language: "en", Channel: "sms", Content: smsContent}, {Language: "en", Channel: "email", Content: emailContent},
		},
		CTAURL:           "https://shop.example.com/cart?utm_campaign=" + id,
		RecipientBatches: 1,
	})
	var rs []flow.BatchRecipient
	for _, c := range smsCustomers[:2] {
		rs = append(rs, flow.BatchRecipient{CustomerID: c.ID, FirstName: c.FirstName, Language: "tr", Timezone: istanbul, Channel: "sms", Contact: c.Contact})
	}
	e.publishBatch(flow.RecipientBatch{CampaignID: id, BatchNo: 1, BatchCount: 1, Recipients: rs})

	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 2})
	if n := e.count(`SELECT count(*) FROM locale_schedule WHERE campaign_id = $1::uuid AND status = 'scheduled'`, id); n != 1 {
		t.Fatalf("scheduled schedules = %d, want 1", n)
	}
	never(t, 2500*time.Millisecond, "the campaign completed with a schedule still to fire", func() bool { return e.campaignStatus(id) != "sending" })
	if got := e.eventCount(completedSubject); got != 0 {
		t.Errorf("completed events = %d, want 0", got)
	}

	// The last slot fires (here: the row is moved by hand): the checker completes the campaign.
	if _, err := e.pool.Exec(t.Context(), `UPDATE locale_schedule SET status = 'fired', fired_at = now() WHERE campaign_id = $1::uuid AND status = 'scheduled'`, id); err != nil {
		t.Fatal(err)
	}
	eventually(t, settleTime, "the campaign to complete", func() bool { return e.campaignStatus(id) == "completed" })
	eventually(t, 10*time.Second, "the completed event", func() bool { return e.eventCount(completedSubject) == 1 })
}

func TestCancelledCampaignNeverCompletes(t *testing.T) {
	e := newEnv(t, withChecker(), withDeferredSender())
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:3], emailCustomers[:2])
	e.waitFired(id, 5)

	e.publishEvent(flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "test"})
	eventually(t, 10*time.Second, "the campaign to be cancelled", func() bool { return e.campaignStatus(id) == "cancelled" })
	e.startSender() // the queued messages of a cancelled campaign are dropped
	e.sendIdle()

	never(t, 2500*time.Millisecond, "a cancelled campaign was completed", func() bool {
		return e.campaignStatus(id) != "cancelled" || e.eventCount(completedSubject) != 0
	})
	if got := e.statusCounts(id); got["cancelled"] != 5 || len(got) != 1 {
		t.Errorf("statuses = %v, want 5 cancelled", got)
	}
	if n := e.piiRows(id); n != 0 {
		t.Errorf("%d deliveries still hold personal data", n)
	}
}

// ---- live REST and SSE ----

// serveAPI serves the live delivery endpoints over the test database.
func (e *env) serveAPI(pollEvery time.Duration) *httptest.Server {
	e.t.Helper()
	r := chi.NewRouter()
	api.New(api.Config{Source: api.NewPoolSource(e.pool), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), PollEvery: pollEvery}).Mount(r)
	srv := httptest.NewServer(r)
	e.t.Cleanup(srv.Close)
	return srv
}

type sseFrame struct{ event, data string }

// readFrames reads an SSE response until it ends or timeout passes.
func readFrames(t *testing.T, resp *http.Response, timeout time.Duration) (frames []sseFrame, ended bool) {
	t.Helper()
	ch := make(chan sseFrame)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(resp.Body)
		var cur sseFrame
		for sc.Scan() {
			switch line := sc.Text(); {
			case line == "":
				if cur.event != "" {
					ch <- cur
				}
				cur = sseFrame{}
			case strings.HasPrefix(line, "event: "):
				cur.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				cur.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	deadline := time.After(timeout)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return frames, true
			}
			frames = append(frames, f)
		case <-deadline:
			return frames, false
		}
	}
}

func TestDeliveryStreamFollowsACampaignToItsEnd(t *testing.T) {
	e := newEnv(t, withChecker(), withDeferredSender())
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:3], emailCustomers[:2])
	e.waitFired(id, 5)
	srv := e.serveAPI(100 * time.Millisecond)

	// The plain endpoint first.
	resp, err := http.Get(srv.URL + "/campaigns/" + id + "/deliveries")
	if err != nil {
		t.Fatal(err)
	}
	var sum api.DeliverySummary
	if err := json.NewDecoder(resp.Body).Decode(&sum); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if sum.Status != "sending" || sum.Total != 5 || len(sum.Counts) != 2 || sum.Counts[0] != (api.ChannelCount{Channel: "email", Status: "pending", Count: 2}) ||
		sum.Counts[1] != (api.ChannelCount{Channel: "sms", Status: "pending", Count: 3}) {
		t.Errorf("summary = %+v", sum)
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/campaigns/"+id+"/deliveries/events", nil)
	stream, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if ct := stream.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	// Let the first snapshot go out before anything changes, then start sending.
	time.AfterFunc(300*time.Millisecond, e.startSender)
	frames, ended := readFrames(t, stream, 30*time.Second)
	if !ended {
		t.Fatalf("the stream did not end; frames: %v", frames)
	}
	if len(frames) < 3 {
		t.Fatalf("frames = %v, want a snapshot, at least one change and done", frames)
	}

	var snaps []api.DeliverySummary
	for i, f := range frames[:len(frames)-1] {
		if f.event == api.EventPing {
			continue
		}
		if f.event != api.EventSnapshot {
			t.Fatalf("frame %d = %s, want snapshot", i, f.event)
		}
		var s api.DeliverySummary
		if err := json.Unmarshal([]byte(f.data), &s); err != nil {
			t.Fatal(err)
		}
		snaps = append(snaps, s)
	}
	if first := snaps[0]; first.Status != "sending" || first.Total != 5 || first.Counts[0].Status != "pending" {
		t.Errorf("first snapshot = %+v", first)
	}
	for i := 1; i < len(snaps); i++ {
		if a, b := snaps[i-1], snaps[i]; a.Status == b.Status && len(a.Counts) == len(b.Counts) && sameCounts(a, b) {
			t.Errorf("snapshots %d and %d show the same state: %+v", i-1, i, b)
		}
	}
	last := snaps[len(snaps)-1]
	if last.Status != "completed" || last.Total != 5 || len(last.Counts) != 2 ||
		last.Counts[0] != (api.ChannelCount{Channel: "email", Status: "sent", Count: 2}) || last.Counts[1] != (api.ChannelCount{Channel: "sms", Status: "sent", Count: 3}) {
		t.Errorf("last snapshot = %+v", last)
	}
	if done := frames[len(frames)-1]; done.event != api.EventDone || done.data != `{"status":"completed"}` {
		t.Errorf("last frame = %+v, want done completed", done)
	}

	// A finished campaign: snapshot and done at once.
	resp2, err := http.Get(srv.URL + "/campaigns/" + id + "/deliveries/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	again, ended := readFrames(t, resp2, 5*time.Second)
	if !ended || len(again) != 2 || again[0].event != api.EventSnapshot || again[1].event != api.EventDone {
		t.Errorf("stream of a finished campaign = %v (ended %v)", again, ended)
	}
}

func sameCounts(a, b api.DeliverySummary) bool {
	for i := range a.Counts {
		if a.Counts[i] != b.Counts[i] {
			return false
		}
	}
	return true
}

func TestDeliveryEndpointsAnswer404ForAnUnknownCampaign(t *testing.T) {
	e := newEnv(t)
	srv := e.serveAPI(time.Second)
	for _, path := range []string{"/deliveries", "/deliveries/events"} {
		resp, err := http.Get(srv.URL + "/campaigns/" + uuid.NewString() + path)
		if err != nil {
			t.Fatal(err)
		}
		var p api.Problem
		err = json.NewDecoder(resp.Body).Decode(&p)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 404 || resp.Header.Get("Content-Type") != api.ProblemContentType || p.Code != api.CodeNotFound {
			t.Errorf("%s: status %d, content type %q, problem %+v (%v)", path, resp.StatusCode, resp.Header.Get("Content-Type"), p, err)
		}
	}
}
