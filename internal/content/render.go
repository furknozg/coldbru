package content

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

// RenderOptions are the campaign-wide inputs of an email render.
type RenderOptions struct {
	// CTAURL is the link of the button, from CAMPAIGN_CTA_URL (Config.CTAURL), never from the model.
	CTAURL string
	// CampaignID becomes utm_campaign on the button link.
	CampaignID string
	// Language selects the footer text: "tr", else English.
	Language string
}

// Rendered is one email ready to hand to a provider.
type Rendered struct {
	Subject string
	HTML    string
	Text    string
}

// Footers are the unsubscribe texts added by code, never by the model (D-071). Languages without an
// entry get the English one.
var footers = map[string]string{
	"en": "You are receiving this email because you agreed to receive marketing messages from us. You can unsubscribe at any time.",
	"tr": "Bu e-postayı, pazarlama iletileri almayı kabul ettiğiniz için aldınız. Aboneliğinizi istediğiniz zaman iptal edebilirsiniz.",
}

// Footer returns the unsubscribe text of language.
func Footer(language string) string {
	if f, ok := footers[language]; ok {
		return f
	}
	return footers["en"]
}

// CTAURL returns base with utm_campaign=campaignID merged into its query: existing parameters are
// kept (an existing utm_campaign is replaced) and the query is URL-encoded. base must be an
// absolute http or https URL.
func CTAURL(base, campaignID string) (string, error) {
	if err := checkCTAURL(base); err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if campaignID != "" {
		q.Set("utm_campaign", campaignID)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// RenderSMS returns the text to send to a recipient: the stored template with {{first_name}}
// replaced by firstName. Callers measure the result again with MeasureSMS (D-028).
func RenderSMS(text, firstName string) string { return Expand(text, firstName) }

// RenderEmail renders e for one recipient. firstName is the recipient's first name at send time, or
// a sample name for a preview. Blocks are expanded by Expand and inserted into the fixed template by
// html/template, which escapes them, so neither the model's text nor a name can add markup. The plain-text alternative is
// made from the same blocks.
func RenderEmail(e Email, o RenderOptions, firstName string) (Rendered, error) {
	link, err := CTAURL(o.CTAURL, o.CampaignID)
	if err != nil {
		return Rendered{}, fmt.Errorf("content: render email: %w", err)
	}
	d := emailData{
		Lang:       "en",
		Subject:    Expand(e.Subject, firstName),
		Preheader:  Expand(e.Preheader, firstName),
		Headline:   Expand(e.Headline, firstName),
		CTALabel:   Expand(e.CTALabel, firstName),
		CTAURL:     link,
		Footer:     Footer(o.Language),
		Paragraphs: make([]string, len(e.Paragraphs)),
	}
	if o.Language == "tr" {
		d.Lang = "tr"
	}
	for i, p := range e.Paragraphs {
		d.Paragraphs[i] = Expand(p, firstName)
	}
	var html bytes.Buffer
	if err := emailTemplate.Execute(&html, d); err != nil {
		return Rendered{}, fmt.Errorf("content: render email: %w", err)
	}
	return Rendered{Subject: d.Subject, HTML: html.String(), Text: plainText(d)}, nil
}

type emailData struct {
	Lang       string
	Subject    string
	Preheader  string
	Headline   string
	Paragraphs []string
	CTALabel   string
	CTAURL     string
	Footer     string
}

func plainText(d emailData) string {
	var b strings.Builder
	b.WriteString(d.Headline + "\n\n")
	for _, p := range d.Paragraphs {
		b.WriteString(p + "\n\n")
	}
	b.WriteString(d.CTALabel + ": " + d.CTAURL + "\n\n-- \n" + d.Footer + "\n")
	return b.String()
}

// emailTemplate is the one HTML layout of every campaign email: a centred 600 px table with inline
// CSS, which is what mail clients render reliably. The preheader is a hidden line that inbox
// previews show.
var emailTemplate = template.Must(template.New("email").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Subject}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;">
<div style="display:none;font-size:1px;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;mso-hide:all;">{{.Preheader}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#f4f4f5;">
<tr><td align="center" style="padding:24px 12px;">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" style="width:100%;max-width:600px;background-color:#ffffff;border-radius:8px;">
<tr><td style="padding:32px 32px 8px 32px;font-family:Arial,Helvetica,sans-serif;font-size:24px;line-height:30px;font-weight:bold;color:#18181b;">{{.Headline}}</td></tr>
{{range .Paragraphs}}<tr><td style="padding:8px 32px;font-family:Arial,Helvetica,sans-serif;font-size:16px;line-height:24px;color:#3f3f46;">{{.}}</td></tr>
{{end}}<tr><td align="left" style="padding:16px 32px 32px 32px;">
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr><td align="center" bgcolor="#18181b" style="border-radius:6px;"><a href="{{.CTAURL}}" style="display:inline-block;padding:12px 24px;font-family:Arial,Helvetica,sans-serif;font-size:16px;line-height:20px;font-weight:bold;color:#ffffff;text-decoration:none;">{{.CTALabel}}</a></td></tr></table>
</td></tr>
<tr><td style="padding:16px 32px;border-top:1px solid #e4e4e7;font-family:Arial,Helvetica,sans-serif;font-size:12px;line-height:18px;color:#71717a;">{{.Footer}}</td></tr>
</table>
</td></tr>
</table>
</body>
</html>
`))
