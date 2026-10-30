package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

const (
	excludeDirsSentinel = "excludedirs-lock-sentinel"
	keptDirsSentinel    = "excludedirs-kept-sentinel"
)

// TestExcludedDirectoriesAreUnseenLikeHiddenOnes composes the production site
// over a vault whose contract excludes courses/*/lab and holds the directory
// to what a dot-directory gets: not shelved, not searched, not judged, not
// served, and answered exactly as a hidden path is. The sibling modules
// directory is read as usual.
func TestExcludedDirectoriesAreUnseenLikeHiddenOnes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	contract := strings.Replace(skipBasenameLockContract,
		`skip_basenames = ["README.md"]`, "skip_basenames = [\"README.md\"]\nexclude_dirs = [\"courses/*/lab\"]", 1)
	files := map[string]string{
		schema.ContractRelPath:             contract,
		"Notes/Kept.md":                    "---\ntitle: Kept\ntype: note\nstatus: draft\n---\nKept body.\n",
		"courses/C1/lab/main.go":           "package main // " + excludeDirsSentinel + "\n",
		"courses/C2/lab/main.go":           "package main\n",
		"courses/C1/lab/Inside.md":         "---\ntitle: Inside\ntype: note\nstatus: draft\n---\n" + excludeDirsSentinel + "\n",
		"courses/C1/modules/Lesson.md":     "---\ntitle: Lesson\ntype: note\nstatus: draft\n---\n" + keptDirsSentinel + "\n",
		"courses/C1/.hidden/main.go":       "package main\n",
		"courses/C1/modules/lab/Shown.txt": "shown\n",
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil { // #nosec G703 -- fixture path under t.TempDir
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	// The folder browser lists one directory per page, so the excluded
	// directory is looked for in its parent's listing and asked for itself.
	folders := readingPage(t, site, "/folders/courses/C1")
	if !strings.Contains(folders, `href="/folders/courses/C1/modules"`) {
		t.Errorf("GET /folders/courses/C1 lost the sibling modules directory; page = %q", folders)
	}
	if strings.Contains(folders, "/folders/courses/C1/lab") {
		t.Errorf("GET /folders/courses/C1 lists the excluded directory; page = %q", folders)
	}
	hiddenFolderCode, _ := skipBasenameGET(t, site, "/folders/courses/C1/.hidden")
	if code, _ := skipBasenameGET(t, site, "/folders/courses/C1/lab"); code != hiddenFolderCode {
		t.Errorf("GET /folders/courses/C1/lab = %d, want %d, the answer for a hidden directory", code, hiddenFolderCode)
	}

	search := readingPage(t, site, "/search?q="+excludeDirsSentinel)
	if !strings.Contains(search, `data-result-count="0"`) {
		t.Errorf("GET /search found text beneath an excluded directory; page = %q", search)
	}

	if kept := readingPage(t, site, "/search?q="+keptDirsSentinel); strings.Contains(kept, `data-result-count="0"`) {
		t.Errorf("GET /search lost the sibling modules directory; page = %q", kept)
	}

	findings, checkErr := judge.Check(t.Context(), root)
	if checkErr != nil {
		t.Fatalf("check: %v", checkErr)
	}
	for _, finding := range findings {
		if strings.Contains(finding.Path, "/lab/") || strings.Contains(finding.Message, "main.go") {
			t.Errorf("check judged a file beneath an excluded directory: %+v", finding)
		}
	}

	hiddenCode, _ := skipBasenameGET(t, site, "/raw/courses/C1/.hidden/main.go")
	for _, target := range []string{
		"/raw/courses/C1/lab/main.go",
		"/notes/courses/C1/lab/Inside.md",
		"/notes/courses/C1/lab/main.go",
	} {
		code, body := skipBasenameGET(t, site, target)
		if code != hiddenCode {
			t.Errorf("GET %s = %d, want %d, the answer for a hidden path", target, code, hiddenCode)
		}
		if strings.Contains(body, excludeDirsSentinel) {
			t.Errorf("GET %s served bytes from an excluded directory", target)
		}
	}
	if hiddenCode != http.StatusNotFound {
		t.Errorf("GET /raw/courses/C1/.hidden/main.go = %d, want 404", hiddenCode)
	}
	if code, _ := skipBasenameGET(t, site, "/raw/courses/C1/modules/lab/Shown.txt"); code != http.StatusOK {
		t.Errorf("GET /raw/courses/C1/modules/lab/Shown.txt = %d, want 200: a directory named lab at another depth is not excluded", code)
	}
}
