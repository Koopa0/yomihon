package render

import (
	"html"
	"path"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

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
	if err := chromaFormatter.Format(&buf, chromaStyle(), iterator); err != nil {
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
// uncolored. A file the highlighter chokes on must never fail the page.
func plainSource(source string) string {
	return "<pre><code>" + html.EscapeString(source) + "</code></pre>"
}
