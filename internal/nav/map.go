package nav

import (
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
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

// Branch is one heading in a map, holding the resolved wikilinks that sit
// directly beneath it — in a list item, in the heading, or in prose — and
// its nested subbranches. A Branch is present only where it or a descendant
// carries an entry, so a heading of pure prose never appears.
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

// MapEntry is one resolved wikilink in a general map's body. A map keeps only
// governed, resolved rows; a study path's PathEntry keeps its warning rows too,
// because their order is a curriculum. The shelf's branch count is how many
// headings this list keeps alive, so the rail and the shelf name the same tree.
type MapEntry struct {
	Text       string
	Target     string
	RelPath    string
	Status     string
	Language   string
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
	langsByPath map[string]string,
	policy schema.ArtifactPolicy,
) Map {
	return Map{
		Title:    n.Title(),
		RelPath:  n.RelPath,
		Domain:   n.Domain(),
		Type:     n.Type(),
		Branches: parseBranches(n.Body, idx, statusByPath, langsByPath, policy),
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
// the reading page's leading-H1 removal. Every live wikilink under the open
// heading becomes an entry: the same scan a study path already uses, so a
// link in a list item, a heading, prose, or a table counts, and a link inside
// a fence, a code span, an authored HTML block, or an Obsidian comment does
// not. Headings read those same skip zones, so a line skipped for a link is
// skipped for a heading. Pruning every heading with no entry beneath it
// leaves a map's pure-prose headings out without naming them; only resolved
// governed rows survive.
func parseBranches(
	body string,
	idx *graph.Index,
	statusByPath map[string]string,
	langsByPath map[string]string,
	policy schema.ArtifactPolicy,
) []Branch {
	var roots []*branchNode
	var stack []*branchNode
	links, zones := sequence.LiveScan(body)
	next := 0
	offset := 0

	for line := range strings.SplitSeq(body, "\n") {
		lineStart := offset
		lineEnd := offset + len(line)
		if text, level, ok := parseHeading(line); ok && !graph.In(zones, lineStart) {
			attachLiveLinks(stack, links, &next, lineStart, idx, statusByPath, langsByPath, policy)
			stack = openBranch(&roots, stack, headingLabel(text), level)
		}
		attachLiveLinks(stack, links, &next, lineEnd, idx, statusByPath, langsByPath, policy)
		offset = lineEnd + 1
	}
	attachLiveLinks(stack, links, &next, offset, idx, statusByPath, langsByPath, policy)
	return convertBranches(pruneBranches(roots))
}

// attachLiveLinks appends every still-unread live link that begins before
// until onto the open heading. A link before the first heading, or one that
// does not resolve uniquely to a governed note, is skipped.
func attachLiveLinks(
	stack []*branchNode,
	links []sequence.Link,
	next *int,
	until int,
	idx *graph.Index,
	statusByPath map[string]string,
	langsByPath map[string]string,
	policy schema.ArtifactPolicy,
) {
	for *next < len(links) && links[*next].Span.Start < until {
		link := links[*next]
		*next++
		if len(stack) == 0 {
			continue
		}
		entry := resolveEntry(link.Target, link.Display, idx, statusByPath, langsByPath, policy)
		if entry.Kind != EntryResolved {
			continue
		}
		top := stack[len(stack)-1]
		top.entries = append(top.entries, entry)
	}
}

// openBranch nests a heading under the nearest still-open shallower heading,
// or as a new root when none remains.
func openBranch(roots *[]*branchNode, stack []*branchNode, heading string, level int) []*branchNode {
	node := &branchNode{heading: heading, level: level}
	for len(stack) > 0 && stack[len(stack)-1].level >= level {
		stack = stack[:len(stack)-1]
	}
	if len(stack) == 0 {
		*roots = append(*roots, node)
	} else {
		top := stack[len(stack)-1]
		top.sub = append(top.sub, node)
	}
	return append(stack, node)
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

// resolveEntry classifies a live wikilink the map's scanner already admitted.
// Unresolved, ambiguous and non-instance targets get distinct warning kinds
// and are dropped by parseBranches; only a uniquely resolved governed row
// becomes an entry the rail can follow.
func resolveEntry(target, display string, idx *graph.Index, statusByPath map[string]string, langsByPath map[string]string, policy schema.ArtifactPolicy) MapEntry {
	res := idx.Resolve(target)
	entry := MapEntry{Text: display, Target: target, Kind: entryKindOf(res, policy)}
	if entry.Kind == EntryResolved {
		entry.RelPath = res.RelPath
		entry.Status = statusByPath[res.RelPath]
		entry.Language = langsByPath[res.RelPath]
	}
	if entry.Kind == EntryAmbiguous {
		entry.Candidates = slices.Clone(res.Candidates)
	}
	return entry
}
