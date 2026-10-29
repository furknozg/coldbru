//go:build integration

package sender_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	custconsent "github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph/model"
	"github.com/furknozg/coldbru/internal/notification/sender"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// noErrors fails the test when a record at error level was logged.
func (e *env) noErrors() {
	e.t.Helper()
	if n := e.logs.atLevel(slog.LevelError); n != 0 {
		e.t.Errorf("%d records at error level, want none", n)
	}
}

func TestSend_SampleCampaignEveryDeliverySentOnce(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	sms, email := smsCustomers[:30], emailCustomers[:20]
	e.campaignOf(id, sms, email)

	e.waitStatuses(id, 30*time.Second, map[string]int{"sent": 50})
	e.sendIdle()
	e.noErrors()

	// The provider saw exactly one request per delivery, with the delivery id as Idempotency-Key.
	byKey := e.prov.byKey()
	rows, err := e.pool.Query(t.Context(), `SELECT id::text, customer_id, channel, status, attempts, provider_message_id, sent_at, lease_until, first_name, contact
		FROM delivery WHERE campaign_id = $1::uuid`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var did, customerID, channel, status string
		var attempts int
		var providerID, firstName, contact *string
		var sentAt, lease *time.Time
		if err := rows.Scan(&did, &customerID, &channel, &status, &attempts, &providerID, &sentAt, &lease, &firstName, &contact); err != nil {
			t.Fatal(err)
		}
		seen++
		reqs := byKey[did]
		if len(reqs) != 1 {
			t.Errorf("delivery %s: provider got %d requests, want 1", did, len(reqs))
			continue
		}
		if attempts != 1 || providerID == nil || *providerID != "prov-"+did || sentAt == nil || lease != nil {
			t.Errorf("delivery %s: attempts = %d, provider id = %v, sent_at = %v, lease = %v", did, attempts, providerID, sentAt, lease)
		}
		// Personal data stay until the campaign ends (D-077, T-030).
		if firstName == nil || contact == nil {
			t.Errorf("delivery %s lost its personal data before the campaign ended", did)
		}
		r := reqs[0]
		wantPath := map[string]string{"sms": "/v1/sms", "email": "/v1/email"}[channel]
		if r.Path != wantPath || r.Body["to"] != *contact {
			t.Errorf("delivery %s: path = %s, to = %s; want %s and %s", did, r.Path, r.Body["to"], wantPath, *contact)
		}
		switch channel {
		case "sms":
			if want := "Merhaba " + *firstName + ", sepetindeki ürünler seni bekliyor. %30 indirim için acele et!"; r.Body["text"] != want {
				t.Errorf("sms text = %q, want %q", r.Body["text"], want)
			}
		case "email":
			if want := "Sepetin seni bekliyor, " + *firstName; r.Body["subject"] != want {
				t.Errorf("email subject = %q, want %q", r.Body["subject"], want)
			}
			for _, part := range []string{"Merhaba " + *firstName, "utm_campaign=" + id, "Bu e-postayı", "Alışverişe dön"} {
				if !strings.Contains(r.Body["html"], part) {
					t.Errorf("email html lacks %q", part)
				}
			}
			if !strings.Contains(r.Body["text"], "utm_campaign="+id) {
				t.Errorf("email text lacks the campaign link")
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen != 50 || len(byKey) != 50 || e.prov.count() != 50 {
		t.Errorf("deliveries = %d, distinct keys = %d, requests = %d; want 50 of each", seen, len(byKey), e.prov.count())
	}

	// Metrics.
	if got := e.metric("deliveries_total", map[string]string{"channel": "sms", "status": "sent"}); got != 30 {
		t.Errorf("deliveries_total{sms,sent} = %v, want 30", got)
	}
	if got := e.metric("deliveries_total", map[string]string{"channel": "email", "status": "sent"}); got != 20 {
		t.Errorf("deliveries_total{email,sent} = %v, want 20", got)
	}
	if got := e.metric("provider_requests_total", map[string]string{"channel": "sms", "code_class": "2xx"}); got != 30 {
		t.Errorf("provider_requests_total{sms,2xx} = %v, want 30", got)
	}
	if got := e.metric("send_latency_seconds", map[string]string{"channel": "email"}); got != 20 {
		t.Errorf("send_latency_seconds{email} observations = %v, want 20", got)
	}
	// The campaign is not completed here (T-030).
	if got := e.scalar(`SELECT status FROM send_campaign WHERE id = $1::uuid`, id); got != "sending" {
		t.Errorf("campaign status = %q, want sending (completion is T-030)", got)
	}
}

func TestSend_TransientFailuresAreRetriedWithTheSameKey(t *testing.T) {
	e := newEnv(t)
	e.prov.setRespond(func(r provRequest) provResponse {
		if r.Attempt <= 2 {
			return provResponse{Status: http.StatusServiceUnavailable, Body: "overloaded"}
		}
		return provResponse{Status: http.StatusAccepted, Body: `{"message_id":"after-retries"}`}
	})
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:1], nil)
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 1})
	e.sendIdle()

	d := e.delivery(id, smsCustomers[0].ID, "sms")
	if d.Attempts != 3 || d.ProviderMessageID == nil || *d.ProviderMessageID != "after-retries" || d.LastError != nil || d.LeaseUntil != nil {
		t.Errorf("delivery = %+v, want sent after 3 attempts with the last error cleared", d)
	}
	reqs := e.prov.requests()
	if len(reqs) != 3 {
		t.Fatalf("provider got %d requests, want 3", len(reqs))
	}
	for _, r := range reqs {
		if r.Key != d.ID {
			t.Errorf("request key = %s, want the delivery id %s", r.Key, d.ID)
		}
	}
	if got := e.metric("provider_requests_total", map[string]string{"channel": "sms", "code_class": "5xx"}); got != 2 {
		t.Errorf("provider_requests_total{sms,5xx} = %v, want 2", got)
	}
	if got := e.metric("deliveries_total", map[string]string{"channel": "sms", "status": "sent"}); got != 1 {
		t.Errorf("deliveries_total{sms,sent} = %v, want 1 (retries are not final)", got)
	}
}

func TestSend_PermanentFailureFailsAtOnce(t *testing.T) {
	e := newEnv(t)
	e.prov.setRespond(func(provRequest) provResponse {
		return provResponse{Status: http.StatusBadRequest, Body: `{"error":"invalid number"}`}
	})
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:1], emailCustomers[:1])
	e.waitStatuses(id, 20*time.Second, map[string]int{"failed": 2})
	e.sendIdle()

	for _, d := range []deliveryRow{e.delivery(id, smsCustomers[0].ID, "sms"), e.delivery(id, emailCustomers[0].ID, "email")} {
		if d.Attempts != 1 || d.LastError == nil || !strings.Contains(*d.LastError, "400") || !strings.Contains(*d.LastError, "invalid number") || d.LeaseUntil != nil || d.SentAt != nil {
			t.Errorf("delivery = %+v (error %v), want failed after one attempt naming the 400", d, d.LastError)
		}
	}
	if e.prov.count() != 2 {
		t.Errorf("provider got %d requests, want 2 (no retry of a 400)", e.prov.count())
	}
	if got := e.metric("deliveries_total", map[string]string{"channel": "sms", "status": "failed"}); got != 1 {
		t.Errorf("deliveries_total{sms,failed} = %v, want 1", got)
	}
	if got := e.metric("provider_requests_total", map[string]string{"channel": "sms", "code_class": "4xx"}); got != 1 {
		t.Errorf("provider_requests_total{sms,4xx} = %v, want 1", got)
	}
}

func TestSend_AlwaysTransientFailsAfterFiveDeliveries(t *testing.T) {
	e := newEnv(t)
	e.prov.setRespond(func(provRequest) provResponse {
		return provResponse{Status: http.StatusServiceUnavailable, Body: "down"}
	})
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:2], nil)
	e.waitStatuses(id, 30*time.Second, map[string]int{"failed": 2})
	e.sendIdle()

	for _, c := range smsCustomers[:2] {
		d := e.delivery(id, c.ID, "sms")
		if d.Attempts != 5 || d.LastError == nil || !strings.Contains(*d.LastError, "gave up after 5 deliveries") || d.LeaseUntil != nil {
			t.Errorf("delivery = %+v (error %v), want failed after 5 attempts", d, d.LastError)
		}
		if reqs := e.prov.byKey()[d.ID]; len(reqs) != 5 {
			t.Errorf("provider got %d requests for %s, want 5", len(reqs), d.ID)
		}
	}
	if got := e.metric("deliveries_total", map[string]string{"channel": "sms", "status": "failed"}); got != 2 {
		t.Errorf("deliveries_total{sms,failed} = %v, want 2", got)
	}
}

func TestSend_RateLimitedPausesTheChannel(t *testing.T) {
	// One sender goroutine, so that the 429 is seen before the next request is made.
	e := newEnv(t, withWorkers(1), withBackoff(fastBackoff))
	var first time.Time
	e.prov.setRespond(func(r provRequest) provResponse {
		if first.IsZero() {
			first = r.At
			return provResponse{Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"2"}}, Body: "slow down"}
		}
		return provResponse{Status: http.StatusAccepted, Body: fmt.Sprintf(`{"message_id":"prov-%s"}`, r.Key)}
	})
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:3], nil)
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 3})
	e.sendIdle()

	reqs := e.prov.requests()
	if len(reqs) != 4 {
		t.Fatalf("provider got %d requests, want 4 (the 429 and 3 sends)", len(reqs))
	}
	for _, r := range reqs[1:] {
		if gap := r.At.Sub(first); gap < 1900*time.Millisecond {
			t.Errorf("request %s came %s after the 429, want it held for the 2s Retry-After", r.Key, gap)
		}
	}
	retried := e.delivery(id, smsCustomers[0].ID, "sms")
	if retried.Attempts != 2 {
		t.Errorf("the delivery that got the 429 has attempts = %d, want 2", retried.Attempts)
	}
	if got := e.metric("provider_429_total", map[string]string{"channel": "sms"}); got != 1 {
		t.Errorf("provider_429_total{sms} = %v, want 1", got)
	}
}

func TestSend_ConsentRevokedAfterFireIsSkipped(t *testing.T) {
	// D-011 target test: the person opted out between the confirmation and the send.
	e := newEnv(t, withDeferredSender())
	id := newCampaignID()
	sms, email := smsCustomers[100:110], emailCustomers[:5]
	e.campaignOf(id, sms, email)
	e.waitFired(id, 15)
	if got := e.sendQueued(); got != 15 {
		t.Fatalf("SEND holds %d messages, want 15", got)
	}

	revokedSMS, revokedEmail := sms[3], email[2]
	svc := custconsent.NewService(customerEnv.Pool)
	for _, r := range []struct {
		c       cust
		channel string
	}{{revokedSMS, "sms"}, {revokedEmail, "email"}} {
		if _, err := svc.Update(t.Context(), model.UpdateConsentInput{CustomerID: r.c.ID, Channel: r.channel, OptedIn: false, Source: "admin_tool"}); err != nil {
			t.Fatalf("revoke consent: %v", err)
		}
		t.Cleanup(func() {
			_, err := svc.Update(context.WithoutCancel(t.Context()), model.UpdateConsentInput{CustomerID: r.c.ID, Channel: r.channel, OptedIn: true, Source: "admin_tool"})
			if err != nil {
				t.Errorf("restore consent: %v", err)
			}
		})
	}

	e.startSender()
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 13, "skipped_no_consent": 2})
	e.sendIdle()
	e.noErrors()

	for _, d := range []deliveryRow{e.delivery(id, revokedSMS.ID, "sms"), e.delivery(id, revokedEmail.ID, "email")} {
		if d.Status != "skipped_no_consent" || d.Attempts != 0 || d.SentAt != nil {
			t.Errorf("delivery = %+v, want skipped_no_consent without an attempt", d)
		}
		if n := len(e.prov.byKey()[d.ID]); n != 0 {
			t.Errorf("provider got %d requests for the skipped delivery %s, want 0", n, d.ID)
		}
		// Personal data stay until the campaign ends (D-077); a skip does not clear them.
		if d.FirstName == nil || d.Contact == nil {
			t.Errorf("skipped delivery %s lost its personal data, want it kept until the campaign ends", d.ID)
		}
	}
	// Not the other channel either: nothing was sent to the revoked person at all.
	if e.prov.count() != 13 {
		t.Errorf("provider got %d requests, want 13", e.prov.count())
	}
	if got := e.metric("deliveries_total", map[string]string{"channel": "sms", "status": "skipped_no_consent"}); got != 1 {
		t.Errorf("deliveries_total{sms,skipped_no_consent} = %v, want 1", got)
	}
}

func TestSend_CustomerDownSendsNothingThenRecovers(t *testing.T) {
	// Five consecutive failures open the breaker for 3 s. The backoff is slow enough that the
	// deliveries outlast the outage (deliveries at 0, 0.5, 1.5, 3.5 and 7.5 s).
	e := newEnv(t, withDeferredSender(),
		withBackoff(consumer.Backoff{Base: 500 * time.Millisecond, Factor: 2, Max: 4 * time.Second}),
		withBreaker(5, 3*time.Second))
	id := newCampaignID()
	sms := smsCustomers[200:204]
	e.campaignOf(id, sms, nil)
	e.waitFired(id, 4)

	e.customer.down.Store(true)
	for range 5 {
		if _, err := e.consent.Eligible(t.Context(), "sms", []string{sms[0].ID}); err == nil {
			t.Fatal("consent answered while Customer is down")
		}
	}
	if e.consent.State() != "open" {
		t.Fatalf("breaker = %s, want open", e.consent.State())
	}
	callsWhenOpened := e.customer.calls.Load()

	e.startSender()
	time.Sleep(700 * time.Millisecond) // the first rounds, inside the open period
	if e.prov.count() != 0 {
		t.Fatalf("provider got %d requests while consent could not be checked, want 0", e.prov.count())
	}
	if got := e.statusCounts(id); got["pending"] != 4 || len(got) != 1 {
		t.Fatalf("deliveries = %v, want 4 pending", got)
	}
	if got := e.customer.calls.Load(); got != callsWhenOpened {
		t.Errorf("Customer got %d requests while the breaker was open, want none", got-callsWhenOpened)
	}

	e.customer.down.Store(false)
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 4})
	e.sendIdle()
	// Consent failures are not provider failures: no provider request failed, no delivery was blamed.
	if got := e.metric("provider_requests_total", map[string]string{"channel": "sms", "code_class": "5xx"}); got != 0 {
		t.Errorf("provider 5xx = %v, want 0", got)
	}
	for _, c := range sms {
		if d := e.delivery(id, c.ID, "sms"); d.Attempts != 1 || d.LastError != nil {
			t.Errorf("delivery = %+v, want exactly one provider attempt and no error", d)
		}
	}
	if e.consent.State() != "closed" {
		t.Errorf("breaker = %s after recovery, want closed", e.consent.State())
	}
}

func TestSend_CustomerDownLongerThanFiveDeliveriesRequeuesAndSendsOnceBack(t *testing.T) {
	e := newEnv(t) // fast backoff: five deliveries of a message take well under a second
	e.customer.down.Store(true)
	id := newCampaignID()
	sms := smsCustomers[300:302]
	e.campaignOf(id, sms, nil)

	// Each message goes through all five deliveries and is then requeued as a fresh message, more
	// than once: far beyond what MaxDeliver 5 alone would allow.
	eventually(t, 30*time.Second, "both messages to be requeued twice", func() bool {
		return len(e.logs.find("consent still unavailable on the last delivery, message requeued")) >= 4
	})
	if e.prov.count() != 0 {
		t.Errorf("provider got %d requests, want 0: consent could not be checked", e.prov.count())
	}
	if got := e.statusCounts(id); got["pending"] != 2 || len(got) != 1 {
		t.Errorf("deliveries = %v, want 2 pending (not failed)", got)
	}
	if got := e.metric("deliveries_total", map[string]string{"channel": "sms", "status": "failed"}); got != 0 {
		t.Errorf("deliveries_total{sms,failed} = %v, want 0", got)
	}
	// Between the ack of an old message and the relay's publish of its replacement the count dips,
	// so look for the moment it is exactly the two requeued messages.
	eventually(t, 10*time.Second, "SEND to hold the 2 requeued messages", func() bool { return e.sendQueued() == 2 })

	e.customer.down.Store(false)
	e.waitStatuses(id, 30*time.Second, map[string]int{"sent": 2})
	e.sendIdle()
	for _, c := range sms {
		if d := e.delivery(id, c.ID, "sms"); d.Attempts != 1 || d.LastError != nil {
			t.Errorf("delivery = %+v, want exactly one provider attempt and no error", d)
		}
		if n := len(e.prov.byKey()[e.delivery(id, c.ID, "sms").ID]); n != 1 {
			t.Errorf("provider got %d requests for %s, want 1", n, c.ID)
		}
	}
	if n := e.sendQueued(); n != 0 {
		t.Errorf("SEND holds %d messages after the sends, want 0", n)
	}
}

func TestSend_DuplicateMessageIsOneProviderCall(t *testing.T) {
	e := newEnv(t, withDeferredSender())
	id := newCampaignID()
	sms := smsCustomers[10:15]
	e.campaignOf(id, sms, nil)
	e.waitFired(id, 5)

	dup := e.delivery(id, sms[0].ID, "sms")
	for range 2 { // two more copies of the same message
		if _, err := e.js.Publish(t.Context(), "send.sms", []byte(fmt.Sprintf(`{"delivery_id":%q}`, dup.ID))); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.sendQueued(); got != 7 {
		t.Fatalf("SEND holds %d messages, want 5 and 2 duplicates", got)
	}
	e.startSender()
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 5})
	e.sendIdle()

	if n := len(e.prov.byKey()[dup.ID]); n != 1 {
		t.Errorf("provider got %d requests for the duplicated delivery, want 1", n)
	}
	if e.prov.count() != 5 {
		t.Errorf("provider got %d requests, want 5", e.prov.count())
	}

	// A copy that arrives after the delivery is final is dropped too.
	if _, err := e.js.Publish(t.Context(), "send.sms", []byte(fmt.Sprintf(`{"delivery_id":%q}`, dup.ID))); err != nil {
		t.Fatal(err)
	}
	e.sendIdle()
	if e.prov.count() != 5 {
		t.Errorf("provider got %d requests after a late duplicate, want 5", e.prov.count())
	}
	e.noErrors()
}

func TestSend_ExpiredLeaseIsReclaimedAfterACrash(t *testing.T) {
	e := newEnv(t, withDeferredSender())
	id := newCampaignID()
	sms := smsCustomers[20:23]
	e.campaignOf(id, sms, nil)
	e.waitFired(id, 3)

	// A worker claimed the first delivery and died: it is sending, its lease ran out a second ago.
	crashed := e.delivery(id, sms[0].ID, "sms")
	// A second one is leased to a worker that is alive: the lease runs for another hour.
	alive := e.delivery(id, sms[1].ID, "sms")
	if _, err := e.pool.Exec(t.Context(), `UPDATE delivery SET status = 'sending', attempts = 1, lease_until = now() - interval '1 second' WHERE id = $1::uuid`, crashed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(t.Context(), `UPDATE delivery SET status = 'sending', attempts = 1, lease_until = now() + interval '1 hour' WHERE id = $1::uuid`, alive.ID); err != nil {
		t.Fatal(err)
	}

	e.startSender()
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 2, "sending": 1})
	e.sendIdle()

	if d := e.delivery(id, sms[0].ID, "sms"); d.Status != "sent" || d.Attempts != 2 || d.LeaseUntil != nil {
		t.Errorf("reclaimed delivery = %+v, want sent on its second attempt", d)
	}
	if d := e.delivery(id, sms[1].ID, "sms"); d.Status != "sending" || d.Attempts != 1 || len(e.prov.byKey()[d.ID]) != 0 {
		t.Errorf("leased delivery = %+v, want untouched and not sent by this worker", d)
	}
	if e.prov.count() != 2 {
		t.Errorf("provider got %d requests, want 2", e.prov.count())
	}
}

func TestSend_CancelledCampaignSendsNoMoreAndClearsPersonalData(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	sms := smsCustomers[:200]
	e.campaignOf(id, sms, nil)

	eventually(t, 20*time.Second, "some deliveries to be sent", func() bool { return e.prov.count() >= 20 })
	e.publishEvent(flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "changed my mind"})
	eventually(t, 30*time.Second, "the campaign to be cancelled", func() bool {
		return e.scalar(`SELECT status FROM send_campaign WHERE id = $1::uuid`, id) == "cancelled"
	})
	e.sendIdle()

	sentToProvider := e.prov.count()
	never(t, 700*time.Millisecond, "the provider got a request after the cancellation", func() bool { return e.prov.count() != sentToProvider })
	got := e.statusCounts(id)
	if got["pending"] != 0 || got["sending"] != 0 || got["sent"] == 0 || got["cancelled"] == 0 || got["sent"]+got["cancelled"] != 200 {
		t.Errorf("deliveries = %v, want only sent and cancelled, both present", got)
	}
	if got["sent"] != sentToProvider {
		t.Errorf("sent = %d but the provider got %d requests", got["sent"], sentToProvider)
	}
	// The cancellation cleared the personal data of every delivery, sent or not.
	if n := e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id); n != 0 {
		t.Errorf("%d deliveries kept personal data after the cancellation", n)
	}
}

func TestSend_CancelledWhileSendingFinishesTheDeliveryAndClearsItsData(t *testing.T) {
	e := newEnv(t, withWorkers(1))
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	e.prov.setRespond(func(r provRequest) provResponse {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return provResponse{Status: http.StatusAccepted, Body: fmt.Sprintf(`{"message_id":"prov-%s"}`, r.Key)}
	})
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:1], nil)
	<-entered // the provider call is in flight: the delivery is sending

	// The campaign is cancelled while the delivery is out. (Through the intake the cancellation waits
	// for the delivery to finish; here the status is set directly to meet the worker's own rule.)
	if _, err := e.pool.Exec(t.Context(), `UPDATE send_campaign SET status = 'cancelled' WHERE id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	close(release)
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 1})
	e.sendIdle()

	d := e.delivery(id, smsCustomers[0].ID, "sms")
	if d.FirstName != nil || d.Contact != nil || d.ProviderMessageID == nil || d.SentAt == nil {
		t.Errorf("delivery = %+v, want sent with its personal data cleared", d)
	}
}

func TestSend_CancelWhileSendingSucceedsAtOnceAndTheWorkerClearsItsData(t *testing.T) {
	e := newEnv(t, withWorkers(1))
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	e.prov.setRespond(func(r provRequest) provResponse {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return provResponse{Status: http.StatusAccepted, Body: fmt.Sprintf(`{"message_id":"prov-%s"}`, r.Key)}
	})
	id := newCampaignID()
	sms := smsCustomers[:3]
	e.campaignOf(id, sms, nil)
	<-entered // one delivery is sending, inside the provider call; the others wait

	e.publishEvent(flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "stop"})
	// The cancellation does not wait for the delivery that is out: it cancels the pending ones, clears
	// their personal data and commits.
	eventually(t, 10*time.Second, "the campaign to be cancelled while a delivery is sending", func() bool {
		return e.scalar(`SELECT status FROM send_campaign WHERE id = $1::uuid`, id) == "cancelled"
	})
	if got := e.statusCounts(id); got["sending"] != 1 || got["cancelled"] != 2 || len(got) != 2 {
		t.Fatalf("deliveries = %v, want 1 sending and 2 cancelled", got)
	}
	if n := e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND status = 'cancelled' AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id); n != 0 {
		t.Errorf("%d cancelled deliveries kept personal data", n)
	}
	if n := e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND status = 'sending' AND first_name IS NOT NULL AND contact IS NOT NULL`, id); n != 1 {
		t.Errorf("the sending delivery must keep its data until it is settled")
	}

	close(release)
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 1, "cancelled": 2})
	e.sendIdle()
	if n := e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id); n != 0 {
		t.Errorf("%d deliveries kept personal data after the worker settled the last one", n)
	}
	if e.prov.count() != 1 {
		t.Errorf("provider got %d requests, want 1 (the delivery that was already out)", e.prov.count())
	}
}

func TestSend_CancelledWhileRetryingBecomesCancelled(t *testing.T) {
	e := newEnv(t, withWorkers(1))
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	e.prov.setRespond(func(provRequest) provResponse {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return provResponse{Status: http.StatusServiceUnavailable, Body: "down"}
	})
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:1], nil)
	<-entered
	if _, err := e.pool.Exec(t.Context(), `UPDATE send_campaign SET status = 'cancelled' WHERE id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	close(release)

	// The failed attempt would be retried, but the campaign is cancelled: the delivery ends cancelled.
	e.waitStatuses(id, 20*time.Second, map[string]int{"cancelled": 1})
	e.sendIdle()
	d := e.delivery(id, smsCustomers[0].ID, "sms")
	if d.FirstName != nil || d.Contact != nil || d.LeaseUntil != nil {
		t.Errorf("delivery = %+v, want cancelled with its personal data cleared", d)
	}
	if e.prov.count() != 1 {
		t.Errorf("provider got %d requests, want 1", e.prov.count())
	}
}

func TestSend_MalformedMessagesAreTerminated(t *testing.T) {
	e := newEnv(t)
	for _, payload := range []string{`not json`, `{"delivery_id":"nope"}`, fmt.Sprintf(`{"delivery_id":%q}`, newCampaignID())} {
		if _, err := e.js.Publish(t.Context(), "send.sms", []byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	e.sendIdle()
	if n := e.sendQueued(); n != 0 {
		t.Errorf("SEND holds %d messages, want 0: terminated messages leave the work queue", n)
	}
	if n := len(e.logs.find("send message terminated")); n != 3 {
		t.Errorf("%d messages terminated, want 3", n)
	}
	if e.prov.count() != 0 {
		t.Errorf("provider got %d requests, want 0", e.prov.count())
	}
}

func TestSend_RateLimitedToAHundredPerSecond(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	sms := smsCustomers[:300]
	e.campaignOf(id, sms, nil)

	e.waitStatuses(id, 30*time.Second, map[string]int{"sent": 300})
	e.sendIdle()

	reqs := e.prov.requests()
	if len(reqs) != 300 {
		t.Fatalf("provider got %d requests, want 300", len(reqs))
	}
	times := make([]time.Time, len(reqs))
	for i, r := range reqs {
		times[i] = r.At
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	span := times[len(times)-1].Sub(times[0])
	t.Logf("300 SMS requests took %s from the first to the last (%.0f/s)", span.Round(time.Millisecond), 300/span.Seconds())
	// A bucket of 10 then 100 a second: 290 tokens after the burst take 2.9 s.
	if span < 2700*time.Millisecond {
		t.Errorf("300 requests took %s, want at least 2.7s at 100/s with a burst of 10", span)
	}
	// No one-second window holds more than the burst plus a second's worth of tokens.
	worst, j := 0, 0
	for i := range times {
		for times[i].Sub(times[j]) >= time.Second {
			j++
		}
		worst = max(worst, i-j+1)
	}
	if worst > sender.RatePerSecond+sender.Burst+5 {
		t.Errorf("%d requests inside one second, want at most about %d", worst, sender.RatePerSecond+sender.Burst)
	}
}
