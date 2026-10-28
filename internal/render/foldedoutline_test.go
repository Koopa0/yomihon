package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheOutlineOmitsHeadingsInsideAClosedCallout pins the contents list of a
// page that hides an answer behind a closed callout: the heading of that
// answer is not named there, while an open callout's heading and the page's
// own headings are. The heading is still a place, so it keeps its id and its
// entry in TOC.
func TestTheOutlineOmitsHeadingsInsideAClosedCallout(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	body := strings.Join([]string{
		"## Before",
		"",
		"> [!question]-",
		"> ## Hidden Answer",
		"> the answer",
		"",
		"> [!tip]+",
		"> ## Open Answer",
		"> shown",
		"",
		"## After",
		"",
	}, "\n")
	got := r.HTML("note.md", "", body, wording.ZhHant)

	want := []render.TOCEntry{
		{Level: 2, Text: "Before", ID: "before"},
		{Level: 2, Text: "Open Answer", ID: "open-answer"},
		{Level: 2, Text: "After", ID: "after"},
	}
	if diff := cmp.Diff(want, got.Outline()); diff != "" {
		t.Errorf("Outline() (-want +got):\n%s", diff)
	}

	wantAll := []render.TOCEntry{
		{Level: 2, Text: "Before", ID: "before"},
		{Level: 2, Text: "Hidden Answer", ID: "hidden-answer", Folded: true},
		{Level: 2, Text: "Open Answer", ID: "open-answer"},
		{Level: 2, Text: "After", ID: "after"},
	}
	if diff := cmp.Diff(wantAll, got.TOC); diff != "" {
		t.Errorf("TOC (-want +got):\n%s", diff)
	}
	if !strings.Contains(got.HTML, `<h3 id="hidden-answer" data-level="2">`) {
		t.Errorf("the closed callout's heading lost its id:\n%s", got.HTML)
	}
}

// TestTheOutlineFollowsNestingOfClosedCallouts pins that a callout is hidden
// when any callout around it is closed, and shown only when every one is not:
// a closed callout inside an ordinary one hides its headings, an ordinary or
// open callout inside a closed one hides its headings too, and closing one
// callout does not hide the headings that follow it.
func TestTheOutlineFollowsNestingOfClosedCallouts(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	body := strings.Join([]string{
		"> [!note]",
		"> ## Plain Outer",
		"> > [!question]-",
		"> > ## Closed Inside Plain",
		"> > answer",
		"> > ## Still Closed",
		"",
		"> [!question]-",
		"> > [!note]",
		"> > ## Plain Inside Closed",
		"> >",
		"> > [!tip]+",
		"> > ## Open Inside Closed",
		"",
		"> [!tip]+",
		"> > [!question]-",
		"> > ## Closed Inside Open",
		"> >",
		"> ## Open Outer",
		"",
		"## Last",
		"",
	}, "\n")
	got := r.HTML("note.md", "", body, wording.ZhHant)

	want := []render.TOCEntry{
		{Level: 2, Text: "Plain Outer", ID: "plain-outer"},
		{Level: 2, Text: "Open Outer", ID: "open-outer"},
		{Level: 2, Text: "Last", ID: "last"},
	}
	if diff := cmp.Diff(want, got.Outline()); diff != "" {
		t.Errorf("Outline() (-want +got):\n%s", diff)
	}
}
