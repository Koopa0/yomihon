package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// When a search finds nothing, the page advises fewer or different words and
// teaches no lifecycle filter: the reading pages hold no status to narrow by.
func TestEmptySearchAdvisesWithoutAStatusFilter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := SearchResults(SearchView{Query: "沒有這個詞"}, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("SearchResults(...).Render() error = %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "請試較少或不同的詞") {
		t.Errorf("SearchResults lost the advice that always applies; html = %q", got)
	}
	if strings.Contains(got, "status:") {
		t.Errorf("SearchResults still teaches a status filter; html = %q", got)
	}
}

// When the empty page has loosened searches to offer, each arrives as a real
// link with its count — and when it has none, the section is absent rather
// than an empty promise.
func TestEmptySearchOffersStepBacks(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	steps := []SearchStepBack{{Query: "20 mg", Count: 1}}
	if err := SearchResults(SearchView{Query: "20mg", StepBacks: steps}, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("SearchResults(...).Render() error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{"改用較短的詞", `href="/search?q=20+mg"`, "1 筆"} {
		if !strings.Contains(got, want) {
			t.Errorf("SearchResults() with step-backs missing %q in:\n%s", want, got)
		}
	}

	buf.Reset()
	if err := SearchResults(SearchView{Query: "20mg"}, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("SearchResults(...).Render() error = %v", err)
	}
	if strings.Contains(buf.String(), "改用較短的詞") {
		t.Error("SearchResults() with no step-backs still renders the offer heading")
	}
}

// TestTheFilterListReadsAsASentenceInBothLanguages holds the separator between
// the filter keys the page offers. It was an ideographic comma whatever the
// reader had chosen, so an English reader was handed "status:、type:、path:".
func TestTheFilterListReadsAsASentenceInBothLanguages(t *testing.T) {
	t.Parallel()

	view := SearchView{FilterKeys: []string{"status", "type"}, UnknownFilterKeys: []string{"tag", "kind"}}
	for lang, want := range map[wording.Lang]string{
		wording.ZhHant: "、",
		wording.En:     ", ",
	} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := SearchResults(view, lang).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, "tag"+want+"kind") {
				t.Errorf("the unrecognised filters are not joined with %q in %q; html = %q", want, lang, html)
			}
			// The offered keys sit in separate elements, so the separator is
			// asserted by its presence rather than by its neighbours: an
			// English page has no business carrying an ideographic comma at
			// all, and a Traditional Chinese one is written with it.
			if got := strings.Contains(html, "、"); got != (lang == wording.ZhHant) {
				t.Errorf("the ideographic comma appears = %t in %q; html = %q", got, lang, html)
			}
		})
	}
}
