package pages

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestSidebarCourseShowsOnePath(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "Writing/lessons/go/L01.md"

	var buf bytes.Buffer
	if err := sidebar(NewNoteSidebar(nav.Shell{Nav: model}, current, "golang"), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-sidebar-mode="course"`,
		`data-book-path="Maps/Go path.md"`,
		`data-book-branch=`,
		`aria-current="page"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("book rail missing %q; html = %q", want, html)
		}
	}
	if strings.Contains(html, `y-lessonsteps`) {
		t.Error("book rail still contains the lesson steps")
	}
	// One book, and the reader's manual choices on its branches.
	// The browser probe owns restoration behavior; this render check ensures
	// that this rail emits exactly one nonce-bound initializer to read them.
	if got := strings.Count(html, `sessionStorage.getItem("yomihon.nav")`); got != 1 {
		t.Errorf("book rail has %d disclosure-state readers, want one", got)
	}
	if !strings.Contains(html, `</aside><script nonce="n">`) {
		t.Error("book rail does not initialize its state after its markup with the response nonce")
	}
}

func TestSidebarFolderShowsSiblings(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "Concepts/go/C01.md"

	var buf bytes.Buffer
	if err := sidebar(NewNoteSidebar(nav.Shell{Nav: model}, current, "golang"), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-sidebar-mode="folder"`,
		`class="y-here"`,
		`href="/notes/Concepts/go/C01.md"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("folder rail missing %q", want)
		}
	}
}

func TestSidebarFolderListsReports(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "System/reports/2026-07-10 vault audit.md"

	var buf bytes.Buffer
	if err := sidebar(NewNoteSidebar(nav.Shell{Nav: model}, current, ""), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-sidebar-mode="folder"`,
		`Vault audit`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("reports rail missing %q", want)
		}
	}
}

// TestTheContentsHangUnderTheRowThatIsTheNoteInEveryKindOfRail holds the one
// place the note's own contents are drawn: in the sidebar, immediately under the
// row that is the note, whichever of the three rails the note has. A rail that
// forgot to hang them left the page with no contents anywhere, since the right
// rail they used to be in is gone, and nothing but a render of each rail says
// so. The list is there once, it follows the row and no other row, it carries
// the headings in order, and it ends on the row that leads down to what the
// note ends on.
func TestTheContentsHangUnderTheRowThatIsTheNoteInEveryKindOfRail(t *testing.T) {
	t.Parallel()
	course, folder := footVault(t), buildModel(t)
	for _, tt := range []struct {
		name    string
		model   *nav.Model
		current string
		domain  string
		kind    SidebarMode
	}{
		{"a lesson in a course", course, "Writing/C03.md", "", SidebarCourse},
		{"a note in no course", folder, "Concepts/go/C01.md", "golang", SidebarFolder},
		{"a report", folder, "System/reports/2026-07-10 vault audit.md", "", SidebarFolder},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := NoteView{
				RelPath: tt.current,
				TOC:     []render.TOCEntry{{Level: 2, Text: "First heading", ID: "first"}, {Level: 3, Text: "Second heading", ID: "second"}},
				CitedBy: []nav.NoteRef{{Name: "Elsewhere", RelPath: "Elsewhere.md"}},
			}
			rail := NewNoteSidebar(nav.Shell{Nav: tt.model}, tt.current, tt.domain)
			if rail.Mode != tt.kind {
				t.Fatalf("the rail for %s is %q, want %q, so this is not the case this subtest is about", tt.current, rail.Mode, tt.kind)
			}
			rail.note = &view
			html := renderedHTML(t, sidebar(rail, layouts.Chrome{Nonce: "n", Lang: wording.En}))

			if got := strings.Count(html, `<nav class="y-railtoc"`); got != 1 {
				t.Fatalf("the rail draws %d lists of the note's contents, want exactly one:\n%s", got, html)
			}
			// The row that is the note is the one marked as the page, and the
			// contents open directly after it, with no other row between.
			row := regexp.MustCompile(`<a class="ui-navitem is-active"[^>]*aria-current="page">.*?</a>`).FindStringIndex(html)
			if row == nil {
				t.Fatalf("no row of the rail is the note, so nothing can hold its contents:\n%s", html)
			}
			contents := strings.TrimLeft(html[row[1]:], " ")
			if !strings.HasPrefix(contents, `<nav class="y-railtoc"`) {
				t.Fatalf("the contents do not open directly after the row that is the note; what follows it is %q", contents[:min(len(contents), 120)])
			}
			first, second, end := strings.Index(contents, "First heading"), strings.Index(contents, "Second heading"), strings.Index(contents, `href="#_y-end"`)
			if first < 0 || second < first || end < second {
				t.Errorf("the contents are not the two headings in order and then the row that leads down (%d, %d, %d):\n%s", first, second, end, contents)
			}
		})
	}
}

// TestTheLibraryListsWhatTheFolderHasAndMarksWhereThePageStands holds the
// library mode: the desk always, each list only where it has something in it
// with how many, the page's own list marked as the one it stands on, and the
// top-level folders below under their own name.
func TestTheLibraryListsWhatTheFolderHasAndMarksWhereThePageStands(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	html := renderedHTML(t, sidebar(NewLibrarySidebar(recordedShell(model), LibraryCourses, ""), layouts.Chrome{Nonce: "n", Lang: wording.En}))

	for _, want := range []string{
		`data-sidebar-mode="library"`,
		`href="/"`,
		`href="` + indexHref(courseMode) + `"`,
		`aria-label="` + wording.Folders.In(wording.En) + `"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("library sidebar is missing %q:\n%s", want, html)
		}
	}
	current := regexp.MustCompile(`<a class="ui-navitem is-active" href="([^"]*)" aria-current="page">`).FindAllStringSubmatch(html, -1)
	if len(current) != 1 || current[0][1] != indexHref(courseMode) {
		t.Errorf("the library marks %v as where the page stands, want only the courses list", current)
	}
	if strings.Contains(html, "data-drawer-only") {
		t.Error("a library sidebar that a page draws as its column says it is only a drawer")
	}
}

// TestADrawerOnlySidebarSaysSoAndKeepsTheLibrary holds the form the pages that
// have no column from the desk width up draw: the same element, the same
// library, marked so the stylesheet can keep it out of the page's way.
func TestADrawerOnlySidebarSaysSoAndKeepsTheLibrary(t *testing.T) {
	t.Parallel()
	html := renderedHTML(t, sidebar(NewDrawerSidebar(recordedShell(buildModel(t))), layouts.Chrome{Nonce: "n", Lang: wording.En}))
	for _, want := range []string{`data-sidebar-mode="library"`, ` data-drawer-only`} {
		if !strings.Contains(html, want) {
			t.Errorf("drawer-only sidebar is missing %q:\n%s", want, html)
		}
	}
}

// TestTheIndexSidebarListsEachIndexAndMarksTheOneBeingRead holds the index
// mode: a note that is an index stands beside the folder's indexes, not its
// folder, with its own row marked.
func TestTheIndexSidebarListsEachIndexAndMarksTheOneBeingRead(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	maps := model.Maps()
	if len(maps) == 0 {
		t.Fatal("the fixture holds no index, so this proves nothing")
	}
	rel := maps[0].RelPath
	sb := NewNoteSidebar(recordedShell(model), rel, "")
	if sb.Mode != SidebarIndex {
		t.Fatalf("a note that is an index stands on the %q sidebar, want %q", sb.Mode, SidebarIndex)
	}
	html := renderedHTML(t, sidebar(sb, layouts.Chrome{Nonce: "n", Lang: wording.En}))
	if !strings.Contains(html, `data-sidebar-mode="index"`) || !strings.Contains(html, `href="`+notesHref(rel)+`" aria-current="page"`) {
		t.Errorf("the index sidebar does not mark the index being read (%s):\n%s", rel, html)
	}
}
