package render

// goldmark's footnote backlink renderer writes the same arrow for every return
// of one footnote. Its only substitution for a per-return mark is RefCount,
// the total, so every arrow of a twice-cited note would still say the same
// thing. The ordinal that distinguishes them is RefIndex, which no option
// exposes. This renderer wins at a priority below the extension's 500 and
// writes that ordinal, plus a name that says which citation the return leads
// back to, only when there is more than one.

import (
	"html"
	"strconv"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/wording"
)

// footnoteBacklinkHTML is goldmark v1.8.6's default backlink body: a return
// arrow forced into text presentation so a colour emoji does not replace it.
const footnoteBacklinkHTML = "&#x21a9;&#xfe0e;"

// footnoteLangAttr names the document attribute carrying the language this
// render's own sentences are written in. The backlink renderer reads it so a
// return can name its destination in the language the page is already speaking.
const footnoteLangAttr = "yomihonFootnoteLang"

type footnoteBacklinkRenderer struct{}

func (footnoteBacklinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(east.KindFootnoteBacklink, renderFootnoteBacklink)
}

func renderFootnoteBacklink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n, ok := node.(*east.FootnoteBacklink)
	if !ok {
		return ast.WalkContinue, nil
	}
	if _, err := w.WriteString(`&#160;<a href="#`); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.Write(footnoteRegionPrefix(node)); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.WriteString("fnref"); err != nil {
		return ast.WalkStop, err
	}
	if n.RefIndex > 0 {
		if _, err := w.WriteString(strconv.Itoa(n.RefIndex)); err != nil {
			return ast.WalkStop, err
		}
	}
	if _, err := w.WriteString(":"); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.WriteString(strconv.Itoa(n.Index)); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.WriteString(`" class="footnote-backref"`); err != nil {
		return ast.WalkStop, err
	}
	if n.RefCount > 1 {
		ordinal := strconv.Itoa(n.RefIndex + 1)
		name := html.EscapeString(footnoteBacklinkName(footnoteLang(node), n.RefIndex+1))
		if _, err := w.WriteString(` role="doc-backlink" aria-label="`); err != nil {
			return ast.WalkStop, err
		}
		if _, err := w.WriteString(name); err != nil {
			return ast.WalkStop, err
		}
		if _, err := w.WriteString(`">`); err != nil {
			return ast.WalkStop, err
		}
		if _, err := w.WriteString(footnoteBacklinkHTML); err != nil {
			return ast.WalkStop, err
		}
		if _, err := w.WriteString(ordinal); err != nil {
			return ast.WalkStop, err
		}
		_, err := w.WriteString("</a>")
		return ast.WalkContinue, err
	}
	_, err := w.WriteString(` role="doc-backlink">` + footnoteBacklinkHTML + `</a>`)
	return ast.WalkContinue, err
}

// footnoteBacklinkName is the accessible name of one return among several. The
// visible ordinal is only a digit; this is what says which citation it leads
// back to. A footnote cited once never asks, so these words never appear.
func footnoteBacklinkName(lang wording.Lang, ordinal int) string {
	if lang == wording.En {
		return "Back to citation " + strconv.Itoa(ordinal)
	}
	return "返回第 " + strconv.Itoa(ordinal) + " 次引用"
}

func footnoteLang(n ast.Node) wording.Lang {
	doc := n.OwnerDocument()
	if doc == nil {
		return wording.ZhHant
	}
	value, ok := doc.AttributeString(footnoteLangAttr)
	if !ok {
		return wording.ZhHant
	}
	tag, ok := value.([]byte)
	if !ok {
		return wording.ZhHant
	}
	lang, known := wording.Known(string(tag))
	if !known {
		return wording.ZhHant
	}
	return lang
}

// footnoteBacklinkExtension overrides goldmark's footnote backlink renderer
// (priority 500) with a lower number so this one wins.
type footnoteBacklinkExtension struct{}

func (footnoteBacklinkExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(footnoteBacklinkRenderer{}, 200),
	))
}
