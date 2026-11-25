package layouts

import (
	"bytes"
	"strings"
	"testing"
)

// TestTheReadingLineIsDrawnOnlyOverANote holds where the header carries the
// line that says how far through the page the reader is: a page reading a note
// has a start and an end to be part way between, and a list, a form or the
// desk has neither, so their header draws no line at all rather than one that
// grows with a scroll meaning nothing.
func TestTheReadingLineIsDrawnOnlyOverANote(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		reading *ReadingNote
		want    int
	}{
		{name: "a note", reading: &ReadingNote{Vault: "v", Path: "Notes/alpha.md"}, want: 1},
		{name: "any other page", reading: nil, want: 0},
	} {
		var buf bytes.Buffer
		if err := header(Chrome{Reading: tc.reading}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("%s: render header: %v", tc.name, err)
		}
		if got := strings.Count(buf.String(), `class="y-readline"`); got != tc.want {
			t.Errorf("%s: the header draws %d reading lines, want %d", tc.name, got, tc.want)
		}
	}
}
