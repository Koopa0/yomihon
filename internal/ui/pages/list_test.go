package pages

import (
	"fmt"
	"net/url"
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
	if view.List.Count != "1 個資料夾 · 3 篇" {
		t.Errorf("count = %q, want the folders and the notes at every depth", view.List.Count)
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

func numbered(names ...string) []ListNote {
	notes := make([]ListNote, len(names))
	for i, name := range names {
		title := strings.TrimSuffix(name, ".md")
		notes[i] = ListNote{Title: title, RelPath: "course/" + name}
	}
	return notes
}

func TestAFolderNumberedByItsAuthorIsReadInTheirOrder(t *testing.T) {
	t.Parallel()

	notes := numbered("10-ten.md", "2-two.md", "zeta.md", "02-again.md", "alpha.md", "1_one.md")
	if !hasAuthorOrder(notes) {
		t.Fatal("four of six file names open with a number: the folder has its author's order")
	}
	got := titles(NoteRows(notes, SortByOrder, listNow, wording.ZhHant)[0].Rows)
	want := []string{"1_one", "02-again", "2-two", "10-ten", "alpha", "zeta"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("order = %v, want value first, then the file name, unnumbered last by title: %v", got, want)
	}
	for i, title := range []string{"Foxtrot", "Bravo", "Alpha", "Delta", "Echo", "Charlie"} {
		notes[i].Title = title
	}
	byTitle := titles(NoteRows(notes, SortByName, listNow, wording.ZhHant)[0].Rows)
	if strings.Join(byTitle, "|") != "Alpha|Bravo|Charlie|Delta|Echo|Foxtrot" {
		t.Errorf("by title = %v, want the titles' own order with no regard for file numbers", byTitle)
	}
}

func TestAnAuthorsOrderNeedsThreeNotesAndMoreThanHalfNumbered(t *testing.T) {
	t.Parallel()

	for name, files := range map[string][]string{
		"two notes":    {"1-a.md", "2-b.md"},
		"exactly half": {"1-a.md", "2-b.md", "c.md", "d.md"},
		"no separator": {"1a.md", "2b.md", "3c.md"},
	} {
		if hasAuthorOrder(numbered(files...)) {
			t.Errorf("%s: want no author's order", name)
		}
	}
	if !hasAuthorOrder(numbered("1 a.md", "2.b.md", "3_c.md")) {
		t.Error("a space, a dot and an underscore each follow a number")
	}
}

func TestANumberedFolderOpensInOrderAndAReadersChoiceIsExactlyThat(t *testing.T) {
	t.Parallel()

	in := func(query string) FolderView {
		v, err := url.ParseQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		return NewFolderLevel(&FolderInput{
			Dir: "course", Name: "course", VaultName: "v",
			Notes: []nav.NoteRef{
				{Name: "Second", RelPath: "course/02-b.md"}, {Name: "First", RelPath: "course/01-a.md"},
				{Name: "Third", RelPath: "course/03-c.md"},
			},
			Model: folderModel(t), Query: ParseListQuery(v), Now: listNow, Lang: wording.ZhHant,
		})
	}
	opened := in("")
	choices := opened.List.Switch.Choices
	if len(choices) != 3 || choices[0].Text != "依順序" || !choices[0].Pressed || choices[1].Text != "依標題" {
		t.Fatalf("choices = %+v, want 依順序｜依標題｜依最近變更 with the first pressed", choices)
	}
	if got := titles(opened.List.Groups[0].Rows); strings.Join(got, "|") != "First|Second|Third" {
		t.Errorf("rows = %v, want the author's order", got)
	}
	if choices[0].Href != "/folders/course" || !strings.Contains(choices[1].Href, "sort=name") {
		t.Errorf("hrefs = %q, %q: the default leaves the sort out and a choice is always written", choices[0].Href, choices[1].Href)
	}
	chosen := in("sort=name")
	if got := titles(chosen.List.Groups[0].Rows); strings.Join(got, "|") != "First|Second|Third" {
		t.Errorf("rows = %v", got)
	}
	if chosen.List.Switch.Choices[1].Pressed != true || chosen.List.Switch.Choices[0].Pressed {
		t.Errorf("a reader who pressed 依標題 has it pressed: %+v", chosen.List.Switch.Choices)
	}
}

func TestALoneGroupsHeadingRisesIntoTheBarWhichCarriesTheRule(t *testing.T) {
	t.Parallel()

	var notes []nav.NoteRef
	for i := range 25 {
		notes = append(notes, nav.NoteRef{Name: fmt.Sprintf("設計題：題%02d", i), RelPath: fmt.Sprintf("d/t%02d.md", i)})
	}
	v := NewFolderLevel(&FolderInput{Dir: "d", Name: "d", VaultName: "v", Notes: notes, Model: folderModel(t), Now: listNow, Lang: wording.ZhHant})
	l := v.List
	if l.Label != "設計題" || l.LabelCount != "25" || !l.Ruled || !l.LabelAuthored {
		t.Errorf("label %q %q ruled=%v: want the group's name and count at the bar's head", l.Label, l.LabelCount, l.Ruled)
	}
	if l.Count != "" || len(l.Groups) != 1 || l.Groups[0].Heading != "" {
		t.Errorf("count %q, groups %+v: the head is said once", l.Count, l.Groups)
	}
	if len(l.Switch.Choices) == 0 {
		t.Error("the order is chosen from the bar when there is no count line")
	}
}

func TestTheFoldersGroupCarriesTheWayToOpenThem(t *testing.T) {
	t.Parallel()

	v := NewFolderLevel(&FolderInput{
		Dir: "a", Name: "a", VaultName: "v",
		Notes:      []nav.NoteRef{{Name: "one", RelPath: "a/one.md"}},
		Subfolders: []nav.Folder{{Name: "b", RelPath: "a/b", Notes: []nav.NoteRef{{Name: "two", RelPath: "a/b/two.md"}}}},
		Model:      folderModel(t), Now: listNow, Lang: wording.ZhHant,
	})
	if v.List.Toggle.Text != "" {
		t.Errorf("toggle = %+v: it belongs to the folders' own head", v.List.Toggle)
	}
	if g := v.List.Groups[0]; g.Heading != "資料夾" || g.Action.Text != "展開資料夾" || !strings.Contains(g.Action.Href, "expand=1") {
		t.Errorf("group = %+v, want the action on its head", g)
	}
}

func TestAGroupHeadThatIsALinkIsOneTargetWithItsCount(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	err := ListBody(List{Groups: []ListGroup{{Heading: "Concepts", Href: "/folders/Concepts", Count: "2 篇"}}}, wording.ZhHant).Render(t.Context(), &out)
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	open, shut := strings.Index(html, `<a class="y-lgroup__link"`), strings.Index(html, "</a>")
	if open < 0 || shut < open || !strings.Contains(html[open:shut], "2 篇") {
		t.Errorf("the count is outside the link: %s", html)
	}
}

func TestTheFolderIndexLeavesOutTheVaultsReadme(t *testing.T) {
	t.Parallel()

	got := withoutReadme([]nav.NoteRef{{Name: "技術自學", RelPath: "README.md"}, {Name: "Agents", RelPath: "AGENTS.md"}, {Name: "Nested", RelPath: "a/README.md"}})
	if len(got) != 2 || got[0].RelPath != "AGENTS.md" || got[1].RelPath != "a/README.md" {
		t.Errorf("notes = %+v, want only the top-level README gone", got)
	}
}

// Ago says how long ago something was in the words the owner approved: minutes
// within the hour, hours within the same day, yesterday for the calendar day
// before, and otherwise the day itself as the lists write one. The day is the
// reader's: the calendar of the zone now is given in, so a moment eight hours
// east of UTC is yesterday there while it is still today in UTC, and the other
// way round.
func TestAgoCountsMinutesHoursThenTheReadersCalendar(t *testing.T) {
	t.Parallel()

	taipei := time.FixedZone("UTC+8", 8*60*60)
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation(America/New_York): %v", err)
	}
	for _, tt := range []struct {
		name    string
		at, now time.Time
		zh, en  string
	}{
		{
			name: "seconds ago is one minute",
			at:   time.Date(2026, 11, 26, 9, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 9, 0, 20, 0, taipei),
			zh: "1 分鐘前", en: "1 min ago",
		},
		{
			name: "the same moment is one minute",
			at:   time.Date(2026, 11, 26, 9, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 9, 0, 0, 0, taipei),
			zh: "1 分鐘前", en: "1 min ago",
		},
		{
			name: "a clock a few seconds behind is one minute",
			at:   time.Date(2026, 11, 26, 9, 0, 30, 0, taipei), now: time.Date(2026, 11, 26, 9, 0, 0, 0, taipei),
			zh: "1 分鐘前", en: "1 min ago",
		},
		{
			name: "minutes are counted down",
			at:   time.Date(2026, 11, 26, 9, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 9, 14, 59, 0, taipei),
			zh: "14 分鐘前", en: "14 min ago",
		},
		{
			name: "the last minute of the hour",
			at:   time.Date(2026, 11, 26, 9, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 9, 59, 59, 0, taipei),
			zh: "59 分鐘前", en: "59 min ago",
		},
		{
			name: "an hour is an hour",
			at:   time.Date(2026, 11, 26, 9, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 10, 0, 0, 0, taipei),
			zh: "1 小時前", en: "1 h ago",
		},
		{
			name: "hours within the same day",
			at:   time.Date(2026, 11, 26, 0, 30, 0, 0, taipei), now: time.Date(2026, 11, 26, 23, 0, 0, 0, taipei),
			zh: "22 小時前", en: "22 h ago",
		},
		{
			name: "under an hour across midnight is still minutes",
			at:   time.Date(2026, 11, 25, 23, 50, 0, 0, taipei), now: time.Date(2026, 11, 26, 0, 10, 0, 0, taipei),
			zh: "20 分鐘前", en: "20 min ago",
		},
		{
			name: "three hours across midnight is yesterday, not three hours",
			at:   time.Date(2026, 11, 25, 22, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 1, 0, 0, 0, taipei),
			zh: "昨天", en: "Yesterday",
		},
		{
			// 15:30 and 17:00 UTC on the 25th are one day in UTC; east of it they
			// are 23:30 on the 25th and 01:00 on the 26th.
			name: "a time stored in UTC is read on the reader's calendar",
			at:   time.Date(2026, 11, 25, 15, 30, 0, 0, time.UTC), now: time.Date(2026, 11, 26, 1, 0, 0, 0, taipei),
			zh: "昨天", en: "Yesterday",
		},
		{
			// 16:30 UTC on the 25th is 00:30 on the 26th east of it: the same day
			// as 23:00 on the 26th, though UTC calls it the day before.
			name: "the reader's today can be UTC's yesterday",
			at:   time.Date(2026, 11, 25, 16, 30, 0, 0, time.UTC), now: time.Date(2026, 11, 26, 23, 0, 0, 0, taipei),
			zh: "22 小時前", en: "22 h ago",
		},
		{
			name: "the whole of yesterday",
			at:   time.Date(2026, 11, 25, 0, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 23, 59, 0, 0, taipei),
			zh: "昨天", en: "Yesterday",
		},
		{
			name: "two calendar days back is the day, though under forty-eight hours",
			at:   time.Date(2026, 11, 24, 23, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 1, 0, 0, 0, taipei),
			zh: "11-24", en: "11-24",
		},
		{
			name: "another year writes the year",
			at:   time.Date(2025, 12, 31, 12, 0, 0, 0, taipei), now: time.Date(2026, 1, 2, 12, 0, 0, 0, taipei),
			zh: "2025-12-31", en: "2025-12-31",
		},
		{
			name: "yesterday across a new year",
			at:   time.Date(2025, 12, 31, 12, 0, 0, 0, taipei), now: time.Date(2026, 1, 1, 12, 0, 0, 0, taipei),
			zh: "昨天", en: "Yesterday",
		},
		{
			// The night the clocks go forward has twenty-three hours, so the
			// calendar and a count of hours disagree.
			name: "yesterday over a short night is still yesterday",
			at:   time.Date(2026, 3, 7, 23, 30, 0, 0, newYork), now: time.Date(2026, 3, 8, 22, 0, 0, 0, newYork),
			zh: "昨天", en: "Yesterday",
		},
		{
			name: "a time well ahead of the clock is its day",
			at:   time.Date(2026, 11, 28, 9, 0, 0, 0, taipei), now: time.Date(2026, 11, 26, 9, 0, 0, 0, taipei),
			zh: "11-28", en: "11-28",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Ago(tt.at, tt.now, wording.ZhHant); got != tt.zh {
				t.Errorf("Ago(%v, %v, zh) = %q, want %q", tt.at, tt.now, got, tt.zh)
			}
			if got := Ago(tt.at, tt.now, wording.En); got != tt.en {
				t.Errorf("Ago(%v, %v, en) = %q, want %q", tt.at, tt.now, got, tt.en)
			}
		})
	}
}

// A bookmark with no time — one kept before bookmarks carried one — says
// nothing about its age rather than a day in the year one.
func TestAgoOfNoTimeIsNothing(t *testing.T) {
	t.Parallel()
	if got := Ago(time.Time{}, listNow, wording.ZhHant); got != "" {
		t.Errorf("Ago(zero, now, zh) = %q, want empty", got)
	}
}
