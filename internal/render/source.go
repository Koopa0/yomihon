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
// file gets an information page pointing at its bytes and search leaves it out,
// so both faces say the same thing. A note is not subject to this.
const MaxSourceBytes = 1 << 20 // 1 MiB

// pictureExts are the kinds an <img> element displays. The extension, not the
// bytes, chooses the viewer; the bytes only decide text from binary. SVG is why
// more than one caller asks: its bytes are characters, so only the name says the
// reader will be shown a drawing rather than words they could look for.
var pictureExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true,
}

// IsPicture reports whether relPath names a file shown as a picture.
func IsPicture(relPath string) bool {
	return pictureExts[strings.ToLower(path.Ext(relPath))]
}

// IsPDF reports whether relPath names a PDF, by its final extension in any case;
// like IsPicture, the name alone chooses the viewer. Search leaves a PDF out of
// the text index, since its bytes are never shown as characters.
func IsPDF(relPath string) bool {
	return strings.EqualFold(path.Ext(relPath), ".pdf")
}

// IsText reports whether b is text yomihon reads as characters: no NUL byte and
// valid UTF-8. The extension is deliberately not consulted, so a .txt holding a
// compiled object stays out and an extensionless build file still reads as what
// it is. b must not end mid-character; pass a truncated window to IsTextPrefix.
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

// lexerAliases teach the highlighter the file kinds carrying a known syntax under
// a name chroma does not recognize: Obsidian's canvas is JSON and its base is
// YAML. A kind whose bytes are genuinely bespoke is absent, since plain source is
// its honest rendering.
var lexerAliases = map[string]string{
	".canvas": "JSON",
	".base":   "YAML",
}

// SourceHTML highlights a whole file as source text, returning the same
// class-based markup a fenced code block produces. filename is the base name and
// selects the lexer by name as well as extension, which is how an extensionless
// build file reads as what it is; an unrecognized name falls back to plain text.
// The formatter escapes the source, so a file carrying markup is shown, not run.
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

// lexerFor picks a highlighter: a taught alias first, so a bespoke extension
// resolves to the syntax it carries rather than to whatever chroma guesses from a
// substring, then chroma's own match on the name, then plain text.
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

// plainSource is the degraded rendering: escaped, readable, uncoloured, in the
// same container the highlighter emits, so the block is dressed identically
// whether or not any token was coloured.
func plainSource(source string) string {
	return `<pre class="chroma"><code>` + html.EscapeString(source) + "</code></pre>"
}
