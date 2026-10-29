package progress_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	nprogress "github.com/furknozg/coldbru/internal/notification/progress"
	"github.com/furknozg/coldbru/internal/proposal/progress"
)

func jsonFields(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// TestWireMirrorsNotification fails when Notification's event data types gain or lose a field that
// Proposal's mirror does not know about, so the two cannot drift apart unnoticed (D-035, D-080).
func TestWireMirrorsNotification(t *testing.T) {
	tests := []struct {
		name         string
		notification reflect.Type
		mirror       reflect.Type
	}{
		{"started data", reflect.TypeFor[nprogress.StartedData](), reflect.TypeFor[progress.StartedData]()},
		{"completed data", reflect.TypeFor[nprogress.CompletedData](), reflect.TypeFor[progress.CompletedData]()},
		{"count", reflect.TypeFor[nprogress.Count](), reflect.TypeFor[progress.Count]()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := jsonFields(tt.mirror), jsonFields(tt.notification); !slices.Equal(got, want) {
				t.Errorf("mirror fields = %v, Notification fields = %v", got, want)
			}
		})
	}
}

func TestWireConstantsMatchNotification(t *testing.T) {
	if progress.EventTypeStarted != nprogress.EventTypeStarted || progress.EventTypeCompleted != nprogress.EventTypeCompleted || progress.EventVersion != nprogress.EventVersion {
		t.Errorf("event types differ: proposal %s %s v%d, notification %s %s v%d",
			progress.EventTypeStarted, progress.EventTypeCompleted, progress.EventVersion,
			nprogress.EventTypeStarted, nprogress.EventTypeCompleted, nprogress.EventVersion)
	}
}

func TestWireDecodesWhatNotificationPublishes(t *testing.T) {
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(nprogress.CompletedData{
		CampaignID: "c", CompletedAt: at, Counts: []nprogress.Count{{Channel: "sms", Status: "sent", Count: 3}}, SMSSegments: 6, EstimatedCostGBP: "0.2400",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got progress.CompletedData
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.CampaignID != "c" || !got.CompletedAt.Equal(at) || len(got.Counts) != 1 || got.Counts[0] != (progress.Count{Channel: "sms", Status: "sent", Count: 3}) ||
		got.SMSSegments != 6 || got.EstimatedCostGBP != "0.2400" {
		t.Errorf("decoded completed data = %+v", got)
	}
	raw, err = json.Marshal(nprogress.StartedData{CampaignID: "c", ScheduleID: "s", Language: "tr", Timezone: "Europe/Istanbul", StartedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	var started progress.StartedData
	if err := json.Unmarshal(raw, &started); err != nil || started.ScheduleID != "s" || started.Language != "tr" || started.Timezone != "Europe/Istanbul" || !started.StartedAt.Equal(at) {
		t.Errorf("decoded started data = %+v (%v)", started, err)
	}
}
