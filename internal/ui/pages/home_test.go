package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestHomeSearchHasNoAutofocusAttribute(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Home(HomeView{}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render Home: %v", err)
	}
	section := homeSearchSection(t, buf.String())
	start := strings.Index(section, `<form class="y-homesearch"`)
	if start < 0 {
		t.Fatalf("Home() has no search form; section = %q", section)
	}
	end := strings.Index(section[start:], "</form>")
	if end < 0 {
		t.Fatalf("Home() search form has no closing tag; section = %q", section)
	}
	form := section[start : start+end]
	if strings.Contains(form, "autofocus") {
		t.Errorf("Home() search form contains autofocus; form = %q", form)
	}
}

func TestHomeSearchIsPlainGETForm(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Home(HomeView{}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render Home: %v", err)
	}
	section := homeSearchSection(t, buf.String())
	for _, want := range []string{
		`method="get" action="/search"`,
		`name="q"`,
		`type="submit"`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("Home() search is missing %q", want)
		}
	}
	for _, absent := range []string{
		`data-live-search`,
		`aria-live`,
		`data-result-count`,
	} {
		if strings.Contains(section, absent) {
			t.Errorf("Home() plain GET search contains live-search marker %q", absent)
		}
	}
}

// TestShelfCountSpansCarryTheUnitInTheDOM locks #295: a shelf count is a
// role-less span, so the unit lives in the DOM rather than on aria-label.
func TestShelfCountSpansCarryTheUnitInTheDOM(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	statuses := NewStatusDistribution(
		[]LifecycleItem{{Name: "draft", Count: 3, Href: statusHref("draft")}},
		[]LifecycleItem{{Count: 2, Label: wording.NoStatusStated.In(wording.ZhHant)}},
		false, wording.ZhHant,
	)
	if err := FolderIndex(ListIndexView{}, RecentBlock{}, statuses, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render FolderIndex: %v", err)
	}
	html := buf.String()
	for _, class := range []string{`class="y-homechip__count"`, `class="y-homeunstated__count"`} {
		at := strings.Index(html, class)
		if at < 0 {
			t.Fatalf("shelf is missing %s", class)
		}
		start := strings.LastIndex(html[:at], "<span")
		close := strings.Index(html[at:], "</span></span>")
		if start < 0 || close < 0 {
			t.Fatalf("count span %s is incomplete", class)
		}
		span := html[start : at+close+len("</span></span>")]
		if strings.Contains(span, "aria-label") {
			t.Errorf("role-less count span still carries aria-label: %q", span)
		}
		if !strings.Contains(span, `<span class="y-offscreen"> 篇筆記</span>`) {
			t.Errorf("count span does not put the unit in the DOM: %q", span)
		}
	}
}

func homeSearchSection(t *testing.T, html string) string {
	t.Helper()

	// The desk's search is one row rather than a block, so it is the search
	// landmark itself that carries the marker.
	marker := `data-home-block="search"`
	markerAt := strings.Index(html, marker)
	if markerAt < 0 {
		t.Fatal("Home() has no search row")
	}
	start := strings.LastIndex(html[:markerAt], "<search")
	end := strings.Index(html[markerAt:], "</search>")
	if start < 0 || end < 0 {
		t.Fatal("Home() search row is incomplete")
	}
	return html[start : markerAt+end]
}
