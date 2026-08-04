package pages

import (
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
)

// PathView is everything the study-path page needs: the current path's
// tree flattened into render-ready branches (with per-branch entry totals
// and anchors precomputed), the switcher across every study-path in
// the vault, and the path-level figures the header metarow reads.
//
// The seal target comes from schema. nav has already classified every entry as
// resolved, unresolved, or ambiguous, so the page never resolves a wikilink.
type PathView struct {
	Title      string
	RelPath    string
	GuideHref  string
	SealTarget string
	Paths      []PathLink
	Branches   []PathBranchView

	// Path-level figures, precomputed so the header metarow is a dumb read.
	// Entries is the whole-path entry total; Ready the subset waiting at the
	// seal for a human to rule on it — a queue, never a measure of progress:
	// a lesson finished and published leaves it.
	Parts   int
	Modules int
	Entries int
	Ready   int
}

// PathBranchView is one heading in the flattened tree. A top-level branch is a
// part (Depth 0): it carries an Anchor the "On this path" rail jumps to and an
// Ordinal (roman numeral). A nested branch is a module (Depth >= 1): it carries
// a Num (its 1-based position among its siblings). Total is how many entries
// this branch and everything beneath it hold.
type PathBranchView struct {
	Anchor  string
	Ordinal string
	Num     int
	Heading string
	Depth   int
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
		GuideHref:  notesHref(current.RelPath),
		SealTarget: schema.SealStatus,
		Paths:      buildPaths(current.RelPath, all),
	}
	v.Ready, _ = current.EntryCounts(schema.SealStatus)
	for i, sec := range current.Branches {
		sv := buildPathBranch(sec, 0, i+1)
		v.Branches = append(v.Branches, sv)
		v.Parts++
		v.Modules += len(sv.Sub)
		v.Entries += sv.Total
	}
	return v
}

// buildPathBranch converts one nav.Branch and its subtree into a
// PathBranchView. depth 0 is a part (anchored, roman-numbered); deeper branches
// are modules (numbered by sibling position). nav owns the subtree counts;
// this function only maps them into presentation values.
func buildPathBranch(sec nav.Branch, depth, num int) PathBranchView {
	sv := PathBranchView{Heading: sec.Heading, Depth: depth, Num: num}
	_, sv.Total = sec.EntryCounts(schema.SealStatus)
	if depth == 0 {
		sv.Anchor = "part-" + strconv.Itoa(num)
		sv.Ordinal = roman(num)
	}
	entries := sec.Entries
	for i := range entries {
		l := &entries[i]
		entry := buildPathEntry(l)
		sv.Entries = append(sv.Entries, entry)
	}
	for i, sub := range sec.Subbranches {
		child := buildPathBranch(sub, depth+1, i+1)
		sv.Sub = append(sv.Sub, child)
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
		return "未解析"
	case nav.EntryAmbiguous:
		return "有歧義"
	case nav.EntryNonInstance:
		return "非治理項目"
	case nav.EntryResolved:
		return "已解析"
	default:
		panic("pages: unknown nav.EntryKind: " + strconv.Itoa(int(kind)))
	}
}

// entryResolutionCode is the stable machine token carried by data-resolution.
// It deliberately remains English while entryResolutionLabel owns the
// Traditional Chinese browser copy.
func entryResolutionCode(kind nav.EntryKind) string {
	switch kind {
	case nav.EntryUnresolved:
		return "unresolved"
	case nav.EntryAmbiguous:
		return "ambiguous"
	case nav.EntryNonInstance:
		return "non-instance"
	case nav.EntryResolved:
		return "resolved"
	default:
		panic("pages: unknown nav.EntryKind: " + strconv.Itoa(int(kind)))
	}
}

func entryResolutionTitle(kind nav.EntryKind) string {
	switch kind {
	case nav.EntryUnresolved:
		return "找不到目標"
	case nav.EntryAmbiguous:
		return "目標有歧義"
	case nav.EntryNonInstance:
		return "目標不屬於生命週期治理範圍"
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
		_, total := s.EntryCounts(schema.SealStatus)
		links = append(links, PathLink{
			Title:   s.Title,
			RelPath: s.RelPath,
			Entries: total,
			Active:  s.RelPath == currentRel,
		})
	}
	return links
}

// countLabel formats a syllabus metarow figure with its supplied Chinese unit.
func countLabel(n int, noun string) string {
	return strconv.Itoa(n) + " " + noun
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
