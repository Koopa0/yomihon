package lexical

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/width"
)

func TestFoldUsesSimpleCaseEquivalence(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, left, right, want string }{
		{name: "final sigma", left: "ΟΔΟΣ", right: "οδος", want: "οδοσ"},
		{name: "long s", left: "ſtop", right: "stop", want: "stop"},
		{name: "beta symbol", left: "ϐ", right: "β", want: "β"},
		{name: "iota chooses a letter", left: "Ιι\u0345\u1fbe", right: "ιιιι", want: "ιιιι"},
		{name: "iota stays normalized after a vowel", left: "ΑΙ", right: "αι", want: "αι"},
		{name: "fullwidth step remains", left: "Ｇｏ", right: "Go", want: "go"},
		{name: "sharp s stays one rune", left: "ẞ", right: "ß", want: "ß"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := []string{Fold(tt.left), Fold(tt.right), Fold(Fold(tt.left))}
			if diff := cmp.Diff([]string{tt.want, tt.want, tt.want}, got); diff != "" {
				t.Errorf("Fold(%q, %q), repeat (-want +got):\n%s", tt.left, tt.right, diff)
			}
		})
	}
	for _, pair := range [][2]string{{"İ", "i"}, {"ı", "i"}, {"ß", "ss"}} {
		if Fold(pair[0]) == Fold(pair[1]) {
			t.Errorf("Fold(%q) = Fold(%q), distinct simple orbits must not meet", pair[0], pair[1])
		}
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

func TestSimpleCaseMatchesKeepOriginalEvidence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, query string
		want              []HitRun
	}{
		{name: "Greek body", body: "here ΟΔΟΣ there", query: "οδος", want: []HitRun{{Text: "here "}, {Text: "ΟΔΟΣ", Hit: true}, {Text: " there"}}},
		{name: "shrinking prefix and hit", body: "ſſſſ ſtop tail", query: "stop", want: []HitRun{{Text: "ſſſſ "}, {Text: "ſtop", Hit: true}, {Text: " tail"}}},
		{name: "symbol byte change", body: "before ϐ after", query: "β", want: []HitRun{{Text: "before "}, {Text: "ϐ", Hit: true}, {Text: " after"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{{RelPath: "Notes/case.md", Title: "Case", PlainText: tt.body}}, validArtifactPolicy(t))
			a, err := idx.Search(Parse(tt.query), -1)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Results) != 1 {
				t.Fatalf("Search(%q) results = %v, want one", tt.query, a.Results)
			}
			if diff := cmp.Diff(tt.want, MarkHits(a.Results[0].Snippet, Parse(tt.query).Tokens())); diff != "" {
				t.Errorf("Search(%q) evidence (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}
