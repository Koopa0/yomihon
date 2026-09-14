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
// non-blank lines that line belongs to rather than over that line alone: an
// expression the author wrapped is still one span, and its closing backtick is
// not a delimiter this pass may eat. The three readers of a block address ask
// this together so a link, an excerpt, and the page's ids stay on one answer.
// Indented code and fences are not this question: those have their own
// readings already.
//
// One blindness is recorded rather than fixed: the run is bounded by blank
// lines and by nothing else, so two stray backticks an author wrote on either
// side of a fence, a heading, or a list marker with no blank line between them
// pair here into a span goldmark would never draw, and an address caught
// between them is left unmarked on all three faces.
func CodeSpanOwnsBlockAddress(lines []string, at int) bool {
	trimmed := strings.TrimRight(lines[at], " \t")
	m := blockMarkerTail.FindStringSubmatchIndex(trimmed)
	if m == nil {
		return false
	}
	start, end := at, at+1
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
		end++
	}
	off := len(strings.Join(lines[start:at+1], "\n")) - len(lines[at])
	return withinAny(codeSpanRanges(strings.Join(lines[start:end], "\n")), off+m[2], off+m[3])
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
