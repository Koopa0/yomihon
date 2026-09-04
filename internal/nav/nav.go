// Package nav turns a scanner-captured vault projection into the folder tree,
// map and study-path trees, report list and recent-note summaries the sidebar
// and Home read. Each Model is immutable after construction and published with
// the atomic vault snapshot, so a request needs no lock and no metadata read.
// A malformed map or unreadable note is reported, never fatal, and every
// contract-derived value arrives from internal/schema — nav reads no contract.
package nav

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// Closure records why a projection carries nothing. It is a reason rather than
// a message: a projection can be withheld while a broader fault owns the only
// sentence worth printing, and an empty projection stays distinguishable from a
// withheld one. The zero Closure is open — the contents are the true answer.
type Closure struct {
	claim schema.Claim
}

// Closed reports whether the projection was withheld. An open projection whose
// contents are empty is not closed: for a vault that declared nothing, empty is
// the answer, not a refusal to answer.
func (c Closure) Closed() bool { return !c.claim.Trustworthy() }

// Diagnostic is the operator-facing sentence, empty when there is none to say
// here — either because nothing went wrong or because a broader fault already
// says it.
func (c Closure) Diagnostic() string { return c.claim.Diagnostic() }

// Close builds a closure from a declaration outcome.
func Close(claim schema.Claim) Closure { return Closure{claim: claim} }

// Model is the complete navigation projection, built once and read-only
// afterward.
type Model struct {
	// navigation records whether contract-derived paths and maps were withheld,
	// and why. It is open when navigation roles were read cleanly and open when
	// no contract ever named a path or map type.
	navigation Closure
	// artifact records whether instance projections were withheld, and why.
	artifact Closure

	// folders are the top-level folders in lifecycleOrder, each holding its
	// notes and subfolders recursively.
	folders []Folder
	// rootNotes are files at the vault root itself, belonging to no folder.
	rootNotes []NoteRef
	// paths are study paths in vault path order.
	paths []Path
	// maps are every other map-note tree, ordered by domain and then title.
	maps []Map
	// journal is the most recent captured journal entries, newest first, taken
	// from the file listing rather than from any note type.
	journal []JournalEntry
	// reports enumerates System/reports/ — the .md reports first, then the
	// daily-briefing/ HTML briefings; contents are never parsed.
	reports []Report
	// knowledgeNotes are markdown notes in vault path order, carrying the
	// scanner's captured times so rendering never stats a note. It is plain
	// reading and is built in every contract state: a folder whose contract
	// broke must not show less than one that never had a contract at all.
	knowledgeNotes []NoteSummary
	// knowledgeScoped records that the contract declared a knowledge layer, so
	// knowledgeNotes lists less than the whole folder.
	knowledgeScoped bool

	// placementIndex maps a note's rel-path to every map placement that lists
	// it. Read it through Placements.
	placementIndex map[string][]Placement
	// dirNotes maps a directory's rel-path to the files directly inside it.
	// Read it through Siblings.
	dirNotes map[string][]NoteRef
}

// NavigationClosure reports whether paths and maps were withheld, and why.
func (m *Model) NavigationClosure() Closure {
	if m == nil {
		return Closure{}
	}
	return m.navigation
}

// ArtifactClosure reports whether instance projections were withheld, and why.
func (m *Model) ArtifactClosure() Closure {
	if m == nil {
		return Closure{}
	}
	return m.artifact
}

// DeclaredClosure is the one answer for whether the projections a contract's
// own declarations produce — study paths and maps — were withheld, and why.
// Either closure can withhold them, and one cause commonly trips both, so a
// surface listing paths or maps asks here rather than keeping its own rule for
// joining two answers.
func (m *Model) DeclaredClosure() Closure {
	if m == nil {
		return Closure{}
	}
	if m.navigation.Closed() {
		return m.navigation
	}
	return m.artifact
}

// Folders returns the top-level lifecycle folders in vault order. The complete
// returned tree is independent of the model.
func (m *Model) Folders() []Folder {
	if m == nil {
		return nil
	}
	return cloneFolders(m.folders)
}

// RootNotes returns the files that live at the vault root itself.
func (m *Model) RootNotes() []NoteRef {
	if m == nil {
		return nil
	}
	return slices.Clone(m.rootNotes)
}

// PathCount reports how many study paths the model holds, so asking whether
// there is a course to show costs a length rather than a copy of every tree.
func (m *Model) PathCount() int {
	if m == nil {
		return 0
	}
	return len(m.paths)
}

// MapCount reports how many general maps the model holds.
func (m *Model) MapCount() int {
	if m == nil {
		return 0
	}
	return len(m.maps)
}

// Paths returns the study paths in vault path order. Everything the caller
// receives is its own: the model is read by every request at once.
func (m *Model) Paths() []Path {
	if m == nil {
		return nil
	}
	out := make([]Path, 0, len(m.paths))
	for i := range m.paths {
		out = append(out, m.paths[i].clone())
	}
	return out
}

// Maps returns every non-path map tree, ordered by domain and title.
func (m *Model) Maps() []Map {
	if m == nil {
		return nil
	}
	return cloneMaps(m.maps)
}

// Journal returns the most recent captured journal entries, newest first.
func (m *Model) Journal() []JournalEntry {
	if m == nil {
		return nil
	}
	return slices.Clone(m.journal)
}

// Reports returns the files captured below System/reports/.
func (m *Model) Reports() []Report {
	if m == nil {
		return nil
	}
	return slices.Clone(m.reports)
}

// KnowledgeNotes returns typed Markdown notes in vault path order.
func (m *Model) KnowledgeNotes() []NoteSummary {
	if m == nil {
		return nil
	}
	return slices.Clone(m.knowledgeNotes)
}

// KnowledgeScoped reports whether KnowledgeNotes lists less than the whole
// folder. Where nothing was declared, nothing is out of scope and this is
// false.
func (m *Model) KnowledgeScoped() bool {
	return m != nil && m.knowledgeScoped
}

// WithoutInstanceProjections returns a request-local view that keeps ordinary
// browsing surfaces but closes every artifact-policy-dependent projection; the
// closure travels with it so a later reader can tell withheld from empty. The
// recent-notes summary stays, being plain reading, but loses its
// knowledge-layer citation — that layer is the contract's own claim.
func (m *Model) WithoutInstanceProjections(closure Closure) *Model {
	if m == nil {
		return &Model{artifact: closure}
	}
	degraded := *m
	degraded.artifact = closure
	degraded.paths = nil
	degraded.maps = nil
	degraded.knowledgeScoped = false
	degraded.placementIndex = nil
	return &degraded
}

// NoteSummary is the navigation metadata Home needs for one knowledge note.
// Modified is the scanner's captured time, zero only where the scan could not
// stat a note that remained readable.
type NoteSummary struct {
	Title    string
	RelPath  string
	Type     string
	Status   string
	Modified time.Time
}

// JournalEntry is one recent Diary markdown file, carrying the scanner's
// captured time rather than one read while rendering.
type JournalEntry struct {
	Title    string
	RelPath  string
	Modified time.Time
}

// Folder is one directory in the browse tree: its display name, its
// vault-relative path, the notes directly inside it, and its subfolders.
type Folder struct {
	Name       string
	RelPath    string
	Notes      []NoteRef
	Subfolders []Folder
}

func cloneFolders(source []Folder) []Folder {
	cloned := slices.Clone(source)
	for i := range cloned {
		cloned[i].Notes = slices.Clone(source[i].Notes)
		cloned[i].Subfolders = cloneFolders(source[i].Subfolders)
	}
	return cloned
}

// NoteRef is one browsable file: a display name and the vault-relative path
// the reading page links to.
type NoteRef struct {
	Name    string
	RelPath string
}

// Report is one file under System/reports/. Briefing marks the daily-briefing/
// HTML files (as opposed to the .md reports); Latest marks latest.html.
// nav records names and paths only — report contents are never parsed.
type Report struct {
	Name     string
	RelPath  string
	Briefing bool
	Latest   bool
}

// lifecycleOrder is the reading order of the top-level folders. It is a
// display ordering, not a vocabulary: vault-schema.toml carries no
// display-order field, so there is nothing here to read it from. A folder the
// list does not name sorts after every named one and keeps the captured path
// order, so it stays reachable instead of vanishing from the sidebar.
var lifecycleOrder = []string{
	"Inbox", "Sources", "Concepts", "Maps", "Synthesis", "Writing",
	"System", "Views", "Diagrams",
}

// lifecycleRank is a top-level folder's position in lifecycleOrder, or
// len(lifecycleOrder) for any folder not listed (sorts last).
func lifecycleRank(name string) int {
	if i := slices.Index(lifecycleOrder, name); i >= 0 {
		return i
	}
	return len(lifecycleOrder)
}

// New constructs a navigation model from one captured vault projection: entries
// supply the canonical paths and observed times, notes the parsed Markdown keyed
// by canonical path. A missing note reads as an unreadable one and does not
// affect its neighbors. New neither enumerates nor reopens the vault.
func New(
	entries []vaultfs.Entry,
	notes map[string]*vault.Note,
	resolver *graph.Index,
	roles schema.NavigationRoles,
	scope schema.KnowledgeScope,
	policy schema.ArtifactPolicy,
) *Model {
	if resolver == nil {
		panic("nav: New requires a non-nil *graph.Index")
	}

	observed := slices.Clone(entries)
	slices.SortFunc(observed, func(a, b vaultfs.Entry) int {
		return vault.ComparePaths(a.Path(), b.Path())
	})
	files := make([]capturedFile, 0, len(observed))
	for _, entry := range observed {
		path := entry.Path()
		note := notes[path]
		if note != nil {
			cloned := *note
			cloned.RelPath = path
			note = &cloned
		}
		files = append(files, capturedFile{
			path:     path,
			modified: entry.ModTime(),
			note:     note,
		})
	}
	return newModel(files, resolver, roles, scope, policy)
}

// capturedFile is the portion of a scanner observation used by navigation.
// note is nil when the file is not Markdown or could not be read.
type capturedFile struct {
	path     string
	modified time.Time
	note     *vault.Note
}

func newModel(
	files []capturedFile,
	resolver *graph.Index,
	roles schema.NavigationRoles,
	scope schema.KnowledgeScope,
	policy schema.ArtifactPolicy,
) *Model {
	paths := make([]string, 0, len(files))
	mtimes := make(map[string]time.Time, len(files))
	for _, file := range files {
		paths = append(paths, file.path)
		mtimes[file.path] = file.modified
	}
	m := &Model{
		reports: buildReports(paths),
		journal: buildJournal(paths, mtimes),
	}
	m.folders, m.rootNotes = buildFolderTree(paths)
	m.dirNotes = buildDirNotes(paths)
	m.navigation = Close(roles.Claim())
	m.artifact = Close(policy.Claim())
	// The recent-notes summary is collected in every contract state; paths and
	// maps exist only as a contract's own classification, so either closed
	// declaration ends the build with none of them.
	statusByPath, mapNotes, knowledgeNotes := collectNavigationNotes(files, roles, scope, policy)
	m.knowledgeNotes = knowledgeNotes
	m.knowledgeScoped = scope.Available()
	if m.artifact.Closed() || m.navigation.Closed() {
		return m
	}

	for _, n := range mapNotes {
		if roles.IsPathType(n.Type()) {
			// A study path reads the declared-sequence grammar, never the
			// general-map parser.
			m.paths = append(m.paths, buildPath(n, resolver, statusByPath, policy))
			continue
		}
		m.maps = append(m.maps, parseMap(n, resolver, statusByPath, policy))
	}
	slices.SortStableFunc(m.maps, func(a, b Map) int {
		if byDomain := cmp.Compare(a.Domain, b.Domain); byDomain != 0 {
			return byDomain
		}
		if byTitle := cmp.Compare(a.Title, b.Title); byTitle != 0 {
			return byTitle
		}
		return cmp.Compare(a.RelPath, b.RelPath)
	})
	// Study paths are indexed first, so a note both a course and a map place
	// answers with its course.
	m.placementIndex = make(map[string][]Placement)
	for i := range m.paths {
		pathPlacements(m.placementIndex, &m.paths[i])
	}
	buildPlacementIndex(m.placementIndex, m.maps)
	return m
}

// collectNavigationNotes projects already parsed notes into entry badges, Home
// summaries, and the map notes parsed by newModel. An absent note is skipped
// without affecting its neighbors.
func collectNavigationNotes(
	files []capturedFile,
	roles schema.NavigationRoles,
	scope schema.KnowledgeScope,
	policy schema.ArtifactPolicy,
) (map[string]string, []*vault.Note, []NoteSummary) {
	statusByPath := make(map[string]string)
	var mapNotes []*vault.Note
	var knowledgeNotes []NoteSummary
	for _, file := range files {
		p := file.path
		if !vault.IsMarkdown(p) || policy.IsNonInstance(p) {
			continue
		}
		n := file.note
		if n == nil {
			continue
		}
		if status := n.Status(); status != "" {
			statusByPath[p] = status
		}
		// Membership is the vault's own declaration, not whether a note happens
		// to carry a type: a note without frontmatter is still one its author
		// wrote and still the newest thing they changed.
		if scope.Includes(p) {
			knowledgeNotes = append(knowledgeNotes, NoteSummary{
				Title:    n.Title(),
				RelPath:  p,
				Type:     n.Type(),
				Status:   n.Status(),
				Modified: file.modified,
			})
		}
		if roles.IsPathType(n.Type()) || roles.IsMapType(n.Type()) {
			mapNotes = append(mapNotes, n)
		}
	}
	return statusByPath, mapNotes, knowledgeNotes
}

// The journal and report projections select by location alone, and the sidebar
// drawers ask the same question, so each prefix is named once and reached
// through a predicate rather than copied.
const (
	journalPrefix = "Diary/"
	reportsPrefix = "System/reports/"
)

// InJournal reports whether relPath lives in the journal.
func InJournal(relPath string) bool { return strings.HasPrefix(relPath, journalPrefix) }

// InReports reports whether relPath lives among the reports.
func InReports(relPath string) bool { return strings.HasPrefix(relPath, reportsPrefix) }

// buildJournal selects markdown files below Diary from the scanner's path and
// mtime captures. It does not parse frontmatter, so an untyped entry remains
// eligible. Nothing here reads a timestamp: the order is the entries' own
// names, for the reason the sort itself gives, and the mtime each entry carries
// is a field no surface draws today — the rail shows a journal entry's title
// and its address, and nothing else.
func buildJournal(paths []string, mtimes map[string]time.Time) []JournalEntry {
	const limit = 5
	entries := make([]JournalEntry, 0, limit)
	for _, p := range paths {
		if !InJournal(p) || !vault.IsMarkdown(p) {
			continue
		}
		_, base := splitDir(p)
		entries = append(entries, JournalEntry{Title: displayName(base), RelPath: p, Modified: mtimes[p]})
	}
	// Ordered by the entries' own names, not by file time: a clone stamps every
	// entry with one moment, and an entry edited today is not today's entry.
	slices.SortStableFunc(entries, func(a, b JournalEntry) int {
		return vault.ComparePaths(b.RelPath, a.RelPath)
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

// buildReports enumerates System/reports/: the .md reports directly in that
// folder (in path order), then the daily-briefing/ HTML briefings (marking
// latest.html). It reads only the path list — report contents are never
// opened. README.md files and any non-.md/.html files fall out naturally.
func buildReports(paths []string) []Report {
	const briefingDir = "daily-briefing"

	var reports, briefings []Report
	for _, p := range paths {
		rest, ok := strings.CutPrefix(p, reportsPrefix)
		if !ok {
			continue
		}
		if !strings.Contains(rest, "/") {
			if vault.IsMarkdown(rest) {
				reports = append(reports, Report{Name: rest, RelPath: p})
			}
			continue
		}
		sub, file, _ := strings.Cut(rest, "/")
		if sub == briefingDir && !strings.Contains(file, "/") && strings.HasSuffix(file, ".html") {
			briefings = append(briefings, Report{
				Name:     file,
				RelPath:  p,
				Briefing: true,
				Latest:   file == "latest.html",
			})
		}
	}
	return append(reports, briefings...)
}

// folderBuilder is the mutable tree node used only while assembling the
// folder tree from the flat path list; it is converted to the read-only
// Folder value type once complete.
type folderBuilder struct {
	name    string
	relPath string
	notes   []NoteRef
	subs    []*folderBuilder
	subIdx  map[string]*folderBuilder
}

// buildFolderTree turns a flat path list, already in the captured reading
// order, into the top-level folder tree plus the vault-root notes. It mirrors
// the directory structure to whatever depth the vault has, inventing no level
// and capping none. Only the top level is reordered into lifecycleOrder.
func buildFolderTree(paths []string) (folders []Folder, rootNotes []NoteRef) {
	root := &folderBuilder{subIdx: map[string]*folderBuilder{}}
	for _, p := range paths {
		dir, base := splitDir(p)
		parent := ensureFolder(root, dir)
		parent.notes = append(parent.notes, NoteRef{Name: displayName(base), RelPath: p})
	}

	top := slices.Clone(root.subs)
	slices.SortStableFunc(top, func(a, b *folderBuilder) int {
		return cmp.Compare(lifecycleRank(a.name), lifecycleRank(b.name))
	})
	folders = make([]Folder, 0, len(top))
	for _, fb := range top {
		folders = append(folders, fb.toFolder())
	}
	return folders, root.notes
}

// ensureFolder walks dir ("Concepts/golang") from root, creating each
// missing segment in the order the input list first names it, and returns
// the deepest folder. dir == "" returns root itself.
func ensureFolder(root *folderBuilder, dir string) *folderBuilder {
	if dir == "" {
		return root
	}
	cur := root
	var prefix string
	for seg := range strings.SplitSeq(dir, "/") {
		if prefix == "" {
			prefix = seg
		} else {
			prefix += "/" + seg
		}
		child, ok := cur.subIdx[seg]
		if !ok {
			child = &folderBuilder{name: seg, relPath: prefix, subIdx: map[string]*folderBuilder{}}
			cur.subIdx[seg] = child
			cur.subs = append(cur.subs, child)
		}
		cur = child
	}
	return cur
}

// toFolder converts a builder subtree to the read-only Folder value.
func (fb *folderBuilder) toFolder() Folder {
	f := Folder{Name: fb.name, RelPath: fb.relPath, Notes: slices.Clone(fb.notes)}
	if len(fb.subs) > 0 {
		f.Subfolders = make([]Folder, 0, len(fb.subs))
		for _, s := range fb.subs {
			f.Subfolders = append(f.Subfolders, s.toFolder())
		}
	}
	return f
}

// splitDir splits a slash path into its directory and base name; a path
// with no slash (a vault-root file) has an empty directory.
func splitDir(p string) (dir, base string) {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i], p[i+1:]
	}
	return "", p
}

// displayName is a file's browse-tree label: markdown notes drop the .md
// suffix (as Obsidian's file explorer shows them), while other resources
// (.base, .canvas, .html, ...) keep their extension so their kind stays
// visible.
func displayName(base string) string {
	return strings.TrimSuffix(base, ".md")
}

// Label names a file from its vault-relative path the way every list here names
// it, so one note cannot appear under two names in two lists.
func Label(relPath string) string {
	_, base := splitDir(relPath)
	return displayName(base)
}
