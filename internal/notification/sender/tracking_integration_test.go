//go:build integration

package sender_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/notification/progress"
)

const trackingBase = "http://localhost:8080"

// Every message carries the short link of its own delivery: the SMS ends with it, the email button
// points at it, and the SMS stays within 160 characters with it (D-106, D-109).
func TestTrackedMessagesCarryTheLinkOfTheirDelivery(t *testing.T) {
	e := newEnv(t, withChecker(), withTracking(trackingBase))
	id := newCampaignID()
	e.campaignWith(id, smsCustomers[:3], emailCustomers[:2], 2)
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 5})

	codeOf := map[string]string{} // delivery id -> link code
	rows, err := e.pool.Query(t.Context(), `SELECT id::text, link_code FROM delivery WHERE campaign_id = $1::uuid`, id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var did, code string
		if err := rows.Scan(&did, &code); err != nil {
			t.Fatal(err)
		}
		codeOf[did] = code
	}
	rows.Close()
	if len(codeOf) != 5 {
		t.Fatalf("%d deliveries, want 5", len(codeOf))
	}

	seen := map[string]bool{}
	var sms, emails int
	for _, r := range e.prov.requests() {
		code := codeOf[r.Key]
		if code == "" {
			t.Fatalf("provider request with unknown key %s", r.Key)
		}
		link := content.TrackingLink(trackingBase, code)
		seen[r.Key] = true
		switch {
		case strings.HasSuffix(r.Path, "sms"):
			sms++
			if !strings.HasSuffix(r.Body["text"], " "+link) || len([]rune(r.Body["text"])) > content.SMSMaxChars {
				t.Errorf("sms %q: want it to end with %q within 160 characters", r.Body["text"], link)
			}
			if strings.Count(r.Body["text"], "http") != 1 {
				t.Errorf("sms %q has more than the one tracking link", r.Body["text"])
			}
		default:
			emails++
			if !strings.Contains(r.Body["html"], `href="`+link+`"`) || !strings.Contains(r.Body["text"], link) || strings.Contains(r.Body["html"], "shop.example.com") {
				t.Errorf("email of %s: html button or plain text lacks %s", r.Key, link)
			}
		}
	}
	if sms != 3 || emails != 2 || len(seen) != 5 {
		t.Errorf("provider got %d sms and %d emails for %d deliveries, want 3, 2, 5", sms, emails, len(seen))
	}
	for did := range codeOf {
		if _, err := uuid.Parse(did); err != nil {
			t.Fatal(err)
		}
	}

	// The completed event carries the denominators of the click-through rate (D-108).
	eventually(t, 10*time.Second, "the completed event", func() bool { return e.eventCount(completedSubject) == 1 })
	var cd progress.CompletedData
	if err := json.Unmarshal(e.lastEvent(completedSubject).Data, &cd); err != nil {
		t.Fatal(err)
	}
	want := []progress.LocaleCount{
		{Channel: "email", Language: "tr", Timezone: istanbul, Sent: 2},
		{Channel: "sms", Language: "tr", Timezone: istanbul, Sent: 3},
	}
	if len(cd.SentByLocale) != 2 || cd.SentByLocale[0] != want[0] || cd.SentByLocale[1] != want[1] {
		t.Errorf("sent_by_locale = %+v, want %+v", cd.SentByLocale, want)
	}
}

// Without a TRACKING_BASE_URL the worker sends what it sent before links existed.
func TestWithoutATrackingBaseNothingIsAppended(t *testing.T) {
	e := newEnv(t)
	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:1], emailCustomers[:1])
	e.waitStatuses(id, 20*time.Second, map[string]int{"sent": 2})
	for _, r := range e.prov.requests() {
		if strings.Contains(r.Body["text"], "/r/") || strings.Contains(r.Body["html"], "/r/") {
			t.Errorf("request %s carries a tracking link without a base URL: %v", r.Path, r.Body)
		}
	}
}
