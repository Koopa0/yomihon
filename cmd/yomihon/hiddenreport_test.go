package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
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

const hiddenCheckInfoOnly = "5 findings: 0 error, 0 warn, 5 hidden (3 planned forward-refs, 2 external paths)\n" +
	"\nmost leveraged (create one, resolve many):\n" +
	"  ×3 [[Future]] (planned) — golang\n" +
	"\nhidden (info):\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md\n" +
	"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/A.md\n" +
	"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md\n" +
	"  [link.broken.path] link to ../../../outside-745.md points outside the vault root — Writing/golang/B.md\n"

const hiddenCheckMissingAuthority = "yomihon: this folder has no vault contract, so there is nothing to judge notes against\n" +
	"  yomihon reads System/schemas/vault-schema.toml for the note types, fields and\n" +
	"  lifecycle that check, coverage and exists judge against, and for the directories\n" +
	"  whose contents must never leave this machine. A folder carrying no such file has\n" +
	"  declared nothing, and these three commands have no vocabulary to answer in.\n" +
	"  Reading and search need none of it: yomihon <dir>\n" +
	"  /health shows link findings even when the folder has no contract.\n" +
	"  A starter is examples/vault/System/schemas/vault-schema.toml in the source archive\n" +
	"  at https://github.com/koopa0/yomihon/releases; choose your binary's release.\n"

const hiddenCheckUnavailableAuthority = "yomihon: privacy authority unavailable; agent-facing output disabled\n" +
	"  The contract is at System/schemas/vault-schema.toml and yomihon could not use it.\n" +
	"  The reason is not printed here: this command's output is written for a program to\n" +
	"  read, and stating the reason would quote the contract back out under exactly the\n" +
	"  policy that is missing. Read it where reading is the point: yomihon serve\n" +
	"  --root <dir> states the cause on the page, and the server logs it at startup.\n"

func TestHiddenCheckDeny(t *testing.T) {
	root := hiddenCheckVault(t)
	hiddenCheckProducers(t, root)
	hiddenCheckReplace(t, root, "Writing/golang/A.md", "[[Missing]]\n", "")
	got, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(info-only): %v", err)
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
		t.Fatalf("info-only whole producer set differs (-want +got):\n%s", diff)
	}
	for _, tt := range []struct {
		name string
		deny string
		exit int
	}{
		{name: "no deny"},
		{name: "warn threshold", deny: "warn"},
		{name: "info threshold", deny: "info", exit: 1},
		{name: "planned rule", deny: "link.broken"},
		{name: "external rule", deny: "link.broken.path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"--root=" + root, "--format=human"}
			if tt.deny != "" {
				args = append(args, "--deny="+tt.deny)
			}
			var stdout, stderr bytes.Buffer
			exit := runCommand(t.Context(), "check", args, &stdout, &stderr, false)
			if exit != tt.exit {
				t.Fatalf("runCommand(info-only) exit = %d, want %d; stderr = %q", exit, tt.exit, stderr.String())
			}
			if diff := cmp.Diff("", stderr.String()); diff != "" {
				t.Fatalf("runCommand(info-only) stderr differs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(hiddenCheckInfoOnly, stdout.String()); diff != "" {
				t.Errorf("runCommand(info-only) stdout differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHiddenCheckCommandFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*testing.T, string)
		args   []string
		cancel bool
		want   string
	}{
		{name: "missing contract", change: func(t *testing.T, root string) {
			t.Helper()
			if err := os.Remove(filepath.Join(root, schema.ContractRelPath)); err != nil {
				t.Fatalf("remove contract: %v", err)
			}
		}, want: hiddenCheckMissingAuthority},
		{name: "invalid contract", change: func(t *testing.T, root string) {
			t.Helper()
			hiddenCheckWrite(t, root, schema.ContractRelPath, "[invalid745\n")
		}, want: hiddenCheckUnavailableAuthority},
		{name: "privacy incomplete", change: func(t *testing.T, root string) {
			t.Helper()
			hiddenCheckReplace(t, root, schema.ContractRelPath, "\n[privacy]\nnever_egress_dirs = [\"Diary\"]\n", "\n")
		}, want: hiddenCheckUnavailableAuthority},
		{name: "already cancelled", cancel: true, want: hiddenCheckUnavailableAuthority},
		{name: "unobserved scope", args: []string{"Writing/Absent745.md"}, want: "yomihon: path filter \"Writing/Absent745.md\" names nothing in this vault; give a vault-relative path such as \"Notes\" or \"Notes/topic.md\", or drop it to judge the whole vault\n"},
		{name: "private scope", change: func(t *testing.T, root string) {
			t.Helper()
			hiddenCheckWrite(t, root, "Diary/Private.md", "## Gaps\n\n[[Private745]]\n")
		}, args: []string{"Diary/Private.md"}, want: "yomihon: path filter \"Diary/Private.md\" lies under a directory this vault's contract withholds from agent-facing output; the scope was scanned but nothing from it can be reported, and an empty answer would read as a clean verdict\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := hiddenCheckVault(t)
			hiddenCheckProducers(t, root)
			if tt.change != nil {
				tt.change(t, root)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancel {
				cancel()
			}
			args := append([]string{"--root=" + root, "--format=human"}, tt.args...)
			var stdout, stderr bytes.Buffer
			exit := runCommand(ctx, "check", args, &stdout, &stderr, false)
			if exit != 2 {
				t.Fatalf("runCommand(refused) exit = %d, want 2; stderr = %q", exit, stderr.String())
			}
			if diff := cmp.Diff(tt.want, stderr.String()); diff != "" {
				t.Fatalf("runCommand(refused) stderr differs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff("", stdout.String()); diff != "" {
				t.Errorf("runCommand(refused) stdout differs (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("malformed baseline", func(t *testing.T) {
		root := hiddenCheckVault(t)
		hiddenCheckProducers(t, root)
		path := filepath.Join(t.TempDir(), "invalid.jsonl")
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatalf("write invalid baseline: %v", err)
		}
		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), "check", []string{"--root=" + root, "--format=human", "--baseline=" + path}, &stdout, &stderr, false)
		if exit != 2 {
			t.Fatalf("runCommand(invalid baseline) exit = %d, want 2; stderr = %q", exit, stderr.String())
		}
		want := "yomihon: baseline " + path + ": line 1 carries no fingerprint\n"
		if diff := cmp.Diff(want, stderr.String()); diff != "" {
			t.Fatalf("runCommand(invalid baseline) stderr differs (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("", stdout.String()); diff != "" {
			t.Errorf("runCommand(invalid baseline) stdout differs (-want +got):\n%s", diff)
		}
	})
	for _, tt := range []struct {
		name   string
		fail   bool
		exit   int
		stderr string
		stdout string
	}{
		{name: "partial stdout write", fail: true, exit: 2, stderr: "yomihon: write output: hidden745 writer failed\n", stdout: "6 findings: "},
		{name: "successful stdout write", stdout: hiddenCheckHuman},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := hiddenCheckVault(t)
			hiddenCheckProducers(t, root)
			writer := &hiddenCheckWriter{limit: -1}
			if tt.fail {
				writer.limit = 12
				writer.err = errors.New("hidden745 writer failed")
			}
			var stderr bytes.Buffer
			exit := runCommand(t.Context(), "check", []string{"--root=" + root, "--format=human"}, writer, &stderr, false)
			if exit != tt.exit {
				t.Fatalf("runCommand(writer) exit = %d, want %d; stderr = %q", exit, tt.exit, stderr.String())
			}
			if diff := cmp.Diff(tt.stderr, stderr.String()); diff != "" {
				t.Fatalf("runCommand(writer) stderr differs (-want +got):\n%s", diff)
			}
			if len(writer.offered) == 0 {
				t.Fatal("runCommand(writer) never offered the nonempty report to stdout")
			}
			t.Log("HIDDEN745_WRITER_INVOKED")
			if diff := cmp.Diff(hiddenCheckHuman, string(writer.offered)); diff != "" {
				t.Errorf("runCommand(writer) offered bytes differ (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.stdout, writer.written.String()); diff != "" {
				t.Errorf("runCommand(writer) committed prefix differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHiddenCheckMain(t *testing.T) {
	if os.Getenv("YOMIHON_TEST_HIDDEN745_MAIN") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
				main()
				t.Fatal("main(check) returned without process exit")
			}
		}
		t.Fatal("hidden main helper received no -- argument delimiter")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	for _, tt := range []struct {
		name    string
		args    []string
		want    string
		exit    int
		missing bool
		stderr  string
	}{
		{name: "pipe implicit json", want: hiddenCheckJSON},
		{name: "pipe explicit human", args: []string{"--format=human"}, want: hiddenCheckHuman},
		{name: "pipe explicit markdown", args: []string{"--format=md"}, want: hiddenCheckMarkdown},
		{name: "pipe deny info exits one", args: []string{"--format=json", "--deny=info"}, want: hiddenCheckJSON, exit: 1},
		{name: "pipe authority refusal exits two", missing: true, exit: 2, stderr: hiddenCheckMissingAuthority},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := hiddenCheckVault(t)
			hiddenCheckProducers(t, root)
			if tt.missing {
				if err := os.Remove(filepath.Join(root, schema.ContractRelPath)); err != nil {
					t.Fatalf("remove child contract: %v", err)
				}
			}
			args := append([]string{"-test.run=^TestHiddenCheckMain$", "--", "check", "--root=" + root}, tt.args...)
			cmd := exec.CommandContext(t.Context(), executable, args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "YOMIHON_TEST_HIDDEN745_MAIN=1")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if tt.exit == 0 {
				if err != nil {
					t.Fatalf("main(check) child error = %v; stderr = %q", err, stderr.String())
				}
			} else {
				exitErr, ok := errors.AsType[*exec.ExitError](err)
				if !ok || exitErr.ExitCode() != tt.exit {
					t.Fatalf("main(check) child error = %v, want exit %d; stderr = %q", err, tt.exit, stderr.String())
				}
			}
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tt.exit {
				t.Fatalf("main(check) process state = %v, want exit %d", cmd.ProcessState, tt.exit)
			}
			if diff := cmp.Diff(tt.stderr, stderr.String()); diff != "" {
				t.Fatalf("main(check) stderr differs (-want +got):\n%s", diff)
			}
			t.Log("HIDDEN745_MAIN_INVOKED")
			if diff := cmp.Diff(tt.want, stdout.String()); diff != "" {
				t.Errorf("main(check) stdout differs (-want +got):\n%s", diff)
			}
		})
	}
}

type hiddenCheckWriter struct {
	offered []byte
	written bytes.Buffer
	limit   int
	err     error
}

func (w *hiddenCheckWriter) Write(data []byte) (int, error) {
	w.offered = append(w.offered, data...)
	count := len(data)
	if w.limit >= 0 && count > w.limit {
		count = w.limit
	}
	n, err := w.written.Write(data[:count])
	if err != nil {
		return n, err
	}
	return n, w.err
}

func hiddenCheckWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", rel, err)
	}
}

func hiddenCheckReplace(t *testing.T, root, rel, old, replacement string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q for replacement: %v", rel, err)
	}
	if count := strings.Count(string(data), old); count != 1 {
		t.Fatalf("fixture %q replacement matches %d sites, want 1", rel, count)
	}
	hiddenCheckWrite(t, root, rel, strings.Replace(string(data), old, replacement, 1))
}
