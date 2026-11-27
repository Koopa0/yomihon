package judge

import (
	"bytes"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The private journal must not change what an agent-readable report says about a
// public note. These two tests pin that boundary on a fixture carrying both
// influence channels next to a public control for each: a concept mounted only
// by a journal map, and a public broken link whose target is planned only in a
// journal note.
const diaryInfluenceVault = "testdata/vault-diary-influence"

// TestCoverageExcludesJournalMountEdges pins that a journal note's map edges do
// not decide a public concept's mount state. A concept whose only mount edge
// comes from a note in the private daily journal is an orphan here, while a
// concept a public map mounts stays mounted. Counting the journal edge would
// file the concept as mounted and let a reader of coverage infer that the
// private journal links it.
func TestCoverageExcludesJournalMountEdges(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRootWithPrivacy(t, diaryInfluenceVault, "Diary")
	cov := coverageOf(t, root)

	const journalMounted = "Concepts/golang/Journal Mounted.md"
	const publicMounted = "Concepts/golang/Public Mounted.md"

	if !slices.Contains(cov.Orphans, journalMounted) {
		t.Errorf("coverage orphans = %v, want %q among them; its only mount edge is a journal map, which must not count",
			cov.Orphans, journalMounted)
	}
	if slices.Contains(cov.Orphans, publicMounted) || slices.Contains(cov.PendingMount, publicMounted) {
		t.Errorf("%q is listed as orphan or pending-mount, want it mounted by its public map", publicMounted)
	}
}

// TestCheckExcludesJournalPlannedNames pins that a planned-name list kept in the
// private daily journal does not soften a public broken link. A link whose
// target is planned only in the journal stays a warning here, while a link whose
// target a public note plans stays a tracked forward-reference. Counting the
// journal list would downgrade the public link to info and let a reader infer
// that the private journal names that target.
func TestCheckExcludesJournalPlannedNames(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRootWithPrivacy(t, diaryInfluenceVault, "Diary")
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check(%q): %v", root, err)
	}

	journalPlanned := findFinding(t, findings, "link.broken", "Concepts/golang/Journal Planned Link.md", "Planned Only In Journal")
	if journalPlanned.Severity != SeverityWarn {
		t.Errorf("journal-planned broken link severity = %s, want warn; a name planned only in the journal must not downgrade it",
			journalPlanned.Severity.String())
	}

	publicPlanned := findFinding(t, findings, "link.broken", "Concepts/golang/Public Planned Link.md", "Planned In Public")
	if publicPlanned.Severity != SeverityInfo {
		t.Errorf("public-planned broken link severity = %s, want info; a name a public note plans still tracks it",
			publicPlanned.Severity.String())
	}
}

func TestCheckBuildsSecondaryEvidenceOnlyFromEgressAllowedNotes(t *testing.T) {
	t.Parallel()

	const (
		publicA = "Concepts/golang/Public A.md"
		publicB = "Concepts/golang/Public B.md"
		linker  = "Concepts/golang/Linker.md"
		source  = "Concepts/golang/Source.md"
	)
	root := t.TempDir()
	writeTestContract(t, root, []string{"Restricted"})
	write(t, root, publicA, `---
aliases: [shared]
---
# Public A
`)
	write(t, root, publicB, `---
aliases: [shared]
---
# Shared Title
`)
	write(t, root, linker, `---
---
# Linker

[[Shared Title]]
`)
	write(t, root, source, `---
based_on: [restricted-slug]
---
# Source
`)
	write(t, root, "Restricted/Hidden.md", `---
aliases: [shared]
type: lesson
slug: restricted-slug
---
# Shared Title
`)

	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check(%q): %v", root, err)
	}

	collision := findFinding(t, findings, "collision.alias", "", "")
	if diff := cmp.Diff([]string{publicA, publicB}, collision.CollisionMembers); diff != "" {
		t.Errorf("collision members mismatch (-want +got):\n%s", diff)
	}
	if title := findFinding(t, findings, "link.title_not_alias", linker, ""); len(title.CollisionMembers) != 0 {
		t.Errorf("title holders = %v, want the one egress-allowed holder alone", title.CollisionMembers)
	}
	findFinding(t, findings, "provenance.unresolved", source, "restricted-slug")
	var wire bytes.Buffer
	if err := writeJSONL(&wire, findings); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	if bytes.Contains(wire.Bytes(), []byte("Restricted")) {
		t.Errorf("a finding names the withheld note:\n%s", wire.Bytes())
	}
}
