package pages

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
)

// pagedReport is a report that runs to more than two pages and does not divide
// evenly, so the last page is short and a window one row wide at either end
// has somewhere to show.
const pagedReport = 2*healthPageSize + 7

// shapeFileCount reads the number of distinct files the shape line claims, off
// the line itself rather than out of the code that wrote it.
var shapeFileCount = regexp.MustCompile(`<span>(\d+)[^<]*</span></p>`)

// guideTally reads what the guide under the table says each kind of finding
// comes to.
var guideTally = regexp.MustCompile(`<span class="ui-navitem__count">(\d+)</span>`)

// TestEveryFindingIsOnExactlyOnePage walks the whole report a page at a time
// and holds the pages against the undivided table: together they have to be
// it, in its order, with no row shown twice and none left out.
func TestEveryFindingIsOnExactlyOnePage(t *testing.T) {
	t.Parallel()

	view := pagedHealthView(pagedReport)
	view.Page = AllPages
	whole := healthRowFiles(t, &view)
	if len(whole) != pagedReport {
		t.Fatalf("the undivided table holds %d rows, want %d", len(whole), pagedReport)
	}
	if len(slices.Compact(slices.Clone(whole))) != len(whole) {
		t.Fatal("the fixture repeats a file, so nothing below could tell a repeat from the fixture")
	}

	lastPage := (pagedReport + healthPageSize - 1) / healthPageSize
	var walked []string
	for number := 1; number <= lastPage; number++ {
		view.Page = PageNumber(number)
		rows := healthRowFiles(t, &view)
		want := healthPageSize
		if number == lastPage {
			want = pagedReport - (lastPage-1)*healthPageSize
		}
		if len(rows) != want {
			t.Errorf("page %d holds %d rows, want %d", number, len(rows), want)
		}
		walked = append(walked, rows...)
	}
	sorted := slices.Clone(walked)
	slices.Sort(sorted)
	if len(slices.Compact(sorted)) != len(walked) {
		t.Error("a file is on more than one page of the report")
	}
	if diff := cmp.Diff(whole, walked); diff != "" {
		t.Errorf("walking the pages is not the undivided table (-whole +walked):\n%s", diff)
	}
}

// TestTheLastPageOfTheReportNamesItsOwnRows holds the sentence over the table
// to the rows under it. The last page is the only one that is not a full page,
// so it is the one that can be wrong while every other page reads correctly.
func TestTheLastPageOfTheReportNamesItsOwnRows(t *testing.T) {
	t.Parallel()

	lastPage := (pagedReport + healthPageSize - 1) / healthPageSize
	view := pagedHealthView(pagedReport)
	view.Page = PageNumber(lastPage)
	page := renderHealth(t, &view)

	first := (lastPage-1)*healthPageSize + 1
	want := fmt.Sprintf("第 %d–%d 列，共 %d 列", first, pagedReport, pagedReport)
	if !strings.Contains(page, want) {
		t.Errorf("the last page of the report does not say %q", want)
	}
	// Counted off the rendered table rather than taken from the sentence, so
	// the two have to agree about the same rows rather than about each other.
	if got := len(fileCell.FindAllString(page, -1)); got != pagedReport-first+1 {
		t.Errorf("the sentence names %d rows and the table draws %d", pagedReport-first+1, got)
	}
}

// TestTheReportsShapeIsTheReportNotThePage holds the two tallies beside the
// table to the whole report. A shape line that followed the page would say the
// folder had different faults depending on which stretch of the table was on
// screen.
func TestTheReportsShapeIsTheReportNotThePage(t *testing.T) {
	t.Parallel()

	view := pagedHealthView(pagedReport)
	for _, asked := range []PageNumber{1, 2, 3, AllPages} {
		view.Page = asked
		page := renderHealth(t, &view)

		match := shapeFileCount.FindStringSubmatch(page)
		if match == nil {
			t.Fatalf("page %v carries no shape line to read", asked)
		}
		files, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("page %v's shape line counts %q, which is no number: %v", asked, match[1], err)
		}
		if files != pagedReport {
			t.Errorf("page %v says the report touches %d files, want %d", asked, files, pagedReport)
		}
		tallies := guideTally.FindAllStringSubmatch(page, -1)
		if len(tallies) != 1 {
			t.Fatalf("page %v explains %d kinds of finding, want the one the fixture holds", asked, len(tallies))
		}
		counted, err := strconv.Atoi(tallies[0][1])
		if err != nil {
			t.Fatalf("page %v's guide counts %q, which is no number: %v", asked, tallies[0][1], err)
		}
		if counted != pagedReport {
			t.Errorf("page %v's guide counts %d findings, want %d", asked, counted, pagedReport)
		}
	}
}

// TestAnUnaskablePageOfTheReportAnswersWithTheFirst holds the closed set the
// findings table reads a page against, which is the one its ordering is read
// against: an unanswerable request is answered with the page.
func TestAnUnaskablePageOfTheReportAnswersWithTheFirst(t *testing.T) {
	t.Parallel()

	view := pagedHealthView(pagedReport)
	view.Page = 1
	first := healthRowFiles(t, &view)
	for _, asked := range []string{"", "0", "-3", "1.5", "two", "99", "9999999999999999999"} {
		view.Page = ParsePageNumber(asked)
		if diff := cmp.Diff(first, healthRowFiles(t, &view)); diff != "" {
			t.Errorf("page=%q is not the first page (-first +asked):\n%s", asked, diff)
		}
	}
}

// TestAShortReportIsNotDivided holds the strip off a table that fits, and puts
// it on the one that does not.
func TestAShortReportIsNotDivided(t *testing.T) {
	t.Parallel()

	short := pagedHealthView(healthPageSize)
	if page := renderHealth(t, &short); strings.Contains(page, "y-pager") {
		t.Errorf("a report of exactly %d rows draws a strip", healthPageSize)
	}
	long := pagedHealthView(healthPageSize + 1)
	page := renderHealth(t, &long)
	if !strings.Contains(page, `<nav class="y-pager"`) {
		t.Fatalf("a report of %d rows draws no strip, so there is no way to its last row", healthPageSize+1)
	}
	if !strings.Contains(page, `href="?page=2&amp;sort=finding"`) {
		t.Error("the strip carries no way to the second page in the ordering in force")
	}
	if !strings.Contains(page, `href="?page=all&amp;sort=finding"`) {
		t.Error("the strip offers no undivided table, so nothing prints whole")
	}
	if got := strings.Count(page, `aria-current="page"`); got != 1 {
		t.Errorf("the strip marks %d pages as the one being read, want 1", got)
	}
}

// TestTheStripKeepsTheOrderingInForce is the composing half: stepping through
// a long report must not quietly put it back in the order it was not being
// read in.
func TestTheStripKeepsTheOrderingInForce(t *testing.T) {
	t.Parallel()

	view := pagedHealthView(pagedReport)
	view.Sort = HealthBySeverity
	view.Page = 2
	page := renderHealth(t, &view)
	for _, link := range regexp.MustCompile(`class="y-pager__[a-z]+" href="([^"]*)"`).FindAllStringSubmatch(page, -1) {
		if !strings.Contains(link[1], "sort=severity") {
			t.Errorf("the strip link %q drops the ordering the table is being read in", link[1])
		}
	}
}

// pagedHealthView is a report of rows uncited notes, each its own row and each
// at its own address, which is the one finding that draws one line per note.
func pagedHealthView(rows int) HealthView {
	notes := make([]nav.NoteRef, 0, rows)
	for i := range rows {
		notes = append(notes, nav.NoteRef{
			Name:    fmt.Sprintf("Note %03d", i),
			RelPath: fmt.Sprintf("Notes/n%03d.md", i),
		})
	}
	return HealthView{
		Islands:     []HealthIslandGroup{{Dir: "Notes", Name: "Notes", Notes: notes}},
		IslandCount: rows,
		// What a request that named no ordering resolves to, so the strip's
		// links read as the ones a reader would actually be handed.
		Sort: HealthByFinding,
	}
}
