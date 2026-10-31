package content

import (
	"strings"
	"unicode/utf8"

	"github.com/furknozg/coldbru/internal/redirect/code"
)

// LinkPath is the path prefix of the short links; the link of a delivery is <base>/r/<code> (D-106).
const LinkPath = "/r/"

// MinSMSBudget is the least room for the model's SMS text that a TRACKING_BASE_URL may leave. Below
// it no useful message fits, so the services refuse to start instead of writing one-word texts.
const MinSMSBudget = 40

// TrackingLink returns the link of the delivery whose short link code is c. base is
// TRACKING_BASE_URL; a trailing slash is ignored.
func TrackingLink(base, c string) string {
	return strings.TrimRight(base, "/") + LinkPath + c
}

// TrackingLinkLength is the length in characters of every tracking link under base: the code has a
// fixed length (code.Length).
func TrackingLinkLength(base string) int {
	return utf8.RuneCountInString(TrackingLink(base, strings.Repeat("x", code.Length)))
}

// SMSBudget is the number of characters the model's SMS text may have when the SMS ends with a
// space and the tracking link: 160 minus the link and the space (D-109). With the default base
// URL, http://localhost:8080, the link has 34 characters and the budget is 125. It can be zero or
// negative for an absurdly long base URL; LoadConfig refuses those.
func SMSBudget(base string) int { return SMSMaxChars - (TrackingLinkLength(base) + 1) }

// PlaceholderLink is a link of exactly the length of every tracking link under base, made of
// characters that are in the GSM-7 alphabet like a real code's. Proposal appends it to an SMS text
// to measure encoding and segments as they will be once Notification adds the real link (D-109).
func PlaceholderLink(base string) string { return TrackingLink(base, strings.Repeat("x", code.Length)) }

// SMSWithLink returns the text of the SMS that is sent: the rendered message, a space and the link.
func SMSWithLink(text, link string) string { return text + " " + link }
