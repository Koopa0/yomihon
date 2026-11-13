package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// The artifact policies below are built the same way the index package builds
// them for its own tests. A test helper travels with the tests that need it
// rather than being exported from the package under test.

func validArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("schema.LoadFile: %v", err)
	}
	return contract.ArtifactPolicy()
}

func invalidArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[folders]\ntemplates = [\".\"]\n")
}

// escapingArtifactPolicy declares a templates folder above the vault root, so
// the contract loaded and the policy it meant cannot be honoured.
func escapingArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[folders]\ntemplates = [\"../outside\"]\n")
}

// blankArtifactPolicy declares a templates folder with no name.
func blankArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[folders]\ntemplates = [\"\"]\n")
}

func artifactPolicyFromSection(tb testing.TB, section string) schema.ArtifactPolicy {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "vault-schema.toml")
	contract := `schema_version = "2"

` + section + `
[types.concept]

[rules]
concept_requires_provenance = ["based_on"]
`
	if err := os.WriteFile(path, []byte(contract), 0o600); err != nil {
		tb.Fatalf("os.WriteFile: %v", err)
	}
	loaded, err := schema.LoadFile(path)
	if err != nil {
		tb.Fatalf("schema.LoadFile: %v", err)
	}
	return loaded.ArtifactPolicy()
}
