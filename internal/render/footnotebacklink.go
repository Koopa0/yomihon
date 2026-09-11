package render

// goldmark's footnote backlink renderer writes the same arrow for every return
// of one footnote. Its only substitution for a per-return mark is RefCount,
// the total, so every arrow of a twice-cited note would still say the same
// thing. The ordinal that distinguishes them is RefIndex, which no option
// exposes. This renderer wins at a priority below the extension's 500 and
// writes that ordinal, plus a name that says which citation the return leads
// back to, only when there is more than one.

import (
	"fmt"
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
	if err := writeFootnoteBacklink(w, n); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}

func writeFootnoteBacklink(w util.BufWriter, n *east.FootnoteBacklink) error {
	if err := writeFootnoteBacklinkOpen(w, n); err != nil {
		return err
	}
	if n.RefCount > 1 {
		return writeNamedFootnoteBacklink(w, n)
	}
	_, err := w.WriteString(` role="doc-backlink">` + footnoteBacklinkHTML + `</a>`)
	return err
}

func writeFootnoteBacklinkOpen(w util.BufWriter, n *east.FootnoteBacklink) error {
	if _, err := w.WriteString(`&#160;<a href="#`); err != nil {
		return err
	}
	if _, err := w.Write(footnoteRegionPrefix(n)); err != nil {
		return err
	}
	if _, err := w.WriteString("fnref"); err != nil {
		return err
	}
	if n.RefIndex > 0 {
		if _, err := w.WriteString(strconv.Itoa(n.RefIndex)); err != nil {
			return err
		}
	}
	if _, err := w.WriteString(":"); err != nil {
		return err
	}
	if _, err := w.WriteString(strconv.Itoa(n.Index)); err != nil {
		return err
	}
	_, err := w.WriteString(`" class="footnote-backref"`)
	return err
}

func writeNamedFootnoteBacklink(w util.BufWriter, n *east.FootnoteBacklink) error {
	ordinal := strconv.Itoa(n.RefIndex + 1)
	name := html.EscapeString(footnoteBacklinkName(footnoteLang(n), n.RefIndex+1))
	if _, err := w.WriteString(` role="doc-backlink" aria-label="`); err != nil {
		return err
	}
	if _, err := w.WriteString(name); err != nil {
		return err
	}
	if _, err := w.WriteString(`">`); err != nil {
		return err
	}
	if _, err := w.WriteString(footnoteBacklinkHTML); err != nil {
		return err
	}
	if _, err := w.WriteString(ordinal); err != nil {
		return err
	}
	_, err := w.WriteString("</a>")
	return err
}

// footnoteBacklinkName is the accessible name of one return among several. The
// visible ordinal is only a digit; this is what says which citation it leads
// back to. A footnote cited once never asks, so these words never appear.
func footnoteBacklinkName(lang wording.Lang, ordinal int) string {
	return fmt.Sprintf(wording.FootnoteBacklinkFmt.In(lang), ordinal)
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
