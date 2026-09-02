package render

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestEmbedSpans pins the excerpt scan's contract on assembled page HTML
// directly: every excerpt container of either class is reported, leftmost
// first, from its opening tag to the close that balances it — a div nested
// inside an excerpt stays inside its span, an excerpt starting exactly where
// the previous one ends is still found, and a tail with no balancing close
// ends its span at the end of the document. The fixture is built by
// concatenation so the expected offsets are recorded where each segment is
// written rather than counted by hand, and the openers are spelled from the
// same table the scan reads.
func TestEmbedSpans(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("<p>lead</p>")
	s1 := b.Len()
	b.WriteString(embedOpeners[0])
	b.WriteString(`<h2>excerpt heading</h2><div class="inner">nested</div>words</div>`)
	e1 := b.Len()
	b.WriteString("<p>between</p>")
	s2 := b.Len()
	b.WriteString(embedOpeners[1])
	b.WriteString(`widened words</div>`)
	e2 := b.Len()
	// The third excerpt opens at the byte the second one ends on, with no
	// prose between, and never closes.
	b.WriteString(embedOpeners[0])
	b.WriteString("unclosed tail")
	doc := b.String()

	want := [][2]int{{s1, e1}, {s2, e2}, {e2, len(doc)}}
	if diff := cmp.Diff(want, embedSpans(doc)); diff != "" {
		t.Errorf("embedSpans spans mismatch (-want +got):\n%s", diff)
	}

	if got := embedSpans("<p>no excerpt at all</p>"); got != nil {
		t.Errorf("embedSpans on a page with no excerpt = %v, want nil", got)
	}
}
