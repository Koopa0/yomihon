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
	}, nav.Closure{}, true, wording.ZhHant, nil)

	want := []Row{
		{Text: "Go path", Href: "/syllabus/Maps/Go%20path.md", Mark: "4 課"},
		{Text: "Unread structure", Href: "/syllabus/Maps/Broken.md", Mark: "0 課 · 未讀到課程結構", Fault: true},
		{Text: "Plans nothing", Href: "/syllabus/Maps/Empty.md", Mark: "0 課"},
	}
	if diff := cmp.Diff(want, view.Shelf.Rows); diff != "" {
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
	view := NewMapIndex([]nav.Map{deep}, nav.Closure{}, true, wording.ZhHant, nil)
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
		{Heading: "Authors", Entries: []nav.MapEntry{{Text: "Norwegian Wood"}, {Text: "Kafka on the Shore"}}},
		{Heading: "Forms", Entries: []nav.MapEntry{{Text: "The Wind-Up Bird Chronicle"}}},
		{Heading: "Editions", Entries: []nav.MapEntry{{Text: "《挪威的森林》"}, {Text: "海辺のカフカ"}}},
		{Heading: "Places — [[Sputnik Sweetheart]]", Entries: []nav.MapEntry{{Text: "Sputnik Sweetheart"}}},
		{Heading: "After", Entries: []nav.MapEntry{{Text: "Colorless Tsukuru Tazaki"}}},
	}}
	view := NewMapIndex([]nav.Map{prose}, nav.Closure{}, true, wording.ZhHant, nil)
	want := []Row{{Text: "A map written as prose", Href: "/notes/Maps/Prose%20map.md", Mark: "5 枝"}}
	if diff := cmp.Diff(want, view.Shelf.Rows); diff != "" {
		t.Errorf("prose map shelf mark mismatch (-want +got):\n%s", diff)
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
		{Name: "Vault audit", RelPath: "System/reports/2026-07-10 vault audit.md"},
		{Name: "notes", RelPath: "System/reports/notes.md"},
		{Name: "latest.html", RelPath: "System/reports/daily-briefing/latest.html", Briefing: true, Latest: true},
	}, wording.ZhHant, nil)

	want := []Row{
		{
			Text: "Vault audit",
			Href: "/notes/System/reports/2026-07-10%20vault%20audit.md",
			Mark: "2026-07-10 · 書庫筆記",
		},
		{Text: "notes", Href: "/notes/System/reports/notes.md", Mark: "書庫筆記"},
		{Text: "latest.html", Href: "/reports/latest.html", Mark: "每日簡報 · 最新"},
	}
	if diff := cmp.Diff(want, view.Shelf.Rows); diff != "" {
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

// TestTheFolderIndexCountsEveryFileOnTheShelf keeps the kicker's figure
// counting every markdown note under a shelf folder and at the vault root.
func TestTheFolderIndexCountsEveryFileOnTheShelf(t *testing.T) {
	t.Parallel()

	// Counted by hand from the fixture's own file list, so this asks whether
	// the figure is right rather than whether the code agrees with itself: two
	// maps, two lessons, two concepts and two sources in the declared knowledge
	// folders, plus the one file at the vault root. Notes in A, B, System and
	// Diary stay outside this shelf's measure.
	const files = 9

	view := NewFolderIndex(buildModel(t), true, wording.ZhHant, nil)
	if view.Kicker != "9 篇" {
		t.Errorf("folder index kicker = %q, want it to name all %d files on the shelf", view.Kicker, files)
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
		{wording.ZhHant, "根目錄筆記"},
		{wording.En, "Root notes"},
	} {
		view := NewFolderIndex(model, true, tt.lang, nil)
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
	zh := NewFolderLevel("Attachments", "Attachments", files, nil, wording.ZhHant, nil)
	if zh.Count != "1 篇" {
		t.Errorf("folder level count = %q, want notes only", zh.Count)
	}
	wantZH := []Row{
		{Text: "note", Href: "/notes/Attachments/note.md"},
		{Text: "其他檔案", Heading: true},
		{Text: "scan.pdf", Href: "/notes/Attachments/scan.pdf"},
	}
	if diff := cmp.Diff(wantZH, zh.Shelf.Rows); diff != "" {
		t.Errorf("folder level rows (zh) mismatch (-want +got):\n%s", diff)
	}
	en := NewFolderLevel("Attachments", "Attachments", files, nil, wording.En, nil)
	if en.Shelf.Rows[1].Text != "Other files" || !en.Shelf.Rows[1].Heading {
		t.Errorf("folder level (%s) other-files label = %+v, want Other files", wording.En, en.Shelf.Rows)
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
		{pathMode, ListIndex(NewPathIndex(model.Paths(), nav.Closure{}, true, wording.ZhHant, nil), layouts.Chrome{})},
		{mapMode, ListIndex(NewMapIndex(model.Maps(), nav.Closure{}, true, wording.ZhHant, nil), layouts.Chrome{})},
		{reportMode, ListIndex(NewReportIndex(model.Reports(), wording.ZhHant, nil), layouts.Chrome{})},
		{folderMode, FolderIndex(NewFolderIndex(model, true, wording.ZhHant, nil), RecentBlock{}, StatusDistribution{}, layouts.Chrome{})},
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

// TestStatusDistributionNamesItsReachBesideANarrowedShelf holds the sentence
// under the distribution to the shelf beside it: a shelf narrowed to the
// declared layer sits above a count that still reaches every indexed note, so
// the sentence says so, and an unscoped shelf carries no sentence — the
// heading already says what the block is.
func TestStatusDistributionNamesItsReachBesideANarrowedShelf(t *testing.T) {
	t.Parallel()
	items := []LifecycleItem{{Name: "draft", Count: 2}}
	for _, tt := range []struct {
		name   string
		scoped bool
		lang   wording.Lang
		want   string
	}{
		{name: "scoped zh", scoped: true, lang: wording.ZhHant, want: "書庫中每篇已索引筆記落在哪裡，含書架之外的資料夾"},
		{name: "scoped en", scoped: true, lang: wording.En, want: "Where each indexed note in the vault sits, including folders off the shelf"},
		{name: "unscoped zh", scoped: false, lang: wording.ZhHant, want: ""},
		{name: "unscoped en", scoped: false, lang: wording.En, want: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewStatusDistribution(items, nil, tt.scoped, tt.lang)
			if got.Lede != tt.want {
				t.Errorf("lede = %q, want %q", got.Lede, tt.want)
			}
			if len(got.Statuses) != 1 {
				t.Errorf("statuses were not carried through: %v", got.Statuses)
			}
		})
	}
}

// TestRecentBlockNamesItsReachBesideANarrowedShelf holds the sentence under
// the recent list to the shelf beside it: a shelf narrowed to the declared
// layer names that set, and an unscoped ordered shelf carries no sentence —
// the heading already says what the list is.
func TestRecentBlockNamesItsReachBesideANarrowedShelf(t *testing.T) {
	t.Parallel()
	notes := []HomeNote{{Title: "n"}}
	for _, tt := range []struct {
		name    string
		ordered bool
		scoped  bool
		lang    wording.Lang
		want    string
	}{
		{name: "ordered scoped zh", ordered: true, scoped: true, lang: wording.ZhHant, want: "知識層資料夾中最近改動過的筆記"},
		{name: "ordered scoped en", ordered: true, scoped: true, lang: wording.En, want: "Notes in the declared knowledge folders changed most recently"},
		{name: "ordered unscoped zh", ordered: true, scoped: false, lang: wording.ZhHant, want: ""},
		{name: "ordered unscoped en", ordered: true, scoped: false, lang: wording.En, want: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewRecentBlock(notes, tt.ordered, tt.scoped, tt.lang)
			if got.Lede != tt.want {
				t.Errorf("lede = %q, want %q", got.Lede, tt.want)
			}
		})
	}
}
