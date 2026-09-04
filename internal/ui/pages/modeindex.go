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

// ListIndexView is one mode's index page, where that mode's contents are a flat
// list. Fault is why a projection was withheld, stated once for the page the
// way the desk states it: a withheld projection lists nothing, and Empty is
// then not the answer — a vault whose declaration could not be read is not a
// vault that declared nothing.
type ListIndexView struct {
	Mode   string
	Kicker string
	Title  string
	Lede   string
	// Empty is what the page says when the mode holds nothing at all.
	Empty string
	Fault string
	Rows  []IndexRow
}

// IndexRow is one line of a mode index. Beyond the title and the address every
// cell belongs to some modes and not others: only a report carries a date, only
// a course or a map carries a measure, and only a row with something to flag
// carries a mark.
type IndexRow struct {
	Title string
	Href  string
	// Date is the day the item names in its own filename, empty where the name
	// carries none.
	Date string
	// Measure is the one figure the mode counts an item by — a course's
	// lessons, a map's branches. It is never a position in that count: how far
	// a reader has got is the browser's business and not a fact about the note.
	Measure string
	// Mark is a short chip beside the row; MarkWarn colours it as a fault
	// rather than as a plain remark.
	Mark     string
	MarkWarn bool
}

// FolderIndexView is the folder mode's index: the vault's own directory tree,
// which is the only one of the four modes whose contents are not a list.
type FolderIndexView struct {
	Mode   string
	Kicker string
	Title  string
	Lede   string
	Empty  string
	Fault  string
	// RootNotes are the files at the vault root, which belong to no folder.
	RootNotes []nav.NoteRef
	Folders   []nav.Folder

	// Recent, Lifecycle and Unstated are the librarian's view of the shelf:
	// what changed last, and where every indexed note sits. They belong to this
	// mode rather than to the desk because they describe how the files are
	// kept, which is a different question from what there is to read.
	Recent []HomeNote
	// RecentOrdered says the recorded times actually put these in order. A
	// fresh clone stamps every file with one moment, and where the times
	// separate nothing the block says so rather than promising recency.
	RecentOrdered bool
	// RecentScoped says the recent list covers the contract's declared
	// knowledge layer rather than the whole folder, which the lede then names:
	// the distribution beside it counts every indexed note.
	RecentScoped bool
	Lifecycle    []LifecycleItem
	// Unstated are the cells for notes carrying no status to be grouped by,
	// kept apart from Lifecycle so the markup cannot dress them as statuses.
	Unstated []LifecycleItem
	// ShowRecent and ShowLifecycle say which of the two this vault fills. A
	// withheld distribution renders nothing; Fault carries the reason.
	ShowRecent    bool
	ShowLifecycle bool
}

// recentTitle picks the recent block's heading: the recency one where the
// recorded times order the list, the plain one where they separate nothing.
func recentTitle(v *FolderIndexView, lang wording.Lang) string {
	if v.RecentOrdered {
		return wording.HomeRecentTitle.In(lang)
	}
	return wording.HomeTiedTitle.In(lang)
}

// recentLede picks the sentence under that heading — ordered or tied, and
// naming the knowledge layer exactly when the list is scoped to one.
func recentLede(v *FolderIndexView, lang wording.Lang) string {
	switch {
	case v.RecentOrdered && v.RecentScoped:
		return wording.HomeRecentLedeScoped.In(lang)
	case v.RecentOrdered:
		return wording.HomeRecentLede.In(lang)
	case v.RecentScoped:
		return wording.HomeTiedLedeScoped.In(lang)
	default:
		return wording.HomeTiedLede.In(lang)
	}
}

// NewPathIndex builds the study-path index. The measure is the course's extent
// — how many lessons it lays out — and never how far anyone has got: a count
// that described a status as progress ran backwards as the work was finished,
// and does not return under another name.
func NewPathIndex(paths []nav.Path, lang wording.Lang) ListIndexView {
	rows := make([]IndexRow, 0, len(paths))
	for i := range paths {
		studyPath := &paths[i]
		row := IndexRow{
			Title:   studyPath.Title,
			Href:    syllabusHref(studyPath.RelPath),
			Measure: plural(studyPath.Planned, wording.LessonCountOne, wording.LessonCountMany, lang),
		}
		// A zero with grammar diagnostics behind it is a fault to repair; a
		// zero without them is the author's answer.
		if studyPath.Planned == 0 && len(studyPath.Diagnostics) > 0 {
			row.Mark = wording.NoStructureRead.In(lang)
			row.MarkWarn = true
		}
		rows = append(rows, row)
	}
	return ListIndexView{
		Mode:   pathMode,
		Kicker: modeKicker(wording.Paths.In(lang), plural(len(paths), wording.PathCountOne, wording.PathCountMany, lang)),
		Title:  wording.Paths.In(lang),
		Lede:   wording.PathIndexLede.In(lang),
		Empty:  wording.PathIndexEmpty.In(lang),
		Rows:   rows,
	}
}

// NewMapIndex builds the map index. A map's measure is how many branches it
// holds at every depth, which is the shape of the subject it draws.
func NewMapIndex(maps []nav.Map, lang wording.Lang) ListIndexView {
	rows := make([]IndexRow, 0, len(maps))
	for i := range maps {
		rows = append(rows, IndexRow{
			Title:   maps[i].Title,
			Href:    notesHref(maps[i].RelPath),
			Measure: plural(countBranches(maps[i].Branches), wording.BranchCountOne, wording.BranchCountMany, lang),
		})
	}
	return ListIndexView{
		Mode:   mapMode,
		Kicker: modeKicker(wording.Maps.In(lang), plural(len(maps), wording.MapCountOne, wording.MapCountMany, lang)),
		Title:  wording.Maps.In(lang),
		Lede:   wording.MapIndexLede.In(lang),
		Empty:  wording.MapIndexEmpty.In(lang),
		Rows:   rows,
	}
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
	rows := make([]IndexRow, 0, len(reports))
	for _, report := range reports {
		row := IndexRow{
			Title:   report.Name,
			Href:    notesHref(report.RelPath),
			Date:    leadingDate(report.Name),
			Measure: wording.WrittenReport.In(lang),
		}
		if report.Briefing {
			row.Href = reportHref(report.Name)
			row.Measure = wording.DailyBriefing.In(lang)
		}
		if report.Latest {
			row.Mark = wording.Newest.In(lang)
		}
		rows = append(rows, row)
	}
	return ListIndexView{
		Mode:   reportMode,
		Kicker: modeKicker(wording.Reports.In(lang), plural(len(reports), wording.ReportCountOne, wording.ReportCountMany, lang)),
		Title:  wording.Reports.In(lang),
		Lede:   wording.ReportIndexLede.In(lang),
		Empty:  wording.ReportIndexEmpty.In(lang),
		Rows:   rows,
	}
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

// NewFolderIndex builds the folder index over the whole directory tree.
func NewFolderIndex(model *nav.Model, lang wording.Lang) FolderIndexView {
	rootNotes := model.RootNotes()
	folders := model.Folders()
	return FolderIndexView{
		Mode:      folderMode,
		Kicker:    modeKicker(wording.Folders.In(lang), plural(countNotes(rootNotes, folders), wording.FolderNoteCountOne, wording.FolderNoteCountMany, lang)),
		Title:     wording.Folders.In(lang),
		Lede:      wording.FolderIndexLede.In(lang),
		Empty:     wording.FolderIndexEmpty.In(lang),
		RootNotes: rootNotes,
		Folders:   folders,
	}
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

// folderNoteCount is what a folder's disclosure shows beside its name: the
// files it holds at every depth, so a branch says how much is under it before
// it is opened.
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
func NewDeskBlocks(model *nav.Model, lang wording.Lang) []DeskBlock {
	// A declaration nobody could read leaves the model with no courses and no
	// maps of its own, so there is no second guard here against listing them:
	// one answer to that question is enough, and withhold takes back only what
	// a block would otherwise claim about how much it holds.
	withheld := model.DeclaredClosure().Closed()
	paths := model.Paths()
	declared := model.Maps()
	folders := model.Folders()
	rootNotes := model.RootNotes()
	reports := model.Reports()
	pathIndex := NewPathIndex(paths, lang)
	mapIndex := NewMapIndex(declared, lang)
	reportIndex := NewReportIndex(reports, lang)
	pathBlock := deskBlock(&pathIndex, wording.DeskPathsLede.In(lang),
		plural(len(paths), wording.PathCountOne, wording.PathCountMany, lang))
	mapBlock := deskBlock(&mapIndex, wording.DeskMapsLede.In(lang),
		plural(len(declared), wording.MapCountOne, wording.MapCountMany, lang))
	if withheld {
		withhold(&pathBlock.Shelf)
		withhold(&mapBlock.Shelf)
	}
	return []DeskBlock{
		pathBlock,
		mapBlock,
		deskBlock(&reportIndex, wording.DeskReportsLede.In(lang),
			plural(len(reports), wording.ReportCountOne, wording.ReportCountMany, lang)),
		folderBlock(rootNotes, folders, lang),
	}
}

// folderBlock is the way in through the vault's own directories. Its rows are
// the folders at the top and its measure is every file under them, so the
// sentence for a folder mode holding nothing belongs to the measure rather than
// to the rows: a vault whose files all sit at the root has no top-level folder
// to list and is not empty, and saying it holds no files beside a count of them
// is the page contradicting itself.
func folderBlock(rootNotes []nav.NoteRef, folders []nav.Folder, lang wording.Lang) DeskBlock {
	files := countNotes(rootNotes, folders)
	empty := ""
	if files == 0 {
		empty = wording.FolderIndexEmpty.In(lang)
	}
	return DeskBlock{
		Mode: folderMode,
		Shelf: Shelf{
			Title: wording.Folders.In(lang),
			Count: plural(files, wording.FolderNoteCountOne, wording.FolderNoteCountMany, lang),
			Lede:  wording.DeskFoldersLede.In(lang),
			Href:  indexHref(folderMode),
			Empty: empty,
			Rows:  deskFolderRows(folders, lang),
		},
	}
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

// deskBlock reads one mode's whole index as the shelf the desk shows a corner
// of. Narrowing it to the rows that fit is the shelf component's own business.
func deskBlock(index *ListIndexView, lede, count string) DeskBlock {
	return DeskBlock{
		Mode: index.Mode,
		Shelf: Shelf{
			Title: index.Title,
			Lede:  lede,
			Count: count,
			Href:  indexHref(index.Mode),
			Empty: index.Empty,
			Rows:  shelfRowsFromIndex(index.Rows),
		},
	}
}

// shelfRowsFromIndex reads index rows as shelf rows. A shelf row carries one
// mark, so a mode that measures a row and also has something to flag about it
// says both in that one place rather than asking for a second.
func shelfRowsFromIndex(rows []IndexRow) []Row {
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, Row{
			Text:  row.Title,
			Href:  row.Href,
			Mark:  joinMarks(row.Measure, row.Date, row.Mark),
			Fault: row.MarkWarn,
		})
	}
	return out
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

// deskFolderRows is the top of the tree: each lifecycle folder, how much sits
// under it, and its own page.
func deskFolderRows(folders []nav.Folder, lang wording.Lang) []Row {
	rows := make([]Row, 0, len(folders))
	for i := range folders {
		rows = append(rows, Row{
			Text: folders[i].Name,
			Href: folderHref(folders[i].RelPath),
			Mark: folderNoteCount(&folders[i], lang),
		})
	}
	return rows
}

// indexHref is where a mode block's heading leads.
func indexHref(mode string) string { return "/" + mode }
