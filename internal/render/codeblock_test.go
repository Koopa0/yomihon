package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestFencedCodeBlockHighlighting covers codeblock.go's chroma
// NodeRenderer: a known language produces chroma's class-based span
// markup, and an unknown or missing language degrades to valid,
// uncrashed, still-escaped output rather than erroring or being skipped.
func TestFencedCodeBlockHighlighting(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name    string
		body    string
		want    []string // every substring must appear
		notWant []string // no substring may appear
	}{
		{
			name: "known language gets chroma's class-based token spans",
			body: "```go\npackage main\n```\n",
			want: []string{
				`<pre class="chroma">`,
				`class="kn"`, // keyword.namespace — "package"
				`class="nx"`, // name — "main"
			},
		},
		{
			name: "empty language degrades to plain, still-wrapped output",
			body: "```\nno language here\n```\n",
			want: []string{
				`<pre class="chroma">`,
				"no language here",
			},
			notWant: []string{
				`class="kn"`,
			},
		},
		{
			name: "unknown language degrades to plain, still-wrapped output",
			body: "```totally-not-a-real-language\nsome text\n```\n",
			want: []string{
				`<pre class="chroma">`,
				"some text",
			},
			notWant: []string{
				`class="kn"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", tt.body, wording.ZhHant)
			for _, w := range tt.want {
				if !strings.Contains(got.HTML, w) {
					t.Errorf("HTML(%q).HTML missing %q:\n%s", tt.body, w, got.HTML)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(got.HTML, nw) {
					t.Errorf("HTML(%q).HTML must not contain %q:\n%s", tt.body, nw, got.HTML)
				}
			}
		})
	}
}

// TestChromaCSS covers ChromaCSS's sync.OnceValue: it must return
// non-empty, valid-looking CSS containing the expected chroma class
// prefix, and it must memoize — steady-state calls return the cached
// string without recomputing.
//
// Not parallel: testing.AllocsPerRun pins GOMAXPROCS for its measurement
// and must not run alongside other parallel tests.
func TestChromaCSS(t *testing.T) {
	first := render.ChromaCSS()
	if first == "" {
		t.Fatal("ChromaCSS() returned an empty string")
	}
	if !strings.Contains(first, ".chroma") {
		t.Errorf("ChromaCSS() missing the .chroma wrapper class:\n%s", first)
	}

	// Memoization can't be proven by byte-equality: the stylesheet is
	// deterministic, so a non-memoized reimplementation (dropping
	// sync.OnceValue and recomputing via strings.Builder + WriteCSS on
	// every call) returns a byte-identical string and passes any
	// first==second check. What DOES change is allocation: a cached
	// string returns with zero allocations, while recomputing allocates a
	// fresh builder and result each call (measured ~450+). This is the
	// regression the assertion catches.
	if allocs := testing.AllocsPerRun(100, func() { _ = render.ChromaCSS() }); allocs != 0 {
		t.Errorf("ChromaCSS() allocates %.0f times per call, want 0 (sync.OnceValue must return the cached string, not recompute)", allocs)
	}
}

// TestHighlightMarkupCarriesNoTheme is the assumption the single stylesheet
// rests on: one rendering of a code block serves both themes, so the renderer
// never has to be handed a cookie, a preference, or any other request state to
// draw one. If the highlighter ever started writing the palette into the
// markup — an inline colour, a mode class beside the token class — the cached
// bytes would silently become a light-mode page that a dark-mode reader still
// received.
func TestHighlightMarkupCarriesNoTheme(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "```go\npackage main // hi\n```\n", wording.ZhHant).HTML
	for _, name := range []string{"github", "github-dark"} {
		if strings.Contains(got, name) {
			t.Errorf("the rendered code block names the palette %q; the markup must be theme-independent:\n%s", name, got)
		}
	}
	if strings.Contains(got, "style=") {
		t.Errorf("the rendered code block carries an inline style, which fixes one theme's colours into the markup:\n%s", got)
	}
	if !strings.Contains(got, `class="kn"`) {
		t.Errorf("the rendered code block lost its class-based tokens, which is how the stylesheet reaches it:\n%s", got)
	}
}

// The two palettes are named here by the literal bytes a browser receives, not
// by the highlighter's style names: the property under test is which colours
// reach the reader, and a name compared against itself would hold while the
// wrong colours shipped. Keyword is the probe colour because every sample of
// code in this repository's tests contains one.
const (
	lightKeywordColor = "#cf222e"
	darkKeywordColor  = "#ff7b72"
	darkScopeOpener   = `:root[data-theme="dark"]`
)

// TestChromaCSSCarriesBothThemes covers the stylesheet's shape rather than its
// appearance: one sheet has to answer for both themes, because the renderer is
// never told which theme a request wants and the root attribute is the only
// thing that knows.
//
// The reading surface stays the product's own panel in both themes, so the
// whole sheet is layered and every unlayered product rule outranks it. Without
// that, the dark scope's extra specificity would win the code block's
// background in dark and lose it in light — the same page, dressed two
// different ways depending on the time of day.
func TestChromaCSSCarriesBothThemes(t *testing.T) {
	t.Parallel()
	css := render.ChromaCSS()

	if !strings.HasPrefix(strings.TrimSpace(css), "@layer ") {
		t.Errorf("the highlighting sheet must open a cascade layer so product rules keep the code surface; it starts:\n%.120s", css)
	}

	scope := strings.Index(css, darkScopeOpener)
	if scope < 0 {
		t.Fatalf("no dark scope %q in the highlighting sheet:\n%s", darkScopeOpener, css)
	}
	beforeScope, insideScope := css[:scope], css[scope:]

	if strings.Contains(beforeScope, darkKeywordColor) {
		t.Errorf("the dark keyword colour %s appears before the dark scope, so a light reader would receive it", darkKeywordColor)
	}
	if !strings.Contains(insideScope, darkKeywordColor) {
		t.Errorf("the dark scope carries no dark keyword colour %s:\n%s", darkKeywordColor, insideScope)
	}
	if !strings.Contains(beforeScope, lightKeywordColor) {
		t.Errorf("the unscoped rules carry no light keyword colour %s:\n%s", lightKeywordColor, beforeScope)
	}
	if strings.Contains(insideScope, lightKeywordColor) {
		t.Errorf("the light keyword colour %s appears inside the dark scope:\n%s", lightKeywordColor, insideScope)
	}

	// Paper has one colour. A reader who prints after an evening in dark mode
	// must get the light syntax rules, which happens by the dark scope simply
	// not existing for print — so the unscoped light rules are what is left.
	if guard := strings.Index(css, "@media not print"); guard < 0 || guard > scope {
		t.Errorf("the dark scope is not held behind a print guard, so bright syntax colours would reach paper:\n%s", css)
	}

	// Forced colours are the reader's decision. Code is text like any other,
	// and a sheet that opted it out would hand back a palette the reader
	// switched their whole system away from.
	if strings.Contains(css, "forced-color-adjust") {
		t.Errorf("the highlighting sheet must not take forced colours away from the browser:\n%s", css)
	}
}
