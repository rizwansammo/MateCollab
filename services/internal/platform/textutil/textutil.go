// Package textutil normalises user-entered names (SECURITY-BASELINE.md §7): NFC
// normalisation, trimmed and collapsed whitespace, and no control or bidirectional-override
// characters (which can disguise names and file extensions).
package textutil

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// isBidiControl reports the explicit directional formatting characters (U+202A–U+202E,
// U+2066–U+2069) and marks that are abused to reorder displayed text.
func isBidiControl(r rune) bool {
	return (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) || r == 0x200E || r == 0x200F || r == 0x061C
}

// CleanName returns a display-safe single-line name.
func CleanName(s string) string {
	s = norm.NFC.String(s)
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case isBidiControl(r) || r == utf8.RuneError:
			continue
		case unicode.IsSpace(r):
			space = true
			continue
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) && r != 0x200D && r != 0x200C: // keep ZWJ/ZWNJ (emoji, Indic and Arabic scripts)
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// Length is the length in characters.
func Length(s string) int { return utf8.RuneCountInString(s) }
