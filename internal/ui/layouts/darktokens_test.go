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

// TestSystemDarkPreferenceGetsTheWholeDarkPalette holds the stylesheet's two
// dark entrances together. A reader who chose dark enters through the
// [data-theme="dark"] attribute the server stamps; a reader who chose nothing
// on a dark system enters through the prefers-color-scheme media block. Both
// must land in the same room: the media block is a second copy of the dark
// declarations, and a value edited in one and not the other would give the
// unchosen dark a subtly different palette nobody designed. The comparison is
// declaration-for-declaration so any drift names the exact token.
//
// The media rule excludes only an explicit light choice, because a stored
// choice must keep beating the system either way — dark-on-dark repeats the
// same values, light-on-dark escapes the block entirely.
func TestSystemDarkPreferenceGetsTheWholeDarkPalette(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/tokens.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := string(source)

	chosen := cssDeclarations(t, ruleBody(t, css, `[data-theme='dark'] {`))

	mediaAt := strings.Index(css, "@media (prefers-color-scheme: dark)")
	if mediaAt < 0 {
		t.Fatalf("%s has no prefers-color-scheme dark block; a reader who chose nothing gets light on a dark system", path)
	}
	media := css[mediaAt:]
	const guard = `:root:not([data-theme='light']) {`
	if !strings.Contains(media[:strings.Index(media, "{")+200], guard) {
		t.Fatalf("the media block does not guard on %q; an explicit light choice must escape it", guard)
	}
	system := cssDeclarations(t, ruleBody(t, media, guard))

	if len(chosen) < 10 || chosen["--bg"] == "" || chosen["color-scheme"] != "dark" {
		t.Fatalf("the chosen-dark block parsed implausibly (%d declarations); the comparison below would prove nothing", len(chosen))
	}
	if diff := cmp.Diff(chosen, system); diff != "" {
		t.Errorf("the two dark entrances disagree (-chosen +system):\n%s", diff)
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
	for _, want := range []string{"[data-theme='dark']", "[data-theme='dark'] .y-ico-moon", "[data-theme='dark'] .yomihon::after"} {
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
