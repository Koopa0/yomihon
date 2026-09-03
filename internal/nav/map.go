package nav

import (
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Map is one parsed map-typed note and its heading/entry tree in document
// order. Domain is the Maps group's primary ordering key.
type Map struct {
	Title    string
	RelPath  string
	Domain   string
	Type     string
	Branches []Branch
}

// Branch is one heading in a map, holding the entries listed directly beneath
// it and its nested subbranches. A Branch is present only where it or a
// descendant carries an entry, so a heading of pure prose never appears.
type Branch struct {
	// Heading is the display label: the English column of a pipe-format
	// "slug | English | Chinese" heading, otherwise the whole heading text.
	Heading string
	// Level is the markdown heading level, kept so a renderer can indent by
	// depth without re-deriving it.
	Level       int
	Entries     []MapEntry
	Subbranches []Branch
}

// MapEntry is one wikilink list-item of a general map. A map keeps only
// governed, resolved rows; a study path's PathEntry keeps its warning rows too,
// because their order is a curriculum.
type MapEntry struct {
	Text       string
	Target     string
	RelPath    string
	Status     string
	Kind       EntryKind
	Candidates []string
}

func cloneMaps(source []Map) []Map {
	cloned := slices.Clone(source)
	for i := range cloned {
		cloned[i].Branches = cloneBranches(source[i].Branches)
	}
	return cloned
}

func cloneBranches(source []Branch) []Branch {
	cloned := slices.Clone(source)
	for i := range cloned {
		cloned[i].Entries = slices.Clone(source[i].Entries)
		for j := range cloned[i].Entries {
			cloned[i].Entries[j].Candidates = slices.Clone(source[i].Entries[j].Candidates)
		}
		cloned[i].Subbranches = cloneBranches(source[i].Subbranches)
	}
	return cloned
}

// parseMap parses one map-typed note into a Map. It reads the
// already-frontmatter-stripped body, so a frontmatter list value cannot look
// like an entry bullet. Study paths read the declared-sequence grammar instead.
func parseMap(
	n *vault.Note,
	idx *graph.Index,
	statusByPath map[string]string,
	policy schema.ArtifactPolicy,
) Map {
	return Map{
		Title:    n.Title(),
		RelPath:  n.RelPath,
		Domain:   n.Domain(),
		Type:     n.Type(),
		Branches: parseBranches(n.Body, idx, statusByPath, policy),
	}
}

// branchNode is the mutable tree node used while parsing; it becomes the
// read-only Branch value after pruning.
type branchNode struct {
	heading string
	level   int
	entries []MapEntry
	sub     []*branchNode
}

// parseBranches builds a map's tree from the body alone, naming no file and no
// heading. A heading at level >= 2 opens a branch nested under the nearest
// shallower open heading; an H1 is the document title and is ignored, mirroring
// the reading page's leading-H1 removal. An entry list-item attaches to the
// open heading: an unordered bullet ("- ", "* ", "+ ") that is not a GFM task
// checkbox and carries a [[wikilink]], resolved by graph semantics. Pruning
// every heading with no entry beneath it leaves a map's prose and checkbox
// branches out without naming them; only resolved governed rows survive.
func parseBranches(
	body string,
	idx *graph.Index,
	statusByPath map[string]string,
	policy schema.ArtifactPolicy,
) []Branch {
	var roots []*branchNode
	var stack []*branchNode

	for line := range strings.SplitSeq(body, "\n") {
		if text, level, ok := parseHeading(line); ok {
			node := &branchNode{heading: headingLabel(text), level: level}
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			if len(stack) == 0 {
				roots = append(roots, node)
			} else {
				top := stack[len(stack)-1]
				top.sub = append(top.sub, node)
			}
			stack = append(stack, node)
			continue
		}
		inner, ok := parseEntryItem(line)
		if !ok {
			continue
		}
		entry, ok := makeEntry(inner, idx, statusByPath, policy)
		if !ok || len(stack) == 0 {
			continue
		}
		if entry.Kind != EntryResolved {
			continue
		}
		top := stack[len(stack)-1]
		top.entries = append(top.entries, entry)
	}
	return convertBranches(pruneBranches(roots))
}

// pruneBranches drops every node with no entries and no surviving descendant
// with entries.
func pruneBranches(nodes []*branchNode) []*branchNode {
	kept := nodes[:0:0]
	for _, n := range nodes {
		n.sub = pruneBranches(n.sub)
		if len(n.entries) > 0 || len(n.sub) > 0 {
			kept = append(kept, n)
		}
	}
	return kept
}

// convertBranches turns the mutable node tree into the read-only Branch tree,
// preserving document order at every level.
func convertBranches(nodes []*branchNode) []Branch {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]Branch, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, Branch{
			Heading:     n.heading,
			Level:       n.level,
			Entries:     slices.Clone(n.entries),
			Subbranches: convertBranches(n.sub),
		})
	}
	return out
}

// parseHeading reports an ATX heading of level >= 2: the "#" run starts the
// line, runs at least twice, and is followed by a space.
func parseHeading(line string) (text string, level int, ok bool) {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n < 2 || n >= len(line) || line[n] != ' ' {
		return "", 0, false
	}
	return strings.TrimSpace(line[n+1:]), n, true
}

// headingLabel is a heading's display label: the English column of a
// pipe-format "slug | English | Chinese" heading, or the whole trimmed text.
func headingLabel(text string) string {
	if strings.Contains(text, "|") {
		if parts := strings.Split(text, "|"); len(parts) >= 2 {
			if en := strings.TrimSpace(parts[1]); en != "" {
				return en
			}
		}
	}
	return text
}

// parseEntryItem reports whether line is an entry list-item and returns the
// inner text of its first wikilink.
func parseEntryItem(line string) (inner string, ok bool) {
	t := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(t, "- "):
	case strings.HasPrefix(t, "* "):
	case strings.HasPrefix(t, "+ "):
	default:
		return "", false
	}
	rest := t[2:]
	if isTaskMarker(rest) {
		return "", false
	}
	return firstWikilink(rest)
}

// isTaskMarker reports whether s, a bullet item's text after its marker, begins
// with a GFM task checkbox. A "[[" wikilink opener is not one, so an entry
// bullet is never mistaken for a task item.
func isTaskMarker(s string) bool {
	if len(s) < 3 || s[0] != '[' || s[2] != ']' {
		return false
	}
	switch s[1] {
	case ' ', 'x', 'X':
	default:
		return false
	}
	return len(s) == 3 || s[3] == ' '
}

// firstWikilink returns the inner text of the first "[[...]]" in s.
func firstWikilink(s string) (inner string, ok bool) {
	_, rest, found := strings.Cut(s, "[[")
	if !found {
		return "", false
	}
	inner, _, found = strings.Cut(rest, "]]")
	if !found {
		return "", false
	}
	return inner, true
}

// makeEntry resolves a wikilink's inner text into an Entry through the same
// extraction and resolution an in-body wikilink gets, so the two agree exactly.
// ok is false for a link with no note target, such as a same-file anchor.
// Unresolved, ambiguous and non-instance targets get distinct warning kinds.
func makeEntry(inner string, idx *graph.Index, statusByPath map[string]string, policy schema.ArtifactPolicy) (MapEntry, bool) {
	target, display, ok := graph.SplitWikilink(inner)
	if !ok {
		return MapEntry{}, false
	}
	res := idx.Resolve(target)
	switch res.Kind {
	case graph.KindUnique:
		if policy.IsNonInstance(res.RelPath) {
			return MapEntry{Text: display, Target: target, Kind: EntryNonInstance}, true
		}
		return MapEntry{Text: display, Target: target, RelPath: res.RelPath, Status: statusByPath[res.RelPath], Kind: EntryResolved}, true
	case graph.KindAmbiguous:
		return MapEntry{Text: display, Target: target, Kind: EntryAmbiguous, Candidates: slices.Clone(res.Candidates)}, true
	case graph.KindUnresolved:
		return MapEntry{Text: display, Target: target, Kind: EntryUnresolved}, true
	default:
		// The resolver's kind set is closed, so a value outside it is a
		// programming error rather than a state a vault can produce: dropping
		// the row would break the promise that a map loses no entry.
		panic("nav: unknown graph.Kind: " + res.Kind.String())
	}
}
