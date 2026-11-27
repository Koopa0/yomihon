package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestThePageShowsQuotedSyntaxAsWritten holds what the reader is shown of a
// citation the page reads as code or as prose: quoted syntax reaches them as
// the bytes their author typed, and a live citation as a link. The blank line
// inside a footnote definition ends its first paragraph, so the indented line
// after it is the definition's second paragraph rather than an indented code
// block; the lazy continuations beside it are prose too.
func TestThePageShowsQuotedSyntaxAsWritten(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		// asWritten is whether the page shows this citation as its author typed
		// it, because the line it sits on is code, rather than as a link.
		asWritten bool
	}{
		{name: "an indented block under nothing", asWritten: true, body: "Prose above.\n\n    plain [[Real Note]] text\n"},
		{name: "an indented block written with a tab", asWritten: true, body: "Prose above.\n\n\tplain [[Real Note]] text\n"},
		{name: "an indented block under a list item", asWritten: true, body: "- a list item\n\n      plain [[Real Note]] text\n"},
		{name: "an indented block under a heading", asWritten: true, body: "Title\n=====\n\n    plain [[Real Note]] text\n"},
		{name: "indented content inside a callout", asWritten: true, body: "> [!note] N\n>     plain [[Real Note]] text\n"},
		{name: "a fenced block", asWritten: true, body: "Prose above.\n\n```\nplain [[Real Note]] text\n```\n"},
		{name: "a footnote definition's second paragraph", asWritten: false, body: "A ref[^n].\n\n[^n]: first para.\n\n    plain [[Real Note]] text\n"},
		{name: "a footnote definition's lazy continuation", asWritten: false, body: "A ref[^n].\n\n[^n]: first para.\n    plain [[Real Note]] text\n"},
		{name: "an indented line after a table", asWritten: false, body: "| a | b |\n|---|---|\n| c | d |\n    plain [[Real Note]] text\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			page := newRenderer(t, []graph.NoteInput{{RelPath: "Notes/Real Note.md"}}, nil, nil)
			got := page.HTML("Notes/Reading.md", "Reading", tt.body, wording.En).HTML
			quoted, shown := splitAtCode(got)

			if strings.Contains(quoted, "<a ") {
				t.Errorf("the page turned quoted syntax into a control:\n%s", got)
			}
			if strings.Contains(shown, "[[") {
				t.Errorf("the page left a citation standing as brackets:\n%s", got)
			}
			if tt.asWritten && !strings.Contains(quoted, "plain [[Real Note]] text") {
				t.Errorf("the code does not carry the line as its author typed it:\n%s", got)
			}
			if !tt.asWritten && !strings.Contains(shown, `class="wikilink"`) {
				t.Errorf("the citation never became a link:\n%s", got)
			}
		})
	}
}

// splitAtCode divides rendered page HTML into what it shows as code and what it
// shows as prose, so each can be asked a question the other would answer wrongly.
// Every code region opens with a "<code" the renderer wrote, block and inline
// alike, since authored markup reaches the page escaped and never as an element.
func splitAtCode(htmlOut string) (quoted, shown string) {
	var q, s strings.Builder
	rest := htmlOut
	for {
		open := strings.Index(rest, "<code")
		if open < 0 {
			break
		}
		shut := strings.Index(rest[open:], "</code>")
		if shut < 0 {
			break
		}
		s.WriteString(rest[:open])
		q.WriteString(rest[open : open+shut])
		rest = rest[open+shut+len("</code>"):]
	}
	s.WriteString(rest)
	return q.String(), s.String()
}
