package main_test

// The composition point is where every face is wired together, so it is the
// only honest home for two guards that are about the whole system rather than
// any one feature: that no read or render face ever writes to the vault, and
// that the command reads no environment beyond the two variables it is allowed.
// Neither exercises main's wiring logic; each pins a system-wide invariant that
// has no other place to live.

import (
	"crypto/sha256"
	"errors"
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
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/report"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/syllabus"
	"github.com/koopa0/yomihon/internal/ui/pages"
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
	contract, err := schema.LoadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	store := snapshot.New(root, log, contract.NavigationRoles(), contract.ArtifactPolicy())

	// A fail-closed writing service: reachable by the reading page as a
	// dependency, but never driven here.
	svc := status.NewService(root, nil, schema.ArtifactPolicy{})
	shellForSnapshot := func(snap *snapshot.Snapshot) pages.ShellData { return note.ShellData(svc, snap) }
	shellProvider := func() pages.ShellData { return shellForSnapshot(store.Current()) }
	searchProvider := func() search.RequestSnapshot {
		snap := store.Current()
		return search.RequestSnapshot{Index: snap.Search, Shell: shellForSnapshot(snap)}
	}
	concepts, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("lesson.BuildConceptIndex(%q) = %v", root, err)
	}

	mux := http.NewServeMux()
	note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   render.New(root, store.Resolver()),
		Status:     svc,
		Snapshot:   store.Current,
		Provenance: svc.LastCommitHash,
		Log:        log,
		Concepts:   concepts,
	}).Register(mux)
	search.NewHandler(searchProvider, log).Register(mux)
	syllabus.NewHandler(syllabus.Deps{Shell: shellProvider, Log: log}).Register(mux)
	report.NewHandler(report.Deps{Root: root, Shell: shellProvider, Log: log}).Register(mux)

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
// environment, keyed by import path and then by symbol. A symbol whose reason is
// empty is judged by the key it is called with; every other symbol is refused
// wherever it appears, and its reason says why.
//
// os.Getenv and os.LookupEnv are the two doors, because the command reads its
// two variables through them. os.Environ takes the whole environment at once;
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
	record := func(pos token.Pos, pkg, symbol, why string) {
		reads = append(reads, envRead{Pos: fset.Position(pos), Pkg: pkg, Symbol: symbol, Why: why})
	}
	at := func(pos token.Pos, pkg, symbol, format string, args ...any) {
		record(pos, pkg, symbol, fmt.Sprintf(format, args...))
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

// module is this repository's import path, used to keep the file listing below
// to the packages written here rather than the ones they borrow.
const module = "github.com/koopa0/yomihon"

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
	// Each line is one file of one package in this module that the command links.
	const format = `{{if .Module}}{{if eq .Module.Path "` + module + `"}}` +
		`{{$dir := .Dir}}{{range .GoFiles}}{{$dir}}/{{.}}{{"\n"}}{{end}}{{end}}{{end}}`
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", format, ".")
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("go list -deps .: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("go list -deps .: %v", err)
	}
	var files []*ast.File
	var paths []string
	for path := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if path == "" {
			continue
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		files = append(files, f)
		paths = append(paths, filepath.ToSlash(path))
	}
	// A listing that stopped reaching the command, or stopped reaching the
	// packages it links, would report no offenders and prove nothing at all. Both
	// ends have to be present before an empty result means anything.
	var sawCommand, sawInternal bool
	for _, p := range paths {
		if strings.HasSuffix(p, "cmd/yomihon/main.go") {
			sawCommand = true
		}
		if strings.Contains(p, "/internal/") {
			sawInternal = true
		}
	}
	if !sawCommand || !sawInternal {
		t.Fatalf("the toolchain listed %d files the command is built from, but not both the command itself and the packages it links (command %v, internal %v), so an empty result would mean nothing",
			len(files), sawCommand, sawInternal)
	}
	return files
}

// TestOnlyKnownEnvVarsAreRead fixes the command's configuration surface. The
// listener binds the loopback address and only the port is configurable, so the
// only environment this binary may read are the vault root and the port. The
// test parses every file the command is built from — its own and each package it
// links, as the toolchain reports them — and asserts that every reach for the
// environment, through os or through the syscall package beneath it, called or
// merely handed over as a value, names one of the two allowed keys.
//
// The subject is the binary, not the repository: a tool that lived here and read
// its own configuration would not be breaking this command's promise, and would
// not be linked into it either.
func TestOnlyKnownEnvVarsAreRead(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	offenders := envOffenders(envReads(fset, productionGoFiles(t, fset), allowedEnvKeys))
	if len(offenders) > 0 {
		lines := make([]string, 0, len(offenders))
		for _, o := range offenders {
			lines = append(lines, fmt.Sprintf("%s: %s", o.Pos, o.Why))
		}
		t.Errorf("this command may read only YOMIHON_ROOT and YOMIHON_PORT (the listener binds loopback, only the port configurable), but found:\n%s",
			strings.Join(lines, "\n"))
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
