package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// This face and the reading page each read a note body for the citations in it,
// with differently configured markdown engines: this one plain, the page's with
// the extensions a note is written against.
//
// The check below compares what the two faces read out of one body rather than
// which function each called, so it keeps holding when either side is rewritten.
// The body carries a citation in every construct the two engines could read
// differently, and one in each construct that must contribute nothing.

// bodiesOnly answers the page's questions about the rest of the vault as if
// nothing else were there: no transcluded body, no declared title, and no
// picture missing, so the page reports exactly the citations it read.
type bodiesOnly struct{}

func (bodiesOnly) Transclusion(string) (string, bool) { return "", false }

func (bodiesOnly) TitledBy(string) []string { return nil }

func (bodiesOnly) MissingFile(string) bool { return false }

func (bodiesOnly) OutsideScan(string) bool { return false }

func (bodiesOnly) IsDirectory(string) bool { return false }

// citationSurface is one body written to disagree: a citation inside a callout,
// a table cell, a strikethrough run, a footnote definition, a task item, and
// beside an autolink, each of which only one of the two engines parses; and a
// citation inside a fence, an indented code block at the top level and under a
// list, an authored HTML block, a code span and a comment, none of which either
// face may read. The indented shapes are written at three depths because the
// indent opening a block is measured from whatever encloses the line.
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
	adjudicated := egress.Targets(judge.LinkTargets(citationSurface))
	if len(adjudicated) == 0 {
		t.Fatalf("the body names no citation at all, so this check compares nothing")
	}

	page := render.New(graph.BuildFromNotes(nil, nil), bodiesOnly{}, bodiesOnly{}, bodiesOnly{})
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
