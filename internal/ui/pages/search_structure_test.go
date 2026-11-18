package pages

import (
	"bytes"
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
		{name: "hits arrive as a set", view: SearchView{Query: "a", Results: hits, Total: 2}, wantList: true, wantItems: 2, wantCount: "2 篇"},
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

			if !strings.Contains(html, `class="y-title y-searchpage__title"`) {
				t.Errorf("the page has no heading to arrive on; html = %q", html)
			}
			if got := strings.Contains(html, `<ol class="y-found"`); got != tt.wantList {
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

// The title belongs to the answer: it is the query in brackets, rewritten with
// each answer, so a heading that stayed would name a search no longer on screen.
func TestSearchResultsFragmentIsTitledByTheQuery(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	results := []SearchResult{{Title: "First", RelPath: "Concepts/first.md"}}
	if err := SearchResults(SearchView{Query: "a", Results: results, Total: len(results)}, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render search results: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `<h1 class="y-title y-searchpage__title">「a」</h1>`) {
		t.Errorf("the fragment is not titled by its query; html = %q", html)
	}
	if !strings.Contains(html, `<ol class="y-found"`) {
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
			RelPath:   "Lessons/Building yomihon from source.md",
			Title:     "Building yomihon from source",
			WhereRuns: []SnippetRun{{Text: "Lessons"}},
			AliasRuns: []SnippetRun{
				{Text: "building "},
				{Text: "yomihon", Hit: true},
				{Text: " from source"},
			},
			TagRuns: []SnippetRun{
				{Text: "safe <"},
				{Text: "yomihon", Hit: true},
				{Text: "> & text"},
			},
			Lead: []SnippetRun{
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
		`<span class="y-result__tag">標籤： safe &lt;<mark>yomihon</mark>&gt; &amp; text</span>`,
		`<span class="y-result__alias">又稱 building <mark>yomihon</mark> from source</span>`,
		`<span class="y-found__lead">how to build <mark>yomihon</mark> from source</span>`,
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

// TestASearchResultMarksOnlyItsDrafts holds the one thing a hit says about the
// state of its note: that the author has not finished it. A finished note says
// nothing, so a page of results carries the word only on the rows that earn it,
// drawn as the mark every other list draws it in.
func TestASearchResultMarksOnlyItsDrafts(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		var buf bytes.Buffer
		view := SearchView{Query: "a", Total: 2, Results: []SearchResult{
			{Title: "Unfinished", RelPath: "a.md", Draft: true},
			{Title: "Finished", RelPath: "b.md"},
		}}
		if err := Search(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render search page: %v", err)
		}
		rows := strings.Split(buf.String(), `<a class="y-found__item"`)[1:]
		if len(rows) != 2 {
			t.Fatalf("%s page draws %d hits, want 2", lang, len(rows))
		}
		mark := `<span class="ui-status">` + wording.Draft.In(lang) + `</span>`
		if !strings.Contains(rows[0], mark) {
			t.Errorf("%s: the draft hit does not carry %q: %s", lang, mark, rows[0])
		}
		if strings.Contains(rows[1], "ui-status") || strings.Contains(rows[1], wording.Draft.In(lang)) {
			t.Errorf("%s: a finished hit says something about its state: %s", lang, rows[1])
		}
	}
}
