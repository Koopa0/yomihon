package judge

import (
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// bilingualGapNote is one note with two structurally identical sections: an
// English heading and the Traditional Chinese heading today's loader default
// treats as a gap. Each holds one broken wikilink. The contract's mark lists
// are what decide whether each finding is a tracked forward-reference or a
// warning.
const bilingualGapNote = `---
title: Gaps
type: writing
domain: golang
status: draft
created: 2026-01-01
updated: 2026-01-01
---

todo: [[MissingTodo]]

## Gaps

[[MissingEN]]

## 缺口

[[MissingZH]]
`

const (
	gapNotePath      = "Writing/Gaps.md"
	missingEN        = "MissingEN"
	missingZH        = "MissingZH"
	missingTodo      = "MissingTodo"
	lueckeComposed   = "Lücke"
	lueckeDecomposed = "Lu\u0308cke"
)

// TestContractGapMarksDecideBrokenLinkSeverity is the lock: a contract that
// declares English heading marks makes the English gap heading info, and a
// contract that declares none makes both headings warn. The marks live on the
// contract, so a vault authored in English can name its own gap vocabulary.
func TestContractGapMarksDecideBrokenLinkSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rulesExtra string
		wantEN     Severity
		wantZH     Severity
		wantTodo   Severity
	}{
		{
			name:       "English marks make the English heading info",
			rulesExtra: "planned_gap_marks = [\"Gaps\"]",
			wantEN:     SeverityInfo,
			wantZH:     SeverityWarn,
			wantTodo:   SeverityWarn,
		},
		{
			name:       "declaring none makes both headings warn",
			rulesExtra: "planned_gap_marks = []\nplanned_inline_marks = []",
			wantEN:     SeverityWarn,
			wantZH:     SeverityWarn,
			wantTodo:   SeverityWarn,
		},
		{
			name:     "omitted keys keep today's dialect",
			wantEN:   SeverityWarn,
			wantZH:   SeverityInfo,
			wantTodo: SeverityWarn,
		},
		{
			name:       "English inline mark makes the inline target info",
			rulesExtra: "planned_gap_marks = []\nplanned_inline_marks = [\"todo\"]",
			wantEN:     SeverityWarn,
			wantZH:     SeverityWarn,
			wantTodo:   SeverityInfo,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := bilingualGapVault(t, tt.rulesExtra)
			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}

			en := findBroken(t, findings, gapNotePath, missingEN)
			zh := findBroken(t, findings, gapNotePath, missingZH)
			todo := findBroken(t, findings, gapNotePath, missingTodo)
			if en.Severity != tt.wantEN {
				t.Errorf("[[%s]] under ## Gaps: severity %s, want %s; evidence %q",
					missingEN, en.Severity, tt.wantEN, en.Evidence)
			}
			if zh.Severity != tt.wantZH {
				t.Errorf("[[%s]] under ## 缺口: severity %s, want %s; evidence %q",
					missingZH, zh.Severity, tt.wantZH, zh.Evidence)
			}
			if todo.Severity != tt.wantTodo {
				t.Errorf("[[%s]] beside todo: severity %s, want %s; evidence %q",
					missingTodo, todo.Severity, tt.wantTodo, todo.Evidence)
			}
			assertBrokenEvidence(t, &en)
			assertBrokenEvidence(t, &zh)
			assertBrokenEvidence(t, &todo)
		})
	}
}

// TestDecomposedHeadingHitsComposedGapMark is the NFC lock: a heading written
// with a combining umlaut (NFD Lücke) is the same mark as the composed
// spelling the contract declared. Case is not folded.
func TestDecomposedHeadingHitsComposedGapMark(t *testing.T) {
	t.Parallel()
	if lueckeComposed == lueckeDecomposed {
		t.Fatal("composed Lücke and decomposed Lu\\u0308cke must differ as written")
	}

	note := `---
title: Gaps
type: writing
domain: golang
status: draft
created: 2026-01-01
updated: 2026-01-01
---

## ` + lueckeDecomposed + `

[[MissingLuecke]]
`
	root := t.TempDir()
	write(t, root, gapNotePath, note)
	write(t, root, schema.ContractRelPath, contractFixture(t, nil,
		[2]string{
			`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			`knowledge_dirs = ["Writing"]`,
		},
		[2]string{
			"forbid_tag_with_slash = true",
			"forbid_tag_with_slash = true\nplanned_gap_marks = [\"" + lueckeComposed + "\"]\nplanned_inline_marks = []",
		},
	))

	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	got := findBroken(t, findings, gapNotePath, "MissingLuecke")
	if got.Severity != SeverityInfo {
		t.Fatalf("[[MissingLuecke]] under decomposed ## %s against composed %q: severity %s, want info; evidence %q",
			lueckeDecomposed, lueckeComposed, got.Severity, got.Evidence)
	}
	assertBrokenEvidence(t, &got)
}

func bilingualGapVault(t *testing.T, rulesExtra string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, gapNotePath, bilingualGapNote)
	replacements := [][2]string{
		{
			`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			`knowledge_dirs = ["Writing"]`,
		},
	}
	if rulesExtra != "" {
		replacements = append(replacements, [2]string{
			"forbid_tag_with_slash = true",
			"forbid_tag_with_slash = true\n" + rulesExtra,
		})
	}
	write(t, root, schema.ContractRelPath, contractFixture(t, nil, replacements...))
	return root
}

func assertBrokenEvidence(t *testing.T, f *Finding) {
	t.Helper()
	switch f.Severity {
	case SeverityInfo:
		if f.Evidence != "a tracked forward-reference (under a gap heading or listed as a planned concept)" {
			t.Errorf("info evidence = %q, want the tracked-forward-reference sentence", f.Evidence)
		}
	case SeverityWarn:
		if f.Evidence != "no filename or alias matches the target" {
			t.Errorf("warn evidence = %q, want the unmatched-target sentence", f.Evidence)
		}
	default:
		t.Errorf("severity %s is outside the lock", f.Severity)
	}
}

// The placement lock reads two notes: a Ledger note carrying one inline planned
// declaration, and a Citer note whose link names a target no note in the vault
// carries.
const (
	citerNotePath  = "Writing/Citer.md"
	ledgerNotePath = "Writing/Ledger.md"
	missingGhost   = "Ghost"
	missingPhantom = "Phantom"
)

// TestPlannedMarkCountsOnlyWhereTheAuthorSpeaks is the lock: an inline planned
// declaration softens another note's broken link to a tracked forward-reference
// only where it is written as prose. Quoted inside a fence or a code span, or
// taken back inside an Obsidian comment, it declares nothing and the other
// note's link stays a warning. The exit of a --deny warn run is asserted beside
// the severity because that exit is what a pipeline reads.
func TestPlannedMarkCountsOnlyWhereTheAuthorSpeaks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ledger string
		want   Severity
		exit   int
	}{
		{name: "written as prose", ledger: "todo: [[Ghost]]", want: SeverityInfo, exit: 0},
		{name: "quoted in a fence", ledger: "```\ntodo: [[Ghost]]\n```", want: SeverityWarn, exit: 1},
		{name: "quoted in a code span", ledger: "Example: `todo: [[Ghost]]`", want: SeverityWarn, exit: 1},
		{name: "taken back in a comment", ledger: "%% todo: [[Ghost]] %%", want: SeverityWarn, exit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := quotedPlannedVault(t, tt.ledger)
			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			got := findBroken(t, findings, citerNotePath, missingGhost)
			if got.Severity != tt.want {
				t.Errorf("[[%s]] cited by %s, declared %s: severity %s, want %s; evidence %q",
					missingGhost, citerNotePath, tt.name, got.Severity, tt.want, got.Evidence)
			}
			assertBrokenEvidence(t, &got)

			_, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"warn"}})
			if err != nil {
				t.Fatalf("RunCheck(--deny warn) error = %v", err)
			}
			if exit != tt.exit {
				t.Errorf("check --deny warn exit = %d, want %d; all findings %v", exit, tt.exit, findings)
			}
		})
	}
}

// TestACommentedHeadingBoundsNoGapSection is the same lock one level up, at the
// public result a pipeline reads: a gap heading Obsidian hides opens no section,
// so the declaration under it softens nothing; a closing heading Obsidian hides
// ends no section, so the declaration under it is still spoken. Both readings of
// the section are asserted at once — the name harvested for another note's link,
// and the ownership of a link written in the section itself.
func TestACommentedHeadingBoundsNoGapSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ledger string
		want   Severity
		exit   int
	}{
		{name: "a visible mark opens the section", ledger: "## Gaps\n\n- Ghost\n\n[[Phantom]]", want: SeverityInfo, exit: 0},
		{name: "a hidden opening mark opens nothing", ledger: "%%\n## Gaps\n%%\n\n- Ghost\n\n[[Phantom]]", want: SeverityWarn, exit: 1},
		{name: "a hidden closing heading closes nothing", ledger: "## Gaps\n\n%%\n## Finished\n%%\n\n- Ghost\n\n[[Phantom]]", want: SeverityInfo, exit: 0},
		{name: "a visible closing heading still closes the section", ledger: "## Gaps\n\n## Finished\n\n- Ghost\n\n[[Phantom]]", want: SeverityWarn, exit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := gapSectionVault(t, tt.ledger)
			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}

			cited := findBroken(t, findings, citerNotePath, missingGhost)
			if cited.Severity != tt.want {
				t.Errorf("[[%s]] cited by %s, declared where %s: severity %s, want %s; evidence %q",
					missingGhost, citerNotePath, tt.name, cited.Severity, tt.want, cited.Evidence)
			}
			assertBrokenEvidence(t, &cited)

			inside := findBroken(t, findings, ledgerNotePath, missingPhantom)
			if inside.Severity != tt.want {
				t.Errorf("[[%s]] written in %s where %s: severity %s, want %s; evidence %q",
					missingPhantom, ledgerNotePath, tt.name, inside.Severity, tt.want, inside.Evidence)
			}
			assertBrokenEvidence(t, &inside)

			_, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"warn"}})
			if err != nil {
				t.Fatalf("RunCheck(--deny warn) error = %v", err)
			}
			if exit != tt.exit {
				t.Errorf("check --deny warn exit = %d, want %d; all findings %v", exit, tt.exit, findings)
			}
		})
	}
}

// gapSectionVault writes the same two notes under a contract that tracks the
// English heading mark and no inline mark, so where the heading is written is
// the only thing that can decide either severity.
func gapSectionVault(t *testing.T, ledgerBody string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, citerNotePath, plannedFixtureNote("Citer", "See [[Ghost]]."))
	write(t, root, ledgerNotePath, plannedFixtureNote("Ledger", ledgerBody))
	write(t, root, schema.ContractRelPath, contractFixture(t, nil,
		[2]string{
			`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			`knowledge_dirs = ["Writing"]`,
		},
		[2]string{
			"forbid_tag_with_slash = true",
			"forbid_tag_with_slash = true\nplanned_gap_marks = [\"Gaps\"]\nplanned_inline_marks = []",
		},
	))
	return root
}

// quotedPlannedVault writes the two notes with ledgerBody as the whole body of
// the Ledger note, under a contract that tracks the inline mark and no heading
// mark, so where the declaration is written is the only thing that can decide
// the Citer note's severity.
func quotedPlannedVault(t *testing.T, ledgerBody string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, citerNotePath, plannedFixtureNote("Citer", "See [[Ghost]]."))
	write(t, root, ledgerNotePath, plannedFixtureNote("Ledger", ledgerBody))
	write(t, root, schema.ContractRelPath, contractFixture(t, nil,
		[2]string{
			`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			`knowledge_dirs = ["Writing"]`,
		},
		[2]string{
			"forbid_tag_with_slash = true",
			"forbid_tag_with_slash = true\nplanned_gap_marks = []\nplanned_inline_marks = [\"todo\"]",
		},
	))
	return root
}

func plannedFixtureNote(title, body string) string {
	return `---
title: ` + title + `
type: writing
domain: golang
status: draft
created: 2026-01-01
updated: 2026-01-01
---

` + body + "\n"
}
