package judge

import (
	"encoding/json"
	"slices"
	"strings"
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
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(%q): %v", root, err)
	}

	journalPlanned := findBroken(t, findings, "Concepts/golang/Journal Planned Link.md", "Planned Only In Journal")
	if journalPlanned.Severity != SeverityWarn {
		t.Errorf("journal-planned broken link severity = %s, want warn; a name planned only in the journal must not downgrade it",
			journalPlanned.Severity.String())
	}

	publicPlanned := findBroken(t, findings, "Concepts/golang/Public Planned Link.md", "Planned In Public")
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
title: Public A
aliases: [shared]
---
`)
	write(t, root, publicB, `---
title: Shared Title
aliases: [shared]
---
`)
	write(t, root, linker, `---
title: Linker
---

[[Shared Title]]
`)
	write(t, root, source, `---
title: Source
based_on: [restricted-slug]
---
`)
	write(t, root, "Restricted/Hidden.md", `---
title: Shared Title
aliases: [shared]
type: lesson
slug: restricted-slug
---
`)

	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(%q): %v", root, err)
	}

	collision, ok := findingByRule(findings, "collision.alias")
	if !ok {
		t.Fatal("no collision.alias finding for two egress-allowed owners")
	}
	if diff := cmp.Diff([]string{publicA, publicB}, collision.CollisionMembers); diff != "" {
		t.Errorf("collision members mismatch (-want +got):\n%s", diff)
	}

	title, ok := findingByRuleAndPath(findings, "link.title_not_alias", linker)
	if !ok {
		t.Fatal("no link.title_not_alias finding for an egress-allowed title owner")
	}
	if want := "the target is the title of " + publicB + " but not one of its aliases"; title.Evidence != want {
		t.Errorf("title evidence = %q, want %q", title.Evidence, want)
	}

	provenance, ok := findingByRuleAndPath(findings, "provenance.unresolved", source)
	if !ok {
		t.Fatal("no provenance.unresolved finding for a slug owned only by an egress-denied note")
	}
	if provenance.Target == nil || *provenance.Target != "restricted-slug" {
		t.Errorf("provenance target = %v, want restricted-slug", provenance.Target)
	}
}

func TestCheckDoesNotInspectOrReportEgressDeniedPathTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		createFile bool
	}{
		{name: "missing"},
		{name: "present", createFile: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeTestContract(t, root, []string{"Restricted"})
			write(t, root, "Concepts/golang/Public.md", `---
title: Public
---

[private target](../../Restricted/secret.md)
`)
			if tt.createFile {
				write(t, root, "Restricted/secret.md", "secret")
			}

			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check(%q): %v", root, err)
			}
			for i := range findings {
				if findings[i].RuleID == "link.broken.path" {
					t.Fatalf("link.broken.path finding leaked an egress-denied target: %+v", findings[i])
				}
			}
		})
	}
}

func TestClassifyPathRefChecksPrivacyBeforeFilesystem(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, []string{"Restricted"})
	authority := loadTestAuthority(t, root)
	source := note{path: "Notes/public.md"}
	refs := []pathRef{
		{target: "../Restricted/secret.md"},
		{target: "Restricted/secret.md", code: true},
	}
	for _, ref := range refs {
		inspected := false
		contains := func(string) bool {
			inspected = true
			return true
		}
		if finding, ok := classifyPathRefWithContains(
			&source,
			"Notes",
			ref,
			authority,
			contains,
		); ok {
			t.Errorf("classifyPathRefWithContains(%+v) = %+v, want no private finding", ref, finding)
		}
		if inspected {
			t.Errorf("classifyPathRefWithContains(%+v) inspected membership before the privacy gate", ref)
		}
	}
}

// coverageOf runs the coverage report over root and parses it back into the
// Coverage value the assertions read.
func coverageOf(t *testing.T, root string) Coverage {
	t.Helper()
	out, _, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
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

func findingByRule(findings []Finding, ruleID RuleID) (Finding, bool) {
	for i := range findings {
		if findings[i].RuleID == ruleID {
			return findings[i], true
		}
	}
	return Finding{}, false
}

func findingByRuleAndPath(findings []Finding, ruleID RuleID, path string) (Finding, bool) {
	for i := range findings {
		if findings[i].RuleID == ruleID && findings[i].Path == path {
			return findings[i], true
		}
	}
	return Finding{}, false
}

// TestClassifyPathRefStaysSilentOutsideTheScan locks the difference between
// looking and finding nothing, and never looking. The scan skips hidden paths,
// so a link into one produced "does not exist in the vault" about a file that
// was sitting right there — the judge reporting its own boundary as a fault in
// the vault. The second half matters as much: a link that really is broken must
// still be reported, or this fix would trade a false positive for a false
// silence.
func TestClassifyPathRefStaysSilentOutsideTheScan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, nil)
	authority := loadTestAuthority(t, root)
	source := note{path: "Writing/README.md"}
	// Nothing is in the scan, so only the boundary decides.
	contains := func(string) bool { return false }

	tests := []struct {
		name        string
		target      string
		code        bool
		wantFinding bool
	}{
		{name: "hidden directory", target: "../.claude/skills/share-rewrite/SKILL.md"},
		{name: "hidden directory beside the note", target: ".config/notes.md"},
		{name: "hidden file", target: "../.hidden.md"},
		{name: "ordinary missing file is still reported", target: "../Notes/gone.md", wantFinding: true},
		{name: "missing file beside the note is still reported", target: "gone.md", wantFinding: true},
		// A reference written in code resolves against the vault root as well
		// as the note, and takes its own branch — the boundary has to hold on
		// both or the quieter one keeps reporting a file that is really there.
		{name: "hidden directory in code", target: ".claude/skills/share-rewrite/SKILL.md", code: true},
		{name: "hidden file in code", target: ".hidden.md", code: true},
		{name: "ordinary missing file in code is still reported", target: "Notes/gone.md", code: true, wantFinding: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			finding, ok := classifyPathRefWithContains(
				&source, "Writing", pathRef{target: tt.target, code: tt.code}, authority, contains,
			)
			if ok != tt.wantFinding {
				t.Fatalf("classifyPathRefWithContains(%q) reported %t, want %t (finding = %+v)",
					tt.target, ok, tt.wantFinding, finding)
			}
			if ok && !strings.Contains(finding.Evidence, "does not exist") {
				t.Errorf("classifyPathRefWithContains(%q) evidence = %q, want it to state the absence",
					tt.target, finding.Evidence)
			}
		})
	}
}
