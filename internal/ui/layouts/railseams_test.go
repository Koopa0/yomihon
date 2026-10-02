package layouts

import (
	"slices"
	"strings"
	"testing"
)

// rulesNaming returns the rules whose selector list names the selector as one
// of its entries, whatever else the list holds.
func rulesNaming(rules []cssRule, selector string) []cssRule {
	var out []cssRule
	for _, rule := range rules {
		for entry := range strings.SplitSeq(rule.selector, ",") {
			if strings.TrimSpace(entry) == selector {
				out = append(out, rule)
				break
			}
		}
	}
	return out
}

// TestTheRailKeepsStillWhereAPageChangeWouldMoveIt holds the two rules that
// stop the rail shifting as the reader moves between pages.
//
// The note being read does not show its status chip, but the chip keeps its
// room: dropping it widened the row's name and moved the text by a line
// between sibling lessons, so the rule has to hide the chip and leave the
// space, which display:none would not.
//
// The remembered filter narrows the rail from the client module, after the
// rail has painted, and the folds and chevrons must not animate that
// correction.
func TestTheRailKeepsStillWhereAPageChangeWouldMoveIt(t *testing.T) {
	t.Parallel()
	rules := componentRules(t)

	reserved := rulesNaming(rules, ".ui-navitem__count--reserved")
	if len(reserved) != 1 {
		t.Fatalf("rules for .ui-navitem__count--reserved = %d, want exactly 1", len(reserved))
	}
	if got := reserved[0].values("visibility"); !slices.Equal(got, []string{"hidden"}) {
		t.Errorf("the reserved status chip has visibility %q, want [hidden]; it must disappear and keep its room", got)
	}
	if got := reserved[0].values("display"); len(got) != 0 {
		t.Errorf("the reserved status chip sets display %q; any display value that removes the box gives its room back to the name", got)
	}

	for _, selector := range []string{
		".y-rail-left[data-rail-restoring] details::details-content",
		".y-rail-left[data-rail-restoring] .y-chevron",
	} {
		held := rulesNaming(rules, selector)
		if len(held) != 1 {
			t.Errorf("rules for %s = %d, want exactly 1", selector, len(held))
			continue
		}
		if got := held[0].values("transition"); !slices.Equal(got, []string{"none"}) {
			t.Errorf("%s has transition %q, want [none]; a restore is not the reader folding a group", selector, got)
		}
	}
}
