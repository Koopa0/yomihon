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
	return artifactPolicyFromSection(tb, "[artifacts]\nnon_instance_dirs = [\".\"]\n")
}

func incompleteArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "[artifacts]\n")
}

func artifactPolicyFromSection(tb testing.TB, section string) schema.ArtifactPolicy {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "vault-schema.toml")
	contract := `schema_version = "1"

[enums]
type = ["concept"]

[enums.status]
note = ["draft"]

[fields]
known = ["based_on"]

[rules]
concept_requires_provenance = ["based_on"]

` + section + `
[[lifecycle]]
status = "draft"
applies_to = ["concept"]
from = []
owner = ["koopa"]
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

// undeclaredArtifactPolicy is a contract that loaded and left [artifacts] out
// altogether. It is not the zero value: the zero value belongs to a folder that
// carries no contract, which excluded nothing on purpose, while this one meant
// to exclude directories yomihon cannot name.
func undeclaredArtifactPolicy(tb testing.TB) schema.ArtifactPolicy {
	tb.Helper()
	return artifactPolicyFromSection(tb, "")
}
