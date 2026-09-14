package sequence_test

import (
	"testing"

	"github.com/koopa0/yomihon/internal/sequence"
)

// A fence needs at most three spaces of indent to open one. Written deeper it
// is an indented code block — an author showing what a fence looks like — and
// a scan that reads it as an opening finds no closer, so the exclusion runs to
// the end of the note and the lesson rows written below it stop being links.
// The course and the map read their links through this one scan, so a part
// that shows a fence loses its lessons on both faces at once.
func TestLiveWikilinksSurviveAnIndentedFenceExample(t *testing.T) {
	t.Parallel()

	const lessons = "- [[L01]]\n- [[L02]]\n"

	tests := []struct {
		name   string
		body   string
		want   []string
		absent string
	}{
		{
			name: "an indented backtick example",
			body: "## Part {sequence=primary}\n\n    ```\n\n" + lessons,
			want: []string{"L01", "L02"},
		},
		{
			name: "an indented tilde example",
			body: "## Part {sequence=primary}\n\n    ~~~\n\n" + lessons,
			want: []string{"L01", "L02"},
		},
		{
			name: "ordinary indented text, the control that already worked",
			body: "## Part {sequence=primary}\n\n    marker\n\n" + lessons,
			want: []string{"L01", "L02"},
		},
		{
			name:   "a real fence still hides the link inside it",
			body:   "## Part {sequence=primary}\n\n```\n[[Hidden]]\n```\n\n" + lessons,
			want:   []string{"L01", "L02"},
			absent: "Hidden",
		},
		{
			name:   "an authored HTML block still hides the link inside it",
			body:   "## Part {sequence=primary}\n\n<div>\n[[Hidden]]\n</div>\n\n" + lessons,
			want:   []string{"L01", "L02"},
			absent: "Hidden",
		},
		{
			// The fence sits four spaces in, which is two inside a list item
			// whose content begins at two — a fence, not indented code. The
			// tree the scan already has is what keeps this one excluded, and
			// it is the case that says the indent rule was applied where the
			// document's shape is known rather than guessed at line by line.
			name:   "a fence nested in a list item still hides the link inside it",
			body:   "## Part {sequence=primary}\n\n- item\n    ```\n    [[Inside]]\n    ```\n" + lessons,
			want:   []string{"L01", "L02"},
			absent: "Inside",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []string
			for _, link := range sequence.LiveWikilinks(tt.body) {
				got = append(got, link.Target)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("LiveWikilinks() = %v, want %v", got, tt.want)
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("LiveWikilinks()[%d] = %q, want %q", i, got[i], want)
				}
			}
			if tt.absent != "" {
				for _, target := range got {
					if target == tt.absent {
						t.Errorf("LiveWikilinks() = %v, must not contain %q", got, tt.absent)
					}
				}
			}
		})
	}
}
