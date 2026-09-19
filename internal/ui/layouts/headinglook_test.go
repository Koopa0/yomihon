package layouts

import (
	"os"
	"strings"
	"testing"
)

// TestProseHeadingLookFollowsAuthoredLevel holds the half of the heading
// stylesheet that is about look rather than outline. The shell writes each
// body heading one tag down so the chrome title is the only h1; size, the
// section bar and the fragment landing pad have to follow the authored
// level stamped on the element, or a #### that the shell emits as h5 draws
// as body prose and a link to it lands under the sticky header. Every level
// takes a size from the reading scale, which the reader's size choice moves,
// and the deepest two stop at the body's size and are told apart by weight
// and letter-spacing rather than by shrinking a sixth time.
func TestProseHeadingLookFollowsAuthoredLevel(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := cssComments.ReplaceAllString(string(source), "")

	shared := cssDeclarations(t, ruleBody(t, css, `.y-prose [data-level='1'],
.y-prose [data-level='2'],
.y-prose [data-level='3'],
.y-prose [data-level='4'] {`))
	if shared["scroll-margin-top"] != "var(--header-clearance)" {
		t.Errorf("authored levels 1–4 declare scroll-margin-top %q, want var(--header-clearance) so a demoted h5 still clears the sticky header", shared["scroll-margin-top"])
	}
	// This rule ties the per-level rules above it on specificity and sits
	// later, so anything it declares beats what a single level asked for. The
	// levels no longer share one line-height, and a value put back here would
	// take the deepest heading's looser measure away with nothing saying so.
	if _, ok := shared["line-height"]; ok {
		t.Errorf("the shared levels 1–4 rule declares line-height %q; it beats the per-level rules above it, so the fourth level loses the looser measure its size was chosen with", shared["line-height"])
	}

	for _, want := range []struct {
		opener   string
		property string
		value    string
		why      string
	}{
		{`.y-prose [data-level='1'] {`, "font-size", "var(--fs-ed-30)", "an authored # is the size it was when the tag was h1"},
		{`.y-prose [data-level='2'] {`, "font-size", "var(--fs-ed-24)", "an authored ## is the size it was when the tag was h2"},
		{`.y-prose [data-level='2']::before {`, "content", `''`, "the section bar belongs to authored ##, not to whichever tag the shell emits"},
		{`.y-prose [data-level='3'] {`, "font-size", "var(--fs-ed-20)", "an authored ### is the size it was when the tag was h3"},
		{`.y-prose [data-level='4'] {`, "font-size", "var(--fs-ed-19)", "an authored #### is a step of the reading scale, so a reader who enlarges the prose enlarges it too and it never ends up smaller than its own paragraphs"},
		{`.y-prose [data-level='4'] {`, "line-height", "1.5", "the smallest heading sits closest to the prose and takes the loosest measure of the four"},
		{`.y-prose [data-level='5'],
.y-prose [data-level='6'] {`, "font-size", "var(--fs-ed-17)", "the deepest two stop at the body's size instead of shrinking below it"},
		{`.y-prose [data-level='5'],
.y-prose [data-level='6'] {`, "font-family", "var(--font-display)", "the deepest two are headings, not the paragraph's face"},
		{`.y-prose [data-level='5'],
.y-prose [data-level='6'] {`, "margin", "var(--sp-h4) 0 0", "a heading with no room above it has less air than the paragraph it introduces"},
		{`.y-prose [data-level='5'],
.y-prose [data-level='6'] {`, "scroll-margin-top", "var(--header-clearance)", "a link may name any heading, and these two land under the sticky header without it"},
		// The openers of the last two start at a blank line, because the compound
		// rule the two share now also breaks its own selector list onto its own
		// lines, so a single newline before the sixth level's selector no longer
		// tells that rule's own line apart from the shared one; the blank line
		// before a standalone rule does.
		{"\n\n" + `.y-prose [data-level='5'] {`, "font-weight", "var(--fw-semibold)", "the fifth level is told from the sixth by weight, because both stop at the same size"},
		{"\n\n" + `.y-prose [data-level='6'] {`, "font-weight", "var(--fw-medium)", "the sixth level is the lighter of the two that share a size"},
		{"\n\n" + `.y-prose [data-level='6'] {`, "letter-spacing", "var(--track-wide)", "the sixth level is told from the fifth by its spacing, because both stop at the same size"},
		{`.y-toc__list a[data-level='3'] {`, "padding-left", "22px", "contents indent follows the authored level, so ### stays one step in"},
		{`.y-toc__list a[data-level='4'] {`, "padding-left", "32px", "contents indent follows the authored level, so #### stays the deepest step"},
	} {
		got := cssDeclarations(t, ruleBody(t, css, want.opener))
		if got[want.property] != want.value {
			t.Errorf("%s declares %s %q, want %q, because %s", strings.TrimSpace(want.opener), want.property, got[want.property], want.value, want.why)
		}
	}

	if strings.Contains(css, ".y-prose h4 {") {
		t.Errorf("heading look is still keyed on the tag .y-prose h4, so a demoted #### loses its size")
	}

	// The size rows and `.y-prose > :first-child` are both (0,2,0). The generic
	// reset used to beat the tag keys; it now ties the data-level keys and
	// loses by source order, so a note that opens with a heading gains that
	// heading's top margin. The heading-only reset is (0,3,0) and must sit
	// after the size rows so the tie cannot flip back silently.
	const openingReset = `.y-prose > [data-level]:first-child {`
	opening := cssDeclarations(t, ruleBody(t, css, openingReset))
	if opening["margin-top"] != "0" {
		t.Errorf("%s declares margin-top %q, want 0, so a note that opens with a heading stays flush with the chrome title", openingReset, opening["margin-top"])
	}
	if strings.Index(css, openingReset) < strings.Index(css, `.y-prose [data-level='1'] {`) {
		t.Error("the opening-heading margin reset is written before the size rows it has to beat, so a first-child heading keeps its 48px")
	}

	const built = "../../../assets/css/output.css"
	stylesheet, err := os.ReadFile(built)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", built, err)
	}
	if !strings.Contains(string(stylesheet), ".y-prose>[data-level]:first-child") &&
		!strings.Contains(string(stylesheet), `.y-prose > [data-level]:first-child`) {
		t.Error("the built stylesheet carries no opening-heading margin reset, so whatever the authored one says a note that opens with a heading keeps its top margin")
	}
}
