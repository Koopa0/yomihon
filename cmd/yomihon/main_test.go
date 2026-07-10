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
	// path: the home redirect, notes, a study-path syllabus, search, both the
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
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Errorf("GET %s = %d, want the route to serve (below 400) so the sweep actually exercises it", url, resp.StatusCode)
	}
}

// allowedEnvKeys are the two variables this command is permitted to read: the
// vault root and the listening port. The listener binds the loopback address,
// so a bind address or host must never become configurable by accident.
var allowedEnvKeys = map[string]bool{"YOMIHON_ROOT": true, "YOMIHON_PORT": true}

// envReaders names every way the two packages that expose one can read the
// environment, keyed by import path and then by symbol. Getenv and LookupEnv
// take the key as their only argument, so a call to either is judged by that
// key and carries no reason here. Every other entry reads more than one
// variable, or reads it through a mapping this guard cannot follow, and so is
// refused wherever it appears — its reason says why.
//
// The list is the set of ways the promise can be broken, not the set of calls
// the command happens to make: syscall sits underneath os and is already
// imported for the termination signal, which makes it the readiest way around a
// guard that watches os alone.
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

// envRead is one place a parsed file reaches for the environment, with the
// reason that reach is not permitted.
type envRead struct {
	Pos token.Position
	Why string
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

// envOffenders reports every environment read in the parsed files that is not an
// os.Getenv or os.LookupEnv call on one of the allowed keys.
//
// A reader mentioned outside a call — os.Expand(s, os.Getenv) hands Getenv over
// as a value — carries no key this guard can read, so a mention is judged as
// well as a call. Because the walk visits a call before the selector inside it,
// a selector already judged as a call is recognised when it comes round again.
func envOffenders(fset *token.FileSet, files []*ast.File, allowed map[string]bool) []envRead {
	var offenders []envRead
	at := func(pos token.Pos, format string, args ...any) {
		offenders = append(offenders, envRead{Pos: fset.Position(pos), Why: fmt.Sprintf(format, args...)})
	}
	for _, f := range files {
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
				at(imp.Pos(), "dot-imports %s, which hides environment reads from this guard", path)
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
					at(n.Pos(), "%s.%s %s", pkg, symbol, why)
					return true
				}
				if len(n.Args) != 1 {
					at(n.Pos(), "%s.%s called with %d arguments, so no single key can be checked", pkg, symbol, len(n.Args))
					return true
				}
				lit, isLit := n.Args[0].(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING {
					at(n.Pos(), "%s.%s called with a non-literal key", pkg, symbol)
					return true
				}
				key, uerr := strconv.Unquote(lit.Value)
				if uerr != nil {
					at(lit.ValuePos, "%s.%s called with an unreadable key %s", pkg, symbol, lit.Value)
					return true
				}
				if !allowed[key] {
					at(lit.ValuePos, "%s.%s(%q)", pkg, symbol, key)
				}
			case *ast.SelectorExpr:
				if judged[n] {
					return true
				}
				pkg, symbol, reads := envReader(n, pkgOf)
				if !reads {
					return true
				}
				at(n.Pos(), "%s.%s referenced as a value, so no key can be checked", pkg, symbol)
			}
			return true
		})
	}
	return offenders
}

// TestOnlyKnownEnvVarsAreRead fixes the command's configuration surface. The
// listener binds the loopback address and only the port is configurable, so the
// only environment this command may read are the vault root and the port. The
// test parses the command's own source and asserts that every reach for the
// environment — through os or through the syscall package beneath it, called or
// merely handed over as a value — names one of the two allowed keys.
func TestOnlyKnownEnvVarsAreRead(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	var files []*ast.File
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
		files = append(files, f)
		return nil
	})
	if err != nil {
		t.Fatalf("walk cmd/yomihon: %v", err)
	}
	// A walk that found nothing would report no offenders and prove nothing.
	if len(files) == 0 {
		t.Fatal("no command source was parsed, so this guard inspected nothing")
	}
	offenders := envOffenders(fset, files, allowedEnvKeys)
	if len(offenders) > 0 {
		lines := make([]string, 0, len(offenders))
		for _, o := range offenders {
			lines = append(lines, fmt.Sprintf("%s: %s", o.Pos, o.Why))
		}
		t.Errorf("this command may read only YOMIHON_ROOT and YOMIHON_PORT (the listener binds loopback, only the port configurable), but found:\n%s",
			strings.Join(lines, "\n"))
	}
}

// TestEnvOffendersSeesEveryWayAround holds the guard above to the shape of the
// promise rather than to the calls the command happens to make today. Each row
// is one way a reader could reach a forbidden variable while the guard stayed
// green; three of them once did. They are the guard's own controls, and they run
// with the ordinary suite, so the guard is watched to fail on every test run
// rather than on the day someone thinks to try.
func TestEnvOffendersSeesEveryWayAround(t *testing.T) {
	t.Parallel()
	tests := []struct {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, tt.name+".go", tt.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse %s: %v", tt.name, err)
			}
			var got []string
			for _, o := range envOffenders(fset, []*ast.File{f}, allowedEnvKeys) {
				got = append(got, o.Why)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("envOffenders(%s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
