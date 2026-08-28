package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// TestDegradedLinkSaysSoBeforeItIsFollowed covers the half of a fragment miss
// the panel beside the article cannot reach: a reader deciding whether to
// follow the link. The panel states the same fact, and a reader who finds it
// there has already arrived somewhere they did not mean to be.
func TestDegradedLinkSaysSoBeforeItIsFollowed(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		renderer   func(*testing.T) *render.Pipeline
		body       string
		wantHref   string
		wantReason string
	}{
		{
			name:       "a section the destination does not answer to",
			renderer:   sectionRenderer,
			body:       "[[B#Gamma]]\n",
			wantHref:   `href="/notes/B.md#gamma"`,
			wantReason: "找不到「Gamma」這個小節，連結會落在筆記最上方",
		},
		{
			name:       "a block the destination does not carry",
			renderer:   blockRenderer,
			body:       "[[B#^nope]]\n",
			wantHref:   `href="/notes/B.md"`,
			wantReason: "找不到這個區塊，連結已改為指向整篇筆記",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.renderer(t).HTML("note.md", "", tt.body)
			if !strings.Contains(got.HTML, "wikilink-degraded") {
				t.Fatalf("the link is not marked as degraded:\n%s", got.HTML)
			}
			if !strings.Contains(got.HTML, tt.wantHref) {
				t.Errorf("the address changed under the marking; want %s in:\n%s", tt.wantHref, got.HTML)
			}
			// Both carriers, because either one alone reaches only half the
			// readers: the title is for whoever can point at the link, the
			// offscreen sentence for whoever is listening to the page.
			if !strings.Contains(got.HTML, `title="`+tt.wantReason+`"`) {
				t.Errorf("the link carries no pointable reason; want %q in:\n%s", tt.wantReason, got.HTML)
			}
			if !strings.Contains(got.HTML, `<span class="y-offscreen">（`+tt.wantReason+`）</span>`) {
				t.Errorf("the link carries no spoken reason; want %q in:\n%s", tt.wantReason, got.HTML)
			}
			// A degraded link is still a link: the note it names is real and the
			// address leads there, which is the whole reason this tier is not
			// the broken one.
			if strings.Contains(got.HTML, "wikilink-broken") {
				t.Errorf("a fragment miss was marked as a name with no target:\n%s", got.HTML)
			}
		})
	}
}

// TestPlacedFragmentsCarryNoMarking is the control the marking is worthless
// without: a renderer that marked every fragment would pass every assertion
// above and would tell a reader nothing.
func TestPlacedFragmentsCarryNoMarking(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		renderer func(*testing.T) *render.Pipeline
		body     string
	}{
		{name: "a section the destination declares", renderer: sectionRenderer, body: "[[B#Alpha]]\n"},
		{name: "a whole note with no fragment at all", renderer: sectionRenderer, body: "[[B]]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.renderer(t).HTML("note.md", "", tt.body)
			if strings.Contains(got.HTML, "wikilink-degraded") {
				t.Errorf("a link that placed its address is marked as degraded:\n%s", got.HTML)
			}
		})
	}
}

// TestUnresolvedNameKeepsItsOwnTier holds the boundary between the two faults.
// A name with no note behind it is a larger thing than a note whose part could
// not be found, and the page has always said so differently.
func TestUnresolvedNameKeepsItsOwnTier(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": sectionDest})
	got := r.HTML("note.md", "", "[[Nowhere#Gamma]]\n")
	if !strings.Contains(got.HTML, "wikilink-broken") {
		t.Fatalf("a name with no note behind it lost its own tier:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, "wikilink-degraded") {
		t.Errorf("a name with no note behind it was marked as a fragment miss:\n%s", got.HTML)
	}
}
