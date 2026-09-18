package layouts

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A page is one of a few shapes, and each shape is a named line length. Five
// different measures, one of them named and four written as literals, is how a
// reader came to get a different line for the same kind of prose on every page.
// These three tests hold the count: the measures that exist, the shells that
// take one, and the absence of a sixth written as a number somewhere else.

const (
	stylesheetDir  = "../../../assets/css"
	componentsPath = stylesheetDir + "/components.css"
)

// TestThePageMeasuresAreTheNamedOnes fails the moment a fourth measure is
// declared, a named one is renamed, or one changes value — whichever stylesheet
// it is written in. A measure declared in a second sheet would otherwise be
// invisible to a check that only read the token file.
func TestThePageMeasuresAreTheNamedOnes(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"--measure-read": "640px",
	}

	sheets := handWrittenStylesheets(t)
	got := map[string]string{}
	pattern := regexp.MustCompile(`(--measure-[a-z0-9-]*)\s*:\s*([^;}]+)`)
	for _, sheet := range sheets {
		source, err := os.ReadFile(sheet)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", sheet, err)
		}
		for _, match := range pattern.FindAllStringSubmatch(withoutComments(string(source)), -1) {
			got[match[1]] = strings.TrimSpace(match[2])
		}
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the declared page measures differ from the named ones (-want +got):\n%s", diff)
	}
}

// TestEveryPageShellTakesItsOwnMeasure holds the other direction: a shell whose
// box stopped resolving from a measure — moved to a character count, say, or to
// a literal — would leave both of the other tests green while the reader got a
// different line.
func TestEveryPageShellTakesItsOwnMeasure(t *testing.T) {
	t.Parallel()

	shells := []struct {
		class   string
		measure string
	}{
		{class: ".y-article", measure: "var(--measure-read)"},
		{class: ".y-prefs", measure: "var(--measure-read)"},
		{class: ".y-recovery", measure: "var(--measure-read)"},
		{class: ".y-syl", measure: "var(--measure-read)"},
	}

	rules := componentRules(t)
	for _, shell := range shells {
		t.Run(shell.class, func(t *testing.T) {
			found := false
			for _, rule := range rules {
				if !selectorNames(rule.selector, shell.class) {
					continue
				}
				for _, value := range rule.values("max-width") {
					found = true
					if !strings.Contains(value, shell.measure) {
						t.Errorf("rule %q gives %s a max-width of %q, want one resolving from %s",
							rule.selector, shell.class, value, shell.measure)
					}
				}
			}
			if !found {
				t.Errorf("no rule in components.css gives %s a max-width, so its line length is whatever its container is", shell.class)
			}
		})
	}
}

// TestNoPageMeasureIsWrittenAsAPixelLiteral names every remaining box in
// components.css that is still a number. Each entry says what that box is, so a
// new number cannot be added without saying what it is, and a page shell cannot
// be given one at all.
func TestNoPageMeasureIsWrittenAsAPixelLiteral(t *testing.T) {
	t.Parallel()

	allowed := map[string]string{
		".y-headerfold:popover-open": "the header's folded panel, kept off a phone's edges",
		".y-home":                    "the shelf shell, until it is given a measure of its own",
		".y-home__head > p":          "the desk's opening line",
		".y-kbdhelp":                 "the keyboard-help popover",
		".y-searchdialog":            "the palette box",
		".y-searchpage":              "the answer shell, until it is given a measure of its own",
	}

	for _, rule := range componentRules(t) {
		for _, value := range rule.values("max-width") {
			if !strings.Contains(value, "px") {
				continue
			}
			if _, ok := allowed[rule.selector]; !ok {
				t.Errorf("rule %q sets max-width: %s, a length written as a number; name it in this test or resolve it from a measure",
					rule.selector, value)
			}
		}
	}
}

// handWrittenStylesheets is every sheet a person edits — the same set the
// stylesheet lint reads, which is all of them but the minified projection.
func handWrittenStylesheets(t *testing.T) []string {
	t.Helper()
	var sheets []string
	err := filepath.WalkDir(stylesheetDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".css" || filepath.Base(path) == "output.css" {
			return nil
		}
		sheets = append(sheets, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %q error = %v", stylesheetDir, err)
	}
	// A search that found a short list would report agreement over sheets it
	// never opened, so the two that carry the shells have to be among them.
	for _, required := range []string{"tokens.css", "components.css"} {
		if !containsBase(sheets, required) {
			t.Fatalf("the stylesheet sweep found %v, which does not include %s; it is reading the wrong tree", sheets, required)
		}
	}
	return sheets
}

func containsBase(paths []string, base string) bool {
	for _, path := range paths {
		if filepath.Base(path) == base {
			return true
		}
	}
	return false
}

// cssRule is one ordinary rule: the selector it was written under, and the
// declarations inside it. A rule nested inside @media or @supports is an
// ordinary rule too, which is the point — a box overridden at one width is
// still a box.
type cssRule struct {
	selector string
	body     string
	depth    int
}

// values returns every value the rule gives that property, in source order. A
// rule may state one twice; both are answered for.
func (r cssRule) values(property string) []string {
	var out []string
	for declaration := range strings.SplitSeq(r.body, ";") {
		name, value, ok := strings.Cut(declaration, ":")
		if !ok || strings.TrimSpace(name) != property {
			continue
		}
		out = append(out, strings.Join(strings.Fields(value), " "))
	}
	return out
}

// selectorNames answers whether a selector list applies the rule to that class
// on its own, rather than to something inside or beside it. It matches the
// class as a whole term, so .y-article answers and .y-article .y-title does not.
func selectorNames(selector, class string) bool {
	for _, part := range strings.Split(selector, ",") {
		if strings.TrimSpace(part) == class {
			return true
		}
	}
	return false
}

var cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

func withoutComments(css string) string {
	return cssComment.ReplaceAllString(css, "")
}

// componentRules parses components.css and refuses to hand back a parse that
// cannot have read it: a sheet of this size holds hundreds of rules, and it
// nests a rule two at-rules deep. A parser that stopped at the first of those
// would report no violations for the best of reasons.
func componentRules(t *testing.T) []cssRule {
	t.Helper()
	source, err := os.ReadFile(componentsPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", componentsPath, err)
	}
	rules := parseRules(t, withoutComments(string(source)), 0)
	if len(rules) < 200 {
		t.Fatalf("the parse of %s found %d rules; a sheet of %d bytes holds many more, so the parser stopped early",
			componentsPath, len(rules), len(source))
	}
	deepest := 0
	for _, rule := range rules {
		if rule.depth > deepest {
			deepest = rule.depth
		}
	}
	if deepest < 2 {
		t.Fatalf("the parse of %s reached a depth of %d; it holds a rule inside an @supports inside an @media, so nested blocks are not being entered",
			componentsPath, deepest)
	}
	return rules
}

func parseRules(t *testing.T, block string, depth int) []cssRule {
	t.Helper()
	var out []cssRule
	for {
		open := strings.IndexByte(block, '{')
		if open < 0 {
			return out
		}
		prelude := strings.Join(strings.Fields(block[:open]), " ")
		rest := block[open+1:]
		end := closingBrace(rest)
		if end < 0 {
			t.Fatalf("the block opened by %q is never closed", prelude)
		}
		body := rest[:end]
		if strings.HasPrefix(prelude, "@") {
			out = append(out, parseRules(t, body, depth+1)...)
		} else {
			out = append(out, cssRule{selector: prelude, body: body, depth: depth})
		}
		block = rest[end+1:]
	}
}

// closingBrace returns the offset of the brace that closes the block this text
// opens in, counting the ones opened inside it.
func closingBrace(block string) int {
	open := 0
	for i := 0; i < len(block); i++ {
		switch block[i] {
		case '{':
			open++
		case '}':
			if open == 0 {
				return i
			}
			open--
		}
	}
	return -1
}
