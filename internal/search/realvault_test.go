package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestBuildRealVault builds the search index from the real vault (~/obsidian or
// YOMIHON_ROOT) and checks that it is non-empty and that a known, stable query
// hits its expected note. It follows internal/nav/realvault_test.go's
// t.Skipf-when-absent pattern, so it runs whenever the vault is present and is
// skipped loudly (not vacuously green) when it is not.
//
// slug:jp-minna-l01 is a stable key: slugs in this vault never change and
// are unique, and the Japanese prefix is jp-minna-lNN, so L01 is
// jp-minna-l01, living under Writing/lessons/japanese.
func TestBuildRealVault(t *testing.T) {
	t.Parallel()
	root := realVaultRoot(t)

	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load(%q) = %v", root, err)
	}
	policy := contract.ArtifactPolicy()
	idx, err := Build(root, policy)
	if err != nil {
		t.Fatalf("Build(%q) = %v", root, err)
	}
	if idx.Len() == 0 {
		t.Fatalf("real vault index is empty (root %q)", root)
	}

	t.Run("bare text", func(t *testing.T) {
		t.Parallel()
		bodyHits, searchErr := idx.Search(Parse("goroutine"))
		if searchErr != nil {
			t.Fatalf("Search(goroutine) = %v", searchErr)
		}
		if len(bodyHits) == 0 {
			t.Error(`body term "goroutine" matched nothing in the real vault`)
		}
	})

	t.Run("metadata", func(t *testing.T) {
		t.Parallel()
		if !policy.Available() {
			t.Skipf("real vault metadata search unavailable: %s", policy.Diagnostic())
		}
		got, searchErr := idx.Search(Parse("slug:jp-minna-l01"))
		if searchErr != nil {
			t.Fatalf("Search(slug:jp-minna-l01) = %v", searchErr)
		}
		if len(got) == 0 {
			t.Fatalf("slug:jp-minna-l01 matched nothing; index has %d notes", idx.Len())
		}
		if !strings.Contains(got[0].RelPath, "japanese/L01") {
			t.Errorf("slug:jp-minna-l01 top hit = %q, want a Writing/lessons/japanese/L01 note", got[0].RelPath)
		}
		t.Logf("real vault: %d notes indexed; slug:jp-minna-l01 -> %q", idx.Len(), got[0].RelPath)
	})
}

func realVaultRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("YOMIHON_ROOT")
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
