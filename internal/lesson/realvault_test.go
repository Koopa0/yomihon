//go:build realvault

package lesson

import (
	"maps"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// TestRealVaultSlotsLoadAndValidate checks only anonymous structural
// invariants against an explicitly selected private vault. Synthetic fixtures
// remain the contract for exact lesson identities and content semantics.
func TestRealVaultSlotsLoadAndValidate(t *testing.T) {
	t.Parallel()
	defer redactRealVaultPanic(t)

	root := requireRealVaultRoot(t)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal("open configured real vault failed")
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error("close configured real vault failed")
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatal("scan configured real vault failed")
	}
	files := make(map[string][]byte)
	for _, entry := range scan.Files() {
		if !IsSlotSidecar(entry.Path()) {
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatal("read configured real-vault slot data failed")
		}
		files[entry.Path()] = data
	}
	if len(files) == 0 {
		t.Skip("configured real-vault lesson data is unavailable")
	}
	idx, problems := NewSlotIndex(files)
	// Every sidecar in the operator's own vault must be usable. One that is not
	// now costs only its own lesson, which is the point — but on this vault
	// nothing should be paying that cost, and a run that reports one is telling
	// the operator which file to open.
	for _, problem := range problems {
		t.Errorf("real-vault sidecar %s is unusable: %s", problem.Source, problem.Message)
	}
	if idx.Len() != len(files) {
		t.Errorf("indexed %d of %d real-vault sidecars", idx.Len(), len(files))
	}

	keys := slices.Sorted(maps.Keys(idx.bySlug))
	for ordinal, key := range keys {
		sidecar := idx.bySlug[key]
		if sidecar == nil {
			t.Errorf("sidecar[%d] is nil", ordinal)
			continue
		}
		if key == "" || sidecar.Slug == "" || key != sidecar.Slug {
			t.Errorf("sidecar[%d] has an invalid join identity", ordinal)
		}
		if sidecar.Lesson == "" {
			t.Errorf("sidecar[%d] has no lesson identity", ordinal)
		}
		if problems := sidecar.Validate(); len(problems) != 0 {
			t.Errorf("sidecar[%d] has %d validation problems", ordinal, len(problems))
		}
		got, ok := idx.Lookup(key)
		if !ok || !reflect.DeepEqual(got, sidecar) {
			t.Errorf("sidecar[%d] does not round-trip through Lookup", ordinal)
		}
		if got == sidecar {
			t.Errorf("sidecar[%d] Lookup exposed index-owned storage", ordinal)
		}
	}

	t.Logf("real-vault aggregate: sidecars=%d", idx.Len())
}

func redactRealVaultPanic(t *testing.T) {
	t.Helper()

	if recover() != nil {
		t.Fatal("real-vault lesson verification panicked")
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
