package snapshot

import (
	"cmp"
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// Backlinks answers "which notes cite this one" for one generation. The
// resolver already knows where every citation points; this is that knowledge
// read in the other direction, which is the direction a reader wants it —
// standing on a note and asking who depends on it. Nothing new is parsed: the
// same extractor the adjudicator reads bodies with produces the edges, so a
// page listing what cites a note and a finding about that citation cannot
// disagree.
type Backlinks struct {
	byTarget map[string][]nav.NoteRef
}

// newBacklinks inverts one generation's citations. A citation counts when it
// resolves to exactly one note: an ambiguous name names no particular note, so
// recording it against a guess would be the resolver's one refusal undone here.
// A note citing itself is not a backlink — it tells the reader nothing they are
// not already looking at.
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
			if res.Kind != graph.Unique || res.RelPath == n.RelPath || seen[res.RelPath] {
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
// when nothing cites it. A note nothing cites is a real answer, not a missing
// one: it is how an island shows up.
func (b *Backlinks) To(relPath string) []nav.NoteRef {
	if b == nil {
		return nil
	}
	return slices.Clone(b.byTarget[relPath])
}

// Any reports whether anything in this generation cites anything at all. A
// folder whose notes never reference one another is not a folder with a link
// problem; it is a folder written in a form that does not use links, and a
// reading page must not keep answering a question nobody in it is asking.
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
