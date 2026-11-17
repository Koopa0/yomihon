package pages

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

var listNow = time.Date(2026, time.November, 18, 12, 0, 0, 0, time.UTC)

func titles(rows []ListRow) []string {
	out := make([]string, len(rows))
	for i := range rows {
		out[i] = rows[i].Title
	}
	return out
}

func TestNamesAreOrderedDigitsLatinThenHanByStroke(t *testing.T) {
	t.Parallel()

	notes := []ListNote{{Title: "面試"}, {Title: "go-spec"}, {Title: "17-code"}, {Title: "書單"}, {Title: "Ardan"}, {Title: "作品集"}}
	got := titles(NoteRows(notes, SortByName, listNow, wording.ZhHant)[0].Rows)
	want := []string{"17-code", "Ardan", "go-spec", "作品集", "面試", "書單"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestATitlePrefixSharedByThreeBecomesAHeading(t *testing.T) {
	t.Parallel()

	notes := []ListNote{
		{Title: "設計題：支付"}, {Title: "設計題：叫車"}, {Title: "設計題：地圖"},
		{Title: "別的：一個"}, {Title: "別的：兩個"}, {Title: "無前綴"},
	}
	groups := NoteRows(notes, SortByName, listNow, wording.ZhHant)
	if len(groups) != 2 || groups[0].Heading != "" || groups[1].Heading != "設計題" {
		t.Fatalf("groups = %+v, want the loose rows then one 設計題 group", groups)
	}
	if got := titles(groups[1].Rows); len(got) != 3 || strings.Contains(strings.Join(got, ""), "：") {
		t.Errorf("rows kept the prefix: %v", got)
	}
	if groups[1].Count != "3" || len(groups[0].Rows) != 3 {
		t.Errorf("a prefix shared by two stays in the rows: loose %d, count %q", len(groups[0].Rows), groups[1].Count)
	}
}

func TestADateThatTellsNothingApartIsNotDrawn(t *testing.T) {
	t.Parallel()

	day := func(d int) time.Time { return time.Date(2026, time.October, d, 9, 0, 0, 0, time.UTC) }
	var notes []ListNote
	for i := range 12 {
		notes = append(notes, ListNote{Title: fmt.Sprintf("n%02d", i), Modified: day(13 + i%2*2)})
	}
	if fact := NoteRows(notes, SortByName, listNow, wording.ZhHant)[0].Rows[0].Fact; fact != "" {
		t.Errorf("twelve rows on two days drew the date %q", fact)
	}
	if fact := NoteRows(notes, SortByRecent, listNow, wording.ZhHant)[0].Rows[0].Fact; fact != "10-15" {
		t.Errorf("read by date the date is the key, got %q", fact)
	}
	notes[0].Modified = day(14)
	if fact := NoteRows(notes, SortByName, listNow, wording.ZhHant)[0].Rows[0].Fact; fact == "" {
		t.Error("three different days should draw the date")
	}
	old := []ListNote{{Title: "a", Modified: time.Date(2025, 10, 13, 0, 0, 0, 0, time.UTC)}}
	if fact := NoteRows(old, SortByName, listNow, wording.ZhHant)[0].Rows[0].Fact; fact != "2025-10-13" {
		t.Errorf("a date from another year = %q, want the whole date", fact)
	}
}

func TestALongListReadByDateIsHeadedByMonth(t *testing.T) {
	t.Parallel()

	var notes []ListNote
	for i := range 25 {
		notes = append(notes, ListNote{Title: fmt.Sprintf("n%02d", i), Modified: time.Date(2026, time.Month(10+i%2), 1+i, 9, 0, 0, 0, time.UTC)})
	}
	groups := NoteRows(notes, SortByRecent, listNow, wording.ZhHant)
	if len(groups) != 2 || groups[0].Heading != "11 月" || groups[1].Heading != "10 月" {
		t.Errorf("month groups = %+v", groups)
	}
	if groups := NoteRows(notes[:20], SortByRecent, listNow, wording.ZhHant); len(groups) != 1 || groups[0].Heading != "" {
		t.Errorf("twenty rows are one glance, got %d groups", len(groups))
	}
}

func TestThousandsSeparatesAfter999(t *testing.T) {
	t.Parallel()

	for n, want := range map[int]string{7: "7", 999: "999", 1000: "1,000", 1044: "1,044", 1234567: "1,234,567"} {
		if got := Thousands(n); got != want {
			t.Errorf("Thousands(%d) = %q, want %q", n, got, want)
		}
	}
}

func folderModel(t *testing.T) *nav.Model {
	t.Helper()
	return &nav.Model{}
}

func TestAFolderPageSaysTheWholeSubtreeAndLeadsWithTheVault(t *testing.T) {
	t.Parallel()

	in := &FolderInput{
		Dir: "a", Name: "a", VaultName: "學習庫",
		Notes: []nav.NoteRef{{Name: "one", RelPath: "a/one.md"}},
		Subfolders: []nav.Folder{
			{Name: "b", RelPath: "a/b", Notes: []nav.NoteRef{{Name: "two", RelPath: "a/b/two.md"}, {Name: "three", RelPath: "a/b/three.md"}}},
		},
		Model: folderModel(t), Now: listNow, Lang: wording.ZhHant,
	}
	view := NewFolderLevel(in)
	if view.Count != "1 個資料夾 · 3 篇" {
		t.Errorf("count = %q, want the folders and the notes at every depth", view.Count)
	}
	if len(view.Crumbs) != 1 || view.Crumbs[0].Name != "學習庫" {
		t.Errorf("crumbs = %+v, want the vault first", view.Crumbs)
	}
	groups := view.List.Groups
	if len(groups) != 2 || groups[0].Heading != "資料夾" || groups[1].Heading != "筆記" {
		t.Fatalf("groups = %+v, want the folders then the notes", groups)
	}
	if got := groups[0].Rows[0].Fact; got != "2 篇" {
		t.Errorf("a folder's fact = %q, want how many notes sit under it", got)
	}
}

func TestExpandingAFolderGroupsEachSubfolderWithItsOwnNotes(t *testing.T) {
	t.Parallel()

	in := &FolderInput{
		Dir: "a", Name: "a",
		Notes: []nav.NoteRef{{Name: "one", RelPath: "a/one.md"}},
		Subfolders: []nav.Folder{
			{Name: "b", RelPath: "a/b", Notes: []nav.NoteRef{{Name: "two", RelPath: "a/b/two.md"}}, Subfolders: []nav.Folder{
				{Name: "deep", RelPath: "a/b/deep", Notes: []nav.NoteRef{{Name: "x", RelPath: "a/b/deep/x.md"}}},
			}},
		},
		Query: ListQuery{Expand: true},
		Model: folderModel(t), Now: listNow, Lang: wording.ZhHant,
	}
	groups := NewFolderLevel(in).List.Groups
	if len(groups) != 2 || groups[0].Heading != "最上層" || groups[1].Heading != "b" {
		t.Fatalf("groups = %+v", groups)
	}
	rows := groups[1].Rows
	if len(rows) != 2 || rows[0].Title != "two" || !rows[1].Folder || rows[1].Title != "deep" {
		t.Errorf("an expanded folder lists its notes, then the folders below as one row each: %+v", rows)
	}
}

func TestTheFilterFindsNotesAtEveryDepthGroupedBySubfolder(t *testing.T) {
	t.Parallel()

	in := &FolderInput{
		Dir: "a", Name: "a",
		Notes: []nav.NoteRef{{Name: "Cache basics", RelPath: "a/c.md"}, {Name: "other", RelPath: "a/o.md"}},
		Subfolders: []nav.Folder{
			{Name: "b", RelPath: "a/b", Notes: []nav.NoteRef{{Name: "distributed CACHE", RelPath: "a/b/d.md"}}},
		},
		Query: ListQuery{Filter: "cache"},
		Model: folderModel(t), Now: listNow, Lang: wording.ZhHant,
	}
	groups := NewFolderLevel(in).List.Groups
	if len(groups) != 2 {
		t.Fatalf("groups = %+v, want the top level and b", groups)
	}
	row := groups[1].Rows[0]
	if groups[1].Heading != "b" || row.Before != "distributed " || row.Match != "CACHE" {
		t.Errorf("the found word should be split out, keeping the title's own case: %+v", row)
	}
	none := *in
	none.Query.Filter = "zzz"
	if view := NewFolderLevel(&none); len(view.List.Groups) != 0 || view.List.Cleared.Text == "" {
		t.Errorf("a filter that finds nothing should say so: %+v", view.List)
	}
}

func TestTheFolderIndexSkipsSystemDotFoldersAndEmptyOnes(t *testing.T) {
	t.Parallel()

	folders := []nav.Folder{
		{Name: "System", RelPath: "System", Notes: []nav.NoteRef{{RelPath: "System/a.md"}}},
		{Name: ".git", RelPath: ".git", Notes: []nav.NoteRef{{RelPath: ".git/a.md"}}},
		{Name: "empty", RelPath: "empty", Notes: []nav.NoteRef{{RelPath: "empty/pic.png"}}},
		{Name: "real", RelPath: "real", Subfolders: []nav.Folder{
			{Name: "kept", RelPath: "real/kept", Notes: []nav.NoteRef{{RelPath: "real/kept/a.md"}}},
			{Name: "__pycache__", RelPath: "real/__pycache__", Notes: []nav.NoteRef{{RelPath: "real/__pycache__/a.pyc"}}},
		}},
	}
	got := listableFolders(folders)
	if len(got) != 1 || got[0].Name != "real" || len(got[0].Subfolders) != 1 {
		t.Errorf("listable folders = %+v, want only real with its kept folder", got)
	}
	if n := countFolders(got); n != 2 {
		t.Errorf("countFolders = %d, want 2", n)
	}
}
