package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheMarkControlSurvivesANoteWithNoOtherAids is the reader's verb's own
// layout lock, and it is about the rail existing rather than about the control
// rendering.
//
// The right rail is dropped whole — the shell takes a modifier class that
// hides it at every width — for a note carrying no reading aids. What counts
// as an aid is the outline, the diagnostics, the declared sources, and the
// cited-by block, and the last of those is shown only once some note in the
// folder cites another. So a vault with no wikilink in it yet has no aids on
// any note, which is the ordinary state of a new one: the control was drawn
// into a column nobody could see, at 1600 as surely as at 320.
//
// Every fixture the recorded pages and the browser probe use carries aids, so
// nothing else in the tree asks this question.
func TestTheMarkControlSurvivesANoteWithNoOtherAids(t *testing.T) {
	t.Parallel()

	// A note in a folder that has no links yet: no outline, no diagnostics, no
	// declared sources, and nothing cites anything, so citedByShown is false.
	bare := NoteView{
		Governed:        true,
		Title:           "T",
		RelPath:         "a.md",
		Status:          "draft",
		ContentIdentity: "abc123",
		MarkAddress:     "/marks",
		VaultHasLinks:   false,
	}
	if bare.citedByShown() {
		t.Fatal("this fixture was meant to have no cited-by block, so it is not the case this test is about")
	}
	if !bare.offersMark() {
		t.Fatal("this fixture was meant to be able to offer a mark, so it is not the case this test is about")
	}
	if !bare.hasAids() {
		t.Error("a note that can offer to keep the reader's place counts as carrying an aid, and this one reports none")
	}

	var buf bytes.Buffer
	if err := Note(bare, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	page := buf.String()
	if strings.Contains(page, "y-shell--rail-empty") {
		t.Error("the shell drops the right rail on a note whose only aid is the control for keeping a reading place, which hides that control at every width")
	}
	if !strings.Contains(page, "data-mark-control") {
		t.Error("the page draws no control for keeping a reading place")
	}
}

// TestANoteThatCannotOfferAMarkStillCollapsesItsRail holds the other side, so
// the rule above cannot be satisfied by never collapsing. A page with no
// identity to bind a mark to — a stale reading, a file with no frontmatter —
// offers no control, and a rail holding nothing else is still dropped.
func TestANoteThatCannotOfferAMarkStillCollapsesItsRail(t *testing.T) {
	t.Parallel()

	stale := NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", MarkAddress: "/marks"}
	if stale.offersMark() {
		t.Fatal("this fixture was meant to have no identity to bind a mark to")
	}
	if stale.hasAids() {
		t.Error("a note with no aids and no mark to offer reports aids anyway, so the rail would never collapse")
	}

	var buf bytes.Buffer
	if err := Note(stale, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	page := buf.String()
	if !strings.Contains(page, "y-shell--rail-empty") {
		t.Error("the shell keeps an empty right rail on a note with nothing to put in it")
	}
	if strings.Contains(page, "data-mark-control") {
		t.Error("the page offers to keep a place it has no identity to bind")
	}
}
