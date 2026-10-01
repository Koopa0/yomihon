package judge

import (
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestAnUnclosedFenceHasAnIdentityOfItsOwn holds what keeps the new reason from
// hiding behind an old one. Under a contract that wants a block, a note whose
// fence never closes draws "is missing" too, because the command already said
// that and adding a finding removes none. Both would hash to the same
// fingerprint if neither named its reason, since neither has a field or a
// value, and a baseline written when only "is missing" was said would then
// silence the one finding that names the actual fault.
func TestAnUnclosedFenceHasAnIdentityOfItsOwn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	text := contractFixture(t, nil,
		[2]string{`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `knowledge_dirs = ["Writing"]`},
	)
	text = strings.Replace(text, "no_frontmatter_is_legal = true", "no_frontmatter_is_legal = false", 1)
	write(t, root, schema.ContractRelPath, text)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}

	const path = "Writing/lessons/golang/L1.md"
	findings, err := LintFrontmatter(path, []byte("---\ntitle: L1\ntype: lesson\nstatus: draft\n--\n\nbody\n"), contract)
	if err != nil {
		t.Fatalf("LintFrontmatter() error = %v", err)
	}
	unclosedAt := slices.IndexFunc(findings, func(f Finding) bool { return strings.HasSuffix(f.Message, "never closes") })
	missingAt := slices.IndexFunc(findings, func(f Finding) bool { return strings.HasSuffix(f.Message, "is missing") })
	if len(findings) != 2 || unclosedAt < 0 || missingAt < 0 {
		t.Fatalf("LintFrontmatter() = %+v, want one finding that the fence never closes and one that the block is missing", findings)
	}
	unclosed, missing := findings[unclosedAt], findings[missingAt]
	if unclosed.Fingerprint == missing.Fingerprint {
		t.Errorf("the unclosed finding and the missing finding share fingerprint %s", unclosed.Fingerprint)
	}

	// The baseline holds what an earlier run said, which was only "is missing".
	kept := retainNew(slices.Clone(findings), map[string]bool{missing.Fingerprint: true})
	if len(kept) != 1 || kept[0].Fingerprint != unclosed.Fingerprint {
		t.Errorf("a baseline holding only the missing finding left %+v, want the unclosed finding to survive it", kept)
	}
}
