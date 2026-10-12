package textutil

import "testing"

func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"  Ayesha   Rahman ":       "Ayesha Rahman",
		"Line\nbreak\tand\rreturn": "Line break and return",
		"evil‮exe.txt":             "evilexe.txt",
		"zero​width":               "zerowidth",
		"Café":                     "Café",
		"আয়েশা রহমান":             "আয়েশা রহমান",
		"Family 👨‍👩‍👧 emoji":       "Family 👨‍👩‍👧 emoji",
		"\x00null\x07bell":         "nullbell",
	}
	for in, want := range cases {
		if got := CleanName(in); got != want {
			t.Errorf("CleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
