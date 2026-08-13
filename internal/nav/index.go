package nav

import (
	"cmp"
	"slices"
	"strings"
)

// Placement records one appearance of a note as a map entry: the map that lists
// it and the chain of branch headings from that map's root down to the branch
// the entry sits under. A note listed by
// several branches yields several placements, so the sidebar can open every
// containing map and mark each occurrence instead of choosing one.
type Placement struct {
	// MapRelPath identifies the containing map by its own note path.
	MapRelPath string
	// Headings is the branch-heading chain from the map root down to the
	// entry's own branch, outermost first.
	Headings []string
}

// Placements returns every map placement that lists the note at relPath, or nil
// when no map references it. relPath is a vault-relative NFC path, the
// same form the rest of the model uses, so no re-normalization is needed.
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

// Siblings returns the files sharing a directory with the note at relPath — the
// "here" list the sidebar shows — together with that directory's vault-relative
// path (empty for a vault-root note). The note at relPath is itself in the list,
// for the caller to mark; the order matches the captured generation's lexical
// path order. The notes slice is nil when the directory holds nothing.
func (m *Model) Siblings(relPath string) (dir string, notes []NoteRef) {
	dir, _ = splitDir(relPath)
	if m == nil {
		return dir, nil
	}
	return dir, slices.Clone(m.dirNotes[dir])
}

// buildDirNotes groups every listed file by its directory, so the sidebar can
// show a note's same-directory siblings in one lookup rather than descending the
// folder tree per request. It mirrors the folder tree's contents and order:
// paths arrive lexically sorted, and each directory keeps that order.
func buildDirNotes(paths []string) map[string][]NoteRef {
	byDir := make(map[string][]NoteRef)
	for _, p := range paths {
		dir, base := splitDir(p)
		byDir[dir] = append(byDir[dir], NoteRef{Name: displayName(base), RelPath: p})
	}
	return byDir
}

// buildPlacementIndex inverts the map trees into a note-path -> placements map,
// so a note page can find every containing branch in one lookup instead of
// re-walking every map. Study paths may also contain warning rows, so only
// explicitly resolved entries with non-empty paths enter the reverse index.
func buildPlacementIndex(index map[string][]Placement, maps []Map) map[string][]Placement {
	for i := range maps {
		relPath := maps[i].RelPath
		var walk func(branches []Branch, chain []string)
		walk = func(branches []Branch, chain []string) {
			for i := range branches {
				branch := &branches[i]
				// Concat always returns a fresh slice, so sibling branches never
				// overwrite each other's heading chain and the stored value is
				// never mutated afterward.
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
// generation's order, and the folders immediately inside it. Both are what the
// browse tree already shows at that level, answered here for a page rather
// than a rail — a reader looking at one folder wants the whole of it at once,
// which a tree that fits in a sidebar cannot give them.
//
// ok is false for a path no folder in this generation answers to, so a caller
// can tell an empty folder from one that is not there.
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
	slices.SortFunc(subfolders, func(a, b NoteRef) int { return cmp.Compare(a.Name, b.Name) })
	return slices.Clone(notes), subfolders, listed || len(subfolders) > 0
}

// Adjacent returns the files on either side of relPath inside its own folder,
// in the folder's captured order. A folder of dated entries is a line, and a
// reader walking it wants the next one — which the rail can only offer as a
// scroll through everything the folder holds.
//
// It answers for any folder, because "the file after this one" is a question
// about a directory rather than about a kind of note: a course's lessons, a
// month of entries, and a book's chapters are all the same shape on disk.
func (m *Model) Adjacent(relPath string) (prev, next NoteRef) {
	if m == nil || relPath == "" {
		return NoteRef{}, NoteRef{}
	}
	dir, _ := splitDir(relPath)
	siblings := m.dirNotes[dir]
	at := slices.IndexFunc(siblings, func(n NoteRef) bool { return n.RelPath == relPath })
	if at < 0 {
		return NoteRef{}, NoteRef{}
	}
	if at > 0 {
		prev = siblings[at-1]
	}
	if at+1 < len(siblings) {
		next = siblings[at+1]
	}
	return prev, next
}
