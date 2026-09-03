package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// titlesByName maps each declared title to every note declaring it. A title is
// deliberately not a name a link resolves by, so this is what lets a reader who
// cited one be told that, rather than that the note does not exist.
func titlesByName(notes []*vault.Note) map[string][]nav.NoteRef {
	byTitle := make(map[string][]nav.NoteRef)
	for _, n := range notes {
		if n == nil {
			continue
		}
		key := graph.NormalizeKey(n.Title())
		if key == "" {
			continue
		}
		byTitle[key] = append(byTitle[key], nav.NoteRef{Name: nav.Label(n.RelPath), RelPath: n.RelPath})
	}
	for key := range byTitle {
		slices.SortFunc(byTitle[key], func(a, b nav.NoteRef) int {
			return vault.ComparePaths(a.RelPath, b.RelPath)
		})
	}
	return byTitle
}

// TitledBy names every note whose declared title is name, in reading order, and
// nothing when no note declares it. It answers with every holder, never a guess.
func (v *Generation) TitledBy(name string) []string {
	if v == nil {
		return nil
	}
	held := v.titles[graph.NormalizeKey(name)]
	names := make([]string, len(held))
	for i, ref := range held {
		names[i] = ref.Name
	}
	return names
}
