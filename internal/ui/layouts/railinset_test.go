package layouts

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestTheLeftRailStartsItsTextCloseToItsEdge holds the inset every rail the
// left column carries is drawn with. A title, a part heading or a summary sits
// six pixels in from the column's edge, and fourteen in the drawer a phone
// gets; a list's rule stands six further in, under a summary's chevron; and a
// row's words stand twelve in from that rule, with its status dot on the rule
// rather than in a slot beside the name. The three lengths are named once and
// every rail — a course, a map, a book, a folder, the drawer — is drawn from
// them, so they are held together and by value.
func TestTheLeftRailStartsItsTextCloseToItsEdge(t *testing.T) {
	t.Parallel()

	rules := componentRules(t)

	for token, want := range map[string][]string{
		"--rail-pad-start":  {"6px", "14px"},
		"--rail-list-inset": {"6px"},
		"--rail-row-inset":  {"12px"},
	} {
		var got []string
		for _, rule := range rules {
			got = append(got, rule.values(token)...)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s is declared with these values, in source order (-want +got):\n%s", token, diff)
		}
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
	if diff := cmp.Diff([]string{"2px 0 8px var(--rail-list-inset)"}, group.values("margin")); diff != "" {
		t.Errorf("a group's rule does not stand at the list inset (-want +got):\n%s", diff)
	}
	lists := onlyRule(t, rules, ".y-rail-left .y-here > .ui-navitem")
	if diff := cmp.Diff([]string{"var(--rail-list-inset)"}, lists.values("margin-left")); diff != "" {
		t.Errorf("a folder's neighbourhood or the reports stand their rule somewhere else (-want +got):\n%s", diff)
	}

	row := onlyRule(t, rules, ".y-rail-left .ui-navitem")
	if diff := cmp.Diff([]string{"var(--rail-row-inset)"}, row.values("padding-left")); diff != "" {
		t.Errorf("a row in the left column insets its text by (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"6px"}, row.values("column-gap")); diff != "" {
		t.Errorf("a row in the left column sets its name and its status apart by (-want +got):\n%s", diff)
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
