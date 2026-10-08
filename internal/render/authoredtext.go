package render

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type authoredTextExtension struct{}

func (authoredTextExtension) Extend(md goldmark.Markdown) {
	r := &authoredTextRenderer{}
	goldmarkhtml.NewRenderer().RegisterFuncs(r)
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(r, 600)))
}

type authoredTextRenderer struct{ renderText renderer.NodeRendererFunc }

func (r *authoredTextRenderer) Register(kind ast.NodeKind, fn renderer.NodeRendererFunc) {
	if kind == ast.KindText {
		r.renderText = fn
	}
}
func (r *authoredTextRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindText, r.renderAuthoredText)
}

// Entity decoding is where authored numeric references could acquire the
// renderer's private delimiters. Literal code and genuine planted markers keep
// their bytes; an escaped ampersand never becomes a numeric reference here.
func (r *authoredTextRenderer) renderAuthoredText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	leaf, ok := node.(*ast.Text)
	if !ok || !entering || leaf.IsRaw() || leaf.Segment.Padding != 0 || leaf.Segment.ForceNewline {
		return r.renderText(w, source, node, entering)
	}
	value := leaf.Segment.Value(source)
	if bytes.IndexByte(value, '&') < 0 {
		return r.renderText(w, source, node, entering)
	}
	var kept strings.Builder
	last := 0
	for start := 0; start < len(value); {
		end := start + roleSourceUnitWidth(value[start:])
		decoded := util.ResolveNumericReferences(value[start:end])
		if value[start] == '&' && strings.ContainsAny(string(decoded), inlinePlaceholderRunes) {
			kept.Write(value[last:start])
			last = end
		}
		start = end
	}
	if last == 0 {
		return r.renderText(w, source, node, entering)
	}
	kept.Write(value[last:])
	clean := []byte(kept.String())
	cleanLeaf := *leaf
	cleanLeaf.Segment = text.NewSegment(0, len(clean))
	return r.renderText(w, clean, &cleanLeaf, entering)
}
