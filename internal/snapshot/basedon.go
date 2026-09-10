package snapshot

import (
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

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
