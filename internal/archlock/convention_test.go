package archlock

import (
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
// ships, skipping tests and generated code, for a check over the text a reader
// reads.
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

// TestNoScriptCarriesASentenceOfItsOwn keeps the client's words where the
// server's are. A script is one file for every reader, so a sentence written
// into one is written in a single language and reaches the reader who asked
// for the other in the middle of a page that is otherwise theirs.
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
