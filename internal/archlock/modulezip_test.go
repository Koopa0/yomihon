package archlock

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
	if _, err := modzip.CheckFiles(files); err != nil {
		t.Errorf("tracked paths are invalid in a module zip, so `go install module@version` fails:\n%s", err)
	}
}

type moduleZipFile struct {
	name     string
	filename string
}

func (f moduleZipFile) Path() string                 { return f.name }
func (f moduleZipFile) Lstat() (os.FileInfo, error)  { return os.Lstat(f.filename) }
func (f moduleZipFile) Open() (io.ReadCloser, error) { return os.Open(f.filename) }
