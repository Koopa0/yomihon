package pages

import (
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/wording"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// PathView is everything the study-path page needs: the current path's branches
// with their totals and anchors, the switcher across every study path, and the
// figures the header metarow reads. Navigation has already classified every
// entry, so the page never resolves a wikilink.
type PathView struct {
	Title      string
	RelPath    string
	GuideHref  string
	SealTarget string
	Paths      []PathLink
	Branches   []PathBranchView

	// Entries is the course's planned lesson total, the main line only and a
	// planned-but-unwritten lesson included; Ready is the subset at the
	// reviewed status, which is not progress — a published lesson leaves it.
	Parts   int
	Modules int
	Entries int
	Ready   int

	// NoCourse is which explanation the empty-course page is entitled to give:
	// that a written sequence marker is among what the grammar reported, that
	// none is, or that the report holds something this page cannot explain.
	NoCourse markerVerdict
}

// PathBranchView is one branch of the course as the page draws it. A top-level
// branch is a part, carrying the anchor the rail jumps to and a roman ordinal; a
// nested one is a module numbered by sibling position. Items hold what the
// branch lists in source order, so a side branch is placed where the author put
// it rather than matched back to a lesson by name.
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

// PathItemView is one thing a branch lists: a row, or a nested branch. A value
// carrying neither is drawn as a fault rather than dropped, so the course still
// reads as the length its author wrote.
type PathItemView struct {
	Entry  *PathEntryView
	Branch *PathBranchView
}

// PathEntryView is one linked or warning row. Only resolved rows have an href, a
// status, or the ready accent. Number is copied from navigation's walk, the one
// owner of sequence position, and zero means the walk never reaches the row.
type PathEntryView struct {
	Text   string
	Href   string
	Status string
	Sealed bool
	Kind   nav.EntryKind
	Number int
}

// PathRunView is one uninterrupted stretch of what a branch lists: a run of
// rows, one nested branch, or a fault standing where the page could read
// neither. Ordered marks a run whose rows carry sequence numbers, so the page
// can render a real ordered list; Label is that list's accessible name.
type PathRunView struct {
	Entries []PathEntryView
	Branch  *PathBranchView
	Ordered bool
	Label   string
	// Fault is what the page says in place of an item it could not read. It
	// keeps the item's position, so the course still reads as the length its
	// author wrote.
	Fault string
}

// Runs regroups a branch's items for rendering: consecutive rows form one run,
// and each nested branch stands alone, in document order. It is derived on
// demand so Items stays the branch's single stored form.
//
// An ordered run is named for the sequence component it belongs to, marking a
// fragment that resumes after an interruption. Assistive technology otherwise
// announces every fragment as an anonymous list, and a course split by its own
// headings becomes several indistinguishable ones; nothing visible names them.
func (v *PathBranchView) Runs(lang wording.Lang) []PathRunView {
	var runs []PathRunView
	for _, item := range v.Items {
		switch {
		case item.Entry != nil:
			if len(runs) == 0 || !runs[len(runs)-1].holdsRows() {
				run := PathRunView{Ordered: item.Entry.Number > 0}
				if run.Ordered {
					run.Label = v.runLabel(item.Entry.Number, lang)
				}
				runs = append(runs, run)
			}
			last := &runs[len(runs)-1]
			last.Entries = append(last.Entries, *item.Entry)
		case item.Branch != nil:
			runs = append(runs, PathRunView{Branch: item.Branch})
		default:
			// The item names neither a row nor a branch, so it is a fault
			// standing in the course where the author put something.
			runs = append(runs, PathRunView{Fault: wording.PathItemUnreadable.In(lang)})
		}
	}
	return runs
}

// holdsRows reports whether another row can join this run. A run standing for a
// nested branch or an unreadable item is closed, the interruption being what the
// reader sees between them.
func (r *PathRunView) holdsRows() bool {
	return r.Branch == nil && r.Fault == ""
}

// runLabel names one ordered fragment. first is the fragment's first walk
// number: one means the component opens here, anything later means the
// fragment resumes an already-open order.
func (v *PathBranchView) runLabel(first int, lang wording.Lang) string {
	if v.Local {
		if first > 1 {
			return wording.BranchPrefix.In(lang) + v.Heading + wording.BranchContinued.In(lang)
		}
		return wording.BranchPrefix.In(lang) + v.Heading
	}
	if first > 1 {
		return wording.MainContinued.In(lang)
	}
	return wording.MainLine.In(lang)
}

// PathLink is one entry in the path switcher: a study path's title, the URL to
// its page, its planned lesson count, and whether it is the one shown.
type PathLink struct {
	Title   string
	RelPath string
	Entries int
	Active  bool
}

// BuildPathView draws one study path's declared structure into the page view and
// builds the switcher from every study path in the vault. It draws what the
// grammar lets navigation read and nothing else; a branch outside the course
// keeps its prose on the note's own page.
func BuildPathView(current *nav.Path, all []nav.Path) PathView {
	v := PathView{
		Title:      current.Title,
		RelPath:    current.RelPath,
		GuideHref:  notesHref(current.RelPath),
		SealTarget: schema.SealStatus,
		Paths:      buildPaths(current.RelPath, all),
		Entries:    current.Planned,
		Ready:      current.Ready,
	}
	// A written marker outranks the rest, and an unrecognised rule outranks
	// the no-marker reading, which would otherwise put words in the author's
	// mouth on the strength of not knowing a rule name.
	for _, d := range current.Diagnostics {
		switch verdict := markerVerdictFor(d.Rule); verdict {
		case markerWritten:
			v.NoCourse = markerWritten
		case markerUnknownRule:
			if v.NoCourse == markerNotWritten {
				v.NoCourse = verdict
			}
		case markerNotWritten:
		}
		if v.NoCourse == markerWritten {
			break
		}
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
	return v
}

// buildPathBranch converts one projectable branch and its drawable subtree into
// a view. ok is false for a branch the course excludes that carries no declared
// branch beneath it; a structural heading still draws, since dropping it would
// orphan its parts. Sequence position is copied from navigation's walk.
func buildPathBranch(g *nav.PathGroup, depth, num int) (PathBranchView, bool) {
	if !g.Drawn() {
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
			if !g.Teaches(item.Entry) {
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

// countModules is how many branches sit beneath a part, at any depth, a side
// branch included.
func countModules(sv *PathBranchView) int {
	n := 0
	for _, item := range sv.Items {
		if item.Branch != nil {
			n += 1 + countModules(item.Branch)
		}
	}
	return n
}

// buildPathEntry maps one nav entry onto a linked or warning study-path row. The
// number is copied for every row, so a planned lesson keeps its place.
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

// A row says three things about how its target resolved: the words a reader
// sees, the token the markup carries, and the explanation behind the row. A kind
// none of them answers for is reported as its number rather than aborting — a
// kind added in navigation compiles here and must not take down every page.
func entryResolutionLabel(kind nav.EntryKind, lang wording.Lang) string {
	switch kind {
	case nav.EntryUnresolved:
		return wording.EntryUnresolved.In(lang)
	case nav.EntryAmbiguous:
		return wording.EntryAmbiguous.In(lang)
	case nav.EntryNonInstance:
		return wording.EntryNonInstance.In(lang)
	case nav.EntryResolved:
		return wording.EntryResolved.In(lang)
	default:
		return entryResolutionCode(kind)
	}
}

// entryResolutionCode is the stable machine token carried by data-resolution. It
// stays English; entryResolutionLabel owns the reader's own words.
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
		return strconv.Itoa(int(kind))
	}
}

func entryResolutionTitle(kind nav.EntryKind, lang wording.Lang) string {
	switch kind {
	case nav.EntryUnresolved:
		return wording.EntryUnresolvedTitle.In(lang)
	case nav.EntryAmbiguous:
		return wording.EntryAmbiguousTitle.In(lang)
	case nav.EntryNonInstance:
		return wording.EntryNonInstanceTitle.In(lang)
	default:
		// A resolved row has nothing to explain, and neither has a kind this
		// page has no words for: the row already carries its token.
		return ""
	}
}

// markerVerdict is what one grammar rule lets the page say about the author of
// the note it came from.
type markerVerdict uint8

const (
	// markerNotWritten is a rule that arises with no sequence marker near it,
	// so the page may say the note carries none.
	markerNotWritten markerVerdict = iota
	// markerWritten is a rule that arises only where a marker was written.
	markerWritten
	// markerUnknownRule is a rule this page has not been told about, and so is
	// no evidence for either claim about what somebody wrote in a file.
	markerUnknownRule
)

// markerVerdictFor classifies one grammar rule. The division is total over the
// grammar's declared rules; the third answer exists for a rule the grammar
// gains later, which a course page is no place to abort on.
func markerVerdictFor(rule sequence.Rule) markerVerdict {
	switch rule {
	case sequence.RuleRoleInvalid,
		sequence.RuleRoleDuplicate,
		sequence.RuleRoleMisplaced,
		sequence.RuleRoleConflict,
		sequence.RuleRoleOnEntry,
		sequence.RuleLocalOrphan,
		sequence.RuleNestingTooDeep:
		return markerWritten
	case sequence.RuleRoleMissing,
		sequence.RuleEntryOutsideBranch,
		sequence.RuleEntryMultiTarget,
		sequence.RuleEntryNoncanonical:
		return markerNotWritten
	default:
		return markerUnknownRule
	}
}

// markerForm spells the full marker for one role from the grammar's own
// vocabulary, so the page cannot drift from what the parser accepts.
func markerForm(role sequence.Role) string {
	return "{sequence=" + role.String() + "}"
}

// buildPaths builds the switcher: every study path in vault order, each with its
// entry count and whether it is the one shown.
func buildPaths(currentRel string, all []nav.Path) []PathLink {
	links := make([]PathLink, 0, len(all))
	for i := range all {
		s := &all[i]
		links = append(links, PathLink{
			Title:   s.Title,
			RelPath: s.RelPath,
			Entries: s.Planned,
			Active:  s.RelPath == currentRel,
		})
	}
	return links
}

// roman renders a positive part number as a roman numeral. A non-positive n
// falls back to its decimal form rather than panicking.
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
