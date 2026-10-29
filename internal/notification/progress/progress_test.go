package progress

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/content"
)

func TestCanComplete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    string
		scheduled int64
		active    int64
		want      bool
	}{
		{"sending, all fired, all final", "sending", 0, 0, true},
		{"a schedule is still scheduled", "sending", 1, 0, false},
		{"a delivery is pending or sending", "sending", 0, 3, false},
		{"both outstanding", "sending", 2, 5, false},
		{"already completed", "completed", 0, 0, false},
		{"cancelled never completes", "cancelled", 0, 0, false},
		{"not started yet", "scheduled", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canComplete(tt.status, tt.scheduled, tt.active); got != tt.want {
				t.Errorf("canComplete(%q, %d, %d) = %v, want %v", tt.status, tt.scheduled, tt.active, got, tt.want)
			}
		})
	}
}

func TestEstimate(t *testing.T) {
	t.Parallel()
	prices := content.Prices{SMSSegment: mustMoney(t, "0.04"), Email: mustMoney(t, "0.001")}
	tests := []struct {
		name     string
		segments int64
		counts   []Count
		want     string
	}{
		{"nothing sent", 0, nil, "0.0000"},
		{"sms only", 6, []Count{{"sms", "sent", 3}}, "0.2400"},
		{"sms and email", 6, []Count{{"email", "sent", 2}, {"sms", "sent", 3}}, "0.2420"},
		{"only sent emails are priced", 0, []Count{{"email", "sent", 5}, {"email", "failed", 100}, {"email", "skipped_no_consent", 7}, {"sms", "cancelled", 9}}, "0.0050"},
		{"a large campaign stays exact", 3_000_000, []Count{{"email", "sent", 1_000_000}}, "121000.0000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Estimate(tt.segments, tt.counts, prices).String(); got != tt.want {
				t.Errorf("Estimate(%d, %v) = %s, want %s", tt.segments, tt.counts, got, tt.want)
			}
		})
	}
}

func mustMoney(t *testing.T, s string) content.Money {
	t.Helper()
	m, err := content.ParseMoney(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWireJSONNames(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	started, err := json.Marshal(StartedData{CampaignID: "c", ScheduleID: "s", Language: "tr", Timezone: "Europe/Istanbul", StartedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"campaign_id":"c","schedule_id":"s","language":"tr","timezone":"Europe/Istanbul","started_at":"2026-10-29T09:00:00Z"}`; string(started) != want {
		t.Errorf("started data = %s, want %s", started, want)
	}
	completed, err := json.Marshal(CompletedData{
		CampaignID: "c", CompletedAt: at, Counts: []Count{{Channel: "sms", Status: "sent", Count: 3}}, SMSSegments: 6, EstimatedCostUSD: "0.2400",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"campaign_id":"c","completed_at":"2026-10-29T09:00:00Z","counts":[{"channel":"sms","status":"sent","count":3}],"sms_segments":6,"estimated_cost_usd":"0.2400"}`; string(completed) != want {
		t.Errorf("completed data = %s, want %s", completed, want)
	}
}

func TestNudgeNeverBlocks(t *testing.T) {
	t.Parallel()
	c, err := New(Config{Pool: nonNilPool(t)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			c.Nudge() // nobody reads: all but the first are dropped
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Nudge blocked")
	}
	if len(c.nudge) != 1 {
		t.Errorf("waiting nudges = %d, want 1", len(c.nudge))
	}
}

func TestNewRequiresAPool(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); err == nil {
		t.Error("New without a pool succeeded")
	}
}

// nonNilPool is a pool that never connects (the first use would fail); the tests that take it do not use the database.
func nonNilPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(t.Context(), "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
