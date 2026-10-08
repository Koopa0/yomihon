package render

import (
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/graph"
)

var (
	safeMarkupBareTag = regexp.MustCompile(`^<(?:ruby|rt|rp|br)[ \t\r\n]*/?>$`)
	safeMarkupEndTag  = regexp.MustCompile(`^</(?:ruby|rt|rp)[ \t\r\n]*>$`)
	// safeFormattingTag is a bare opener or closer of the inline formatting a
	// note may write. A browser reopens an underline left open in every later
	// block, past the note and into the page around it, so an opener becomes
	// markup only when its own closer follows in the same container; see
	// pairFormatting. A self-closing spelling opens the element all the same and
	// is not admitted.
	safeFormattingTag = regexp.MustCompile(`^<(/?)(kbd|sub|sup|mark|u)[ \t\r\n]*>$`)
	safeMarkupLangTag = regexp.MustCompile(`^<(?:ruby|rt|rp)[ \t\r\n]+lang=(?:"[A-Za-z0-9]{1,8}(?:-[A-Za-z0-9]{1,8})*"|'[A-Za-z0-9]{1,8}(?:-[A-Za-z0-9]{1,8})*')[ \t\r\n]*>$`)
	trustedBlockTag = regexp.MustCompile(`^<!--yomihon-block:\d+-->$`)
)

// safeMarkupRenderer is the note-body authority boundary. Authored HTML is still
// parsed so the ruby dialect stays expressible, but only the small inert subset
// the vault uses becomes markup and every other tag renders as text. Markdown
// links remain links; a remote image becomes an explicit link, not a request.
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
	// The block's lines and its closing line are one container, so a
	// formatting tag pairs across them and nowhere else. Leaving the block's
	// closing line to a second call would decide its tags without the opener
	// the first call already saw.
	if !entering {
		return ast.WalkContinue, nil
	}
	chunks := make([][]byte, 0, n.Lines().Len()+1)
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		chunks = append(chunks, line.Value(source))
	}
	if n.HasClosure() {
		chunks = append(chunks, n.ClosureLine.Value(source))
	}
	gate := pairFormatting(chunks...)
	for _, chunk := range chunks {
		if err := writeSafeMarkup(w, chunk, gate); err != nil {
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
	gate := admitAllFormatting
	if _, unpaired := n.AttributeString(unpairedFormattingAttribute); unpaired {
		gate = &formattingGate{}
	}
	for i := range n.Segments.Len() {
		segment := n.Segments.At(i)
		if err := writeSafeMarkup(w, segment.Value(source), gate); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkSkipChildren, nil
}

func isAllowlistedMarkup(tag []byte) bool {
	_, readAloud := readAloudLanguage(string(tag))
	return safeMarkupBareTag.Match(tag) || safeMarkupEndTag.Match(tag) || safeMarkupLangTag.Match(tag) ||
		readAloud || trustedBlockTag.Match(tag)
}

// visitSafeMarkup is the one tag walk the body renderer and the heading fold
// share. Each complete tag is kept, dropped, or escaped according to the
// allowlist; the bytes between tags, and a tail with no closing '>', go to
// text. Comments are consumed through their own closer, including any tags
// or greater-than signs inside, so none of their words reach heading names.
//
// gate decides the formatting tags among them; it carries over from one call to
// the next when a container arrives in pieces.
func visitSafeMarkup(raw []byte, gate *formattingGate, text, keep, escape func([]byte) error, drop func([]byte)) error {
	original := raw
	code := htmlCommentCode(string(original))
	for len(raw) > 0 {
		start := bytes.IndexByte(raw, '<')
		if start < 0 {
			return text(raw)
		}
		literal := literalHTMLComment(original, len(original)-len(raw)+start, code)
		if start > 0 {
			if err := text(raw[:start]); err != nil {
				return err
			}
			raw = raw[start:]
		}
		end := safeMarkupEnd(raw)
		if end < 0 {
			return visitMarkupTail(markupToken{raw: raw, literal: literal}, text, drop)
		}
		if err := visitMarkupToken(markupToken{raw: raw[:end+1], literal: literal}, gate, keep, escape, drop); err != nil {
			return err
		}
		raw = raw[end+1:]
	}
	return nil
}

// A comment ends at its own closer, so a tag or a greater-than sign inside it,
// such as one in a malformed read-aloud language, never becomes authored prose.
func safeMarkupEnd(raw []byte) int {
	if bytes.HasPrefix(raw, []byte("<!--")) {
		span, closed := graph.HTMLCommentSpan(string(raw), 0)
		if !closed {
			return -1
		}
		return span.Stop - 1
	}
	return bytes.IndexByte(raw, '>')
}

type markupToken struct {
	raw     []byte
	literal bool
}

func literalHTMLComment(source []byte, open int, code []graph.Span) bool {
	if !bytes.HasPrefix(source[open:], []byte("<!--")) {
		return false
	}
	span, _ := graph.HTMLCommentSpan(string(source), open)
	return span.Zero() || graph.In(code, open)
}

func visitMarkupToken(token markupToken, gate *formattingGate, keep, escape func([]byte) error, drop func([]byte)) error {
	switch {
	case token.literal:
		return escape(token.raw)
	case isAllowlistedMarkup(token.raw) || gate.admits(token.raw):
		return keep(token.raw)
	case bytes.HasPrefix(token.raw, []byte("<!--")):
		drop(token.raw)
		return nil
	default:
		return escape(token.raw)
	}
}

func visitMarkupTail(token markupToken, text func([]byte) error, drop func([]byte)) error {
	if !token.literal && bytes.HasPrefix(token.raw, []byte("<!--")) {
		drop(token.raw)
		return nil
	}
	return text(token.raw)
}

// applySafeMarkup runs authored heading source through the same tag allowlist
// the body renderer uses, so a later headingInnerText sees escaped tags where
// the page already did and live ruby where the page already did. Text between
// tags is left as written: goldmark has already resolved character references
// by the time the page stamps an id, and escaping them here would fold a
// second pass of `&amp;` into a different slug. A blanket escape of the whole
// source would also turn the ruby the reduction is meant to strip into words.
func applySafeMarkup(raw string, gate *formattingGate) string {
	var b strings.Builder
	err := visitSafeMarkup([]byte(raw), gate,
		func(p []byte) error { b.Write(p); return nil },
		func(p []byte) error { b.Write(p); return nil },
		func(p []byte) error { b.Write(util.EscapeHTML(p)); return nil },
		func([]byte) {},
	)
	if err != nil {
		return raw
	}
	return b.String()
}

func writeSafeMarkup(w util.BufWriter, raw []byte, gate *formattingGate) error {
	escape := func(p []byte) error {
		_, err := w.Write(util.EscapeHTML(p))
		return err
	}
	return visitSafeMarkup(raw, gate, escape, func(p []byte) error {
		_, err := w.Write(p)
		return err
	}, escape, func([]byte) {
		// A private remark does not become text on the reading page.
	})
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
	var label bytes.Buffer
	if err := writeImageLabel(&label, source, node); err != nil {
		return err
	}
	alt := label.String()
	size := ""
	if separator := strings.LastIndex(alt, "|"); separator >= 0 {
		size = imageSizeAttributes(alt[separator+1:])
		if size != "" {
			alt = alt[:separator]
		}
	}
	if _, err := w.WriteString(`<img src="`); err != nil {
		return err
	}
	if _, err := w.Write(util.EscapeHTML(destination)); err != nil {
		return err
	}
	if _, err := w.WriteString(`" alt="`); err != nil {
		return err
	}
	if _, err := w.WriteString(alt); err != nil {
		return err
	}
	_, err := w.WriteString(`"` + size + `>`)
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

func writeImageLabel(w io.Writer, source []byte, n ast.Node) error {
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
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(pairFormattingInline{}, 1000),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(safeMarkupRenderer{}, 100),
	))
}
