package pages

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
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

	view := NewPathIndex([]nav.Course{
		{Title: "Go path", RelPath: "Maps/Go path.md", Planned: 4},
		{Title: "Unread structure", RelPath: "Maps/Broken.md", Diagnostics: []sequence.Diagnostic{{}}},
		{Title: "Plans nothing", RelPath: "Maps/Empty.md"},
		// A path of notes that are not lessons is counted in items, in the same
		// place and the same single figure.
		{Title: "Queue kit", RelPath: "Maps/QueueKit.md", Planned: 2},
	}, schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, wording.ZhHant, nil)

	want := []Row{
		{Text: "Go path", Href: "/courses/Maps/Go%20path.md", Mark: "4 單元"},
		{Text: "Unread structure", Href: "/courses/Maps/Broken.md", Mark: "0 單元 · 未讀到課程結構", Fault: true},
		{Text: "Plans nothing", Href: "/courses/Maps/Empty.md", Mark: "0 單元"},
		{Text: "Queue kit", Href: "/courses/Maps/QueueKit.md", Mark: "2 單元"},
	}
	if diff := cmp.Diff(want, view.Shelf.Rows); diff != "" {
		t.Errorf("study-path rows mismatch (-want +got):\n%s", diff)
	}
	if view.Mode != courseMode {
		t.Errorf("study-path index is marked %q, want %q", view.Mode, courseMode)
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
	view := NewMapIndex([]nav.Map{deep}, schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, wording.ZhHant, nil)
	want := []Row{{Text: "Deep", Href: "/notes/Maps/Deep.md", Mark: "4 枝"}}
	if diff := cmp.Diff(want, view.Shelf.Rows); diff != "" {
		t.Errorf("map rows mismatch (-want +got):\n%s", diff)
	}
}

// A map written as headings and prose still has a branch count: the mark is
// the number of surviving headings, which is the tree the rail draws, not a
// second figure invented for the shelf.
func TestAProseMapRowCountsTheBranchesTheRailWouldDraw(t *testing.T) {
	t.Parallel()

	prose := nav.Map{Title: "A map written as prose", RelPath: "Maps/Prose map.md", Branches: []nav.Branch{
		{Heading: "Authors", Entries: []nav.MapEntry{{Name: "Norwegian Wood"}, {Name: "Kafka on the Shore"}}},
		{Heading: "Forms", Entries: []nav.MapEntry{{Name: "The Wind-Up Bird Chronicle"}}},
		{Heading: "Editions", Entries: []nav.MapEntry{{Name: "《挪威的森林》"}, {Name: "海辺のカフカ"}}},
		{Heading: "Places — [[Sputnik Sweetheart]]", Entries: []nav.MapEntry{{Name: "Sputnik Sweetheart"}}},
		{Heading: "After", Entries: []nav.MapEntry{{Name: "Colorless Tsukuru Tazaki"}}},
	}}
	view := NewMapIndex([]nav.Map{prose}, schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, wording.ZhHant, nil)
	want := []Row{{Text: "A map written as prose", Href: "/notes/Maps/Prose%20map.md", Mark: "5 枝"}}
	if diff := cmp.Diff(want, view.Shelf.Rows); diff != "" {
		t.Errorf("prose map shelf mark mismatch (-want +got):\n%s", diff)
	}
}

// TestAReportRowOpensWithItsTitleThenItsDay pins the faces a report row shows
// and which face each thing lands on. The row opens with what the report calls
// itself — never the name of its file — and the day it is for follows. Every
// row is a report, so none carries a mark of its kind.
func TestAReportRowOpensWithItsTitleThenItsDay(t *testing.T) {
	t.Parallel()

	reports := []nav.Report{
		{
			Name:    "2026-07-10 vault audit.md",
			Title:   "Vault audit",
			RelPath: "System/reports/2026-07-10 vault audit.md",
			Date:    "2026-07-10",
			Opening: "Four notes went from draft to ready.",
		},
		{Name: "notes.md", Title: "notes", RelPath: "System/reports/notes.md"},
	}

	for _, tt := range []struct {
		lang wording.Lang
		want []Row
	}{
		{
			lang: wording.ZhHant,
			want: []Row{
				{
					When:    "2026-07-10",
					Text:    "Vault audit",
					Opening: "Four notes went from draft to ready.",
					Href:    "/notes/System/reports/2026-07-10%20vault%20audit.md",
					ByTitle: true,
				},
				{When: "沒有寫日期", Text: "notes", Href: "/notes/System/reports/notes.md", ByTitle: true},
			},
		},
		{
			lang: wording.En,
			want: []Row{
				{
					When:    "2026-07-10",
					Text:    "Vault audit",
					Opening: "Four notes went from draft to ready.",
					Href:    "/notes/System/reports/2026-07-10%20vault%20audit.md",
					ByTitle: true,
				},
				{When: "No date", Text: "notes", Href: "/notes/System/reports/notes.md", ByTitle: true},
			},
		},
	} {
		view := NewReportIndex(reports, tt.lang, nil)
		if diff := cmp.Diff(tt.want, view.Shelf.Rows); diff != "" {
			t.Errorf("report rows in %v (-want +got):\n%s", tt.lang, diff)
		}
	}
}

// TestEveryReportRowAnswersInTheDateColumn keeps the column a reader scans
// whole. A row left blank there reads as a day the page failed to look up,
// which is a different claim from the one the shelf is making — that this
// report never wrote one.
func TestEveryReportRowAnswersInTheDateColumn(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		view := NewReportIndex([]nav.Report{
			{Name: "a.md", Title: "Vault audit", RelPath: "System/reports/a.md", Date: "2026-07-10"},
			{Name: "notes.md", Title: "notes", RelPath: "System/reports/notes.md"},
		}, lang, nil)
		for i, row := range view.Shelf.Rows {
			if row.When == "" {
				t.Errorf("row %d (%q) in %v says nothing in the date column", i, row.Text, lang)
			}
		}
	}
}

// TestASharedReportTitleIsToldApartWhereItStandsAlone pins where a qualifier
// is said. A row already shows the day, so a day that does the telling is not
// repeated in its title; a file name is, because the day could not tell the two
// apart. The rail and the sidebar show a title with no day beside it, so the
// label always carries the qualifier.
func TestASharedReportTitleIsToldApartWhereItStandsAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		report    nav.Report
		wantRow   string
		wantLabel string
	}{
		{
			name:      "a title nobody else carries",
			report:    nav.Report{Title: "Weekly review", Date: "2026-09-22"},
			wantRow:   "Weekly review",
			wantLabel: "Weekly review",
		},
		{
			name:      "a day that tells it apart is not said twice in a row",
			report:    nav.Report{Title: "Weekly review", Date: "2026-09-22", Qualifier: "2026-09-22"},
			wantRow:   "Weekly review",
			wantLabel: "Weekly review · 2026-09-22",
		},
		{
			name:      "a file name is said wherever the day could not tell it apart",
			report:    nav.Report{Title: "Weekly review", Date: "2026-09-22", Qualifier: "2026-09-22 pm.md"},
			wantRow:   "Weekly review · 2026-09-22 pm.md",
			wantLabel: "Weekly review · 2026-09-22 pm.md",
		},
		{
			name:      "a report with no day is told apart by its file name",
			report:    nav.Report{Title: "Weekly review", Qualifier: "alpha.md"},
			wantRow:   "Weekly review · alpha.md",
			wantLabel: "Weekly review · alpha.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := reportRowTitle(&tt.report); got != tt.wantRow {
				t.Errorf("reportRowTitle() = %q, want %q", got, tt.wantRow)
			}
			if got := reportLabel(&tt.report); got != tt.wantLabel {
				t.Errorf("reportLabel() = %q, want %q", got, tt.wantLabel)
			}
		})
	}
}

// TestTheFolderIndexCountsEveryFileOnTheShelf keeps the count's figure
// counting every markdown note under a shelf folder and at the vault root.
func TestTheFolderIndexCountsEveryFileOnTheShelf(t *testing.T) {
	t.Parallel()

	// Counted by hand from the fixture's own file list, so this asks whether
	// the figure is right rather than whether the code agrees with itself: two
	// maps, two lessons, two concepts and two sources in the declared knowledge
	// folders, plus the one file at the vault root. Notes in A, B, System and
	// Diary stay outside this shelf's measure.
	const files = 9

	view := NewFolderIndex(buildModel(t), ContractGoverning, wording.ZhHant, nil)
	if view.Count != "9 篇" {
		t.Errorf("folder index count = %q, want it to name all %d files on the shelf", view.Count, files)
	}
}

// TestFolderIndexLabelsRootNotesAsTheirOwnGroup holds the half of the root
// symptom the tree narrowing does not touch: a markdown file at the vault
// root stays on the shelf, and it is labelled so it is not read as another
// folder. Both languages carry the same cut.
func TestFolderIndexLabelsRootNotesAsTheirOwnGroup(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	for _, tt := range []struct {
		lang wording.Lang
		want string
	}{
		{wording.ZhHant, "最上層的筆記"},
		{wording.En, "Top-level notes"},
	} {
		view := NewFolderIndex(model, ContractGoverning, tt.lang, nil)
		at := -1
		for i, row := range view.Shelf.Rows {
			if row.Heading && row.Text == tt.want {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatalf("folder index (%s) has no %q group; rows = %+v", tt.lang, tt.want, view.Shelf.Rows)
		}
		if at+1 >= len(view.Shelf.Rows) || view.Shelf.Rows[at+1].Text != "Reading list" || view.Shelf.Rows[at+1].Heading {
			t.Errorf("folder index (%s) does not list the root note under the label; rows = %+v", tt.lang, view.Shelf.Rows)
		}
	}
}

// TestFolderLevelLabelsOtherFilesAndLeavesThemUncounted holds the folder page
// to the shelf rule: 篇 counts notes; the files the desk still serves sit in
// their own labelled group.
func TestFolderLevelLabelsOtherFilesAndLeavesThemUncounted(t *testing.T) {
	t.Parallel()

	files := []nav.NoteRef{
		{Name: "note", RelPath: "Attachments/note.md"},
		{Name: "scan.pdf", RelPath: "Attachments/scan.pdf"},
	}
	for _, tt := range []struct {
		lang  wording.Lang
		other string
	}{{wording.ZhHant, "其他檔案"}, {wording.En, "Other files"}} {
		view := NewFolderLevel(&FolderInput{Dir: "Attachments", Name: "Attachments", Notes: files, Model: &nav.Model{}, Now: time.Now(), Lang: tt.lang})
		if tt.lang == wording.ZhHant && view.List.Count != "1 篇" {
			t.Errorf("folder level count = %q, want notes only", view.List.Count)
		}
		last := view.List.Groups[len(view.List.Groups)-1]
		if last.Heading != tt.other || len(last.Rows) != 1 || last.Rows[0].Title != "scan.pdf" {
			t.Errorf("folder level (%s) other-files group = %+v, want %q holding scan.pdf", tt.lang, last, tt.other)
		}
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
		{courseMode, ListIndex(NewPathIndex(model.Paths(), schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, wording.ZhHant, nil), layouts.Chrome{})},
		{mapMode, ListIndex(NewMapIndex(model.Maps(), schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, wording.ZhHant, nil), layouts.Chrome{})},
		{reportMode, ListIndex(NewReportIndex(model.Reports(), wording.ZhHant, nil), layouts.Chrome{})},
		{folderMode, FolderIndex(NewFolderIndexView(model, ContractGoverning, ListQuery{}, time.Now(), wording.ZhHant), layouts.Chrome{})},
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
