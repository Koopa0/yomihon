package search

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/schema"
)

// The count on an offer is a promise about the vault, not about one response
// page, so it must not inherit the rendered list's bound: an offer that says
// two hundred where the vault holds more would teach the reader the same
// distrust a false zero does.
func TestStepBackCountIsNotBoundedByThePage(t *testing.T) {
	t.Parallel()

	docs := make([]lexical.Document, 0, maxRenderedResults+30)
	for i := range maxRenderedResults + 30 {
		docs = append(docs, lexical.Document{
			RelPath:   fmt.Sprintf("臨床/n%03d.md", i),
			Title:     fmt.Sprintf("Note %03d", i),
			PlainText: "furosemide dosing",
		})
	}
	idx := lexical.NewIndex(docs, schema.ArtifactPolicy{})

	got := idx.StepBacks("furosemide zzznotpresent")
	want := []lexical.StepBack{{Query: "furosemide", Count: maxRenderedResults + 30}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("StepBacks() mismatch (-want +got):\n%s", diff)
	}
}
