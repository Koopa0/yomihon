package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestHomeSearchStartsAtTopWithGETFallback(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := Home(HomeView{}, layouts.Chrome{}).Render(t.Context(), &out); err != nil {
		t.Fatalf("Home().Render: %v", err)
	}
	page := out.String()
	formStart := strings.Index(page, `<form class="y-homesearch"`)
	if formStart < 0 {
		t.Fatal("Home search form is absent")
	}
	formEnd := strings.Index(page[formStart:], "</form>")
	if formEnd < 0 {
		t.Fatal("Home search form has no closing tag")
	}
	homeSearch := page[formStart : formStart+formEnd]
	for _, want := range []string{
		`<form class="y-homesearch" method="get" action="/search" role="search">`,
		`<input type="search" name="q"`,
		`<button type="submit" class="y-xbtn">Search</button>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("Home search is missing %q", want)
		}
	}
	if strings.Contains(homeSearch, "autofocus") {
		t.Error("Home search contains autofocus, which can move the initial scroll position")
	}
}
