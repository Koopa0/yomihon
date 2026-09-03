package pages

import (
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
