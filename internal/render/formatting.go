package render

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// formattingGate decides, in source order, whether each formatting opener of
// one container becomes markup. A closer is always kept: one that closes
// nothing is ignored by the browser and opens nothing. A gate that decides
// every tag holds a decision for closers as well, in the order the walk meets
// them.
type formattingGate struct {
	paired   []bool
	next     int
	all      bool
	everyTag bool
}

// admitAllFormatting is the gate for an inline tag the parse already paired:
// pairFormattingInline marked the ones it could not.
var admitAllFormatting = &formattingGate{all: true}

func (g *formattingGate) admits(tag []byte) bool {
	m := safeFormattingTag.FindSubmatch(tag)
	switch {
	case m == nil:
		return false
	case g.all, len(m[1]) > 0 && !g.everyTag:
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
//
// An excerpt written mid-paragraph parts that paragraph in two once it is
// rendered, so each side is a container of its own: an opener before the
// excerpt cannot claim a closer after it. Paired across the excerpt, the
// underline would reopen over the excerpt's own chrome, and the listening page,
// which keeps only the marked side, would never receive the closer at all.
type pairFormattingInline struct{}

func (pairFormattingInline) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	var open []*formattingContainer
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		switch {
		case n.Type() == ast.TypeBlock && entering:
			open = append(open, &formattingContainer{parted: n.Kind() == ast.KindParagraph})
		case n.Type() == ast.TypeBlock:
			open[len(open)-1].markUnpaired()
			open = open[:len(open)-1]
		case !entering:
		case n.Kind() == ast.KindImage:
			return ast.WalkSkipChildren, nil
		case len(open) > 0 && open[len(open)-1].partsAt(n, source):
			open[len(open)-1].markUnpaired()
			open[len(open)-1] = &formattingContainer{parted: true}
		case len(open) > 0:
			open[len(open)-1].add(n, source)
		}
		return ast.WalkContinue, nil
	})
}

// formattingContainer is one block's inline formatting tags, in source order,
// beside the nodes that carry them. parted marks a paragraph, the one block the
// renderer parts around an excerpt written inside it.
type formattingContainer struct {
	nodes  []*ast.RawHTML
	tags   [][]byte
	parted bool
}

// partsAt reports whether n carries the marker of an excerpt that will part
// this container's paragraph once rendered.
func (c *formattingContainer) partsAt(n ast.Node, source []byte) bool {
	t, ok := n.(*ast.Text)
	return ok && c.parted && bytes.Contains(t.Segment.Value(source), []byte(wideMarkOpen))
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

// headingParser reads a heading's words with the inline grammar the page reads
// them with, so a code span, a backslash escape or an autolink claims the bytes
// it claims there, and the formatting pairing runs as it does on the page. It
// keeps nothing between parses.
var headingParser = goldmark.New(goldmark.WithExtensions(extension.GFM, safeMarkupExtension{})).Parser()

// headingFormattingGate decides each formatting tag in a heading's words, closer
// and opener alike, by whether the page's parse of that heading reads it as a
// tag, and an opener also by whether the parse paired it. A tag the page shows
// as text is escaped here too, so the words name the section the way the page
// stamps its id. parseable is words with every byte a link displays replaced by
// a letter: the page writes a link's label as text, so nothing in it is a tag
// or opens a code span around one.
func headingFormattingGate(words, parseable string) *formattingGate {
	gate := &formattingGate{everyTag: true}
	if !strings.Contains(parseable, "<") {
		return gate
	}
	const prefix = "# "
	source := []byte(prefix + parseable)
	parsed := map[int]bool{}
	_ = ast.Walk(headingParser.Parse(text.NewReader(source)), func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		raw, ok := n.(*ast.RawHTML)
		if !entering || !ok || raw.Segments.Len() != 1 {
			return ast.WalkContinue, nil
		}
		segment := raw.Segments.At(0)
		if safeFormattingTag.Match(segment.Value(source)) {
			_, unpaired := raw.AttributeString(unpairedFormattingAttribute)
			parsed[segment.Start-len(prefix)] = !unpaired
		}
		return ast.WalkContinue, nil
	})
	offset := 0
	_ = visitSafeMarkup([]byte(words), admitAllFormatting, //nolint:errcheck // none of the callbacks returns an error
		func(p []byte) error { offset += len(p); return nil },
		func(tag []byte) error {
			if safeFormattingTag.Match(tag) {
				gate.paired = append(gate.paired, parsed[offset])
			}
			offset += len(tag)
			return nil
		},
		func(tag []byte) error { offset += len(tag); return nil },
		func(tag []byte) { offset += len(tag) },
	)
	return gate
}
