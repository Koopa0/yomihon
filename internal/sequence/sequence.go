// Package sequence reads the declared structure of a study path: which rows a
// course lists, which branch each row belongs to, what role the author gave
// that branch, and what could not be determined.
//
// It is the one place that answers those questions. Navigation, the judge and
// any later reader consume the same typed interpretation rather than scanning
// the Markdown again, because two scans of one document are two answers to
// "what does this course list" and the disagreement is silent.
//
// Two words carry the whole design and are not interchangeable. A *candidate*
// is a source row this grammar recognizes: a list row holding a live wikilink
// in its own target scope. An *accepted entry* is a candidate that also passed
// canonical validation — its single live link opens the row. Branch state is
// settled from candidates alone, so fixing a malformed row never makes a
// branch change role and report a fresh problem. Whether an accepted entry's
// note exists is the consumer's question, asked later: a lesson that is
// planned but unwritten is still one of the course's lessons.
//
// The grammar is declared, never inferred: a branch takes part in the course
// order only because the author wrote a role marker on it. Where a document
// does not meet the contract, the prose still reads, the projection stops, and
// a diagnostic names the line.
package sequence

import (
	"cmp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// Role is a branch's declared part in the course order.
type Role uint8

const (
	// RoleStructural is a branch that lists no rows of its own — it groups
	// other branches. Declaring nothing on it is correct, so it draws no
	// diagnostic.
	RoleStructural Role = iota
	// RoleUnclassified is a branch that lists rows but was never given a role.
	// It is the one state the author has to resolve; nothing projects from it.
	RoleUnclassified
	// RolePrimary is the course's main line. Consecutive primary groups join
	// end to end in declared order.
	RolePrimary
	// RoleLocal is a side branch: its own order, its own count, hanging from
	// the entry it is nested under and never rejoining the main line.
	RoleLocal
	// RoleNone is an authored answer, not an omission: prose that reads
	// normally and stays out of the course entirely.
	RoleNone
)

// String names a role for a diagnostic message.
func (r Role) String() string {
	switch r {
	case RoleStructural:
		return "structural"
	case RoleUnclassified:
		return "unclassified"
	case RolePrimary:
		return "primary"
	case RoleLocal:
		return "local"
	case RoleNone:
		return "none"
	default:
		panic("sequence: unknown Role")
	}
}

// Declared reports whether the author wrote this role, as opposed to it being
// what remained once nothing was written.
func (r Role) Declared() bool {
	return r == RolePrimary || r == RoleLocal || r == RoleNone
}

// Span is a half-open byte range into the body a document was parsed from,
// always counted from the body's first byte — every Span this package hands
// out or stores is in that one frame, and a range measured from somewhere else
// carries a different type. It is a row's stable identity: two rows naming the
// same note still differ by where they sit in the source, which is what lets a
// side branch say which of them it hangs from.
type Span struct{ Start, Stop int }

func (s Span) contains(off int) bool { return off >= s.Start && off < s.Stop }

// Zero reports whether this span identifies nothing — a heading group's anchor,
// or an orphan's.
func (s Span) Zero() bool { return s == Span{} }

// EntryState is what canonical validation decided about one candidate. Only
// EntryAccepted may become a lesson; every other state keeps the row a row —
// the branch it sits in still lists it — while naming why no entry came of it.
type EntryState uint8

const (
	// EntryAccepted is a canonical row: its single live link is the first
	// visible inline, so the row is the lesson it names.
	EntryAccepted EntryState = iota
	// EntryNoncanonical is a row whose single live link comes after something
	// else — a label, an embed, a same-file anchor, inline code. The author
	// moves the link to the front or takes the row out of the course; choosing
	// for them would be a guess.
	EntryNoncanonical
	// EntryMultiTarget is a row whose target scope names more than one note.
	// Choosing the first would be a guess and taking both would invent a
	// lesson the author did not write.
	EntryMultiTarget
	// EntryRoleOnEntry is a row that is both a lesson and a branch container.
	// Neither the entry nor the branch it opens projects until the author
	// splits them apart.
	EntryRoleOnEntry
)

// String names an entry state for a diagnostic or a consumer's log line.
func (s EntryState) String() string {
	switch s {
	case EntryAccepted:
		return "accepted"
	case EntryNoncanonical:
		return "noncanonical"
	case EntryMultiTarget:
		return "multi-target"
	case EntryRoleOnEntry:
		return "role-on-entry"
	default:
		panic("sequence: unknown EntryState")
	}
}

// Candidate is one source row the grammar recognizes: a list item, ordered or
// unordered, carrying a live wikilink in its own target scope.
//
// Target is the row's single live target — what a consumer resolves — and is
// empty for a multi-target row, where naming one would be a guess. Text is
// what the author wrote to be read. Line is 1-based in the file, so a
// diagnostic and an editor agree. Span is the row's own first text block in
// the body, the stable identity an anchor points at.
type Candidate struct {
	Text   string
	Target string
	Line   int
	Span   Span
	// TargetSpan is where Target is written: the bytes of that one wikilink,
	// from its opening bracket through its closing one. Span says which row
	// this is; TargetSpan says which link in the row the course resolves to,
	// and a row can carry others — an embed, a same-file anchor, a mention in
	// the sentence beside it. A consumer that has to match a link it read
	// itself back to this entry joins on these bytes, because a line holds
	// several links and a name can be written on several rows. It is zero
	// exactly when Target is empty.
	TargetSpan Span
	State      EntryState
}

// Accepted reports whether this row passed canonical validation and is an
// entry. Only accepted entries are counted, walked and placed; every other
// state still makes the row a row for the purpose of its branch's state.
func (c *Candidate) Accepted() bool { return c.State == EntryAccepted }

// Group is one branch of a study path: a heading, or a nested list container
// the author marked with a role.
//
// Name is the author's text with a recognized marker removed — the marker is a
// declaration, not part of what the branch is called. The source bytes are
// untouched.
type Group struct {
	Name string
	// Level is the heading level, 2 through 6. A container group has level 0:
	// its place comes from the list it sits in, not from a heading rank.
	Level int
	Line  int
	Role  Role
	// Container is true for a group opened by a nested list item rather than a
	// heading. A container is a boundary: its rows belong to it alone and are
	// not a continuation of the entry it hangs from.
	Container bool
	// AnchorTarget names the entry a container hangs from, empty for a heading
	// group. It is the resolution target of the enclosing list row, so a reader
	// can be shown where the side branch attaches.
	AnchorTarget string
	// AnchorSpan is the enclosing candidate's Span — the identity behind
	// AnchorTarget, still unique when two rows name the same note. Zero for a
	// heading group and for an orphan.
	AnchorSpan Span
	// Invalid marks a branch carrying a structural error the author has to
	// resolve — a role conflict, an orphaned side branch, one nested too deep,
	// or a container that is also a lesson. An invalid branch keeps its rows
	// and its diagnostics, and nothing projects from it: a consumer reads this
	// field, never re-derives the verdict from the diagnostics.
	Invalid bool
	// Items hold everything this branch lists, in source order: a side branch
	// follows the entry it hangs from because that is where the author wrote
	// it. A consumer never reconstructs this order from targets, lines or two
	// parallel slices.
	Items []Item
}

// Item is one thing a branch holds in source order: a row it lists, or a
// branch opened beneath it. Exactly one field is set.
type Item struct {
	Entry  *Candidate
	Branch *Group
}

// Projectable reports whether navigation may read this branch's accepted
// entries. Only a declared primary or local branch free of structural error
// projects; none, unclassified, structural and invalid branches never do.
// This is the one verdict, stated here so navigation and the judge cannot
// interpret the diagnostics two different ways.
func (g *Group) Projectable() bool {
	return !g.Invalid && (g.Role == RolePrimary || g.Role == RoleLocal)
}

// Entries are the rows this branch lists, in source order.
func (g *Group) Entries() []*Candidate {
	var out []*Candidate
	for _, item := range g.Items {
		if item.Entry != nil {
			out = append(out, item.Entry)
		}
	}
	return out
}

// Subgroups are the branches opened beneath this one, in source order.
func (g *Group) Subgroups() []*Group {
	var out []*Group
	for _, item := range g.Items {
		if item.Branch != nil {
			out = append(out, item.Branch)
		}
	}
	return out
}

// Diagnostic is one thing the author has to decide, addressed to the author.
// Rule is the stable identifier; Line is 1-based in the file.
type Diagnostic struct {
	Rule     string
	Line     int
	Message  string
	Evidence string
}

// The rules this grammar reports. Every one names a decision only the author
// can make: yomihon reports, and a human edits the file.
const (
	RuleRoleMissing        = "path.role_missing"
	RuleRoleDuplicate      = "path.role_duplicate"
	RuleRoleConflict       = "path.role_conflict"
	RuleLocalOrphan        = "path.local_orphan"
	RuleNestingTooDeep     = "path.nesting_too_deep"
	RuleRoleOnEntry        = "path.role_on_entry"
	RuleRoleInvalid        = "path.role_invalid"
	RuleRoleMisplaced      = "path.role_misplaced"
	RuleEntryOutsideBranch = "path.entry_outside_branch"
	RuleEntryMultiTarget   = "path.entry_multi_target"
	RuleEntryNoncanonical  = "path.entry_noncanonical"
)

// Document is a study path's whole interpretation: its branches in document
// order, and everything that could not be determined.
type Document struct {
	Groups      []*Group
	Diagnostics []Diagnostic
}

// mdParser is plain CommonMark, the same dialect the rest of the vault's
// tooling reads, so a fence, a block quote and a nested list are located the
// same way everywhere. Task list items are deliberately not enabled: the
// checkbox is recognized from the row's own text, which keeps "[x]" meaning
// the same thing inside a study path and outside one.
var mdParser = goldmark.New().Parser()

// Parse reads one study path body into its declared structure. bodyStartLine is
// the file line the body begins on, so a note with frontmatter reports the
// lines an editor shows.
//
// The order below is fixed and each step is named, because the split between
// them is what stops one problem from becoming two. Branch state is settled
// from candidates (steps 3-5); whether an accepted entry resolves is the
// consumer's question and is asked afterwards.
func Parse(body string, bodyStartLine int) Document {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))

	p := &parser{
		body:          body,
		bodyStartLine: bodyStartLine,
	}
	p.zones = skipZones(body)
	p.openers = emphasisOpeners(body)
	p.rows = make(map[int]*Candidate)

	// 1. Parse headings, containers and role declarations, collecting the rows
	//    each branch lists on the way.
	p.walkBlocks(doc)
	p.close()

	// 2. Content under a declared none is body-only for navigation: only an
	//    explicitly declared child role is checked for conflict beneath it.
	//
	// 3-5. Settle every branch that carries no declaration: rows and no marker
	//    is unclassified and reportable; no rows and no marker is a branch that
	//    merely groups others, which is correct and silent.
	p.classify(p.roots, false)

	return Document{Groups: p.roots, Diagnostics: p.diagnostics}
}

// parser is the walk's state: the open heading stack, the rows collected so
// far, and the diagnostics raised.
type parser struct {
	body          string
	bodyStartLine int
	zones         []Span
	// openers are the opening delimiter runs of the emphasis Markdown actually
	// paired, so an unpaired run is known to be printed rather than assumed to
	// be markup.
	openers []Span
	// rows is each source row's one decision, keyed by where the row's own text
	// starts. Whether a row is a lesson is settled once, when the row is read;
	// anything that needs the answer later asks here rather than re-deriving it
	// from the text, which is how a row refused for one reason came to pass a
	// second, looser test.
	rows map[int]*Candidate

	open        []*Group
	roots       []*Group
	diagnostics []Diagnostic
}

func (p *parser) report(rule string, line int, message, evidence string) {
	p.diagnostics = append(p.diagnostics, Diagnostic{
		Rule:     rule,
		Line:     line,
		Message:  message,
		Evidence: evidence,
	})
}

// line is the 1-based file line a body offset falls on.
func (p *parser) line(offset int) int {
	if offset < 0 || offset > len(p.body) {
		return p.bodyStartLine
	}
	return p.bodyStartLine + strings.Count(p.body[:offset], "\n")
}

// walkBlocks visits the document's top-level blocks in order. It descends only
// into headings and lists: a block quote, a callout, a code block and a
// paragraph carry prose, and prose is not a course listing however many links
// it holds.
func (p *parser) walkBlocks(doc ast.Node) {
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch node := n.(type) {
		case *ast.Heading:
			p.heading(node)
		case *ast.List:
			p.topLevelList(node)
		default:
			p.reportStrayMarker(n)
		}
	}
}

// heading opens a branch. A marker on an H1 cannot open one — the contract
// places branches at H2 through H6 — so it is reported rather than obeyed.
func (p *parser) heading(node *ast.Heading) {
	rng, ok := linesRange(node)
	if !ok {
		return
	}
	raw := strings.TrimRight(p.body[rng.Start:rng.Stop], "\r\n")
	line := p.line(rng.Start)

	if node.Level < 2 {
		if _, _, decl := readMarker(raw, p.visibleMarkerSpans(raw, rng.Start)); decl != declNone {
			p.report(RuleRoleMisplaced, line,
				"a sequence marker on a level-1 heading declares nothing; a branch is a heading from level 2 to 6",
				strings.TrimSpace(raw))
		}
		return
	}
	name, role, _ := p.declaration(raw, rng.Start, line)

	p.closeTo(node.Level)
	group := &Group{Name: strings.TrimSpace(name), Level: node.Level, Line: line, Role: role}
	p.open = append(p.open, group)
}

// topLevelList reads a list that sits directly under a heading. Rows here
// belong to the open branch; before the first branch there is nothing for them
// to belong to, and the contract does not invent a root to hold them.
func (p *parser) topLevelList(node *ast.List) {
	current := p.current()
	for item := node.FirstChild(); item != nil; item = item.NextSibling() {
		li, ok := item.(*ast.ListItem)
		if !ok {
			continue
		}
		p.listItem(li, current, 0)
	}
}

// listItem reads one row. It is either a container — a row carrying a marker
// with a child list beneath it, opening a branch of its own — or a plain row,
// which is a candidate when its own target scope carries a live wikilink.
//
// localDepth counts the declared local containers already enclosing this row,
// so a side branch inside a side branch is reported rather than nested.
func (p *parser) listItem(item *ast.ListItem, group *Group, localDepth int) {
	spans := p.ownSpans(item)
	child := childList(item)

	own := ""
	head := ""
	line := p.bodyStartLine
	abs := 0
	if len(spans) > 0 {
		abs = spans[0].Start
		own = p.body[spans[0].Start:spans[0].Stop]
		head = firstSourceLine(own)
		line = p.line(abs)
	} else if r, ok := linesRange(item); ok {
		line = p.line(r.Start)
	}

	// A declaration is read on the row's own line. A row may run to several
	// lines, and a marker further down is read by nobody: it names a branch
	// nothing opens, so it is reported rather than obeyed.
	name, role, declared := p.declaration(head, abs, line)
	p.reportContinuationMarkers(spans)
	var hits []linkHit
	if !isTaskRow(own) {
		hits = p.liveWikilinks(spans)
	}

	switch {
	case declared && child == nil:
		p.report(RuleRoleMisplaced, line,
			"a sequence marker on a row with no list beneath it declares nothing; a container declares the child list it opens",
			strings.TrimSpace(own))
		p.plainRow(hits, spans, name, line, own, group)
	case declared:
		p.container(item, child, group, name, role, hits, spans, line, own, localDepth)
	case child != nil:
		// The row's own entry comes first, then whatever its nested list
		// holds, because that is the order the author wrote them in.
		p.plainRow(hits, spans, name, line, own, group)
		p.undeclaredChildList(item, child, group, localDepth)
	default:
		p.plainRow(hits, spans, name, line, own, group)
	}
}

// reportContinuationMarkers names a visible marker written in a row's
// continuation blocks. A declaration lives on the row's own line; a marker in
// the prose that continues the item is read by nobody and must not fail
// silently.
func (p *parser) reportContinuationMarkers(spans []Span) {
	for i, s := range spans {
		segment := p.body[s.Start:s.Stop]
		lines := splitLines(segment)
		if i == 0 {
			// The row's own line is where a declaration belongs; everything
			// after it in the same block is continuation like any other.
			lines = lines[min(len(lines), 1):]
		}
		for _, raw := range lines {
			trimmed := strings.TrimRight(raw.text, "\r\n")
			if len(p.visibleMarkerSpans(trimmed, s.Start+raw.offset)) == 0 {
				continue
			}
			p.report(RuleRoleMisplaced, p.line(s.Start+raw.offset),
				"a sequence marker is read on the row's own line, not in its continuation",
				strings.TrimSpace(trimmed))
		}
	}
}

// container opens a branch from a nested list row. A valid local container is
// a structural edge: it is not itself a row of the course, and the rows beneath
// it belong to it alone.
func (p *parser) container(
	item *ast.ListItem,
	child *ast.List,
	group *Group,
	name string,
	role Role,
	hits []linkHit,
	spans []Span,
	line int,
	own string,
	localDepth int,
) {
	invalid := false
	if len(hits) > 0 {
		p.report(RuleRoleOnEntry, line,
			"a row cannot be both a lesson and a branch heading; give the branch its own row above the list it opens",
			strings.TrimSpace(own))
		invalid = true
		// The row is still a row the branch lists — its state must not read as
		// "merely structural" — but neither the entry nor the branch it opens
		// projects until the author splits them.
		if group != nil {
			entry := &Candidate{
				Text:  strings.TrimSpace(name),
				Line:  line,
				Span:  spans[0],
				State: EntryRoleOnEntry,
			}
			// A row that names one note resolves to it, and says where that
			// name is written; a row that names several says neither, because
			// picking one would be the guess the grammar refuses.
			if len(hits) == 1 {
				entry.Target = hits[0].target
				entry.TargetSpan = hits[0].span()
			}
			p.rows[spans[0].Start] = entry
			group.Items = append(group.Items, Item{Entry: entry})
		}
	}
	if role == RoleLocal && localDepth > 0 {
		p.report(RuleNestingTooDeep, line,
			"a side branch inside a side branch has no place in the course order; keep it to one level below what it hangs from",
			strings.TrimSpace(own))
		invalid = true
	}
	anchor := ""
	var anchorSpan Span
	if role == RoleLocal {
		target, span, ok := p.anchorTarget(item)
		if !ok {
			p.report(RuleLocalOrphan, line,
				"a side branch has nothing to hang from; nest it under the lesson it belongs to",
				strings.TrimSpace(own))
			invalid = true
		} else {
			anchor, anchorSpan = target, span
		}
	}

	opened := &Group{
		Name:         strings.TrimSpace(name),
		Line:         line,
		Role:         role,
		Container:    true,
		AnchorTarget: anchor,
		AnchorSpan:   anchorSpan,
		Invalid:      invalid,
	}
	nextDepth := localDepth
	if role == RoleLocal {
		nextDepth++
	}
	p.readChildren(child, opened, nextDepth)
	if group == nil {
		p.report(RuleEntryOutsideBranch, line,
			"a branch declared before the first level-2 heading belongs to no part of the course",
			strings.TrimSpace(own))
		return
	}
	group.Items = append(group.Items, Item{Branch: opened})
}

// undeclaredChildList reads a nested list that the row above it did not
// declare. The list is not a branch in itself: a declared container inside it
// opens its own branch and attaches where the row above sits, exactly as the
// contract says — a container's parent is the enclosing list item structurally.
//
// What remains — rows nobody declared — is held in one branch of its own so
// nothing projects from it and the author is told once, on the first such row.
// The source stays exactly as written: nothing is flattened into the enclosing
// branch, and nothing is guessed. A nested list holding only declared
// containers leaves nothing behind and is silent, which is what the contract's
// own example depends on.
func (p *parser) undeclaredChildList(
	item *ast.ListItem,
	child *ast.List,
	group *Group,
	localDepth int,
) {
	rest := &Group{Container: true}
	if target, span, ok := p.anchorOwnTarget(item); ok {
		rest.AnchorTarget, rest.AnchorSpan = target, span
	}
	rows, firstRowLine := 0, 0
	for sub := child.FirstChild(); sub != nil; sub = sub.NextSibling() {
		li, ok := sub.(*ast.ListItem)
		if !ok {
			continue
		}
		if p.declaresContainer(li) {
			p.listItem(li, group, localDepth)
			continue
		}
		rows++
		if firstRowLine == 0 {
			firstRowLine = p.rowLine(li)
		}
		p.listItem(li, rest, localDepth)
	}
	// A nested list whose every row declared its own branch leaves nothing
	// behind, which is what the contract's worked example depends on. One that
	// left any row behind is nesting nobody explained — and it stays, rows or
	// no rows, because a list that quietly disappeared is the flattening the
	// contract refuses.
	if rows == 0 {
		return
	}
	rest.Line = cmp.Or(firstLine(rest), firstRowLine)
	if group == nil {
		p.report(RuleEntryOutsideBranch, rest.Line,
			"rows nested before the first level-2 heading belong to no part of the course",
			"")
		return
	}
	group.Items = append(group.Items, Item{Branch: rest})
}

// declaresContainer reports whether a row opens a branch of its own: a readable
// marker with a list beneath it.
func (p *parser) declaresContainer(item *ast.ListItem) bool {
	spans := p.ownSpans(item)
	if len(spans) == 0 {
		return false
	}
	// The same boundary listItem reads. Two readers of one row that disagree
	// about where its declaration lives route the nested list one way and name
	// it the other.
	head := firstSourceLine(p.body[spans[0].Start:spans[0].Stop])
	if _, _, decl := readMarker(head, p.visibleMarkerSpans(head, spans[0].Start)); decl != declValid {
		return false
	}
	return childList(item) != nil
}

// firstLine is the earliest source line a branch holds, so an undeclared group
// is reported where its first row sits rather than where its parent does.
func firstLine(g *Group) int {
	line := 0
	consider := func(at int) {
		if at > 0 && (line == 0 || at < line) {
			line = at
		}
	}
	for _, item := range g.Items {
		switch {
		case item.Entry != nil:
			consider(item.Entry.Line)
		case item.Branch != nil:
			consider(firstLine(item.Branch))
		}
	}
	return line
}

// readChildren reads a container's own list into it.
func (p *parser) readChildren(child *ast.List, into *Group, localDepth int) {
	for sub := child.FirstChild(); sub != nil; sub = sub.NextSibling() {
		li, ok := sub.(*ast.ListItem)
		if !ok {
			continue
		}
		p.listItem(li, into, localDepth)
	}
}

// plainRow records an ordinary row against its branch. Every row with a live
// link in its target scope is a candidate — the branch it sits in has rows —
// but only a canonical one is accepted: a row naming two notes and a row whose
// link is not first both stay rows, each under its own rule, because choosing
// for the author would be a guess.
func (p *parser) plainRow(hits []linkHit, spans []Span, name string, line int, own string, group *Group) {
	if len(hits) == 0 {
		return
	}
	if group == nil {
		p.report(RuleEntryOutsideBranch, line,
			"a lesson listed before the first level-2 heading belongs to no part of the course",
			strings.TrimSpace(own))
		return
	}
	entry := &Candidate{Line: line, Span: spans[0]}
	switch {
	case len(hits) > 1:
		p.report(RuleEntryMultiTarget, line,
			"a row naming more than one note does not say which lesson it is; give each lesson its own row",
			strings.TrimSpace(own))
		// The row still counts as a row for the branch's state, which is why it
		// is recorded with no target rather than dropped.
		entry.Text = strings.TrimSpace(name)
		entry.State = EntryMultiTarget
	case !p.linkFirst(hits[0], spans[0]):
		p.report(RuleEntryNoncanonical, line,
			"a lesson row opens with its link; move the link to the front, or take the row out of the course",
			strings.TrimSpace(own))
		entry.Text = hits[0].display
		entry.Target = hits[0].target
		entry.TargetSpan = hits[0].span()
		entry.State = EntryNoncanonical
	default:
		entry.Text = hits[0].display
		entry.Target = hits[0].target
		entry.TargetSpan = hits[0].span()
		entry.State = EntryAccepted
	}
	p.rows[spans[0].Start] = entry
	group.Items = append(group.Items, Item{Entry: entry})
}

// linkFirst reports whether a hit opens its row: it sits in the row's first
// text block and is the first thing there a reader sees.
//
// "Sees" is the whole question, so it is answered from what renders rather than
// from the bytes. An Obsidian comment renders as nothing and is stepped over; a
// run of asterisks or underscores is stepped over only when it actually opens
// emphasis, which is when a non-space follows it — "**[[A]]**" wraps the link,
// while "__ [[A]]" prints two underscores in front of it.
func (p *parser) linkFirst(hit linkHit, first Span) bool {
	if !first.contains(hit.start) {
		return false
	}
	at, ok := p.firstVisible(first)
	return ok && at == hit.start
}

// firstVisible is the offset of the first thing in a span a reader actually
// sees, or false when the span shows nothing at all.
func (p *parser) firstVisible(span Span) (int, bool) {
	for i := span.Start; i < span.Stop; {
		if zone, inside := p.zoneAt(i); inside {
			i = zone.Stop
			continue
		}
		switch b := p.body[i]; b {
		case ' ', '\t', '\r', '\n':
			i++
		case '*', '_':
			opener, paired := p.openerAt(i)
			if paired {
				i = opener.Stop // Markdown paired this run; what it wraps is what shows
				continue
			}
			return i, true // an unpaired delimiter, printed as itself
		default:
			return i, true
		}
	}
	return 0, false
}

// zoneAt is the invisible or quoted zone covering an offset. A comment shows
// nothing; a code span shows its content but not as prose, and its opening
// backtick is outside the zone, so a row beginning with code is seen to begin
// with code.
func (p *parser) zoneAt(off int) (Span, bool) {
	for _, z := range p.zones {
		if z.contains(off) {
			return z, true
		}
	}
	return Span{}, false
}

// openerAt is the opening delimiter run of an emphasis that starts at this
// offset. Whether a run of asterisks is emphasis or two printed characters is
// Markdown's answer, not a shape this package can read off the run itself: the
// same "**" opens emphasis when something closes it and prints as itself when
// nothing does.
func (p *parser) openerAt(off int) (Span, bool) {
	for _, span := range p.openers {
		if span.Start == off {
			return span, true
		}
	}
	return Span{}, false
}

// anchorTarget is the row a container hangs from: the enclosing list item's
// own single live link, with that row's identity. ok is false when the
// container sits at the top of a list, with no row above it to attach to.
func (p *parser) anchorTarget(item *ast.ListItem) (string, Span, bool) {
	list, ok := item.Parent().(*ast.List)
	if !ok {
		return "", Span{}, false
	}
	parent, ok := list.Parent().(*ast.ListItem)
	if !ok {
		return "", Span{}, false
	}
	return p.anchorOwnTarget(parent)
}

// anchorOwnTarget is a row's own lesson and identity, and only when the row is
// one. A side branch hangs from a lesson; a row the grammar refused is not a
// lesson however it reads, so a task row, a row naming two notes and a row
// whose link is not first all leave a branch beneath them with nothing to hang
// from. Accepting one would give the reader a side branch attached to something
// the course does not contain.
func (p *parser) anchorOwnTarget(item *ast.ListItem) (string, Span, bool) {
	spans := p.ownSpans(item)
	if len(spans) == 0 {
		return "", Span{}, false
	}
	entry, read := p.rows[spans[0].Start]
	if !read || entry.State != EntryAccepted {
		return "", Span{}, false
	}
	return entry.Target, spans[0], true
}

// classify settles every branch the author did not declare, and reports a
// declared role that contradicts the branch it sits under.
//
// underNone carries the one inherited fact the contract names: beneath a
// declared none, rows are body-only, so an undeclared branch there is not a
// missing declaration — but a branch that explicitly declares itself part of
// the course under one that declares itself out of it is a contradiction the
// author has to resolve, and nothing projects from it until they do.
func (p *parser) classify(groups []*Group, underNone bool) {
	for _, g := range groups {
		switch {
		case underNone && g.Role.Declared() && g.Role != RoleNone:
			p.report(RuleRoleConflict, g.Line,
				"a branch cannot take part in the course while the branch above it declares itself out of it",
				g.Name)
			g.Invalid = true
		case g.Role.Declared():
		case underNone:
			// Body-only: nothing to declare and nothing to report.
		case g.Container:
			// Nesting the author never declared. Saying it "lists lessons"
			// would send them looking for lessons that may not be there; what
			// is true is that the list is nested and unexplained.
			g.Role = RoleUnclassified
			p.report(RuleRoleMissing, g.Line,
				"this nested list never says what part it plays; declare {sequence=local} on the row that opens it, or unnest it",
				"a nested list carrying no declaration")
		case len(g.Entries()) > 0:
			g.Role = RoleUnclassified
			p.report(RuleRoleMissing, g.Line,
				"this branch lists lessons but never says what part it plays; declare {sequence=primary}, {sequence=local} or {sequence=none}",
				g.Name)
		default:
			g.Role = RoleStructural
		}
		p.classify(g.Subgroups(), underNone || g.Role == RoleNone)
	}
}

// current is the branch a row read right now belongs to, or nil before the
// first heading opens one.
func (p *parser) current() *Group {
	if len(p.open) == 0 {
		return nil
	}
	return p.open[len(p.open)-1]
}

// closeTo closes every open branch at or below the given heading level, so a
// new heading attaches where its rank says.
func (p *parser) closeTo(level int) {
	for len(p.open) > 0 && p.open[len(p.open)-1].Level >= level {
		p.pop()
	}
}

// close closes every branch still open at the end of the document.
func (p *parser) close() {
	for len(p.open) > 0 {
		p.pop()
	}
}

func (p *parser) pop() {
	last := p.open[len(p.open)-1]
	p.open = p.open[:len(p.open)-1]
	if len(p.open) == 0 {
		p.roots = append(p.roots, last)
		return
	}
	parent := p.open[len(p.open)-1]
	parent.Items = append(parent.Items, Item{Branch: last})
}

// visibleMarkerSpans locates the marker spans on a line that are actually
// written: one inside code or an Obsidian comment is quoted or switched off,
// so it neither declares nor draws a report, while a real marker on the same
// line still does. lineStart is the line's own start offset in the body, and
// adding it here is the one place the two frames meet — the zones are the
// body's, the spans are the line's, and the returned spans stay the line's
// because that is the only frame the line's own text can be sliced with.
func (p *parser) visibleMarkerSpans(raw string, lineStart int) []lineSpan {
	var out []lineSpan
	for _, s := range markerSpans(raw) {
		if inAnyZone(p.zones, lineStart+s.Start) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// reportStrayMarker names a well-formed marker written where no branch can read
// it — in a paragraph, a quote, or any block that is not a heading or a list.
//
// A marker inside code or an Obsidian comment is not written, it is quoted or
// switched off: a note that documents this syntax has to be able to show it.
// The same zones that keep a quoted [[link]] out of the course keep a quoted
// marker out of the report.
func (p *parser) reportStrayMarker(n ast.Node) {
	rng, ok := linesRange(n)
	if !ok {
		// A container such as a quote holds no lines of its own; its children
		// do. Descending is what lets the promise above hold for a marker
		// written inside one. Only on this branch, and only into blocks: a
		// node that does carry lines has already reported them, and walking
		// into a paragraph's inlines would report the same line a second time.
		if n.Type() == ast.TypeBlock {
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				p.reportStrayMarker(c)
			}
		}
		return
	}
	for _, raw := range splitLines(p.body[rng.Start:rng.Stop]) {
		trimmed := strings.TrimRight(raw.text, "\r\n")
		if len(p.visibleMarkerSpans(trimmed, rng.Start+raw.offset)) == 0 {
			continue
		}
		p.report(RuleRoleMisplaced, p.line(rng.Start+raw.offset),
			"a sequence marker here declares nothing; it is read only on a heading or on a row that opens a list",
			strings.TrimSpace(trimmed))
	}
}

// declaration reads a role marker off a source line, reporting a marker that is
// present but unreadable. The returned name is the text with the marker
// removed; what is displayed never includes the declaration. abs is the line's
// start offset in the body, so a marker being quoted in code is recognized as
// a quotation.
//
// declared is true only for a marker that was actually read. An unreadable one
// declares nothing — the branch stays undeclared and is settled like any other
// — so the author gets the reason it failed rather than a role they did not
// write.
func (p *parser) declaration(raw string, abs, line int) (name string, role Role, declared bool) {
	name, role, decl := readMarker(raw, p.visibleMarkerSpans(raw, abs))
	switch decl {
	case declInvalid:
		p.report(RuleRoleInvalid, line,
			"a sequence marker takes exactly one of primary, local or none",
			strings.TrimSpace(raw))
	case declDuplicate:
		p.report(RuleRoleDuplicate, line,
			"a branch declares one role; this line declares more than one",
			strings.TrimSpace(raw))
	case declMisplaced:
		p.report(RuleRoleMisplaced, line,
			"a sequence marker is read only at the end of the line it declares",
			strings.TrimSpace(raw))
	case declNone, declValid:
	}
	return name, role, decl == declValid
}

// ownSpans is a list row's own text — every block it says itself — as source
// ranges in order, skipping any list nested beneath it. The scope deliberately
// continues past a nested list: a loose item's paragraph after its child list
// is still the item's own words, and a second live link there still makes the
// row multi-target. The nested list subtrees themselves are never the row's
// text — a declared container is a boundary, and an undeclared one is its own
// undeclared branch.
func (p *parser) ownSpans(item *ast.ListItem) []Span {
	var spans []Span
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		if _, nested := c.(*ast.List); nested {
			continue
		}
		if r, ok := linesRange(c); ok {
			spans = append(spans, r)
		}
	}
	return spans
}

// linkHit is one live wikilink found in a row's target scope, with the
// absolute bounds of its own bytes in the body. start is the fact canonical
// validation reads; the pair is the occurrence identity an accepted row keeps,
// so a consumer can match its own reading of the same link back to this row.
type linkHit struct {
	target  string
	display string
	start   int
	stop    int
}

// span is where this link is written, as the occurrence identity a row keeps.
func (h linkHit) span() Span { return Span{Start: h.start, Stop: h.stop} }

// liveWikilinks are the wikilinks in a row's target scope that actually
// address another note: an embed shows a note rather than listing it, a
// same-file anchor names no note, and a link inside code or an Obsidian
// comment is quoted or switched off rather than written.
func (p *parser) liveWikilinks(spans []Span) []linkHit {
	var out []linkHit
	for _, rng := range spans {
		out = append(out, p.linksIn(rng)...)
	}
	return out
}

// linksIn scans one source range for live wikilinks.
func (p *parser) linksIn(rng Span) []linkHit {
	if rng.Stop <= rng.Start {
		return nil
	}
	var out []linkHit
	segment := p.body[rng.Start:rng.Stop]
	for i := 0; ; {
		rel := strings.Index(segment[i:], "[[")
		if rel < 0 {
			break
		}
		open := i + rel
		relEnd := strings.Index(segment[open+2:], "]]")
		if relEnd < 0 {
			break
		}
		inner := segment[open+2 : open+2+relEnd]
		next := open + 2 + relEnd + 2
		i = next
		if strings.Contains(inner, "\n") {
			continue
		}
		absolute := rng.Start + open
		if inAnyZone(p.zones, absolute) {
			continue
		}
		if open > 0 && segment[open-1] == '!' {
			continue // an embed shows the note; it does not list it
		}
		if graph.EscapedWikilinkAt(p.body, absolute) {
			// A backslash before the brackets prints them: the renderer and
			// the judge both read it as text, and every reader of this vault
			// has to agree about that. Counting it as a link here made an
			// author who escaped one accidentally disqualify the row beside
			// it, which reads as a lesson quietly leaving the course.
			continue
		}
		target, display, ok := graph.SplitWikilink(inner)
		if !ok {
			continue
		}
		out = append(out, linkHit{target: target, display: display, start: absolute, stop: rng.Start + next})
	}
	return out
}

// childList is the list nested directly under a row, or nil.
func childList(item *ast.ListItem) *ast.List {
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		if list, ok := c.(*ast.List); ok {
			return list
		}
	}
	return nil
}

func inAnyZone(zones []Span, off int) bool {
	for _, z := range zones {
		if z.contains(off) {
			return true
		}
	}
	return false
}

// skipZones are the byte ranges whose brackets are not live links: code blocks
// and code spans, where a link is being quoted, and Obsidian comments, where it
// has been switched off.
func skipZones(body string) []Span {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))
	var code []Span
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never fails, so the walk cannot
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			if r, ok := linesRange(node); ok {
				code = append(code, r)
			}
		case *ast.CodeSpan:
			if r, ok := inlineRange(node); ok {
				code = append(code, r)
			}
		}
		return ast.WalkContinue, nil
	})
	return append(code, commentZones(body, code)...)
}

// emphasisOpeners are the opening delimiter runs of every emphasis in the body.
// Markdown decides what an asterisk run is by finding a closer for it, so the
// runs it paired are the ones that vanish into markup; every other run prints.
func emphasisOpeners(body string) []Span {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))
	var out []Span
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never fails, so the walk cannot
		if !entering {
			return ast.WalkContinue, nil
		}
		emphasis, ok := n.(*ast.Emphasis)
		if !ok {
			return ast.WalkContinue, nil
		}
		if start, content, ok := emphasisOpener(emphasis); ok {
			out = append(out, Span{start, content})
		}
		return ast.WalkContinue, nil
	})
	return out
}

// emphasisOpener is where one emphasis's own delimiter run begins and ends.
// Emphasis carries no source segment of its own, so the run is measured back
// from what it wraps — and what it wraps may be another emphasis, whose own run
// has to be stepped over first. Bold and italic together is a shape the
// contract allows, and measuring back from the innermost text would land in the
// middle of the delimiters.
func emphasisOpener(n *ast.Emphasis) (start, content int, ok bool) {
	child := n.FirstChild()
	if child == nil {
		return 0, 0, false
	}
	if inner, isEmphasis := child.(*ast.Emphasis); isEmphasis {
		innerStart, _, innerOK := emphasisOpener(inner)
		if !innerOK {
			return 0, 0, false
		}
		content = innerStart
	} else {
		at, found := firstTextStart(child)
		if !found {
			return 0, 0, false
		}
		content = at
	}
	start = content - n.Level
	if start < 0 {
		return 0, 0, false
	}
	return start, content, true
}

// firstTextStart is where an inline node's own text begins in the source.
func firstTextStart(n ast.Node) (int, bool) {
	if t, ok := n.(*ast.Text); ok {
		return t.Segment.Start, true
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if off, ok := firstTextStart(c); ok {
			return off, true
		}
	}
	return 0, false
}

// commentZones are the Obsidian %%...%% spans. A %% inside code is ignored
// first so it cannot shift the pairing of real comments; an unpaired trailing
// mark is dropped.
func commentZones(body string, code []Span) []Span {
	var marks []int
	for off := 0; ; {
		rel := strings.Index(body[off:], "%%")
		if rel < 0 {
			break
		}
		at := off + rel
		if !inAnyZone(code, at) {
			marks = append(marks, at)
		}
		off = at + 2
	}
	var zones []Span
	for k := 0; k+1 < len(marks); k += 2 {
		zones = append(zones, Span{marks[k], marks[k+1] + 2})
	}
	return zones
}

// linesRange is a block node's source span.
func linesRange(n ast.Node) (Span, bool) {
	ls := n.Lines()
	if ls == nil || ls.Len() == 0 {
		return Span{}, false
	}
	return Span{ls.At(0).Start, ls.At(ls.Len() - 1).Stop}, true
}

// inlineRange covers an inline node's text children.
func inlineRange(n ast.Node) (Span, bool) {
	start, stop, found := 0, 0, false
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			continue
		}
		if !found || t.Segment.Start < start {
			start = t.Segment.Start
		}
		if !found || t.Segment.Stop > stop {
			stop = t.Segment.Stop
		}
		found = true
	}
	if !found {
		return Span{}, false
	}
	return Span{start, stop}, true
}

// sourceLine is one line of a block with its offset from the block's start.
type sourceLine struct {
	text   string
	offset int
}

func splitLines(s string) []sourceLine {
	var out []sourceLine
	off := 0
	for off < len(s) {
		i := strings.IndexByte(s[off:], '\n')
		if i < 0 {
			out = append(out, sourceLine{text: s[off:], offset: off})
			break
		}
		out = append(out, sourceLine{text: s[off : off+i], offset: off})
		off += i + 1
	}
	return out
}

// firstSourceLine is a block's own first physical line.
func firstSourceLine(block string) string {
	head, _, _ := strings.Cut(block, "\n")
	return strings.TrimRight(head, "\r")
}

// rowLine is the file line a list row starts on.
func (p *parser) rowLine(item *ast.ListItem) int {
	if spans := p.ownSpans(item); len(spans) > 0 {
		return p.line(spans[0].Start)
	}
	if r, ok := linesRange(item); ok {
		return p.line(r.Start)
	}
	return p.bodyStartLine
}
