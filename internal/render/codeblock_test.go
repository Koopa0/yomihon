package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
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
			got := r.HTML(tt.body)
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
