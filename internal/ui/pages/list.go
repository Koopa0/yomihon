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
)

// ParseListSort reads the sort a request asked for; a word outside the set is
// the default.
func ParseListSort(value string) ListSort {
	if value == string(SortByRecent) {
		return SortByRecent
	}
	return SortByName
}

// manyRows is how long a list may be before it earns a filter and, sorted by
// date, a heading for each month.
const manyRows = 20

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
	Count    string
	Rows     []ListRow
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
	// Label says how much there is when the page has not already.
	Label  string
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

// NoteRows applies the list rule to the notes of one folder: order, the
// shared-prefix groups, the month groups, and the one fact a row carries.
func NoteRows(notes []ListNote, sort ListSort, now time.Time, lang wording.Lang) []ListGroup {
	notes = slices.Clone(notes)
	orderNotes(notes, sort)
	rows := noteListRows(notes, sort == SortByRecent || datesDistinguish(notes), now, lang)
	if sort == SortByRecent {
		return monthGroups(notes, rows, now, lang)
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

// listQuery writes the page's query with the parts that differ from its
// defaults, so a plain page keeps a plain address.
func listQuery(sort ListSort, filter string, expand bool) string {
	q := url.Values{}
	if sort != SortByName {
		q.Set("sort", string(sort))
	}
	if filter != "" {
		q.Set("filter", filter)
	}
	if expand {
		q.Set("expand", "1")
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// ListSwitchFor is the order choice for a page: which order it is in, and the
// words for the by-name choice (a folder is "by name", a note "by title").
func ListSwitchFor(base string, sort ListSort, filter string, expand bool, nameWord wording.Phrase, lang wording.Lang) ListSwitch {
	return ListSwitch{Choices: []ListChoice{
		{Href: base + listQuery(SortByName, filter, expand), Text: nameWord.In(lang), Pressed: sort == SortByName},
		{Href: base + listQuery(SortByRecent, filter, expand), Text: wording.SortByRecent.In(lang), Pressed: sort == SortByRecent},
	}}
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
