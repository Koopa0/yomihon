//go:build realvault

package search

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// TestNewIndexRealVault checks that an explicitly selected private vault produces
// a non-empty, metadata-capable lexical index. Synthetic fixtures remain the
// contract for exact query semantics and ranking.
func TestNewIndexRealVault(t *testing.T) {
	t.Parallel()
	defer redactRealVaultPanic(t)

	root := requireRealVaultRoot(t)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal("open configured real vault failed")
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Error("close configured real vault failed")
		}
	})
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatal("load configured real-vault contract failed")
	}
	policy := contract.ArtifactPolicy()
	scan, err := reader.ScanAvailable(t.Context())
	if err != nil {
		t.Fatal("scan configured real vault failed")
	}
	documents := make([]Document, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		if !strings.HasSuffix(entry.Path(), ".md") {
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatal("read configured real-vault note failed")
		}
		documents = append(documents, DocumentFromNote(vault.Parse(entry.Path(), data)))
	}
	idx := NewIndex(documents, policy)
	if idx.Len() == 0 {
		t.Fatal("NewIndex() produced an empty real-vault index")
	}

	counts, err := idx.CountByStatus()
	indexedByStatus := 0
	if policy.Available() {
		if err != nil {
			t.Fatal("CountByStatus() failed with an available artifact policy")
		}
		for _, count := range counts {
			indexedByStatus += count
		}
		if indexedByStatus > idx.Len() {
			t.Errorf("metadata-capable notes = %d, want at most %d", indexedByStatus, idx.Len())
		}
	} else if !errors.Is(err, ErrMetadataUnavailable) {
		t.Fatal("CountByStatus() did not fail closed with an unavailable artifact policy")
	}

	t.Logf(
		"real-vault aggregate: notes=%d artifact_available=%t metadata_notes=%d status_categories=%d",
		idx.Len(),
		policy.Available(),
		indexedByStatus,
		len(counts),
	)
}

func redactRealVaultPanic(t *testing.T) {
	t.Helper()

	if recover() != nil {
		t.Fatal("real-vault search verification panicked")
	}
}

func requireRealVaultRoot(t *testing.T) string {
	t.Helper()

	root := os.Getenv("YOMIHON_ROOT")
	if root == "" {
		t.Skip("YOMIHON_ROOT is required for real-vault verification")
	}
	if _, err := os.Stat(root); err != nil { // #nosec G703 -- an explicit test-only opt-in selects the private root
		t.Fatal("configured real vault is unavailable")
	}
	return root
}
