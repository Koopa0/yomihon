package assets

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The product's class names live in three places that nothing tied together:
// the markup that emits them (templates, Go string literals, scripts that
// build elements), the stylesheets that draw them, and the scripts, probes and
// tests that address them. A class emitted with no rule and nobody addressing
// it is dead markup; a rule for a class nothing emits is a dead rule. Neither
// announces itself, which is how a finished chip row came to sit beside text
// nobody had styled, and how a stylesheet came to carry rules for classes that
// had gone.
//
// Both sets are derived from the sources on every run, so there is no list to
// fall behind them. Only the y- and ui- families are the product's own; the
// renderer's unprefixed callout names are out of scope.

// classToken matches one class name written out in full, or the stem of a
// name finished at run time (a trailing hyphen or underscore).
var classToken = regexp.MustCompile(`(?:y|ui)-[a-z0-9]+(?:(?:__|--|-)[a-z0-9]+)*(?:--|__|-)?`)

// cssSelectorClass matches a class selector in a stylesheet.
var cssSelectorClass = regexp.MustCompile(`\.((?:y|ui)-[a-z0-9]+(?:(?:__|--|-)[a-z0-9]+)*)`)

var (
	quotedLiteral = regexp.MustCompile("\"(?:[^\"\\\\\\n]|\\\\.)*\"|`[^`]*`")
	cssComment    = regexp.MustCompile(`(?s)/\*.*?\*/`)

	// scriptLookup marks a script line that asks about a class rather than
	// putting one on an element.
	scriptLookup = regexp.MustCompile(`contains\(|matches\(|closest\(|querySelector|getElementsByClassName`)
)

const (
	identChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	alnumChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// classSpans returns where the class names and run-time stems sit in s. A span
// is not one when it continues a longer identifier (an id such as _y-main),
// when a colon follows it (a storage key), or when more letters follow it.
func classSpans(s string) [][]int {
	var out [][]int
	for _, at := range classToken.FindAllStringIndex(s, -1) {
		if at[0] > 0 && strings.IndexByte(identChars, s[at[0]-1]) >= 0 {
			continue
		}
		if at[1] < len(s) && (s[at[1]] == ':' || strings.IndexByte(alnumChars, s[at[1]]) >= 0) {
			continue
		}
		out = append(out, at)
	}
	return out
}

// classNames returns the class names and run-time stems written in s.
func classNames(s string) []string {
	var out []string
	for _, at := range classSpans(s) {
		out = append(out, s[at[0]:at[1]])
	}
	return out
}

func isStem(name string) bool { return strings.HasSuffix(name, "-") || strings.HasSuffix(name, "_") }

// classSources reads the three kinds of source and returns what each says.
type classSources struct {
	emitted   map[string]bool // exact names the markup and scripts put on elements
	stems     []string        // prefixes of names finished at run time
	addressed map[string]bool // names a script, probe or test looks up
	styled    map[string]bool // names a stylesheet selects
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) // #nosec G304 -- a path the walk of this repository's own tree produced
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// filesUnder lists the files below root that match keep, skipping the
// vendored diagram library.
func filesUnder(t *testing.T, root string, keep func(path string) bool) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "mermaid" || d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if keep(path) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// goStringLiterals returns the text of every string literal in a Go source,
// comments excluded.
func goStringLiterals(src string) []string {
	var out []string
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), func(token.Position, string) {}, 0)
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.STRING {
			out = append(out, lit)
		}
	}
}

func collectClassSources(t *testing.T) classSources {
	t.Helper()
	cs := classSources{emitted: map[string]bool{}, addressed: map[string]bool{}, styled: map[string]bool{}}
	var fromTemplates, fromGo, fromScripts int
	emit := func(count *int, names []string) {
		for _, name := range names {
			if isStem(name) {
				if !slices.Contains(cs.stems, name) {
					cs.stems = append(cs.stems, name)
				}
				continue
			}
			if !cs.emitted[name] {
				cs.emitted[name] = true
				*count++
			}
		}
	}

	for _, path := range filesUnder(t, "../internal", func(p string) bool { return strings.HasSuffix(p, ".templ") }) {
		for _, lit := range quotedLiteral.FindAllString(readFile(t, path), -1) {
			emit(&fromTemplates, classNames(lit))
		}
	}
	isGo := func(p string) bool { return strings.HasSuffix(p, ".go") }
	for _, root := range []string{"../internal", "../cmd"} {
		for _, path := range filesUnder(t, root, isGo) {
			switch {
			case strings.HasSuffix(path, "_templ.go"):
				// Generated from the templates already read.
			case strings.HasSuffix(path, "_test.go"):
				for _, name := range classNames(readFile(t, path)) {
					cs.addressed[name] = true
				}
			default:
				for _, lit := range goStringLiterals(readFile(t, path)) {
					emit(&fromGo, classNames(lit))
				}
			}
		}
	}
	for _, path := range filesUnder(t, ".", func(p string) bool {
		return strings.HasSuffix(p, "_test.go") && !strings.HasSuffix(p, "classrules_test.go")
	}) {
		for _, name := range classNames(readFile(t, path)) {
			cs.addressed[name] = true
		}
	}
	for _, path := range filesUnder(t, "../.github/e2e", func(p string) bool { return strings.HasSuffix(p, ".mjs") }) {
		for _, name := range classNames(readFile(t, path)) {
			cs.addressed[name] = true
		}
	}
	for _, path := range filesUnder(t, "js", func(p string) bool { return strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".mjs") }) {
		for line := range strings.SplitSeq(readFile(t, path), "\n") {
			lookup := scriptLookup.MatchString(line)
			for _, at := range classSpans(line) {
				name := line[at[0]:at[1]]
				if lookup || (at[0] > 0 && line[at[0]-1] == '.') {
					cs.addressed[name] = true
				} else {
					emit(&fromScripts, []string{name})
				}
			}
		}
	}
	for _, path := range filesUnder(t, "css", func(p string) bool { return strings.HasSuffix(p, ".css") }) {
		for _, m := range cssSelectorClass.FindAllStringSubmatch(cssComment.ReplaceAllString(readFile(t, path), ""), -1) {
			cs.styled[m[1]] = true
		}
	}

	// A scan that read the wrong tree would report that everything agrees.
	if fromTemplates < 300 || fromGo < 4 || fromScripts < 5 || len(cs.styled) < 300 || len(cs.addressed) < 50 {
		t.Fatalf("class scan is too thin to trust: %d from templates, %d from Go, %d from scripts, %d styled, %d addressed", fromTemplates, fromGo, fromScripts, len(cs.styled), len(cs.addressed))
	}
	return cs
}

// TestEveryClassIsStyledOrAddressedAndEveryRuleIsUsed locks both directions at
// once. An emitted class needs a rule or something that addresses it, and
// when it has neither it is dead markup to remove or to style. A rule needs
// something that emits its class, and when nothing does it is dead and goes.
func TestEveryClassIsStyledOrAddressedAndEveryRuleIsUsed(t *testing.T) {
	t.Parallel()
	cs := collectClassSources(t)

	var deadMarkup, deadRules []string
	for name := range cs.emitted {
		if !cs.styled[name] && !cs.addressed[name] {
			deadMarkup = append(deadMarkup, name)
		}
	}
	for name := range cs.styled {
		if cs.emitted[name] {
			continue
		}
		if slices.ContainsFunc(cs.stems, func(stem string) bool { return strings.HasPrefix(name, stem) }) {
			continue
		}
		deadRules = append(deadRules, name)
	}
	slices.Sort(deadMarkup)
	slices.Sort(deadRules)
	for _, name := range deadMarkup {
		t.Errorf("%s is emitted, has no stylesheet rule, and nothing looks it up: remove it from the markup, or style it if it was meant to have a look", name)
	}
	for _, name := range deadRules {
		t.Errorf("%s has a stylesheet rule and nothing emits it: delete the rule", name)
	}
}
