package shell

import (
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestGatheringSurvivesAFolderWithNoGeneration holds the one projection here
// that is a question rather than a field. Every rail states how many findings
// stand against the folder, which means the gathering now runs on every page —
// including one answered before a folder has ever been read, where the search
// index is absent and asking it anything at all would take the page down
// instead of saying there is nothing to report.
func TestGatheringSurvivesAFolderWithNoGeneration(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile() error = %v", err)
	}
	found := GatherFindings(lifecycleView(t, contract), nil)
	if total := found.Total(); total != 0 {
		t.Errorf("Total() = %d for a folder with no generation, want 0", total)
	}
}
