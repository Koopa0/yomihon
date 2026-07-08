package nav

import (
	"fmt"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
)

// Syllabus is one parsed study-path note: its title, its own vault path
// (the sidebar links to it), and its heading/lesson tree in document order.
type Syllabus struct {
	Title    string
	RelPath  string
	Sections []Section
}

// Section is one heading in a syllabus (an H2 part, an H3 module/stage, or
// any deeper heading), holding the lessons listed directly beneath it and
// its nested subsections. A Section is present only if it, or a descendant,
// carries at least one lesson (see pruneSections) — so a syllabus's prose,
// daily-loop, and gap sections, which list no lessons, never appear.
type Section struct {
	// Heading is the display label: for a pipe-format heading
	// "slug | English | Chinese" it is the English column; otherwise the
	// whole heading text (e.g. a plain-text stage name).
	Heading string
	// Level is the markdown heading level (2 for a part, 3 for a module),
	// kept so a renderer can indent by depth without re-deriving it.
	Level   int
	Lessons []Lesson
	Sub     []Section
}

// Lesson is one lesson list-item: the wikilink's display text, its raw
// target, and how that target resolved. An unresolved or ambiguous lesson
// is still a Lesson (surfaced with its problem, never silently dropped);
// RelPath and Status are set only when the target resolves uniquely, and
// Candidates only when it is ambiguous.
type Lesson struct {
	Text       string
	Target     string
	RelPath    string
	Status     string
	Resolution graph.Kind
	Candidates []string
}

// parseSyllabus parses one study-path note into a Syllabus. It reads the
// already-frontmatter-stripped body (vault.Parse split it out), so a
// frontmatter list value can never look like a lesson bullet.
func parseSyllabus(n *vault.Note, idx Resolver, statusByPath map[string]string) Syllabus {
	return Syllabus{
		Title:    n.Title(),
		RelPath:  n.RelPath,
		Sections: parseSections(n.Body, idx, statusByPath),
	}
}

// sectionNode is the mutable tree node used while parsing; it becomes the
// read-only Section value after pruning.
type sectionNode struct {
	heading string
	level   int
	lessons []Lesson
	sub     []*sectionNode
}

// parseSections is the one mechanical rule that produces the correct tree
// for both real syllabus shapes without hardcoding filenames or section
// titles:
//
//   - Walk the body line by line. A heading at level >= 2 opens a section
//     nested under the nearest shallower open heading (a stack); an H1 (the
//     document title) is ignored, mirroring the reading page's leading-H1
//     removal.
//   - A lesson list-item attaches to the currently open heading. A lesson
//     list-item is an unordered bullet ("- ", "* ", "+ "), not a GFM task
//     checkbox ("- [ ]" / "- [x]"), that contains at least one [[wikilink]].
//     Its link is that first wikilink, resolved by graph semantics.
//   - Finally, prune every heading that has no lesson anywhere beneath it.
//
// That predicate is what distinguishes the two files' non-navigation
// sections without naming them: the Go syllabus's parts/modules hold plain
// "- [[Lesson]]" bullets (all kept); the 大家 syllabus's course-sequence
// stages hold "- **L1** ... · [[L01 ...]]" bullets (kept), while its
// daily-loop section uses an ordered list (excluded — not a bullet), its
// learning-stages section is a table (no list items), and its gaps section
// uses task checkboxes (excluded — even the one carrying a [[wikilink]]),
// so all three prune away for having no lessons. A "待建" bullet with no
// wikilink is not counted (there is no lesson to link to) — the lesson
// count is exactly the number of wikilink-bearing list-items.
func parseSections(body string, idx Resolver, statusByPath map[string]string) []Section {
	var roots []*sectionNode
	var stack []*sectionNode

	for line := range strings.SplitSeq(body, "\n") {
		if text, level, ok := parseHeading(line); ok {
			node := &sectionNode{heading: headingLabel(text), level: level}
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
		inner, ok := parseLessonItem(line)
		if !ok {
			continue
		}
		lesson, ok := makeLesson(inner, idx, statusByPath)
		if !ok || len(stack) == 0 {
			// No resolvable link target, or a lesson bullet before any
			// heading (nowhere to attach) — neither happens in the real
			// syllabi; both are dropped without panicking.
			continue
		}
		top := stack[len(stack)-1]
		top.lessons = append(top.lessons, lesson)
	}
	return convertSections(pruneSections(roots))
}

// pruneSections drops every node with no lessons and no surviving
// descendant with lessons — the step that removes a syllabus's prose,
// loop, and gap headings without ever naming them.
func pruneSections(nodes []*sectionNode) []*sectionNode {
	kept := nodes[:0:0]
	for _, n := range nodes {
		n.sub = pruneSections(n.sub)
		if len(n.lessons) > 0 || len(n.sub) > 0 {
			kept = append(kept, n)
		}
	}
	return kept
}

// convertSections turns the mutable node tree into the read-only Section
// tree, preserving document order at every level.
func convertSections(nodes []*sectionNode) []Section {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]Section, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, Section{
			Heading: n.heading,
			Level:   n.level,
			Lessons: n.lessons,
			Sub:     convertSections(n.sub),
		})
	}
	return out
}

// parseHeading reports an ATX heading of level >= 2: the "#" run must start
// the line, be at least two long, and be followed by a space. Level 0/1
// (body text, or the document-title H1) is not a section.
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

// parseLessonItem reports whether line is a lesson list-item and returns the
// inner text of its first wikilink. It requires an unordered bullet marker,
// rejects GFM task checkboxes, and requires a [[wikilink]] to be present —
// see parseSections's doc for why each condition matters.
func parseLessonItem(line string) (inner string, ok bool) {
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
// second byte is "[", not a space or x), so "- [[Lesson]]" is never
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

// makeLesson resolves a wikilink's inner text into a Lesson. It reuses
// graph.SplitWikilink (the same target/display extraction the renderer
// uses) and idx.Resolve (the same normalization and ambiguity rules applied
// to in-body wikilinks), so a sidebar lesson link and the in-body wikilink
// to the same note agree exactly. ok is false only when the wikilink strips
// to an empty target (a same-file anchor such as [[#heading]]), which is not
// a lesson.
func makeLesson(inner string, idx Resolver, statusByPath map[string]string) (Lesson, bool) {
	target, display, ok := graph.SplitWikilink(inner)
	if !ok {
		return Lesson{}, false
	}
	l := Lesson{Text: display, Target: target}
	res := idx.Resolve(target)
	l.Resolution = res.Kind
	switch res.Kind {
	case graph.Unique:
		l.RelPath = res.Path
		l.Status = statusByPath[res.Path]
	case graph.Ambiguous:
		l.Candidates = res.Candidates
	case graph.Unresolved:
		// Still listed, carrying only its target for the diagnostic — never dropped.
	default:
		panic(fmt.Sprintf("nav: unknown graph.Kind %d", res.Kind))
	}
	return l, true
}
