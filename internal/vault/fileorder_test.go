package vault

import (
	"cmp"
	"slices"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
)

func TestComparePathsReadingOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		paths, want []string
	}{
		{"folded issue names", []string{"Untitled 2.md", "Zed.md", "Untitled.md", "index.md"}, []string{"index.md", "Untitled.md", "Untitled 2.md", "Zed.md"}},
		{"letters before case", []string{"TODO.md", "readme.md", "index.md"}, []string{"index.md", "readme.md", "TODO.md"}},
		{"stems before extensions", []string{"Untitled 2.md", "Untitled.txt", "Untitled.md", "Untitled"}, []string{"Untitled", "Untitled.md", "Untitled.txt", "Untitled 2.md"}},
		{"segment prefix before sibling suffix", []string{"Writing-old/Legacy.md", "Writing/Kafka.md"}, []string{"Writing/Kafka.md", "Writing-old/Legacy.md"}},
		{"every segment", []string{"Untitled 2/first.md", "Untitled.md/first.md", "Untitled/first.md"}, []string{"Untitled/first.md", "Untitled.md/first.md", "Untitled 2/first.md"}},
		{"raw tie after all segments", []string{"A/z.md", "a/b.md"}, []string{"a/b.md", "A/z.md"}},
		{"numeric tie after all segments", []string{"Part01/z.md", "Part1/b.md"}, []string{"Part1/b.md", "Part01/z.md"}},
		{"raw case tie", []string{"readme.md", "README.md", "Readme.md"}, []string{"README.md", "Readme.md", "readme.md"}},
		{"Kelvin equivalence before later text", []string{"Kz.md", "Ka.md", "kb.md"}, []string{"Ka.md", "kb.md", "Kz.md"}},
		{"sigma equivalence before later text", []string{"Σz.md", "ςa.md", "σb.md"}, []string{"ςa.md", "σb.md", "Σz.md"}},
		{"three member Kelvin fold", []string{"K.md", "k.md", "K.md"}, []string{"K.md", "k.md", "K.md"}},
		{"three member sigma fold", []string{"σ.md", "ς.md", "Σ.md"}, []string{"Σ.md", "ς.md", "σ.md"}},
		{"extension case and numeric reading", []string{"same.X10", "same.x2", "same.X2"}, []string{"same.X2", "same.x2", "same.X10"}},
		{"numbers still last", []string{"2.md", "一.md", "abc.md"}, []string{"abc.md", "一.md", "2.md"}},
		{"Chinese and ASCII numbers", []string{"第10課.md", "第三課.md", "第2課.md"}, []string{"第2課.md", "第三課.md", "第10課.md"}},
		{"segment prefix", []string{"a/b.md", "A"}, []string{"A", "a/b.md"}},
		{"literal punctuation", []string{"part-a.md", "part_a.md", "part a.md"}, []string{"part a.md", "part-a.md", "part_a.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Log("invoked: whole reading-order contract")
			got := slices.Clone(tt.paths)
			slices.SortFunc(got, ComparePaths)
			if diff := gocmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: ComparePaths sorted %q mismatch (-want +got):\n%s", tt.paths, diff)
			}
		})
	}
}

func FuzzComparePaths(f *testing.F) {
	for _, seed := range [][3]string{
		{"Untitled.md", "Untitled 2.md", "index.md"},
		{"A/z.md", "a/b.md", "A/c.md"},
		{"Part01/z.md", "Part1/b.md", "Part一/c.md"},
		{"K.md", "k.md", "K.md"},
		{"σ.md", "ς.md", "Σ.md"},
		{"abc", "一", "2"},
		{"a\x00", "a/", "a\xff"},
	} {
		f.Add(seed[0], seed[1], seed[2])
	}
	f.Fuzz(func(t *testing.T, a, b, c string) {
		if len(a)+len(b)+len(c) > 4096 {
			t.Skip("ordering corpus exceeds a filesystem path")
		}
		ab, ba := cmp.Compare(ComparePaths(a, b), 0), cmp.Compare(ComparePaths(b, a), 0)
		if ab != -ba || (a == b) != (ab == 0) {
			t.Fatalf("ComparePaths(%q,%q)=%d reverse=%d: unequal strings need an antisymmetric order", a, b, ab, ba)
		}
		if bc, ac := ComparePaths(b, c), ComparePaths(a, c); ab < 0 && bc < 0 && ac >= 0 {
			t.Fatalf("ComparePaths(%q,%q)<0 and ComparePaths(%q,%q)<0 but ComparePaths(%q,%q)=%d", a, b, b, c, a, c, ac)
		}
	})
}
