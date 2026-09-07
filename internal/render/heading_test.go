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

// TestHeadingWordsDropsAnUnspokenReadAloudMarker locks the read-aloud
// branch of the shared tag walk. An instruction the renderer cannot carry
// out is dropped from the body; a heading that names a section must drop
// it too, or the marker becomes part of the id. The ja form is
// allowlisted and then stripped as a tag; this row is the unmarked
// language, which only the drop arm handles. Deleting that arm leaves
// the escaped comment in the heading's words.
func TestHeadingWordsDropsAnUnspokenReadAloudMarker(t *testing.T) {
	t.Parallel()

	got := HeadingWords("Spoken <!-- read-aloud: fr --> title")
	if got != "Spoken  title" {
		t.Errorf("an unspoken read-aloud marker stayed in the heading name: got %q", got)
	}
}

func TestShellHeadingLevelStepsDownAndClamps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		authored int
		want     int
	}{
		{1, 2},
		{2, 3},
		{5, 6},
		{6, 6},
	}
	for _, tt := range tests {
		if got := shellHeadingLevel(tt.authored); got != tt.want {
			t.Errorf("shellHeadingLevel(%d) = %d, want %d", tt.authored, got, tt.want)
		}
	}
}

func TestAssignHeadingIDsDemotesBodyHeadingsUnderTheTitle(t *testing.T) {
	t.Parallel()
	got, toc := assignHeadingIDs("<h1>Alpha</h1><h6>Zeta</h6>", "")
	if !strings.Contains(got, `<h2 id="alpha" data-level="1">Alpha</h2>`) {
		t.Errorf("authored h1 was not written as h2 carrying its authored level:\n%s", got)
	}
	if !strings.Contains(got, `<h6 id="zeta" data-level="6">Zeta</h6>`) {
		t.Errorf("authored h6 must stay h6:\n%s", got)
	}
	if strings.Contains(got, "<h1") {
		t.Errorf("a body heading survived as h1:\n%s", got)
	}
	want := []TOCEntry{
		{Level: 1, Text: "Alpha", ID: "alpha"},
		{Level: 6, Text: "Zeta", ID: "zeta"},
	}
	if diff := cmp.Diff(want, toc); diff != "" {
		t.Errorf("TOC mismatch (-want +got):\n%s", diff)
	}
}
