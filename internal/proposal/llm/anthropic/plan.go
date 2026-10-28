package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// planUser is the user message of the plan call: one JSON object (D-060, D-070).
type planUser struct {
	Brief        string       `json:"brief"`
	ContextNotes []string     `json:"context_notes,omitempty"`
	Coverage     planCoverage `json:"coverage"`
}

// planCoverage is the coverage table of the audience in snake_case, the figures SQL computed.
type planCoverage struct {
	Count       int                `json:"count"`
	Both        int                `json:"both"`
	EmailOnly   int                `json:"email_only"`
	SMSOnly     int                `json:"sms_only"`
	Unreachable int                `json:"unreachable"`
	ByLanguage  []planLanguageCove `json:"by_language"`
}

type planLanguageCove struct {
	Language    string `json:"language"`
	Count       int    `json:"count"`
	Both        int    `json:"both"`
	EmailOnly   int    `json:"email_only"`
	SMSOnly     int    `json:"sms_only"`
	Unreachable int    `json:"unreachable"`
}

func coverageOf(p *customer.Preview) planCoverage {
	if p == nil {
		return planCoverage{ByLanguage: []planLanguageCove{}}
	}
	out := planCoverage{
		Count: p.Count, Both: p.Coverage.Both, EmailOnly: p.Coverage.EmailOnly,
		SMSOnly: p.Coverage.SmsOnly, Unreachable: p.Coverage.Unreachable,
		ByLanguage: make([]planLanguageCove, 0, len(p.ByLanguage)),
	}
	for _, l := range p.ByLanguage {
		out.ByLanguage = append(out.ByLanguage, planLanguageCove{
			Language: l.Language, Count: l.Count, Both: l.Coverage.Both, EmailOnly: l.Coverage.EmailOnly,
			SMSOnly: l.Coverage.SmsOnly, Unreachable: l.Coverage.Unreachable,
		})
	}
	return out
}

// ProposeChannelPlan asks Haiku 5.5 for the channel plan of an audience (D-010, D-070) and repairs
// it in the same conversation until validate accepts it: at most llm.MaxProposeAttempts calls.
func (c *Client) ProposeChannelPlan(ctx context.Context, in llm.ChannelPlanInput, validate func(llm.ChannelPlan) error) (llm.ChannelPlanResult, error) {
	user, err := marshal(planUser{Brief: in.Brief, ContextNotes: in.ContextNotes, Coverage: coverageOf(in.Coverage)})
	if err != nil {
		return llm.ChannelPlanResult{}, errorf("channel plan: encode input: %w", err)
	}
	plan, attempts, usage, err := repairLoop[llm.ChannelPlan]{
		purpose: llm.PurposeChannelPlan, pkg: c.channelPlan, maxTokens: PlanMaxTokens,
		user: string(user), decode: decodePlan, validate: validate,
	}.run(ctx, c)
	if err != nil {
		return llm.ChannelPlanResult{}, err
	}
	return llm.ChannelPlanResult{Plan: plan, Attempts: attempts, Usage: usage}, nil
}

// decodePlan reads {mode, primary, rationale}. The schema guarantees the shape; the check stays
// because the guarantee is the API's, not ours (D-069). The enums are compared case-insensitively.
func decodePlan(text string) (llm.ChannelPlan, []string) {
	var raw struct {
		Mode      *string `json:"mode"`
		Primary   *string `json:"primary"`
		Rationale *string `json:"rationale"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return llm.ChannelPlan{}, []string{fmt.Sprintf(`the answer is not the plan object {"mode","primary","rationale"}: %v`, err)}
	}
	if raw.Mode == nil || raw.Primary == nil || raw.Rationale == nil {
		return llm.ChannelPlan{}, []string{`"mode", "primary" and "rationale" are all required`}
	}
	plan := llm.ChannelPlan{
		Mode:      llm.PlanMode(strings.ToLower(strings.TrimSpace(*raw.Mode))),
		Primary:   llm.Channel(strings.ToLower(strings.TrimSpace(*raw.Primary))),
		Rationale: strings.TrimSpace(*raw.Rationale),
	}
	var problems []string
	if !plan.Mode.Valid() {
		problems = append(problems, fmt.Sprintf(`unknown mode %q; use single, primary_fallback or both`, *raw.Mode))
	}
	if !plan.Primary.Valid() {
		problems = append(problems, fmt.Sprintf(`unknown primary %q; use sms or email`, *raw.Primary))
	}
	return plan, problems
}

// scheduleUser is the user message of the schedule call (D-060, D-070).
type scheduleUser struct {
	Brief        string             `json:"brief"`
	ContextNotes []string           `json:"context_notes,omitempty"`
	Now          string             `json:"now"`
	Languages    []scheduleLanguage `json:"languages"`
}

// scheduleLanguage is a language with the zone its time is read in. NowLocal is the current
// moment in that zone, so the model does not convert UTC itself.
type scheduleLanguage struct {
	Language string `json:"language"`
	Timezone string `json:"timezone"`
	NowLocal string `json:"now_local"`
}

// ProposeSchedule asks Haiku 5.5 for one send time per language (D-009, D-070) and repairs it in
// the same conversation until validate accepts it: at most llm.MaxProposeAttempts calls.
func (c *Client) ProposeSchedule(ctx context.Context, in llm.ScheduleInput, validate func([]llm.ScheduleEntry) error) (llm.ScheduleResult, error) {
	now := in.Now.UTC()
	u := scheduleUser{
		Brief: in.Brief, ContextNotes: in.ContextNotes, Now: now.Format(time.RFC3339),
		Languages: make([]scheduleLanguage, 0, len(in.Languages)),
	}
	for _, lang := range in.Languages {
		zone := llm.TimezoneOf(lang)
		local := now
		if loc, err := time.LoadLocation(zone); err == nil {
			local = now.In(loc)
		}
		u.Languages = append(u.Languages, scheduleLanguage{Language: lang, Timezone: zone, NowLocal: local.Format(time.RFC3339)})
	}
	user, err := marshal(u)
	if err != nil {
		return llm.ScheduleResult{}, errorf("schedule: encode input: %w", err)
	}
	entries, attempts, usage, err := repairLoop[[]llm.ScheduleEntry]{
		purpose: llm.PurposeSchedule, pkg: c.schedule, maxTokens: ScheduleMaxTokens,
		user: string(user), decode: decodeSchedule, validate: validate,
	}.run(ctx, c)
	if err != nil {
		return llm.ScheduleResult{}, err
	}
	return llm.ScheduleResult{Entries: entries, Attempts: attempts, Usage: usage}, nil
}

// decodeSchedule reads {entries:[{language, send_at, rationale}]}. send_at is null or an RFC 3339
// string with an offset.
func decodeSchedule(text string) ([]llm.ScheduleEntry, []string) {
	var raw struct {
		Entries *[]struct {
			Language  *string         `json:"language"`
			SendAt    json.RawMessage `json:"send_at"`
			Rationale *string         `json:"rationale"`
		} `json:"entries"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, []string{fmt.Sprintf(`the answer is not the schedule object {"entries":[...]}: %v`, err)}
	}
	if raw.Entries == nil {
		return nil, []string{`"entries" is required`}
	}
	var (
		out      = make([]llm.ScheduleEntry, 0, len(*raw.Entries))
		problems []string
	)
	for i, e := range *raw.Entries {
		at := fmt.Sprintf("entries[%d]", i)
		if e.Language == nil || strings.TrimSpace(*e.Language) == "" {
			problems = append(problems, at+`: "language" is required`)
			continue
		}
		at = fmt.Sprintf("entries[%d] (%s)", i, *e.Language)
		if e.Rationale == nil {
			problems = append(problems, at+`: "rationale" is required`)
			continue
		}
		entry := llm.ScheduleEntry{Language: strings.TrimSpace(*e.Language), Rationale: strings.TrimSpace(*e.Rationale)}
		switch trimmed := bytes.TrimSpace(e.SendAt); {
		case len(trimmed) == 0:
			problems = append(problems, at+`: "send_at" is required; use null to send immediately`)
			continue
		case string(trimmed) == "null":
		default:
			var s string
			if err := json.Unmarshal(trimmed, &s); err != nil {
				problems = append(problems, at+`: "send_at" must be a string or null`)
				continue
			}
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
			if err != nil {
				problems = append(problems, fmt.Sprintf(`%s: send_at %q is not an RFC 3339 date and time with an offset (for example 2026-11-02T10:00:00+03:00)`, at, s))
				continue
			}
			entry.SendAt = &t
		}
		out = append(out, entry)
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return out, nil
}
