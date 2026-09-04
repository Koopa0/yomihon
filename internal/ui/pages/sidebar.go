package pages

import (
	"fmt"
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// Sidebar is the fully resolved left navigation for one request: the shared
// whole-vault model plus the current note's context. A page with no current note
// builds one with an empty path, so every branch renders closed and unmarked.
// The open sets are resolved once here, so a template looks up a boolean rather
// than re-walking the tree.
type Sidebar struct {
	Model *nav.Model

	// CurrentPath is the note being read, empty on a page with no note.
	CurrentPath string
	// HereDir is the current note's directory; Here lists every file in it,
	// the current one included, to be marked. The rail width decides how much
	// of that list it has room for, so nothing is trimmed here.
	HereDir string
	Here    []nav.NoteRef
	// Steps holds, per study path teaching the current note, the readable
	// lessons on either side of it — the course order the paths drawer walks.
	Steps        []nav.Neighbors
	openMaps     map[string]bool
	openBranches map[string]bool
}

// NewSidebar resolves the left navigation for one page; currentPath is empty for
// a page with no note. It takes no language, because the rail speaks whatever
// the page around it speaks and the chrome carries that answer.
//
// A nil model is tolerated where every other constructor here panics: the rail
// is drawn beside the not-found page a reader reaches when the vault could not
// be projected at all, and stopping there would take down the page that exists
// to explain why.
func NewSidebar(model *nav.Model, currentPath string) Sidebar {
	// The model's indexes are keyed by NFC paths; a request URL is not.
	currentPath = vault.NormalizeNFC(currentPath)
	sb := Sidebar{
		Model:        model,
		CurrentPath:  currentPath,
		openMaps:     map[string]bool{},
		openBranches: map[string]bool{},
	}
	if model == nil || currentPath == "" {
		return sb
	}

	sb.HereDir, sb.Here = model.Siblings(currentPath)
	sb.Steps = model.PathNeighbors(currentPath)

	// Every heading prefix, so a branch's ancestors open with it.
	for _, p := range model.Placements(currentPath) {
		sb.openMaps[p.MapRelPath] = true
		headings := p.Headings
		for i := 1; i <= len(headings); i++ {
			sb.openBranches[branchKey(p.MapRelPath, headings[:i])] = true
		}
	}

	// A map being read is its own wayfinding, which Placements cannot say
	// because a map does not place itself.
	if model.IsMap(currentPath) || model.IsPath(currentPath) {
		sb.openMaps[currentPath] = true
	}

	return sb
}

// HereShelf is the folder the reader is in, as a shelf: every file in it in the
// folder's own order, the one being read marked, and the folder's page as the
// way to the rest of it.
//
// It is built here at render time because the words in it are the page's. The
// rail speaks whatever the page around it speaks, and the model that resolved
// this navigation was read before any of that was known.
func (s *Sidebar) HereShelf(lang wording.Lang) Shelf {
	rows := make([]Row, 0, len(s.Here))
	for _, n := range s.Here {
		rows = append(rows, Row{Text: n.Name, Href: notesHref(n.RelPath), Current: s.current(n.RelPath)})
	}
	return Shelf{Title: hereLabel(s.HereDir, lang), Href: folderHref(s.HereDir), Rows: rows}
}

// current reports whether relPath is the note being read. The receiver is a
// pointer only to avoid copying the resolved sets per lookup; nothing mutates.
func (s *Sidebar) current(relPath string) bool {
	return relPath != "" && relPath == s.CurrentPath
}

// mapOpen reports whether a map branch contains the current note.
func (s *Sidebar) mapOpen(relPath string) bool { return s.openMaps[relPath] }

// branchOpen reports whether a map branch lies on the path to the current note,
// rebuilding the key NewSidebar stored.
func (s *Sidebar) branchOpen(mapRelPath string, headings []string) bool {
	return s.openBranches[branchKey(mapRelPath, headings)]
}

// currentAttr marks the entry the reader is on with aria-current="page" and
// contributes nothing otherwise.
func currentAttr(isCurrent bool) templ.Attributes {
	if isCurrent {
		return templ.Attributes{"aria-current": "page"}
	}
	return nil
}

// pathsChainOpen reports whether a study path holds the current note, which
// makes the surrounding group part of the wayfinding chain.
func (s *Sidebar) pathsChainOpen() bool {
	return s.chainOpen(s.Model.IsPath)
}

// The drawers below feed only the server-rendered default: each opens to meet
// the page the reader is on and renders closed everywhere else. A disclosure the
// reader toggled by hand still wins over that, and the wayfinding chain over
// both.

// mapsChainOpen reports whether a general map holds — or is — the current
// note, which makes the surrounding group part of the wayfinding chain.
func (s *Sidebar) mapsChainOpen() bool {
	return s.chainOpen(s.Model.IsMap)
}

// chainOpen reports whether anything the wayfinding opened is of the kind
// belongs answers for, asking the open set rather than every tree in the model.
func (s *Sidebar) chainOpen(belongs func(relPath string) bool) bool {
	for relPath := range s.openMaps {
		if belongs(relPath) {
			return true
		}
	}
	return false
}

// journalOpen reports whether the page being read lives in the journal.
func (s *Sidebar) journalOpen() bool { return nav.InJournal(s.CurrentPath) }

// reportsOpen reports whether the page being read is one of the reports.
func (s *Sidebar) reportsOpen() bool { return nav.InReports(s.CurrentPath) }

// disclosureAttrs marks one sidebar disclosure for the state owner coordinating
// <details open>: key names the section stably across pages, and chain marks a
// branch of the current note's wayfinding, which is forced open and never
// persisted — recording a close there would hide the reader's own location.
func disclosureAttrs(key string, chain bool) templ.Attributes {
	attrs := templ.Attributes{"data-key": key}
	if chain {
		attrs["data-chain"] = true
	}
	return attrs
}

// railWindow is how many of a folder's files the rail lists around the one being
// read, in the siblings block and the tree alike. The rail is a reading aid, not
// the folder: it owes the reader where they are and what is either side.
const railWindow = 24

// Breadcrumb names each folder above a file, with the path that reaches it, so a
// reader can climb out of where they are.
func Breadcrumb(relPath string) []nav.NoteRef {
	segs := strings.Split(relPath, "/")
	if len(segs) <= 1 {
		return nil
	}
	crumbs := make([]nav.NoteRef, 0, len(segs)-1)
	for i, seg := range segs[:len(segs)-1] {
		crumbs = append(crumbs, nav.NoteRef{Name: seg, RelPath: strings.Join(segs[:i+1], "/")})
	}
	return crumbs
}

// hereLabel names the siblings block after the current note's directory: its
// innermost folder, or the vault root for a file that lives at the top.
func hereLabel(dir string, lang wording.Lang) string {
	if dir == "" {
		return wording.VaultRoot.In(lang)
	}
	if i := strings.LastIndexByte(dir, '/'); i >= 0 {
		return dir[i+1:]
	}
	return dir
}

// childChain extends a branch-heading chain by one level, returning a fresh
// slice so a recursive render never mutates its parent's chain.
func childChain(chain []string, heading string) []string {
	return slices.Concat(chain, []string{heading})
}

// branchKey identifies a map branch by its path and heading chain, so the
// resolved open set and the renderer name the same branch. The separator is a
// control character that never appears in a heading.
func branchKey(mapRelPath string, headings []string) string {
	return mapRelPath + "\x1f" + strings.Join(headings, "\x1f")
}

// CapabilityFault is one closed navigation projection stated in the rail: what
// is unavailable, in the reader's language, and the detail written by whoever
// refused. Detail is empty where the refusal has no sentence of its own.
type CapabilityFault struct {
	Summary string
	Detail  string
}

// CapabilityFaults lists what the rail has to say about closed navigation
// projections, one entry per distinct cause: one cause commonly closes both
// paths and instance projections, and saying it twice reads as two faults.
func (s *Sidebar) CapabilityFaults(lang wording.Lang) []CapabilityFault {
	if s.Model == nil {
		return nil
	}
	navigation := s.Model.NavigationClosure()
	artifact := s.Model.ArtifactClosure()
	switch {
	case navigation.Closed() && artifact.Closed() && navigation.Diagnostic() == artifact.Diagnostic():
		return []CapabilityFault{{Summary: wording.PathsMapsAndArtifactsUnavailable.In(lang), Detail: navigation.Diagnostic()}}
	case navigation.Closed() && artifact.Closed():
		return []CapabilityFault{
			{Summary: wording.PathsAndMapsUnavailable.In(lang), Detail: navigation.Diagnostic()},
			{Summary: wording.ArtifactsUnavailable.In(lang), Detail: artifact.Diagnostic()},
		}
	case navigation.Closed():
		return []CapabilityFault{{Summary: wording.PathsAndMapsUnavailable.In(lang), Detail: navigation.Diagnostic()}}
	case artifact.Closed():
		return []CapabilityFault{{Summary: wording.ArtifactsUnavailable.In(lang), Detail: artifact.Diagnostic()}}
	default:
		return nil
	}
}

// FooterSequence chooses which order the foot of the article offers, and what to
// call it. A note exactly one course teaches steps through that course, whose
// declared order the folder's alphabetical one can contradict completely; every
// other note keeps the folder, two courses included, because nothing here knows
// which the reader is walking.
//
// course reports which order won, so the foot can print it: that and the step
// words are all a sighted reader has to tell a course from folder adjacency.
func FooterSequence(model *nav.Model, relPath string, lang wording.Lang) (prev, next nav.NoteRef, label string, course bool) {
	relPath = vault.NormalizeNFC(relPath)
	if model == nil || relPath == "" {
		return prev, next, "", false
	}
	if steps := model.PathNeighbors(relPath); len(steps) == 1 {
		return steps[0].Prev, steps[0].Next, fmt.Sprintf(wording.CourseOrderOf.In(lang), steps[0].PathTitle), true
	}
	prev, next = model.FolderStep(relPath)
	return prev, next, wording.FolderAdjacency.In(lang), false
}

// stepWordPrev and stepWordNext name a footer step for the order it walks: a
// course hands over a lesson, a folder merely the file beside this one.
func stepWordPrev(course bool, lang wording.Lang) string {
	if course {
		return wording.PreviousLesson.In(lang)
	}
	return wording.PreviousFile.In(lang)
}

func stepWordNext(course bool, lang wording.Lang) string {
	if course {
		return wording.NextLesson.In(lang)
	}
	return wording.NextFile.In(lang)
}
