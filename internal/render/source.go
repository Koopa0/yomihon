package render

import (
	"html"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

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
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, source)
	if err != nil {
		return plainSource(source)
	}
	var buf strings.Builder
	if err := chromaFormatter.Format(&buf, chromaStyle(), iterator); err != nil {
		return plainSource(source)
	}
	return buf.String()
}

// plainSource is the degraded rendering: still escaped, still readable, just
// uncolored. A file the highlighter chokes on must never fail the page.
func plainSource(source string) string {
	return "<pre><code>" + html.EscapeString(source) + "</code></pre>"
}
