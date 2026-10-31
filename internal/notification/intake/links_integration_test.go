//go:build integration

package intake_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/notification/links"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// deliveryIDs lists the delivery ids of a campaign with their link codes.
func (e *env) linkCodes(t *testing.T, campaignID string) map[uuid.UUID]string {
	t.Helper()
	rows, err := e.pool.Query(t.Context(), `SELECT id, coalesce(link_code, '') FROM delivery WHERE campaign_id = $1::uuid`, campaignID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]string{}
	for rows.Next() {
		var id uuid.UUID
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			t.Fatal(err)
		}
		out[id] = code
	}
	return out
}

func TestIntake_EveryDeliveryGetsTheLinkOfItsBatchRequest(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	cdata := confirmedData(id, 2, immediate("tr", istanbul), immediate("en", london))
	e.publishEvent(t, flow.EventTypeConfirmed, id, cdata)
	e.publishBatch(t, batch(id, 1, 2,
		recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "tr", istanbul, "email"), recipient("CUS-3", "en", london, "sms")))
	e.publishBatch(t, batch(id, 2, 2,
		recipient("CUS-4", "tr", istanbul, "sms"), recipient("CUS-5", "tr", istanbul, "email")))
	eventually(t, 15*time.Second, "SEND messages", func() bool { return e.sendTotal(t) == 5 })
	e.idle(t)

	if got := e.links.Calls(); got != 2 {
		t.Errorf("%d link requests, want one per recipient batch (2)", got)
	}
	codes := e.linkCodes(t, id)
	if len(codes) != 5 {
		t.Fatalf("%d deliveries, want 5", len(codes))
	}
	seen := map[string]bool{}
	wantTarget := cdata.CTAURL // already carries utm_campaign=<id>; CTAURL keeps it
	for did, code := range codes {
		if code == "" || code != e.links.Code(did) || seen[code] {
			t.Errorf("delivery %s has link_code %q, the redirect service gave %q (duplicate: %v)", did, code, e.links.Code(did), seen[code])
		}
		seen[code] = true
		if did.Version() != 7 {
			t.Errorf("delivery id %s is UUIDv%d, want UUIDv7 (D-030)", did, did.Version())
		}
		req, ok := e.links.Request(did)
		if !ok {
			t.Fatalf("no link request for delivery %s", did)
		}
		var channel, language, tz string
		if err := e.pool.QueryRow(t.Context(), `SELECT d.channel, m.language, s.timezone FROM delivery d
			JOIN message m ON m.id = d.message_id JOIN locale_schedule s ON s.id = d.schedule_id WHERE d.id = $1`, did).Scan(&channel, &language, &tz); err != nil {
			t.Fatal(err)
		}
		if req.CampaignID.String() != id || req.Channel != channel || req.Language != language || req.Timezone != tz || req.TargetURL != wantTarget {
			t.Errorf("link request of %s = %+v; want campaign %s, %s/%s/%s, target %s", did, req, id, channel, language, tz, wantTarget)
		}
	}
}

func TestIntake_TargetCarriesTheCampaignInUTM(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	cdata := confirmedData(id, 1, immediate("tr", istanbul))
	cdata.CTAURL = "https://shop.example.com/cart" // no utm_campaign yet
	e.publishEvent(t, flow.EventTypeConfirmed, id, cdata)
	e.publishBatch(t, batch(id, 1, 1, recipient("CUS-1", "tr", istanbul, "sms")))
	eventually(t, 15*time.Second, "SEND message", func() bool { return e.sendTotal(t) == 1 })
	want, err := content.CTAURL(cdata.CTAURL, id)
	if err != nil {
		t.Fatal(err)
	}
	for did := range e.linkCodes(t, id) {
		req, _ := e.links.Request(did)
		if req.TargetURL != want || !strings.Contains(req.TargetURL, "utm_campaign="+id) {
			t.Errorf("target = %q, want %q", req.TargetURL, want)
		}
	}
}

func TestIntake_RedirectDownDelaysTheBatchAndNothingIsHalfWritten(t *testing.T) {
	e := newEnv(t)
	e.links.SetDown(true)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, immediate("tr", istanbul)))
	e.publishBatch(t, batch(id, 1, 1,
		recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "tr", istanbul, "email"), recipient("CUS-3", "tr", istanbul, "sms")))

	// The batch is retried again and again, with the transaction rolled back each time: no batch
	// record, no delivery, nothing queued, and the schedule has not fired.
	eventually(t, 15*time.Second, "the batch to be retried", func() bool { return e.links.Calls() >= 3 })
	if n := e.count(t, `SELECT count(*) FROM recipient_batch`); n != 0 {
		t.Errorf("recipient_batch rows = %d while the redirect service is down, want 0", n)
	}
	if n := e.count(t, `SELECT count(*) FROM delivery`); n != 0 {
		t.Errorf("delivery rows = %d while the redirect service is down, want 0", n)
	}
	if n := e.sendTotal(t); n != 0 {
		t.Errorf("%d messages queued while the redirect service is down", n)
	}
	if got := e.campaignStatus(t, id); got != "scheduled" {
		t.Errorf("campaign status = %q, want scheduled: the batch is not in yet", got)
	}

	// The service comes back: the next attempt writes the whole batch with its links.
	e.links.SetDown(false)
	eventually(t, 15*time.Second, "SEND messages", func() bool { return e.sendTotal(t) == 3 })
	e.idle(t)
	codes := e.linkCodes(t, id)
	if len(codes) != 3 {
		t.Fatalf("%d deliveries, want 3", len(codes))
	}
	for did, code := range codes {
		if code == "" || code != e.links.Code(did) {
			t.Errorf("delivery %s link_code = %q, want %q", did, code, e.links.Code(did))
		}
	}
	if n := e.count(t, `SELECT count(*) FROM recipient_batch`); n != 1 {
		t.Errorf("recipient_batch rows = %d, want 1", n)
	}
}

func TestIntake_RejectedLinkRequestIsTerminatedNotRetried(t *testing.T) {
	e := newEnv(t)
	e.links.Fail(&links.RejectedError{Status: 422, Detail: "target_url: must be an absolute http or https URL"})
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, immediate("tr", istanbul)))
	e.publishBatch(t, batch(id, 1, 1, recipient("CUS-1", "tr", istanbul, "sms")))
	eventually(t, 10*time.Second, "the batch to be terminated", func() bool { return len(e.logs.find("malformed message terminated")) == 1 })
	e.idle(t)
	time.Sleep(200 * time.Millisecond) // a retry would call again
	if got := e.links.Calls(); got != 1 {
		t.Errorf("%d link requests, want exactly 1: a rejected request is not retried", got)
	}
	if n := e.count(t, `SELECT count(*) FROM delivery`); n != 0 {
		t.Errorf("delivery rows = %d, want 0", n)
	}
}

func TestIntake_DuplicateBatchDoesNotAskForLinksAgain(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, immediate("tr", istanbul)))
	b := batch(id, 1, 1, recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "tr", istanbul, "email"))
	e.publishBatch(t, b)
	eventually(t, 15*time.Second, "SEND messages", func() bool { return e.sendTotal(t) == 2 })
	e.publishBatch(t, b) // redelivered by the stream
	eventually(t, 10*time.Second, "the duplicate to be skipped", func() bool { return len(e.logs.find("duplicate batch skipped")) == 1 })
	e.idle(t)
	if got := e.links.Calls(); got != 1 {
		t.Errorf("%d link requests, want 1 (the duplicate batch is recognised before any call)", got)
	}
	// CUS-1 twice in one batch is one delivery, so one link.
	if got := e.links.Len(); got != 2 {
		t.Errorf("%d links made, want 2", got)
	}
	if n := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id); n != 2 {
		t.Errorf("delivery rows = %d, want 2", n)
	}
}
