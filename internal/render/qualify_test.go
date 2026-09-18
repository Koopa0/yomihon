package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestQualifyRenamesEveryPlaceAndEveryReferenceToOne walks the shapes a
// rendered note can carry a place in, one row each. The expected side is
// written out as literal bytes rather than assembled from the prefix, so a pass
// that renamed the attribute instead of its value, or that stopped at the first
// name of a list, cannot agree with it.
func TestQualifyRenamesEveryPlaceAndEveryReferenceToOne(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		html string
		want string
	}{{
		name: "a heading keeps its words and takes a new name",
		html: `<h2 id="ledger" data-level="1">Ledger</h2>`,
		want: `<h2 id="b-ledger" data-level="1">Ledger</h2>`,
	}, {
		name: "an address the author wrote at their own section follows it",
		html: `<p><a href="#second-movement">the second movement</a></p>`,
		want: `<p><a href="#b-second-movement">the second movement</a></p>`,
	}, {
		name: "a footnote marker and the note it returns to move together",
		html: `<sup id="fnref:1"><a href="#fn:1">1</a></sup><li id="fn:1"><a href="#fnref:1">back</a></li>`,
		want: `<sup id="b-fnref:1"><a href="#b-fn:1">1</a></sup><li id="b-fn:1"><a href="#b-fnref:1">back</a></li>`,
	}, {
		name: "a block address keeps the escaping its link was written with",
		html: `<a href="#%5Eintro">to the block</a><span id="^intro">^intro</span>`,
		want: `<a href="#b-%5Eintro">to the block</a><span id="b-^intro">^intro</span>`,
	}, {
		name: "an element described by another is described by the same one",
		html: `<article aria-labelledby="slot-pattern-1"><h3 id="slot-pattern-1">frame</h3></article>`,
		want: `<article aria-labelledby="b-slot-pattern-1"><h3 id="b-slot-pattern-1">frame</h3></article>`,
	}, {
		name: "every name in a list of them is renamed",
		html: `<p aria-describedby="one two  three">words</p>`,
		want: `<p aria-describedby="b-one b-two b-three">words</p>`,
	}, {
		name: "the attribute whose name opens another one is still itself",
		html: `<button form="sheet" for="field">press</button>`,
		want: `<button form="b-sheet" for="b-field">press</button>`,
	}, {
		name: "an empty address still means the top of the document",
		html: `<a href="#">top</a>`,
		want: `<a href="#">top</a>`,
	}, {
		name: "an address into another note is not a place in this one",
		html: `<a href="/notes/Notes/cutover.md#ledger" class="wikilink">Cutover</a>`,
		want: `<a href="/notes/Notes/cutover.md#ledger" class="wikilink">Cutover</a>`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := render.Result{HTML: tt.html}
			render.Qualify("b-", &got)
			if diff := cmp.Diff(tt.want, got.HTML); diff != "" {
				t.Errorf("Qualify HTML mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestQualifyRenamesTheContentsListWithTheHeadings keeps the list and the
// headings it points at under one name, and leaves the result the caller passed
// in untouched: the entries are the caller's slice until this returns a new one.
func TestQualifyRenamesTheContentsListWithTheHeadings(t *testing.T) {
	t.Parallel()
	entries := []render.TOCEntry{{Level: 1, Text: "Ledger", ID: "ledger"}}
	got := render.Result{
		HTML:        `<h2 id="ledger" data-level="1">Ledger</h2>`,
		TOC:         entries,
		TitleAnchor: "cutover",
	}
	render.Qualify("a-", &got)

	wantTOC := []render.TOCEntry{{Level: 1, Text: "Ledger", ID: "a-ledger"}}
	if diff := cmp.Diff(wantTOC, got.TOC); diff != "" {
		t.Errorf("Qualify contents mismatch (-want +got):\n%s", diff)
	}
	if got.TitleAnchor != "a-cutover" {
		t.Errorf("Qualify TitleAnchor = %q, want %q", got.TitleAnchor, "a-cutover")
	}
	// The entries the result arrived with, which anything else holding that
	// slice still sees.
	if entries[0].ID != "ledger" {
		t.Errorf("Qualify renamed through the slice it was handed: got %q, want %q", entries[0].ID, "ledger")
	}
}

// TestQualifyUnderNoPrefixIsTheNoteShownAlone locks the one page that holds a
// single note: the same call is made there, and nothing may move.
func TestQualifyUnderNoPrefixIsTheNoteShownAlone(t *testing.T) {
	t.Parallel()
	before := render.Result{
		HTML:        `<h2 id="ledger" data-level="1">Ledger</h2><a href="#ledger">here</a>`,
		TOC:         []render.TOCEntry{{Level: 1, Text: "Ledger", ID: "ledger"}},
		TitleAnchor: "cutover",
	}
	after := before
	render.Qualify("", &after)
	if diff := cmp.Diff(before, after); diff != "" {
		t.Errorf("Qualify under no prefix moved something (-before +after):\n%s", diff)
	}
}

// TestQualifyLeavesMarkupThatIsOnlyBeingShown is the assumption the whole pass
// rests on: it reads attribute values the renderer wrote, and a note printing
// markup inside a code fence has no attribute values at all, because the quotes
// arrived escaped. If that ever stops being true this renaming reaches into a
// reader's own words, so the fence is rendered by the real pipeline rather than
// spelled out here.
func TestQualifyLeavesMarkupThatIsOnlyBeingShown(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	body := "```\n<a id=\"shown\" href=\"#shown\" aria-labelledby=\"shown\">printed</a>\n```\n"
	res := r.HTML("Notes/fence.md", "", body, wording.ZhHant)
	if !strings.Contains(res.HTML, "shown") {
		t.Fatalf("the fence lost its words, so this test asks nothing: %s", res.HTML)
	}
	renamed := res
	render.Qualify("b-", &renamed)
	if diff := cmp.Diff(res.HTML, renamed.HTML); diff != "" {
		t.Errorf("Qualify rewrote markup a note was only printing (-shown +renamed):\n%s", diff)
	}
}
