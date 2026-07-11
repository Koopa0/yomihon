package note

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
)

// TestPending checks the pending-decision tally: a note counts when its
// (type, status) still has an owner-held onward move, except when its status is
// the seal itself. The fixture extends the ordinary lesson flow beyond the seal
// so the exclusion has an onward move to override rather than passing because
// the seal happens to be terminal.
func TestPending(t *testing.T) {
	t.Parallel()

	// imported -> draft -> seal -> beyond, all owner-held. Every state but beyond
	// therefore has a named onward move, and the seal has one too (-> beyond) —
	// so only the seal exclusion, not a lack of onward move, keeps seal notes out.
	contract, err := schema.LoadFile(filepath.Join("testdata", "pending-contract.toml"))
	if err != nil {
		t.Fatalf("load contract fixture: %v", err)
	}

	counts := map[search.TypeStatus]int{
		{Type: "lesson", Status: "imported"}:        2, // onward: imported -> draft
		{Type: "lesson", Status: "draft"}:           4, // onward: draft -> seal
		{Type: "lesson", Status: schema.SealStatus}: 3, // onward exists (-> beyond) but is the seal
		{Type: "lesson", Status: "beyond"}:          9, // no named onward move
	}
	var docs []search.Doc
	for ts, n := range counts {
		for i := range n {
			docs = append(docs, search.Doc{
				RelPath:  fmt.Sprintf("%s/%s-%d.md", ts.Type, ts.Status, i),
				NoteType: ts.Type,
				Status:   ts.Status,
			})
		}
	}
	snap := &snapshot.Snapshot{Search: search.BuildFromDocs(docs, contract.ArtifactPolicy())}
	policy := status.NewService(t.TempDir(), contract)

	count, known := pending(policy, snap)
	if !known {
		t.Fatal("pending() known = false on an open write face, want true")
	}
	if count != 6 {
		t.Errorf("pending() = %d, want 6 (imported 2 + draft 4; the seal is excluded, beyond has no onward move)", count)
	}

	closed := status.NewService(t.TempDir(), nil)
	if _, known := pending(closed, snap); known {
		t.Error("pending() known = true on a closed write face, want false")
	}

	unavailable := &snapshot.Snapshot{Search: search.BuildFromDocs(docs, schema.ArtifactPolicy{})}
	if _, known := pending(policy, unavailable); known {
		t.Error("pending() known = true without artifact metadata capability, want false")
	}
	h := &Handler{deps: Deps{Status: policy}}
	if got, _ := h.lifecycle(unavailable, ""); got != nil {
		t.Errorf("lifecycle() = %+v without artifact metadata capability, want nil", got)
	}
}
