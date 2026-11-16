package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheMarkControlSurvivesANoteWithNoOtherAids is the reader's verb's own
// layout lock. The control lives in the header, which every reading page has,
// so it is drawn whatever else the note carries: a vault with no wikilink in it
// yet has no cited-by block on any note and a short note has no outline, which
// is the ordinary state of a new vault, and the control must still be there.
// It is drawn once, because a second face would be a second way for two copies
// to disagree about what a kept place is.
//
// Every fixture the recorded pages and the browser probe use carries aids, so
// nothing else in the tree asks this question.
func TestTheMarkControlSurvivesANoteWithNoOtherAids(t *testing.T) {
	t.Parallel()

	// A note in a folder that has no links yet: no outline, no diagnostics, no
	// declared sources, and nothing cites anything, so citedByShown is false.
	bare := NoteView{
		Title:           "T",
		RelPath:         "a.md",
		ContentIdentity: "abc123",
		MarkAddress:     "/marks",
		VaultHasLinks:   false,
	}
	if bare.citedByShown() || bare.endShown() {
		t.Fatal("this fixture was meant to have no aids at all, so it is not the case this test is about")
	}
	if !bare.offersMark() {
		t.Fatal("this fixture was meant to be able to offer a mark, so it is not the case this test is about")
	}

	var buf bytes.Buffer
	if err := Note(bare, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	page := buf.String()
	if got := strings.Count(page, "data-mark-control"); got != 1 {
		t.Errorf("the page draws the control for keeping a reading place %d times, want once in the header", got)
	}
	if !strings.Contains(page, "y-headermark") {
		t.Error("the page draws the control outside the header, where it would be reached by travelling and keep that place instead of the reader's")
	}
	if strings.Contains(page, "y-rail-right") {
		t.Error("the page draws a right rail; the contents are in the sidebar and everything else is at the end of the text")
	}
}

// TestANoteThatCannotOfferAMarkDrawsNoControl holds the other side, so the rule
// above cannot be satisfied by always drawing one. A page with no identity to
// bind a mark to — a stale reading, a file with no frontmatter — offers no
// control.
func TestANoteThatCannotOfferAMarkDrawsNoControl(t *testing.T) {
	t.Parallel()

	stale := NoteView{Title: "T", RelPath: "a.md", MarkAddress: "/marks"}
	if stale.offersMark() {
		t.Fatal("this fixture was meant to have no identity to bind a mark to")
	}

	var buf bytes.Buffer
	if err := Note(stale, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	page := buf.String()
	if strings.Contains(page, "data-mark-control") {
		t.Error("the page offers to keep a place it has no identity to bind")
	}
	if strings.Contains(page, "y-headermark") {
		t.Error("the header offers to keep a place the page has no identity to bind")
	}
}
