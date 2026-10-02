package render_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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
func TestChromaCSS(t *testing.T) {
	// Not parallel: testing.AllocsPerRun pins GOMAXPROCS for its measurement
	// and must not run alongside other parallel tests.
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
	systemScopeOpener = `:root:not([data-theme="light"])`
)

// TestChromaCSSCarriesBothThemes covers the stylesheet's shape rather than its
// appearance: one sheet has to answer for both themes, because the renderer is
// never told which theme a request wants; only the root attribute and the
// system's preference know.
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

// TestChromaCSSDarkPaletteHasBothEntrances holds the highlighting sheet's two
// dark entrances together. A reader who chose dark enters through the root
// attribute; a reader who chose nothing on a dark system enters through the
// system preference, which the stylesheet's colour tokens already honour. A
// sheet that dressed only the first would draw the light palette's ink on the
// dark panel for the second, which is most readers on a dark system: keyword
// colours survive that, identifiers and punctuation do not.
//
// Each scope is read with the at-rules that enclose it, because the system
// scope is only correct inside its own media query (outside it, a light system
// would be dressed dark) and both are only correct inside the print guard.
func TestChromaCSSDarkPaletteHasBothEntrances(t *testing.T) {
	t.Parallel()
	css := render.ChromaCSS()

	const layer = "@layer yomihon-code"
	wantEnclosing := map[string][]string{
		darkScopeOpener:   {layer, "@media not print"},
		systemScopeOpener: {layer, "@media not print", "@media (prefers-color-scheme: dark)"},
	}
	bodies := map[string]string{}
	for opener, want := range wantEnclosing {
		enclosing, body := scopeAt(t, css, opener)
		if !slices.Equal(enclosing, want) {
			t.Errorf("%s is enclosed by %q, want %q", opener, enclosing, want)
		}
		if !strings.Contains(body, darkKeywordColor) {
			t.Errorf("%s carries no dark keyword colour %s:\n%s", opener, darkKeywordColor, body)
		}
		bodies[opener] = body
	}
	if diff := cmp.Diff(bodies[darkScopeOpener], bodies[systemScopeOpener]); diff != "" {
		t.Errorf("the two dark entrances dress code differently (-attribute +system):\n%s", diff)
	}
}

// scopeAt returns the at-rule preludes that enclose the first rule opening with
// opener, outermost first, and the text between that rule's braces. Rules nest
// here, so the end of the body is found by counting braces.
func scopeAt(t *testing.T, css, opener string) (enclosing []string, body string) {
	t.Helper()
	at := strings.Index(css, opener+" {")
	if at < 0 {
		t.Fatalf("no scope %q in the highlighting sheet:\n%s", opener, css)
	}
	start := 0
	for i, r := range css[:at] {
		switch r {
		case '{':
			enclosing = append(enclosing, strings.TrimSpace(css[start:i]))
			start = i + 1
		case '}':
			enclosing = enclosing[:len(enclosing)-1]
			start = i + 1
		}
	}
	open := at + len(opener) + 1
	for depth, i := 0, open; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return enclosing, css[open+1 : i]
			}
		}
	}
	t.Fatalf("scope %q is never closed:\n%s", opener, css)
	return nil, ""
}
