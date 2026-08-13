package render

import (
	"bytes"
	"html"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v3"
	"github.com/alecthomas/chroma/v3/lexers"
)

// MaxSourceBytes is the comfort cap on a file read as characters. Past it the
// page would cost more to build and scroll than it is worth, so the file gets
// an information page pointing at its bytes instead, and search does not read
// it either — the reader is told the same thing by both faces. A note is not
// subject to this: markdown keeps the reading page at any size.
const MaxSourceBytes = 1 << 20 // 1 MiB

// pictureExts are the kinds an <img> element displays. A file's extension —
// not its bytes — chooses the viewer, because that is the only thing that says
// how the reader wants to see it; the bytes only decide text from binary.
//
// The list has more than one caller because of SVG: its bytes are characters,
// so nothing but the name says the reader will be shown a drawing rather than
// words they could look for. A term found inside one would point at a page
// where that term is nowhere on the screen.
var pictureExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true,
}

// IsPicture reports whether relPath names a file shown as a picture.
func IsPicture(relPath string) bool {
	return pictureExts[strings.ToLower(path.Ext(relPath))]
}

// IsText reports whether b is text yomihon reads as characters: no NUL byte,
// and valid UTF-8. The extension is deliberately not consulted — a .txt holding
// a compiled object must never be poured into a source page, and a build file
// with no extension at all must still read as what it is.
//
// b must not end mid-character; pass a truncated sniff window to IsTextPrefix.
func IsText(b []byte) bool {
	return bytes.IndexByte(b, 0) < 0 && utf8.Valid(b)
}

// IsTextPrefix is IsText over a fixed-size window, ignoring a character the
// window cut in half, so a text file does not read as binary merely because its
// opening bytes ended in the middle of one.
func IsTextPrefix(b []byte) bool {
	return IsText(trimPartialRune(b))
}

// trimPartialRune drops a multi-byte character that a fixed-size sniff window
// cut in half.
func trimPartialRune(b []byte) []byte {
	for i := len(b) - 1; i >= 0 && i >= len(b)-3; i-- {
		if b[i]&0xC0 == 0x80 {
			continue // a continuation byte: the lead is further back
		}
		if b[i]&0x80 == 0 {
			return b // plain ASCII: nothing was cut
		}
		var size int
		switch {
		case b[i]&0xE0 == 0xC0:
			size = 2
		case b[i]&0xF0 == 0xE0:
			size = 3
		case b[i]&0xF8 == 0xF0:
			size = 4
		default:
			return b // not a lead byte at all; let the UTF-8 check reject it
		}
		if i+size > len(b) {
			return b[:i]
		}
		return b
	}
	return b
}

// lexerAliases teach the highlighter the file kinds that carry a known syntax
// under a name chroma does not recognize. Obsidian's canvas is a JSON document
// and its base is a YAML one; without this each would fall back to plain text.
// A kind whose bytes are genuinely opaque or bespoke (a .d2 diagram, say) is
// deliberately absent — plain source is its honest rendering.
var lexerAliases = map[string]string{
	".canvas": "JSON",
	".base":   "YAML",
}

// SourceHTML highlights a whole file as source text, returning the same
// class-based markup a fenced code block produces, so one stylesheet dresses
// both. filename — the base name, not the vault path — selects the lexer:
// chroma matches on the name as well as the extension, which is the only way a
// build file carrying no extension at all reads as what it is. A name chroma
// does not recognize falls back to its plain-text lexer, so an unknown kind
// renders uncolored rather than not at all.
//
// The source is escaped by the formatter on the way out, and this function is
// the only path a non-markdown file's bytes take into a first-party page: a
// vault file that happens to contain markup is shown, never executed.
func SourceHTML(filename, source string) string {
	lexer := lexerFor(filename)
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, source)
	if err != nil {
		return plainSource(source)
	}
	var buf strings.Builder
	if err := chromaFormatter.Format(&buf, markupStyle(), iterator); err != nil {
		return plainSource(source)
	}
	return buf.String()
}

// lexerFor picks a highlighter for a file: a taught alias for a kind chroma
// does not know by name, then chroma's own match on the name, then plain text.
// The alias comes first so a bespoke extension resolves to the syntax it
// actually carries rather than to whatever chroma might guess from a substring.
func lexerFor(filename string) chroma.Lexer {
	if name, ok := lexerAliases[strings.ToLower(path.Ext(filename))]; ok {
		if l := lexers.Get(name); l != nil {
			return l
		}
	}
	if l := lexers.Match(filename); l != nil {
		return l
	}
	return lexers.Fallback
}

// plainSource is the degraded rendering: still escaped, still readable, just
// uncolored. A file the highlighter chokes on must never fail the page. It
// keeps the same container the highlighter emits, so the block a reader sees
// is dressed identically whether or not any token was ever coloured.
func plainSource(source string) string {
	return `<pre class="chroma"><code>` + html.EscapeString(source) + "</code></pre>"
}
