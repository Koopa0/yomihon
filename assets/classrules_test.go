package assets

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// unstyledAllowed lists the y- classes a template renders that no stylesheet
// rule selects, each kept with the reason it is allowed rather than styled
// here. The undecided ones are open for a ruling on whether they want a look. An
// entry that gains a rule, or stops being rendered, fails the test so the list
// cannot outlive its cause.
const (
	unstyledHook      = "a hook that scripts, probes or tests address; no look of its own is decided"
	unstyledUndecided = "rendered with no rule and no consumer outside tests; whether it needs a look is undecided"
)

var unstyledAllowed = map[string]string{
	"y-crumbs--course":         unstyledHook,
	"y-facetgroup":             unstyledUndecided,
	"y-findings__file":         unstyledHook,
	"y-findings__finding":      unstyledUndecided,
	"y-healthsection--unknown": unstyledUndecided,
	"y-healthshape__total":     unstyledHook,
	"y-helpbtn":                unstyledHook,
	"y-listen__bar":            unstyledUndecided,
	"y-module--local":          unstyledHook,
	"y-pathitem":               unstyledUndecided,
	"y-railbook":               unstyledHook,
	"y-railfoot__label":        unstyledUndecided,
	"y-recoverymain":           unstyledHook,
	"y-result__alias":          unstyledHook,
	"y-result__source":         unstyledHook,
	"y-result__tag":            unstyledHook,
	"y-result__topic":          unstyledHook,
	"y-slotdata":               unstyledHook,
	"y-slotlive":               unstyledHook,
	"y-steps__asidelabel":      unstyledHook,
	"y-steps__link--back":      unstyledHook,
	"y-steps__link--onward":    unstyledHook,
	"y-steps__link--prev":      unstyledHook,
	"y-syl-unsettled":          unstyledHook,
	"y-thought__said":          unstyledHook,
}

var (
	classAttr   = regexp.MustCompile(`\bclass="([^"]*)"`)
	classExpr   = regexp.MustCompile(`\bclass=\{([^}]*)\}`)
	stringLit   = regexp.MustCompile(`"([^"]*)"`)
	cssComment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssSelector = regexp.MustCompile(`\.(y-[A-Za-z0-9_-]+)`)
)

// renderedClasses returns every y- class a .templ file names as a literal,
// either in a class="..." attribute or among the string literals of a
// class={ ... } expression.
func renderedClasses(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	add := func(list, file string) {
		for _, c := range strings.Fields(list) {
			// A literal ending in a hyphen is the stem of a name finished at
			// render time, not a class of its own.
			if strings.HasPrefix(c, "y-") && !strings.HasSuffix(c, "-") {
				if _, seen := out[c]; !seen {
					out[c] = file
				}
			}
		}
	}
	err := filepath.WalkDir("../internal", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".templ") {
			return err
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		src := string(b)
		for _, m := range classAttr.FindAllStringSubmatch(src, -1) {
			add(m[1], path)
		}
		for _, m := range classExpr.FindAllStringSubmatch(src, -1) {
			for _, lit := range stringLit.FindAllStringSubmatch(m[1], -1) {
				add(lit[1], path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
	return out
}

// styledClasses returns every y- class that appears in a selector of any
// stylesheet under css/, comments excluded.
func styledClasses(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("css/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("no stylesheets found: %v", err)
	}
	out := map[string]bool{}
	for _, f := range files {
		b, readErr := os.ReadFile(f)
		if readErr != nil {
			t.Fatalf("read %s: %v", f, readErr)
		}
		for _, m := range cssSelector.FindAllStringSubmatch(cssComment.ReplaceAllString(string(b), ""), -1) {
			out[m[1]] = true
		}
	}
	return out
}

// TestEveryRenderedClassHasARule locks that a class a template renders is
// drawn by some rule. A class with none renders as bare browser defaults, which
// is how a finished chip row came to sit beside text nobody had styled.
func TestEveryRenderedClassHasARule(t *testing.T) {
	t.Parallel()
	rendered, styled := renderedClasses(t), styledClasses(t)
	if len(rendered) < 100 || len(styled) < 100 {
		t.Fatalf("collected %d rendered and %d styled classes; the collectors are not reading the tree", len(rendered), len(styled))
	}
	var missing, stale []string
	for c, file := range rendered {
		if _, ok := unstyledAllowed[c]; ok {
			if styled[c] {
				stale = append(stale, c+" now has a rule; drop it from unstyledAllowed")
			}
			continue
		}
		if !styled[c] {
			missing = append(missing, c+" ("+file+")")
		}
	}
	for c := range unstyledAllowed {
		if _, ok := rendered[c]; !ok {
			stale = append(stale, c+" is no longer rendered; drop it from unstyledAllowed")
		}
	}
	slices.Sort(missing)
	slices.Sort(stale)
	for _, m := range missing {
		t.Errorf("rendered class with no stylesheet rule: %s", m)
	}
	for _, s := range stale {
		t.Error(s)
	}
}
