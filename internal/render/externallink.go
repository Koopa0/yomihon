package render

// A link whose destination is an absolute http or https address leaves the
// library, and it opens in a tab of its own so the page being read stays where
// the reader left it. The anchor says so in three places that reach three
// readers: the attributes keep the destination from learning where it was
// opened from, the stylesheet draws an arrow after it, and a sentence carried
// out of sight names the behaviour for whoever is listening. A link to another
// note uses its resolved route, and an unusable Markdown path retains its
// label with an explanation. Other local links and wikilinks keep their
// existing rendering.

import (
	"fmt"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/wording"
)

// externalLinkAttributes are what a link leaving the library carries beyond its
// destination. The relation and the referrer policy are the ones a remote image
// is already written with; target and noopener are what opening it in another
// tab adds.
const externalLinkAttributes = ` target="_blank" rel="external noopener noreferrer" referrerpolicy="no-referrer"`

// leavesTheLibrary reports whether a destination is an absolute http or https
// address. The scheme is compared without regard to case and the rest of the
// address is not parsed: a destination that fails to parse is still one a
// browser follows, so it still leaves.
func leavesTheLibrary(destination []byte) bool {
	value := strings.ToLower(strings.TrimSpace(string(destination)))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// defaultRenderers collects the functions goldmark's own HTML renderer
// registers, so that a link which does not leave the library is written by the
// code that wrote it before this one existed rather than by a copy of it.
type defaultRenderers map[ast.NodeKind]renderer.NodeRendererFunc

func (d defaultRenderers) Register(kind ast.NodeKind, fn renderer.NodeRendererFunc) {
	d[kind] = fn
}

// externalLinkRenderer overrides the two link kinds, delegates marked Markdown
// refusals, and otherwise keeps goldmark's local-link rendering.
type externalLinkRenderer struct {
	link     renderer.NodeRendererFunc
	autoLink renderer.NodeRendererFunc
}

func newExternalLinkRenderer() externalLinkRenderer {
	defaults := defaultRenderers{}
	goldmarkhtml.NewRenderer().RegisterFuncs(defaults)
	r := externalLinkRenderer{link: defaults[ast.KindLink], autoLink: defaults[ast.KindAutoLink]}
	if r.link == nil || r.autoLink == nil {
		panic("render: newExternalLinkRenderer requires a non-nil goldmark link renderer")
	}
	return r
}

func (r externalLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
}

func (r externalLinkRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.Link)
	if ok {
		if value, found := n.AttributeString(markdownFaultAttr); found {
			return renderMarkdownFault(w, node, value, entering)
		}
	}
	if !ok || !leavesTheLibrary(n.Destination) {
		return r.link(w, source, node, entering)
	}
	if !entering {
		if err := writeStrings(w, opensInNewTabNote(footnoteLang(node)), "</a>"); err != nil {
			return ast.WalkStop, err
		}
		return ast.WalkContinue, nil
	}
	if err := writeStrings(w, `<a href="`, string(util.EscapeHTML(util.URLEscape(n.Destination, true))), `"`); err != nil {
		return ast.WalkStop, err
	}
	if n.Title != nil {
		if err := writeStrings(w, ` title="`); err != nil {
			return ast.WalkStop, err
		}
		goldmarkhtml.DefaultWriter.Write(w, n.Title)
		if err := w.WriteByte('"'); err != nil {
			return ast.WalkStop, err
		}
	}
	if err := writeStrings(w, externalLinkAttributes, ">"); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}

func (r externalLinkRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.AutoLink)
	if !ok || n.AutoLinkType != ast.AutoLinkURL || !leavesTheLibrary(n.URL(source)) {
		return r.autoLink(w, source, node, entering)
	}
	if !entering {
		return ast.WalkContinue, nil
	}
	err := writeStrings(w,
		`<a href="`, string(util.EscapeHTML(util.URLEscape(n.URL(source), false))), `"`, externalLinkAttributes, ">",
		string(util.EscapeHTML(n.Label(source))), opensInNewTabNote(footnoteLang(node)), "</a>")
	if err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}

func renderMarkdownFault(w util.BufWriter, node ast.Node, value any, entering bool) (ast.WalkStatus, error) {
	fault, valid := value.(markdownFault)
	if !valid {
		return ast.WalkStop, fmt.Errorf("render: Markdown fault attribute has type %T, want markdownFault", value)
	}
	if entering {
		return ast.WalkContinue, writeStrings(w, `<span class="`, fault.class, `" title="`, html.EscapeString(fault.reason), `">`)
	}
	return ast.WalkContinue, writeStrings(w, `<span class="`+offscreenNoteClass+`">`, html.EscapeString(wording.ParenOpen.In(footnoteLang(node))), html.EscapeString(fault.reason), html.EscapeString(wording.ParenClose.In(footnoteLang(node))), `</span></span>`)
}

// opensInNewTabNote is the sentence a listener is told after the link's own
// words. It is written the way every other explanation carried out of sight is,
// in the element a heading's words are read without, so a link inside a heading
// never becomes part of the name of the section around it.
func opensInNewTabNote(lang wording.Lang) string {
	return `<span class="` + offscreenNoteClass + `">` +
		html.EscapeString(wording.ParenOpen.In(lang)) + html.EscapeString(wording.OpensInNewTab.In(lang)) + html.EscapeString(wording.ParenClose.In(lang)) +
		`</span>`
}

func writeStrings(w util.BufWriter, parts ...string) error {
	for _, part := range parts {
		if _, err := w.WriteString(part); err != nil {
			return err
		}
	}
	return nil
}

// externalLinkExtension registers externalLinkRenderer ahead of goldmark's own
// link renderers (priority 1000) and the GFM autolink renderer (500).
type externalLinkExtension struct{}

func (externalLinkExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newExternalLinkRenderer(), 300),
	))
}
