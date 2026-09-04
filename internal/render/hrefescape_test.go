package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAmpersandInADestination covers the one byte that is safe in a URL and
// unsafe in an attribute. A picture or a note whose name carries "&" is
// ordinary in this vault, and every route to one of them passes through an
// attribute, so each way a destination is written gets its own case: the two a
// reader types as markdown, the two the wikilink dialect adds, and a remote
// address as the control that must come through with its query intact.
func TestAmpersandInADestination(t *testing.T) {
	t.Parallel()

	r := newRenderer(t,
		[]graph.NoteInput{{RelPath: "Notes/Q&A.md"}},
		[]string{"Notes/a&b.png", "Notes/a&b.pdf"},
		transclusions{"Notes/Q&A.md": "## Heading Inside\n\nThe paragraph a block address names. ^para\n"},
	)

	tests := []struct {
		name   string
		body   string
		want   string
		absent string
	}{
		{
			name:   "a markdown image beside the note",
			body:   "![](a&b.png)",
			want:   `<img src="/raw/Notes/a&amp;b.png"`,
			absent: "%3B",
		},
		{
			name:   "a markdown image whose name reads as a character reference",
			body:   "![](a&copy.png)",
			want:   `<img src="/raw/Notes/a&amp;copy.png"`,
			absent: "%3B",
		},
		{
			name:   "a wikilink to a name carrying one",
			body:   "[[Q&A]]",
			want:   `href="/notes/Notes/Q&amp;A.md"`,
			absent: `Q&A.md"`,
		},
		{
			name:   "an embedded picture",
			body:   "![[a&b.png]]",
			want:   `<img src="/raw/Notes/a&amp;b.png"`,
			absent: `src="/raw/Notes/a&b.png"`,
		},
		{
			name:   "an embed of a note names the note it quotes",
			body:   "![[Q&A]]",
			want:   `<a href="/notes/Notes/Q&amp;A.md"`,
			absent: `<a href="/notes/Notes/Q&A.md"`,
		},
		{
			name:   "an embed of something with no inline display",
			body:   "![[a&b.pdf]]",
			want:   `<a href="/notes/Notes/a&amp;b.pdf"`,
			absent: `<a href="/notes/Notes/a&b.pdf"`,
		},
		{
			name:   "a link whose fragment the note does not answer to",
			body:   "[[Q&A#^nowhere]]",
			want:   `<a href="/notes/Notes/Q&amp;A.md"`,
			absent: `<a href="/notes/Notes/Q&A.md"`,
		},
		{
			name:   "a remote address keeps both query parameters",
			body:   "![](https://example.com/x.png?a=1&b=2)",
			want:   "?a=1&amp;b=2",
			absent: "%3B",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Notes/note.md", "", tt.body, wording.ZhHant).HTML
			if !strings.Contains(got, tt.want) {
				t.Errorf("rendering %q is missing %q:\n%s", tt.body, tt.want, got)
			}
			if strings.Contains(got, tt.absent) {
				t.Errorf("rendering %q still carries %q:\n%s", tt.body, tt.absent, got)
			}
		})
	}
}
