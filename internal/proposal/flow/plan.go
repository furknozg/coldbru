package flow

import (
	"fmt"
	"strings"
	"time"
	// The zone database is embedded so that the offset check of ScheduleProblems works in an image
	// without /usr/share/zoneinfo (D-076).
	_ "time/tzdata"

	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Limits of a send time, relative to now (D-070).
const (
	MinSendDelay = 5 * time.Minute
	MaxSendDelay = 30 * 24 * time.Hour
)

// Reach is how many people can be reached on each channel: the people who have both plus those
// who have only that one.
func (c Coverage) Reach() (email, sms int) {
	return c.Both + c.EmailOnly, c.Both + c.SMSOnly
}

// DefaultPrimary is the primary channel the plan prompt asks for: the channel that reaches more
// people, email on a tie (D-010). The model's answer is not forced to it; it is the rule the
// prompt states.
func DefaultPrimary(c Coverage) llm.Channel {
	email, sms := c.Reach()
	if sms > email {
		return llm.ChannelSMS
	}
	return llm.ChannelEmail
}

// MessagesFor counts the messages a plan sends on each channel for the people in c (D-010):
//
//	single(p)           p gets everyone reachable on p; the other channel gets nobody.
//	primary_fallback(p) p gets those with both channels or only p; the other channel gets those
//	                    who have only the other one.
//	both                email gets both + email_only; sms gets both + sms_only.
//
// It fails for an unknown mode or channel.
func MessagesFor(mode llm.PlanMode, primary llm.Channel, c Coverage) (email, sms int, err error) {
	if !mode.Valid() {
		return 0, 0, fmt.Errorf("unknown plan mode %q", mode)
	}
	if !primary.Valid() {
		return 0, 0, fmt.Errorf("unknown primary channel %q", primary)
	}
	reachEmail, reachSMS := c.Reach()
	switch mode {
	case llm.ModeBoth:
		return reachEmail, reachSMS, nil
	case llm.ModeSingle:
		if primary == llm.ChannelEmail {
			return reachEmail, 0, nil
		}
		return 0, reachSMS, nil
	}
	// primary_fallback
	if primary == llm.ChannelEmail {
		return reachEmail, c.SMSOnly, nil
	}
	return c.EmailOnly, reachSMS, nil
}

// NewPlanContent builds the plan variant content: the model's choice plus the recipient counts,
// computed here from the audience coverage, in total and per language.
func NewPlanContent(p llm.ChannelPlan, a AudienceContent) (PlanContent, error) {
	email, sms, err := MessagesFor(p.Mode, p.Primary, a.Coverage)
	if err != nil {
		return PlanContent{}, err
	}
	out := PlanContent{
		Mode: p.Mode, Primary: p.Primary, Rationale: strings.TrimSpace(p.Rationale),
		Recipients: Recipients{Email: email, SMS: sms, Total: email + sms},
		ByLanguage: make([]LanguageRecipients, 0, len(a.ByLanguage)),
	}
	for _, l := range a.ByLanguage {
		e, s, err := MessagesFor(p.Mode, p.Primary, l.Coverage)
		if err != nil {
			return PlanContent{}, err
		}
		out.ByLanguage = append(out.ByLanguage, LanguageRecipients{Language: l.Language, Email: e, SMS: s})
	}
	return out, nil
}

// PlanProblems validates a channel plan against the approved audience (D-070): the mode and the
// primary channel must be known, and every channel the plan sends on must reach somebody. It
// returns the problems in the words the model reads back on a repair.
func PlanProblems(p llm.ChannelPlan, a AudienceContent) []string {
	var problems []string
	if !p.Mode.Valid() {
		problems = append(problems, fmt.Sprintf("mode %q is not one of single, primary_fallback, both", p.Mode))
	}
	if !p.Primary.Valid() {
		problems = append(problems, fmt.Sprintf("primary %q is not one of sms, email", p.Primary))
	}
	if len(problems) > 0 {
		return problems
	}
	email, sms, err := MessagesFor(p.Mode, p.Primary, a.Coverage)
	if err != nil {
		return []string{err.Error()}
	}
	reachEmail, reachSMS := a.Coverage.Reach()
	table := fmt.Sprintf("email reaches %d people, sms reaches %d", reachEmail, reachSMS)
	switch p.Mode {
	case llm.ModeSingle:
		if email+sms == 0 {
			problems = append(problems, fmt.Sprintf("single %s would send to nobody (%s); choose a channel that reaches people", p.Primary, table))
		}
	case llm.ModePrimaryFallback:
		primary := email
		if p.Primary == llm.ChannelSMS {
			primary = sms
		}
		if primary == 0 {
			problems = append(problems, fmt.Sprintf("primary %s reaches nobody (%s); make the channel that reaches people the primary", p.Primary, table))
		}
	case llm.ModeBoth:
		if email == 0 {
			problems = append(problems, fmt.Sprintf("mode both sends on email, but email reaches nobody (%s)", table))
		}
		if sms == 0 {
			problems = append(problems, fmt.Sprintf("mode both sends on sms, but sms reaches nobody (%s)", table))
		}
	}
	return problems
}

// LocaleMessages is the messages a plan sends to one (language, timezone) pair (D-076), computed
// from the pair's coverage like the per-language figures of PlanContent.
type LocaleMessages struct {
	Language string
	Timezone string
	Email    int
	SMS      int
}

// Total is the number of messages to the pair.
func (m LocaleMessages) Total() int { return m.Email + m.SMS }

// MessagesByLocale counts, for every (language, timezone) pair of the audience, the messages the
// plan sends (D-010, D-076), in the order of a.ByLocale. It fails for an unknown mode or channel.
func MessagesByLocale(p PlanContent, a AudienceContent) ([]LocaleMessages, error) {
	out := make([]LocaleMessages, 0, len(a.ByLocale))
	for _, l := range a.ByLocale {
		e, s, err := MessagesFor(p.Mode, p.Primary, l.Coverage)
		if err != nil {
			return nil, err
		}
		out = append(out, LocaleMessages{Language: l.Language, Timezone: l.Timezone, Email: e, SMS: s})
	}
	return out, nil
}

// offsetString formats a UTC offset in seconds as +hh:mm.
func offsetString(sec int) string {
	sign := '+'
	if sec < 0 {
		sign, sec = '-', -sec
	}
	return fmt.Sprintf("%c%02d:%02d", sign, sec/3600, sec%3600/60)
}

// ScheduleProblems validates a schedule against the (language, timezone) pairs of the audience
// (D-070, D-076): exactly one entry per pair, a send time that is null (send immediately) or within
// [now + 5 minutes, now + 30 days], and an offset that is the one the pair's time zone has at that
// instant (daylight saving included), so that the wall-clock time the model wrote is the one the
// customers see.
func ScheduleProblems(entries []llm.ScheduleEntry, locales []llm.ScheduleLocale, now time.Time) []string {
	var problems []string
	type key struct{ language, timezone string }
	want := make(map[key]bool, len(locales))
	names := make([]string, len(locales))
	for i, l := range locales {
		want[key{l.Language, l.Timezone}] = true
		names[i] = l.Language + " / " + l.Timezone
	}
	expected := strings.Join(names, ", ")
	earliest, latest := now.Add(MinSendDelay), now.Add(MaxSendDelay)
	seen := map[key]bool{}
	for _, e := range entries {
		k := key{e.Language, e.Timezone}
		label := e.Language + " / " + e.Timezone
		switch {
		case !want[k]:
			problems = append(problems, fmt.Sprintf("locale %s is not in the audience (expected %s)", label, expected))
			continue
		case seen[k]:
			problems = append(problems, fmt.Sprintf("locale %s has more than one entry", label))
			continue
		}
		seen[k] = true
		if e.SendAt == nil {
			continue
		}
		at := e.SendAt.UTC()
		if at.Before(earliest) {
			problems = append(problems, fmt.Sprintf("%s: send_at %s is earlier than now + 5 minutes (%s); use a later time or null", label, e.SendAt.Format(time.RFC3339), earliest.Format(time.RFC3339)))
		} else if at.After(latest) {
			problems = append(problems, fmt.Sprintf("%s: send_at %s is later than now + 30 days (%s); use an earlier time", label, e.SendAt.Format(time.RFC3339), latest.Format(time.RFC3339)))
		}
		loc, err := time.LoadLocation(e.Timezone)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %q is not a time zone the server knows", label, e.Timezone))
			continue
		}
		_, wantOff := at.In(loc).Zone()
		if _, gotOff := e.SendAt.Zone(); gotOff != wantOff {
			// Keep the wall-clock time the model wrote and show it with the offset that is right.
			y, mo, d := e.SendAt.Date()
			h, mi, sec := e.SendAt.Clock()
			fixed := time.Date(y, mo, d, h, mi, sec, 0, loc)
			problems = append(problems, fmt.Sprintf("%s: send_at %s has the offset %s, but %s is at %s at that time (daylight saving included); write the same wall-clock time as %s",
				label, e.SendAt.Format(time.RFC3339), offsetString(gotOff), e.Timezone, offsetString(wantOff), fixed.Format(time.RFC3339)))
		}
	}
	for _, l := range locales {
		if !seen[key{l.Language, l.Timezone}] {
			problems = append(problems, fmt.Sprintf("locale %s / %s has no entry; return exactly one entry for each of %s", l.Language, l.Timezone, expected))
		}
	}
	return problems
}

// expectedSlots returns the slots a step needs (D-034, D-010, D-076): one for the audience and
// channel plan steps; for the content step every (language, channel) pair the plan sends to; for
// the schedule step every (language, timezone) pair of the audience the plan sends at least one
// message to. plan is required for the last two, and the audience for the schedule step.
func expectedSlots(kind string, aud *AudienceContent, plan *PlanContent) ([]Slot, error) {
	switch kind {
	case KindAudience, KindChannelPlan:
		return []Slot{{}}, nil
	case KindContent, KindSchedule:
		if plan == nil {
			return nil, fmt.Errorf("flow: the %s step needs an approved channel plan", kind)
		}
		var out []Slot
		if kind == KindSchedule {
			if aud == nil {
				return nil, fmt.Errorf("flow: the %s step needs an approved audience", kind)
			}
			if len(aud.ByLocale) == 0 {
				return nil, fmt.Errorf("flow: the approved audience has no by_locale figures (it was stored before D-076); reopen the audience step")
			}
			msgs, err := MessagesByLocale(*plan, *aud)
			if err != nil {
				return nil, err
			}
			for _, m := range msgs {
				if m.Total() > 0 {
					out = append(out, Slot{Language: m.Language, Timezone: m.Timezone})
				}
			}
			return out, nil
		}
		for _, l := range plan.ByLanguage {
			if l.Email > 0 {
				out = append(out, Slot{Language: l.Language, Channel: string(llm.ChannelEmail)})
			}
			if l.SMS > 0 {
				out = append(out, Slot{Language: l.Language, Channel: string(llm.ChannelSMS)})
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("flow: unknown step kind %q", kind)
}

// scheduleLocales are the (language, timezone) pairs of the schedule slots with the messages the
// plan sends to each, the input of the schedule call.
func scheduleLocales(slots []Slot, aud AudienceContent, plan PlanContent) ([]llm.ScheduleLocale, error) {
	msgs, err := MessagesByLocale(plan, aud)
	if err != nil {
		return nil, err
	}
	total := map[Slot]int{}
	for _, m := range msgs {
		total[Slot{Language: m.Language, Timezone: m.Timezone}] = m.Total()
	}
	out := make([]llm.ScheduleLocale, len(slots))
	for i, sl := range slots {
		out[i] = llm.ScheduleLocale{Language: sl.Language, Timezone: sl.Timezone, Recipients: total[sl]}
	}
	return out, nil
}
