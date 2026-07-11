package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
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

func homeSearchSection(t *testing.T, html string) string {
	t.Helper()

	marker := `data-home-block="search"`
	markerAt := strings.Index(html, marker)
	if markerAt < 0 {
		t.Fatalf("Home() has no search block; html = %q", html)
	}
	start := strings.LastIndex(html[:markerAt], "<section")
	end := strings.Index(html[markerAt:], "</section>")
	if start < 0 || end < 0 {
		t.Fatalf("Home() search block is incomplete; html = %q", html)
	}
	return html[start : markerAt+end]
}
