package judge

import (
	"strings"
	"testing"
)

// TestExistsCountsNotesRatherThanMatches holds the one number this command's
// human answer states. A note can answer to a name more than once — its
// filename and its title, or a title and an alias — and each is a row, so
// counting rows told a reader there were two notes and then printed one path
// twice. Somebody deciding whether to write a note under that name is exactly
// who reads this, and "2 notes" is the answer that stops them.
//
// The rows stay as they are: which of a note's names answered is a different
// fact from how many notes did, and both are worth printing.
func TestExistsCountsNotesRatherThanMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		report  existsReport
		want    string
		wantRow int
	}{
		{
			name: "one note answering twice",
			report: existsReport{Query: "What yomihon is", Matches: []existsMatch{
				{Path: "Notes/What yomihon is.md", Field: "filename"},
				{Path: "Notes/What yomihon is.md", Field: "title"},
			}},
			want:    `"What yomihon is" exists in 1 note(s):`,
			wantRow: 2,
		},
		{
			name: "two notes answering once each",
			report: existsReport{Query: "shared", Matches: []existsMatch{
				{Path: "Concepts/golang/Map.md", Field: "alias"},
				{Path: "Concepts/golang/Slice.md", Field: "alias"},
			}},
			want:    `"shared" exists in 2 note(s):`,
			wantRow: 2,
		},
		{
			name: "two notes, one of them answering twice",
			report: existsReport{Query: "both", Matches: []existsMatch{
				{Path: "A.md", Field: "filename"},
				{Path: "A.md", Field: "alias"},
				{Path: "B.md", Field: "title"},
			}},
			want:    `"both" exists in 2 note(s):`,
			wantRow: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rendered := renderExists(tt.report)
			if !strings.Contains(rendered, tt.want) {
				t.Errorf("the count line is wrong:\n%s\nwant a line reading %s", rendered, tt.want)
			}
			rows := 0
			for line := range strings.SplitSeq(rendered, "\n") {
				if strings.HasPrefix(line, "  ") {
					rows++
				}
			}
			if rows != tt.wantRow {
				t.Errorf("printed %d rows, want %d:\n%s", rows, tt.wantRow, rendered)
			}
		})
	}
}
