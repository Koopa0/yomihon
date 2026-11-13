package layouts

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestEveryColourTokenIsOnePairDeclaredOnce holds the palette to the shape that
// keeps the two themes from drifting apart. Each colour token is written once,
// on the root, as light-dark(light, dark), so a value cannot be edited for one
// theme and forgotten in the other. The theme itself is the root's
// color-scheme, and the ways into it — a stored choice on the attribute, a
// dark system for a reader who stored none — set that and nothing else: a
// token declared again inside one of them would be the second copy this shape
// exists to remove.
//
// A colour literal outside light-dark() is a colour chosen for one theme only.
// The overlay mask is the one such value: a fixed black wash that reads over
// either theme's surfaces, so it has no dark twin to keep in step.
func TestEveryColourTokenIsOnePairDeclaredOnce(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/tokens.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	pairs := map[string][]string{}
	schemes := map[string]string{}
	singleTheme := 0
	for _, rule := range parseRules(t, withoutComments(string(source)), nil) {
		where := strings.Join(append(slices.Clone(rule.within), rule.selector), " ")
		declarations := cssDeclarations(t, rule.body)
		if strings.Contains(rule.selector, "data-theme") || slices.ContainsFunc(rule.within, func(at string) bool {
			return strings.Contains(at, "prefers-color-scheme")
		}) {
			if names := slices.Sorted(maps.Keys(declarations)); !slices.Equal(names, []string{"color-scheme"}) {
				t.Errorf("%s declares %v; a way into a theme sets color-scheme and nothing else, and the tokens answer it", where, names)
			}
		}
		if scheme, ok := declarations["color-scheme"]; ok {
			schemes[where] = scheme
		}
		for property, value := range declarations {
			if !strings.HasPrefix(property, "--") {
				continue
			}
			if strings.Contains(value, "light-dark(") {
				pairs[property] = append(pairs[property], where)
			}
			if property == overlayMask {
				singleTheme++
				continue
			}
			if literal := oneThemeColour.FindString(withoutLightDark(value)); literal != "" {
				t.Errorf("%s in %s carries %q outside light-dark(), a colour for one theme only", property, where, literal)
			}
		}
	}

	// A parse that found too little would pass every check above.
	for _, token := range []string{"--bg", "--panel", "--fg", "--fg-muted", "--accent", "--link", "--outline"} {
		if pairs[token] == nil {
			t.Errorf("%s is not declared as a light-dark() pair in %s; the scan is reading the wrong text or the palette lost a token", token, path)
		}
	}
	if singleTheme != 1 {
		t.Errorf("%s is declared %d times in %s, want 1; the one exemption above names a token that is not there", overlayMask, singleTheme, path)
	}
	for _, token := range slices.Sorted(maps.Keys(pairs)) {
		if places := pairs[token]; !slices.Equal(places, []string{":root"}) {
			t.Errorf("%s is declared at %v, want once, at :root", token, places)
		}
	}

	want := map[string]string{
		":root":                "light",
		"[data-theme='light']": "light",
		"[data-theme='dark']":  "dark",
		systemDarkMedia + " " + systemDarkSelector: "dark",
	}
	if diff := cmp.Diff(want, schemes); diff != "" {
		t.Errorf("the theme entrances set color-scheme differently (-want +got):\n%s", diff)
	}
}

const overlayMask = "--overlay-mask"

// oneThemeColour matches a colour written as a value rather than through a
// token: a hex colour or a colour function. color-mix() is not one; it mixes
// what it is given.
var oneThemeColour = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(`)

// withoutLightDark removes every light-dark(...) call from a value, counting
// the parentheses inside it, so what is left is what was written outside one.
func withoutLightDark(value string) string {
	const opener = "light-dark("
	for {
		at := strings.Index(value, opener)
		if at < 0 {
			return value
		}
		depth, end := 1, -1
		for i := at + len(opener); i < len(value) && end < 0; i++ {
			switch value[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					end = i
				}
			}
		}
		if end < 0 {
			return value
		}
		value = value[:at] + value[end+1:]
	}
}

// ruleBody returns the text between the named rule opener and its closing
// brace. The token blocks under test nest nothing, so the first unmatched
// close brace ends the body.
func ruleBody(t *testing.T, css, opener string) string {
	t.Helper()
	at := strings.Index(css, opener)
	if at < 0 {
		t.Fatalf("stylesheet has no rule %q", opener)
	}
	body := css[at+len(opener):]
	end := strings.IndexByte(body, '}')
	if end < 0 {
		t.Fatalf("rule %q is not closed", opener)
	}
	return body[:end]
}

// cssDeclarations parses one rule body into property -> value, with comments
// stripped and whitespace folded so formatting differences cannot register as
// palette drift.
func cssDeclarations(t *testing.T, body string) map[string]string {
	t.Helper()
	body = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(body, "")
	out := map[string]string{}
	for decl := range strings.SplitSeq(body, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		property, value, ok := strings.Cut(decl, ":")
		if !ok {
			t.Fatalf("unparsable declaration %q", decl)
		}
		out[strings.TrimSpace(property)] = strings.Join(strings.Fields(value), " ")
	}
	return out
}

// darkAttribute matches the stored-choice entrance to the dark theme, in any
// spelling of its quotes, together with the :root it usually rides on. The
// system entrance replaces the whole match.
var darkAttribute = regexp.MustCompile(`(:root)?\[data-theme=["']?dark["']?\]`)

const (
	systemDarkMedia    = "@media (prefers-color-scheme: dark)"
	systemDarkSelector = ":root:not([data-theme='light'])"
)

// TestEveryDarkRuleHasASystemTwin holds every hand-written stylesheet to the
// rule the colour tokens already follow: a rule that dresses the page for a
// stored dark choice must dress it the same way for a reader who stored
// nothing on a dark system, or that reader gets the light desk's version of
// it. The set is derived from the stylesheets rather than listed, so the next
// rule written against the attribute alone fails here instead of being found
// on a reader's screen.
//
// A twin is only a twin inside the system-dark media query, and with the same
// declarations: the same selector outside it would dress light systems dark,
// and an empty twin would hold the pair together while changing nothing.
//
// The print reset is the one rule that names the attribute for a different
// reason. It does not dress dark, it undoes it, and it must reach every root
// the dark palette reaches — including the one with no attribute, which the
// system twin's extra specificity otherwise keeps for the screen's ink. Its
// twin is therefore a member of the same selector list.
func TestEveryDarkRuleHasASystemTwin(t *testing.T) {
	t.Parallel()

	// Rules naming the same selector more than once join, as the cascade joins
	// them, so a twin written in two pieces is judged as one.
	chosen := map[string]map[string]string{}
	system := map[string]map[string]string{}
	printResets := 0
	for _, sheet := range handWrittenStylesheets(t) {
		source, err := os.ReadFile(sheet) // #nosec G304 -- sheet came from walking the fixed stylesheetDir constant, not from any input outside this test
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", sheet, err)
		}
		for _, rule := range parseRules(t, withoutComments(string(source)), nil) {
			selectors := splitSelectors(rule.selector)
			switch {
			case slices.Contains(rule.within, systemDarkMedia):
				addDeclarations(t, system, selectors, rule.body)
			case slices.Contains(rule.within, "@media print"):
				if !slices.ContainsFunc(selectors, darkAttribute.MatchString) {
					continue
				}
				printResets++
				for _, selector := range selectors {
					if !darkAttribute.MatchString(selector) {
						continue
					}
					if twin := darkAttribute.ReplaceAllString(selector, systemDarkSelector); !slices.Contains(selectors, twin) {
						t.Errorf("the print reset in %s names %s but not %s, so a reader with no stored choice on a dark system prints the screen's dark palette", filepath.Base(sheet), selector, twin)
					}
				}
			default:
				addDeclarations(t, chosen, slices.DeleteFunc(slices.Clone(selectors), func(selector string) bool {
					return !darkAttribute.MatchString(selector)
				}), rule.body)
			}
		}
	}

	// A scan that found nothing proves nothing, and a parse that loses a block
	// would report every rule inside it as absent rather than as twinned.
	for _, want := range []string{"[data-theme='dark']"} {
		if chosen[want] == nil {
			t.Errorf("the scan did not find %q among the stored-dark rules %v; it is reading the wrong text and nothing below it means anything", want, slices.Sorted(maps.Keys(chosen)))
		}
	}
	if printResets == 0 {
		t.Error("the scan found no print rule naming the stored-dark attribute, so it is not reading the print reset")
	}

	for _, selector := range slices.Sorted(maps.Keys(chosen)) {
		twin := darkAttribute.ReplaceAllString(selector, systemDarkSelector)
		got, ok := system[twin]
		if !ok {
			t.Errorf("%s is written for a stored dark choice only; a reader who follows a dark system needs %s inside %s with the same declarations", selector, twin, systemDarkMedia)
			continue
		}
		if diff := cmp.Diff(chosen[selector], got); diff != "" {
			t.Errorf("%s and its system twin %s disagree (-stored +system):\n%s", selector, twin, diff)
		}
	}
}

// addDeclarations joins one rule's declarations into into, under each of the
// selectors it was written for.
func addDeclarations(t *testing.T, into map[string]map[string]string, selectors []string, body string) {
	t.Helper()
	if len(selectors) == 0 {
		return
	}
	declarations := cssDeclarations(t, body)
	for _, selector := range selectors {
		if into[selector] == nil {
			into[selector] = map[string]string{}
		}
		maps.Copy(into[selector], declarations)
	}
}

// splitSelectors splits a selector list at its top-level commas, with
// whitespace folded and quotes made single; a comma inside :is(), :not() or an
// attribute selector belongs to that selector.
func splitSelectors(list string) []string {
	var selectors []string
	depth, start := 0, 0
	for i, r := range list {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				selectors = append(selectors, list[start:i])
				start = i + 1
			}
		}
	}
	selectors = append(selectors, list[start:])
	out := selectors[:0]
	for _, selector := range selectors {
		if selector = strings.ReplaceAll(strings.Join(strings.Fields(selector), " "), `"`, `'`); selector != "" {
			out = append(out, selector)
		}
	}
	return out
}
