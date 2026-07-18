//go:build realvault

package schema_test

import (
	"os"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestLoadRealContract checks that an explicitly selected private vault's
// current contract passes strict decoding. Synthetic fixtures remain the
// contract for exact vocabulary, lifecycle behavior, and capability
// degradation.
func TestLoadRealContract(t *testing.T) {
	t.Parallel()
	defer redactRealVaultPanic(t)

	root := requireRealVaultRoot(t)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal("schema.Load() failed for the configured real vault")
	}
	definition := contract.Definition()

	t.Logf(
		"real-vault aggregate: note_types=%d status_groups=%d lifecycle_stages=%d navigation_available=%t artifact_available=%t privacy_available=%t",
		len(definition.Enums.Type),
		len(definition.Enums.Status),
		contract.StageCount(),
		contract.NavigationRoles().Available(),
		contract.ArtifactPolicy().Available(),
		contract.PrivacyPolicy().Available(),
	)
}

func redactRealVaultPanic(t *testing.T) {
	t.Helper()

	if recover() != nil {
		t.Fatal("real-vault schema verification panicked")
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
