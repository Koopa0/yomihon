package render_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestASectionIsGivenTheLineItsHeadingOpensOnOnlyWhereTheSourceVouchesForIt
// holds the line a section's heading opens on, counted from the first line of
// the body, to what the file says: a wrong line sends a reader's editor to the
// wrong place, so a section the source cannot vouch for has none. Every
// expected line is counted by hand from the body above it.
func TestASectionIsGivenTheLineItsHeadingOpensOnOnlyWhereTheSourceVouchesForIt(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Other.md"}}, nil, transclusions{
		"Other.md": "## Two\n\nFrom the other note.\n",
	})
	tests := []struct {
		name  string
		title string
		body  string
		want  map[string]int
	}{
		{
			name:  "the title the page shows is not a section, and an underlined one moves no line after it",
			title: "Title words",
			body:  "Title words\n===========\n\n## A\n\nText.\n",
			want:  map[string]int{"a": 3},
		},
		{
			name:  "a later heading with the title's words is a section, and the title still is not",
			title: "T",
			body:  "# T\n\n## A\n\n# T\n",
			want:  map[string]int{"a": 2, "t-2": 4},
		},
		{
			name:  "headings with the same words each keep their own line",
			title: "T",
			body:  "## Same\n\nOne.\n\n## Same\n\nTwo.\n\n### Same\n",
			want:  map[string]int{"same": 0, "same-2": 4, "same-3": 8},
		},
		{
			name:  "a heading in a callout has no line",
			title: "T",
			body:  "## Before\n\n> [!note]\n> ## Inside\n> Words.\n\n## After\n",
			want:  map[string]int{"before": 0, "after": 6},
		},
		{
			name:  "a heading in a callout with a section's words leaves neither with a line",
			title: "T",
			body:  "## Setup\n\nWords.\n\n> [!tip]\n> ## Setup\n> More.\n\n## Next\n",
			want:  map[string]int{"next": 8},
		},
		{
			name:  "a comment hides its heading, though it has a section's words, and moves no line",
			title: "T",
			body:  "%%\n## Shown\nstill hidden\n%%\n\n## Shown\n\nText %%inline%% here.\n\n## Last\n",
			want:  map[string]int{"shown": 5, "last": 9},
		},
		{
			name:  "a heading-shaped line in code is not a section",
			title: "T",
			body:  "```\n## Not a heading\n```\n\n## Real\n",
			want:  map[string]int{"real": 4},
		},
		{
			name:  "an underlined heading opens on its words",
			title: "T",
			body:  "Intro.\n\nTwo lines\nof words\n---------\n\nText.\n",
			want:  map[string]int{"two-lines-of-words": 2},
		},
		{
			name:  "an embedded note's heading is not the note's, and the note's own keeps its line",
			title: "T",
			body:  "![[Other]]\n\n## Two\n\nText.\n",
			want:  map[string]int{"two-2": 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := r.HTML("Note.md", tt.title, tt.body, wording.En)
			got := result.HeadingLines(tt.title, tt.body)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("HeadingLines() mismatch (-want +got):\n%s\ncontents: %+v", diff, result.TOC)
			}
		})
	}
}
