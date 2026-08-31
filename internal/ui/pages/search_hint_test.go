package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// When a search finds nothing, the page offers a way to narrow it. Offering to
// narrow by lifecycle status only helps where statuses exist: a reader whose
// folder carries no contract followed that suggestion, matched nothing again,
// and was handed the identical sentence a second time — the page teaching a
// filter it had already made useless.
func TestEmptySearchOffersLifecycleFilterOnlyWhereLifecycleExists(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		governed   bool
		wantOffer  bool
		wantAlways string
	}{
		{name: "a governed vault has statuses to filter by", governed: true, wantOffer: true, wantAlways: "請嘗試較少或不同的詞"},
		{name: "a plain folder has none", governed: false, wantOffer: false, wantAlways: "請嘗試較少或不同的詞"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := SearchResults("沒有這個詞", nil, 0, "", tt.governed, nil, wording.ZhHant).Render(t.Context(), &buf); err != nil {
				t.Fatalf("SearchResults(...).Render() error = %v", err)
			}
			got := buf.String()
			if !strings.Contains(got, tt.wantAlways) {
				t.Errorf("SearchResults(governed=%v) lost the advice that always applies (%q)", tt.governed, tt.wantAlways)
			}
			if offered := strings.Contains(got, "status:draft"); offered != tt.wantOffer {
				t.Errorf("SearchResults(governed=%v) offered the status filter = %v, want %v", tt.governed, offered, tt.wantOffer)
			}
		})
	}
}

// When the empty page has loosened searches to offer, each arrives as a real
// link with its count — and when it has none, the section is absent rather
// than an empty promise.
func TestEmptySearchOffersStepBacks(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	steps := []SearchStepBack{{Query: "20 mg", Count: 1}}
	if err := SearchResults("20mg", nil, 0, "", false, steps, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("SearchResults(...).Render() error = %v", err)
	}
	got := buf.String()
	for _, want := range []string{"退一步找", `href="/search?q=20+mg"`, "1 筆"} {
		if !strings.Contains(got, want) {
			t.Errorf("SearchResults() with step-backs missing %q in:\n%s", want, got)
		}
	}

	buf.Reset()
	if err := SearchResults("20mg", nil, 0, "", false, nil, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("SearchResults(...).Render() error = %v", err)
	}
	if strings.Contains(buf.String(), "退一步找") {
		t.Error("SearchResults() with no step-backs still renders the offer heading")
	}
}
