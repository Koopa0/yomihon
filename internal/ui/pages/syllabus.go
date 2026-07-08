package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
)

// SyllabusView is everything the study-path page needs: the current path's
// tree flattened into render-ready sections (with per-section ready/total
// tallies and anchors precomputed), the switcher across every study-path in
// the vault, and the path-level figures the header and progress bar read.
//
// It duplicates no schema enum: "ready" comes from sealStatus (the same single
// constant the reading page keys the seal on), and lesson resolution comes from
// nav's already-parsed graph.Kind — the page only reports what nav resolved,
// never re-resolves.
type SyllabusView struct {
	Title    string
	RelPath  string
	Paths    []SyllabusLink
	Sections []SectionView

	// Path-level figures, precomputed so the header metarow and progress bar
	// are dumb reads. Lessons is the whole-path lesson total; Ready the subset
	// that has reached the seal.
	Parts   int
	Modules int
	Lessons int
	Ready   int
}

// SectionView is one heading in the flattened tree. A top-level section is a
// part (Depth 0): it carries an Anchor the "On this path" rail jumps to and an
// Ordinal (roman numeral). A nested section is a module (Depth >= 1): it carries
// a Num (its 1-based position among its siblings). Ready/Total are the lesson
// tallies for this section and everything beneath it.
type SectionView struct {
	Anchor  string
	Ordinal string
	Num     int
	Heading string
	Depth   int
	Ready   int
	Total   int
	Lessons []LessonView
	Sub     []SectionView
}

// LessonView is one lesson row. A uniquely-resolved lesson has an Href and,
// when the target note carries one, a Status; an ambiguous or unresolved lesson
// has an empty Href and a Mark ("ambiguous" / "unresolved") instead — still
// listed, never dropped: a broken link is surfaced to the reader, not hidden.
type LessonView struct {
	Text   string
	Href   string
	Status string
	Mark   string
}

// SyllabusLink is one entry in the path switcher: a study-path's title, the
// URL to its page, its whole-path lesson count, and whether it is the path
// currently shown.
type SyllabusLink struct {
	Title   string
	RelPath string
	Lessons int
	Active  bool
}

// BuildSyllabusView flattens one parsed study-path (current) into the page
// view, and builds the switcher from every study-path in the vault (all). It is
// pure: the lesson count it reports for a section equals the number of resolved
// lesson list-items beneath it, and document order is
// preserved at every level (nav already guarantees it).
func BuildSyllabusView(current nav.Syllabus, all []nav.Syllabus) SyllabusView {
	v := SyllabusView{
		Title:   current.Title,
		RelPath: current.RelPath,
		Paths:   buildPaths(current.RelPath, all),
	}
	for i, sec := range current.Sections {
		sv := buildSection(sec, 0, i+1)
		v.Sections = append(v.Sections, sv)
		v.Parts++
		v.Modules += len(sv.Sub)
		v.Lessons += sv.Total
		v.Ready += sv.Ready
	}
	return v
}

// buildSection converts one nav.Section (and its subtree) into a SectionView,
// tallying lessons on the way up. depth 0 is a part (anchored, roman-numbered);
// deeper sections are modules (numbered by sibling position).
func buildSection(sec nav.Section, depth, num int) SectionView {
	sv := SectionView{Heading: sec.Heading, Depth: depth, Num: num}
	if depth == 0 {
		sv.Anchor = "part-" + strconv.Itoa(num)
		sv.Ordinal = roman(num)
	}
	for i := range sec.Lessons {
		l := &sec.Lessons[i]
		sv.Lessons = append(sv.Lessons, buildLesson(l))
		sv.Total++
		if l.Status == sealStatus {
			sv.Ready++
		}
	}
	for i, sub := range sec.Sub {
		child := buildSection(sub, depth+1, i+1)
		sv.Sub = append(sv.Sub, child)
		sv.Total += child.Total
		sv.Ready += child.Ready
	}
	return sv
}

// buildLesson maps a nav.Lesson's resolution to a row: a link when unique, a
// warn-marked non-link when ambiguous or unresolved. The default panics
// because a new graph.Kind must force this switch to be updated, not silently
// render a blank row.
func buildLesson(l *nav.Lesson) LessonView {
	switch l.Resolution {
	case graph.Unique:
		return LessonView{Text: l.Text, Href: notesHref(l.RelPath), Status: l.Status}
	case graph.Ambiguous:
		return LessonView{Text: l.Text, Mark: "ambiguous"}
	case graph.Unresolved:
		return LessonView{Text: l.Text, Mark: "unresolved"}
	default:
		panic(fmt.Sprintf("pages: unknown graph.Kind %d", l.Resolution))
	}
}

// buildPaths builds the switcher: every study-path in vault order, each with
// its whole-path lesson count and whether it is the one currently shown.
func buildPaths(currentRel string, all []nav.Syllabus) []SyllabusLink {
	links := make([]SyllabusLink, 0, len(all))
	for _, s := range all {
		links = append(links, SyllabusLink{
			Title:   s.Title,
			RelPath: s.RelPath,
			Lessons: lessonTotal(s.Sections),
			Active:  s.RelPath == currentRel,
		})
	}
	return links
}

// lessonTotal counts every resolved lesson list-item in a section slice, at any
// depth — the same count BuildSyllabusView tallies, kept separate so the
// switcher can label a path it is not flattening.
func lessonTotal(sections []nav.Section) int {
	n := 0
	for _, sec := range sections {
		n += len(sec.Lessons) + lessonTotal(sec.Sub)
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

// countLabel is a metarow figure with a correctly-pluralised English noun:
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
