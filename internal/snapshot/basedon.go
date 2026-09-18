package snapshot

import (
	"cmp"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// basedOnBy answers the other direction of a based_on declaration for one
// generation: which notes named this one as the source they came from. The
// forward answer is read per request from the note's own frontmatter, and a
// reverse answer cannot be, so it is inverted once when the generation is
// built.
type basedOnBy struct {
	bySource map[string][]nav.NoteRef
}

// newBasedOnBy inverts every based_on declaration this generation captured,
// reading them through the same projection the forward answer uses, so the two
// directions can never disagree about which note a value named. Only a value
// that placed exactly one note is inverted: one left as the author's own text
// names nothing to hang a reverse answer on, and a note naming itself is not a
// pair.
func newBasedOnBy(notes []*vault.Note, idx *graph.Index) *basedOnBy {
	b := &basedOnBy{bySource: make(map[string][]nav.NoteRef)}
	for _, n := range notes {
		if n == nil {
			continue
		}
		for _, ref := range projectBasedOn(n, idx) {
			if ref.RelPath == "" || ref.RelPath == n.RelPath {
				continue
			}
			b.bySource[ref.RelPath] = append(b.bySource[ref.RelPath], nav.NoteRef{
				Name:    nav.Label(n.RelPath),
				RelPath: n.RelPath,
			})
		}
	}
	for source := range b.bySource {
		slices.SortFunc(b.bySource[source], func(a, c nav.NoteRef) int {
			return cmp.Or(vault.ComparePaths(a.Name, c.Name), vault.ComparePaths(a.RelPath, c.RelPath))
		})
	}
	return b
}

// of returns the notes declaring relPath as their source, sorted by the name
// each shows, and nil when none does. The caller receives its own copy.
func (b *basedOnBy) of(relPath string) []nav.NoteRef {
	if b == nil {
		return nil
	}
	return slices.Clone(b.bySource[relPath])
}

// projectBasedOn lists one note's based_on values as a reader can walk them.
// A unique resolution is a name and a path; anything else keeps the author's
// own text and no path. Empty members are dropped, and a value that repeats
// a path or a piece of text already listed is kept only the first time.
func projectBasedOn(n *vault.Note, idx *graph.Index) []nav.NoteRef {
	values := basedOnValues(n)
	if len(values) == 0 {
		return nil
	}
	out := make([]nav.NoteRef, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		ref, ok := declaredSource(idx, value)
		key := declaredSourceKey(ref)
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ref)
	}
	return out
}

// basedOnValues reads based_on the way the reading path is allowed to: the
// list form first, then a lone string. Strings does not take a scalar, and
// authors write both shapes.
func basedOnValues(n *vault.Note) []string {
	if n == nil {
		return nil
	}
	if values := n.Strings("based_on"); values != nil {
		return values
	}
	if s, ok := n.Text("based_on"); ok {
		return []string{s}
	}
	return nil
}

func declaredSource(idx *graph.Index, value string) (nav.NoteRef, bool) {
	written := strings.TrimSpace(value)
	if written == "" {
		return nav.NoteRef{}, false
	}
	target, ok := basedOnTarget(written)
	if !ok || idx == nil {
		return nav.NoteRef{Name: written}, true
	}
	res := idx.Resolve(target)
	if res.Kind != graph.KindUnique {
		return nav.NoteRef{Name: written}, true
	}
	// Unique rows use the target's nav.Label, not any |display text the author wrote.
	return nav.NoteRef{Name: nav.Label(res.RelPath), RelPath: res.RelPath}, true
}

// basedOnTarget strips a based_on value the way coverage strips one: optional
// [[…]] wrapping, then |display, #heading, and ^block. A value that strips to
// nothing is not a name.
func basedOnTarget(value string) (string, bool) {
	inner := value
	if rest, ok := strings.CutPrefix(value, "[["); ok {
		if stripped, ok := strings.CutSuffix(rest, "]]"); ok {
			inner = stripped
		}
	}
	link, ok := graph.ParseWikilink(inner)
	return link.Target, ok
}

func declaredSourceKey(ref nav.NoteRef) string {
	if ref.RelPath != "" {
		return ref.RelPath
	}
	return "\x00" + ref.Name
}
