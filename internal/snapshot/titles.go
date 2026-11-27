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
	if g == nil {
		return nil
	}
	held := g.titledBy[graph.NormalizeKey(name)]
	names := make([]string, len(held))
	for i, ref := range held {
		names[i] = ref.Name
	}
	return names
}
