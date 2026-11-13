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

func TestInlineReadingAidsUseChromeLanguage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{
		Language:   "ja",
		Diagnostic: "invalid frontmatter",
	}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<div class="y-inlineaids" lang="zh-Hant">`) {
		t.Errorf("inline reading aids do not restore the chrome language inside a Japanese article: %q", html)
	}
}

// TestSchemaNoticesStandInReadingColumn pins where unknown-field guidance
// renders: once, above the prose. Hiding it inside the right rail made it
// vanish at the widths where that rail is gone.
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
	if strings.Contains(article, `aside class="y-rail-right"`) {
		t.Fatal("the reading column should not contain the right rail")
	}
	railStart := strings.Index(html, `<aside class="y-rail-right"`)
	if railStart >= 0 && strings.Contains(html[railStart:], notice) {
		t.Errorf("the schema notices block still renders inside the right rail")
	}
	proseStart := strings.Index(article, `<div class="y-prose">`)
	noticeStart := strings.Index(article, notice)
	if proseStart < 0 || noticeStart < 0 || noticeStart > proseStart {
		t.Errorf("the schema notices block must sit above the prose; notice@%d prose@%d", noticeStart, proseStart)
	}
}

// TestInlineDiagnosticsFoldAboveTheProse pins the placement, not the contents.
// At widths where the right rail is gone this block sits between the title and
// the first sentence, so an open list of findings was the last thing the page
// said before the prose it was about — worst on the notes that have the most of
// them. It folds like the contents above it, so the count is still stated and
// nothing is hidden from a reader who wants it.
func TestInlineDiagnosticsFoldAboveTheProse(t *testing.T) {
	t.Parallel()

	view := NoteView{
		Title:   "DDIA",
		RelPath: "Sources/DDIA.md",
		RenderDiagnostics: []render.Diagnostic{
			{Kind: render.DiagWikilinkBroken, Target: "Relational Model", Message: "does not resolve"},
			{Kind: render.DiagWikilinkBroken, Target: "Leader Election", Message: "does not resolve"},
		},
	}
	var buf bytes.Buffer
	if err := inlineAids(view, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render inlineAids: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, "y-diaglist") {
		t.Fatalf("the diagnostics did not render at all:\n%s", got)
	}
	before, _, _ := strings.Cut(got, "y-diaglist")
	if !strings.Contains(before, "<summary") {
		t.Errorf("the diagnostics are not inside a disclosure; they open above the prose:\n%s", got)
	}
	if !strings.Contains(got, "筆記狀況") {
		t.Errorf("the disclosure does not name what it holds:\n%s", got)
	}
	if !strings.Contains(got, ">2<") {
		t.Errorf("the count is not stated on the closed disclosure:\n%s", got)
	}
}

// TestNarrowAidsAreOneClosedRowAndTheNotesBesideFollowTheText holds the shape
// the reading aids take where the right rail is hidden. Four folds stacked
// between the title and the first sentence pushed the prose a third of a screen
// down, so every aid is named in one closed row and the two a reader looks for
// once finished — the note to read beside this one and the notes citing it —
// stand after the text instead, where the row leads down to them. The rail
// carries the same blocks at the widths that show it, so they are not added to
// the article twice there; that half is held by the stylesheet and the browser.
func TestNarrowAidsAreOneClosedRowAndTheNotesBesideFollowTheText(t *testing.T) {
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

	rowStart := strings.Index(article, `<div class="y-inlineaids"`)
	prose := strings.Index(article, `<div class="y-prose">`)
	related := strings.Index(article, `<div class="y-related"`)
	steps := strings.Index(article, `<nav class="y-steps`)
	if rowStart < 0 || prose < 0 || related < 0 || steps < 0 {
		t.Fatalf("row@%d prose@%d related@%d steps@%d; every one of them has to render:\n%s", rowStart, prose, related, steps, article)
	}
	if rowStart >= prose || prose >= related || related >= steps {
		t.Fatalf("row@%d prose@%d related@%d steps@%d; want the row, the text, the notes beside it, then the way onward", rowStart, prose, related, steps)
	}
	row := article[rowStart:prose]
	if n := strings.Count(row, "<details"); n != 1 {
		t.Errorf("the aids above the prose are %d disclosures, want one row:\n%s", n, row)
	}
	if strings.Contains(row, "<details open") || strings.Contains(row, " open>") {
		t.Errorf("the row of aids is drawn open, so it stands between the title and the text:\n%s", row)
	}
	summary, _, _ := strings.Cut(row, "</summary>")
	for _, name := range []string{wording.OnThisPage.In(wording.En), wording.NoteHealth.In(wording.En), wording.BasedOn.In(wording.En), wording.DeclaredBy.In(wording.En), wording.CompareOffer.In(wording.En), wording.CitedBy.In(wording.En)} {
		if !strings.Contains(summary, name) {
			t.Errorf("the closed row does not name %q:\n%s", name, summary)
		}
	}
	if strings.Contains(row, `class="y-pair"`) || strings.Contains(row, `class="y-citedby"`) {
		t.Errorf("the note beside this one or the notes citing it still stand above the text:\n%s", row)
	}
	after := article[related:steps]
	for _, block := range []string{`class="y-pair"`, `class="y-citedby"`} {
		if !strings.Contains(after, block) {
			t.Errorf("%s is not between the text and the way onward:\n%s", block, after)
		}
	}
	for _, id := range []string{view.relatedPairID(), view.relatedCitedID()} {
		if !strings.Contains(row, `href="#`+id+`"`) {
			t.Errorf("the row does not lead down to #%s:\n%s", id, row)
		}
		if !strings.Contains(after, `id="`+id+`"`) {
			t.Errorf("nothing after the text answers to #%s:\n%s", id, after)
		}
	}
}

// TestNoteMetarowOmitsAMissingDate holds the one dateless case: a view built
// with no date at all renders no time element and neither label, rather than
// an empty claim. Every served note has a scanned file behind it, so the case
// is a generation that could not produce the file's identity — and a blank
// beats a fabricated moment.
func TestNoteMetarowOmitsAMissingDate(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{Title: "A", RelPath: "Writing/A.md"}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render note: %v", err)
	}
	html := buf.String()
	at := strings.Index(html, `class="y-metarow"`)
	if at < 0 {
		t.Fatalf("the page carries no metarow; html = %q", html)
	}
	metarow := html[at:]
	if end := strings.Index(metarow, "</details>"); end >= 0 {
		metarow = metarow[:end]
	}
	if strings.Contains(metarow, "<time") {
		t.Errorf("a dateless view still renders a time element; metarow = %q", metarow)
	}
	for _, label := range []string{"更新於", "檔案變更於", "Updated", "File changed"} {
		if strings.Contains(metarow, label) {
			t.Errorf("a dateless view still carries the label %q; metarow = %q", label, metarow)
		}
	}
}

// TestNoteFileRowKeepsItsAddressBehindAClosedSummary holds the opening the
// reading page is built to give: a title, one quiet line, then the prose. The
// line is a native disclosure that starts closed and carries the two facts a
// reader wants at a glance; the note's address and the two doors out of the
// page wait inside it.
//
// A closed disclosure keeps its content in the document, so the doors are
// asked for by the exact address each one resolves to. Nothing here depends on
// a script: opening the line is the browser's own behaviour.
func TestNoteFileRowKeepsItsAddressBehindAClosedSummary(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{
		Title:        "L01",
		Type:         "lesson",
		RelPath:      "Writing/lessons/go/L01.md",
		Updated:      "2026-07-10",
		UpdatedAt:    "2026-07-10",
		ObsidianHref: "obsidian://open?path=/vault/Writing/lessons/go/L01.md",
	}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render note: %v", err)
	}
	html := buf.String()

	at := strings.Index(html, `<details class="y-metarow"`)
	if at < 0 {
		t.Fatalf("the file row is not a disclosure; html = %q", html)
	}
	row, _, closed := strings.Cut(html[at:], "</details>")
	if !closed {
		t.Fatalf("the file row never closes; html = %q", html)
	}
	// Read the attribute out of the opening tag rather than matching a whole
	// tag as a substring: open arrives wherever the serializer puts it, and a
	// pattern that fixes its neighbours matches nothing in any regression.
	tag, _, tagClosed := strings.Cut(row, ">")
	if !tagClosed {
		t.Fatalf("the disclosure's opening tag never closes; row = %q", row)
	}
	if strings.Contains(tag, " open") {
		t.Errorf("the file row starts expanded, so the opening column is not calm; tag = %q", tag)
	}

	summary, body, split := strings.Cut(row, "</summary>")
	if !split {
		t.Fatalf("the file row has no summary, so nothing says what it holds; row = %q", row)
	}
	for _, want := range []string{
		`<span class="ui-type">lesson</span>`,
		`<time datetime="2026-07-10">2026-07-10</time>`,
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("the closed line is missing the fact %q; summary = %q", want, summary)
		}
	}
	for _, want := range []string{
		`<dd><a href="/raw/Writing/lessons/go/L01.md">Writing/lessons/go/L01.md</a></dd>`,
		`href="obsidian://open?path=/vault/Writing/lessons/go/L01.md"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("opening the file row would not reach %q; body = %q", want, body)
		}
	}
	for _, moved := range []string{"/raw/", "obsidian://", "Writing/lessons/go/L01.md"} {
		if strings.Contains(summary, moved) {
			t.Errorf("%q is still on the closed line; summary = %q", moved, summary)
		}
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
