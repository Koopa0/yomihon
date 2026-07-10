// Package nav builds the vault's navigation model: the reading page's
// sidebar. It has one responsibility — turn the vault on disk into the
// three read-only structures the sidebar shows:
//
//  1. a lifecycle folder tree (browse every note without typing a URL),
//  2. the parsed study-path syllabus trees (part -> module/stage -> lesson,
//     in document order — the rendered order must match the file's own
//     listing order), and
//  3. a flat list of the reports under System/reports/.
//
// The Model is built once at process startup and never mutated afterward
// (mirroring how graph.Index is built in cmd/yomihon/main.go), so every
// request hands out the same immutable value with no per-request work and
// no locking.
//
// Two invariants shape every decision here:
//
//   - Fault-tolerant. A malformed syllabus, a broken lesson link,
//     an unreadable note, an odd folder — none may panic or drop the rest.
//     A lesson whose wikilink does not resolve (or is ambiguous) is STILL
//     listed, carrying its resolution state, never silently dropped.
//   - DB-independent. nav reads only files (vault.List /
//     vault.ReadNote) and the wikilink index; it never touches PostgreSQL
//     or the schema contract, so the sidebar renders whether or not those
//     are available.
package nav

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
)

// typeStudyPath is the single frontmatter `type` value nav keys on to find
// syllabus notes. It is one member of the schema's `type` enum, referenced
// by its stable value — NOT a copy of the enum list or the state machine,
// which must never be duplicated outside vault-schema.toml. nav deliberately
// does not load the schema contract to check it: the reading/navigation face
// must keep working even when the contract cannot be read (reading is
// fail-open; only the write face is fail-closed), and
// the write face gets its distinguished seal target from the schema package.
const typeStudyPath = "study-path"

// Resolver is the minimal wikilink-resolution capability nav needs to turn
// a lesson's [[wikilink]] into a note path (and to distinguish unique /
// ambiguous / unresolved). Defined here, in the consumer, as the subset of
// the producer's surface nav actually uses — the same shape as
// render.Resolver; internal/graph's concrete *Index satisfies it with no
// explicit binding.
type Resolver interface {
	Resolve(name string) graph.Resolution
}

// Model is the whole navigation model: the sidebar's three sections, built
// once and read-only afterward.
type Model struct {
	// Folders are the top-level lifecycle folders in the vault's own
	// artifact-lifecycle order (see lifecycleOrder), each holding its notes
	// and subfolders recursively.
	Folders []Folder
	// RootNotes are files that live at the vault root itself (README.md,
	// CLAUDE.md, ...), which belong to no top-level folder.
	RootNotes []NoteRef
	// Syllabi are the parsed study-path trees, one per `type: study-path`
	// note, in vault.List (path) order.
	Syllabi []Syllabus
	// Reports enumerates System/reports/ — the .md reports first, then the
	// daily-briefing/ HTML briefings; contents are never parsed.
	Reports []Report

	// lessonIndex maps a note's rel-path to the syllabus placements that list it
	// — the reverse of Syllabi — built once so the sidebar can open to the
	// current note without re-walking every study-path per request. Read it
	// through Placements.
	lessonIndex map[string][]Placement
	// dirNotes maps a directory's rel-path to the files directly inside it, built
	// once so the sidebar can show a note's same-directory siblings without
	// descending the folder tree per request. Read it through Siblings.
	dirNotes map[string][]NoteRef
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
// resolves lesson wikilinks and must not be nil (a wiring bug, treated the
// same as render.New's nil Resolver). The returned error is reserved for a
// failure to even list the vault; every finer-grained problem (an
// unreadable note, a broken lesson link, a malformed syllabus) is tolerated
// and surfaced in the model rather than returned. cmd/yomihon/main.go
// treats a returned error the same asymmetric way it treats a graph build
// failure: log and serve an empty model, never abort the server.
func Build(root string, idx Resolver) (*Model, error) {
	if idx == nil {
		panic("nav: Build requires a non-nil Resolver")
	}

	paths, err := vault.List(root)
	if err != nil {
		return nil, fmt.Errorf("nav: build: %w", err)
	}

	m := &Model{Reports: buildReports(paths)}
	m.Folders, m.RootNotes = buildFolderTree(paths)
	m.dirNotes = buildDirNotes(paths)

	// One read pass over every markdown note: record each note's status
	// (for lesson badges) and collect the study-path notes to parse. An
	// unreadable note is skipped, not fatal.
	statusByPath := make(map[string]string)
	var studyPaths []*vault.Note
	for _, p := range paths {
		if !strings.HasSuffix(p, ".md") {
			continue
		}
		n, readErr := vault.ReadNote(root, p)
		if readErr != nil {
			continue // best-effort: a note that vanished or is unreadable drops out, the rest stand
		}
		if s := n.Status(); s != "" {
			statusByPath[p] = s
		}
		if n.Type() == typeStudyPath {
			studyPaths = append(studyPaths, n)
		}
	}

	for _, n := range studyPaths {
		m.Syllabi = append(m.Syllabi, parseSyllabus(n, idx, statusByPath))
	}
	m.lessonIndex = buildLessonIndex(m.Syllabi)
	return m, nil
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
