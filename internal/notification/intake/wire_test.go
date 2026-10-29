package intake

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func validConfirmed() ConfirmedData {
	return ConfirmedData{
		CampaignID: "0199f3a1-0000-7000-8000-000000000001",
		Schedules: []ConfirmedSchedule{
			{Language: "tr", Timezone: "Europe/Istanbul"},
			{Language: "en", Timezone: "Europe/London"},
		},
		Messages: []ConfirmedMessage{
			{Language: "tr", Channel: "sms", Content: json.RawMessage(`{"text":"x"}`)},
			{Language: "tr", Channel: "email", Content: json.RawMessage(`{"subject":"x"}`)},
		},
		CTAURL:           "https://shop.example.com",
		RecipientBatches: 3,
	}
}

func TestConfirmedData_Validate(t *testing.T) {
	t.Parallel()
	valid := validConfirmed()
	if _, err := valid.validate(); err != nil {
		t.Fatalf("validate() on valid data: %v", err)
	}
	tests := []struct {
		name    string
		mutate  func(*ConfirmedData)
		wantErr string
	}{
		{"campaign id not a uuid", func(d *ConfirmedData) { d.CampaignID = "abc" }, "not a uuid"},
		{"empty campaign id", func(d *ConfirmedData) { d.CampaignID = "" }, "not a uuid"},
		{"blank cta url", func(d *ConfirmedData) { d.CTAURL = "  " }, "cta_url"},
		{"no batches", func(d *ConfirmedData) { d.RecipientBatches = 0 }, "recipient_batches"},
		{"too many batches", func(d *ConfirmedData) { d.RecipientBatches = math.MaxInt32 + 1 }, "recipient_batches"},
		{"no schedules", func(d *ConfirmedData) { d.Schedules = nil }, "no schedules"},
		{"no messages", func(d *ConfirmedData) { d.Messages = nil }, "no messages"},
		{"unsupported schedule language", func(d *ConfirmedData) { d.Schedules[0].Language = "de" }, "language"},
		{"empty timezone", func(d *ConfirmedData) { d.Schedules[0].Timezone = " " }, "timezone"},
		{"duplicate schedule", func(d *ConfirmedData) { d.Schedules[1] = d.Schedules[0] }, "appears twice"},
		{"unsupported message language", func(d *ConfirmedData) { d.Messages[0].Language = "fr" }, "language"},
		{"unsupported channel", func(d *ConfirmedData) { d.Messages[0].Channel = "push" }, "channel"},
		{"missing content", func(d *ConfirmedData) { d.Messages[0].Content = nil }, "content"},
		{"null content", func(d *ConfirmedData) { d.Messages[0].Content = json.RawMessage(`null`) }, "content"},
		{"broken content", func(d *ConfirmedData) { d.Messages[0].Content = json.RawMessage(`{x`) }, "content"},
		{"duplicate message", func(d *ConfirmedData) { d.Messages[1] = d.Messages[0] }, "appears twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := validConfirmed()
			d.Schedules = append([]ConfirmedSchedule(nil), d.Schedules...)
			d.Messages = append([]ConfirmedMessage(nil), d.Messages...)
			tt.mutate(&d)
			_, err := d.validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validate() error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestScheduleHeader(t *testing.T) {
	t.Parallel()
	ist := time.FixedZone("TRT", 3*3600)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"whole second, UTC", time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC), "@at 2026-10-29T12:00:00Z"},
		{"other zone is written as UTC", time.Date(2026, 10, 29, 15, 0, 0, 0, ist), "@at 2026-10-29T12:00:00Z"},
		{"a fraction rounds up, never early", time.Date(2026, 10, 29, 12, 0, 0, 1, time.UTC), "@at 2026-10-29T12:00:01Z"},
		{"half a second rounds up", time.Date(2026, 10, 29, 12, 0, 59, 500_000_000, time.UTC), "@at 2026-10-29T12:01:00Z"},
		{"microsecond precision of the database", time.Date(2026, 10, 29, 12, 0, 0, 123_456_000, time.UTC), "@at 2026-10-29T12:00:01Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := scheduleHeader(tt.at); got != tt.want {
				t.Errorf("scheduleHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubjects(t *testing.T) {
	t.Parallel()
	if got := ScheduleSubject("abc"); got != "sched.abc" {
		t.Errorf("ScheduleSubject = %q", got)
	}
	if got := FireSubject("abc"); got != "fire.abc" {
		t.Errorf("FireSubject = %q", got)
	}
	if got := SendSubject("sms"); got != "send.sms" {
		t.Errorf("SendSubject = %q", got)
	}
}

func TestConsumerConfigs(t *testing.T) {
	t.Parallel()
	ev, rc, sc := ConsumerConfigs()
	tests := []struct {
		name          string
		got           jetstream.ConsumerConfig
		durable       string
		filter        string
		ackWait       time.Duration
		maxAckPending int
	}{
		{"events", ev, "notification-events", "evt.proposal.>", 30 * time.Second, 100},
		{"recipients", rc, "notification-recipients", "recipients.>", 30 * time.Second, 500},
		{"schedules", sc, "notification-schedules", "fire.>", time.Minute, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := tt.got
			if c.Durable != tt.durable || c.FilterSubject != tt.filter || c.AckWait != tt.ackWait || c.MaxAckPending != tt.maxAckPending {
				t.Errorf("durable/filter/ack_wait/max_ack_pending = %s/%s/%v/%d, want %s/%s/%v/%d",
					c.Durable, c.FilterSubject, c.AckWait, c.MaxAckPending, tt.durable, tt.filter, tt.ackWait, tt.maxAckPending)
			}
			if c.MaxDeliver != 20 || c.AckPolicy != jetstream.AckExplicitPolicy {
				t.Errorf("max_deliver/ack_policy = %d/%v, want 20/explicit", c.MaxDeliver, c.AckPolicy)
			}
		})
	}
}
