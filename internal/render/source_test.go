package render

import (
	"strings"
	"testing"
)

// TestSourceHTMLChoosesALexer pins the lexer selection: a taught alias for the
// two vault kinds chroma does not know by name, chroma's own match for the kinds
// it does, and plain escaped text for everything else. The token spans are the
// observable proof that highlighting ran; plainSource emits none.
func TestSourceHTMLChoosesALexer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		filename     string
		source       string
		wantHighlaid bool // a <span class="..."> token appears (chroma ran)
	}{
		{
			name:         "canvas is highlighted as JSON",
			filename:     "board.canvas",
			source:       `{"nodes":[{"id":"a"}]}`,
			wantHighlaid: true,
		},
		{
			name:         "base is highlighted as YAML",
			filename:     "view.base",
			source:       "filters:\n  and: []\n",
			wantHighlaid: true,
		},
		{
			name:         "a go file is matched by chroma itself",
			filename:     "main.go",
			source:       "package main\n",
			wantHighlaid: true,
		},
		{
			name:         "an unknown kind falls back to plain escaped text",
			filename:     "diagram.d2",
			source:       "a -> b: label\n",
			wantHighlaid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SourceHTML(tt.filename, tt.source)
			if !strings.Contains(got, `class="chroma"`) {
				t.Fatalf("SourceHTML(%q) produced no chroma block", tt.filename)
			}
			if hasTokenSpan(got) != tt.wantHighlaid {
				t.Errorf("SourceHTML(%q) highlighted = %v, want %v", tt.filename, hasTokenSpan(got), tt.wantHighlaid)
			}
		})
	}
}

// hasTokenSpan reports whether chroma coloured any token, as opposed to only
// wrapping lines. The fallback plain-text lexer still emits the structural
// "line" and "cl" wrappers, so their presence proves nothing; a token class
// like "nt" or "k" is what a real lexer adds.
func hasTokenSpan(html string) bool {
	structural := strings.NewReplacer(
		`<span class="line">`, "",
		`<span class="cl">`, "",
	).Replace(html)
	return strings.Contains(structural, `<span class="`)
}

// TestPlainSourceKeepsTheHighlighterContainer pins the degraded rendering's
// shape. A file the highlighter chokes on still reaches the reader inside the
// block the stylesheet dresses; losing the container would leave that one file
// looking like a different kind of page.
func TestPlainSourceKeepsTheHighlighterContainer(t *testing.T) {
	t.Parallel()
	got := plainSource("a -> b\n")
	if !strings.Contains(got, `<pre class="chroma">`) {
		t.Errorf("plainSource() = %q, want the highlighter's own container", got)
	}
	if strings.Contains(got, "<script") {
		t.Error("plainSource did not escape its input")
	}
}

// TestSourceHTMLEscapes is the safety property: whatever the lexer, the file's
// own bytes never become live markup. A file that is all angle brackets renders
// as escaped text, not as tags.
func TestSourceHTMLEscapes(t *testing.T) {
	t.Parallel()
	got := SourceHTML("evil.txt", "<script>alert(1)</script>")
	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Error("SourceHTML let a script tag through unescaped")
	}
	if !strings.Contains(got, "&lt;") {
		t.Error("SourceHTML did not escape the angle brackets")
	}
}
