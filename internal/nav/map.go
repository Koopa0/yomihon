package nav

import (
	"fmt"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Map is one parsed map-typed note and its heading/entry tree in document
// order. Type distinguishes study paths from the Maps group; Domain supplies
// the Maps group's primary ordering key.
type Map struct {
	Title    string
	RelPath  string
	Domain   string
	Type     string
	Branches []Branch
}

// Branch is one heading in a map (an H2 part, an H3 module or level, or
// any deeper heading), holding the entries listed directly beneath it and
// its nested subbranches. A Branch is present only if it, or a descendant,
// carries at least one entry (see pruneBranches) — so a map's prose,
// daily-loop, and gap branches, which list no entries, never appear.
type Branch struct {
	// Heading is the display label: for a pipe-format heading
	// "slug | English | Chinese" it is the English column; otherwise the
	// whole heading text (e.g. a plain-text level name).
	Heading string
	// Level is the markdown heading level (2 for a part, 3 for a module),
	// kept so a renderer can indent by depth without re-deriving it.
	Level       int
	Entries     []Entry
	Subbranches []Branch
}

// EntryKind distinguishes a linked entry from each warning-row reason.
type EntryKind uint8

const (
	// EntryResolved is a unique wikilink target that can be navigated to.
	EntryResolved EntryKind = iota
	// EntryUnresolved has no matching vault target.
	EntryUnresolved
	// EntryAmbiguous has several candidates and deliberately names none of them.
	EntryAmbiguous
	// EntryNonInstance resolves uniquely to a readable artifact that is outside
	// the governed instance set.
	EntryNonInstance
)

// Entry is one wikilink list-item. Study paths retain warning rows for honest
// sequencing; general maps contain only governed, resolved entries.
type Entry struct {
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
// already-frontmatter-stripped body (vault.Parse split it out), so a
// frontmatter list value cannot look like an entry bullet. Study paths no
// longer pass through here — they read the declared-sequence grammar — so this
// parser keeps only the general-map behavior: uniquely resolved governed rows.
func parseMap(
	n *vault.Note,
	idx Resolver,
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
	entries []Entry
	sub     []*branchNode
}

// parseBranches is the one mechanical rule that produces the correct tree
// for both real map shapes without hardcoding filenames or branch
// titles:
//
//   - Walk the body line by line. A heading at level >= 2 opens a branch
//     nested under the nearest shallower open heading (a stack); an H1 (the
//     document title) is ignored, mirroring the reading page's leading-H1
//     removal.
//   - An entry list-item attaches to the currently open heading. An entry
//     list-item is an unordered bullet ("- ", "* ", "+ "), not a GFM task
//     checkbox ("- [ ]" / "- [x]"), that contains at least one [[wikilink]].
//     Its link is that first wikilink, resolved by graph semantics.
//   - Finally, prune every heading that has no entry anywhere beneath it.
//
// That predicate is what distinguishes the two files' non-navigation
// branches without naming them: the Go map's parts/modules hold plain
// "- [[Entry]]" bullets (all kept); the 大家 map's warm-up part holds direct
// "- **P01** ... [[P01 ...]]" entries and its course-sequence levels hold
// "- **L1** ... · [[L01 ...]]" entries (both kept), while its daily-loop
// branch uses an ordered list (excluded — not a bullet), its learning-level
// branch is a table (no list items), and its gaps branch uses task checkboxes
// (excluded — even the one carrying a [[wikilink]]), so all three prune away
// for having no entries. A "待建" bullet with no wikilink is not counted
// because it names no target. General maps keep only uniquely resolved governed
// rows; study paths also keep unresolved, ambiguous, and uniquely resolved
// non-instance targets as warnings in their original position.
func parseBranches(
	body string,
	idx Resolver,
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

// pruneBranches drops every node with no entries and no surviving
// descendant with entries — the step that removes a map's prose,
// loop, and gap headings without ever naming them.
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

// convertBranches turns the mutable node tree into the read-only Branch
// tree, preserving document order at every level.
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

// parseHeading reports an ATX heading of level >= 2: the "#" run must start
// the line, be at least two long, and be followed by a space. Level 0/1
// (body text, or the document-title H1) is not a branch.
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

// headingLabel surfaces the display label for a heading: the English column
// of a pipe-format "slug | English | Chinese" heading, or the whole trimmed
// text for any heading that is not pipe-format.
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
// inner text of its first wikilink. It requires an unordered bullet marker,
// rejects GFM task checkboxes, and requires a [[wikilink]] to be present —
// see parseBranches's doc for why each condition matters.
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

// isTaskMarker reports whether s (a bullet item's text, after the marker)
// begins with a GFM task checkbox: "[ ]", "[x]", or "[X]" followed by end
// of string or a space. A "[[" wikilink opener is not a checkbox (its
// second byte is "[", not a space or x), so "- [[Entry]]" is never
// mistaken for a task item.
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

// makeEntry resolves a wikilink's inner text into an Entry. It reuses
// graph.SplitWikilink (the same target/display extraction the renderer
// uses) and idx.Resolve (the same normalization and ambiguity rules applied
// to in-body wikilinks), so a sidebar entry link and the in-body wikilink
// to the same note agree exactly. ok is false when the link has no note target
// (a same-file anchor such as [[#heading]]). Unresolved, ambiguous, and unique
// non-instance targets return distinct warning kinds so the caller can preserve
// or omit them according to map role.
func makeEntry(inner string, idx Resolver, statusByPath map[string]string, policy schema.ArtifactPolicy) (Entry, bool) {
	target, display, ok := graph.SplitWikilink(inner)
	if !ok {
		return Entry{}, false
	}
	res := idx.Resolve(target)
	switch res.Kind {
	case graph.Unique:
		if policy.IsNonInstance(res.Path) {
			return Entry{Text: display, Target: target, Kind: EntryNonInstance}, true
		}
		return Entry{Text: display, Target: target, RelPath: res.Path, Status: statusByPath[res.Path], Kind: EntryResolved}, true
	case graph.Ambiguous:
		return Entry{Text: display, Target: target, Kind: EntryAmbiguous, Candidates: slices.Clone(res.Candidates)}, true
	case graph.Unresolved:
		return Entry{Text: display, Target: target, Kind: EntryUnresolved}, true
	default:
		// The resolver's kind set is closed — unresolved, unique, ambiguous —
		// so a value outside it is a programming error in the resolver, not a
		// state a vault can produce. Panicking is the deliberate response:
		// dropping the row would quietly break the promise that a map loses
		// no entry, and misfiling it would present a guess as a fact. A new
		// kind has to be met here by name.
		panic(fmt.Sprintf("nav: unknown graph.Kind %d", res.Kind))
	}
}
