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

// planUser is the user message of the plan call: one JSON object (D-060, D-070, D-088).
type planUser struct {
	Brief          string          `json:"brief"`
	ContextNotes   []string        `json:"context_notes,omitempty"`
	Clarifications []clarification `json:"clarifications,omitempty"`
	QuestionsLeft  int             `json:"questions_left"`
	Coverage       planCoverage    `json:"coverage"`
}

// questionOut is the "question" object of the plan and schedule answers (D-088).
type questionOut struct {
	Text    *string   `json:"text"`
	Options *[]string `json:"options"`
}

// toQuestion checks a decoded question with llm.CheckQuestion.
func (q *questionOut) toQuestion() (*llm.Question, []string) {
	if q.Text == nil {
		return nil, []string{`"question.text" is required`}
	}
	out := &llm.Question{Text: *q.Text}
	if q.Options != nil {
		out.Options = *q.Options
	}
	if problems := llm.CheckQuestion(out); len(problems) > 0 {
		return nil, problems
	}
	return out, nil
}

// summaryOf checks the summary that must come with a result (D-088).
func summaryOf(summary *string) (string, []string) {
	if summary == nil {
		return "", []string{`"summary" is required with a result`}
	}
	md, problems := llm.CheckSummary(*summary)
	return md, problems
}

// planAnswer is a decoded plan with its summary.
type planAnswer struct {
	plan    llm.ChannelPlan
	summary string
}

// scheduleAnswer is a decoded schedule with its summary.
type scheduleAnswer struct {
	entries []llm.ScheduleEntry
	summary string
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
// it in the same conversation until validate accepts it: at most llm.MaxProposeAttempts calls. An
// answer that asks the marketer a question is returned as such, when the step may still ask (D-088).
func (c *Client) ProposeChannelPlan(ctx context.Context, in llm.ChannelPlanInput, validate func(llm.ChannelPlan) error) (llm.ChannelPlanResult, error) {
	user, err := marshal(planUser{
		Brief: in.Brief, ContextNotes: in.ContextNotes, Clarifications: clarificationsOf(in.Clarifications),
		QuestionsLeft: in.QuestionsLeft, Coverage: coverageOf(in.Coverage),
	})
	if err != nil {
		return llm.ChannelPlanResult{}, errorf("channel plan: encode input: %w", err)
	}
	var check func(planAnswer) error
	if validate != nil {
		check = func(a planAnswer) error { return validate(a.plan) }
	}
	ans, q, attempts, usage, err := repairLoop[planAnswer]{
		purpose: llm.PurposeChannelPlan, pkg: c.channelPlan, maxTokens: PlanMaxTokens,
		user: string(user), decode: decodePlan, validate: check, questionsLeft: in.QuestionsLeft,
	}.run(ctx, c)
	if err != nil {
		return llm.ChannelPlanResult{}, err
	}
	return llm.ChannelPlanResult{Question: q, Plan: ans.plan, Summary: ans.summary, Attempts: attempts, Usage: usage}, nil
}

// decodePlan reads {question, plan: {mode, primary, rationale}, summary}: either a question, or a
// plan with its summary (D-088). The schema guarantees the shape; the check stays because the
// guarantee is the API's, not ours (D-069). The enums are compared case-insensitively.
func decodePlan(text string) (planAnswer, *llm.Question, []string) {
	var raw struct {
		Question *questionOut `json:"question"`
		Plan     *struct {
			Mode      *string `json:"mode"`
			Primary   *string `json:"primary"`
			Rationale *string `json:"rationale"`
		} `json:"plan"`
		Summary *string `json:"summary"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return planAnswer{}, nil, []string{fmt.Sprintf(`the answer is not the object {"question","plan","summary"}: %v`, err)}
	}
	if raw.Question != nil {
		if raw.Plan != nil {
			return planAnswer{}, nil, []string{`either ask a question or give a plan, not both: with "question" set "plan" and "summary" to null`}
		}
		q, problems := raw.Question.toQuestion()
		return planAnswer{}, q, problems
	}
	if raw.Plan == nil {
		return planAnswer{}, nil, []string{`"plan" is required when there is no question`}
	}
	if raw.Plan.Mode == nil || raw.Plan.Primary == nil || raw.Plan.Rationale == nil {
		return planAnswer{}, nil, []string{`"plan.mode", "plan.primary" and "plan.rationale" are all required`}
	}
	plan := llm.ChannelPlan{
		Mode:      llm.PlanMode(strings.ToLower(strings.TrimSpace(*raw.Plan.Mode))),
		Primary:   llm.Channel(strings.ToLower(strings.TrimSpace(*raw.Plan.Primary))),
		Rationale: strings.TrimSpace(*raw.Plan.Rationale),
	}
	var problems []string
	if !plan.Mode.Valid() {
		problems = append(problems, fmt.Sprintf(`unknown mode %q; use single, primary_fallback or both`, *raw.Plan.Mode))
	}
	if !plan.Primary.Valid() {
		problems = append(problems, fmt.Sprintf(`unknown primary %q; use sms or email`, *raw.Plan.Primary))
	}
	summary, sp := summaryOf(raw.Summary)
	problems = append(problems, sp...)
	return planAnswer{plan: plan, summary: summary}, nil, problems
}

// scheduleUser is the user message of the schedule call (D-060, D-070, D-076, D-088).
type scheduleUser struct {
	Brief          string           `json:"brief"`
	ContextNotes   []string         `json:"context_notes,omitempty"`
	Clarifications []clarification  `json:"clarifications,omitempty"`
	QuestionsLeft  int              `json:"questions_left"`
	Now            string           `json:"now"`
	Locales        []scheduleLocale `json:"locales"`
}

// scheduleLocale is a (language, timezone) pair of the audience. NowLocal is the current moment
// in that zone, so the model does not convert UTC itself.
type scheduleLocale struct {
	Language   string `json:"language"`
	Timezone   string `json:"timezone"`
	Recipients int    `json:"recipients"`
	NowLocal   string `json:"now_local"`
}

// ProposeSchedule asks Haiku 5.5 for one send time per (language, timezone) pair of the audience
// (D-009, D-070, D-076) and repairs it in the same conversation until validate accepts it: at most
// llm.MaxProposeAttempts calls.
func (c *Client) ProposeSchedule(ctx context.Context, in llm.ScheduleInput, validate func([]llm.ScheduleEntry) error) (llm.ScheduleResult, error) {
	now := in.Now.UTC()
	u := scheduleUser{
		Brief: in.Brief, ContextNotes: in.ContextNotes, Clarifications: clarificationsOf(in.Clarifications),
		QuestionsLeft: in.QuestionsLeft, Now: now.Format(time.RFC3339),
		Locales: make([]scheduleLocale, 0, len(in.Locales)),
	}
	for _, l := range in.Locales {
		local := now
		if loc, err := time.LoadLocation(l.Timezone); err == nil {
			local = now.In(loc)
		}
		u.Locales = append(u.Locales, scheduleLocale{
			Language: l.Language, Timezone: l.Timezone, Recipients: l.Recipients, NowLocal: local.Format(time.RFC3339),
		})
	}
	user, err := marshal(u)
	if err != nil {
		return llm.ScheduleResult{}, errorf("schedule: encode input: %w", err)
	}
	var check func(scheduleAnswer) error
	if validate != nil {
		check = func(a scheduleAnswer) error { return validate(a.entries) }
	}
	ans, q, attempts, usage, err := repairLoop[scheduleAnswer]{
		purpose: llm.PurposeSchedule, pkg: c.schedule, maxTokens: ScheduleMaxTokens,
		user: string(user), decode: decodeSchedule, validate: check, questionsLeft: in.QuestionsLeft,
	}.run(ctx, c)
	if err != nil {
		return llm.ScheduleResult{}, err
	}
	return llm.ScheduleResult{Question: q, Entries: ans.entries, Summary: ans.summary, Attempts: attempts, Usage: usage}, nil
}

// decodeSchedule reads {question, entries:[{language, timezone, send_at, rationale}], summary}:
// either a question, or the entries with their summary (D-088). send_at is null or an RFC 3339
// string with an offset.
func decodeSchedule(text string) (scheduleAnswer, *llm.Question, []string) {
	var raw struct {
		Question *questionOut `json:"question"`
		Entries  *[]struct {
			Language  *string         `json:"language"`
			Timezone  *string         `json:"timezone"`
			SendAt    json.RawMessage `json:"send_at"`
			Rationale *string         `json:"rationale"`
		} `json:"entries"`
		Summary *string `json:"summary"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return scheduleAnswer{}, nil, []string{fmt.Sprintf(`the answer is not the object {"question","entries","summary"}: %v`, err)}
	}
	if raw.Question != nil {
		if raw.Entries != nil {
			return scheduleAnswer{}, nil, []string{`either ask a question or give the entries, not both: with "question" set "entries" and "summary" to null`}
		}
		q, problems := raw.Question.toQuestion()
		return scheduleAnswer{}, q, problems
	}
	if raw.Entries == nil {
		return scheduleAnswer{}, nil, []string{`"entries" is required when there is no question`}
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
		if e.Timezone == nil || strings.TrimSpace(*e.Timezone) == "" {
			problems = append(problems, at+`: "timezone" is required`)
			continue
		}
		at = fmt.Sprintf("entries[%d] (%s, %s)", i, *e.Language, *e.Timezone)
		if e.Rationale == nil {
			problems = append(problems, at+`: "rationale" is required`)
			continue
		}
		entry := llm.ScheduleEntry{
			Language: strings.TrimSpace(*e.Language), Timezone: strings.TrimSpace(*e.Timezone), Rationale: strings.TrimSpace(*e.Rationale),
		}
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
	summary, sp := summaryOf(raw.Summary)
	problems = append(problems, sp...)
	if len(problems) > 0 {
		return scheduleAnswer{}, nil, problems
	}
	return scheduleAnswer{entries: out, summary: summary}, nil, nil
}
