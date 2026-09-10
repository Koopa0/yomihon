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
// as body prose and a link to it lands under the sticky header.
func TestProseHeadingLookFollowsAuthoredLevel(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := cssComments.ReplaceAllString(string(source), "")

	shared := cssDeclarations(t, ruleBody(t, css, `.y-prose [data-level="1"], .y-prose [data-level="2"], .y-prose [data-level="3"], .y-prose [data-level="4"] {`))
	if shared["scroll-margin-top"] != "var(--header-clearance)" {
		t.Errorf("authored levels 1–4 declare scroll-margin-top %q, want var(--header-clearance) so a demoted h5 still clears the sticky header", shared["scroll-margin-top"])
	}

	for _, want := range []struct {
		opener   string
		property string
		value    string
		why      string
	}{
		{`.y-prose [data-level="1"] {`, "font-size", "var(--fs-ed-30)", "an authored # is the size it was when the tag was h1"},
		{`.y-prose [data-level="2"] {`, "font-size", "var(--fs-ed-24)", "an authored ## is the size it was when the tag was h2"},
		{`.y-prose [data-level="2"]::before {`, "content", `""`, "the section bar belongs to authored ##, not to whichever tag the shell emits"},
		{`.y-prose [data-level="3"] {`, "font-size", "var(--fs-ed-20)", "an authored ### is the size it was when the tag was h3"},
		{`.y-prose [data-level="4"] {`, "font-size", "var(--fs-18)", "an authored #### is the size it was when the tag was h4"},
		{`.y-toc__list a[data-level="3"] {`, "padding-left", "22px", "contents indent follows the authored level, so ### stays one step in"},
		{`.y-toc__list a[data-level="4"] {`, "padding-left", "32px", "contents indent follows the authored level, so #### stays the deepest step"},
	} {
		got := cssDeclarations(t, ruleBody(t, css, want.opener))
		if got[want.property] != want.value {
			t.Errorf("%s declares %s %q, want %q, because %s", want.opener, want.property, got[want.property], want.value, want.why)
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
	if strings.Index(css, openingReset) < strings.Index(css, `.y-prose [data-level="1"] {`) {
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
