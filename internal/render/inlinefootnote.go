package render

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// inlineFootnote records a source span without rendering it. Goldmark decides
// where inline syntax is legal, so code, escapes and HTML attributes do not
// become notes. The second parse uses ordinary footnotes for numbering and ids.
type inlineFootnote struct {
	ast.BaseInline

	span    text.Segment
	content *text.Segments
}

var kindInlineFootnote = ast.NewNodeKind("InlineFootnote")

func (*inlineFootnote) Kind() ast.NodeKind { return kindInlineFootnote }

func (n *inlineFootnote) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type inlineFootnoteParser struct{}

func (inlineFootnoteParser) Trigger() []byte { return []byte{'^'} }

func (inlineFootnoteParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, start := block.PeekLine()
	if !strings.HasPrefix(string(line), "^[") {
		return nil
	}
	block.Advance(2)
	content, found := block.FindClosure('[', ']', text.FindClosureOptions{
		CodeSpan: true, Nesting: true, Newline: true, Advance: true,
	})
	if !found || util.IsBlank(content.Value(block.Source())) {
		return nil
	}
	_, end := block.Position()
	return &inlineFootnote{span: text.NewSegment(start.Start, end.Start), content: content}
}

var inlineFootnoteRecognizer = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithInlineParsers(util.Prioritized(inlineFootnoteParser{}, 100))),
).Parser()

// expandInlineFootnotes translates Obsidian's inline form into ordinary
// references and definitions. Definitions precede the source so an unclosed
// code fence or HTML block at its end cannot swallow them. Their position does
// not number them: goldmark numbers references in the document's reading order.
func expandInlineFootnotes(body string) string {
	if !strings.Contains(body, "^[") {
		return body
	}
	source := []byte(body)
	doc := inlineFootnoteRecognizer.Parse(text.NewReader(source))
	var notes []*inlineFootnote
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if note, ok := n.(*inlineFootnote); ok && entering {
			notes = append(notes, note)
		}
		return ast.WalkContinue, nil
	}); err != nil || len(notes) == 0 {
		// The walk has no error-producing branch; preserving the source is
		// the safe fallback if the dependency ever introduces one.
		return body
	}
	slices.SortFunc(notes, func(a, b *inlineFootnote) int { return cmp.Compare(a.span.Start, b.span.Start) })
	// The internal labels must not borrow an authored definition, including
	// one the recognizer discarded because it had no reference yet.
	var definitions, prose strings.Builder
	start := 0
	serial := 1
	for _, note := range notes {
		label := "[^yomihon-inline-footnote-" + strconv.Itoa(serial) + "]"
		for strings.Contains(body, label) {
			serial++
			label = "[^yomihon-inline-footnote-" + strconv.Itoa(serial) + "]"
		}
		serial++
		definitions.WriteString(label + ": ")
		definitions.WriteString(strings.ReplaceAll(string(note.content.Value(source)), "\n", "\n    "))
		definitions.WriteString("\n\n")
		prose.WriteString(body[start:note.span.Start])
		prose.WriteString(label)
		start = note.span.Stop
	}
	prose.WriteString(body[start:])
	return definitions.String() + prose.String()
}
