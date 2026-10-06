package render_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestHTMLFiveThousandDuplicateHeadings(t *testing.T) {
	t.Parallel()
	const count = 5000
	r := newRenderer(t, nil, nil, nil)
	got := r.HTML("journal.md", "", strings.Repeat("## Repeat\n\n", count), wording.ZhHant)
	wantTOC := make([]render.TOCEntry, count)
	var wantHTML strings.Builder
	for i := range count {
		id := "repeat"
		if i > 0 {
			id = fmt.Sprintf("repeat-%d", i+1)
		}
		wantTOC[i] = render.TOCEntry{Level: 2, Text: "Repeat", ID: id}
		fmt.Fprintf(&wantHTML, "<h3 id=\"%s\" data-level=\"2\">Repeat</h3>\n", id)
	}
	want := render.Result{HTML: wantHTML.String(), TOC: wantTOC}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("5000 heading identities changed (-want +got):\n%s", diff)
	}
}
