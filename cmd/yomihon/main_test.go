package main_test

// The composition point is where every face is wired together, so it is the
// home of the guard that no read or render face ever writes to the vault: an
// invariant of the whole system rather than of any one feature.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/course"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/report"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestHelpIsSideEffectFree(t *testing.T) {
	t.Parallel()
	binary := buildYomihonBinary(t)
	home := t.TempDir()
	env := append(isolatedUserEnv(home),
		"YOMIHON_PORT=not-a-port",
	)

	// Every command the first argument can name has a help page of its own,
	// and asking for it opens nothing: no folder, no port, no configuration.
	commands := []string{"serve", "check", "coverage", "exists", "marks", "vaults", "approve-figure", "revoke-figure"}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		exit, stdout, stderr := runYomihonBinary(t, binary, args, env)
		if exit != 0 || !strings.HasPrefix(stdout, "Usage:") || stderr != "" {
			t.Errorf("yomihon %v = exit %d, stdout %q, stderr %q; want 0, a usage, empty", args, exit, stdout, stderr)
		}
		for _, name := range commands {
			if !strings.Contains(stdout, "yomihon "+name) {
				t.Errorf("yomihon %v does not list %s", args, name)
			}
		}
	}
	for _, name := range commands {
		for _, args := range [][]string{{name, "--help"}, {name, "-h"}, {"help", name}} {
			exit, stdout, stderr := runYomihonBinary(t, binary, args, env)
			if exit != 0 || !strings.HasPrefix(stdout, "Usage:") || stderr != "" {
				t.Errorf("yomihon %v = exit %d, stdout %q, stderr %q; want 0, a usage, empty", args, exit, stdout, stderr)
			}
		}
	}
	assertHomeUntouched(t, home)
}

func TestServeRejectsArgumentsBeforeLoadingConfiguration(t *testing.T) {
	t.Parallel()
	binary := buildYomihonBinary(t)
	home := t.TempDir()
	// Two folders is the argument error now that one is the ordinary way to
	// name a folder: the check has to be a shape the parser still refuses, or
	// it stops proving that refusal comes before any configuration is read.
	exit, stdout, stderr := runYomihonBinary(t, binary, []string{"serve", "one", "two"}, append(isolatedUserEnv(home),
		"YOMIHON_PORT=not-a-port",
	))
	const wantUsage = "yomihon: usage: yomihon [dir] — or yomihon serve [dir] — or yomihon serve --root <dir>\n"
	if exit != 2 || stdout != "" || stderr != wantUsage {
		t.Errorf("yomihon serve unexpected = exit %d, stdout %q, stderr %q; want 2, empty, %q", exit, stdout, stderr, wantUsage)
	}
	assertHomeUntouched(t, home)
}

func TestServeRejectsNonDirectoryVaultRoot(t *testing.T) {
	t.Parallel()
	binary := buildYomihonBinary(t)
	root := filepath.Join(t.TempDir(), "vault.md")
	if err := os.WriteFile(root, []byte("not a vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	exit, stdout, stderr := runYomihonBinary(t, binary, []string{"serve", root}, append(isolatedUserEnv(t.TempDir()),
		"YOMIHON_PORT=not-a-port",
	))
	if exit != 1 || stdout != "" || !strings.Contains(stderr, root) ||
		!strings.Contains(stderr, "is not a directory") || strings.Contains(stderr, "listen") {
		t.Errorf("yomihon serve with file root = exit %d, stdout %q, stderr %q; want 1, empty, root and directory error before listen", exit, stdout, stderr)
	}
}

func buildYomihonBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "yomihon")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".") // #nosec G204 -- fixed tool and arguments plus a test-owned temporary output path
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build yomihon: %v\n%s", err, output)
	}
	return binary
}

func runYomihonBinary(t *testing.T, binary string, args, env []string) (exit int, stdout, stderr string) {
	t.Helper()
	var stdoutBuffer, stderrBuffer bytes.Buffer
	cmd := exec.CommandContext(t.Context(), binary, args...) // #nosec G204 -- binary and arguments are constructed entirely by this test
	cmd.Env = env
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer
	err := cmd.Run()
	if err == nil {
		return 0, stdoutBuffer.String(), stderrBuffer.String()
	}
	exitError, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("run yomihon %v: %v", args, err)
	}
	return exitError.ExitCode(), stdoutBuffer.String(), stderrBuffer.String()
}

func isolatedUserEnv(home string) []string {
	cache := filepath.Join(home, "cache")
	return []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"XDG_CACHE_HOME=" + cache,
		"LOCALAPPDATA=" + cache,
	}
}

func assertHomeUntouched(t *testing.T, home string) {
	t.Helper()
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatalf("read isolated home: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i := range entries {
			names[i] = entries[i].Name()
		}
		t.Errorf("command touched isolated home: entries = %q, want none", names)
	}
}

// TestReadFacesNeverWriteTheVault drives every read and render face against a
// fixture vault and asserts not one byte of the vault changed. The renderer
// reads fault-tolerantly and reports; a human edits files — nothing on the read
// path may write. The invariant is system-level: it is not that each face
// avoids writing in isolation but that all of them summed do, so the guard
// lives at the composition point and drives the faces as they are actually
// wired.
func TestReadFacesNeverWriteTheVault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSweepFixture(t, root)

	before := hashTree(t, root)

	log := slog.New(slog.DiscardHandler)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatalf("schema.LoadReader: %v", err)
	}
	store, err := snapshot.New(t.Context(), reader, log, contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}

	authority := func() schema.Authority { return schema.CaptureAuthority(contract, contract.Governance()) }
	vaultName := shell.VaultName(reader.Name())
	searchProvider := func() search.RequestSnapshot {
		snap := store.Current().Capture()
		return search.RequestSnapshot{Index: snap.Search(), Shell: shell.Project(vaultName, authority(), snap), Roles: snap.NavigationRoles()}
	}
	pathProvider := func() course.RequestSnapshot {
		snap := store.Current().Capture()
		captured := authority()
		return course.RequestSnapshot{Shell: shell.Project(vaultName, captured, snap), Generation: snap, Authority: captured}
	}

	mux := http.NewServeMux()
	note.New(&note.Sources{
		Bookmark:  func() (mark.Bookmark, bool) { return mark.Bookmark{}, false },
		Source:    reader,
		Authority: authority,
		Snapshot:  store.Current,
		Log:       log,
	}).Register(mux)
	search.NewHandler(searchProvider, log).Register(mux)
	course.New(pathProvider, log).Register(mux)
	report.New(func() report.Request {
		snap := store.Current().Capture()
		return report.Request{Generation: snap, Shell: shell.Project("vault", authority(), snap)}
	}, log).Register(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Drive each read/render route at least once, covering its disk-touching
	// path: the direct-render Home, notes, a study-path syllabus, search, the
	// report shelf and a written report, and the faces that open a file that is
	// not a note — a text file rendered as source, an HTML file, a picture, and
	// the unchanged bytes behind each of them.
	for _, path := range []string{
		"/",
		"/notes/README.md",
		"/notes/Notes/alpha.md",
		"/notes/Notes/beta.md",
		"/courses/Maps/study.md",
		"/search?q=tortoise",
		"/search/results?q=tortoise",
		"/reports",
		"/notes/System/reports/2026-09-02 review.md",
		"/notes/System/reports/daily-briefing/latest.html",
		"/raw/System/reports/daily-briefing/latest.html",
		"/notes/Makefile",
		"/raw/Makefile",
		"/notes/Diagrams/pic.png",
		"/raw/Diagrams/pic.png",
	} {
		drive(t, srv.Client(), srv.URL+path)
	}

	// The adjudicator reads the whole vault as well; it reports, never repairs.
	if _, err := judge.Check(t.Context(), root); err != nil {
		t.Fatalf("judge.Check(t.Context(), %q) = %v", root, err)
	}

	after := hashTree(t, root)
	if diff := cmp.Diff(before, after); diff != "" {
		t.Errorf("the vault changed after driving every read face (-before +after):\n%s", diff)
	}
}

// writeSweepFixture lays down a vault that exercises each read face: a home
// note, two linked notes, a study-path note (a syllabus), a written report, and
// three files that are not notes — one text, carrying no extension so its kind
// is decided by its bytes, an HTML file left where a retired generator wrote
// its briefings, and one picture, whose few bytes only have to be enough for
// the route to name and serve them.
func writeSweepFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"README.md":                           "# Sweep\n\nHome, linking to [[Alpha]].\n",
		"Notes/alpha.md":                      "---\ntype: concept\naliases: [Alpha]\n---\n# Alpha\n\nAlpha links to [[Beta]] and mentions a tortoise.\n",
		"Notes/beta.md":                       "---\ntype: concept\naliases: [Beta]\n---\n# Beta\n\nBeta body.\n",
		"Maps/study.md":                       "---\ntype: study-path\n---\n# Study Path\n\n- [[Alpha]]\n- [[Beta]]\n",
		"Makefile":                            "build:\n\tgo build ./...\n",
		"Diagrams/pic.png":                    "\x89PNG\r\n\x1a\n and a few bytes more",
		"System/reports/2026-09-02 review.md": "# Review\n\nThe week in the vault.\n",
		"System/reports/daily-briefing/latest.html": "<!doctype html><h1>Daily briefing</h1><p>body</p>\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	contract = append(contract, []byte("\n[privacy]\nnever_egress_dirs = []\n")...)
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err = os.MkdirAll(filepath.Dir(contractPath), 0o750); err != nil {
		t.Fatalf("mkdir contract directory: %v", err)
	}
	if err = os.WriteFile(contractPath, contract, 0o600); err != nil { // #nosec G703 -- path is the fixed contract location under t.TempDir
		t.Fatalf("write contract: %v", err)
	}
}

// hashTree returns a map from each file's vault-relative path to the SHA-256 of
// its bytes, so a later diff catches any content change, addition, or removal.
func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	sums := map[string]string{}
	fsys := os.DirFS(root)
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := fs.ReadFile(fsys, path)
		if rerr != nil {
			return rerr
		}
		sums[path] = fmt.Sprintf("%x", sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatalf("hash %s: %v", root, err)
	}
	return sums
}

// drive issues a GET, discards the body, and asserts the route actually served —
// a status below 400. Without that assertion the sweep could pass without ever
// exercising a route: one regressed to a 404 or a 500 answers before it touches
// disk, so "nothing was read" reads as "nothing was written" and the guard
// proves nothing at all. A status at or above 400 means the route did not
// serve.
func drive(t *testing.T, client *http.Client, url string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil {
		t.Errorf("read GET %s response: %v", url, copyErr)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Errorf("close GET %s response: %v", url, closeErr)
	}
	if resp.StatusCode >= 400 {
		t.Errorf("GET %s = %d, want the route to serve (below 400) so the sweep actually exercises it", url, resp.StatusCode)
	}
}
