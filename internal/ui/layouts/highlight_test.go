package layouts

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// markRule matches one whole rule whose selector list ends at a bare <mark>.
// The word boundary keeps the product's own names out of it: y-navmark,
// y-brand__mark and option::checkmark all end in the same four letters and
// none of them is the element.
var markRule = regexp.MustCompile(`(?m)^\s*([^{}\n]*?\bmark)\s*\{([^}]*)\}`)

// TestTheHighlightWashReachesEveryMarkAndNotOnlyTheProse holds where the gold
// highlight applies. The renderer turns ==text== into a bare <mark>, and so
// does the search face for every hit it found — in a title, in a path, in a
// snippet — and those sit outside the reading column. While the wash was
// written against the column alone, a page of results fell back to the
// browser's own pure yellow: the only saturated primary colour anywhere in
// this interface, fired dozens of times at once, and on a dark evening laid
// over a near-black ground.
//
// So the contract is not "the prose is dressed" but "a mark is dressed
// wherever it appears", and the way to hold that is to require the rule to be
// scoped to the shell every surface renders inside.
func TestTheHighlightWashReachesEveryMarkAndNotOnlyTheProse(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := cssComments.ReplaceAllString(string(source), "")

	rules := markRule.FindAllStringSubmatch(css, -1)
	if len(rules) == 0 {
		t.Fatal("the stylesheet dresses no <mark> at all, so either the highlight is gone or this scan is reading the wrong file")
	}
	var washed []string
	for _, rule := range rules {
		selector := strings.Join(strings.Fields(rule[1]), " ")
		if cssDeclarations(t, rule[2])["background"] == "" {
			continue
		}
		washed = append(washed, selector)
	}
	slices.Sort(washed)
	want := []string{".yomihon mark"}
	if !slices.Equal(washed, want) {
		t.Errorf("the highlight background is written for %v, want %v — anything narrower leaves some surface's marks to the browser's own yellow, and anything wider means two rules now decide one colour", washed, want)
	}
}
