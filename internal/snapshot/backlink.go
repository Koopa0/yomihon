package snapshot

import (
	"cmp"
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
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
func newBacklinks(notes []*vault.Note, idx *graph.Index) *Backlinks {
	b := &Backlinks{byTarget: make(map[string][]nav.NoteRef)}
	if idx == nil {
		return b
	}
	for _, n := range notes {
		if n == nil {
			continue
		}
		seen := make(map[string]bool)
		for _, target := range judge.LinkTargets(n.Body) {
			res := idx.Resolve(target)
			if res.Kind != graph.KindUnique || res.RelPath == n.RelPath || seen[res.RelPath] {
				continue
			}
			seen[res.RelPath] = true
			b.byTarget[res.RelPath] = append(b.byTarget[res.RelPath], nav.NoteRef{
				Name:    nav.Label(n.RelPath),
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
	if b == nil {
		return nil
	}
	return slices.Clone(b.byTarget[relPath])
}

// Any reports whether anything in this generation cites anything at all — a
// folder written in a form that does not use links has no link problem.
func (b *Backlinks) Any() bool {
	return b != nil && len(b.byTarget) > 0
}

// Citing reports how many notes cite relPath.
func (b *Backlinks) Citing(relPath string) int {
	if b == nil {
		return 0
	}
	return len(b.byTarget[relPath])
}
