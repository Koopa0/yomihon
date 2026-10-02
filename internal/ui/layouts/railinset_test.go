package layouts

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestTheLeftRailStartsItsTextCloseToItsEdge holds the inset every rail the
// left column carries is drawn with. A title or a part heading sits ten pixels
// in from the column's edge; a lesson's text sits a group margin, the shift a
// row with a status takes, and a row inset further, and its status dot sits on
// the row's own rule rather than in a slot beside the name. The numbers are one
// rhythm: widening any one of them moves the text of every course, map, book
// and folder rail, and of the drawer they become on a phone, by the same
// amount, so they are held together and by value.
func TestTheLeftRailStartsItsTextCloseToItsEdge(t *testing.T) {
	t.Parallel()

	rules := componentRules(t)

	var pads []string
	for _, rule := range rules {
		pads = append(pads, rule.values("--rail-pad-start")...)
	}
	if diff := cmp.Diff([]string{"10px"}, pads); diff != "" {
		t.Errorf("--rail-pad-start is declared with these values (-want +got):\n%s", diff)
	}

	// The column's own box is the one rule outside every media query that sets
	// its padding; the others narrow it, fold it or turn it into a drawer.
	var columns []cssRule
	for _, rule := range rules {
		if selectorNames(rule.selector, ".y-rail-left") && len(rule.within) == 0 && len(rule.values("padding")) > 0 {
			columns = append(columns, rule)
		}
	}
	if len(columns) != 1 {
		t.Fatalf("%d top-level rules give .y-rail-left a padding, want exactly 1", len(columns))
	}
	column := columns[0]
	if diff := cmp.Diff([]string{"6px var(--rail-pad) 48px var(--rail-pad-start)"}, column.values("padding")); diff != "" {
		t.Errorf("the left column's padding is not the one that starts its text at the inset (-want +got):\n%s", diff)
	}

	group := onlyRule(t, rules, ".y-railgroup")
	if diff := cmp.Diff([]string{"2px 0 8px 2px"}, group.values("margin")); diff != "" {
		t.Errorf("a group's margin is not the one that keeps its rows near the edge (-want +got):\n%s", diff)
	}

	row := onlyRule(t, rules, ".y-rail-left .ui-navitem")
	if diff := cmp.Diff([]string{"12px"}, row.values("padding-left")); diff != "" {
		t.Errorf("a row in the left column insets its text by (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"6px"}, row.values("column-gap")); diff != "" {
		t.Errorf("a row in the left column sets its name and its status apart by (-want +got):\n%s", diff)
	}

	marked := onlyRule(t, rules, ".y-rail-left .ui-navitem:has(> .y-navdot)")
	if diff := cmp.Diff([]string{"10px"}, marked.values("margin-inline-start")); diff != "" {
		t.Errorf("a row with a status is not shifted in by the width its dot used to take (-want +got):\n%s", diff)
	}
	dot := onlyRule(t, rules, ".y-rail-left .ui-navitem > .y-navdot")
	if diff := cmp.Diff([]string{"absolute"}, dot.values("position")); diff != "" {
		t.Errorf("the status dot still takes a slot beside the name (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"-3px"}, dot.values("inset-inline-start")); diff != "" {
		t.Errorf("the status dot is not centred on the row's rule (-want +got):\n%s", diff)
	}
	if got := row.values("gap"); len(got) > 0 {
		t.Errorf("a row in the left column sets gap: %v, which would also move a wrapped row's lines apart; the rule sets the column gap alone", got)
	}
}
