package render

import (
	"strings"
	"testing"
)

// TestSourceHTMLChoosesALexer pins the lexer selection: a taught alias for the
// two vault kinds chroma does not know by name, chroma's own match for the
// kinds it does, and the plain-text fallback for everything else.
//
// This asserts lexerFor's own return value rather than running the chosen
// lexer through Format and looking for a coloured span. Which lexer a
// filename picks is a fact about the filename alone and carries no wall
// time; chroma hardcodes a 250ms match-timeout budget on every compiled rule
// (see maybeCompile, and the recover path highlightCode added in 338943fe),
// and that budget is a fact about how long the machine took to run a real
// match, not about which lexer got chosen. A machine busy with other work
// can make even a two-token JSON fixture read as though it had timed out,
// which flipped this table's positive cases from highlighted to plain under
// load — a false reading about lexer selection, since selection never
// touched the timeout at all. The degraded output for a real timeout is
// pinned separately and deterministically, by forcing one with a lexer whose
// own rule regresses catastrophically (TestAHighlighterTimeoutLeavesTheFileView),
// so this test does not need to touch that budget to prove selection worked.
func TestSourceHTMLChoosesALexer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		filename  string
		wantLexer string // lexerFor's own Config().Name, the literal chroma registers it under
	}{
		{
			name:      "canvas is highlighted as JSON",
			filename:  "board.canvas",
			wantLexer: "JSON",
		},
		{
			name:      "base is highlighted as YAML",
			filename:  "view.base",
			wantLexer: "YAML",
		},
		{
			name:      "a go file is matched by chroma itself",
			filename:  "main.go",
			wantLexer: "Go",
		},
		{
			name:      "an unknown kind falls back to plain text",
			filename:  "diagram.d2",
			wantLexer: "fallback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lexerFor(tt.filename).Config().Name; got != tt.wantLexer {
				t.Errorf("lexerFor(%q).Config().Name = %q, want %q", tt.filename, got, tt.wantLexer)
			}
		})
	}
}

// TestSourceHTMLDegradesUnknownKinds keeps one end-to-end check through the
// real Format path, for the one shape a chroma match timeout cannot fake: no
// token span can appear where the lexer is already the plain-text fallback,
// because a timeout only ever degrades highlighted output toward plain,
// never the other way. It is safe to run through SourceHTML, unlike the
// positive cases TestSourceHTMLChoosesALexer moved off that path.
func TestSourceHTMLDegradesUnknownKinds(t *testing.T) {
	t.Parallel()
	got := SourceHTML("diagram.d2", "a -> b: label\n")
	if !strings.Contains(got, `class="chroma"`) {
		t.Fatalf("SourceHTML(%q) produced no chroma block", "diagram.d2")
	}
	if hasTokenSpan(got) {
		t.Errorf("SourceHTML(%q) coloured a token, want plain escaped text for a kind no lexer is registered for", "diagram.d2")
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
