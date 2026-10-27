package consent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

// countingDB fails every BeginTx and counts the calls. A request that is
// rejected by validation never gets this far.
type countingDB struct{ calls int }

var errDBDown = errors.New("database down")

func (d *countingDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	d.calls++
	return nil, errDBDown
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ae *audience.Error
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestStatusValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ids     []string
		channel string
	}{
		{"unknown channel", []string{"CUS-000001"}, "push"},
		{"empty channel", []string{"CUS-000001"}, ""},
		{"wrong case channel", []string{"CUS-000001"}, "SMS"},
		{"channel is checked before the empty list", nil, "push"},
		{"1001 ids", make([]string, MaxIDs+1), ChannelSMS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			db := &countingDB{}
			_, err := NewService(db).Status(t.Context(), tt.ids, tt.channel)
			wantCode(t, err, audience.CodeBadUserInput)
			if db.calls != 0 {
				t.Errorf("BeginTx calls = %d, want 0", db.calls)
			}
		})
	}
}

func TestStatusWithoutIDsIsEmptyAndSkipsTheDatabase(t *testing.T) {
	t.Parallel()
	db := &countingDB{}
	got, err := NewService(db).Status(t.Context(), nil, ChannelEmail)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("result = %#v, want an empty non-nil slice", got)
	}
	if db.calls != 0 {
		t.Errorf("BeginTx calls = %d, want 0", db.calls)
	}
}

func TestStatusAtTheLimitReachesTheDatabase(t *testing.T) {
	t.Parallel()
	db := &countingDB{}
	_, err := NewService(db).Status(t.Context(), make([]string, MaxIDs), ChannelSMS)
	if !errors.Is(err, errDBDown) || db.calls != 1 {
		t.Errorf("err = %v, calls = %d, want the database error after one BeginTx", err, db.calls)
	}
}

func TestUpdateValidation(t *testing.T) {
	t.Parallel()
	ok := model.UpdateConsentInput{CustomerID: "CUS-000001", Channel: ChannelSMS, OptedIn: false, Source: "admin_tool"}
	tests := []struct {
		name   string
		mutate func(*model.UpdateConsentInput)
	}{
		{"unknown channel", func(in *model.UpdateConsentInput) { in.Channel = "push" }},
		{"empty channel", func(in *model.UpdateConsentInput) { in.Channel = "" }},
		{"free text source", func(in *model.UpdateConsentInput) { in.Source = "because" }},
		{"empty source", func(in *model.UpdateConsentInput) { in.Source = "" }},
		{"seed only source signup_form", func(in *model.UpdateConsentInput) { in.Source = "signup_form" }},
		{"seed only source account_deletion", func(in *model.UpdateConsentInput) { in.Source = "account_deletion" }},
		{"source in other case", func(in *model.UpdateConsentInput) { in.Source = "Admin_Tool" }},
		{"empty customer id", func(in *model.UpdateConsentInput) { in.CustomerID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := ok
			tt.mutate(&in)
			db := &countingDB{}
			_, err := NewService(db).Update(t.Context(), in)
			wantCode(t, err, audience.CodeBadUserInput)
			if db.calls != 0 {
				t.Errorf("BeginTx calls = %d, want 0", db.calls)
			}
		})
	}
}

func TestEverySourceIsAccepted(t *testing.T) {
	t.Parallel()
	want := []string{"preference_centre", "support_request", "unsubscribe_link", "admin_tool"}
	if fmt.Sprint(Sources) != fmt.Sprint(want) {
		t.Fatalf("Sources = %v, want %v", Sources, want)
	}
	for _, src := range Sources {
		db := &countingDB{}
		_, err := NewService(db).Update(t.Context(), model.UpdateConsentInput{
			CustomerID: "CUS-000001", Channel: ChannelEmail, Source: src,
		})
		if !errors.Is(err, errDBDown) || db.calls != 1 {
			t.Errorf("source %q: err = %v, calls = %d, want it to reach the database", src, err, db.calls)
		}
	}
}

func TestConsentID(t *testing.T) {
	t.Parallel()
	tests := map[int64]string{
		1:       "CNS-000001",
		2220:    "CNS-002220",
		999999:  "CNS-999999",
		1000000: "CNS-1000000", // wider, never truncated
	}
	for n, want := range tests {
		if got := ConsentID(n); got != want {
			t.Errorf("ConsentID(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestChannelSQLReusesTheReachabilityRule pins the eligibility condition to the
// audience constants, so the two cannot drift apart (D-062, D-064).
func TestChannelSQLReusesTheReachabilityRule(t *testing.T) {
	t.Parallel()
	col, cond, err := channelSQL(ChannelEmail)
	if err != nil || col != "email_opt_in" || !strings.Contains(cond, audience.SQLEmailOK) || !strings.Contains(cond, audience.SQLActive) {
		t.Errorf("email: %q %q %v", col, cond, err)
	}
	col, cond, err = channelSQL(ChannelSMS)
	if err != nil || col != "sms_opt_in" || !strings.Contains(cond, audience.SQLSMSOK) || !strings.Contains(cond, audience.SQLActive) {
		t.Errorf("sms: %q %q %v", col, cond, err)
	}
}
