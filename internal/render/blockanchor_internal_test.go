package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBlockAddressExtractsOnlySupportedAuthoredTokens(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, line, want string }{
		{name: "paragraph", line: "Para ^abc-1", want: "^abc-1"},
		{name: "marker only", line: "^abc-1", want: "^abc-1"},
		{name: "ASCII case retained", line: "Para ^AbC-1", want: "^AbC-1"},
		{name: "digits and hyphens", line: "Para ^0-9--", want: "^0-9--"},
		{name: "hyphen only", line: "Para ^-", want: "^-"},
		{name: "tab boundary", line: "Para\t^abc-1", want: "^abc-1"},
		{name: "trailing spaces and tabs", line: "Para ^abc-1 \t", want: "^abc-1"},
		{name: "last token", line: "Para ^first ^last", want: "^last"},
		{name: "Unicode prefix", line: "\u5168 ^abc-1", want: "^abc-1"},
		{name: "empty line"},
		{name: "spaces only", line: " \t"},
		{name: "no caret", line: "Para abc-1"},
		{name: "empty token", line: "Para ^"},
		{name: "glued caret", line: "Para^abc-1"},
		{name: "underscore", line: "Para ^a_b"},
		{name: "dot", line: "Para ^a.b"},
		{name: "Unicode token", line: "Para ^\u304c"},
		{name: "combining token", line: "Para ^a\u0301"},
		{name: "fullwidth token", line: "Para ^\uff21"},
		{name: "punctuation", line: "Para ^abc!"},
		{name: "inline footnote", line: "Para ^[note]"},
		{name: "interior space", line: "Para ^a b"},
		{name: "interior tab", line: "Para ^a\tb"},
		{name: "nonbreaking boundary", line: "Para\u00a0^abc-1"},
		{name: "newline boundary", line: "Para\n^abc-1"},
		{name: "carriage return tail", line: "Para ^abc-1\r"},
		{name: "newline tail", line: "Para ^abc-1\n"},
		{name: "nonbreaking tail", line: "Para ^abc-1\u00a0"},
		{name: "adjacent wikilink", line: "Some text ^ref[[Note]]"},
		{name: "inline placeholder", line: "Some text ^ref\ue0000\ue001"},
		{name: "wide placeholder", line: "Some text ^ref\ue0020\ue003"},
		{name: "NUL tail", line: "Para ^abc-1\x00"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("hit: BlockAddress literal case %s", tt.name)
			if got := BlockAddress(tt.line); got != tt.want {
				t.Errorf("caught: shared block grammar changed: BlockAddress(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

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

func FuzzBlockAddress(f *testing.F) {
	for _, line := range []string{
		"", "^", "^abc-1", "Para ^AbC-1 \t", "Para\t^0--9", "Para^glued",
		"Para ^a_b", "Para ^a.b", "Para ^\u304c", "Para ^[note]", "Para ^a b",
		"Some text ^ref[[Note]]", "Some text ^ref\ue0000\ue001", "^abc\x00", "^abc\n",
		string([]byte{0xff, ' ', '^', 'a'}),
	} {
		f.Add(line)
	}
	f.Fuzz(func(_ *testing.T, line string) {
		BlockAddress(line)
	})
}
