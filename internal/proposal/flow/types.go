package flow

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Step kinds, in the order the steps open (D-033).
const (
	KindAudience    = "audience"
	KindChannelPlan = "channel_plan"
	KindContent     = "content"
	KindSchedule    = "schedule"
)

var kindOrder = []string{KindAudience, KindChannelPlan, KindContent, KindSchedule}

// kindIndex returns the position of kind in the step order, -1 for an unknown kind.
func kindIndex(kind string) int {
	for i, k := range kindOrder {
		if k == kind {
			return i
		}
	}
	return -1
}

// Campaign, step and variant statuses (D-033, D-068).
const (
	CampaignStatusDrafting  = "drafting"
	CampaignStatusScheduled = "scheduled"
	CampaignStatusSending   = "sending"
	CampaignStatusCompleted = "completed"
	CampaignStatusCancelled = "cancelled"

	StepStatusOpen       = "open"
	StepStatusApproved   = "approved"
	StepStatusSuperseded = "superseded"

	VariantStatusDraft      = "draft"
	VariantStatusApproved   = "approved"
	VariantStatusRejected   = "rejected"
	VariantStatusSuperseded = "superseded"
)

// Slot is the place of a variant in its step: the language, channel and timezone columns of
// proposal_variant (D-034, D-076). The audience and channel plan steps have one slot with all
// three empty, the content step one per (language, channel), the schedule step one per
// (language, timezone).
type Slot struct {
	Language string `json:"language,omitempty"`
	Channel  string `json:"channel,omitempty" enums:"sms,email"`
	// Timezone is the IANA time zone of a schedule slot, for example Europe/Istanbul; empty for
	// every other step.
	Timezone string `json:"timezone,omitempty"`
}

func (s Slot) String() string {
	out := s.Language
	if out == "" && s.Channel == "" && s.Timezone == "" {
		return "-"
	}
	if s.Channel != "" {
		out += "/" + s.Channel
	}
	if s.Timezone != "" {
		out += "@" + s.Timezone
	}
	return out
}

// Coverage counts customers by the channels they can be reached on (D-010), in the snake_case of
// the variant content.
type Coverage struct {
	Both        int `json:"both"`
	EmailOnly   int `json:"email_only"`
	SMSOnly     int `json:"sms_only"`
	Unreachable int `json:"unreachable"`
}

// LanguageAudience is the audience figures of one language (D-009).
type LanguageAudience struct {
	Language string   `json:"language"`
	Count    int      `json:"count"`
	Coverage Coverage `json:"coverage"`
	// MaxFirstNameLength is the longest first name, in Unicode characters, among the reachable
	// customers of this language; 0 when there are none (D-071). The texts of this language are
	// measured with it.
	MaxFirstNameLength int `json:"max_first_name_length"`
}

// LocaleAudience is the audience figures of one (language, timezone) pair (D-076). The schedule
// step opens one slot for every pair the plan sends a message to.
type LocaleAudience struct {
	Language string   `json:"language"`
	Timezone string   `json:"timezone"`
	Count    int      `json:"count"`
	Coverage Coverage `json:"coverage"`
	// MaxFirstNameLength is the longest first name, in Unicode characters, among the reachable
	// customers of this pair; 0 when there are none.
	MaxFirstNameLength int `json:"max_first_name_length"`
}

// AudienceContent is the content of the audience variant (D-034, D-070, D-076): the DSL the model
// wrote, the compiled GraphQL filter, and the audience size and coverage Customer computed.
type AudienceContent struct {
	DSL        string             `json:"dsl"`
	Filter     json.RawMessage    `json:"filter" swaggertype:"object"`
	Count      int                `json:"count"`
	Coverage   Coverage           `json:"coverage"`
	ByLanguage []LanguageAudience `json:"by_language"`
	// ByLocale is the same figures per (language, timezone), ordered by language then timezone
	// (D-076). A variant stored before D-076 has none.
	ByLocale []LocaleAudience `json:"by_locale"`
	// MaxFirstNameLength is the longest first name among all reachable customers (D-071); it
	// serves languages that have no entry of their own.
	MaxFirstNameLength int `json:"max_first_name_length"`
}

func coverageFrom(c customer.Coverage) Coverage {
	return Coverage{Both: c.Both, EmailOnly: c.EmailOnly, SMSOnly: c.SmsOnly, Unreachable: c.Unreachable}
}

func (c Coverage) toCustomer() customer.Coverage {
	return customer.Coverage{Both: c.Both, EmailOnly: c.EmailOnly, SmsOnly: c.SMSOnly, Unreachable: c.Unreachable}
}

// NewAudienceContent builds the audience variant content from an accepted filter and its preview.
// Languages are sorted so that the content is deterministic.
func NewAudienceContent(dslSrc string, f *compile.Filter, p *customer.Preview) (AudienceContent, error) {
	if p == nil {
		return AudienceContent{}, fmt.Errorf("flow: audience without a preview")
	}
	filter, err := json.Marshal(f) // a nil *Filter encodes as null
	if err != nil {
		return AudienceContent{}, fmt.Errorf("flow: encode filter: %w", err)
	}
	out := AudienceContent{
		DSL: dslSrc, Filter: filter, Count: p.Count, Coverage: coverageFrom(p.Coverage),
		ByLanguage: make([]LanguageAudience, 0, len(p.ByLanguage)), ByLocale: make([]LocaleAudience, 0, len(p.ByLocale)),
		MaxFirstNameLength: p.MaxFirstNameLength,
	}
	for _, l := range p.ByLanguage {
		out.ByLanguage = append(out.ByLanguage, LanguageAudience{
			Language: l.Language, Count: l.Count, Coverage: coverageFrom(l.Coverage), MaxFirstNameLength: l.MaxFirstNameLength,
		})
	}
	for _, l := range p.ByLocale {
		out.ByLocale = append(out.ByLocale, LocaleAudience{
			Language: l.Language, Timezone: l.Timezone, Count: l.Count, Coverage: coverageFrom(l.Coverage), MaxFirstNameLength: l.MaxFirstNameLength,
		})
	}
	sort.Slice(out.ByLanguage, func(i, j int) bool { return out.ByLanguage[i].Language < out.ByLanguage[j].Language })
	sort.Slice(out.ByLocale, func(i, j int) bool {
		a, b := out.ByLocale[i], out.ByLocale[j]
		return a.Language < b.Language || a.Language == b.Language && a.Timezone < b.Timezone
	})
	return out, nil
}

// Preview returns the audience as the coverage table the plan prompt receives.
func (a AudienceContent) Preview() *customer.Preview {
	p := &customer.Preview{Count: a.Count, Coverage: a.Coverage.toCustomer(), MaxFirstNameLength: a.MaxFirstNameLength}
	for _, l := range a.ByLanguage {
		p.ByLanguage = append(p.ByLanguage, customer.LanguageCoverage{
			Language: l.Language, Count: l.Count, Coverage: l.Coverage.toCustomer(), MaxFirstNameLength: l.MaxFirstNameLength,
		})
	}
	for _, l := range a.ByLocale {
		p.ByLocale = append(p.ByLocale, customer.LocaleCoverage{
			Language: l.Language, Timezone: l.Timezone, Count: l.Count, Coverage: l.Coverage.toCustomer(), MaxFirstNameLength: l.MaxFirstNameLength,
		})
	}
	return p
}

// MaxFirstNameLengthOf returns the longest first name of the audience of language, the figure the
// texts of that language are measured with (D-071). A language without an entry gets the overall
// figure.
func (a AudienceContent) MaxFirstNameLengthOf(language string) int {
	for _, l := range a.ByLanguage {
		if l.Language == language {
			return l.MaxFirstNameLength
		}
	}
	return a.MaxFirstNameLength
}

// Recipients counts the messages a plan sends: per channel, and in total. For mode both a person
// reachable on both channels counts twice, in Total as well.
type Recipients struct {
	Email int `json:"email"`
	SMS   int `json:"sms"`
	Total int `json:"total"`
}

// LanguageRecipients is the messages per channel in one language.
type LanguageRecipients struct {
	Language string `json:"language"`
	Email    int    `json:"email"`
	SMS      int    `json:"sms"`
}

// PlanContent is the content of the channel plan variant (D-010, D-034, D-070). The recipient
// counts are computed in code from the audience coverage and the plan, never by the model.
type PlanContent struct {
	Mode       llm.PlanMode         `json:"mode"`
	Primary    llm.Channel          `json:"primary"`
	Rationale  string               `json:"rationale"`
	Recipients Recipients           `json:"recipients"`
	ByLanguage []LanguageRecipients `json:"by_language"`
}

// ScheduleContent is the content of a schedule variant (D-009). SendAt is nil for "send
// immediately". The (language, timezone) it belongs to is the variant's slot, not part of the
// content (D-076).
type ScheduleContent struct {
	SendAt    *time.Time `json:"send_at"`
	Rationale string     `json:"rationale"`
}

// CampaignInfo is a campaign row.
type CampaignInfo struct {
	ID        uuid.UUID `json:"id" validate:"required" format:"uuid"`
	Brief     string    `json:"brief" validate:"required"`
	Status    string    `json:"status" validate:"required" enums:"drafting,scheduled,sending,completed,cancelled"`
	CreatedAt time.Time `json:"created_at" validate:"required" format:"date-time"`
	UpdatedAt time.Time `json:"updated_at" validate:"required" format:"date-time"`
}

// StepInfo is a proposal_step row.
type StepInfo struct {
	ID         uuid.UUID  `json:"id" validate:"required" format:"uuid"`
	CampaignID uuid.UUID  `json:"campaign_id" validate:"required" format:"uuid"`
	Kind       string     `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
	Version    int        `json:"version" validate:"required"`
	Status     string     `json:"status" validate:"required" enums:"open,approved,superseded"`
	CreatedAt  time.Time  `json:"created_at" validate:"required" format:"date-time"`
	ApprovedAt *time.Time `json:"approved_at,omitempty" format:"date-time"`
}

// VariantView is a proposal_variant row.
type VariantView struct {
	ID              uuid.UUID       `json:"id" validate:"required" format:"uuid"`
	StepID          uuid.UUID       `json:"step_id" validate:"required" format:"uuid"`
	Kind            string          `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
	Language        string          `json:"language,omitempty"`
	Channel         string          `json:"channel,omitempty" enums:"sms,email"`
	Timezone        string          `json:"timezone,omitempty"` // schedule variants only (D-076)
	Status          string          `json:"status" validate:"required" enums:"draft,approved,rejected,superseded"`
	Content         json.RawMessage `json:"content" swaggertype:"object" validate:"required"`
	RejectionReason string          `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time       `json:"created_at" validate:"required" format:"date-time"`
	ApprovedAt      *time.Time      `json:"approved_at,omitempty" format:"date-time"`
	RejectedAt      *time.Time      `json:"rejected_at,omitempty" format:"date-time"`
}

// NoteView is a context_note row.
type NoteView struct {
	ID        uuid.UUID  `json:"id" validate:"required" format:"uuid"`
	StepID    *uuid.UUID `json:"step_id,omitempty" format:"uuid"`
	Body      string     `json:"body" validate:"required"`
	CreatedAt time.Time  `json:"created_at" validate:"required" format:"date-time"`
}

// StepView is a current step with its live variants and the state of its generation.
type StepView struct {
	StepInfo
	// Variants are the draft, approved and rejected variants, oldest first.
	Variants []VariantView `json:"variants" validate:"required"`
	// Generating: a generation of this step is queued or running in this process.
	Generating bool `json:"generating" validate:"required"`
	// Failed: the last generation of the step failed (a refusal, an exhausted repair loop, an
	// API error), and Reason says why. Regenerate retries it.
	Failed bool   `json:"failed" validate:"required"`
	Reason string `json:"failure_reason,omitempty"`
	// Missing are the slots of an open step that have no draft or approved variant yet.
	Missing []Slot `json:"missing,omitempty"`
}

// CampaignView is the read model of a campaign (Get).
type CampaignView struct {
	CampaignInfo
	// Steps are the current steps, in step order: everything that is not superseded.
	Steps []StepView `json:"steps" validate:"required"`
	// Notes are all context notes, oldest first.
	Notes []NoteView `json:"notes" validate:"required"`
	// Cost is the estimated cost of the live content variants (D-071).
	Cost CostSummary `json:"cost" validate:"required"`
}

// CostSummary adds up the estimated_cost_gbp of the draft and approved variants of the current
// content step (D-071): per channel and in total, as decimal strings with four decimals. Rejected
// and superseded variants are not counted. A variant without an estimate counts as zero. The figures
// are estimates at the configured demo prices.
type CostSummary struct {
	SMSGBP   string `json:"sms_gbp" validate:"required"`
	EmailGBP string `json:"email_gbp" validate:"required"`
	TotalGBP string `json:"total_gbp" validate:"required"`
}
