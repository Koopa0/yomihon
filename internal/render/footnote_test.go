package render_test

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// hrefValue reads every href attribute the renderer emitted, so a test can
// assert about the whole set rather than about the one shape it happened to
// think of. The renderer writes its own anchors, so this reads what this
// package wrote rather than parsing arbitrary HTML.
var hrefValue = regexp.MustCompile(`href="([^"]*)"`)

func hrefs(htmlOut string) []string {
	var out []string
	for _, m := range hrefValue.FindAllStringSubmatch(htmlOut, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestFootnoteSemanticContract is the footnote assertion site. Standard
// Markdown footnote syntax has one meaning — a reference that reaches its
// definition and a definition that reaches every reference back — and this
// pins all four halves of it: the reference is an anchor into the same page,
// the definition carries the note's text, each of two references to one
// definition gets its own return path, and nothing in the passage becomes a
// link out of the page.
//
// The last clause is the defect this exists to stop: read as ordinary
// reference-link syntax, "[^scope]" resolved against the definition's prose
// and produced an anchor to a page that does not exist, while the note text
// itself disappeared.
func TestFootnoteSemanticContract(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	body := "研究結論受範圍限制[^scope]，第二次引用[^scope]。\n\n[^scope]: 僅涵蓋本次研究範圍。\n"
	got := r.HTML("Notes/研究.md", "", body, wording.ZhHant)

	// Every anchor on the page is a same-page jump. A footnote never
	// navigates, so an href that is not a fragment is the fabricated link.
	for _, href := range hrefs(got.HTML) {
		if !strings.HasPrefix(href, "#") {
			t.Errorf("footnote passage produced the off-page href %q; every footnote anchor must be a same-page fragment:\n%s", href, got.HTML)
		}
	}

	// Both references reach the one definition, and each carries a distinct
	// id so the definition has something distinct to return to.
	for _, want := range []string{
		`<sup id="fnref:1"><a href="#fn:1"`,
		`<sup id="fnref1:1"><a href="#fn:1"`,
		`<li id="fn:1">`,
		`href="#fnref:1"`,
		`href="#fnref1:1"`,
		"僅涵蓋本次研究範圍。",
	} {
		if !strings.Contains(got.HTML, want) {
			t.Errorf("footnote rendering missing %q:\n%s", want, got.HTML)
		}
	}

	// Two references means two return paths, not one shared arrow: a reader
	// who followed the second reference has to land back at the second.
	if n := strings.Count(got.HTML, `class="footnote-backref"`); n != 2 {
		t.Errorf("backref count = %d, want 2 (one per reference):\n%s", n, got.HTML)
	}

	if len(got.Diagnostics) != 0 {
		t.Errorf("Diagnostics = %+v, want none for a well-formed footnote", got.Diagnostics)
	}
}

var (
	elementID    = regexp.MustCompile(`id="([^"]*)"`)
	fragmentHref = regexp.MustCompile(`href="#([^"]*)"`)
	footnoteRef  = regexp.MustCompile(`<sup id="([^"]+)"><a href="#([^"]+)"`)
)

func allMatches(re *regexp.Regexp, htmlOut string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(htmlOut, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestComposedFootnoteIDsAreUnique is the assertion site for a page assembled
// out of more than one body. A note's own text, each callout in it, and each
// note transcluded into it are rendered separately and then spliced together,
// and every one of those passes numbers its footnotes from one. Composed, the
// page carries the same id several times: the browser resolves a fragment to
// the first element bearing it, so a reader following the second note's
// citation is handed the first note's — silently, with nothing on screen
// saying the wrong text was reached.
//
// The same note is embedded twice on purpose. Two identical bodies cannot be
// told apart by their content, so anything identifying a region has to include
// which occurrence it is.
func TestComposedFootnoteIDsAreUnique(t *testing.T) {
	t.Parallel()
	// The embedded note carries a callout of its own, so the composition goes
	// two levels deep: a body inside a body inside the page. Testing only the
	// flat cases would leave the one place a region has to be claimed by
	// something other than the page's own top level.
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Embedded.md"}}, nil, transclusions{
		"Embedded.md": strings.Join([]string{
			"Embedded text[^e].",
			"",
			"> [!note] The embedded note's own aside",
			"> Inner text[^i].",
			">",
			"> [^i]: The inner aside's note.",
			"",
			"[^e]: The embedded note's own note.",
		}, "\n"),
	})

	body := strings.Join([]string{
		"Host text[^h], cited again[^h].",
		"",
		"> [!note] Aside",
		"> Callout text[^c].",
		">",
		"> [^c]: The callout's own note.",
		"",
		"![[Embedded]]",
		"",
		"![[Embedded]]",
		"",
		"[^h]: The host's own note.",
	}, "\n")
	got := r.HTML("Notes/host.md", "", body, wording.ZhHant).HTML

	// The page has to be the composed one this test claims to describe: six
	// definitions from six separately rendered regions, seven citations of
	// them. Without this the assertions below could hold over a page that
	// never assembled anything.
	if n := strings.Count(got, `class="footnote-ref"`); n != 7 {
		t.Fatalf("page carries %d footnote references, want 7 — the composed fixture did not render:\n%s", n, got)
	}
	if n := strings.Count(got, `class="footnotes"`); n != 6 {
		t.Fatalf("page carries %d footnote sections, want 6 (host, its callout, two embeds, and each embed's own callout):\n%s", n, got)
	}

	ids := allMatches(elementID, got)
	seen := map[string]int{}
	for _, id := range ids {
		seen[id]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("id %q appears %d times on one page; a fragment naming it reaches whichever came first:\n%s", id, n, got)
		}
	}

	// Every address the page offers has to name exactly one place on it.
	for _, fragment := range allMatches(fragmentHref, got) {
		if n := seen[fragment]; n != 1 {
			t.Errorf("href %q has %d destinations on this page, want exactly 1", "#"+fragment, n)
		}
	}

	// And each citation's way back has to lead to that citation. A reference
	// whose id is shared would send every reader of it to the first one.
	for _, pair := range footnoteRef.FindAllStringSubmatch(got, -1) {
		reference, definition := pair[1], pair[2]
		if !strings.Contains(got, `<li id="`+definition+`">`) {
			t.Errorf("the reference %q cites %q, which is on no list on this page", reference, "#"+definition)
		}
		if !strings.Contains(got, `href="#`+reference+`"`) {
			t.Errorf("nothing on the page returns to the reference %q, so its footnote is a one-way trip", reference)
		}
	}
}

// TestSeparatelyRenderedBodiesKeepDistinctFootnoteIDs covers the bodies a page
// assembles that this package never sees together. A lesson's concept sheets
// are rendered by their own calls and shipped inside <template> elements; the
// moment the reader opens one it is cloned into the lesson's own document, so
// two renders that never met are suddenly in one id space. Each therefore has
// to be told where it sits, and the regions it opens inside itself — for a
// callout, for a transclusion — have to be named under that too.
//
// Footnote ids are what this asserts, because they are what the region
// qualifies. Heading ids are unique within a body and are not qualified, so
// two bodies composed onto one page can still stamp the same heading id; that
// is a separate gap and is not what this test speaks for.
func TestSeparatelyRenderedBodiesKeepDistinctFootnoteIDs(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, transclusions{})

	// One body per render call, each with a footnote of its own and a callout
	// carrying another, which is where a region that did not compose would
	// collide: the sheet's first callout and the lesson's first callout.
	body := strings.Join([]string{
		"Body text[^b].",
		"",
		"> [!note] Aside",
		"> Aside text[^a].",
		">",
		"> [^a]: The aside's note.",
		"",
		"[^b]: The body's own note.",
	}, "\n")

	page := r.HTML("Writing/lesson.md", "", body, wording.ZhHant)
	sheets := []string{
		r.HTMLIn("c1-", "Concepts/one.md", "", body, wording.ZhHant).HTML,
		r.HTMLIn("c2-", "Concepts/two.md", "", body, wording.ZhHant).HTML,
	}

	seen := map[string]string{}
	for where, htmlOut := range map[string]string{
		"the lesson body":  page.HTML,
		"the first sheet":  sheets[0],
		"the second sheet": sheets[1],
	} {
		for _, id := range allMatches(elementID, htmlOut) {
			// A footnote id is the one this region names; anything else on
			// these bodies is outside what the region qualifies, so counting
			// it here would assert a guarantee that was not made.
			if !strings.Contains(id, "fn:") && !strings.Contains(id, "fnref") {
				continue
			}
			if owner, taken := seen[id]; taken {
				t.Errorf("the footnote id %q is minted by both %s and %s; once the sheet is cloned into the page both are in one document", id, owner, where)
			}
			seen[id] = where
		}
	}

	// The composition has to be real, or the loop above compared nothing.
	for _, want := range []string{"fn:1", "y1-fn:1", "c1-fn:1", "c1-y1-fn:1", "c2-y1-fn:1"} {
		if !slices.Contains(slices.Collect(maps.Keys(seen)), want) {
			t.Errorf("no body minted the id %q, so this page did not assemble the regions the test describes: %v", want, slices.Sorted(maps.Keys(seen)))
		}
	}
}

// TestComposedFootnoteIDsAreDeterministic is what keeps the previous test's
// answer from depending on when it was asked. Region identity has to come from
// the page being assembled and nothing else: a counter living beside the
// pipeline would number the same note differently on its second reading, so
// two readers of one note would receive different ids, a cached page would
// stop matching the page beside it, and the failure would appear only under
// load — the condition least likely to be reproduced.
func TestComposedFootnoteIDsAreDeterministic(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Embedded.md"}}, nil, transclusions{
		"Embedded.md": "Embedded text[^e].\n\n[^e]: The embedded note's own note.\n",
	})
	body := "Host[^h].\n\n![[Embedded]]\n\n![[Embedded]]\n\n[^h]: Host note.\n"

	want := r.HTML("Notes/host.md", "", body, wording.ZhHant).HTML
	if !strings.Contains(want, "y1-fn:1") || !strings.Contains(want, "y2-fn:1") {
		t.Fatalf("the fixture did not produce two distinct regions, so repeating it proves nothing:\n%s", want)
	}

	var wg sync.WaitGroup
	got := make([]string, 32)
	for i := range got {
		wg.Go(func() { got[i] = r.HTML("Notes/host.md", "", body, wording.ZhHant).HTML })
	}
	wg.Wait()
	for i, out := range got {
		if out != want {
			t.Fatalf("render %d of the same note differs from the first:\n%s", i, cmp.Diff(want, out))
		}
	}
}

// TestFootnoteTextIsSearchable pins the two passes over one body to the same
// reading. A note that displays a sentence and cannot be found by it is a
// worse failure than one that displays nothing, because the reader has seen
// the words and has no reason to doubt the search.
//
// The definition here is written in Chinese on purpose. Its text has no spaces
// in it, which makes the line a valid ordinary link reference definition, and
// a pass that does not know about footnotes therefore swallows it whole — the
// same reading that produced the fabricated link this file's first test
// forbids. An English definition happens to survive that misreading, so a
// test written in English would have passed either way.
func TestFootnoteTextIsSearchable(t *testing.T) {
	t.Parallel()

	const definition = "僅涵蓋本次研究範圍。"
	body := "研究結論受範圍限制[^scope]，第二次引用[^scope]。\n\n[^scope]: " + definition + "\n"

	if got := render.PlainText(body); !strings.Contains(got, definition) {
		t.Errorf("PlainText() = %q, want it to carry the footnote's own words %q — the reading page shows them", got, definition)
	}

	var sectionText strings.Builder
	for _, section := range render.PlainSections(body) {
		sectionText.WriteString(section.Text)
	}
	if got := sectionText.String(); !strings.Contains(got, definition) {
		t.Errorf("PlainSections() text = %q, want it to carry the footnote's own words %q", got, definition)
	}
}

// TestFootnoteWithoutDefinitionStaysLiteral is the honesty half: a reference
// nobody defined has nowhere to go, so it stays the characters the author
// typed. Inventing a destination for it is the same fabrication the contract
// above forbids, and an unwritten footnote must not become a dead link.
func TestFootnoteWithoutDefinitionStaysLiteral(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("Notes/研究.md", "", "未定義的註腳[^missing]。\n", wording.ZhHant)

	if !strings.Contains(got.HTML, "[^missing]") {
		t.Errorf("an undefined footnote reference must stay readable as written:\n%s", got.HTML)
	}
	if hs := hrefs(got.HTML); len(hs) != 0 {
		t.Errorf("an undefined footnote reference produced hrefs %q, want none:\n%s", hs, got.HTML)
	}
}
