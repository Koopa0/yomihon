package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

const hiddenReportHuman = "6 findings: 0 error, 1 warn, 5 hidden (3 planned forward-refs, 2 external paths)\n" +
	"\ndebt by domain:\n" +
	"  golang               0 error · 1 warn\n" +
	"\nmost leveraged (create one, resolve many):\n" +
	"  ×3 [[Future]] (planned) — golang\n" +
	"\n▌ golang\n" +
	"  [warn] [[Missing]] resolves to no note  (Writing/golang/A.md)\n" +
	"\nhidden (info):\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
	"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/A.md\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md\n" +
	"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/B.md\n"

func TestHiddenReportRealProducers(t *testing.T) {
	t.Parallel()
	root := hiddenReportVault(t)
	hiddenReportProducers(t, root)
}

func TestHiddenReportFullHuman(t *testing.T) {
	t.Parallel()
	root := hiddenReportVault(t)
	hiddenReportProducers(t, root)
	stdout, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("RunCheck(human): %v", err)
	}
	if exit != 0 {
		t.Fatalf("RunCheck(human) exit = %d, want 0", exit)
	}
	t.Log("HIDDEN745_HUMAN_INVOKED")
	if diff := cmp.Diff(hiddenReportHuman, string(stdout)); diff != "" {
		t.Errorf("caught: HIDDEN745_FULL_HUMAN_MISMATCH: RunCheck(human) bytes differ (-want +got):\n%s", diff)
	}
}

func TestHiddenReportZeroInfo(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		findings []Finding
		want     string
	}{
		{
			name: "empty",
			want: "0 findings: 0 error, 0 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n",
		},
		{
			name:     "warning",
			findings: []Finding{{RuleID: "link.broken", Severity: SeverityWarn, Path: "Writing/golang/A.md", Message: "warning text"}},
			want:     "1 findings: 0 error, 1 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n" +
				"  golang               0 error · 1 warn\n" +
				"\n▌ golang\n" +
				"  [warn] warning text  (Writing/golang/A.md)\n",
		},
		{
			name:     "error",
			findings: []Finding{{RuleID: "schema.required", Severity: SeverityError, Path: "Writing/golang/A.md", Message: "error text"}},
			want:     "1 findings: 1 error, 0 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n" +
				"  golang               1 error · 0 warn\n" +
				"\n▌ golang\n" +
				"  [error] error text  (Writing/golang/A.md)\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := humanReport(tt.findings, domainRoots{"Writing"})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("humanReport(no info) bytes differ (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHiddenReportMixedSeverities(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		findings []Finding
		want     string
	}{
		{
			name: "actionable prefix and repeated other info rule",
			findings: []Finding{
				{RuleID: "schema.language", Severity: SeverityInfo, Path: "Writing/golang/B.md", Message: "informational text"},
				{RuleID: "link.broken", Severity: SeverityWarn, Path: "Writing/golang/A.md", Message: "warning text"},
				{RuleID: "schema.required", Severity: SeverityError, Path: "Writing/golang/A.md", Message: "error text"},
				{RuleID: "schema.language", Severity: SeverityInfo, Path: "Writing/golang/B.md", Message: "informational text"},
			},
			want: "4 findings: 1 error, 1 warn, 2 hidden (2 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n" +
				"  golang               1 error · 1 warn\n" +
				"\n▌ golang\n" +
				"  [error] error text  (Writing/golang/A.md)\n" +
				"  [warn] warning text  (Writing/golang/A.md)\n" +
				"\nhidden (info):\n" +
				"  [schema.language] informational text — Writing/golang/B.md\n" +
				"  [schema.language] informational text — Writing/golang/B.md\n",
		},
		{
			name:     "info with no path",
			findings: []Finding{{RuleID: "schema.language", Severity: SeverityInfo, Message: "informational text"}},
			want:     "1 findings: 0 error, 0 warn, 1 hidden (1 planned forward-refs, 0 external paths)\n" +
				"\nhidden (info):\n" +
				"  [schema.language] informational text — \n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := humanReport(tt.findings, domainRoots{"Writing"})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("humanReport(mixed) bytes differ (-want +got):\n%s", diff)
			}
		})
	}
}

func hiddenReportVault(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	contract := string(data)
	for _, edit := range []struct {
		old string
		new string
	}{
		{old: `domain_equals_folder_under = ["Concepts"]`, new: `domain_equals_folder_under = ["Writing"]`},
		{old: `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, new: `knowledge_dirs = ["Writing"]`},
		{old: "forbid_tag_with_slash = true", new: "forbid_tag_with_slash = true\nplanned_gap_marks = [\"Gaps\"]\nplanned_inline_marks = []"},
	} {
		if count := strings.Count(contract, edit.old); count != 1 {
			t.Fatalf("contract replacement %q matches %d sites, want 1", edit.old, count)
		}
		contract = strings.Replace(contract, edit.old, edit.new, 1)
	}
	if strings.Contains(contract, "[privacy]") {
		t.Fatal("contract fixture already declares privacy; adding a second table would invalidate the fixture")
	}
	root := t.TempDir()
	for _, file := range []struct {
		path string
		body string
	}{
		{path: schema.ContractRelPath, body: contract + "\n[privacy]\nnever_egress_dirs = [\"Diary\"]\n"},
		{path: "Writing/golang/A.md", body: "## Gaps\n\n[[Future]]\n[[Future]]\n\n## Current\n\n[external](../../../outside-745.md)\n[[Missing]]\n"},
		{path: "Writing/golang/B.md", body: "## Gaps\n\n[[Future]]\n\n## Current\n\n[external](../../../outside-745.md)\n"},
	} {
		full := filepath.Join(root, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(full, []byte(file.body), 0o600); err != nil {
			t.Fatalf("write fixture %q: %v", file.path, err)
		}
	}
	return root
}

func hiddenReportProducers(t *testing.T, root string) {
	t.Helper()
	got, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(hidden fixture): %v", err)
	}
	want := []Finding{
		{
			RuleID: "link.broken", Severity: SeverityInfo, Path: "Writing/golang/A.md", Line: new(3),
			Message: "[[Future]] resolves to no note", Evidence: "a tracked forward-reference (under a gap heading or listed as a planned concept)",
			SuggestedAction: "if it is written, check the filename/alias matches; otherwise leave it tracked", SourceRule: "yomihon",
			Target: new("Future"), Fingerprint: "v1:5afd0b44ba074aae",
		},
		{
			RuleID: "link.broken", Severity: SeverityInfo, Path: "Writing/golang/A.md", Line: new(4),
			Message: "[[Future]] resolves to no note", Evidence: "a tracked forward-reference (under a gap heading or listed as a planned concept)",
			SuggestedAction: "if it is written, check the filename/alias matches; otherwise leave it tracked", SourceRule: "yomihon",
			Target: new("Future"), Fingerprint: "v1:5afd0b44ba074aae",
		},
		{
			RuleID: "link.broken.path", Severity: SeverityInfo, Path: "Writing/golang/A.md", Line: new(8),
			Message: "link to ../../../outside-745.md points outside the vault root", Evidence: "external path, not stat'd (existence varies by environment)",
			SuggestedAction: "if it should be in the vault, fix the path; otherwise informational", SourceRule: "yomihon",
			Target: new("../../../outside-745.md"), Fingerprint: "v1:54e3ea23a7b5bc80",
		},
		{
			RuleID: "link.broken", Severity: SeverityWarn, Path: "Writing/golang/A.md", Line: new(9),
			Message: "[[Missing]] resolves to no note", Evidence: "no filename or alias matches the target",
			SuggestedAction: "create the target note, or change the link to an existing filename/alias", SourceRule: "yomihon",
			Target: new("Missing"), Fingerprint: "v1:dbf84169c7863c17",
		},
		{
			RuleID: "link.broken", Severity: SeverityInfo, Path: "Writing/golang/B.md", Line: new(3),
			Message: "[[Future]] resolves to no note", Evidence: "a tracked forward-reference (under a gap heading or listed as a planned concept)",
			SuggestedAction: "if it is written, check the filename/alias matches; otherwise leave it tracked", SourceRule: "yomihon",
			Target: new("Future"), Fingerprint: "v1:2fb3f6a685f5135d",
		},
		{
			RuleID: "link.broken.path", Severity: SeverityInfo, Path: "Writing/golang/B.md", Line: new(7),
			Message: "link to ../../../outside-745.md points outside the vault root", Evidence: "external path, not stat'd (existence varies by environment)",
			SuggestedAction: "if it should be in the vault, fix the path; otherwise informational", SourceRule: "yomihon",
			Target: new("../../../outside-745.md"), Fingerprint: "v1:5b26cdfc3b8a4ba1",
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Check(hidden fixture) producers differ (-want +got):\n%s", diff)
	}
	planned, external, warnings := 0, 0, 0
	for _, finding := range got {
		switch {
		case finding.RuleID == "link.broken" && finding.Severity == SeverityInfo:
			planned++
		case finding.RuleID == "link.broken.path" && finding.Severity == SeverityInfo:
			external++
		case finding.Severity == SeverityWarn:
			warnings++
		}
	}
	if planned != 3 || external != 2 || warnings != 1 {
		t.Fatalf("Check(hidden fixture) counts = planned:%d external:%d warn:%d, want 3/2/1", planned, external, warnings)
	}
	t.Log("HIDDEN745_PRODUCERS_INVOKED planned=3 external=2 warn=1")
}
