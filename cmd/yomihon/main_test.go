package main_test

// The composition point is where every face is wired together, so it is the
// only honest home for two guards that are about the whole system rather than
// any one feature: that no read or render face ever writes to the vault, and
// that the command reads no environment beyond the three variables it is allowed.
// Neither exercises main's wiring logic; each pins a system-wide invariant that
// has no other place to live.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/report"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/syllabus"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestHelpIsSideEffectFree(t *testing.T) {
	t.Parallel()
	binary := buildYomihonBinary(t)
	home := t.TempDir()
	env := append(isolatedUserEnv(home),
		"YOMIHON_ROOT="+filepath.Join(t.TempDir(), "missing-vault"),
		"YOMIHON_PORT=not-a-port",
		"YOMIHON_EMBED_KEY=help-must-not-touch-this-key",
	)

	top := "Usage:\n" +
		"  yomihon [<dir>]                       read a folder (default: this one)\n" +
		"  yomihon serve [<dir>]\n" +
		"  yomihon search [options] <query...>\n" +
		"  yomihon search-index build [options]\n" +
		"  yomihon check [options] [path...]\n" +
		"  yomihon coverage [options]\n" +
		"  yomihon exists [options] <name>\n\n" +
		"Use \"yomihon <command> --help\" for command help.\n"
	command := map[string]string{
		"serve": "Usage: yomihon [<dir>]  —  or  yomihon serve [<dir>]\n" +
			"\n" +
			"Reads the folder named on the line, or $YOMIHON_ROOT, or the current\n" +
			"directory. Serves it on 127.0.0.1:$YOMIHON_PORT (default 9610).\n" +
			"\n" +
			"The folder is fixed for the life of the process: reading another one\n" +
			"means another yomihon, on another port.\n",
		"search":       "Usage: yomihon search [--json] [--semantic] [--root <dir>] [--limit <1..1000>] [--] <query...>\n",
		"search-index": "Usage: yomihon search-index build [--json] [--renew-attempt-budget] [--root <dir>]\n",
		"check":        "Usage: yomihon check [--root <dir>] [--format json|human|md] [--all] [--deny <severity|rule-id>]... [--baseline <file>] [path...]\n",
		"coverage":     "Usage: yomihon coverage [--root <dir>] [--format json|human|md]\n",
		"exists":       "Usage: yomihon exists [--root <dir>] [--format json|human|md] <name>\n",
	}

	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		exit, stdout, stderr := runYomihonBinary(t, binary, args, env)
		if exit != 0 || stdout != top || stderr != "" {
			t.Errorf("yomihon %v = exit %d, stdout %q, stderr %q; want 0, %q, empty", args, exit, stdout, stderr, top)
		}
	}
	for name, want := range command {
		for _, args := range [][]string{{name, "--help"}, {name, "-h"}, {"help", name}} {
			exit, stdout, stderr := runYomihonBinary(t, binary, args, env)
			if exit != 0 || stdout != want || stderr != "" {
				t.Errorf("yomihon %v = exit %d, stdout %q, stderr %q; want 0, %q, empty", args, exit, stdout, stderr, want)
			}
		}
	}
	for _, args := range [][]string{{"search-index", "build", "--help"}, {"search-index", "build", "-h"}, {"help", "search-index", "build"}} {
		exit, stdout, stderr := runYomihonBinary(t, binary, args, env)
		if exit != 0 || stdout != command["search-index"] || stderr != "" {
			t.Errorf("yomihon %v = exit %d, stdout %q, stderr %q; want 0, %q, empty", args, exit, stdout, stderr, command["search-index"])
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
		"YOMIHON_ROOT="+filepath.Join(t.TempDir(), "missing-vault"),
		"YOMIHON_PORT=not-a-port",
	))
	const wantUsage = "yomihon: usage: yomihon [dir] — or yomihon serve [dir]\n"
	if exit != 2 || stdout != "" || stderr != wantUsage {
		t.Errorf("yomihon serve unexpected = exit %d, stdout %q, stderr %q; want 2, empty, %q", exit, stdout, stderr, wantUsage)
	}
	assertHomeUntouched(t, home)
}

func TestProductionBinaryDispatchesPrivateGitChildBeforeCLIParsing(t *testing.T) {
	t.Parallel()
	binary := buildYomihonBinary(t)
	exit, stdout, stderr := runYomihonBinary(
		t,
		binary,
		[]string{"__yomihon-status-git"},
		isolatedUserEnv(t.TempDir()),
	)
	if exit != 1 || stdout != "" || stderr != "yomihon: status git child: missing git arguments\n" {
		t.Errorf("private git child = exit %d, stdout %q, stderr %q; want 1, empty, missing-arguments diagnostic", exit, stdout, stderr)
	}
}

func TestServeRejectsNonDirectoryVaultRoot(t *testing.T) {
	t.Parallel()
	binary := buildYomihonBinary(t)
	root := filepath.Join(t.TempDir(), "vault.md")
	if err := os.WriteFile(root, []byte("not a vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	exit, stdout, stderr := runYomihonBinary(t, binary, []string{"serve"}, append(isolatedUserEnv(t.TempDir()),
		"YOMIHON_ROOT="+root,
		"YOMIHON_PORT=not-a-port",
	))
	if exit != 1 || stdout != "" || !strings.Contains(stderr, root) ||
		!strings.Contains(stderr, "is not a directory") || strings.Contains(stderr, "listen") {
		t.Errorf("yomihon serve with file root = exit %d, stdout %q, stderr %q; want 1, empty, root and directory error before listen", exit, stdout, stderr)
	}
}

func TestAgentCommandsReachProductionComposition(t *testing.T) {
	root := t.TempDir()
	writeSweepFixture(t, root)
	binary := buildYomihonBinary(t)

	t.Run("lexical search ignores semantic state", func(t *testing.T) {
		home := t.TempDir()
		exit, stdout, stderr := runYomihonBinary(t, binary, []string{"search", "--json", "tortoise"}, append(isolatedUserEnv(home),
			"YOMIHON_ROOT="+root,
			"YOMIHON_EMBED_KEY=key-must-remain-dormant",
		))
		if exit != 0 {
			t.Fatalf("yomihon search exit = %d, want 0; stderr = %q", exit, stderr)
		}
		if !strings.Contains(stdout, `"semantic":"off"`) || !strings.Contains(stdout, `"rel_path":"Notes/alpha.md"`) {
			t.Errorf("yomihon search stdout = %q, want lexical answer", stdout)
		}
		if stderr != "" {
			t.Errorf("yomihon search stderr = %q, want empty", stderr)
		}
		if strings.Contains(stdout+stderr, "key-must-remain-dormant") {
			t.Error("yomihon search exposed the embedding key")
		}
		assertNoSemanticStore(t, home)
	})

	t.Run("semantic search reports cold before provider", func(t *testing.T) {
		home := t.TempDir()
		exit, stdout, stderr := runYomihonBinary(t, binary, []string{"search", "--json", "--semantic", "tortoise"}, append(isolatedUserEnv(home),
			"YOMIHON_ROOT="+root,
			"YOMIHON_EMBED_KEY=key-must-not-reach-a-client",
		))
		if exit != 3 {
			t.Fatalf("yomihon search --semantic exit = %d, want 3; stderr = %q", exit, stderr)
		}
		if !strings.Contains(stdout, `"reason":"cache-cold"`) {
			t.Errorf("yomihon search --semantic stdout = %q, want cache-cold", stdout)
		}
		if got, want := stderr, "yomihon search: cache-cold: no semantic index exists; run yomihon search-index build\n"; got != want {
			t.Errorf("yomihon search --semantic stderr = %q, want %q", got, want)
		}
		if strings.Contains(stdout+stderr, "key-must-not-reach-a-client") {
			t.Error("yomihon search --semantic exposed the embedding key")
		}
		assertNoSemanticStore(t, home)
	})

	t.Run("explicit build without credential is unavailable", func(t *testing.T) {
		home := t.TempDir()
		exit, stdout, stderr := runYomihonBinary(t, binary, []string{"search-index", "build", "--json"}, append(isolatedUserEnv(home),
			"YOMIHON_ROOT="+root,
		))
		if exit != 3 {
			t.Fatalf("yomihon search-index build exit = %d, want 3; stderr = %q", exit, stderr)
		}
		if got, want := stdout, "{\"error\":{\"reason\":\"embedder-unconfigured\",\"active_generation\":\"absent\",\"staging_generation\":\"resumable\",\"retry_safe\":false,\"next_action\":\"repair-configuration\"}}\n"; got != want {
			t.Errorf("yomihon search-index build stdout = %q, want %q", got, want)
		}
		if got, want := stderr, "yomihon search-index: embedder-unconfigured: no embedding key is configured, so semantic search is off\n"; got != want {
			t.Errorf("yomihon search-index build stderr = %q, want %q", got, want)
		}
	})

	for _, tt := range []struct {
		name string
		key  string
	}{
		{name: "serve without key"},
		{name: "serve with key", key: "serve-key-must-remain-dormant"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			log := runYomihonServe(t, binary, root, home, tt.key)
			if strings.Contains(log, "semantic") {
				t.Errorf("yomihon serve log mentions semantic state: %q", log)
			}
			if tt.key != "" && strings.Contains(log, tt.key) {
				t.Error("yomihon serve exposed the embedding key")
			}
			assertNoSemanticStore(t, home)
		})
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

func assertNoSemanticStore(t *testing.T, home string) {
	t.Helper()
	assertHomeUntouched(t, home)
}

func runYomihonServe(t *testing.T, binary, root, home, key string) string {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "serve.log")
	logFile, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- logPath is rooted in t.TempDir
	if openErr != nil {
		t.Fatalf("open serve log: %v", openErr)
	}
	cmd := exec.CommandContext(t.Context(), binary, "serve")
	cmd.Env = append(isolatedUserEnv(home),
		"YOMIHON_ROOT="+root,
		"YOMIHON_PORT=0",
		"YOMIHON_EMBED_KEY="+key,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if startErr := cmd.Start(); startErr != nil {
		closeErr := logFile.Close()
		t.Fatalf("start yomihon serve: %v", errors.Join(startErr, closeErr))
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped && cmd.Process != nil {
			if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				t.Logf("kill unfinished yomihon serve: %v", killErr)
			}
			if waitErr := cmd.Wait(); waitErr != nil {
				t.Logf("wait for killed yomihon serve: %v", waitErr)
			}
		}
		if closeErr := logFile.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			t.Errorf("close serve log during cleanup: %v", closeErr)
		}
	})

	deadline := time.Now().Add(10 * time.Second)
	for {
		data, readErr := os.ReadFile(logPath) // #nosec G304 -- logPath is rooted in t.TempDir
		if readErr != nil {
			t.Fatalf("read serve log: %v", readErr)
		}
		if bytes.Contains(data, []byte("yomihon serving")) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("yomihon serve did not start; log = %q", data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if signalErr := cmd.Process.Signal(os.Interrupt); signalErr != nil {
		t.Fatalf("interrupt yomihon serve: %v", signalErr)
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		t.Fatalf("wait yomihon serve: %v", waitErr)
	}
	stopped = true
	if closeErr := logFile.Close(); closeErr != nil {
		t.Fatalf("close serve log: %v", closeErr)
	}
	data, readErr := os.ReadFile(logPath) // #nosec G304 -- logPath is rooted in t.TempDir
	if readErr != nil {
		t.Fatalf("read final serve log: %v", readErr)
	}
	return string(data)
}

// TestReadFacesNeverWriteTheVault drives every read and render face against a
// fixture vault and asserts not one byte of the vault changed. The renderer
// reads fault-tolerantly and reports; a human edits files — nothing on the read
// path may write. The invariant is system-level: it is not that each face
// avoids writing in isolation but that all of them summed do, so the guard
// lives at the composition point and drives the faces as they are actually
// wired. The one writing endpoint (POST /status) is deliberately not wired,
// because writing the single status field is its job.
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

	// The lifecycle shares the fixture's contract authority with the snapshot,
	// but its writing endpoint is deliberately not registered below.
	lifecycle, err := status.Open(reader, contract, contract.Governance())
	if err != nil {
		t.Fatalf("status.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	projectShell := func(statusView status.View, snap *snapshot.View) pages.Shell {
		return shell.Project(statusView, snap.ArtifactPolicy(), snap)
	}
	shellForSnapshot := func(snap *snapshot.View) pages.Shell {
		return projectShell(lifecycle.View(), snap)
	}
	shellProvider := func() pages.Shell {
		statusView := lifecycle.View()
		return projectShell(statusView, store.Current().Capture())
	}
	searchProvider := func() search.RequestSnapshot {
		statusView := lifecycle.View()
		snap := store.Current().Capture()
		return search.RequestSnapshot{Index: snap.Search(), Shell: projectShell(statusView, snap)}
	}

	mux := http.NewServeMux()
	note.New(&note.Dependencies{
		ObservedStatus: lifecycle.ObservedStatus,
		Source:         reader,
		Status:         lifecycle.View,
		Snapshot:       store.Current,
		Provenance:     lifecycle.LastCommitHash,
		Log:            log,
	}).Register(mux)
	search.NewHandler(searchProvider, log).Register(mux)
	syllabus.New(shellProvider, log).Register(mux)
	report.New(reader, store.Current, shellForSnapshot, log).Register(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Drive each read/render route at least once, covering its disk-touching
	// path: the direct-render Home, notes, a study-path syllabus, search, both the
	// report shell and its verbatim raw read, and the faces that open a file
	// that is not a note — a text file rendered as source, a picture, and the
	// unchanged bytes behind each of them.
	for _, path := range []string{
		"/",
		"/notes/README.md",
		"/notes/Notes/alpha.md",
		"/notes/Notes/beta.md",
		"/syllabus/Maps/study.md",
		"/search?q=tortoise",
		"/search/results?q=tortoise",
		"/reports/latest.html",
		"/reports/latest.html/raw",
		"/notes/Makefile",
		"/raw/Makefile",
		"/notes/Diagrams/pic.png",
		"/raw/Diagrams/pic.png",
	} {
		drive(t, srv.URL+path)
	}

	// The adjudicator reads the whole vault as well; it reports, never repairs.
	if _, err := judge.Check(root); err != nil {
		t.Fatalf("judge.Check(%q) = %v", root, err)
	}

	after := hashTree(t, root)
	if diff := cmp.Diff(before, after); diff != "" {
		t.Errorf("the vault changed after driving every read face (-before +after):\n%s", diff)
	}
}

// writeSweepFixture lays down a vault that exercises each read face: a home
// note, two linked notes, a study-path note (a syllabus), a briefing (the
// verbatim raw read), and two files that are not notes — one text, carrying no
// extension so its kind is decided by its bytes, and one picture, whose few
// bytes only have to be enough for the route to name and serve them.
func writeSweepFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"README.md":        "# Sweep\n\nHome, linking to [[Alpha]].\n",
		"Notes/alpha.md":   "---\ntype: concept\naliases: [Alpha]\n---\n# Alpha\n\nAlpha links to [[Beta]] and mentions a tortoise.\n",
		"Notes/beta.md":    "---\ntype: concept\naliases: [Beta]\n---\n# Beta\n\nBeta body.\n",
		"Maps/study.md":    "---\ntype: study-path\ntitle: Study Path\n---\n# Study Path\n\n- [[Alpha]]\n- [[Beta]]\n",
		"Makefile":         "build:\n\tgo build ./...\n",
		"Diagrams/pic.png": "\x89PNG\r\n\x1a\n and a few bytes more",
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
// proves nothing at all. The default client follows redirects, so the home
// page's 302 lands on its final 2xx; a status at or above 400 means the route
// did not serve.
func drive(t *testing.T, url string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
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

// allowedEnvKeys are the three variables this command may interpret as
// configuration: the vault root, the listening port, and the embedding
// credential read lazily by an explicit semantic CLI action. The private git
// child has one separately audited whole-environment read whose only purpose is
// to remove every GIT_* repository redirection and YOMIHON_* application value
// before the external git process and its hooks can observe them.
var allowedEnvKeys = map[string]bool{
	"YOMIHON_ROOT":      true,
	"YOMIHON_PORT":      true,
	"YOMIHON_EMBED_KEY": true,
}

// envReaders names every way the two packages that expose one can read the
// environment, keyed by import path and then by symbol. A symbol whose reason is
// empty is judged by the key it is called with; every other symbol is refused
// wherever it appears, and its reason says why.
//
// os.Getenv and os.LookupEnv are the two doors, because the command reads its
// three variables through them. os.Environ takes the whole environment at once;
// os.ExpandEnv names its variables inside a string, and os.Expand hands the
// reading to a mapping — neither offers a key to check. syscall.Getenv takes a
// single literal key exactly as os.Getenv does, and is refused all the same: an
// allowlist guards nothing once there are two doors, and a read that goes around
// os goes around this audit. syscall is already imported for the termination
// signal, which makes it the nearest way around.
//
// The map is the set of ways the promise can be broken, not the set of calls the
// command happens to make. Every row is held to a control below.
var envReaders = map[string]map[string]string{
	"os": {
		"Getenv":    "",
		"LookupEnv": "",
		"Environ":   "reads the whole environment",
		"ExpandEnv": "reads every variable named in its argument",
		"Expand":    "reads the environment through a mapping this guard cannot follow",
	},
	"syscall": {
		"Getenv":  "reads the environment beneath the os package",
		"Environ": "reads the whole environment",
	},
}

// envRead is one place a parsed file reaches for the environment. Why is empty
// when the reach is permitted — an allowed key through one of the two doors —
// and otherwise says why it is refused. Recording the permitted reaches as well
// as the refused ones is what lets the controls below prove that every reader
// named in envReaders is actually watched.
type envRead struct {
	Pos    token.Position
	Pkg    string // the import path the read goes through
	Symbol string // the symbol read through; empty for a finding about an import itself
	Func   string // containing function, when the read occurs in one
	Why    string
}

// envReader reports the canonical import path and symbol when sel names one of
// the environment readers, resolving the file's own import names so that an
// aliased import is recognised for what it is.
func envReader(sel *ast.SelectorExpr, pkgOf map[string]string) (pkg, symbol string, ok bool) {
	ident, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return "", "", false
	}
	path, imported := pkgOf[ident.Name]
	if !imported {
		return "", "", false
	}
	if _, reads := envReaders[path][sel.Sel.Name]; !reads {
		return "", "", false
	}
	return path, sel.Sel.Name, true
}

// envReads reports every reach for the environment in the parsed files, refused
// or permitted. A reach is permitted only when it is an os.Getenv or
// os.LookupEnv call on one of the allowed keys.
//
// A reader mentioned outside a call — os.Expand(s, os.Getenv) hands Getenv over
// as a value — carries no key this guard can read, so a mention is judged as
// well as a call. Because the walk visits a call before the selector inside it,
// a selector already judged as a call is recognised when it comes round again.
func envReads(fset *token.FileSet, files []*ast.File, allowed map[string]bool) []envRead {
	var reads []envRead
	for _, f := range files {
		functionAt := func(pos token.Pos) string {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Pos() <= pos && pos <= fn.End() {
					return fn.Name.Name
				}
			}
			return ""
		}
		record := func(pos token.Pos, pkg, symbol, why string) {
			reads = append(reads, envRead{
				Pos:    fset.Position(pos),
				Pkg:    pkg,
				Symbol: symbol,
				Func:   functionAt(pos),
				Why:    why,
			})
		}
		at := func(pos token.Pos, pkg, symbol, format string, args ...any) {
			record(pos, pkg, symbol, fmt.Sprintf(format, args...))
		}
		// Resolve which local name binds to each environment-reading package, so
		// an aliased import (import o "os") cannot slip a read past this guard by
		// calling o.Getenv. A dot-import hides its calls entirely — a bare Getenv
		// with no package selector — so it is refused outright, not audited. A
		// blank import makes no calls at all.
		pkgOf := map[string]string{}
		for _, imp := range f.Imports {
			path, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil || envReaders[path] == nil {
				continue
			}
			switch {
			case imp.Name == nil:
				pkgOf[path] = path
			case imp.Name.Name == ".":
				at(imp.Pos(), path, "", "dot-imports %s, which hides environment reads from this guard", path)
			case imp.Name.Name == "_":
				// a side-effect import makes no calls
			default:
				pkgOf[imp.Name.Name] = path
			}
		}
		judged := map[*ast.SelectorExpr]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				sel, isSel := n.Fun.(*ast.SelectorExpr)
				if !isSel {
					return true
				}
				pkg, symbol, reads := envReader(sel, pkgOf)
				if !reads {
					return true
				}
				judged[sel] = true
				if why := envReaders[pkg][symbol]; why != "" {
					at(n.Pos(), pkg, symbol, "%s.%s %s", pkg, symbol, why)
					return true
				}
				if len(n.Args) != 1 {
					at(n.Pos(), pkg, symbol, "%s.%s called with %d arguments, so no single key can be checked", pkg, symbol, len(n.Args))
					return true
				}
				lit, isLit := n.Args[0].(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING {
					at(n.Pos(), pkg, symbol, "%s.%s called with a non-literal key", pkg, symbol)
					return true
				}
				key, uerr := strconv.Unquote(lit.Value)
				if uerr != nil {
					at(lit.ValuePos, pkg, symbol, "%s.%s called with an unreadable key %s", pkg, symbol, lit.Value)
					return true
				}
				if !allowed[key] {
					at(lit.ValuePos, pkg, symbol, "%s.%s(%q)", pkg, symbol, key)
					return true
				}
				record(lit.ValuePos, pkg, symbol, "")
			case *ast.SelectorExpr:
				if judged[n] {
					return true
				}
				pkg, symbol, isReader := envReader(n, pkgOf)
				if !isReader {
					return true
				}
				at(n.Pos(), pkg, symbol, "%s.%s referenced as a value, so no key can be checked", pkg, symbol)
			}
			return true
		})
	}
	return reads
}

// envOffenders keeps the reaches this command is not permitted to make.
func envOffenders(reads []envRead) []envRead {
	var offenders []envRead
	for _, r := range reads {
		if r.Why != "" {
			offenders = append(offenders, r)
		}
	}
	return offenders
}

func isGitEnvironmentSanitizer(r *envRead) bool {
	return r.Pkg == "os" && r.Symbol == "Environ" && r.Func == "execGitChild" &&
		strings.HasSuffix(filepath.ToSlash(r.Pos.Filename), "/internal/status/git_supported.go")
}

// module is this repository's import path, used to keep the file listing below
// to the packages written here rather than the ones they borrow.
const module = "github.com/koopa0/yomihon"

type goTarget struct {
	os   string
	arch string
}

// productionGoFiles parses every Go file the yomihon binary is built from: the
// command's own, and those of each package it links.
//
// The set comes from the toolchain rather than from a walk of the directories,
// because the promise is about what the process reads. A walk answers a
// different question — what the repository contains — and would call a second
// command's own configuration a breach of this one's, while a build constraint
// that excludes a file from the binary would not exclude it from the walk. Ask
// the linker's question, get the linker's answer.
//
// The environment a binary reads is the environment every package it links
// reads, so a guard that inspected only the command's own directory would be
// answered by moving the read one import away.
func productionGoFiles(t *testing.T, fset *token.FileSet) []*ast.File {
	t.Helper()
	paths := productionGoFilePaths(t, ".", module, envGuardTargets())
	validateProductionGoPaths(t, paths)
	files := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, f)
	}
	return files
}

func productionGoFilePaths(t *testing.T, directory, modulePath string, targets []goTarget) []string {
	t.Helper()
	// Each line is one file of one package in this module that the command links.
	format := `{{if .Module}}{{if eq .Module.Path ` + strconv.Quote(modulePath) + `}}` +
		`{{$dir := .Dir}}{{range .GoFiles}}{{$dir}}/{{.}}{{"\n"}}{{end}}{{end}}{{end}}`
	paths := make(map[string]struct{})
	for _, target := range targets {
		cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", format, ".") // #nosec G204 -- fixed Go invocation; format and target environment are test-owned inputs, never shell-interpreted
		cmd.Dir = directory
		cmd.Env = goTargetEnvironment(target)
		out, err := cmd.Output()
		if err != nil {
			if exit, ok := errors.AsType[*exec.ExitError](err); ok {
				t.Fatalf("go list -deps for %s/%s: %v\n%s", target.os, target.arch, err, exit.Stderr)
			}
			t.Fatalf("go list -deps for %s/%s: %v", target.os, target.arch, err)
		}
		for path := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if path != "" {
				paths[filepath.Clean(path)] = struct{}{}
			}
		}
	}
	return slices.Sorted(maps.Keys(paths))
}

func validateProductionGoPaths(t *testing.T, paths []string) {
	t.Helper()
	// A listing that stopped reaching the command, or stopped reaching the
	// packages it links, would report no offenders and prove nothing at all. Both
	// ends have to be present before an empty result means anything.
	var sawCommand, sawInternal bool
	for _, p := range paths {
		p = filepath.ToSlash(p)
		if strings.HasSuffix(p, "cmd/yomihon/main.go") {
			sawCommand = true
		}
		if strings.Contains(p, "/internal/") {
			sawInternal = true
		}
	}
	if !sawCommand || !sawInternal {
		t.Fatalf("the toolchain listed %d files the command is built from, but not both the command itself and the packages it links (command %v, internal %v), so an empty result would mean nothing",
			len(paths), sawCommand, sawInternal)
	}
}

func envGuardTargets() []goTarget {
	// The release surface is the two 64-bit architectures on each supported
	// reader OS. Do not derive this from go tool dist list: that list describes
	// targets the Go toolchain knows how to emit, including architectures on
	// which this binary's SQLite dependency is not buildable and which yomihon
	// does not certify. Keeping the matrix explicit makes a newly supported
	// architecture an intentional review of both CI and the environment wall.
	return []goTarget{
		{os: "darwin", arch: "amd64"},
		{os: "darwin", arch: "arm64"},
		{os: "linux", arch: "amd64"},
		{os: "linux", arch: "arm64"},
		{os: "windows", arch: "amd64"},
		{os: "windows", arch: "arm64"},
	}
}

func goTargetEnvironment(target goTarget) []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "GOOS=") || strings.HasPrefix(value, "GOARCH=") ||
			strings.HasPrefix(value, "CGO_ENABLED=") {
			continue
		}
		environment = append(environment, value)
	}
	return append(environment, "GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0")
}

// TestOnlyKnownEnvVarsAreRead fixes the command's configuration surface. The
// listener binds the loopback address and only the port is configurable, so the
// only environment values this binary may interpret are the vault root, the
// port, and the embedding credential. The credential is dormant unless an
// explicit semantic CLI action reaches the provider gate. One exact whole-env
// read is required at the private git exec boundary to remove every GIT_*
// repository redirection and YOMIHON_* application value; it is pinned to that
// function and tested separately for total prefix removal. The
// test parses every file the command is built from — its own and each package it
// links, as the toolchain reports them — and asserts that every reach for the
// environment, through os or through the syscall package beneath it, called or
// merely handed over as a value, names one of the three allowed keys.
//
// The subject is the binary, not the repository: a tool that lived here and read
// its own configuration would not be breaking this command's promise, and would
// not be linked into it either.
func TestOnlyKnownEnvVarsAreRead(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	reads := envReads(fset, productionGoFiles(t, fset), allowedEnvKeys)
	var offenders []envRead
	sanitizers := 0
	for _, read := range reads {
		switch {
		case read.Why == "":
		case isGitEnvironmentSanitizer(&read):
			sanitizers++
		default:
			offenders = append(offenders, read)
		}
	}
	if sanitizers != 1 {
		t.Errorf("private git boundary has %d whole-environment sanitizer reads, want exactly 1", sanitizers)
	}
	if len(offenders) > 0 {
		lines := make([]string, 0, len(offenders))
		for _, o := range offenders {
			lines = append(lines, fmt.Sprintf("%s: %s", o.Pos, o.Why))
		}
		t.Errorf("this command may configure only YOMIHON_ROOT, YOMIHON_PORT, and YOMIHON_EMBED_KEY, plus one exact git-environment sanitizer read, but found:\n%s",
			strings.Join(lines, "\n"))
	}
}

func TestEnvironmentGuardIncludesTargetSpecificProductionFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const fixtureModule = "example.invalid/envguard"
	fixtures := map[string]string{
		"go.mod":  "module " + fixtureModule + "\n\ngo 1.26.0\n",
		"main.go": "package main\nfunc main() {}\n",
		"extra_windows.go": `//go:build windows

package main

import "os"

func extra() string { return os.Getenv("YOMIHON_EXTRA") }
`,
	}
	for name, body := range fixtures {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths := productionGoFilePaths(t, root, fixtureModule, envGuardTargets())
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	offenders := envOffenders(envReads(fset, files, allowedEnvKeys))
	if len(offenders) != 1 || offenders[0].Why != `os.Getenv("YOMIHON_EXTRA")` {
		t.Fatalf("target-union offenders = %+v, want the Windows-only environment read", offenders)
	}
}

// envFixtures are the guard's own controls. Each is one way a reader could reach
// a forbidden variable while the guard stayed green, and four of them once did.
// want lists the reason for every refused read, in the order the scan reports
// them. They run with the ordinary suite, so the guard is watched to fail on
// every test run rather than on the day someone thinks to try.
var envFixtures = []struct {
	name string
	src  string
	want []string
}{
	{
		name: "allowed keys",
		src: `package main
import "os"
func f() (string, string, bool) {
	v, ok := os.LookupEnv("YOMIHON_PORT")
	_ = os.Getenv("YOMIHON_EMBED_KEY")
	return os.Getenv("YOMIHON_ROOT"), v, ok
}`,
		want: nil,
	},
	{
		name: "aliased import",
		src: `package main
import o "os"
func f() string { return o.Getenv("YOMIHON_SECRET") }`,
		want: []string{`os.Getenv("YOMIHON_SECRET")`},
	},
	{
		name: "dot import",
		src: `package main
import . "os"
func f() string { return Getenv("YOMIHON_SECRET") }`,
		want: []string{"dot-imports os, which hides environment reads from this guard"},
	},
	{
		name: "blank import",
		src: `package main
import _ "os"
func f() {}`,
		want: nil,
	},
	{
		name: "getenv with a bad key",
		src: `package main
import "os"
func f() string { return os.Getenv("YOMIHON_SECRET") }`,
		want: []string{`os.Getenv("YOMIHON_SECRET")`},
	},
	{
		name: "lookupenv with a bad key",
		src: `package main
import "os"
func f() (string, bool) { return os.LookupEnv("YOMIHON_SECRET") }`,
		want: []string{`os.LookupEnv("YOMIHON_SECRET")`},
	},
	{
		name: "os environ",
		src: `package main
import "os"
func f() []string { return os.Environ() }`,
		want: []string{"os.Environ reads the whole environment"},
	},
	{
		name: "syscall getenv",
		src: `package main
import "syscall"
func f() (string, bool) { return syscall.Getenv("YOMIHON_SECRET") }`,
		want: []string{"syscall.Getenv reads the environment beneath the os package"},
	},
	{
		name: "syscall environ",
		src: `package main
import "syscall"
func f() []string { return syscall.Environ() }`,
		want: []string{"syscall.Environ reads the whole environment"},
	},
	{
		name: "os expandenv",
		src: `package main
import "os"
func f() string { return os.ExpandEnv("$YOMIHON_SECRET") }`,
		want: []string{"os.ExpandEnv reads every variable named in its argument"},
	},
	{
		name: "getenv passed as a value",
		src: `package main
import "os"
func f() string { return os.Expand("$YOMIHON_SECRET", os.Getenv) }`,
		want: []string{
			"os.Expand reads the environment through a mapping this guard cannot follow",
			"os.Getenv referenced as a value, so no key can be checked",
		},
	},
}

// parseEnvFixture parses one control's source, or stops the test that needs it.
func parseEnvFixture(t *testing.T, name, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name+".go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return fset, f
}

// TestEnvOffendersSeesEveryWayAround holds the guard to the shape of the promise
// rather than to the calls the command happens to make today.
func TestEnvOffendersSeesEveryWayAround(t *testing.T) {
	t.Parallel()
	for _, tt := range envFixtures {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fset, f := parseEnvFixture(t, tt.name, tt.src)
			var got []string
			for _, o := range envOffenders(envReads(fset, []*ast.File{f}, allowedEnvKeys)) {
				got = append(got, o.Why)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("envOffenders(%s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

// TestEveryEnvReaderHasAControl closes the gap the controls above cannot see on
// their own. A row of envReaders that no fixture ever reaches through is a row
// nothing would miss: delete it and the guard silently stops watching that
// reader while every test stays green. So each row has to be refused by at least
// one control, and a reader added to the map without one fails here — by
// construction, on the commit that adds it.
func TestEveryEnvReaderHasAControl(t *testing.T) {
	t.Parallel()
	refused := map[string]bool{}
	for _, tt := range envFixtures {
		fset, f := parseEnvFixture(t, tt.name, tt.src)
		for _, o := range envOffenders(envReads(fset, []*ast.File{f}, allowedEnvKeys)) {
			// A finding about an import itself names no symbol.
			if o.Symbol != "" {
				refused[o.Pkg+"."+o.Symbol] = true
			}
		}
	}
	var uncontrolled []string
	for pkg, symbols := range envReaders {
		for symbol := range symbols {
			if !refused[pkg+"."+symbol] {
				uncontrolled = append(uncontrolled, pkg+"."+symbol)
			}
		}
	}
	slices.Sort(uncontrolled)
	if len(uncontrolled) > 0 {
		t.Errorf("these environment readers are watched by the guard but refused by no control, so nothing would notice the day the guard stopped watching them:\n%s",
			strings.Join(uncontrolled, "\n"))
	}
}
