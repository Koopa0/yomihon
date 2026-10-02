package layouts

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestTheArrowAfterAnExternalLinkIsAPictureOnly holds the arrow drawn after a
// link that opens in a new tab to generated content with an empty alternative.
// The sentence a listener is told is written into the link, so an arrow that
// carried a text of its own would be read out as well, once as an arrow and
// once as the sentence.
func TestTheArrowAfterAnExternalLinkIsAPictureOnly(t *testing.T) {
	t.Parallel()

	arrow := onlyRule(t, componentRules(t), ".y-prose a[target='_blank']::after")
	if diff := cmp.Diff([]string{`'\2197' / ''`}, arrow.values("content")); diff != "" {
		t.Errorf("the arrow's content is not the arrow with an empty alternative (-want +got):\n%s", diff)
	}
}
