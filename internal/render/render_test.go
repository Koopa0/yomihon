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

type transclusions map[string]string

func (b transclusions) Transclusion(path string) (string, bool) {
	body, ok := b[path]
	return body, ok
}

// newRenderer builds a Pipeline from one in-memory graph and captured body
// set. Tests never need a filesystem to exercise the rendering projection.
func newRenderer(t *testing.T, notes []graph.NoteInput, resources []string, bodies transclusions) *render.Pipeline {
	t.Helper()
	return render.New(graph.BuildFromNotes(notes, resources), bodies)
}

func TestHTMLExistingDialectRegressions(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

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
			got := r.HTML("note.md", "", tt.body)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q).HTML missing %q:\n%s", tt.body, tt.want, got.HTML)
			}
		})
	}
}

func TestHTMLPreservesOnlyAuthorizedAuthoredMarkup(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	body := strings.Join([]string{
		`<ruby lang="ja">今日<rt lang="ja">きょう</rt><rp>（</rp></ruby><br>`,
		`<script>globalThis.noteScriptRan = true</script>`,
		`<meta http-equiv="refresh" content="0;url=https://example.invalid/leave">`,
		`<img src="https://example.invalid/pixel" onerror="globalThis.noteEventRan = true">`,
		`<ruby onclick="globalThis.noteEventRan = true">危険</ruby>`,
		`<style>body { background: url(https://example.invalid/style) }</style>`,
	}, "\n\n")

	got := r.HTML("note.md", "", body).HTML
	for _, want := range []string{
		`<ruby lang="ja">今日<rt lang="ja">きょう</rt><rp>（</rp></ruby><br>`,
		`&lt;script&gt;globalThis.noteScriptRan = true&lt;/script&gt;`,
		`&lt;meta http-equiv=&quot;refresh&quot;`,
		`&lt;img src=&quot;https://example.invalid/pixel&quot;`,
		`&lt;ruby onclick=&quot;globalThis.noteEventRan = true&quot;&gt;危険</ruby>`,
		`&lt;style&gt;body { background: url(https://example.invalid/style) }&lt;/style&gt;`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML() missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{
		`<script>`,
		`<meta http-equiv=`,
		`<img src="https://example.invalid`,
		`<ruby onclick=`,
		`<style>`,
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("HTML() retained active authored markup %q:\n%s", forbidden, got)
		}
	}
}

func TestHTMLTurnsRemoteMarkdownImagesIntoExplicitLinks(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", strings.Join([]string{
		`![remote chart](https://example.invalid/chart.png "chart")`,
		`![scheme relative](//example.invalid/pixel.png)`,
		`![local diagram](/raw/Diagrams/diagram.png)`,
		`![embedded pixel](data:image/png;base64,iVBORw0KGgo=)`,
		`[explicit external link](https://example.invalid/read)`,
		`[dangerous link](javascript:globalThis.linkRan=true)`,
	}, "\n\n")).HTML

	for _, forbidden := range []string{
		`<img src="https://example.invalid/chart.png"`,
		`<img src="//example.invalid/pixel.png"`,
		`href="javascript:`,
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("HTML() retained automatic or dangerous URL %q:\n%s", forbidden, got)
		}
	}
	for _, want := range []string{
		`<a href="https://example.invalid/chart.png" rel="external noreferrer" referrerpolicy="no-referrer">remote chart</a>`,
		`<a href="//example.invalid/pixel.png" rel="external noreferrer" referrerpolicy="no-referrer">scheme relative</a>`,
		`<img src="/raw/Diagrams/diagram.png" alt="local diagram">`,
		`<img src="data:image/png;base64,iVBORw0KGgo=" alt="embedded pixel">`,
		`<a href="https://example.invalid/read">explicit external link</a>`,
		`<a href="">dangerous link</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML() missing %q:\n%s", want, got)
		}
	}
}

func TestHTMLDoesNotLetAuthoredTextSelectRendererBlocks(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "Target.md"}}, nil, nil)

	got := r.HTML("note.md", "", `<!--yomihon-block:0--> [[Target]]`).HTML
	if !strings.Contains(got, `&lt;!--yomihon-block:0--&gt;`) {
		t.Fatalf("authored reserved marker was not rendered as inert text: %s", got)
	}
	if n := strings.Count(got, `<a href="/notes/Target.md" class="wikilink">Target</a>`); n != 1 {
		t.Errorf("renderer-owned wikilink count = %d, want exactly 1: %s", n, got)
	}
}

// TestTableWrappedForOverflow pins the horizontal-overflow guard: every GFM
// table is nested in a scroll container so a table too wide for the reading
// column scrolls inside its own box instead of stretching the article. The
// element stays a real <table> with its header and cells intact — the wrapper
// changes only the outer element, so the table's semantics survive.
func TestTableWrappedForOverflow(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	// A wide table: an unbreakable token in one cell is exactly what pushes a
	// table past the column and, without the wrapper, spills into the article.
	body := "| head | value |\n|---|---|\n| a | superlongunbreakabletokenwithoutspaces_0123456789 |\n"
	got := r.HTML("note.md", "", body).HTML

	if !strings.Contains(got, `<div class="y-tablewrap"><table>`) {
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
	if n := strings.Count(got, "y-tablewrap"); n != 1 {
		t.Errorf("y-tablewrap count = %d, want 1:\n%s", n, got)
	}
}

// TestHeadingSlugsTreatInertRawHeadingAsText pins the new authority boundary:
// an authored heading tag is visible source text, not a nested live heading.
// The containing Markdown heading therefore remains a normal navigable heading
// instead of taking the old WithUnsafe corruption-avoidance path.
func TestHeadingSlugsTreatInertRawHeadingAsText(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "## foo <h3>bar</h3> baz\n")

	const wantHTML = "<h2 id=\"foo-h3-bar-h3-baz\">foo &lt;h3&gt;bar&lt;/h3&gt; baz</h2>\n"
	if got.HTML != wantHTML {
		t.Errorf("HTML() = %q, want the authored tag inert inside one navigable heading %q", got.HTML, wantHTML)
	}
	wantTOC := []render.TOCEntry{{Level: 2, Text: "foo <h3>bar</h3> baz", ID: "foo-h3-bar-h3-baz"}}
	if diff := cmp.Diff(wantTOC, got.TOC); diff != "" {
		t.Errorf("TOC mismatch (-want +got):\n%s", diff)
	}
}

func TestWikilinkUnique(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "Target.md"}}, nil, nil)

	got := r.HTML("note.md", "", "See [[Target]] for details.\n")
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
	r := newRenderer(t, []graph.NoteInput{{Path: "a/Foo.md"}, {Path: "b/Foo.md"}}, nil, nil)

	got := r.HTML("note.md", "", "[[Foo]]\n")
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
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "[[Ghost]]\n")
	if !strings.Contains(got.HTML, `class="wikilink-broken"`) {
		t.Errorf("HTML().HTML missing wikilink-broken span:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkBroken {
		t.Errorf("Diagnostics = %+v, want one DiagWikilinkBroken", got.Diagnostics)
	}
}

func TestWikilinkBareAnchorIsPlainText(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "jump [[#Section]] here\n")
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
	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, transclusions{
		"B.md": "B's own body text.\n",
	})
	got := r.HTML("note.md", "", "![[B]]\n")

	if !strings.Contains(got.HTML, `<div class="embed">`) {
		t.Errorf("HTML().HTML missing embed container:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "B's own body text.") {
		t.Errorf("HTML().HTML missing transcluded body text:\n%s", got.HTML)
	}
}

func TestEmbedUsesTheCapturedGeneration(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "vault")
	const captured = "captured body\n"
	writeFile(t, root, "B.md", captured)

	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, transclusions{
		"B.md": captured,
	})
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove captured vault: %v", err)
	}
	writeFile(t, root, "B.md", "replacement body\n")

	got := r.HTML("note.md", "", "![[B]]\n")
	if !strings.Contains(got.HTML, "captured body") {
		t.Errorf("HTML().HTML = %q, want the captured transclusion body", got.HTML)
	}
	if strings.Contains(got.HTML, "replacement body") {
		t.Errorf("HTML().HTML read the replacement source tree: %q", got.HTML)
	}
}

func TestEmbedReportsMissingCapturedBody(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, nil)

	got := r.HTML("note.md", "", "![[B]]\n")
	if !strings.Contains(got.HTML, `class="wikilink-broken"`) {
		t.Errorf("HTML().HTML = %q, want a broken embed diagnostic", got.HTML)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkBroken {
		t.Errorf("HTML().Diagnostics = %+v, want one broken-wikilink diagnostic", got.Diagnostics)
	}
}

// TestEmbedDepthCapPreventsCycles constructs two notes that embed each
// other (A embeds B, B embeds A) and asserts the render terminates (the
// test itself completing is part of that proof) and produces sane
// output: exactly one level of transclusion, not an infinite chain.
func TestEmbedDepthCapPreventsCycles(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "A.md"}, {Path: "B.md"}}, nil, transclusions{
		"B.md": "![[A]]\n",
	})
	got := r.HTML("note.md", "", "![[B]]\n")

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
	r := newRenderer(t, nil, []string{"Diagrams/x.canvas"}, nil)

	got := r.HTML("note.md", "", "![[x.canvas]]\n")
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
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", "![[Ghost]]\n")
		if !strings.Contains(got.HTML, `class="wikilink-broken"`) {
			t.Errorf("HTML().HTML missing wikilink-broken span:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkBroken {
			t.Errorf("Diagnostics = %+v, want one DiagWikilinkBroken", got.Diagnostics)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{Path: "a/Dup.md"}, {Path: "b/Dup.md"}}, nil, nil)
		got := r.HTML("note.md", "", "![[Dup]]\n")
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
	r := newRenderer(t, nil, nil, nil)

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
			got := r.HTML("note.md", "", body)
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
	r := newRenderer(t, nil, nil, nil)

	t.Run("closed by default (-)", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "> [!note]-\n> hidden\n")
		if !strings.Contains(got.HTML, `<details class="callout callout-note">`) {
			t.Errorf("HTML().HTML missing closed <details>:\n%s", got.HTML)
		}
	})

	t.Run("open by default (+)", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "> [!note]+\n> shown\n")
		if !strings.Contains(got.HTML, `<details class="callout callout-note" open>`) {
			t.Errorf("HTML().HTML missing open <details>:\n%s", got.HTML)
		}
	})

	t.Run("static, no fold control", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "> [!note]\n> static\n")
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
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "> [!banana] Weird\n> body\n")
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
	r := newRenderer(t, []graph.NoteInput{{Path: "Target.md"}}, nil, nil)

	got := r.HTML("note.md", "", "> [!note]\n> See [[Target]] here\n")
	want := `<a href="/notes/Target.md" class="wikilink">Target</a>`
	if !strings.Contains(got.HTML, want) {
		t.Errorf("a callout's body must be rendered through the same pipeline (nested wikilinks); HTML missing %q:\n%s", want, got.HTML)
	}
}

func TestHighlightRendersMark(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "plain ==highlighted== text\n")
	if !strings.Contains(got.HTML, "<mark>highlighted</mark>") {
		t.Errorf("HTML().HTML missing <mark>highlighted</mark>:\n%s", got.HTML)
	}
}

func TestHighlightIgnoresCodeSpan(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "literal `==not==` marker\n")
	if strings.Contains(got.HTML, "<mark>") {
		t.Errorf("== inside a code span must not become <mark>:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "<code>==not==</code>") {
		t.Errorf("HTML().HTML missing untouched code span:\n%s", got.HTML)
	}
}

func TestHeadingSlugsCJKAndCollision(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	body := "## 日本語 Go！\n\ntext\n\n## 日本語 Go！\n\nmore text\n"
	got := r.HTML("note.md", "", body)

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
	r := newRenderer(t, nil, nil, nil)

	// A heading whose text is entirely punctuation strips to nothing —
	// slugify falls back to the literal string "section". (A trailing
	// run of "#" would be parsed as ATX's optional closing sequence and
	// stripped from the text by goldmark itself, so this uses "!" only.)
	got := r.HTML("note.md", "", "## !!! !!!\n")
	want := []render.TOCEntry{{Level: 2, Text: "!!! !!!", ID: "section"}}
	if diff := cmp.Diff(want, got.TOC); diff != "" {
		t.Errorf("TOC mismatch (-want +got):\n%s", diff)
	}
}

func TestHeadingSlugStripsRubyReading(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	// A furigana heading keeps only its base characters in the entry and the
	// anchor — the reading inside <rt> must not echo after the kanji. The second
	// heading carries an attribute on the reading tag, which must still be
	// stripped whole rather than leaving the reading behind.
	got := r.HTML("note.md", "", "## <ruby>漢字<rt>かんじ</rt></ruby>\n\n## <ruby>音<rt lang=\"ja\">おと</rt></ruby>\n\n## <ruby>加藤<rt>かとう</rt></ruby>\n")

	want := []render.TOCEntry{
		{Level: 2, Text: "漢字", ID: "漢字"},
		{Level: 2, Text: "音", ID: "音"},
		{Level: 2, Text: "加藤", ID: "加藤"},
	}
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
	r := newRenderer(t, []graph.NoteInput{{Path: "Real.md"}}, nil, nil)

	body := "before\n\n```text\n[[Fake Link]]\n> [!note] also risky\n```\n\nafter [[Real]]\n"
	got := r.HTML("note.md", "", body)

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
	r := newRenderer(t, nil, nil, nil)

	t.Run("leading H1 (after blank lines) is removed when it repeats the title", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "Title", "\n\n# Title\n\nbody text\n")
		if strings.Contains(got.HTML, "Title") {
			t.Errorf("the leading H1 must be removed entirely, not shown as a paragraph; HTML:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "body text") {
			t.Errorf("HTML().HTML missing the rest of the body:\n%s", got.HTML)
		}
	})

	t.Run("a later H1 is untouched when it is not first", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "Title", "# Title\n\nbody\n\n## Second\n\n# NotFirst\n")
		if strings.Contains(got.HTML, "Title") {
			t.Errorf("the truly-first H1 must still be removed; HTML:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "NotFirst") {
			t.Errorf("a second, non-first H1 must be left untouched; HTML:\n%s", got.HTML)
		}
	})

	// The case that matters on any folder without frontmatter, which is most
	// folders: the page is titled from the filename, so the opening heading is
	// the only place the document says what it is. Removing it left a reader
	// looking at "notes" where the file said "Quorum reads and writes".
	t.Run("a leading H1 survives when the page is titled from the filename", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("notes.md", "notes", "# Quorum reads and writes\n\nbody text\n")
		if !strings.Contains(got.HTML, "Quorum reads and writes") {
			t.Errorf("the document's own heading was destroyed and the filename put in its place; HTML:\n%s", got.HTML)
		}
	})

	// A heading that merely differs is also not the title being repeated.
	t.Run("a leading H1 that is not the title survives", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "Frontmatter Title", "# A different opening\n\nbody\n")
		if !strings.Contains(got.HTML, "A different opening") {
			t.Errorf("a heading that does not repeat the title must be kept; HTML:\n%s", got.HTML)
		}
	})

	t.Run("no removal when the first content is not an H1", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "Not a heading\n\n# Title\n")
		if !strings.Contains(got.HTML, "Title") {
			t.Errorf("an H1 that is not the document's first content must be kept; HTML:\n%s", got.HTML)
		}
	})
}

// TestMermaidFenceRendersDiagramDiv covers wikilink.go's consumeMermaid:
// a ```mermaid fence must become exactly one div.mermaid-diagram element
// carrying the raw source twice — human-readable (HTML-escaped) as text
// content for the no-JS/SSR fallback, and URL-encoded in data-mermaid-code
// for assets/js/diagrams.js to decode client-side. The two encodings must
// not corrupt each other (net/url.QueryEscape's output charset never
// needs HTML-attribute escaping, so there is no double-encoding to get
// wrong — see consumeMermaid's doc comment).
func TestMermaidFenceRendersDiagramDiv(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	src := "graph TD\n  A[\"a & b\"] --> B{decide?}"
	got := r.HTML("note.md", "", "```mermaid\n"+src+"\n```\n")

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
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// TestHTMLResolvesImagesAgainstTheNotesOwnDirectory locks the wiring, not the
// rewrite: the pass has to run inside HTML, because every caller that renders a
// body displays whatever comes back and none of them look at the sources again.
func TestHTMLResolvesImagesAgainstTheNotesOwnDirectory(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, nil, nil, nil)
	got := r.HTML("Concepts/golang/Slice.md", "", "![diagram](../../Assets/slice.png)\n").HTML

	const want = `src="/raw/Assets/slice.png"`
	if !strings.Contains(got, want) {
		t.Errorf("HTML() = %q, want an image resolved to %q", got, want)
	}
}

// TestEmbedResolvesImagesAgainstTheTranscludedNote is the case that neither the
// unit rewrite nor a single pass over the finished page can get right. A
// transcluded body was written somewhere else, so its images are relative to
// that note's directory; resolving the assembled page against the note being
// read would silently address a file beside the reader instead.
func TestEmbedResolvesImagesAgainstTheTranscludedNote(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{Path: "Concepts/golang/Slice.md"}}, nil, transclusions{
		"Concepts/golang/Slice.md": "![diagram](./slice.png)\n",
	})
	got := r.HTML("Journal/2026-07-26.md", "", "![[Slice]]\n").HTML

	const want = `src="/raw/Concepts/golang/slice.png"`
	const wrong = `src="/raw/Journal/slice.png"`
	if strings.Contains(got, wrong) {
		t.Errorf("HTML() resolved a transcluded image against the host note (%s):\n%s", wrong, got)
	}
	if !strings.Contains(got, want) {
		t.Errorf("HTML() = %q, want the transcluded image resolved to %q", got, want)
	}
}

// TestWikilinkInsideACodeSpanIsQuotedNotResolved covers the sentence that
// explains the syntax. A code span is the author showing a reader what a
// wikilink looks like; converting it substituted a renderer-owned placeholder,
// which goldmark then escaped as the span's literal content, so the page
// printed an internal comment in the middle of the prose. The link the author
// never made was also resolved, so the note collected a broken-link diagnostic
// for a target it does not link to.
func TestWikilinkInsideACodeSpanIsQuotedNotResolved(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "Concepts/Real.md"}}, nil, nil)

	got := r.HTML("Maps/MOC.md", "MOC", "用 `[[概念]]` 互連,以及 `![[嵌入]]` 禁止,而 [[Real]] 是真的。\n")

	if strings.Contains(got.HTML, "yomihon-block") {
		t.Errorf("a renderer placeholder reached the page:\n%s", got.HTML)
	}
	for _, want := range []string{"<code>[[概念]]</code>", "<code>![[嵌入]]</code>"} {
		if !strings.Contains(got.HTML, want) {
			t.Errorf("the quoted syntax was rewritten; want %q in:\n%s", want, got.HTML)
		}
	}
	if !strings.Contains(got.HTML, `class="wikilink"`) {
		t.Errorf("the real wikilink outside the code spans stopped resolving:\n%s", got.HTML)
	}
	for _, d := range got.Diagnostics {
		if strings.Contains(d.Target, "概念") || strings.Contains(d.Target, "嵌入") {
			t.Errorf("a quoted target was reported as a broken link: %+v", d)
		}
	}
}

// TestUnclosedBacktickRunIsOrdinaryText keeps the span scanner from swallowing
// the rest of a document: a run with no closer of the same width is not a code
// span, and a wikilink after it still resolves.
func TestUnclosedBacktickRunIsOrdinaryText(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "Concepts/Real.md"}}, nil, nil)

	got := r.HTML("note.md", "note", "一個沒有結尾的 ` 反引號,然後 [[Real]]。\n")
	if !strings.Contains(got.HTML, `class="wikilink"`) {
		t.Errorf("an unclosed backtick swallowed the rest of the document:\n%s", got.HTML)
	}
}
