package render_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// newRenderer builds a Renderer over a real (not faked)
// *graph.Index built from in-memory note/resource data, no
// disk access required unless the test itself writes files under root
// (embed transclusion needs the target's actual body on disk; a plain
// wikilink never reads the file, so most tests pass an empty root).
func newRenderer(t *testing.T, root string, notes []graph.NoteInput, resources []string) *render.Renderer {
	t.Helper()
	return render.New(root, graph.BuildFromNotes(notes, resources))
}

func TestHTMLExistingDialectRegressions(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "ruby markup passes through untouched",
			body: "<ruby>今日<rt>きょう</rt></ruby>は晴れ。",
			want: "<ruby>今日<rt>きょう</rt></ruby>",
		},
		{
			name: "explicit br passes through",
			body: "一行目<br>二行目",
			want: "<br>",
		},
		{
			name: "gfm table renders",
			body: "| a | b |\n|---|---|\n| 1 | 2 |\n",
			want: "<table>",
		},
		{
			name: "gfm task list renders",
			body: "- [ ] todo\n- [x] done\n",
			want: `type="checkbox"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML(tt.body)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q).HTML missing %q:\n%s", tt.body, tt.want, got.HTML)
			}
		})
	}
}

// TestTableWrappedForOverflow pins the horizontal-overflow guard: every GFM
// table is nested in a scroll container so a table too wide for the reading
// column scrolls inside its own box instead of stretching the article. The
// element stays a real <table> with its header and cells intact — the wrapper
// changes only the outer element, so the table's semantics survive.
func TestTableWrappedForOverflow(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	// A wide table: an unbreakable token in one cell is exactly what pushes a
	// table past the column and, without the wrapper, spills into the article.
	body := "| head | value |\n|---|---|\n| a | superlongunbreakabletokenwithoutspaces_0123456789 |\n"
	got := r.HTML(body).HTML

	if !strings.Contains(got, `<div class="k-tablewrap"><table>`) {
		t.Errorf("table is not wrapped in the scroll container:\n%s", got)
	}
	if !strings.Contains(got, "</table></div>") {
		t.Errorf("table wrapper is not closed:\n%s", got)
	}
	// The real table survives — the wrapper must not have stripped table markup.
	for _, want := range []string{"<thead>", "<th>head</th>", "<td>a</td>"} {
		if !strings.Contains(got, want) {
			t.Errorf("wrapped table lost %q:\n%s", want, got)
		}
	}
	// Exactly one wrapper for one table — no double wrap, no stray container.
	if n := strings.Count(got, "k-tablewrap"); n != 1 {
		t.Errorf("k-tablewrap count = %d, want 1:\n%s", n, got)
	}
}

// TestHeadingSlugsSkipNestedRawHeading pins the graceful-skip: a heading with a
// raw inline <hN> in it (an authoring accident goldmark+WithUnsafe passes
// through verbatim) is left byte-identical — no id assigned, absent from the
// TOC — instead of being truncated by the <h1-6> pass's non-greedy match
// stopping at the inner </hN>. Mirrors the TTS nested-<p> guard.
func TestHeadingSlugsSkipNestedRawHeading(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	got := r.HTML("## foo <h3>bar</h3> baz\n")

	// Without the guard the match stops at the inner </h3>, re-emitting
	// `<h2 id="foo-bar">foo <h3>bar</h2> baz</h2>` — a corrupted, unbalanced
	// heading carrying an id and a TOC entry. The guard leaves it untouched.
	if strings.Contains(got.HTML, "id=") {
		t.Errorf("a heading with a nested raw <hN> must not get an id:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "<h3>bar</h3>") {
		t.Errorf("the nested <h3> must survive intact (not truncated to <h3>bar</h2>):\n%s", got.HTML)
	}
	if len(got.TOC) != 0 {
		t.Errorf("a skipped heading must not appear in the TOC; got %d entries: %#v", len(got.TOC), got.TOC)
	}
}

func TestWikilinkUnique(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), []graph.NoteInput{{Path: "Target.md"}}, nil)

	got := r.HTML("See [[Target]] for details.\n")
	want := `<a href="/notes/Target.md" class="wikilink">Target</a>`
	if !strings.Contains(got.HTML, want) {
		t.Errorf("HTML().HTML missing %q:\n%s", want, got.HTML)
	}
	if len(got.Diagnostics) != 0 {
		t.Errorf("Diagnostics = %+v, want none for a resolvable wikilink", got.Diagnostics)
	}
}

func TestWikilinkAmbiguous(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), []graph.NoteInput{{Path: "a/Foo.md"}, {Path: "b/Foo.md"}}, nil)

	got := r.HTML("[[Foo]]\n")
	if !strings.Contains(got.HTML, `class="wikilink-ambiguous"`) {
		t.Errorf("HTML().HTML missing wikilink-ambiguous span:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, `class="wikilink"`) {
		t.Errorf("an ambiguous target must not render as a plain link:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkAmbiguous {
		t.Errorf("Diagnostics = %+v, want one DiagWikilinkAmbiguous", got.Diagnostics)
	}
}

func TestWikilinkBroken(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	got := r.HTML("[[Ghost]]\n")
	if !strings.Contains(got.HTML, `class="wikilink-broken"`) {
		t.Errorf("HTML().HTML missing wikilink-broken span:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkBroken {
		t.Errorf("Diagnostics = %+v, want one DiagWikilinkBroken", got.Diagnostics)
	}
}

func TestWikilinkBareAnchorIsPlainText(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	got := r.HTML("jump [[#Section]] here\n")
	if strings.Contains(got.HTML, "wikilink") {
		t.Errorf("a same-file anchor must not become any wikilink markup:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "#Section") {
		t.Errorf("HTML().HTML missing literal display text %q:\n%s", "#Section", got.HTML)
	}
	if len(got.Diagnostics) != 0 {
		t.Errorf("Diagnostics = %+v, want none — a same-file anchor is not resolved at all", got.Diagnostics)
	}
}

func TestEmbedTranscludesNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "B.md", "B's own body text.\n")

	r := newRenderer(t, root, []graph.NoteInput{{Path: "B.md"}}, nil)
	got := r.HTML("![[B]]\n")

	if !strings.Contains(got.HTML, `<div class="embed">`) {
		t.Errorf("HTML().HTML missing embed container:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "B's own body text.") {
		t.Errorf("HTML().HTML missing transcluded body text:\n%s", got.HTML)
	}
}

// TestEmbedDepthCapPreventsCycles constructs two notes that embed each
// other (A embeds B, B embeds A) and asserts the render terminates (the
// test itself completing is part of that proof) and produces sane
// output: exactly one level of transclusion, not an infinite chain.
func TestEmbedDepthCapPreventsCycles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "B.md", "![[A]]\n")

	r := newRenderer(t, root, []graph.NoteInput{{Path: "A.md"}, {Path: "B.md"}}, nil)
	got := r.HTML("![[B]]\n")

	if n := strings.Count(got.HTML, `class="embed"`); n != 1 {
		t.Errorf("embed container count = %d, want exactly 1 (one level of transclusion):\n%s", n, got.HTML)
	}
	// B's own "![[A]]" must render as a plain wikilink-style link, not a
	// second, nested transclusion.
	if !strings.Contains(got.HTML, `<a href="/notes/A.md" class="wikilink">A</a>`) {
		t.Errorf("HTML().HTML missing the depth-capped plain link for A:\n%s", got.HTML)
	}
}

func TestEmbedNonMarkdownTargetIsPlaceholder(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, []string{"Diagrams/x.canvas"})

	got := r.HTML("![[x.canvas]]\n")
	if !strings.Contains(got.HTML, `class="embed-media"`) {
		t.Errorf("HTML().HTML missing embed-media placeholder:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "x.canvas") {
		t.Errorf("HTML().HTML missing the resource's filename:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 0 {
		t.Errorf("Diagnostics = %+v, want none — a resolved-but-unsupported embed is not a diagnostic", got.Diagnostics)
	}
}

func TestEmbedUnresolvedAndAmbiguous(t *testing.T) {
	t.Parallel()

	t.Run("unresolved", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, t.TempDir(), nil, nil)
		got := r.HTML("![[Ghost]]\n")
		if !strings.Contains(got.HTML, `class="wikilink-broken"`) {
			t.Errorf("HTML().HTML missing wikilink-broken span:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkBroken {
			t.Errorf("Diagnostics = %+v, want one DiagWikilinkBroken", got.Diagnostics)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, t.TempDir(), []graph.NoteInput{{Path: "a/Dup.md"}, {Path: "b/Dup.md"}}, nil)
		got := r.HTML("![[Dup]]\n")
		if !strings.Contains(got.HTML, `class="wikilink-ambiguous"`) {
			t.Errorf("HTML().HTML missing wikilink-ambiguous span:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkAmbiguous {
			t.Errorf("Diagnostics = %+v, want one DiagWikilinkAmbiguous", got.Diagnostics)
		}
	})
}

// TestCalloutTypeTable covers every recognized callout type: correct
// bucket class and default English title, with no explicit title given.
func TestCalloutTypeTable(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	tests := []struct {
		typ, bucketClass, title string
	}{
		{"info", "note", "Note"}, {"note", "note", "Note"}, {"tip", "note", "Note"},
		{"hint", "note", "Note"}, {"abstract", "note", "Note"}, {"summary", "note", "Note"},
		{"todo", "note", "Note"},
		{"question", "note", "Question"}, {"help", "note", "Question"}, {"faq", "note", "Question"},
		{"example", "note", "Example"}, {"quote", "note", "Example"}, {"cite", "note", "Example"},
		{"warning", "warning", "Warning"}, {"caution", "warning", "Warning"}, {"attention", "warning", "Warning"},
		{"danger", "warning", "Danger"}, {"error", "warning", "Danger"}, {"bug", "warning", "Danger"},
		{"fail", "warning", "Danger"}, {"failure", "warning", "Danger"}, {"missing", "warning", "Danger"},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			t.Parallel()
			body := "> [!" + tt.typ + "]\n> body text\n"
			got := r.HTML(body)
			wantClass := `class="callout callout-` + tt.bucketClass + `"`
			if !strings.Contains(got.HTML, wantClass) {
				t.Errorf("[!%s] HTML missing %q:\n%s", tt.typ, wantClass, got.HTML)
			}
			if !strings.Contains(got.HTML, tt.title) {
				t.Errorf("[!%s] HTML missing default title %q:\n%s", tt.typ, tt.title, got.HTML)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("[!%s] Diagnostics = %+v, want none — a known type is not a diagnostic", tt.typ, got.Diagnostics)
			}
		})
	}
}

func TestCalloutFoldSuffixes(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	t.Run("closed by default (-)", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("> [!note]-\n> hidden\n")
		if !strings.Contains(got.HTML, `<details class="callout callout-note">`) {
			t.Errorf("HTML().HTML missing closed <details>:\n%s", got.HTML)
		}
	})

	t.Run("open by default (+)", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("> [!note]+\n> shown\n")
		if !strings.Contains(got.HTML, `<details class="callout callout-note" open>`) {
			t.Errorf("HTML().HTML missing open <details>:\n%s", got.HTML)
		}
	})

	t.Run("static, no fold control", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("> [!note]\n> static\n")
		if strings.Contains(got.HTML, "<details") {
			t.Errorf("a static (no-suffix) callout must not render <details>:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, `<div class="callout callout-note">`) {
			t.Errorf("HTML().HTML missing static callout div:\n%s", got.HTML)
		}
	})
}

func TestCalloutUnknownTypeFallsBackToBlockquote(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	got := r.HTML("> [!banana] Weird\n> body\n")
	if !strings.Contains(got.HTML, "<blockquote>") {
		t.Errorf("HTML().HTML missing plain <blockquote> fallback:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, "callout") {
		t.Errorf("an unknown callout type must not get any callout styling:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagUnknownCallout || got.Diagnostics[0].Target != "banana" {
		t.Errorf("Diagnostics = %+v, want one DiagUnknownCallout for target %q", got.Diagnostics, "banana")
	}
}

func TestCalloutBodyRendersNestedWikilinks(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), []graph.NoteInput{{Path: "Target.md"}}, nil)

	got := r.HTML("> [!note]\n> See [[Target]] here\n")
	want := `<a href="/notes/Target.md" class="wikilink">Target</a>`
	if !strings.Contains(got.HTML, want) {
		t.Errorf("a callout's body must be rendered through the same pipeline (nested wikilinks); HTML missing %q:\n%s", want, got.HTML)
	}
}

func TestHighlightRendersMark(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	got := r.HTML("plain ==highlighted== text\n")
	if !strings.Contains(got.HTML, "<mark>highlighted</mark>") {
		t.Errorf("HTML().HTML missing <mark>highlighted</mark>:\n%s", got.HTML)
	}
}

func TestHighlightIgnoresCodeSpan(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	got := r.HTML("literal `==not==` marker\n")
	if strings.Contains(got.HTML, "<mark>") {
		t.Errorf("== inside a code span must not become <mark>:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "<code>==not==</code>") {
		t.Errorf("HTML().HTML missing untouched code span:\n%s", got.HTML)
	}
}

func TestHeadingSlugsCJKAndCollision(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	body := "## 日本語 Go！\n\ntext\n\n## 日本語 Go！\n\nmore text\n"
	got := r.HTML(body)

	want := []render.TOCEntry{
		{Level: 2, Text: "日本語 Go！", ID: "日本語-go"},
		{Level: 2, Text: "日本語 Go！", ID: "日本語-go-2"},
	}
	if diff := cmp.Diff(want, got.TOC); diff != "" {
		t.Errorf("TOC mismatch (-want +got):\n%s", diff)
	}
	if !strings.Contains(got.HTML, `id="日本語-go"`) || !strings.Contains(got.HTML, `id="日本語-go-2"`) {
		t.Errorf("HTML().HTML missing the assigned heading ids:\n%s", got.HTML)
	}
}

func TestHeadingSlugFallsBackToSection(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	// A heading whose text is entirely punctuation strips to nothing —
	// slugify falls back to the literal string "section". (A trailing
	// run of "#" would be parsed as ATX's optional closing sequence and
	// stripped from the text by goldmark itself, so this uses "!" only.)
	got := r.HTML("## !!! !!!\n")
	want := []render.TOCEntry{{Level: 2, Text: "!!! !!!", ID: "section"}}
	if diff := cmp.Diff(want, got.TOC); diff != "" {
		t.Errorf("TOC mismatch (-want +got):\n%s", diff)
	}
}

func TestHeadingSlugStripsRubyReading(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	// A furigana heading keeps only its base characters in the entry and the
	// anchor — the reading inside <rt> must not echo after the kanji.
	got := r.HTML("## <ruby>漢字<rt>かんじ</rt></ruby>\n")

	want := []render.TOCEntry{{Level: 2, Text: "漢字", ID: "漢字"}}
	if diff := cmp.Diff(want, got.TOC); diff != "" {
		t.Errorf("TOC mismatch (-want +got):\n%s", diff)
	}
	// The heading body is untouched: the ruby markup survives byte-for-byte and
	// the heading carries the base-character id.
	if !strings.Contains(got.HTML, `<ruby>漢字<rt>かんじ</rt></ruby>`) {
		t.Errorf("HTML().HTML dropped the ruby markup from the heading body:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `id="漢字"`) {
		t.Errorf("HTML().HTML missing the base-character heading id:\n%s", got.HTML)
	}
}

func TestFenceSafety(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), []graph.NoteInput{{Path: "Real.md"}}, nil)

	body := "before\n\n```text\n[[Fake Link]]\n> [!note] also risky\n```\n\nafter [[Real]]\n"
	got := r.HTML(body)

	// "[[Fake Link]]" surviving literally IS the proof it was never
	// converted — a converted wikilink would replace this exact text with
	// <a .../> or <span class="wikilink-broken">Fake Link</span> markup,
	// so the raw brackets could no longer appear intact.
	if !strings.Contains(got.HTML, "[[Fake Link]]") {
		t.Errorf("wikilink-looking text inside a fence must survive untouched; HTML:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "[!note] also risky") {
		t.Errorf("callout-looking text inside a fence must survive untouched; HTML:\n%s", got.HTML)
	}

	var risky int
	for _, d := range got.Diagnostics {
		if d.Kind == render.DiagRiskyFence {
			risky++
		}
	}
	if risky != 1 {
		t.Errorf("risky-fence diagnostic count = %d, want exactly 1 (not zero, not several)", risky)
	}

	if !strings.Contains(got.HTML, `<a href="/notes/Real.md" class="wikilink">Real</a>`) {
		t.Errorf("a real wikilink outside the fence must still be converted; HTML:\n%s", got.HTML)
	}
}

func TestBodyFirstH1RemovedOnlyWhenTrulyFirst(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	t.Run("leading H1 (after blank lines) is removed", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("\n\n# Title\n\nbody text\n")
		if strings.Contains(got.HTML, "Title") {
			t.Errorf("the leading H1 must be removed entirely, not shown as a paragraph; HTML:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "body text") {
			t.Errorf("HTML().HTML missing the rest of the body:\n%s", got.HTML)
		}
	})

	t.Run("a later H1 is untouched when it is not first", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("# Title\n\nbody\n\n## Second\n\n# NotFirst\n")
		if strings.Contains(got.HTML, "Title") {
			t.Errorf("the truly-first H1 must still be removed; HTML:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "NotFirst") {
			t.Errorf("a second, non-first H1 must be left untouched; HTML:\n%s", got.HTML)
		}
	})

	t.Run("no removal when the first content is not an H1", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("Not a heading\n\n# Title\n")
		if !strings.Contains(got.HTML, "Title") {
			t.Errorf("an H1 that is not the document's first content must be kept; HTML:\n%s", got.HTML)
		}
	})
}

// TestMermaidFenceRendersDiagramDiv covers wikilink.go's consumeMermaid:
// a ```mermaid fence must become exactly one div.mermaid-diagram element
// carrying the raw source twice — human-readable (HTML-escaped) as text
// content for the no-JS/SSR fallback, and URL-encoded in data-mermaid-code
// for assets/js/yomihon.js to decode client-side. The two encodings must
// not corrupt each other (net/url.QueryEscape's output charset never
// needs HTML-attribute escaping, so there is no double-encoding to get
// wrong — see consumeMermaid's doc comment).
func TestMermaidFenceRendersDiagramDiv(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, t.TempDir(), nil, nil)

	src := "graph TD\n  A[\"a & b\"] --> B{decide?}"
	got := r.HTML("```mermaid\n" + src + "\n```\n")

	wantText := `<div class="mermaid-diagram" data-mermaid-code="graph+TD%0A++A%5B%22a+%26+b%22%5D+--%3E+B%7Bdecide%3F%7D">graph TD
  A[&#34;a &amp; b&#34;] --&gt; B{decide?}</div>`
	if !strings.Contains(got.HTML, wantText) {
		t.Errorf("HTML().HTML missing the expected mermaid-diagram div; HTML:\n%s\nwant substring:\n%s", got.HTML, wantText)
	}

	// The data attribute's value, independently URL-decoded, must
	// reproduce the exact original source — proof the encode/escape pass
	// didn't corrupt it.
	start := strings.Index(got.HTML, `data-mermaid-code="`) + len(`data-mermaid-code="`)
	end := start + strings.Index(got.HTML[start:], `"`)
	encoded := got.HTML[start:end]
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		t.Fatalf("data-mermaid-code is not valid URL-encoding: %v", err)
	}
	if decoded != src {
		t.Errorf("data-mermaid-code round-trip = %q, want %q", decoded, src)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}
