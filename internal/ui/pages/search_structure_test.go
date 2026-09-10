package pages

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// Arriving here by ear, the first thing announced was a text field with no
// word for where it sat, and the hits below it were loose links rather than a
// set — no start, no count, no end. Both are page structure, so both are
// asserted on the rendered page rather than inferred from the template.
func TestSearchPageHeadingAndResultList(t *testing.T) {
	t.Parallel()

	hits := []SearchResult{
		{Title: "First", RelPath: "Concepts/first.md"},
		{Title: "Second", RelPath: "Concepts/second.md"},
	}
	tests := []struct {
		name      string
		view      SearchView
		wantList  bool
		wantItems int
		wantCount string
	}{
		{name: "hits arrive as a set", view: SearchView{Query: "a", Results: hits, Total: 2}, wantList: true, wantItems: 2, wantCount: "共 2 筆"},
		// The two below are the control: a page that always carried a list
		// would pass the first row while proving nothing, and a heading tucked
		// inside the results branch would pass it too.
		{name: "nothing matched", view: SearchView{Query: "沒有這個詞"}},
		{name: "nothing asked yet", view: SearchView{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Search(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render search page: %v", err)
			}
			html := buf.String()

			if !strings.Contains(html, `<h1 class="y-title">搜尋</h1>`) {
				t.Errorf("the page has no heading to arrive on; html = %q", html)
			}
			if got := strings.Contains(html, `<ol class="y-results"`); got != tt.wantList {
				t.Errorf("results are marked as a list = %v, want %v", got, tt.wantList)
			}
			if got := strings.Count(html, "<li>"); got != tt.wantItems {
				t.Errorf("list items = %d, want %d", got, tt.wantItems)
			}
			if tt.wantCount != "" && strings.Count(html, tt.wantCount) != 1 {
				t.Errorf("the page states %q %d times, want once", tt.wantCount, strings.Count(html, tt.wantCount))
			}
		})
	}
}

// The heading belongs to the page, not to the answer. This fragment is what a
// keystroke re-fetches and what the command palette shows, so a heading here
// would be rewritten on every letter typed and would give the palette a page
// title it is not a page for.
func TestSearchResultsFragmentCarriesNoHeading(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	results := []SearchResult{{Title: "First", RelPath: "Concepts/first.md"}}
	if err := SearchResults(SearchView{Query: "a", Results: results, Total: len(results)}, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render search results: %v", err)
	}
	html := buf.String()

	if strings.Contains(html, "<h1") {
		t.Errorf("the fragment carries a page heading; html = %q", html)
	}
	// The control: the fragment is the piece that holds the list, so an empty
	// render cannot satisfy the assertion above.
	if !strings.Contains(html, `<ol class="y-results"`) {
		t.Errorf("the fragment lost the result list; html = %q", html)
	}
}

// A row's path, alias and excerpt are each one string cut into the stretches
// the query matched and the stretches it did not. Rendered, they have to read
// back as that same string: a path shown as "Maps/ yomihon .md" names no file,
// and a reader comparing the row against their own folders is told the wrong
// thing. The multi-word case carries the finding, because a match that ends the
// string shows one stray space where a match with words on both sides shows
// two.
func TestSearchResultMarkedRunsIntroduceNoSpacing(t *testing.T) {
	t.Parallel()

	view := SearchView{
		Query: "yomihon",
		Total: 1,
		Results: []SearchResult{{
			RelPath: "Lessons/Building yomihon from source.md",
			Title:   "Building yomihon from source",
			PathRuns: []SnippetRun{
				{Text: "Lessons/Building "},
				{Text: "yomihon", Hit: true},
				{Text: " from source.md"},
			},
			AliasRuns: []SnippetRun{
				{Text: "building "},
				{Text: "yomihon", Hit: true},
				{Text: " from source"},
			},
			TopicRuns: []SnippetRun{
				{Text: "building "},
				{Text: "yomihon", Hit: true},
			},
			SnippetRuns: []SnippetRun{
				{Text: "how to build "},
				{Text: "yomihon", Hit: true},
				{Text: " from source"},
			},
		}},
	}
	var buf bytes.Buffer
	if err := SearchResults(view, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render search results: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		`<span class="y-result__meta">Lessons/Building <mark>yomihon</mark> from source.md`,
		`<span class="y-result__topic">主題: building <mark>yomihon</mark></span>`,
		`<span class="y-result__alias">別名: building <mark>yomihon</mark> from source</span>`,
		`<span class="y-result__snippet">how to build <mark>yomihon</mark> from source</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("a marked run was not written back as its own string:\nwant %q\n  in %s", want, html)
		}
	}
}

// Every answer carries the query it answers, hidden. The live search shows it
// when a refresh fails and the previous rows are left standing, so they stop
// reading as an answer to what the reader has since typed. Hidden is the
// server's whole part: with no script there is no failed refresh and no stale
// answer, so the page a reader without one sees is the page they saw before.
func TestSearchResultsCarryTheQueryTheyAnswer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		view     SearchView
		wantNote bool
	}{
		{
			name:     "an answer names its query",
			view:     SearchView{Query: "tortoise", Total: 1, Results: []SearchResult{{Title: "Alpha", RelPath: "Notes/alpha.md"}}},
			wantNote: true,
		},
		{
			// An answer to nothing cannot go stale, and a note naming an empty
			// query would say nothing a reader could use.
			name: "nothing asked yet",
			view: SearchView{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := SearchResults(tt.view, wording.ZhHant).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render search results: %v", err)
			}
			html := buf.String()

			note := `<p class="y-live-search__status" data-live-search-stale="tortoise" hidden>下方結果回答的是先前的查詢「tortoise」。</p>`
			if got := strings.Contains(html, note); got != tt.wantNote {
				t.Errorf("the answer carries the query it answers = %v, want %v; html = %s", got, tt.wantNote, html)
			}
			if got := strings.Contains(html, "data-live-search-stale"); got != tt.wantNote {
				t.Errorf("the stale-query marker is present = %v, want %v; html = %s", got, tt.wantNote, html)
			}
		})
	}
}

// TestSearchResultSourceLabelIsNotAHit: the fence mark lives on the row,
// not in the excerpt. An English query for "source" must still show the
// label as ordinary text beside the note's own Source:. MarkHits never
// sees the label; that half is locked in internal/search/source_test.go.
func TestSearchResultSourceLabelIsNotAHit(t *testing.T) {
	t.Parallel()

	view := SearchView{
		Query: "source",
		Total: 1,
		Results: []SearchResult{{
			RelPath:   "Notes/Fence.md",
			Title:     "Fence",
			FromFence: true,
			Snippet:   `direction: right Source: "source\nowns jobs close"`,
		}},
	}
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		var buf bytes.Buffer
		if err := SearchResults(view, lang).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render search results: %v", err)
		}
		html := buf.String()
		want := `<span class="y-result__source">` + wording.ResultSourceLabel.In(lang) + `</span>`
		if !strings.Contains(html, want) {
			t.Errorf("lang %s: missing unmarked source label %q in %s", lang, want, html)
		}
	}
}

// The palette and the page share one results fragment. The fragment prints a
// count so a reader without scripting still sees a number on /search; the
// dialog already announces the same number through its live status. Showing
// both is two voices for one fact, so the fragment's count is hidden there.
func TestSearchDialogStatesTheCountOnce(t *testing.T) {
	t.Parallel()

	hits := []SearchResult{{Title: "First", RelPath: "Concepts/first.md"}}
	view := SearchView{Query: "日本語", Results: hits, Total: 83}

	var pageBuf bytes.Buffer
	if err := Search(view, layouts.Chrome{}).Render(t.Context(), &pageBuf); err != nil {
		t.Fatalf("render search page: %v", err)
	}
	page := pageBuf.String()
	dialog, ok := cutElement(page, `<dialog class="y-searchdialog`, "</dialog>")
	if !ok {
		t.Fatal("the page has no command palette")
	}

	var fragBuf bytes.Buffer
	if err := SearchResults(view, wording.ZhHant).Render(t.Context(), &fragBuf); err != nil {
		t.Fatalf("render search results: %v", err)
	}
	fragment := fragBuf.String()

	const emptySlot = `<div class="y-searchresults" data-live-search-results data-result-count="0" aria-busy="false"></div>`
	if !strings.Contains(dialog, emptySlot) {
		t.Fatalf("the palette has no empty results slot to fill; dialog = %q", dialog)
	}
	filled := strings.Replace(dialog, emptySlot, fragment, 1)

	if !strings.Contains(filled, `data-live-search-status`) || !strings.Contains(filled, `role="status"`) || !strings.Contains(filled, `aria-live="polite"`) {
		t.Error("the palette lost the live status that announces the count")
	}
	if !strings.Contains(filled, `class="y-results__count"`) {
		t.Fatal("the fragment no longer carries a count, so hiding it inside the dialog proves nothing")
	}

	visible := 1
	if !dialogHidesFragmentCount(t) {
		visible++
	}
	if visible != 1 {
		t.Errorf("the palette states the count %d times, want once", visible)
	}

	if strings.Count(page, `class="y-results__count"`) != 1 {
		t.Errorf("the full search page states the fragment count %d times, want once", strings.Count(page, `class="y-results__count"`))
	}
	if !strings.Contains(page, "共 83 筆") {
		t.Error("the full search page lost its visible count")
	}
}

func dialogHidesFragmentCount(t *testing.T) bool {
	t.Helper()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	const opener = `.y-searchdialog .y-results__count {`
	css := string(source)
	at := strings.Index(css, opener)
	if at < 0 {
		return false
	}
	body := css[at+len(opener):]
	end := strings.IndexByte(body, '}')
	if end < 0 {
		t.Fatalf("rule %q is not closed", opener)
	}
	for decl := range strings.SplitSeq(body[:end], ";") {
		property, value, ok := strings.Cut(strings.TrimSpace(decl), ":")
		if ok && strings.TrimSpace(property) == "display" && strings.TrimSpace(value) == "none" {
			return true
		}
	}
	return false
}

func cutElement(html, open, close string) (string, bool) {
	start := strings.Index(html, open)
	if start < 0 {
		return "", false
	}
	end := strings.Index(html[start:], close)
	if end < 0 {
		return "", false
	}
	return html[start : start+end+len(close)], true
}
