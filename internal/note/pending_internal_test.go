package note

import (
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/status"
)

// TestPending checks the pending-decision tally: a note counts when its
// (type, status) still has an owner-held onward move, except when its status is
// the seal itself. The lifecycle states are invented (start, advance, beyond) so
// the tally is what is proven, not any real vault's words; only the seal is
// referenced by its own constant so the exclusion has something real to bite on.
func TestPending(t *testing.T) {
	t.Parallel()

	// start -> advance -> seal -> beyond, all owner-held. Every state but beyond
	// therefore has a named onward move, and the seal has one too (-> beyond) —
	// so only the seal exclusion, not a lack of onward move, keeps seal notes out.
	contract := &schema.Schema{Lifecycle: []schema.Stage{
		{Status: "advance", AppliesTo: []string{"lesson"}, From: []string{"start"}, Owner: []string{"koopa"}},
		{Status: schema.SealStatus, AppliesTo: []string{"lesson"}, From: []string{"advance"}, Owner: []string{"koopa"}},
		{Status: "beyond", AppliesTo: []string{"lesson"}, From: []string{schema.SealStatus}, Owner: []string{"koopa"}},
	}}

	counts := map[search.TypeStatus]int{
		{Type: "lesson", Status: "start"}:           2, // onward: start -> advance
		{Type: "lesson", Status: "advance"}:         4, // onward: advance -> seal
		{Type: "lesson", Status: schema.SealStatus}: 3, // onward exists (-> beyond) but is the seal
		{Type: "lesson", Status: "beyond"}:          9, // no named onward move
	}
	h := &Handler{deps: Deps{
		Status:           status.NewService(t.TempDir(), contract),
		TypeStatusCounts: func() map[search.TypeStatus]int { return counts },
	}}

	count, known := h.pending()
	if !known {
		t.Fatal("pending() known = false on an open write face, want true")
	}
	if count != 6 {
		t.Errorf("pending() = %d, want 6 (start 2 + advance 4; the seal is excluded, beyond has no onward move)", count)
	}

	closed := &Handler{deps: Deps{
		Status:           status.NewService(t.TempDir(), nil),
		TypeStatusCounts: func() map[search.TypeStatus]int { return counts },
	}}
	if _, known := closed.pending(); known {
		t.Error("pending() known = true on a closed write face, want false")
	}
}
