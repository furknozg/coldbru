package sender

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/furknozg/coldbru/internal/content"
)

// nameFit says what became of the recipient's first name in an SMS (D-079).
type nameFit int

const (
	// nameFull: the text fits with the whole name.
	nameFull nameFit = iota
	// nameShortened: the name was cut to the longest prefix that fits.
	nameShortened
	// nameDropped: not even one character of the name fits; the placeholder was removed.
	nameDropped
)

func (f nameFit) String() string {
	switch f {
	case nameShortened:
		return "shortened"
	case nameDropped:
		return "dropped"
	}
	return "full"
}

var (
	// spaceBeforePunct finds a space the removal of a name leaves in front of punctuation.
	spaceBeforePunct = regexp.MustCompile(`\s+([,.;:!?])`)
	manySpaces       = regexp.MustCompile(`[ \t]{2,}`)
	leadingJunk      = regexp.MustCompile(`^[\s,;:.\-–—]+`)
)

// fitSMS renders template for a recipient so that the text stays within the 160 characters of the
// brief (D-028, D-079). The approved text was measured with the longest first name of the audience,
// but the real name is what goes out: if the expanded text is longer than 160 Unicode characters,
// the name is shortened to the longest prefix that fits; if not even one character fits, the name
// is removed. The result can still exceed the limit when the text without the name does; the caller
// must check that (it cannot happen for text that passed validation).
func fitSMS(template, firstName string) (string, nameFit) {
	limit := content.SMSMaxChars
	if text := content.RenderSMS(template, firstName); utf8.RuneCountInString(text) <= limit {
		return text, nameFull
	}

	base := utf8.RuneCountInString(content.Expand(template, ""))
	// Placeholders in the template: each one adds the name's length.
	placeholders := utf8.RuneCountInString(content.Expand(template, "x")) - base
	if placeholders > 0 && base < limit {
		if keep := (limit - base) / placeholders; keep >= 1 {
			name := strings.TrimSpace(string([]rune(firstName)[:min(keep, utf8.RuneCountInString(firstName))]))
			if name != "" {
				if text := content.Expand(template, name); utf8.RuneCountInString(text) <= limit {
					return text, nameShortened
				}
			}
		}
	}
	return tidyAfterDrop(content.Expand(template, "")), nameDropped
}

// tidyAfterDrop repairs the spacing and punctuation that removing a name leaves behind:
// "Merhaba , sepetin" becomes "Merhaba, sepetin" and ", sepetin" becomes "sepetin".
func tidyAfterDrop(s string) string {
	s = spaceBeforePunct.ReplaceAllString(s, "$1")
	s = manySpaces.ReplaceAllString(s, " ")
	s = leadingJunk.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
