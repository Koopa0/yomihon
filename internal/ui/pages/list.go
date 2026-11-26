package pages

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// The list rule — lists-and-search.md §2 — lives here once: how rows are
// grouped, which fact each row carries, and how a list is ordered. Pages hand
// over the notes and folders they have and draw the groups that come back, so
// a folder, the folder index and whatever lists next read as one list.

// ListSort is how a list is ordered. The page's query carries it, so the
// choice is an ordinary link and a page opens in its own default each time.
type ListSort string

const (
	// SortByName orders by the name each row goes by.
	SortByName ListSort = "name"
	// SortByRecent puts what changed last first.
	SortByRecent ListSort = "recent"
	// SortByOrder follows the numbers an author opened the file names with.
	SortByOrder ListSort = "order"
)

// ParseListSort reads the sort a request asked for. A word outside the set is
// no choice at all, and the page's own default stands.
func ParseListSort(value string) ListSort {
	switch s := ListSort(value); s {
	case SortByName, SortByRecent, SortByOrder:
		return s
	}
	return ""
}

// manyRows is how long a list may be before it earns a filter and, sorted by
// date, a heading for each month.
const manyRows = 20

// splitRows is how long a group's run of rows may be before a wide page sets it
// in two columns: past ten, the eye is faster down two short columns than down
// one tall one.
const splitRows = 10

// distinguishingRows is how long a list must be before a date that barely
// varies stops being drawn: ten rows or fewer are all in view at once.
const distinguishingRows = 10

// ListRow is one line of a list: a name, the one fact that says how this row
// differs from the next, and the tags a reader needs before opening it.
type ListRow struct {
	Title    string
	Href     string
	Language string
	Draft    bool
	// DraftWord is the tag a draft carries, in the reader's language.
	DraftWord string
	// Folder says the row is a folder, drawn with an arrow before its name.
	Folder bool
	// Fact is the sort key written out, and FactTime its machine form when it
	// is a date.
	Fact     string
	FactTime string
	// Matched is the part of the title a filter found, drawn heavier.
	Before, Match, After string
}

// ListGroup is a run of rows under one heading. A heading that is the
// author's words is set in their face; one that is yomihon's own is not.
type ListGroup struct {
	Heading string
	// Href makes the heading a link, as a top-level folder is.
	Href     string
	Authored bool
	// Split says the group's own rows run in two columns on a wide page, its
	// heading across both. A group that is not long keeps one column at the
	// full width of the list.
	Split bool
	Count string
	Rows  []ListRow
	// Action is the one thing a group offers about itself, such as opening the
	// folders it names.
	Action ListAction
}

// ListAction is a link a group heading carries at its far end.
type ListAction struct {
	Text string
	Href string
}

// ListNote is what the list rule needs to know of a note.
type ListNote struct {
	Title    string
	RelPath  string
	Language string
	Draft    bool
	Modified time.Time
}

// ListFolder is what the list rule needs to know of a folder.
type ListFolder struct {
	Name    string
	RelPath string
	Notes   int
	Latest  time.Time
}

// ListChoice is one way to order a list, as the link that gets there.
type ListChoice struct {
	Href    string
	Text    string
	Pressed bool
}

// ListSwitch is the choice of order drawn above a list.
type ListSwitch struct {
	Choices []ListChoice
}

// ListFilter is the field above a list that narrows it to what contains a
// word. The page itself answers it, so a reader with no script gets the same
// list from a form; the script asks for the list alone, at Action, to put in
// place.
type ListFilter struct {
	Hint   string
	Value  string
	Action string
}

// ListParam is one query parameter a bar keeps when its form is submitted.
type ListParam struct{ Name, Value string }

// List is every group of a list with the controls over it.
type List struct {
	// Count is the page's line saying what the list holds, above the bar that
	// carries the choice of order.
	Count string
	// Label heads the list when it is one group whose heading has risen into
	// the bar, with its count; the bar's rule then closes the head.
	Label         string
	LabelCount    string
	LabelAuthored bool
	// Ruled says the bar heads the list itself, a lone group's heading having
	// risen into it, so no group heading follows its rule.
	Ruled  bool
	Groups []ListGroup
	Switch ListSwitch
	Filter *ListFilter
	// Toggle opens or closes what the list holds below its own level.
	Toggle ListAction
	// Keep is the query the bar's form carries along with the filter.
	Keep []ListParam
	// Wide says the groups flow into two columns on a wide page.
	Wide bool
	// Empty is what the list says when it holds nothing at all, and Cleared
	// when a filter held everything back.
	Empty   string
	Cleared ListCleared
}

// ListCleared is what a filter that matched nothing says, and the way back.
type ListCleared struct {
	Text string
	Href string
	Link string
}

// nameCollator orders names the way the folder index promises: digits by
// value, then Latin letters, then Han by stroke count. A collator holds a
// buffer, so each list takes its own.
func nameCollator() *collate.Collator {
	return collate.New(language.MustParse("zh-Hant-u-co-stroke"), collate.Numeric)
}

// listDate writes a day as the list rule does: month and day in the current
// year, the whole date in another.
func listDate(at, now time.Time) string {
	if at.Year() == now.Year() {
		return at.Format("01-02")
	}
	return at.Format("2006-01-02")
}

// Ago says how long before now at was, in the reader's words: minutes within
// the hour, never fewer than one; hours within the same day; yesterday for the
// calendar day before; and otherwise the day itself, as listDate writes it.
//
// The day is a calendar day in the zone now is given in, which is the reader's:
// a moment late in the evening is yesterday the next morning, however few hours
// have passed, and it is the calendar rather than a count of hours that says so
// on a night the clocks change. at is read in that zone whatever zone it was
// stored in. A clock that runs a little behind the moment it is asked about
// still says one minute; a moment well ahead of it is not made into an age and
// is given as its day. The zero time has no age and says nothing.
func Ago(at, now time.Time, lang wording.Lang) string {
	if at.IsZero() {
		return ""
	}
	at = at.In(now.Location())
	since := now.Sub(at)
	if since < -time.Minute {
		return listDate(at, now)
	}
	if since < time.Hour {
		return fmt.Sprintf(wording.MinutesAgoFmt.In(lang), max(1, int(since/time.Minute)))
	}
	switch calendarDays(at, now) {
	case 0:
		return fmt.Sprintf(wording.HoursAgoFmt.In(lang), int(since/time.Hour))
	case 1:
		return wording.Yesterday.In(lang)
	}
	return listDate(at, now)
}

// calendarDays counts the days on the calendar from at's day to now's, each
// read in its own zone. Counting whole days between two midnights in UTC keeps
// a day that the clocks shortened or lengthened a day.
func calendarDays(at, now time.Time) int {
	day := func(t time.Time) time.Time {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	return int(day(now).Sub(day(at)) / (24 * time.Hour))
}

// NoteRows applies the list rule to the notes of one folder: order, the
// shared-prefix groups, the month groups, and the one fact a row carries.
func NoteRows(notes []ListNote, sort ListSort, now time.Time, lang wording.Lang) []ListGroup {
	notes = slices.Clone(notes)
	orderNotes(notes, sort)
	rows := noteListRows(notes, sort == SortByRecent || datesDistinguish(notes), now, lang)
	switch sort {
	case SortByRecent:
		return monthGroups(notes, rows, now, lang)
	case SortByOrder:
		return []ListGroup{{Rows: rows}}
	case SortByName:
	}
	return prefixGroups(rows)
}

// noteListRows draws notes already in order as rows.
func noteListRows(notes []ListNote, showDate bool, now time.Time, lang wording.Lang) []ListRow {
	rows := make([]ListRow, len(notes))
	for i := range notes {
		rows[i] = noteRow(&notes[i], showDate, now, lang)
	}
	return rows
}

func orderNotes(notes []ListNote, sort ListSort) {
	c := nameCollator()
	byName := func(a, b ListNote) int {
		if r := c.CompareString(a.Title, b.Title); r != 0 {
			return r
		}
		return vault.ComparePaths(a.RelPath, b.RelPath)
	}
	if sort == SortByOrder {
		slices.SortStableFunc(notes, func(a, b ListNote) int {
			an, aok := fileNumber(a.RelPath)
			bn, bok := fileNumber(b.RelPath)
			switch {
			case aok && bok:
				if r := compareDigits(an, bn); r != 0 {
					return r
				}
				return vault.ComparePaths(a.RelPath, b.RelPath)
			case aok:
				return -1
			case bok:
				return 1
			}
			return byName(a, b)
		})
		return
	}
	if sort == SortByRecent {
		slices.SortStableFunc(notes, func(a, b ListNote) int {
			if r := b.Modified.Compare(a.Modified); r != 0 {
				return r
			}
			return byName(a, b)
		})
		return
	}
	slices.SortStableFunc(notes, byName)
}

// minOrderedNotes is how many notes a folder needs before the numbers its file
// names open with can be an order its author gave.
const minOrderedNotes = 3

// fileNumber reads the number a file name opens with: digits followed by a
// hyphen, an underscore, a dot or a space.
func fileNumber(relPath string) (digits string, ok bool) {
	name := relPath[strings.LastIndex(relPath, "/")+1:]
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	if end == 0 || end == len(name) || !strings.ContainsRune("-_. ", rune(name[end])) {
		return "", false
	}
	return strings.TrimLeft(name[:end], "0"), true
}

// compareDigits compares two numbers written without leading zeros by value,
// however many digits they have.
func compareDigits(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

// hasAuthorOrder says whether a folder's own notes are numbered by their
// author: at least three of them, and more than half open their file name with
// a number.
func hasAuthorOrder(notes []ListNote) bool {
	if len(notes) < minOrderedNotes {
		return false
	}
	numbered := 0
	for i := range notes {
		if _, ok := fileNumber(notes[i].RelPath); ok {
			numbered++
		}
	}
	return numbered*2 > len(notes)
}

// datesDistinguish says whether a date column tells these rows apart: a list
// of ten or fewer always does, and a longer one does once its rows fall on
// three different days.
func datesDistinguish(notes []ListNote) bool {
	if len(notes) <= distinguishingRows {
		return true
	}
	days := map[string]bool{}
	for i := range notes {
		days[notes[i].Modified.Format(time.DateOnly)] = true
		if len(days) >= 3 {
			return true
		}
	}
	return false
}

func noteRow(n *ListNote, showDate bool, now time.Time, lang wording.Lang) ListRow {
	row := ListRow{Title: n.Title, Href: notesHref(n.RelPath), Language: n.Language, Draft: n.Draft, DraftWord: wording.Draft.In(lang)}
	if showDate && !n.Modified.IsZero() {
		row.Fact = listDate(n.Modified, now)
		row.FactTime = n.Modified.Format(time.DateOnly)
	}
	return row
}

// minPrefixTitles is how many titles must open with the same prefix before it
// is a heading rather than a coincidence.
const minPrefixTitles = 3

// prefixGroups lifts a prefix shared by at least three titles — the words up
// to a 「：」「｜」 or 「:」 — out of the rows and onto a heading, so the rows keep
// only what tells them apart. Rows with no shared prefix stay in the opening
// group, which has no heading of its own.
func prefixGroups(rows []ListRow) []ListGroup {
	type split struct{ key, heading, rest string }
	splits := make([]split, len(rows))
	shared := map[string]int{}
	for i := range rows {
		splits[i] = splitPrefix(rows[i].Title)
		if splits[i].key != "" {
			shared[splits[i].key]++
		}
	}
	var loose ListGroup
	grouped := map[string]*ListGroup{}
	var order []string
	for i := range rows {
		sp := splits[i]
		if sp.key == "" || shared[sp.key] < minPrefixTitles {
			loose.Rows = append(loose.Rows, rows[i])
			continue
		}
		g, seen := grouped[sp.key]
		if !seen {
			g = &ListGroup{Heading: sp.heading, Authored: true}
			grouped[sp.key] = g
			order = append(order, sp.key)
		}
		rows[i].Title = sp.rest
		g.Rows = append(g.Rows, rows[i])
	}
	var groups []ListGroup
	if len(loose.Rows) > 0 {
		groups = append(groups, loose)
	}
	for _, key := range order {
		g := *grouped[key]
		g.Count = Thousands(len(g.Rows))
		groups = append(groups, g)
	}
	return groups
}

// splitPrefix reads a title's prefix: the words before its first 「：」「｜」 or
// 「:」. The key is the prefix with its mark, so two titles share a prefix only
// when both are spelled the same; it is empty when the title has no prefix or
// nothing follows it.
func splitPrefix(title string) (p struct{ key, heading, rest string }) {
	at := strings.IndexAny(title, "：｜:")
	if at <= 0 {
		return p
	}
	_, size := utf8.DecodeRuneInString(title[at:])
	rest := strings.TrimSpace(title[at+size:])
	heading := strings.TrimSpace(title[:at])
	if rest == "" || heading == "" {
		return p
	}
	return struct{ key, heading, rest string }{title[:at+size], heading, rest}
}

// monthGroups heads a long list sorted by date with one heading per month, and
// leaves a short one as a single run: twenty rows are one glance.
func monthGroups(notes []ListNote, rows []ListRow, now time.Time, lang wording.Lang) []ListGroup {
	if len(rows) <= manyRows {
		return []ListGroup{{Rows: rows}}
	}
	var groups []ListGroup
	var last string
	for i := range rows {
		key := notes[i].Modified.Format("2006-01")
		if key != last {
			groups = append(groups, ListGroup{Heading: monthHeading(notes[i].Modified, now, lang)})
			last = key
		}
		g := &groups[len(groups)-1]
		g.Rows = append(g.Rows, rows[i])
	}
	return groups
}

func monthHeading(at, now time.Time, lang wording.Lang) string {
	if at.IsZero() {
		return ""
	}
	if at.Year() != now.Year() {
		return at.Format("2006-01")
	}
	if lang == wording.En {
		return at.Month().String()
	}
	return fmt.Sprintf(wording.MonthHeading.In(lang), int(at.Month()))
}

// FolderRows turns folders into rows whose fact is how many notes sit under
// each, or the day of the latest change when the list is read by date.
func FolderRows(folders []ListFolder, sort ListSort, now time.Time, lang wording.Lang) []ListRow {
	folders = SortFolders(folders, sort)
	rows := make([]ListRow, len(folders))
	for i := range folders {
		f := &folders[i]
		rows[i] = ListRow{Title: f.Name, Href: folderHref(f.RelPath), Fact: noteCountFact(f.Notes, lang)}
		if sort == SortByRecent && !f.Latest.IsZero() {
			rows[i].Fact, rows[i].FactTime = listDate(f.Latest, now), f.Latest.Format(time.DateOnly)
		}
	}
	return rows
}

// SortFolders orders folders by name, or by the newest change under each.
func SortFolders(folders []ListFolder, sort ListSort) []ListFolder {
	folders = slices.Clone(folders)
	c := nameCollator()
	slices.SortStableFunc(folders, func(a, b ListFolder) int {
		if sort == SortByRecent {
			if r := b.Latest.Compare(a.Latest); r != 0 {
				return r
			}
		}
		return c.CompareString(a.Name, b.Name)
	})
	return folders
}

// noteCountFact is a count of notes with its unit and, past 999, its
// thousands separator.
func noteCountFact(n int, lang wording.Lang) string {
	return counted(n, wording.FolderNoteCountOne, wording.FolderNoteCountMany, lang)
}

// counted says a number with its unit, the number carrying its thousands
// separator.
func counted(n int, one, many wording.Phrase, lang wording.Lang) string {
	phrase := many
	if n == 1 {
		phrase = one
	}
	return strings.Replace(phrase.In(lang), "%d", Thousands(n), 1)
}

// Thousands writes a count with its thousands separator once it passes 999.
func Thousands(n int) string {
	s := strconv.Itoa(n)
	if n < 1000 {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Address writes the query that shows a list in the given order, filter and
// unfolding, leaving out what is the page's default so a plain page keeps a
// plain address.
func (q ListQuery) Address(sort ListSort, filter string, expand bool) string {
	v := url.Values{}
	if sort != q.defaultSort() {
		v.Set("sort", string(sort))
	}
	if filter != "" {
		v.Set("filter", filter)
	}
	if expand {
		v.Set("expand", "1")
	}
	if len(v) == 0 {
		return ""
	}
	return "?" + v.Encode()
}

// ListSwitchFor is the order choice for a page: which order it is in, and the
// words for the by-name choice (a folder is "by name", a note "by title").
// A folder its author numbered offers its own order first.
func (q ListQuery) ListSwitchFor(base string, expand, ordered bool, nameWord wording.Phrase, lang wording.Lang) ListSwitch {
	var choices []ListChoice
	add := func(sort ListSort, word wording.Phrase) {
		choices = append(choices, ListChoice{Href: base + q.Address(sort, q.Filter, expand), Text: word.In(lang), Pressed: q.Sort == sort})
	}
	if ordered {
		add(SortByOrder, wording.SortByOrder)
	}
	add(SortByName, nameWord)
	add(SortByRecent, wording.SortByRecent)
	return ListSwitch{Choices: choices}
}

// mark finds the first occurrence of word in a title, ignoring case, and
// splits the title around it so the template can draw the found part heavier.
// It reports whether the title holds the word at all, which is also how a
// filter decides what stays.
func (r *ListRow) mark(word string) bool {
	if word == "" {
		return true
	}
	runes, want := []rune(r.Title), len([]rune(word))
	for i := 0; i+want <= len(runes); i++ {
		if strings.EqualFold(string(runes[i:i+want]), word) {
			r.Before, r.Match, r.After = string(runes[:i]), string(runes[i:i+want]), string(runes[i+want:])
			return true
		}
	}
	return false
}
