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

// outsideTheSource reports whether a directory holds nothing this module
// builds or tests. docs/ is among them: its tracked files are prose, and the
// maintainer's local design material sits there untracked.
func outsideTheSource(p, name string) bool {
	if p == repoRoot {
		return false
	}
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == "bin" || name == "testdata" ||
		p == filepath.Join(repoRoot, "docs")
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
			if outsideTheSource(p, name) {
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

// everyGoFile lists this module's own Go source with the tests included, for a
// rule that holds in test files too.
func everyGoFile(t *testing.T) []string {
	t.Helper()

	var paths []string
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if d.IsDir() {
			if outsideTheSource(p, name) {
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

// TestAConditionalClassOutranksTheClassItModifies covers the one thing that
// decides which of two colours a reader sees. A mark is written as a base class
// and a conditional one on the same element; both are single class selectors,
// so neither outranks the other and the order of the stylesheet settles it. A
// modifier declared above the class it modifies loses in silence: the markup is
// right, a check that reads the markup is right, and the colour on the screen
// is the other one.
//
// What it covers is bounded and worth stating: of the conditional classes
// the templates write, the pairs that both set a colour are what this
// compares — both the templ.KV form and a plain class="…" literal that puts
// a base and a modifier on the same element. The rest are conditions on
// layout or spacing, where order settles nothing a reader would notice. A
// modifier that starts setting a colour joins the comparison by doing so;
// the guard below fails rather than passing quietly if that set ever empties.
func TestAConditionalClassOutranksTheClassItModifies(t *testing.T) {
	t.Parallel()

	coloured := colouredClasses(t)
	pairs := conditionalClassPairs(t)
	if len(pairs) == 0 {
		t.Fatal("no conditional class was found in any template, so this compares nothing")
	}
	compared := 0
	for _, pair := range pairs {
		baseAt, baseColours := coloured[pair.base]
		modifierAt, modifierColours := coloured[pair.modifier]
		if !baseColours || !modifierColours {
			continue
		}
		compared++
		if modifierAt < baseAt {
			t.Errorf("%s:%d .%s is declared before the .%s it modifies, so the base colour wins and the condition changes nothing a reader can see",
				pair.path, pair.line, pair.modifier, pair.base)
		}
	}
	if compared == 0 {
		t.Fatal("no pair had both of its classes setting a colour, so this compares nothing")
	}
}

// classPair is one element's base class and one class applied to it by
// condition, as a template writes them together.
type classPair struct {
	path     string
	line     int
	base     string
	modifier string
}

// conditionalClassPairs reads every class attribute a template writes, and
// pairs each modifier with the base it sits beside. A templ.KV class is
// paired with every unconditional class in the same class={…} list. A plain
// class="…" literal is paired when one name carries a BEM `--` modifier and
// the other does not: that is the form that puts a base and a condition on
// the same element as a string, which the KV reader cannot see.
func conditionalClassPairs(t *testing.T) []classPair {
	t.Helper()

	attribute := regexp.MustCompile(`class=\{([^}]*)\}`)
	literal := regexp.MustCompile(`"([A-Za-z0-9_ -]+)"`)
	conditional := regexp.MustCompile(`templ\.KV\("([A-Za-z0-9_-]+)"`)
	literalAttr := regexp.MustCompile(`class="([^"]*)"`)

	var pairs []classPair
	for _, path := range productionFiles(t, ".templ") {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path this walk produced under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, attr := range attribute.FindAllStringSubmatch(line, -1) {
				modifiers := make([]string, 0, 2)
				for _, m := range conditional.FindAllStringSubmatch(attr[1], -1) {
					modifiers = append(modifiers, m[1])
				}
				if len(modifiers) == 0 {
					continue
				}
				for _, l := range literal.FindAllStringSubmatch(attr[1], -1) {
					for class := range strings.FieldsSeq(l[1]) {
						if slices.Contains(modifiers, class) {
							continue
						}
						for _, modifier := range modifiers {
							pairs = append(pairs, classPair{path: path, line: i + 1, base: class, modifier: modifier})
						}
					}
				}
			}
			for _, attr := range literalAttr.FindAllStringSubmatch(line, -1) {
				classes := strings.Fields(attr[1])
				for iA, a := range classes {
					for _, b := range classes[iA+1:] {
						base, modifier, ok := bemPair(a, b)
						if !ok {
							continue
						}
						pairs = append(pairs, classPair{path: path, line: i + 1, base: base, modifier: modifier})
					}
				}
			}
		}
	}
	return pairs
}

// bemPair reports a base and a modifier when exactly one of the two class
// names carries a BEM `--` suffix. Two names that both have one, or neither,
// are siblings rather than a condition, and pairing them would compare
// unrelated colours that happen to share an element.
func bemPair(a, b string) (base, modifier string, ok bool) {
	aMod := strings.Contains(a, "--")
	bMod := strings.Contains(b, "--")
	if aMod == bMod {
		return "", "", false
	}
	if aMod {
		return b, a, true
	}
	return a, b, true
}

// colouredClasses maps each class whose own rule sets a colour to where that
// rule sits in the stylesheet. Only rules written as one class are read: a
// compound selector already outranks a single one, so the order does not decide
// it.
func colouredClasses(t *testing.T) map[string]int {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, "assets", "css", "components.css"))
	if err != nil {
		t.Fatalf("read the stylesheet: %v", err)
	}
	rule := regexp.MustCompile(`(?m)^\.([A-Za-z0-9_-]+)\s*\{([^}]*)\}`)
	sets := regexp.MustCompile(`(^|[;{])\s*color:`)
	at := make(map[string]int)
	for _, m := range rule.FindAllStringSubmatchIndex(string(data), -1) {
		class := string(data[m[2]:m[3]])
		body := string(data[m[4]:m[5]])
		if !sets.MatchString(body) {
			continue
		}
		if _, seen := at[class]; seen {
			continue
		}
		at[class] = m[0]
	}
	if len(at) == 0 {
		t.Fatal("no single-class colour rule was read from the stylesheet, so this compares nothing")
	}
	return at
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

// TestTheCJKPredicateAnswersBothWays locks the predicate itself. The checks
// that use it are looking for something the tree is not supposed to contain, so
// they are green when it finds nothing and green when it can no longer find
// anything — a predicate that always answered false would leave every one of
// them passing, and there is no CJK in the tree to notice its absence.
//
// The rows below are the four blocks it claims, one character each, and the
// scripts that must stay outside it: a check that fired on ordinary Latin text
// or on the accented letters a European name carries would report every script
// in the repository.
func TestTheCJKPredicateAnswersBothWays(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		r    rune
		want bool
	}{
		{"CJK punctuation", '\u3001', true},
		{"hiragana", 'の', true},
		{"katakana", 'ハ', true},
		{"a unified ideograph", '語', true},
		{"a fullwidth form", '\uff21', true},
		{"a Latin letter", 'a', false},
		{"an ASCII digit", '7', false},
		{"ordinary punctuation", ',', false},
		{"a letter with a diacritic", 'á', false},
		{"an emoji, which is not a sentence", '\U0001f600', false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cjk(tt.r); got != tt.want {
				t.Errorf("cjk(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
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
		{"the run of characters a section id keeps", `[\p{L}\p{N}][^\P{M}\p{Me}\x{FE0E}\x{FE0F}]*`},
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

// TestOneOwnerScansALine keeps each line-scan spelling written once, under the
// package that owns it.
//
// The page and the check each kept a copy. The copies were byte-identical on
// eight patterns and then they were not: a callout title and a self-closing
// <pre/> were an address on one face and missing on the other, so a reader
// was told a link was fine to a place the page had already marked broken.
// The scan is found by the bytes that make it a scan rather than by its type
// name, since the two copies already disagreed about what to call it. Each
// row names its owner: the heading, fence, quote-prefix, HTML-block, and
// Obsidian comment-pairing patterns live in internal/graph/; the callout
// opening, the closed type list, and the line no address can survive on live
// in internal/render/,
// because only that package holds the vocabulary the page answers to.
func TestOneOwnerScansALine(t *testing.T) {
	t.Parallel()

	for _, spelling := range []struct{ what, bytes, owner string }{
		{"an ATX heading the scan recognises", `^ {0,3}(#{1,6})[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$`, "internal/graph/"},
		{"the HTML block tags a line scan can open without paragraph state", `address|article|aside|base|basefont|blockquote`, "internal/graph/"},
		{"the scan's running HTML-block close test", "htmlCloses func(string) bool", "internal/graph/"},
		{"an Obsidian callout's opening line", `^\s*>\s*\[!([A-Za-z0-9_-]+)\]([+-]?)\s?(.*)$`, "internal/render/"},
		{"the single leading quote marker a line scan peels", `^\s*>\s?`, "internal/graph/"},
		{"a backtick fence the scan opens", "strings.HasPrefix(t, \"```\")", "internal/graph/"},
		{"a tilde fence the scan opens", `strings.HasPrefix(t, "~~~")`, "internal/graph/"},
		{"a fence-close line the scan recognises", `strings.Count(t, string(marker)) == len(t)`, "internal/graph/"},
		{"an unpaired Obsidian comment running to the end of the body", "Span{Start: start, Stop: len(body)}", "internal/graph/"},
		{"the pairing of a closed Obsidian comment mark", "stop = off + end + len(closer)", "internal/graph/"},
		{"the function that refuses a line no block address can survive on", "func UnanchorableLine(line string) bool", "internal/render/"},
		{"the grammar that extracts a supported block address", "func BlockAddress(line string) string", "internal/render/"},
		{"the function that refuses a caret a code span owns as an address", "func CodeSpanOwnedAddresses(lines []string) []bool", "internal/render/"},
		{"the first group of callout types the page answers to", `"info", "note", "tip", "important", "hint", "abstract", "tldr", "summary", "todo"`, "internal/render/"},
	} {
		t.Run(spelling.what, func(t *testing.T) {
			t.Parallel()
			written := findLines(t, func(line string) bool { return strings.Contains(line, spelling.bytes) })
			if len(written) == 0 {
				t.Fatalf("nothing in the tree writes %q any more, so this check passes for the wrong reason", spelling.bytes)
			}
			var elsewhere []site
			for _, s := range written {
				if !strings.HasPrefix(s.path, spelling.owner) {
					elsewhere = append(elsewhere, s)
				}
			}
			report(t, spelling.what+" is "+spelling.owner+"'s; call it rather than writing a second copy", elsewhere)
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

// TestProjectionPackagesHaveNoLegacyEntryPoints keeps the shape the projection
// packages settled on. Each of them used to expose a package-level entry point
// that walked the vault itself; they are built behind the generation store now,
// and a free function with one of those names would be a second way in that
// nothing else knows about.
//
// The check lives here rather than beside any one of the packages it reads,
// because it is about how the module is arranged: a failure in it is news for
// whoever is changing the arrangement, and reporting it from a package whose
// own behaviour is fine sends them to the wrong address.
func TestProjectionPackagesHaveNoLegacyEntryPoints(t *testing.T) {
	t.Parallel()

	forbiddenByDir := map[string]map[string]bool{
		"internal/vault": {
			"List":              true,
			"ListStrict":        true,
			"ListStrictContext": true,
			"ReadNote":          true,
		},
		"internal/graph":   {"Build": true},
		"internal/lesson":  {"BuildConceptIndex": true, "BuildSlotIndex": true, "LoadConcept": true, "readSidecar": true},
		"internal/nav":     {"Build": true},
		"internal/lexical": {"Build": true},
	}
	var found []string
	for dir, forbidden := range forbiddenByDir {
		entries, err := os.ReadDir(filepath.Join(repoRoot, dir))
		if err != nil {
			t.Fatalf("ReadDir(%q) error = %v", dir, err)
		}
		inspected := 0
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(repoRoot, path), nil, 0)
			if parseErr != nil {
				t.Fatalf("ParseFile(%q) error = %v", path, parseErr)
			}
			inspected++
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Recv == nil && forbidden[function.Name.Name] {
					found = append(found, filepath.ToSlash(path)+":"+function.Name.Name)
				}
			}
		}
		if inspected == 0 {
			t.Errorf("no production source was read under %s, so this row passes for the wrong reason", dir)
		}
	}
	slices.Sort(found)
	if len(found) != 0 {
		t.Errorf("legacy root entrypoint declarations = %v, want none", found)
	}
}
