package nav

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/vault"
)

// Path is one study path read through the declared-sequence grammar, with each
// accepted entry's resolution outcome attached. Every surface reads this one
// interpretation; none parses the Markdown again.
type Path struct {
	Title   string
	RelPath string
	Domain  string
	Type    string
	// Groups are the top-level branches in document order.
	Groups []*PathGroup
	// Planned is the course total: the main line's accepted entries, resolved
	// or not, since a planned but unwritten lesson is still one of them.
	// Nothing outside the projectable primary line counts here.
	Planned int
	// Ready is how many lessons sit at the reviewed status the contract names.
	// It is not progress — a published lesson leaves it — and unlike Planned it
	// counts every branch a surface draws, side branches included.
	Ready int
	// Diagnostics is everything the grammar left the author to decide, so a
	// reading surface can tell a course that plans nothing from one whose
	// structure could not be read.
	Diagnostics []sequence.Diagnostic

	// components are the walkable sequences: the main line first when it has an
	// openable stop, then each projectable local branch in document order.
	// Prev/next reads these and nothing else, so primary and local never link.
	components [][]NoteRef
}

// clone is a Path a caller may hold, with the whole branch tree copied so
// nothing it does reaches the shared model. The walkable sequences are shared
// rather than copied: no caller outside this package can name them, and the
// walk reads them off the model rather than off a clone.
func (p *Path) clone() Path {
	out := *p
	out.Groups = cloneGroups(p.Groups)
	out.Diagnostics = slices.Clone(p.Diagnostics)
	return out
}

func cloneGroups(groups []*PathGroup) []*PathGroup {
	if groups == nil {
		return nil
	}
	out := make([]*PathGroup, 0, len(groups))
	for _, g := range groups {
		copied := *g
		copied.Items = make([]PathItem, 0, len(g.Items))
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				entry := *item.Entry
				entry.Candidates = slices.Clone(item.Entry.Candidates)
				copied.Items = append(copied.Items, PathItem{Entry: &entry})
			case item.Group != nil:
				copied.Items = append(copied.Items, PathItem{Group: cloneGroups([]*PathGroup{item.Group})[0]})
			}
		}
		out = append(out, &copied)
	}
	return out
}

// PathGroup is one branch of a study path as navigation sees it. Every verdict
// is copied from the sequence interpretation, never re-derived here.
type PathGroup struct {
	Name  string
	Level int
	Role  sequence.Role
	// Container is true for a branch opened by a nested list row.
	Container bool
	// Projectable is the grammar's first verdict: only a declared primary or
	// local branch free of structural error projects.
	Projectable bool
	// Carries is its second: a sound branch that merely groups others, which a
	// walk descends through to reach the course beneath it.
	Carries bool
	// Invalid is its structural-error verdict. An invalid branch keeps its rows
	// for the author; nothing projects from it.
	Invalid bool
	// AnchorTarget and AnchorSpan identify the entry a local branch hangs from;
	// the span tells two rows naming the same note apart.
	AnchorTarget string
	AnchorSpan   sequence.Span
	// Planned is how many accepted entries this branch itself lists.
	Planned int
	// Items hold everything the branch lists in source order, so a renderer
	// walking them places a local branch without guessing from targets or lines.
	Items []PathItem
}

// Drawn reports whether a surface showing this course shows this branch: one
// the grammar projects, or a structural heading that carries one. Every surface
// asks here, so none keeps a rule of its own.
func (g *PathGroup) Drawn() bool {
	if g.Projectable {
		return true
	}
	if !g.Carries {
		return false
	}
	for _, item := range g.Items {
		if item.Group != nil && item.Group.Drawn() {
			return true
		}
	}
	return false
}

// Teaches reports whether this branch offers the row as one of the course's
// lessons: the grammar projects the branch and accepted the row.
func (g *PathGroup) Teaches(entry *PathEntry) bool {
	return entry != nil && g.Projectable && entry.State == sequence.EntryAccepted
}

// PathItem is one thing a branch holds in source order. Exactly one field is
// set.
type PathItem struct {
	Entry *PathEntry
	Group *PathGroup
}

// PathEntry is one candidate row with its resolution outcome. State says what
// the grammar decided; the resolution fields are meaningful only for an
// accepted entry, since a refused row is never resolved.
type PathEntry struct {
	Text   string
	Target string
	Line   int
	Span   sequence.Span
	State  sequence.EntryState

	// Number is the row's position in the sequence its branch projects: the
	// main line counts on through every primary branch, each side branch counts
	// its own rows from one, and a warning row keeps its number. Zero means the
	// walk never reaches the row's branch. Line is a source location instead.
	Number int

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

// buildPath reads one path-typed note through the sequence grammar and attaches
// resolution outcomes. It is the once-per-snapshot parse.
func buildPath(
	n *vault.Note,
	idx *graph.Index,
	statusByPath map[string]string,
	policy schema.ArtifactPolicy,
) Path {
	doc := sequence.Parse(n.Body, n.BodyLine)
	p := Path{
		Title:       n.Title(),
		RelPath:     n.RelPath,
		Domain:      n.Domain(),
		Type:        n.Type(),
		Diagnostics: doc.Diagnostics,
	}
	for _, g := range doc.Groups {
		p.Groups = append(p.Groups, buildPathGroup(g, idx, statusByPath, policy))
	}
	main, locals := projectStops(p.Groups)
	p.Planned = main.planned
	p.Ready = readyLessons(p.Groups)
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
	idx *graph.Index,
	statusByPath map[string]string,
	policy schema.ArtifactPolicy,
) *PathGroup {
	out := &PathGroup{
		// The grammar reads structure; what a branch is called is still read in
		// the vault's own heading dialect, as a map's branches are.
		Name:         headingLabel(g.Name),
		Level:        g.Level,
		Role:         g.Role,
		Container:    g.Container,
		Projectable:  g.Projectable(),
		Carries:      g.Carries(),
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
			child := buildPathGroup(item.Branch, idx, statusByPath, policy)
			// A branch counts what the main line beneath it carries, because a
			// part whose lessons all sit in child branches would otherwise read
			// zero. Only the main line joins counts end to end: a side branch
			// walks its own steps and shows its own number, so folding a nested
			// part into one would print a figure no walk matches.
			if out.Role == sequence.RolePrimary || out.Role == sequence.RoleStructural {
				if child.Role == sequence.RolePrimary || child.Role == sequence.RoleStructural {
					out.Planned += child.Planned
				}
			}
			out.Items = append(out.Items, PathItem{Group: child})
		}
	}
	if !out.Projectable && out.Role != sequence.RoleStructural {
		// Only a branch the course includes, or one carrying such branches, has
		// a course count at all.
		out.Planned = 0
	}
	return out
}

// buildPathEntry attaches a resolution outcome to one accepted entry. A refused
// row keeps the zero outcome: it is not a lesson, so there is nothing to
// resolve.
func buildPathEntry(
	c *sequence.Candidate,
	idx *graph.Index,
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
	case graph.KindUnique:
		if policy.IsNonInstance(res.RelPath) {
			entry.Kind = EntryNonInstance
			return entry
		}
		entry.Kind = EntryResolved
		entry.RelPath = res.RelPath
		entry.Status = statusByPath[res.RelPath]
	case graph.KindAmbiguous:
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
// grammar lets navigation read: every projectable primary group joins the one
// main line end to end, each projectable local group is a component of its own,
// a structural branch carries its descendants and contributes nothing itself,
// and every other branch projects nothing, subtree included.
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
	case g.Carries:
		w.descend(g)
	}
}

// primary folds one main-line branch into the course: every accepted row is a
// planned lesson carrying its position, and the resolved ones are walkable.
func (w *stopWalk) primary(g *PathGroup) {
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			if item.Entry.State != sequence.EntryAccepted {
				continue
			}
			w.main.planned++
			item.Entry.Number = w.main.planned
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
// open, numbered from one on the branch's own count because the declared orders
// never join. A local branch carries no other walkable branch, so a branch
// nested beneath it is drawn but not walked and its rows keep number zero.
func localStops(g *PathGroup) []NoteRef {
	var stops []NoteRef
	n := 0
	for _, item := range g.Items {
		if item.Entry == nil || item.Entry.State != sequence.EntryAccepted {
			continue
		}
		n++
		item.Entry.Number = n
		if item.Entry.Openable() {
			stops = append(stops, NoteRef{Name: item.Entry.Text, RelPath: item.Entry.RelPath})
		}
	}
	return stops
}

// readyLessons counts the lessons at the reviewed status across every branch a
// surface draws, side branches included: a branch outside the course is drawn
// by nobody and counted by nobody.
func readyLessons(groups []*PathGroup) int {
	n := 0
	for _, g := range groups {
		if !g.Drawn() {
			continue
		}
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				if g.Teaches(item.Entry) && item.Entry.Kind == EntryResolved && item.Entry.Status == schema.SealStatus {
					n++
				}
			case item.Group != nil:
				n += readyLessons([]*PathGroup{item.Group})
			}
		}
	}
	return n
}

// pathPlacements records every projectable accepted, resolved entry of one path
// into the reverse index. Nothing else is a course membership.
func pathPlacements(index map[string][]Placement, p *Path) {
	for _, g := range p.Groups {
		placeGroup(index, p.RelPath, g, nil)
	}
}

// placeGroup records one branch's lessons and recurses. A branch the course
// does not include contributes nothing.
func placeGroup(index map[string][]Placement, pathRel string, g *PathGroup, chain []string) {
	if !g.Projectable && !g.Carries {
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
