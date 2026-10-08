package render

// ==highlight== as a real goldmark inline extension rather than a pass over raw
// source: goldmark's inline-parsing trigger already skips code spans and respects
// nested formatting, so "==**bold**==" and an "==" beside a code span come out
// right for free. It is the same shape as goldmark's own strikethrough.

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/graph"
)

type highlightHTMLRenderer struct{}

func (highlightHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(graph.KindHighlight, renderHighlightNode)
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

// highlightExtension draws the shared body's recognized highlights.
type highlightExtension struct{}

func (highlightExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(highlightHTMLRenderer{}, 500),
	))
}
