package content

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// Config keys read from the environment (D-071).
const (
	// KeyCTAURL is the link of the call-to-action button of every email.
	KeyCTAURL = "CAMPAIGN_CTA_URL"
	// KeySMSSegmentPrice is the price of one SMS segment in GBP.
	KeySMSSegmentPrice = "SMS_SEGMENT_PRICE_GBP"
	// KeyEmailPrice is the price of one email in GBP.
	KeyEmailPrice = "EMAIL_PRICE_GBP"
)

// Demo defaults of D-071. The prices are not those of a real provider.
const (
	DefaultCTAURL          = "https://shop.example.com"
	DefaultSMSSegmentPrice = "0.04"
	DefaultEmailPrice      = "0.001"
)

// Defaults returns the defaults for the platform config (config.New), merge them with the other
// service defaults.
func Defaults() map[string]any {
	return map[string]any{
		KeyCTAURL:          DefaultCTAURL,
		KeySMSSegmentPrice: DefaultSMSSegmentPrice,
		KeyEmailPrice:      DefaultEmailPrice,
	}
}

// Config is the campaign-wide setting of the content: where the email button points and what a
// message costs.
type Config struct {
	CTAURL string
	Prices Prices
}

// LoadConfig reads and validates the Config from v, a Viper built by config.New. An empty value
// falls back to the default, so a service works without the keys; a malformed one is an error that
// names the key.
func LoadConfig(v *viper.Viper) (Config, error) {
	get := func(key, def string) string {
		if s := strings.TrimSpace(v.GetString(key)); s != "" {
			return s
		}
		return def
	}
	cta := get(KeyCTAURL, DefaultCTAURL)
	if err := checkCTAURL(cta); err != nil {
		return Config{}, fmt.Errorf("%s: %w", KeyCTAURL, err)
	}
	sms, err := ParseMoney(get(KeySMSSegmentPrice, DefaultSMSSegmentPrice))
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", KeySMSSegmentPrice, err)
	}
	email, err := ParseMoney(get(KeyEmailPrice, DefaultEmailPrice))
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", KeyEmailPrice, err)
	}
	return Config{CTAURL: cta, Prices: Prices{SMSSegment: sms, Email: email}}, nil
}

// checkCTAURL accepts an absolute http or https URL with a host and no credentials.
func checkCTAURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return fmt.Errorf("%q must be an absolute http or https URL without credentials", raw)
	}
	return nil
}
