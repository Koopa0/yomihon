package layouts

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// onlyRule returns the one rule written under exactly this selector. A selector
// that matches no rule, or two, is a lock reading nothing or reading whichever
// came first, so both are refused.
func onlyRule(t *testing.T, rules []cssRule, selector string) cssRule {
	t.Helper()
	var found []cssRule
	for _, rule := range rules {
		if selectorNames(rule.selector, selector) {
			found = append(found, rule)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d rules are written under %q, want exactly 1", len(found), selector)
	}
	return found[0]
}

// TestACourseCountIsAFigureAndNotABadge holds the number of lessons beside a
// part or a module to the treatment of the rail's own counts: ink and figures
// and nothing drawn around them. The heading already says what the number is
// counting, so a box around it states it a second time.
func TestACourseCountIsAFigureAndNotABadge(t *testing.T) {
	t.Parallel()

	count := onlyRule(t, componentRules(t), ".y-partcount")
	for _, property := range []string{"background", "border", "border-radius", "padding"} {
		if got := count.values(property); len(got) > 0 {
			t.Errorf(".y-partcount sets %s: %v, which draws the count as a badge", property, got)
		}
	}
	if diff := cmp.Diff([]string{"var(--fg-subtle)"}, count.values("color")); diff != "" {
		t.Errorf(".y-partcount is not the quiet ink the rail's counts use (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"tabular-nums"}, count.values("font-variant-numeric")); diff != "" {
		t.Errorf(".y-partcount does not set its figures in tabular numerals (-want +got):\n%s", diff)
	}
}

// TestACourseActionStaysOneRunOfInlineText holds the arrow after a course's
// actions to the space the words end on. That space is trailing whitespace, and
// inside a flex or grid box it is the end of an item of its own and collapses,
// so the arrow touches the last word; the rest of the interface sets its arrows
// as text after a space, and this is the one place a box once took it away.
func TestACourseActionStaysOneRunOfInlineText(t *testing.T) {
	t.Parallel()

	action := onlyRule(t, componentRules(t), ".y-syl-read")
	for _, value := range action.values("display") {
		if strings.Contains(value, "flex") || strings.Contains(value, "grid") {
			t.Errorf(".y-syl-read is display: %s, which drops the space before its arrow", value)
		}
	}
}
