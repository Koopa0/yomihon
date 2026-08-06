package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
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
		{name: "hits arrive as a set", view: SearchView{Query: "a", Results: hits}, wantList: true, wantItems: 2, wantCount: "共 2 筆"},
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
	if err := SearchResults("a", results, "", false, nil).Render(t.Context(), &buf); err != nil {
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
