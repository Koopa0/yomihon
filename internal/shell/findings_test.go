package shell

import "testing"

// findingLists is how many entries the finding lists of a gathering hold
// between them, so that a test can ask whether it found anything at all. The
// health page counts its own rows from the same lists; this is no count a page
// states.
func findingLists(f *Findings) int {
	total := len(f.Blocked) + len(f.Skipped) + len(f.Unwritten) + len(f.TitleOnly) +
		len(f.Collisions) + len(f.NavigationFaults) + len(f.FrontmatterUnreadable) + len(f.SchemaFaults)
	for _, group := range f.Islands {
		total += len(group.Notes)
	}
	return total
}

// TestGatheringSurvivesAFolderWithNoGeneration holds the one projection here
// that is a question rather than a field. The gathering runs for the health
// page, which can be answered before a folder has ever been read, where the
// search index is absent and asking it anything at all would take the page down
// instead of saying there is nothing to report.
func TestGatheringSurvivesAFolderWithNoGeneration(t *testing.T) {
	t.Parallel()

	found := GatherFindings(nil)
	if total := findingLists(&found); total != 0 {
		t.Errorf("a folder with no generation gathers %d findings, want 0", total)
	}
}
