package pages

import (
	"fmt"

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

	book         *nav.Path
	neighbors    nav.Neighbors
	hereDir      string
	here         []nav.NoteRef
	openBranches map[string]bool
}

// NewReadingRail resolves the one map a note page should show. noteDomain is
// the declared domain of the note being read, used only to break a tie among
// several study paths.
func NewReadingRail(model *nav.Model, currentPath, noteDomain string) ReadingRail {
	currentPath = vault.NormalizeNFC(currentPath)
	rr := ReadingRail{
		Model:       model,
		CurrentPath: currentPath,
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
func NewReportReadingRail(model *nav.Model, briefingRelPath string) ReadingRail {
	return ReadingRail{
		Model:       model,
		CurrentPath: vault.NormalizeNFC(briefingRelPath),
		Kind:        ReadingRailReports,
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

func (r *ReadingRail) currentHref(href string) bool {
	return href != "" && notesHref(r.CurrentPath) == href
}

// CapabilityFaults lists closed navigation projections for the reading rail.
func (r *ReadingRail) CapabilityFaults(lang wording.Lang) []CapabilityFault {
	return ModelCapabilityFaults(r.Model, lang)
}

// bookView draws the teaching path into the page view the rail reuses.
func (r *ReadingRail) bookView() PathView {
	if r.book == nil {
		return PathView{}
	}
	return BuildPathView(r.book, nil)
}

// courseStepsLabel names the path's whole order for the book rail's step links.
func (r *ReadingRail) courseStepsLabel(lang wording.Lang) string {
	if r.book == nil {
		return ""
	}
	return fmt.Sprintf(wording.CourseOrderOf.In(lang), r.book.Title)
}
