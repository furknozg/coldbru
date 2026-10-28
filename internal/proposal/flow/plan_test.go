package flow

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// sample is the coverage of the sample brief (D-010): 46 reachable, email and SMS tie at 33.
var sample = Coverage{Both: 20, EmailOnly: 13, SMSOnly: 13, Unreachable: 3}

func TestMessagesFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		mode      llm.PlanMode
		primary   llm.Channel
		cov       Coverage
		wantEmail int
		wantSMS   int
	}{
		{"single email", llm.ModeSingle, llm.ChannelEmail, sample, 33, 0},
		{"single sms", llm.ModeSingle, llm.ChannelSMS, sample, 0, 33},
		{"fallback email primary: the sample plan", llm.ModePrimaryFallback, llm.ChannelEmail, sample, 33, 13},
		{"fallback sms primary", llm.ModePrimaryFallback, llm.ChannelSMS, sample, 13, 33},
		{"both", llm.ModeBoth, llm.ChannelEmail, sample, 33, 33},
		{"both ignores primary", llm.ModeBoth, llm.ChannelSMS, sample, 33, 33},
		{"fallback with nobody on the other channel", llm.ModePrimaryFallback, llm.ChannelEmail, Coverage{Both: 5, EmailOnly: 2}, 7, 0},
		{"fallback whose primary reaches nobody", llm.ModePrimaryFallback, llm.ChannelEmail, Coverage{SMSOnly: 4}, 0, 4},
		{"single on a channel nobody has", llm.ModeSingle, llm.ChannelSMS, Coverage{EmailOnly: 9}, 0, 0},
		{"unreachable are never counted", llm.ModeBoth, llm.ChannelEmail, Coverage{Unreachable: 9}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e, s, err := MessagesFor(tt.mode, tt.primary, tt.cov)
			if err != nil || e != tt.wantEmail || s != tt.wantSMS {
				t.Errorf("MessagesFor = %d, %d, %v; want %d, %d", e, s, err, tt.wantEmail, tt.wantSMS)
			}
		})
	}
	if _, _, err := MessagesFor("all", llm.ChannelSMS, sample); err == nil {
		t.Error("an unknown mode was accepted")
	}
	if _, _, err := MessagesFor(llm.ModeBoth, "push", sample); err == nil {
		t.Error("an unknown channel was accepted")
	}
}

// Whatever the mode, in primary_fallback every reachable person gets exactly one message.
func TestFallbackSendsOneMessagePerPerson(t *testing.T) {
	t.Parallel()
	for _, cov := range []Coverage{sample, {Both: 3}, {EmailOnly: 3}, {SMSOnly: 3, Both: 1}, {}} {
		for _, p := range []llm.Channel{llm.ChannelEmail, llm.ChannelSMS} {
			e, s, _ := MessagesFor(llm.ModePrimaryFallback, p, cov)
			if want := cov.Both + cov.EmailOnly + cov.SMSOnly; e+s != want {
				t.Errorf("%+v primary %s: %d messages for %d people", cov, p, e+s, want)
			}
		}
	}
}

func TestDefaultPrimaryTieRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cov  Coverage
		want llm.Channel
	}{
		{"tie of the sample goes to email", sample, llm.ChannelEmail},
		{"email reaches more", Coverage{Both: 1, EmailOnly: 5, SMSOnly: 2}, llm.ChannelEmail},
		{"sms reaches more", Coverage{Both: 1, EmailOnly: 2, SMSOnly: 5}, llm.ChannelSMS},
		{"nobody: email", Coverage{}, llm.ChannelEmail},
		{"only sms people", Coverage{SMSOnly: 1}, llm.ChannelSMS},
	}
	for _, tt := range tests {
		if got := DefaultPrimary(tt.cov); got != tt.want {
			t.Errorf("%s: DefaultPrimary = %s, want %s", tt.name, got, tt.want)
		}
	}
	if e, s := sample.Reach(); e != 33 || s != 33 {
		t.Errorf("Reach = %d, %d", e, s)
	}
}

func audienceOf(cov Coverage, langs ...LanguageAudience) AudienceContent {
	return AudienceContent{DSL: "city === \"Ankara\"", Count: cov.Both + cov.EmailOnly + cov.SMSOnly, Coverage: cov, ByLanguage: langs}
}

func TestNewPlanContent(t *testing.T) {
	t.Parallel()
	tr := LanguageAudience{Language: "tr", Count: 30, Coverage: Coverage{Both: 13, EmailOnly: 8, SMSOnly: 9}}
	en := LanguageAudience{Language: "en", Count: 16, Coverage: Coverage{Both: 7, EmailOnly: 5, SMSOnly: 4}}
	aud := audienceOf(Coverage{Both: 20, EmailOnly: 13, SMSOnly: 13, Unreachable: 3}, en, tr)
	got, err := NewPlanContent(llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: " tie "}, aud)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != llm.ModePrimaryFallback || got.Primary != llm.ChannelEmail || got.Rationale != "tie" {
		t.Errorf("plan = %+v", got)
	}
	if got.Recipients != (Recipients{Email: 33, SMS: 13, Total: 46}) {
		t.Errorf("recipients = %+v, want email 33 sms 13 total 46", got.Recipients)
	}
	want := []LanguageRecipients{{"en", 12, 4}, {"tr", 21, 9}}
	if len(got.ByLanguage) != 2 || got.ByLanguage[0] != want[0] || got.ByLanguage[1] != want[1] {
		t.Errorf("by_language = %+v, want %+v", got.ByLanguage, want)
	}
	var e, s int
	for _, l := range got.ByLanguage {
		e, s = e+l.Email, s+l.SMS
	}
	if e != got.Recipients.Email || s != got.Recipients.SMS {
		t.Errorf("languages sum to %d/%d, total is %d/%d", e, s, got.Recipients.Email, got.Recipients.SMS)
	}
	both, _ := NewPlanContent(llm.ChannelPlan{Mode: llm.ModeBoth, Primary: llm.ChannelEmail}, aud)
	if both.Recipients != (Recipients{Email: 33, SMS: 33, Total: 66}) {
		t.Errorf("both = %+v: total counts messages, a person on both channels counts twice", both.Recipients)
	}
	if _, err := NewPlanContent(llm.ChannelPlan{Mode: "x", Primary: llm.ChannelSMS}, aud); err == nil {
		t.Error("an invalid plan was built")
	}
}

func TestPlanProblems(t *testing.T) {
	t.Parallel()
	emailOnly := audienceOf(Coverage{EmailOnly: 6})
	smsOnly := audienceOf(Coverage{SMSOnly: 7})
	tests := []struct {
		name string
		plan llm.ChannelPlan
		aud  AudienceContent
		want []string // substrings, one per problem; nil = valid
	}{
		{"the sample plan", llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail}, audienceOf(sample), nil},
		{"fallback on sms for an sms-only audience", llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelSMS}, smsOnly, nil},
		{"both on the sample", llm.ChannelPlan{Mode: llm.ModeBoth, Primary: llm.ChannelEmail}, audienceOf(sample), nil},
		{"single email", llm.ChannelPlan{Mode: llm.ModeSingle, Primary: llm.ChannelEmail}, emailOnly, nil},
		{"unknown mode", llm.ChannelPlan{Mode: "all", Primary: llm.ChannelEmail}, audienceOf(sample), []string{`mode "all"`}},
		{"unknown primary", llm.ChannelPlan{Mode: llm.ModeBoth, Primary: "push"}, audienceOf(sample), []string{`primary "push"`}},
		{"both invalid", llm.ChannelPlan{}, audienceOf(sample), []string{"mode", "primary"}},
		{"single on a channel nobody has", llm.ChannelPlan{Mode: llm.ModeSingle, Primary: llm.ChannelSMS}, emailOnly, []string{"single sms would send to nobody"}},
		{"fallback whose primary reaches nobody", llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail}, smsOnly, []string{"primary email reaches nobody"}},
		{"both needs both channels: sms missing", llm.ChannelPlan{Mode: llm.ModeBoth, Primary: llm.ChannelEmail}, audienceOf(Coverage{EmailOnly: 5}), []string{"sms reaches nobody"}},
		{"both needs both channels: neither", llm.ChannelPlan{Mode: llm.ModeBoth, Primary: llm.ChannelEmail}, audienceOf(Coverage{}), []string{"email reaches nobody", "sms reaches nobody"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PlanProblems(tt.plan, tt.aud)
			if len(got) != len(tt.want) {
				t.Fatalf("problems = %q, want %d", got, len(tt.want))
			}
			for i, w := range tt.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("problem %d = %q, want it to contain %q", i, got[i], w)
				}
			}
		})
	}
	// The problems carry the numbers the model needs for its repair.
	got := PlanProblems(llm.ChannelPlan{Mode: llm.ModeSingle, Primary: llm.ChannelSMS}, emailOnly)
	if len(got) != 1 || !strings.Contains(got[0], "email reaches 6 people, sms reaches 0") {
		t.Errorf("problem = %q", got)
	}
}

func TestScheduleProblems(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	istanbul := time.FixedZone("+03", 3*3600)
	inZone := func(d time.Duration) *time.Time { v := now.Add(d).In(istanbul); return &v }
	entry := func(lang string, at *time.Time) llm.ScheduleEntry {
		return llm.ScheduleEntry{Language: lang, SendAt: at}
	}
	langs := []string{"tr", "en"}
	tests := []struct {
		name    string
		entries []llm.ScheduleEntry
		langs   []string
		want    []string
	}{
		{"null for both: send now", []llm.ScheduleEntry{entry("tr", nil), entry("en", nil)}, langs, nil},
		{"inside the window", []llm.ScheduleEntry{entry("tr", at(24*time.Hour)), entry("en", at(26*time.Hour))}, langs, nil},
		{"exactly now + 5 minutes", []llm.ScheduleEntry{entry("tr", at(5*time.Minute)), entry("en", nil)}, langs, nil},
		{"exactly now + 30 days", []llm.ScheduleEntry{entry("tr", at(30*24*time.Hour)), entry("en", nil)}, langs, nil},
		{"a second too soon", []llm.ScheduleEntry{entry("tr", at(5*time.Minute-time.Second)), entry("en", nil)}, langs, []string{"tr: send_at 2026-10-28T12:04:59Z is earlier than now + 5 minutes"}},
		{"in the past", []llm.ScheduleEntry{entry("tr", at(-time.Hour)), entry("en", nil)}, langs, []string{"earlier than now + 5 minutes"}},
		{"a second too late", []llm.ScheduleEntry{entry("tr", nil), entry("en", at(30*24*time.Hour+time.Second))}, langs, []string{"en: send_at 2026-11-27T12:00:01Z is later than now + 30 days"}},
		{"the window is read in UTC whatever the offset", []llm.ScheduleEntry{entry("tr", inZone(6*time.Minute)), entry("en", inZone(4*time.Minute))}, langs, []string{"en: send_at"}},
		{"a language is missing", []llm.ScheduleEntry{entry("tr", nil)}, langs, []string{`language "en" has no entry`}},
		{"no entries", nil, langs, []string{`"tr" has no entry`, `"en" has no entry`}},
		{"an unknown language", []llm.ScheduleEntry{entry("tr", nil), entry("en", nil), entry("de", nil)}, langs, []string{`language "de" is not in the audience`}},
		{"a duplicate", []llm.ScheduleEntry{entry("tr", nil), entry("tr", nil), entry("en", nil)}, langs, []string{`language "tr" has more than one entry`}},
		{"one language only", []llm.ScheduleEntry{entry("en", nil)}, []string{"en"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ScheduleProblems(tt.entries, tt.langs, now)
			if len(got) != len(tt.want) {
				t.Fatalf("problems = %q, want %d", got, len(tt.want))
			}
			for i, w := range tt.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("problem %d = %q, want it to contain %q", i, got[i], w)
				}
			}
		})
	}
}

func TestExpectedSlots(t *testing.T) {
	t.Parallel()
	plan := &PlanContent{ByLanguage: []LanguageRecipients{{"en", 0, 4}, {"tr", 21, 9}, {"de", 0, 0}}}
	tests := []struct {
		kind string
		plan *PlanContent
		want []Slot
	}{
		{KindAudience, nil, []Slot{{}}},
		{KindChannelPlan, nil, []Slot{{}}},
		// Only (language, channel) pairs with recipients (D-010): en has no email, de nobody.
		{KindContent, plan, []Slot{{"en", "sms"}, {"tr", "email"}, {"tr", "sms"}}},
		// One slot per language that receives anything (D-009).
		{KindSchedule, plan, []Slot{{Language: "en"}, {Language: "tr"}}},
	}
	for _, tt := range tests {
		got, err := expectedSlots(tt.kind, tt.plan)
		if err != nil || len(got) != len(tt.want) {
			t.Fatalf("%s: slots = %v, %v; want %v", tt.kind, got, err, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: slot %d = %v, want %v", tt.kind, i, got[i], tt.want[i])
			}
		}
	}
	for _, kind := range []string{KindContent, KindSchedule} {
		if _, err := expectedSlots(kind, nil); err == nil {
			t.Errorf("%s without a plan was accepted", kind)
		}
	}
	if _, err := expectedSlots("nope", nil); err == nil {
		t.Error("an unknown kind was accepted")
	}
	if got := scheduleLanguages([]Slot{{Language: "tr"}, {Language: "en"}}); len(got) != 2 || got[0] != "tr" || got[1] != "en" {
		t.Errorf("scheduleLanguages = %v", got)
	}
}

func variant(status, lang, channel string) store.ProposalVariant {
	return store.ProposalVariant{ID: uuid.New(), Status: status, Language: pgtype.Text{String: lang, Valid: lang != ""}, Channel: pgtype.Text{String: channel, Valid: channel != ""}}
}

func TestStepCompleteAndMissing(t *testing.T) {
	t.Parallel()
	expected := []Slot{{"tr", "email"}, {"tr", "sms"}}
	a := variant(VariantStatusApproved, "tr", "email")
	b := variant(VariantStatusApproved, "tr", "sms")
	d := variant(VariantStatusDraft, "tr", "sms")
	r := variant(VariantStatusRejected, "tr", "sms")
	tests := []struct {
		name        string
		live        []store.ProposalVariant
		wantDone    bool
		wantMissing []Slot
	}{
		{"all approved", []store.ProposalVariant{a, b}, true, nil},
		{"a draft is not enough", []store.ProposalVariant{a, d}, false, nil},
		{"a rejected slot waits for its replacement", []store.ProposalVariant{a, r}, false, []Slot{{"tr", "sms"}}},
		{"a slot with no variant yet is not done even when the others are approved", []store.ProposalVariant{a}, false, []Slot{{"tr", "sms"}}},
		{"nothing yet", nil, false, expected},
	}
	for _, tt := range tests {
		if got := stepComplete(expected, tt.live); got != tt.wantDone {
			t.Errorf("%s: stepComplete = %v, want %v", tt.name, got, tt.wantDone)
		}
		got := missingSlots(expected, tt.live)
		if len(got) != len(tt.wantMissing) {
			t.Errorf("%s: missing = %v, want %v", tt.name, got, tt.wantMissing)
		}
	}
	if stepComplete(nil, []store.ProposalVariant{a}) {
		t.Error("a step with no expected slots is never complete")
	}
}
