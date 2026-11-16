package pages

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestThoughtDoorsStayInsideTheirPlaces locks the places a door is not: the
// sidebar's contents list is a list of links, one row per heading and no door
// beside any, and the one page-level door stands among the facts at the end of
// the text, after the prose, beside the file's own. A door for each section
// stands in that section's margin, which sections_test.go holds.
// Every door is asserted against literal bytes, not the constants that draw it.
func TestThoughtDoorsStayInsideTheirPlaces(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		lang wording.Lang
		door string
	}{
		{wording.ZhHant, `<a class="y-notefacts__door" lang="zh-Hant" href="/thought/Notes/Probe.md" aria-label="留下自己的想法：Probe">留下自己的想法</a>`},
		{wording.En, `<a class="y-notefacts__door" lang="en" href="/thought/Notes/Probe.md" aria-label="Leave a thought: Probe">Leave a thought</a>`},
	} {
		t.Run(tc.lang.Tag(), func(t *testing.T) {
			t.Parallel()
			headings := []render.TOCEntry{
				{Level: 2, Text: "First", ID: "first"},
				{Level: 3, Text: "Second", ID: "second"},
				{Level: 2, Text: "Third", ID: "third"},
			}
			view := NoteView{
				Title:       "Probe",
				RelPath:     "Notes/Probe.md",
				Type:        "note",
				ThoughtDoor: true,
				TOC:         headings,
				BodyHTML:    "<p>body</p>",
			}
			var buf bytes.Buffer
			if err := Note(view, layouts.Chrome{Lang: tc.lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			page := buf.String()

			// The contents hang under the row that is the note in the sidebar,
			// which is where the list is drawn; one row per heading and nothing
			// else in it.
			contents := renderedHTML(t, railContents(&view, tc.lang))
			rows := regexp.MustCompile(`(?s)<div class="y-toc__row">(.*?)</div>`).FindAllStringSubmatch(contents, -1)
			if len(rows) != len(headings) {
				t.Fatalf("contents rows = %d, want %d: one per heading", len(rows), len(headings))
			}
			for i, row := range rows {
				heading := headings[i]
				if got := strings.Count(row[1], `href="#`+heading.ID+`"`); got != 1 {
					t.Errorf("row %d holds %d links to #%s, want 1:\n%s", i, got, heading.ID, row[1])
				}
				if strings.Count(row[1], "<a ") != 1 || strings.Contains(row[1], "/thought/") {
					t.Errorf("row %d holds more than its link, a door beside the heading it names:\n%s", i, row[1])
				}
			}
			if strings.Contains(contents, "/thought/") || strings.Contains(page, `y-toc__door`) {
				t.Errorf("the sidebar carries a door to a thought; a section's door stands in its own margin")
			}

			// The facts end the text, so the prose's own opening is the anchor
			// the door has to come after.
			prose := strings.Index(page, `<div class="y-prose">`)
			facts := strings.Index(page, `<dl class="y-notefacts">`)
			if prose < 0 || facts < prose {
				t.Fatalf("prose at %d, facts at %d; want the facts after the prose", prose, facts)
			}
			if got := strings.Count(page, `href="/thought/Notes/Probe.md"`); got != 1 {
				t.Errorf("page-level doors = %d, want 1", got)
			}
			if at := strings.Index(page, tc.door); at < facts {
				t.Errorf("the page door %q is at %d, not among the facts that begin at %d", tc.door, at, facts)
			}
		})
	}
}
