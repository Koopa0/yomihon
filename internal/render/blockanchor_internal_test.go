package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Positions refer to authored bytes, including the boundary before the token.
// Unicode prefix width and trailing blanks must not move the span a page plants.
func TestBlockAddressInAdaptsAuthoredBytePositions(t *testing.T) {
	t.Parallel()

	type positions struct {
		Trimmed string
		Match   []int
	}
	for _, tt := range []struct {
		name string
		line string
		want positions
	}{
		{name: "marker only", line: "^id", want: positions{Trimmed: "^id", Match: []int{0, 3, 0, 3}}},
		{name: "space boundary", line: "x ^id", want: positions{Trimmed: "x ^id", Match: []int{1, 5, 2, 5}}},
		{name: "tab boundary", line: "x\t^id", want: positions{Trimmed: "x\t^id", Match: []int{1, 5, 2, 5}}},
		{name: "trailing blanks", line: "^id \t", want: positions{Trimmed: "^id", Match: []int{0, 3, 0, 3}}},
		{name: "Unicode prefix bytes", line: "\u5168 ^id", want: positions{Trimmed: "\u5168 ^id", Match: []int{3, 7, 4, 7}}},
		{name: "last token", line: "x ^a ^b", want: positions{Trimmed: "x ^a ^b", Match: []int{4, 7, 5, 7}}},
		{name: "authored case", line: "x ^AbC", want: positions{Trimmed: "x ^AbC", Match: []int{1, 6, 2, 6}}},
		{name: "empty", line: ""},
		{name: "no caret", line: "no caret"},
		{name: "unsupported token", line: "x ^a_b"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("hit: block position case %s", tt.name)
			trimmed, match := blockAddressIn(tt.line)
			got := positions{Trimmed: trimmed, Match: match}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: shared block address positions changed: blockAddressIn(%q) (-want +got):\n%s", tt.line, diff)
			}
		})
	}
}
