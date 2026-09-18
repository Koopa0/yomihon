package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// The four kinds of thing a reader comes in through, named once. The value is
// stamped on the page so a check can name the surface it is looking at without
// depending on the words, which change with the reader's language.
const (
	pathMode   = "paths"
	mapMode    = "maps"
	reportMode = "reports"
	folderMode = "folders"
)

// ListIndexView is one mode's index page: the page's own chrome, and the shelf
// it unfolds. Fault is why a projection was withheld, stated once for the page;
// a withheld projection lists nothing, and its shelf then says neither how much
// it holds nor that it holds none, because a vault whose declaration could not
// be read is not a vault that declared nothing.
type ListIndexView struct {
	Mode   string
	Kicker string
	Fault  string
	Shelf  Shelf
}

// RecentBlock is what changed last, and the words the list is entitled to. It
// belongs to the folder mode rather than to the desk because it describes how
// the files are kept, which is a different question from what there is to read.
//
// The heading and the sentence are chosen together, because they answer one
// question between them: what this list may promise. They are chosen once, at
// the moment the notes are known, so no later reader has to work out which
// pairing applies.
type RecentBlock struct {
	Title string
	Lede  string
	Notes []HomeNote
}

// NewRecentBlock builds that list with the words it can stand behind. A fresh
// clone stamps every file with one moment: where the recorded times separate
// nothing, the block says so instead of promising recency. Where the contract
// declares a knowledge layer the list covers that layer and the sentence names
// it, because the distribution beside it counts every indexed note, and two
// true figures over unnamed sets read as a contradiction.
func NewRecentBlock(notes []HomeNote, ordered, scoped bool, lang wording.Lang) RecentBlock {
	block := RecentBlock{Title: wording.FolderTiedTitle.In(lang), Notes: notes}
	switch {
	case ordered && scoped:
		block.Title, block.Lede = wording.FolderRecentTitle.In(lang), wording.FolderRecentLedeScoped.In(lang)
	case ordered:
		block.Title = wording.FolderRecentTitle.In(lang)
	case scoped:
		block.Lede = wording.FolderTiedLedeScoped.In(lang)
	default:
		block.Lede = wording.FolderTiedLede.In(lang)
	}
	return block
}

// StatusDistribution is where every indexed note sits: one cell per status the
// notes carry, and beside them the cells for notes carrying none, kept apart so
// the markup cannot dress those as statuses. A distribution holding no status
// draws nothing — a vault whose vocabulary could not be read leaves it empty,
// and the reason is stated once for the page rather than inside the block that
// would have shown it.
type StatusDistribution struct {
	Lede     string
	Statuses []LifecycleItem
	Unstated []LifecycleItem
}

// NewStatusDistribution builds the block with the sentence it can stand
// behind. The distribution counts every indexed note whatever the shelf shows,
// so beside a shelf narrowed to the declared knowledge layer the sentence says
// the count reaches past the shelf; an unscoped shelf carries no sentence,
// because the heading already says what the block is.
func NewStatusDistribution(statuses, unstated []LifecycleItem, scoped bool, lang wording.Lang) StatusDistribution {
	lede := ""
	if scoped {
		lede = wording.FolderLifecycleLedeScoped.In(lang)
	}
	return StatusDistribution{Lede: lede, Statuses: statuses, Unstated: unstated}
}

// NewPathIndex builds the study-path index. The measure is the course's extent
// — how many lessons it lays out — and never how far anyone has got: a count
// that described a status as progress ran backwards as the work was finished,
// and does not return under another name.
func NewPathIndex(paths []nav.Path, roles schema.NavigationRoles, closure nav.Closure, contract ContractState, lang wording.Lang, articleLang ArticleLanguageFor) ListIndexView {
	rows := make([]Row, 0, len(paths))
	for i := range paths {
		studyPath := &paths[i]
		extent := plural(studyPath.Planned, wording.LessonCountOne, wording.LessonCountMany, lang)
		// A zero with grammar diagnostics behind it is a fault to repair; a
		// zero without them is the author's answer.
		unread := studyPath.Planned == 0 && len(studyPath.Diagnostics) > 0
		mark := extent
		if unread {
			mark = joinMarks(extent, wording.NoStructureRead.In(lang))
		}
		rows = append(rows, Row{
			Text:     studyPath.Title,
			Href:     syllabusHref(studyPath.RelPath),
			Mark:     mark,
			Fault:    unread,
			Language: rowLanguage(articleLang, studyPath.RelPath),
		})
	}
	view := listIndex(pathMode, wording.Paths.In(lang),
		plural(len(paths), wording.PathCountOne, wording.PathCountMany, lang),
		"", emptySentence(contract, declarationSentence(roles.PathTypes(), lang), lang), rows)
	view.Fault = closure.Diagnostic()
	withholdListing(&view, closure)
	return view
}

// ContractState is what an empty listing may say about the folder's contract.
// It holds apart the two absences a reader cannot tell apart from the page: a
// folder carrying no contract, and a contract sitting in the folder that this
// process never loaded.
type ContractState int

const (
	// ContractGoverning is a folder something claimed authority over — a
	// contract that loaded, one that could not be read, and one that left a
	// section out, because the claim is what governs rather than its
	// completeness.
	ContractGoverning ContractState = iota
	// ContractAbsent is a folder holding no contract file.
	ContractAbsent
	// ContractUnloaded is a contract file this reading holds and this process
	// does not. The contract is read once, when yomihon starts, so one that
	// arrives afterwards is a file on the shelf and no authority at all.
	ContractUnloaded
)

// String names a contract state for a diagnostic or a log line. A state
// outside the three constants is a programming error and panics.
func (s ContractState) String() string {
	switch s {
	case ContractGoverning:
		return "governing"
	case ContractAbsent:
		return "absent"
	case ContractUnloaded:
		return "unloaded"
	default:
		panic("pages: unknown ContractState: " + strconv.Itoa(int(s)))
	}
}

// ContractStateFrom names that state from what one request already holds:
// whether anything claimed authority over the folder, and whether the
// generation this page lists from saw the contract file. Seeing the name in
// that reading is the whole of the evidence — nothing here opens the file,
// reads it or acts on it, and the folder stays ungoverned until yomihon is
// started again.
func ContractStateFrom(governed bool, snap *snapshot.Generation) ContractState {
	switch {
	case governed:
		return ContractGoverning
	case snap.Contains(schema.ContractRelPath):
		return ContractUnloaded
	}
	return ContractAbsent
}

// emptySentence chooses what an empty listing says. A folder no contract
// governs has declared nothing to be empty of, so telling it that it "declares
// none" answers a question it was never asked; the other two sentences say what
// is true of it instead, and they are two because the way out is two: one
// reader has a contract to write, the other only has yomihon to start again.
// Under a contract that governs, what the listing is empty of is the mode's own
// question, answered by the caller.
func emptySentence(contract ContractState, governed string, lang wording.Lang) string {
	switch contract {
	case ContractUnloaded:
		return wording.JoinGuide(wording.IndexContractUnloaded, wording.IndexContractUnloadedNext, lang)
	case ContractAbsent:
		return wording.JoinGuide(wording.IndexUngoverned, wording.IndexUngovernedNext, lang)
	default:
		return governed
	}
}

// declarationSentence is what a shelf filled by a declaration says while
// nothing has been declared onto it: which type puts a note here, spelled the
// way the contract spells it, and the one edit that follows. The words arrive
// from the contract because they are the vault's; a listing of several is
// joined the way this interface joins a list inside a sentence.
//
// A contract declaring no type at all for the shelf leaves the reader nothing
// to complete, and this says nothing rather than name an edit that would not
// fill it — the same silence a declaration that could not be read is left in.
func declarationSentence(declaredTypes []string, lang wording.Lang) string {
	if len(declaredTypes) == 0 {
		return ""
	}
	sentence := wording.NoDeclaredTypeEmptyFmt
	if len(declaredTypes) > 1 {
		sentence = wording.NoDeclaredTypesEmptyFmt
	}
	return fmt.Sprintf(sentence.In(lang), strings.Join(declaredTypes, wording.ListSeparator.In(lang)))
}

// listIndex assembles a mode's page from the parts every one of them has. The
// kicker is the shelf's own measure, and the title is the mode's name; reading
// the count from the shelf is what keeps the two from disagreeing.
//
// A page that reads a declaration takes that declaration's closure and states
// its reason. It does not also refuse to list: a closure that is shut leaves
// the model with nothing of that kind to hand over, so a second refusal here
// would be a guard over a case that cannot arrive, in a third place. What a
// page owes the reader is the reason, and that is what it carries.
func listIndex(mode, title, count, lede, empty string, rows []Row) ListIndexView {
	return ListIndexView{
		Mode:   mode,
		Kicker: modeKicker(count),
		Shelf: Shelf{
			Title: title,
			Lede:  lede,
			Count: count,
			Empty: empty,
			Rows:  rows,
		},
	}
}

// NewMapIndex builds the map index. A map's measure is how many branches it
// holds at every depth, which is the shape of the subject it draws. Those
// branches are the same tree the rail lists, so a map whose only wikilinks
// sit in prose or a table still has a count.
func NewMapIndex(maps []nav.Map, roles schema.NavigationRoles, closure nav.Closure, contract ContractState, lang wording.Lang, articleLang ArticleLanguageFor) ListIndexView {
	rows := make([]Row, 0, len(maps))
	for i := range maps {
		rows = append(rows, Row{
			Text:     maps[i].Title,
			Href:     notesHref(maps[i].RelPath),
			Mark:     plural(countBranches(maps[i].Branches), wording.BranchCountOne, wording.BranchCountMany, lang),
			Language: rowLanguage(articleLang, maps[i].RelPath),
		})
	}
	view := listIndex(mapMode, wording.Maps.In(lang),
		plural(len(maps), wording.MapCountOne, wording.MapCountMany, lang),
		"", emptySentence(contract, declarationSentence(roles.MapTypes(), lang), lang), rows)
	view.Fault = closure.Diagnostic()
	withholdListing(&view, closure)
	return view
}

// countBranches totals a map's branches at every depth.
func countBranches(branches []nav.Branch) int {
	total := len(branches)
	for i := range branches {
		total += countBranches(branches[i].Subbranches)
	}
	return total
}

// NewReportIndex builds the report index. A report is dated by nature — a daily
// briefing, an audit run — so the row leads with its day, then its name, then
// the line the report opens with, then which of the two kinds it is. The two
// kinds are named apart because they are read apart: a briefing is a program's
// output, shown as bytes inside an isolated frame, and a written report is a
// note like any other. The day and the opening arrive already read; nothing
// here goes looking for either.
func NewReportIndex(reports []nav.Report, lang wording.Lang, articleLang ArticleLanguageFor) ListIndexView {
	rows := make([]Row, 0, len(reports))
	for _, report := range reports {
		href, kind := notesHref(report.RelPath), wording.WrittenReport.In(lang)
		if report.Briefing {
			href, kind = reportHref(report.Name), wording.DailyBriefing.In(lang)
		}
		rows = append(rows, Row{
			When:     reportWhen(report, lang),
			Text:     report.Name,
			Opening:  report.Opening,
			Href:     href,
			Mark:     kind,
			Language: rowLanguage(articleLang, report.RelPath),
		})
	}
	return listIndex(reportMode, wording.Reports.In(lang),
		plural(len(reports), wording.ReportCountOne, wording.ReportCountMany, lang),
		wording.ReportIndexLede.In(lang), wording.ReportIndexEmpty.In(lang), rows)
}

// reportWhen is the one answer a report's date face gives, and it always gives
// one. A report carrying a day shows it. The briefing the vault keeps current
// is named for being the latest rather than for a day, so it says that instead
// — which is also where the shelf puts it. A report with neither says it wrote
// no day, because a row left blank in the column every other row answers reads
// as something the page failed to look up.
func reportWhen(report nav.Report, lang wording.Lang) string {
	switch {
	case report.Date != "":
		return report.Date
	case report.Latest:
		return wording.Newest.In(lang)
	default:
		return wording.ReportUndated.In(lang)
	}
}

// ArticleLanguageFor returns a note's declared article language by path, or
// empty when the note declared none or the contract gave no authority.
type ArticleLanguageFor func(relPath string) string

// rowLanguage reads one listing row's declared language from the lookup the
// handler built for this request.
func rowLanguage(articleLang ArticleLanguageFor, relPath string) string {
	if articleLang == nil || relPath == "" {
		return ""
	}
	return articleLang(relPath)
}

// ArticleLanguageFromSnapshot returns a lookup backed by one captured
// generation's resolved note languages.
func ArticleLanguageFromSnapshot(snap *snapshot.Generation) ArticleLanguageFor {
	if snap == nil {
		return nil
	}
	return func(relPath string) string {
		note, ok := snap.Note(relPath)
		if !ok {
			return ""
		}
		return note.Language
	}
}

// NewFolderIndex builds the folder shelf from the declared knowledge layer,
// or the full directory tree when no scope is available. Its measure includes
// every file below those folders and every root file, so a vault whose files
// all sit at the root counts and lists them without calling itself empty.
func NewFolderIndex(model *nav.Model, contract ContractState, lang wording.Lang, articleLang ArticleLanguageFor) ListIndexView {
	rootNotes := model.RootNotes()
	folders := model.ShelfFolders()
	return listIndex(folderMode, wording.Folders.In(lang),
		plural(countNotes(rootNotes, folders), wording.FolderNoteCountOne, wording.FolderNoteCountMany, lang),
		wording.FolderIndexLede.In(lang),
		emptySentence(contract, wording.JoinGuide(wording.FolderIndexEmpty, wording.IndexDeclaredEmptyNext, lang), lang),
		folderRows(rootNotes, folders, lang, true, articleLang))
}

// folderRows is one level of the tree. The folders come first, because a reader
// descending a tree chooses a branch before a leaf, and each says how many
// notes sit under it and opens its own page. Markdown files follow, then the
// other files the desk can open, in their own labelled group and uncounted.
// At the vault root the notes are labelled too, so they are not read as
// another folder.
//
// One level is the whole of it. A page that unfolded every depth at once would
// be the drawer the reading desk was built to replace, and the level below is
// one row away.
func folderRows(files []nav.NoteRef, folders []nav.Folder, lang wording.Lang, root bool, articleLang ArticleLanguageFor) []Row {
	notes, others := splitNotesAndFiles(files)
	rows := make([]Row, 0, len(folders)+len(notes)+len(others)+2)
	for i := range folders {
		rows = append(rows, Row{
			Text: folders[i].Name,
			Href: folderHref(folders[i].RelPath),
			Mark: folderNoteCount(&folders[i], lang),
		})
	}
	if len(notes) > 0 {
		if root {
			rows = append(rows, Row{Text: wording.RootNotes.In(lang), Heading: true})
		}
		for _, note := range notes {
			language := note.Language
			if language == "" && articleLang != nil {
				language = articleLang(note.RelPath)
			}
			rows = append(rows, Row{Text: note.Name, Href: notesHref(note.RelPath), Language: language})
		}
	}
	if len(others) > 0 {
		rows = append(rows, Row{Text: wording.OtherFiles.In(lang), Heading: true})
		for _, file := range others {
			rows = append(rows, Row{Text: file.Name, Href: notesHref(file.RelPath)})
		}
	}
	return rows
}

func splitNotesAndFiles(files []nav.NoteRef) (notes, others []nav.NoteRef) {
	for _, file := range files {
		if vault.IsMarkdown(file.RelPath) {
			notes = append(notes, file)
			continue
		}
		others = append(others, file)
	}
	return notes, others
}

// countNotes totals the markdown notes the tree holds at every depth. Files
// that are not notes stay on the shelf and are not this figure.
func countNotes(files []nav.NoteRef, folders []nav.Folder) int {
	total := 0
	for _, file := range files {
		if vault.IsMarkdown(file.RelPath) {
			total++
		}
	}
	for i := range folders {
		total += countNotes(folders[i].Notes, folders[i].Subfolders)
	}
	return total
}

// modeKicker is the line above a mode index's title: how much of the mode
// there is. The name sits in the heading below, once.
func modeKicker(count string) string {
	return count
}

// folderNoteCount is what a folder shows beside its name wherever it is listed:
// the files it holds at every depth, so a row says how much is behind it before
// anyone opens it.
func folderNoteCount(f *nav.Folder, lang wording.Lang) string {
	return plural(countNotes(f.Notes, f.Subfolders), wording.FolderNoteCountOne, wording.FolderNoteCountMany, lang)
}

// DeskBlock is one of the four ways into the library: which organisation it is,
// and that organisation's shelf. The mode name stays here rather than on the
// shelf because it is the desk's own hook for the block, not something the
// listing knows about itself.
type DeskBlock struct {
	Mode  string
	Shelf Shelf
}

// deskBlockItems is how many of a mode's items the desk shows before the mode's
// own page takes over. The desk is a way in, not a fifth listing.
const deskBlockItems = 3

// NewDeskBlocks builds the four ways in from the same projections the mode
// index pages list, so a block and the page its heading opens can never
// disagree about what the vault holds. A withheld declaration leaves its block
// empty; the reason is stated once for the whole desk, below the seam.
func NewDeskBlocks(model *nav.Model, roles schema.NavigationRoles, contract ContractState, lang wording.Lang, articleLang ArticleLanguageFor) []DeskBlock {
	// The blocks are the mode pages narrowed, so they refuse what those pages
	// refuse: each constructor is handed the same declaration closure the page
	// is, and withhold then takes back only what a block would otherwise claim
	// about how much it holds.
	closure := model.DeclaredClosure()
	withheld := closure.Closed()
	pathIndex := NewPathIndex(model.Paths(), roles, closure, contract, lang, articleLang)
	mapIndex := NewMapIndex(model.Maps(), roles, closure, contract, lang, articleLang)
	reportIndex := NewReportIndex(model.Reports(), lang, articleLang)
	folderIndex := NewFolderIndex(model, contract, lang, articleLang)
	pathBlock := deskBlock(&pathIndex, wording.DeskPathsLede.In(lang))
	mapBlock := deskBlock(&mapIndex, wording.DeskMapsLede.In(lang))
	if withheld {
		withhold(&pathBlock.Shelf)
		withhold(&mapBlock.Shelf)
	}
	return []DeskBlock{
		pathBlock,
		mapBlock,
		deskBlock(&reportIndex, wording.DeskReportsLede.In(lang)),
		deskBlock(&folderIndex, wording.DeskFoldersLede.In(lang)),
	}
}

// withholdListing takes back what a page may not claim about a declaration that
// was closed: how much it holds, and that it holds none. It is the same
// withdrawal the desk's blocks make, made here as well so the two cannot
// disagree — and it does not wait for the closure to have brought a sentence
// with it, because a closure that came silently withholds exactly as much.
func withholdListing(v *ListIndexView, closure nav.Closure) {
	if !closure.Closed() {
		return
	}
	withhold(&v.Shelf)
	v.Kicker = ""
}

// withhold takes back what a shelf would otherwise claim about an organisation
// the contract could not describe. A declaration that could not be read is not
// a declaration of nothing: "no courses" and "no courses declared" are both
// answers this page does not have, and the reason it has neither is stated once
// for the whole desk, below the seam.
func withhold(s *Shelf) {
	s.Count = ""
	s.Empty = ""
}

// deskBlock shows the desk a corner of the shelf a mode's page unfolds. It is
// the same shelf, with the address of that page and the shorter sentence a
// block has room for; narrowing it to the rows that fit is the shelf
// component's own business.
//
// The measure comes across untouched. A block that recomputed it could count
// something the page did not, which is the one disagreement this arrangement
// exists to make impossible.
//
// The block and the page share the rows rather than copying them, which is
// what makes the two the same shelf rather than two shelves that agree today.
// The rows are read-only by the shelf's own contract, and both views are built
// for one request from a projection the model already handed over as a copy.
func deskBlock(index *ListIndexView, lede string) DeskBlock {
	shelf := index.Shelf
	shelf.Lede = lede
	shelf.Href = indexHref(index.Mode)
	return DeskBlock{Mode: index.Mode, Shelf: shelf}
}

// joinMarks writes what a row is measured by and what is wrong with it as one
// line, in the order a reader reads them, skipping whichever the mode does not
// have.
func joinMarks(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " · ")
}

// indexHref is where a mode block's heading leads.
func indexHref(mode string) string { return "/" + mode }
