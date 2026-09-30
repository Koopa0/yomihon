package report

import (
	"bytes"
	"regexp"
)

// linkOpener matches every place an HTML tokenizer opens a link element: the
// tag name in any letter case, followed by one of the characters that end a
// tag name. A carriage return is listed because the parser reads it as a line
// feed. The match is made over bytes, which a UTF-8 document shares with ASCII
// for every character in it, so no decoding can hide an opener from it; one
// found inside a comment, a script or a title is disarmed as well, which costs
// nothing there.
var linkOpener = regexp.MustCompile(`(?i)(<link)([\t\n\f\r />])`)

// inertLinks gives every link element in a briefing an empty rel ahead of any
// the author wrote. The report policy already refuses stylesheets, icons and
// fetches, but a link element can also ask the browser to fetch a page ahead
// of a visit — a prerender — and browsers make that request outside the
// document's own policy. With an empty rel first, the author's rel is a
// duplicate the parser discards, so the element names no relation and fetches
// nothing, while the document keeps the same elements in the same places.
//
// This is the one change the report route makes to a briefing's bytes, and it
// is made because the frame cannot refuse the request any other way. A link
// element does nothing else under this policy, so nothing the reader sees
// changes with it.
func inertLinks(doc []byte) []byte {
	return linkOpener.ReplaceAll(doc, []byte(`${1} rel=""${2}`))
}

// utf16Marked reports whether doc opens with a UTF-16 byte order mark. A
// browser obeys that mark over the charset the response declares, so such a
// document would be read as characters the byte match above never sees.
func utf16Marked(doc []byte) bool {
	return bytes.HasPrefix(doc, []byte{0xFE, 0xFF}) || bytes.HasPrefix(doc, []byte{0xFF, 0xFE})
}
