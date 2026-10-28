package content

import (
	"net/url"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

const campaign = "0198a3b2-7c1d-7e00-8000-000000000001"

func opts(lang string) RenderOptions {
	return RenderOptions{CTAURL: "https://shop.example.com", CampaignID: campaign, Language: lang}
}

func TestRenderEmailStructure(t *testing.T) {
	t.Parallel()
	r, err := RenderEmail(validEmail(), opts("tr"), "Ayşe")
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject != "Sepetin seni bekliyor, Ayşe" {
		t.Errorf("subject = %q", r.Subject)
	}
	for _, want := range []string{
		`<!DOCTYPE html>`, `<html lang="tr">`, `<table role="presentation"`, `Merhaba Ayşe!`,
		`Sepetindeki ürünler hâlâ seni bekliyor.`, `%30 indirim fırsatını kaçırma.`, `Sepete dön</a>`,
		`display:none`, `%30 indirim sadece bu hafta</div>`, // the hidden preheader
		`href="https://shop.example.com?utm_campaign=` + campaign + `"`,
		Footer("tr"),
	} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("HTML lacks %q", want)
		}
	}
	// The preheader comes before the visible content.
	if strings.Index(r.HTML, "%30 indirim sadece") > strings.Index(r.HTML, "Merhaba Ayşe") {
		t.Error("the preheader is not first")
	}
	if strings.Contains(r.HTML, "{{") {
		t.Error("an unexpanded placeholder is left in the HTML")
	}
}

func TestRenderEmailEscapesEverything(t *testing.T) {
	t.Parallel()
	e := Email{
		Subject: "S", Preheader: "P", Headline: `<script>alert(1)</script>`,
		Paragraphs: []string{`<script>alert("x")</script> & <img src=x onerror=y>`, `Hi {{first_name}}`}, CTALabel: `"><a href=//evil>`,
	}
	r, err := RenderEmail(e, opts("en"), `<b>Mal'ory</b>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<script>", "<img", "<b>", "<a href=//evil>"} {
		if strings.Contains(r.HTML, bad) {
			t.Errorf("HTML contains unescaped %q:\n%s", bad, r.HTML)
		}
	}
	for _, want := range []string{"&lt;script&gt;alert(1)&lt;/script&gt;", "&amp;", "&lt;b&gt;Mal&#39;ory&lt;/b&gt;"} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("HTML lacks the escaped %q", want)
		}
	}
	// The plain-text alternative is text: it keeps the characters as they are.
	if !strings.Contains(r.Text, `<script>alert(1)</script>`) {
		t.Errorf("text = %q", r.Text)
	}
}

func TestRenderEmailPlainText(t *testing.T) {
	t.Parallel()
	r, err := RenderEmail(validEmail(), opts("en"), "Ayşe")
	if err != nil {
		t.Fatal(err)
	}
	want := "Merhaba Ayşe!\n\n" +
		"Sepetindeki ürünler hâlâ seni bekliyor.\n\n" +
		"%30 indirim fırsatını kaçırma.\n\n" +
		"Sepete dön: https://shop.example.com?utm_campaign=" + campaign + "\n\n" +
		"-- \n" + Footer("en") + "\n"
	if r.Text != want {
		t.Errorf("text =\n%q\nwant\n%q", r.Text, want)
	}
}

func TestFooterPerLanguage(t *testing.T) {
	t.Parallel()
	if !strings.Contains(Footer("tr"), "Aboneliğinizi") || !strings.Contains(Footer("en"), "unsubscribe") {
		t.Errorf("footers: %q / %q", Footer("tr"), Footer("en"))
	}
	if Footer("de") != Footer("en") || Footer("") != Footer("en") {
		t.Error("an unknown language must fall back to English")
	}
	en, _ := RenderEmail(validEmail(), opts("en"), "A")
	tr, _ := RenderEmail(validEmail(), opts("tr"), "A")
	if !strings.Contains(en.HTML, `lang="en"`) || !strings.Contains(en.HTML, Footer("en")) || strings.Contains(en.HTML, Footer("tr")) {
		t.Error("English render has the wrong footer or lang")
	}
	if !strings.Contains(tr.HTML, `lang="tr"`) || !strings.Contains(tr.HTML, Footer("tr")) {
		t.Error("Turkish render has the wrong footer or lang")
	}
}

func TestCTAURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, base, id, want string
		wantErr              bool
	}{
		{"no query", "https://shop.example.com", "c1", "https://shop.example.com?utm_campaign=c1", false},
		{"path", "https://shop.example.com/sale", "c1", "https://shop.example.com/sale?utm_campaign=c1", false},
		{"existing query kept", "https://shop.example.com/?a=1&b=x%20y", "c1", "https://shop.example.com/?a=1&b=x+y&utm_campaign=c1", false},
		{"existing utm_campaign replaced", "https://shop.example.com/?utm_campaign=old&utm_source=sms", "c1", "https://shop.example.com/?utm_campaign=c1&utm_source=sms", false},
		{"id is url-encoded", "https://shop.example.com", "a b&c=d", "https://shop.example.com?utm_campaign=a+b%26c%3Dd", false},
		{"empty id adds nothing", "https://shop.example.com/?a=1", "", "https://shop.example.com/?a=1", false},
		{"fragment kept", "https://shop.example.com/x#top", "c1", "https://shop.example.com/x?utm_campaign=c1#top", false},
		{"http allowed", "http://localhost:3000", "c1", "http://localhost:3000?utm_campaign=c1", false},
		{"javascript refused", "javascript:alert(1)", "c1", "", true},
		{"relative refused", "/sale", "c1", "", true},
		{"credentials refused", "https://u:p@shop.example.com", "c1", "", true},
		{"empty refused", "", "c1", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CTAURL(tt.base, tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if got != tt.want {
				t.Errorf("CTAURL = %q, want %q", got, tt.want)
			}
			if err == nil {
				u, perr := url.Parse(got)
				if perr != nil || (tt.id != "" && u.Query().Get("utm_campaign") != tt.id) {
					t.Errorf("round trip: %v, utm_campaign = %q", perr, u.Query().Get("utm_campaign"))
				}
			}
		})
	}
}

func TestRenderEmailRejectsABadCTA(t *testing.T) {
	t.Parallel()
	o := opts("en")
	o.CTAURL = "javascript:alert(1)"
	if _, err := RenderEmail(validEmail(), o, "A"); err == nil {
		t.Error("a javascript: CTA was rendered")
	}
}

func TestRenderSMS(t *testing.T) {
	t.Parallel()
	got := RenderSMS("Merhaba {{first_name}}!", "Ayşe")
	if got != "Merhaba Ayşe!" {
		t.Errorf("RenderSMS = %q", got)
	}
	// The measurement with the real name can differ from the preview's: ş forces UCS-2.
	if MeasureSMS(got).Encoding != UCS2 || MeasureSMS("Merhaba Ayse!").Encoding != GSM7 {
		t.Error("the encoding of the rendered text must follow the real name")
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	t.Run("defaults without any key", func(t *testing.T) {
		t.Parallel()
		cfg, err := LoadConfig(viper.New())
		if err != nil {
			t.Fatal(err)
		}
		if cfg.CTAURL != "https://shop.example.com" || cfg.Prices.SMSSegment.String() != "0.0400" || cfg.Prices.Email.String() != "0.0010" {
			t.Errorf("cfg = %+v", cfg)
		}
	})
	t.Run("defaults map", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		for k, val := range Defaults() {
			v.SetDefault(k, val)
		}
		cfg, err := LoadConfig(v)
		if err != nil || cfg.Prices.SMSSegment != 40_000 {
			t.Errorf("cfg = %+v, %v", cfg, err)
		}
	})
	t.Run("overrides", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(KeyCTAURL, "https://example.org/go?x=1")
		v.Set(KeySMSSegmentPrice, "0.055")
		v.Set(KeyEmailPrice, "0.0004")
		cfg, err := LoadConfig(v)
		if err != nil || cfg.CTAURL != "https://example.org/go?x=1" || cfg.Prices.SMSSegment != 55_000 || cfg.Prices.Email != 400 {
			t.Errorf("cfg = %+v, %v", cfg, err)
		}
	})
	for key, bad := range map[string]string{KeyCTAURL: "not a url", KeySMSSegmentPrice: "-1", KeyEmailPrice: "free"} {
		t.Run("bad "+key, func(t *testing.T) {
			t.Parallel()
			v := viper.New()
			v.Set(key, bad)
			if _, err := LoadConfig(v); err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("err = %v, want it to name %s", err, key)
			}
		})
	}
}
