package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A heading's inline note is not part of the section's name on the page, so
// the check face reads the same name: the heading's own words reach it, and
// the note's words do not.
func TestInlineFootnoteInAHeadingIsNotPartOfTheSectionName(t *testing.T) {
	t.Parallel()
	got := ruleTargets(fragmentRun(t, map[string]string{
		"N.md": "## Heading^[note]\n\ntext\n",
		"C.md": "[[N#Heading]]\n\n[[N#Heading note]]\n",
	}))
	want := []string{"link.section_missing N#Heading note"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fragmentFindings(inline note in a heading) mismatch (-want +got):\n%s", diff)
	}
}
