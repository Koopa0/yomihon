package judge

import (
	"context"
	"encoding/json"
	"errors"
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
			want: "1 findings: 0 error, 1 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n" +
				"  golang               0 error · 1 warn\n" +
				"\n▌ golang\n" +
				"  [warn] warning text  (Writing/golang/A.md)\n",
		},
		{
			name:     "error",
			findings: []Finding{{RuleID: "schema.required", Severity: SeverityError, Path: "Writing/golang/A.md", Message: "error text"}},
			want: "1 findings: 1 error, 0 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
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
			want: "1 findings: 0 error, 0 warn, 1 hidden (1 planned forward-refs, 0 external paths)\n" +
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

const hiddenReportInfoOnly = "5 findings: 0 error, 0 warn, 5 hidden (3 planned forward-refs, 2 external paths)\n" +
	"\nmost leveraged (create one, resolve many):\n" +
	"  ×3 [[Future]] (planned) — golang\n" +
	"\nhidden (info):\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
	"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/A.md\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md\n" +
	"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/B.md\n"

func TestHiddenReportRetention(t *testing.T) {
	t.Parallel()
	root := hiddenReportVault(t)
	base := hiddenReportBase(t, root)
	hiddenReportWrite(t, root, "Diary/Private.md", "## Gaps\n\n[[Private745]]\n[[Missing]]\n- Private745 / Missing\n\n## Current\n\n[external](../../private-outside-745.md)\n")
	a, err := openAction(t.Context(), root, actionHooks{})
	if err != nil {
		t.Fatalf("openAction(private stimulus): %v", err)
	}
	t.Cleanup(func() {
		if err := a.close(); err != nil {
			t.Errorf("close(private observation): %v", err)
		}
	})
	var private *note
	for i := range a.notes {
		if a.notes[i].path == "Diary/Private.md" {
			private = &a.notes[i]
		}
	}
	if private == nil {
		t.Fatal("openAction(private stimulus) did not capture Diary/Private.md")
	}
	if diff := cmp.Diff([]string{"Private745", "Missing"}, private.plannedNames); diff != "" {
		t.Fatalf("captured private planned vocabulary differs (-want +got):\n%s", diff)
	}
	type linkStimulus struct {
		target  string
		line    int
		planned bool
	}
	var links []linkStimulus
	for _, link := range private.wikilinks {
		links = append(links, linkStimulus{target: link.target, line: link.line, planned: link.underGapHeading})
	}
	if diff := cmp.Diff([]linkStimulus{{target: "Private745", line: 3, planned: true}, {target: "Missing", line: 4, planned: true}}, links, cmp.AllowUnexported(linkStimulus{})); diff != "" {
		t.Fatalf("captured private wikilinks differ (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]pathRef{{target: "../../private-outside-745.md", line: 9}}, private.pathRefs, cmp.AllowUnexported(pathRef{})); diff != "" {
		t.Fatalf("captured private path references differ (-want +got):\n%s", diff)
	}
	var rawPrivate []Finding
	for _, f := range runGraphRules(a, buildIndex(a.notes, a.unreadable, a.resources)) {
		if f.Path == private.path {
			rawPrivate = append(rawPrivate, f)
		}
	}
	wantPrivate := []Finding{
		hiddenReportPlanned("Diary/Private.md", 3, "Private745", "v1:7c03ef351e42c0cc"),
		hiddenReportPlanned("Diary/Private.md", 4, "Missing", "v1:1a4c5a76d8f72519"),
	}
	if diff := cmp.Diff(wantPrivate, rawPrivate); diff != "" {
		t.Fatalf("raw private graph findings differ (-want +got):\n%s", diff)
	}
	if a.authority.egressAllowed(private.path) {
		t.Fatal("privacy authority allows the captured private citer")
	}
	f, ok := classifyCapturedPathRef(private, "Diary", private.pathRefs[0], diskRefContext{authority: a.authority, contains: a.scan.Contains})
	if ok {
		t.Fatalf("classifyCapturedPathRef(private) = %+v, true, want zero finding, false", f)
	}
	if diff := cmp.Diff(Finding{}, f); diff != "" {
		t.Fatalf("classifyCapturedPathRef(private) denied finding differs (-want +got):\n%s", diff)
	}
	if err := a.finish(); err != nil {
		t.Fatalf("finish(private observation): %v", err)
	}
	t.Log("HIDDEN745_PRIVATE_STIMULUS_INVOKED planned=2 external-denied=1")
	for _, all := range []bool{false, true} {
		got, err := runCheckAction(t.Context(), root, nil, all)
		if err != nil {
			t.Fatalf("runCheckAction(private, all=%t): %v", all, err)
		}
		if diff := cmp.Diff(base, got); diff != "" {
			t.Fatalf("caught: HIDDEN745_PRIVATE_INFLUENCE_MISMATCH: private note changed retained findings, all=%t (-want +got):\n%s", all, diff)
		}
		hiddenReportOutput(t, &CheckOptions{Root: root, All: all, Format: FormatHuman}, hiddenReportHuman, 0)
	}
	hiddenReportWrite(t, root, "Other/Outside.md", "## Gaps\n\n[[Outside745]]\n\n## Current\n\n[external](../../outside-other-745.md)\n")
	other := []Finding{
		hiddenReportPlanned("Other/Outside.md", 3, "Outside745", "v1:e301d311cf352387"),
		{
			RuleID: "link.broken.path", Severity: SeverityInfo, Path: "Other/Outside.md", Line: new(7),
			Message: "link to ../../outside-other-745.md points outside the vault root", Evidence: "external path, not stat'd (existence varies by environment)",
			SuggestedAction: "if it should be in the vault, fix the path; otherwise informational", SourceRule: "yomihon",
			Target: new("../../outside-other-745.md"), Fingerprint: "v1:59d6f4e98c36c079",
		},
	}
	allRows := append(other, base...)
	for _, tt := range []struct {
		name string
		all  bool
		want []Finding
	}{
		{name: "default knowledge excludes Other", want: base},
		{name: "all includes Other and still excludes Diary", all: true, want: allRows},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCheckAction(t.Context(), root, nil, tt.all)
			if err != nil {
				t.Fatalf("runCheckAction(retention): %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("retained whole set differs (-want +got):\n%s", diff)
			}
			if tt.all {
				t.Log("HIDDEN745_OUTSIDE_KNOWLEDGE_INVOKED planned=1 external=1")
			}
		})
	}
	hiddenReportOutput(t, &CheckOptions{Root: root, Format: FormatHuman}, hiddenReportHuman, 0)
	hiddenReportOutput(t, &CheckOptions{Root: root, All: true, Format: FormatHuman},
		"8 findings: 0 error, 1 warn, 7 hidden (4 planned forward-refs, 3 external paths)\n" +
			"\ndebt by domain:\n  golang               0 error · 1 warn\n" +
			"\nmost leveraged (create one, resolve many):\n  ×3 [[Future]] (planned) — golang\n" +
			"\n▌ golang\n  [warn] [[Missing]] resolves to no note  (Writing/golang/A.md)\n" +
			"\nhidden (info):\n" +
			"  [link.broken] [[Outside745]] resolves to no note — Other/Outside.md\n" +
			"  [link.broken.path] link to ../../outside-other-745.md points outside the vault root — Other/Outside.md\n" +
			"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
			"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
			"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/A.md\n" +
			"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md\n" +
			"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/B.md\n", 0)
	got, err := runCheckAction(t.Context(), root, []string{"Writing/golang/B.md"}, true)
	if err != nil {
		t.Fatalf("runCheckAction(B scope): %v", err)
	}
	if diff := cmp.Diff(base[4:], got); diff != "" {
		t.Fatalf("B scope whole set differs (-want +got):\n%s", diff)
	}
	hiddenReportOutput(t, &CheckOptions{Root: root, Paths: []string{"Writing/golang/B.md"}, All: true, Format: FormatHuman},
		"2 findings: 0 error, 0 warn, 2 hidden (1 planned forward-refs, 1 external paths)\n" +
			"\nhidden (info):\n" +
			"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md\n" +
			"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/B.md\n", 0)
}

func TestHiddenReportBaseline(t *testing.T) {
	t.Parallel()
	root := hiddenReportVault(t)
	base := hiddenReportBase(t, root)
	for _, tt := range []struct {
		name string
		rows []Finding
		want string
	}{
		{
			name: "both A duplicates subtracted and B retained", rows: base[:1],
			want: "4 findings: 0 error, 1 warn, 3 hidden (1 planned forward-refs, 2 external paths)\n" +
				"\ndebt by domain:\n  golang               0 error · 1 warn\n" +
				"\n▌ golang\n  [warn] [[Missing]] resolves to no note  (Writing/golang/A.md)\n" +
				"\nhidden (info):\n" +
				"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/A.md\n" +
				"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md\n" +
				"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/B.md\n",
		},
		{
			name: "all infos subtracted", rows: []Finding{base[0], base[2], base[4], base[5]},
			want: "1 findings: 0 error, 1 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n  golang               0 error · 1 warn\n" +
				"\n▌ golang\n  [warn] [[Missing]] resolves to no note  (Writing/golang/A.md)\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var baseline []byte
			for _, row := range tt.rows {
				data, err := json.Marshal(struct {
					Fingerprint string `json:"fingerprint"`
				}{Fingerprint: row.Fingerprint})
				if err != nil {
					t.Fatalf("marshal baseline fingerprint: %v", err)
				}
				baseline = append(baseline, data...)
				baseline = append(baseline, '\n')
			}
			path := filepath.Join(t.TempDir(), "baseline.jsonl")
			if err := os.WriteFile(path, baseline, 0o600); err != nil {
				t.Fatalf("write baseline: %v", err)
			}
			hiddenReportOutput(t, &CheckOptions{Root: root, Baseline: path, Format: FormatHuman}, tt.want, 0)
		})
	}
}

func TestHiddenReportDeny(t *testing.T) {
	t.Parallel()
	root := hiddenReportVault(t)
	base := hiddenReportBase(t, root)
	hiddenReportReplace(t, root, "Writing/golang/A.md", "[[Missing]]\n", "")
	got, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(info-only): %v", err)
	}
	want := []Finding{base[0], base[1], base[2], base[4], base[5]}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("info-only whole producer set differs (-want +got):\n%s", diff)
	}
	for _, tt := range []struct {
		name string
		deny []string
		exit int
	}{
		{name: "no deny"},
		{name: "warn threshold", deny: []string{"warn"}},
		{name: "info threshold", deny: []string{"info"}, exit: 1},
		{name: "planned rule", deny: []string{"link.broken"}},
		{name: "external rule", deny: []string{"link.broken.path"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hiddenReportOutput(t, &CheckOptions{Root: root, Deny: tt.deny, Format: FormatHuman}, hiddenReportInfoOnly, tt.exit)
		})
	}
}

func TestHiddenReportAuthority(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		change func(*testing.T, string)
		cancel bool
		want   error
	}{
		{name: "missing contract", change: func(t *testing.T, root string) {
			t.Helper()
			if err := os.Remove(filepath.Join(root, schema.ContractRelPath)); err != nil {
				t.Fatalf("remove contract: %v", err)
			}
		}, want: ErrNoVaultContract},
		{name: "invalid contract", change: func(t *testing.T, root string) {
			t.Helper()
			hiddenReportWrite(t, root, schema.ContractRelPath, "[invalid745\n")
		}, want: ErrPrivacyAuthorityUnavailable},
		{name: "privacy incomplete", change: func(t *testing.T, root string) {
			t.Helper()
			hiddenReportReplace(t, root, schema.ContractRelPath, "\n[privacy]\nnever_egress_dirs = [\"Diary\"]\n", "\n")
		}, want: ErrPrivacyAuthorityUnavailable},
		{name: "already cancelled", cancel: true, want: ErrPrivacyAuthorityUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := hiddenReportVault(t)
			hiddenReportProducers(t, root)
			if tt.change != nil {
				tt.change(t, root)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancel {
				cancel()
			}
			rows, err := Check(ctx, root)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Check(refused) error = %v, want %v", err, tt.want)
			}
			if rows != nil {
				t.Errorf("Check(refused) rows = %+v, want nil", rows)
			}
			stdout, exit, err := RunCheck(ctx, &CheckOptions{Root: root, Format: FormatHuman})
			if !errors.Is(err, tt.want) {
				t.Fatalf("RunCheck(refused) error = %v, want %v", err, tt.want)
			}
			if stdout != nil || exit != 0 {
				t.Errorf("RunCheck(refused) payload/exit = %q/%d, want nil/0", stdout, exit)
			}
		})
	}
	t.Run("rendered payload loses authority before publication", func(t *testing.T) {
		root := hiddenReportVault(t)
		hiddenReportProducers(t, root)
		prepared, err := prepareCheckWithHooks(t.Context(), &CheckOptions{Root: root, Format: FormatHuman}, actionHooks{})
		if err != nil {
			t.Fatalf("prepareCheckWithHooks(stale): %v", err)
		}
		a := prepared.action
		t.Cleanup(func() {
			if err := a.close(); err != nil {
				t.Errorf("close(stale observation): %v", err)
			}
		})
		if prepared.exit != 0 {
			t.Fatalf("prepareCheckWithHooks(stale) exit = %d, want 0", prepared.exit)
		}
		t.Log("HIDDEN745_PREPARED_INVOKED")
		if diff := cmp.Diff(hiddenReportHuman, string(prepared.stdout)); diff != "" {
			t.Errorf("prepared hidden bytes differ (-want +got):\n%s", diff)
		}
		path := filepath.Join(root, schema.ContractRelPath)
		contract, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read captured contract: %v", err)
		}
		if err := os.WriteFile(path, append(contract, '\n'), 0o600); err != nil {
			t.Fatalf("replace captured authority: %v", err)
		}
		stdout, exit, err := hiddenReportPublish(&prepared)
		if !errors.Is(err, ErrPrivacyAuthorityUnavailable) {
			t.Fatalf("finish(stale) error = %v, want %v", err, ErrPrivacyAuthorityUnavailable)
		}
		if stdout != nil || exit != 0 {
			t.Errorf("finish(stale) payload/exit = %q/%d, want nil/0", stdout, exit)
		}
	})
	for _, tt := range []struct {
		name string
		path string
		want string
	}{
		{name: "unobserved scope", path: "Writing/Absent745.md", want: "path filter \"Writing/Absent745.md\" names nothing in this vault; give a vault-relative path such as \"Notes\" or \"Notes/topic.md\", or drop it to judge the whole vault"},
		{name: "private scope", path: "Diary/Private.md", want: "path filter \"Diary/Private.md\" lies under a directory this vault's contract withholds from agent-facing output; the scope was scanned but nothing from it can be reported, and an empty answer would read as a clean verdict"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := hiddenReportVault(t)
			hiddenReportProducers(t, root)
			hiddenReportWrite(t, root, "Diary/Private.md", "## Gaps\n\n[[Private745]]\n")
			stdout, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Paths: []string{tt.path}, Format: FormatHuman})
			if err == nil || err.Error() != tt.want {
				t.Fatalf("RunCheck(invalid scope) error = %v, want %q", err, tt.want)
			}
			if stdout != nil || exit != 0 {
				t.Errorf("RunCheck(invalid scope) payload/exit = %q/%d, want nil/0", stdout, exit)
			}
		})
	}
	t.Run("malformed baseline", func(t *testing.T) {
		root := hiddenReportVault(t)
		hiddenReportProducers(t, root)
		path := filepath.Join(t.TempDir(), "invalid.jsonl")
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatalf("write invalid baseline: %v", err)
		}
		stdout, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Baseline: path, Format: FormatHuman})
		want := "baseline " + path + ": line 1 carries no fingerprint"
		if err == nil || err.Error() != want {
			t.Fatalf("RunCheck(invalid baseline) error = %v, want %q", err, want)
		}
		if stdout != nil || exit != 0 {
			t.Errorf("RunCheck(invalid baseline) payload/exit = %q/%d, want nil/0", stdout, exit)
		}
	})
}

func hiddenReportBase(t *testing.T, root string) []Finding {
	t.Helper()
	hiddenReportProducers(t, root)
	return []Finding{
		hiddenReportPlanned("Writing/golang/A.md", 3, "Future", "v1:5afd0b44ba074aae"),
		hiddenReportPlanned("Writing/golang/A.md", 4, "Future", "v1:5afd0b44ba074aae"),
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
		hiddenReportPlanned("Writing/golang/B.md", 3, "Future", "v1:2fb3f6a685f5135d"),
		{
			RuleID: "link.broken.path", Severity: SeverityInfo, Path: "Writing/golang/B.md", Line: new(7),
			Message: "link to ../../../outside-745.md points outside the vault root", Evidence: "external path, not stat'd (existence varies by environment)",
			SuggestedAction: "if it should be in the vault, fix the path; otherwise informational", SourceRule: "yomihon",
			Target: new("../../../outside-745.md"), Fingerprint: "v1:5b26cdfc3b8a4ba1",
		},
	}
}

func hiddenReportPlanned(path string, line int, target, fp string) Finding {
	return Finding{
		RuleID: "link.broken", Severity: SeverityInfo, Path: path, Line: new(line),
		Message: "[[" + target + "]] resolves to no note", Evidence: "a tracked forward-reference (under a gap heading or listed as a planned concept)",
		SuggestedAction: "if it is written, check the filename/alias matches; otherwise leave it tracked", SourceRule: "yomihon",
		Target: new(target), Fingerprint: fp,
	}
}

func hiddenReportOutput(t *testing.T, options *CheckOptions, want string, wantExit int) {
	t.Helper()
	stdout, exit, err := RunCheck(t.Context(), options)
	if err != nil {
		t.Fatalf("RunCheck(hidden preservation): %v", err)
	}
	if exit != wantExit {
		t.Fatalf("RunCheck(hidden preservation) exit = %d, want %d", exit, wantExit)
	}
	if diff := cmp.Diff(want, string(stdout)); diff != "" {
		t.Errorf("RunCheck(hidden preservation) bytes differ (-want +got):\n%s", diff)
	}
}

func hiddenReportWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", rel, err)
	}
}

func hiddenReportReplace(t *testing.T, root, rel, old, replacement string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q for replacement: %v", rel, err)
	}
	if count := strings.Count(string(data), old); count != 1 {
		t.Fatalf("fixture %q replacement matches %d sites, want 1", rel, count)
	}
	hiddenReportWrite(t, root, rel, strings.Replace(string(data), old, replacement, 1))
}

// hiddenReportPublish drives the same finish-before-return boundary as RunCheck
// while the test controls the interval after the payload was rendered.
func hiddenReportPublish(prepared *preparedCommand) ([]byte, int, error) {
	if err := prepared.finish(); err != nil {
		return nil, 0, err
	}
	return prepared.stdout, prepared.exit, nil
}
