package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// This face and the reading page each read a note body for the citations in it,
// and they read it with differently configured markdown engines: this one plain,
// the page's with the extensions a note is written against. Nothing held the two
// readings together, and the way that showed was a bracket pair the page turned
// into a live link inside a code block while this face, correctly, saw quoted
// text and reported nothing — one fault, one face silent, the other wrong.
//
// The check below compares what the two faces read out of one body rather than
// which function each called, so it keeps holding when either side is rewritten.
// The body carries a citation in every construct the two engines could read
// differently, and one in each construct that must contribute nothing.

type capturedBodies map[string]string

func (b capturedBodies) Transclusion(path string) (string, bool) {
	body, ok := b[path]
	return body, ok
}

type noTitlesDeclared struct{}

func (noTitlesDeclared) TitledBy(string) []string { return nil }

// citationSurface is one body written to disagree: a citation inside a callout,
// a table cell, a strikethrough run, a footnote definition, a task item, and
// beside an autolink, each of which only one of the two engines parses; and a
// citation inside a fence, an indented code block at the top level and under a
// list, an authored HTML block, a code span and a comment, none of which either
// face may read. The indented shapes are what the two faces used to part over,
// and the reason they are written at three depths is that the indent opening a
// block is measured from whatever encloses the line.
const citationSurface = "" +
	"> [!note] Callout\n" +
	"> A citation inside a callout: [[Callout Target]].\n" +
	"\n" +
	"| col | link |\n" +
	"|---|---|\n" +
	"| a | [[Table Target]] |\n" +
	"\n" +
	"~~struck [[Struck Target]]~~\n" +
	"\n" +
	"A footnote reference[^n].\n" +
	"\n" +
	"[^n]: the definition cites [[Footnote Target]].\n" +
	"\n" +
	"- [ ] a task citing [[Task Target]]\n" +
	"\n" +
	"Autolink https://example.com beside [[Autolink Target]].\n" +
	"\n" +
	"```\n" +
	"[[Fenced Target]]\n" +
	"```\n" +
	"\n" +
	"Prose, and then a block indented under nothing:\n" +
	"\n" +
	"    [[Indented Target]]\n" +
	"\n" +
	"- a list item\n" +
	"\n" +
	"      [[Deeply Indented Target]]\n" +
	"\n" +
	"1. a numbered item\n" +
	"\n" +
	"       [[Numbered Indented Target]]\n" +
	"\n" +
	"<div>\n" +
	"[[Authored Block Target]]\n" +
	"</div>\n" +
	"\n" +
	"An inline `[[Code Span Target]]` span.\n" +
	"\n" +
	"%%[[Commented Target]]%%\n"

// TestBothFacesReadOneSetOfCitations holds the adjudicator's reading of a body
// equal to the reading page's. An index that resolves nothing makes the page
// report every name it read, which is what lets the two lists be compared.
func TestBothFacesReadOneSetOfCitations(t *testing.T) {
	t.Parallel()

	// A body that named nothing either engine parses would let both lists be
	// empty and the comparison pass having compared nothing.
	adjudicated := judge.LinkTargets(citationSurface)
	if len(adjudicated) == 0 {
		t.Fatalf("the body names no citation at all, so this check compares nothing")
	}

	page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{})
	result := page.HTML("Notes/Reading.md", "Reading", citationSurface, wording.En)
	var read []string
	for _, d := range result.Diagnostics {
		if d.Kind == render.DiagWikilinkBroken {
			read = append(read, d.Target)
		}
	}

	if diff := cmp.Diff(adjudicated, read); diff != "" {
		t.Errorf("the two faces read different citations out of one body (-adjudicated +page):\n%s", diff)
	}
}

// TestThePageShowsQuotedSyntaxAsWritten is the half of the agreement the check
// above cannot see: what the reader is shown. A citation the page decides is
// quoted has to reach them as the bytes their author typed, and one the page
// decides is live has to reach them as a link — the two ways of getting this
// wrong are a control the author never wrote and a pair of brackets where a
// link should be, and neither face reports either.
//
// One row is what makes the oracle's identity matter, and it is the blank line
// inside a footnote definition: the line ends the definition's first paragraph,
// so what follows is its second paragraph to the pipeline the page is built from
// and an indented code block to a parser without that extension. An oracle
// configured apart from the pipeline therefore shows the reader raw brackets in
// the middle of a sentence, and that row alone fails when one is. The two rows
// beside it are lazy continuations, which both readings call prose; they hold
// the neighbouring shapes still rather than separating the two readings.
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
			page := render.New(
				graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/Real Note.md"}}, nil),
				capturedBodies{},
				noTitlesDeclared{},
			)
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
// Every code region opens with a "<code" this package wrote, block and inline
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
