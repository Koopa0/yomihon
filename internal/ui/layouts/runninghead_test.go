package layouts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheRunningHeadIsDrawnOnlyOverANoteWithSections holds where the header
// carries the running head and the contents it opens: over a note that has
// sections, and nowhere else. The press names the popover by id, so the two are
// drawn together or not at all, and each section is a jump to its heading, in
// the note's own language.
func TestTheRunningHeadIsDrawnOnlyOverANoteWithSections(t *testing.T) {
	t.Parallel()
	head := &RunningHead{Language: "ja", Sections: []RunningSection{{ID: "first", Text: "第一"}, {ID: "second", Text: "第二"}}}
	for _, tc := range []struct {
		name string
		head *RunningHead
		want int
	}{
		{name: "a note with sections", head: head, want: 1},
		{name: "any other page", head: nil, want: 0},
	} {
		var buf bytes.Buffer
		if err := header(Chrome{RunningHead: tc.head, Lang: wording.En}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("%s: render header: %v", tc.name, err)
		}
		got := buf.String()
		if n := strings.Count(got, `popovertarget="_y-contents"`); n != tc.want {
			t.Errorf("%s: the header draws %d running heads, want %d", tc.name, n, tc.want)
		}
		if n := strings.Count(got, `id="_y-contents"`); n != tc.want {
			t.Errorf("%s: the header draws %d contents for a running head, want %d", tc.name, n, tc.want)
		}
		if tc.want == 0 {
			continue
		}
		_, spacer, _ := strings.Cut(got, `<div class="y-header__spacer">`)
		if !strings.HasPrefix(spacer, `<button class="y-runhead"`) {
			t.Errorf("%s: the running head is not drawn in the room between the folder's name and the search: %.120q", tc.name, spacer)
		}
		for _, want := range []string{`href="#first"`, `href="#second"`, `<span class="y-contentspop__title">第一</span>`, `class="y-toc__list" lang="ja"`, `class="y-runhead__title" lang="ja"`} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: the running head's contents carry no %s", tc.name, want)
			}
		}
	}
}
