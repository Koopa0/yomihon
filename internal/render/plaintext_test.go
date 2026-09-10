package render_test

import (
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
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
			present: []string{"今日", "きょう", "は晴れ", "今日は"},
			absent:  []string{"<ruby>", "<rt>", "</rt>", "</ruby>"},
		},
		{
			name:    "ruby parenthesis fallback dropped so the base stays one phrase",
			body:    "<ruby>漢<rp>(</rp><rt>かん</rt><rp>)</rp></ruby>字\n",
			present: []string{"漢字", "かん"},
			absent:  []string{"(", ")", "<rp>"},
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

// A wrap inside one paragraph and a paragraph boundary look the same in the
// stored text — one newline — so the only way to tell them apart is the
// offsets taken while walking. The first pair of words sits in one block; the
// second pair starts in one and finishes in the next.
func TestPlainBlocksTellAWrapFromAParagraphBoundary(t *testing.T) {
	t.Parallel()

	body := "" +
		"The evidence records a bright\ncrimson heron near the tower.\n\n" +
		"The field notebook calls this bird cobalt\n\n" +
		"egret beside the old lighthouse.\n"
	text, ends, _ := render.PlainBlocks(body)
	if text != render.PlainText(body) {
		t.Fatalf("PlainBlocks text = %q, want the same bytes PlainText returns", text)
	}
	if len(ends) != 3 {
		t.Fatalf("block ends = %v, want three blocks", ends)
	}
	if !inOneBlock(text, ends, "bright", "crimson") {
		t.Errorf("bright and crimson are not in one block; text = %q ends = %v", text, ends)
	}
	if inOneBlock(text, ends, "cobalt", "egret") {
		t.Errorf("cobalt and egret share a block, so the walk did not part the paragraphs; text = %q ends = %v", text, ends)
	}
}

// TestPlainBlocksReportFenceRanges holds the coordinate the excerpt uses to
// tell a fence from prose: the same bytes stay in the searchable text, and
// the ranges name exactly those bytes. An indented code block is not a fence.
func TestPlainBlocksReportFenceRanges(t *testing.T) {
	t.Parallel()

	body := "" +
		"# Title\n\n" +
		"```d2\n" +
		"direction: right\n" +
		"```\n\n" +
		"prose owns jobs here\n\n" +
		"    indented left alone\n"
	text, _, fences := render.PlainBlocks(body)
	if text != render.PlainText(body) {
		t.Fatalf("PlainBlocks text = %q, want the same bytes PlainText returns", text)
	}
	if !strings.Contains(text, "direction: right") {
		t.Fatalf("PlainBlocks dropped the fence body from the searchable text: %q", text)
	}
	if !strings.Contains(text, "prose owns jobs here") {
		t.Fatalf("PlainBlocks dropped the prose: %q", text)
	}
	if len(fences) != 1 {
		t.Fatalf("fence ranges = %v, want one fenced span", fences)
	}
	got := text[fences[0][0]:fences[0][1]]
	if got != "direction: right" {
		t.Errorf("fence span = %q, want the fence body and nothing beside it", got)
	}
	if strings.Contains(got, "prose") || strings.Contains(got, "indented") {
		t.Errorf("fence span = %q, swallowed prose or indented code", got)
	}
	proseAt := strings.Index(text, "prose owns jobs here")
	if proseAt >= fences[0][0] && proseAt < fences[0][1] {
		t.Errorf("prose offset %d sits inside the fence range %v", proseAt, fences[0])
	}

	// A four-marker opener may hold a shorter all-marker line as content.
	// goldmark already does; the preprocess close must agree, or the range
	// names bytes that were rewritten as prose and Source would show them.
	nested := "" +
		"~~~~\n" +
		"code alpha\n" +
		"~~~\n" +
		"see [[Some Note]] inside the fence\n" +
		"~~~~\n"
	text, _, fences = render.PlainBlocks(nested)
	if text != render.PlainText(nested) {
		t.Fatalf("nested PlainBlocks text = %q, want the same bytes PlainText returns", text)
	}
	if !strings.Contains(text, "[[Some Note]]") {
		t.Fatalf("plain rewrote the nested wikilink as prose: %q", text)
	}
	if len(fences) != 1 {
		t.Fatalf("nested fence ranges = %v, want one fenced span", fences)
	}
	got = text[fences[0][0]:fences[0][1]]
	if !strings.Contains(got, "code alpha") || !strings.Contains(got, "~~~") || !strings.Contains(got, "[[Some Note]]") {
		t.Errorf("nested fence span = %q, want the body goldmark keeps, including the shorter closer and the literal wikilink", got)
	}
	if strings.Contains(got, "see Some Note") && !strings.Contains(got, "[[Some Note]]") {
		t.Errorf("nested fence span = %q, named rewritten prose as Source", got)
	}
}

// bashoRubyBody is the synthetic isolate from the #341 audit: the same
// sentence the bundled 芭蕉の句 shows, with a reading on each content word.
const bashoRubyBody = "<ruby>古池<rt>ふるいけ</rt></ruby>や<ruby>蛙<rt>かわず</rt></ruby><ruby>飛<rt>と</rt></ruby>びこむ<ruby>水<rt>みず</rt></ruby>の<ruby>音<rt>おと</rt></ruby>。"

const bashoPlainBody = "古池や蛙飛びこむ水の音。"

// TestRubyReadingsDoNotSplitBasePhrases locks the retrieval contract the
// page already keeps: a base phrase that hits on a note without ruby also
// hits when the same sentence is written with furigana, and each reading
// stays findable on its own. The lock goes through DocumentFromNote and
// Index.SearchN so a walk that only concatenates text nodes cannot hide
// behind a substring table that never asked for 今日は or 古池や.
func TestRubyReadingsDoNotSplitBasePhrases(t *testing.T) {
	t.Parallel()

	got := render.PlainText(bashoRubyBody)
	for _, want := range []string{"古池や", "古池や蛙飛びこむ水の音", "ふるいけ", "かわず", "おと"} {
		if !strings.Contains(got, want) {
			t.Errorf("PlainText() = %q, missing %q", got, want)
		}
	}
	for _, absent := range []string{"<ruby>", "<rt>", "古池ふるいけや"} {
		if strings.Contains(got, absent) {
			t.Errorf("PlainText() = %q, must not contain %q", got, absent)
		}
	}

	rubyNote := vault.Parse("Notes/Ruby probe.md", []byte("---\ntitle: Probe\n---\n\n"+bashoRubyBody+"\n"))
	plainNote := vault.Parse("Notes/Plain probe.md", []byte("---\ntitle: Control\n---\n\n"+bashoPlainBody+"\n"))
	idx := lexical.NewIndex([]lexical.Document{
		lexical.DocumentFromNote(rubyNote),
		lexical.DocumentFromNote(plainNote),
	}, schema.ArtifactPolicy{})

	queries := []string{"古池", "ふるいけ", "古池や", "古池や蛙飛びこむ水の音"}
	for _, q := range queries {
		results, _, err := idx.SearchN(lexical.Parse(q), -1)
		if err != nil {
			t.Fatalf("SearchN(%q): %v", q, err)
		}
		gotHits := searchPaths(results)
		if !containsPath(gotHits, rubyNote.RelPath) {
			t.Errorf("query %q on the ruby note = %v, want a hit", q, gotHits)
		}
		if q != "ふるいけ" && !containsPath(gotHits, plainNote.RelPath) {
			t.Errorf("query %q on the plain control = %v, want a hit", q, gotHits)
		}
	}

	// A base-phrase hit must land on the sentence the page shows, not on a
	// corpus that spliced readings through it.
	baseHits, _, err := idx.SearchN(lexical.Parse("古池や"), -1)
	if err != nil {
		t.Fatalf("SearchN(古池や): %v", err)
	}
	for _, hit := range baseHits {
		if hit.RelPath != rubyNote.RelPath {
			continue
		}
		if !strings.Contains(hit.Landing, "古池や") {
			t.Errorf("ruby landing = %q, want the visible base phrase", hit.Landing)
		}
		if strings.Contains(hit.Landing, "ふるいけ") {
			t.Errorf("ruby landing = %q, mixed a reading into the base-phrase destination", hit.Landing)
		}
	}
}

// TestShippedHaikuBasePhraseIsSearchable is the cheap stand-in for a
// browser pass on the bundled 芭蕉の句: the note the audit queried, through
// the same DocumentFromNote path search uses.
func TestShippedHaikuBasePhraseIsSearchable(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../examples/vault/Notes/芭蕉の句.md")
	if err != nil {
		t.Fatal(err)
	}
	note := vault.Parse("Notes/芭蕉の句.md", data)
	idx := lexical.NewIndex([]lexical.Document{lexical.DocumentFromNote(note)}, schema.ArtifactPolicy{})
	for _, q := range []string{"古池や", "ふるいけ"} {
		results, _, err := idx.SearchN(lexical.Parse(q), -1)
		if err != nil {
			t.Fatalf("SearchN(%q): %v", q, err)
		}
		if len(results) != 1 {
			t.Errorf("shipped 芭蕉の句 query %q = %d hits, want 1", q, len(results))
		}
	}
}

func searchPaths(results []lexical.Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.RelPath
	}
	return out
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func inOneBlock(text string, ends []int, a, b string) bool {
	start := 0
	for _, end := range ends {
		if end < start || end > len(text) {
			return false
		}
		block := text[start:end]
		if strings.Contains(block, a) && strings.Contains(block, b) {
			return true
		}
		start = end
		if start < len(text) && text[start] == '\n' {
			start++
		}
	}
	return false
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
