package nav

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
)

// Placement records one appearance of a note as a syllabus lesson: the
// study-path that lists it, and the chain of section headings from that
// study-path's root down to the section the lesson sits under. A note listed by
// several sections yields several placements, so the sidebar can open every
// containing path and mark each occurrence instead of choosing one.
type Placement struct {
	// SyllabusRelPath identifies the containing study-path by its own note path.
	SyllabusRelPath string
	// Headings is the section-heading chain from the study-path root down to the
	// lesson's own section, outermost first.
	Headings []string
}

// Placements returns every syllabus placement that lists the note at relPath, or
// nil when no study-path references it. relPath is a vault-relative NFC path, the
// same form the rest of the model uses, so no re-normalization is needed.
func (m *Model) Placements(relPath string) []Placement {
	return m.lessonIndex[relPath]
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

// buildLessonIndex inverts the syllabus trees into a note-path -> placements map,
// so a note page can find the study-path sections that reference it in one lookup
// instead of re-walking every study-path. Only a uniquely-resolved lesson has a
// path; an ambiguous or unresolved lesson has none and is skipped here (it still
// shows in the tree, carrying its diagnostic mark).
func buildLessonIndex(syllabi []Syllabus) map[string][]Placement {
	index := make(map[string][]Placement)
	for i := range syllabi {
		relPath := syllabi[i].RelPath
		var walk func(sections []Section, chain []string)
		walk = func(sections []Section, chain []string) {
			for _, sec := range sections {
				// Concat always returns a fresh slice, so sibling sections never
				// overwrite each other's heading chain and the stored value is
				// never mutated afterward.
				here := slices.Concat(chain, []string{sec.Heading})
				for _, l := range sec.Lessons {
					if l.Resolution == graph.Unique && l.RelPath != "" {
						index[l.RelPath] = append(index[l.RelPath], Placement{
							SyllabusRelPath: relPath,
							Headings:        here,
						})
					}
				}
				walk(sec.Sub, here)
			}
		}
		walk(syllabi[i].Sections, nil)
	}
	return index
}
