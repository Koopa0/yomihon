package snapshot

import (
	"testing"
)

// TestExcludedDirectoryIsUnseenLikeAHiddenOne holds a directory the contract
// excludes to the answers a dot-directory gets: nothing under it is held, none
// of it is served, and a picture under it is not called missing, because the
// scan never looked there. A sibling directory stays read.
func TestExcludedDirectoryIsUnseenLikeAHiddenOne(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nbody\n")
	writeNote(t, root, "Concepts/there.png", "not really a picture\n")
	writeNote(t, root, "Concepts/lab/there.png", "not really a picture\n")
	writeNote(t, root, "Concepts/lab/Beta.md", "---\ntype: concept\n---\nbody\n")

	contract := writeContract(t, root, replacing(t, "[folders]\n", "[folders]\nignored = [\"Concepts/lab\"]\n"))
	reader := openReader(t, root)
	reader.SetBoundary(contract.ScanBoundary())
	store := buildStore(t, reader, discardLogger(), contract)
	view := store.Current()

	for _, path := range []string{"Concepts/lab/there.png", "Concepts/lab/Beta.md", "Concepts/lab"} {
		if view.Contains(path) {
			t.Errorf("Contains(%q) = true, want false", path)
		}
		if _, held := view.Entry(path); held {
			t.Errorf("Entry(%q) is held, want not", path)
		}
	}
	for _, path := range []string{"Concepts/there.png", "Concepts/Alpha.md"} {
		if !view.Contains(path) {
			t.Errorf("Contains(%q) = false, want true", path)
		}
	}
	tests := []struct {
		path        string
		wantMissing bool
		wantOutside bool
	}{
		{path: "Concepts/lab/gone.png", wantOutside: true},
		{path: "Concepts/lab/deeper/gone.png", wantOutside: true},
		{path: "Concepts/gone.png", wantMissing: true},
		{path: "Concepts/there.png"},
	}
	for _, tt := range tests {
		if got := view.MissingFile(tt.path); got != tt.wantMissing {
			t.Errorf("MissingFile(%q) = %v, want %v", tt.path, got, tt.wantMissing)
		}
		if got := view.OutsideScan(tt.path); got != tt.wantOutside {
			t.Errorf("OutsideScan(%q) = %v, want %v", tt.path, got, tt.wantOutside)
		}
	}
}
