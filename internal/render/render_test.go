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
	"github.com/koopa0/yomihon/internal/wording"
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
	return render.New(graph.BuildFromNotes(notes, resources), bodies, noTitles{})
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
			got := r.HTML("note.md", "", tt.body, wording.ZhHant)
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

	got := r.HTML("note.md", "", body, wording.ZhHant).HTML
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
	}, "\n\n"), wording.ZhHant).HTML

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
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, nil)

	got := r.HTML("note.md", "", `<!--yomihon-block:0--> [[Target]]`, wording.ZhHant).HTML
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
	got := r.HTML("note.md", "", body, wording.ZhHant).HTML

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

	got := r.HTML("note.md", "", "## foo <h3>bar</h3> baz\n", wording.ZhHant)

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
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, nil)

	got := r.HTML("note.md", "", "See [[Target]] for details.\n", wording.ZhHant)
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
	r := newRenderer(t, []graph.NoteInput{{RelPath: "a/Foo.md"}, {RelPath: "b/Foo.md"}}, nil, nil)

	got := r.HTML("note.md", "", "[[Foo]]\n", wording.ZhHant)
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

	got := r.HTML("note.md", "", "[[Ghost]]\n", wording.ZhHant)
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

	got := r.HTML("note.md", "", "jump [[#Section]] here\n", wording.ZhHant)
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

// TestBracketedOrEmptyInnerTextIsNotALink pins the wikilink token boundary:
// the inner text is one or more characters, none of them a square bracket. An
// inner bracket or an empty inner leaves the whole run as plain text — no
// link markup, no resolution, no diagnostic — matching how Obsidian refuses
// to open a link there.
func TestBracketedOrEmptyInnerTextIsNotALink(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "a.md"}}, nil, nil)

	for _, body := range []string{"before [[a[b]]] after\n", "before [[]] after\n", "before ![[]] after\n"} {
		got := r.HTML("note.md", "", body, wording.ZhHant)
		if strings.Contains(got.HTML, "wikilink") {
			t.Errorf("HTML(%q) produced wikilink markup:\n%s", body, got.HTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("HTML(%q) diagnostics = %+v, want none — text that is not a link resolves nothing", body, got.Diagnostics)
		}
	}

	if got := r.HTML("note.md", "", "before [[a[b]]] after\n", wording.ZhHant); !strings.Contains(got.HTML, "[[a[b]]]") {
		t.Errorf("HTML() dropped the literal bracketed text:\n%s", got.HTML)
	}
}

// headingID reads the id the renderer actually stamped on the heading whose
// text is want, so a test can ask where a link has to land instead of writing
// the answer down a second time.
func headingID(t *testing.T, res *render.Result, want string) string {
	t.Helper()
	for _, entry := range res.TOC {
		if entry.Text == want {
			if !strings.Contains(res.HTML, `id="`+entry.ID+`"`) {
				t.Fatalf("TOC reports id %q for heading %q, but the HTML does not carry it:\n%s", entry.ID, want, res.HTML)
			}
			return entry.ID
		}
	}
	t.Fatalf("no heading %q in the rendered destination:\n%+v", want, res.TOC)
	return ""
}

// TestWikilinkKeepsCrossNoteHeadingFragment is the fragment assertion site. A
// link written at a section means that section: dropping the "#heading" half
// of it puts the reader at the top of a long note with no sign of what they
// were promised.
//
// The expected fragment is read out of the destination page's own rendering
// rather than written down twice, because the property under test is that the
// two agree — a second slug rule that looked right in isolation would still
// send every reader to the wrong place.
func TestWikilinkKeepsCrossNoteHeadingFragment(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{
		{RelPath: "Writing/玻璃潮初稿.md"},
		{RelPath: "Sources/鹽霧碼頭.md"},
	}, nil, nil)

	draft := r.HTML("Writing/玻璃潮初稿.md", "玻璃潮初稿", "## 第三節：失約的燈\n\n內文。\n", wording.ZhHant)
	quay := r.HTML("Sources/鹽霧碼頭.md", "鹽霧碼頭", "## 可直接取用的感官素材\n\n內文。\n", wording.ZhHant)

	draftID := headingID(t, &draft, "第三節：失約的燈")
	quayID := headingID(t, &quay, "可直接取用的感官素材")

	// Both halves are pinned: the destination's id is this exact string, and
	// the link names it. Reading the id alone would still pass if the slug
	// rule itself changed under both.
	if draftID != "第三節-失約的燈" {
		t.Errorf("destination heading id = %q, want %q", draftID, "第三節-失約的燈")
	}
	if quayID != "可直接取用的感官素材" {
		t.Errorf("destination heading id = %q, want %q", quayID, "可直接取用的感官素材")
	}

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "heading fragment survives with the implicit display text",
			body: "[[玻璃潮初稿#第三節：失約的燈]]\n",
			want: `<a href="/notes/Writing/%E7%8E%BB%E7%92%83%E6%BD%AE%E5%88%9D%E7%A8%BF.md#` + draftID +
				`" class="wikilink">玻璃潮初稿#第三節：失約的燈</a>`,
		},
		{
			name: "explicit display text keeps its own words and still lands on the section",
			body: "[[鹽霧碼頭#可直接取用的感官素材|回到素材段落]]\n",
			want: `<a href="/notes/Sources/%E9%B9%BD%E9%9C%A7%E7%A2%BC%E9%A0%AD.md#` + quayID +
				`" class="wikilink">回到素材段落</a>`,
		},
		{
			name: "a link naming no section still addresses the note itself",
			body: "[[玻璃潮初稿]]\n",
			want: `<a href="/notes/Writing/%E7%8E%BB%E7%92%83%E6%BD%AE%E5%88%9D%E7%A8%BF.md" class="wikilink">玻璃潮初稿</a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Notes/來源.md", "", tt.body, wording.ZhHant)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q).HTML missing %q:\n%s", tt.body, tt.want, got.HTML)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("Diagnostics = %+v, want none for a resolvable wikilink", got.Diagnostics)
			}
		})
	}
}

// TestWikilinkFragmentOnlyWhenTheNoteIsCertain pins what a fragment must not
// do. It is an addition to a link that already resolved: a name yomihon cannot
// place, or places in more than one file, has no page for the fragment to be
// an offset into.
func TestWikilinkFragmentOnlyWhenTheNoteIsCertain(t *testing.T) {
	t.Parallel()

	t.Run("an unresolved target keeps the note name as the reported target", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", "[[Ghost#Some Section]]\n", wording.ZhHant)
		if strings.Contains(got.HTML, "href=") {
			t.Errorf("an unresolved target must not become a link at all:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Target != "Ghost" {
			t.Errorf("Diagnostics = %+v, want one naming the note %q", got.Diagnostics, "Ghost")
		}
	})

	t.Run("an ambiguous target gets no fragment", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "a/Foo.md"}, {RelPath: "b/Foo.md"}}, nil, nil)
		got := r.HTML("note.md", "", "[[Foo#Some Section]]\n", wording.ZhHant)
		if strings.Contains(got.HTML, "href=") {
			t.Errorf("an ambiguous target must not become a link at all:\n%s", got.HTML)
		}
	})

	// A block address answers to an anchor the destination page writes, so it
	// is written only against a body this generation actually read. Where the
	// name resolved but the bytes did not arrive, nothing is claimed in either
	// direction: yomihon cannot tell a block that is absent from one it never
	// saw, and both "it is missing" and "here is where it is" would be
	// statements about a note it did not read.
	blocks := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a block address written the way Obsidian writes it",
			body: "[[Target#^abc123]]\n",
			want: `<a href="/notes/Target.md" class="wikilink">Target#^abc123</a>`,
		},
		{
			name: "a bare block address",
			body: "[[Target^abc123]]\n",
			want: `<a href="/notes/Target.md" class="wikilink">Target^abc123</a>`,
		},
		{
			name: "a block address beside a section name",
			body: "[[Target^abc123#Internals]]\n",
			want: `<a href="/notes/Target.md" class="wikilink">Target^abc123#Internals</a>`,
		},
	}
	for _, tt := range blocks {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The note is in the resolver and its body was not captured, which
			// is what a generation that read the name but not the bytes looks
			// like.
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, nil)
			got := r.HTML("note.md", "", tt.body, wording.ZhHant)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q).HTML missing %q:\n%s", tt.body, tt.want, got.HTML)
			}
			for _, d := range got.Diagnostics {
				if d.Kind == render.DiagLinkFragmentMissing {
					t.Errorf("a block was called absent from a note whose body was never read: %s", d.Message)
				}
			}
		})
	}
}

// TestRemovedOpeningHeadingPassesItsAnchorToTheTitle is the assertion site for
// the one heading a reader can see and the document no longer contains. A note
// opening with a heading that repeats its title has that heading dropped, so
// the words appear once instead of twice — but a link written at that section
// still names it, and after the removal nothing in the body answers to the
// name. The anchor moves to where those words now are.
func TestRemovedOpeningHeadingPassesItsAnchorToTheTitle(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Notes/Glass Tide.md"}}, nil, nil)

	page := r.HTML("Notes/Glass Tide.md", "Glass Tide", "# Glass Tide\n\nOpening paragraph.\n\n## Later\n\n本文。\n", wording.ZhHant)

	if page.TitleAnchor == "" {
		t.Fatal("an authored opening heading was removed and left no anchor behind, so a link written at it reaches nothing")
	}
	if strings.Contains(page.HTML, "<h1") {
		t.Errorf("the opening heading was kept as well as the title, so the reader sees the name twice:\n%s", page.HTML)
	}

	// The link and the title have to agree, and the agreement is checked
	// against what the renderer produced rather than against a slug written
	// down here a second time.
	link := r.HTML("Notes/source.md", "", "[[Glass Tide#Glass Tide]]\n", wording.ZhHant)
	want := `<a href="/notes/Notes/Glass%20Tide.md#` + page.TitleAnchor + `"`
	if !strings.Contains(link.HTML, want) {
		t.Errorf("the link does not name the anchor the title carries; want %q in:\n%s", want, link.HTML)
	}
}

// TestTitleAnchorIsClaimedBeforeTheBodyIsSlugged covers the collision the
// transfer creates. The title's anchor is on the page but not in the body
// HTML, so a section further down reducing to the same name cannot see it and
// would issue the id a second time — the browser then resolves the title's own
// address to the section instead.
func TestTitleAnchorIsClaimedBeforeTheBodyIsSlugged(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	page := r.HTML("Notes/Glass Tide.md", "Glass Tide", "# Glass Tide\n\nOpening.\n\n## Glass Tide\n\n本文。\n", wording.ZhHant)

	if page.TitleAnchor != "glass-tide" {
		t.Fatalf("TitleAnchor = %q, want %q", page.TitleAnchor, "glass-tide")
	}
	if strings.Contains(page.HTML, `id="glass-tide"`) {
		t.Errorf("a body heading took the id the title already carries:\n%s", page.HTML)
	}
	if !strings.Contains(page.HTML, `id="glass-tide-2"`) {
		t.Errorf("the body heading did not move aside for the title's anchor:\n%s", page.HTML)
	}
	if len(page.TOC) != 1 || page.TOC[0].ID != "glass-tide-2" {
		t.Errorf("TOC = %+v, want the one body heading under its moved id", page.TOC)
	}
}

// TestTitleAnchorIsOnlyClaimedForARemovedHeading keeps the anchor honest. A
// note that never wrote an opening heading has no such section, and stamping
// the title anyway would answer a link naming a place the author did not
// write — the same invention this round removed from footnotes and fragments.
func TestTitleAnchorIsOnlyClaimedForARemovedHeading(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name  string
		title string
		body  string
	}{
		{name: "no opening heading at all", title: "Glass Tide", body: "Just prose.\n"},
		{name: "an opening heading that says something else", title: "Glass Tide", body: "# Salt Quay\n\nProse.\n"},
		{name: "a heading that is not the first thing in the body", title: "Glass Tide", body: "Prose.\n\n# Glass Tide\n"},
		{name: "a lower-level opening heading", title: "Glass Tide", body: "## Glass Tide\n\nProse.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := r.HTML("Notes/n.md", tt.title, tt.body, wording.ZhHant).TitleAnchor; got != "" {
				t.Errorf("TitleAnchor = %q, want empty — no authored opening heading was removed", got)
			}
		})
	}
}

// TestHeadingFragmentFoldsUnicodeForm is the assertion site for a link and a
// heading that read identically on screen and are different bytes underneath.
// macOS stores filenames decomposed and editors type composed, so a link
// written in one form and the heading it names written in the other is an
// ordinary thing to find in this vault — and the reader has no way to see the
// difference, so a link that silently misses looks like the renderer losing
// the section.
func TestHeadingFragmentFoldsUnicodeForm(t *testing.T) {
	t.Parallel()

	// Built from code points rather than typed, so the two forms stay legible
	// in the source and cannot be mixed up by an editor normalizing the file.
	const (
		composed   = "\u304C\u3093"       // がん
		decomposed = "\u304B\u3099\u3093" // か + combining dakuten, then ん
	)
	if composed == decomposed {
		t.Fatal("the two Unicode forms are byte-equal, so this test compares nothing")
	}

	tests := []struct {
		name        string
		destination string
		link        string
	}{
		{name: "composed link, decomposed heading", destination: decomposed, link: composed},
		{name: "decomposed link, composed heading", destination: composed, link: decomposed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Dest.md"}}, nil, nil)

			dest := r.HTML("Dest.md", "Dest", "## "+tt.destination+"\n\n本文。\n", wording.ZhHant)
			wanted := headingID(t, &dest, tt.destination)

			got := r.HTML("Notes/source.md", "", "[[Dest#"+tt.link+"]]\n", wording.ZhHant)
			want := `<a href="/notes/Dest.md#` + wanted + `"`
			if !strings.Contains(got.HTML, want) {
				t.Errorf("the link names a different anchor than the destination stamps; want %q in:\n%s", want, got.HTML)
			}
		})
	}
}

// TestNonMarkdownTargetTakesNoHeadingFragment keeps the fragment to the one
// kind of destination that has headings. A PDF, a canvas, or a picture is
// served whole; the reading page stamps nothing inside it, so a fragment
// appended to one names a place that cannot exist and, on a route that ignores
// it, quietly reads as though it worked.
func TestNonMarkdownTargetTakesNoHeadingFragment(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Notes/Real.md"}}, []string{
		"Attachments/paper.pdf",
		"Diagrams/canvas/board.canvas",
		"Attachments/plate.png",
	}, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a pdf",
			body: "[[paper.pdf#Section 2]]\n",
			want: `<a href="/notes/Attachments/paper.pdf" class="wikilink">paper.pdf#Section 2</a>`,
		},
		{
			name: "a canvas",
			body: "[[board.canvas#Overview]]\n",
			want: `<a href="/notes/Diagrams/canvas/board.canvas" class="wikilink">board.canvas#Overview</a>`,
		},
		{
			name: "a picture",
			body: "[[plate.png#Detail]]\n",
			want: `<a href="/notes/Attachments/plate.png" class="wikilink">plate.png#Detail</a>`,
		},
		{
			// The control: the same shape of link, at the one kind of
			// destination that does carry anchors, still gets its fragment.
			name: "a note, which does have headings",
			body: "[[Real#Section 2]]\n",
			want: `<a href="/notes/Notes/Real.md#section-2" class="wikilink">Real#Section 2</a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", tt.body, wording.ZhHant)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q).HTML missing %q:\n%s", tt.body, tt.want, got.HTML)
			}
		})
	}
}

func TestEmbedTranscludesNote(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": "B's own body text.\n",
	})
	got := r.HTML("note.md", "", "![[B]]\n", wording.ZhHant)

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

	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": captured,
	})
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove captured vault: %v", err)
	}
	writeFile(t, root, "B.md", "replacement body\n")

	got := r.HTML("note.md", "", "![[B]]\n", wording.ZhHant)
	if !strings.Contains(got.HTML, "captured body") {
		t.Errorf("HTML().HTML = %q, want the captured transclusion body", got.HTML)
	}
	if strings.Contains(got.HTML, "replacement body") {
		t.Errorf("HTML().HTML read the replacement source tree: %q", got.HTML)
	}
}

func TestEmbedReportsMissingCapturedBody(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, nil)

	got := r.HTML("note.md", "", "![[B]]\n", wording.ZhHant)
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
	r := newRenderer(t, []graph.NoteInput{{RelPath: "A.md"}, {RelPath: "B.md"}}, nil, transclusions{
		"B.md": "![[A]]\n",
	})
	got := r.HTML("note.md", "", "![[B]]\n", wording.ZhHant)

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

	got := r.HTML("note.md", "", "![[x.canvas]]\n", wording.ZhHant)
	if !strings.Contains(got.HTML, `class="embed-media"`) {
		t.Errorf("HTML().HTML missing embed-media placeholder:\n%s", got.HTML)
	}
	// The named file has a working page of its own, so the placeholder's
	// filename is a way there rather than dead text.
	if !strings.Contains(got.HTML, `<a href="/notes/Diagrams/x.canvas">x.canvas</a>`) {
		t.Errorf("HTML().HTML missing the placeholder's link to the file's own page:\n%s", got.HTML)
	}
	if len(got.Diagnostics) != 0 {
		t.Errorf("Diagnostics = %+v, want none — a resolved-but-unsupported embed is not a diagnostic", got.Diagnostics)
	}
}

// TestEmbedNonMarkdownTargetSpeaksTheReadersLanguage holds the one sentence in
// an article that was written in English whoever was reading it. Everything
// around it answers to the reader's choice — the notice for an unwritten name,
// three lines away in the same function, does — so a reader working in Chinese
// met one English sentence in the middle of their own page, standing where a
// file they embedded should have been.
func TestEmbedNonMarkdownTargetSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lang wording.Lang
		want string
	}{
		{name: "the reader chose Chinese", lang: wording.ZhHant, want: "還沒辦法"},
		{name: "the reader chose English", lang: wording.En, want: "inline display not yet supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, []string{"Notes/sample.pdf"}, nil)
			got := r.HTML("note.md", "", "![[sample.pdf]]\n", tt.lang).HTML

			if !strings.Contains(got, tt.want) {
				t.Errorf("the stub does not carry %q:\n%s", tt.want, got)
			}
			// The sentence names the file, and the name is the way to it.
			if !strings.Contains(got, `<a href="/notes/Notes/sample.pdf">sample.pdf</a>`) {
				t.Errorf("the stub lost its link to the file's own page:\n%s", got)
			}
		})
	}

	// Two languages that produced the same bytes would pass both rows above
	// while the sentence stayed written in one of them.
	r := newRenderer(t, nil, []string{"Notes/sample.pdf"}, nil)
	zh := r.HTML("note.md", "", "![[sample.pdf]]\n", wording.ZhHant).HTML
	en := r.HTML("note.md", "", "![[sample.pdf]]\n", wording.En).HTML
	if zh == en {
		t.Errorf("both readers are shown the same sentence:\n%s", zh)
	}
}

// TestEmbedPictureTargetPaintsInline is the assertion site for Obsidian's
// ordinary image syntax. A picture embed and a markdown image are the same
// request written two ways, so both must land on the raw-bytes route; before
// this branch existed the wikilink form got a labelled placeholder while the
// markdown form beside it painted, and a vault pasted full of ![[...]] images
// showed none of them.
func TestEmbedPictureTargetPaintsInline(t *testing.T) {
	t.Parallel()

	t.Run("plain filename embeds as an exact img element", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, []string{"attachments/pic.png"}, nil)
		got := r.HTML("note.md", "", "![[pic.png]]\n", wording.ZhHant)
		const wantHTML = "<p><img src=\"/raw/attachments/pic.png\" alt=\"pic.png\"></p>\n"
		if got.HTML != wantHTML {
			t.Errorf("HTML().HTML = %q, want exactly %q", got.HTML, wantHTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none for a resolvable picture embed", got.Diagnostics)
		}
	})

	tests := []struct {
		name     string
		resource string
		body     string
		want     string
	}{
		{
			name:     "space and cjk in the filename are percent-escaped per segment",
			resource: "attachments/pic with space 図.png",
			body:     "![[pic with space 図.png]]\n",
			want:     `<img src="/raw/attachments/pic%20with%20space%20%E5%9B%B3.png" alt="pic with space 図.png">`,
		},
		{
			name:     "full-path embed resolves to the same raw route",
			resource: "attachments/pic.png",
			body:     "![[attachments/pic.png]]\n",
			want:     `<img src="/raw/attachments/pic.png" alt="pic.png">`,
		},
		{
			name:     "svg embeds as a picture too",
			resource: "Diagrams/flow.svg",
			body:     "![[flow.svg]]\n",
			want:     `<img src="/raw/Diagrams/flow.svg" alt="flow.svg">`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, []string{tt.resource}, nil)
			got := r.HTML("note.md", "", tt.body, wording.ZhHant)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q).HTML missing %q:\n%s", tt.body, tt.want, got.HTML)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("Diagnostics = %+v, want none for a resolvable picture embed", got.Diagnostics)
			}
		})
	}

	t.Run("missing picture target keeps the broken-embed treatment", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", "![[missing.png]]\n", wording.ZhHant)
		if strings.Contains(got.HTML, "<img") {
			t.Errorf("an unresolved picture embed must not paint an image:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, `class="wikilink-broken"`) {
			t.Errorf("HTML().HTML missing wikilink-broken span:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkBroken {
			t.Errorf("Diagnostics = %+v, want one DiagWikilinkBroken", got.Diagnostics)
		}
	})

	t.Run("ambiguous picture target keeps the ambiguous treatment", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, []string{"a/pic.png", "b/pic.png"}, nil)
		got := r.HTML("note.md", "", "![[pic.png]]\n", wording.ZhHant)
		if strings.Contains(got.HTML, "<img") {
			t.Errorf("an ambiguous picture embed must not paint an image:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, `class="wikilink-ambiguous"`) {
			t.Errorf("HTML().HTML missing wikilink-ambiguous span:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkAmbiguous {
			t.Errorf("Diagnostics = %+v, want one DiagWikilinkAmbiguous", got.Diagnostics)
		}
	})
}

// TestEmbedHeadingFragmentScopesTheTransclusion is the assertion site for
// "![[note#section]]". Obsidian embeds only the named section — the heading
// through to the next heading of the same or a higher level — and an author
// who wrote the fragment scoped the excerpt on purpose, so showing the whole
// note presents content they deliberately left out as if they had chosen it.
// The match folds heading text the same way the destination's anchors are
// stamped, so an embed and a link to one section agree on what it is named.
func TestEmbedHeadingFragmentScopesTheTransclusion(t *testing.T) {
	t.Parallel()

	const body = "INTRO-MARKER opening prose.\n\n" +
		"## Alpha\n\nALPHA-MARKER section body.\n\n" +
		"### Alpha Sub\n\nSUB-MARKER nested body.\n\n" +
		"## Beta\n\nBETA-MARKER other body.\n"

	t.Run("the named section embeds with its subsections and nothing else", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": body})
		got := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, `<div class="embed">`) {
			t.Errorf("HTML().HTML missing embed container:\n%s", got.HTML)
		}
		for _, want := range []string{"ALPHA-MARKER", "SUB-MARKER"} {
			if !strings.Contains(got.HTML, want) {
				t.Errorf("the named section lost %q:\n%s", want, got.HTML)
			}
		}
		for _, forbidden := range []string{"INTRO-MARKER", "BETA-MARKER"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("content outside the named section leaked in (%q):\n%s", forbidden, got.HTML)
			}
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none for a section that exists", got.Diagnostics)
		}
	})

	// The author named one section. Answering with the whole note when the name
	// matches nothing is a wider answer than the one they wrote, so the page
	// keeps the note's words back and says what it could not find, with the way
	// on to the note for a reader who wants the rest.
	t.Run("a missing heading withholds the excerpt and says so", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": body})
		got := r.HTML("note.md", "", "![[B#No Such Section]]\n", wording.ZhHant)
		for _, forbidden := range []string{"INTRO-MARKER", "ALPHA-MARKER", "BETA-MARKER"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("a section the note does not have was widened into %q:\n%s", forbidden, got.HTML)
			}
		}
		for _, want := range []string{`class="embed embed--withheld"`, "#No Such Section", `href="/notes/B.md"`} {
			if !strings.Contains(got.HTML, want) {
				t.Errorf("the notice standing in for the excerpt lacks %s:\n%s", want, got.HTML)
			}
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagEmbedFragmentMissing {
			t.Fatalf("Diagnostics = %+v, want one DiagEmbedFragmentMissing", got.Diagnostics)
		}
		// The name and the address after it stay apart: readers that look a
		// planned name up by Target match a bare name and nothing else.
		if got, want := got.Diagnostics[0].Target, "B"; got != want {
			t.Errorf("Diagnostic.Target = %q, want the bare name %q", got, want)
		}
		if got, want := got.Diagnostics[0].Section, "No Such Section"; got != want {
			t.Errorf("Diagnostic.Section = %q, want %q", got, want)
		}
		if got := got.Diagnostics[0].Block; got != "" {
			t.Errorf("Diagnostic.Block = %q, want empty: a heading address is not a block address", got)
		}
	})

	// Nothing external rules on a duplicated section name inside an embed
	// target, so the choice is stated here: the first occurrence wins,
	// matching how Obsidian's reading view resolves the same embed — a
	// deterministic answer rather than a silent pick of a later candidate.
	// Which one is shown is no longer left unsaid, though: the pick is
	// reported so the note's own page can account for it.
	t.Run("a duplicated heading embeds its first occurrence", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "## Alpha\n\nFIRST-MARKER\n\n## Alpha\n\nSECOND-MARKER\n",
		})
		got := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "FIRST-MARKER") {
			t.Errorf("the first occurrence's body is missing:\n%s", got.HTML)
		}
		if strings.Contains(got.HTML, "SECOND-MARKER") {
			t.Errorf("a later occurrence leaked into the embed:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagEmbedFragmentRepeated {
			t.Fatalf("Diagnostics = %+v, want one DiagEmbedFragmentRepeated", got.Diagnostics)
		}
		if got, want := got.Diagnostics[0].Section, "Alpha"; got != want {
			t.Errorf("Diagnostic.Section = %q, want %q", got, want)
		}
	})

	t.Run("the heading match folds unicode form like the anchor rule", func(t *testing.T) {
		t.Parallel()
		const (
			composed   = "\u304C\u3093"       // がん
			decomposed = "\u304B\u3099\u3093" // か + combining dakuten, then ん
		)
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "OUTSIDE-MARKER\n\n## " + decomposed + "\n\nINSIDE-MARKER\n",
		})
		got := r.HTML("note.md", "", "![[B#"+composed+"]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "INSIDE-MARKER") {
			t.Errorf("the section written in the other unicode form was not found:\n%s", got.HTML)
		}
		if strings.Contains(got.HTML, "OUTSIDE-MARKER") {
			t.Errorf("content outside the named section leaked in:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none", got.Diagnostics)
		}
	})

	t.Run("a heading-looking line inside a fence is not a section start", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "```\n## Alpha\nFENCED-MARKER\n```\n\n## Alpha\n\nREAL-MARKER\n",
		})
		got := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "REAL-MARKER") {
			t.Errorf("the real section body is missing:\n%s", got.HTML)
		}
		if strings.Contains(got.HTML, "FENCED-MARKER") {
			t.Errorf("the fenced copy of the heading started the section:\n%s", got.HTML)
		}
	})
}

// TestEmbedHeadingSliceReadsAuthoredHTMLBlocks pins where an embedded section
// ends when the note hands the reader raw HTML. A '#' line inside an authored
// HTML block is text — markdown passes such a block through verbatim — so it
// does not end the section above it, and an excerpt that stopped there would
// drop the rest of a section the destination page shows in full, with nothing
// on either page saying so.
//
// The last two rows hold the guard to its other edge: a block ends where
// markdown ends it, and a line that merely carries a tag opens nothing. Were
// either wrong, the excerpt would run past the next heading instead — the same
// silent change of scope in the opposite direction.
func TestEmbedHeadingSliceReadsAuthoredHTMLBlocks(t *testing.T) {
	t.Parallel()

	const tail = "\n\n## Beta\n\nBETA-MARKER other body.\n"

	tests := []struct {
		name string
		body string
		want []string
		omit []string
	}{
		{
			name: "a div block keeps its heading-looking line",
			body: "## Alpha\n\nTOP-MARKER\n\n<div>\n# not a heading, inside an HTML block\nINNER-MARKER\n</div>\n\nTAIL-MARKER" + tail,
			want: []string{"TOP-MARKER", "INNER-MARKER", "TAIL-MARKER"},
			omit: []string{"BETA-MARKER"},
		},
		{
			name: "an html comment keeps its heading-looking line",
			body: "## Alpha\n\nTOP-MARKER\n\n<!--\n# not a heading, inside a comment\n-->\n\nTAIL-MARKER" + tail,
			want: []string{"TOP-MARKER", "TAIL-MARKER"},
			omit: []string{"BETA-MARKER"},
		},
		{
			name: "a pre block keeps its heading-looking line",
			body: "## Alpha\n\nTOP-MARKER\n\n<pre>\n# not a heading, inside pre\nINNER-MARKER\n</pre>\n\nTAIL-MARKER" + tail,
			want: []string{"TOP-MARKER", "INNER-MARKER", "TAIL-MARKER"},
			omit: []string{"BETA-MARKER"},
		},
		{
			name: "a closed block still lets the next heading end the section",
			body: "## Alpha\n\n<div>\nINNER-MARKER\n</div>\n\nTAIL-MARKER" + tail,
			want: []string{"INNER-MARKER", "TAIL-MARKER"},
			omit: []string{"BETA-MARKER"},
		},
		{
			name: "a line opening with ruby markup opens no block",
			body: "## Alpha\n\nTOP-MARKER\n\n<ruby>今日<rt>きょう</rt></ruby>\n## Beta\n\nBETA-MARKER other body.\n",
			want: []string{"TOP-MARKER", "今日"},
			omit: []string{"BETA-MARKER"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": tt.body})
			got := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
			for _, want := range tt.want {
				if !strings.Contains(got.HTML, want) {
					t.Errorf("the named section lost %q:\n%s", want, got.HTML)
				}
			}
			for _, omit := range tt.omit {
				if strings.Contains(got.HTML, omit) {
					t.Errorf("content outside the named section leaked in (%q):\n%s", omit, got.HTML)
				}
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("Diagnostics = %+v, want none for a section that exists", got.Diagnostics)
			}
		})
	}
}

// TestEmbedHeadingFragmentAcceptsWhatTheDestinationAnchors is the agreement
// oracle for the two surfaces that read a heading: the destination page's own
// table of contents, and the scan that answers "![[note#section]]". Every
// spelling the page lists is one a reader can copy into an embed, so each of
// them has to slice — a report that the section is not there, about a heading
// the page itself lists and links to, sends the reader looking for a fault in
// their own note.
func TestEmbedHeadingFragmentAcceptsWhatTheDestinationAnchors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "atx", body: "## Alpha\n\nA-BODY\n"},
		{name: "setext underlined with dashes", body: "Alpha\n-----\n\nA-BODY\n"},
		{name: "setext underlined with equals", body: "Alpha\n=====\n\nA-BODY\n"},
		{name: "display alias", body: "## See [[Other|display]]\n\nA-BODY\n"},
		{name: "bare wikilink", body: "## See [[Other]]\n\nA-BODY\n"},
		{name: "ruby reading", body: "## <ruby>今日<rt>きょう</rt></ruby>\n\nA-BODY\n"},
		{name: "emphasis", body: "## **Bold** heading\n\nA-BODY\n"},
		{name: "character reference", body: "## A &amp; B\n\nA-BODY\n"},
		{name: "authored html block inside the section", body: "## Alpha\n\n<div>\n# not a heading\n</div>\n\n## Beta\n\nB-BODY\n"},
		{name: "a fenced copy of a heading", body: "```\n## Fenced\n```\n\n## Real\n\nR-BODY\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}, {RelPath: "Other.md"}}, nil, transclusions{"B.md": tt.body})
			dest := r.HTML("B.md", "", tt.body, wording.ZhHant)
			if len(dest.TOC) == 0 {
				t.Fatalf("the destination page stamped no anchor at all for:\n%s", tt.body)
			}
			for _, entry := range dest.TOC {
				if entry.Text == "" {
					continue // no fragment can spell a heading with no text
				}
				got := r.HTML("note.md", "", "![["+"B#"+entry.Text+"]]\n", wording.ZhHant)
				for _, d := range got.Diagnostics {
					if d.Kind == render.DiagEmbedFragmentMissing {
						t.Errorf("the page anchors %q as %q, but the embed of that spelling reports: %s", entry.Text, entry.ID, d.Message)
					}
				}
			}
		})
	}

	// A heading underlined across two lines has no spelling a fragment can
	// carry — a wikilink never spans lines — so what matters about it is that
	// the section above it ends where the reader sees it end.
	t.Run("a heading underlined under two lines ends the section above it", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "## Alpha\n\nINSIDE-MARKER\n\nBeta\nGamma\n-----\n\nAFTER-MARKER\n",
		})
		got := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "INSIDE-MARKER") {
			t.Errorf("the named section is missing:\n%s", got.HTML)
		}
		if strings.Contains(got.HTML, "AFTER-MARKER") {
			t.Errorf("content past the underlined heading leaked in:\n%s", got.HTML)
		}
	})

	// The two underlines are different levels — '=' opens a level-1 section,
	// '-' a level-2 one — so an underlined top-level section owns the '##'
	// subsections written under it, exactly as a '#' one does.
	t.Run("an equals-underlined section owns its subsections", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "Alpha\n=====\n\nINSIDE-MARKER\n\n## Sub\n\nSUB-MARKER\n\n# Beta\n\nBETA-MARKER\n",
		})
		got := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
		for _, want := range []string{"INSIDE-MARKER", "SUB-MARKER"} {
			if !strings.Contains(got.HTML, want) {
				t.Errorf("the named section lost %q:\n%s", want, got.HTML)
			}
		}
		if strings.Contains(got.HTML, "BETA-MARKER") {
			t.Errorf("content outside the named section leaked in:\n%s", got.HTML)
		}
	})

	t.Run("a setext section ends at the next heading of its level", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "TOP-MARKER\n\nAlpha\n-----\n\nINSIDE-MARKER\n\n## Beta\n\nBETA-MARKER\n",
		})
		got := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "INSIDE-MARKER") {
			t.Errorf("the named section is missing:\n%s", got.HTML)
		}
		for _, forbidden := range []string{"TOP-MARKER", "BETA-MARKER"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("content outside the named section leaked in (%q):\n%s", forbidden, got.HTML)
			}
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none for a section that exists", got.Diagnostics)
		}
	})
}

// TestEmbedHeadingFragmentReportsWhatItCouldNotMatch holds the spelling the
// scan still cannot reproduce from a note's own source: a heading carrying a
// markdown link keeps the address the rendered heading drops. It degrades the
// way the fault-tolerance rule requires — the excerpt withheld, plus a report —
// and the report says what actually happened, which is that nothing matched,
// rather than asserting a section the reader can see on the destination page
// is absent.
//
// A heading whose link target is unwritten used to sit here too, because the
// sentence saying the note was unwritten was read into the anchor. It is a
// section like any other now, and its case is asserted where the explanation's
// placement is.
func TestEmbedHeadingFragmentReportsWhatItCouldNotMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		fragment string
		want     string
	}{
		{
			name:     "a markdown link keeps its address in the source",
			body:     "## See [docs](https://example.invalid/x)\n\nA-BODY\n",
			fragment: "See docs",
			want:     `no heading in "B.md" matched "See docs"; the excerpt is withheld`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": tt.body})
			dest := r.HTML("B.md", "", tt.body, wording.ZhHant)
			if len(dest.TOC) != 1 || dest.TOC[0].Text != tt.fragment {
				t.Fatalf("destination TOC = %+v, want one entry reading %q", dest.TOC, tt.fragment)
			}
			got := r.HTML("note.md", "", "![[B#"+tt.fragment+"]]\n", wording.ZhHant)
			if strings.Contains(got.HTML, "A-BODY") {
				t.Errorf("a name that matched nothing was widened into the body:\n%s", got.HTML)
			}
			var messages []string
			for _, d := range got.Diagnostics {
				if d.Kind == render.DiagEmbedFragmentMissing {
					messages = append(messages, d.Message)
				}
			}
			if len(messages) != 1 || messages[0] != tt.want {
				t.Errorf("fragment-missing diagnostics = %q, want exactly one reading %q", messages, tt.want)
			}
		})
	}
}

// TestEmbedBlockFragmentScopesTheTransclusion is the assertion site for
// "![[note#^block]]". Obsidian embeds only the paragraph carrying the
// "^block" marker; before the fragment was threaded through, the whole note
// appeared with nothing anywhere saying the scope had widened.
func TestEmbedBlockFragmentScopesTheTransclusion(t *testing.T) {
	t.Parallel()

	const body = "FIRST-MARKER first paragraph.\n\n" +
		"SECOND-MARKER the addressed paragraph. ^quux\n\n" +
		"THIRD-MARKER third paragraph.\n"

	t.Run("only the addressed paragraph embeds", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": body})
		got := r.HTML("note.md", "", "![[B#^quux]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "SECOND-MARKER") {
			t.Errorf("the addressed paragraph is missing:\n%s", got.HTML)
		}
		for _, forbidden := range []string{"FIRST-MARKER", "THIRD-MARKER"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("content outside the addressed paragraph leaked in (%q):\n%s", forbidden, got.HTML)
			}
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none for a block that exists", got.Diagnostics)
		}
	})

	// A marker sitting on a list item addresses that item. Nothing external
	// rules on it — the vault's own dialect notes say only that "^block" names
	// a block — so the narrower reading is the one taken: the reader asked for
	// the line the author marked, and handing them the whole list would be
	// this renderer widening a scope the author set.
	t.Run("a marker on a list item embeds that item alone", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "- ITEM-A first\n- ITEM-B addressed ^id\n- ITEM-C last\n",
		})
		got := r.HTML("note.md", "", "![[B#^id]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "ITEM-B") {
			t.Errorf("the addressed item is missing:\n%s", got.HTML)
		}
		for _, forbidden := range []string{"ITEM-A", "ITEM-C"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("a neighbouring item leaked in (%q):\n%s", forbidden, got.HTML)
			}
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none for a block that exists", got.Diagnostics)
		}
	})

	t.Run("a marker under a list item's own continuation embeds the item whole", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "- ITEM-A first\n- ITEM-B opens\n  ITEM-B continues ^id\n- ITEM-C last\n",
		})
		got := r.HTML("note.md", "", "![[B#^id]]\n", wording.ZhHant)
		for _, want := range []string{"ITEM-B opens", "ITEM-B continues"} {
			if !strings.Contains(got.HTML, want) {
				t.Errorf("the addressed item lost %q:\n%s", want, got.HTML)
			}
		}
		for _, forbidden := range []string{"ITEM-A", "ITEM-C"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("a neighbouring item leaked in (%q):\n%s", forbidden, got.HTML)
			}
		}
	})

	// A block address and a heading name are both written by hand on one side
	// and read back on the other, so they fold the same way: case and Unicode
	// form, and nothing else. An address is an identifier the author chose, so
	// the punctuation inside it is part of the name rather than a separator.
	t.Run("the block address folds case and unicode form", func(t *testing.T) {
		t.Parallel()
		const (
			composed   = "\u304C"       // が
			decomposed = "\u304B\u3099" // か + combining dakuten
		)
		tests := []struct {
			name     string
			marker   string
			fragment string
			want     bool
		}{
			{name: "case differs", marker: "^Quote1", fragment: "^quote1", want: true},
			{name: "unicode form differs", marker: "^" + decomposed, fragment: "^" + composed, want: true},
			{name: "punctuation differs", marker: "^quote-1", fragment: "^quote1", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
					"B.md": "OUTSIDE-MARKER first.\n\nADDRESSED-MARKER the paragraph. " + tt.marker + "\n",
				})
				got := r.HTML("note.md", "", "![[B#"+tt.fragment+"]]\n", wording.ZhHant)
				// A matched address brings exactly the addressed paragraph; an
				// unmatched one brings nothing. Neither brings the words outside it.
				if addressed := strings.Contains(got.HTML, "ADDRESSED-MARKER"); addressed != tt.want {
					t.Errorf("embed of %q against marker %q reached the addressed paragraph = %v, want %v:\n%s",
						tt.fragment, tt.marker, addressed, tt.want, got.HTML)
				}
				if strings.Contains(got.HTML, "OUTSIDE-MARKER") {
					t.Errorf("embed of %q against marker %q brought words from outside the addressed paragraph:\n%s",
						tt.fragment, tt.marker, got.HTML)
				}
				if tt.want && len(got.Diagnostics) != 0 {
					t.Errorf("Diagnostics = %+v, want none for a block that exists", got.Diagnostics)
				}
				if !tt.want && len(got.Diagnostics) != 1 {
					t.Errorf("Diagnostics = %+v, want the withheld excerpt reported once", got.Diagnostics)
				}
			})
		}
	})

	t.Run("a multi-line paragraph embeds whole", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
			"B.md": "OUTSIDE-MARKER\n\nLINE-ONE continues\nLINE-TWO ends here. ^blk\n\nAFTER-MARKER\n",
		})
		got := r.HTML("note.md", "", "![[B#^blk]]\n", wording.ZhHant)
		for _, want := range []string{"LINE-ONE", "LINE-TWO"} {
			if !strings.Contains(got.HTML, want) {
				t.Errorf("the addressed paragraph lost %q:\n%s", want, got.HTML)
			}
		}
		for _, forbidden := range []string{"OUTSIDE-MARKER", "AFTER-MARKER"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("content outside the addressed paragraph leaked in (%q):\n%s", forbidden, got.HTML)
			}
		}
	})

	t.Run("a missing block withholds the excerpt and says so", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": body})
		got := r.HTML("note.md", "", "![[B#^missing]]\n", wording.ZhHant)
		for _, forbidden := range []string{"FIRST-MARKER", "SECOND-MARKER", "THIRD-MARKER"} {
			if strings.Contains(got.HTML, forbidden) {
				t.Errorf("a block the note does not have was widened into %q:\n%s", forbidden, got.HTML)
			}
		}
		for _, want := range []string{`class="embed embed--withheld"`, "#^missing", `href="/notes/B.md"`} {
			if !strings.Contains(got.HTML, want) {
				t.Errorf("the notice standing in for the excerpt lacks %s:\n%s", want, got.HTML)
			}
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagEmbedFragmentMissing {
			t.Fatalf("Diagnostics = %+v, want one DiagEmbedFragmentMissing", got.Diagnostics)
		}
		if got, want := got.Diagnostics[0].Target, "B"; got != want {
			t.Errorf("Diagnostic.Target = %q, want the bare name %q", got, want)
		}
		if got, want := got.Diagnostics[0].Block, "missing"; got != want {
			t.Errorf("Diagnostic.Block = %q, want %q", got, want)
		}
		if got := got.Diagnostics[0].Section; got != "" {
			t.Errorf("Diagnostic.Section = %q, want empty: a block address is not a section name", got)
		}
	})
}

func TestEmbedUnresolvedAndAmbiguous(t *testing.T) {
	t.Parallel()

	t.Run("unresolved", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", "![[Ghost]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, `class="wikilink-broken"`) {
			t.Errorf("HTML().HTML missing wikilink-broken span:\n%s", got.HTML)
		}
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagWikilinkBroken {
			t.Errorf("Diagnostics = %+v, want one DiagWikilinkBroken", got.Diagnostics)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "a/Dup.md"}, {RelPath: "b/Dup.md"}}, nil, nil)
		got := r.HTML("note.md", "", "![[Dup]]\n", wording.ZhHant)
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
		{"example", "note", "Example"}, {"quote", "quote", "Quote"}, {"cite", "quote", "Quote"},
		{"warning", "warning", "Warning"}, {"caution", "warning", "Warning"}, {"attention", "warning", "Warning"},
		{"danger", "warning", "Danger"}, {"error", "warning", "Danger"}, {"bug", "warning", "Danger"},
		{"fail", "warning", "Danger"}, {"failure", "warning", "Danger"}, {"missing", "warning", "Danger"},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			t.Parallel()
			body := "> [!" + tt.typ + "]\n> body text\n"
			got := r.HTML("note.md", "", body, wording.ZhHant)
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
		got := r.HTML("note.md", "", "> [!note]-\n> hidden\n", wording.ZhHant)
		if !strings.Contains(got.HTML, `<details class="callout callout-note">`) {
			t.Errorf("HTML().HTML missing closed <details>:\n%s", got.HTML)
		}
	})

	t.Run("open by default (+)", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "> [!note]+\n> shown\n", wording.ZhHant)
		if !strings.Contains(got.HTML, `<details class="callout callout-note" open>`) {
			t.Errorf("HTML().HTML missing open <details>:\n%s", got.HTML)
		}
	})

	t.Run("static, no fold control", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "> [!note]\n> static\n", wording.ZhHant)
		if strings.Contains(got.HTML, "<details") {
			t.Errorf("a static (no-suffix) callout must not render <details>:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, `<div class="callout callout-note">`) {
			t.Errorf("HTML().HTML missing static callout div:\n%s", got.HTML)
		}
	})
}

// TestCalloutSerializationLocks pins each callout variant's full
// serialization byte-exact for one fixed body, the way the heading and table
// surfaces are already locked. The class-fragment assertions above prove the
// pieces exist; only an exact string proves there is exactly one body
// wrapper, that attributes keep their order, and that the icon precedes the
// title — the drifts a substring check waves through.
//
// The newline between the body wrapper and the first paragraph is the callout's
// opening markup standing on a line of its own in the source the note is parsed
// from. It is whitespace between two block elements and no reader sees it.
func TestCalloutSerializationLocks(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "static callout",
			body: "> [!note]\n> body text\n",
			want: `<div class="callout callout-note"><p class="callout-title">` +
				`<span class="callout-icon" aria-hidden="true">ℹ</span>Note</p>` +
				`<div class="callout-body">` + "\n" + `<p>body text</p>` + "\n</div></div>\n",
		},
		{
			name: "foldable callout closed by default",
			body: "> [!note]-\n> body text\n",
			want: `<details class="callout callout-note"><summary class="callout-title">` +
				`<span class="callout-icon" aria-hidden="true">ℹ</span>Note</summary>` +
				`<div class="callout-body">` + "\n" + `<p>body text</p>` + "\n</div></details>\n",
		},
		{
			name: "foldable callout open by default",
			body: "> [!note]+\n> body text\n",
			want: `<details class="callout callout-note" open><summary class="callout-title">` +
				`<span class="callout-icon" aria-hidden="true">ℹ</span>Note</summary>` +
				`<div class="callout-body">` + "\n" + `<p>body text</p>` + "\n</div></details>\n",
		},
		{
			name: "static warning callout with an authored title",
			body: "> [!warning] 自訂標題\n> body text\n",
			want: `<div class="callout callout-warning"><p class="callout-title">` +
				`<span class="callout-icon" aria-hidden="true">⚠</span>自訂標題</p>` +
				`<div class="callout-body">` + "\n" + `<p>body text</p>` + "\n</div></div>\n",
		},
		{
			// A quotation is not an aside about the text — it is someone's
			// words. It keeps its own class and a quotation mark for an icon,
			// the reading Obsidian gives the same type, so a spoken record
			// stops arriving dressed as an information note.
			name: "static quote callout with an authored title",
			body: "> [!quote] 口述紀錄\n> body text\n",
			want: `<div class="callout callout-quote"><p class="callout-title">` +
				`<span class="callout-icon" aria-hidden="true">❝</span>口述紀錄</p>` +
				`<div class="callout-body">` + "\n" + `<p>body text</p>` + "\n</div></div>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", tt.body, wording.ZhHant)
			if got.HTML != tt.want {
				t.Errorf("HTML(%q).HTML = %q, want exactly %q", tt.body, got.HTML, tt.want)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("Diagnostics = %+v, want none", got.Diagnostics)
			}
		})
	}
}

func TestCalloutUnknownTypeFallsBackToBlockquote(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "> [!banana] Weird\n> body\n", wording.ZhHant)
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
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, nil)

	got := r.HTML("note.md", "", "> [!note]\n> See [[Target]] here\n", wording.ZhHant)
	want := `<a href="/notes/Target.md" class="wikilink">Target</a>`
	if !strings.Contains(got.HTML, want) {
		t.Errorf("a callout's body must be rendered through the same pipeline (nested wikilinks); HTML missing %q:\n%s", want, got.HTML)
	}
}

func TestHighlightRendersMark(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "plain ==highlighted== text\n", wording.ZhHant)
	if !strings.Contains(got.HTML, "<mark>highlighted</mark>") {
		t.Errorf("HTML().HTML missing <mark>highlighted</mark>:\n%s", got.HTML)
	}
}

func TestHighlightIgnoresCodeSpan(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "literal `==not==` marker\n", wording.ZhHant)
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
	got := r.HTML("note.md", "", body, wording.ZhHant)

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
	// the section id falls back to the literal string "section". (A trailing
	// run of "#" would be parsed as ATX's optional closing sequence and
	// stripped from the text by goldmark itself, so this uses "!" only.)
	got := r.HTML("note.md", "", "## !!! !!!\n", wording.ZhHant)
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
	got := r.HTML("note.md", "", "## <ruby>漢字<rt>かんじ</rt></ruby>\n\n## <ruby>音<rt lang=\"ja\">おと</rt></ruby>\n\n## <ruby>加藤<rt>かとう</rt></ruby>\n", wording.ZhHant)

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
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Real.md"}}, nil, nil)

	body := "before\n\n```text\n[[Fake Link]]\n> [!note] also risky\n```\n\nafter [[Real]]\n"
	got := r.HTML("note.md", "", body, wording.ZhHant)

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
		got := r.HTML("note.md", "Title", "\n\n# Title\n\nbody text\n", wording.ZhHant)
		if strings.Contains(got.HTML, "Title") {
			t.Errorf("the leading H1 must be removed entirely, not shown as a paragraph; HTML:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "body text") {
			t.Errorf("HTML().HTML missing the rest of the body:\n%s", got.HTML)
		}
	})

	t.Run("a later H1 is untouched when it is not first", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "Title", "# Title\n\nbody\n\n## Second\n\n# NotFirst\n", wording.ZhHant)
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
		got := r.HTML("notes.md", "notes", "# Quorum reads and writes\n\nbody text\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "Quorum reads and writes") {
			t.Errorf("the document's own heading was destroyed and the filename put in its place; HTML:\n%s", got.HTML)
		}
	})

	// A heading that merely differs is also not the title being repeated.
	t.Run("a leading H1 that is not the title survives", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "Frontmatter Title", "# A different opening\n\nbody\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "A different opening") {
			t.Errorf("a heading that does not repeat the title must be kept; HTML:\n%s", got.HTML)
		}
	})

	t.Run("no removal when the first content is not an H1", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "Not a heading\n\n# Title\n", wording.ZhHant)
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
	got := r.HTML("note.md", "", "```mermaid\n"+src+"\n```\n", wording.ZhHant)

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
	got := r.HTML("Concepts/golang/Slice.md", "", "![diagram](../../Assets/slice.png)\n", wording.ZhHant).HTML

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

	r := newRenderer(t, []graph.NoteInput{{RelPath: "Concepts/golang/Slice.md"}}, nil, transclusions{
		"Concepts/golang/Slice.md": "![diagram](./slice.png)\n",
	})
	got := r.HTML("Journal/2026-07-26.md", "", "![[Slice]]\n", wording.ZhHant).HTML

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
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Concepts/Real.md"}}, nil, nil)

	got := r.HTML("Maps/MOC.md", "MOC", "用 `[[概念]]` 互連,以及 `![[嵌入]]` 禁止,而 [[Real]] 是真的。\n", wording.ZhHant)

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

// TestQuotedWikilinkSurvivesAStrayBacktickRun covers the same sentence one
// typo further on. A backtick run that meets no run of its own length is
// ordinary text, and goldmark pairs the runs after it as usual — so the span
// following a stray run really is drawn as code. The line scan read it the
// other way and reported no span at all from the stray run onward, which
// converted the syntax the author was quoting: the reader was handed a
// rendered link inside the code they were being shown, the words of the
// example vanished, and the note collected a broken-link report for a target
// it never linked to.
func TestQuotedWikilinkSurvivesAStrayBacktickRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{
			name: "one stray run before the span",
			body: "``例 `[[概念]]` 之後，[[Real]] 仍是真的。\n",
		},
		{
			name: "several stray runs before the span",
			body: "``例 ```例 `[[概念]]` 之後，[[Real]] 仍是真的。\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Concepts/Real.md"}}, nil, nil)

			got := r.HTML("Maps/MOC.md", "MOC", tt.body, wording.ZhHant)

			if !strings.Contains(got.HTML, "<code>[[概念]]</code>") {
				t.Errorf("the quoted syntax after a stray backtick run was rewritten:\n%s", got.HTML)
			}
			if strings.Contains(got.HTML, "wikilink-broken") {
				t.Errorf("a quoted target was rendered as an unwritten link:\n%s", got.HTML)
			}
			if !strings.Contains(got.HTML, `class="wikilink"`) {
				t.Errorf("the real wikilink outside the code span stopped resolving:\n%s", got.HTML)
			}
			for _, d := range got.Diagnostics {
				if strings.Contains(d.Target, "概念") {
					t.Errorf("a quoted target was reported as a broken link: %+v", d)
				}
			}
		})
	}
}

// TestEscapedWikilinkStaysLiteral covers CommonMark's backslash escape at the
// wikilink seam. "\[[X]]" is the author showing the syntax, not writing a
// link; converting it anyway printed a stray backslash and — when X named
// nothing — reported a broken link the author never made, telling them the
// opposite of what they did.
func TestEscapedWikilinkStaysLiteral(t *testing.T) {
	t.Parallel()

	t.Run("an escaped wikilink renders literally with no diagnostic", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", `\[[Nonexistent]]`+"\n", wording.ZhHant)
		if strings.Contains(got.HTML, "wikilink") {
			t.Errorf("an escaped wikilink must not become any wikilink markup:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "[[Nonexistent]]") {
			t.Errorf("HTML().HTML missing the literal text %q:\n%s", "[[Nonexistent]]", got.HTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none — the author escaped the link on purpose", got.Diagnostics)
		}
	})

	t.Run("an escaped embed renders literally too", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", `\![[Ghost]]`+"\n", wording.ZhHant)
		if strings.Contains(got.HTML, "wikilink") || strings.Contains(got.HTML, `class="embed`) {
			t.Errorf("an escaped embed must not become link or embed markup:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "![[Ghost]]") {
			t.Errorf("HTML().HTML missing the literal text %q:\n%s", "![[Ghost]]", got.HTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none", got.Diagnostics)
		}
	})

	t.Run("an escaped backslash before a wikilink still converts", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, nil)
		got := r.HTML("note.md", "", `\\[[Target]]`+"\n", wording.ZhHant)
		// "\\" is one literal backslash; the link after it is a real link.
		want := `\<a href="/notes/Target.md" class="wikilink">Target</a>`
		if !strings.Contains(got.HTML, want) {
			t.Errorf("HTML().HTML missing %q:\n%s", want, got.HTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none for a resolvable wikilink", got.Diagnostics)
		}
	})

	t.Run("a doubly escaped backslash run escapes the link again", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", `\\\[[Ghost]]`+"\n", wording.ZhHant)
		if strings.Contains(got.HTML, "wikilink") {
			t.Errorf("an odd backslash run must keep the link escaped:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, `[[Ghost]]`) {
			t.Errorf("HTML().HTML missing the literal text %q:\n%s", "[[Ghost]]", got.HTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none", got.Diagnostics)
		}
	})

	t.Run("an escape on the line after a fence still holds", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, nil)
		got := r.HTML("note.md", "", "```\ncode\n```\n\n"+`\[[Ghost]]`+"\n", wording.ZhHant)
		if strings.Contains(got.HTML, "wikilink") {
			t.Errorf("an escaped wikilink after a fence must stay literal:\n%s", got.HTML)
		}
		if !strings.Contains(got.HTML, "[[Ghost]]") {
			t.Errorf("HTML().HTML missing the literal text %q:\n%s", "[[Ghost]]", got.HTML)
		}
		if len(got.Diagnostics) != 0 {
			t.Errorf("Diagnostics = %+v, want none", got.Diagnostics)
		}
	})
}

// TestUnclosedBacktickRunIsOrdinaryText keeps the span scanner from swallowing
// the rest of a document: a run with no closer of the same width is not a code
// span, and a wikilink after it still resolves.
func TestUnclosedBacktickRunIsOrdinaryText(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Concepts/Real.md"}}, nil, nil)

	got := r.HTML("note.md", "note", "一個沒有結尾的 ` 反引號,然後 [[Real]]。\n", wording.ZhHant)
	if !strings.Contains(got.HTML, `class="wikilink"`) {
		t.Errorf("an unclosed backtick swallowed the rest of the document:\n%s", got.HTML)
	}
}

// noTitles answers that nothing declares any title, which is what these tests
// are about: they exercise resolution and rendering, not the sentence a page
// says when a name turns out to be some note's title.
type noTitles struct{}

func (noTitles) TitledBy(string) []string { return nil }
