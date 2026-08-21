package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

// TestPlainText is the table-driven acceptance for the plain-text
// extraction rules: what a note body contributes to the search index. Each row
// asserts substrings that must be present and, where a rule is exclusionary,
// substrings that must be absent.
func TestPlainText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		present []string
		absent  []string
	}{
		{
			name:    "heading and body text included",
			body:    "# Title Here\n\nSome body text.\n",
			present: []string{"Title Here", "Some body text"},
		},
		{
			name:    "wikilink target and display both included",
			body:    "See [[Target Note|the alias]] for more.\n",
			present: []string{"Target Note", "the alias"},
		},
		{
			name:    "code fence contents included",
			body:    "```go\nfunc Foo() int { return 42 }\n```\n",
			present: []string{"func Foo", "return 42"},
		},
		{
			name:    "ruby base and rt included, tags excluded",
			body:    "<ruby>今日<rt>きょう</rt></ruby>は晴れ\n",
			present: []string{"今日", "きょう", "は晴れ"},
			absent:  []string{"<ruby>", "<rt>", "</rt>", "</ruby>"},
		},
		{
			name:    "html tags themselves excluded",
			body:    "first line<br>second line\n",
			present: []string{"first line", "second line"},
			absent:  []string{"<br>"},
		},
		{
			name:    "callout marker excluded, title and body kept",
			body:    "> [!note] Important Heading\n> the detail text\n",
			present: []string{"Important Heading", "the detail text"},
			absent:  []string{"[!note]", "!note"},
		},
		{
			name:    "table cells included",
			body:    "| Command | Description |\n|---|---|\n| status | flip it |\n",
			present: []string{"Command", "Description", "status", "flip it"},
		},
		{
			name:    "task text included",
			body:    "- [ ] buy milk\n- [x] write tests\n",
			present: []string{"buy milk", "write tests"},
		},
		{
			name:    "wikilink with no display alias contributes its name once",
			body:    "See [[Same Note]].\n",
			present: []string{"Same Note"},
			absent:  []string{"Same Note Same Note"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := render.PlainText(tt.body)
			for _, p := range tt.present {
				if !strings.Contains(got, p) {
					t.Errorf("PlainText(%q) = %q, missing %q", tt.body, got, p)
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(got, a) {
					t.Errorf("PlainText(%q) = %q, must not contain %q", tt.body, got, a)
				}
			}
		})
	}
}

// TestAWikilinkInsideAFenceStaysLiteralInTheCorpus pins the fence guard of
// the plain-text preprocessing: a wikilink written inside a fenced code block
// is code content, indexed byte-for-byte as written, never rewritten to its
// target and display words.
func TestAWikilinkInsideAFenceStaysLiteralInTheCorpus(t *testing.T) {
	t.Parallel()

	got := render.PlainText("```\n[[Target Note|the alias]]\n```\n")
	if !strings.Contains(got, "[[Target Note|the alias]]") {
		t.Errorf("PlainText() = %q, missing the literal fenced wikilink", got)
	}
	if strings.Contains(got, "Target Note the alias") {
		t.Errorf("PlainText() = %q, rewrote a fenced wikilink", got)
	}
}

// TestCorpusRewritesInsideCodeSpansAndIgnoresTheEscape pins the two
// boundaries the plain-text pass deliberately does not honor, unlike the
// reading page: a wikilink inside an inline code span is rewritten anyway
// (the pass is line-based, and fences alone protect their contents), and a
// backslash escape ahead of the brackets is not consulted, so a shown link's
// words are indexed like a followed link's. Changing either boundary changes
// what a search can find, so it is a corpus decision rather than a cleanup.
func TestCorpusRewritesInsideCodeSpansAndIgnoresTheEscape(t *testing.T) {
	t.Parallel()

	got := render.PlainText("a `[[Span Note|span label]]` span\n\nshown \\[[Escaped Note|escape label]] link\n")
	for _, want := range []string{"Span Note span label", "Escaped Note escape label"} {
		if !strings.Contains(got, want) {
			t.Errorf("PlainText() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "[[") {
		t.Errorf("PlainText() = %q, kept a literal wikilink token", got)
	}
}

// TestAdjacentBlocksAreSeparatedByOneNewline pins the block separator:
// exactly one newline between adjacent blocks' text, never a blank line, so
// the corpus bytes for a body are deterministic.
func TestAdjacentBlocksAreSeparatedByOneNewline(t *testing.T) {
	t.Parallel()

	got := render.PlainText("# Title\n\nfirst paragraph\n\nsecond paragraph\n")
	want := "Title\nfirst paragraph\nsecond paragraph"
	if got != want {
		t.Errorf("PlainText() = %q, want %q", got, want)
	}
}

// TestBareURLsAreIndexedVerbatim pins the deliberate absence of
// linkification in the plain-text parser: a bare URL contributes exactly the
// bytes in the file (what a grep of the vault would see), and is never
// synthesized into a link — linkification would invent an http:// prefix for
// a www name. An angle-bracket autolink contributes its URL; a markdown link
// contributes its label only, never its destination.
func TestBareURLsAreIndexedVerbatim(t *testing.T) {
	t.Parallel()

	got := render.PlainText(strings.Join([]string{
		"bare https://example.com/bare?q=1 stays",
		"plain www.example.com/plain stays",
		"angle <https://example.com/angle> stays",
		"labeled [the docs](https://example.com/dropped-path) stays",
	}, "\n\n"))
	for _, want := range []string{
		"https://example.com/bare?q=1",
		"www.example.com/plain",
		"https://example.com/angle",
		"the docs",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PlainText() = %q, missing %q", got, want)
		}
	}
	for _, absent := range []string{"http://www.example.com", "dropped-path"} {
		if strings.Contains(got, absent) {
			t.Errorf("PlainText() = %q, must not contain %q", got, absent)
		}
	}
}

func TestPlainSectionsFollowDocumentHeadingBoundaries(t *testing.T) {
	t.Parallel()

	body := `intro text

## Alpha

first paragraph

### Beta

See [[Target Note|the alias]].

## Empty

---

## Code

` + "```go\nfunc answer() int { return 42 }\n```\n"
	sections := render.PlainSections(body)
	if len(sections) != 5 {
		t.Fatalf("PlainSections() returned %d sections, want 5: %+v", len(sections), sections)
	}

	tests := []struct {
		index       int
		headings    []string
		text        string
		blocks      int
		firstIsCode bool
	}{
		{index: 0, headings: nil, text: "intro text", blocks: 1},
		{index: 1, headings: []string{"Alpha"}, text: "first paragraph", blocks: 1},
		{index: 2, headings: []string{"Alpha", "Beta"}, text: "Target Note the alias", blocks: 1},
		{index: 3, headings: []string{"Empty"}, text: "", blocks: 0},
		{index: 4, headings: []string{"Code"}, text: "func answer() int { return 42 }", blocks: 1, firstIsCode: true},
	}
	for _, tt := range tests {
		got := sections[tt.index]
		if strings.Join(got.Headings, "/") != strings.Join(tt.headings, "/") {
			t.Errorf("section %d headings = %v, want %v", tt.index, got.Headings, tt.headings)
		}
		if !strings.Contains(got.Text, tt.text) {
			t.Errorf("section %d text = %q, want substring %q", tt.index, got.Text, tt.text)
		}
		if len(got.Blocks) != tt.blocks {
			t.Errorf("section %d blocks = %+v, want %d", tt.index, got.Blocks, tt.blocks)
			continue
		}
		if tt.blocks > 0 && got.Blocks[0].Code != tt.firstIsCode {
			t.Errorf("section %d first block Code = %v, want %v", tt.index, got.Blocks[0].Code, tt.firstIsCode)
		}
	}
}
