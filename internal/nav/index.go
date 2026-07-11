package nav

import (
	"slices"
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
	return m.placementIndex[relPath]
}

// Siblings returns the files sharing a directory with the note at relPath — the
// "here" list the sidebar shows — together with that directory's vault-relative
// path (empty for a vault-root note). The note at relPath is itself in the list,
// for the caller to mark; the order matches the folder tree (vault.List's
// lexical order). The notes slice is nil when the directory holds nothing.
func (m *Model) Siblings(relPath string) (dir string, notes []NoteRef) {
	dir, _ = splitDir(relPath)
	return dir, m.dirNotes[dir]
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
func buildPlacementIndex(maps []Map) map[string][]Placement {
	index := make(map[string][]Placement)
	for i := range maps {
		relPath := maps[i].RelPath
		var walk func(branches []Branch, chain []string)
		walk = func(branches []Branch, chain []string) {
			for _, branch := range branches {
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
						Headings:   here,
					})
				}
				walk(branch.Sub, here)
			}
		}
		walk(maps[i].Branches, nil)
	}
	return index
}
