package search

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// resultLink is the address on every rendered hit, which is the one thing that
// identifies a row across two responses: the title and the excerpt of two
// notes can read the same, and the address of two notes cannot.
var resultLink = regexp.MustCompile(`<a class="y-result" href="([^"]*)"`)

// pagedAnswer is a query whose answer runs to more than two pages and does not
// divide evenly, so the last page is short and an off-by-one at either end of
// the window has somewhere to show.
const pagedAnswer = 2*searchPageSize + 5

// TestEveryHitIsOnExactlyOnePage walks the whole answer a page at a time and
// holds the pages against the undivided listing: together they have to be it,
// in its order, with nothing shown twice and nothing left out. A window that
// starts or ends one row wide leaves a hit on two pages or on none, and either
// is invisible from a single page.
func TestEveryHitIsOnExactlyOnePage(t *testing.T) {
	t.Parallel()

	h := pagedHandler(t, pagedAnswer)
	whole := pageHits(t, h, "all")
	if len(whole) != pagedAnswer {
		t.Fatalf("the undivided listing holds %d hits, want %d", len(whole), pagedAnswer)
	}
	if repeated := repeats(whole); len(repeated) > 0 {
		t.Fatalf("the fixture itself repeats %v, so nothing below could tell a repeat from the fixture", repeated)
	}

	lastPage := (pagedAnswer + searchPageSize - 1) / searchPageSize
	var walked []string
	for number := 1; number <= lastPage; number++ {
		hits := pageHits(t, h, strconv.Itoa(number))
		want := searchPageSize
		if number == lastPage {
			want = pagedAnswer - (lastPage-1)*searchPageSize
		}
		if len(hits) != want {
			t.Errorf("page %d holds %d hits, want %d", number, len(hits), want)
		}
		walked = append(walked, hits...)
	}
	if repeated := repeats(walked); len(repeated) > 0 {
		t.Errorf("these hits are on more than one page: %v", repeated)
	}
	if diff := cmp.Diff(whole, walked); diff != "" {
		t.Errorf("walking the pages is not the undivided listing (-whole +walked):\n%s", diff)
	}
}

// TestTheLastPageNamesItsOwnRows holds the sentence over the rows to the rows
// under it. The last page is the one that can be wrong while every other page
// reads correctly, because it is the only one that is not a full page.
func TestTheLastPageNamesItsOwnRows(t *testing.T) {
	t.Parallel()

	h := pagedHandler(t, pagedAnswer)
	lastPage := (pagedAnswer + searchPageSize - 1) / searchPageSize
	body := searchPage(t, h, strconv.Itoa(lastPage))

	first := (lastPage-1)*searchPageSize + 1
	want := fmt.Sprintf("第 %d–%d 筆，共 %d 筆", first, pagedAnswer, pagedAnswer)
	if !strings.Contains(body, want) {
		t.Errorf("the last page does not say %q", want)
	}
	// Counted off the page itself rather than taken from the sentence, so the
	// two have to agree about the same rows rather than about each other.
	if got := len(resultLink.FindAllString(body, -1)); got != pagedAnswer-first+1 {
		t.Errorf("the sentence names %d rows and the page draws %d", pagedAnswer-first+1, got)
	}
}

// TestTheTallyIsTheAnswerNotThePage holds the number every division beside the
// rows is answerable to. It is the whole answer's on every page: a tally that
// followed the page would make each page a different search.
func TestTheTallyIsTheAnswerNotThePage(t *testing.T) {
	t.Parallel()

	h := pagedHandler(t, pagedAnswer)
	want := fmt.Sprintf(`data-result-count="%d"`, pagedAnswer)
	for _, asked := range []string{"", "1", "2", "3", "all"} {
		if body := searchPage(t, h, asked); !strings.Contains(body, want) {
			t.Errorf("the page at page=%q does not carry %s", asked, want)
		}
	}
}

// TestAnUnaskablePageAnswersWithTheFirst holds the closed set. A word the page
// cannot read, a page the answer does not have, a number nobody could have
// reached — each is answered with the listing rather than with an argument
// about the request, the way an unreadable ordering is.
func TestAnUnaskablePageAnswersWithTheFirst(t *testing.T) {
	t.Parallel()

	h := pagedHandler(t, pagedAnswer)
	first := pageHits(t, h, "1")
	for _, asked := range []string{"", "0", "-3", "1.5", "two", "99", "9999999999999999999", " 1"} {
		if diff := cmp.Diff(first, pageHits(t, h, asked)); diff != "" {
			t.Errorf("page=%q is not the first page (-first +asked):\n%s", asked, diff)
		}
	}
}

// TestTheUndividedListingCarriesNoStrip is the other half of the print
// decision: the whole listing is one page, so there is nothing to step to and
// no strip is drawn over it.
func TestTheUndividedListingCarriesNoStrip(t *testing.T) {
	t.Parallel()

	h := pagedHandler(t, pagedAnswer)
	whole := searchPage(t, h, "all")
	if strings.Contains(whole, "y-pager") {
		t.Error("the undivided listing draws a strip of page links over a listing with one page")
	}
	if !strings.Contains(searchPage(t, h, "1"), `href="/search?page=all&amp;q=needle"`) {
		t.Error("a divided listing offers no way to the undivided one, so nothing prints whole")
	}
}

// TestAShortAnswerIsNotDivided holds the strip off a listing that fits: a
// single page number with nothing either side of it is a control that cannot
// be used.
func TestAShortAnswerIsNotDivided(t *testing.T) {
	t.Parallel()

	h := pagedHandler(t, searchPageSize)
	body := searchPage(t, h, "1")
	if strings.Contains(body, "y-pager") {
		t.Errorf("an answer of exactly %d hits draws a strip", searchPageSize)
	}
	if want := fmt.Sprintf("共 %d 筆", searchPageSize); !strings.Contains(body, want) {
		t.Errorf("an undivided answer does not say %q", want)
	}
}

// TestTheStripMarksThePageBeingRead holds the one mark a reader arriving by ear
// has: which of the numbers is where they are.
func TestTheStripMarksThePageBeingRead(t *testing.T) {
	t.Parallel()

	body := searchPage(t, pagedHandler(t, pagedAnswer), "2")
	if got := strings.Count(body, `aria-current="page"`); got != 1 {
		t.Errorf("the strip marks %d pages as the one being read, want 1", got)
	}
	current := regexp.MustCompile(`<a class="y-pager__page" href="([^"]*)" aria-current="page">(\d+)</a>`)
	match := current.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("no page link carries the mark; body holds %q", strip(body))
	}
	if match[2] != "2" {
		t.Errorf("page %s is marked as the one being read on page 2", match[2])
	}
	if match[1] != "/search?page=2&amp;q=needle" {
		t.Errorf("the marked page links to %q, which is not the page being read", match[1])
	}
}

// pagedHandler answers "needle" with hits notes, each at its own address.
func pagedHandler(t *testing.T, hits int) *Handler {
	t.Helper()
	docs := make([]lexical.Document, 0, hits)
	for i := range hits {
		docs = append(docs, lexical.Document{
			RelPath:   fmt.Sprintf("Notes/n%03d.md", i),
			Title:     fmt.Sprintf("Note %03d", i),
			PlainText: "paged needle body",
		})
	}
	idx := lexical.NewIndex(docs, validArtifactPolicy(t))
	return NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler))
}

// searchPage is the whole page one request gets.
func searchPage(t *testing.T, h *Handler, asked string) string {
	t.Helper()
	values := url.Values{"q": {"needle"}}
	if asked != "" {
		values.Set("page", asked)
	}
	address := "/search?" + values.Encode()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	rr := httptest.NewRecorder()
	h.search(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", address, rr.Code)
	}
	return rr.Body.String()
}

// pageHits is the address of every hit one page drew, in the order it drew
// them.
func pageHits(t *testing.T, h *Handler, asked string) []string {
	t.Helper()
	var out []string
	for _, match := range resultLink.FindAllStringSubmatch(searchPage(t, h, asked), -1) {
		out = append(out, match[1])
	}
	if len(out) == 0 {
		t.Fatalf("page=%q drew no hits at all, so nothing can be read off it", asked)
	}
	return out
}

// repeats names every value that appears more than once.
func repeats(values []string) []string {
	seen := make(map[string]int, len(values))
	for _, value := range values {
		seen[value]++
	}
	var out []string
	for value, times := range seen {
		if times > 1 {
			out = append(out, value)
		}
	}
	slices.Sort(out)
	return out
}

// strip cuts a page down to the strip, for a failure message that is readable.
func strip(body string) string {
	_, rest, found := strings.Cut(body, `<nav class="y-pager"`)
	if !found {
		return "no strip"
	}
	rest, _, _ = strings.Cut(rest, "</nav>")
	return rest
}

// The page number this interface will read out of a request is bounded so it
// can be multiplied by a page size, and the bound belongs to the parse rather
// than to each listing that multiplies.
func TestThePageNumberIsBoundedBeforeItIsMultiplied(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		asked string
		want  int
	}{
		{"1", searchPageSize},
		{"7", 7 * searchPageSize},
		{"1000000", 1_000_000 * searchPageSize},
		{"1000001", searchPageSize},
		{"all", -1},
	} {
		if got := searchLimit(pages.ParsePageNumber(tt.asked)); got != tt.want {
			t.Errorf("searchLimit(page=%q) = %d, want %d", tt.asked, got, tt.want)
		}
	}
}
