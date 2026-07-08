package main_test

// The composition point is where every face is wired together, so it is the
// only honest home for two guards that are about the whole system rather than
// any one feature: that no read or render face ever writes to the vault, and
// that the command reads no environment beyond the two variables it is allowed.
// Neither exercises main's wiring logic; each pins a system-wide invariant that
// has no other place to live.

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/report"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/syllabus"
)

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
	store := snapshot.New(root, log)
	navProvider := func() *nav.Model { return store.Current().Nav }
	searchProvider := func() *search.Index { return store.Current().Search }
	countsProvider := func() map[string]int { return store.Current().Search.CountByStatus() }
	typeStatusCountsProvider := func() map[search.TypeStatus]int { return store.Current().Search.CountByTypeStatus() }

	// A fail-closed writing service: reachable by the reading page as a
	// dependency, but never driven here.
	svc := status.NewService(root, nil)
	concepts, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("lesson.BuildConceptIndex(%q) = %v", root, err)
	}

	mux := http.NewServeMux()
	note.NewHandler(note.Deps{
		Root:             root,
		Renderer:         render.New(root, store.Resolver()),
		Status:           svc,
		Nav:              navProvider,
		Counts:           countsProvider,
		TypeStatusCounts: typeStatusCountsProvider,
		Provenance:       svc.LastCommitHash,
		Log:              log,
		Concepts:         concepts,
	}).Register(mux)
	search.NewHandler(searchProvider, navProvider, log).Register(mux)
	syllabus.NewHandler(syllabus.Deps{Nav: navProvider, Log: log}).Register(mux)
	report.NewHandler(report.Deps{Root: root, Nav: navProvider, Log: log}).Register(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Drive each read/render route at least once, covering its disk-touching
	// path: the home redirect, notes, a study-path syllabus, search, and both
	// the report shell and its verbatim raw read.
	for _, path := range []string{
		"/",
		"/notes/README.md",
		"/notes/Notes/alpha.md",
		"/notes/Notes/beta.md",
		"/syllabus/Maps/study.md",
		"/search?q=tortoise",
		"/reports/latest.html",
		"/reports/latest.html/raw",
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
// note, two linked notes, a study-path note (a syllabus), and a briefing (the
// verbatim raw read).
func writeSweepFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"README.md":      "# Sweep\n\nHome, linking to [[Alpha]].\n",
		"Notes/alpha.md": "---\ntype: concept\naliases: [Alpha]\n---\n# Alpha\n\nAlpha links to [[Beta]] and mentions a tortoise.\n",
		"Notes/beta.md":  "---\ntype: concept\naliases: [Beta]\n---\n# Beta\n\nBeta body.\n",
		"Maps/study.md":  "---\ntype: study-path\ntitle: Study Path\n---\n# Study Path\n\n- [[Alpha]]\n- [[Beta]]\n",
		"System/reports/daily-briefing/latest.html": "<!doctype html><h1>Daily briefing</h1><p>body</p>\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
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

// drive issues a GET and discards the body; the sweep cares that the request
// touched the vault's read path, not what it returned.
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
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// TestOnlyKnownEnvVarsAreRead fixes the command's configuration surface. The
// listener binds the loopback address and only the port is configurable, so the
// only environment this command may read are the vault root and the port; a
// bind address or host must never become configurable by accident. The test
// parses the command's own source and asserts every environment read —
// os.Getenv or os.LookupEnv — uses one of the two allowed keys, and that
// os.Environ never reads the whole environment.
func TestOnlyKnownEnvVarsAreRead(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{"YOMIHON_ROOT": true, "YOMIHON_PORT": true}
	fset := token.NewFileSet()
	var offenders []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, isIdent := sel.X.(*ast.Ident)
			if !isIdent || pkg.Name != "os" {
				return true
			}
			// Getenv and LookupEnv both take the key as their first argument;
			// Environ reads the whole environment and so is always suspect.
			switch sel.Sel.Name {
			case "Getenv", "LookupEnv":
			case "Environ":
				offenders = append(offenders, fmt.Sprintf("%s: os.Environ reads the whole environment", fset.Position(call.Pos())))
				return true
			default:
				return true
			}
			if len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				offenders = append(offenders, fmt.Sprintf("%s: os.%s called with a non-literal key", fset.Position(call.Pos()), sel.Sel.Name))
				return true
			}
			key, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			if !allowed[key] {
				offenders = append(offenders, fmt.Sprintf("%s: os.%s(%q)", fset.Position(lit.ValuePos), sel.Sel.Name, key))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk cmd/yomihon: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("this command may read only YOMIHON_ROOT and YOMIHON_PORT (the listener binds loopback, only the port configurable), but found:\n%s",
			strings.Join(offenders, "\n"))
	}
}
