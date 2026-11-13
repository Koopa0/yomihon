package judge

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// scopeLockContract is the Lock vault: one concept under Notes, one under
// System/notes, and a System note whose type the contract refuses. The
// folders.checked line is the only thing each case changes.
const scopeLockContract = `schema_version = "2"

[folders]
KNOWLEDGE_DIRS
templates = []
skipped_files = ["README.md"]

[enums]

[fields]
common = ["type", "based_on"]
required = ["type"]

[types.concept]

[types.note]

[rules]
concept_requires_provenance = ["based_on"]

[privacy]
never_egress_dirs = []
`

const (
	scopeLockKnowledgeDirs = "KNOWLEDGE_DIRS"
	scopeLockSystemNote    = "System/notes/Probe.md"
	scopeLockSystemConcept = "System/notes/Idea.md"
	scopeLockNotesConcept  = "Notes/Kept.md"
	scopeLockAwayNote      = "Away/Loose.md"
)

func writeScopeLockVault(t *testing.T, knowledgeDirsLine string) string {
	t.Helper()
	root := t.TempDir()
	text := strings.Replace(scopeLockContract, scopeLockKnowledgeDirs, knowledgeDirsLine, 1)
	if text == scopeLockContract {
		t.Fatal("scopeLockContract no longer contains KNOWLEDGE_DIRS")
	}
	write(t, root, schema.ContractRelPath, text)
	write(t, root, scopeLockNotesConcept, "---\ntype: concept\nbased_on:\n  - \"[[Idea]]\"\n---\n# Kept\nBody.\n")
	write(t, root, scopeLockSystemConcept, "---\ntype: concept\nbased_on:\n  - \"[[Kept]]\"\n---\n# Idea\nBody.\n")
	write(t, root, scopeLockSystemNote, "---\ntype: probe\n---\n# Probe\nBody.\n")
	write(t, root, scopeLockAwayNote, "---\ntype: probe\n---\n# Loose\nBody.\n")
	write(t, root, "Notes/README.md", "---\ntype: probe\n---\n# Skip me\n")
	return root
}

func findingPaths(findings []Finding, rule RuleID) []string {
	var paths []string
	for i := range findings {
		if findings[i].RuleID == rule {
			paths = append(paths, findings[i].Path)
		}
	}
	slices.Sort(paths)
	return paths
}

// TestDefaultCheckFollowsDeclaredKnowledgeScope is the check half of the
// Lock: with System declared, the default check reports the System finding;
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
		got := findingPaths(defaultFindings, "schema.enum")
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
			got := findingPaths(defaultFindings, "schema.enum")
			if diff := cmp.Diff([]string{scopeLockAwayNote, scopeLockSystemNote}, got); diff != "" {
				t.Errorf("default check schema.enum paths mismatch (-want +got):\n%s", diff)
			}
			if slices.Contains(got, "Notes/README.md") {
				t.Errorf("default check linted Notes/README.md; skipped_files still applies when no layer is declared")
			}
		})
	}
}

// TestCoverageFollowsDeclaredKnowledgeScope is the coverage half of the Lock:
// with System declared, coverage counts the System concept.
func TestCoverageFollowsDeclaredKnowledgeScope(t *testing.T) {
	t.Parallel()

	root := writeScopeLockVault(t, `checked = ["Notes", "System"]`)
	got, exit, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if exit != 0 {
		t.Errorf("RunCoverage() exit = %d, want 0", exit)
	}
	want := `{"total_concepts":2,"domains":[{"domain":"(none)","concepts":2,"mounted":0,"pending_mount":2,"orphan":0}],"pending_mount":["Notes/Kept.md","System/notes/Idea.md"],"orphans":[],"unrouted":[]}` + "\n"
	if string(got) != want {
		t.Errorf("RunCoverage() =\n%s\nwant\n%s", got, want)
	}
}

// TestCoverageIgnoresMountSourcesOutsideTheDeclaredLayer holds that a map
// sitting outside the declared knowledge layer cannot decide a public
// concept's mount state. The privacy cut already treated a withheld source
// that way; the knowledge cut is the same silence.
func TestCoverageIgnoresMountSourcesOutsideTheDeclaredLayer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, `schema_version = "2"

[folders]
checked = ["Notes"]
templates = []

[enums]

[fields]
common = ["type", "based_on"]
required = ["type"]

[types.concept]

[types.atlas]
role = "map"

[rules]
concept_requires_provenance = ["based_on"]

[privacy]
never_egress_dirs = []
`)
	write(t, root, "Notes/Idea.md", "---\ntype: concept\nbased_on:\n  - \"[[Atlas]]\"\n---\n# Idea\nBody.\n")
	write(t, root, "Maps/Atlas.md", "---\ntype: atlas\n---\n# Atlas\n\n- [[Idea]]\n")

	got, _, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	want := `{"total_concepts":1,"domains":[{"domain":"(none)","concepts":1,"mounted":0,"pending_mount":0,"orphan":1}],"pending_mount":[],"orphans":["Notes/Idea.md"],"unrouted":[]}` + "\n"
	if string(got) != want {
		t.Errorf("RunCoverage() =\n%s\nwant\n%s", got, want)
	}
}

// TestCoverageUnroutedNotesFollowDeclaredKnowledgeScope is the unroutedNotes
// half of the Lock: coverageRoutes only fires when research-brief is declared,
// so a vault without that type never reaches the knowledge cut at all. With
// System declared, the System brief is unrouted; an Away brief is not. A
// HasPrefix(path, "System/") skip would hide the System brief and keep the Away
// one, and this test would go red.
func TestCoverageUnroutedNotesFollowDeclaredKnowledgeScope(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, `schema_version = "2"

[folders]
checked = ["Notes", "System"]
templates = []

[enums]

[fields]
common = ["type", "based_on"]
required = ["type"]

[types.concept]

[types.research-brief]

[rules]
concept_requires_provenance = ["based_on"]

[privacy]
never_egress_dirs = []
`)
	write(t, root, "Notes/Kept.md", "---\ntype: concept\nbased_on:\n  - \"[[Brief]]\"\n---\n# Kept\nBody.\n")
	write(t, root, "System/notes/Brief.md", "---\ntype: research-brief\n---\n# Brief\nBody.\n")
	write(t, root, "Away/Loose.md", "---\ntype: research-brief\n---\n# Loose\nBody.\n")

	got, exit, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if exit != 0 {
		t.Errorf("RunCoverage() exit = %d, want 0", exit)
	}
	want := `{"total_concepts":1,"domains":[{"domain":"(none)","concepts":1,"mounted":0,"pending_mount":0,"orphan":1}],"pending_mount":[],"orphans":["Notes/Kept.md"],"unrouted":[{"path":"System/notes/Brief.md","note_type":"research-brief","expected_route":"Maps/研究 Brief 索引"}]}` + "\n"
	if string(got) != want {
		t.Errorf("RunCoverage() =\n%s\nwant\n%s", got, want)
	}
}
