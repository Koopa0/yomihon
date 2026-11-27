package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// titleFunc names the note at a path by its title, and a path that holds none
// by its file name.
type titleFunc func(relPath string) string

// titlesIn names each note by the title its reading carries, so a build reads
// every note's title once and every projection asks it here.
func titlesIn(readings map[string]Reading) titleFunc {
	return func(relPath string) string {
		if reading, ok := readings[relPath]; ok {
			return reading.Title
		}
		return nav.Label(relPath)
	}
}

// titlesByName maps each title an author wrote to every note writing it. A
// title is deliberately not a name a link resolves by, so this is what lets a
// reader who cited one be told that, rather than that the note does not exist.
// A note that wrote none is left out: its file name already resolves.
func titlesByName(readings map[string]Reading) map[string][]nav.NoteRef {
	byTitle := make(map[string][]nav.NoteRef)
	for relPath := range readings {
		key := graph.NormalizeKey(readings[relPath].Title)
		if !readings[relPath].Titled || key == "" {
			continue
		}
		byTitle[key] = append(byTitle[key], nav.NoteRef{Name: nav.Label(relPath), RelPath: relPath})
	}
	for key := range byTitle {
		slices.SortFunc(byTitle[key], func(a, b nav.NoteRef) int {
			return vault.ComparePaths(a.RelPath, b.RelPath)
		})
	}
	return byTitle
}

// TitledBy names every note whose author wrote name as its title, in path
// order, and nothing when no note writes it. It answers with every holder,
// never a guess.
func (g *Generation) TitledBy(name string) []string {
	held := g.titledBy[graph.NormalizeKey(name)]
	names := make([]string, len(held))
	for i, ref := range held {
		names[i] = ref.Name
	}
	return names
}

// ReadmeRelPath is the folder's own introduction, whose title is what the
// folder is called to a reader. It is the one place the name is written.
const ReadmeRelPath = "README.md"

// VaultTitle is what the folder is called to a reader: the title its README
// wrote, and when there is none, folder, the name of the directory itself. A
// README the rules list under folders.skipped_files is a file here, not a note,
// and nothing of it is read, so such a folder is called by its directory.
func (g *Generation) VaultTitle(folder string) string {
	if title := g.DeclaredTitle(ReadmeRelPath); title != "" {
		return title
	}
	return folder
}

// DeclaredTitle is the title rel's author wrote, a first heading or else a
// frontmatter title, empty when it has neither. A reading's Title falls back to
// the file name, which is a path rather than a title.
func (g *Generation) DeclaredTitle(rel string) string {
	if !g.notes[rel].Titled {
		return ""
	}
	return g.notes[rel].Title
}
