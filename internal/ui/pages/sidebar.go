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
// whole-vault model, plus the current note's context — its same-directory
// siblings, which map and folder branches to open, which entry is the one
// being read. A page with no current note builds one with an empty current path,
// so every branch renders in its default closed, unmarked state.
//
// The open sets are resolved once here from the model's precomputed indexes, so
// the templates only look up a boolean while rendering rather than re-walking the
// tree.
type Sidebar struct {
	Model *nav.Model

	// CurrentPath is the note being read, empty on a page with no note.
	CurrentPath string
	// Lang is the language the rail speaks. It travels on the rail rather than
	// through every component that draws part of one, because the rail is a
	// tree and threading a language down it would touch every branch.
	Lang wording.Lang
	// HereDir labels the siblings block with the current note's directory; Here
	// lists that directory's files (the current one included, to be marked).
	HereDir string
	Here    []nav.NoteRef
	// HereTrimmed is how many of the folder's files the block leaves out. A
	// folder of three years of entries put seven hundred rows in the rail of
	// every page — twice, since the folder tree carries them too — and the
	// reader's own entry sat hundreds of rows below the fold. The block shows
	// the neighbourhood and says what it left, because a list that quietly
	// stopped being the folder would be worse than a long one.
	HereTrimmed int
	// Steps holds, per study path teaching the current note, the readable
	// lessons on either side of it — the course order the paths drawer walks.
	Steps        []nav.Neighbors
	openMaps     map[string]bool
	openBranches map[string]bool
	openFolders  map[string]bool
}

// NewSidebar resolves the left navigation for one page. model is the whole-vault
// nav model; currentPath is the note being read ("" for a page with no note).
func NewSidebar(model *nav.Model, currentPath string, lang wording.Lang) Sidebar {
	// The current path arrives from the request URL; the model's indexes are
	// keyed by NFC paths, so fold it once here to match on either form.
	currentPath = vault.NormalizeNFC(currentPath)
	sb := Sidebar{
		Model:        model,
		CurrentPath:  currentPath,
		Lang:         lang,
		openMaps:     map[string]bool{},
		openBranches: map[string]bool{},
		openFolders:  map[string]bool{},
	}
	if model == nil || currentPath == "" {
		return sb
	}

	sb.HereDir, sb.Here = model.Siblings(currentPath)
	sb.Here, sb.HereTrimmed = windowAround(sb.Here, currentPath)
	sb.Steps = model.PathNeighbors(currentPath)

	// Open every map branch that lists the current note, down to the
	// branch it sits in (each heading prefix, so the ancestors open too).
	for _, p := range model.Placements(currentPath) {
		sb.openMaps[p.MapRelPath] = true
		headings := p.Headings
		for i := 1; i <= len(headings); i++ {
			sb.openBranches[branchKey(p.MapRelPath, headings[:i])] = true
		}
	}

	// A map or study path being read is its own wayfinding: the drawer
	// holding it opens with its tree, the same as for a note the map places.
	// Placements cannot say this — a map does not place itself.
	if model.IsMap(currentPath) || model.IsPath(currentPath) {
		sb.openMaps[currentPath] = true
	}

	// Expand the folder branches on the path down to the current note.
	for _, dir := range ancestorDirs(currentPath) {
		sb.openFolders[dir] = true
	}
	return sb
}

// current reports whether relPath is the note being read. The receiver is a
// pointer only to avoid copying the resolved sets on each of the many lookups a
// render makes; the methods never mutate.
func (s *Sidebar) current(relPath string) bool {
	return relPath != "" && relPath == s.CurrentPath
}

// mapOpen reports whether a map branch contains the current note.
func (s *Sidebar) mapOpen(relPath string) bool { return s.openMaps[relPath] }

// branchOpen reports whether a map branch lies on the path to the
// current note. It rebuilds the same key NewSidebar stored, so the two agree.
func (s *Sidebar) branchOpen(mapRelPath string, headings []string) bool {
	return s.openBranches[branchKey(mapRelPath, headings)]
}

// folderOpen reports whether a folder is an ancestor of the current note.
func (s *Sidebar) folderOpen(relPath string) bool { return s.openFolders[relPath] }

// folderTreeOpen reports whether the folder tree should start open — true when
// the current note lives in a folder, so its branch is revealed by default.
func (s *Sidebar) folderTreeOpen() bool { return len(s.openFolders) > 0 }

// currentAttr marks the entry the reader is on with aria-current="page", and
// contributes nothing otherwise, so assistive technology announces the one
// current location without every other link carrying a redundant marker.
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

// The type drawers open to meet the page the reader is on: the journal drawer
// on a journal page, the report drawer on a report, the study-path drawer on a
// note some path has placed. Everywhere else they render closed and the rail
// keeps one shape — the drawer for the thing in the reader's hand is the one
// standing open, and nothing reorders. These feed only the server-rendered
// default; a disclosure the reader toggled by hand is remembered for the
// session and still wins (the settle script replays it), and the wayfinding
// chain still wins over both.

// mapsChainOpen reports whether a general map holds — or is — the current
// note, which makes the surrounding group part of the wayfinding chain.
func (s *Sidebar) mapsChainOpen() bool {
	return s.chainOpen(s.Model.IsMap)
}

// chainOpen reports whether anything the wayfinding opened is of the kind
// belongs answers for. The open set is the handful of maps and courses that
// place the current note, plus the note itself when it is one of them, so the
// question is asked of those names rather than of every tree in the model.
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

// disclosureAttrs marks one sidebar disclosure for the single state owner
// that coordinates <details open>: key names the section stably across pages
// (the reader's manual toggles persist against it for the session), and
// chain marks the branch as part of the current note's wayfinding chain,
// which is always forced open and never persisted — recording a manual
// close would either hide the reader's own location on the next page or
// make persistence look broken.
func disclosureAttrs(key string, chain bool) templ.Attributes {
	attrs := templ.Attributes{"data-key": key}
	if chain {
		attrs["data-chain"] = true
	}
	return attrs
}

// railWindow is how many of a folder's files the rail lists around the one
// being read — in the siblings block and in the tree alike, which are two
// lists of the same folder and were both reciting all of it. The rail is a reading
// aid, not the folder: the folder page lists everything, the filter narrows to
// anything, and the step links under the article walk the sequence. What the
// rail owes the reader is where they are and what is immediately either side.
const railWindow = 24

// windowAround trims a folder's file list to the neighbourhood of the current
// file, and reports how many it left out. A folder within the window is
// returned whole, so the common case carries no elision at all.
func windowAround(files []nav.NoteRef, currentPath string) (window []nav.NoteRef, trimmed int) {
	if len(files) <= railWindow {
		return files, 0
	}
	at := slices.IndexFunc(files, func(n nav.NoteRef) bool { return n.RelPath == currentPath })
	if at < 0 {
		return files[:railWindow], len(files) - railWindow
	}
	start := max(0, at-railWindow/2)
	end := min(len(files), start+railWindow)
	start = max(0, end-railWindow)
	return files[start:end], len(files) - (end - start)
}

// TreeFiles returns the files a folder's branch lists inline, and how many it
// left for the folder's own page. The branch holding the note being read keeps
// that note in view: a tree that hid the reader's own location would be worse
// than a long one.
func (s *Sidebar) TreeFiles(f *nav.Folder) (shown []nav.NoteRef, trimmed int) {
	return windowAround(f.Notes, s.CurrentPath)
}

// Breadcrumb names each folder above a file, with the path that reaches it, so
// a reader can climb out of where they are. The trail was plain text for as
// long as there was nowhere for it to lead; three trial readers clicked it
// anyway, which is what a trail of folder names says it does.
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

// branchKey identifies a map branch by its path and the heading chain leading
// to it, so the resolved open set and the renderer name the same branch.
// The separator is a control character that never appears in a heading.
func branchKey(mapRelPath string, headings []string) string {
	return mapRelPath + "\x1f" + strings.Join(headings, "\x1f")
}

// ancestorDirs lists the directory chain from the vault root down to the file at
// relPath, so the folder tree expands exactly the branches that reveal it:
// "Concepts/golang/Foo.md" yields ["Concepts", "Concepts/golang"]. A vault-root
// file has no folder ancestors.
func ancestorDirs(relPath string) []string {
	i := strings.LastIndexByte(relPath, '/')
	if i < 0 {
		return nil
	}
	dir := relPath[:i]
	var dirs []string
	for start := 0; start <= len(dir); {
		next := strings.IndexByte(dir[start:], '/')
		if next < 0 {
			dirs = append(dirs, dir)
			break
		}
		dirs = append(dirs, dir[:start+next])
		start += next + 1
	}
	return dirs
}

// CapabilityFault is one closed navigation projection stated in the rail: the
// Traditional Chinese summary of what is unavailable, and the contract's own
// English detail.
type CapabilityFault struct {
	Summary string
	Detail  string
}

// CapabilityFaults lists what the rail has to say about closed navigation
// projections, one entry per distinct cause. One cause commonly closes both
// paths and instance projections — a contract that could not be read closes
// everything under it — and printing the same sentence twice under two headings
// reads as two faults rather than one.
func (s *Sidebar) CapabilityFaults() []CapabilityFault {
	if s.Model == nil {
		return nil
	}
	navigation := s.Model.NavigationClosure().Diagnostic()
	artifact := s.Model.ArtifactClosure().Diagnostic()
	switch {
	case navigation != "" && navigation == artifact:
		return []CapabilityFault{{Summary: wording.PathsMapsAndArtifactsUnavailable.In(s.Lang), Detail: navigation}}
	case navigation != "" && artifact != "":
		return []CapabilityFault{
			{Summary: wording.PathsAndMapsUnavailable.In(s.Lang), Detail: navigation},
			{Summary: wording.ArtifactsUnavailable.In(s.Lang), Detail: artifact},
		}
	case navigation != "":
		return []CapabilityFault{{Summary: wording.PathsAndMapsUnavailable.In(s.Lang), Detail: navigation}}
	case artifact != "":
		return []CapabilityFault{{Summary: wording.ArtifactsUnavailable.In(s.Lang), Detail: artifact}}
	default:
		return nil
	}
}

// FooterSequence chooses which order the foot of the article offers, and what
// to call it.
//
// The folder is a line for a folder of dated entries, and the arrow at the foot
// was born for that reader. A lesson is different: the course it belongs to
// declares its own order, printed in the rail on the same page, and the folder's
// alphabetical order is not it. A reader whose first course happened to sort
// into its teaching order learned to trust the arrow, then met a course where
// the same arrow led sixty lessons away from the next thing to learn.
//
// So a note exactly one course teaches steps through that course. A note no
// course teaches keeps the folder — nothing else is truer for it. A note two
// courses teach also keeps the folder, and the rail still reports both courses
// in full: which one the reader is walking is not something this can know, and
// this vault does not guess.
//
// course reports which order won, so the foot can print it: the label and the
// step words are all a sighted reader has to tell an author-declared course
// from mere folder adjacency, and the two orders can disagree completely.
//
// It is asked for by the one page that has a foot to put it in, rather than
// resolved for every page and read by that one: the rail on a search result or
// a folder listing carried this answer too, and nothing there ever looked at it.
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
// course hands the reader the previous or next lesson, a folder merely the
// file beside this one. A neighbouring file is not a recommended next step,
// and the word is where the foot says so.
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

// pathGroupDrawn reports whether the rail shows this branch of a study path:
// one the course includes, or a structural heading that carries one. A branch
// the grammar does not project is not a way to a lesson, so it is not a way
// through the rail either.
func pathGroupDrawn(g *nav.PathGroup) bool {
	if g.Projectable {
		return true
	}
	if !g.Carries {
		return false
	}
	for _, item := range g.Items {
		if item.Group != nil && pathGroupDrawn(item.Group) {
			return true
		}
	}
	return false
}
