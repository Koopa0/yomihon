package nav

import (
	"bytes"
	"cmp"
	"strings"

	"golang.org/x/net/html"
)

// HTMLTitleBytes is how much of an HTML file is read to learn what it calls
// itself. A name lives at the top of a document, so a file that has not given
// one by then is called by its file name instead of being read to its end —
// and a briefing carries its fonts and pictures inline, which is most of what
// lies past this.
const HTMLTitleBytes = 64 << 10

// HTMLTitle is what an HTML document calls itself: the words of its <title>,
// else the words of its first <h1>, else nothing, and the caller then names it
// by its file. Only the first HTMLTitleBytes of document are looked at.
//
// The document is tokenized and nothing more. No tree is built, no script runs
// and nothing is fetched, so a hostile file can cost this the time to read a
// window of bytes and no more. A <title> is text to the tokenizer, which is also
// what a browser makes of it: markup inside one is characters, and a character
// reference is decoded exactly once. The caller prints the result through a
// template that escapes it, so decoding a second time here would hand a
// literal "&lt;" in the author's title back to the page as a tag.
//
// An element that is still open when the window ends is not a name: its words
// would run to wherever the read stopped, and a title cut off by the bound is
// not the title. A <title> inside an <svg> or <math> element is that element's
// own label, not the document's — HTML says the document's title is the first
// one in its own namespace — so it is passed over. Only the first <title>
// answers, even when empty, because a later one is not the document's either.
//
// Words are whitespace-collapsed the way a browser's tab collapses them: runs
// of ASCII whitespace become one space and the ends are trimmed. A no-break
// space is a character of the title and stays.
func HTMLTitle(document []byte) string {
	window := document[:min(len(document), HTMLTitleBytes)]
	return cmp.Or(titleElementWords(window), firstHeadingWords(window))
}

// titleElementWords is the words of the document's own <title>: the first one
// outside an svg or math element, empty when it is empty, still open at the end
// of window, or not there at all.
func titleElementWords(window []byte) string {
	z := html.NewTokenizer(bytes.NewReader(window))
	foreign := 0
	for {
		switch z.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "svg", "math":
				foreign++
			case "title":
				if foreign == 0 {
					words, _ := elementWords(z, "title")
					return words
				}
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if foreign > 0 && (string(name) == "svg" || string(name) == "math") {
				foreign--
			}
		default:
			// Text, comments, doctypes and self-closing tags name nothing here.
		}
	}
}

// firstHeadingWords is the words of the first <h1> in window that closes inside
// it and has any.
func firstHeadingWords(window []byte) string {
	z := html.NewTokenizer(bytes.NewReader(window))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return ""
		case html.StartTagToken:
			if name, _ := z.TagName(); string(name) == "h1" {
				if words, closed := elementWords(z, "h1"); closed && words != "" {
					return words
				}
			}
		default:
			// Everything else is not a heading.
		}
	}
}

// elementWords reads the tokenizer from just after element's start tag to its
// end tag and returns the text between, collapsed. closed is false when the
// input ended first. A script or style inside the element is not words and its
// text is skipped; a line break stands for the space it separates.
func elementWords(z *html.Tokenizer, element string) (words string, closed bool) {
	var text strings.Builder
	hidden := ""
	for {
		switch z.Next() {
		case html.ErrorToken:
			return "", false
		case html.TextToken:
			if hidden == "" {
				text.Write(z.Text())
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "br":
				text.WriteByte(' ')
			case "script", "style":
				if hidden == "" {
					hidden = string(name)
				}
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch {
			case hidden != "":
				if string(name) == hidden {
					hidden = ""
				}
			case string(name) == element:
				return collapseSpace(text.String()), true
			}
		default:
			// Comments and doctypes carry no words of the element.
		}
	}
}

// collapseSpace turns each run of ASCII whitespace into one space and trims
// the ends. HTML's whitespace is exactly space, tab, line feed, form feed and
// carriage return; strings.Fields would also split on a no-break space, which
// the title means.
func collapseSpace(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\f' || r == '\r'
	}), " ")
}
