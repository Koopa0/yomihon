package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
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

	stdout, err := runPrepared(t.Context(), t, "exists", root, "Pinned",
		actionHooks{afterScan: func() {
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
		}}, nil)
	if err != nil {
		t.Fatalf("exists on a pinned root error = %v", err)
	}
	var report existsReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		t.Fatalf("Unmarshal(exists output) error = %v; output=%q", err, stdout)
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

	stdout, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "Caf\u00e9", Format: FormatJSON})
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

// unreadable makes path unreadable for the rest of the test and reports
// whether the filesystem honoured it. A container running as root, and a
// filesystem that does not carry permissions, both ignore the request; a test
// that assumed otherwise would assert against a vault it never broke.
func unreadable(tb testing.TB, path string) bool {
	tb.Helper()
	if err := os.Chmod(path, 0); err != nil {
		tb.Fatalf("Chmod(%q, 0) error = %v", path, err)
	}
	tb.Cleanup(func() {
		if err := os.Chmod(path, 0o700); err != nil && !os.IsNotExist(err) { // #nosec G302 -- a directory needs owner execute permission so TempDir cleanup can traverse it
			tb.Errorf("restore %q: %v", path, err)
		}
	})
	info, err := os.Stat(path)
	if err != nil {
		tb.Fatalf("Stat(%q) error = %v", path, err)
	}
	if info.IsDir() {
		_, err = os.ReadDir(path)
	} else {
		_, err = os.ReadFile(path) // #nosec G304 -- the path is this test's own temporary file
	}
	return err != nil
}

// TestScanStoppedNamesOnlyAPathItCanAskAbout covers the failures a walk reports
// without naming one file inside the vault. The refusal for a withheld file and
// the refusal for a path the contract cannot be asked about are different
// sentences on purpose: answering the withheld one for an unreadable vault root
// would tell an operator whose contract withholds nothing that his own folder
// is private, and send him to a privacy policy to fix a permission.
func TestScanStoppedNamesOnlyAPathItCanAskAbout(t *testing.T) {
	t.Parallel()

	authority := testScanAuthority(t, "Diary")
	tests := []struct {
		name string
		path string
		want error
	}{
		{name: "the folder itself", path: ".", want: errVaultScan},
		{name: "outside the vault", path: "/etc/passwd", want: errVaultScan},
		{name: "climbing out", path: "../elsewhere", want: errVaultScan},
		{name: "nothing at all", path: "", want: errVaultScan},
		{name: "a withheld directory", path: "Diary/2026-08-27.md", want: errWithheldUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cause := fmt.Errorf("list pinned vault: %w", &fs.PathError{
				Op: "openat", Path: tt.path, Err: fs.ErrPermission,
			})
			if got := scanStopped(cause, authority); !errors.Is(got, tt.want) {
				t.Errorf("scanStopped(path %q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}

	// The control: a describable path is named, so an empty answer above cannot
	// pass by refusing everything.
	cause := fmt.Errorf("list pinned vault: %w", &fs.PathError{
		Op: "openat", Path: "Concepts/blocked", Err: fs.ErrPermission,
	})
	got := scanStopped(cause, authority)
	if errors.Is(got, errVaultScan) || errors.Is(got, errWithheldUnreadable) {
		t.Fatalf("scanStopped(describable path) = %v, want the path named", got)
	}
	for _, part := range []string{"Concepts/blocked", "permission denied"} {
		if !strings.Contains(got.Error(), part) {
			t.Errorf("scanStopped(describable path) = %v, want it to name %q", got, part)
		}
	}
}

// TestJudgeNamesTheFileItCouldNotRead holds the diagnostic the reading face has
// always given and this one withheld. One file yomihon cannot read ends the
// judgement, correctly — a report over a partial corpus would answer about
// ground it never read — but the operator was told only that a scan failed, so
// on a folder of any size there was nowhere to start looking.
func TestJudgeNamesTheFileItCouldNotRead(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\n")
	write(t, root, "Notes/bad.md", "---\ntitle: Unreadable\n---\n")
	if !unreadable(t, filepath.Join(root, "Notes", "bad.md")) {
		t.Skip("filesystem permissions do not make the note unreadable for this process")
	}

	got := refuse(t.Context(), t, "check", root).Error()
	if !strings.HasPrefix(got, "vault scan failed: ") {
		t.Errorf("check error = %q, want the scan refusal", got)
	}
	for _, part := range []string{"Notes/bad.md", "permission denied"} {
		if !strings.Contains(got, part) {
			t.Errorf("check error = %q, want it to name %q", got, part)
		}
	}
}

// TestJudgeWithholdsAnUnreadableFileUnderAPrivateDirectory is the other side of
// the same repair. The refusal has to be said, since the command is exiting
// without an answer, but the file's name and the reason it could not be read
// are description of ground the contract closed, and a refusal that varied with
// either would answer questions about what is in there. The sentence is fixed,
// so every such file and every cause produce the same line.
func TestJudgeWithholdsAnUnreadableFileUnderAPrivateDirectory(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, []string{"Diary"})
	write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\n")
	write(t, root, "Diary/2026-08-27.md", "---\ntitle: Private\n---\n")
	if !unreadable(t, filepath.Join(root, "Diary", "2026-08-27.md")) {
		t.Skip("filesystem permissions do not make the note unreadable for this process")
	}

	got := refuse(t.Context(), t, "check", root).Error()
	if want := errWithheldUnreadable.Error(); got != want {
		t.Errorf("check error = %q, want %q", got, want)
	}
	for _, leaked := range []string{"Diary", "2026-08-27", "permission denied"} {
		if strings.Contains(got, leaked) {
			t.Errorf("check error = %q, which describes withheld ground with %q", got, leaked)
		}
	}
}

// TestJudgeWithholdsAPrivateDirectoryTheScanCouldNotEnter carries the same
// refusal to the other place a read stops, where the path has to be recovered
// from the failure rather than taken from a scan entry. The directory is
// spelled on disk in the decomposed form a Mac keyboard produces and in the
// contract in the composed form, which is the same directory under a different
// string: asking the contract about the spelling the walk reported, without
// composing it first, would answer about a directory nobody declared and hand
// the private name to the caller.
func TestJudgeWithholdsAPrivateDirectoryTheScanCouldNotEnter(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, []string{"だ体"})
	write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\n")
	decomposed := norm.NFD.String("だ体")
	write(t, root, decomposed+"/private.md", "---\ntitle: Private\n---\n")
	if !unreadable(t, filepath.Join(root, decomposed)) {
		t.Skip("filesystem permissions do not make the directory unreadable for this process")
	}

	got := refuse(t.Context(), t, "check", root).Error()
	if want := errWithheldUnreadable.Error(); got != want {
		t.Errorf("check error = %q, want %q", got, want)
	}
	for _, leaked := range []string{decomposed, "だ体", "private.md", "permission denied"} {
		if strings.Contains(got, leaked) {
			t.Errorf("check error = %q, which describes withheld ground with %q", got, leaked)
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

					stdout, err := runPrepared(t.Context(), t, command.name, root, "Target",
						actionHooks{afterScan: func() { swap.swap(t, root) }}, nil)
					if err == nil {
						t.Errorf("%s produced %d bytes from a vault swapped under it", command.name, len(stdout))
					}
					if stdout != nil {
						t.Errorf("%s produced a payload alongside its refusal: %q", command.name, stdout)
					}
					// The refusal names the file it stopped on. An operator
					// told only that a scan failed, on a folder of any size,
					// has nowhere to start looking.
					want := "vault scan failed: Notes/Target.md: vault entry no longer names the observed file"
					if err != nil && err.Error() != want {
						t.Errorf("%s error = %q, want %q", command.name, err, want)
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
			_, err := runPrepared(t.Context(), t, tt.command, root, "A", actionHooks{
				afterScan: func() { scans++ },
				afterNoteRead: func(path string) {
					reads[path]++
				},
			}, nil)
			if err != nil {
				t.Fatalf("%s failed: %v", tt.command, err)
			}
			if scans != 1 {
				t.Errorf("%s scans = %d, want 1", tt.command, scans)
			}
			wantReads := map[string]int{"Notes/A.md": 1, "Notes/B.md": 1}
			if diff := cmp.Diff(wantReads, reads); diff != "" {
				t.Errorf("%s note reads mismatch (-want +got):\n%s", tt.command, diff)
			}
		})
	}
}

func TestCheckDiskReferencesUseCapturedMembership(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "Notes/Source.md", "[late](../Attachments/late.md)\n")

	stdout, err := runPrepared(t.Context(), t, "check", root, "",
		actionHooks{afterScan: func() {
			write(t, root, "Attachments/late.md", "arrived after the scan\n")
		}}, nil)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if !bytes.Contains(stdout, []byte(`"rule_id":"link.broken.path"`)) ||
		!bytes.Contains(stdout, []byte(`"target":"../Attachments/late.md"`)) {
		t.Errorf("check output = %q, want captured-missing path finding", stdout)
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
				case "vaultfs.Open", "schema.LoadReader":
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
		"vaultfs.Open":      1,
		"schema.LoadReader": 1,
		"ScanComplete":      1,
		"ReadFile":          1,
		"os.ReadFile":       1,
	}
	if diff := cmp.Diff(wantCounts, counts); diff != "" {
		t.Errorf("rooted read call sites mismatch (-want +got):\n%s", diff)
	}
}

// TestAScanStopsWhenTheCallerGivesUp holds the one thing taking a context is
// for. A whole-vault scan is the slow part of these three commands, and before
// the context reached this far the library built its own, so a caller who had
// given up waiting could not say so and the walk ran to the end regardless.
//
// The refusal is deliberately the ordinary scan refusal rather than a new one:
// what the caller needs to know is that no verdict was reached, and a cancelled
// run has read part of a vault, which is exactly the partial corpus every other
// refusal here exists to avoid answering from.
func TestAScanStopsWhenTheCallerGivesUp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "Notes/one.md", "---\ntitle: One\n---\nbody\n")

	if _, err := Check(t.Context(), root); err != nil {
		t.Fatalf("Check() on a live context error = %v; the fixture has to succeed or the cancelled case proves nothing", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	findings, err := Check(ctx, root)
	if err == nil {
		t.Fatalf("Check() on a cancelled context returned %d findings and no error", len(findings))
	}
	if findings != nil {
		t.Errorf("Check() returned %d findings from a scan that was stopped", len(findings))
	}
}
