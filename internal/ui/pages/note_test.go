package pages

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNoteArticleLanguageComesOnlyFromAuthority holds what the article element
// is allowed to say about the language a note is written in. A tag that
// reached the view is rendered as the note declared it; a view carrying none
// leaves the element with no language of its own, so the language the page is
// written in stands for the article too.
//
// Each expectation is the whole opening tag rather than the attribute alone.
// The absent case differs from the present ones by the space before the
// attribute as much as by the attribute, so an expectation that named only
// `lang="…"` would have no way to state the absent case at all, and one that
// named only the absence would be satisfied by an article that had lost its
// class as readily as by the right one.
func TestNoteArticleLanguageComesOnlyFromAuthority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "no authority states no language", want: `<article class="y-article">`},
		{name: "Japanese", lang: "ja", want: `<article class="y-article" lang="ja">`},
		{name: "Traditional Chinese", lang: "zh-Hant", want: `<article class="y-article" lang="zh-Hant">`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Note(NoteView{Language: tt.lang}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if html := buf.String(); !strings.Contains(html, tt.want) {
				t.Errorf("article opening tag = missing %q in %q", tt.want, html)
			}
		})
	}
}

func TestTheEndOfTheTextUsesChromeLanguage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{
		Language:   "ja",
		Diagnostic: "invalid frontmatter",
	}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<div class="y-related" id="_y-end" tabindex="-1" lang="zh-Hant">`) {
		t.Errorf("the end of the text does not restore the chrome language inside a Japanese article: %q", html)
	}
}

// TestSchemaNoticesStandInReadingColumn pins where unknown-field guidance
// renders: once, above the prose, in the reading column, at every width.
func TestSchemaNoticesStandInReadingColumn(t *testing.T) {
	t.Parallel()

	v := NoteView{
		Title:   "Probe",
		RelPath: "Writing/lessons/japanese/L01.md",
		SchemaNotices: [][]wording.SchemaPart{
			{{Text: "mystery_key", Code: true}, {Text: " 不是 schema 認得的欄位。"}},
		},
	}
	var buf bytes.Buffer
	if err := Note(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	_, afterArticle, ok := strings.Cut(html, `<article class="y-article">`)
	if !ok {
		t.Fatal("the page has no article")
	}
	article, _, ok := strings.Cut(afterArticle, "</article>")
	if !ok {
		t.Fatal("the article is not closed")
	}

	const notice = `<code>mystery_key</code>`
	if got := strings.Count(html, notice); got != 1 {
		t.Fatalf("the schema notice appears %d times in the page, want once", got)
	}
	if !strings.Contains(article, notice) {
		t.Fatalf("the schema notices block is not in the reading column:\n%s", article)
	}
	if strings.Contains(html, "y-rail-right") {
		t.Fatal("the page draws a right rail, which is gone")
	}
	proseStart := strings.Index(article, `<div class="y-prose">`)
	noticeStart := strings.Index(article, notice)
	if proseStart < 0 || noticeStart < 0 || noticeStart > proseStart {
		t.Errorf("the schema notices block must sit above the prose; notice@%d prose@%d", noticeStart, proseStart)
	}
}

// TestDiagnosticsStandAtTheEndOfTheText pins the placement, not the contents.
// The check's findings about a note are what a reader who wants them goes to
// look for, and a list of them above the prose was the last thing the page said
// before the text it was about, worst on the notes that have the most of them.
// They wait at the end of the text with the other aids, at every width, with
// their count stated, and the page's first screen is the title and the text.
func TestDiagnosticsStandAtTheEndOfTheText(t *testing.T) {
	t.Parallel()

	view := NoteView{
		Title:   "DDIA",
		RelPath: "Sources/DDIA.md",
		RenderDiagnostics: []render.Diagnostic{
			{Kind: render.DiagWikilinkBroken, Target: "Relational Model", Message: "does not resolve"},
			{Kind: render.DiagWikilinkBroken, Target: "Leader Election", Message: "does not resolve"},
		},
		BodyHTML: "<p>body</p>",
	}
	var buf bytes.Buffer
	if err := noteArticle(view, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render noteArticle: %v", err)
	}
	got := buf.String()

	prose := strings.Index(got, `<div class="y-prose">`)
	related := strings.Index(got, `<div class="y-related"`)
	list := strings.Index(got, "y-diaglist")
	if prose < 0 || related < 0 || list < 0 {
		t.Fatalf("prose@%d related@%d diaglist@%d; every one of them has to render:\n%s", prose, related, list, got)
	}
	if prose >= related || related >= list {
		t.Errorf("prose@%d related@%d diaglist@%d; want the findings inside the block that ends the text", prose, related, list)
	}
	if above := got[:prose]; strings.Contains(above, "y-diaglist") || strings.Contains(above, "y-inlineaids") {
		t.Errorf("a list of findings or a folded row of aids stands above the prose:\n%s", above)
	}
	if !strings.Contains(got, "筆記狀況") {
		t.Errorf("the block does not name what it holds:\n%s", got)
	}
	if !strings.Contains(got, ">2<") {
		t.Errorf("the count is not stated:\n%s", got)
	}
}

// TestTheEndOfTheTextHoldsEveryAidAndNothingStandsAboveTheProse holds the shape
// of a note that has them all. The contents are in the sidebar and everything
// else waits after the text, in one block, in one order: the note to read
// beside this one, the sources it declared, the notes declared on it, the notes
// whose text links here, what the check found. Nothing is folded between the
// title and the first sentence, and the block comes before the way onward.
func TestTheEndOfTheTextHoldsEveryAidAndNothingStandsAboveTheProse(t *testing.T) {
	t.Parallel()

	view := NoteView{
		Title:      "T",
		RelPath:    "Notes/T.md",
		TOC:        []render.TOCEntry{{Level: 2, Text: "H", ID: "h"}},
		Diagnostic: "unterminated string",
		BasedOn:    []snapshot.DeclaredSource{{Name: "Book notes", RelPath: "Book notes.md"}},
		DeclaredBy: []DeclaringNoteView{{Note: nav.NoteRef{Name: "Essay", RelPath: "Essay.md"}}},
		Pair:       nav.NoteRef{Name: "Other half", RelPath: "Notes/Other.md"},
		CitedBy:    []nav.NoteRef{{Name: "C01", RelPath: "C01.md"}, {Name: "C02", RelPath: "C02.md"}},
		Next:       nav.NoteRef{Name: "Next", RelPath: "Notes/U.md"},
		StepsLabel: "Notes",
		BodyHTML:   "<p>body</p>",
	}
	var buf bytes.Buffer
	if err := noteArticle(view, wording.En).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render noteArticle: %v", err)
	}
	article := buf.String()

	prose := strings.Index(article, `<div class="y-prose">`)
	related := strings.Index(article, `<div class="y-related" id="_y-end"`)
	steps := strings.Index(article, `<nav class="y-steps`)
	if prose < 0 || related < 0 || steps < 0 {
		t.Fatalf("prose@%d related@%d steps@%d; every one of them has to render:\n%s", prose, related, steps, article)
	}
	if prose >= related || related >= steps {
		t.Fatalf("prose@%d related@%d steps@%d; want the text, the block that ends it, then the way onward", prose, related, steps)
	}
	if above := article[:prose]; strings.Contains(above, "y-inlineaids") || strings.Contains(above, "y-toc-inline") || strings.Contains(above, "y-toc__list") {
		t.Errorf("an aid stands above the prose of a note that has its own page:\n%s", above)
	}
	after := article[related:steps]
	last := -1
	for _, block := range []string{`class="y-pair"`, `class="y-basedon"`, `declared-by`, `class="y-citedby"`, `y-diaglist`} {
		at := strings.Index(after, block)
		if at < 0 {
			t.Errorf("%s is not in the block that ends the text:\n%s", block, after)
			continue
		}
		if at < last {
			t.Errorf("%s comes before the block that should precede it", block)
		}
		last = at
	}
}

// TestTheContentsRowThatLeadsDownIsNamedByWhatItLeadsTo holds the sidebar's last
// contents row to the heading the end block itself opens with, whichever of its
// blocks that is, and holds it absent where the note has no end block. A row
// named with words the block does not print, or leading to an id nothing
// answers to, is a promise the page does not keep.
func TestTheContentsRowThatLeadsDownIsNamedByWhatItLeadsTo(t *testing.T) {
	t.Parallel()

	toc := []render.TOCEntry{{Level: 2, Text: "H", ID: "h"}}
	cited := []nav.NoteRef{{Name: "C01", RelPath: "C01.md"}}
	tests := []struct {
		name string
		view NoteView
		want func(wording.Lang) string
	}{
		{"a note to read beside", NoteView{Pair: nav.NoteRef{Name: "O", RelPath: "O.md"}, CitedBy: cited}, func(l wording.Lang) string { return wording.CompareOffer.In(l) }},
		{"declared sources", NoteView{BasedOn: []snapshot.DeclaredSource{{Name: "B", RelPath: "B.md"}}, CitedBy: cited}, func(l wording.Lang) string { return wording.BasedOn.In(l) }},
		{"notes declared on it", NoteView{DeclaredBy: []DeclaringNoteView{{Note: nav.NoteRef{Name: "E", RelPath: "E.md"}}}, CitedBy: cited}, func(l wording.Lang) string { return wording.DeclaredBy.In(l) }},
		{"notes linking here", NoteView{CitedBy: cited}, func(l wording.Lang) string { return wording.CitedBy.In(l) }},
		{"only the findings", NoteView{Diagnostic: "bad"}, func(l wording.Lang) string { return wording.NoteHealth.In(l) }},
	}
	for _, tt := range tests {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(tt.name+" "+lang.Tag(), func(t *testing.T) {
				t.Parallel()
				view := tt.view
				view.RelPath, view.TOC, view.BodyHTML = "Notes/T.md", toc, "<p>body</p>"
				contents := renderedHTML(t, railContents(&view, lang))
				want := `<a href="#_y-end" lang="` + lang.Tag() + `" data-level="2">` + tt.want(lang) + `</a>`
				if !strings.Contains(contents, want) {
					t.Fatalf("the contents do not end on %q:\n%s", want, contents)
				}
				if strings.Index(contents, `href="#h"`) > strings.Index(contents, `href="#_y-end"`) {
					t.Errorf("the row that leads down comes before the headings:\n%s", contents)
				}
				page := renderedHTML(t, noteArticle(view, lang))
				_, end, found := strings.Cut(page, `id="_y-end"`)
				if !found || !strings.Contains(end, tt.want(lang)) {
					t.Errorf("the end of the text does not answer to the row, or does not print its name %q:\n%s", tt.want(lang), page)
				}
			})
		}
	}

	// A note with no aids ends on its text, and the facts that follow it are not
	// a place the contents lead to: the contents have no row, and the block
	// carries no address for one to name.
	bare := NoteView{RelPath: "Notes/T.md", TOC: toc, BodyHTML: "<p>body</p>"}
	if bare.endShown() {
		t.Fatal("this fixture was meant to end on its text")
	}
	if contents := renderedHTML(t, railContents(&bare, wording.En)); strings.Contains(contents, "_y-end") {
		t.Errorf("the contents lead to an end the note does not have:\n%s", contents)
	}
	if page := renderedHTML(t, noteArticle(bare, wording.En)); strings.Contains(page, "_y-end") || !strings.Contains(page, `<div class="y-related" lang="en"><div class="y-facts">`) {
		t.Errorf("a note with no aids does not end on its facts alone, with no address:\n%s", page)
	}
}

// TestTheConceptSheetCloseNamesWhatItCloses holds the one half of this drawer
// the markup can carry. Opening it stays with the script — the trigger is a
// link the renderer wrote into the note's prose, and the words in the sheet
// are cloned there by that same script — but the press that shuts it says what
// it shuts, so the browser does it and the module keeps no listener for it.
func TestTheConceptSheetCloseNamesWhatItCloses(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	concepts := []lesson.ConceptDoc{{ID: "c1", Title: "は", HTML: "<p>concept</p>"}}
	if err := conceptSheet(concepts, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render concept sheet: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `<dialog id="_y-concept-sheet"`) {
		t.Errorf("the sheet carries no id for a press to name; html = %q", html)
	}
	if !strings.Contains(html, `command="close" commandfor="_y-concept-sheet"`) {
		t.Errorf("the close press does not declare what it closes; html = %q", html)
	}
	// The sheet is also asked to keep the platform's own two ways out, because
	// the press above is the third and the least reachable of them.
	if !strings.Contains(html, `closedby="any"`) {
		t.Errorf("the sheet no longer closes on Escape or a press beside it; html = %q", html)
	}
}

// TestTheConceptSheetIsNamedByItsTitle holds the sheet's accessible name to
// the words the reader sees in its head: the dialog points at the heading the
// script fills, rather than carrying a fixed word that would drift from it. A
// concept that arrives untitled still gets a name, because the heading is what
// names the sheet and an empty one would leave it unnamed.
func TestTheConceptSheetIsNamedByItsTitle(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		lang wording.Lang
		want string
	}{
		{"zh-Hant", wording.ZhHant, "概念筆記"},
		{"en", wording.En, "Concept note"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			concepts := []lesson.ConceptDoc{
				{ID: "c1", Title: "は", HTML: "<p>concept</p>"},
				{ID: "c2", Title: "  ", HTML: "<p>untitled</p>"},
			}
			if err := conceptSheet(concepts, tc.lang).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render concept sheet: %v", err)
			}
			html := buf.String()

			dialog := regexp.MustCompile(`<dialog [^>]*>`).FindString(html)
			if dialog == "" {
				t.Fatalf("no dialog rendered; html = %q", html)
			}
			match := regexp.MustCompile(`aria-labelledby="([^"]+)"`).FindStringSubmatch(dialog)
			if match == nil {
				t.Fatalf("the sheet is not named by a title; dialog = %q", dialog)
			}
			if strings.Contains(dialog, "aria-label=") {
				t.Errorf("a fixed aria-label still names the sheet; dialog = %q", dialog)
			}
			heading := regexp.MustCompile(`<h2 [^>]*id="` + regexp.QuoteMeta(match[1]) + `"[^>]*>`).FindString(html)
			if heading == "" {
				t.Fatalf("the sheet names id %q and no heading carries it; html = %q", match[1], html)
			}
			if strings.Count(html, `id="`+match[1]+`"`) != 1 {
				t.Errorf("id %q is not unique on the sheet; html = %q", match[1], html)
			}
			if graph.SectionID(match[1]) == match[1] {
				t.Errorf("title id %q is inside the authored section namespace", match[1])
			}
			if !strings.Contains(html, `data-title="は"`) {
				t.Errorf("a titled concept lost its title; html = %q", html)
			}
			if !strings.Contains(html, `data-title="`+tc.want+`"`) {
				t.Errorf("an untitled concept has no neutral name %q; html = %q", tc.want, html)
			}
		})
	}
}

// TestAPressNamesAnOverlayThePageAnswersFirst keeps every invoker target
// unique even when authored headings use the old chrome names. The chrome owns
// its own namespace; rendering must preserve the author's section addresses.
func TestAPressNamesAnOverlayThePageAnswersFirst(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	var buf bytes.Buffer
	const headings = "## Search dialog\n\n## Concept sheet\n\n## Header fold\n\n## Kbd help\n"
	view := renderedNoteView(t, model, "Writing/lessons/go/L01.md", headings)
	for _, id := range []string{"search-dialog", "concept-sheet", "header-fold", "kbd-help"} {
		if !strings.Contains(view.BodyHTML, `id="`+id+`"`) {
			t.Errorf("authored section address %q was changed or omitted", id)
		}
	}
	page := Note(view, recordedChrome())
	if err := page.Render(t.Context(), &buf); err != nil {
		t.Fatalf("render note page: %v", err)
	}
	html := buf.String()

	body := strings.Index(html, "<main")
	if body < 0 {
		t.Fatal("the page has no reading region, so there is nothing for an overlay to be ahead of")
	}
	named := regexp.MustCompile(`(?:commandfor|popovertarget)="([^"]+)"`).FindAllStringSubmatch(html, -1)
	if len(named) == 0 {
		t.Fatal("no press on this page names a surface, so this test read a page it cannot ask anything of")
	}
	for _, match := range named {
		if graph.SectionID(match[1]) == match[1] {
			t.Errorf("overlay target %q is inside the authored section namespace", match[1])
		}
		id := `id="` + match[1] + `"`
		first := strings.Index(html, id)
		switch {
		case first < 0:
			t.Errorf("a press names %q and the page carries no surface by that name", match[1])
		case strings.Count(html, id) != 1:
			t.Errorf("the page carries %d surfaces named %q, and a press reaches whichever comes first", strings.Count(html, id), match[1])
		case first > body:
			t.Errorf("the surface named %q is drawn after the note's own words, where a heading folding to that name would answer the press before it does", match[1])
		}
	}
}

// TestTheFuriganaSwitchIsDrawnOnlyWhereThePageHasFurigana holds the header to
// the page it sits over. A note whose rendered body carries a reading gets the
// switch; a note without one does not, and neither does a page that renders no
// note at all. The switch stays reachable on the settings page either way, so
// a reader who turned readings off is never left without a way back.
func TestTheFuriganaSwitchIsDrawnOnlyWhereThePageHasFurigana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "a note with readings", body: `<p><ruby>漢字<rt>かんじ</rt></ruby></p>`, want: true},
		{name: "a note with readings in a language", body: `<p><ruby lang="ja">漢字<rp>(</rp><rt lang="ja">かんじ</rt><rp>)</rp></ruby></p>`, want: true},
		{name: "a note without readings", body: `<p>漢字</p>`},
		{name: "a note that only writes about the tag", body: `<p><code>&lt;rt&gt;</code></p>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			view := NoteView{Title: "T", RelPath: "a.md", BodyHTML: tt.body}
			if err := Note(view, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "data-ruby-toggle"); got != tt.want {
				t.Errorf("the header draws the furigana switch = %v, want %v", got, tt.want)
			}
		})
	}
}
