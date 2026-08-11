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
// is a source row this grammar recognizes. An *accepted entry* is a candidate
// that also passed the consumer's canonical validation — resolving the link
// against the vault. Branch state is settled from candidates alone, so fixing a
// malformed link never makes a branch change role and report a fresh problem.
//
// The grammar is declared, never inferred: a branch takes part in the course
// order only because the author wrote a role marker on it. Where a document
// does not meet the contract, the prose still reads, the projection stops, and
// a diagnostic names the line.
package sequence

import (
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

// Candidate is one source row the grammar recognizes: a list item, ordered or
// unordered, carrying exactly one live wikilink of its own.
//
// Target is what a consumer resolves; Text is what the author wrote to be read.
// Line is 1-based in the file, so a diagnostic and an editor agree.
type Candidate struct {
	Text   string
	Target string
	Line   int
}

// Accepted reports whether this row passed the grammar's own validation and may
// become an entry. A row the grammar recognized but could not read as one
// lesson — it named more than one note — is a row for the purpose of a branch's
// state and never a lesson for the purpose of counting or walking the course.
//
// Whether the named note exists is a separate question, asked later: a lesson
// that is planned but unwritten is still one of the course's lessons.
func (c Candidate) Accepted() bool { return c.Target != "" }

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
	Candidates   []Candidate
	Groups       []Group
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
)

// Document is a study path's whole interpretation: its branches in document
// order, and everything that could not be determined.
type Document struct {
	Groups      []Group
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
// from candidates (steps 3-5); whether a candidate resolves is the consumer's
// question and is asked afterwards.
func Parse(body string, bodyStartLine int) Document {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))

	p := &parser{
		src:           src,
		body:          body,
		bodyStartLine: bodyStartLine,
	}
	p.zones = skipZones(body)

	// 1. Parse headings, containers and role declarations, collecting the rows
	//    each branch lists on the way.
	p.walkBlocks(doc)

	groups := p.close(0)

	// 2. Content under a declared none is body-only for navigation: only an
	//    explicitly declared child role is checked for conflict beneath it.
	//
	// 3-5. Settle every branch that carries no declaration: rows and no marker
	//    is unclassified and reportable; no rows and no marker is a branch that
	//    merely groups others, which is correct and silent.
	p.classify(groups, false)

	return Document{Groups: groups, Diagnostics: p.diagnostics}
}

// parser is the walk's state: the open heading stack, the rows collected so
// far, and the diagnostics raised.
type parser struct {
	src           []byte
	body          string
	bodyStartLine int
	zones         []byteRange

	open        []*Group
	roots       []Group
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
	raw := strings.TrimRight(p.body[rng.start:rng.stop], "\r\n")
	line := p.line(rng.start)

	if node.Level < 2 {
		if _, _, decl := parseMarker(raw); decl != declNone {
			p.report(RuleRoleMisplaced, line,
				"a sequence marker on a level-1 heading declares nothing; a branch is a heading from level 2 to 6",
				strings.TrimSpace(raw))
		}
		return
	}
	name, role, _ := p.declaration(raw, line)

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
// which is a candidate when it carries exactly one live wikilink.
//
// localDepth counts the declared local containers already enclosing this row,
// so a side branch inside a side branch is reported rather than nested.
func (p *parser) listItem(item *ast.ListItem, group *Group, localDepth int) {
	own, ownRange, hasOwn := p.ownText(item)
	child := childList(item)
	line := p.bodyStartLine
	if hasOwn {
		line = p.line(ownRange.start)
	} else if r, ok := linesRange(item); ok {
		line = p.line(r.start)
	}

	name, role, declared := p.declaration(own, line)
	var links []Candidate
	if !isTaskRow(own) {
		links = p.liveWikilinks(ownRange)
	}

	switch {
	case declared && child == nil:
		p.report(RuleRoleMisplaced, line,
			"a sequence marker on a row with no list beneath it declares nothing; a container declares the child list it opens",
			strings.TrimSpace(own))
		p.plainRow(links, name, line, own, group)
	case declared:
		p.container(item, child, group, name, role, links, line, own, localDepth)
	case child != nil:
		// An undeclared nested list is its own branch. It projects nothing
		// until the author says what it is, and the rows stay where they were
		// written.
		p.undeclaredChildList(item, child, group, localDepth)
		p.plainRow(links, name, line, own, group)
	default:
		p.plainRow(links, name, line, own, group)
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
	links []Candidate,
	line int,
	own string,
	localDepth int,
) {
	if len(links) > 0 {
		p.report(RuleRoleOnEntry, line,
			"a row cannot be both a lesson and a branch heading; give the branch its own row above the list it opens",
			strings.TrimSpace(own))
	}
	if role == RoleLocal && localDepth > 0 {
		p.report(RuleNestingTooDeep, line,
			"a side branch inside a side branch has no place in the course order; keep it to one level below what it hangs from",
			strings.TrimSpace(own))
	}
	anchor := ""
	if role == RoleLocal {
		var ok bool
		anchor, ok = p.anchorTarget(item)
		if !ok {
			p.report(RuleLocalOrphan, line,
				"a side branch has nothing to hang from; nest it under the lesson it belongs to",
				strings.TrimSpace(own))
		}
	}

	opened := Group{
		Name:         strings.TrimSpace(name),
		Line:         line,
		Role:         role,
		Container:    true,
		AnchorTarget: anchor,
	}
	nextDepth := localDepth
	if role == RoleLocal {
		nextDepth++
	}
	p.readChildren(child, &opened, nextDepth)
	if group == nil {
		p.report(RuleEntryOutsideBranch, line,
			"a branch declared before the first level-2 heading belongs to no part of the course",
			strings.TrimSpace(own))
		return
	}
	group.Groups = append(group.Groups, opened)
}

// undeclaredChildList reads a nested list that the row above it did not
// declare. The list is not a branch in itself: a declared container inside it
// opens its own branch and attaches where the row above sits, exactly as the
// contract says — a container's parent is the enclosing list item structurally.
//
// What remains — rows nobody declared — is held in one branch of its own so
// nothing projects from it and the author is told once, on the first such row.
// A nested list holding only declared containers leaves nothing behind and is
// silent, which is what the contract's own example depends on.
func (p *parser) undeclaredChildList(
	item *ast.ListItem,
	child *ast.List,
	group *Group,
	localDepth int,
) {
	rest := Group{Container: true}
	if anchor, ok := p.anchorOwnTarget(item); ok {
		rest.AnchorTarget = anchor
	}
	for sub := child.FirstChild(); sub != nil; sub = sub.NextSibling() {
		li, ok := sub.(*ast.ListItem)
		if !ok {
			continue
		}
		if p.declaresContainer(li) {
			p.listItem(li, group, localDepth)
			continue
		}
		p.listItem(li, &rest, localDepth)
	}
	if len(rest.Candidates) == 0 && len(rest.Groups) == 0 {
		return
	}
	rest.Line = p.firstLine(&rest)
	if group == nil {
		p.report(RuleEntryOutsideBranch, rest.Line,
			"rows nested before the first level-2 heading belong to no part of the course",
			"")
		return
	}
	group.Groups = append(group.Groups, rest)
}

// declaresContainer reports whether a row opens a branch of its own: a readable
// marker with a list beneath it.
func (p *parser) declaresContainer(item *ast.ListItem) bool {
	own, _, ok := p.ownText(item)
	if !ok {
		return false
	}
	if _, _, decl := parseMarker(own); decl != declValid {
		return false
	}
	return childList(item) != nil
}

// firstLine is the earliest source line a branch holds, so an undeclared group
// is reported where its first row sits rather than where its parent does.
func (p *parser) firstLine(g *Group) int {
	line := 0
	consider := func(at int) {
		if at > 0 && (line == 0 || at < line) {
			line = at
		}
	}
	for _, c := range g.Candidates {
		consider(c.Line)
	}
	for i := range g.Groups {
		consider(p.firstLine(&g.Groups[i]))
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

// plainRow records an ordinary row against its branch. A row with two live
// targets is a candidate — it is a row the grammar recognizes, so the branch it
// sits in has rows — but it never becomes an entry: choosing the first would be
// a guess and taking both would invent a lesson the author did not write.
func (p *parser) plainRow(links []Candidate, name string, line int, own string, group *Group) {
	if len(links) == 0 {
		return
	}
	if group == nil {
		p.report(RuleEntryOutsideBranch, line,
			"a lesson listed before the first level-2 heading belongs to no part of the course",
			strings.TrimSpace(own))
		return
	}
	if len(links) > 1 {
		p.report(RuleEntryMultiTarget, line,
			"a row naming more than one note does not say which lesson it is; give each lesson its own row",
			strings.TrimSpace(own))
		// The row still counts as a row for the branch's state, which is why it
		// is recorded with no target rather than dropped.
		group.Candidates = append(group.Candidates, Candidate{Text: strings.TrimSpace(name), Line: line})
		return
	}
	only := links[0]
	only.Line = line
	group.Candidates = append(group.Candidates, only)
}

// anchorTarget is the resolution target of the row a container hangs from: the
// enclosing list item's own link. ok is false when the container sits at the
// top of a list, with no row above it to attach to.
func (p *parser) anchorTarget(item *ast.ListItem) (string, bool) {
	list, ok := item.Parent().(*ast.List)
	if !ok {
		return "", false
	}
	parent, ok := list.Parent().(*ast.ListItem)
	if !ok {
		return "", false
	}
	return p.anchorOwnTarget(parent)
}

// anchorOwnTarget is a row's own single resolution target.
func (p *parser) anchorOwnTarget(item *ast.ListItem) (string, bool) {
	_, rng, ok := p.ownText(item)
	if !ok {
		return "", false
	}
	links := p.liveWikilinks(rng)
	if len(links) != 1 {
		return "", false
	}
	return links[0].Target, true
}

// classify settles every branch the author did not declare, and reports a
// declared role that contradicts the branch it sits under.
//
// underNone carries the one inherited fact the contract names: beneath a
// declared none, rows are body-only, so an undeclared branch there is not a
// missing declaration — but a branch that explicitly declares itself part of
// the course under one that declares itself out of it is a contradiction the
// author has to resolve.
func (p *parser) classify(groups []Group, underNone bool) {
	for i := range groups {
		g := &groups[i]
		switch {
		case underNone && g.Role.Declared() && g.Role != RoleNone:
			p.report(RuleRoleConflict, g.Line,
				"a branch cannot take part in the course while the branch above it declares itself out of it",
				g.Name)
		case g.Role.Declared():
		case underNone:
			// Body-only: nothing to declare and nothing to report.
		case len(g.Candidates) > 0:
			g.Role = RoleUnclassified
			p.report(RuleRoleMissing, g.Line,
				"this branch lists lessons but never says what part it plays; declare {sequence=primary}, {sequence=local} or {sequence=none}",
				g.Name)
		default:
			g.Role = RoleStructural
		}
		p.classify(g.Groups, underNone || g.Role == RoleNone)
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

// close closes the remaining open branches and returns the document's roots.
func (p *parser) close(level int) []Group {
	p.closeTo(level + 1)
	for len(p.open) > 0 {
		p.pop()
	}
	return p.roots
}

func (p *parser) pop() {
	last := p.open[len(p.open)-1]
	p.open = p.open[:len(p.open)-1]
	if len(p.open) == 0 {
		p.roots = append(p.roots, *last)
		return
	}
	parent := p.open[len(p.open)-1]
	parent.Groups = append(parent.Groups, *last)
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
		return
	}
	for _, raw := range splitLines(p.body[rng.start:rng.stop]) {
		trimmed := strings.TrimRight(raw.text, "\r\n")
		spans := markerSpans(trimmed)
		if len(spans) == 0 {
			continue
		}
		if _, _, decl := parseMarker(trimmed); decl == declNone {
			continue
		}
		if inAnyZone(p.zones, rng.start+raw.offset+spans[0].start) {
			continue
		}
		p.report(RuleRoleMisplaced, p.line(rng.start+raw.offset),
			"a sequence marker here declares nothing; it is read only on a heading or on a row that opens a list",
			strings.TrimSpace(trimmed))
	}
}

// declaration reads a role marker off a source line, reporting a marker that is
// present but unreadable. The returned name is the text with the marker
// removed; what is displayed never includes the declaration.
//
// declared is true only for a marker that was actually read. An unreadable one
// declares nothing — the branch stays undeclared and is settled like any other
// — so the author gets the reason it failed rather than a role they did not
// write.
func (p *parser) declaration(raw string, line int) (name string, role Role, declared bool) {
	name, role, decl := parseMarker(raw)
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

// ownText is a list row's own text — everything it says before any list nested
// beneath it — as a source range, so the rows of a child list are never read as
// part of the row they hang from.
func (p *parser) ownText(item *ast.ListItem) (string, byteRange, bool) {
	start, stop, found := 0, 0, false
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		if _, nested := c.(*ast.List); nested {
			break
		}
		r, ok := linesRange(c)
		if !ok {
			continue
		}
		if !found || r.start < start {
			start = r.start
		}
		if !found || r.stop > stop {
			stop = r.stop
		}
		found = true
	}
	if !found {
		return "", byteRange{}, false
	}
	return p.body[start:stop], byteRange{start, stop}, true
}

// liveWikilinks are the wikilinks in a source range that actually address
// another note: an embed shows a note rather than listing it, a same-file
// anchor names no note, and a link inside code or an Obsidian comment is quoted
// or switched off rather than written.
func (p *parser) liveWikilinks(rng byteRange) []Candidate {
	if rng.stop <= rng.start {
		return nil
	}
	var out []Candidate
	segment := p.body[rng.start:rng.stop]
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
		absolute := rng.start + open
		if inAnyZone(p.zones, absolute) {
			continue
		}
		if open > 0 && segment[open-1] == '!' {
			continue // an embed shows the note; it does not list it
		}
		target, display, ok := graph.SplitWikilink(inner)
		if !ok {
			continue
		}
		out = append(out, Candidate{Text: display, Target: target})
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

// byteRange is a half-open span into a body.
type byteRange struct{ start, stop int }

func (r byteRange) contains(off int) bool { return off >= r.start && off < r.stop }

func inAnyZone(zones []byteRange, off int) bool {
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
func skipZones(body string) []byteRange {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))
	var code []byteRange
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

// commentZones are the Obsidian %%...%% spans. A %% inside code is ignored
// first so it cannot shift the pairing of real comments; an unpaired trailing
// mark is dropped.
func commentZones(body string, code []byteRange) []byteRange {
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
	var zones []byteRange
	for k := 0; k+1 < len(marks); k += 2 {
		zones = append(zones, byteRange{marks[k], marks[k+1] + 2})
	}
	return zones
}

// linesRange is a block node's source span.
func linesRange(n ast.Node) (byteRange, bool) {
	ls := n.Lines()
	if ls == nil || ls.Len() == 0 {
		return byteRange{}, false
	}
	return byteRange{ls.At(0).Start, ls.At(ls.Len() - 1).Stop}, true
}

// inlineRange covers an inline node's text children.
func inlineRange(n ast.Node) (byteRange, bool) {
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
		return byteRange{}, false
	}
	return byteRange{start, stop}, true
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
