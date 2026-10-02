package pages

import (
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// ReadingRailKind names which single map the reading rail shows.
type ReadingRailKind string

const (
	ReadingRailBook    ReadingRailKind = "book"
	ReadingRailFolder  ReadingRailKind = "folder"
	ReadingRailReports ReadingRailKind = "reports"
)

// ReadingRail is the narrow left navigation for one reading surface: the book
// alone inside a study path, the folder on a plain note, or the list of reports
// on a report. It never carries the whole-vault drawers the desk offers.
type ReadingRail struct {
	Model       *nav.Model
	CurrentPath string
	Kind        ReadingRailKind
	// Vault is the folder this rail is a rail of, which its foot states.
	Vault nav.Vault
	// KeptNote is the vault-relative note the reader kept a place in, empty
	// where they kept none. The book marks the row of that lesson, and a place
	// kept in a note the book does not teach marks nothing.
	KeptNote string

	book      *nav.Path
	neighbors nav.Neighbors
	// place is where the note sits in the book: its part, and what a side
	// branch hands over at its ends. It is the zero value for a note the book
	// does not walk, which is the case neighbors.PathRelPath says.
	place        nav.Place
	hereDir      string
	here         []nav.NoteRef
	openBranches map[string]bool
}

// NewReadingRail resolves the one map a note page should show. noteDomain is
// the declared domain of the note being read, used only to break a tie among
// several study paths.
func NewReadingRail(shell nav.Shell, currentPath, noteDomain string) ReadingRail {
	model := shell.Nav
	currentPath = vault.NormalizeNFC(currentPath)
	rr := ReadingRail{
		Model:       model,
		CurrentPath: currentPath,
		Vault:       shell.Vault,
	}
	if model == nil || currentPath == "" {
		rr.Kind = ReadingRailFolder
		return rr
	}
	if nav.InReports(currentPath) {
		rr.Kind = ReadingRailReports
		return rr
	}
	if book := model.TeachingPath(currentPath, noteDomain); book != nil {
		rr.Kind = ReadingRailBook
		rr.book = book
		rr.openBranches = map[string]bool{}
		for _, p := range model.Placements(currentPath) {
			if p.MapRelPath != book.RelPath {
				continue
			}
			headings := p.Headings
			for i := 1; i <= len(headings); i++ {
				rr.openBranches[branchKey(p.MapRelPath, headings[:i])] = true
			}
		}
		neighbors := model.PathNeighbors(currentPath)
		for i := range neighbors {
			step := neighbors[i]
			if step.PathRelPath == book.RelPath {
				rr.neighbors = step
				rr.place, _ = model.PathPlace(currentPath, book.RelPath)
				break
			}
		}
		return rr
	}
	rr.Kind = ReadingRailFolder
	rr.hereDir, rr.here = model.Siblings(currentPath)
	return rr
}

// NewReportReadingRail resolves the reports list for a sandboxed briefing page.
func NewReportReadingRail(shell nav.Shell, briefingRelPath string) ReadingRail {
	return ReadingRail{
		Model:       shell.Nav,
		CurrentPath: vault.NormalizeNFC(briefingRelPath),
		Kind:        ReadingRailReports,
		Vault:       shell.Vault,
	}
}

// HereShelf is the folder the reader is in, as a shelf at rail width.
func (r *ReadingRail) HereShelf(lang wording.Lang) Shelf {
	rows := make([]Row, 0, len(r.here))
	for _, n := range r.here {
		rows = append(rows, Row{
			Text:     n.Name,
			Href:     notesHref(n.RelPath),
			Current:  r.current(n.RelPath),
			Language: n.Language,
		})
	}
	return Shelf{Title: hereLabel(r.hereDir, lang), Href: folderHref(r.hereDir), Rows: rows}
}

func (r *ReadingRail) current(relPath string) bool {
	return relPath != "" && relPath == r.CurrentPath
}

// branchOpen reports whether a book branch lies on the path to the current note.
func (r *ReadingRail) branchOpen(pathRel string, headings []string) bool {
	return r.openBranches[branchKey(pathRel, headings)]
}

// CapabilityFaults lists closed navigation projections for the reading rail.
func (r *ReadingRail) CapabilityFaults(lang wording.Lang) []CapabilityFault {
	return ModelCapabilityFaults(r.Model, lang)
}

// railRow is one stop of the book in the rail: a lesson, or a branch drawn
// after the lesson it hangs from.
type railRow struct {
	entry  *PathEntryView
	branch *PathBranchView
}

// railRows lists what the rail draws for a run, in the order it draws it: a
// nested branch alone, or each lesson followed by the side branches hanging from
// it, which is where the author wrote them. The rail draws a branch as a
// disclosure of its own, so it is a row of its own here, and the order is
// settled before the template walks it.
func (r *PathRunView) railRows() []railRow {
	if r.Branch != nil {
		return []railRow{{branch: r.Branch}}
	}
	rows := make([]railRow, 0, len(r.Entries))
	for i := range r.Entries {
		rows = append(rows, railRow{entry: &r.Entries[i]})
		for j := range r.Entries[i].Branches {
			rows = append(rows, railRow{branch: &r.Entries[i].Branches[j]})
		}
	}
	return rows
}

// bookView draws the teaching path into the page view the rail reuses. The note
// being read is handed over as the row to mark, so the rail and the course page
// both learn which row that is from the one comparison, and neither keeps a
// second answer that could disagree with the other.
//
// Nothing else of the cover reaches here beyond the reader's kept place: the
// rail is the book beside the note being read, and the opening and the verb that
// opens a course belong to the page that is the course.
func (r *ReadingRail) bookView() PathView {
	if r.book == nil {
		return PathView{}
	}
	return BuildPathView(r.book, nil, &CourseCover{Here: r.CurrentPath, KeptNote: r.KeptNote})
}
