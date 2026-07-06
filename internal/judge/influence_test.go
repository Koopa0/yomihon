package judge

import (
	"encoding/json"
	"slices"
	"testing"
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
	cov := coverageOf(t, diaryInfluenceVault)

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
	findings, err := Check(diaryInfluenceVault)
	if err != nil {
		t.Fatalf("Check(%q): %v", diaryInfluenceVault, err)
	}

	journalPlanned := findBroken(t, findings, "Concepts/golang/Journal Planned Link.md", "Planned Only In Journal")
	if journalPlanned.Severity != SeverityWarn {
		t.Errorf("journal-planned broken link severity = %s, want warn; a name planned only in the journal must not downgrade it",
			journalPlanned.Severity.name())
	}

	publicPlanned := findBroken(t, findings, "Concepts/golang/Public Planned Link.md", "Planned In Public")
	if publicPlanned.Severity != SeverityInfo {
		t.Errorf("public-planned broken link severity = %s, want info; a name a public note plans still tracks it",
			publicPlanned.Severity.name())
	}
}

// coverageOf runs the coverage report over root and parses it back into the
// Coverage value the assertions read.
func coverageOf(t *testing.T, root string) Coverage {
	t.Helper()
	out, _, err := RunCoverage(&CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage(%q): %v", root, err)
	}
	var cov Coverage
	if err := json.Unmarshal(out, &cov); err != nil {
		t.Fatalf("parse coverage %q: %v", out, err)
	}
	return cov
}

// findBroken returns the one broken-link finding filed against path for target,
// failing when none is present so a missing finding reads as loudly as a wrong
// severity.
func findBroken(t *testing.T, findings []Finding, path, target string) Finding {
	t.Helper()
	for i := range findings {
		f := &findings[i]
		if f.RuleID == "link.broken" && f.Path == path && f.Target != nil && *f.Target == target {
			return *f
		}
	}
	t.Fatalf("no link.broken finding for %q -> %q; got %v", path, target, findings)
	return Finding{}
}
