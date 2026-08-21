package pages

import (
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// PathView is everything the study-path page needs: the current path's
// tree flattened into render-ready branches (with per-branch entry totals
// and anchors precomputed), the switcher across every study-path in
// the vault, and the path-level figures the header metarow reads.
//
// SealTarget is the schema-pinned ready value the header's queue figure
// names. nav has already classified every entry as resolved, unresolved, or
// ambiguous, so the page never resolves a wikilink.
type PathView struct {
	Title      string
	RelPath    string
	GuideHref  string
	SealTarget string
	Paths      []PathLink
	Branches   []PathBranchView

	// Path-level figures, precomputed so the header metarow is a dumb read.
	// Entries is the course's planned lesson total — the main line only, a
	// planned-but-unwritten lesson included. Ready is the subset sitting at
	// ready — never a measure of progress: a lesson finished and published
	// leaves it.
	Parts   int
	Modules int
	Entries int
	Ready   int
}

// PathBranchView is one branch of the course as the page draws it. A top-level
// branch is a part (Depth 0): it carries an Anchor the "On this path" rail
// jumps to and an Ordinal (roman numeral). A nested branch is a module
// (Depth >= 1) numbered by sibling position, and a side branch is a module the
// author declared local, drawn under the lesson it hangs from.
//
// Items hold what the branch lists in source order, entries and side branches
// together, so the page places a side branch where the author put it instead of
// matching it back to a lesson by name.
type PathBranchView struct {
	Anchor  string
	Ordinal string
	Num     int
	Heading string
	Depth   int
	// Local marks a side branch: its own order and its own count, never part of
	// the main line.
	Local bool
	Total int
	Items []PathItemView
}

// PathItemView is one thing a branch lists. Exactly one field is set.
type PathItemView struct {
	Entry  *PathEntryView
	Branch *PathBranchView
}

// PathEntryView is one linked or warning row. Only resolved rows have an
// href, a status, or the ready accent. Number is copied from navigation's own
// walk — the
// single owner of sequence position — and keeps its meaning: zero says the
// walk never reaches the row's branch, and the page must not print an ordinal
// no walk can match.
type PathEntryView struct {
	Text   string
	Href   string
	Status string
	Sealed bool
	Kind   nav.EntryKind
	Number int
}

// PathRunView is one uninterrupted stretch of what a branch lists: either a
// run of rows or one nested branch. Ordered marks a run whose rows carry
// sequence numbers, which is what lets the page render it as a real ordered
// list instead of look-alike siblings; Label is that list's accessible name.
type PathRunView struct {
	Entries []PathEntryView
	Branch  *PathBranchView
	Ordered bool
	Label   string
}

// Runs regroups a branch's items for rendering: consecutive rows form one
// run, and each nested branch stands alone, exactly in document order. It is
// derived on demand so Items stays the single stored form of the branch.
//
// An ordered run is named for the sequence component it belongs to — 主線 for
// the main line, 支線：name for a side branch — with （接續） marking a
// fragment that resumes after an interruption, read off the first row's walk
// number. Assistive technology otherwise announces every fragment as an
// anonymous list, and a course split by its own headings and side branches
// becomes several indistinguishable ones. The name is a plain attribute
// string: no visible text says 主線 or （接續） anywhere — the part headings
// are the author's own words — so there is nothing for the accessible name to
// point at, and the side-branch heading beside its list also carries the
// count chip, which a name should not swallow. The rail's lesson-steps
// label set the precedent of a chrome phrase carrying an authored title.
func (v *PathBranchView) Runs() []PathRunView {
	var runs []PathRunView
	for _, item := range v.Items {
		switch {
		case item.Entry != nil:
			if len(runs) == 0 || runs[len(runs)-1].Branch != nil {
				run := PathRunView{Ordered: item.Entry.Number > 0}
				if run.Ordered {
					run.Label = v.runLabel(item.Entry.Number)
				}
				runs = append(runs, run)
			}
			last := &runs[len(runs)-1]
			last.Entries = append(last.Entries, *item.Entry)
		case item.Branch != nil:
			runs = append(runs, PathRunView{Branch: item.Branch})
		}
	}
	return runs
}

// runLabel names one ordered fragment. first is the fragment's first walk
// number: one means the component opens here, anything later means the
// fragment resumes an already-open order.
func (v *PathBranchView) runLabel(first int) string {
	if v.Local {
		if first > 1 {
			return "支線：" + v.Heading + "（接續）"
		}
		return "支線：" + v.Heading
	}
	if first > 1 {
		return "主線（接續）"
	}
	return "主線"
}

// PathLink is one entry in the path switcher: a study-path's title, the
// URL to its page, its planned lesson count, and whether it is the path
// currently shown.
type PathLink struct {
	Title   string
	RelPath string
	Entries int
	Active  bool
}

// BuildPathView draws one study path's declared structure into the page view,
// and builds the switcher from every study path in the vault.
//
// It draws what the grammar lets navigation read and nothing else: a branch the
// author declared out of the course, never declared at all, or wrote with a
// structural error is not part of the course, so the course page does not show
// one. Those branches keep their prose on the note's own page, and the judge
// carries the reason to the author.
func BuildPathView(current *nav.Path, all []nav.Path) PathView {
	v := PathView{
		Title:      current.Title,
		RelPath:    current.RelPath,
		GuideHref:  notesHref(current.RelPath),
		SealTarget: schema.SealStatus,
		Paths:      buildPaths(current.RelPath, all),
		Entries:    current.Planned,
	}
	for _, g := range current.Groups {
		sv, ok := buildPathBranch(g, 0, v.Parts+1)
		if !ok {
			continue
		}
		v.Branches = append(v.Branches, sv)
		v.Parts++
		v.Modules += countModules(&sv)
	}
	v.Ready = countReady(v.Branches)
	return v
}

// buildPathBranch converts one projectable branch and its drawable subtree into
// a view. ok is false for a branch the course does not include and that carries
// no declared branch beneath it — a structural heading whose whole job is to
// hold parts still draws, because dropping it would orphan them. Sequence
// position is not decided here: navigation's walk already numbered every row
// it reaches, and the view only copies that answer.
func buildPathBranch(g *nav.PathGroup, depth, num int) (PathBranchView, bool) {
	if !drawable(g) {
		return PathBranchView{}, false
	}
	sv := PathBranchView{
		Heading: g.Name,
		Depth:   depth,
		Num:     num,
		Local:   g.Role == sequence.RoleLocal,
		Total:   g.Planned,
	}
	if depth == 0 {
		sv.Anchor = "part-" + strconv.Itoa(num)
		sv.Ordinal = roman(num)
	}
	children := 0
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			if !g.Projectable || item.Entry.State != sequence.EntryAccepted {
				continue
			}
			entry := buildPathEntry(item.Entry)
			sv.Items = append(sv.Items, PathItemView{Entry: &entry})
		case item.Group != nil:
			children++
			child, ok := buildPathBranch(item.Group, depth+1, children)
			if !ok {
				children--
				continue
			}
			sv.Items = append(sv.Items, PathItemView{Branch: &child})
		}
	}
	return sv, true
}

// drawable reports whether the course page shows this branch: one the grammar
// projects, or a structural heading that carries one.
func drawable(g *nav.PathGroup) bool {
	if g.Projectable {
		return true
	}
	if g.Invalid || g.Role != sequence.RoleStructural {
		return false
	}
	for _, item := range g.Items {
		if item.Group != nil && drawable(item.Group) {
			return true
		}
	}
	return false
}

// countModules is how many branches sit beneath a part, at any depth. The
// metarow says "modules", and a side branch is one of them.
func countModules(sv *PathBranchView) int {
	n := 0
	for _, item := range sv.Items {
		if item.Branch != nil {
			n += 1 + countModules(item.Branch)
		}
	}
	return n
}

// countReady counts the lessons sitting at ready — never a measure of
// progress, because a lesson finished and published leaves it.
func countReady(branches []PathBranchView) int {
	n := 0
	for _, sv := range branches {
		for _, item := range sv.Items {
			switch {
			case item.Entry != nil:
				if item.Entry.Sealed {
					n++
				}
			case item.Branch != nil:
				n += countReady([]PathBranchView{*item.Branch})
			}
		}
	}
	return n
}

// buildPathEntry maps one nav entry onto a linked or warning study-path row.
// The number is copied for every row, warning rows included — a planned
// lesson keeps its place on the line it is planned into.
func buildPathEntry(entry *nav.PathEntry) PathEntryView {
	v := PathEntryView{Text: entry.Text, Kind: entry.Kind, Number: entry.Number}
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
func buildPaths(currentRel string, all []nav.Path) []PathLink {
	links := make([]PathLink, 0, len(all))
	for _, s := range all {
		links = append(links, PathLink{
			Title:   s.Title,
			RelPath: s.RelPath,
			Entries: s.Planned,
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
