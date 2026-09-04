package archlock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

// panicShapes are the wordings a panic written here may take, in the order the
// check below reports them. A panic is how this repository says the code is
// wrong rather than the input, and there are three ways for it to be wrong that
// reach one: a value arrived from outside a closed set, the wiring did not
// supply something a constructor needs, and a method that may run once was
// entered again. The list is closed on purpose — a fourth wording is a fourth
// thing the person reading a crash has to recognize — so growing it is an
// edit here, not a choice made at a call site.
var panicShapes = []struct {
	name   string
	says   func(line string) bool
	advice string
}{
	{
		name:   "unknown value",
		says:   func(line string) bool { return strings.Contains(line, "unknown") },
		advice: `panic("<package>: unknown <Type>: " + v.String())`,
	},
	{
		name:   "missing dependency",
		says:   func(line string) bool { return strings.Contains(line, "requires a non-nil") },
		advice: `panic("<package>: <Constructor> requires a non-nil <Field>")`,
	},
	{
		name:   "once-only method entered again",
		says:   func(line string) bool { return strings.HasSuffix(line, ` called twice")`) },
		advice: `panic("<package>: <Type>.<Method> called twice")`,
	},
}

// panicShape names which wording a line carries, and reports whether it carries
// one at all.
func panicShape(line string) (string, bool) {
	for _, shape := range panicShapes {
		if shape.says(line) {
			return shape.name, true
		}
	}
	return "", false
}

// TestALiteralPanicUsesOneOfTheAgreedWordings keeps the messages a programming
// error produces readable, and keeps them few. Every guard of one kind says the
// same sentence, so the person reading a crash on the first request reads a
// field name, a value, or the method they entered twice — rather than a package
// name and a guess.
func TestALiteralPanicUsesOneOfTheAgreedWordings(t *testing.T) {
	t.Parallel()

	seen := make(map[string]int, len(panicShapes))
	var found []site
	for _, s := range productionLines(t) {
		if !strings.Contains(s.text, `panic("`) {
			continue
		}
		if name, ok := panicShape(s.text); ok {
			seen[name]++
			continue
		}
		found = append(found, s)
	}
	var advice []string
	for _, shape := range panicShapes {
		if seen[shape.name] == 0 {
			t.Fatalf("no panic in the tree uses the %s wording, so this check is comparing against nothing", shape.name)
		}
		advice = append(advice, shape.advice)
	}
	report(t, "a literal panic must use one of the agreed wordings: "+strings.Join(advice, ", "), found)
}

// TestAWordingOutsideTheListIsNotAShape asks the predicate directly about
// wordings the check has to reject, because a predicate that accepted
// everything would leave the check above reporting nothing whatever the tree
// said, and the tree passing is the answer that looks the same either way.
func TestAWordingOutsideTheListIsNotAShape(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		line string
		want bool
	}{
		{"a value outside a closed set", `panic("graph: unknown Kind: " + strconv.Itoa(int(k)))`, true},
		{"a dependency the wiring did not supply", `panic("note: New requires a non-nil Sources")`, true},
		{"a once-only method entered again", `panic("snapshot: Store.Run called twice")`, true},
		{"a sentence nobody agreed on", `panic("snapshot: the store is in a state it should not be in")`, false},
		{"the third idea said another way", `panic("snapshot: Store.Run is already running")`, false},
		{"the third wording with nothing said before it", `panic("called twice")`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, ok := panicShape(tt.line); ok != tt.want {
				t.Errorf("panicShape(%q) = %v, want %v", tt.line, ok, tt.want)
			}
		})
	}
}

// httpPackageNames reports every name net/http answers to in one file. It is
// almost always the one name, and the cases worth resolving rather than
// assuming are the others: a file that imports it under another name writes
// calls no pattern over the text will recognize, and a file that imports it
// twice answers to both, so stopping at the first name found would leave the
// second one unread.
func httpPackageNames(file *ast.File) (names []string, readable bool) {
	for _, spec := range file.Imports {
		if spec.Path == nil || spec.Path.Value != `"net/http"` {
			continue
		}
		switch {
		case spec.Name == nil:
			names = append(names, "http")
		case spec.Name.Name == ".":
			// The calls lose their qualifier entirely, so this file cannot be
			// read by the walk below.
			return nil, false
		case spec.Name.Name == "_":
			// Imported for its initialisers; nothing here is callable through it.
		default:
			names = append(names, spec.Name.Name)
		}
	}
	return names, true
}

// carriesWrittenSentence reports whether an argument is words written at the
// call rather than words fetched from somewhere. A literal is, so is a literal
// joined to something else, and so is a format string, which is the sentence
// however much is interpolated into it. An ordinary call is not, even one whose
// own arguments hold literals — those are keys and names, not the sentence.
//
// It reads the expression it is given and nothing before it, so a sentence
// bound to a name first — a constant, or a local assigned on the line above —
// reads as words fetched from somewhere. Following those would mean resolving
// identifiers across the file and the package, which is a different instrument
// from this one; the table below states that boundary rather than leaving it to
// be discovered.
func carriesWrittenSentence(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.ParenExpr:
		return carriesWrittenSentence(e.X)
	case *ast.BinaryExpr:
		return e.Op == token.ADD && (carriesWrittenSentence(e.X) || carriesWrittenSentence(e.Y))
	case *ast.CallExpr:
		if selector, ok := e.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Sprintf" {
			return len(e.Args) > 0 && carriesWrittenSentence(e.Args[0])
		}
		return false
	default:
		return false
	}
}

// responseSentenceSites finds, in one file, the calls that put words in front
// of a reader without going through the dictionary, and counts the calls that
// take their words from somewhere else, so a caller can tell an empty answer
// from an unasked question.
func responseSentenceSites(path string, fset *token.FileSet, file *ast.File) (found []site, fromElsewhere int) {
	names, readable := httpPackageNames(file)
	if readable && len(names) == 0 {
		return nil, 0
	}
	if !readable {
		return []site{{
			path: path,
			line: fset.Position(file.Pos()).Line,
			text: "net/http is imported into this file's own namespace, so its calls cannot be read here",
		}}, 0
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || !slices.Contains(names, qualifier.Name) {
			return true
		}
		line := fset.Position(call.Pos()).Line
		switch {
		case selector.Sel.Name == "NotFound":
			found = append(found, site{path: path, line: line, text: "NotFound writes the standard library's own sentence"})
		case selector.Sel.Name != "Error" || len(call.Args) < 2:
		case carriesWrittenSentence(call.Args[1]):
			found = append(found, site{path: path, line: line, text: "the body is written at this call"})
		default:
			fromElsewhere++
		}
		return true
	})
	return found, fromElsewhere
}

// TestNoResponseCarriesASentenceWrittenInTheSource keeps every string a reader
// can see inside the dictionary that has both languages of it. A sentence
// written in a handler is the one that gets missed, because it looks like code.
//
// It reads the call rather than the line. The line is written differently by
// different hands — the response parameter is named w almost everywhere and
// nothing makes it so, and net/http can be imported under another name — and a
// check that matches one spelling is a check a rename walks through in
// silence, with every gate still green.
func TestNoResponseCarriesASentenceWrittenInTheSource(t *testing.T) {
	t.Parallel()

	var found []site
	fromElsewhere := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		sites, conforming := responseSentenceSites(path, fset, file)
		found = append(found, sites...)
		fromElsewhere += conforming
	})
	if fromElsewhere == 0 {
		t.Fatal("no handler takes its words from anywhere but its own source, so this check is comparing against nothing")
	}
	report(t, "a response body a reader sees must come from internal/wording, not from a sentence written here", found)
}

// TestTheResponseCheckReadsTheCallNotItsSpelling asks the walk directly about
// the shapes it has to separate, because a tree that passes looks the same
// whether the check is strict or blind. The two rows that matter are the ones
// the previous check let through: it matched the text `http.Error(w, "`, so
// renaming the parameter was enough to hide a hardcoded sentence from it.
func TestTheResponseCheckReadsTheCallNotItsSpelling(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		imports       string
		body          string
		wantFound     int
		wantElsewhere int
	}{
		{
			name:      "a sentence written at the call",
			imports:   `"net/http"`,
			body:      `func h(w http.ResponseWriter) { http.Error(w, "boom", 500) }`,
			wantFound: 1,
		},
		{
			name:      "the same call with the writer named otherwise",
			imports:   `"net/http"`,
			body:      `func h(rw http.ResponseWriter) { http.Error(rw, "boom", 500) }`,
			wantFound: 1,
		},
		{
			name:      "the same call through an aliased import",
			imports:   `nethttp "net/http"`,
			body:      `func h(rw nethttp.ResponseWriter) { nethttp.Error(rw, "boom", 500) }`,
			wantFound: 1,
		},
		{
			name:      "a sentence joined to something computed",
			imports:   `"net/http"`,
			body:      `func h(w http.ResponseWriter, err error) { http.Error(w, "boom: "+err.Error(), 500) }`,
			wantFound: 1,
		},
		{
			name:      "the standard library's own not-found body",
			imports:   `"net/http"`,
			body:      `func h(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }`,
			wantFound: 1,
		},
		{
			name:          "words fetched from somewhere else",
			imports:       `"net/http"`,
			body:          `func h(w http.ResponseWriter, d fmt.Stringer) { http.Error(w, d.String(), 500) }`,
			wantElsewhere: 1,
		},
		{
			name:          "a helper whose own arguments are keys, not the sentence",
			imports:       `"net/http"`,
			body:          `func h(w http.ResponseWriter, l int) { http.Error(w, malformed("identity", l), 500) }`,
			wantElsewhere: 1,
		},
		{
			name:      "a sentence handed to a format call first",
			imports:   `"net/http"`,
			body:      `func h(w http.ResponseWriter, n int) { http.Error(w, fmt.Sprintf("boom %d", n), 500) }`,
			wantFound: 1,
		},
		{
			name:      "the same package imported under two names",
			imports:   "\"net/http\"\n\tnethttp \"net/http\"",
			body:      `func h(rw nethttp.ResponseWriter) { nethttp.Error(rw, "boom", 500) }`,
			wantFound: 1,
		},
		{
			// Known and not covered: the walk reads the argument, and by then
			// the sentence is a name. Catching it means resolving identifiers,
			// which is a different instrument. The row is here so the boundary
			// is written down rather than found later, and so that closing it
			// has to come back and change this line.
			name:          "a sentence bound to a name before the call",
			imports:       `"net/http"`,
			body:          `func h(w http.ResponseWriter) { body := "boom"; http.Error(w, body, 500) }`,
			wantElsewhere: 1,
		},
		{
			name:    "a file that never imports it",
			imports: `"strings"`,
			body:    `func h() string { return strings.TrimSpace(" x ") }`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			source := "package probe\n\nimport (\n\t" + tt.imports + "\n)\n\n" + tt.body + "\n"
			file, err := parser.ParseFile(fset, "probe.go", source, 0)
			if err != nil {
				t.Fatalf("parse the probe source: %v", err)
			}
			found, fromElsewhere := responseSentenceSites("probe.go", fset, file)
			if len(found) != tt.wantFound {
				t.Errorf("reported %d sites, want %d: %v", len(found), tt.wantFound, found)
			}
			if fromElsewhere != tt.wantElsewhere {
				t.Errorf("counted %d answers taken from elsewhere, want %d", fromElsewhere, tt.wantElsewhere)
			}
		})
	}
}

// TestNoScriptCarriesASentenceOfItsOwn keeps the client's words where the
// server's are. A script is one file for every reader, so a sentence written
// into one is written in a single language and reaches the reader who asked
// for the other in the middle of a page that is otherwise theirs — which is
// what a live search's result count, a rail filter's overflow notice and a
// read-aloud bar all used to do.
//
// The rule is stricter than the fault, and deliberately: no CJK anywhere in
// these files, comments included. The interface's words live in the
// dictionary, so a comment quoting one is a second copy of it, and a carve-out
// for comments would have to decide what is a comment in a language where a
// pair of slashes lives inside every URL. Describe the Japanese a passage
// carries; do not paste it.
func TestNoScriptCarriesASentenceOfItsOwn(t *testing.T) {
	t.Parallel()

	scripts := productionFiles(t, ".js")
	if len(scripts) == 0 {
		t.Fatal("no script was read at all, so this check asserts nothing")
	}
	for _, path := range scripts {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path this walk produced under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, r := range line {
				if cjk(r) {
					t.Errorf("%s:%d a reader's words belong in internal/wording, rendered onto the element this script reads them from\n\t%s",
						path, i+1, strings.TrimSpace(line))
					break
				}
			}
		}
	}
}

// cjk reports whether a rune is one only a sentence would carry: the two kana
// blocks, the ideographs, CJK punctuation, and the fullwidth forms.
func cjk(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303f, // CJK punctuation
		r >= 0x3040 && r <= 0x30ff, // hiragana and katakana
		r >= 0x4e00 && r <= 0x9fff, // unified ideographs
		r >= 0xff00 && r <= 0xffef: // fullwidth and halfwidth forms
		return true
	}
	return false
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

// fragmentAddressing is where a name written after a link's "#" is turned into
// the id it has to match. It lives in one package because a link and the heading
// it names have to fold the same way to meet at all.
const fragmentAddressing = "internal/graph/"

// TestOneOwnerFoldsAFragment keeps the two halves of fragment addressing — the
// fold that decides which spellings of a name are one name, and the id a page
// stamps for a section — written once.
//
// They were written twice, once for the page and once for the adjudicator, in
// two packages, under four names, and the copies agreed only because they were
// typed from each other. A drift between them does not fail anything: the page
// stamps one id, the adjudicator asks for another, and the reader is told a
// section is missing from a page that is serving it. The copies are found by the
// bytes that make them what they are rather than by their names, since a
// re-introduced copy would be given a new name and nothing else.
func TestOneOwnerFoldsAFragment(t *testing.T) {
	t.Parallel()

	for _, spelling := range []struct{ what, bytes string }{
		{"the fold that decides which spellings of a name are one name", "strings.ToLower(vault.NormalizeNFC("},
		{"the run of characters a section id collapses", `[^\p{L}\p{N}]+`},
	} {
		t.Run(spelling.what, func(t *testing.T) {
			t.Parallel()
			written := findLines(t, func(line string) bool { return strings.Contains(line, spelling.bytes) })
			if len(written) == 0 {
				t.Fatalf("nothing in the tree writes %q any more, so this check passes for the wrong reason", spelling.bytes)
			}
			var elsewhere []site
			for _, s := range written {
				if !strings.HasPrefix(s.path, fragmentAddressing) {
					elsewhere = append(elsewhere, s)
				}
			}
			report(t, "fragment addressing is "+fragmentAddressing+"'s; call it rather than writing a second copy", elsewhere)
		})
	}
}

// escapingVaultPathSegments is the shape of a loop that percent-escapes a vault
// path one segment at a time, and pathSegmentEscapers is how many places in the
// tree are allowed to write one.
const (
	escapingVaultPathSegments = "segments[i] = url.PathEscape("
	pathSegmentEscapers       = 2
)

// TestAVaultPathIsEscapedInTwoPlaces holds the number of copies of the one
// operation a URL to a note depends on.
//
// Escaping a path whole rather than a segment at a time turns a note whose name
// carries a slash into a path, which is the sort of mistake that gets fixed in
// the copy someone happens to be reading. There were three copies; there are two,
// and the second is there because the renderer may not import the pages it is
// rendered into. The count is written down rather than described, so a fourth
// fails here instead of drifting quietly.
func TestAVaultPathIsEscapedInTwoPlaces(t *testing.T) {
	t.Parallel()

	written := findLines(t, func(line string) bool { return strings.Contains(line, escapingVaultPathSegments) })
	if len(written) == pathSegmentEscapers {
		return
	}
	for _, s := range written {
		t.Logf("%s:%d %s", s.path, s.line, s.text)
	}
	t.Errorf("%d places escape a vault path a segment at a time, want %d; call the one that already does rather than writing another",
		len(written), pathSegmentEscapers)
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

// testNamePattern is a word shaped like a test in this module: the prefix
// followed by a capital, which is what a reader writing about one types.
var testNamePattern = regexp.MustCompile(`^Test[A-Z]\w*`)

// everyGoFile lists this module's own source, tests included — which is the one
// difference from productionFiles, and the reason it exists: the names this
// check is about are declared and quoted in test files.
func everyGoFile(t *testing.T) []string {
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
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_templ.go") {
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

// TestACommentNamingATestNamesOneThatExists keeps the name written above a test
// attached to the test underneath it. Renaming one is two edits — the function
// and the sentence about it — and the second is the one that gets forgotten,
// leaving a comment describing something the reader then cannot find.
//
// The general Go convention, that a doc comment opens with the name it
// documents, is not what this repository does: test docs here open with prose
// about the behaviour, deliberately. Checking that would report most of the
// tree. So the question is the narrow one, and narrow in a second way worth
// stating: only the first word of a comment line is read, and only where the
// marker is followed by a space. A dead name written mid-sentence is not
// reported, and neither is one behind a bare //. That is where the mistake
// this exists for happens — a name is written at the head of the sentence
// about it — and widening the reading is a change to make when something gets
// past it, not before.
//
// Existence is asked of the whole module rather than the file's own package. A
// comment pointing at a sibling package's test is a working reference and says
// where the property is really held; only a name nothing declares is a dead
// end.
func TestACommentNamingATestNamesOneThatExists(t *testing.T) {
	t.Parallel()

	declared := make(map[string]bool)
	files := everyGoFile(t)
	for _, path := range files {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path this walk produced under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			if after, ok := strings.CutPrefix(line, "func "); ok {
				declared[testNamePattern.FindString(after)] = true
			}
		}
	}
	delete(declared, "")
	if len(declared) == 0 {
		t.Fatal("no test was found in the module, so every name below would read as dead")
	}

	var found []site
	resolved := 0
	for _, path := range files {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path this walk produced under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			comment, ok := strings.CutPrefix(strings.TrimSpace(line), "// ")
			if !ok {
				continue
			}
			name := testNamePattern.FindString(comment)
			if name == "" {
				continue
			}
			if declared[name] {
				resolved++
				continue
			}
			found = append(found, site{path: path, line: i + 1, text: name})
		}
	}
	if resolved == 0 {
		t.Fatal("no comment in the tree names a test that exists, so this check is comparing against nothing")
	}
	report(t, "a comment names a test this module does not declare; rename the sentence with the function, or point it at the test that holds the property", found)
}
