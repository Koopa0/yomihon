package archlock

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	modzip "golang.org/x/mod/zip"
)

// TestTrackedPathsAreValidInAModuleZip holds the release path open: `go
// install pkg@version` builds the module zip from the tracked tree, and the zip
// refuses a whole module when a path is malformed or two paths collide under
// case folding, including their directory components. Invalid fixture names
// are supplied at test time instead of being tracked.
func TestTrackedPathsAreValidInAModuleZip(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is not available: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), "git", "-c", "core.quotepath=off", "ls-files", "-z")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("no git work tree to list: %v: %s", err, strings.TrimSpace(stderr.String()))
	}

	var files []modzip.File
	for path := range strings.SplitSeq(string(out), "\x00") {
		if path == "" {
			continue
		}
		files = append(files, moduleZipFile{name: path, filename: filepath.Join(repoRoot, filepath.FromSlash(path))})
	}
	if err := checkModuleZipFiles(files); err != nil {
		t.Errorf("tracked paths are invalid in a module zip, so `go install module@version` fails:\n%s", err)
	}
}

func checkModuleZipFiles(files []modzip.File) error {
	_, err := modzip.CheckFiles(files)
	return err
}

type moduleZipFile struct {
	name     string
	filename string
}

func (f moduleZipFile) Path() string                 { return f.name }
func (f moduleZipFile) Lstat() (os.FileInfo, error)  { return os.Lstat(f.filename) }
func (f moduleZipFile) Open() (io.ReadCloser, error) { return os.Open(f.filename) }

func TestModuleZipRejectsWholePathSet(t *testing.T) {
	t.Parallel()
	filename := filepath.Join(t.TempDir(), "fixture")
	if err := os.WriteFile(filename, []byte("fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "empty"},
		{name: "distinct", paths: []string{"notes/Alpha.md", "notes/Beta.md", "notes/a space.md"}},
		{name: "case-only files", paths: []string{"fixtures/CaseProbe.md", "fixtures/caseprobe.md"}, want: []string{`fixtures/caseprobe.md: case-insensitive file name collision: "fixtures/CaseProbe.md" and "fixtures/caseprobe.md"`}},
		{name: "case-only directories", paths: []string{"Fixtures/first.md", "fixtures/second.md"}, want: []string{`fixtures/second.md: case-insensitive file name collision: "Fixtures" and "fixtures"`}},
		{name: "file and directory", paths: []string{"fixtures/item", "fixtures/item/note.md"}, want: []string{`fixtures/item/note.md: entry "fixtures/item" is both a file and a directory`}},
		{name: "fullwidth colon", paths: []string{"fixtures/label\uff1a note.md"}, want: []string{"fixtures/label\uff1a note.md: malformed file path \"fixtures/label\uff1a note.md\": invalid char '\uff1a'"}},
		{name: "all collisions", paths: []string{"fixtures/A.md", "fixtures/a.md", "fixtures/B.md", "fixtures/b.md"}, want: []string{`fixtures/a.md: case-insensitive file name collision: "fixtures/A.md" and "fixtures/a.md"`, `fixtures/b.md: case-insensitive file name collision: "fixtures/B.md" and "fixtures/b.md"`}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files := make([]modzip.File, 0, len(tt.paths))
			for _, name := range tt.paths {
				files = append(files, moduleZipFile{name: name, filename: filename})
			}
			err := checkModuleZipFiles(files)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("checkModuleZipFiles(%q) error = %v, want nil", tt.paths, err)
				}
				return
			}
			invalid, ok := errors.AsType[modzip.FileErrorList](err)
			if !ok {
				t.Fatalf("checkModuleZipFiles(%q) error = %v, want module zip path errors", tt.paths, err)
			}
			got := make([]string, 0, len(invalid))
			for _, entry := range invalid {
				got = append(got, entry.Error())
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("checkModuleZipFiles(%q) errors mismatch (-want +got):\n%s", tt.paths, diff)
			}
		})
	}
}
