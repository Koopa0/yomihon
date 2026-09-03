package render

// ==highlight== as a real goldmark inline extension rather than a pass over raw
// source: goldmark's inline-parsing trigger already skips code spans and respects
// nested formatting, so "==**bold**==" and an "==" beside a code span come out
// right for free. It is the same shape as goldmark's own strikethrough.

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// highlightNode is the AST node for one ==highlighted== span.
type highlightNode struct {
	ast.BaseInline
}

var kindHighlight = ast.NewNodeKind("Highlight")

func (n *highlightNode) Kind() ast.NodeKind { return kindHighlight }

func (n *highlightNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type highlightDelimiterProcessor struct{}

func (highlightDelimiterProcessor) IsDelimiter(b byte) bool { return b == '=' }

func (highlightDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

func (highlightDelimiterProcessor) OnMatch(int) ast.Node {
	return &highlightNode{}
}

var defaultHighlightDelimiterProcessor = highlightDelimiterProcessor{}

type highlightParser struct{}

var defaultHighlightParser = highlightParser{}

func (highlightParser) Trigger() []byte { return []byte{'='} }

// Parse recognizes exactly a double "==" run — Obsidian's highlight
// delimiter is always two characters, never one or three-plus.
func (highlightParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 2, defaultHighlightDelimiterProcessor)
	if node == nil || node.OriginalLength != 2 {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

type highlightHTMLRenderer struct{}

func (highlightHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindHighlight, renderHighlightNode)
}

func renderHighlightNode(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	var err error
	if entering {
		_, err = w.WriteString("<mark>")
	} else {
		_, err = w.WriteString("</mark>")
	}
	return ast.WalkContinue, err
}

// highlightExtension registers the ==highlight== parser and renderer
// into a goldmark.Markdown built by New.
type highlightExtension struct{}

func (highlightExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(defaultHighlightParser, 500),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(highlightHTMLRenderer{}, 500),
	))
}
