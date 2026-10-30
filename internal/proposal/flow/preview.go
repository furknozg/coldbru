package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Sample first names of the email preview (D-090): {{first_name}} is replaced by a fixed name of the
// variant's language, never by a real customer's name.
const (
	SampleFirstNameTR = "Ayşe"
	SampleFirstNameEN = "Alex"
)

// SampleFirstName returns the sample name of language: Turkish gets SampleFirstNameTR, every other
// language SampleFirstNameEN.
func SampleFirstName(language string) string {
	if language == "tr" {
		return SampleFirstNameTR
	}
	return SampleFirstNameEN
}

// EmailPreview is a rendered email variant (D-090): the subject with the sample name in it, the
// HTML document and its plain-text alternative, as content.RenderEmail builds them for a recipient.
type EmailPreview struct {
	Subject string `json:"subject" validate:"required" example:"Sepetin seni bekliyor, Ayşe"`
	HTML    string `json:"html" validate:"required"`
	Text    string `json:"text" validate:"required"`
}

// PreviewEmail renders an email variant the way Notification sends it (D-090). With an empty
// edited, the stored blocks are rendered; otherwise edited is unsaved content in the shape Edit
// accepts for an email ({"subject","preheader","headline","paragraphs","cta_label"}), checked
// with the validators Edit applies and rendered without being stored. {{first_name}} becomes
// SampleFirstName of the variant's language; the button links to the campaign's CTA URL. Nothing is
// written. Errors: ErrNotFound; ErrNotEmail for a variant that is not an email of the content step;
// *InvalidContentError when edited does not pass the validators.
func (s *Service) PreviewEmail(ctx context.Context, variantID uuid.UUID, edited json.RawMessage) (EmailPreview, error) {
	v, st, err := s.lookup(ctx, variantID)
	if err != nil {
		return EmailPreview{}, err
	}
	if st.Kind != KindContent || v.Channel.String != string(llm.ChannelEmail) {
		return EmailPreview{}, ErrNotEmail
	}
	stored := v.Content
	if len(bytes.TrimSpace(edited)) > 0 {
		if s.content == nil {
			return EmailPreview{}, ErrNoContentGenerator
		}
		if err := checkObject(edited); err != nil {
			return EmailPreview{}, &InvalidContentError{Problems: []string{"the content must be a JSON object"}}
		}
		aud, plan, err := priorContent(ctx, s.q, st.CampaignID, st.Kind)
		if err != nil {
			return EmailPreview{}, err
		}
		slot := slotOf(v)
		stored, err = s.content.Validate(ContentRequest{
			CampaignID: st.CampaignID, StepID: st.ID, Language: slot.Language, Channel: slot.Channel,
			Audience: *aud, Plan: *plan, Recipients: plan.recipientsOf(slot),
		}, edited)
		if err != nil {
			return EmailPreview{}, &InvalidContentError{Problems: llm.Problems(err)}
		}
	}
	var ec content.EmailContent
	if err := json.Unmarshal(stored, &ec); err != nil {
		return EmailPreview{}, fmt.Errorf("flow: decode the email content of variant %s: %w", v.ID, err)
	}
	r, err := content.RenderEmail(ec.Email(), content.RenderOptions{
		CTAURL: s.cta, CampaignID: st.CampaignID.String(), Language: v.Language.String,
	}, SampleFirstName(v.Language.String))
	if err != nil {
		return EmailPreview{}, err
	}
	return EmailPreview{Subject: r.Subject, HTML: r.HTML, Text: r.Text}, nil
}
