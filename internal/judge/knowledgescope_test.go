package judge

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

const (
	scopeLockSystemNote    = "System/notes/Probe.md"
	scopeLockSystemConcept = "System/notes/Idea.md"
	scopeLockNotesConcept  = "Notes/Kept.md"
	scopeLockAwayNote      = "Away/Loose.md"
)

// writeScopeLockVault writes one concept under Notes, one under System/notes,
// and two notes of a type the contract refuses, under the fixture contract
// with its folders.checked line replaced by checkedLine, which is the only
// thing each case changes.
func writeScopeLockVault(t *testing.T, checkedLine string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil, [2]string{
		`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, checkedLine,
	}))
	write(t, root, scopeLockNotesConcept, "---\ntype: concept\nbased_on:\n  - \"[[Idea]]\"\n---\n# Kept\nBody.\n")
	write(t, root, scopeLockSystemConcept, "---\ntype: concept\nbased_on:\n  - \"[[Kept]]\"\n---\n# Idea\nBody.\n")
	write(t, root, scopeLockSystemNote, "---\ntype: probe\n---\n# Probe\nBody.\n")
	write(t, root, scopeLockAwayNote, "---\ntype: probe\n---\n# Loose\nBody.\n")
	write(t, root, "Notes/README.md", "---\ntype: probe\n---\n# Skip me\n")
	return root
}

// enumPaths returns the sorted paths of the schema.enum findings.
func enumPaths(findings []Finding) []string {
	var paths []string
	for i := range findings {
		if findings[i].RuleID == "schema.enum" {
			paths = append(paths, findings[i].Path)
		}
	}
	slices.Sort(paths)
	return paths
}

// TestDefaultCheckFollowsDeclaredKnowledgeScope holds the check to the declared
// knowledge layer: with System declared, the default check reports the System finding;
// with folders.checked omitted or set to [], every note is linted.
func TestDefaultCheckFollowsDeclaredKnowledgeScope(t *testing.T) {
	t.Parallel()

	t.Run("System declared reports the System finding", func(t *testing.T) {
		t.Parallel()
		root := writeScopeLockVault(t, `checked = ["Notes", "System"]`)
		defaultFindings, err := runCheckAction(t.Context(), root, nil, false)
		if err != nil {
			t.Fatalf("check(default): %v", err)
		}
		got := enumPaths(defaultFindings)
		if diff := cmp.Diff([]string{scopeLockSystemNote}, got); diff != "" {
			t.Errorf("default check schema.enum paths mismatch (-want +got):\n%s", diff)
		}
	})

	for _, tt := range []struct {
		name string
		line string
	}{
		{name: "omitted folders.checked lints every note", line: ""},
		{name: "empty folders.checked lints every note", line: `checked = []`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := writeScopeLockVault(t, tt.line)

			defaultFindings, err := runCheckAction(t.Context(), root, nil, false)
			if err != nil {
				t.Fatalf("check(default): %v", err)
			}
			got := enumPaths(defaultFindings)
			if diff := cmp.Diff([]string{scopeLockAwayNote, scopeLockSystemNote}, got); diff != "" {
				t.Errorf("default check schema.enum paths mismatch (-want +got):\n%s", diff)
			}
			if slices.Contains(got, "Notes/README.md") {
				t.Errorf("default check linted Notes/README.md; skipped_files still applies when no layer is declared")
			}
		})
	}
}

// TestCoverageFollowsDeclaredKnowledgeScope holds coverage to the declared
// knowledge layer: with System declared, coverage counts the System concept.
func TestCoverageFollowsDeclaredKnowledgeScope(t *testing.T) {
	t.Parallel()

	root := writeScopeLockVault(t, `checked = ["Notes", "System"]`)
	wantMounts(t, coverageOf(t, root),
		[]domainCoverage{{Domain: "(none)", Concepts: 2, PendingMount: 2}},
		[]string{"Notes/Kept.md", "System/notes/Idea.md"}, nil)
}

// TestCoverageIgnoresMountSourcesOutsideTheDeclaredLayer holds that a map
// sitting outside the declared knowledge layer cannot decide a concept's mount
// state, the same silence the privacy cut keeps for a withheld source.
func TestCoverageIgnoresMountSourcesOutsideTheDeclaredLayer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract(t, `["Notes"]`))
	write(t, root, "Notes/Idea.md", "---\ntype: concept\nbased_on:\n  - \"[[Atlas]]\"\n---\n# Idea\nBody.\n")
	write(t, root, "Maps/Atlas.md", "---\ntype: atlas\n---\n# Atlas\n\n- [[Idea]]\n")

	wantMounts(t, coverageOf(t, root), []domainCoverage{{Domain: "(none)", Concepts: 1, Orphan: 1}}, nil, []string{"Notes/Idea.md"})
}
