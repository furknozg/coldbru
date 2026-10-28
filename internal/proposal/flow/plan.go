package flow

import (
	"fmt"
	"strings"
	"time"

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

// ScheduleProblems validates a schedule against the languages of the audience (D-070): exactly
// one entry per language, and a send time that is null (send immediately) or within
// [now + 5 minutes, now + 30 days].
func ScheduleProblems(entries []llm.ScheduleEntry, languages []string, now time.Time) []string {
	var problems []string
	want := make(map[string]bool, len(languages))
	for _, l := range languages {
		want[l] = true
	}
	earliest, latest := now.Add(MinSendDelay), now.Add(MaxSendDelay)
	seen := map[string]bool{}
	for _, e := range entries {
		switch {
		case !want[e.Language]:
			problems = append(problems, fmt.Sprintf("language %q is not in the audience (expected %s)", e.Language, strings.Join(languages, ", ")))
			continue
		case seen[e.Language]:
			problems = append(problems, fmt.Sprintf("language %q has more than one entry", e.Language))
			continue
		}
		seen[e.Language] = true
		if e.SendAt == nil {
			continue
		}
		at := e.SendAt.UTC()
		if at.Before(earliest) {
			problems = append(problems, fmt.Sprintf("%s: send_at %s is earlier than now + 5 minutes (%s); use a later time or null", e.Language, e.SendAt.Format(time.RFC3339), earliest.Format(time.RFC3339)))
		} else if at.After(latest) {
			problems = append(problems, fmt.Sprintf("%s: send_at %s is later than now + 30 days (%s); use an earlier time", e.Language, e.SendAt.Format(time.RFC3339), latest.Format(time.RFC3339)))
		}
	}
	for _, l := range languages {
		if !seen[l] {
			problems = append(problems, fmt.Sprintf("language %q has no entry; return exactly one entry for each of %s", l, strings.Join(languages, ", ")))
		}
	}
	return problems
}

// expectedSlots returns the slots a step needs (D-034, D-010): one for the audience and channel
// plan steps; for the content step every (language, channel) pair the plan sends to; for the
// schedule step every language the plan sends to. plan is required for the last two.
func expectedSlots(kind string, plan *PlanContent) ([]Slot, error) {
	switch kind {
	case KindAudience, KindChannelPlan:
		return []Slot{{}}, nil
	case KindContent, KindSchedule:
		if plan == nil {
			return nil, fmt.Errorf("flow: the %s step needs an approved channel plan", kind)
		}
		var out []Slot
		for _, l := range plan.ByLanguage {
			if kind == KindSchedule {
				if l.Email+l.SMS > 0 {
					out = append(out, Slot{Language: l.Language})
				}
				continue
			}
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

// scheduleLanguages are the languages of the schedule step's slots.
func scheduleLanguages(slots []Slot) []string {
	out := make([]string, len(slots))
	for i, s := range slots {
		out[i] = s.Language
	}
	return out
}
