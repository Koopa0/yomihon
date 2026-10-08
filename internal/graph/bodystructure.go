package graph

import (
	"iter"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// BodyOutline contains exactly one heading, direct list, or stray block group.
// Positive IDs belong to this BodyFacts value; zero means absent.
type BodyOutline struct {
	Heading BodyOutlineHeading
	ListID  int
	StrayID int
}

// BodyOutlineHeading retains original lines, without consumer interpretation.
type BodyOutlineHeading struct {
	Span  Span
	Level int
}

// BodyList identifies a list and its immediate owning row, if any.
type BodyList struct {
	ID          int
	ParentRowID int
}

// BodyRow retains direct list relationships and explicit line presence.
type BodyRow struct {
	ID                 int
	ListID             int
	ParentRowID        int
	ChildListID        int
	Fallback           Span
	HasFallback        bool
	FirstBlockHasLines bool
}

// BodyInlinePart retains original text bytes and independent break flags.
// Code denotes an exact parser CodeSpan; its span covers raw content text,
// rather than normalized words or the complete delimiter-bearing CodeFact.
type BodyInlinePart struct {
	Span          Span
	Code          bool
	SoftLineBreak bool
	HardLineBreak bool
}

// RichBodyHeading retains the pre-expansion comment-free presentation bytes.
// Original is only an envelope; Origins supplies actual piece attribution.
type RichBodyHeading struct {
	ID       int
	Level    int
	Raw      string
	Span     Span
	Original Span
}

// BodyOrigin attributes one presentation piece. Synthetic pieces have no
// original identity and carry a zero Original span.
type BodyOrigin struct {
	Presentation Span
	Original     Span
	Synthetic    bool
}

type bodyCollection struct{ rows, rich bool }

type bodyStructure struct {
	outline  []BodyOutline
	lists    []BodyList
	listRows [][]int
	rows     []BodyRow
	blocks   [][]Span
	parts    [][]BodyInlinePart
	strays   [][]Span
	openers  []Span
}

// Outline yields top-level structure in parser order over original bytes.
func (f BodyFacts) Outline() iter.Seq[BodyOutline] {
	if f.data == nil {
		return bodyValues[BodyOutline](nil)
	}
	return bodyValues(f.data.structure.outline)
}

// List returns a local list value, or false for an absent ID.
func (f BodyFacts) List(id int) (BodyList, bool) {
	if f.data == nil {
		return BodyList{}, false
	}

	if id <= 0 || id > len(f.data.structure.lists) {
		return BodyList{}, false
	}
	return f.data.structure.lists[id-1], true
}

// Row returns a local row value, or false for an absent ID.
func (f BodyFacts) Row(id int) (BodyRow, bool) {
	if f.data == nil {
		return BodyRow{}, false
	}

	if id <= 0 || id > len(f.data.structure.rows) {
		return BodyRow{}, false
	}
	return f.data.structure.rows[id-1], true
}

// ListRows yields only direct rows of a list in parser order.
func (f BodyFacts) ListRows(id int) iter.Seq[BodyRow] {
	if f.data == nil {
		return bodyValues[BodyRow](nil)
	}

	return func(yield func(BodyRow) bool) {
		if id <= 0 || id > len(f.data.structure.listRows) {
			return
		}
		for _, rowID := range f.data.structure.listRows[id-1] {
			if !yield(f.data.structure.rows[rowID-1]) {
				return
			}
		}
	}
}

// RowBlocks yields owned blocks, excluding every nested list at every depth.
func (f BodyFacts) RowBlocks(id int) iter.Seq[Span] {
	if f.data == nil {
		return bodyValues[Span](nil)
	}

	if id <= 0 || id > len(f.data.structure.blocks) {
		return bodyValues[Span](nil)
	}
	return bodyValues(f.data.structure.blocks[id-1])
}

// RowInlineParts yields the first non-list child's words only when that child
// has its own lines. A container without lines does not select a later child.
func (f BodyFacts) RowInlineParts(id int) iter.Seq[BodyInlinePart] {
	if f.data == nil {
		return bodyValues[BodyInlinePart](nil)
	}

	if id <= 0 || id > len(f.data.structure.parts) {
		return bodyValues[BodyInlinePart](nil)
	}
	return bodyValues(f.data.structure.parts[id-1])
}

// StrayBlocks yields own-lines-first block spans for one outline group.
func (f BodyFacts) StrayBlocks(id int) iter.Seq[Span] {
	if f.data == nil {
		return bodyValues[Span](nil)
	}

	if id <= 0 || id > len(f.data.structure.strays) {
		return bodyValues[Span](nil)
	}
	return bodyValues(f.data.structure.strays[id-1])
}

// PairedEmphasisOpeners yields paired runs in parser traversal order.
func (f BodyFacts) PairedEmphasisOpeners() iter.Seq[Span] {
	if f.data == nil {
		return bodyValues[Span](nil)
	}
	return bodyValues(f.data.structure.openers)
}

// RichHeadings yields headings after ordinary footnote placement.
func (f BodyFacts) RichHeadings() iter.Seq[RichBodyHeading] {
	if f.data == nil {
		return bodyValues[RichBodyHeading](nil)
	}
	return bodyValues(f.data.richHeadings)
}

// RichHeadingOrigins yields actual pieces in presentation order.
func (f BodyFacts) RichHeadingOrigins(id int) iter.Seq[BodyOrigin] {
	if f.data == nil {
		return bodyValues[BodyOrigin](nil)
	}

	if id <= 0 || id > len(f.data.richOrigins) {
		return bodyValues[BodyOrigin](nil)
	}
	return bodyValues(f.data.richOrigins[id-1])
}

type bodyStructureCollector struct{}

func (bodyStructureCollector) Transform(doc *ast.Document, _ text.Reader, context parser.Context) {
	o := bodyObservationIn(context)
	if o == nil || (!o.collection.rows && !o.collection.rich) {
		return
	}
	index := bodyStructureIndex{lists: make(map[ast.Node]int), rows: make(map[ast.Node]int)}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // visitor never fails
		if entering {
			o.collectStructureNode(n, index)
		}
		return ast.WalkContinue, nil
	})
	if !o.collection.rows {
		return
	}
	o.collectListMembership(index)
	for node, id := range index.rows {
		o.collectRow(node, id, index)
	}
	o.collectOutline(doc, index)
}

// bodyStructureIndex exists only while the collector converts the tree.
type bodyStructureIndex struct {
	lists map[ast.Node]int
	rows  map[ast.Node]int
}

func (o *bodyObservation) collectStructureNode(n ast.Node, index bodyStructureIndex) {
	if o.collection.rich {
		if h, ok := n.(*ast.Heading); ok {
			span, _ := bodyLinesRange(h)
			o.rich = append(o.rich, BodyOutlineHeading{Span: span, Level: h.Level})
		}
	}
	if !o.collection.rows {
		return
	}
	switch node := n.(type) {
	case *ast.List:
		id := len(o.structure.lists) + 1
		index.lists[node] = id
		o.structure.lists = append(o.structure.lists, BodyList{ID: id})
		o.structure.listRows = append(o.structure.listRows, nil)
	case *ast.ListItem:
		id := len(o.structure.rows) + 1
		index.rows[node] = id
		o.structure.rows = append(o.structure.rows, BodyRow{ID: id})
		o.structure.blocks = append(o.structure.blocks, nil)
		o.structure.parts = append(o.structure.parts, nil)
	case *ast.Emphasis:
		if span, ok := bodyEmphasisOpener(node); ok {
			o.structure.openers = append(o.structure.openers, span)
		}
	}
}

func (o *bodyObservation) collectListMembership(index bodyStructureIndex) {
	for node, id := range index.lists {
		o.structure.lists[id-1].ParentRowID = index.rows[node.Parent()]
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			if rowID := index.rows[c]; rowID != 0 {
				o.structure.listRows[id-1] = append(o.structure.listRows[id-1], rowID)
			}
		}
	}
}

func (o *bodyObservation) collectRow(node ast.Node, id int, index bodyStructureIndex) {
	row := &o.structure.rows[id-1]
	row.ListID = index.lists[node.Parent()]
	if row.ListID != 0 {
		row.ParentRowID = o.structure.lists[row.ListID-1].ParentRowID
	}
	row.Fallback, row.HasFallback = bodyLinesRange(node)
	o.structure.blocks[id-1] = bodyOwnedBlocks(node)
	selected := false
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if childID := index.lists[c]; childID != 0 {
			if row.ChildListID == 0 {
				row.ChildListID = childID
			}
			continue
		}
		if selected {
			continue
		}
		selected = true
		_, row.FirstBlockHasLines = bodyLinesRange(c)
		if row.FirstBlockHasLines {
			o.structure.parts[id-1] = bodyInlineParts(c)
		}
	}
}

func (o *bodyObservation) collectOutline(doc *ast.Document, index bodyStructureIndex) {
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if h, ok := c.(*ast.Heading); ok {
			if span, present := bodyLinesRange(h); present {
				o.structure.outline = append(o.structure.outline, BodyOutline{Heading: BodyOutlineHeading{Span: span, Level: h.Level}})
			}
			continue
		}
		if id := index.lists[c]; id != 0 {
			o.structure.outline = append(o.structure.outline, BodyOutline{ListID: id})
			continue
		}
		o.structure.strays = append(o.structure.strays, bodyStrayBlocks(c))
		o.structure.outline = append(o.structure.outline, BodyOutline{StrayID: len(o.structure.strays)})
	}
}

func bodyLinesRange(n ast.Node) (Span, bool) {
	if n.Type() != ast.TypeBlock {
		return Span{}, false
	}
	lines := n.Lines()
	if lines == nil || lines.Len() == 0 {
		return Span{}, false
	}
	return Span{Start: lines.At(0).Start, Stop: lines.At(lines.Len() - 1).Stop}, true
}

func bodyOwnedBlocks(n ast.Node) []Span {
	var spans []Span
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if _, nested := c.(*ast.List); nested {
			continue
		}
		if span, ok := bodyLinesRange(c); ok {
			spans = append(spans, span)
		} else {
			spans = append(spans, bodyOwnedBlocks(c)...)
		}
	}
	return spans
}

func bodyStrayBlocks(n ast.Node) []Span {
	if span, ok := bodyLinesRange(n); ok {
		return []Span{span}
	}
	var spans []Span
	if n.Type() == ast.TypeBlock {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			spans = append(spans, bodyStrayBlocks(c)...)
		}
	}
	return spans
}

func bodyInlineParts(n ast.Node) []BodyInlinePart {
	var parts []BodyInlinePart
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Text:
			parts = append(parts, BodyInlinePart{Span: Span{Start: node.Segment.Start, Stop: node.Segment.Stop}, SoftLineBreak: node.SoftLineBreak(), HardLineBreak: node.HardLineBreak()})
		case *ast.CodeSpan:
			span, found := bodyCodeContentSpan(node)
			if found {
				parts = append(parts, BodyInlinePart{Span: span, Code: true})
			}
		default:
			parts = append(parts, bodyInlineParts(c)...)
		}
	}
	return parts
}

func bodyCodeContentSpan(node *ast.CodeSpan) (Span, bool) {
	span, found := Span{}, false
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		t, ok := child.(*ast.Text)
		if !ok {
			continue
		}
		if !found || t.Segment.Start < span.Start {
			span.Start = t.Segment.Start
		}
		if !found || t.Segment.Stop > span.Stop {
			span.Stop = t.Segment.Stop
		}
		found = true
	}
	return span, found
}

func bodyEmphasisOpener(n *ast.Emphasis) (Span, bool) {
	child := n.FirstChild()
	if child == nil {
		return Span{}, false
	}
	content := 0
	if inner, ok := child.(*ast.Emphasis); ok {
		span, found := bodyEmphasisOpener(inner)
		if !found {
			return Span{}, false
		}
		content = span.Start
	} else {
		var found bool
		content, found = bodyFirstTextStart(child)
		if !found {
			return Span{}, false
		}
	}
	if content < n.Level {
		return Span{}, false
	}
	return Span{Start: content - n.Level, Stop: content}, true
}

func bodyFirstTextStart(n ast.Node) (int, bool) {
	if t, ok := n.(*ast.Text); ok {
		return t.Segment.Start, true
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if offset, ok := bodyFirstTextStart(c); ok {
			return offset, true
		}
	}
	return 0, false
}

func (f *bodyFactsData) attachRichHeadings(p bodyProjection, headings []BodyOutlineHeading) {
	for _, h := range headings {
		raw := ""
		if h.Span.Start >= 0 && h.Span.Start <= h.Span.Stop && h.Span.Stop <= len(p.text) {
			raw = p.text[h.Span.Start:h.Span.Stop]
		}
		f.richHeadings = append(f.richHeadings, RichBodyHeading{
			ID: len(f.richHeadings) + 1, Level: h.Level, Raw: raw,
			Span: h.Span, Original: p.sourceSpan(h.Span),
		})
		var origins []BodyOrigin
		for _, piece := range p.pieces {
			left, right := max(h.Span.Start, piece.start), min(h.Span.Stop, piece.stop)
			if right <= left {
				continue
			}
			origin := BodyOrigin{Presentation: Span{Start: left, Stop: right}, Synthetic: piece.origin < 0}
			if !origin.Synthetic {
				start := piece.origin + left - piece.start
				origin.Original = Span{Start: start, Stop: start + right - left}
			}
			origins = append(origins, origin)
		}
		f.richOrigins = append(f.richOrigins, origins)
	}
}
