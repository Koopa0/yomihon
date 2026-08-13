package render

// This file implements server-side syntax highlighting as a goldmark
// renderer.NodeRenderer that intercepts ast.KindFencedCodeBlock directly
// and formats it via chroma — the same approach Hugo uses internally,
// rather than depending on github.com/yuin/goldmark-highlighting/v2
// (unmaintained since 2023-10, and one more dependency than needed). Its
// structural registration mechanism (a lower util.Prioritized number wins
// goldmark's map-of-NodeKind "last Register call for this kind wins" rule
// — see goldmark's renderer.renderer.Render: NodeRenderers are sorted
// ascending by priority, then RegisterFuncs is called from the highest
// priority number down to the lowest, so the lowest number's call happens
// last and overwrites the default HTML renderer's own FencedCodeBlock
// handler, registered by goldmark.New at priority 1000) mirrors that
// project's approach; this file does not import or vendor its code, and
// deliberately skips its configuration surface (custom formatters,
// per-language style overrides, line highlighting) — yomihon needs none of
// that.

import (
	"fmt"
	"html"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v3"
	chromahtml "github.com/alecthomas/chroma/v3/formatters/html"
	"github.com/alecthomas/chroma/v3/lexers"
	"github.com/alecthomas/chroma/v3/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// The two highlighting palettes, one per reading theme. They are a matched
// pair from the same family, so a reader who switches theme sees the same
// language coloured the same way at a legible weight, not a second colour
// scheme's opinion.
const (
	chromaLightStyleName = "github"
	chromaDarkStyleName  = "github-dark"
)

// codeLayerName is the cascade layer the whole highlighting sheet lives in.
// The reading surface owns the code block's ground and its plain ink — a code
// block is a panel of the page, not a window onto someone else's editor — and
// these rules only colour the syntax inside it. A layer states that ranking
// once: every unlayered product rule outranks anything here, whatever selector
// the theme scope needs. Without it the dark scope's extra specificity would
// capture the block's background in dark mode and lose it in light, so one
// page would be dressed two different ways depending on the time of day.
const codeLayerName = "yomihon-code"

// chromaFormatter emits class-based HTML (html.WithClasses(true)) rather
// than inline styles, so one stylesheet (ChromaCSS, served at
// /static/chroma.css by internal/asset) controls every code block's
// colors.
var chromaFormatter = chromahtml.New(chromahtml.WithClasses(true))

// chromaStyle resolves a palette by name; styles.Get returns nil for an
// unknown name, in which case chroma's own plain fallback style is used
// rather than passing a nil *chroma.Style into the formatter.
func chromaStyle(name string) *chroma.Style {
	if s := styles.Get(name); s != nil {
		return s
	}
	return styles.Fallback
}

// markupStyle is the palette handed to the formatter while it writes a code
// block's markup. Which one it is cannot reach the page: with class-based
// output every span is named from chroma's own token-type table, so the same
// bytes serve both themes and the renderer never has to be told which one a
// request wants. The lock for that property is in this package's tests.
func markupStyle() *chroma.Style { return chromaStyle(chromaLightStyleName) }

// paletteCSS is one palette's class-based rules, exactly as chroma writes
// them. strings.Builder's Write never returns an error, so the failure branch
// is unreachable in practice — but an empty stylesheet (colourless code) is
// the correct degraded behaviour if that ever changes, not a panic over one
// missing CSS file.
func paletteCSS(styleName string) string {
	var buf strings.Builder
	if err := chromaFormatter.WriteCSS(&buf, chromaStyle(styleName)); err != nil {
		return ""
	}
	return buf.String()
}

// ChromaCSS is the highlighting stylesheet, computed once and cached for the
// process's lifetime — simpler than a dev-time go:generate step that could
// drift stale.
//
// It carries both palettes, because the markup is theme-independent and the
// page's root attribute is the only thing that knows which theme a reader
// chose. The light rules are the base and the dark ones a scoped override, in
// the same shape the design tokens use.
//
// Two details are load-bearing. The dark scope opens by returning every token
// to the surrounding ink, because the two palettes do not name the same set of
// tokens: the dark one leaves identifiers and punctuation to the body colour,
// and without the reset those spans would keep the light palette's near-black
// on a near-black panel — the exact unreadability this sheet exists to fix,
// surviving in the tokens that make up most of a line. And the whole scope is
// held behind a print guard, so a reader who prints after an evening in dark
// mode gets the light rules on white paper rather than bright ink no printer
// can render.
var ChromaCSS = sync.OnceValue(func() string {
	light, dark := paletteCSS(chromaLightStyleName), paletteCSS(chromaDarkStyleName)
	if light == "" || dark == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("@layer " + codeLayerName + " {\n")
	b.WriteString(light)
	b.WriteString("@media not print {\n:root[data-theme=\"dark\"] {\n")
	// Colour and background only: weight, slant and spacing say what kind of
	// token this is, which does not change with the light in the room.
	b.WriteString(".chroma span { color: inherit; background-color: transparent; }\n")
	b.WriteString(dark)
	b.WriteString("}\n}\n}\n")
	return b.String()
})

// codeBlockRenderer renders a fenced code block via chroma in place of
// goldmark's own default plain <pre><code> output.
type codeBlockRenderer struct{}

func (codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, renderCodeBlock)
}

// renderCodeBlock writes one fenced code block's chroma-highlighted HTML.
// An empty or unrecognized language (lexers.Get returns nil for both — a
// common case, never a crash) falls back to lexers.Fallback, chroma's own
// plain-text lexer, so the block still renders as valid, if uncolored,
// HTML rather than being skipped or erroring.
func renderCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	node, ok := n.(*ast.FencedCodeBlock)
	if !ok {
		// ast.KindFencedCodeBlock guarantees this in practice (goldmark
		// never dispatches a different concrete type to a KindFencedCodeBlock
		// renderer); degrade gracefully on a mismatch (skip this node)
		// rather than panic — one odd node must never fail the whole
		// render.
		return ast.WalkContinue, nil
	}

	var src strings.Builder
	lines := node.Lines()
	for i := range lines.Len() {
		line := lines.At(i)
		src.Write(line.Value(source))
	}

	lexer := lexers.Fallback
	if lang := node.Language(source); len(lang) > 0 {
		if l := lexers.Get(string(lang)); l != nil {
			lexer = l
		}
	}

	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, src.String())
	if err != nil {
		// Never fail the whole render over one bad fence — fall
		// back to plain, unhighlighted, still-escaped output.
		_, werr := fmt.Fprintf(w, "<pre><code>%s</code></pre>\n", html.EscapeString(src.String()))
		return ast.WalkContinue, werr
	}
	return ast.WalkContinue, chromaFormatter.Format(w, markupStyle(), iterator)
}

// codeBlockExtension registers codeBlockRenderer into a goldmark.Markdown
// built by New, overriding the default HTML renderer's FencedCodeBlock
// handler (priority 200 beats the default's 1000 — lower wins, see this
// file's top comment).
type codeBlockExtension struct{}

func (codeBlockExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(codeBlockRenderer{}, 200),
	))
}
