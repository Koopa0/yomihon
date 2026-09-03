package archlock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
		// A <type>_string.go is a stringer's output. The checks over text are
		// about how this repository writes, and nobody writes that file.
		if strings.HasSuffix(path, "_string.go") {
			continue
		}
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
// vault produced, and a number says a number for something that reads as a word
// everywhere else — so a message the reader has to decode against a constant
// block they do not have open is only half an answer.
//
// The one place a number is the whole answer is the method that produces the
// names. It is exempted by shape rather than by a list: a panic over the
// receiver of the method it sits in is that method, and it cannot ask itself
// for a name it is in the middle of failing to supply.
func TestAnExhaustiveSwitchPanicNamesTheValueItDidNotKnow(t *testing.T) {
	t.Parallel()

	naming := panicsOverTheirOwnReceiver(t)
	var found []site
	conforming := 0
	for _, s := range productionLines(t) {
		if !strings.Contains(s.text, "panic(") || !strings.Contains(s.text, "unknown") {
			continue
		}
		if naming[s.path+":"+strconv.Itoa(s.line)] {
			continue
		}
		namesTheValue := strings.Contains(s.text, ".String()") || strings.Contains(s.text, "+ string(")
		asANumber := strings.Contains(s.text, "strconv.Itoa(int(") || strings.Contains(s.text, "%d")
		if namesTheValue && !asANumber {
			conforming++
			continue
		}
		found = append(found, s)
	}
	if conforming == 0 {
		t.Fatal("no panic in the tree uses the shape this check asks for, so it is checking a rule nobody follows")
	}
	report(t, `an exhaustive-switch panic must end with the value's own name: panic("pkg: unknown Type: " + v.String())`, found)
}

// panicsOverTheirOwnReceiver locates, as "path:line", every panic whose message
// mentions the receiver of the method it is written in. Those are the sites
// asked for a value's name, so a number there is the honest answer rather than
// an unread one.
func panicsOverTheirOwnReceiver(t *testing.T) map[string]bool {
	t.Helper()

	sites := make(map[string]bool)
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) != 1 || len(fn.Recv.List[0].Names) != 1 {
				continue
			}
			receiver := fn.Recv.List[0].Names[0].Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				if name, ok := call.Fun.(*ast.Ident); !ok || name.Name != "panic" {
					return true
				}
				mentions := false
				ast.Inspect(call.Args[0], func(arg ast.Node) bool {
					if id, ok := arg.(*ast.Ident); ok && id.Name == receiver {
						mentions = true
					}
					return true
				})
				if mentions {
					sites[path+":"+strconv.Itoa(fset.Position(call.Pos()).Line)] = true
				}
				return true
			})
		}
	})
	return sites
}

// TestAConstructorGuardSaysWhichDependencyWasNil keeps the one message a wiring
// bug produces readable. Every guard in the tree says the same sentence, so the
// person reading a crash on the first request reads a field name rather than a
// package name and a guess.
func TestAConstructorGuardSaysWhichDependencyWasNil(t *testing.T) {
	t.Parallel()

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
	report(t, `a constructor guard must read panic("<package>: <Constructor> requires a non-nil <Field>")`, found)
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
		return strings.Contains(line, ".In(origin.Language(r))") || strings.Contains(line, ".In(lang)")
	})
	if len(conforming) == 0 {
		t.Fatal("nothing in the tree resolves a phrase from the request, so this check is comparing against nothing")
	}
	report(t, "a sentence must carry both languages until the surface that knows the reader resolves it", found)
}

// TestEveryExportedNumericEnumCanSayItsOwnName keeps a closed set printable.
// Without it a log line, a panic and a rendered fault all fall back to the
// number, and a number means nothing to anyone who does not have the constant
// block open beside them.
//
// The method is looked for across the whole package rather than beside the type,
// because a package is the unit a method belongs to: a hand-written one in a
// role-named file and a generated one in <type>_string.go both answer for the
// type, and requiring the same file would report a type that already has a name.
func TestEveryExportedNumericEnumCanSayItsOwnName(t *testing.T) {
	t.Parallel()

	numeric := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	}

	stringers := make(map[string]bool)
	forEachProductionFile(t, func(path string, _ *token.FileSet, file *ast.File) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "String" || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			stringers[filepath.Dir(path)+"."+receiverTypeName(fn.Recv.List[0].Type)] = true
		}
	})

	var found []site
	total := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
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
				if !stringers[filepath.Dir(path)+"."+ts.Name.Name] {
					found = append(found, site{path: path, line: fset.Position(ts.Pos()).Line, text: ts.Name.Name})
				}
			}
		}
	})
	if total == 0 {
		t.Fatal("no exported numeric enum was found at all, so this check asserts nothing")
	}
	report(t, "an exported enum over a numeric type must have a String method", found)
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
