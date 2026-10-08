package graph_test

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestSectionIDKeepsTheMarksWrittenOnALetter(t *testing.T) {
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
		{"enclosing mark", "A\u20dd", "a"},
		{"emoji presentation before words", "\u26a0\ufe0f Warning", "warning"},
		{"emoji presentation alone", "\u2764\ufe0f", "section"},
		{"keycap", "1\ufe0f\u20e3 Step one", "1-step-one"},
		{"text presentation", "a\ufe0e b", "a-b"},
		{"mark on a dropped symbol", "\u2192\u0301 x", "x"},
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
	f.Add("Y\u030a0000000000")
	for _, seed := range []string{"कि", "की", "ปู่", "葛\U000e0100城", "か\u309a行", "A\u20dd", "e\u0301", "a_ b! Ⅲ", "\u26a0\ufe0f Warning", "1\ufe0f\u20e3 Step one", "", "\xff\u0301"} {
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
		previous := '-'
		for _, r := range id {
			if r != '-' && !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) {
				t.Errorf("SectionID(%q) = %q, contains disallowed character %U", heading, id, r)
			}
			if unicode.IsMark(r) && (previous == '-' || unicode.Is(unicode.Me, r) || r == '\ufe0e' || r == '\ufe0f') {
				t.Errorf("SectionID(%q) = %q, keeps mark %U that belongs to nothing kept", heading, id, r)
			}
			previous = r
		}
		if second := graph.SectionID(id); second != id {
			t.Errorf("SectionID(%q) = %q, want canonical id %q unchanged", id, second, id)
		}
	})
}

func TestFragmentNormalizationReturnsCanonicalIDs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, input, want string }{
		{"uppercase ring", "Y\u030a", "\u1e99"},
		{"lowercase ring", "y\u030a", "\u1e99"},
		{"precomposed ring", "\u1e99", "\u1e99"},
		{"uppercase digits", "Y\u030a0000000000", "\u1e990000000000"},
		{"lowercase digits", "y\u030a0000000000", "\u1e990000000000"},
		{"precomposed digits", "\u1e990000000000", "\u1e990000000000"},
		{"H line", "H\u0331", "\u1e96"},
		{"T diaeresis", "T\u0308", "\u1e97"},
		{"W ring", "W\u030a", "\u1e98"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, owner := range []struct {
				name string
				fold func(string) string
			}{
				{"FoldFragment", graph.FoldFragment},
				{"SectionID", graph.SectionID},
			} {
				got := owner.fold(tt.input)
				if got != tt.want || !norm.NFC.IsNormalString(got) {
					t.Errorf("caught: fragment-normalization %s(%q) = %q, want NFC %q", owner.name, tt.input, got, tt.want)
				}
				if rebuilt := owner.fold(got); rebuilt != got {
					t.Errorf("caught: fragment-normalization %s reread(%q) = %q, want unchanged", owner.name, got, rebuilt)
				}
			}
		})
	}
}

func TestFragmentNormalizationPreservesPolicy(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, input, fold, section string }{
		{"dotted I", "\u0130", "i", "i"},
		{"decomposed dotted I", "I\u0307", "i", "i"},
		{"simple sigma", "\u03a3", "\u03c3", "\u03c3"},
		{"final sigma", "\u03c2", "\u03c2", "\u03c2"},
		{"sharp s", "\u00df", "\u00df", "\u00df"},
		{"fullwidth", "\uff39", "\uff59", "\uff59"},
		{"punctuation", "Y\u030a!Y\u030a", "\u1e99!\u1e99", "\u1e99-\u1e99"},
		{"orphan mark", "\u030aY\u030a", "\u030a\u1e99", "\u1e99"},
		{"discarded mark owner", "\u2192\u030aY", "\u2192\u030ay", "y"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := graph.FoldFragment(tt.input); got != tt.fold {
				t.Errorf("caught: fragment-normalization FoldFragment(%q) = %q, want %q", tt.input, got, tt.fold)
			}
			if got := graph.SectionID(tt.input); got != tt.section {
				t.Errorf("caught: fragment-normalization SectionID(%q) = %q, want %q", tt.input, got, tt.section)
			}
		})
	}
	t.Run("pure caret transform", func(t *testing.T) {
		t.Parallel()
		if got := graph.FoldFragment("^Y\u030a-1"); got != "^\u1e99-1" {
			t.Errorf("caught: fragment-normalization FoldFragment caret string = %q, want %q", got, "^\u1e99-1")
		}
	})
}
