package report

import (
	"bytes"
	"regexp"
)

// linkOpener matches every place an HTML tokenizer opens a link element: the
// tag name in any ASCII letter case, followed by one of the characters that
// end a tag name. The letters are spelled out rather than folded, since a
// folding match would also take the Kelvin sign for a k, which no tokenizer
// does. A carriage return is listed because the parser reads it as a line
// feed. The match is made over bytes, which a UTF-8 document shares with ASCII
// for every character in it, so no decoding can hide an opener from it.
var linkOpener = regexp.MustCompile(`(<[Ll][Ii][Nn][Kk])([\t\n\f\r />])`)

// inertRel is the relation every link element is given ahead of any its
// author wrote: an unquoted value no browser knows, closed by a space.
//
// The value carries no quote, so an opener that sits inside a quoted
// attribute value — a title that mentions a link element, say — leaves that
// value whole. The closing space keeps the value to that one character: an
// unquoted value runs on to the next space, and one run on through the
// author's "/" could pick up a character reference that decodes to a space
// and a relation of the author's choosing after it. A bare name with no value
// would do neither, since the author could then supply its value with an "=".
const inertRel = ` rel=_ `

// inertLinks gives every link element in a briefing the relation inertRel
// ahead of any the author wrote. The report policy already refuses
// stylesheets, icons and fetches, but a link element can also ask the browser
// to fetch a page ahead of a visit — a prerender — and browsers make that
// request outside the document's own policy. With inertRel first, the
// author's rel is a duplicate the parser discards, so the element names no
// relation a browser acts on and fetches nothing, while the document keeps the
// same elements in the same places.
//
// This is the one change the report route makes to a briefing's bytes, and it
// is made because the frame cannot refuse the request any other way. A link
// element does nothing else under this policy. A reader can see the change
// only where "<link" is text rather than a tag — a textarea, a title, an
// attribute value such as a tooltip, a string in a style sheet — which shows
// the inserted relation as text.
func inertLinks(doc []byte) []byte {
	return linkOpener.ReplaceAll(doc, []byte("${1}"+inertRel+"${2}"))
}

// utf16Marked reports whether doc opens with a UTF-16 byte order mark. A
// browser obeys that mark over the charset the response declares, so such a
// document would be read as characters the byte match above never sees.
func utf16Marked(doc []byte) bool {
	return bytes.HasPrefix(doc, []byte{0xFE, 0xFF}) || bytes.HasPrefix(doc, []byte{0xFF, 0xFE})
}
