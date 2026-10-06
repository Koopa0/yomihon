package render

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// formattingGate decides, in source order, whether each formatting opener of
// one container becomes markup. A closer is always kept: one that closes
// nothing is ignored by the browser and opens nothing.
type formattingGate struct {
	paired []bool
	next   int
	all    bool
}

// admitAllFormatting is the gate for an inline tag the parse already paired:
// pairFormattingInline marked the ones it could not.
var admitAllFormatting = &formattingGate{all: true}

func (g *formattingGate) admits(tag []byte) bool {
	m := safeFormattingTag.FindSubmatch(tag)
	switch {
	case m == nil:
		return false
	case len(m[1]) > 0 || g.all:
		return true
	}
	ok := g.next < len(g.paired) && g.paired[g.next]
	g.next++
	return ok
}

// pairFormatting reads one container's raw markup, in the pieces it is written
// in, and pairs each formatting opener with the next unclaimed closer of the
// same name after it. The pieces are walked the way they are later written, so
// the gate's openers are the ones the writing walk meets.
func pairFormatting(chunks ...[]byte) *formattingGate {
	var tags [][]byte
	collect := &formattingGate{all: true}
	for _, chunk := range chunks {
		_ = visitSafeMarkup(chunk, collect, //nolint:errcheck // none of the callbacks returns an error
			func([]byte) error { return nil },
			func(tag []byte) error {
				if safeFormattingTag.Match(tag) {
					tags = append(tags, tag)
				}
				return nil
			},
			func([]byte) error { return nil },
			func([]byte) {},
		)
	}
	return &formattingGate{paired: pairedOpeners(tags)}
}

// pairedOpeners reports, for each opener among tags in order, whether a later
// closer of the same name claims it.
func pairedOpeners(tags [][]byte) []bool {
	var paired []bool
	open := map[string][]int{}
	for _, tag := range tags {
		name := string(safeFormattingTag.FindSubmatch(tag)[2])
		if formattingOpener(tag) {
			open[name] = append(open[name], len(paired))
			paired = append(paired, false)
			continue
		}
		if waiting := open[name]; len(waiting) > 0 {
			paired[waiting[len(waiting)-1]] = true
			open[name] = waiting[:len(waiting)-1]
		}
	}
	return paired
}

func formattingOpener(tag []byte) bool {
	m := safeFormattingTag.FindSubmatch(tag)
	return len(m) == 3 && len(m[1]) == 0
}

// unpairedFormattingAttribute marks an inline formatting opener that no closer
// in its block claims, so the renderer escapes it.
const unpairedFormattingAttribute = "yomihon-unpaired-formatting"

// pairFormattingInline pairs the inline formatting tags of each block. Inline
// raw HTML arrives one tag per node, so the pairing is decided once over the
// whole document, before any node is written. An image's description is
// written as its words alone, so a tag there neither opens nor closes anything.
type pairFormattingInline struct{}

func (pairFormattingInline) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	var open []*formattingContainer
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		switch {
		case n.Type() == ast.TypeBlock && entering:
			open = append(open, &formattingContainer{})
		case n.Type() == ast.TypeBlock:
			open[len(open)-1].markUnpaired()
			open = open[:len(open)-1]
		case !entering:
		case n.Kind() == ast.KindImage:
			return ast.WalkSkipChildren, nil
		case len(open) > 0:
			open[len(open)-1].add(n, source)
		}
		return ast.WalkContinue, nil
	})
}

// formattingContainer is one block's inline formatting tags, in source order,
// beside the nodes that carry them.
type formattingContainer struct {
	nodes []*ast.RawHTML
	tags  [][]byte
}

func (c *formattingContainer) add(n ast.Node, source []byte) {
	raw, ok := n.(*ast.RawHTML)
	if !ok || raw.Segments.Len() != 1 {
		return
	}
	segment := raw.Segments.At(0)
	if tag := segment.Value(source); safeFormattingTag.Match(tag) {
		c.nodes = append(c.nodes, raw)
		c.tags = append(c.tags, tag)
	}
}

func (c *formattingContainer) markUnpaired() {
	paired := pairedOpeners(c.tags)
	opener := 0
	for i, tag := range c.tags {
		if !formattingOpener(tag) {
			continue
		}
		if !paired[opener] {
			c.nodes[i].SetAttributeString(unpairedFormattingAttribute, true)
		}
		opener++
	}
}
