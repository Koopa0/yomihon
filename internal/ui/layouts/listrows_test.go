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

// TestAStatusIsNeutralWhereverItIsDrawn holds the status word to the one look it
// has. The badge on a note is a wash under muted ink, one fact about the page.
// Down a list the word is the exception on a row and is quiet text: the faint
// ink with no fill, padding or dot to hold one. Neither names the accent, and no
// rule names a status, because which statuses need noticing is the contract's
// to say and the vermilion belongs to the reader's own place. A fix that quieted
// the badge itself would pass the list half and fail the reader.
func TestAStatusIsNeutralWhereverItIsDrawn(t *testing.T) {
	t.Parallel()

	rules := componentRules(t)
	for _, context := range []string{".y-lesson", ".y-homenote__meta"} {
		t.Run(context, func(t *testing.T) {
			t.Parallel()
			word := onlyRule(t, rules, context+" .ui-status")
			if diff := cmp.Diff([]string{"none"}, word.values("background")); diff != "" {
				t.Errorf("the status word in %s keeps a fill (-want +got):\n%s", context, diff)
			}
			if diff := cmp.Diff([]string{"var(--fg-subtle)"}, word.values("color")); diff != "" {
				t.Errorf("the status word in %s is not the quiet ink (-want +got):\n%s", context, diff)
			}
			if diff := cmp.Diff([]string{"0"}, word.values("padding")); diff != "" {
				t.Errorf("the status word in %s keeps the badge's padding (-want +got):\n%s", context, diff)
			}
			dot := onlyRule(t, rules, context+" .ui-status::before")
			if diff := cmp.Diff([]string{"none"}, dot.values("display")); diff != "" {
				t.Errorf("the status word in %s keeps the badge's dot (-want +got):\n%s", context, diff)
			}
		})
	}

	badge := onlyRule(t, rules, ".ui-status")
	if diff := cmp.Diff([]string{"var(--wash)"}, badge.values("background")); diff != "" {
		t.Errorf("the status badge on a note is not the neutral wash (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"var(--fg-muted)"}, badge.values("color")); diff != "" {
		t.Errorf("the status badge on a note is not the muted ink (-want +got):\n%s", diff)
	}

	read := 0
	for _, rule := range rules {
		if !strings.Contains(rule.selector, ".ui-status") {
			continue
		}
		read++
		if strings.Contains(rule.selector, ".ui-status--") {
			t.Errorf("%q dresses one status by name, which copies the contract's vocabulary into the stylesheet", rule.selector)
		}
		if strings.Contains(rule.body, "accent") {
			t.Errorf("%q draws a status in the accent, which marks the reader's own place and nothing the author wrote", rule.selector)
		}
	}
	if read < 4 {
		t.Fatalf("only %d rules name .ui-status, so the scan above read almost nothing", read)
	}
}

// TestACoursesGlossRunsInWithItsTitleAndIsNeverABlock holds what makes the
// author's sentence a continuation of the lesson's name: the run it sits in is
// not clamped to a number of lines, which would cut the sentence off, and the
// sentence itself is inline, so it follows the name on its line instead of
// being a second line under it.
func TestACoursesGlossRunsInWithItsTitleAndIsNeverABlock(t *testing.T) {
	t.Parallel()

	rules := componentRules(t)
	gloss := onlyRule(t, rules, ".y-gloss")
	for _, value := range gloss.values("display") {
		t.Errorf(".y-gloss is display: %s, which sets the sentence apart from its title as a line of its own", value)
	}
	text := onlyRule(t, rules, ".y-lesson__text")
	for _, property := range []string{"-webkit-line-clamp", "line-clamp", "max-height", "overflow", "text-overflow"} {
		if got := text.values(property); len(got) > 0 {
			t.Errorf(".y-lesson__text sets %s: %v, which cuts the sentence a row's author wrote", property, got)
		}
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
