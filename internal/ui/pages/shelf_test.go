package pages

import (
	"strings"
	"testing"

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
