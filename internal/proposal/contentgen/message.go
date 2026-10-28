package contentgen

import (
	"bytes"
	"encoding/json"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// userMessage is the one JSON object of the user message of the content prompt (D-060, D-071): all
// call-specific data lives here, never in the system text. The audience is a summary, never customer
// rows (research 03 §7).
type userMessage struct {
	Language           string          `json:"language"`
	Channel            string          `json:"channel"`
	Brief              string          `json:"brief"`
	ContextNotes       []string        `json:"context_notes,omitempty"`
	Audience           audienceSummary `json:"audience"`
	Plan               planSummary     `json:"plan"`
	Recipients         int             `json:"recipients"`
	MaxFirstNameLength int             `json:"max_first_name_length"`
	Limits             content.Limits  `json:"limits"`
	RejectionReason    string          `json:"rejection_reason,omitempty"`
	Repair             *repair         `json:"repair,omitempty"`
}

// audienceSummary is how many people the campaign reaches in all and in the slot's language.
type audienceSummary struct {
	Count         int `json:"count"`
	LanguageCount int `json:"language_count"`
}

type planSummary struct {
	Mode    string `json:"mode"`
	Primary string `json:"primary"`
}

// repair carries the failed answer and what was wrong with it into the next attempt.
type repair struct {
	Attempt        int               `json:"attempt"`
	PreviousOutput json.RawMessage   `json:"previous_output,omitempty"`
	Problems       []content.Problem `json:"problems"`
}

// userMessage builds the user JSON of one attempt. For the first attempt previous and problems are
// empty.
func (g *Generator) userMessage(req flow.ContentRequest, sl slot, attempt int, previous json.RawMessage, problems []content.Problem) ([]byte, error) {
	m := userMessage{
		Language: req.Language, Channel: req.Channel, Brief: req.Brief, ContextNotes: req.ContextNotes,
		Audience:   audienceSummary{Count: req.Audience.Count, LanguageCount: languageCount(req)},
		Plan:       planSummary{Mode: string(req.Plan.Mode), Primary: string(req.Plan.Primary)},
		Recipients: req.Recipients, MaxFirstNameLength: sl.maxName, Limits: content.DefaultLimits(),
		RejectionReason: req.RejectionReason,
	}
	if attempt > 1 {
		m.Repair = &repair{Attempt: attempt, PreviousOutput: previous, Problems: problems}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func languageCount(req flow.ContentRequest) int {
	for _, l := range req.Audience.ByLanguage {
		if l.Language == req.Language {
			return l.Count
		}
	}
	return 0
}
