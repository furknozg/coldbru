package content

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/redirect/code"
)

func TestTrackingLink(t *testing.T) {
	t.Parallel()
	for base, want := range map[string]string{
		"http://localhost:8080":      "http://localhost:8080/r/k3JdQ9aB7x",
		"http://localhost:8080/":     "http://localhost:8080/r/k3JdQ9aB7x",
		"https://go.example.com///":  "https://go.example.com/r/k3JdQ9aB7x",
		"https://example.com/promo/": "https://example.com/promo/r/k3JdQ9aB7x",
	} {
		if got := TrackingLink(base, "k3JdQ9aB7x"); got != want {
			t.Errorf("TrackingLink(%q) = %q, want %q", base, got, want)
		}
	}
}

func TestSMSBudgetFollowsTheLinkLength(t *testing.T) {
	t.Parallel()
	// The default base URL: 21 + len("/r/") + 10 = 34 characters, plus the space, leaves 125 (D-109).
	if got := TrackingLinkLength(DefaultTrackingBaseURL); got != 34 {
		t.Errorf("TrackingLinkLength(default) = %d, want 34", got)
	}
	if got := SMSBudget(DefaultTrackingBaseURL); got != 125 {
		t.Errorf("SMSBudget(default) = %d, want 125", got)
	}
	// The budget is computed from a real link of the shape the redirect service makes.
	signer, err := code.NewSigner("budget-test-signing-key")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := signer.New()
	link := TrackingLink(DefaultTrackingBaseURL, c)
	if got := SMSMaxChars - SMSBudget(DefaultTrackingBaseURL); got != utf8.RuneCountInString(SMSWithLink("", link)) {
		t.Errorf("the room the link takes = %d, the link and its space are %d characters", got, utf8.RuneCountInString(SMSWithLink("", link)))
	}
	if short, long := SMSBudget("https://a.co"), SMSBudget("https://a-much-longer-domain.example.com"); short <= long {
		t.Errorf("a longer base URL must leave less room: %d vs %d", short, long)
	}
	// A text at the budget plus the link is exactly one SMS of 160 characters.
	text := strings.Repeat("a", SMSBudget(DefaultTrackingBaseURL))
	if n := utf8.RuneCountInString(SMSWithLink(text, link)); n != SMSMaxChars {
		t.Errorf("a budget-long text with the link has %d characters, want %d", n, SMSMaxChars)
	}
}

func TestLoadConfigTracking(t *testing.T) {
	t.Parallel()
	t.Run("default", func(t *testing.T) {
		t.Parallel()
		cfg, err := LoadConfig(viper.New())
		if err != nil || cfg.TrackingBaseURL != "http://localhost:8080" || cfg.SMSBudget != 125 {
			t.Errorf("cfg = %+v, %v; want the default base URL and a budget of 125", cfg, err)
		}
	})
	t.Run("override and trailing slash", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(KeyTrackingBaseURL, "https://go.example.com/")
		cfg, err := LoadConfig(v)
		if err != nil || cfg.TrackingBaseURL != "https://go.example.com" || cfg.SMSBudget != SMSMaxChars-len("https://go.example.com/r/")-code.Length-1 {
			t.Errorf("cfg = %+v, %v", cfg, err)
		}
	})
	t.Run("not a URL", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(KeyTrackingBaseURL, "go.example.com")
		if _, err := LoadConfig(v); err == nil || !strings.Contains(err.Error(), KeyTrackingBaseURL) {
			t.Errorf("err = %v, want it to name %s", err, KeyTrackingBaseURL)
		}
	})
	t.Run("too long to leave room for a message", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(KeyTrackingBaseURL, "https://"+strings.Repeat("a", 100)+".example.com")
		if _, err := LoadConfig(v); err == nil || !strings.Contains(err.Error(), KeyTrackingBaseURL) {
			t.Errorf("err = %v, want it to name %s", err, KeyTrackingBaseURL)
		}
	})
}

func TestCheckSMSWithTheBudget(t *testing.T) {
	t.Parallel()
	budget := SMSBudget(DefaultTrackingBaseURL)
	at, over := strings.Repeat("a", budget), strings.Repeat("a", budget+1)
	if _, p := CheckSMS(at, 0, budget); len(p) != 0 {
		t.Errorf("a text of exactly the budget: problems = %+v", p)
	}
	_, p := CheckSMS(over, 0, budget)
	if len(p) != 1 || p[0].Rule != RuleMaxLength || p[0].Limit != budget || p[0].Got != budget+1 {
		t.Errorf("a text one over the budget: problems = %+v, want max_length with limit %d", p, budget)
	}
	// The same text is fine without a link (limit 160).
	if _, p := CheckSMS(over, 0, SMSMaxChars); len(p) != 0 {
		t.Errorf("without a link the text is within 160: problems = %+v", p)
	}
	// The name counts too: a long name uses the budget up.
	if _, p := CheckSMS("{{first_name}} "+strings.Repeat("a", budget-14), 20, budget); len(p) != 1 {
		t.Errorf("name plus text over the budget: problems = %+v", p)
	}
	if got := NewLimits(budget); got.SMSMaxChars != budget || got.SubjectMax != SubjectMax {
		t.Errorf("NewLimits(%d) = %+v", budget, got)
	}
	if DefaultLimits() != NewLimits(SMSMaxChars) {
		t.Error("DefaultLimits must be NewLimits(160)")
	}
}

func TestRenderEmailUsesTheTrackingLinkAsTheButton(t *testing.T) {
	t.Parallel()
	e := Email{Subject: "Hi", Preheader: "p", Headline: "h", Paragraphs: []string{"body"}, CTALabel: "Go"}
	link := "http://localhost:8080/r/k3JdQ9aB7x"
	o := opts("en")
	o.TrackingURL = link
	r, err := RenderEmail(e, o, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.HTML, `href="`+link+`"`) || !strings.Contains(r.Text, "Go: "+link) {
		t.Errorf("the button must point at the tracking link; html button %q, text %q", r.HTML, r.Text)
	}
	if strings.Contains(r.HTML, "shop.example.com") || strings.Contains(r.Text, "shop.example.com") {
		t.Error("the campaign page must not appear in the email: the tracking link redirects to it")
	}
	// Previews have no tracking link and show the campaign page.
	p, err := RenderEmail(e, opts("en"), "Ada")
	if err != nil || !strings.Contains(p.HTML, "https://shop.example.com?utm_campaign="+campaign) {
		t.Errorf("preview = %v, %v", p.HTML, err)
	}
	// With a tracking link the CTA URL is not even validated: it is not used.
	o.CTAURL = ""
	if _, err := RenderEmail(e, o, "Ada"); err != nil {
		t.Errorf("RenderEmail with a tracking link and no CTAURL: %v", err)
	}
}

func TestPlaceholderLinkHasTheLengthOfARealLink(t *testing.T) {
	t.Parallel()
	signer, err := code.NewSigner("placeholder-test-signing-key")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := signer.New()
	for _, base := range []string{DefaultTrackingBaseURL, "https://go.example.com/", "https://a.co"} {
		ph, real := PlaceholderLink(base), TrackingLink(base, c)
		if utf8.RuneCountInString(ph) != utf8.RuneCountInString(real) || utf8.RuneCountInString(ph) != TrackingLinkLength(base) {
			t.Errorf("PlaceholderLink(%q) = %q (%d), a real link has %d", base, ph, utf8.RuneCountInString(ph), utf8.RuneCountInString(real))
		}
		if MeasureSMS(ph).Encoding != GSM7 {
			t.Errorf("PlaceholderLink(%q) = %q is not GSM-7: it would change the encoding of every text", base, ph)
		}
	}
}

// The stored measurement is that of the SMS as sent: the text, a space and the link (D-109).
func TestBuildSMSMeasuresTheTextWithItsLink(t *testing.T) {
	t.Parallel()
	prices := Prices{SMSSegment: 40_000, Email: 1_000}
	link := PlaceholderLink(DefaultTrackingBaseURL) // 34 characters, plus the space
	budget := SMSBudget(DefaultTrackingBaseURL)

	// UCS-2 (the ş): 40 characters are one segment of 70 units alone, 75 units with the link: two.
	ucs := "Şimdi sepete dön, " + strings.Repeat("b", 22)
	alone, p := BuildSMS(ucs, 0, 10, budget, "", prices)
	if len(p) != 0 {
		t.Fatal(p)
	}
	with, p := BuildSMS(ucs, 0, 10, budget, link, prices)
	if len(p) != 0 {
		t.Fatal(p)
	}
	if alone.Encoding != UCS2 || alone.Segments != 1 || alone.EstimatedCostUSD != "0.4000" {
		t.Errorf("without the link = %+v, want ucs2, 1 segment, 0.4000", alone)
	}
	if with.Encoding != UCS2 || with.Segments != 2 || with.EstimatedCostUSD != "0.8000" {
		t.Errorf("with the link = %+v, want ucs2, 2 segments, 0.8000", with)
	}
	if with.RenderedLength != 40 || with.Text != ucs {
		t.Errorf("rendered_length = %d, want the 40 characters of the text alone", with.RenderedLength)
	}

	// GSM-7 text at the budget is one segment of exactly 160 characters with the link.
	gsm, p := BuildSMS(strings.Repeat("a", budget), 0, 10, budget, link, prices)
	if len(p) != 0 || gsm.Encoding != GSM7 || gsm.Segments != 1 || gsm.RenderedLength != budget {
		t.Errorf("gsm at the budget = %+v, %v", gsm, p)
	}

	// The name counts at its longest length in the measured text.
	named, p := BuildSMS("{{first_name}} "+strings.Repeat("ş", 24), 20, 1, budget, link, prices)
	if len(p) != 0 || named.Segments != 2 { // 20 + 1 + 24 + 35 = 80 units
		t.Errorf("named = %+v, %v; want 2 segments (80 UCS-2 units)", named, p)
	}

	// The character limit stays the budget's, link or not.
	if _, p := BuildSMS(strings.Repeat("a", budget+1), 0, 10, budget, link, prices); len(p) != 1 || p[0].Rule != RuleMaxLength || p[0].Limit != budget {
		t.Errorf("a text over the budget: problems = %+v", p)
	}
}
