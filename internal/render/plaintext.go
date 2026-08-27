package render

// This file extracts a note body's searchable plain text by walking a
// goldmark AST — reusing the same markdown engine the HTML pipeline uses
// rather than hand-rolling a second parser that could drift from it.
// internal/search consumes PlainText; it lives here because deriving
// plain text from a body is a rendering-adjacent concern that needs render's
// dialect helpers (the wikilink and callout preprocessing goldmark has no
// concept of), and duplicating those in search would be a second
// implementation of the dialect, free to disagree with the renderer.

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// plainParser is a minimal goldmark parser used only to walk a note body for
// PlainText. It enables Table and TaskList so table-cell and task text arrive
// as clean text nodes, but deliberately NOT Linkify (a bare URL then stays a
// plain Text node, indexed verbatim, matching what ripgrep sees in the file)
// and NOT the HTML pipeline's own highlight/chroma renderers (PlainText walks
// the parsed AST, it never renders HTML). Its Parser is safe for concurrent
// use — each Parse call builds its own context.
//
// Footnotes are enabled because the reading page shows them. Without this the
// two passes read one line two different ways: a definition whose text has no
// spaces in it — every CJK one — is a valid link reference definition, so the
// plain pass swallowed it and a note could display a sentence that no search
// for it would ever find.
var plainParser = goldmark.New(goldmark.WithExtensions(extension.Table, extension.TaskList, extension.Footnote)).Parser()

// PlainBlock is one top-level markdown block's searchable text. Code reports
// whether it came from a fenced or indented code block so a downstream bounded
// chunker can preserve code-line boundaries without parsing Markdown again.
type PlainBlock struct {
	Text string
	Code bool
}

// PlainSection is the preamble or one heading-bounded section in document
// order. Headings is the active document heading path. Text and Blocks use the
// same extraction rules as PlainText; heading scaffolding itself is carried in
// Headings rather than duplicated into Text.
type PlainSection struct {
	Headings []string
	Text     string
	Blocks   []PlainBlock
}

type headingFrame struct {
	level int
	text  string
}

// PlainSections returns the preamble followed by every heading-bounded section
// in body. Empty sections are retained so the semantic chunker, not the
// Markdown layer, owns the explicit drop rule. Frontmatter must already be
// removed, exactly as for PlainText.
func PlainSections(body string) []PlainSection {
	src := []byte(plainPreprocess(body))
	doc := plainParser.Parse(text.NewReader(src))

	sections := make([]PlainSection, 0, 1)
	current := PlainSection{}
	var headings []headingFrame
	flush := func() {
		parts := make([]string, 0, len(current.Blocks))
		for _, block := range current.Blocks {
			parts = append(parts, block.Text)
		}
		current.Text = strings.TrimSpace(strings.Join(parts, "\n"))
		sections = append(sections, current)
	}

	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*ast.Heading); ok {
			flush()
			for len(headings) > 0 && headings[len(headings)-1].level >= heading.Level {
				headings = headings[:len(headings)-1]
			}
			headings = append(headings, headingFrame{level: heading.Level, text: plainNode(node, src)})
			path := make([]string, len(headings))
			for i, frame := range headings {
				path[i] = frame.text
			}
			current = PlainSection{Headings: path}
			continue
		}

		blockText := plainNode(node, src)
		if blockText == "" {
			continue
		}
		current.Blocks = append(current.Blocks, PlainBlock{
			Text: blockText,
			Code: node.Kind() == ast.KindFencedCodeBlock || node.Kind() == ast.KindCodeBlock,
		})
	}
	flush()
	return sections
}

// PlainText returns the searchable plain text of a note body: body
// text, headings, table cells,
// task text, code-fence contents, and the base+rt of hand-written <ruby>
// markup are included; the HTML tags themselves and the callout-marker syntax
// ("> [!type]") are not. A wikilink [[target|display]] contributes both its
// target and its display text, so searching a linked note's filename hits even
// when a display alias is what is shown.
//
// The body must already have its frontmatter removed (vault.Parse does this) —
// PlainText has no frontmatter concept; frontmatter's structured fields are
// indexed separately, not as body text.
// The text keeps its original case and Unicode form; internal/search applies
// NFC + case folding when it stores and matches.
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

func plainNode(node ast.Node, source []byte) string {
	var b strings.Builder
	if err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		return walkPlain(&b, n, entering, source)
	}); err != nil {
		return ""
	}
	return strings.TrimSpace(b.String())
}

// plainPreprocess rewrites the two Obsidian-dialect constructs goldmark has no
// concept of into plain text before parsing, so the AST walk sees clean text:
// each [[wikilink]] / ![[embed]] becomes "target display" (both, so a filename
// search hits through a display alias), and a callout marker line
// "> [!type] Title" loses its "[!type]" marker while keeping the title. It is
// fence-aware — inside a fenced code block every line is preserved verbatim, so
// a [[link]] written inside a code sample stays literal and is indexed as code
// content, not resolved.
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
		// Drop the "> [!type][+-]" marker; keep only the callout's title text
		// (m[3]). The marker syntax itself is not searchable content; the
		// callout body lines that follow keep their "> " and are collected as
		// ordinary blockquote text.
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
		// The HTML tags themselves are not content. Any inline text
		// between them — e.g. the base and rt of a <ruby>…<rt>… run — are
		// separate Text nodes collected by the KindText case, not children of
		// these nodes, so skipping here drops only the tags.
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
