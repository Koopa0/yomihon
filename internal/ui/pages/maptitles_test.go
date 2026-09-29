package pages

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
)

// drawerRow is one row of a sidebar drawer as a reader meets it: where it
// leads, the words it prints, and the language stamped on those words.
type drawerRow struct {
	Href string
	Name string
	Lang string
}

var drawerRowPattern = regexp.MustCompile(
	`<a class="ui-navitem[^"]*" href="(/notes/[^"]+)"[^>]*>\s*<span class="y-navdot" aria-hidden="true"></span>\s*<span(?: lang="([^"]*)")?>([^<]*)</span>`)

// drawerSection is the markup of the sidebar drawer marked group, up to the
// next drawer.
func drawerSection(t *testing.T, html, group string) string {
	t.Helper()
	_, after, found := strings.Cut(html, `data-sidebar-group="`+group+`"`)
	if !found {
		t.Fatalf("the sidebar has no %q drawer", group)
	}
	if next := strings.Index(after, `data-sidebar-group="`); next >= 0 {
		after = after[:next]
	}
	return after
}

// drawerRows reads the rows of the sidebar drawer marked group, up to the next
// drawer.
func drawerRows(t *testing.T, html, group string) []drawerRow {
	t.Helper()
	after := drawerSection(t, html, group)
	var rows []drawerRow
	for _, m := range drawerRowPattern.FindAllStringSubmatch(after, -1) {
		rows = append(rows, drawerRow{Href: m[1], Lang: m[2], Name: m[3]})
	}
	return rows
}

// TestMapRowsNameTheNoteAsTheAuthorAndNoteDeclared holds the name a map prints
// for a row in the sidebar's maps drawer, which every page carries: the alias
// the row wrote, else the resolved note's own title, else the link text. A link
// to a heading keeps its link text, a separator with nothing after it wrote no
// alias, and a row that resolves to no note is not drawn at all. Each name
// carries the language its note declared.
func TestMapRowsNameTheNoteAsTheAuthorAndNoteDeclared(t *testing.T) {
	t.Parallel()
	model := modelOf(t, "testdata/maptitles")
	chrome := recordedChrome()

	const (
		slice = "/notes/Lessons/slice-declare.md"
		iota  = "/notes/Lessons/iota-constants.md"
		last  = "/notes/Lessons/last-lesson.md"
		bare  = "/notes/Lessons/no-title.md"
	)
	want := []drawerRow{
		{Href: slice, Name: "Slices, in my words"},
		{Href: iota, Name: "iota: The Compile-Time Constant Generator", Lang: "ja"},
		{Href: bare, Name: "Lessons/no-title"},
		{Href: iota, Name: "Lessons/iota-constants#Part A", Lang: "ja"},
		{Href: last, Name: "Last: The Final Lesson"},
		{Href: bare, Name: "Lessons/no-title"},
		{Href: last, Name: "Last: The Final Lesson"},
	}
	for _, current := range []string{"Lessons/unlisted.md", "Lessons/slice-declare.md"} {
		t.Run("beside "+current, func(t *testing.T) {
			t.Parallel()
			html := renderedHTML(t, sidebar(NewSidebar(nav.Shell{Nav: model}, current), chrome))
			if diff := cmp.Diff(want, drawerRows(t, html, "maps")); diff != "" {
				t.Errorf("maps drawer rows (-want +got):\n%s", diff)
			}
			// The row to a note nobody wrote is dropped, not drawn as a
			// broken row under its link text.
			if section := drawerSection(t, html, "maps"); strings.Contains(section, "y-navitem--broken") || strings.Contains(section, "not-written-yet") {
				t.Errorf("the maps drawer draws the row that resolves to no note")
			}
		})
	}
}

// TestOneNoteHasOneNameInThePathsAndMapsDrawers holds that a note listed
// without an alias by both a course and a map prints the same words in both
// drawers, and the same language stamp.
func TestOneNoteHasOneNameInThePathsAndMapsDrawers(t *testing.T) {
	t.Parallel()
	model := modelOf(t, "testdata/maptitles")
	html := renderedHTML(t, sidebar(NewSidebar(nav.Shell{Nav: model}, "Lessons/unlisted.md"), recordedChrome()))

	paths := drawerRows(t, html, "paths")
	maps := drawerRows(t, html, "maps")
	if len(paths) != 3 || len(maps) != 7 {
		t.Fatalf("drawer rows = %d paths, %d maps, want 3 and 7", len(paths), len(maps))
	}
	// The course lists iota, last-lesson and no-title with no alias. The map
	// lists the same three plainly at positions 1, 6 and 2; its other rows
	// speak in an alias, a heading, or a bare separator and are held by the
	// test above.
	for _, tt := range []struct{ path, mapRow int }{{0, 1}, {1, 6}, {2, 2}} {
		if diff := cmp.Diff(paths[tt.path], maps[tt.mapRow]); diff != "" {
			t.Errorf("the course row %d and the map row %d name one note differently (-paths +maps):\n%s", tt.path, tt.mapRow, diff)
		}
	}
}
