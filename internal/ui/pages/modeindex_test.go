package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAStudyPathRowStatesExtentAndNothingElse holds the index to the one figure
// a course may show. A number presented as how far the reader has got counted a
// status, so it fell as lessons were finished; the fix was to remove it, and a
// page listing every course is exactly where it would come back.
func TestAStudyPathRowStatesExtentAndNothingElse(t *testing.T) {
	t.Parallel()

	view := NewPathIndex([]nav.Path{
		{Title: "Go path", RelPath: "Maps/Go path.md", Planned: 4},
		{Title: "Unread structure", RelPath: "Maps/Broken.md", Diagnostics: []sequence.Diagnostic{{}}},
		{Title: "Plans nothing", RelPath: "Maps/Empty.md"},
	}, wording.ZhHant)

	want := []IndexRow{
		{Title: "Go path", Href: "/syllabus/Maps/Go%20path.md", Measure: "4 課"},
		{Title: "Unread structure", Href: "/syllabus/Maps/Broken.md", Measure: "0 課", Mark: "未讀到課程結構", MarkWarn: true},
		{Title: "Plans nothing", Href: "/syllabus/Maps/Empty.md", Measure: "0 課"},
	}
	if diff := cmp.Diff(want, view.Rows); diff != "" {
		t.Errorf("study-path rows mismatch (-want +got):\n%s", diff)
	}
	if view.Mode != pathMode {
		t.Errorf("study-path index is marked %q, want %q", view.Mode, pathMode)
	}
}

// TestAMapRowCountsBranchesAtEveryDepth keeps the measure describing the whole
// shape of the subject. Counting only the top level made a map of nine
// top-level headings and two hundred leaves read as the smaller of two maps.
func TestAMapRowCountsBranchesAtEveryDepth(t *testing.T) {
	t.Parallel()

	deep := nav.Map{Title: "Deep", RelPath: "Maps/Deep.md", Branches: []nav.Branch{
		{Heading: "One", Subbranches: []nav.Branch{{Heading: "One a"}, {Heading: "One b"}}},
		{Heading: "Two"},
	}}
	view := NewMapIndex([]nav.Map{deep}, wording.ZhHant)
	want := []IndexRow{{Title: "Deep", Href: "/notes/Maps/Deep.md", Measure: "4 枝"}}
	if diff := cmp.Diff(want, view.Rows); diff != "" {
		t.Errorf("map rows mismatch (-want +got):\n%s", diff)
	}
}

// TestAReportRowNamesItsKindAndItsDay keeps the two kinds of report apart and
// lifts the day out of a filename written as one. The kinds open differently —
// a briefing's bytes are shown inside an isolated frame, a written report is a
// note — so a row that named neither would leave the reader guessing which link
// they were about to follow.
func TestAReportRowNamesItsKindAndItsDay(t *testing.T) {
	t.Parallel()

	view := NewReportIndex([]nav.Report{
		{Name: "2026-07-10 vault audit.md", RelPath: "System/reports/2026-07-10 vault audit.md"},
		{Name: "notes.md", RelPath: "System/reports/notes.md"},
		{Name: "latest.html", RelPath: "System/reports/daily-briefing/latest.html", Briefing: true, Latest: true},
	}, wording.ZhHant)

	want := []IndexRow{
		{
			Title:   "2026-07-10 vault audit.md",
			Href:    "/notes/System/reports/2026-07-10%20vault%20audit.md",
			Date:    "2026-07-10",
			Measure: "書庫筆記",
		},
		{Title: "notes.md", Href: "/notes/System/reports/notes.md", Measure: "書庫筆記"},
		{Title: "latest.html", Href: "/reports/latest.html", Measure: "每日簡報", Mark: "最新"},
	}
	if diff := cmp.Diff(want, view.Rows); diff != "" {
		t.Errorf("report rows mismatch (-want +got):\n%s", diff)
	}
}

// TestLeadingDateReadsOnlyAWholeDayAtTheFront keeps the date cell out of the
// business of guessing. A name that merely starts with digits is not a day, and
// a row that showed one would be yomihon asserting something the author never
// wrote.
func TestLeadingDateReadsOnlyAWholeDayAtTheFront(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"2026-07-10.html", "2026-07-10"},
		{"2026-07-10 vault audit.md", "2026-07-10"},
		{"20260710.md", ""},
		{"2026-07.md", ""},
		{"v2026-07-10.md", ""},
		{"notes.md", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := leadingDate(tt.name); got != tt.want {
				t.Errorf("leadingDate(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestTheFolderIndexCountsEveryFileUnderEveryFolder keeps the kicker's figure
// answering the question a reader asks of a library — how much is in here —
// rather than how many things sit at the top of it.
func TestTheFolderIndexCountsEveryFileUnderEveryFolder(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	view := NewFolderIndex(model, wording.ZhHant)
	want := countNotes(model.RootNotes(), model.Folders())
	if want < 5 {
		t.Fatalf("the fixture vault holds %d files, too few for this to be measuring anything", want)
	}
	if view.Kicker != "資料夾 · "+plural(want, wording.FolderNoteCountOne, wording.FolderNoteCountMany, wording.ZhHant) {
		t.Errorf("folder index kicker = %q, want it to name %d files", view.Kicker, want)
	}
}

// TestEveryModeIndexNamesItself keeps a marker on each page that says which of
// the four modes it is, independent of the words on it. A check that had to
// recognise a page by its heading would be reading the reader's language, and
// would stop recognising it the moment the interface was switched.
func TestEveryModeIndexNamesItself(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	tests := []struct {
		mode      string
		component templ.Component
	}{
		{pathMode, ListIndex(NewPathIndex(model.Paths(), wording.ZhHant), layouts.Chrome{})},
		{mapMode, ListIndex(NewMapIndex(model.Maps(), wording.ZhHant), layouts.Chrome{})},
		{reportMode, ListIndex(NewReportIndex(model.Reports(), wording.ZhHant), layouts.Chrome{})},
		{folderMode, FolderIndex(NewFolderIndex(model, wording.ZhHant), layouts.Chrome{})},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := tt.component.Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if want := `data-index="` + tt.mode + `"`; !strings.Contains(buf.String(), want) {
				t.Errorf("the %s index does not carry %s", tt.mode, want)
			}
		})
	}
}
