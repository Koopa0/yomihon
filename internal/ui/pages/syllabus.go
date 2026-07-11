package pages

import (
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
)

// PathView is everything the study-path page needs: the current path's
// tree flattened into render-ready branches (with per-branch ready/total
// tallies and anchors precomputed), the switcher across every study-path in
// the vault, and the path-level figures the header and progress bar read.
//
// The seal target comes from schema. nav has already classified every entry as
// resolved, unresolved, or ambiguous, so the page never resolves a wikilink.
type PathView struct {
	Title      string
	RelPath    string
	SealTarget string
	Paths      []PathLink
	Branches   []PathBranchView

	// Path-level figures, precomputed so the header metarow and progress bar
	// are dumb reads. Entries is the whole-path entry total; Ready the subset
	// that has reached the seal.
	Parts   int
	Modules int
	Entries int
	Ready   int
}

// PathBranchView is one heading in the flattened tree. A top-level branch is a
// part (Depth 0): it carries an Anchor the "On this path" rail jumps to and an
// Ordinal (roman numeral). A nested branch is a module (Depth >= 1): it carries
// a Num (its 1-based position among its siblings). Ready/Total are the entry
// tallies for this branch and everything beneath it.
type PathBranchView struct {
	Anchor  string
	Ordinal string
	Num     int
	Heading string
	Depth   int
	Ready   int
	Total   int
	Entries []PathEntryView
	Sub     []PathBranchView
}

// PathEntryView is one linked or warning row. Only resolved rows have an href,
// status, or sealed state.
type PathEntryView struct {
	Text   string
	Href   string
	Status string
	Sealed bool
	Kind   nav.EntryKind
}

// PathLink is one entry in the path switcher: a study-path's title, the
// URL to its page, its whole-path entry count, and whether it is the path
// currently shown.
type PathLink struct {
	Title   string
	RelPath string
	Entries int
	Active  bool
}

// BuildPathView flattens one parsed study-path (current) into the page
// view, and builds the switcher from every study-path in the vault (all). It is
// pure: entry totals include linked and warning rows, Ready includes only
// resolved rows at the seal status, and document order is preserved at every
// level (nav already guarantees it).
func BuildPathView(current *nav.Map, all []nav.Map) PathView {
	v := PathView{
		Title:      current.Title,
		RelPath:    current.RelPath,
		SealTarget: schema.SealStatus,
		Paths:      buildPaths(current.RelPath, all),
	}
	for i, sec := range current.Branches {
		sv := buildPathBranch(sec, 0, i+1)
		v.Branches = append(v.Branches, sv)
		v.Parts++
		v.Modules += len(sv.Sub)
		v.Entries += sv.Total
		v.Ready += sv.Ready
	}
	return v
}

// buildPathBranch converts one nav.Branch (and its subtree) into a PathBranchView,
// tallying entries on the way up. depth 0 is a part (anchored, roman-numbered);
// deeper branches are modules (numbered by sibling position).
func buildPathBranch(sec nav.Branch, depth, num int) PathBranchView {
	sv := PathBranchView{Heading: sec.Heading, Depth: depth, Num: num}
	if depth == 0 {
		sv.Anchor = "part-" + strconv.Itoa(num)
		sv.Ordinal = roman(num)
	}
	for i := range sec.Entries {
		l := &sec.Entries[i]
		entry := buildPathEntry(l)
		sv.Entries = append(sv.Entries, entry)
		sv.Total++
		if entry.Sealed {
			sv.Ready++
		}
	}
	for i, sub := range sec.Sub {
		child := buildPathBranch(sub, depth+1, i+1)
		sv.Sub = append(sv.Sub, child)
		sv.Total += child.Total
		sv.Ready += child.Ready
	}
	return sv
}

// buildPathEntry maps one nav entry onto a linked or warning study-path row.
func buildPathEntry(entry *nav.Entry) PathEntryView {
	v := PathEntryView{Text: entry.Text, Kind: entry.Kind}
	if entry.Kind != nav.EntryResolved {
		return v
	}
	v.Href = notesHref(entry.RelPath)
	v.Status = entry.Status
	v.Sealed = entry.Status == schema.SealStatus
	return v
}

func entryResolutionLabel(kind nav.EntryKind) string {
	switch kind {
	case nav.EntryUnresolved:
		return "unresolved"
	case nav.EntryAmbiguous:
		return "ambiguous"
	case nav.EntryResolved:
		return "resolved"
	default:
		panic("pages: unknown nav.EntryKind: " + strconv.Itoa(int(kind)))
	}
}

func entryResolutionTitle(kind nav.EntryKind) string {
	switch kind {
	case nav.EntryUnresolved:
		return "Target not found"
	case nav.EntryAmbiguous:
		return "Target is ambiguous"
	case nav.EntryResolved:
		return ""
	default:
		panic("pages: unknown nav.EntryKind: " + strconv.Itoa(int(kind)))
	}
}

// buildPaths builds the switcher: every study-path in vault order, each with
// its whole-path entry count and whether it is the one currently shown.
func buildPaths(currentRel string, all []nav.Map) []PathLink {
	links := make([]PathLink, 0, len(all))
	for _, s := range all {
		links = append(links, PathLink{
			Title:   s.Title,
			RelPath: s.RelPath,
			Entries: entryTotal(s.Branches),
			Active:  s.RelPath == currentRel,
		})
	}
	return links
}

// entryTotal counts every linked or warning row in a branch slice, at any depth
// — the same count BuildPathView tallies, kept separate so the switcher can
// label a path it is not flattening.
func entryTotal(branches []nav.Branch) int {
	n := 0
	for _, sec := range branches {
		n += len(sec.Entries) + entryTotal(sec.Sub)
	}
	return n
}

// fillBucket is the progress bar's width, in whole percent rounded to the
// nearest 5, so a data-fill attribute selects one of a fixed set of CSS widths
// (no inline style, no JS). An empty path is 0.
func fillBucket(ready, total int) int {
	if total <= 0 {
		return 0
	}
	pct := ready * 100 / total
	return (pct + 2) / 5 * 5
}

// countLabel is a syllabus metarow figure for regular English nouns:
// "1 part", "5 modules", "20 lessons". Functional chrome stays pure English;
// bilingual text is reserved for ritual identity markers.
func countLabel(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// roman renders a positive part number as a roman numeral (the part label). A
// non-positive n falls back to its decimal form rather than panicking — a
// renderer must never abort on odd data.
func roman(n int) string {
	if n < 1 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}
