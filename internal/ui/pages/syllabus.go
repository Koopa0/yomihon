package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/wording"
)

// PathView is everything the study-path page needs: the current path's branches
// with their anchors, the switcher across every study path, and the figures the
// header metarow reads. Navigation has already classified every
// entry, so the page never resolves a wikilink.
type PathView struct {
	Title     string
	RelPath   string
	GuideHref string
	// ListenHref is the same path as something to be listened to, and is empty
	// where nothing in it is marked to be read aloud: an offer whose page says
	// there is nothing to hear is a dead end, so it is not made.
	ListenHref string
	Paths      []PathLink
	Branches   []PathBranchView
	// Appendix is what the author kept out of the course and listed beneath
	// it: each section declared out of the sequence, in document order. It
	// carries no counts and no order, and nothing in it is a stop on the walk.
	Appendix []PathBranchView

	// Unit is the noun the path's entries are counted and stepped in.
	Unit nav.Unit
	// Entries is the course's planned total, the main line only and a
	// planned-but-unwritten entry included, and Branched how many the side
	// branches list between them, which never join it. Unsettled is how many of
	// the main line's entries carry a status the contract does not settle: what
	// is still to be finished, which says nothing about how far the reader has
	// come. It is zero when every entry is settled, and for a contract that
	// settles no status, where it would say nothing.
	Entries   int
	Branched  int
	Unsettled int

	// Vault is the folder this course sits in, which the rail's foot states.
	// The reading rail builds one of these views for its own book and states
	// the folder from its own value, so this one is filled where the page is
	// assembled rather than by the builder the two share.
	Vault nav.Vault

	// NoCourse is which explanation the empty-course page is entitled to give:
	// that a written sequence marker is among what the grammar reported, that
	// none is, or that the report holds something this page cannot explain.
	NoCourse markerVerdict

	// OpeningHTML is the map note's own opening, already rendered: the words
	// its author wrote above its first heading, saying what the course is and
	// who it is for. Empty for a note that opens straight on a heading, and the
	// page then prints nothing rather than an empty box — the course has no
	// opening of its own and inventing one would put words in the author's
	// mouth.
	OpeningHTML string
	// OpeningLanguage is the tag that note declared, carried so the opening is
	// announced in the language it was written in rather than inheriting the
	// page's. Empty is the note's own silence, never a guess.
	OpeningLanguage string
	// Action is the one verb the cover offers.
	Action CourseAction
}

// CourseCover is what a request can tell the page about itself, beside the
// course: the map note's own opening, where the reader is standing, and the
// place they kept. It is three strings and a language rather than the records
// they were read out of — a view has no business holding a reader's mark or a
// generation — and each is compared against, or printed by, the course alone.
type CourseCover struct {
	// OpeningHTML and OpeningLanguage become the view's own fields of those
	// names; the handler renders the opening through the generation the rest of
	// the page was built from.
	OpeningHTML     string
	OpeningLanguage string
	// Here is the vault-relative note the reader opened this course from, or
	// empty. It marks the row they are standing on and nothing else.
	Here string
	// KeptNote is the vault-relative note the reader kept a place in, empty
	// where they kept none. It is matched against the course's own rows the way
	// Here is, so a place kept in a note this course does not teach leaves the
	// verb where it was and marks no row.
	KeptNote string
	// KeptHref is where that place is, already built. The course decides
	// whether to offer it, never where it leads.
	KeptHref string
	// Listenable is whether anything in the course is marked to be read aloud,
	// which is the one thing that decides whether the cover offers the page
	// that plays it.
	Listenable bool
}

// CourseAction is the one verb a course cover offers: open a lesson. Which
// lesson, and which word, are the same decision — start at the course's first
// stop, or go back to the place the reader kept inside it — so one value owns
// both, and a course with nowhere to send anyone carries an empty Href and is
// not drawn.
//
// It says nothing about how much of the course is behind the reader. There is
// no such reading here: a kept place is a position, and a course that has never
// been opened and one nearly finished offer the same single verb.
type CourseAction struct {
	// Href is where the verb leads: the first lesson, or the kept place.
	Href string
	// Continuing is whether that lesson is the one the reader kept a place in.
	Continuing bool
	// Lesson is that lesson's own words. It stands beside the verb only where
	// the reader is being sent back into the middle of the course, because the
	// first lesson is the next thing on the page and naming it twice says
	// nothing; a lesson somewhere inside it cannot be found by looking.
	Lesson string
	// LessonLanguage is the language that lesson's note declared, empty where
	// it declared none, so its name is read in its own language.
	LessonLanguage string
}

// Token is the machine word the markup carries for which verb this is. It
// stays English and is what a check reads, so a lock on the page's behaviour
// does not turn on the reader's own language.
func (a CourseAction) Token() string {
	if a.Continuing {
		return "continue"
	}
	return "start"
}

// Verb is the reader's own word for it.
func (a CourseAction) Verb(lang wording.Lang) string {
	if a.Continuing {
		return wording.CourseContinueReading.In(lang)
	}
	return wording.CourseStartReading.In(lang)
}

// PathBranchView is one branch of the course as the page draws it. A top-level
// branch is a part, carrying the anchor the rail jumps to and its number; a
// nested one is drawn under its parent by its heading alone. Items hold what the
// branch lists in source order. A side branch hangs from the row it was nested
// under, which is how the author attached it, and is carried by that row.
type PathBranchView struct {
	Anchor string
	// Ordinal is a part's number as a roman numeral, the compact form the
	// rail's jump list uses. The cover's own heading says it in words with
	// PartLabel, which is language dependent.
	Ordinal string
	// Num is a part's position among the parts, from one. Zero for a nested
	// branch, which has no number of its own.
	Num     int
	Heading string
	Depth   int
	// Local marks a side branch: its own order, never part of the main line.
	Local bool
	Items []PathItemView
}

// PartLabel is a part's number as the cover's heading says it: a Chinese
// heading reads 第一部 and an English one Part I. The numeral is the language's
// own — Chinese counts parts in hanzi, as a book's table of contents does.
func (v *PathBranchView) PartLabel(lang wording.Lang) string {
	return fmt.Sprintf(wording.PartLabelFmt.In(lang), partNumeral(v.Num, lang))
}

// partNumeral is a part's number in the form the language's headings use: a
// roman numeral in English, hanzi in Chinese. A number past what hanzi are
// written for here, or one that is not positive, falls back to its digits
// rather than panicking.
func partNumeral(n int, lang wording.Lang) string {
	if lang == wording.En {
		return roman(n)
	}
	return hanziNumeral(n)
}

// hanziNumeral writes 1 through 99 the way a Chinese heading counts: 十 stands
// alone for ten, and a leading 一 is dropped before 十.
func hanziNumeral(n int) string {
	const digits = "零一二三四五六七八九"
	runes := []rune(digits)
	switch {
	case n < 1 || n > 99:
		return strconv.Itoa(n)
	case n < 10:
		return string(runes[n])
	case n == 10:
		return "十"
	case n < 20:
		return "十" + string(runes[n-10])
	case n%10 == 0:
		return string(runes[n/10]) + "十"
	default:
		return string(runes[n/10]) + "十" + string(runes[n%10])
	}
}

// PathItemView is one thing a branch lists: a row, or a nested branch. A value
// carrying neither is drawn as a fault rather than dropped, so the course still
// reads as the length its author wrote.
type PathItemView struct {
	Entry  *PathEntryView
	Branch *PathBranchView
}

// PathEntryView is one linked or warning row. Only resolved rows have an href, a
// status, a marker for the reader's own place, or a language. Number is copied from navigation's
// walk, the one owner of sequence position, and zero means the walk never
// reaches the row.
type PathEntryView struct {
	Name string
	// RelPath is the note this row reached, empty for a row that reached none.
	// It is the row's own identity: the href and the two marks below are read
	// off it, and the cover's verb asks which row is the note a reader kept a
	// place in.
	RelPath string
	Href    string
	Status  string
	// Settled is whether the contract declares Status settled, the resting
	// state of a finished lesson. The row prints its status only when it is
	// not, so what a reader sees on a row is the exception.
	Settled bool
	// Here marks the lesson the reader is at. It says where they are standing,
	// never how far they have come: the words beside the row stay the
	// contract's, and a course carries no reading of its own about which
	// lessons are behind the reader.
	Here bool
	// Kept marks the lesson holding the place the reader deliberately left off
	// at. Like Here it says where they are, and never how much of the course is
	// behind them.
	Kept   bool
	Kind   nav.EntryKind
	Number int
	// Language is the tag the note this row reached declared, carried so a
	// surface can stamp the title it prints rather than leaving it to inherit
	// the page. It is that note's own answer or empty, never a guess: a row
	// that reached no note has nothing to have read a declaration from.
	Language string
	// Gloss is what the course's author wrote after the row's link, run in
	// after the title as they typed it, and GlossLanguage the tag the course's
	// own note declared, because the sentence is that author's and not the
	// lesson's. Only the cover prints it.
	Gloss         string
	GlossLanguage string
	// Branches are the side branches hanging from this row, in the order the
	// author wrote them. They are drawn under the row they were nested under,
	// inside the same list item, so the line the main line is read along
	// passes beside them instead of stopping where they begin.
	Branches []PathBranchView
}

// labelled reports whether the row prints its status: only an exception does,
// a lesson at a status the contract settles being the ordinary state.
func (e *PathEntryView) labelled() bool {
	return e.Status != "" && !e.Settled
}

// PathRunView is one uninterrupted stretch of what a branch lists: a run of
// rows, one nested branch, or a fault standing where the page could read
// neither. Ordered marks a run whose rows carry sequence numbers, so the page
// can render a real ordered list; Label is that list's accessible name.
type PathRunView struct {
	Entries []PathEntryView
	Branch  *PathBranchView
	Ordered bool
	Label   string
	// Fault is what the page says in place of an item it could not read. It
	// keeps the item's position, so the course still reads as the length its
	// author wrote.
	Fault string
}

// Runs regroups a branch's items for rendering: consecutive rows form one run,
// and each nested branch stands alone, in document order. It is derived on
// demand so Items stays the branch's single stored form.
//
// An ordered run is named for the sequence component it belongs to, marking a
// fragment that resumes after an interruption. Assistive technology otherwise
// announces every fragment as an anonymous list, and a course split by its own
// headings becomes several indistinguishable ones; nothing visible names them.
func (v *PathBranchView) Runs(lang wording.Lang) []PathRunView {
	var runs []PathRunView
	for _, item := range v.Items {
		switch {
		case item.Entry != nil:
			if len(runs) == 0 || !runs[len(runs)-1].holdsRows() {
				run := PathRunView{Ordered: item.Entry.Number > 0}
				if run.Ordered {
					run.Label = v.runLabel(item.Entry.Number, lang)
				}
				runs = append(runs, run)
			}
			last := &runs[len(runs)-1]
			last.Entries = append(last.Entries, *item.Entry)
		case item.Branch != nil:
			runs = append(runs, PathRunView{Branch: item.Branch})
		default:
			// The item names neither a row nor a branch, so it is a fault
			// standing in the course where the author put something.
			runs = append(runs, PathRunView{Fault: wording.PathItemUnreadable.In(lang)})
		}
	}
	return runs
}

// holdsRows reports whether another row can join this run. A run standing for a
// nested branch or an unreadable item is closed, the interruption being what the
// reader sees between them.
func (r *PathRunView) holdsRows() bool {
	return r.Branch == nil && r.Fault == ""
}

// runLabel names one ordered fragment. first is the fragment's first walk
// number: one means the component opens here, anything later means the
// fragment resumes an already-open order.
func (v *PathBranchView) runLabel(first int, lang wording.Lang) string {
	if v.Local {
		if first > 1 {
			return wording.BranchPrefix.In(lang) + v.Heading + wording.BranchContinued.In(lang)
		}
		return wording.BranchPrefix.In(lang) + v.Heading
	}
	if first > 1 {
		return wording.MainContinued.In(lang)
	}
	return wording.MainLine.In(lang)
}

// PathLink is one entry in the path switcher: a study path's title, the URL to
// its page, its planned lesson count, and whether it is the one shown.
type PathLink struct {
	Title   string
	RelPath string
	Entries int
	Active  bool
}

// BuildPathView draws one study path's declared structure into the page view and
// builds the switcher from every study path in the vault. It draws what the
// grammar lets navigation read and nothing else; a branch outside the course
// keeps its prose on the note's own page.
//
// cover is what the request brought with it. Its Here is the vault-relative
// note the reader is at, or empty when nothing said; it is matched against the
// course's own resolved rows, so a note this course does not list marks
// nothing, and so does a reader who arrived from the desk. A course that lists
// the same note twice marks it twice: both rows are that note, and choosing
// between them would be a guess.
func BuildPathView(current *nav.Path, all []nav.Path, cover *CourseCover) PathView {
	v := PathView{
		Title:           current.Title,
		RelPath:         current.RelPath,
		GuideHref:       notesHref(current.RelPath),
		Paths:           buildPaths(current.RelPath, all),
		Unit:            current.Unit,
		Entries:         current.Planned,
		Branched:        current.Branched,
		Unsettled:       current.Unsettled,
		OpeningHTML:     cover.OpeningHTML,
		OpeningLanguage: cover.OpeningLanguage,
	}
	if cover.Listenable {
		v.ListenHref = VaultHref("/listen/", current.RelPath)
	}
	// A written marker outranks the rest, and an unrecognised rule outranks
	// the no-marker reading, which would otherwise put words in the author's
	// mouth on the strength of not knowing a rule name.
	for _, d := range current.Diagnostics {
		switch verdict := markerVerdictFor(d.Rule); verdict {
		case markerWritten:
			v.NoCourse = markerWritten
		case markerUnknownRule:
			if v.NoCourse == markerNotWritten {
				v.NoCourse = verdict
			}
		case markerNotWritten:
		}
		if v.NoCourse == markerWritten {
			break
		}
	}
	for _, g := range current.Groups {
		if appendix, ok := buildAppendix(g, cover); ok {
			v.Appendix = append(v.Appendix, appendix)
			continue
		}
		sv, ok := buildPathBranch(g, 0, len(v.Branches)+1, cover)
		if !ok {
			continue
		}
		v.Branches = append(v.Branches, sv)
	}
	v.Action = courseAction(v.Branches, cover)
	return v
}

// unitWords is the interface's words for the noun this path is read in.
func unitWords(unit nav.Unit) wording.UnitWords {
	switch unit {
	case nav.UnitItem:
		return wording.ItemWords
	case nav.UnitLesson:
		return wording.LessonWords
	default:
		return wording.LessonWords
	}
}

// Extent is what the path says about its own size, one figure per phrase: the
// course's total in the noun it is read in, then, where side branches exist,
// how many entries they list between them. The branches' figure stands beside
// the total and never inside it. The cover prints it under the title and the
// reading rail under the book's name, so the two cannot count differently.
func (v *PathView) Extent(lang wording.Lang) []string {
	words := unitWords(v.Unit)
	figures := []string{plural(v.Entries, words.CountOne, words.CountMany, lang)}
	if v.Branched > 0 {
		figures = append(figures, fmt.Sprintf(words.BranchedFmt.In(lang), v.Branched))
	}
	return figures
}

// UnsettledNote is the exception the head states after the extent, in the noun
// the path is read in, or empty where there is none to state.
func (v *PathView) UnsettledNote(lang wording.Lang) string {
	if v.Unsettled == 0 {
		return ""
	}
	return fmt.Sprintf(unitWords(v.Unit).UnsettledFmt.In(lang), v.Unsettled)
}

// buildAppendix draws a top-level section the author declared out of the
// sequence as the list of rows it holds, each with its gloss. ok is false for
// any other branch, and for one declared out that lists no row, which has
// nothing to put under a heading. A branch carrying a structural error keeps
// to the page's note, as it does everywhere: nothing about it is read.
func buildAppendix(g *nav.PathGroup, cover *CourseCover) (PathBranchView, bool) {
	if g.Role != sequence.RoleNone || g.Invalid {
		return PathBranchView{}, false
	}
	sv := PathBranchView{Heading: g.Name}
	for _, item := range g.Items {
		if item.Entry == nil || item.Entry.State != sequence.EntryAccepted {
			continue
		}
		entry := buildPathEntry(item.Entry, cover)
		sv.Items = append(sv.Items, PathItemView{Entry: &entry})
	}
	return sv, len(sv.Items) > 0
}

// courseAction settles the cover's one verb against the course as drawn.
//
// Going back to a kept place wins wherever the course lists the note that place
// is in — on any branch it draws, a side branch included, because a lesson this
// course teaches is one of its lessons wherever the author hung it. Otherwise
// the verb starts the course at its first stop: the first linked row on the
// main line, in document order, which is the order the walk numbers it in. The
// main line and not a side branch, because starting a course means its first
// lesson and a branch hangs off the middle of one; a row that reached no note,
// or one the walk never reaches, is not a stop anyone can be sent to.
//
// A course whose main line links nothing gets no verb at all, and the page
// draws none: an offer to start something the page cannot open is worse than
// the parts standing on their own.
func courseAction(branches []PathBranchView, cover *CourseCover) CourseAction {
	if cover.KeptNote != "" && cover.KeptHref != "" {
		if kept := firstLesson(branches, false, func(entry *PathEntryView, _ bool) bool {
			return entry.RelPath == cover.KeptNote
		}); kept != nil {
			return CourseAction{Href: cover.KeptHref, Continuing: true, Lesson: kept.Name, LessonLanguage: kept.Language}
		}
	}
	first := firstLesson(branches, false, func(entry *PathEntryView, local bool) bool {
		return !local && entry.Number > 0
	})
	if first == nil {
		return CourseAction{}
	}
	return CourseAction{Href: first.Href}
}

// firstLesson walks the drawn course in document order and answers with the
// first linked row want accepts, or nil. Only a linked row is put to want, so
// no caller has to remember to ask again.
func firstLesson(branches []PathBranchView, local bool, want func(entry *PathEntryView, local bool) bool) *PathEntryView {
	for i := range branches {
		if found := firstLessonIn(&branches[i], local, want); found != nil {
			return found
		}
	}
	return nil
}

// firstLessonIn is that walk inside one branch. local says whether this branch
// hangs off the main line, and travels down: a branch nested under a side
// branch is a side branch too, whatever its own marker says.
func firstLessonIn(branch *PathBranchView, local bool, want func(entry *PathEntryView, local bool) bool) *PathEntryView {
	aside := local || branch.Local
	for i := range branch.Items {
		item := &branch.Items[i]
		switch {
		case item.Entry != nil:
			if item.Entry.Href != "" && want(item.Entry, aside) {
				return item.Entry
			}
			// What hangs from a row follows it, which is the order the author
			// wrote them in.
			for j := range item.Entry.Branches {
				if found := firstLessonIn(&item.Entry.Branches[j], aside, want); found != nil {
					return found
				}
			}
		case item.Branch != nil:
			if found := firstLessonIn(item.Branch, aside, want); found != nil {
				return found
			}
		}
	}
	return nil
}

// buildPathBranch converts one projectable branch and its drawable subtree into
// a view. ok is false for a branch the course excludes that carries no declared
// branch beneath it; a structural heading still draws, since dropping it would
// orphan its parts. Sequence position is copied from navigation's walk.
//
// A side branch is carried by the row it was nested under. The row is found by
// the identity navigation recorded for it, the span its source occupies, and
// never by its name, since two rows can name one note. A side branch whose row
// is not drawn here stays an item of its own, where the author wrote it.
func buildPathBranch(g *nav.PathGroup, depth, num int, cover *CourseCover) (PathBranchView, bool) {
	if !g.Drawn() {
		return PathBranchView{}, false
	}
	sv := PathBranchView{
		Heading: g.Name,
		Depth:   depth,
		Num:     num,
		Local:   g.Role == sequence.RoleLocal,
	}
	if depth == 0 {
		sv.Anchor = "part-" + strconv.Itoa(num)
		sv.Ordinal = roman(num)
	}
	// anchors maps a drawn row's source span to its place among the items, so
	// a side branch can be handed to the row it hangs from.
	anchors := map[sequence.Span]int{}
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			if !g.Teaches(item.Entry) {
				continue
			}
			entry := buildPathEntry(item.Entry, cover)
			anchors[item.Entry.Span] = len(sv.Items)
			sv.Items = append(sv.Items, PathItemView{Entry: &entry})
		case item.Group != nil:
			child, ok := buildPathBranch(item.Group, depth+1, 0, cover)
			if !ok {
				continue
			}
			if at, hangs := anchors[item.Group.AnchorSpan]; hangs && child.Local {
				row := sv.Items[at].Entry
				row.Branches = append(row.Branches, child)
				continue
			}
			sv.Items = append(sv.Items, PathItemView{Branch: &child})
		}
	}
	return sv, true
}

// buildPathEntry maps one nav entry onto a linked or warning study-path row. The
// number is copied for every row, so a planned lesson keeps its place; the
// language travels with the rest of what a resolved target answered, because a
// row that resolved to nothing read no note and so carries no declaration.
//
// Only a resolved row can be the one the reader is at or the one holding their
// kept place: a row that reached no note is not a note anyone can have been
// reading or left off in.
func buildPathEntry(entry *nav.PathEntry, cover *CourseCover) PathEntryView {
	v := PathEntryView{
		Name:          entry.Name,
		Kind:          entry.Kind,
		Number:        entry.Number,
		Language:      entry.Language,
		Gloss:         entry.Gloss,
		GlossLanguage: cover.OpeningLanguage,
	}
	if entry.Kind != nav.EntryResolved {
		return v
	}
	v.RelPath = entry.RelPath
	v.Href = notesHref(entry.RelPath)
	v.Status = entry.Status
	v.Settled = entry.Settled
	v.Here = cover.Here != "" && entry.RelPath == cover.Here
	v.Kept = cover.KeptNote != "" && entry.RelPath == cover.KeptNote
	return v
}

// hereAttr marks the row the reader is standing on. The value is "location"
// rather than "page": the page is the whole course, and the row is where the
// reader is inside it, which is what a link to somewhere else can truthfully
// claim. A row that is not the one contributes nothing.
func hereAttr(here bool) templ.Attributes {
	if here {
		return templ.Attributes{"aria-current": "location"}
	}
	return nil
}

// A row says three things about how its target resolved: the words a reader
// sees, the token the markup carries, and the explanation behind the row. A kind
// none of them answers for is reported as its number rather than aborting — a
// kind added in navigation compiles here and must not take down every page.
func entryResolutionLabel(kind nav.EntryKind, lang wording.Lang) string {
	switch kind {
	case nav.EntryUnresolved:
		return wording.EntryUnresolved.In(lang)
	case nav.EntryAmbiguous:
		return wording.EntryAmbiguous.In(lang)
	case nav.EntryNonInstance:
		return wording.EntryNonInstance.In(lang)
	default:
		return entryResolutionCode(kind)
	}
}

// entryResolutionCode is the stable machine token carried by data-resolution. It
// stays English; entryResolutionLabel owns the reader's own words. The token
// itself belongs to navigation, which classified the entry, so this asks for it
// rather than keeping a copy: a copy goes on stamping the old spelling after the
// outcome is renamed, and the rail looks right while it says something the rest
// of the program no longer says.
func entryResolutionCode(kind nav.EntryKind) string {
	token, known := kind.Token()
	if !known {
		return strconv.Itoa(int(kind))
	}
	return token
}

func entryResolutionTitle(kind nav.EntryKind, lang wording.Lang) string {
	switch kind {
	case nav.EntryUnresolved:
		return wording.EntryUnresolvedTitle.In(lang)
	case nav.EntryAmbiguous:
		return wording.EntryAmbiguousTitle.In(lang)
	case nav.EntryNonInstance:
		return wording.EntryNonInstanceTitle.In(lang)
	default:
		// A resolved row has nothing to explain, and neither has a kind this
		// page has no words for: the row already carries its token.
		return ""
	}
}

// markerVerdict is what one grammar rule lets the page say about the author of
// the note it came from.
type markerVerdict uint8

const (
	// markerNotWritten is a rule that arises with no sequence marker near it,
	// so the page may say the note carries none.
	markerNotWritten markerVerdict = iota
	// markerWritten is a rule that arises only where a marker was written.
	markerWritten
	// markerUnknownRule is a rule this page has not been told about, and so is
	// no evidence for either claim about what somebody wrote in a file.
	markerUnknownRule
)

// markerVerdictFor classifies one grammar rule. The division is total over the
// grammar's declared rules; the third answer exists for a rule the grammar
// gains later, which a course page is no place to abort on.
func markerVerdictFor(rule sequence.Rule) markerVerdict {
	switch rule {
	case sequence.RuleRoleInvalid,
		sequence.RuleRoleDuplicate,
		sequence.RuleRoleMisplaced,
		sequence.RuleRoleConflict,
		sequence.RuleRoleNestedPrimary,
		sequence.RuleRoleOnEntry,
		sequence.RuleLocalOrphan,
		sequence.RuleNestingTooDeep:
		return markerWritten
	case sequence.RuleRoleMissing,
		sequence.RuleEntryOutsideBranch,
		sequence.RuleEntryMultiTarget,
		sequence.RuleEntryNoncanonical:
		return markerNotWritten
	default:
		return markerUnknownRule
	}
}

// buildPaths builds the switcher: every study path in vault order, each with its
// entry count and whether it is the one shown.
func buildPaths(currentRel string, all []nav.Path) []PathLink {
	links := make([]PathLink, 0, len(all))
	for i := range all {
		s := &all[i]
		links = append(links, PathLink{
			Title:   s.Title,
			RelPath: s.RelPath,
			Entries: s.Planned,
			Active:  s.RelPath == currentRel,
		})
	}
	return links
}

// roman renders a positive part number as a roman numeral. A non-positive n
// falls back to its decimal form rather than panicking.
func roman(n int) string {
	if n < 1 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}
