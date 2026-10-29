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
	istanbul, _ := time.LoadLocation("Europe/Istanbul")
	london, _ := time.LoadLocation("Europe/London")
	inIstanbul := func(d time.Duration) *time.Time { v := now.Add(d).In(istanbul); return &v }
	inLondon := func(d time.Duration) *time.Time { v := now.Add(d).In(london); return &v }
	const ist, lon = "Europe/Istanbul", "Europe/London"
	entry := func(lang, tz string, at *time.Time) llm.ScheduleEntry {
		return llm.ScheduleEntry{Language: lang, Timezone: tz, SendAt: at}
	}
	pair := func(lang, tz string) llm.ScheduleLocale { return llm.ScheduleLocale{Language: lang, Timezone: tz} }
	// The four pairs of the seed; "tr / Europe/London" and "en / Europe/Istanbul" are the minority.
	four := []llm.ScheduleLocale{pair("tr", ist), pair("tr", lon), pair("en", ist), pair("en", lon)}
	nulls := []llm.ScheduleEntry{entry("tr", ist, nil), entry("tr", lon, nil), entry("en", ist, nil), entry("en", lon, nil)}
	with := func(i int, e llm.ScheduleEntry) []llm.ScheduleEntry {
		out := append([]llm.ScheduleEntry(nil), nulls...)
		out[i] = e
		return out
	}
	tests := []struct {
		name    string
		entries []llm.ScheduleEntry
		locales []llm.ScheduleLocale
		want    []string
	}{
		{"null for every pair: send now", nulls, four, nil},
		{"inside the window", with(0, entry("tr", ist, inIstanbul(24*time.Hour))), four, nil},
		{"exactly now + 5 minutes", with(0, entry("tr", ist, inIstanbul(5*time.Minute))), four, nil},
		{"exactly now + 30 days", with(3, entry("en", lon, inLondon(30*24*time.Hour))), four, nil},
		{"a second too soon", with(0, entry("tr", ist, inIstanbul(5*time.Minute-time.Second))), four, []string{"tr / Europe/Istanbul: send_at 2026-10-28T15:04:59+03:00 is earlier than now + 5 minutes"}},
		{"in the past", with(0, entry("tr", ist, inIstanbul(-time.Hour))), four, []string{"earlier than now + 5 minutes"}},
		{"a second too late", with(3, entry("en", lon, inLondon(30*24*time.Hour+time.Second))), four, []string{"en / Europe/London: send_at 2026-11-27T12:00:01Z is later than now + 30 days"}},
		{"the window is read in UTC whatever the offset", with(0, entry("tr", ist, inIstanbul(4*time.Minute))), four, []string{"tr / Europe/Istanbul: send_at"}},
		{"a pair is missing", nulls[:3], four, []string{"locale en / Europe/London has no entry"}},
		{"no entries", nil, four[:2], []string{"locale tr / Europe/Istanbul has no entry", "locale tr / Europe/London has no entry"}},
		{"one entry per language is not enough", []llm.ScheduleEntry{entry("tr", ist, nil), entry("en", lon, nil)}, four, []string{"locale tr / Europe/London has no entry", "locale en / Europe/Istanbul has no entry"}},
		{"an unknown pair", append(append([]llm.ScheduleEntry(nil), nulls...), entry("de", "Europe/Berlin", nil)), four, []string{"locale de / Europe/Berlin is not in the audience"}},
		{"a known language in a zone that is not in the audience", []llm.ScheduleEntry{entry("tr", lon, nil)}, []llm.ScheduleLocale{pair("tr", ist)}, []string{"locale tr / Europe/London is not in the audience", "locale tr / Europe/Istanbul has no entry"}},
		{"a duplicate", append(append([]llm.ScheduleEntry(nil), nulls...), entry("tr", ist, nil)), four, []string{"locale tr / Europe/Istanbul has more than one entry"}},
		{"one pair only", []llm.ScheduleEntry{entry("en", ist, nil)}, []llm.ScheduleLocale{pair("en", ist)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ScheduleProblems(tt.entries, tt.locales, now)
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

// The offset of send_at must be the one the pair's zone has at that instant (D-076), daylight
// saving included: Europe/London is +00:00 in winter and +01:00 in summer, Europe/Istanbul always
// +03:00.
func TestScheduleProblemsOffsets(t *testing.T) {
	t.Parallel()
	// London changes to summer time on 2026-03-29 01:00 UTC. now is 2026-03-20, so 03-25 is
	// winter time and 04-10 summer time, both inside the 30 day window.
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	parse := func(s string) *time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return &v
	}
	london := []llm.ScheduleLocale{{Language: "en", Timezone: "Europe/London"}}
	istanbul := []llm.ScheduleLocale{{Language: "tr", Timezone: "Europe/Istanbul"}}
	tests := []struct {
		name   string
		entry  llm.ScheduleEntry
		locale []llm.ScheduleLocale
		want   string // substring of the single problem, "" for none
	}{
		{"london winter +00:00", llm.ScheduleEntry{Language: "en", Timezone: "Europe/London", SendAt: parse("2026-03-25T09:00:00+00:00")}, london, ""},
		{"london winter written as Z", llm.ScheduleEntry{Language: "en", Timezone: "Europe/London", SendAt: parse("2026-03-25T09:00:00Z")}, london, ""},
		{"london winter with the summer offset", llm.ScheduleEntry{Language: "en", Timezone: "Europe/London", SendAt: parse("2026-03-25T09:00:00+01:00")}, london,
			"en / Europe/London: send_at 2026-03-25T09:00:00+01:00 has the offset +01:00, but Europe/London is at +00:00 at that time (daylight saving included); write the same wall-clock time as 2026-03-25T09:00:00Z"},
		{"london summer +01:00", llm.ScheduleEntry{Language: "en", Timezone: "Europe/London", SendAt: parse("2026-04-10T09:00:00+01:00")}, london, ""},
		{"london summer with the winter offset", llm.ScheduleEntry{Language: "en", Timezone: "Europe/London", SendAt: parse("2026-04-10T09:00:00+00:00")}, london,
			"has the offset +00:00, but Europe/London is at +01:00 at that time"},
		{"istanbul +03:00", llm.ScheduleEntry{Language: "tr", Timezone: "Europe/Istanbul", SendAt: parse("2026-03-25T09:00:00+03:00")}, istanbul, ""},
		{"istanbul in summer is still +03:00", llm.ScheduleEntry{Language: "tr", Timezone: "Europe/Istanbul", SendAt: parse("2026-04-10T09:00:00+03:00")}, istanbul, ""},
		{"istanbul with London's offset", llm.ScheduleEntry{Language: "tr", Timezone: "Europe/Istanbul", SendAt: parse("2026-03-25T09:00:00+00:00")}, istanbul,
			"has the offset +00:00, but Europe/Istanbul is at +03:00 at that time"},
		{"a right instant with another zone's offset is wrong too", llm.ScheduleEntry{Language: "tr", Timezone: "Europe/Istanbul", SendAt: parse("2026-03-25T06:00:00Z")}, istanbul,
			"has the offset +00:00, but Europe/Istanbul is at +03:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ScheduleProblems([]llm.ScheduleEntry{tt.entry}, tt.locale, now)
			switch {
			case tt.want == "" && len(got) != 0:
				t.Errorf("problems = %q, want none", got)
			case tt.want != "" && (len(got) != 1 || !strings.Contains(got[0], tt.want)):
				t.Errorf("problems = %q, want one containing %q", got, tt.want)
			}
		})
	}

	// The changeover itself: 2026-03-29 01:00 UTC is when London moves to +01:00.
	before := parse("2026-03-29T00:59:00+00:00")
	after := parse("2026-03-29T02:00:00+01:00")
	for _, at := range []*time.Time{before, after} {
		if got := ScheduleProblems([]llm.ScheduleEntry{{Language: "en", Timezone: "Europe/London", SendAt: at}}, london, now); len(got) != 0 {
			t.Errorf("%s around the changeover: %q", at.Format(time.RFC3339), got)
		}
	}
	if got := ScheduleProblems([]llm.ScheduleEntry{{Language: "en", Timezone: "Europe/London", SendAt: parse("2026-03-29T02:00:00+00:00")}}, london, now); len(got) != 1 {
		t.Errorf("02:00+00:00 on the changeover day is already summer time: %q", got)
	}
}

func TestExpectedSlots(t *testing.T) {
	t.Parallel()
	plan := &PlanContent{
		Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail,
		ByLanguage: []LanguageRecipients{{"en", 0, 4}, {"tr", 21, 9}, {"de", 0, 0}},
	}
	// The sample brief's audience is all in Istanbul; the unreachable-only pair has no slot. The
	// plan is primary_fallback with email primary: sms goes to the people who have only sms.
	aud := &AudienceContent{ByLocale: []LocaleAudience{
		{Language: "en", Timezone: "Europe/Istanbul", Count: 4, Coverage: Coverage{SMSOnly: 4}},
		{Language: "en", Timezone: "Europe/London", Count: 0, Coverage: Coverage{Unreachable: 2}},
		{Language: "tr", Timezone: "Europe/Istanbul", Count: 30, Coverage: Coverage{Both: 15, EmailOnly: 6, SMSOnly: 9}},
		{Language: "tr", Timezone: "Europe/London", Count: 1, Coverage: Coverage{EmailOnly: 1}},
	}}
	tests := []struct {
		kind string
		want []Slot
	}{
		{KindAudience, []Slot{{}}},
		{KindChannelPlan, []Slot{{}}},
		// Only (language, channel) pairs with recipients (D-010): en has no email, de nobody.
		{KindContent, []Slot{{"en", "sms", ""}, {"tr", "email", ""}, {"tr", "sms", ""}}},
		// One slot per (language, timezone) pair that receives anything (D-076).
		{KindSchedule, []Slot{{"en", "", "Europe/Istanbul"}, {"tr", "", "Europe/Istanbul"}, {"tr", "", "Europe/London"}}},
	}
	for _, tt := range tests {
		got, err := expectedSlots(tt.kind, aud, plan)
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
		if _, err := expectedSlots(kind, aud, nil); err == nil {
			t.Errorf("%s without a plan was accepted", kind)
		}
	}
	if _, err := expectedSlots(KindSchedule, nil, plan); err == nil {
		t.Error("the schedule step without an audience was accepted")
	}
	if _, err := expectedSlots(KindSchedule, &AudienceContent{}, plan); err == nil || !strings.Contains(err.Error(), "by_locale") {
		t.Errorf("an audience without by_locale: err = %v", err)
	}
	if _, err := expectedSlots("nope", aud, plan); err == nil {
		t.Error("an unknown kind was accepted")
	}
}

// The messages per (language, timezone) pair follow the per-language math of the plan (D-010,
// D-076) and add up to the plan's totals.
func TestMessagesByLocale(t *testing.T) {
	t.Parallel()
	aud := AudienceContent{ByLocale: []LocaleAudience{
		{Language: "en", Timezone: "Europe/Istanbul", Coverage: Coverage{Both: 1, EmailOnly: 2, SMSOnly: 3, Unreachable: 4}},
		{Language: "en", Timezone: "Europe/London", Coverage: Coverage{Unreachable: 5}},
		{Language: "tr", Timezone: "Europe/Istanbul", Coverage: Coverage{Both: 10, EmailOnly: 6, SMSOnly: 7}},
	}}
	tests := []struct {
		name string
		plan PlanContent
		want []LocaleMessages
	}{
		{"fallback email primary", PlanContent{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail}, []LocaleMessages{
			{"en", "Europe/Istanbul", 3, 3}, {"en", "Europe/London", 0, 0}, {"tr", "Europe/Istanbul", 16, 7}}},
		{"single sms", PlanContent{Mode: llm.ModeSingle, Primary: llm.ChannelSMS}, []LocaleMessages{
			{"en", "Europe/Istanbul", 0, 4}, {"en", "Europe/London", 0, 0}, {"tr", "Europe/Istanbul", 0, 17}}},
		{"both", PlanContent{Mode: llm.ModeBoth, Primary: llm.ChannelEmail}, []LocaleMessages{
			{"en", "Europe/Istanbul", 3, 4}, {"en", "Europe/London", 0, 0}, {"tr", "Europe/Istanbul", 16, 17}}},
	}
	for _, tt := range tests {
		got, err := MessagesByLocale(tt.plan, aud)
		if err != nil || len(got) != len(tt.want) {
			t.Fatalf("%s: %v, %v", tt.name, got, err)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: pair %d = %+v, want %+v", tt.name, i, got[i], tt.want[i])
			}
		}
		// Per-pair figures add up to the totals NewPlanContent computes from the whole coverage.
		var email, sms int
		for _, m := range got {
			email, sms = email+m.Email, sms+m.SMS
		}
		var total Coverage
		for _, l := range aud.ByLocale {
			total.Both += l.Coverage.Both
			total.EmailOnly += l.Coverage.EmailOnly
			total.SMSOnly += l.Coverage.SMSOnly
		}
		if e, s, _ := MessagesFor(tt.plan.Mode, tt.plan.Primary, total); e != email || s != sms {
			t.Errorf("%s: pairs sum to %d/%d, the whole audience gives %d/%d", tt.name, email, sms, e, s)
		}
	}
	if _, err := MessagesByLocale(PlanContent{Mode: "x", Primary: llm.ChannelSMS}, aud); err == nil {
		t.Error("an unknown mode was accepted")
	}

	// scheduleLocales hands the model the pair and its message count.
	plan := PlanContent{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail}
	locales, err := scheduleLocales([]Slot{{Language: "tr", Timezone: "Europe/Istanbul"}, {Language: "en", Timezone: "Europe/Istanbul"}}, aud, plan)
	want := []llm.ScheduleLocale{{Language: "tr", Timezone: "Europe/Istanbul", Recipients: 23}, {Language: "en", Timezone: "Europe/Istanbul", Recipients: 6}}
	if err != nil || len(locales) != 2 || locales[0] != want[0] || locales[1] != want[1] {
		t.Errorf("scheduleLocales = %+v, %v; want %+v", locales, err, want)
	}
}

func variant(status, lang, channel string) store.ProposalVariant {
	return store.ProposalVariant{ID: uuid.New(), Status: status, Language: pgtype.Text{String: lang, Valid: lang != ""}, Channel: pgtype.Text{String: channel, Valid: channel != ""}}
}

func TestStepCompleteAndMissing(t *testing.T) {
	t.Parallel()
	expected := []Slot{{"tr", "email", ""}, {"tr", "sms", ""}}
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
		{"a rejected slot waits for its replacement", []store.ProposalVariant{a, r}, false, []Slot{{"tr", "sms", ""}}},
		{"a slot with no variant yet is not done even when the others are approved", []store.ProposalVariant{a}, false, []Slot{{"tr", "sms", ""}}},
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
