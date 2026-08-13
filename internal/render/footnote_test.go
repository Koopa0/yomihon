package render_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
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
	got := r.HTML("Notes/研究.md", "", body)

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

	got := r.HTML("Notes/研究.md", "", "未定義的註腳[^missing]。\n")

	if !strings.Contains(got.HTML, "[^missing]") {
		t.Errorf("an undefined footnote reference must stay readable as written:\n%s", got.HTML)
	}
	if hs := hrefs(got.HTML); len(hs) != 0 {
		t.Errorf("an undefined footnote reference produced hrefs %q, want none:\n%s", hs, got.HTML)
	}
}
