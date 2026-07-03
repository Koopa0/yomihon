package render

// This file implements server-side syntax highlighting (docs/spec.md §1's
// "syntax highlighting server-side (chroma)") as a goldmark renderer.NodeRenderer
// that intercepts ast.KindFencedCodeBlock directly and formats it via
// chroma — the same approach Hugo uses internally, rather than depending
// on github.com/yuin/goldmark-highlighting/v2 (unmaintained since
// 2023-10; wall 7 leans toward fewer dependencies, not more). Its
// structural registration mechanism (a lower util.Prioritized number wins
// goldmark's map-of-NodeKind "last Register call for this kind wins" rule
// — see goldmark's renderer.renderer.Render: NodeRenderers are sorted
// ascending by priority, then RegisterFuncs is called from the highest
// priority number down to the lowest, so the lowest number's call happens
// last and overwrites the default HTML renderer's own FencedCodeBlock
// handler, registered by goldmark.New at priority 1000) mirrors that
// project's approach; this file does not import or vendor its code, and
// deliberately skips its configuration surface (custom formatters,
// per-language style overrides, line highlighting) — kurodo needs none of
// that.

import (
	"fmt"
	"html"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// chromaStyleName is kurodo's single fixed highlighting theme: a
// light-background style consistent with base.templ's current minimal
// light UI, which has no dark-mode toggle yet (that lands once Claude's
// design tokens do — dark-mode-aware highlighting is deliberately out of
// scope until then).
const chromaStyleName = "github"

// chromaFormatter emits class-based HTML (html.WithClasses(true)) rather
// than inline styles, so one stylesheet (ChromaCSS, served at
// /static/chroma.css by internal/asset) controls every code block's
// colors.
var chromaFormatter = chromahtml.New(chromahtml.WithClasses(true))

// chromaStyle resolves chromaStyleName once; styles.Get returns nil for an
// unknown name, in which case chroma's own plain fallback style is used
// rather than passing a nil *chroma.Style into the formatter.
func chromaStyle() *chroma.Style {
	if s := styles.Get(chromaStyleName); s != nil {
		return s
	}
	return styles.Fallback
}

// ChromaCSS is chroma's class-based stylesheet for chromaStyleName,
// computed once (sync.OnceValue, go-version.md's modern replacement for a
// package-level sync.Once) and cached for the process's lifetime — simpler
// than a dev-time go:generate step that could drift stale.
var ChromaCSS = sync.OnceValue(func() string {
	var buf strings.Builder
	if err := chromaFormatter.WriteCSS(&buf, chromaStyle()); err != nil {
		// strings.Builder's Write never returns an error, so this is
		// unreachable in practice — but an empty stylesheet (colorless
		// code blocks) is the correct degraded behavior if it ever
		// changes, not a panic over one missing CSS file.
		return ""
	}
	return buf.String()
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
		// renderer); treat a mismatch as wall-4 graceful degradation
		// (skip this node) rather than a panic.
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
		// wall 4: never fail the whole render over one bad fence — fall
		// back to plain, unhighlighted, still-escaped output.
		_, werr := fmt.Fprintf(w, "<pre><code>%s</code></pre>\n", html.EscapeString(src.String()))
		return ast.WalkContinue, werr
	}
	return ast.WalkContinue, chromaFormatter.Format(w, chromaStyle(), iterator)
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
