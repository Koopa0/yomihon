// Package nav builds the vault's read-only navigation model. It turns a
// scanner-captured vault projection into the structures used by the sidebar
// and Home:
//
//  1. a lifecycle folder tree (browse every note without typing a URL),
//  2. parsed map-note trees (heading branches -> resolved note entries, in
//     document order),
//  3. a flat list of the reports under System/reports/, and
//  4. recent Journal entries and knowledge-note summaries carrying
//     scanner-captured modification times.
//
// Each Model is immutable after construction and is published as part of the
// atomic vault snapshot, so requests need no locking or filesystem metadata
// reads.
//
// Two invariants shape every decision here:
//
//   - Fault-tolerant. A malformed map, a broken entry link,
//     an unreadable note, an odd folder — none may panic or drop the rest.
//     Study paths retain unresolved, ambiguous, and non-instance targets as
//     warnings because their order is a curriculum; general maps omit warnings
//     because their tree is link navigation.
//   - Contract-derived. Navigation roles and artifact boundaries are immutable
//     values derived by internal/schema at startup; nav never reads the contract.
package nav

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Resolver is the minimal wikilink-resolution capability nav needs to turn
// an entry's [[wikilink]] into a note path (and to distinguish unique /
// ambiguous / unresolved). Defined here, in the consumer, as the subset of
// the producer's surface nav actually uses — the same shape as
// render.Resolver; internal/graph's concrete *Index satisfies it with no
// explicit binding.
type Resolver interface {
	Resolve(name string) graph.Resolution
}

// Closure records why a projection carries nothing. It is a reason, not a
// message: a projection can be withheld while a broader fault owns the only
// sentence worth printing, and a projection that is simply empty must stay
// distinguishable from one that was withheld. The zero Closure is open — the
// contents are the vault's true answer.
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

	// folders are the top-level lifecycle folders in the vault's own
	// artifact-lifecycle order (see lifecycleOrder), each holding its notes
	// and subfolders recursively.
	folders []Folder
	// rootNotes are files that live at the vault root itself (README.md and
	// its siblings), which belong to no top-level folder.
	rootNotes []NoteRef
	// paths are study paths in vault path order, read once through the
	// declared-sequence grammar (internal/sequence) at snapshot build.
	paths []Path
	// maps are every other map-note tree, ordered by domain and then title.
	maps []Map
	// journal is the most recent captured journal entries, newest first. It comes
	// from the scanner's file listing and mtime capture, independent of note type.
	journal []JournalEntry
	// reports enumerates System/reports/ — the .md reports first, then the
	// daily-briefing/ HTML briefings; contents are never parsed.
	reports []Report
	// knowledgeNotes are typed markdown notes in vault path order. Home sorts a
	// copy by Modified for its recent-changes block; the time is the scanner's
	// captured value, so rendering never stats a note. This projection is
	// withheld when the artifact declaration could not be honoured, and built
	// over every readable note when none was ever made.
	knowledgeNotes []NoteSummary

	// placementIndex maps a note's rel-path to every map placement that lists it,
	// built once so the sidebar can open each containing branch without re-walking
	// every map per request. Read it through Placements.
	placementIndex map[string][]Placement
	// dirNotes maps a directory's rel-path to the files directly inside it, built
	// once so the sidebar can show a note's same-directory siblings without
	// descending the folder tree per request. Read it through Siblings.
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

// NavigationDiagnostic explains why contract-derived paths and maps could not
// be built. It is empty when navigation roles were read cleanly, empty when
// nothing declared them, and empty when a broader fault owns the sentence — ask
// NavigationClosure whether the projection was withheld.
func (m *Model) NavigationDiagnostic() string {
	return m.NavigationClosure().Diagnostic()
}

// ArtifactDiagnostic explains why instance projections could not be built,
// under the same rule as NavigationDiagnostic.
func (m *Model) ArtifactDiagnostic() string {
	return m.ArtifactClosure().Diagnostic()
}

// InstanceProjectionsClosed reports whether the artifact-dependent projections
// were withheld rather than genuinely empty. Without it a vault whose
// exclusions could not be read is indistinguishable from a vault that has no
// study paths, which is the conflation this model exists to keep apart.
func (m *Model) InstanceProjectionsClosed() bool {
	return m.ArtifactClosure().Closed()
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

// Paths returns the study paths in vault path order. Everything the caller
// receives is its own: the model is read by every request at once, so a caller
// that could reach into it would change what the next reader sees.
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

// WithoutInstanceProjections returns a request-local view that keeps ordinary
// browsing surfaces but closes every artifact-policy-dependent projection. The
// closure travels with the model so a later reader can tell a withheld
// projection from an empty one.
func (m *Model) WithoutInstanceProjections(closure Closure) *Model {
	if m == nil {
		return &Model{artifact: closure}
	}
	degraded := *m
	degraded.artifact = closure
	degraded.paths = nil
	degraded.maps = nil
	degraded.knowledgeNotes = nil
	degraded.placementIndex = nil
	return &degraded
}

// NoteSummary is the navigation metadata Home needs for one knowledge note.
// Modified is captured by the snapshot scanner before the model build; it is
// zero only when that scan could not stat a note which remained readable.
type NoteSummary struct {
	Title    string
	RelPath  string
	Type     string
	Status   string
	Modified time.Time
}

// JournalEntry is one recent Diary markdown file. Modified is the scanner's
// captured value, not a timestamp read while rendering a request.
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

// lifecycleOrder is the vault's documented artifact-lifecycle order for the
// top-level folders: the main flow Inbox -> Sources -> Concepts -> Maps /
// Synthesis -> Writing, then System = governance, Views = dashboards,
// Diagrams = diagrams. It is an architectural ordering, not a schema enum —
// the toml has no display-order field (its [scan] knowledge_dirs is a
// checker scan policy in a different order and omitting
// System/Views/Diagrams), so there is nothing to read it from: this list is
// not a forbidden second copy of a vault-schema.toml enum. A top-level
// folder not in this list (a future Diary/, the current Drafts/) sorts
// after all known ones, in lexical order, so it stays reachable rather than
// silently vanishing from the sidebar.
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

// New constructs a navigation model from one captured vault projection.
// entries supply the canonical paths and observed modification times; notes
// contains the successfully parsed Markdown notes keyed by canonical path.
// A missing note is treated as an unreadable note and does not affect its
// neighbors. New neither enumerates nor reopens the vault.
func New(
	entries []vault.Entry,
	notes map[string]*vault.Note,
	resolver Resolver,
	roles schema.NavigationRoles,
	scope schema.KnowledgeScope,
	policy schema.ArtifactPolicy,
) *Model {
	if resolver == nil {
		panic("nav: New requires a non-nil Resolver")
	}

	observed := slices.Clone(entries)
	slices.SortFunc(observed, func(a, b vault.Entry) int {
		return comparePathsForReading(a.Path(), b.Path())
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
	resolver Resolver,
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
	if m.artifact.Closed() {
		return m
	}

	statusByPath, mapNotes, knowledgeNotes := collectNavigationNotes(files, roles, scope, policy)
	m.knowledgeNotes = knowledgeNotes
	if m.navigation.Closed() {
		return m
	}

	for _, n := range mapNotes {
		if roles.IsPathType(n.Type()) {
			// A study path reads the declared-sequence grammar; the legacy
			// map parser never sees it.
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
	// Study paths are indexed before general maps, so a note both a course and
	// a map place answers with its course first — the reader's own question is
	// about the course they are walking.
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
		if !strings.HasSuffix(p, ".md") || policy.IsNonInstance(p) {
			continue
		}
		n := file.note
		if n == nil {
			continue
		}
		if status := n.Status(); status != "" {
			statusByPath[p] = status
		}
		// What belongs to the knowledge layer is the vault's own declaration,
		// not whether a note happens to carry a type. A note without
		// frontmatter is still something its author wrote and still the newest
		// thing they changed; filtering on the type field hid exactly those,
		// and on a folder that declares nothing it hid everything.
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

// The journal and report projections select by location alone. The same two
// prefixes also answer "is the reader in the journal / among the reports right
// now" for the sidebar's drawers, so they are named once here and exposed as
// predicates — a second copy of either string would let the drawer that opens
// for a place and the projection that lists it drift apart.
const (
	journalPrefix = "Diary/"
	reportsPrefix = "System/reports/"
)

// InJournal reports whether relPath lives in the journal — the same location
// signal the journal projection selects its entries by.
func InJournal(relPath string) bool { return strings.HasPrefix(relPath, journalPrefix) }

// InReports reports whether relPath lives among the reports — the same
// location signal the report projection selects its entries by.
func InReports(relPath string) bool { return strings.HasPrefix(relPath, reportsPrefix) }

// buildJournal selects markdown files below Diary from the scanner's path and
// mtime captures. It does not parse frontmatter, so an untyped entry remains
// eligible. Equal timestamps fall back to path order for stable rebuilds.
func buildJournal(paths []string, mtimes map[string]time.Time) []JournalEntry {
	const limit = 5
	entries := make([]JournalEntry, 0, limit)
	for _, p := range paths {
		if !InJournal(p) || !strings.HasSuffix(p, ".md") {
			continue
		}
		_, base := splitDir(p)
		entries = append(entries, JournalEntry{Title: displayName(base), RelPath: p, Modified: mtimes[p]})
	}
	// Newest last by the entries' own names, then reversed — the same reading
	// order every other projection here uses, so a journal drawer and the
	// folder tree beside it cannot disagree about which entry follows which.
	// It was the file times, which is a different question from the one a
	// journal answers: a clone stamps every entry with one moment, and an
	// entry edited today is not today's entry.
	slices.SortStableFunc(entries, func(a, b JournalEntry) int {
		return comparePathsForReading(b.RelPath, a.RelPath)
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
			if strings.HasSuffix(rest, ".md") {
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

// buildFolderTree turns a flat, lexically ordered path list into
// the top-level folder tree plus the vault-root notes. It mirrors the real
// directory structure exactly, to whatever depth the vault actually has
// (Concepts is one domain level deep, Writing/lessons/<domain> is two) —
// it never invents levels the vault does not have, and never caps them, so
// every file stays a single click away in the rendered tree, keeping any
// vault file reachable in at most three clicks. Only the top level is reordered
// into lifecycle order; deeper folders keep the captured path order.
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
// missing segment in the input's lexical order, and returns the deepest
// folder. dir == "" returns root itself.
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

// Label names a file the way every list in this navigation names it, taking
// the vault-relative path rather than the bare filename. A projection built
// outside this package labels its rows through here, so one note cannot appear
// under two different names depending on which list the reader is looking at.
func Label(relPath string) string {
	_, base := splitDir(relPath)
	return displayName(base)
}
