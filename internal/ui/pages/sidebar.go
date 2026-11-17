package pages

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// SidebarMode names which of the four contents the sidebar holds. The page
// decides, by what it is: the library is the way into the whole folder, a
// course is the units of the course being read, a folder is the notes beside
// the one being read, an index is the list of indexes.
type SidebarMode string

const (
	SidebarLibrary SidebarMode = "library"
	SidebarCourse  SidebarMode = "course"
	SidebarFolder  SidebarMode = "folder"
	SidebarIndex   SidebarMode = "index"
)

// LibraryItem names the row of the library the page being read stands on.
type LibraryItem string

const (
	LibraryNone    LibraryItem = ""
	LibraryDesk    LibraryItem = "desk"
	LibraryCourses LibraryItem = "courses"
	LibraryIndexes LibraryItem = "indexes"
	LibraryJournal LibraryItem = "journal"
	LibraryReports LibraryItem = "reports"
)

// Sidebar is the data of the left column for one request, whichever of its four
// modes the page asks for: the mode, what the page stands on, and what that mode
// lists. One component draws it, so what the four share (the landmark, the fold
// button, the folded spine, the filter, the foot) is drawn once. A nil model is
// tolerated, because the column is drawn beside the not-found page a reader
// reaches when the folder could not be projected at all, and stopping there
// would take down the page that exists to explain why.
type Sidebar struct {
	Mode  SidebarMode
	Model *nav.Model
	// Vault is the folder this sidebar is of, which its foot states.
	Vault nav.Vault
	// DrawerOnly is a page with no column of its own: the sidebar is there to be
	// opened below the width where it is a drawer and is not drawn above it.
	DrawerOnly bool
	// CurrentPath is the note being read, empty on a page with no note.
	CurrentPath string
	// KeptNote is the vault-relative note the reader kept a place in, empty
	// where they kept none. The course marks the row of that unit, and a place
	// kept in a note the course does not teach marks nothing.
	KeptNote string

	// note is the page this sidebar is drawn beside, which the row that is the
	// note opens into the contents of. It is nil beside no note.
	note *NoteView

	// Library: the row the page stands on, and the top-level folder it is in.
	selected LibraryItem
	inFolder string
	// Course.
	book      *nav.Course
	neighbors nav.Neighbors
	// place is where the note sits in the course: its part, and what a side
	// branch hands over at its ends. It is the zero value for a note the course
	// does not walk, which is the case neighbors.PathRelPath says.
	place        nav.Place
	openBranches map[string]bool
	// Folder: the folder the note is in and what is beside it, or the reports
	// where the note is one.
	hereDir string
	here    []nav.NoteRef
	reports bool
}

// NewLibrarySidebar is the sidebar of a page that is a way into the folder
// rather than one note: the desk, the lists, a search. selected is the row it
// stands on, and inFolder the top-level folder it is in, either of them empty.
func NewLibrarySidebar(shell nav.Shell, selected LibraryItem, inFolder string) Sidebar {
	return Sidebar{Mode: SidebarLibrary, Model: shell.Nav, Vault: shell.Vault, selected: selected, inFolder: inFolder}
}

// NewDrawerSidebar is the sidebar of a page that has no column: the library,
// drawn only where the sidebar is a drawer.
func NewDrawerSidebar(shell nav.Shell) Sidebar {
	s := NewLibrarySidebar(shell, LibraryNone, "")
	s.DrawerOnly = true
	return s
}

// NewFolderSidebar is the sidebar of a page about one file that no course or
// index teaches: the folder it is in and the files beside it.
func NewFolderSidebar(shell nav.Shell, currentPath string) Sidebar {
	currentPath = vault.NormalizeNFC(currentPath)
	s := Sidebar{Mode: SidebarFolder, Model: shell.Nav, Vault: shell.Vault, CurrentPath: currentPath}
	if s.Model != nil && currentPath != "" {
		s.hereDir, s.here = s.Model.Siblings(currentPath)
	}
	return s
}

// NewNoteSidebar resolves the mode a note page should show: the course that
// teaches it, else the index it is, else the folder it is in. noteDomain is the
// declared domain of the note being read, used only to break a tie among several
// courses.
func NewNoteSidebar(shell nav.Shell, currentPath, noteDomain string) Sidebar {
	model := shell.Nav
	currentPath = vault.NormalizeNFC(currentPath)
	s := Sidebar{Mode: SidebarFolder, Model: model, Vault: shell.Vault, CurrentPath: currentPath}
	if model == nil || currentPath == "" {
		return s
	}
	if nav.InReports(currentPath) {
		s.reports = true
		return s
	}
	if book := model.TeachingPath(currentPath, noteDomain); book != nil {
		s.Mode = SidebarCourse
		s.book = book
		s.openBranches = map[string]bool{}
		for _, p := range model.Placements(currentPath) {
			if p.MapRelPath != book.RelPath {
				continue
			}
			headings := p.Headings
			for i := 1; i <= len(headings); i++ {
				s.openBranches[branchKey(p.MapRelPath, headings[:i])] = true
			}
		}
		neighbors := model.PathNeighbors(currentPath)
		for i := range neighbors {
			step := neighbors[i]
			if step.PathRelPath == book.RelPath {
				s.neighbors = step
				s.place, _ = model.PathPlace(currentPath, book.RelPath)
				break
			}
		}
		return s
	}
	if model.IsMap(currentPath) {
		s.Mode = SidebarIndex
		return s
	}
	s.hereDir, s.here = model.Siblings(currentPath)
	return s
}

// HereShelf is what the folder mode lists, as a shelf at sidebar width: every
// file in the folder in its own order, the one being read marked, and the
// folder's page as the way to the rest of it. Beside a report it is the
// reports. It is built at render time because the words in it are the page's.
func (s *Sidebar) HereShelf(lang wording.Lang) Shelf {
	if s.reports {
		rows := make([]Row, 0)
		if s.Model != nil {
			for _, rep := range s.Model.Reports() {
				rows = append(rows, Row{Text: reportLabel(&rep), Href: notesHref(rep.RelPath), Current: s.current(rep.RelPath)})
			}
		}
		return Shelf{Title: wording.Reports.In(lang), Href: indexHref(reportMode), Rows: rows}
	}
	rows := make([]Row, 0, len(s.here))
	for _, n := range s.here {
		rows = append(rows, Row{Text: n.Name, Href: notesHref(n.RelPath), Current: s.current(n.RelPath), Language: n.Language})
	}
	return Shelf{Title: hereLabel(s.hereDir, lang), Href: folderHref(s.hereDir), Rows: rows}
}

// where is how the folder mode says where it is: the folder's own path, or the
// word for the folder's root where the note sits at the top.
func (s *Sidebar) where(lang wording.Lang) string {
	if s.reports {
		return wording.Reports.In(lang)
	}
	if s.hereDir == "" {
		return wording.VaultRoot.In(lang)
	}
	return s.hereDir
}

// railSpineView is the name a sidebar keeps on its folded edge and where that
// name leads.
type railSpineView struct {
	Title string
	Href  string
}

// spine is what this sidebar is called when it is folded to a strip: the
// course's own title, the folder's path, the indexes, or the folder itself. A
// sidebar that has none to give draws nothing there.
func (s *Sidebar) spine(lang wording.Lang) railSpineView {
	switch s.Mode {
	case SidebarCourse:
		if s.book != nil {
			return railSpineView{Title: s.book.Title, Href: courseHrefFrom(s.book.RelPath, s.CurrentPath)}
		}
	case SidebarFolder:
		if s.Model != nil {
			if s.reports {
				return railSpineView{Title: wording.Reports.In(lang), Href: indexHref(reportMode)}
			}
			return railSpineView{Title: s.where(lang), Href: folderHref(s.hereDir)}
		}
	case SidebarIndex:
		return railSpineView{Title: wording.Maps.In(lang), Href: indexHref(mapMode)}
	case SidebarLibrary:
		if name := libraryName(s.Vault.Name, lang); name != "" {
			return railSpineView{Title: name, Href: "/"}
		}
	}
	return railSpineView{}
}

func (s *Sidebar) current(relPath string) bool {
	return relPath != "" && relPath == s.CurrentPath
}

// branchOpen reports whether a course branch lies on the path to the current
// note.
func (s *Sidebar) branchOpen(pathRel string, headings []string) bool {
	return s.openBranches[branchKey(pathRel, headings)]
}

// CapabilityFaults lists what the sidebar has to say about closed navigation
// projections, one entry per distinct cause: one cause commonly closes both
// paths and instance projections, and saying it twice reads as two faults.
func (s *Sidebar) CapabilityFaults(lang wording.Lang) []CapabilityFault {
	return ModelCapabilityFaults(s.Model, lang)
}

// sidebarLink is one row of the library or the index list: where it goes, what
// it is called, and the figure at its end where it has one.
type sidebarLink struct {
	Text    string
	Href    string
	Count   int
	Current bool
}

// libraryLinks is the first block of the library: the desk, then each list the
// folder has anything in, with how many. A list with nothing in it is not
// offered, and the desk always is.
func (s *Sidebar) libraryLinks(lang wording.Lang) []sidebarLink {
	links := []sidebarLink{{Text: wording.HomeTitle.In(lang), Href: "/", Current: s.selected == LibraryDesk}}
	if s.Model == nil {
		return links
	}
	for _, item := range []struct {
		id    LibraryItem
		text  wording.Phrase
		href  string
		count int
	}{
		{LibraryCourses, wording.Courses, indexHref(courseMode), s.Model.PathCount()},
		{LibraryIndexes, wording.Maps, indexHref(mapMode), s.Model.MapCount()},
		{LibraryJournal, wording.Journal, indexHref(journalMode), s.Model.JournalCount()},
		{LibraryReports, wording.Reports, indexHref(reportMode), len(s.Model.Reports())},
	} {
		if item.count > 0 {
			links = append(links, sidebarLink{Text: item.text.In(lang), Href: item.href, Count: item.count, Current: s.selected == item.id})
		}
	}
	return links
}

// libraryFolders is the folder's top-level folders, the fullest first, as far as
// the sidebar has room for, and how many were left off.
func (s *Sidebar) libraryFolders() (shown []sidebarLink, trimmed int) {
	if s.Model == nil {
		return nil, 0
	}
	folders := s.Model.ShelfFolders()
	links := make([]sidebarLink, 0, len(folders))
	for i := range folders {
		links = append(links, sidebarLink{Text: folders[i].Name, Href: folderHref(folders[i].RelPath), Count: folderNotes(&folders[i]), Current: folders[i].RelPath == s.inFolder})
	}
	slices.SortStableFunc(links, func(a, b sidebarLink) int { return cmp.Compare(b.Count, a.Count) })
	if len(links) > railWindow {
		return links[:railWindow], len(links) - railWindow
	}
	return links, 0
}

// folderNotes is how many markdown notes a folder holds at every depth.
func folderNotes(f *nav.Folder) int {
	total := 0
	for _, note := range f.Notes {
		if vault.IsMarkdown(note.RelPath) {
			total++
		}
	}
	for i := range f.Subfolders {
		total += folderNotes(&f.Subfolders[i])
	}
	return total
}

// indexLinks is the index mode's list: each index with how many notes it names,
// the one being read marked.
func (s *Sidebar) indexLinks() []sidebarLink {
	if s.Model == nil {
		return nil
	}
	maps := s.Model.Maps()
	links := make([]sidebarLink, 0, len(maps))
	for i := range maps {
		links = append(links, sidebarLink{Text: maps[i].Title, Href: notesHref(maps[i].RelPath), Count: mapNotes(maps[i].Branches), Current: s.current(maps[i].RelPath)})
	}
	return links
}

// mapNotes is how many notes an index's branches name that it could reach.
func mapNotes(branches []nav.Branch) int {
	total := 0
	for i := range branches {
		for _, entry := range branches[i].Entries {
			if entry.Kind == nav.EntryResolved {
				total++
			}
		}
		total += mapNotes(branches[i].Subbranches)
	}
	return total
}

// railRow is one stop of the course in the sidebar: a unit, or a branch drawn
// after the unit it hangs from.
type railRow struct {
	entry  *PathEntryView
	branch *PathBranchView
}

// railRows lists what the sidebar draws for a run, in the order it draws it: a
// nested branch alone, or each unit followed by the side branches hanging from
// it, which is where the author wrote them. The sidebar draws a branch as a
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

// bookView draws the teaching course into the page view the sidebar reuses. The
// note being read is handed over as the row to mark, so the sidebar and the
// course page both learn which row that is from the one comparison, and neither
// keeps a second answer that could disagree with the other.
//
// Nothing else of the cover reaches here beyond the reader's kept place: the
// sidebar is the course beside the note being read, and the opening and the verb
// that opens a course belong to the page that is the course.
func (s *Sidebar) bookView() PathView {
	if s.book == nil {
		return PathView{}
	}
	return BuildPathView(s.book, &CourseCover{Here: s.CurrentPath, KeptNote: s.KeptNote})
}

// currentAttr marks the entry the reader is on with aria-current="page" and
// contributes nothing otherwise.
func currentAttr(isCurrent bool) templ.Attributes {
	if isCurrent {
		return templ.Attributes{"aria-current": "page"}
	}
	return nil
}

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
	if _, after, found := strings.CutLast(dir, "/"); found {
		return after
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

// ModelCapabilityFaults is the shared closure logic for any rail that carries
// capability faults beside a reading surface.
func ModelCapabilityFaults(model *nav.Model, lang wording.Lang) []CapabilityFault {
	if model == nil {
		return nil
	}
	navigation := model.NavigationClosure()
	artifact := model.ArtifactClosure()
	switch {
	case navigation.Closed() && artifact.Closed() && navigation.Diagnostic() == artifact.Diagnostic():
		return []CapabilityFault{{Summary: wording.CoursesMapsAndArtifactsUnavailable.In(lang), Detail: navigation.Diagnostic()}}
	case navigation.Closed() && artifact.Closed():
		return []CapabilityFault{
			{Summary: wording.CoursesAndMapsUnavailable.In(lang), Detail: navigation.Diagnostic()},
			{Summary: wording.ArtifactsUnavailable.In(lang), Detail: artifact.Diagnostic()},
		}
	case navigation.Closed():
		return []CapabilityFault{{Summary: wording.CoursesAndMapsUnavailable.In(lang), Detail: navigation.Diagnostic()}}
	case artifact.Closed():
		return []CapabilityFault{{Summary: wording.ArtifactsUnavailable.In(lang), Detail: artifact.Diagnostic()}}
	default:
		return nil
	}
}

// Foot is the way onward the foot of an article offers: the notes either side
// of this one in the order that won, what to call that order, and which kind it
// is.
//
// The rest belongs to a course. Path is the path whose order it is, which the
// foot names with a link to the contents. A side branch's lessons carry the
// ways off it: Back on its first lesson, Onward or ToContents on its last. The
// lesson a branch hangs from carries Asides, one for each branch. None of them
// is Prev or Next, because none of them is a step in the course's order, and
// the main line's own steps are what they were without the branches.
type Foot struct {
	Prev, Next nav.NoteRef
	// Label names the order, spoken as the foot's name.
	Label string
	// Course says the order is a study path's and not the folder's, which
	// picks the step words.
	Course bool

	Path       nav.NoteRef
	Back       nav.NoteRef
	Onward     nav.NoteRef
	ToContents bool
	Asides     []nav.NoteRef
}

// FooterSequence chooses which order the foot of the article offers, and what to
// call it. The rail has already resolved which book, if any, the page is being
// read inside, and the foot walks that same book: the foot is the one place a
// page in a book offers a lesson onward, since the rail's head carries only the
// course's name and extent.
//
// A note the book walks never gets the folder's order, even where the course
// has no step to offer it: a course's declared order can contradict the
// folder's alphabetical one completely, so a foot that fell back to it would
// point the wrong way and call it previous. A lesson with no step then shows
// nothing but what a branch hands it. A page whose rail resolved no book — none
// teaches this note, or several do and none of them is its own — keeps the
// folder, and so does the course's own note, which the book does not walk.
// A course foot names the step onward, not the path's whole order.
//
// Foot.Course reports which order won, so the foot can print it: that and the
// step words are all a sighted reader has to tell a course from folder
// adjacency.
func FooterSequence(rail *Sidebar, lang wording.Lang) Foot {
	if rail == nil || rail.Model == nil || rail.CurrentPath == "" {
		return Foot{}
	}
	step := rail.neighbors
	if step.PathRelPath != "" {
		return Foot{
			Prev:       step.Prev,
			Next:       step.Next,
			Label:      fmt.Sprintf(wording.CourseOnwardOf.In(lang), step.PathTitle),
			Course:     true,
			Path:       nav.NoteRef{Name: step.PathTitle, RelPath: step.PathRelPath},
			Back:       rail.place.Back,
			Onward:     rail.place.Onward,
			ToContents: rail.place.ToContents,
			Asides:     rail.place.Asides,
		}
	}
	prev, next := rail.Model.FolderStep(rail.CurrentPath)
	return Foot{Prev: prev, Next: next, Label: wording.FolderAdjacency.In(lang)}
}

// stepWordPrev and stepWordNext name a footer step for the order it walks: a
// course hands over what it is made of, a folder merely the file beside this
// one.
func stepWordPrev(course bool, lang wording.Lang) string {
	if course {
		return wording.Units.Previous.In(lang)
	}
	return wording.PreviousFile.In(lang)
}

func stepWordNext(course bool, lang wording.Lang) string {
	if course {
		return wording.Units.Next.In(lang)
	}
	return wording.NextFile.In(lang)
}

// railToggleTitle is the tooltip the rail's fold control carries on the first
// byte: the action it will take from the state the cookie says the column is
// in, with the key that takes it when single keys are on. The script keeps it
// in step afterwards from the same phrases, which the control carries beside it.
func railToggleTitle(rail string, lang wording.Lang, keys bool) string {
	title := wording.HideRail.In(lang)
	if rail == "collapsed" {
		title = wording.ShowRail.In(lang)
	}
	if keys {
		title += wording.RailKeyHint.In(lang)
	}
	return title
}
