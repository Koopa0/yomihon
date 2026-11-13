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

// draftBody is the course the tests below read: Slices and GC finished,
// Arrays and the side branch's Tuning declaring draft: true, a template, and a
// lesson nobody wrote. It is read through the real navigation build, so the
// rows' flags are the ones the product would give.
const draftBody = "## Data {sequence=primary}\n\n" +
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

// TestACourseRowMarksOnlyItsDrafts holds the rule the course rows follow:
// a finished lesson carries no word and a draft says so, in the interface's
// language. It reads the rows through the real navigation build, so a row's
// flag is the one the lesson's own frontmatter gave it.
func TestACourseRowMarksOnlyItsDrafts(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, draftBody)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		html := renderCourse(t, courseOf(&path, &CourseCover{}), lang)
		mark := `<span class="ui-status">` + wording.Draft.In(lang) + `</span>`
		if got := strings.Count(html, `class="ui-status"`); got != 2 {
			t.Errorf("%s: the course prints %d marks, want 2: Arrays and Tuning, the two drafts", lang.Tag(), got)
		}
		if got := strings.Count(html, mark); got != 2 {
			t.Errorf("%s: the course carries %q %d times, want 2", lang.Tag(), mark, got)
		}
	}
}

// TestTheCourseHeadCountsItsDrafts holds the one figure the head states after
// the extent, in both languages, and its silence when there is nothing to say.
func TestTheCourseHeadCountsItsDrafts(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, draftBody)
	view := courseOf(&path, &CourseCover{})
	if view.Drafts != 1 || view.Entries != 3 {
		t.Fatalf("the fixture course has %d drafts of %d lessons, want 1 of 3: the side branch's draft is outside the main line", view.Drafts, view.Entries)
	}

	for lang, want := range map[wording.Lang]string{
		wording.ZhHant: `<span class="y-syl-drafts" data-course-drafts>其中 1 課是草稿</span>`,
		wording.En:     `<span class="y-syl-drafts" data-course-drafts>1 are drafts</span>`,
	} {
		if html := renderCourse(t, view, lang); !strings.Contains(html, want) {
			t.Errorf("%s course head lacks %q", lang.Tag(), want)
		}
	}

	view.Drafts = 0
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		if html := renderCourse(t, view, lang); strings.Contains(html, "data-course-drafts") {
			t.Errorf("%s course head draws a count when no lesson is a draft", lang.Tag())
		}
	}
}

// TestTheKeptPlaceMarksOneCourseRowAndOnlyThatRow holds the reader's bookmark:
// the one lesson that holds the place they kept carries it, with its words for
// a reader who is listening, and a place kept in a note the course does not
// teach marks nothing.
func TestTheKeptPlaceMarksOneCourseRowAndOnlyThatRow(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, draftBody)

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

func TestTheBookRailMarksDraftsAndBookmarkByTheSameRules(t *testing.T) {
	t.Parallel()

	row := func(entry PathEntryView, lang wording.Lang) string {
		entry.Kind = nav.EntryResolved
		entry.Href = "/notes/L.md"
		entry.Name = "L"
		return renderComponent(t, bookRailEntry(layouts.Chrome{Lang: lang}, entry))
	}

	if html := row(PathEntryView{}, wording.En); strings.Contains(html, "ui-navitem__count") {
		t.Errorf("a finished lesson prints a count chip in the rail: %s", html)
	}
	if html := row(PathEntryView{Draft: true}, wording.En); !strings.Contains(html, `<span class="ui-navitem__count">Draft</span>`) {
		t.Errorf("a draft lesson prints no mark in the rail: %s", html)
	}
	if html := row(PathEntryView{Draft: true}, wording.ZhHant); !strings.Contains(html, `<span class="ui-navitem__count">草稿</span>`) {
		t.Errorf("a draft lesson's mark is not in the interface's language: %s", html)
	}
	if html := row(PathEntryView{Draft: true, Here: true}, wording.En); !strings.Contains(html, "ui-navitem__count--reserved") {
		t.Errorf("the draft being read does not hold the hidden chip that keeps its name wrapping where its neighbours' do: %s", html)
	}
	if html := row(PathEntryView{Here: true}, wording.En); strings.Contains(html, "ui-navitem__count") {
		t.Errorf("a finished lesson being read holds a chip no finished neighbour holds: %s", html)
	}
	if html := row(PathEntryView{Kept: true}, wording.ZhHant); !strings.Contains(html, `class="y-keptmark" aria-hidden="true"`) || !strings.Contains(html, wording.MarkKeptHere.In(wording.ZhHant)) {
		t.Errorf("the lesson holding the kept place carries no bookmark in the rail: %s", html)
	}
	if html := row(PathEntryView{Draft: true}, wording.En); strings.Contains(html, "y-keptmark") {
		t.Errorf("a lesson that holds no kept place carries the bookmark: %s", html)
	}
}

func TestTheVaultSidebarMarksOnlyDrafts(t *testing.T) {
	t.Parallel()

	chrome := layouts.Chrome{Lang: wording.En}
	pathRow := func(draft bool) string {
		return renderComponent(t, pathEntryLink(Sidebar{}, chrome, &nav.PathEntry{Name: "N", RelPath: "N.md", Kind: nav.EntryResolved, Draft: draft}))
	}
	mapRow := func(draft bool) string {
		return renderComponent(t, entryLink(Sidebar{}, chrome, nav.MapEntry{Name: "N", RelPath: "N.md", Kind: nav.EntryResolved, Draft: draft}))
	}
	for name, row := range map[string]func(bool) string{"course row": pathRow, "map row": mapRow} {
		if html := row(false); strings.Contains(html, "ui-navitem__count") {
			t.Errorf("%s: a finished note prints a mark: %s", name, html)
		}
		if html := row(true); !strings.Contains(html, `<span class="ui-navitem__count">Draft</span>`) {
			t.Errorf("%s: a draft prints no mark: %s", name, html)
		}
	}
}

func TestTheRecentListMarksOnlyDrafts(t *testing.T) {
	t.Parallel()

	recent := NewRecentBlock([]HomeNote{
		{Title: "Finished", RelPath: "A.md"},
		{Title: "Open", RelPath: "B.md", Draft: true},
	}, true, false, wording.En)
	html := renderComponent(t, shelfKeeping(recent, layouts.Chrome{Lang: wording.En}))

	if got := strings.Count(html, `class="ui-status"`); got != 1 {
		t.Errorf("the recent list prints %d marks, want 1: the draft's", got)
	}
	_, open, _ := strings.Cut(html, `href="/notes/B.md"`)
	open, _, _ = strings.Cut(open, "</a>")
	if !strings.Contains(open, `<span class="ui-status">Draft</span>`) {
		t.Errorf("the recent list does not mark the draft: %s", html)
	}
}

// TestTheNoteHeadMarksADraftInEachOfItsCopies holds the quiet word a draft
// carries under its title. The head is drawn twice — the narrow disclosure and
// the wide-open copy, CSS choosing one — so a draft carries the mark once in
// each, in the interface's language, and a finished note carries none.
func TestTheNoteHeadMarksADraftInEachOfItsCopies(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		draft := renderComponent(t, articleHead(NoteView{Title: "A", RelPath: "a.md", Type: "note", Draft: true}, lang))
		if got := strings.Count(draft, "data-note-draft"); got != 2 {
			t.Errorf("%s: a draft's head carries %d marks, want one in each of its two copies", lang.Tag(), got)
		}
		mark := `<span class="ui-status" lang="` + lang.Tag() + `" data-note-draft>` + wording.Draft.In(lang) + `</span>`
		if got := strings.Count(draft, mark); got != 2 {
			t.Errorf("%s: the head's mark is not %q in both copies: %s", lang.Tag(), mark, draft)
		}
		finished := renderComponent(t, articleHead(NoteView{Title: "A", RelPath: "a.md", Type: "note"}, lang))
		if strings.Contains(finished, "data-note-draft") || strings.Contains(finished, "ui-status") {
			t.Errorf("%s: a finished note's head carries a mark: %s", lang.Tag(), finished)
		}
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
