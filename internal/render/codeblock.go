package render

// Server-side syntax highlighting, as a goldmark node renderer that intercepts
// fenced code blocks and formats them through chroma. Registration relies on
// goldmark calling node renderers from the highest priority number down, so the
// lowest number registers last and overwrites the default HTML renderer's own
// fenced-code handler.

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

// The two highlighting palettes, one per reading theme, a matched pair from the
// same family so switching theme does not switch colour scheme.
const (
	chromaLightStyleName = "github"
	chromaDarkStyleName  = "github-dark"
)

// codeLayerName is the cascade layer the whole highlighting sheet lives in. The
// reading surface owns the code block's ground and plain ink and these rules only
// colour the syntax inside it, so the layer states once that every unlayered
// product rule outranks anything here, whatever selector the theme scope needs.
const codeLayerName = "yomihon-code"

// chromaFormatter emits class-based HTML rather than inline styles, so one
// stylesheet controls every code block's colours.
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
// block's markup. Which one it is cannot reach the page: class-based output names
// every span from chroma's token table, so the same bytes serve both themes.
func markupStyle() *chroma.Style { return chromaStyle(chromaLightStyleName) }

// paletteCSS is one palette's class-based rules, exactly as chroma writes them.
// The failure branch is unreachable in practice; an empty stylesheet is the right
// degraded behaviour if that changes, rather than a panic over missing colour.
func paletteCSS(styleName string) string {
	var buf strings.Builder
	if err := chromaFormatter.WriteCSS(&buf, chromaStyle(styleName)); err != nil {
		return ""
	}
	return buf.String()
}

// ChromaCSS is the highlighting stylesheet, computed once and cached. It carries
// both palettes, light as the base and dark as a scoped override, because the
// markup is theme-independent. The dark scope opens by returning every token to
// the surrounding ink, since the palettes name different token sets, and sits
// behind a print guard, so printing in dark mode gives light rules on paper.
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

// lexerCacheBound is how many distinct names or filenames one lookup will
// remember. Past it, a new key is answered and forgotten, so a note full of
// invented info-strings cannot grow the maps without limit. The vault's own
// languages and file kinds sit well under this; the bound is for the ones
// that are not.
const lexerCacheBound = 1024

// lexerNames remembers lexers.Get; lexerFiles remembers lexers.Match. They
// are separate because the keys are different questions — an info string
// and a filename — and a miss on either is the expensive answer, including
// when the answer is nil.
var (
	lexerNames = newLexerCache(lexerCacheBound)
	lexerFiles = newLexerCache(lexerCacheBound)
)

// lexerCache is one bounded map in front of one chroma registry lookup.
// nil is stored as a real answer: the miss is what costs milliseconds.
type lexerCache struct {
	mu    sync.Mutex
	bound int
	m     map[string]chroma.Lexer
}

func newLexerCache(bound int) *lexerCache {
	return &lexerCache{
		bound: bound,
		m:     make(map[string]chroma.Lexer, bound),
	}
}

// lookup is the seam: get stands in for chroma, so a test can count how
// often a name actually reaches the registry. A hit, including a cached
// nil, never calls get. A key that arrives after the map is full is
// answered and forgotten, so the names that already fit stay.
func (c *lexerCache) lookup(key string, get func(string) chroma.Lexer) chroma.Lexer {
	c.mu.Lock()
	if l, ok := c.m[key]; ok {
		c.mu.Unlock()
		return l
	}
	c.mu.Unlock()

	l := get(key)

	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.m[key]; ok {
		return existing
	}
	if len(c.m) < c.bound {
		c.m[key] = l
	}
	return l
}

func namedLexer(name string) chroma.Lexer {
	return lexerNames.lookup(name, lexers.Get)
}

func matchedLexer(filename string) chroma.Lexer {
	return lexerFiles.lookup(filename, lexers.Match)
}

// codeBlockRenderer renders a fenced code block via chroma in place of
// goldmark's own default plain <pre><code> output.
type codeBlockRenderer struct{}

func (codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, renderCodeBlock)
}

// renderCodeBlock writes one fenced code block's highlighted HTML. An empty or
// unrecognized language falls back to the plain-text lexer, so the block still
// renders as valid, if uncoloured, HTML.
func renderCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	node, ok := n.(*ast.FencedCodeBlock)
	if !ok {
		// Unreachable in practice; skipping the node degrades gracefully rather
		// than letting one odd node fail the whole render.
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
		if l := namedLexer(string(lang)); l != nil {
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

// codeBlockExtension registers codeBlockRenderer into a goldmark.Markdown built
// by New, overriding the default fenced-code handler: the lower priority
// registers last and wins.
type codeBlockExtension struct{}

func (codeBlockExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(codeBlockRenderer{}, 200),
	))
}
