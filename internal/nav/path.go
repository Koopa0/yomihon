package nav

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/vault"
)

// Path is one study path read through the declared-sequence grammar: the
// branch tree exactly as internal/sequence interpreted it, with each accepted
// entry's resolution outcome attached. Navigation, the syllabus page, Home and
// the sidebar all read this one interpretation; none of them parses the
// Markdown again.
type Path struct {
	Title   string
	RelPath string
	Domain  string
	Type    string
	// Groups are the top-level branches in document order.
	Groups []*PathGroup
	// Planned is the course total Home shows: the main line's accepted
	// entries, resolved or not — a lesson that is planned but unwritten is
	// still one of the course's lessons. Nothing outside the projectable
	// primary line counts here.
	Planned int

	// components are the walkable sequences, precomputed at build: the main
	// line first when it has any openable stop, then each projectable local
	// branch in document order. Prev/next reads these and nothing else, so
	// primary and local can never link to each other.
	components [][]NoteRef
}

// PathGroup is one branch of a study path as navigation sees it. Role,
// Projectable and the anchor identity come straight from the sequence
// interpretation; nothing here re-derives a verdict from diagnostics.
type PathGroup struct {
	Name  string
	Level int
	Role  sequence.Role
	// Container is true for a branch opened by a nested list row.
	Container bool
	// Projectable is the sequence grammar's one verdict: only a declared
	// primary or local branch free of structural error projects.
	Projectable bool
	// Invalid carries the sequence grammar's structural-error verdict. An
	// invalid branch keeps its rows for the author; nothing projects from it.
	Invalid bool
	// AnchorTarget and AnchorSpan name and identify the entry a local branch
	// hangs from — the span still tells two rows naming the same note apart.
	AnchorTarget string
	AnchorSpan   sequence.Span
	// Planned is how many accepted entries this branch itself lists — the
	// count a local branch shows beside its name.
	Planned int
	// Items hold everything the branch lists in source order: a local branch
	// follows the entry it hangs from, and a renderer that walks Items in
	// order places it there without guessing from targets or lines.
	Items []PathItem
}

// PathItem is one thing a branch holds in source order. Exactly one field is
// set.
type PathItem struct {
	Entry *PathEntry
	Group *PathGroup
}

// PathEntry is one candidate row with its resolution outcome. State says what
// the grammar decided; Kind, RelPath, Status and Candidates say what resolving
// the target found, and are meaningful only for an accepted entry — a row the
// grammar did not accept is never resolved, because it never becomes a lesson.
type PathEntry struct {
	Text   string
	Target string
	Line   int
	Span   sequence.Span
	State  sequence.EntryState

	Kind       EntryKind
	RelPath    string
	Status     string
	Candidates []string
}

// Openable reports whether this row is a lesson a reader can walk to: an
// accepted entry whose target resolved to a governed note.
func (e *PathEntry) Openable() bool {
	return e.State == sequence.EntryAccepted && e.Kind == EntryResolved
}

// buildPath reads one path-typed note through the sequence grammar and
// attaches resolution outcomes. This is the once-per-snapshot parse; every
// surface downstream reads the result.
func buildPath(
	n *vault.Note,
	idx Resolver,
	statusByPath map[string]string,
	policy schema.ArtifactPolicy,
) Path {
	doc := sequence.Parse(n.Body, n.BodyLine)
	p := Path{
		Title:   n.Title(),
		RelPath: n.RelPath,
		Domain:  n.Domain(),
		Type:    n.Type(),
	}
	for _, g := range doc.Groups {
		p.Groups = append(p.Groups, buildPathGroup(g, idx, statusByPath, policy))
	}
	main, locals := projectStops(p.Groups)
	p.Planned = main.planned
	if len(main.stops) > 0 {
		p.components = append(p.components, main.stops)
	}
	for _, local := range locals {
		if len(local) > 0 {
			p.components = append(p.components, local)
		}
	}
	return p
}

// buildPathGroup mirrors one sequence group into the nav model, resolving each
// accepted entry against the vault.
func buildPathGroup(
	g *sequence.Group,
	idx Resolver,
	statusByPath map[string]string,
	policy schema.ArtifactPolicy,
) *PathGroup {
	out := &PathGroup{
		// headingLabel is this vault's own heading dialect — a "slug | English |
		// 中文" heading displays its English column — and it applies to a course's
		// branches exactly as it does to a map's. The grammar reads structure;
		// what a branch is called is still read the way the vault writes it.
		Name:         headingLabel(g.Name),
		Level:        g.Level,
		Role:         g.Role,
		Container:    g.Container,
		Projectable:  g.Projectable(),
		Invalid:      g.Invalid,
		AnchorTarget: g.AnchorTarget,
		AnchorSpan:   g.AnchorSpan,
	}
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			entry := buildPathEntry(item.Entry, idx, statusByPath, policy)
			if entry.State == sequence.EntryAccepted {
				out.Planned++
			}
			out.Items = append(out.Items, PathItem{Entry: entry})
		case item.Branch != nil:
			out.Items = append(out.Items, PathItem{Group: buildPathGroup(item.Branch, idx, statusByPath, policy)})
		}
	}
	return out
}

// buildPathEntry attaches a resolution outcome to one accepted entry. A row
// the grammar did not accept keeps the zero outcome: it is not a lesson, so
// there is nothing to resolve.
func buildPathEntry(
	c *sequence.Candidate,
	idx Resolver,
	statusByPath map[string]string,
	policy schema.ArtifactPolicy,
) *PathEntry {
	entry := &PathEntry{
		Text:   c.Text,
		Target: c.Target,
		Line:   c.Line,
		Span:   c.Span,
		State:  c.State,
	}
	if c.State != sequence.EntryAccepted {
		return entry
	}
	res := idx.Resolve(c.Target)
	switch res.Kind {
	case graph.Unique:
		if policy.IsNonInstance(res.Path) {
			entry.Kind = EntryNonInstance
			return entry
		}
		entry.Kind = EntryResolved
		entry.RelPath = res.Path
		entry.Status = statusByPath[res.Path]
	case graph.Ambiguous:
		entry.Kind = EntryAmbiguous
		entry.Candidates = slices.Clone(res.Candidates)
	default:
		entry.Kind = EntryUnresolved
	}
	return entry
}

// mainLine is the primary walk's result: how many lessons the course plans,
// and the resolved stops a reader can actually open.
type mainLine struct {
	planned int
	stops   []NoteRef
}

// projectStops walks a path's groups in document order and separates what the
// grammar lets navigation read:
//
//   - every projectable primary group joins one main line, end to end in
//     declared order — the course has one main line, and a none block between
//     two primary parts does not cut it;
//   - each projectable local group is a component of its own, never joining
//     the main line;
//   - a structural branch carries its declared descendants and contributes
//     nothing itself;
//   - none, unclassified and invalid branches project nothing, subtree
//     included — their prose still reads on the note page, and their
//     diagnostics reach the author through the judge.
func projectStops(groups []*PathGroup) (main mainLine, locals [][]NoteRef) {
	walker := &stopWalk{}
	for _, g := range groups {
		walker.walk(g)
	}
	return walker.main, walker.locals
}

// stopWalk carries the walk's result while it recurses.
type stopWalk struct {
	main   mainLine
	locals [][]NoteRef
}

func (w *stopWalk) walk(g *PathGroup) {
	switch {
	case g.Projectable && g.Role == sequence.RolePrimary:
		w.primary(g)
	case g.Projectable && g.Role == sequence.RoleLocal:
		w.locals = append(w.locals, localStops(g))
	case g.Role == sequence.RoleStructural && !g.Invalid:
		w.descend(g)
	}
}

// primary folds one main-line branch into the course: every accepted row is a
// planned lesson, and the ones that resolve are the stops a reader can walk.
func (w *stopWalk) primary(g *PathGroup) {
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			if item.Entry.State != sequence.EntryAccepted {
				continue
			}
			w.main.planned++
			if item.Entry.Openable() {
				w.main.stops = append(w.main.stops, NoteRef{Name: item.Entry.Text, RelPath: item.Entry.RelPath})
			}
		case item.Group != nil:
			w.walk(item.Group)
		}
	}
}

// descend carries a branch that only groups other branches.
func (w *stopWalk) descend(g *PathGroup) {
	for _, item := range g.Items {
		if item.Group != nil {
			w.walk(item.Group)
		}
	}
}

// localStops are the lessons a local branch itself lists that a reader can
// open. A local branch never carries another walkable branch: an undeclared
// nested list beneath it projects nothing, and a local inside a local was
// already refused.
func localStops(g *PathGroup) []NoteRef {
	var stops []NoteRef
	for _, item := range g.Items {
		if item.Entry != nil && item.Entry.Openable() {
			stops = append(stops, NoteRef{Name: item.Entry.Text, RelPath: item.Entry.RelPath})
		}
	}
	return stops
}

// pathPlacements records every projectable accepted, resolved entry of one
// path into the reverse index — and nothing else: a row the grammar did not
// accept, and every entry of a none, unclassified or invalid branch, is not a
// course membership.
func pathPlacements(index map[string][]Placement, p *Path) {
	for _, g := range p.Groups {
		placeGroup(index, p.RelPath, g, nil)
	}
}

// placeGroup records one branch's lessons and recurses. A branch the course
// does not include contributes nothing: being listed there is not membership.
func placeGroup(index map[string][]Placement, pathRel string, g *PathGroup, chain []string) {
	if !g.Projectable && (g.Role != sequence.RoleStructural || g.Invalid) {
		return
	}
	here := chain
	if g.Name != "" {
		here = slices.Concat(chain, []string{g.Name})
	}
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			if !g.Projectable || !item.Entry.Openable() || item.Entry.RelPath == "" {
				continue
			}
			index[item.Entry.RelPath] = append(index[item.Entry.RelPath], Placement{
				MapRelPath: pathRel,
				Headings:   slices.Clone(here),
			})
		case item.Group != nil:
			placeGroup(index, pathRel, item.Group, here)
		}
	}
}
