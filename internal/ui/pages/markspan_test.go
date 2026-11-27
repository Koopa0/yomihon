package pages

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestAMarksWrappersWriteWhatTheyAreGivenAsText holds the attributes of the
// wrappers a mark's passage is drawn in: what is put into them is text, whatever
// it holds, so a value that carries a quote or an angle bracket cannot end the
// attribute it stands in and begin another thing.
func TestAMarksWrappersWriteWhatTheyAreGivenAsText(t *testing.T) {
	t.Parallel()
	m := &MarkNote{ID: `a" onfocus="x`, Kind: `thought"><b>`, Lang: wording.En}
	span := m.Span(0, 4)
	for name, markup := range map[string]string{"first": span.First, "rest": span.Rest} {
		if strings.Contains(markup, `onfocus="x`) || strings.Contains(markup, "<b>") {
			t.Errorf("the %s wrapper lets a value end its attribute: %s", name, markup)
		}
	}
}
