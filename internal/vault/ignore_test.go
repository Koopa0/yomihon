package vault

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A .gitignore is the one place a folder already says which of its files are
// not for reading, so the scan takes it at its word. These cases pin the parts
// of git's rules a real file leans on; each row is one line of a .gitignore and
// one path it is asked about.
func TestIgnoreRulesMatchTheWayGitReadsThem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		rules string
		path  string
		isDir bool
		want  bool
	}{
		{"a folder name with a slash matches folders at any depth", "__pycache__/\n", "tools/figures/__pycache__", true, true},
		{"a folder rule does not match a file of that name", "__pycache__/\n", "tools/__pycache__", false, false},
		{"a bare name matches at the root", "drafts\n", "drafts", true, true},
		{"a bare name matches files too", "drafts\n", "Notes/drafts", false, true},
		{"a star matches within one name", "*.original.txt\n", "work/a.original.txt", false, true},
		{"a star leaves other extensions alone", "*.original.txt\n", "work/a.txt", false, false},
		{"a star does not cross a slash", "a*b\n", "a/b", false, false},
		{"a leading slash ties the rule to the root", "/build\n", "build", true, true},
		{"a leading slash does not reach deeper", "/build\n", "src/build", true, false},
		{"a slash in the middle ties the rule to the root", "docs/drafts\n", "docs/drafts", true, true},
		{"a slash in the middle does not reach deeper", "docs/drafts\n", "site/docs/drafts", true, false},
		{"a question mark is one character", "note?.md\n", "note1.md", false, true},
		{"a question mark is not zero characters", "note?.md\n", "note.md", false, false},
		{"a set matches its members", "[ab].md\n", "a.md", false, true},
		{"a set leaves others alone", "[ab].md\n", "c.md", false, false},
		{"a negated set matches what it does not name", "[!ab].md\n", "c.md", false, true},
		{"a range in a set", "v[0-9].md\n", "v7.md", false, true},
		{"a leading double star matches at every depth", "**/cache\n", "a/b/cache", true, true},
		{"a leading double star matches the root too", "**/cache\n", "cache", true, true},
		{"a trailing double star holds what is inside", "logs/**\n", "logs/today.md", false, true},
		{"a trailing double star does not hold the folder itself", "logs/**\n", "logs", true, false},
		{"a middle double star spans any number of folders", "a/**/b\n", "a/x/y/b", false, true},
		{"a middle double star spans none", "a/**/b\n", "a/b", false, true},
		{"a later rule re-includes", "*.md\n!keep.md\n", "keep.md", false, false},
		{"a re-included name is re-included at depth", "*.md\n!keep.md\n", "Notes/keep.md", false, false},
		{"the rest of the rule's matches stay left out", "*.md\n!keep.md\n", "other.md", false, true},
		{"the last matching rule decides", "x\n!x\nx\n", "x", false, true},
		{"a comment is not a rule", "# drafts\n", "drafts", true, false},
		{"a blank line is not a rule", "\n\n", "drafts", true, false},
		{"an escaped hash is a name", "\\#notes.md\n", "#notes.md", false, true},
		{"an escaped bang is a name", "\\!important.md\n", "!important.md", false, true},
		{"trailing spaces are dropped", "drafts   \n", "drafts", true, true},
		{"an escaped trailing space is kept", "a\\ \n", "a ", false, true},
		{"a Windows line ending is not part of the name", "drafts\r\n", "drafts", true, true},
		{"a file with no final newline still counts its last line", "drafts", "drafts", true, true},
		{"names are compared in the composed form", "Cafe\u0301/\n", "Caf\u00e9", true, true},
		{"names are case sensitive", "Build/\n", "build", true, false},
		{"a bracket that never closes is a plain character", "[abc\n", "a", false, false},
		{"a bracket that never closes matches itself", "[abc\n", "[abc", false, true},
		{"a lone slash names nothing", "/\n", "a", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rules := parseIgnoreRules([]byte(tt.rules))
			if got := rules.ignored(NormalizeNFC(tt.path), tt.isDir); got != tt.want {
				t.Errorf("rules %q ignored(%q, dir=%t) = %t, want %t", tt.rules, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

// Everything the scan leaves out by its own rules, in one vault: the root's
// .gitignore, a node_modules at any depth, and a folder carrying pyvenv.cfg.
// The neighbours that look like them but are not — a folder called venv with no
// marker, a pyvenv.cfg that is a folder, and a .gitignore inside a subfolder —
// are scanned, because each of them is a note somebody wrote.
func TestScanLeavesOutWhatTheVaultAndTheBuiltInRulesName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, body := range map[string]string{
		".gitignore":                          "drafts/\n*.log.md\n!keep.log.md\n",
		"Notes/a.md":                          "a",
		"Notes/x.log.md":                      "ignored by pattern",
		"keep.log.md":                         "re-included",
		"drafts/b.md":                         "ignored folder",
		"Notes/drafts/c.md":                   "ignored folder at depth",
		"Notes/nested/.gitignore":             "*.md\n",
		"Notes/nested/n.md":                   "a nested .gitignore is not read",
		"node_modules/pkg/README.md":          "installed copy",
		"Notes/node_modules/pkg/README.md":    "installed copy at depth",
		"env/pyvenv.cfg":                      "home = /usr/bin",
		"env/lib/site-packages/pip/README.md": "a virtual environment's own documents",
		"venv/notes.md":                       "a folder called venv with no marker is just a folder",
		"odd/pyvenv.cfg/inner.md":             "a folder called pyvenv.cfg is not the marker",
	} {
		writeReaderFixture(t, root, rel, body)
	}
	wantFiles := []string{
		"Notes/a.md",
		"Notes/nested/n.md",
		"keep.log.md",
		"odd/pyvenv.cfg/inner.md",
		"venv/notes.md",
	}
	wantLeftOut := []string{"Notes/drafts", "Notes/node_modules", "Notes/x.log.md", "drafts", "env", "node_modules"}
	beyond := []string{
		"drafts/b.md", "drafts/not-written-yet.md", "Notes/drafts/c.md", "Notes/x.log.md",
		"node_modules/pkg/README.md", "Notes/node_modules/pkg/README.md",
		"env/lib/site-packages/pip/README.md", "env/pyvenv.cfg", "env",
	}
	within := []string{"Notes/a.md", "Notes/missing.md", "Notes/nested/n.md", "keep.log.md", "venv/notes.md", "odd/pyvenv.cfg/inner.md", "elsewhere/gone.md"}

	reader := openTestReader(t, root)
	for name, scan := range map[string]func() (Scan, error){
		"available": func() (Scan, error) { return reader.ScanAvailable(t.Context()) },
		"complete":  func() (Scan, error) { return reader.ScanComplete(t.Context()) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := scan()
			if err != nil {
				t.Fatalf("scan error = %v", err)
			}
			var files []string
			for _, entry := range got.Files() {
				files = append(files, entry.Path())
			}
			if diff := cmp.Diff(wantFiles, files); diff != "" {
				t.Errorf("scanned files (-want +got):\n%s", diff)
			}
			var left []string
			for p := range got.state.leftOut {
				left = append(left, p)
			}
			slices.Sort(left)
			if diff := cmp.Diff(wantLeftOut, left); diff != "" {
				t.Errorf("paths left out by rule (-want +got):\n%s", diff)
			}
			for _, p := range beyond {
				if !got.OutsideScan(p) {
					t.Errorf("OutsideScan(%q) = false, want true: the scan did not look there", p)
				}
				if got.Contains(p) {
					t.Errorf("Contains(%q) = true, want false", p)
				}
			}
			for _, p := range within {
				if got.OutsideScan(p) {
					t.Errorf("OutsideScan(%q) = true, want false: the scan looked there", p)
				}
			}
			if problems := got.Problems(); len(problems) != 0 {
				t.Errorf("Problems() = %v, want none", problems)
			}
			if skipped := got.Skipped(); len(skipped) != 0 {
				t.Errorf("Skipped() = %v, want none: a left-out path is a rule, not a symlink", skipped)
			}
		})
	}
}

// Promise 1: the scan reads the vault only through the pinned root. A
// .gitignore that is a symbolic link would otherwise hand the vault's contents
// to whatever file the link points at, so it is not followed: the vault has no
// rules, and every note stays in the scan.
func TestScanDoesNotFollowASymlinkedIgnoreFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "elsewhere.ignore")
	if err := os.WriteFile(outside, []byte("*.md\n"), 0o600); err != nil {
		t.Fatalf("write outside rules: %v", err)
	}
	writeReaderFixture(t, root, "Notes/a.md", "a")
	if err := os.Symlink(outside, filepath.Join(root, ".gitignore")); err != nil {
		t.Fatalf("symlink .gitignore: %v", err)
	}

	reader := openTestReader(t, root)
	for _, scan := range []func() (Scan, error){
		func() (Scan, error) { return reader.ScanAvailable(t.Context()) },
		func() (Scan, error) { return reader.ScanComplete(t.Context()) },
	} {
		got, err := scan()
		if err != nil {
			t.Fatalf("scan error = %v", err)
		}
		if files := got.Files(); len(files) != 1 || files[0].Path() != "Notes/a.md" {
			t.Errorf("scanned files = %v, want Notes/a.md: rules from outside the vault were applied", files)
		}
		if problems := got.Problems(); len(problems) != 0 {
			t.Errorf("Problems() = %v, want none", problems)
		}
	}
}

// A .gitignore that is a folder is no rule file, and a vault without one has
// no rules. Neither is an error: the file is optional.
func TestScanWithoutARegularIgnoreFileHasNoRules(t *testing.T) {
	t.Parallel()
	for name, setup := range map[string]func(root string){
		"absent":   func(string) {},
		"a folder": func(root string) { writeReaderFixture(t, root, ".gitignore/inner", "*.md\n") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeReaderFixture(t, root, "a.md", "a")
			setup(root)
			scan, err := openTestReader(t, root).ScanComplete(t.Context())
			if err != nil {
				t.Fatalf("ScanComplete() error = %v", err)
			}
			if files := scan.Files(); len(files) != 1 || files[0].Path() != "a.md" {
				t.Errorf("scanned files = %v, want a.md", files)
			}
		})
	}
}

// Promise 1: scanning a vault that has rules, a virtual environment and an
// installed-packages folder changes nothing in it — not a name, a byte, a mode
// or a modification time, including the rule file's own.
func TestScanLeavesTheVaultUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, body := range map[string]string{
		".gitignore":               "drafts/\n",
		"Notes/a.md":               "a",
		"drafts/b.md":              "b",
		"node_modules/pkg/x.md":    "x",
		"env/pyvenv.cfg":           "home = /usr/bin",
		"env/lib/README.md":        "readme",
		"Notes/figure.png":         "png",
		"Notes/.hidden/secret.txt": "secret",
	} {
		writeReaderFixture(t, root, rel, body)
	}
	before := vaultFingerprint(t, root)

	reader := openTestReader(t, root)
	if _, err := reader.ScanComplete(t.Context()); err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	if _, err := reader.ScanAvailable(t.Context()); err != nil {
		t.Fatalf("ScanAvailable() error = %v", err)
	}
	if diff := cmp.Diff(before, vaultFingerprint(t, root)); diff != "" {
		t.Errorf("the vault changed under a scan (-before +after):\n%s", diff)
	}
}

// vaultFingerprint lists every path under root with its mode, size,
// modification time and content, so a write of any kind shows as a difference.
func vaultFingerprint(t *testing.T, root string) []string {
	t.Helper()
	pinned, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", root, err)
	}
	defer func() {
		if closeErr := pinned.Close(); closeErr != nil {
			t.Errorf("close root: %v", closeErr)
		}
	}()
	var lines []string
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		line := rel + " " + info.Mode().String() + " " + info.ModTime().UTC().Format("2006-01-02T15:04:05.000000000Z")
		if info.Mode().IsRegular() {
			body, readErr := pinned.ReadFile(rel)
			if readErr != nil {
				return readErr
			}
			line += " " + string(body)
		}
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatalf("fingerprint %s: %v", root, err)
	}
	return lines
}

// A rule file the reader cannot read is a fact about the folder, not a reason
// to guess: a complete scan stops, as it does for any path it cannot observe,
// and an available scan records the file as a problem and carries on without
// rules.
func TestScanReportsAnIgnoreFileItCannotRead(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a root process reads a file whatever its mode")
	}
	root := t.TempDir()
	writeReaderFixture(t, root, "a.md", "a")
	writeReaderFixture(t, root, ".gitignore", "a.md\n")
	if err := os.Chmod(filepath.Join(root, ".gitignore"), 0); err != nil {
		t.Fatalf("chmod .gitignore: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(filepath.Join(root, ".gitignore"), 0o600); err != nil {
			t.Errorf("restore .gitignore mode: %v", err)
		}
	})

	reader := openTestReader(t, root)
	if _, err := reader.ScanComplete(t.Context()); err == nil {
		t.Error("ScanComplete() error = nil, want the unreadable .gitignore to stop it")
	}
	scan, err := reader.ScanAvailable(t.Context())
	if err != nil {
		t.Fatalf("ScanAvailable() error = %v", err)
	}
	if problems := scan.Problems(); len(problems) != 1 || problems[0].Path() != ".gitignore" {
		t.Errorf("Problems() = %v, want the .gitignore named", problems)
	}
	if files := scan.Files(); len(files) != 1 || files[0].Path() != "a.md" {
		t.Errorf("scanned files = %v, want a.md: with no readable rules nothing is left out", files)
	}
}

// A BOM opens some editors' files, and a file too long to be a list of folders
// is read up to the last whole line.
func TestIgnoreFileIsReadThroughABOMAndUpToItsLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, ".gitignore", "\xEF\xBB\xBFdrafts/\n"+strings.Repeat("x", maxIgnoreBytes))
	writeReaderFixture(t, root, "drafts/b.md", "b")
	writeReaderFixture(t, root, "Notes/a.md", "a")

	scan, err := openTestReader(t, root).ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	var files []string
	for _, entry := range scan.Files() {
		files = append(files, entry.Path())
	}
	if diff := cmp.Diff([]string{"Notes/a.md"}, files); diff != "" {
		t.Errorf("scanned files (-want +got):\n%s", diff)
	}
}

// A rescan keeps the generation it has while the files are the same, so a scan
// that answers from what the walk left out has to count as changed when that
// changes, even though no file did. An empty folder is the case: it holds no
// file for the file list to lose when its rule is removed.
func TestScanSameFilesNoticesWhatTheWalkLeavesOutChanging(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/a.md", "a")
	writeReaderFixture(t, root, ".gitignore", "drafts/\n")
	if err := os.Mkdir(filepath.Join(root, "drafts"), 0o750); err != nil {
		t.Fatalf("mkdir drafts: %v", err)
	}
	reader := openTestReader(t, root)
	first, err := reader.ScanAvailable(t.Context())
	if err != nil {
		t.Fatalf("ScanAvailable() error = %v", err)
	}
	if !first.OutsideScan("drafts/missing.md") {
		t.Fatal("the empty ignored folder was not left out; the case below proves nothing")
	}

	again, err := reader.ScanAvailable(t.Context())
	if err != nil {
		t.Fatalf("ScanAvailable() error = %v", err)
	}
	if !first.SameFiles(again) {
		t.Error("SameFiles(unchanged vault) = false, want true")
	}

	writeReaderFixture(t, root, ".gitignore", "*.tmp\n")
	freed, err := reader.ScanAvailable(t.Context())
	if err != nil {
		t.Fatalf("ScanAvailable() error = %v", err)
	}
	if freed.OutsideScan("drafts/missing.md") {
		t.Fatal("the folder is still left out after its rule was removed")
	}
	if first.SameFiles(freed) {
		t.Error("SameFiles(rule removed, files unchanged) = true, want false: the scan would keep answering that drafts/ is left out")
	}
	if freed.SameFiles(first) {
		t.Error("SameFiles(rule added, files unchanged) = true, want false")
	}
}
