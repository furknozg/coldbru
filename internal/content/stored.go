package content

// SMSContent is the content of an SMS variant as it is stored (D-034, D-071). Text is the template
// with {{first_name}} still in it; the other fields are measured with the longest first name of the
// variant's language audience and priced for the recipients of its slot. Notification renders Text
// for each recipient at send time and measures again.
type SMSContent struct {
	Text             string   `json:"text"`
	Encoding         Encoding `json:"encoding"`
	Segments         int      `json:"segments"`
	RenderedLength   int      `json:"rendered_length"`
	EstimatedCostUSD string   `json:"estimated_cost_usd"`
}

// EmailContent is the content of an email variant as it is stored. HTML is not stored: RenderEmail
// builds it from these blocks on demand. MaxRenderedLengths are measured like SMSContent's length.
type EmailContent struct {
	Subject            string       `json:"subject"`
	Preheader          string       `json:"preheader"`
	Headline           string       `json:"headline"`
	Paragraphs         []string     `json:"paragraphs"`
	CTALabel           string       `json:"cta_label"`
	MaxRenderedLengths EmailLengths `json:"max_rendered_lengths"`
	EstimatedCostUSD   string       `json:"estimated_cost_usd"`
}

// Email returns the blocks of the stored content.
func (c EmailContent) Email() Email {
	return Email{Subject: c.Subject, Preheader: c.Preheader, Headline: c.Headline, Paragraphs: c.Paragraphs, CTALabel: c.CTALabel}
}

// BuildSMS validates text and returns the content to store. maxNameLength is the longest first name
// of the audience of the slot's language, recipients the number of messages the plan sends in the
// slot. On problems the returned content is the zero value.
func BuildSMS(text string, maxNameLength, recipients int, p Prices) (SMSContent, []Problem) {
	info, problems := CheckSMS(text, maxNameLength)
	if len(problems) > 0 {
		return SMSContent{}, problems
	}
	return SMSContent{
		Text: text, Encoding: info.Encoding, Segments: info.Segments, RenderedLength: info.Length,
		EstimatedCostUSD: p.EstimateSMS(info.Segments, recipients).String(),
	}, nil
}

// BuildEmail validates the blocks and returns the content to store; see BuildSMS.
func BuildEmail(e Email, maxNameLength, recipients int, p Prices) (EmailContent, []Problem) {
	lens, problems := CheckEmail(e, maxNameLength)
	if len(problems) > 0 {
		return EmailContent{}, problems
	}
	return EmailContent{
		Subject: e.Subject, Preheader: e.Preheader, Headline: e.Headline, Paragraphs: e.Paragraphs, CTALabel: e.CTALabel,
		MaxRenderedLengths: lens, EstimatedCostUSD: p.EstimateEmail(recipients).String(),
	}, nil
}
