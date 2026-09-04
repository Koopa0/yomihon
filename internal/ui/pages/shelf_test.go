package pages

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestShelfRowsNarrowsToTheRowsThatLeadSomewhere covers what a narrow width
// takes and what it refuses. A row that is listed without being a stop is real
// — an unwritten lesson is part of its course — but it cannot be a way in, and
// a block that spent one of its three lines on one would offer the reader
// something that does not open.
func TestShelfRowsNarrowsToTheRowsThatLeadSomewhere(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		shelf Shelf
		limit int
		want  []string
	}{
		{
			name:  "a flat shelf keeps its order",
			shelf: Shelf{Rows: []Row{{Text: "a", Href: "/a"}, {Text: "b", Href: "/b"}}},
			limit: 3,
			want:  []string{"a", "b"},
		},
		{
			name:  "a row that is not a stop is passed over, not counted",
			shelf: Shelf{Rows: []Row{{Text: "a", Href: "/a"}, {Text: "unwritten"}, {Text: "b", Href: "/b"}}},
			limit: 2,
			want:  []string{"a", "b"},
		},
		{
			name:  "the limit holds",
			shelf: Shelf{Rows: []Row{{Text: "a", Href: "/a"}, {Text: "b", Href: "/b"}, {Text: "c", Href: "/c"}}},
			limit: 2,
			want:  []string{"a", "b"},
		},
		{
			name:  "a limit below zero is read as none",
			shelf: Shelf{Rows: []Row{{Text: "a", Href: "/a"}}},
			limit: -1,
			want:  []string{},
		},
		{
			name:  "a shelf holding nothing narrows to nothing",
			shelf: Shelf{},
			limit: 3,
			want:  []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shelfRows(&tt.shelf, tt.limit)
			if len(got) != len(tt.want) {
				t.Fatalf("shelfRows gave %d rows, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i].Text != tt.want[i] {
					t.Errorf("row %d is %q, want %q", i, got[i].Text, tt.want[i])
				}
			}
		})
	}
}

// TestShelfBlockHandsTheReaderTheRestOfTheShelf keeps the block a way in rather
// than a listing of its own: whatever it could not show is one click away, and
// the link says so without a second figure that could disagree with the count
// already in the block's head.
func TestShelfBlockHandsTheReaderTheRestOfTheShelf(t *testing.T) {
	t.Parallel()

	shelf := Shelf{
		Title: "路徑", Count: "2 條", Lede: "課程。", Href: "/paths", Empty: "沒有課程。",
		Rows: []Row{{Text: "one", Href: "/a"}, {Text: "two", Href: "/b"}},
	}
	var full strings.Builder
	if err := ShelfBlock(shelf, "paths", 1, wording.ZhHant).Render(t.Context(), &full); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := full.String()
	if !strings.Contains(got, `<a class="ui-navitem y-shelfall" href="/paths">`+wording.ShelfAll.In(wording.ZhHant)+`</a>`) {
		t.Errorf("a narrowed block does not hand the reader the rest of the shelf:\n%s", got)
	}
	if strings.Contains(got, ">two<") {
		t.Errorf("the block showed a row past its limit:\n%s", got)
	}
	if strings.Contains(got, shelf.Empty) {
		t.Errorf("a block with rows spoke its empty sentence:\n%s", got)
	}

	var bare strings.Builder
	if err := ShelfBlock(Shelf{Title: "路徑", Href: "/paths", Empty: "沒有課程。"}, "paths", 3, wording.ZhHant).Render(t.Context(), &bare); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(bare.String(), "沒有課程。") {
		t.Errorf("an empty shelf did not say what it means:\n%s", bare.String())
	}
	if strings.Contains(bare.String(), "y-shelfall") {
		t.Errorf("an empty shelf offered the rest of itself:\n%s", bare.String())
	}
}

// TestAShelfOfRowsThatAreNotStopsIsNotEmpty separates what a block can show
// from what its shelf holds. A course whose lessons are none of them written
// yet fills a shelf with rows that lead nowhere; a block that read its own
// narrowing as the answer would call that shelf empty, print the sentence for a
// vault that declares none, and withdraw the way to the page that lists them.
func TestAShelfOfRowsThatAreNotStopsIsNotEmpty(t *testing.T) {
	t.Parallel()

	shelf := Shelf{
		Title: "路徑", Count: "1 條", Href: "/paths", Empty: "沒有課程。",
		Rows: []Row{{Text: "unwritten"}},
	}
	var out strings.Builder
	if err := ShelfBlock(shelf, "paths", 3, wording.ZhHant).Render(t.Context(), &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := out.String()
	if strings.Contains(got, shelf.Empty) {
		t.Errorf("a shelf holding a row nobody can open was called empty:\n%s", got)
	}
	if !strings.Contains(got, "y-shelfall") {
		t.Errorf("a shelf holding something withdrew the way to the rest of it:\n%s", got)
	}
}

// TestARailOpensAtTheRowTheReaderIsOn is the claim that separates this width
// from the others. A block and a page both start at the top, because both are
// ways in; a rail is drawn beside something already open, and a folder of
// hundreds would show its first two dozen and never the one being read.
func TestARailOpensAtTheRowTheReaderIsOn(t *testing.T) {
	t.Parallel()

	rows := make([]Row, 100)
	for i := range rows {
		rows[i] = Row{Text: fmt.Sprintf("row %02d", i), Href: fmt.Sprintf("/notes/%02d.md", i)}
	}

	tests := []struct {
		name        string
		current     int
		limit       int
		wantFirst   string
		wantTrimmed int
	}{
		{name: "the row being read is in the middle", current: 50, limit: 24, wantFirst: "row 38", wantTrimmed: 76},
		{name: "the row being read is at the top", current: 0, limit: 24, wantFirst: "row 00", wantTrimmed: 76},
		// The window slides back rather than running off the end, so the last
		// row of a folder is shown with the rows before it rather than alone.
		{name: "the row being read is the last one", current: 99, limit: 24, wantFirst: "row 76", wantTrimmed: 76},
		{name: "no row is being read", current: -1, limit: 24, wantFirst: "row 00", wantTrimmed: 76},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shelf := Shelf{Rows: slices.Clone(rows)}
			if tt.current >= 0 {
				shelf.Rows[tt.current].Current = true
			}
			window, trimmed := railRows(&shelf, tt.limit)
			if len(window) != tt.limit {
				t.Fatalf("the rail shows %d rows, want %d", len(window), tt.limit)
			}
			if trimmed != tt.wantTrimmed {
				t.Errorf("the rail says it left out %d, want %d", trimmed, tt.wantTrimmed)
			}
			if window[0].Text != tt.wantFirst {
				t.Errorf("the window opens at %q, want %q", window[0].Text, tt.wantFirst)
			}
			if tt.current >= 0 && !slices.ContainsFunc(window, func(r Row) bool { return r.Current }) {
				t.Errorf("the rail left out the row the reader is on; it shows %q to %q", window[0].Text, window[len(window)-1].Text)
			}
		})
	}
}

// TestARailThatFitsSaysItLeftNothing keeps the tail honest at the size where it
// matters most: a folder small enough to show whole must not offer a way to
// rows that are all already there.
func TestARailThatFitsSaysItLeftNothing(t *testing.T) {
	t.Parallel()

	shelf := Shelf{Rows: []Row{{Text: "a", Href: "/a", Current: true}, {Text: "b", Href: "/b"}}}
	window, trimmed := railRows(&shelf, 24)
	if len(window) != 2 || trimmed != 0 {
		t.Fatalf("a shelf of two at a limit of 24 gave %d rows and %d left out, want 2 and 0", len(window), trimmed)
	}

	var out strings.Builder
	if err := ShelfRail(shelf, 24, "Notes", "另外 %d 篇 →").Render(t.Context(), &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "y-here__more") {
		t.Errorf("a rail showing every row offered the rest of them:\n%s", got)
	}
	if !strings.Contains(got, `<a class="ui-navitem is-active" href="/a" aria-current="page">a</a>`) {
		t.Errorf("the rail does not mark the row the reader is on:\n%s", got)
	}
}

// TestARailSaysWhatItCouldNotShow covers the tail the filter box reads. The
// sentence is the owner's and the number is this width's, and the folder the
// rows live under travels with them so a reader can search the whole of it.
func TestARailSaysWhatItCouldNotShow(t *testing.T) {
	t.Parallel()

	rows := make([]Row, 30)
	for i := range rows {
		rows[i] = Row{Text: fmt.Sprintf("row %02d", i), Href: fmt.Sprintf("/notes/Diary/%02d.md", i)}
	}
	rows[0].Current = true
	shelf := Shelf{Title: "Diary", Href: "/folders/Diary", Rows: rows}

	var out strings.Builder
	if err := ShelfRail(shelf, 24, "Diary", "另外 %d 篇 →").Render(t.Context(), &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `data-rail-trimmed="6" data-rail-dir="Diary"`) {
		t.Errorf("the tail does not carry the count and the folder the filter needs:\n%s", got)
	}
	if !strings.Contains(got, "另外 6 篇 →") {
		t.Errorf("the tail does not say how many it left out:\n%s", got)
	}
	if !strings.Contains(got, `<nav class="y-here" aria-label="Diary">`) {
		t.Errorf("the rail is not named by the shelf it shows:\n%s", got)
	}
}

// TestADeskBlockIsItsPageNarrowed is the claim the desk rests on: a block and
// the page its heading opens read the same shelf, so a block can never name
// something its page does not list, or measure it differently. It compares the
// rows and the title rather than the sentence, which a block states shorter.
func TestADeskBlockIsItsPageNarrowed(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	lang := wording.ZhHant
	pages := map[string]ListIndexView{
		pathMode:   NewPathIndex(model.Paths(), nav.Closure{}, true, lang),
		mapMode:    NewMapIndex(model.Maps(), nav.Closure{}, true, lang),
		reportMode: NewReportIndex(model.Reports(), lang),
		folderMode: NewFolderIndex(model, lang),
	}
	blocks := NewDeskBlocks(model, true, lang)
	seen := 0
	for _, block := range blocks {
		page, ok := pages[block.Mode]
		if !ok {
			continue
		}
		seen++
		if block.Shelf.Title != page.Shelf.Title {
			t.Errorf("the %s block is titled %q and its page %q", block.Mode, block.Shelf.Title, page.Shelf.Title)
		}
		shown := shelfRows(&block.Shelf, deskBlockItems)
		listed := shelfRows(&page.Shelf, len(page.Shelf.Rows))
		// A block that showed nothing at all used to satisfy this: comparing
		// no rows against any number of rows found no disagreement. It owes
		// the reader as many as it has room for.
		if want := min(deskBlockItems, len(listed)); len(shown) != want {
			t.Fatalf("the %s block shows %d rows over a page listing %d, want %d", block.Mode, len(shown), len(listed), want)
		}
		for i := range shown {
			if shown[i] != listed[i] {
				t.Errorf("the %s block's row %d is %+v, its page's is %+v", block.Mode, i, shown[i], listed[i])
			}
		}
	}
	if seen != len(pages) {
		t.Fatalf("compared %d modes, want %d — a mode with no block compares nothing", seen, len(pages))
	}
}
