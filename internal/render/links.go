package render

import (
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// SourceLink is a place in a body's source that the page's parser reads as a
// link, an image, an autolink, raw HTML or the definition of a link reference.
// Start and End are byte offsets into the body, so a caller can cut the text
// by what the parser found and not by a second reading of the Markdown.
type SourceLink struct {
	Start, End int
	// Destination is the address as the parser read it: unescaped, and for a
	// use of a reference the one its definition gave. It is empty for raw HTML,
	// which is not read for an address.
	Destination string
	// HTML is whether the node is raw HTML, an element or a comment.
	HTML bool
}

// SourceLinks reports every such place in body, in the order the parser met
// them, read with the page's own grammar. A link inside a code span or a code
// block is not one to this parser, and is not reported.
func SourceLinks(body string) []SourceLink {
	var found []SourceLink
	var inline []util.PrioritizedValue
	for _, p := range parser.DefaultInlineParsers() {
		if delegate, ok := p.Value.(parser.InlineParser); ok && watched(delegate) {
			p.Value = recordedParser{delegate: delegate, found: &found}
		}
		inline = append(inline, p)
	}
	md := pageMarkdown(goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(inline...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)))
	source := []byte(body)
	doc := md.Parser().Parse(text.NewReader(source))
	// The walk reaches the blocks. Its visitor never returns an error.
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.LinkReferenceDefinition:
			if start, end, ok := lineSpan(n); ok {
				found = append(found, SourceLink{Start: start, End: end, Destination: string(n.Destination)})
			}
		case *ast.HTMLBlock:
			start, end, ok := lineSpan(n)
			if closure := n.ClosureLine; closure.Len() > 0 && ok {
				end = max(end, closure.Stop)
			}
			if ok {
				found = append(found, SourceLink{Start: start, End: len(strings.TrimRight(body[:end], "\r\n")), HTML: true})
			}
		}
		return ast.WalkContinue, nil
	})
	slices.SortStableFunc(found, func(a, b SourceLink) int { return a.Start - b.Start })
	return found
}

// lineSpan is the bytes from the first line of a block to the end of its last.
func lineSpan(n ast.Node) (start, end int, ok bool) {
	lines := n.Lines()
	if lines.Len() == 0 {
		return 0, 0, false
	}
	return lines.At(0).Start, lines.At(lines.Len() - 1).Stop, true
}

// watched is whether delegate is one of the three inline parsers that read a
// link or a piece of raw HTML.
func watched(delegate parser.InlineParser) bool {
	return delegate == parser.NewLinkParser() || delegate == parser.NewAutoLinkParser() || delegate == parser.NewRawHTMLParser()
}

// recordedParser is an inline parser that reads what its delegate reads and
// notes where in the source each link, autolink and piece of raw HTML it
// produced begins and ends. It changes nothing the delegate does.
type recordedParser struct {
	delegate parser.InlineParser
	found    *[]SourceLink
}

func (p recordedParser) Trigger() []byte { return p.delegate.Trigger() }

func (p recordedParser) Parse(parent ast.Node, reader text.Reader, pc parser.Context) ast.Node {
	_, here := reader.PeekLine()
	node := p.delegate.Parse(parent, reader, pc)
	_, after := reader.Position()
	switch n := node.(type) {
	case *ast.Link:
		*p.found = append(*p.found, SourceLink{Start: n.Pos(), End: after.Start, Destination: string(n.Destination)})
	case *ast.Image:
		*p.found = append(*p.found, SourceLink{Start: n.Pos(), End: after.Start, Destination: string(n.Destination)})
	case *ast.AutoLink:
		*p.found = append(*p.found, SourceLink{Start: here.Start, End: after.Start, Destination: string(n.URL(reader.Source()))})
	case *ast.RawHTML:
		*p.found = append(*p.found, SourceLink{Start: here.Start, End: after.Start, HTML: true})
	}
	return node
}

// CloseBlock hands the end of a block to a delegate that has state to clear.
func (p recordedParser) CloseBlock(parent ast.Node, reader text.Reader, pc parser.Context) {
	if closer, ok := p.delegate.(parser.CloseBlocker); ok {
		closer.CloseBlock(parent, reader, pc)
	}
}
