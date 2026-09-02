package render

import (
	"html"
	"regexp"
	"strings"
)

// A block address is the "^name" an author writes at the end of a line to give
// that one block a name a link can reach — "[[note#^name]]". Obsidian scrolls
// such a link to the block it names; without an anchor on the destination the
// reader was told which paragraph they were going to and arrived at the top of
// the note, with nothing on screen saying the address had been dropped.
//
// The anchor is placed on the address itself rather than on the element around
// it, and that is what makes it dependable: the excerpt scan an embed uses
// finds a marker by reading lines, so every line it can match is a line this
// pass sees too, and a link's fragment therefore never names a place the page
// left unmarked. Hanging the anchor on the surrounding element instead would
// have meant asking what element a line became — a question with a different
// answer for a heading, a table row, and an indented code line, and a wrong
// fragment for each one the two answers disagreed about.
//
// The id keeps the caret the author wrote. Nothing else on the page can
// collide with it: a heading's anchor is built by dropping everything that is
// not a letter or a digit, so no heading can ever produce a name opening with
// one.

// blockMarkerTail matches the address a line ends with: a caret that opens a
// word, taking everything to the end of the line with it. A caret glued to the
// end of a word is part of that word and not an address, which is the same
// reading the excerpt scan takes.
var blockMarkerTail = regexp.MustCompile(`(?:\A|[ \t])(\^\S+)\z`)

// blockAnchorID is the single definition of the id a block address makes: the
// address folded the way both kinds of fragment fold, so an address written in
// one set of capitals reaches a marker written in another, and a name that
// arrived decomposed from the filesystem reaches one composed by an editor.
func blockAnchorID(address string) string {
	return foldFragment(address)
}

// unanchorableLine reports whether a line is one no anchor can survive on, and
// which therefore carries no block address at all. The excerpt scan asks the
// same question, so a link never writes a fragment for a place the page left
// unmarked. Both entries are lines whose text is taken apart by something
// downstream of the marking:
//
//   - A callout's opening line is that callout's title. The whole block is
//     consumed as one and the title never runs through the line passes, so no
//     anchor is made there. A callout of a type this renderer does not know is
//     not consumed, and its opening line is ordinary prose.
//   - A table row is cut into cells against the column count its header
//     declares, and whatever follows the last of them is dropped — the address
//     and its anchor with it. A leading pipe is the shape of that syntax; a
//     paragraph opening with one is rare enough to be worth the same caution,
//     since refusing costs an address and allowing invents one.
func unanchorableLine(line string) bool {
	if typ, _, _, ok := calloutStart(line); ok {
		if bucket, _ := calloutBucketOf(typ); bucket != bucketUnknown {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimLeft(quotePrefix.ReplaceAllString(line, ""), " \t"), "|")
}

// markBlockAnchor gives the address at the end of line an anchor the reader's
// browser can scroll to, leaving every visible character exactly where it was:
// the marker keeps the capitals the author typed, and only the id it answers to
// is folded.
//
// A page carries one anchor per address. When the same name is written twice
// the first block keeps it, which is the reading the excerpt scan already takes
// for a repeated address and the one a browser would take anyway for a repeated
// id; the second block is left as the text it is rather than issuing a name two
// places answer to.
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
