package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

const hiddenCheckHuman = "6 findings: 0 error, 1 warn, 5 hidden (3 planned forward-refs, 2 external paths)\n" +
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

const hiddenCheckJSON = `{"rule_id":"link.broken","severity":"info","path":"Writing/golang/A.md","line":3,"message":"[[Future]] resolves to no note","evidence":"a tracked forward-reference (under a gap heading or listed as a planned concept)","suggested_action":"if it is written, check the filename/alias matches; otherwise leave it tracked","source_rule":"yomihon","target":"Future","fingerprint":"v1:5afd0b44ba074aae"}` + "\n" +
	`{"rule_id":"link.broken","severity":"info","path":"Writing/golang/A.md","line":4,"message":"[[Future]] resolves to no note","evidence":"a tracked forward-reference (under a gap heading or listed as a planned concept)","suggested_action":"if it is written, check the filename/alias matches; otherwise leave it tracked","source_rule":"yomihon","target":"Future","fingerprint":"v1:5afd0b44ba074aae"}` + "\n" +
	`{"rule_id":"link.broken.path","severity":"info","path":"Writing/golang/A.md","line":8,"message":"link to ../../../outside-745.md points outside the vault root","evidence":"external path, not stat'd (existence varies by environment)","suggested_action":"if it should be in the vault, fix the path; otherwise informational","source_rule":"yomihon","target":"../../../outside-745.md","fingerprint":"v1:54e3ea23a7b5bc80"}` + "\n" +
	`{"rule_id":"link.broken","severity":"warn","path":"Writing/golang/A.md","line":9,"message":"[[Missing]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"yomihon","target":"Missing","fingerprint":"v1:dbf84169c7863c17"}` + "\n" +
	`{"rule_id":"link.broken","severity":"info","path":"Writing/golang/B.md","line":3,"message":"[[Future]] resolves to no note","evidence":"a tracked forward-reference (under a gap heading or listed as a planned concept)","suggested_action":"if it is written, check the filename/alias matches; otherwise leave it tracked","source_rule":"yomihon","target":"Future","fingerprint":"v1:2fb3f6a685f5135d"}` + "\n" +
	`{"rule_id":"link.broken.path","severity":"info","path":"Writing/golang/B.md","line":7,"message":"link to ../../../outside-745.md points outside the vault root","evidence":"external path, not stat'd (existence varies by environment)","suggested_action":"if it should be in the vault, fix the path; otherwise informational","source_rule":"yomihon","target":"../../../outside-745.md","fingerprint":"v1:5b26cdfc3b8a4ba1"}` + "\n"

const hiddenCheckMarkdown = "This vault's contract does not accept a fileable check report, so this body is not a note.\n\n" +
	"# yomihon check\n\n" +
	"6 findings — **0 error**, **1 warn**, 5 hidden.\n\n" +
	"## Debt by domain\n\n| domain | error | warn |\n|---|--:|--:|\n" +
	"| golang | 0 | 1 |\n\n" +
	"## Most leveraged (create one, resolve many)\n\n" +
	"- **×3** `[[Future]]` (planned) — golang\n\n" +
	"## golang\n\n" +
	"- `warn` [\\[Missing]\\] resolves to no note — Writing/golang/A.md\n\n" +
	"<details><summary>3 tracked forward-references · 2 external paths (info)</summary>\n\n" +
	"- `link.broken` [\\[Future]\\] resolves to no note — Writing/golang/A.md\n" +
	"- `link.broken` [\\[Future]\\] resolves to no note — Writing/golang/A.md\n" +
	"- `link.broken.path` link to ../../../outside-745.md points outside the vault root — Writing/golang/A.md\n" +
	"- `link.broken` [\\[Future]\\] resolves to no note — Writing/golang/B.md\n" +
	"- `link.broken.path` link to ../../../outside-745.md points outside the vault root — Writing/golang/B.md\n" +
	"\n</details>\n"

func TestHiddenCheckCommandFormats(t *testing.T) {
	root := hiddenCheckVault(t)
	hiddenCheckProducers(t, root)
	for _, tt := range []struct {
		name     string
		args     []string
		terminal bool
		cwdRoot  bool
		human    bool
		want     string
	}{
		{name: "terminal implicit human", terminal: true, human: true, want: hiddenCheckHuman},
		{name: "pipe explicit human", args: []string{"--format=human"}, human: true, want: hiddenCheckHuman},
		{name: "terminal explicit human", args: []string{"--format=human"}, terminal: true, human: true, want: hiddenCheckHuman},
		{name: "pipe implicit json", want: hiddenCheckJSON},
		{name: "pipe explicit json", args: []string{"--format=json"}, want: hiddenCheckJSON},
		{name: "terminal explicit json", args: []string{"--format=json"}, terminal: true, want: hiddenCheckJSON},
		{name: "pipe explicit markdown", args: []string{"--format=md"}, want: hiddenCheckMarkdown},
		{name: "terminal explicit markdown", args: []string{"--format=md"}, terminal: true, want: hiddenCheckMarkdown},
		{name: "cwd implicit human", terminal: true, cwdRoot: true, human: true, want: hiddenCheckHuman},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--root=" + root}, tt.args...)
			if tt.cwdRoot {
				t.Chdir(root)
				args = tt.args
			}
			var stdout, stderr bytes.Buffer
			exit := runCommand(t.Context(), "check", args, &stdout, &stderr, tt.terminal)
			if exit != 0 {
				t.Fatalf("runCommand(check) exit = %d, want 0; stderr = %q", exit, stderr.String())
			}
			if diff := cmp.Diff("", stderr.String()); diff != "" {
				t.Fatalf("runCommand(check) stderr differs (-want +got):\n%s", diff)
			}
			t.Log("HIDDEN745_COMMAND_INVOKED")
			if diff := cmp.Diff(tt.want, stdout.String()); diff != "" {
				if tt.human {
					t.Errorf("caught: HIDDEN745_FULL_HUMAN_MISMATCH: runCommand(check) stdout differs (-want +got):\n%s", diff)
				} else {
					t.Errorf("runCommand(check) stdout differs (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func hiddenCheckVault(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
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

func hiddenCheckProducers(t *testing.T, root string) {
	t.Helper()
	got, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(hidden fixture): %v", err)
	}
	want := []judge.Finding{
		{
			RuleID: "link.broken", Severity: judge.SeverityInfo, Path: "Writing/golang/A.md", Line: new(3),
			Message: "[[Future]] resolves to no note", Evidence: "a tracked forward-reference (under a gap heading or listed as a planned concept)",
			SuggestedAction: "if it is written, check the filename/alias matches; otherwise leave it tracked", SourceRule: "yomihon",
			Target: new("Future"), Fingerprint: "v1:5afd0b44ba074aae",
		},
		{
			RuleID: "link.broken", Severity: judge.SeverityInfo, Path: "Writing/golang/A.md", Line: new(4),
			Message: "[[Future]] resolves to no note", Evidence: "a tracked forward-reference (under a gap heading or listed as a planned concept)",
			SuggestedAction: "if it is written, check the filename/alias matches; otherwise leave it tracked", SourceRule: "yomihon",
			Target: new("Future"), Fingerprint: "v1:5afd0b44ba074aae",
		},
		{
			RuleID: "link.broken.path", Severity: judge.SeverityInfo, Path: "Writing/golang/A.md", Line: new(8),
			Message: "link to ../../../outside-745.md points outside the vault root", Evidence: "external path, not stat'd (existence varies by environment)",
			SuggestedAction: "if it should be in the vault, fix the path; otherwise informational", SourceRule: "yomihon",
			Target: new("../../../outside-745.md"), Fingerprint: "v1:54e3ea23a7b5bc80",
		},
		{
			RuleID: "link.broken", Severity: judge.SeverityWarn, Path: "Writing/golang/A.md", Line: new(9),
			Message: "[[Missing]] resolves to no note", Evidence: "no filename or alias matches the target",
			SuggestedAction: "create the target note, or change the link to an existing filename/alias", SourceRule: "yomihon",
			Target: new("Missing"), Fingerprint: "v1:dbf84169c7863c17",
		},
		{
			RuleID: "link.broken", Severity: judge.SeverityInfo, Path: "Writing/golang/B.md", Line: new(3),
			Message: "[[Future]] resolves to no note", Evidence: "a tracked forward-reference (under a gap heading or listed as a planned concept)",
			SuggestedAction: "if it is written, check the filename/alias matches; otherwise leave it tracked", SourceRule: "yomihon",
			Target: new("Future"), Fingerprint: "v1:2fb3f6a685f5135d",
		},
		{
			RuleID: "link.broken.path", Severity: judge.SeverityInfo, Path: "Writing/golang/B.md", Line: new(7),
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
		case finding.RuleID == "link.broken" && finding.Severity == judge.SeverityInfo:
			planned++
		case finding.RuleID == "link.broken.path" && finding.Severity == judge.SeverityInfo:
			external++
		case finding.Severity == judge.SeverityWarn:
			warnings++
		}
	}
	if planned != 3 || external != 2 || warnings != 1 {
		t.Fatalf("Check(hidden fixture) counts = planned:%d external:%d warn:%d, want 3/2/1", planned, external, warnings)
	}
	t.Log("HIDDEN745_PRODUCERS_INVOKED planned=3 external=2 warn=1")
}
