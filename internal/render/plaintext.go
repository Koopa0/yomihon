package render

// A note body's searchable plain text, walked off the same markdown engine the
// HTML pipeline uses. It lives here rather than beside its consumer because it
// needs this package's dialect passes, and a second copy of those would be free
// to disagree with the renderer about what a note says.

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// plainParser is a minimal goldmark parser used only to walk a note body for
// PlainText. Table and TaskList are on so cell and task text arrive as clean text
// nodes; linkify is off, so a bare URL stays a plain text node indexed verbatim.
// Footnotes are on because a definition whose text has no spaces — every CJK one —
// otherwise parses as a link reference definition and no search could find it.
var plainParser = goldmark.New(goldmark.WithExtensions(extension.Table, extension.TaskList, extension.Footnote)).Parser()

// PlainText returns the searchable plain text of a note body: prose, headings,
// table cells, task text, code-fence contents and the base and reading of
// hand-written ruby, but not the HTML tags or the callout marker syntax. A
// wikilink contributes both its target and its display text. The body must arrive
// with its frontmatter removed, and the text keeps its case and Unicode form.
//
// A course branch's role declaration stays in the text, although the page takes
// it off the heading it shows. Keeping it is what lets an author search for the
// notes that declare one; taking it off would mean assembling each heading's
// words here and reading a declaration back out of them, which is the page's own
// job and not this walk's. The cost is one incongruity: a search for the
// declaration finds a note whose page no longer shows those words.
func PlainText(body string) string {
	src := []byte(plainPreprocess(body))
	doc := plainParser.Parse(text.NewReader(src))

	var b strings.Builder
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		return walkPlain(&b, n, entering, src)
	}); err != nil {
		// Unreachable: walkPlain never returns a non-nil error. If a future
		// goldmark change ever makes Walk itself fail, fall back to the raw
		// body (whitespace-collapsed) so a note is never left unsearchable.
		return strings.Join(strings.Fields(body), " ")
	}
	return strings.TrimSpace(b.String())
}

// plainPreprocess rewrites the two Obsidian-dialect constructs goldmark has no
// concept of into plain text before parsing: a wikilink or embed becomes "target
// display", both, so a filename search hits through a display alias, and a
// callout marker line loses its marker while keeping the title. It is
// fence-aware, so a link written inside a code sample stays literal.
func plainPreprocess(body string) string {
	// The retrieval projections report nothing: a corpus entry is not a page,
	// and a fault in a note is the reading page's news to break.
	body, _ = stripObsidianComments(body)
	lines := strings.Split(body, "\n")
	inFence := false
	var fenceByte byte
	for i, line := range lines {
		switch {
		case inFence:
			if fenceCloses(line, fenceByte) {
				inFence = false
			}
		default:
			if marker, _, ok := fenceOpen(line); ok {
				inFence, fenceByte = true, marker
			} else {
				lines[i] = plainLine(line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// plainLine normalizes one non-fence line: it strips a callout marker (keeping
// the title) and rewrites wikilinks to plain "target display" text.
func plainLine(line string) string {
	if m := calloutStartPattern.FindStringSubmatch(line); m != nil {
		// Drop the marker, keep the callout's title. The body lines that follow
		// keep their quote marker and are collected as ordinary quoted text.
		line = m[3]
	}
	return replaceWikilinksPlain(line)
}

// replaceWikilinksPlain replaces every [[...]]/![[...]] token in line with its
// clean target and display text (deduplicated when they are equal), using the
// same target/display split the renderer uses (graph.SplitWikilink).
func replaceWikilinksPlain(line string) string {
	return wikilinkToken.ReplaceAllStringFunc(line, func(token string) string {
		inner := strings.TrimPrefix(token, "!")
		inner = inner[2 : len(inner)-2] // strip the enclosing "[[" and "]]"
		target, display, ok := graph.SplitWikilink(inner)
		switch {
		case !ok:
			return display // e.g. [[#heading]] — a same-file anchor: display only
		case target == display:
			return target
		default:
			return target + " " + display
		}
	})
}

// walkPlain appends one AST node's contribution to b. It never returns an
// error (the ast.Walk error path in PlainText is therefore unreachable).
func walkPlain(b *strings.Builder, n ast.Node, entering bool, source []byte) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	switch n.Kind() {
	case ast.KindRawHTML, ast.KindHTMLBlock:
		// The tags are not content. Text between them arrives as separate text
		// nodes rather than children, so skipping here drops only the tags.
		return ast.WalkSkipChildren, nil
	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		// Code content lives in the node's line segments, not in child Text
		// nodes; write it directly and do not descend — code contents are
		// searchable (people search for code snippets).
		writeSeparator(b)
		writeBlockLines(b, n, source)
		return ast.WalkSkipChildren, nil
	case ast.KindText:
		if t, ok := n.(*ast.Text); ok {
			b.Write(t.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte('\n')
			}
		}
	case ast.KindString:
		if s, ok := n.(*ast.String); ok {
			b.Write(s.Value)
		}
	case ast.KindAutoLink:
		if a, ok := n.(*ast.AutoLink); ok {
			b.Write(a.URL(source))
		}
	default:
		if n.Type() == ast.TypeBlock {
			// Separate block-level text so tokens from adjacent blocks (a
			// heading then its paragraph) do not run together.
			writeSeparator(b)
		}
	}
	return ast.WalkContinue, nil
}

// writeSeparator appends a newline unless b is empty or already ends in one.
func writeSeparator(b *strings.Builder) {
	if b.Len() == 0 {
		return
	}
	s := b.String()
	if s[len(s)-1] != '\n' {
		b.WriteByte('\n')
	}
}

// writeBlockLines appends the raw source of a node's line segments (used for
// code blocks, whose content is not held as child text nodes).
func writeBlockLines(b *strings.Builder, n ast.Node, source []byte) {
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
}
