package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

// A generation preserves the same literal verdict the command publishes,
// including the type reason instead of two false unknown-field findings.
func TestTypeDependentGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractBytes, err := os.ReadFile(filepath.Join("..", "judge", "testdata", "vault-type-dependent", schema.ContractRelPath))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	writeNote(t, root, schema.ContractRelPath, string(contractBytes))
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	const rel = "Notes/InvalidType.md"
	writeNote(t, root, rel, "---\ntitle: Invalid type\ntype: Lesson\nlevel: fundamental\nslug: invalid-type\n---\n\nBody.\n")
	store, _ := newTestStore(t, root, contract)
	want := []judge.Finding{
		{
			RuleID: "schema.enum", Severity: judge.SeverityError, Path: rel,
			Field: new("type"), Target: new("Lesson"),
			Message: "type \"Lesson\" is not an allowed type",
			Evidence: "frontmatter validated against vault-schema.toml",
			SuggestedAction: "fix the frontmatter to match the schema",
			SourceRule: "vault-schema.toml", Fingerprint: "v1:e6ac109dac9a51e1",
		},
		{
			RuleID: "schema.type_dependent", Severity: judge.SeverityError, Path: rel,
			Field: new("type"), Target: new("Lesson"),
			Message: "type \"Lesson\" is not valid, so type-only fields cannot be judged until type is valid",
			Evidence: "frontmatter validated against vault-schema.toml",
			SuggestedAction: "fix type before judging type-only fields",
			SourceRule: "vault-schema.toml", Fingerprint: "v1:f4f4f6f2e1bdf95c",
		},
	}
	if diff := cmp.Diff(want, store.Current().SchemaFindings(rel)); diff != "" {
		t.Errorf("caught: generation type-dependent verdict mismatch (-want +got):\n%s", diff)
	}
}
