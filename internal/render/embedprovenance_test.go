package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAnExcerptNamesTheNoteItCameFrom pins the provenance line: an excerpt
// opens by saying whose words follow, and the name is a way there. Without it
// the words simply continue in the host's voice, and the one thing an excerpt
// cannot say for itself is that they are someone else's.
func TestAnExcerptNamesTheNoteItCameFrom(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Notes/引用來源.md"}}, nil, transclusions{
		"Notes/引用來源.md": "excerpt body\n",
	})

	got := r.HTML("note.md", "", "![[引用來源]]\n", wording.ZhHant)
	if !strings.Contains(got.HTML, `<div class="embed"><p class="embed__source">`) {
		t.Errorf("the excerpt does not open with its provenance line:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `<a href="/notes/Notes/%E5%BC%95%E7%94%A8%E4%BE%86%E6%BA%90.md">引用來源</a>`) {
		t.Errorf("the provenance line does not link the source note by name:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "出自") {
		t.Errorf("the zh-Hant provenance wording is missing:\n%s", got.HTML)
	}

	english := r.HTML("note.md", "", "![[引用來源]]\n", wording.En)
	if !strings.Contains(english.HTML, "From") {
		t.Errorf("the English provenance wording is missing:\n%s", english.HTML)
	}
}

// TestTheProvenanceLineSitsWithTheWithheldNotice puts the two on one block: a
// fragment that matched nothing withholds the excerpt and says so, and the
// provenance line stands first — both are answers to what this block is, and
// whose note it is comes before what could not be found in it. The provenance
// line is also the way on to that note, which a block showing none of it owes
// the reader.
func TestTheProvenanceLineSitsWithTheWithheldNotice(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": "## Real\n\nbody\n",
	})

	got := r.HTML("note.md", "", "![[B#nope]]\n", wording.ZhHant)
	source := strings.Index(got.HTML, `class="embed__source"`)
	withheld := strings.Index(got.HTML, `class="embed__note"`)
	if source < 0 || withheld < 0 {
		t.Fatalf("provenance line at %d and withheld notice at %d; both must render:\n%s", source, withheld, got.HTML)
	}
	if source > withheld {
		t.Errorf("the provenance line follows the withheld notice; it opens the block:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `<div class="embed embed--withheld"><p class="embed__source">`) {
		t.Errorf("a withheld excerpt no longer opens with its provenance line:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, "body") {
		t.Errorf("a withheld excerpt still carries the note's words:\n%s", got.HTML)
	}
}

// TestTheContentsListOnlyTheHostsOwnHeadings is the outline's ownership rule,
// the reading Obsidian's outline takes too: a transcluded excerpt's headings
// are stamped on this page — a link can land on them — but they are another
// note's structure, and the list titled with this page's contents does not
// claim them.
func TestTheContentsListOnlyTheHostsOwnHeadings(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": "## Inner Section\n\nbody\n\n## Another\n\ntail\n",
	})

	t.Run("a host with no headings of its own lists none", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "host prose\n\n![[B]]\n", wording.ZhHant)
		if len(got.TOC) != 0 {
			t.Errorf("TOC = %+v, want empty — every heading on this page is the excerpt's", got.TOC)
		}
		if !strings.Contains(got.HTML, `<h2 id="inner-section">`) {
			t.Errorf("the excerpt's heading lost its id, so links naming it now miss:\n%s", got.HTML)
		}
	})

	t.Run("a host's own headings stay, in order, without the excerpt's", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "## Host Heading\n\n![[B]]\n\n## Tail\n", wording.ZhHant)
		want := []render.TOCEntry{
			{Level: 2, Text: "Host Heading", ID: "host-heading"},
			{Level: 2, Text: "Tail", ID: "tail"},
		}
		if diff := cmp.Diff(want, got.TOC); diff != "" {
			t.Errorf("TOC (-want +got):\n%s", diff)
		}
	})
}

// TestAHeadingInsideAHostCalloutStaysInTheContents is the over-exclusion
// control: a callout is the host's own text, and its headings are the host's
// own sections. A scan that dropped every heading inside a div would drop
// these too.
func TestAHeadingInsideAHostCalloutStaysInTheContents(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "> [!note]\n> ## Inside The Callout\n> body\n", wording.ZhHant)
	want := []render.TOCEntry{{Level: 2, Text: "Inside The Callout", ID: "inside-the-callout"}}
	if diff := cmp.Diff(want, got.TOC); diff != "" {
		t.Errorf("TOC (-want +got):\n%s", diff)
	}
}

// TestAnExcerptInsideACalloutKeepsItsHeadingsOutOfTheContents nests the
// excerpt one level down: an embed written inside a host callout still brings
// another note's structure, wherever its container sits.
func TestAnExcerptInsideACalloutKeepsItsHeadingsOutOfTheContents(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": "## Inner Section\n\nbody\n",
	})

	got := r.HTML("note.md", "", "> [!note]\n> ![[B]]\n", wording.ZhHant)
	if len(got.TOC) != 0 {
		t.Errorf("TOC = %+v, want empty — the only heading on this page is the excerpt's", got.TOC)
	}
	if !strings.Contains(got.HTML, `<h2 id="inner-section">`) {
		t.Errorf("the excerpt's heading lost its id inside the callout:\n%s", got.HTML)
	}
}
