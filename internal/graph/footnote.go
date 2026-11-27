package graph

import (
	"cmp"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// InlineFootnote is one Obsidian inline footnote, ^[…], in a Markdown source.
type InlineFootnote struct {
	// Span is the whole note, the caret and both brackets included.
	Span Span
	// Content is the note's own words, the text between its brackets.
	Content string
}

// footnoteNode records an inline note's source without rendering it. Goldmark
// decides where inline syntax is legal, so code, escapes and HTML attributes
// do not become notes.
type footnoteNode struct {
	ast.BaseInline

	span    text.Segment
	content *text.Segments
}

var kindFootnoteNode = ast.NewNodeKind("InlineFootnote")

func (*footnoteNode) Kind() ast.NodeKind { return kindFootnoteNode }

func (n *footnoteNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type inlineFootnoteParser struct{}

func (inlineFootnoteParser) Trigger() []byte { return []byte{'^'} }

func (inlineFootnoteParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, start := block.PeekLine()
	if !strings.HasPrefix(string(line), "^[") {
		return nil
	}
	block.Advance(2)
	content, found := block.FindClosure('[', ']', text.FindClosureOptions{
		CodeSpan: true, Nesting: true, Newline: true, Advance: true,
	})
	if !found || util.IsBlank(content.Value(block.Source())) {
		return nil
	}
	_, end := block.Position()
	return &footnoteNode{span: text.NewSegment(start.Start, end.Start), content: content}
}

// footnoteReader reads a source with the page's footnote grammar, ordinary and
// inline.
var footnoteReader = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithInlineParsers(util.Prioritized(inlineFootnoteParser{}, 100))),
).Parser()

// InlineFootnotes is every inline footnote in source, in source order. One
// written in code, escaped, or inside a raw attribute is not among them, and
// neither is one with nothing between its brackets.
func InlineFootnotes(source string) []InlineFootnote {
	if !strings.Contains(source, "^[") {
		return nil
	}
	src := []byte(source)
	var notes []InlineFootnote
	_ = ast.Walk(footnoteReader.Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		if note, ok := n.(*footnoteNode); ok && entering {
			notes = append(notes, InlineFootnote{
				Span:    Span{Start: note.span.Start, Stop: note.span.Stop},
				Content: string(note.content.Value(src)),
			})
		}
		return ast.WalkContinue, nil
	})
	slices.SortFunc(notes, func(a, b InlineFootnote) int { return cmp.Compare(a.Span.Start, b.Span.Start) })
	return notes
}

// HeadingSource is a heading's source without what is never part of the words
// it is called by: its block address and its inline footnotes. The title and
// the id the page stamps both start from here.
func HeadingSource(raw string) string {
	source := WithoutBlockAddresses(raw)
	notes := InlineFootnotes(source)
	if len(notes) == 0 {
		return source
	}
	var b strings.Builder
	last := 0
	for _, note := range notes {
		b.WriteString(source[last:note.Span.Start])
		last = note.Span.Stop
	}
	b.WriteString(source[last:])
	return b.String()
}

// footnoteDefinitions is a stand-in definition for every ordinary footnote
// body both defines and cites, so a heading read on its own with them after it
// takes a reference as the page does: a reference to a defined note, and any
// other "[^…]" as the text it was typed.
func footnoteDefinitions(body string) string {
	if !strings.Contains(body, "[^") {
		return ""
	}
	var b strings.Builder
	for n := footnoteReader.Parse(text.NewReader([]byte(body))).LastChild(); n != nil; n = n.PreviousSibling() {
		list, ok := n.(*east.FootnoteList)
		if !ok {
			continue
		}
		for note := list.FirstChild(); note != nil; note = note.NextSibling() {
			if note, ok := note.(*east.Footnote); ok {
				b.WriteString("\n\n[^" + string(note.Ref) + "]: .")
			}
		}
	}
	return b.String()
}
