package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

// skipBasenameLockContract is the Lock vault for #363: one knowledge note and
// a README.md the contract names under scan.skip_basenames. The README carries
// a type the contract refuses, so check would report it if the scan still
// treated it as a note.
const skipBasenameLockContract = `schema_version = "1"

[enums]
type = ["note"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status"]

[scan]
knowledge_dirs = ["Notes"]
skip_basenames = ["README.md"]

[navigation]
path_types = []
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`

const skipBasenameLockSentinel = "skipbasename-lock-sentinel"

func writeSkipBasenameLockVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		schema.ContractRelPath: skipBasenameLockContract,
		"README.md": "---\ntitle: Skip me\ntype: probe\n---\n" +
			skipBasenameLockSentinel + "\n",
		"Notes/Kept.md": "---\ntitle: Kept\ntype: note\nstatus: draft\n---\nKept body.\n",
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
	return root
}

// TestSkipBasenamesAreNotNotes is the Lock for #363. A basename the contract
// names under scan.skip_basenames is not a note: the shelf, search, exists and
// check all omit it, /notes/ serves it as a file (showFile), and /raw/ still
// serves the bytes.
func TestSkipBasenamesAreNotNotes(t *testing.T) {
	t.Parallel()

	root := writeSkipBasenameLockVault(t)
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	folders := readingPage(t, site, "/folders")
	if strings.Contains(folders, `href="/notes/README.md"`) {
		t.Errorf("GET /folders still shelves the skipped basename; page = %q", folders)
	}
	if !strings.Contains(folders, `href="/notes/Notes/Kept.md"`) {
		t.Errorf("GET /folders lost the kept note; page = %q", folders)
	}

	search := readingPage(t, site, "/search?q="+skipBasenameLockSentinel)
	if strings.Contains(search, `href="/notes/README.md"`) {
		t.Errorf("GET /search indexed the skipped basename; page = %q", search)
	}
	if !strings.Contains(search, `data-result-count="0"`) {
		t.Errorf("GET /search for the skipped sentinel was not empty; page = %q", search)
	}
	keptSearch := readingPage(t, site, "/search?q=Kept")
	if !strings.Contains(keptSearch, `href="/notes/Notes/Kept.md`) {
		t.Errorf("GET /search lost the kept note; page = %q", keptSearch)
	}

	existsOut, existsExit, existsErr := judge.RunExists(t.Context(), &judge.ExistsOptions{
		Root:   root,
		Name:   "README.md",
		Format: judge.FormatHuman,
	})
	if existsErr != nil {
		t.Fatalf("exists README.md: %v", existsErr)
	}
	if existsExit != 1 {
		t.Errorf("exists README.md exit = %d, want 1 (absent); stdout = %s", existsExit, existsOut)
	}
	if !strings.Contains(string(existsOut), "does not exist") {
		t.Errorf("exists README.md did not report absence; stdout = %s", existsOut)
	}
	keptOut, keptExit, keptErr := judge.RunExists(t.Context(), &judge.ExistsOptions{
		Root: root,
		Name: "Kept",
	})
	if keptErr != nil {
		t.Fatalf("exists Kept: %v", keptErr)
	}
	if keptExit != 0 {
		t.Errorf("exists Kept exit = %d, want 0; stdout = %s", keptExit, keptOut)
	}

	findings, checkErr := judge.Check(t.Context(), root)
	if checkErr != nil {
		t.Fatalf("check: %v", checkErr)
	}
	for _, finding := range findings {
		if strings.Contains(finding.Path, "README.md") {
			t.Errorf("check judged the skipped basename: %+v", finding)
		}
	}

	notesCode, shown := skipBasenameGET(t, site, "/notes/README.md")
	if notesCode != http.StatusOK {
		t.Errorf("GET /notes/README.md = %d, want 200 (showFile)", notesCode)
	}
	if !strings.Contains(shown, `class="y-prose y-source"`) {
		t.Errorf("GET /notes/README.md is not the showFile source view; page = %q", shown)
	}
	if strings.Contains(shown, `y-statuspanel`) {
		t.Errorf("GET /notes/README.md is a note page; page = %q", shown)
	}

	raw := skipBasenameRaw(t, site, "/raw/README.md")
	if !strings.Contains(raw, skipBasenameLockSentinel) {
		t.Errorf("GET /raw/README.md did not serve the skipped file; body = %q", raw)
	}
}

func skipBasenameRaw(t *testing.T, site http.Handler, target string) string {
	t.Helper()
	code, body := skipBasenameGET(t, site, target)
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", target, code)
	}
	return body
}

func skipBasenameGET(t *testing.T, site http.Handler, target string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, target, nil))
	response := recorder.Result()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close %s response: %v", target, err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", target, err)
	}
	return response.StatusCode, string(body)
}
