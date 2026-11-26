package pages

import (
	"bytes"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func renderHome(t *testing.T, v *HomeView, lang wording.Lang) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Home(*v, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render Home: %v", err)
	}
	// The header carries the product's one search; what the page itself draws
	// is what is asked about, so the header is cut off.
	_, page, ok := strings.Cut(buf.String(), `id="_y-main"`)
	if !ok {
		t.Fatalf("Home() draws no main region")
	}
	return page
}

func keptView() *HomeView {
	return &HomeView{Continue: ContinueRow{
		Show: true, Title: "Alpha", Href: "/notes/a.md?at=40#cancel", Kicker: "A course · Part one",
		Section: RowPlace{Text: "Cancel"}, Age: "14 分鐘前", AgeAt: "2026-11-26T09:00:00+08:00",
	}}
}

// TestTheDeskDrawsNoSearchField holds that the page itself asks for no search:
// the header carries the one search the product has, and a second field on the
// desk asked for as much of the screen as any of the ways in.
func TestTheDeskDrawsNoSearchField(t *testing.T) {
	t.Parallel()

	for name, v := range map[string]*HomeView{"fresh": {}, "kept": keptView()} {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			page := renderHome(t, v, lang)
			for _, field := range []string{"<search", `type="search"`, `action="/search"`, "<form", "<input", "y-homesearch"} {
				if strings.Contains(page, field) {
					t.Errorf("the %s desk in %s draws %q, want no search field of its own", name, lang, field)
				}
			}
		}
	}
}

// TestTheDeskNamesItselfAsALabelOrNotAtAll holds that the desk's own name is
// never the loudest thing on it: a label where nothing stands first, and a
// heading kept for the listener alone where the reader's place does.
func TestTheDeskNamesItselfAsALabelOrNotAtAll(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		name := wording.HomeTitle.In(lang)
		if page := renderHome(t, &HomeView{}, lang); !strings.Contains(page, `<h1 class="y-title y-desklabel">`+name+`</h1>`) {
			t.Errorf("%s desk with no place kept does not name itself as a label: %q", lang, page)
		}
		page := renderHome(t, keptView(), lang)
		if !strings.Contains(page, `<h1 class="y-offscreen y-offscreen--block">`+name+`</h1>`) {
			t.Errorf("%s desk with a place kept does not leave its name to the listener: %q", lang, page)
		}
		if strings.Contains(page, "y-desklabel") {
			t.Errorf("%s desk with a place kept still draws its name as a label: %q", lang, page)
		}
	}
}

// TestTheKeptPlaceIsTheLargestTextOnTheDesk holds the shape of the place a
// reader returns to: that it is the bookmark, where it sits and how long ago it
// was added, its name, the heading the reader stopped at, and one way back that
// is not the name — the desk offers exactly one.
func TestTheKeptPlaceIsTheLargestTextOnTheDesk(t *testing.T) {
	t.Parallel()

	page := renderHome(t, keptView(), wording.ZhHant)
	section := between(t, page, `<section class="y-continue"`, "</section>")
	order := []string{
		`<p class="y-continue__tag">書籤</p>`,
		`class="y-continue__kicker"`, "A course · Part one",
		`<p class="y-continue__age"><time datetime="2026-11-26T09:00:00+08:00">14 分鐘前</time></p>`,
		`<h2 class="y-continue__title" id="home-continue-title">Alpha</h2>`,
		`<p class="y-continue__section">停在 <span class="y-continue__place">Cancel</span></p>`,
		`class="y-continue__note" href="/notes/a.md?at=40#cancel"`,
	}
	at := 0
	for _, want := range order {
		i := strings.Index(section[at:], want)
		if i < 0 {
			t.Fatalf("the kept place lacks %q after offset %d, in order: %s", want, at, section)
		}
		at += i
	}
	if got := strings.Count(section, "data-continue-link"); got != 1 {
		t.Errorf("the kept place offers %d ways back, want exactly 1: %s", got, section)
	}
	if strings.Contains(section, `<h2 class="y-continue__title" id="home-continue-title"><a`) {
		t.Errorf("the name of the place is itself a link, which would make two ways back: %s", section)
	}
	if !strings.Contains(section, wording.ContinueReading.In(wording.ZhHant)) {
		t.Errorf("the way back does not say what it does: %s", section)
	}
}

// TestTheCoursesLeadTheDeskOfAReaderWhoKeptNoPlace holds that the largest text
// on a desk with no place is the courses, in a block of their own above the
// rest, and that a desk with a place kept does not set them above it.
func TestTheCoursesLeadTheDeskOfAReaderWhoKeptNoPlace(t *testing.T) {
	t.Parallel()

	courses := DeskBlock{Mode: courseMode, Shelf: Shelf{Title: "Courses", Href: "/courses", Rows: []Row{{Text: "Go", Href: "/courses/go.md"}}}}
	maps := DeskBlock{Mode: mapMode, Shelf: Shelf{Title: "Maps", Href: "/indexes", Rows: []Row{{Text: "M", Href: "/notes/m.md"}}}}

	fresh := renderHome(t, &HomeView{Blocks: []DeskBlock{courses, maps}}, wording.En)
	lead := between(t, fresh, `<div class="y-homelead">`, "</div>")
	if !strings.Contains(lead, `data-home-block="courses"`) || !strings.Contains(lead, "y-modeblock--lead") {
		t.Errorf("a desk with no place kept does not open on the courses: %s", lead)
	}
	if strings.Count(fresh, `data-home-block="courses"`) != 1 {
		t.Errorf("the courses are drawn more than once: %s", fresh)
	}
	_, grid, ok := strings.Cut(fresh, `class="y-homegrid"`)
	if !ok {
		t.Fatalf("a desk with courses and maps draws no grid: %s", fresh)
	}
	if strings.Contains(grid, "y-modeblock--lead") {
		t.Errorf("a block inside the grid is drawn as the lead: %s", fresh)
	}

	kept := keptView()
	kept.Blocks = []DeskBlock{courses, maps}
	if page := renderHome(t, kept, wording.En); strings.Contains(page, "y-homelead") || strings.Contains(page, "y-modeblock--lead") {
		t.Errorf("a desk with a place kept sets the courses above it: %s", page)
	}

	empty := courses
	empty.Shelf.Rows = nil
	if page := renderHome(t, &HomeView{Blocks: []DeskBlock{empty, maps}}, wording.En); strings.Contains(page, "y-homelead") {
		t.Errorf("a vault with no course to open leads with one: %s", page)
	}
}

// TestTheDeskLeadsToTheHealthViewAndCountsNothing holds the desk's last line: a
// way to the whole-folder view on every desk, by the page's name, and no figure
// beside it. A desk that said how many things are wrong turned a way in into a
// debt, and the page it leads to is where the number is read and acted on.
func TestTheDeskLeadsToTheHealthViewAndCountsNothing(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		page := renderHome(t, &HomeView{}, lang)
		_, rest, ok := strings.Cut(page, `class="y-deskhealth"`)
		if !ok {
			t.Fatalf("%s desk has no way to the health view", lang)
		}
		row, after, ok := strings.Cut(rest, "</p>")
		if !ok {
			t.Fatalf("%s desk row is not a closed paragraph", lang)
		}
		if !strings.Contains(row, `href="/health"`) || !strings.Contains(row, wording.HealthTitle.In(lang)) {
			t.Errorf("%s desk row does not lead to the health view by its name: %q", lang, row)
		}
		if strings.ContainsAny(row, "0123456789") || strings.Contains(row, "<span") || strings.Contains(row, "findings") || strings.Contains(row, "項發現") {
			t.Errorf("%s desk row states a count: %q", lang, row)
		}
		if strings.Contains(after, "<p") {
			t.Errorf("%s desk draws something after the way to the health view, which is meant to be its last line", lang)
		}
	}
}

// TestTheDeskSaysWhichFolderItIsOfJustAboveItsLastLine holds the line that
// names the folder and where it is. It sits before the way to the health view,
// which stays the desk's last line, and a desk with neither fact draws none.
func TestTheDeskSaysWhichFolderItIsOfJustAboveItsLastLine(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		page := renderHome(t, &HomeView{VaultName: "Notes <&>", VaultPath: "/home/reader/notes"}, lang)
		line, _, ok := strings.Cut(page, `class="y-deskhealth"`)
		if !ok {
			t.Fatalf("%s desk has no way to the health view", lang)
		}
		_, line, ok = strings.Cut(line, `data-home-vault`)
		if !ok {
			t.Fatalf("%s desk does not say which folder it is of, or says it after its last line", lang)
		}
		for _, want := range []string{"<span>Notes &lt;&amp;&gt;</span>", "<code>/home/reader/notes</code>"} {
			if !strings.Contains(line, want) {
				t.Errorf("%s desk line does not carry %q: %q", lang, want, line)
			}
		}
		if page := renderHome(t, &HomeView{}, lang); strings.Contains(page, "data-home-vault") {
			t.Errorf("%s desk with no name and no path draws the line anyway", lang)
		}
	}
}

// TestTheDeskDoesNotCountAMapsBranches holds that a map is listed on the desk
// by its name alone, while a course keeps the lesson count that is its extent.
func TestTheDeskDoesNotCountAMapsBranches(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	if len(model.Maps()) == 0 || len(model.Paths()) == 0 {
		t.Fatalf("the fixture has %d maps and %d courses, so the lock reads nothing", len(model.Maps()), len(model.Paths()))
	}
	lang := wording.ZhHant
	for _, block := range NewDeskBlocks(model, schema.NavigationRoles{}, ContractGoverning, lang, nil) {
		for _, row := range shelfRows(&block.Shelf, deskBlockItems) {
			switch block.Mode {
			case mapMode:
				if row.Mark != "" {
					t.Errorf("the desk counts the branches of map %q: %q", row.Text, row.Mark)
				}
			case courseMode:
				if row.Mark == "" {
					t.Errorf("the desk lost the lesson count of course %q", row.Text)
				}
			}
		}
	}
}

func between(t *testing.T, s, open, closing string) string {
	t.Helper()
	start := strings.Index(s, open)
	if start < 0 {
		t.Fatalf("%q is not in the page: %s", open, s)
	}
	end := strings.Index(s[start:], closing)
	if end < 0 {
		t.Fatalf("%q is not closed by %q: %s", open, closing, s[start:])
	}
	return s[start : start+end]
}

// The plate says where the reader stopped in a sentence of the interface's own
// round the heading's words, which keep their author's language, in both of the
// interface's languages; a bookmark with no heading or no time says neither.
func TestTheBookmarkPlateSaysWhereAndWhenInEitherLanguage(t *testing.T) {
	t.Parallel()

	v := keptView()
	v.Continue.Section = RowPlace{Text: "Cancel", Language: "en"}
	v.Continue.Age = "14 min ago"
	page := renderHome(t, v, wording.En)
	for _, want := range []string{
		`<p class="y-continue__tag">Bookmark</p>`,
		`<time datetime="2026-11-26T09:00:00+08:00">14 min ago</time>`,
		`<p class="y-continue__section">Stopped at <span class="y-continue__place" lang="en">Cancel</span></p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the English plate lacks %q: %s", want, between(t, page, `<section class="y-continue"`, "</section>"))
		}
	}
	for _, retired := range []string{wording.BookmarkKeptHere.In(wording.En), wording.BookmarkKeptHere.In(wording.ZhHant)} {
		if strings.Contains(page, retired) {
			t.Errorf("the plate still heads itself %q", retired)
		}
	}

	bare := keptView()
	bare.Continue.Section, bare.Continue.Age, bare.Continue.AgeAt = RowPlace{}, "", ""
	section := between(t, renderHome(t, bare, wording.ZhHant), `<section class="y-continue"`, "</section>")
	for _, absent := range []string{"y-continue__section", "停在", "y-continue__age", "<time"} {
		if strings.Contains(section, absent) {
			t.Errorf("a bookmark with no heading and no time draws %q: %s", absent, section)
		}
	}
}

// The band of recent changes lists the notes that changed last, newest first,
// five at most, each with its day and folder and whether it is a draft; it leaves out a note with no time
// and any note that shares its second with one the band has no room for, which
// is how a copy stamps its files, and is not drawn where that leaves nothing or
// where the band would hold every note there is.
func TestTheRecentBandListsWhatChangedLast(t *testing.T) {
	t.Parallel()

	taipei := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, taipei)
	at := func(day, hour, minute, second int) time.Time {
		return time.Date(2026, 11, day, hour, minute, second, 0, time.UTC)
	}
	note := func(rel string, when time.Time) nav.NoteSummary {
		return nav.NoteSummary{Title: strings.TrimSuffix(path.Base(rel), ".md"), RelPath: rel, Modified: when}
	}
	type line struct{ title, href, when, where string }
	lines := func(s Shelf) []line {
		out := make([]line, 0, len(s.Rows))
		for _, r := range s.Rows {
			if !r.ByTitle {
				t.Errorf("row %q is not read by its title", r.Text)
			}
			out = append(out, line{r.Text, r.Href, r.When, r.Where})
		}
		return out
	}

	for _, tt := range []struct {
		name  string
		notes []nav.NoteSummary
		want  []line
	}{
		{
			name: "newest first, five at most, a root note with no folder, a draft said so",
			notes: []nav.NoteSummary{
				note("Notes/a.md", at(20, 1, 0, 0)),
				note("Notes/Books/b.md", at(25, 17, 0, 0)), // 01:00 on the 26th east of UTC
				{Title: "c", RelPath: "c.md", Modified: at(24, 1, 0, 0), Draft: true},
				{Title: "d", RelPath: "Notes/d.md", Modified: at(23, 1, 0, 0), Draft: true},
				note("Notes/e.md", at(22, 1, 0, 0)),
				note("Notes/f.md", at(21, 1, 0, 0)),
				note("Notes/none.md", time.Time{}),
			},
			want: []line{
				{"b", "/notes/Notes/Books/b.md", "11-26", "Notes/Books"},
				{"c", "/notes/c.md", "11-24", "草稿"},
				{"d", "/notes/Notes/d.md", "11-23", "Notes · 草稿"},
				{"e", "/notes/Notes/e.md", "11-22", "Notes"},
				{"f", "/notes/Notes/f.md", "11-21", "Notes"},
			},
		},
		{
			name: "a second shared inside the band is still listed",
			notes: []nav.NoteSummary{
				note("Notes/a.md", at(25, 1, 0, 0).Add(300*time.Millisecond)),
				note("Notes/b.md", at(25, 1, 0, 0).Add(100*time.Millisecond)),
				note("Notes/c.md", at(24, 1, 0, 0)),
				note("Notes/d.md", at(23, 1, 0, 0)),
				note("Notes/e.md", at(22, 1, 0, 0)),
				note("Notes/f.md", at(21, 1, 0, 0)),
			},
			want: []line{
				{"a", "/notes/Notes/a.md", "11-25", "Notes"},
				{"b", "/notes/Notes/b.md", "11-25", "Notes"},
				{"c", "/notes/Notes/c.md", "11-24", "Notes"},
				{"d", "/notes/Notes/d.md", "11-23", "Notes"},
				{"e", "/notes/Notes/e.md", "11-22", "Notes"},
			},
		},
		{
			name: "no more notes than the band holds",
			notes: []nav.NoteSummary{
				note("Notes/a.md", at(25, 1, 0, 0)),
				note("Notes/b.md", at(24, 1, 0, 0)),
				note("Notes/c.md", at(23, 1, 0, 0)),
				note("Notes/d.md", at(22, 1, 0, 0)),
				note("Notes/e.md", at(21, 1, 0, 0)),
				note("Notes/none.md", time.Time{}),
			},
		},
		{
			name: "a copy that stamped more notes in one second than the band holds",
			notes: []nav.NoteSummary{
				note("Notes/a.md", at(25, 1, 0, 0).Add(600*time.Millisecond)),
				note("Notes/b.md", at(25, 1, 0, 0).Add(500*time.Millisecond)),
				note("Notes/c.md", at(25, 1, 0, 0).Add(400*time.Millisecond)),
				note("Notes/d.md", at(25, 1, 0, 0).Add(300*time.Millisecond)),
				note("Notes/e.md", at(25, 1, 0, 0).Add(200*time.Millisecond)),
				note("Notes/f.md", at(25, 1, 0, 0).Add(100*time.Millisecond)),
			},
		},
		{
			name: "notes a copy stamped in the second of one left out are not listed",
			notes: []nav.NoteSummary{
				note("Notes/new.md", at(25, 2, 0, 0)),
				note("Notes/a.md", at(25, 1, 0, 0).Add(600*time.Millisecond)),
				note("Notes/b.md", at(25, 1, 0, 0).Add(500*time.Millisecond)),
				note("Notes/c.md", at(25, 1, 0, 0).Add(400*time.Millisecond)),
				note("Notes/d.md", at(25, 1, 0, 0).Add(300*time.Millisecond)),
				note("Notes/e.md", at(25, 1, 0, 0).Add(200*time.Millisecond)),
			},
			want: []line{{"new", "/notes/Notes/new.md", "11-25", "Notes"}},
		},
		{name: "no notes"},
		{name: "no note with a time", notes: []nav.NoteSummary{note("Notes/a.md", time.Time{})}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shelf := NewRecentShelf(tt.notes, now, wording.ZhHant, nil)
			if got := lines(shelf); !slices.Equal(got, tt.want) {
				t.Errorf("NewRecentShelf() rows = %v, want %v", got, tt.want)
			}
			if len(tt.want) > 0 && shelf.Title != "最近變更" {
				t.Errorf("NewRecentShelf() title = %q, want 最近變更", shelf.Title)
			}
			page := renderHome(t, &HomeView{Recent: shelf}, wording.ZhHant)
			if drawn := strings.Contains(page, `data-home-block="recent"`); drawn != (len(tt.want) > 0) {
				t.Errorf("the desk draws the band = %v, want %v", drawn, len(tt.want) > 0)
			}
		})
	}
}

// The band is a heading and its rows: no page stands behind it, so its heading
// is no link and it offers no way to all of it, and each row's folder is its
// own line after the day.
func TestTheRecentBandIsAHeadingOverItsRows(t *testing.T) {
	t.Parallel()

	notes := []nav.NoteSummary{{Title: "Alpha", RelPath: "Notes/alpha.md", Modified: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)}}
	for day := range 5 {
		notes = append(notes, nav.NoteSummary{Title: "Older", RelPath: "Older.md", Modified: time.Date(2026, 11, 10+day, 12, 0, 0, 0, time.UTC)})
	}
	shelf := NewRecentShelf(notes, time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC), wording.En, func(string) string { return "en" })
	block := between(t, renderHome(t, &HomeView{Recent: shelf}, wording.En), `data-home-block="recent"`, "</section>")
	for _, want := range []string{
		`<h2 id="home-recent-title">Recently changed</h2>`,
		`<a class="ui-navitem ui-navitem--by-title" href="/notes/Notes/alpha.md" data-desk-item><span lang="en">Alpha</span> <span class="ui-navitem__when">11-25</span> <span class="y-modeblock__where">Notes</span></a>`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("the band lacks %q: %s", want, block)
		}
	}
	if strings.Contains(block, "y-shelfall") || strings.Contains(block, `href=""`) {
		t.Errorf("the band offers a way on to a page it does not have: %s", block)
	}
}
