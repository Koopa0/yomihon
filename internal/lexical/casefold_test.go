package lexical

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"
)

// TestFold pins what one folded spelling is: a fullwidth ASCII character meets
// its halfwidth counterpart, a letter meets its simple case orbit, and the two
// compositions of one character meet. Nothing else does: not a full case fold,
// so ss never finds ß, not a compatibility fold, so halfwidth katakana stays
// halfwidth, and not a halfwidth voiced pair, which would fold two runes to one
// and move every offset after it. want is the one spelling a pair meets at,
// and empty where the two must stay apart.
func TestFold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, left, right, want string
	}{
		{name: "final sigma", left: "\u039f\u0394\u039f\u03a3", right: "\u03bf\u03b4\u03bf\u03c2", want: "\u03bf\u03b4\u03bf\u03c3"},
		{name: "long s", left: "\u017ftop", right: "stop", want: "stop"},
		{name: "beta symbol", left: "\u03d0", right: "\u03b2", want: "\u03b2"},
		{name: "iota chooses a letter", left: "\u0399\u03b9\u0345\u1fbe", right: "\u03b9\u03b9\u03b9\u03b9", want: "\u03b9\u03b9\u03b9\u03b9"},
		{name: "iota stays normalized after a vowel", left: "\u0391\u0399", right: "\u03b1\u03b9", want: "\u03b1\u03b9"},
		{name: "capital sharp s lowers to sharp s", left: "\u1e9e", right: "\u00df", want: "\u00df"},
		{name: "fullwidth latin", left: "\uff27\uff4f", right: "Go", want: "go"},
		{name: "fullwidth digit", left: "\uff13\u3064", right: "3\u3064", want: "3\u3064"},
		{name: "fullwidth parentheses", left: "\u5fc3\uff08\u3053\u3053\u308d\uff09", right: "\u5fc3(\u3053\u3053\u308d)", want: "\u5fc3(\u3053\u3053\u308d)"},
		{name: "decomposed kana", left: "\u304b\u3099", right: "\u304c", want: "\u304c"},
		{name: "dotted capital I", left: "\u0130", right: "i"},
		{name: "dotless i", left: "\u0131", right: "i"},
		{name: "sharp s does not expand", left: "\u00df", right: "ss"},
		{name: "halfwidth katakana stays halfwidth", left: "\uff76", right: "\u30ab"},
		{name: "a halfwidth voiced pair stays two runes", left: "\uff8a\uff9e", right: "\u30d0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			left, right := Fold(tt.left), Fold(tt.right)
			if tt.want == "" {
				if left == right {
					t.Errorf("Fold(%+q) = Fold(%+q) = %+q, want them apart", tt.left, tt.right, left)
				}
				return
			}
			if left != tt.want || right != tt.want {
				t.Errorf("Fold(%+q), Fold(%+q) = %+q, %+q, want both %+q", tt.left, tt.right, left, right, tt.want)
			}
		})
	}
}

func TestFoldRunePreservesSimpleOrbitsAndOneRune(t *testing.T) {
	t.Parallel()
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		got := foldRune(r)
		if !utf8.ValidRune(got) || utf8.RuneCountInString(string(got)) != 1 {
			t.Fatalf("foldRune(%U) = %U, want one valid rune", r, got)
		}
		narrowed := r
		if r >= fullwidthASCIIMin && r <= fullwidthASCIIMax {
			narrowed = width.LookupRune(r).Narrow()
		}
		if !strings.EqualFold(string(narrowed), string(got)) {
			t.Fatalf("foldRune(%U) = %U outside narrowed simple orbit %U", r, got, narrowed)
		}
		if other := foldRune(unicode.SimpleFold(r)); other != got {
			t.Fatalf("foldRune(%U) = %U, orbit peer %U gives %U", r, got, unicode.SimpleFold(r), other)
		}
		if twice := foldRune(got); twice != got {
			t.Fatalf("foldRune(%U) = %U then %U, representative is not fixed", r, got, twice)
		}
	}
}
