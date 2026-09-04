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

// unanchorableLine reports whether a line is one no anchor can survive on, and so
// carries no block address at all. The excerpt scan asks the same question, so a
// link never writes a fragment for a place the page left unmarked. Both entries
// are lines something downstream takes apart: a callout's opening line, which is
// consumed as the block's title, and a table row, which is cut into cells against
// its header's column count and drops whatever follows the last.
func unanchorableLine(line string) bool {
	if typ, _, _, ok := calloutStart(line); ok {
		if bucket, _ := calloutBucketOf(typ); bucket != bucketUnknown {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimLeft(quotePrefix.ReplaceAllString(line, ""), " \t"), "|")
}

// markBlockAnchor gives the address at the end of line an anchor a browser can
// scroll to, leaving every visible character where it was: the marker keeps the
// author's capitals and only the id is folded. A page carries one anchor per
// address, and a repeated name stays with the first block, which is what the
// excerpt scan and a browser would both do anyway.
func markBlockAnchor(line string, page *composition, inline *[]string) string {
	trimmed := strings.TrimRight(line, " \t")
	m := blockMarkerTail.FindStringSubmatchIndex(trimmed)
	if m == nil {
		return line
	}
	address := trimmed[m[2]:m[3]]
	if !page.claimBlockAnchor(blockAnchorID(address)) {
		return line
	}
	anchor := `<span id="` + html.EscapeString(blockAnchorID(address)) + `">` +
		html.EscapeString(address) + `</span>`
	*inline = append(*inline, anchor)
	return trimmed[:m[2]] + placeholderFor(len(*inline)-1, anchor) + line[len(trimmed):]
}
