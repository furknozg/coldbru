package content

import (
	"strings"
	"unicode/utf8"
)

// Encoding is how an SMS goes over the air (D-028).
type Encoding string

// Encodings.
const (
	// GSM7 is the 7-bit default alphabet plus its extension table.
	GSM7 Encoding = "gsm7"
	// UCS2 is 16-bit text; characters outside the BMP take two code units.
	UCS2 Encoding = "ucs2"
)

// Segment capacities (docs/research/03 §5): one message holds 160 GSM-7 septets or 70 UCS-2 code
// units; in a concatenated message the user data header leaves 153 and 67.
const (
	gsmSingle, gsmMulti   = 160, 153
	ucs2Single, ucs2Multi = 70, 67
)

// gsmBasic is the GSM 03.38 default alphabet, one septet each. Ç Ö Ü ö ü are in it; ç ş Ş ğ Ğ ı İ
// are not, so one of them makes the whole message UCS-2 (research 03 §5).
const gsmBasic = "@£$¥èéùìòÇ\nØø\rÅå" + "Δ_ΦΓΛΩΠΨΣΘΞ\x1bÆæßÉ" + ` !"#¤%&'()*+,-./` + "0123456789:;<=>?" +
	"¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§" + "¿abcdefghijklmnopqrstuvwxyzäöñüà"

// gsmExtension is the extension table: each of these takes two septets (escape plus the code).
const gsmExtension = "^{}\\[~]|€"

var (
	gsmBasicSet = func() map[rune]bool {
		m := map[rune]bool{}
		for _, r := range gsmBasic {
			if r != 0x1b { // the escape code is not a character of its own
				m[r] = true
			}
		}
		return m
	}()
	gsmExtSet = func() map[rune]bool {
		m := map[rune]bool{}
		for _, r := range gsmExtension {
			m[r] = true
		}
		return m
	}()
)

// SMSInfo is the measurement of one rendered SMS text.
type SMSInfo struct {
	Encoding Encoding `json:"encoding"`
	// Units are the septets (GSM-7, extension characters count 2) or the UTF-16 code units (UCS-2,
	// an emoji counts 2) of the text.
	Units int `json:"units"`
	// Segments is the number of messages the text is sent as: 1 up to 160 septets or 70 units, else
	// the number of 153 or 67 unit parts, filled greedily without splitting a character.
	Segments int `json:"segments"`
	// Length is the number of Unicode characters, the figure the 160 limit applies to.
	Length int `json:"rendered_length"`
}

// MeasureSMS returns the encoding, segment count and length of rendered, a text that has no
// placeholders left.
func MeasureSMS(rendered string) SMSInfo {
	info := SMSInfo{Encoding: GSM7, Length: utf8.RuneCountInString(rendered)}
	sizes := make([]int, 0, info.Length)
	for _, r := range rendered {
		switch {
		case gsmBasicSet[r]:
			sizes = append(sizes, 1)
		case gsmExtSet[r]:
			sizes = append(sizes, 2)
		default:
			info.Encoding = UCS2
		}
	}
	if info.Encoding == UCS2 {
		sizes = sizes[:0]
		for _, r := range rendered {
			if r >= 0x10000 {
				sizes = append(sizes, 2) // surrogate pair
			} else {
				sizes = append(sizes, 1)
			}
		}
	}
	single, multi := gsmSingle, gsmMulti
	if info.Encoding == UCS2 {
		single, multi = ucs2Single, ucs2Multi
	}
	for _, n := range sizes {
		info.Units += n
	}
	info.Segments = segments(sizes, info.Units, single, multi)
	return info
}

// segments splits characters of the given unit sizes into messages. A character never straddles
// two segments, so a size-2 character that does not fit moves to the next one.
func segments(sizes []int, total, single, multi int) int {
	if total == 0 {
		return 0
	}
	if total <= single {
		return 1
	}
	n, used := 1, 0
	for _, s := range sizes {
		if used+s > multi {
			n++
			used = 0
		}
		used += s
	}
	return n
}

// CheckSMS validates the template text of an SMS ("sms.text") and measures it with {{first_name}}
// replaced by a name of maxNameLength characters. The limit applies to Unicode characters of the
// expanded text (D-028, D-071); it is SMSMaxChars less the room of the tracking link (SMSBudget,
// D-109). The measurement is returned even when problems are found, unless
// the text is unusable (not UTF-8).
func CheckSMS(text string, maxNameLength, limit int) (SMSInfo, []Problem) {
	const field = "sms.text"
	var problems []Problem
	if strings.TrimSpace(text) == "" {
		return SMSInfo{}, []Problem{{Field: field, Rule: RuleEmpty}}
	}
	problems = append(problems, checkPlainText(field, text, true)...)
	if !utf8.ValidString(text) {
		return SMSInfo{}, problems
	}
	info := MeasureSMS(ExpandMax(text, maxNameLength))
	if info.Length > limit {
		problems = append(problems, Problem{Field: field, Rule: RuleMaxLength, Got: info.Length, Limit: limit})
	}
	return info, problems
}
