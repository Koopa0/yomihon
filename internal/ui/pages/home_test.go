package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func renderHome(t *testing.T, v HomeView, lang wording.Lang) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Home(v, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
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

func keptView() HomeView {
	return HomeView{Continue: ContinueRow{
		Show: true, Title: "Alpha", Href: "/notes/a.md?at=40#cancel", Kicker: "A course · Part one",
		Section: RowPlace{Text: "Cancel"},
	}}
}

// TestTheDeskDrawsNoSearchField holds that the page itself asks for no search:
// the header carries the one search the product has, and a second field on the
// desk asked for as much of the screen as any of the ways in.
func TestTheDeskDrawsNoSearchField(t *testing.T) {
	t.Parallel()

	for name, v := range map[string]HomeView{"fresh": {}, "kept": keptView()} {
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
		if page := renderHome(t, HomeView{}, lang); !strings.Contains(page, `<h1 class="y-title y-desklabel">`+name+`</h1>`) {
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
// reader returns to: where it sits, its name, the heading it was kept under,
// and one way back that is not the name — the desk offers exactly one.
func TestTheKeptPlaceIsTheLargestTextOnTheDesk(t *testing.T) {
	t.Parallel()

	page := renderHome(t, keptView(), wording.ZhHant)
	section := between(t, page, `<section class="y-continue"`, "</section>")
	order := []string{
		`class="y-continue__kicker"`, "A course · Part one",
		`<h2 class="y-continue__title" id="home-continue-title">Alpha</h2>`,
		`<p class="y-continue__section">Cancel</p>`,
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
	maps := DeskBlock{Mode: mapMode, Shelf: Shelf{Title: "Maps", Href: "/maps", Rows: []Row{{Text: "M", Href: "/notes/m.md"}}}}

	fresh := renderHome(t, HomeView{Blocks: []DeskBlock{courses, maps}}, wording.En)
	lead := between(t, fresh, `<div class="y-homelead">`, "</div>")
	if !strings.Contains(lead, `data-home-block="courses"`) || !strings.Contains(lead, "y-modeblock--lead") {
		t.Errorf("a desk with no place kept does not open on the courses: %s", lead)
	}
	if strings.Count(fresh, `data-home-block="courses"`) != 1 {
		t.Errorf("the courses are drawn more than once: %s", fresh)
	}
	if strings.Contains(fresh[strings.Index(fresh, `class="y-homegrid"`):], "y-modeblock--lead") {
		t.Errorf("a block inside the grid is drawn as the lead: %s", fresh)
	}

	kept := keptView()
	kept.Blocks = []DeskBlock{courses, maps}
	if page := renderHome(t, kept, wording.En); strings.Contains(page, "y-homelead") || strings.Contains(page, "y-modeblock--lead") {
		t.Errorf("a desk with a place kept sets the courses above it: %s", page)
	}

	empty := courses
	empty.Shelf.Rows = nil
	if page := renderHome(t, HomeView{Blocks: []DeskBlock{empty, maps}}, wording.En); strings.Contains(page, "y-homelead") {
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
		page := renderHome(t, HomeView{}, lang)
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
