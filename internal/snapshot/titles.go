package snapshot

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// titlesByName maps each declared title to every note declaring it.
//
// A title is deliberately not a name a link resolves by — a citation written
// against one fails to find anything, which is what the vault's own reader
// does and what this program reproduces. That failure is exactly why this
// exists: the resolver cannot say what a citation was reaching for, and a
// reader who wrote a title deserves to be told that rather than that the note
// does not exist.
//
// Every holder is kept. Which of them to say out loud belongs to whoever is
// asking: a page listing all of them states a fact, while a page picking one
// would be guessing, and those are different pages' decisions to make.
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
			if a.RelPath == b.RelPath {
				return 0
			}
			if a.RelPath < b.RelPath {
				return -1
			}
			return 1
		})
	}
	return byTitle
}

// TitledBy names every note whose declared title is name, in path order, and
// returns nothing when no note declares it.
//
// It answers the question the resolver is built not to answer, and it answers
// with all the holders rather than one: naming a single note out of several
// would be a guess, and this generation does not make those on a caller's
// behalf. What to do about several is the caller's call — one of them shows a
// count and another shows the list — and both are served by the whole answer.
//
// The returned slice is the caller's own.
func (v *View) TitledBy(name string) []string {
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
