package archlock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repoRoot is where this package's tests walk from. The checks below are about
// the repository's own files, so they are rooted at it rather than at a package.
const repoRoot = "../.."

// site is one line of one file, as the checks below report it.
type site struct {
	path string
	line int
	text string
}

// key identifies a site across edits that only move it. A check that keyed on
// the line number would fail every time an unrelated line was inserted above.
func (s site) key() string { return s.path + " | " + s.text }

// openSites are the violations a check knows about and has not fixed. Each one
// must still be found: an entry that matches nothing is a fix that landed
// without its permission being withdrawn, and the check fails until the entry
// is deleted. That is the whole point of writing them here rather than
// narrowing the pattern — a stale exemption is louder than a silent one.
type openSites struct {
	// why says who owns closing these, so a reader meeting a failure knows
	// whether to fix the code or delete the line.
	why  string
	keys []string
}

// stillOpen removes the known violations from found, and reports any entry that
// matched nothing so it can be deleted.
func (o openSites) stillOpen(t *testing.T, found []site) []site {
	t.Helper()

	matched := make(map[string]bool, len(o.keys))
	var remaining []site
	for _, s := range found {
		if slices.Contains(o.keys, s.key()) {
			matched[s.key()] = true
			continue
		}
		remaining = append(remaining, s)
	}
	for _, k := range o.keys {
		if !matched[k] {
			t.Errorf("this violation is gone, so delete its line from the list above:\n\t%q\n(%s)", k, o.why)
		}
	}
	return remaining
}

// report fails with one line per violation, naming where it is and what to do.
func report(t *testing.T, rule string, remaining []site) {
	t.Helper()
	for _, s := range remaining {
		t.Errorf("%s:%d %s\n\t%s", s.path, s.line, rule, s.text)
	}
}

// productionLines yields every line of every Go source file this repository
// ships, skipping tests and generated code. A check over the text answers the
// same question a reader answers by reading, which is what these rules are
// about; the two checks that need the syntax tree ask for it separately.
func productionLines(t *testing.T) []site {
	t.Helper()

	var sites []site
	for _, path := range productionFiles(t, ".go") {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path this walk produced under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			sites = append(sites, site{path: path, line: i + 1, text: strings.TrimSpace(line)})
		}
	}
	if len(sites) == 0 {
		t.Fatal("no production source was read, so every check over it passes for the wrong reason")
	}
	return sites
}

// productionFiles lists the repository's own source files with the given
// extension: no tests, no generated output, nothing under a directory that is
// not this module's source.
func productionFiles(t *testing.T, ext string) []string {
	t.Helper()

	var paths []string
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if d.IsDir() {
			if p != repoRoot && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "bin" || name == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(name) != ext || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_templ.go") {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, p)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk the repository: %v", err)
	}
	slices.Sort(paths)
	return paths
}

// findLines collects every production line the predicate accepts.
func findLines(t *testing.T, accept func(string) bool) []site {
	t.Helper()

	var found []site
	for _, s := range productionLines(t) {
		if accept(s.text) {
			found = append(found, s)
		}
	}
	return found
}

// TestAnExhaustiveSwitchPanicNamesTheValueItDidNotKnow keeps the last arm of a
// switch over a closed set useful. A panic that says only which type it was
// holding leaves whoever meets it in a log with no way to tell which value the
// vault produced, and a %d says a number for something that reads as a word
// everywhere else.
func TestAnExhaustiveSwitchPanicNamesTheValueItDidNotKnow(t *testing.T) {
	t.Parallel()

	open := openSites{
		why: "the switch-panic shapes are being changed by another line of this work; " +
			"when its change lands, these lines stop matching and belong deleted",
		keys: []string{
			`internal/nav/map.go | panic(fmt.Sprintf("nav: unknown graph.Kind %d", res.Kind))`,
			`internal/render/wikilink.go | panic(fmt.Sprintf("render: unknown graph.Kind %d", res.Kind))`,
			`internal/sequence/sequence.go | panic("sequence: unknown Role")`,
			`internal/sequence/sequence.go | panic("sequence: unknown EntryState")`,
		},
	}
	found := findLines(t, func(line string) bool {
		return strings.Contains(line, "panic(") && strings.Contains(line, "unknown") &&
			!strings.Contains(line, `: " +`)
	})
	conforming := findLines(t, func(line string) bool {
		return strings.Contains(line, "panic(") && strings.Contains(line, "unknown") &&
			strings.Contains(line, `: " +`)
	})
	if len(conforming) == 0 {
		t.Fatal("no panic in the tree uses the shape this check asks for, so it is checking a rule nobody follows")
	}
	report(t, `an exhaustive-switch panic must end with the value: panic("pkg: unknown Type: " + v.String())`, open.stillOpen(t, found))
}

// TestAConstructorGuardSaysWhichDependencyWasNil keeps the one message a wiring
// bug produces readable. Every guard in the tree says the same sentence, so the
// person reading a crash on the first request reads a field name rather than a
// package name and a guess.
func TestAConstructorGuardSaysWhichDependencyWasNil(t *testing.T) {
	t.Parallel()

	open := openSites{
		why: "these two guard messages are being corrected by another line of this work",
		keys: []string{
			`internal/render/render.go | panic("render: New requires non-nil Transclusions")`,
			`internal/render/render.go | panic("render: New requires non-nil Titles")`,
		},
	}
	found := findLines(t, func(line string) bool {
		return strings.Contains(line, `panic("`) && !strings.Contains(line, "unknown") &&
			!strings.Contains(line, "requires a non-nil")
	})
	conforming := findLines(t, func(line string) bool {
		return strings.Contains(line, `panic("`) && strings.Contains(line, "requires a non-nil")
	})
	if len(conforming) == 0 {
		t.Fatal("no constructor guard uses the shape this check asks for, so it is checking a rule nobody follows")
	}
	report(t, `a constructor guard must read panic("<package>: <Constructor> requires a non-nil <Field>")`, open.stillOpen(t, found))
}

// TestNoResponseCarriesASentenceWrittenInTheSource keeps every string a reader
// can see inside the dictionary that has both languages of it. A literal in a
// handler is the one that gets missed, because it looks like code.
func TestNoResponseCarriesASentenceWrittenInTheSource(t *testing.T) {
	t.Parallel()

	found := findLines(t, func(line string) bool {
		return strings.Contains(line, `http.Error(w, "`) || strings.Contains(line, "http.NotFound(")
	})
	conforming := findLines(t, func(line string) bool {
		return strings.Contains(line, "http.Error(w, wording.")
	})
	if len(conforming) == 0 {
		t.Fatal("no handler answers from the dictionary, so this check is comparing against nothing")
	}
	report(t, "a response body a reader sees must come from internal/wording, not from a literal here", found)
}

// TestNoSentenceChoosesALanguageBeforeItsReaderArrives keeps the choice with
// the surface that knows who is reading. A phrase resolved at package scope, or
// against a fixed language in a handler, was written for whichever reader the
// author had in mind, and the other one meets it in a language they did not
// choose.
func TestNoSentenceChoosesALanguageBeforeItsReaderArrives(t *testing.T) {
	t.Parallel()

	open := openSites{
		why: "carrying these phrases to their surface is another line of this work",
		keys: []string{
			`internal/snapshot/health.go | var vaultRootLabel = wording.VaultRoot.In(wording.ZhHant)`,
			`internal/status/status.go | CoreUnavailableDiagnostic           = wording.ContractUnavailable.In(wording.ZhHant)`,
			`internal/status/status.go | DurableInstallUnavailableDiagnostic = wording.DurabilityUnsupported.In(wording.ZhHant)`,
			`internal/status/status.go | NoteUnreadableDiagnostic            = wording.NoteStatusUnreadable.In(wording.ZhHant)`,
		},
	}
	fixed := func(line string) bool {
		return strings.Contains(line, ".In(wording.ZhHant)") || strings.Contains(line, ".In(wording.En)")
	}
	found := findLines(t, fixed)
	for _, path := range productionFiles(t, ".templ") {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path this walk produced under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if text := strings.TrimSpace(line); fixed(text) {
				found = append(found, site{path: path, line: i + 1, text: text})
			}
		}
	}
	conforming := findLines(t, func(line string) bool {
		return strings.Contains(line, ".In(wording.LanguageFromRequest(r))") || strings.Contains(line, ".In(lang)")
	})
	if len(conforming) == 0 {
		t.Fatal("nothing in the tree resolves a phrase from the request, so this check is comparing against nothing")
	}
	report(t, "a sentence must carry both languages until the surface that knows the reader resolves it", open.stillOpen(t, found))
}

// TestEveryExportedNumericEnumCanSayItsOwnName keeps a closed set printable.
// Without it a log line, a panic and a rendered fault all fall back to the
// number, and a number means nothing to anyone who does not have the constant
// block open beside them.
func TestEveryExportedNumericEnumCanSayItsOwnName(t *testing.T) {
	t.Parallel()

	open := openSites{
		why: "giving these five their String method is another line of this work",
		keys: []string{
			"internal/graph/graph.go | Kind",
			"internal/judge/command.go | Format",
			"internal/judge/judge.go | Severity",
			"internal/nav/entry.go | EntryKind",
			"internal/schema/grant.go | Reason",
		},
	}
	numeric := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	}

	var found []site
	total := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		stringers := make(map[string]bool)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "String" || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			stringers[receiverTypeName(fn.Recv.List[0].Type)] = true
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				ident, ok := ts.Type.(*ast.Ident)
				if !ok || !numeric[ident.Name] {
					continue
				}
				total++
				if !stringers[ts.Name.Name] {
					found = append(found, site{path: path, line: fset.Position(ts.Pos()).Line, text: ts.Name.Name})
				}
			}
		}
	})
	if total == 0 {
		t.Fatal("no exported numeric enum was found at all, so this check asserts nothing")
	}
	report(t, "an exported enum over a numeric type must have a String method", open.stillOpen(t, found))
}

// TestAContextIsAlwaysTheFirstParameter keeps one shape for the value every
// cancellable call carries. A signature that hides it in the middle is read
// past, and the call that should have been cancellable is the one that was not.
func TestAContextIsAlwaysTheFirstParameter(t *testing.T) {
	t.Parallel()

	var found []site
	carrying := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			// Only the function type is examined: a declaration's own
			// signature is one of these, so matching both would count every
			// declared function twice.
			fn, ok := n.(*ast.FuncType)
			if !ok || fn.Params == nil {
				return true
			}
			params := fn.Params
			index := 0
			for _, field := range params.List {
				names := max(len(field.Names), 1)
				if isContext(field.Type) {
					carrying++
					if index != 0 {
						found = append(found, site{
							path: path,
							line: fset.Position(field.Pos()).Line,
							text: "context.Context is parameter " + string(rune('1'+index)),
						})
					}
				}
				index += names
			}
			return true
		})
	})
	if carrying == 0 {
		t.Fatal("nothing in the tree takes a context, so this check asserts nothing")
	}
	report(t, "context.Context must be the first parameter", found)
}

// TestNoTestFileCarriesTheInternalSuffixWithoutNeedingIt keeps one name for one
// kind of file. The suffix exists for a package that needs an internal and an
// external test file under the same stem; used anywhere else it distinguishes
// nothing and leaves two conventions in one tree.
func TestNoTestFileCarriesTheInternalSuffixWithoutNeedingIt(t *testing.T) {
	t.Parallel()

	suffixed := 0
	paired := 0
	var found []site
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if p != repoRoot && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_internal_test.go") {
			return nil
		}
		suffixed++
		rel, relErr := filepath.Rel(repoRoot, p)
		if relErr != nil {
			return relErr
		}
		sibling := strings.TrimSuffix(p, "_internal_test.go") + "_test.go"
		if _, statErr := os.Stat(sibling); statErr == nil {
			paired++
			return nil
		}
		found = append(found, site{path: filepath.ToSlash(rel), line: 1, text: d.Name()})
		return nil
	})
	if err != nil {
		t.Fatalf("walk the repository: %v", err)
	}
	if suffixed == 0 || paired == 0 {
		t.Fatalf("suffixed files = %d, of them paired = %d; with either at zero this check has nothing to tell apart", suffixed, paired)
	}
	report(t, "this file has no same-stem external sibling, so the suffix distinguishes nothing: name it <feature>_test.go", found)
}

// forEachProductionFile parses every shipped Go file once and hands it over.
func forEachProductionFile(t *testing.T, visit func(path string, fset *token.FileSet, file *ast.File)) {
	t.Helper()

	fset := token.NewFileSet()
	paths := productionFiles(t, ".go")
	if len(paths) == 0 {
		t.Fatal("no production source was parsed, so every check over the syntax tree passes for the wrong reason")
	}
	for _, path := range paths {
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		visit(path, fset, file)
	}
}

// receiverTypeName is the name of the type a method is declared on, whether the
// receiver is the value or a pointer to it.
func receiverTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if index, ok := expr.(*ast.IndexExpr); ok {
		expr = index.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// isContext reports whether a parameter's type is context.Context.
func isContext(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Context" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "context"
}
