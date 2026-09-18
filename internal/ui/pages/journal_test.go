package pages

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// journalFixture is a journal written unevenly across three months: days with
// an entry and days without, one day carrying two, one entry whose declared day
// disagrees with the name of its own file, and one that wrote no day at all. It
// arrives newest first, which is the order the model hands it over in.
func journalFixture() []nav.JournalEntry {
	return []nav.JournalEntry{
		{Title: "2026-08-03", RelPath: "Diary/2026-08-03.md", Date: "2026-08-03"},
		{Title: "2026-07-31", RelPath: "Diary/2026-07-31.md", Date: "2026-07-31"},
		{Title: "2026-07-20 夜", RelPath: "Diary/2026-07-20 夜.md", Date: "2026-07-20"},
		{Title: "2026-07-20", RelPath: "Diary/2026-07-20.md", Date: "2026-07-20"},
		{Title: "week in Kyoto", RelPath: "Diary/week in Kyoto.md", Date: "2026-07-13"},
		{Title: "2026-07-01", RelPath: "Diary/2026-07-01.md", Date: "2026-07-01"},
		{Title: "2026-06-28", RelPath: "Diary/2026-06-28.md", Date: "2026-06-28"},
		{Title: "loose thoughts", RelPath: "Diary/loose thoughts.md"},
	}
}

func julyMonth(t *testing.T) Month {
	t.Helper()
	month, ok := ParseMonth("2026-07")
	if !ok {
		t.Fatal(`ParseMonth("2026-07") refused a month it has to accept`)
	}
	return month
}

// gridEntries is every entry link the calendar draws, in the order it draws
// them, each paired with the square it was drawn in.
func gridEntries(grid *MonthGrid) map[string][]string {
	drawn := make(map[string][]string)
	for _, week := range grid.Weeks {
		for _, cell := range week {
			for _, entry := range cell.Entries {
				drawn[entry.Href] = append(drawn[entry.Href], cell.Date)
			}
		}
	}
	return drawn
}

// TestJournalMonthDrawsEveryEntryOnceInTheGridAndOnceInTheRows is the claim the
// two halves of the page make together: the calendar is not a sample of the
// month and the rows are not a second month. An entry drawn twice in a square,
// or drawn in a square whose day is not its own, or listed below without
// appearing above, is the page disagreeing with itself about what was written.
//
// The entries of the neighbouring months and the one that wrote no day are here
// to be left out: a builder that simply drew everything it was handed would
// pass a test that only counted what it expected to find.
func TestJournalMonthDrawsEveryEntryOnceInTheGridAndOnceInTheRows(t *testing.T) {
	t.Parallel()

	view := NewJournalIndex(journalFixture(), julyMonth(t), nav.Closure{}, wording.ZhHant, nil)

	wantHrefs := []string{
		"/notes/Diary/2026-07-31.md",
		"/notes/Diary/2026-07-20%20%E5%A4%9C.md",
		"/notes/Diary/2026-07-20.md",
		"/notes/Diary/week%20in%20Kyoto.md",
		"/notes/Diary/2026-07-01.md",
	}
	wantDays := map[string]string{
		"/notes/Diary/2026-07-31.md":             "2026-07-31",
		"/notes/Diary/2026-07-20%20%E5%A4%9C.md": "2026-07-20",
		"/notes/Diary/2026-07-20.md":             "2026-07-20",
		"/notes/Diary/week%20in%20Kyoto.md":      "2026-07-13",
		"/notes/Diary/2026-07-01.md":             "2026-07-01",
	}

	drawn := gridEntries(&view.Grid)
	for href, squares := range drawn {
		day, wanted := wantDays[href]
		switch {
		case !wanted:
			t.Errorf("the grid draws %s, which is not written in this month", href)
		case len(squares) != 1:
			t.Errorf("the grid draws %s in %d squares (%v), want exactly one", href, len(squares), squares)
		case squares[0] != day:
			t.Errorf("the grid draws %s in the square for %s, want %s", href, squares[0], day)
		}
	}
	for href := range wantDays {
		if _, drawnHere := drawn[href]; !drawnHere {
			t.Errorf("the grid never draws %s, which is written in this month", href)
		}
	}

	var listed []string
	for _, row := range view.Entries.Rows {
		listed = append(listed, row.Href)
	}
	if diff := cmp.Diff(wantHrefs, listed); diff != "" {
		t.Errorf("the month's rows mismatch (-want +got):\n%s", diff)
	}
}

// TestJournalEntryWithNoDayIsGatheredUnderTheMonth keeps an entry that named no
// day reachable. It belongs to no month, so a calendar has no square for it and
// the month's rows are not its home either; it is given a group of its own with
// a heading that says why, under whichever month is open — under every one of
// them, because a group shown only on the current month could not be reached
// from any other.
func TestJournalEntryWithNoDayIsGatheredUnderTheMonth(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"2026-07", "2026-08", "2029-02"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			month, ok := ParseMonth(name)
			if !ok {
				t.Fatalf("ParseMonth(%q) refused a month it has to accept", name)
			}
			view := NewJournalIndex(journalFixture(), month, nav.Closure{}, wording.ZhHant, nil)

			const undated = "/notes/Diary/loose%20thoughts.md"
			if _, drawn := gridEntries(&view.Grid)[undated]; drawn {
				t.Error("the entry that wrote no day was drawn in a square of a month it is not in")
			}
			if slices.ContainsFunc(view.Entries.Rows, func(r Row) bool { return r.Href == undated }) {
				t.Error("the entry that wrote no day was listed among the month's own rows")
			}
			if view.Undated.Title != wording.JournalUndatedTitle.In(wording.ZhHant) {
				t.Errorf("the group of entries with no day is headed %q, want the heading that says so", view.Undated.Title)
			}
			want := []Row{{
				When: wording.ReportUndated.In(wording.ZhHant),
				Text: "loose thoughts",
				Href: undated,
			}}
			if diff := cmp.Diff(want, view.Undated.Rows); diff != "" {
				t.Errorf("the group of entries with no day mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestAJournalNameIsShownOnlyWhereItAddsToTheDay keeps the page from saying one
// thing twice. A vault names its journal entries for the days they are for, so
// beside the day a square already shows, most names add nothing at all; an
// entry named for half a day adds that half, and one named for what it is about
// keeps its name whole. The row under the calendar answers the same way, and a
// square with nothing to add draws a mark that still carries the entry's whole
// name for a reader who cannot see which square it is in.
func TestAJournalNameIsShownOnlyWhereItAddsToTheDay(t *testing.T) {
	t.Parallel()

	view := NewJournalIndex(journalFixture(), julyMonth(t), nav.Closure{}, wording.ZhHant, nil)

	want := map[string]string{
		"/notes/Diary/2026-07-31.md":             "",
		"/notes/Diary/2026-07-20.md":             "",
		"/notes/Diary/2026-07-20%20%E5%A4%9C.md": "夜",
		"/notes/Diary/week%20in%20Kyoto.md":      "week in Kyoto",
		"/notes/Diary/2026-07-01.md":             "",
	}
	inSquares := make(map[string]string, len(want))
	names := make(map[string]string, len(want))
	for _, week := range view.Grid.Weeks {
		for _, cell := range week {
			for _, entry := range cell.Entries {
				inSquares[entry.Href] = entry.Text
				names[entry.Href] = entry.Name
			}
		}
	}
	if diff := cmp.Diff(want, inSquares); diff != "" {
		t.Errorf("the names drawn in the squares mismatch (-want +got):\n%s", diff)
	}
	// A mark has no words of its own, so the name it carries is the whole of
	// what a reader who cannot see the square is told.
	if got := names["/notes/Diary/2026-07-01.md"]; got != "2026-07-01" {
		t.Errorf("the mark for 2026-07-01 carries the name %q, want the entry's own", got)
	}

	inRows := make(map[string]string, len(want))
	for _, row := range view.Entries.Rows {
		inRows[row.Href] = row.Text
	}
	if diff := cmp.Diff(want, inRows); diff != "" {
		t.Errorf("the names drawn in the rows mismatch (-want +got):\n%s", diff)
	}

	// The entry whose declared day disagrees with the day its own file is named
	// for keeps that name whole: the two disagreeing is the author's to see.
	quarrel := []nav.JournalEntry{{Title: "2026-01-01 backfilled", RelPath: "Diary/2026-01-01 backfilled.md", Date: "2026-07-04"}}
	rows := NewJournalIndex(quarrel, julyMonth(t), nav.Closure{}, wording.ZhHant, nil).Entries.Rows
	if len(rows) != 1 || rows[0].Text != "2026-01-01 backfilled" {
		t.Errorf("the row for an entry whose name names another day = %v, want its name kept whole", rows)
	}
}

// TestJournalWeeksStartOnMondayAndHoldEveryDayOnce walks the table's own shape.
// The vault writes every day as an ISO 8601 calendar date and that standard's
// week starts on Monday, so the first column is a Monday; the squares before
// the first of the month and after the last belong to the table rather than to
// the month and carry no day; and every day of the month appears once, in
// order, so a reader counting down a column lands where the calendar says.
func TestJournalWeeksStartOnMondayAndHoldEveryDayOnce(t *testing.T) {
	t.Parallel()

	// A month opening on the first column, one opening on the last, and the
	// February that has a twenty-ninth: the three shapes a rule written by hand
	// gets wrong, and none of them is the month the recordings use.
	for _, name := range []string{"2026-06", "2026-02", "2026-11", "2028-02"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			month, ok := ParseMonth(name)
			if !ok {
				t.Fatalf("ParseMonth(%q) refused a month it has to accept", name)
			}
			grid := NewJournalIndex(nil, month, nav.Closure{}, wording.ZhHant, nil).Grid

			if len(grid.Weekdays) != 7 {
				t.Fatalf("the month has %d column headings, want 7", len(grid.Weekdays))
			}
			var days []string
			for i, week := range grid.Weeks {
				if len(week) != 7 {
					t.Fatalf("week %d has %d squares, want 7", i, len(week))
				}
				for column, cell := range week {
					if cell.Date == "" {
						continue
					}
					at, err := time.Parse(time.DateOnly, cell.Date)
					if err != nil {
						t.Fatalf("square %d of week %d carries %q, which is not a day: %v", column, i, cell.Date, err)
					}
					// The column a day is expected in is worked out here rather
					// than asked of the builder's own weekdayColumn, which
					// would agree with itself however it was changed. Column 0
					// is Monday, which Go numbers 1, so the seventh is Sunday.
					if want := time.Weekday((column + 1) % 7); at.Weekday() != want {
						t.Errorf("%s is a %s and sits in column %d, which is the column for %s", cell.Date, at.Weekday(), column, want)
					}
					if cell.Number != strconv.Itoa(at.Day()) {
						t.Errorf("the square for %s is numbered %q, want %d", cell.Date, cell.Number, at.Day())
					}
					days = append(days, cell.Date)
				}
			}

			want := make([]string, 0, 31)
			for day := month.first(); month.Holds(day.Format(time.DateOnly)); day = day.AddDate(0, 0, 1) {
				want = append(want, day.Format(time.DateOnly))
			}
			if diff := cmp.Diff(want, days); diff != "" {
				t.Errorf("the days the month draws mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestJournalStepsReachTheNearestMonthThatHoldsSomething keeps the two ways out
// of a month leading somewhere. A journal is written in the months its author
// sat down, so a step that moved one calendar month at a time would land on a
// bare grid and ask the reader to click again; and a month with nothing on one
// side offers no link there rather than one that leads nowhere.
func TestJournalStepsReachTheNearestMonthThatHoldsSomething(t *testing.T) {
	t.Parallel()

	// March is empty and the journal knows nothing before June, so a reader
	// standing in March steps back to June's far side and forward to June.
	tests := []struct {
		month          string
		earlier, later string
	}{
		{month: "2026-07", earlier: "/journal?month=2026-06", later: "/journal?month=2026-08"},
		{month: "2026-06", earlier: "", later: "/journal?month=2026-07"},
		{month: "2026-08", earlier: "/journal?month=2026-07", later: ""},
		{month: "2026-03", earlier: "", later: "/journal?month=2026-06"},
		{month: "2027-01", earlier: "/journal?month=2026-08", later: ""},
	}
	for _, tt := range tests {
		t.Run(tt.month, func(t *testing.T) {
			t.Parallel()
			month, ok := ParseMonth(tt.month)
			if !ok {
				t.Fatalf("ParseMonth(%q) refused a month it has to accept", tt.month)
			}
			grid := NewJournalIndex(journalFixture(), month, nav.Closure{}, wording.En, nil).Grid
			if grid.Earlier.Href != tt.earlier {
				t.Errorf("the step back from %s leads to %q, want %q", tt.month, grid.Earlier.Href, tt.earlier)
			}
			if grid.Later.Href != tt.later {
				t.Errorf("the step on from %s leads to %q, want %q", tt.month, grid.Later.Href, tt.later)
			}
		})
	}
}

// TestJournalStepIsNamedForTheMonthItLeadsTo checks the words on those links.
// They are named for where they go rather than for being the previous or the
// next one, which is the whole reason they are allowed to skip a month nobody
// wrote in, and each language names a month its own way.
func TestJournalStepIsNamedForTheMonthItLeadsTo(t *testing.T) {
	t.Parallel()

	for lang, want := range map[wording.Lang][3]string{
		wording.ZhHant: {"← 2026 年 6 月", "2026 年 8 月 →", "2026 年 7 月"},
		wording.En:     {"← June 2026", "August 2026 →", "July 2026"},
	} {
		grid := NewJournalIndex(journalFixture(), julyMonth(t), nav.Closure{}, lang, nil).Grid
		got := [3]string{grid.Earlier.Text, grid.Later.Text, grid.Caption}
		if got != want {
			t.Errorf("in %s the month reads %q, want %q", lang, got, want)
		}
	}
}

// TestJournalSaysWhichAbsenceItIsLookingAt holds apart the two empty calendars.
// A vault with no journal at all is not repaired by opening another month, and
// a month nobody wrote in is; a page that said the same thing in both would
// send one of the two readers the wrong way.
func TestJournalSaysWhichAbsenceItIsLookingAt(t *testing.T) {
	t.Parallel()

	empty := NewJournalIndex(nil, julyMonth(t), nav.Closure{}, wording.En, nil)
	if got, want := empty.Entries.Empty, wording.JournalIndexEmpty.In(wording.En); got != want {
		t.Errorf("a vault with no journal says %q, want %q", got, want)
	}
	quiet, ok := ParseMonth("2026-03")
	if !ok {
		t.Fatal(`ParseMonth("2026-03") refused a month it has to accept`)
	}
	unwritten := NewJournalIndex(journalFixture(), quiet, nav.Closure{}, wording.En, nil)
	if got, want := unwritten.Entries.Empty, wording.JournalMonthEmpty.In(wording.En); got != want {
		t.Errorf("a month nobody wrote in says %q, want %q", got, want)
	}
}

// TestParseMonthRefusesWhatIsNotAMonth keeps a hand-written address from
// reaching the calendar as something else. The page falls back to the month the
// reader is in when this refuses, so anything it let through would be drawn as
// a month of somewhere the reader never asked for.
func TestParseMonthRefusesWhatIsNotAMonth(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "2026", "2026-7", "2026-13", "2026-00", "2026-07-10", "banana", "2026-07x"} {
		if month, ok := ParseMonth(value); ok {
			t.Errorf("ParseMonth(%q) = %s, want a refusal", value, month)
		}
	}
	for value, want := range map[string]string{"2026-07": "2026-07", "0001-01": "0001-01", "2026-12": "2026-12"} {
		month, ok := ParseMonth(value)
		if !ok {
			t.Errorf("ParseMonth(%q) refused a month", value)
			continue
		}
		if month.String() != want {
			t.Errorf("ParseMonth(%q).String() = %q, want %q", value, month.String(), want)
		}
	}
}

// TestWithheldJournalClaimsNothingAboutTheMonth keeps the page from turning a
// declaration it could not read into a statement about the vault. "4 entries"
// and "nothing was written this month" are both claims a closed journal cannot
// make, and the reason is stated once at the head of the page instead.
func TestWithheldJournalClaimsNothingAboutTheMonth(t *testing.T) {
	t.Parallel()

	closure := nav.Close(schema.Rejected("invalid journal directory"))
	view := NewJournalIndex(nil, julyMonth(t), closure, wording.En, nil)
	if view.Fault == "" {
		t.Error("a withheld journal states no reason")
	}
	if view.Kicker != "" {
		t.Errorf("a withheld journal still measures the month: %q", view.Kicker)
	}
	if view.Entries.Empty != "" {
		t.Errorf("a withheld journal still says the month holds nothing: %q", view.Entries.Empty)
	}
}
