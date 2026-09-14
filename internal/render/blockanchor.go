package render

import (
	"html"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

// A block address is the "^name" an author writes at the end of a line to give
// that block a name a link can reach, and Obsidian scrolls such a link to it. The
// anchor goes on the address itself rather than on the element around it: the
// excerpt scan finds a marker by reading lines, so every line it matches is a line
// this pass sees. The id keeps the caret, which no heading anchor can collide with.

// blockMarkerTail matches the address a line ends with: a caret opening a word,
// taking the rest of the line with it. A caret glued to the end of a word is part
// of that word, the same reading the excerpt scan takes.
var blockMarkerTail = regexp.MustCompile(`(?:\A|[ \t])(\^\S+)\z`)

// blockAnchorID is the single definition of the id a block address makes: the
// address folded the way both kinds of fragment fold, so capitals and Unicode
// form never keep an address from its marker.
func blockAnchorID(address string) string {
	return graph.FoldFragment(address)
}

// UnanchorableLine reports whether a line is one no block address can survive
// on. The check face asks the same question, so a link never writes a fragment
// for a place the page left unmarked. Both entries are lines something
// downstream takes apart: a recognised callout's opening line, which is
// consumed as the block's title, and a table row, which is cut into cells
// against its header's column count and drops whatever follows the last. An
// unknown callout type is a blockquote, not a callout, and can carry an
// address. The type set is calloutVocabulary; a second copy is how a title
// became an address on one face and missing on the other.
func UnanchorableLine(line string) bool {
	if typ, _, _, ok := calloutStart(line); ok {
		if bucket, _ := calloutBucketOf(typ); bucket != bucketUnknown {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimLeft(quotePrefix.ReplaceAllString(line, ""), " \t"), "|")
}

// CodeSpanOwnsBlockAddress reports whether the caret lines[at] would take as a
// block address sits inside a code span. A caret there is the author
// showing an expression, not naming a block. The CommonMark spec lets a line
// ending stand inside a code span, so the span is looked for over the run of
// lines goldmark reads with that one rather than over that line alone: an
// expression the author wrapped is still one span, and its closing backtick is
// not a delimiter this pass may eat. The three readers of a block address ask
// this together so a link, an excerpt, and the page's ids stay on one answer.
// Indented code and fences are not this question: those have their own
// readings already.
//
// The run ends at a blank line and at every other line that ends a block, so
// two stray backticks an author wrote on either side of a heading, a list
// marker, a quote or callout opener, a thematic break or a fence never pair
// into a span goldmark would not draw, and the ordinary paragraph between them
// keeps its address on all three faces.
func CodeSpanOwnsBlockAddress(lines []string, at int) bool {
	trimmed := strings.TrimRight(lines[at], " \t")
	m := blockMarkerTail.FindStringSubmatchIndex(trimmed)
	if m == nil {
		return false
	}
	start, end := at, at+1
	for start > 0 && !inlineRunBreaks(lines[start-1], lines[start]) {
		start--
	}
	for end < len(lines) && !inlineRunBreaks(lines[end-1], lines[end]) {
		end++
	}
	off := len(strings.Join(lines[start:at+1], "\n")) - len(lines[at])
	return withinAny(codeSpanRanges(strings.Join(lines[start:end], "\n")), off+m[2], off+m[3])
}

// inlineRunBreaks reports whether two adjacent lines, above then below, reach
// goldmark in separate blocks, so a code span opened on one cannot close on
// the other. A blank line separates them, and so does a line that is a block
// of its own on either side: an ATX heading, the line underlining one, a
// thematic break, a fence opening or closing. A list marker or a quote opener
// separates them only as the lower line, since CommonMark lets the line under
// one continue it lazily. The quote marker comes off first, so a heading
// written inside a callout separates them there too. Indented code and
// authored markup keep the blindness they already had.
func inlineRunBreaks(above, below string) bool {
	if graph.BlankLine(above) || graph.BlankLine(below) {
		return true
	}
	if graph.QuotedLine.MatchString(below) && !graph.QuotedLine.MatchString(above) {
		return true
	}
	above, below = quotePrefix.ReplaceAllString(above, ""), quotePrefix.ReplaceAllString(below, "")
	if graph.ListItemLine.MatchString(below) {
		return true
	}
	for _, line := range []string{above, below} {
		_, _, fence := graph.FenceOpens(line)
		if fence || graph.ATXHeading.MatchString(line) ||
			graph.SetextUnderline.MatchString(line) || graph.BreakRuleLine.MatchString(line) {
			return true
		}
	}
	return false
}

// markBlockAnchor gives the address at the end of line an anchor a browser can
// scroll to, leaving every visible character where it was: the marker keeps the
// author's capitals and only the id is folded. A page carries one anchor per
// address, and a repeated name stays with the first block, which is what the
// excerpt scan and a browser would both do anyway. claim is whether this line
// is the note's own text; a transcluded body still wraps a classified address
// so speech can see it, but never takes the id. The span it plants is the
// signal the speech pass reads. A caret a code span owns never arrives here:
// that question is asked of the source the author wrote, which the scan holds
// and this already-converted line no longer is.
func markBlockAnchor(line string, page *composition, inline *[]string, claim bool) string {
	trimmed := strings.TrimRight(line, " \t")
	m := blockMarkerTail.FindStringSubmatchIndex(trimmed)
	if m == nil {
		return line
	}
	address := trimmed[m[2]:m[3]]
	id := blockAnchorID(address)
	var anchor string
	if claim && page.claimBlockAnchor(id) {
		anchor = `<span id="` + html.EscapeString(id) + `">` +
			html.EscapeString(address) + `</span>`
	} else {
		// The same classified tail, without an id: a duplicate name, or an
		// address that belongs to the note it was transcluded from. Speech
		// reads the span, not a flattened caret word, so an escaped or
		// entity-spelled caret that goldmark later draws the same way is left
		// alone.
		anchor = `<span>` + html.EscapeString(address) + `</span>`
	}
	*inline = append(*inline, anchor)
	return trimmed[:m[2]] + placeholderFor(len(*inline)-1, anchor) + line[len(trimmed):]
}
