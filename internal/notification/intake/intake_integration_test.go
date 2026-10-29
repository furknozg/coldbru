//go:build integration

package intake_test

import (
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// status helpers.
func (e *env) campaignStatus(t *testing.T, id string) string {
	t.Helper()
	return e.scalar(t, `SELECT status FROM send_campaign WHERE id = $1::uuid`, id)
}

func (e *env) scheduleStatus(t *testing.T, campaignID, language, tz string) string {
	t.Helper()
	return e.scalar(t, `SELECT status FROM locale_schedule WHERE campaign_id = $1::uuid AND language = $2 AND timezone = $3`,
		campaignID, language, tz)
}

func (e *env) scheduleID(t *testing.T, campaignID, language, tz string) string {
	t.Helper()
	return e.scalar(t, `SELECT id::text FROM locale_schedule WHERE campaign_id = $1::uuid AND language = $2 AND timezone = $3`,
		campaignID, language, tz)
}

func (e *env) deliveries(t *testing.T, campaignID, status string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND status = $2`, campaignID, status)
}

func TestIntake_ConfirmedEventCreatesRowsAndFiresImmediateSchedule(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	cdata := confirmedData(id, 1, immediate("tr", istanbul))
	rs := []flow.BatchRecipient{
		recipient("CUS-1", "tr", istanbul, "sms"),
		recipient("CUS-2", "tr", istanbul, "sms"),
		recipient("CUS-3", "tr", istanbul, "email"),
		recipient("CUS-4", "tr", istanbul, "email"),
		recipient("CUS-5", "tr", istanbul, "email"),
	}
	e.publishEvent(t, flow.EventTypeConfirmed, id, cdata)
	e.publishBatch(t, batch(id, 1, 1, rs...))

	eventually(t, 15*time.Second, "SEND messages", func() bool { return e.sendTotal(t) == 5 })
	e.idle(t)

	// Rows of the event.
	if got := e.scalar(t, `SELECT cta_url FROM send_campaign WHERE id = $1::uuid`, id); got != cdata.CTAURL {
		t.Errorf("cta_url = %q, want %q", got, cdata.CTAURL)
	}
	if got := e.count(t, `SELECT batch_count FROM send_campaign WHERE id = $1::uuid`, id); got != 1 {
		t.Errorf("batch_count = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM locale_schedule WHERE campaign_id = $1::uuid`, id); got != 1 {
		t.Errorf("locale_schedule rows = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM message WHERE campaign_id = $1::uuid`, id); got != 3 {
		t.Errorf("message rows = %d, want 3", got)
	}
	if got := e.scalar(t, `SELECT content::text FROM message WHERE campaign_id = $1::uuid AND language = 'tr' AND channel = 'sms'`, id); !strings.Contains(got, "sepetin seni bekliyor") {
		t.Errorf("sms content = %s", got)
	}
	if got := e.count(t, `SELECT count(*) FROM recipient_batch WHERE campaign_id = $1::uuid`, id); got != 1 {
		t.Errorf("recipient_batch rows = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id); got != 5 {
		t.Errorf("delivery rows = %d, want 5", got)
	}
	if got := e.count(t, `SELECT count(*) FROM processed_event`); got != 1 {
		t.Errorf("processed_event rows = %d, want 1 (the event; raw messages leave none)", got)
	}

	// The firing.
	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "fired" {
		t.Errorf("schedule status = %q, want fired", got)
	}
	if got := e.scalar(t, `SELECT (fired_at IS NOT NULL)::text FROM locale_schedule WHERE campaign_id = $1::uuid`, id); got != "true" {
		t.Errorf("fired_at is set = %s, want true", got)
	}
	if got := e.campaignStatus(t, id); got != "sending" {
		t.Errorf("campaign status = %q, want sending", got)
	}
	// Deliveries stay pending; the send worker moves them.
	if got := e.deliveries(t, id, "pending"); got != 5 {
		t.Errorf("pending deliveries = %d, want 5", got)
	}

	// SEND: one message per delivery, subject by channel, payload is the delivery id.
	byChannel := e.sendDeliveryIDs(t)
	if len(byChannel["send.sms"]) != 2 || len(byChannel["send.email"]) != 3 || len(byChannel) != 2 {
		t.Fatalf("SEND = %v, want 2 on send.sms and 3 on send.email", byChannel)
	}
	for channel, ids := range byChannel {
		want := strings.TrimPrefix(channel, "send.")
		for _, did := range ids {
			if got := e.scalar(t, `SELECT channel FROM delivery WHERE id = $1::uuid`, did); got != want {
				t.Errorf("delivery %s has channel %q but was queued on %s", did, got, channel)
			}
		}
	}
	// The deliveries carry the right schedule and message.
	if got := e.count(t, `SELECT count(*) FROM delivery d JOIN message m ON m.id = d.message_id
		WHERE d.campaign_id = $1::uuid AND m.language = 'tr' AND m.channel = d.channel`, id); got != 5 {
		t.Errorf("deliveries joined to their tr message = %d, want 5", got)
	}
	// A direct firing is published without schedule headers: no sched.* message exists.
	for subject := range e.subjectCounts(t, "SCHEDULES") {
		if strings.HasPrefix(subject, "sched.") {
			t.Errorf("SCHEDULES holds %s, want only fire.* for an immediate schedule", subject)
		}
	}
}

func TestIntake_ScheduledScheduleFiresAtItsTime(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	sendAt := time.Now().Add(3 * time.Second).UTC().Truncate(time.Second).Add(time.Second)
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, at("tr", istanbul, sendAt)))
	e.publishBatch(t, batch(id, 1, 1,
		recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "tr", istanbul, "email")))

	// Rows exist and the schedule is waiting in NATS, not fired.
	eventually(t, 10*time.Second, "batch and schedule rows", func() bool {
		return e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id) == 2
	})
	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "scheduled" {
		t.Errorf("schedule status before the send time = %q, want scheduled", got)
	}
	if got := e.campaignStatus(t, id); got != "scheduled" {
		t.Errorf("campaign status before the send time = %q, want scheduled", got)
	}
	schedID := e.scheduleID(t, id, "tr", istanbul)
	if got := e.subjectCounts(t, "SCHEDULES")[intake.ScheduleSubject(schedID)]; got != 1 {
		t.Errorf("messages on %s = %d, want 1 waiting for its time", intake.ScheduleSubject(schedID), got)
	}
	// The relay published the headers the outbox row holds.
	var hdr string
	if err := e.pool.QueryRow(t.Context(), `SELECT headers::text FROM outbox WHERE subject = $1`, intake.ScheduleSubject(schedID)).Scan(&hdr); err != nil {
		t.Fatalf("outbox headers: %v", err)
	}
	for _, want := range []string{`"Nats-Schedule": "@at ` + sendAt.Format(time.RFC3339) + `"`, `"Nats-Schedule-Target": "fire.` + schedID + `"`} {
		if !strings.Contains(hdr, want) {
			t.Errorf("outbox headers = %s, want it to contain %s", hdr, want)
		}
	}
	if got := e.sendTotal(t); got != 0 {
		t.Fatalf("SEND holds %d messages before the send time", got)
	}

	eventually(t, 15*time.Second, "schedule to fire", func() bool { return e.scheduleStatus(t, id, "tr", istanbul) == "fired" })
	e.idle(t)
	firedAt := time.Now()
	if firedAt.Before(sendAt) {
		t.Errorf("fired at %s, before the send time %s", firedAt.Format(time.RFC3339Nano), sendAt.Format(time.RFC3339Nano))
	}
	if got := e.campaignStatus(t, id); got != "sending" {
		t.Errorf("campaign status = %q, want sending", got)
	}
	if got := e.sendTotal(t); got != 2 {
		t.Errorf("SEND messages = %d, want 2", got)
	}
}

func TestIntake_TwoSchedulesFireIndependently(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	later := time.Now().Add(1 * time.Hour)
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, immediate("tr", istanbul), at("en", london, later)))
	e.publishBatch(t, batch(id, 1, 1,
		recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "en", london, "sms")))

	eventually(t, 15*time.Second, "tr schedule fired", func() bool { return e.scheduleStatus(t, id, "tr", istanbul) == "fired" })
	e.idle(t)
	if got := e.scheduleStatus(t, id, "en", london); got != "scheduled" {
		t.Errorf("en schedule = %q, want scheduled (an hour away)", got)
	}
	byChannel := e.sendDeliveryIDs(t)
	if len(byChannel["send.sms"]) != 1 {
		t.Fatalf("SEND = %v, want only the tr delivery", byChannel)
	}
	if got := e.scalar(t, `SELECT language FROM delivery d JOIN message m ON m.id = d.message_id WHERE d.id = $1::uuid`, byChannel["send.sms"][0]); got != "tr" {
		t.Errorf("queued delivery is %q, want tr", got)
	}
	if got := e.campaignStatus(t, id); got != "sending" {
		t.Errorf("campaign status = %q, want sending after the first firing", got)
	}
}

func TestIntake_BatchBeforeEventIsRetriedThenAccepted(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishBatch(t, batch(id, 1, 1, recipient("CUS-1", "tr", istanbul, "sms")))

	// The campaign is not there: the batch is nak'd, not failed, terminated or recorded.
	eventually(t, 10*time.Second, "batch to be deferred", func() bool { return len(e.logs.find("message deferred")) >= 2 })
	if got := e.count(t, `SELECT count(*) FROM recipient_batch`); got != 0 {
		t.Errorf("recipient_batch rows = %d, want 0 before the event", got)
	}
	if got := e.logs.atLevel(slog.LevelWarn); got != 0 {
		t.Errorf("records at warn or above = %d, want 0 (waiting is not a failure)", got)
	}

	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, immediate("tr", istanbul)))
	eventually(t, 15*time.Second, "SEND message", func() bool { return e.sendTotal(t) == 1 })
	e.idle(t)
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id); got != 1 {
		t.Errorf("delivery rows = %d, want 1", got)
	}
	if got := e.logs.atLevel(slog.LevelWarn); got != 0 {
		t.Errorf("records at warn or above = %d, want 0", got)
	}
}

func TestIntake_IncompleteBatchesDelayTheFire(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 2, immediate("tr", istanbul)))
	e.publishBatch(t, batch(id, 1, 2, recipient("CUS-1", "tr", istanbul, "sms")))

	eventually(t, 10*time.Second, "first batch stored", func() bool {
		return e.count(t, `SELECT count(*) FROM recipient_batch WHERE campaign_id = $1::uuid`, id) == 1
	})
	// The fire message is waiting for the second batch: deferred several times, nothing sent.
	eventually(t, 10*time.Second, "fire to be deferred", func() bool { return len(e.logs.find("message deferred")) >= 3 })
	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "scheduled" {
		t.Errorf("schedule status = %q, want scheduled", got)
	}
	if got := e.campaignStatus(t, id); got != "scheduled" {
		t.Errorf("campaign status = %q, want scheduled", got)
	}
	if got := e.sendTotal(t); got != 0 {
		t.Errorf("SEND messages = %d, want 0 while a batch is missing", got)
	}

	e.publishBatch(t, batch(id, 2, 2, recipient("CUS-2", "tr", istanbul, "email")))
	eventually(t, 15*time.Second, "SEND messages", func() bool { return e.sendTotal(t) == 2 })
	e.idle(t)
	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "fired" {
		t.Errorf("schedule status = %q, want fired", got)
	}
	byChannel := e.sendDeliveryIDs(t)
	if len(byChannel["send.sms"]) != 1 || len(byChannel["send.email"]) != 1 {
		t.Errorf("SEND = %v, want one sms and one email", byChannel)
	}
}

func TestIntake_DuplicatesAreNoOps(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	cdata := confirmedData(id, 1, immediate("tr", istanbul))
	b := batch(id, 1, 1, recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "tr", istanbul, "email"))

	evID := e.publishEvent(t, flow.EventTypeConfirmed, id, cdata)
	e.publishBatch(t, b)
	eventually(t, 15*time.Second, "SEND messages", func() bool { return e.sendTotal(t) == 2 })
	e.idle(t)
	schedID := e.scheduleID(t, id, "tr", istanbul)
	outboxBefore := e.count(t, `SELECT count(*) FROM outbox`)

	// 1. The same envelope again (a redelivery with a new Nats-Msg-Id): processed_event stops it.
	env, err := events.New(flow.EventTypeConfirmed, flow.EventVersion, flow.EventProducer, id, cdata)
	if err != nil {
		t.Fatal(err)
	}
	env.ID = evID
	e.publishEnvelope(t, env) // same Msg-Id: JetStream drops it inside the window ...
	env2 := env
	env2.OccurredAt = env.OccurredAt.Add(time.Second) // ... so vary the body to force a second delivery.
	if _, err := e.js.Publish(t.Context(), "evt.proposal.confirmed.v1", mustMarshal(t, env2)); err != nil {
		t.Fatal(err)
	}
	// 2. A different envelope for the same campaign: the existing campaign makes it a no-op.
	e.publishEvent(t, flow.EventTypeConfirmed, id, cdata)
	// 3. The same batch again.
	e.publishBatch(t, b)
	// 4. The same firing again.
	e.publishRaw(t, intake.FireSubject(schedID), mustMarshal(t, intake.FirePayload{ScheduleID: schedID, CampaignID: id}))
	e.idle(t)
	time.Sleep(200 * time.Millisecond)
	e.idle(t)

	if got := e.count(t, `SELECT count(*) FROM send_campaign`); got != 1 {
		t.Errorf("send_campaign rows = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM locale_schedule`); got != 1 {
		t.Errorf("locale_schedule rows = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM message`); got != 3 {
		t.Errorf("message rows = %d, want 3", got)
	}
	if got := e.count(t, `SELECT count(*) FROM recipient_batch`); got != 1 {
		t.Errorf("recipient_batch rows = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery`); got != 2 {
		t.Errorf("delivery rows = %d, want 2", got)
	}
	if got := e.sendTotal(t); got != 2 {
		t.Errorf("SEND messages = %d, want 2 (the repeated firing queues nothing)", got)
	}
	if got := e.count(t, `SELECT count(*) FROM outbox`); got != outboxBefore {
		t.Errorf("outbox rows = %d, want %d (no repeat writes anything)", got, outboxBefore)
	}
	if got := e.logs.atLevel(slog.LevelWarn); got != 0 {
		t.Errorf("records at warn or above = %d, want 0", got)
	}
}

func TestIntake_CancelBeforeFire(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	soon := time.Now().Add(4 * time.Second)
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, at("tr", istanbul, soon), at("en", london, time.Now().Add(time.Hour))))
	e.publishBatch(t, batch(id, 1, 1,
		recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "tr", istanbul, "email"), recipient("CUS-3", "en", london, "sms")))
	eventually(t, 10*time.Second, "deliveries", func() bool { return e.deliveries(t, id, "pending") == 3 })

	e.publishEvent(t, flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "changed my mind"})
	eventually(t, 10*time.Second, "campaign cancelled", func() bool { return e.campaignStatus(t, id) == "cancelled" })
	e.idle(t)

	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "cancelled" {
		t.Errorf("tr schedule = %q, want cancelled", got)
	}
	if got := e.scheduleStatus(t, id, "en", london); got != "cancelled" {
		t.Errorf("en schedule = %q, want cancelled", got)
	}
	if got := e.deliveries(t, id, "cancelled"); got != 3 {
		t.Errorf("cancelled deliveries = %d, want 3", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id); got != 0 {
		t.Errorf("deliveries still holding personal data = %d, want 0", got)
	}
	// customer_id, channel and status stay for the dashboard.
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND customer_id <> ''`, id); got != 3 {
		t.Errorf("deliveries keeping customer_id = %d, want 3", got)
	}

	// The scheduled message fires anyway (NATS cannot cancel it); the handler drops it.
	never(t, 6*time.Second, "a cancelled campaign sent something", func() bool { return e.sendTotal(t) != 0 })
	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "cancelled" {
		t.Errorf("tr schedule after its time = %q, want cancelled", got)
	}
	if len(e.logs.find("fire for cancelled campaign dropped")) == 0 {
		t.Error("no log of the dropped firing; the scheduled message should have arrived and been dropped")
	}
}

func TestIntake_CancelAfterFire(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, immediate("tr", istanbul), at("en", london, time.Now().Add(time.Hour))))
	e.publishBatch(t, batch(id, 1, 1, recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "en", london, "sms")))
	eventually(t, 15*time.Second, "tr schedule fired", func() bool { return e.sendTotal(t) == 1 })
	e.idle(t)
	if got := e.campaignStatus(t, id); got != "sending" {
		t.Fatalf("campaign status = %q, want sending", got)
	}

	e.publishEvent(t, flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "stop"})
	eventually(t, 10*time.Second, "campaign cancelled", func() bool { return e.campaignStatus(t, id) == "cancelled" })
	e.idle(t)

	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "fired" {
		t.Errorf("tr schedule = %q, want fired (it did fire)", got)
	}
	if got := e.scheduleStatus(t, id, "en", london); got != "cancelled" {
		t.Errorf("en schedule = %q, want cancelled", got)
	}
	// Both deliveries were still pending (the send worker is not there yet): both are cancelled.
	if got := e.deliveries(t, id, "cancelled"); got != 2 {
		t.Errorf("cancelled deliveries = %d, want 2", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id); got != 0 {
		t.Errorf("deliveries still holding personal data = %d, want 0", got)
	}
	// The message already on SEND stays; the worker drops it on the delivery's status (T-029).
	if got := e.sendTotal(t); got != 1 {
		t.Errorf("SEND messages = %d, want the 1 queued before the cancel", got)
	}
}

func TestIntake_CancelIsIdempotentAndIgnoresLateBatches(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 2, at("tr", istanbul, time.Now().Add(time.Hour))))
	e.publishBatch(t, batch(id, 1, 2, recipient("CUS-1", "tr", istanbul, "sms")))
	eventually(t, 10*time.Second, "first batch", func() bool { return e.deliveries(t, id, "pending") == 1 })
	e.publishEvent(t, flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "x"})
	eventually(t, 10*time.Second, "campaign cancelled", func() bool { return e.campaignStatus(t, id) == "cancelled" })
	e.publishEvent(t, flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "again"})
	eventually(t, 10*time.Second, "second cancel ignored", func() bool { return len(e.logs.find("cancel for finished campaign ignored")) == 1 })

	// The second batch arrives after the cancel: dropped, so no personal data lands on the campaign.
	e.publishBatch(t, batch(id, 2, 2, recipient("CUS-2", "tr", istanbul, "email")))
	eventually(t, 10*time.Second, "late batch dropped", func() bool { return len(e.logs.find("batch for finished campaign dropped")) == 1 })
	e.idle(t)
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id); got != 1 {
		t.Errorf("delivery rows = %d, want 1 (the late batch writes none)", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id); got != 0 {
		t.Errorf("deliveries holding personal data = %d, want 0", got)
	}
	if got := e.logs.atLevel(slog.LevelWarn); got != 0 {
		t.Errorf("records at warn or above = %d, want 0", got)
	}
}

func TestIntake_CancelBeforeConfirmedTakesEffectOnceTheCampaignExists(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "early"})

	// The cancel is deferred, not dropped and not logged as a failure.
	eventually(t, 10*time.Second, "cancel to be deferred", func() bool { return len(e.logs.find("message deferred")) >= 2 })
	if got := e.logs.atLevel(slog.LevelWarn); got != 0 {
		t.Errorf("records at warn or above = %d, want 0", got)
	}

	// The confirmed event lands; the schedule is an hour away, so only the cancel can matter.
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, at("tr", istanbul, time.Now().Add(time.Hour))))
	e.publishBatch(t, batch(id, 1, 1, recipient("CUS-1", "tr", istanbul, "sms"), recipient("CUS-2", "tr", istanbul, "email")))
	eventually(t, 15*time.Second, "campaign cancelled", func() bool { return e.campaignStatus(t, id) == "cancelled" })
	eventually(t, 15*time.Second, "batch handled", func() bool {
		return e.count(t, `SELECT count(*) FROM recipient_batch WHERE campaign_id = $1::uuid`, id) == 1 ||
			len(e.logs.find("batch for finished campaign dropped")) == 1
	})
	e.idle(t)

	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "cancelled" {
		t.Errorf("schedule = %q, want cancelled", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND status <> 'cancelled'`, id); got != 0 {
		t.Errorf("deliveries not cancelled = %d, want 0", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND (first_name IS NOT NULL OR contact IS NOT NULL)`, id); got != 0 {
		t.Errorf("deliveries holding personal data = %d, want 0", got)
	}
	if got := e.sendTotal(t); got != 0 {
		t.Errorf("SEND messages = %d, want 0", got)
	}
}

func TestIntake_CancelForUnknownCampaignIsTerminatedAtMaxDeliver(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "never confirmed"})

	eventually(t, 30*time.Second, "cancel terminated", func() bool {
		return len(e.logs.find("message still not applicable, delivery limit reached, terminated")) == 1
	})
	e.idle(t)
	if got := len(e.logs.find("message deferred")); got != 19 {
		t.Errorf("deferrals = %d, want 19 (attempts 1 to 19)", got)
	}
	if got := e.logs.atLevel(slog.LevelError); got != 1 {
		t.Errorf("error records = %d, want exactly the termination", got)
	}
	if got := e.count(t, `SELECT count(*) FROM send_campaign`); got != 0 {
		t.Errorf("send_campaign rows = %d, want 0", got)
	}
	if got := e.count(t, `SELECT count(*) FROM processed_event`); got != 0 {
		t.Errorf("processed_event rows = %d, want 0 (every attempt rolled back)", got)
	}
}

// firesLeft counts the messages on fire.* in the SCHEDULES stream; schedulesLeft those on sched.*.
func (e *env) firesLeft(t *testing.T) (fires, schedules uint64) {
	t.Helper()
	for subject, n := range e.subjectCounts(t, "SCHEDULES") {
		switch {
		case strings.HasPrefix(subject, "fire."):
			fires += n
		case strings.HasPrefix(subject, "sched."):
			schedules += n
		}
	}
	return fires, schedules
}

func TestIntake_ProcessedFireMessagesAreDeletedFromTheStream(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	// tr fires at once; en waits an hour as a sched.* message that must stay.
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 2, immediate("tr", istanbul), at("en", london, time.Now().Add(time.Hour))))
	e.publishBatch(t, batch(id, 1, 2, recipient("CUS-1", "tr", istanbul, "sms")))

	// While the second batch is missing the firing is deferred and its message must stay.
	eventually(t, 10*time.Second, "firing deferred", func() bool {
		n := 0
		for _, r := range e.logs.find("message deferred") {
			if subject, _ := r.attrs["subject"].(string); strings.HasPrefix(subject, "fire.") {
				n++
			}
		}
		return n >= 2
	})
	if fires, _ := e.firesLeft(t); fires != 1 {
		t.Fatalf("fire messages while deferred = %d, want 1 (RetryLater keeps it)", fires)
	}

	e.publishBatch(t, batch(id, 2, 2, recipient("CUS-2", "en", london, "sms")))
	eventually(t, 15*time.Second, "tr schedule fired", func() bool { return e.scheduleStatus(t, id, "tr", istanbul) == "fired" })
	e.idle(t)
	eventually(t, 5*time.Second, "fire message deleted", func() bool {
		fires, _ := e.firesLeft(t)
		return fires == 0
	})
	enID := e.scheduleID(t, id, "en", london)
	if _, schedules := e.firesLeft(t); schedules != 1 {
		t.Errorf("sched.* messages = %d, want the 1 pending en schedule", schedules)
	}
	if got := e.subjectCounts(t, "SCHEDULES")[intake.ScheduleSubject(enID)]; got != 1 {
		t.Errorf("messages on %s = %d, want 1", intake.ScheduleSubject(enID), got)
	}

	// A repeated firing (no-op as already fired) is deleted too, and so is one dropped as cancelled.
	trID := e.scheduleID(t, id, "tr", istanbul)
	e.publishRaw(t, intake.FireSubject(trID), mustMarshal(t, intake.FirePayload{ScheduleID: trID, CampaignID: id}))
	eventually(t, 10*time.Second, "duplicate fire skipped", func() bool { return len(e.logs.find("duplicate fire skipped")) == 1 })
	e.idle(t)
	eventually(t, 5*time.Second, "duplicate fire deleted", func() bool {
		fires, _ := e.firesLeft(t)
		return fires == 0
	})

	e.publishEvent(t, flow.EventTypeCancelled, id, flow.CancelledData{CampaignID: id, Reason: "stop"})
	eventually(t, 10*time.Second, "campaign cancelled", func() bool { return e.campaignStatus(t, id) == "cancelled" })
	e.publishRaw(t, intake.FireSubject(enID), mustMarshal(t, intake.FirePayload{ScheduleID: enID, CampaignID: id}))
	eventually(t, 10*time.Second, "fire dropped", func() bool { return len(e.logs.find("fire for cancelled campaign dropped")) == 1 })
	e.idle(t)
	eventually(t, 5*time.Second, "dropped fire deleted", func() bool {
		fires, _ := e.firesLeft(t)
		return fires == 0
	})
	if got := e.logs.atLevel(slog.LevelWarn); got != 0 {
		t.Errorf("records at warn or above = %d, want 0 (no failed delete)", got)
	}
}

func TestIntake_UnroutableRecipientsAreSkippedAndLogged(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	// Schedules: tr/Istanbul only. Messages: tr sms, tr email, en sms.
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, immediate("tr", istanbul)))
	good := []flow.BatchRecipient{
		recipient("CUS-1", "tr", istanbul, "sms"),
		recipient("CUS-2", "tr", istanbul, "email"),
	}
	skippedRecipients := []flow.BatchRecipient{
		recipient("CUS-3", "tr", london, "sms"),                                                        // no schedule for (tr, Europe/London)
		recipient("CUS-4", "tr", london, "sms"),                                                        // same group, counted together
		recipient("CUS-5", "en", istanbul, "sms"),                                                      // no schedule for (en, Europe/Istanbul)
		recipient("CUS-6", "en", istanbul, "email"),                                                    // no schedule (checked before the message)
		{CustomerID: "", Language: "tr", Timezone: istanbul, Channel: "sms", Contact: "+905551112233"}, // no customer id
	}
	e.publishBatch(t, batch(id, 1, 1, append(append([]flow.BatchRecipient{}, good...), skippedRecipients...)...))

	eventually(t, 15*time.Second, "SEND messages", func() bool { return e.sendTotal(t) == 2 })
	e.idle(t)

	// The batch counts as received and the routable recipients are written.
	if got := e.count(t, `SELECT count(*) FROM recipient_batch WHERE campaign_id = $1::uuid`, id); got != 1 {
		t.Errorf("recipient_batch rows = %d, want 1", got)
	}
	if got := e.count(t, `SELECT recipients FROM recipient_batch WHERE campaign_id = $1::uuid`, id); got != 7 {
		t.Errorf("recipient_batch.recipients = %d, want 7 (received, skipped ones included)", got)
	}
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id); got != 2 {
		t.Errorf("delivery rows = %d, want 2", got)
	}
	if got := e.scheduleStatus(t, id, "tr", istanbul); got != "fired" {
		t.Errorf("schedule = %q, want fired (the skipped recipients do not hold the batch count back)", got)
	}

	// One error record per (language, timezone, channel, reason) with the counts.
	got := map[string]int64{}
	for _, r := range e.logs.find("recipients skipped") {
		if r.level != slog.LevelError {
			t.Errorf("skipped record level = %v, want error", r.level)
		}
		if r.attrs["campaign_id"] != id || r.attrs["batch_no"] != int64(1) {
			t.Errorf("skipped record campaign_id/batch_no = %v/%v, want %s/1", r.attrs["campaign_id"], r.attrs["batch_no"], id)
		}
		key := r.attrs["language"].(string) + "|" + r.attrs["timezone"].(string) + "|" + r.attrs["channel"].(string)
		got[key] += r.attrs["skipped"].(int64)
	}
	want := map[string]int64{
		"tr|Europe/London|sms":     2,
		"en|Europe/Istanbul|sms":   1,
		"en|Europe/Istanbul|email": 1,
		"tr|Europe/Istanbul|sms":   1, // the recipient without a customer id
	}
	if len(got) != len(want) {
		t.Errorf("skipped groups = %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("skipped[%s] = %d, want %d", k, got[k], n)
		}
	}
	reasons := []string{}
	for _, r := range e.logs.find("recipients skipped") {
		reasons = append(reasons, r.attrs["reason"].(string))
	}
	sort.Strings(reasons)
	if len(reasons) != 4 {
		t.Errorf("skipped records = %d (%v), want 4", len(reasons), reasons)
	}

	// A recipient with a schedule but no message for its (language, channel).
	id2 := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id2, confirmedData(id2, 1, immediate("tr", istanbul), immediate("en", london)))
	e.publishBatch(t, batch(id2, 1, 1,
		recipient("CUS-1", "en", london, "sms"),
		recipient("CUS-2", "en", london, "email"))) // messages hold en sms only
	eventually(t, 15*time.Second, "second campaign fired", func() bool { return e.scheduleStatus(t, id2, "en", london) == "fired" })
	e.idle(t)
	if got := e.count(t, `SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id2); got != 1 {
		t.Errorf("campaign 2 delivery rows = %d, want 1", got)
	}
	found := false
	for _, r := range e.logs.find("recipients skipped") {
		if r.attrs["campaign_id"] == id2 && r.attrs["channel"] == "email" && r.attrs["language"] == "en" && r.attrs["skipped"] == int64(1) {
			found = true
		}
	}
	if !found {
		t.Error("no skipped record for the en email recipient without a message")
	}
}

func TestIntake_MalformedMessagesAreTerminatedWithoutRetry(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()

	// A confirmed event no row can be made from (empty cta_url), an unparseable batch and a fire
	// payload that is not JSON.
	bad := confirmedData(id, 1, immediate("tr", istanbul))
	bad.CTAURL = " "
	e.publishEvent(t, flow.EventTypeConfirmed, id, bad)
	e.publishRaw(t, flow.RecipientsSubject(id), []byte(`{not json`))
	e.publishRaw(t, intake.FireSubject(newCampaignID()), []byte(`[]`))

	eventually(t, 10*time.Second, "three malformed records", func() bool {
		return len(e.logs.find("malformed message terminated"))+len(e.logs.find("malformed event terminated")) == 3
	})
	e.idle(t)
	time.Sleep(200 * time.Millisecond) // a retry would add records
	if got := len(e.logs.find("malformed message terminated")) + len(e.logs.find("malformed event terminated")); got != 3 {
		t.Errorf("malformed records = %d, want 3 (no retries)", got)
	}
	if got := e.count(t, `SELECT count(*) FROM send_campaign`); got != 0 {
		t.Errorf("send_campaign rows = %d, want 0", got)
	}
	if got := e.count(t, `SELECT count(*) FROM processed_event`); got != 0 {
		t.Errorf("processed_event rows = %d, want 0 (the malformed event rolled back)", got)
	}
	if got := e.logs.atLevel(slog.LevelError); got != 3 {
		t.Errorf("error records = %d, want exactly the 3 terminations", got)
	}
}

func TestIntake_BatchNumberAboveCountIsMalformed(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, flow.EventTypeConfirmed, id, confirmedData(id, 1, at("tr", istanbul, time.Now().Add(time.Hour))))
	e.publishBatch(t, batch(id, 2, 1, recipient("CUS-1", "tr", istanbul, "sms")))
	eventually(t, 10*time.Second, "batch terminated", func() bool { return len(e.logs.find("malformed message terminated")) == 1 })
	e.idle(t)
	if got := e.count(t, `SELECT count(*) FROM recipient_batch`); got != 0 {
		t.Errorf("recipient_batch rows = %d, want 0", got)
	}
}

func TestIntake_UnknownProposalEventIsIgnored(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.publishEvent(t, "proposal.something_new", id, map[string]string{"campaign_id": id})
	e.idle(t)
	if got := e.count(t, `SELECT count(*) FROM send_campaign`); got != 0 {
		t.Errorf("send_campaign rows = %d, want 0", got)
	}
	if got := e.logs.atLevel(slog.LevelWarn); got != 0 {
		t.Errorf("records at warn or above = %d, want 0", got)
	}
}

func TestStart_CreatesDurableConsumers(t *testing.T) {
	e := newEnv(t)
	ev, rc, sc := intake.ConsumerConfigs()
	for stream, want := range map[string]struct {
		name, filter  string
		maxAckPending int
	}{
		"EVENTS":     {ev.Durable, "evt.proposal.>", 100},
		"RECIPIENTS": {rc.Durable, "recipients.>", 500},
		"SCHEDULES":  {sc.Durable, "fire.>", 100},
	} {
		c, err := e.js.Consumer(t.Context(), stream, want.name)
		if err != nil {
			t.Fatalf("consumer %s on %s: %v", want.name, stream, err)
		}
		cfg := c.CachedInfo().Config
		if cfg.FilterSubject != want.filter || cfg.MaxDeliver != 20 || cfg.MaxAckPending != want.maxAckPending {
			t.Errorf("%s: filter %q max_deliver %d max_ack_pending %d, want %q 20 %d",
				want.name, cfg.FilterSubject, cfg.MaxDeliver, cfg.MaxAckPending, want.filter, want.maxAckPending)
		}
	}
}
