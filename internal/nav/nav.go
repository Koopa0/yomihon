// Package nav builds the vault's read-only navigation model. It turns the
// vault on disk into the structures used by the sidebar and Home:
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
//     Study paths retain unresolved rows as warnings because their order is a
//     curriculum; general maps omit them because their tree is link navigation.
//   - Contract-derived. Navigation roles and artifact boundaries are immutable
//     values derived by internal/schema at startup; nav never reads the contract.
package nav

import (
	"cmp"
	"fmt"
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

// Model is the whole navigation model, built
// once and read-only afterward.
type Model struct {
	// NavigationDiagnostic explains why contract-derived Paths and Maps are
	// unavailable. It is empty when navigation roles are valid.
	NavigationDiagnostic string
	// ArtifactDiagnostic explains why instance projections are unavailable. It
	// is empty when the artifact policy is valid.
	ArtifactDiagnostic string

	// Folders are the top-level lifecycle folders in the vault's own
	// artifact-lifecycle order (see lifecycleOrder), each holding its notes
	// and subfolders recursively.
	Folders []Folder
	// RootNotes are files that live at the vault root itself (README.md,
	// CLAUDE.md, ...), which belong to no top-level folder.
	RootNotes []NoteRef
	// Paths are study-path map trees in vault path order.
	Paths []Map
	// Maps are every other map-note tree, ordered by domain and then title.
	Maps []Map
	// Journal is the most recent Diary markdown entries, newest first. It comes
	// from the scanner's file listing and mtime capture, independent of note type.
	Journal []JournalEntry
	// Reports enumerates System/reports/ — the .md reports first, then the
	// daily-briefing/ HTML briefings; contents are never parsed.
	Reports []Report
	// KnowledgeNotes are typed markdown notes in vault path order. Home sorts a
	// copy by Modified for its recent-changes block; the time is the scanner's
	// captured value, so rendering never stats a note. This projection is empty
	// when the artifact policy is unavailable.
	KnowledgeNotes []NoteSummary

	// placementIndex maps a note's rel-path to every map placement that lists it,
	// built once so the sidebar can open each containing branch without re-walking
	// every map per request. Read it through Placements.
	placementIndex map[string][]Placement
	// dirNotes maps a directory's rel-path to the files directly inside it, built
	// once so the sidebar can show a note's same-directory siblings without
	// descending the folder tree per request. Read it through Siblings.
	dirNotes map[string][]NoteRef
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
	Name    string
	RelPath string
	Notes   []NoteRef
	Sub     []Folder
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

// Build assembles the navigation model for the vault rooted at root. idx
// resolves map-entry wikilinks and must not be nil (a wiring bug, treated the
// same as render.New's nil Resolver). The returned error is reserved for a
// failure to even list the vault; every finer-grained problem (an
// unreadable note, an unresolved map entry, a malformed map) is tolerated
// and surfaced in the model rather than returned. cmd/yomihon/main.go
// treats a returned error the same asymmetric way it treats a graph build
// failure: log and serve an empty model, never abort the server. mtimes is the
// scanner-owned path-to-mtime capture for this snapshot; Build copies values
// from it and never stats files itself.
func Build(
	root string,
	idx Resolver,
	mtimes map[string]time.Time,
	roles schema.NavigationRoles,
	policy schema.ArtifactPolicy,
) (*Model, error) {
	if idx == nil {
		panic("nav: Build requires a non-nil Resolver")
	}

	paths, err := vault.List(root)
	if err != nil {
		return nil, fmt.Errorf("nav: build: %w", err)
	}

	m := &Model{
		Reports: buildReports(paths),
		Journal: buildJournal(paths, mtimes),
	}
	m.Folders, m.RootNotes = buildFolderTree(paths)
	m.dirNotes = buildDirNotes(paths)
	if !roles.Available() {
		m.NavigationDiagnostic = roles.Diagnostic()
	}
	if !policy.Available() {
		m.ArtifactDiagnostic = policy.Diagnostic()
		return m, nil
	}

	statusByPath, mapNotes, knowledgeNotes := collectNavigationNotes(root, paths, mtimes, roles, policy)
	m.KnowledgeNotes = knowledgeNotes
	if !roles.Available() {
		return m, nil
	}

	for _, n := range mapNotes {
		isPath := roles.IsPathType(n.Type())
		tree := parseMap(n, idx, statusByPath, policy, isPath)
		if isPath {
			m.Paths = append(m.Paths, tree)
		} else {
			m.Maps = append(m.Maps, tree)
		}
	}
	slices.SortStableFunc(m.Maps, func(a, b Map) int {
		if byDomain := cmp.Compare(a.Domain, b.Domain); byDomain != 0 {
			return byDomain
		}
		if byTitle := cmp.Compare(a.Title, b.Title); byTitle != 0 {
			return byTitle
		}
		return cmp.Compare(a.RelPath, b.RelPath)
	})
	m.placementIndex = buildPlacementIndex(slices.Concat(m.Paths, m.Maps))
	return m, nil
}

// collectNavigationNotes performs the model build's one markdown read pass. It
// records statuses for entry badges, typed-note summaries for Home, and the map
// notes whose bodies Build parses next. An unreadable note is skipped without
// affecting its neighbors.
func collectNavigationNotes(
	root string,
	paths []string,
	mtimes map[string]time.Time,
	roles schema.NavigationRoles,
	policy schema.ArtifactPolicy,
) (map[string]string, []*vault.Note, []NoteSummary) {
	statusByPath := make(map[string]string)
	var mapNotes []*vault.Note
	var knowledgeNotes []NoteSummary
	for _, p := range paths {
		if !strings.HasSuffix(p, ".md") || policy.IsNonInstance(p) {
			continue
		}
		n, err := vault.ReadNote(root, p)
		if err != nil {
			continue
		}
		if status := n.Status(); status != "" {
			statusByPath[p] = status
		}
		if noteType := n.Type(); noteType != "" {
			knowledgeNotes = append(knowledgeNotes, NoteSummary{
				Title:    n.Title(),
				RelPath:  n.RelPath,
				Type:     noteType,
				Status:   n.Status(),
				Modified: mtimes[p],
			})
		}
		if roles.IsPathType(n.Type()) || roles.IsMapType(n.Type()) {
			mapNotes = append(mapNotes, n)
		}
	}
	return statusByPath, mapNotes, knowledgeNotes
}

// buildJournal selects markdown files below Diary from the scanner's path and
// mtime captures. It does not parse frontmatter, so an untyped entry remains
// eligible. Equal timestamps fall back to path order for stable rebuilds.
func buildJournal(paths []string, mtimes map[string]time.Time) []JournalEntry {
	const limit = 5
	entries := make([]JournalEntry, 0, limit)
	for _, p := range paths {
		if !strings.HasPrefix(p, "Diary/") || !strings.HasSuffix(p, ".md") {
			continue
		}
		_, base := splitDir(p)
		entries = append(entries, JournalEntry{Title: displayName(base), RelPath: p, Modified: mtimes[p]})
	}
	slices.SortStableFunc(entries, func(a, b JournalEntry) int {
		switch {
		case a.Modified.Equal(b.Modified):
			return cmp.Compare(a.RelPath, b.RelPath)
		case a.Modified.After(b.Modified):
			return -1
		default:
			return 1
		}
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
	const prefix = "System/reports/"
	const briefingDir = "daily-briefing"

	var reports, briefings []Report
	for _, p := range paths {
		rest, ok := strings.CutPrefix(p, prefix)
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

// buildFolderTree turns vault.List's flat, lexically-ordered path list into
// the top-level folder tree plus the vault-root notes. It mirrors the real
// directory structure exactly, to whatever depth the vault actually has
// (Concepts is one domain level deep, Writing/lessons/<domain> is two) —
// it never invents levels the vault does not have, and never caps them, so
// every file stays a single click away in the rendered tree, keeping any
// vault file reachable in at most three clicks. Only the top level is reordered
// into lifecycle order; deeper folders keep vault.List's lexical order.
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
// missing segment (in first-seen order, which is lexical because vault.List
// is), and returns the deepest folder. dir == "" returns root itself.
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
	f := Folder{Name: fb.name, RelPath: fb.relPath, Notes: fb.notes}
	if len(fb.subs) > 0 {
		f.Sub = make([]Folder, 0, len(fb.subs))
		for _, s := range fb.subs {
			f.Sub = append(f.Sub, s.toFolder())
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
