package schema

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFoldedKeyPairsPreserveSimpleFoldAndOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		keys []string
		want []string
	}{
		{name: "empty"},
		{name: "singleton", keys: []string{"K"}},
		{name: "full fold stays distinct", keys: []string{"ss", "ß"}},
		{name: "composition stays distinct", keys: []string{"e\u0301", "é"}},
		{name: "kelvin orbit", keys: []string{"K", "k", "K"}, want: []string{`"table.K" and "table.k"`, `"table.K" and "table.K"`, `"table.k" and "table.K"`}},
		{name: "sharp s orbit", keys: []string{"ss", "ß", "ẞ"}, want: []string{`"table.ß" and "table.ẞ"`}},
		{name: "long s orbit", keys: []string{"S", "s", "ſ"}, want: []string{`"table.S" and "table.s"`, `"table.S" and "table.ſ"`, `"table.s" and "table.ſ"`}},
		{name: "sigma orbit", keys: []string{"Σ", "ς", "σ"}, want: []string{`"table.Σ" and "table.ς"`, `"table.Σ" and "table.σ"`, `"table.ς" and "table.σ"`}},
		{name: "interleaved groups", keys: []string{"K", "S", "k", "s", "ſ", "K"}, want: []string{`"table.K" and "table.k"`, `"table.K" and "table.K"`, `"table.S" and "table.s"`, `"table.S" and "table.ſ"`, `"table.k" and "table.K"`, `"table.s" and "table.ſ"`}},
		{name: "whole names", keys: []string{"AK", "Ak", "BK", "ak", "aK"}, want: []string{`"table.AK" and "table.Ak"`, `"table.AK" and "table.ak"`, `"table.AK" and "table.aK"`, `"table.Ak" and "table.ak"`, `"table.Ak" and "table.aK"`, `"table.ak" and "table.aK"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			keys := slices.Clone(tt.keys)
			if got := siblingsThatFold("table.", keys); !cmp.Equal(tt.want, got) {
				t.Errorf("siblingsThatFold(%q) mismatch (-want +got):\n%s", tt.keys, cmp.Diff(tt.want, got))
			}
			if diff := cmp.Diff(tt.keys, keys); diff != "" {
				t.Errorf("siblingsThatFold modified its input (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDistinctKeyRefusalBytes(t *testing.T) {
	t.Parallel()

	const data = `"K" = 1
"k" = 2
"K" = 3
"ss" = 4
"ß" = 5
"ẞ" = 6
[nested]
"A" = 1
"a" = 2
[[rows]]
"Q" = 1
"q" = 2
[[rows]]
"A" = 3
[[rows]]
"a" = 4
`
	const want = `contract keys differ only by letter case: "K" and "k"; "K" and "K"; "k" and "K"; "ß" and "ẞ"; "nested.A" and "nested.a"; "rows.Q" and "rows.q"`
	err := validateDistinctKeys([]byte(data))
	if err == nil {
		t.Fatal("validateDistinctKeys(folding siblings) error = nil, want a refusal")
	}
	if got := err.Error(); got != want {
		t.Errorf("validateDistinctKeys(folding siblings) = %q, want %q", got, want)
	}
}

// BenchmarkValidateDistinctKeys measures only validation, using flat distinct
// keys up to the contract's size limit. Input construction is outside timing.
func BenchmarkValidateDistinctKeys(b *testing.B) {
	for _, kib := range []int{64, 128, 256} {
		b.Run(fmt.Sprintf("%dKiB", kib), func(b *testing.B) {
			var data strings.Builder
			for i := range kib * 1024 / 16 {
				fmt.Fprintf(&data, "\"key%06d\" = 0\n", i)
			}
			input := []byte(data.String())
			if got := len(input); got != kib*1024 {
				b.Fatalf("contract size = %d, want %d", got, kib*1024)
			}
			b.SetBytes(int64(len(input)))
			b.ReportAllocs()
			for b.Loop() {
				if err := validateDistinctKeys(input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSiblingFoldDistinct isolates sibling grouping from TOML decoding
// and sorting, so unrelated decoder work cannot obscure its growth rate.
func BenchmarkSiblingFoldDistinct(b *testing.B) {
	for _, count := range []int{1024, 2048, 4096} {
		b.Run(fmt.Sprintf("%dkeys", count), func(b *testing.B) {
			names := make([]string, count)
			for i := range names {
				names[i] = fmt.Sprintf("key%06d", i)
			}
			b.ReportAllocs()
			for b.Loop() {
				if got := siblingsThatFold("", names); len(got) != 0 {
					b.Fatalf("siblingsThatFold(distinct keys) = %v, want no pairs", got)
				}
			}
		})
	}
}

func FuzzSiblingFoldIdentity(f *testing.F) {
	for _, seed := range [][2]string{{"K", "K"}, {"S", "ſ"}, {"Σ", "ς"}, {"ß", "ss"}, {"ß", "ẞ"}, {"e\u0301", "é"}, {"i", "İ"}, {"\xff", "\xfe"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		names := []string{a, b}
		slices.Sort(names)
		want := 0
		if strings.EqualFold(a, b) {
			want = 1
		}
		if got := siblingsThatFold("", names); len(got) != want {
			t.Errorf("siblingsThatFold(%q) = %v, want %d pairs under strings.EqualFold", names, got, want)
		}
	})
}
