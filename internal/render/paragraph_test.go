package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
)

// A wikilink is substituted into the source before the markdown pass and put
// back afterwards. Where the substitution sits in its line must not change what
// the line is: prose that opens with a link is still prose, and it has to reach
// the reader wrapped like the paragraph above and below it, or its spacing
// collapses and two paragraphs run together on the page.
func TestParagraphKeepsItsWrapperWhenItOpensWithAWikilink(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "運弓筆記.md"}}, nil, nil)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "resolved link mid-paragraph",
			body: "音階練習\n\n那天讀的 [[運弓筆記]] 裡那個手腕角度有效\n",
		},
		{
			name: "resolved link opens the paragraph",
			body: "音階練習\n\n[[運弓筆記]] 裡那個手腕角度有效\n",
		},
		{
			name: "unresolved link opens the paragraph",
			body: "音階練習\n\n[[還沒寫的一篇]] 裡那個手腕角度有效\n",
		},
		{
			name: "link is the whole paragraph",
			body: "音階練習\n\n[[運弓筆記]]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("練習日誌/2026-07-29.md", "2026-07-29", tt.body)
			// Two authored paragraphs must arrive as two wrapped paragraphs.
			// Counting them is what distinguishes a lost wrapper from a merely
			// different one: the trailing text stays either way.
			if n := strings.Count(got.HTML, "<p>"); n != 2 {
				t.Errorf("HTML(%q) produced %d paragraph wrappers, want 2\ngot: %s", tt.body, n, got.HTML)
			}
		})
	}
}
