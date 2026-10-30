package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// TestExcludedDirectoryIsUnseenLikeAHiddenOne holds a directory the contract
// excludes to the answers a dot-directory gets: nothing under it is held, none
// of it is served, and a picture under it is not called missing, because the
// scan never looked there. A sibling directory stays read.
func TestExcludedDirectoryIsUnseenLikeAHiddenOne(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\nstatus: draft\n---\nbody\n")
	writeNote(t, root, "Concepts/there.png", "not really a picture\n")
	writeNote(t, root, "Concepts/lab/there.png", "not really a picture\n")
	writeNote(t, root, "Concepts/lab/Beta.md", "---\ntitle: Beta\ntype: concept\nstatus: draft\n---\nbody\n")

	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	const needle = "[scan]\n"
	text := strings.Replace(string(data), needle, needle+"exclude_dirs = [\"Concepts/lab\"]\n", 1)
	if text == string(data) {
		t.Fatal("the contract fixture does not contain the scan needle")
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err = os.MkdirAll(filepath.Dir(contractPath), 0o750); err != nil {
		t.Fatalf("mkdir contract fixture: %v", err)
	}
	if err = os.WriteFile(contractPath, []byte(text), 0o600); err != nil { // #nosec G703 -- the root is this test's own temporary directory
		t.Fatalf("write contract fixture: %v", err)
	}
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load: %v", err)
	}

	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	reader.SetBoundary(contract.ScanBoundary())
	store, err := New(t.Context(), reader, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
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
