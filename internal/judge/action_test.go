package judge

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestJudgeActionPinsOneRootAcrossRenameAndReplacement(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	writeTestContract(t, root, nil)
	write(t, root, "Notes/Pinned.md", "---\ntitle: Pinned\n---\n")
	moved := filepath.Join(parent, "selected-vault")

	var stdout, stderr bytes.Buffer
	exit := runCommand(
		"exists",
		[]string{"--root=" + root, "--format=json", "Pinned"},
		&stdout,
		&stderr,
		false,
		commandHooks{afterScan: func() {
			if err := os.Rename(root, moved); err != nil {
				t.Fatalf("Rename(root) error = %v", err)
			}
			writeTestContract(t, root, nil)
			contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
			contract, err := os.ReadFile(contractPath) // #nosec G304 -- contractPath is rooted in this test's private TempDir
			if err != nil {
				t.Fatalf("ReadFile(replacement contract) error = %v", err)
			}
			if err := os.WriteFile(contractPath, append(contract, '\n'), 0o600); err != nil { // #nosec G703 -- contractPath is rooted in this test's private TempDir
				t.Fatalf("WriteFile(replacement contract) error = %v", err)
			}
			write(t, root, "Notes/Replacement.md", "---\ntitle: Replacement\n---\n")
		}},
	)
	if exit != 0 {
		t.Fatalf("runCommand(exists) exit = %d, want 0; stderr=%q", exit, stderr.String())
	}
	var report existsReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("Unmarshal(exists output) error = %v; output=%q", err, stdout.Bytes())
	}
	want := existsReport{
		Query: "Pinned",
		Matches: []existsMatch{{Path: "Notes/Pinned.md", Field: "filename", Value: "Pinned"}, {
			Path: "Notes/Pinned.md", Field: "title", Value: "Pinned",
		}},
	}
	if diff := cmp.Diff(want, report); diff != "" {
		t.Errorf("exists report mismatch (-want +got):\n%s", diff)
	}
}

func TestJudgeActionReadsNFDEntryThroughCapturedRawSpelling(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, nil)
	canonical := "Notes/Caf\u00e9.md"
	raw := norm.NFD.String(canonical)
	if raw == canonical {
		t.Fatal("NFD fixture did not decompose")
	}
	write(t, root, raw, "---\ntitle: Caf\u00e9\n---\n")

	stdout, exit, err := RunExists(&ExistsOptions{Root: root, Name: "Caf\u00e9", Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunExists(NFD entry) error = %v", err)
	}
	if exit != 0 {
		t.Fatalf("RunExists(NFD entry) exit = %d, want 0", exit)
	}
	var report existsReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		t.Fatalf("Unmarshal(exists output) error = %v", err)
	}
	if len(report.Matches) == 0 {
		t.Fatal("RunExists(NFD entry) returned no matches")
	}
	for _, match := range report.Matches {
		if match.Path != canonical {
			t.Errorf("RunExists(NFD entry) path = %q, want canonical %q", match.Path, canonical)
		}
	}
}

func TestJudgeActionRejectsSourceSwapWithoutPayload(t *testing.T) {
	swaps := []struct {
		name string
		swap func(*testing.T, string)
	}{
		{
			name: "leaf",
			swap: func(t *testing.T, root string) {
				t.Helper()
				path := filepath.Join(root, "Notes", "Target.md")
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatalf("Rename(leaf) error = %v", err)
				}
				write(t, root, "Notes/Target.md", "---\ntitle: Replacement\n---\n")
			},
		},
		{
			name: "parent",
			swap: func(t *testing.T, root string) {
				t.Helper()
				parent := filepath.Join(root, "Notes")
				if err := os.Rename(parent, parent+"-old"); err != nil {
					t.Fatalf("Rename(parent) error = %v", err)
				}
				write(t, root, "Notes/Target.md", "---\ntitle: Replacement\n---\n")
			},
		},
	}
	commands := []struct {
		name string
		args func(string) []string
	}{
		{name: "check", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{name: "coverage", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{name: "exists", args: func(root string) []string { return []string{"--root=" + root, "--format=json", "Target"} }},
	}
	for _, swap := range swaps {
		t.Run(swap.name, func(t *testing.T) {
			for _, command := range commands {
				t.Run(command.name, func(t *testing.T) {
					root := t.TempDir()
					writeTestContract(t, root, nil)
					write(t, root, "Notes/Target.md", "---\ntitle: Target\n---\n")

					var stdout, stderr bytes.Buffer
					exit := runCommand(
						command.name,
						command.args(root),
						&stdout,
						&stderr,
						false,
						commandHooks{afterScan: func() { swap.swap(t, root) }},
					)
					if exit != 2 {
						t.Errorf("runCommand(%q) exit = %d, want 2", command.name, exit)
					}
					if stdout.Len() != 0 {
						t.Errorf("runCommand(%q) stdout = %q, want empty", command.name, stdout.String())
					}
					if want := "yomihon: vault scan failed\n"; stderr.String() != want {
						t.Errorf("runCommand(%q) stderr = %q, want %q", command.name, stderr.String(), want)
					}
				})
			}
		})
	}
}

func TestJudgeCommandsScanOnceAndReadEachMarkdownOnce(t *testing.T) {
	tests := []struct {
		command string
		args    func(string) []string
	}{
		{command: "check", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{command: "coverage", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{command: "exists", args: func(root string) []string { return []string{"--root=" + root, "--format=json", "A"} }},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			root := t.TempDir()
			writeTestContract(t, root, nil)
			write(t, root, "Notes/A.md", "---\ntitle: A\n---\n")
			write(t, root, "Notes/B.md", "---\ntitle: B\n---\n")

			scans := 0
			reads := make(map[string]int)
			var stdout, stderr bytes.Buffer
			exit := runCommand(
				tt.command,
				tt.args(root),
				&stdout,
				&stderr,
				false,
				commandHooks{
					afterScan: func() { scans++ },
					afterNoteRead: func(path string) {
						reads[path]++
					},
				},
			)
			if exit == 2 {
				t.Fatalf("runCommand(%q) failed: %s", tt.command, stderr.String())
			}
			if scans != 1 {
				t.Errorf("runCommand(%q) scans = %d, want 1", tt.command, scans)
			}
			wantReads := map[string]int{"Notes/A.md": 1, "Notes/B.md": 1}
			if diff := cmp.Diff(wantReads, reads); diff != "" {
				t.Errorf("runCommand(%q) note reads mismatch (-want +got):\n%s", tt.command, diff)
			}
		})
	}
}

func TestCheckDiskReferencesUseCapturedMembership(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "Notes/Source.md", "[late](../Attachments/late.md)\n")

	var stdout, stderr bytes.Buffer
	exit := runCommand(
		"check",
		[]string{"--root=" + root, "--format=json"},
		&stdout,
		&stderr,
		false,
		commandHooks{afterScan: func() {
			write(t, root, "Attachments/late.md", "arrived after the scan\n")
		}},
	)
	if exit == 2 {
		t.Fatalf("runCommand(check) failed: %s", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"rule_id":"link.broken.path"`)) ||
		!bytes.Contains(stdout.Bytes(), []byte(`"target":"../Attachments/late.md"`)) {
		t.Errorf("runCommand(check) output = %q, want captured-missing path finding", stdout.String())
	}
}

func TestJudgeProductionUsesOneRootedReadPath(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.) error = %v", err)
	}
	counts := make(map[string]int)
	var forbidden []string
	forbiddenQualified := map[string]bool{
		"vault.List": true, "vault.ListStrict": true, "vault.ReadNote": true,
		"schema.Load": true, "schema.LoadFile": true,
		"os.Open": true, "os.OpenFile": true, "os.OpenRoot": true,
		"os.ReadDir": true, "os.Stat": true, "os.Lstat": true,
		"os.Readlink": true, "os.DirFS": true,
		"filepath.Join": true, "filepath.FromSlash": true,
		"filepath.Walk": true, "filepath.WalkDir": true, "filepath.Glob": true,
		"fs.WalkDir": true,
	}
	forbiddenMethods := map[string]bool{
		"Entries": true, "Lookup": true, "ReadPrefix": true,
		"Refresh": true, "ScanAvailable": true,
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if parseErr != nil {
			t.Fatalf("ParseFile(%q) error = %v", entry.Name(), parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			var owner *ast.Ident
			if identifier, ok := selector.X.(*ast.Ident); ok {
				owner = identifier
			}
			if owner != nil {
				qualified := owner.Name + "." + selector.Sel.Name
				switch qualified {
				case "vault.Open", "schema.LoadReader":
					counts[qualified]++
				case "os.ReadFile":
					counts[qualified]++
					if entry.Name() != "command.go" {
						forbidden = append(forbidden, entry.Name()+":"+qualified)
					}
				default:
					if forbiddenQualified[qualified] {
						forbidden = append(forbidden, entry.Name()+":"+qualified)
					}
				}
			}
			if forbiddenMethods[selector.Sel.Name] {
				forbidden = append(forbidden, entry.Name()+":method."+selector.Sel.Name)
			}
			switch selector.Sel.Name {
			case "ScanComplete":
				counts[selector.Sel.Name]++
			case "ReadFile":
				if owner == nil || owner.Name != "os" {
					counts[selector.Sel.Name]++
				}
			}
			return true
		})
	}
	slices.Sort(forbidden)
	if len(forbidden) != 0 {
		t.Errorf("production path bypasses = %v, want none", forbidden)
	}
	wantCounts := map[string]int{
		"vault.Open":        1,
		"schema.LoadReader": 1,
		"ScanComplete":      1,
		"ReadFile":          1,
		"os.ReadFile":       1,
	}
	if diff := cmp.Diff(wantCounts, counts); diff != "" {
		t.Errorf("rooted read call sites mismatch (-want +got):\n%s", diff)
	}
}
