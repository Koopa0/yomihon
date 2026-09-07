package judge

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// scopeLockContract is the Lock vault for KOO-50 / #235: one concept under
// Notes, one under System/notes, and a System note whose type the contract
// refuses. The knowledge_dirs line is the only thing each case changes.
const scopeLockContract = `schema_version = "1"

[enums]
type = ["concept", "note"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status", "based_on"]

[rules]
concept_requires_provenance = ["based_on"]

[scan]
KNOWLEDGE_DIRS
skip_basenames = ["README.md"]

[navigation]
path_types = []
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
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
	write(t, root, scopeLockNotesConcept, "---\ntitle: Kept\ntype: concept\nbased_on:\n  - \"[[Idea]]\"\n---\nBody.\n")
	write(t, root, scopeLockSystemConcept, "---\ntitle: Idea\ntype: concept\nbased_on:\n  - \"[[Kept]]\"\n---\nBody.\n")
	write(t, root, scopeLockSystemNote, "---\ntitle: Probe\ntype: probe\n---\nBody.\n")
	write(t, root, scopeLockAwayNote, "---\ntitle: Loose\ntype: probe\n---\nBody.\n")
	write(t, root, "Notes/README.md", "---\ntitle: Skip me\ntype: probe\n---\n")
	return root
}

func findingPaths(findings []Finding, rule RuleID) []string {
	var paths []string
	for i := range findings {
		if findings[i].RuleID == rule {
			paths = append(paths, findings[i].Path)
		}
	}
	return paths
}

// TestDefaultCheckFollowsDeclaredKnowledgeScope is the check half of the
// Lock: with System declared, the default check reports the System finding;
// with knowledge_dirs omitted or set to [], every note is linted.
func TestDefaultCheckFollowsDeclaredKnowledgeScope(t *testing.T) {
	t.Parallel()

	t.Run("System declared reports the System finding", func(t *testing.T) {
		t.Parallel()
		root := writeScopeLockVault(t, `knowledge_dirs = ["Notes", "System"]`)
		defaultFindings, err := runCheckAction(t.Context(), root, nil, false)
		if err != nil {
			t.Fatalf("check(default): %v", err)
		}
		got := findingPaths(defaultFindings, "schema.enum")
		if !slices.Contains(got, scopeLockSystemNote) {
			t.Errorf("default check paths = %v, want %s reported when System is declared", got, scopeLockSystemNote)
		}
		if slices.Contains(got, scopeLockAwayNote) {
			t.Errorf("default check reported %s; Away is outside the declared layer", scopeLockAwayNote)
		}
	})

	for _, tt := range []struct {
		name string
		line string
	}{
		{name: "omitted knowledge_dirs lints every note", line: ""},
		{name: "empty knowledge_dirs lints every note", line: `knowledge_dirs = []`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := writeScopeLockVault(t, tt.line)

			defaultFindings, err := runCheckAction(t.Context(), root, nil, false)
			if err != nil {
				t.Fatalf("check(default): %v", err)
			}
			got := findingPaths(defaultFindings, "schema.enum")
			for _, want := range []string{scopeLockSystemNote, scopeLockAwayNote} {
				if !slices.Contains(got, want) {
					t.Errorf("default check paths = %v, want %s linted when no layer is declared", got, want)
				}
			}
			if slices.Contains(got, "Notes/README.md") {
				t.Errorf("default check linted Notes/README.md; skip_basenames still applies when no layer is declared")
			}
		})
	}
}

// TestCoverageFollowsDeclaredKnowledgeScope is the coverage half of the Lock:
// with System declared, coverage counts the System concept.
func TestCoverageFollowsDeclaredKnowledgeScope(t *testing.T) {
	t.Parallel()

	root := writeScopeLockVault(t, `knowledge_dirs = ["Notes", "System"]`)
	got, exit, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if exit != 0 {
		t.Errorf("RunCoverage() exit = %d, want 0", exit)
	}
	if !bytes.Contains(got, []byte(`"total_concepts":2`)) {
		t.Errorf("RunCoverage() = %s, want total_concepts 2 (Notes and System)", got)
	}
	if !bytes.Contains(got, []byte(scopeLockSystemConcept)) {
		t.Errorf("RunCoverage() = %s, want the System concept counted when System is declared", got)
	}
}

// TestCoverageIgnoresMountSourcesOutsideTheDeclaredLayer holds that a map
// sitting outside the declared knowledge layer cannot decide a public
// concept's mount state. The privacy cut already treated a withheld source
// that way; the knowledge cut is the same silence.
func TestCoverageIgnoresMountSourcesOutsideTheDeclaredLayer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, `schema_version = "1"

[enums]
type = ["concept", "atlas"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status", "based_on"]

[rules]
concept_requires_provenance = ["based_on"]

[scan]
knowledge_dirs = ["Notes"]
skip_basenames = []

[navigation]
path_types = []
map_types = ["atlas"]

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`)
	write(t, root, "Notes/Idea.md", "---\ntitle: Idea\ntype: concept\nbased_on:\n  - \"[[Atlas]]\"\n---\nBody.\n")
	write(t, root, "Maps/Atlas.md", "---\ntitle: Atlas\ntype: atlas\n---\n\n- [[Idea]]\n")

	got, _, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if bytes.Contains(got, []byte(`"pending_mount":["Notes/Idea.md"]`)) {
		t.Errorf("RunCoverage() = %s, want Idea orphan; the atlas sits outside the declared layer", got)
	}
	if !bytes.Contains(got, []byte(`"orphans":["Notes/Idea.md"]`)) {
		t.Errorf("RunCoverage() = %s, want Idea counted as an orphan", got)
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
	write(t, root, schema.ContractRelPath, `schema_version = "1"

[enums]
type = ["concept", "research-brief"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status", "based_on"]

[rules]
concept_requires_provenance = ["based_on"]

[scan]
knowledge_dirs = ["Notes", "System"]
skip_basenames = []

[navigation]
path_types = []
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`)
	write(t, root, "Notes/Kept.md", "---\ntitle: Kept\ntype: concept\nbased_on:\n  - \"[[Brief]]\"\n---\nBody.\n")
	write(t, root, "System/notes/Brief.md", "---\ntitle: Brief\ntype: research-brief\n---\nBody.\n")
	write(t, root, "Away/Loose.md", "---\ntitle: Loose\ntype: research-brief\n---\nBody.\n")

	got, exit, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if exit != 0 {
		t.Errorf("RunCoverage() exit = %d, want 0", exit)
	}
	if !bytes.Contains(got, []byte(`"path":"System/notes/Brief.md"`)) {
		t.Errorf("RunCoverage() = %s, want the System brief unrouted when System is declared", got)
	}
	if bytes.Contains(got, []byte("Away/Loose.md")) {
		t.Errorf("RunCoverage() = %s, want the Away brief omitted; Away is outside the declared layer", got)
	}
}
