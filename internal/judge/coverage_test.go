package judge

import (
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// atlasContract is the fixture contract with its first map type named atlas and
// the knowledge layer set to checked, so the coverage face has to read the map
// vocabulary from the contract rather than recognise a fixed set.
func atlasContract(t *testing.T, checked string, replacements ...[2]string) string {
	t.Helper()
	return contractFixture(t, nil, append([][2]string{
		{`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, "checked = " + checked},
		{"[types.moc]", "[types.atlas]"},
	}, replacements...)...)
}

// TestCoverageMountsOnContractMapTypes asserts a concept is mounted by an edge
// from the type its own contract declares as a map, whatever that type is
// called.
func TestCoverageMountsOnContractMapTypes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract(t, `["Concepts", "Atlases"]`))
	write(t, root, "Concepts/Slice.md", "---\ntype: concept\nbased_on:\n  - \"[[Go Atlas]]\"\n---\n# Slice\n\nBody.\n")
	write(t, root, "Atlases/Go Atlas.md", "---\ntype: atlas\n---\n# Go Atlas\n\n- [[Slice]]\n")

	wantMounts(t, coverageOf(t, root), []domainCoverage{{Domain: "(none)", Concepts: 1, Mounted: 1}}, nil, nil)
}

// TestCoveragePendingMountIsANonMapReference pins the advisory state: a concept
// reached only by another concept, never by a map, is pending a map, and one
// nothing reaches is an orphan.
func TestCoveragePendingMountIsANonMapReference(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract(t, `["Concepts", "Atlases"]`))
	write(t, root, "Concepts/Idea.md", "---\ntype: concept\nbased_on:\n  - \"[[Nowhere]]\"\n---\n# Idea\n\nBody.\n")
	write(t, root, "Concepts/Seed.md", "---\ntype: concept\nbased_on:\n  - \"[[Idea]]\"\n---\n# Seed\n\nBody.\n")

	wantMounts(t, coverageOf(t, root),
		[]domainCoverage{{Domain: "(none)", Concepts: 2, PendingMount: 1, Orphan: 1}},
		[]string{"Concepts/Idea.md"}, []string{"Concepts/Seed.md"})
}

// TestCoverageSaysSoWhenNoConceptTypeIsDeclared asserts a vault whose contract
// names no concept type is told, on both faces, that the concept tally does not
// apply to it, rather than handed a zero tally that reads as a clean vault.
func TestCoverageSaysSoWhenNoConceptTypeIsDeclared(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract(t, `["Notes", "Atlases"]`, [2]string{"[types.concept]\n", ""}))
	write(t, root, "Notes/Alpha.md", "---\ntype: note\n---\n# Alpha\n\nBody.\n")
	write(t, root, "Atlases/Go Atlas.md", "---\ntype: atlas\n---\n# Go Atlas\n\n- [[Alpha]]\n")

	cov := coverageOf(t, root)
	if cov.NotApplicable == "" || cov.TotalConcepts != 0 {
		t.Errorf("coverage = %+v, want a not-applicable reason and no tally", cov)
	}
	wantMounts(t, cov, nil, nil, nil)
	human, _, err := coverageBytes(t.Context(), &CoverageOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("RunCoverage(human) error = %v", err)
	}
	if got, want := string(human), cov.NotApplicable+"\n"; got != want {
		t.Errorf("RunCoverage(human) = %q, want the JSON's reason %q", got, want)
	}
}
