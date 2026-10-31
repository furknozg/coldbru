package content

// Channel limits of D-071 (brief: SMS 160 characters, email subject 80). Characters are Unicode
// characters, counted after {{first_name}} is replaced by the longest first name of the audience.
const (
	SMSMaxChars   = 160
	SubjectMax    = 80
	PreheaderMax  = 100
	HeadlineMax   = 80
	ParagraphsMin = 1
	ParagraphsMax = 4
	ParagraphMax  = 600
	CTALabelMax   = 30
)

// Limits are the limits above in the shape the model's user message carries them.
type Limits struct {
	SMSMaxChars   int `json:"sms_max_chars"`
	SubjectMax    int `json:"subject_max_chars"`
	PreheaderMax  int `json:"preheader_max_chars"`
	HeadlineMax   int `json:"headline_max_chars"`
	ParagraphsMin int `json:"paragraphs_min"`
	ParagraphsMax int `json:"paragraphs_max"`
	ParagraphMax  int `json:"paragraph_max_chars"`
	CTALabelMax   int `json:"cta_label_max_chars"`
}

// DefaultLimits returns the limits of D-071: the whole 160 characters for the SMS text.
func DefaultLimits() Limits { return NewLimits(SMSMaxChars) }

// NewLimits returns the limits with smsMaxChars as the length the SMS text may have. The model is
// given the text budget that is left once the tracking link is counted (SMSBudget, D-109).
func NewLimits(smsMaxChars int) Limits {
	return Limits{
		SMSMaxChars: smsMaxChars, SubjectMax: SubjectMax, PreheaderMax: PreheaderMax, HeadlineMax: HeadlineMax,
		ParagraphsMin: ParagraphsMin, ParagraphsMax: ParagraphsMax, ParagraphMax: ParagraphMax, CTALabelMax: CTALabelMax,
	}
}
