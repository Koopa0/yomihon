package render

// Every GFM table is wrapped in a horizontally scrollable container, so a table
// wider than the reading column scrolls in its own box rather than stretching the
// article. Only the table element renderer is overridden; the element stays a
// real <table>, so its role survives in the accessibility tree.

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// tableWrapClass is the scroll container the reading stylesheet gives an
// overflow rule; the same name lives there.
const tableWrapClass = "y-tablewrap"

// tableWrapRenderer overrides goldmark's <table> element rendering to nest it
// in a scroll container. Only the table element is overridden; its header,
// rows, and cells are still emitted by goldmark's default table renderers.
type tableWrapRenderer struct{}

func (tableWrapRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(east.KindTable, renderWrappedTable)
}

// renderWrappedTable emits goldmark's own <table> open and close nested in the
// scroll container. The pipeline enables no attribute parser, so the open tag is
// the bare <table> goldmark itself would write.
func renderWrappedTable(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	var err error
	if entering {
		_, err = w.WriteString(`<div class="` + tableWrapClass + `"><table>` + "\n")
	} else {
		_, err = w.WriteString("</table></div>\n")
	}
	return ast.WalkContinue, err
}

// tableWrapExtension registers tableWrapRenderer, overriding goldmark's GFM
// table element renderer (priority 500) with a lower number so this one wins.
type tableWrapExtension struct{}

func (tableWrapExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(tableWrapRenderer{}, 200),
	))
}
