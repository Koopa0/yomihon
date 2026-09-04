package pages

import (
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
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
	block := RecentBlock{Title: wording.HomeTiedTitle.In(lang), Notes: notes}
	switch {
	case ordered && scoped:
		block.Title, block.Lede = wording.HomeRecentTitle.In(lang), wording.HomeRecentLedeScoped.In(lang)
	case ordered:
		block.Title, block.Lede = wording.HomeRecentTitle.In(lang), wording.HomeRecentLede.In(lang)
	case scoped:
		block.Lede = wording.HomeTiedLedeScoped.In(lang)
	default:
		block.Lede = wording.HomeTiedLede.In(lang)
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
	Statuses []LifecycleItem
	Unstated []LifecycleItem
}

// NewPathIndex builds the study-path index. The measure is the course's extent
// — how many lessons it lays out — and never how far anyone has got: a count
// that described a status as progress ran backwards as the work was finished,
// and does not return under another name.
func NewPathIndex(paths []nav.Path, closure nav.Closure, governed bool, lang wording.Lang) ListIndexView {
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
			Text:  studyPath.Title,
			Href:  syllabusHref(studyPath.RelPath),
			Mark:  mark,
			Fault: unread,
		})
	}
	view := listIndex(pathMode, wording.Paths.In(lang),
		plural(len(paths), wording.PathCountOne, wording.PathCountMany, lang),
		wording.PathIndexLede.In(lang), emptySentence(governed, wording.PathIndexEmpty, wording.PathIndexUngoverned, lang), rows)
	view.Fault = closure.Diagnostic()
	withholdListing(&view, closure)
	return view
}

// emptySentence chooses what an empty listing says. A folder no contract
// governs has declared nothing to be empty of, so telling it that it "declares
// none" answers a question it was never asked; the ungoverned sentence states
// what is true of it instead.
//
// governed is whether anything claimed authority over this folder at all — true
// for a contract that loaded, for one that could not be read, and for one that
// left a section out, because the claim is what governs rather than its
// completeness. So the two sentences separate a folder with no contract from a
// folder whose contract declares none of this kind, which is the distinction a
// reader needs and the one the old single sentence could not make.
func emptySentence(governed bool, declared, ungoverned wording.Phrase, lang wording.Lang) string {
	if governed {
		return declared.In(lang)
	}
	return ungoverned.In(lang)
}

// listIndex assembles a mode's page from the parts every one of them has. The
// kicker repeats the shelf's own name and measure because it is the page's
// heading rather than the shelf's, and reading them from the shelf is what
// keeps the two from disagreeing.
//
// A page that reads a declaration takes that declaration's closure and states
// its reason. It does not also refuse to list: a closure that is shut leaves
// the model with nothing of that kind to hand over, so a second refusal here
// would be a guard over a case that cannot arrive, in a third place. What a
// page owes the reader is the reason, and that is what it carries.
func listIndex(mode, title, count, lede, empty string, rows []Row) ListIndexView {
	return ListIndexView{
		Mode:   mode,
		Kicker: modeKicker(title, count),
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
// holds at every depth, which is the shape of the subject it draws.
func NewMapIndex(maps []nav.Map, closure nav.Closure, governed bool, lang wording.Lang) ListIndexView {
	rows := make([]Row, 0, len(maps))
	for i := range maps {
		rows = append(rows, Row{
			Text: maps[i].Title,
			Href: notesHref(maps[i].RelPath),
			Mark: plural(countBranches(maps[i].Branches), wording.BranchCountOne, wording.BranchCountMany, lang),
		})
	}
	view := listIndex(mapMode, wording.Maps.In(lang),
		plural(len(maps), wording.MapCountOne, wording.MapCountMany, lang),
		wording.MapIndexLede.In(lang), emptySentence(governed, wording.MapIndexEmpty, wording.MapIndexUngoverned, lang), rows)
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

// NewReportIndex builds the report index. The two kinds are named apart because
// they are read apart: a briefing is a program's output, shown as bytes inside
// an isolated frame, and a written report is a note like any other. The row
// keeps the filename the author gave it and lifts the day out of the front of
// that name where there is one; nothing here opens a report to describe it.
func NewReportIndex(reports []nav.Report, lang wording.Lang) ListIndexView {
	rows := make([]Row, 0, len(reports))
	for _, report := range reports {
		href, kind := notesHref(report.RelPath), wording.WrittenReport.In(lang)
		if report.Briefing {
			href, kind = reportHref(report.Name), wording.DailyBriefing.In(lang)
		}
		newest := ""
		if report.Latest {
			newest = wording.Newest.In(lang)
		}
		rows = append(rows, Row{
			Text: report.Name,
			Href: href,
			Mark: joinMarks(leadingDate(report.Name), kind, newest),
		})
	}
	return listIndex(reportMode, wording.Reports.In(lang),
		plural(len(reports), wording.ReportCountOne, wording.ReportCountMany, lang),
		wording.ReportIndexLede.In(lang), wording.ReportIndexEmpty.In(lang), rows)
}

// leadingDate reads the day off the front of a filename written as one, and
// answers with nothing for a name that does not start with one. It parses no
// contents: what a report says is the author's, and this is only how the vault
// named the file.
func leadingDate(name string) string {
	const iso = len("2026-09-03")
	if len(name) < iso {
		return ""
	}
	head := name[:iso]
	for i, r := range head {
		digit := r >= '0' && r <= '9'
		switch i {
		case 4, 7:
			if r != '-' {
				return ""
			}
		default:
			if !digit {
				return ""
			}
		}
	}
	return head
}

// NewFolderIndex builds the folder index: the top of the vault's own directory
// tree, listed the way every other mode is listed. Its measure is every file
// under it at any depth, which is why a vault whose files all sit at the root
// counts them and lists them without ever calling itself empty.
func NewFolderIndex(model *nav.Model, lang wording.Lang) ListIndexView {
	rootNotes := model.RootNotes()
	folders := model.Folders()
	return listIndex(folderMode, wording.Folders.In(lang),
		plural(countNotes(rootNotes, folders), wording.FolderNoteCountOne, wording.FolderNoteCountMany, lang),
		wording.FolderIndexLede.In(lang), wording.FolderIndexEmpty.In(lang),
		folderRows(rootNotes, folders, lang))
}

// folderRows is one level of the tree. The folders come first, because a reader
// descending a tree chooses a branch before a leaf, and each says how much sits
// under it and opens its own page. The files belonging to no folder follow,
// carrying no measure: the measure is what a row opens onto, and a note opens
// onto itself.
//
// One level is the whole of it. A page that unfolded every depth at once would
// be the drawer the reading desk was built to replace, and the level below is
// one row away.
func folderRows(rootNotes []nav.NoteRef, folders []nav.Folder, lang wording.Lang) []Row {
	rows := make([]Row, 0, len(folders)+len(rootNotes))
	for i := range folders {
		rows = append(rows, Row{
			Text: folders[i].Name,
			Href: folderHref(folders[i].RelPath),
			Mark: folderNoteCount(&folders[i], lang),
		})
	}
	for _, note := range rootNotes {
		rows = append(rows, Row{Text: note.Name, Href: notesHref(note.RelPath)})
	}
	return rows
}

// countNotes totals the files the tree holds at every depth.
func countNotes(rootNotes []nav.NoteRef, folders []nav.Folder) int {
	total := len(rootNotes)
	for i := range folders {
		total += countNotes(folders[i].Notes, folders[i].Subfolders)
	}
	return total
}

// modeKicker is the line above a mode index's title: what the mode is called,
// and how much of it there is. The separator is punctuation both languages set
// the same way, so it is written here rather than carried in the dictionary.
func modeKicker(name, count string) string {
	return name + " · " + count
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
func NewDeskBlocks(model *nav.Model, governed bool, lang wording.Lang) []DeskBlock {
	// The blocks are the mode pages narrowed, so they refuse what those pages
	// refuse: each constructor is handed the same declaration closure the page
	// is, and withhold then takes back only what a block would otherwise claim
	// about how much it holds.
	closure := model.DeclaredClosure()
	withheld := closure.Closed()
	pathIndex := NewPathIndex(model.Paths(), closure, governed, lang)
	mapIndex := NewMapIndex(model.Maps(), closure, governed, lang)
	reportIndex := NewReportIndex(model.Reports(), lang)
	folderIndex := NewFolderIndex(model, lang)
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
	v.Kicker = v.Shelf.Title
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
