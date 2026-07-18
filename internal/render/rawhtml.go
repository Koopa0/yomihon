package render

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var (
	safeMarkupBareTag = regexp.MustCompile(`^<(?:ruby|rt|rp|br)[ \t\r\n]*/?>$`)
	safeMarkupEndTag  = regexp.MustCompile(`^</(?:ruby|rt|rp)[ \t\r\n]*>$`)
	safeMarkupLangTag = regexp.MustCompile(`^<(?:ruby|rt|rp)[ \t\r\n]+lang=(?:"[A-Za-z0-9]{1,8}(?:-[A-Za-z0-9]{1,8})*"|'[A-Za-z0-9]{1,8}(?:-[A-Za-z0-9]{1,8})*')[ \t\r\n]*>$`)
	safeReadAloudTag  = regexp.MustCompile(`^<!--[ \t\r\n]*read-aloud:[ \t\r\n]*ja[ \t\r\n]*-->$`)
	trustedBlockTag   = regexp.MustCompile(`^<!--yomihon-block:\d+-->$`)
)

// safeMarkupRenderer is the note-body authority boundary. Goldmark still
// parses authored HTML so the Japanese ruby dialect remains expressible, but
// only the small inert subset the vault actually uses becomes markup. Every
// other authored tag is rendered as text. Markdown links remain links; remote
// Markdown images become explicit links instead of automatic requests.
type safeMarkupRenderer struct{}

func (safeMarkupRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, renderSafeHTMLBlock)
	reg.Register(ast.KindRawHTML, renderSafeRawHTML)
	reg.Register(ast.KindImage, renderSafeImage)
}

func renderSafeHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.HTMLBlock)
	if !ok {
		return ast.WalkContinue, nil
	}
	if entering {
		for i := range n.Lines().Len() {
			line := n.Lines().At(i)
			if err := writeSafeMarkup(w, line.Value(source)); err != nil {
				return ast.WalkStop, err
			}
		}
		return ast.WalkContinue, nil
	}
	if n.HasClosure() {
		if err := writeSafeMarkup(w, n.ClosureLine.Value(source)); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkContinue, nil
}

func renderSafeRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n, ok := node.(*ast.RawHTML)
	if !ok {
		return ast.WalkSkipChildren, nil
	}
	for i := range n.Segments.Len() {
		segment := n.Segments.At(i)
		if err := writeSafeMarkup(w, segment.Value(source)); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkSkipChildren, nil
}

func writeSafeMarkup(w util.BufWriter, raw []byte) error {
	for len(raw) > 0 {
		start := bytes.IndexByte(raw, '<')
		if start < 0 {
			_, err := w.Write(util.EscapeHTML(raw))
			return err
		}
		if start > 0 {
			if _, err := w.Write(util.EscapeHTML(raw[:start])); err != nil {
				return err
			}
			raw = raw[start:]
		}
		end := bytes.IndexByte(raw, '>')
		if end < 0 {
			_, err := w.Write(util.EscapeHTML(raw))
			return err
		}
		tag := raw[:end+1]
		if safeMarkupBareTag.Match(tag) || safeMarkupEndTag.Match(tag) || safeMarkupLangTag.Match(tag) ||
			safeReadAloudTag.Match(tag) || trustedBlockTag.Match(tag) {
			if _, err := w.Write(tag); err != nil {
				return err
			}
		} else if _, err := w.Write(util.EscapeHTML(tag)); err != nil {
			return err
		}
		raw = raw[end+1:]
	}
	return nil
}

func renderSafeImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n, ok := node.(*ast.Image)
	if !ok {
		return ast.WalkSkipChildren, nil
	}
	dest := util.URLEscape(n.Destination, true)
	var err error
	switch {
	case localImageDestination(n.Destination, dest):
		err = writeLocalImage(w, source, n, dest)
	case linkableRemoteImage(n.Destination):
		err = writeRemoteImageLink(w, source, n, dest)
	default:
		err = writeImageLabel(w, source, n)
	}
	if err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkSkipChildren, nil
}

func writeLocalImage(w util.BufWriter, source []byte, node ast.Node, destination []byte) error {
	if _, err := w.WriteString(`<img src="`); err != nil {
		return err
	}
	if _, err := w.Write(util.EscapeHTML(destination)); err != nil {
		return err
	}
	if _, err := w.WriteString(`" alt="`); err != nil {
		return err
	}
	if err := writeImageLabel(w, source, node); err != nil {
		return err
	}
	_, err := w.WriteString(`">`)
	return err
}

func writeRemoteImageLink(w util.BufWriter, source []byte, node ast.Node, destination []byte) error {
	if _, err := w.WriteString(`<a href="`); err != nil {
		return err
	}
	if _, err := w.Write(util.EscapeHTML(destination)); err != nil {
		return err
	}
	if _, err := w.WriteString(`" rel="external noreferrer" referrerpolicy="no-referrer">`); err != nil {
		return err
	}
	if err := writeImageLabel(w, source, node); err != nil {
		return err
	}
	_, err := w.WriteString(`</a>`)
	return err
}

func localImageDestination(raw, escaped []byte) bool {
	if goldmarkhtml.IsDangerousURL(escaped) {
		return false
	}
	value := string(raw)
	if strings.Contains(value, `\`) || strings.HasPrefix(value, "//") {
		return false
	}
	// Goldmark's URL classifier admits only non-scriptable raster data URLs
	// (PNG, GIF, JPEG, and WebP). They are self-contained rather than remote,
	// so they belong on the same side of the boundary as a vault-local image.
	if strings.HasPrefix(strings.ToLower(value), "data:image/") {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && !u.IsAbs() && u.Host == ""
}

func linkableRemoteImage(raw []byte) bool {
	value := string(raw)
	if strings.HasPrefix(value, "//") && !strings.Contains(value, `\`) {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https"))
}

func writeImageLabel(w util.BufWriter, source []byte, n ast.Node) error {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch child := child.(type) {
		case *ast.Text:
			if _, err := w.Write(util.EscapeHTML(child.Segment.Value(source))); err != nil {
				return err
			}
		case *ast.String:
			if _, err := w.Write(util.EscapeHTML(child.Value)); err != nil {
				return err
			}
		default:
			if err := writeImageLabel(w, source, child); err != nil {
				return err
			}
		}
	}
	return nil
}

type safeMarkupExtension struct{}

func (safeMarkupExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(safeMarkupRenderer{}, 100),
	))
}
