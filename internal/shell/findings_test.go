package shell

import (
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
)

// TestGatheringSurvivesAFolderWithNoGeneration holds the one projection here
// that is a question rather than a field. Every rail states how many findings
// stand against the folder, which means the gathering now runs on every page —
// including one answered before a folder has ever been read, where the search
// index is absent and asking it anything at all would take the page down
// instead of saying there is nothing to report.
func TestGatheringSurvivesAFolderWithNoGeneration(t *testing.T) {
	t.Parallel()

	found := GatherFindings(nil)
	if total := found.Total(); total != 0 {
		t.Errorf("Total() = %d for a folder with no generation, want 0", total)
	}
}

func TestNavigationFaultCountsOnceBesideOtherFindings(t *testing.T) {
	t.Parallel()
	found := Findings{NavigationFaults: []nav.CoreFault{{}, {}}, Skipped: []snapshot.Skipped{{}}, SchemaFaults: []snapshot.HealthNoteFindings{{Count: 4}}}
	if got := found.Total(); got != 7 {
		t.Errorf("Total() = %d, want two navigation faults plus five existing findings", got)
	}
	only := Findings{NavigationFaults: []nav.CoreFault{{}}}
	if got := only.Total(); got != 1 {
		t.Errorf("fault-only Total() = %d, want 1", got)
	}
}
