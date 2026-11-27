package snapshot

import (
	"cmp"
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// Backlinks answers "which notes cite this one" for one generation, inverting
// what the resolver knows from the extractor the adjudicator reads bodies with.
type Backlinks struct {
	byTarget map[string][]nav.NoteRef
}

// newBacklinks inverts one generation's citations. A citation counts only when
// it resolves to exactly one other note, so an ambiguous name records nothing.
// links is what each note cites, by path, as it was read when the note was.
func newBacklinks(notes []*vault.Note, links map[string][]string, idx *graph.Index, titles titleFunc) *Backlinks {
	b := &Backlinks{byTarget: make(map[string][]nav.NoteRef)}
	for _, n := range notes {
		seen := make(map[string]bool)
		for _, target := range links[n.RelPath] {
			res := idx.Resolve(target)
			if res.Kind != graph.KindUnique || res.RelPath == n.RelPath || seen[res.RelPath] {
				continue
			}
			seen[res.RelPath] = true
			b.byTarget[res.RelPath] = append(b.byTarget[res.RelPath], nav.NoteRef{
				Name:    titles(n.RelPath),
				RelPath: n.RelPath,
			})
		}
	}
	for path := range b.byTarget {
		slices.SortFunc(b.byTarget[path], func(a, c nav.NoteRef) int {
			return cmp.Or(vault.ComparePaths(a.Name, c.Name), vault.ComparePaths(a.RelPath, c.RelPath))
		})
	}
	return b
}

// To returns the notes citing relPath, sorted by the name each shows, and nil
// when nothing cites it, which is an answer rather than a gap.
func (b *Backlinks) To(relPath string) []nav.NoteRef {
	return slices.Clone(b.byTarget[relPath])
}

// Citing reports how many notes cite relPath.
func (b *Backlinks) Citing(relPath string) int {
	return len(b.byTarget[relPath])
}
