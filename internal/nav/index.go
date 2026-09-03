package nav

import (
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// Placement records one appearance of a note as a map entry: the map that lists
// it and the chain of branch headings down to the entry's own branch. A note
// several branches list yields several placements rather than one chosen one.
type Placement struct {
	// MapRelPath identifies the containing map by its own note path.
	MapRelPath string
	// Headings is the branch-heading chain from the map root down to the
	// entry's own branch, outermost first.
	Headings []string
}

// Placements returns every map placement that lists the note at relPath, or nil
// when no map references it. relPath is a vault-relative NFC path, the form the
// rest of the model uses.
func (m *Model) Placements(relPath string) []Placement {
	if m == nil {
		return nil
	}
	placements := slices.Clone(m.placementIndex[relPath])
	for i := range placements {
		placements[i].Headings = slices.Clone(placements[i].Headings)
	}
	return placements
}

// IsPath reports whether relPath names one of the study paths. Asking by name
// costs no copy of the tree.
func (m *Model) IsPath(relPath string) bool {
	return m.indexOfPath(relPath) >= 0
}

// IsMap reports whether relPath names one of the general maps.
func (m *Model) IsMap(relPath string) bool {
	if m == nil {
		return false
	}
	return slices.IndexFunc(m.maps, func(x Map) bool { return x.RelPath == relPath }) >= 0
}

// Path returns the study path at relPath, or nil when no course in this
// generation answers to that name. What comes back is the caller's own, one
// course copied rather than all of them.
func (m *Model) Path(relPath string) *Path {
	at := m.indexOfPath(relPath)
	if at < 0 {
		return nil
	}
	cloned := m.paths[at].clone()
	return &cloned
}

// indexOfPath is where relPath sits among the study paths, or -1.
func (m *Model) indexOfPath(relPath string) int {
	if m == nil {
		return -1
	}
	return slices.IndexFunc(m.paths, func(p Path) bool { return p.RelPath == relPath })
}

// Siblings returns the files sharing a directory with the note at relPath,
// together with that directory's vault-relative path (empty for a vault-root
// note). relPath is itself in the list, for the caller to mark, in the captured
// reading order; the slice is nil when the directory holds nothing.
func (m *Model) Siblings(relPath string) (dir string, notes []NoteRef) {
	dir, _ = splitDir(relPath)
	if m == nil {
		return dir, nil
	}
	return dir, slices.Clone(m.dirNotes[dir])
}

// buildDirNotes groups every listed file by its directory, keeping the captured
// reading order so the grouping matches the folder tree.
func buildDirNotes(paths []string) map[string][]NoteRef {
	byDir := make(map[string][]NoteRef)
	for _, p := range paths {
		dir, base := splitDir(p)
		byDir[dir] = append(byDir[dir], NoteRef{Name: displayName(base), RelPath: p})
	}
	return byDir
}

// buildPlacementIndex inverts the map trees into a note-path -> placements map.
// Only resolved entries with a path enter it: a warning row places nothing.
func buildPlacementIndex(index map[string][]Placement, maps []Map) map[string][]Placement {
	for i := range maps {
		relPath := maps[i].RelPath
		var walk func(branches []Branch, chain []string)
		walk = func(branches []Branch, chain []string) {
			for i := range branches {
				branch := &branches[i]
				// Concat returns a fresh slice, so sibling branches never
				// overwrite each other's heading chain.
				here := slices.Concat(chain, []string{branch.Heading})
				for _, entry := range branch.Entries {
					if entry.Kind != EntryResolved || entry.RelPath == "" {
						continue
					}
					index[entry.RelPath] = append(index[entry.RelPath], Placement{
						MapRelPath: relPath,
						Headings:   slices.Clone(here),
					})
				}
				walk(branch.Subbranches, here)
			}
		}
		walk(maps[i].Branches, nil)
	}
	return index
}

// Directory returns what a folder holds directly: its files in the captured
// order, and the folders immediately inside it. ok is false for a path no
// folder in this generation answers to, so a caller can tell an empty folder
// from one that is not there.
func (m *Model) Directory(dir string) (notes, subfolders []NoteRef, ok bool) {
	if m == nil {
		return nil, nil, false
	}
	notes, listed := m.dirNotes[dir]
	prefix := dir + "/"
	seen := make(map[string]bool)
	for path := range m.dirNotes {
		if path == dir || !strings.HasPrefix(path, prefix) {
			continue
		}
		child := path[len(prefix):]
		if i := strings.IndexByte(child, '/'); i >= 0 {
			child = child[:i]
		}
		if child == "" || seen[child] {
			continue
		}
		seen[child] = true
		subfolders = append(subfolders, NoteRef{Name: child, RelPath: prefix + child})
	}
	slices.SortFunc(subfolders, func(a, b NoteRef) int { return vault.ComparePaths(a.Name, b.Name) })
	return slices.Clone(notes), subfolders, listed || len(subfolders) > 0
}

// FolderStep returns the neighbors on either side of relPath inside its own
// folder, in the folder's captured order — the folder's answer to what is near
// this note, where PathNeighbors gives a course's.
//
// What the line holds depends on what is being read. From a note the step
// passes over assets, because an image stored between two entries belongs to
// the folder rather than to the rhythm of reading it; from a file that is not a
// note the walk is the whole folder, a run of scans being its own line.
func (m *Model) FolderStep(relPath string) (prev, next NoteRef) {
	if m == nil || relPath == "" {
		return NoteRef{}, NoteRef{}
	}
	dir, _ := splitDir(relPath)
	siblings := m.dirNotes[dir]
	at := slices.IndexFunc(siblings, func(n NoteRef) bool { return n.RelPath == relPath })
	if at < 0 {
		return NoteRef{}, NoteRef{}
	}
	notesOnly := vault.IsMarkdown(relPath)
	steppable := func(n NoteRef) bool {
		return !notesOnly || vault.IsMarkdown(n.RelPath)
	}
	for i := at - 1; i >= 0; i-- {
		if steppable(siblings[i]) {
			prev = siblings[i]
			break
		}
	}
	for i := at + 1; i < len(siblings); i++ {
		if steppable(siblings[i]) {
			next = siblings[i]
			break
		}
	}
	return prev, next
}
