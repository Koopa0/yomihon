package render

import (
	"slices"
	"strings"
	"testing"
)

// Only a line with a caret in it can end in an address, so blockAddressIn
// answers any other without asking the pattern. That is a shortcut and not a
// reading: for every line, with a caret in any position or none, it gives the
// match the pattern gives on the line without its trailing blanks.
func TestBlockAddressInAnswersAsTheBarePatternDoes(t *testing.T) {
	t.Parallel()

	for _, line := range []string{
		"",
		"   ",
		"no caret at all",
		"a `code span` line",
		"^",
		"^id",
		"^id \t",
		"x ^id",
		"x\t^id",
		"x ^a ^b",
		"x^glued",
		"x ^a b",
		"全角 ^字幕",
	} {
		trimmed, m := blockAddressIn(line)
		wantTrimmed := strings.TrimRight(line, " \t")
		want := blockMarkerTail.FindStringSubmatchIndex(wantTrimmed)
		if !slices.Equal(m, want) {
			t.Errorf("blockAddressIn(%q) match = %v, the pattern on %q gives %v", line, m, wantTrimmed, want)
		}
		if m != nil && trimmed != wantTrimmed {
			t.Errorf("blockAddressIn(%q) trimmed = %q, want %q", line, trimmed, wantTrimmed)
		}
	}
}
