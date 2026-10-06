package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestCombiningMarkLinksNameTheSecondActualHeading(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, first, second string }{
		{"Hindi vowel", "कि", "की"},
		{"Thai tone", "ปู่", "ปู"},
		{"variation selector", "葛城", "葛\U000e0100城"},
		{"Kana mark", "か行", "か\u309a行"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := "## " + tt.first + "\n\nFirst passage.\n\n## " + tt.second + "\n\nSecond passage.\n"
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
			destination := r.HTML("Target.md", "", body, wording.En)
			want := []string{tt.first, tt.second}
			if diff := cmp.Diff(want, allMatches(elementID, destination.HTML)); diff != "" {
				t.Errorf("HTML(mark headings) ids mismatch (-want +got):\n%s", diff)
			}
			source := r.HTML("Source.md", "", "[[Target#"+tt.second+"|second]]", wording.En)
			if !strings.Contains(source.HTML, `href="/notes/Target.md#`+tt.second+`"`) || len(source.Diagnostics) != 0 {
				t.Errorf("HTML(second mark link) = %+v, want the second actual heading id without diagnostics", source)
			}
		})
	}
}
