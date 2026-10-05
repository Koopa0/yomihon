package graph_test

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestSectionIDKeepsEveryUnicodeMarkCategory(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, heading, id string }{
		{"Hindi short vowel", "कि", "कि"},
		{"Hindi long vowel", "की", "की"},
		{"Hindi word", "हिन्दी", "हिन्दी"},
		{"Thai tone", "ปู่", "ปู่"},
		{"Thai vowel", "ปู", "ปู"},
		{"Thai word", "สวัสดี", "สวัสดี"},
		{"supplementary variation selector", "葛\U000e0100城", "葛\U000e0100城"},
		{"Kana combining mark", "か\u309a行", "か\u309a行"},
		{"enclosing mark", "A\u20dd", "a\u20dd"},
		{"NFC still composes", "e\u0301", "é"},
		{"punctuation still drops", "!किं_दु?", "किं-दु"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := graph.SectionID(tt.heading); got != tt.id {
				t.Errorf("SectionID(%q) = %q, want %q", tt.heading, got, tt.id)
			}
		})
	}
}

func FuzzSectionID(f *testing.F) {
	for _, seed := range []string{"कि", "की", "ปู่", "葛\U000e0100城", "か\u309a行", "A\u20dd", "e\u0301", "a_ b! Ⅲ", "", "\xff\u0301"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, heading string) {
		if len(heading) > 32<<10 {
			return
		}
		id := graph.SectionID(heading)
		if id == "" || !utf8.ValidString(id) || strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") || strings.Contains(id, "--") {
			t.Fatalf("SectionID(%q) = %q, want a valid nonempty id with single internal separators", heading, id)
		}
		for _, r := range id {
			if r != '-' && !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) {
				t.Errorf("SectionID(%q) = %q, contains disallowed character %U", heading, id, r)
			}
		}
		if second := graph.SectionID(id); second != id {
			t.Errorf("SectionID(%q) = %q, want canonical id %q unchanged", id, second, id)
		}
	})
}
