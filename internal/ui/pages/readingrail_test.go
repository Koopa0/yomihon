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

func TestReadingRailBookShowsOnePath(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "Writing/lessons/go/L01.md"

	var buf bytes.Buffer
	if err := readingRail(NewReadingRail(nav.Shell{Nav: model}, current, "golang"), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-reading-rail="book"`,
		`data-book-path="Maps/Go path.md"`,
		`data-book-branch=`,
		`aria-current="page"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("book rail missing %q; html = %q", want, html)
		}
	}
	for _, ban := range []string{
		`y-lessonsteps`,
		`data-sidebar-group="paths"`,
		`data-sidebar-group="maps"`,
		`data-sidebar-group="journal"`,
		`data-sidebar-group="reports"`,
	} {
		if strings.Contains(html, ban) {
			t.Errorf("book rail still contains %q", ban)
		}
	}
	// One book replaces the vault drawers, not the reader's manual choices.
	// The browser probe owns restoration behavior; this render check ensures
	// that this rail emits exactly one nonce-bound initializer to read them.
	if got := strings.Count(html, `sessionStorage.getItem("yomihon.nav")`); got != 1 {
		t.Errorf("book rail has %d disclosure-state readers, want one", got)
	}
	if !strings.Contains(html, `</aside><script nonce="n">`) {
		t.Error("book rail does not initialize its state after its markup with the response nonce")
	}
}

func TestReadingRailFolderShowsSiblings(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "Concepts/go/C01.md"

	var buf bytes.Buffer
	if err := readingRail(NewReadingRail(nav.Shell{Nav: model}, current, "golang"), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-reading-rail="folder"`,
		`class="y-here"`,
		`href="/notes/Concepts/go/C01.md"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("folder rail missing %q", want)
		}
	}
	if strings.Contains(html, `data-sidebar-group=`) {
		t.Error("folder rail still carries vault drawer groups")
	}
}

func TestReadingRailReportsListsReports(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "System/reports/2026-07-10 vault audit.md"

	var buf bytes.Buffer
	if err := readingRail(NewReadingRail(nav.Shell{Nav: model}, current, ""), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-reading-rail="reports"`,
		`data-reading-reports`,
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
		kind    ReadingRailKind
	}{
		{"a lesson in a course", course, "Writing/C03.md", "", ReadingRailBook},
		{"a note in no course", folder, "Concepts/go/C01.md", "golang", ReadingRailFolder},
		{"a report", folder, "System/reports/2026-07-10 vault audit.md", "", ReadingRailReports},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := NoteView{
				RelPath: tt.current,
				TOC:     []render.TOCEntry{{Level: 2, Text: "First heading", ID: "first"}, {Level: 3, Text: "Second heading", ID: "second"}},
				CitedBy: []nav.NoteRef{{Name: "Elsewhere", RelPath: "Elsewhere.md"}},
			}
			rail := NewReadingRail(nav.Shell{Nav: tt.model}, tt.current, tt.domain)
			if rail.Kind != tt.kind {
				t.Fatalf("the rail for %s is %q, want %q, so this is not the case this subtest is about", tt.current, rail.Kind, tt.kind)
			}
			rail.note = &view
			html := renderedHTML(t, readingRail(rail, layouts.Chrome{Nonce: "n", Lang: wording.En}))

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
