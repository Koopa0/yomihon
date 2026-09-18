package pages

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// journalMode is the journal's name for the page it is drawn on, stamped on the
// markup the way the other organisations stamp theirs.
const journalMode = "journal"

// monthLayout is a month written as the vault writes a day, minus the day. The
// journal's own dates are ISO 8601 calendar dates, and the address a reader can
// see in the location bar stays in the same notation as the days inside it.
const monthLayout = "2006-01"

// Month is one calendar month, which is the unit the journal is read in. It
// owns the pair that names a month and nothing else — no clock, no zone, no
// day — because the only questions asked of it are which days it holds, which
// month lies on either side, and what to call it.
type Month struct {
	year  int
	month time.Month
}

// MonthOf is the month a moment falls in. It is the one place a clock reaches
// the journal: the page is built from a Month, so every recording, test and
// probe of it names a month outright and answers the same tomorrow.
func MonthOf(at time.Time) Month {
	year, month, _ := at.Date()
	return Month{year: year, month: month}
}

// ParseMonth reads a month from the address. It refuses anything that is not
// exactly a year and a month in ISO 8601 notation — a missing zero, a
// thirteenth month, a whole day — so a hand-written address is answered with
// the caller's own fallback rather than with a grid of somewhere else.
func ParseMonth(value string) (Month, bool) {
	at, err := time.Parse(monthLayout, value)
	if err != nil {
		return Month{}, false
	}
	return MonthOf(at), true
}

// String writes the month the way the address carries it.
func (m Month) String() string {
	return m.first().Format(monthLayout)
}

// Holds reports whether an ISO 8601 calendar date falls in this month. It
// compares the written form, which is what a journal entry carries: every day
// here is produced zero-padded, so the month is a prefix of the day and nothing
// has to be parsed to find out.
func (m Month) Holds(day string) bool {
	return day != "" && strings.HasPrefix(day, m.String()+"-")
}

// Shift is the month a given number of months away, forward or back. Go's own
// date arithmetic normalises the roll over a year end, so December's next is
// the following January without a rule written here for it.
func (m Month) Shift(months int) Month {
	return MonthOf(m.first().AddDate(0, months, 0))
}

// Label names the month in the reader's language: 繁中 counts it, English calls
// it by name, and each puts the year where that language puts it.
func (m Month) Label(lang wording.Lang) string {
	return fmt.Sprintf(wording.MonthFmt.In(lang), m.year, wording.MonthNames[m.month-1].In(lang))
}

// first is the first day of the month, where the calendar arithmetic starts.
// UTC because nothing here is a moment: it is a position in a table.
func (m Month) first() time.Time {
	return time.Date(m.year, m.month, 1, 0, 0, 0, 0, time.UTC)
}

// days is how many days this month has. Asking Go for the day before the first
// of the next month is what keeps a leap year from being a rule written here.
func (m Month) days() int {
	return m.Shift(1).first().AddDate(0, 0, -1).Day()
}

// monthOfDay is the month an ISO 8601 calendar date falls in.
func monthOfDay(day string) (Month, bool) {
	if len(day) < len(monthLayout) {
		return Month{}, false
	}
	return ParseMonth(day[:len(monthLayout)])
}

// weekColumns is how wide a week is, which is also how many column headings the
// table carries. It reads off the weekday names so the two cannot disagree.
const weekColumns = len(wording.WeekdayNames)

// weekdayColumn is which of the seven columns a day falls in, counting from
// Monday. The week starts on Monday because the vault writes every day as an
// ISO 8601 calendar date and that standard's week starts there. One rule serves
// both languages, so one month cannot look like two different months for being
// read in the other one.
func weekdayColumn(day time.Weekday) int {
	return (int(day) + 6) % weekColumns
}

// MonthLink is one step out of the month being read: where it leads and what it
// is called. A month with nothing on that side carries an empty one, which the
// page draws as no link at all rather than as a dead one.
type MonthLink struct {
	Text string
	Href string
}

// DayEntry is one journal entry inside a day's square: what its name adds to
// the day the square already shows, its whole name for anyone who cannot see
// the square it is in, where it opens, and the language it was written in. Text
// is empty for an entry named for nothing but its own day, which is what most
// journal entries are named for.
type DayEntry struct {
	Text     string
	Name     string
	Href     string
	Language string
}

// DayCell is one square of the month. Number and Date are empty for the squares
// before the month began and after it ended, which belong to the table's shape
// rather than to the month.
type DayCell struct {
	Number  string
	Date    string
	Entries []DayEntry
}

// MonthGrid is the month as a reader looks at it: what it is called, the seven
// columns, the weeks it falls into, and the way out on either side.
type MonthGrid struct {
	Caption  string
	NavLabel string
	Weekdays []string
	Weeks    [][]DayCell
	Earlier  MonthLink
	Later    MonthLink
}

// JournalView is the journal opened at one month: the month drawn as a
// calendar, that month's entries as the rows every shelf draws, and — under
// every month, because they belong to none — the entries that wrote no day.
//
// Fault is why the journal projection was withheld, stated once for the page. A
// withheld journal lists nothing, and its shelf then says neither how much it
// holds nor that it holds none, because a vault whose declaration could not be
// read is not a vault that declared nothing.
type JournalView struct {
	Kicker  string
	Title   string
	Lede    string
	Fault   string
	Grid    MonthGrid
	Entries Shelf
	Undated Shelf
}

// NewJournalIndex builds the journal at one month from the whole journal
// projection, newest first. It takes every entry rather than the month's own
// because the two steps out of the month are answered from the rest: a journal
// is written in the months its author sat down, and a link labelled "previous
// month" that lands on a bare grid is a link to nothing.
func NewJournalIndex(entries []nav.JournalEntry, month Month, closure nav.Closure, lang wording.Lang, articleLang ArticleLanguageFor) JournalView {
	var inMonth, undated []nav.JournalEntry
	for _, entry := range entries {
		switch {
		case month.Holds(entry.Date):
			inMonth = append(inMonth, entry)
		case entry.Date == "":
			undated = append(undated, entry)
		}
	}
	view := JournalView{
		Kicker: plural(len(inMonth), wording.JournalCountOne, wording.JournalCountMany, lang),
		Title:  wording.Journal.In(lang),
		Lede:   wording.JournalIndexLede.In(lang),
		Grid:   newMonthGrid(inMonth, entries, month, lang, articleLang),
		Entries: Shelf{
			Empty: monthEmptySentence(entries, lang),
			Rows:  journalRows(inMonth, lang, articleLang),
		},
		Undated: Shelf{
			Title: wording.JournalUndatedTitle.In(lang),
			Rows:  journalRows(undated, lang, articleLang),
		},
	}
	view.Fault = closure.Diagnostic()
	withholdJournal(&view, closure)
	return view
}

// monthEmptySentence is what the page says where the calendar is bare. A vault
// holding no journal at all and a month nobody wrote in are different absences,
// and only one of them is repaired by looking in another month.
func monthEmptySentence(entries []nav.JournalEntry, lang wording.Lang) string {
	if len(entries) == 0 {
		return wording.JournalIndexEmpty.In(lang)
	}
	return wording.JournalMonthEmpty.In(lang)
}

// journalRows lays entries out as the rows every shelf draws, each leading with
// the day it is for. An entry that wrote none says so in that column rather
// than leaving it blank, because a blank in the column the rest of the shelf
// answers in reads as a day the page failed to look up.
//
// The name beside the day is what the name adds to it. Journal entries are
// mostly named for the day they are for, and a row reading "2026-07-23
// 2026-07-23" says one thing twice; the entries named for something else keep
// their names, which is the whole of what the name column is for here.
func journalRows(entries []nav.JournalEntry, lang wording.Lang, articleLang ArticleLanguageFor) []Row {
	rows := make([]Row, 0, len(entries))
	for _, entry := range entries {
		when := entry.Date
		if when == "" {
			when = wording.ReportUndated.In(lang)
		}
		rows = append(rows, Row{
			When:     when,
			Text:     nameBeyondTheDay(entry),
			Href:     notesHref(entry.RelPath),
			Language: rowLanguage(articleLang, entry.RelPath),
		})
	}
	return rows
}

// nameBeyondTheDay is what an entry's name says that the day beside it does not
// already. A vault names its journal entries for the days they are for, so most
// of them add nothing and the face they are drawn on shows the day alone; one
// named "2026-07-10 夜" adds the half of the day it was written in, and one
// named for what it is about keeps its name whole. A name that leads with some
// other day than the one the entry is for is not trimmed, because the two
// disagreeing is the author's to see rather than yomihon's to tidy away.
func nameBeyondTheDay(entry nav.JournalEntry) string {
	return strings.TrimSpace(strings.TrimPrefix(entry.Title, entry.Date))
}

// newMonthGrid lays the month out as the weeks it falls into. Every square of
// every week is present, including the ones before the first day and after the
// last, so each column stays under its own weekday however the month begins.
//
// A day carries one link per entry written on it, in the order the rows below
// carry them. Two entries on one day is exactly where a rule that made the
// square itself the link would have to pick one of them, and picking is what
// this vault's reading never does.
//
// Each link is named by what the entry's name adds to the day its square
// already shows. An entry named for nothing but that day adds nothing and is
// drawn as a mark instead, carrying its whole name for a reader who cannot see
// which square it is in.
func newMonthGrid(inMonth, all []nav.JournalEntry, month Month, lang wording.Lang, articleLang ArticleLanguageFor) MonthGrid {
	byDay := make(map[string][]DayEntry, len(inMonth))
	for _, entry := range inMonth {
		byDay[entry.Date] = append(byDay[entry.Date], DayEntry{
			Text:     nameBeyondTheDay(entry),
			Name:     entry.Title,
			Href:     notesHref(entry.RelPath),
			Language: rowLanguage(articleLang, entry.RelPath),
		})
	}

	weekdays := make([]string, 0, weekColumns)
	for _, name := range wording.WeekdayNames {
		weekdays = append(weekdays, name.In(lang))
	}

	// The month is laid out as one run of squares and then cut into weeks. The
	// run opens with the squares of the week before the first falls in and
	// closes with the ones after the last, so every week is a week wide and
	// each column stays under its own weekday however the month begins.
	cells := make([]DayCell, 0, weekColumns*6)
	for range weekdayColumn(month.first().Weekday()) {
		cells = append(cells, DayCell{})
	}
	for day := 1; day <= month.days(); day++ {
		date := month.first().AddDate(0, 0, day-1).Format(time.DateOnly)
		cells = append(cells, DayCell{Number: strconv.Itoa(day), Date: date, Entries: byDay[date]})
	}
	for len(cells)%weekColumns != 0 {
		cells = append(cells, DayCell{})
	}
	var weeks [][]DayCell
	for week := range slices.Chunk(cells, weekColumns) {
		weeks = append(weeks, week)
	}

	return MonthGrid{
		Caption:  month.Label(lang),
		NavLabel: wording.JournalMonthsNav.In(lang),
		Weekdays: weekdays,
		Weeks:    weeks,
		Earlier:  monthStep(all, month, earlier, lang),
		Later:    monthStep(all, month, later, lang),
	}
}

// The two directions out of a month, named rather than written as a sign so the
// call that asks for one says which way it is going.
const (
	earlier = -1
	later   = 1
)

// monthStep is the nearest month on one side of this one that holds an entry,
// as a link named for that month. It answers with nothing where there is none,
// and the page then draws no link rather than one that leads to a bare grid.
//
// An entry that wrote no day is in no month and cannot be stepped to. It sits
// under every month instead, which is where a reader meets it whichever month
// they opened.
func monthStep(all []nav.JournalEntry, from Month, direction int, lang wording.Lang) MonthLink {
	here, nearest := from.String(), ""
	for _, entry := range all {
		candidate, ok := monthOfDay(entry.Date)
		if !ok {
			continue
		}
		written := candidate.String()
		if beyond(written, here, direction) && (nearest == "" || beyond(nearest, written, direction)) {
			nearest = written
		}
	}
	step, ok := ParseMonth(nearest)
	if !ok {
		return MonthLink{}
	}
	format := wording.JournalLaterFmt
	if direction == earlier {
		format = wording.JournalEarlierFmt
	}
	return MonthLink{
		Text: fmt.Sprintf(format.In(lang), step.Label(lang)),
		Href: journalHref(step),
	}
}

// beyond reports whether one written month lies past another in the direction
// being stepped. ISO 8601 months compare as text exactly as they compare as
// months, both being zero-padded and largest unit first, so this is a
// comparison of two written months and no calendar arithmetic enters.
func beyond(month, than string, direction int) bool {
	if direction == earlier {
		return month < than
	}
	return month > than
}

// withholdJournal takes back what the page may not claim about a journal
// declaration that was closed: how much the month holds, and that it holds
// none. It is the same withdrawal the mode indexes make, and for the same
// reason — a declaration that could not be read is not a declaration of
// nothing, and the reason is stated once at the head of the page.
func withholdJournal(v *JournalView, closure nav.Closure) {
	if !closure.Closed() {
		return
	}
	v.Kicker = ""
	v.Entries.Empty = ""
}
