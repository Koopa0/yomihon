package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestCodeSpanRangesReadPastARunThatFoundNoCloser holds the rule the page
// itself renders by: a backtick run that meets no run of its own length is
// ordinary text, and the runs after it still pair. Reading the rest of a line
// as "not code, and nothing after it either" hid every span an author wrote
// after a stray backtick, so the syntax they were quoting was converted into
// live markup on the page.
func TestCodeSpanRangesReadPastARunThatFoundNoCloser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want [][2]int
	}{
		{
			name: "a plain span with nothing before it",
			text: "`a` b",
			want: [][2]int{{0, 3}},
		},
		{
			name: "a stray run does not stop the span after it",
			text: "``a `b`",
			want: [][2]int{{4, 7}},
		},
		{
			name: "several stray runs do not stop the span after them",
			text: "``a ```b `c`",
			want: [][2]int{{9, 12}},
		},
		{
			name: "a stray run between two spans stops neither",
			text: "`a` `` `b`",
			want: [][2]int{{0, 3}, {7, 10}},
		},
		{
			name: "a stray run at the end of the line is not a span",
			text: "`a` b `",
			want: [][2]int{{0, 3}},
		},
		{
			name: "a stray run with nothing after it is not a span",
			text: "``a",
			want: nil,
		},
		{
			name: "two spans in a row both count",
			text: "`a` `b`",
			want: [][2]int{{0, 3}, {4, 7}},
		},
		{
			name: "a shorter run inside a span is its content",
			text: "``a `b``",
			want: [][2]int{{0, 8}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tt.want, codeSpanRanges(tt.text)); diff != "" {
				t.Errorf("codeSpanRanges(%q) mismatch (-want +got):\n%s", tt.text, diff)
			}
		})
	}
}
