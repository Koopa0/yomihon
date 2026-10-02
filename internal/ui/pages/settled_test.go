package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// settledBody is the course the tests below read: Slices and GC at ready,
// Arrays and the side branch's Tuning at draft, a template, and a lesson nobody
// wrote. It is read through the real navigation build under a contract that
// settles ready, so the rows' flags are the ones the product would give.
const settledBody = "## Data {sequence=primary}\n\n" +
	"### Text {sequence=primary}\n\n" +
	"- [[Slices]]\n" +
	"- [[Arrays]]\n" +
	"\n## Memory {sequence=primary}\n\n" +
	"- [[GC]]\n" +
	"\t- 選修 {sequence=local}\n" +
	"\t\t- [[Tuning]]\n"

func renderComponent(t *testing.T, component templ.Component) string {
	t.Helper()
	var out bytes.Buffer
	if err := component.Render(t.Context(), &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	return out.String()
}

func renderCourse(t *testing.T, view *PathView, lang wording.Lang) string {
	t.Helper()
	return renderComponent(t, Syllabus(*view, layouts.Chrome{Lang: lang}))
}

// courseOf reads a built course into the page view the cover gives it.
func courseOf(path *nav.Path, cover *CourseCover) *PathView {
	view := BuildPathView(path, nil, cover)
	return &view
}

// TestACourseRowPrintsItsStatusOnlyWhenTheContractDoesNotSettleIt holds the
// rule the whole change is for: down a course the finished lessons carry no
// word and the lessons still open say what they are, in the vault's own
// spelling. It reads the rows through the real navigation build, so a row's
// flag is the one the contract gave it, and it draws the same course under a
// contract that settles nothing to show that the labels are the fallback and
// not an accident of the fixture.
func TestACourseRowPrintsItsStatusOnlyWhenTheContractDoesNotSettleIt(t *testing.T) {
	t.Parallel()

	settledPath := buildSettledTestPath(t, settledBody)
	settled := renderCourse(t, courseOf(&settledPath, &CourseCover{}), wording.ZhHant)

	if got := strings.Count(settled, `class="ui-status"`); got != 2 {
		t.Errorf("a course whose contract settles ready prints %d status labels, want 2: Arrays and Tuning, the two lessons still at draft", got)
	}
	for _, want := range []string{`<span class="ui-status">draft</span>`} {
		if !strings.Contains(settled, want) {
			t.Errorf("the course lacks %q, the vault's own spelling of a status that needs noticing", want)
		}
	}
	if strings.Contains(settled, ">ready<") {
		t.Error("a lesson at the settled status prints its word, which repeats the ordinary state on every row")
	}

	openPath := buildTestPath(t, settledBody)
	open := renderCourse(t, courseOf(&openPath, &CourseCover{}), wording.ZhHant)
	if got := strings.Count(open, `class="ui-status"`); got != 4 {
		t.Errorf("a course whose contract settles nothing prints %d status labels, want 4: every lesson keeps its word", got)
	}
	if !strings.Contains(open, `<span class="ui-status">ready</span>`) {
		t.Error("the fallback course lacks the ready label, so the settled course above proves nothing about hiding it")
	}
	if strings.Contains(open, "data-course-unsettled") {
		t.Error("a contract that settles nothing prints a count of lessons not yet settled, which would be every lesson")
	}
}

// TestTheCourseHeadCountsWhatIsNotYetSettled holds the one figure that stands
// where the count of ready lessons did, in both languages, and its silence when
// there is nothing to say.
func TestTheCourseHeadCountsWhatIsNotYetSettled(t *testing.T) {
	t.Parallel()

	path := buildSettledTestPath(t, settledBody)
	view := courseOf(&path, &CourseCover{})
	if view.Unsettled != 1 || view.Entries != 3 {
		t.Fatalf("the fixture course has %d unsettled of %d lessons, want 1 of 3: the side branch's draft is outside the main line", view.Unsettled, view.Entries)
	}

	for lang, want := range map[wording.Lang]string{
		wording.ZhHant: `<span class="y-syl-unsettled" data-course-unsettled>其中 1 課尚未定案</span>`,
		wording.En:     `<span class="y-syl-unsettled" data-course-unsettled>1 not yet settled</span>`,
	} {
		if html := renderCourse(t, view, lang); !strings.Contains(html, want) {
			t.Errorf("%s course head lacks %q", lang.Tag(), want)
		}
	}

	view.Unsettled = 0
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		if html := renderCourse(t, view, lang); strings.Contains(html, "data-course-unsettled") {
			t.Errorf("%s course head draws a count when every lesson is settled", lang.Tag())
		}
	}
}

// TestTheKeptPlaceMarksOneCourseRowAndOnlyThatRow holds the reader's bookmark:
// the one lesson that holds the place they kept carries it, with its words for
// a reader who is listening, and a place kept in a note the course does not
// teach marks nothing.
func TestTheKeptPlaceMarksOneCourseRowAndOnlyThatRow(t *testing.T) {
	t.Parallel()

	path := buildSettledTestPath(t, settledBody)

	kept := renderCourse(t, courseOf(&path, &CourseCover{KeptNote: "Writing/Arrays.md"}), wording.En)
	if got := strings.Count(kept, `class="y-keptplace"`); got != 1 {
		t.Fatalf("%d lessons carry the bookmark, want exactly 1", got)
	}
	_, row, found := strings.Cut(kept, `href="/notes/Writing/Arrays.md"`)
	if !found {
		t.Fatal("the course does not list the Arrays lesson")
	}
	row, _, _ = strings.Cut(row, "</a>")
	for _, want := range []string{`<span class="y-keptmark" aria-hidden="true"></span>`, `Where you left off`} {
		if !strings.Contains(row, want) {
			t.Errorf("the lesson that holds the kept place lacks %q; row = %s", want, row)
		}
	}
	if zh := renderCourse(t, courseOf(&path, &CourseCover{KeptNote: "Writing/Arrays.md"}), wording.ZhHant); !strings.Contains(zh, wording.MarkKeptHere.In(wording.ZhHant)) {
		t.Error("the Chinese course does not say in words where the place was kept")
	}

	elsewhere := renderCourse(t, courseOf(&path, &CourseCover{KeptNote: "Writing/NotInThisCourse.md"}), wording.En)
	if strings.Contains(elsewhere, "y-keptplace") {
		t.Error("a place kept in a note this course does not teach marks a lesson")
	}
	none := renderCourse(t, courseOf(&path, &CourseCover{}), wording.En)
	if strings.Contains(none, "y-keptplace") {
		t.Error("a reader who kept no place sees a bookmark")
	}
}

func TestTheBookRailPrintsStatusAndBookmarkByTheSameRules(t *testing.T) {
	t.Parallel()

	row := func(entry PathEntryView, lang wording.Lang) string {
		entry.Kind = nav.EntryResolved
		entry.Href = "/notes/L.md"
		entry.Name = "L"
		return renderComponent(t, bookRailEntry(layouts.Chrome{Lang: lang}, entry))
	}

	if html := row(PathEntryView{Status: "ready", Settled: true}, wording.En); strings.Contains(html, "ui-navitem__count") {
		t.Errorf("a settled lesson prints a count chip in the rail: %s", html)
	}
	if html := row(PathEntryView{Status: "draft"}, wording.En); !strings.Contains(html, `<span class="ui-navitem__count">draft</span>`) {
		t.Errorf("an open lesson prints no status in the rail: %s", html)
	}
	if html := row(PathEntryView{Status: "draft", Here: true}, wording.En); !strings.Contains(html, "ui-navitem__count--reserved") {
		t.Errorf("the lesson being read does not hold the hidden chip that keeps its name wrapping where its neighbours' do: %s", html)
	}
	if html := row(PathEntryView{Status: "ready", Settled: true, Here: true}, wording.En); strings.Contains(html, "ui-navitem__count") {
		t.Errorf("a settled lesson being read holds a chip no neighbour at that status holds: %s", html)
	}
	if html := row(PathEntryView{Status: "ready", Settled: true, Kept: true}, wording.ZhHant); !strings.Contains(html, `class="y-keptmark" aria-hidden="true"`) || !strings.Contains(html, wording.MarkKeptHere.In(wording.ZhHant)) {
		t.Errorf("the lesson holding the kept place carries no bookmark in the rail: %s", html)
	}
	if html := row(PathEntryView{Status: "draft"}, wording.En); strings.Contains(html, "y-keptmark") {
		t.Errorf("a lesson that holds no kept place carries the bookmark: %s", html)
	}
}

func TestTheVaultSidebarPrintsAStatusOnlyForTheNotesNotYetSettled(t *testing.T) {
	t.Parallel()

	chrome := layouts.Chrome{Lang: wording.En}
	pathRow := func(status string, settled bool) string {
		return renderComponent(t, pathEntryLink(Sidebar{}, chrome, &nav.PathEntry{Name: "N", RelPath: "N.md", Kind: nav.EntryResolved, Status: status, Settled: settled}))
	}
	mapRow := func(status string, settled bool) string {
		return renderComponent(t, entryLink(Sidebar{}, chrome, nav.MapEntry{Name: "N", RelPath: "N.md", Kind: nav.EntryResolved, Status: status, Settled: settled}))
	}
	for name, row := range map[string]func(string, bool) string{"course row": pathRow, "map row": mapRow} {
		if html := row("ready", true); strings.Contains(html, "ui-navitem__count") {
			t.Errorf("%s: a settled note prints a status: %s", name, html)
		}
		if html := row("draft", false); !strings.Contains(html, `<span class="ui-navitem__count">draft</span>`) {
			t.Errorf("%s: an unsettled note prints no status: %s", name, html)
		}
	}
}

func TestTheRecentListPrintsAStatusOnlyForTheNotesNotYetSettled(t *testing.T) {
	t.Parallel()

	recent := NewRecentBlock([]HomeNote{
		{Title: "Finished", RelPath: "A.md", Status: "ready", Settled: true},
		{Title: "Open", RelPath: "B.md", Status: "draft"},
	}, true, false, wording.En)
	html := renderComponent(t, shelfKeeping(recent, StatusDistribution{}, layouts.Chrome{Lang: wording.En}))

	if got := strings.Count(html, `class="ui-status"`); got != 1 {
		t.Errorf("the recent list prints %d status labels, want 1: the open note's", got)
	}
	if !strings.Contains(html, `<span class="ui-status">draft</span>`) || strings.Contains(html, ">ready<") {
		t.Errorf("the recent list labels the wrong note: %s", html)
	}
}

// TestTheNoteBadgeIsTheSameNeutralBadgeForEveryWord holds the badge on the
// note's own page to one look. No status gets a class of its own, whatever its
// word and whether or not the contract settles it, because the stylesheet keeps
// no colour for any of them.
func TestTheNoteBadgeIsTheSameNeutralBadgeForEveryWord(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"ready", "draft", "evergreen", "published", "archived", "nearly-ready"} {
		view := NoteView{Governed: true, RelPath: "a.md", Status: status, Transitions: []Transition{{To: "archived"}}}
		for name, html := range map[string]string{
			"panel": renderComponent(t, statusPanel(view, wording.En)),
			"bar":   renderComponent(t, statusBar(view, wording.En)),
		} {
			if !strings.Contains(html, `<span class="ui-status">`+status+`</span>`) {
				t.Errorf("%s: the %s badge is not the plain neutral one: %s", name, status, html)
			}
			if strings.Contains(html, "ui-status--") {
				t.Errorf("%s: the %s badge carries a class of its own", name, status)
			}
		}
	}
	if html := renderComponent(t, statusPanel(NoteView{Governed: true, RelPath: "a.md", Status: "draft"}, wording.En)); !strings.Contains(html, ">Status set by the author<") {
		t.Errorf("the panel is not titled as the author's status in English: %s", html)
	}
	if html := renderComponent(t, statusPanel(NoteView{Governed: true, RelPath: "a.md", Status: "draft"}, wording.ZhHant)); !strings.Contains(html, ">作者標記的狀態<") {
		t.Errorf("the panel is not titled as the author's status in Chinese: %s", html)
	}
}

// TestTheKeepControlWearsTheBookmarkOnlyWhereThePlaceIsInThisNote holds both
// faces of the control: the rail's and the header's draw the reader's bookmark
// through one attribute, with the sentence for a reader who cannot see it, and a
// note that holds no kept place draws neither.
func TestTheKeepControlWearsTheBookmarkOnlyWhereThePlaceIsInThisNote(t *testing.T) {
	t.Parallel()

	view := NoteView{RelPath: "a.md", ContentIdentity: "abc", MarkAddress: "/mark"}
	if html := renderComponent(t, markControl(view, wording.En)); strings.Contains(html, " data-mark-kept") || strings.Contains(html, "y-offscreen") {
		t.Errorf("a note that holds no kept place draws the bookmark or says it does: %s", html)
	}
	view.PlaceKept = true
	rail := renderComponent(t, markControl(view, wording.En))
	if !strings.Contains(rail, " data-mark-kept") || !strings.Contains(rail, `<span class="y-offscreen">Your place is kept.</span>`) {
		t.Errorf("the rail's control does not wear the bookmark where the note holds the place: %s", rail)
	}

	for kept, want := range map[bool]bool{true: true, false: false} {
		header := renderComponent(t, layouts.Base(layouts.Chrome{Lang: wording.En, Mark: &layouts.MarkOffer{Path: "a.md", Identity: "abc", Endpoint: "/mark", Kept: kept}}))
		if got := strings.Contains(header, " data-mark-kept"); got != want {
			t.Errorf("header control with Kept=%v draws data-mark-kept = %v, want %v", kept, got, want)
		}
	}
}

func TestTheDeskLinkBackToTheKeptPlaceWearsTheBookmarkAndASentenceDoesNot(t *testing.T) {
	t.Parallel()

	link := renderComponent(t, continueRow(ContinueRow{Show: true, Title: "Alpha", Href: "/notes/a.md"}, wording.En))
	if !strings.Contains(link, `data-continue-link>`) || !strings.Contains(link, `<span class="y-keptmark" aria-hidden="true"></span>`) {
		t.Errorf("the way back to the place does not wear the bookmark: %s", link)
	}
	gone := renderComponent(t, continueRow(ContinueRow{Show: true, Title: "Alpha", Notice: "gone"}, wording.En))
	if strings.Contains(gone, "y-keptmark") {
		t.Errorf("a row with nowhere to go wears the bookmark: %s", gone)
	}
}
