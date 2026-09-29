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

// TestThoughtDoorsStayInsideTheirPlaces locks the two places the doors live:
// each heading of the contents list is one row that carries its own section
// door, and the page-level door stands in the head's actions before the seam.
// Every door is asserted against literal bytes, not the constants that draw it.
func TestThoughtDoorsStayInsideTheirPlaces(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		lang      wording.Lang
		page      string
		sectionOf func(heading string) string
	}{
		{wording.ZhHant, "留下自己的想法", func(h string) string { return `aria-label="留想法：` + h + `"` }},
		{wording.En, "Leave a thought", func(h string) string { return `aria-label="Leave a thought: ` + h + `"` }},
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

			rows := regexp.MustCompile(`(?s)<div class="y-toc__row">(.*?)</div>`).FindAllStringSubmatch(page, -1)
			// The list is drawn twice (rail and inline fold); each copy has one
			// row per heading, never a second row for the door.
			if len(rows) == 0 || len(rows)%len(headings) != 0 {
				t.Fatalf("contents rows = %d, want a whole number of copies of %d rows", len(rows), len(headings))
			}
			for i, row := range rows {
				heading := headings[i%len(headings)]
				if got := strings.Count(row[1], `href="#`+heading.ID+`"`); got != 1 {
					t.Errorf("row %d holds %d links to #%s, want 1:\n%s", i, got, heading.ID, row[1])
				}
				if got := strings.Count(row[1], `class="y-toc__door"`); got != 1 {
					t.Errorf("row %d holds %d section doors, want 1", i, got)
				}
				if !strings.Contains(row[1], tc.sectionOf(heading.Text)) {
					t.Errorf("row %d door is not named for %q:\n%s", i, heading.Text, row[1])
				}
				if !strings.Contains(row[1], `href="/thought/Notes/Probe.md?section=`+heading.ID+`"`) {
					t.Errorf("row %d door does not open the section handoff:\n%s", i, row[1])
				}
			}
			if got := strings.Count(page, `class="y-toc__door"`); got != len(rows) {
				t.Errorf("section doors = %d, rows = %d; a door outside a row makes a second row", got, len(rows))
			}

			seam := strings.Index(page, `class="y-seam"`)
			door := strings.Index(page, `href="/thought/Notes/Probe.md"`)
			if seam < 0 || door < 0 {
				t.Fatalf("seam at %d, page door at %d; both must be drawn", seam, door)
			}
			if door > seam {
				t.Errorf("the page door is drawn after the seam (%d > %d), it belongs in the head's actions", door, seam)
			}
			if !strings.Contains(page, `class="y-metarow__raw" lang="`+tc.lang.Tag()+`" href="/thought/Notes/Probe.md">`+tc.page+`</a>`) {
				t.Errorf("the page door does not use the head actions' link style and words %q", tc.page)
			}
		})
	}
}
