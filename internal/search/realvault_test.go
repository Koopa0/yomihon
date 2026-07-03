package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildRealVault builds the search index from the real vault (~/obsidian or
// KURODO_ROOT) and checks that it is non-empty and that a known, stable query
// hits its expected note. It follows internal/nav/realvault_test.go's
// t.Skipf-when-absent pattern, so it runs whenever the vault is present and is
// skipped loudly (not vacuously green) when it is not.
//
// slug:jp-minna-l01 is a spec-anchored, stable key: docs/vault-model.md states
// slugs never change and are unique, and the Japanese prefix is jp-minna-lNN,
// so L01 is jp-minna-l01, living under Writing/lessons/japanese.
func TestBuildRealVault(t *testing.T) {
	t.Parallel()
	root := realVaultRoot(t)

	idx, err := Build(root)
	if err != nil {
		t.Fatalf("Build(%q) = %v", root, err)
	}
	if idx.Len() == 0 {
		t.Fatalf("real vault index is empty (root %q)", root)
	}

	got := idx.Search(Parse("slug:jp-minna-l01"))
	if len(got) == 0 {
		t.Fatalf("slug:jp-minna-l01 matched nothing; index has %d notes", idx.Len())
	}
	if !strings.Contains(got[0].RelPath, "japanese/L01") {
		t.Errorf("slug:jp-minna-l01 top hit = %q, want a Writing/lessons/japanese/L01 note", got[0].RelPath)
	}

	// A plain body term also hits (spot check against the corpus).
	if len(idx.Search(Parse("goroutine"))) == 0 {
		t.Error(`body term "goroutine" matched nothing in the real vault`)
	}

	t.Logf("real vault: %d notes indexed; slug:jp-minna-l01 -> %q", idx.Len(), got[0].RelPath)
}

func realVaultRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("KURODO_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home dir: %v", err)
		}
		root = filepath.Join(home, "obsidian")
	}
	if _, err := os.Stat(root); err != nil { // #nosec G703 -- probing the operator's own vault to decide whether to skip
		t.Skipf("real vault not available: %v", err)
	}
	return root
}
