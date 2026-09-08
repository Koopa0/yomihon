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

## Gaps

[[MissingEN]]

## 缺口

[[MissingZH]]
`

const (
	gapNotePath = "Writing/Gaps.md"
	missingEN   = "MissingEN"
	missingZH   = "MissingZH"
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
	}{
		{
			name:       "English marks make the English heading info",
			rulesExtra: "planned_gap_marks = [\"Gaps\"]",
			wantEN:     SeverityInfo,
			wantZH:     SeverityWarn,
		},
		{
			name:       "declaring none makes both headings warn",
			rulesExtra: "planned_gap_marks = []\nplanned_inline_marks = []",
			wantEN:     SeverityWarn,
			wantZH:     SeverityWarn,
		},
		{
			name:   "omitted keys keep today's dialect",
			wantEN: SeverityWarn,
			wantZH: SeverityInfo,
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
			if en.Severity != tt.wantEN {
				t.Errorf("[[%s]] under ## Gaps: severity %s, want %s; evidence %q",
					missingEN, en.Severity, tt.wantEN, en.Evidence)
			}
			if zh.Severity != tt.wantZH {
				t.Errorf("[[%s]] under ## 缺口: severity %s, want %s; evidence %q",
					missingZH, zh.Severity, tt.wantZH, zh.Evidence)
			}
			assertBrokenEvidence(t, &en)
			assertBrokenEvidence(t, &zh)
		})
	}
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
