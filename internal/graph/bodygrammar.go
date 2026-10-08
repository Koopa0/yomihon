package graph

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type bodyHighlight struct{ ast.BaseInline }

// KindHighlight identifies the authored double-equals highlight node.
var KindHighlight = ast.NewNodeKind("Highlight")

func (*bodyHighlight) Kind() ast.NodeKind { return KindHighlight }

// Dump implements goldmark's node inspection without retaining source bytes.
func (n *bodyHighlight) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// TaskMarkerAttr carries the author's neutral task character to presentation.
const TaskMarkerAttr = "yomihonTaskMarker"

type bodyHighlightDelimiter struct{}

func (bodyHighlightDelimiter) IsDelimiter(b byte) bool { return b == '=' }
func (bodyHighlightDelimiter) CanOpenCloser(a, b *parser.Delimiter) bool {
	return a.Char == b.Char
}
func (bodyHighlightDelimiter) OnMatch(int) ast.Node { return &bodyHighlight{} }

type bodyHighlightParser struct{}

// NewHighlightParser exposes the canonical stateless delegate to source-text
// observation. Callers wrap its observations without changing recognition.
func NewHighlightParser() parser.InlineParser { return bodyHighlightParser{} }

func (bodyHighlightParser) Trigger() []byte { return []byte{'='} }
func (bodyHighlightParser) Parse(_ ast.Node, reader text.Reader, pc parser.Context) ast.Node {
	line, segment := reader.PeekLine()
	node := parser.ScanDelimiter(line, reader.PrecendingCharacter(), 2, bodyHighlightDelimiter{})
	if node == nil || node.OriginalLength != 2 {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	reader.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

type bodyTaskParser struct{}

func (bodyTaskParser) Trigger() []byte { return []byte{'['} }
func (bodyTaskParser) Parse(parent ast.Node, reader text.Reader, _ parser.Context) ast.Node {
	item, ok := parent.Parent().(*ast.ListItem)
	if !ok || item.FirstChild() != parent || parent.HasChildren() {
		return nil
	}
	line, _ := reader.PeekLine()
	if len(line) < 3 || line[0] != '[' || line[2] != ']' || !strings.ContainsRune("/->", rune(line[1])) {
		return nil
	}
	if len(line) > 3 && !util.IsSpace(line[3]) {
		return nil
	}
	end := 3
	for end < len(line) && util.IsSpace(line[end]) {
		end++
	}
	reader.Advance(end)
	node := east.NewTaskCheckBox(false)
	node.SetAttributeString(TaskMarkerAttr, string(line[1]))
	return node
}

// NewBodyMarkdown constructs the fixed Expanded presentation grammar. The
// private Authored role additionally recognizes opaque inline-note admission.
// The prefix changes presentation only; callers freeze configuration before
// their first Parse, and each call owns its parser context.
func NewBodyMarkdown(prefix func(ast.Node) []byte) goldmark.Markdown {
	return newBodyMarkdown(prefix, bodyExpanded)
}

type bodyGrammarRole uint8

const (
	bodyAuthored bodyGrammarRole = iota
	bodyExpanded
)

func newBodyMarkdown(prefix func(ast.Node) []byte, role bodyGrammarRole) goldmark.Markdown {
	blocks := parser.DefaultBlockParsers()
	for i := range blocks {
		switch blocks[i].Priority {
		case 500, 700:
			blocks[i].Value = bodyBlockParser{delegate: blocks[i].Value.(parser.BlockParser)}
		}
	}
	inlines := parser.DefaultInlineParsers()
	for i := range inlines {
		if inlines[i].Priority == 100 {
			inlines[i].Value = bodyCodeParser{delegate: inlines[i].Value.(parser.InlineParser)}
		}
	}
	markdown := goldmark.New(
		goldmark.WithParser(parser.NewParser(
			parser.WithBlockParsers(blocks...),
			parser.WithInlineParsers(inlines...),
			parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
		)),
		goldmark.WithExtensions(extension.GFM, bodyFootnotes{prefix: prefix}),
		goldmark.WithParserOptions(parser.WithInlineParsers(
			util.Prioritized(NewHighlightParser(), 500),
			util.Prioritized(bodyTaskParser{}, -100),
		), parser.WithASTTransformers(util.Prioritized(bodySourceCollector{}, 998))),
	)
	if role == bodyAuthored {
		markdown.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(bodyInlineFootnoteParser{}, 100)))
	}
	return markdown
}

type bodyFootnotes struct{ prefix func(ast.Node) []byte }

func (e bodyFootnotes) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(
		parser.WithBlockParsers(util.Prioritized(bodyBlockParser{delegate: extension.NewFootnoteBlockParser()}, 999)),
		parser.WithInlineParsers(util.Prioritized(extension.NewFootnoteParser(), 101)),
		parser.WithASTTransformers(util.Prioritized(extension.NewFootnoteASTTransformer(), 999)),
	)
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(
		extension.NewFootnoteHTMLRenderer(extension.WithFootnoteIDPrefixFunction(e.prefix)), 500,
	)))
}

var bodyObservationKey = parser.NewContextKey()

type bodyObservation struct {
	codes          []CodeFact
	footnotes      []FootnoteFact
	headings       []BodyHeading
	destinations   []BodyDestination
	literals       []CodeLiteral
	htmlLimits     []CommentLimit
	blocks         map[ast.Node]int
	definitions    map[ast.Node]int
	inlineNotes    []InlineFootnoteFact
	inlineSegments map[int][]text.Segment
}

func bodyObservationIn(pc parser.Context) *bodyObservation {
	observation, _ := pc.Get(bodyObservationKey).(*bodyObservation)
	return observation
}

type bodyBlockParser struct{ delegate parser.BlockParser }

func (p bodyBlockParser) Trigger() []byte             { return p.delegate.Trigger() }
func (p bodyBlockParser) CanInterruptParagraph() bool { return p.delegate.CanInterruptParagraph() }
func (p bodyBlockParser) CanAcceptIndentedLine() bool { return p.delegate.CanAcceptIndentedLine() }

func (p bodyBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	_, segment := reader.PeekLine()
	start := segment.Start + max(pc.BlockOffset()-segment.Padding, 0)
	node, state := p.delegate.Open(parent, reader, pc)
	observation := bodyObservationIn(pc)
	if node == nil || observation == nil {
		return node, state
	}
	switch n := node.(type) {
	case *ast.FencedCodeBlock:
		fact := CodeFact{Kind: CodeFence, Span: Span{Start: start, Stop: segment.Stop}, Opener: Span{Start: start, Stop: segment.Stop}}
		if n.Info != nil {
			fact.Info = Span{Start: n.Info.Segment.Start, Stop: n.Info.Segment.Stop}
		}
		observation.blocks[node] = len(observation.codes)
		observation.codes = append(observation.codes, fact)
	case *ast.CodeBlock:
		observation.blocks[node] = len(observation.codes)
		observation.codes = append(observation.codes, CodeFact{Kind: CodeIndent, Span: Span{Start: start, Stop: segment.Stop}})
	case *east.Footnote:
		observation.definitions[node] = len(observation.footnotes)
		observation.footnotes = append(observation.footnotes, FootnoteFact{Span: Span{Start: start, Stop: segment.Stop}, Label: string(n.Ref)})
	}
	return node, state
}

func (p bodyBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	_, segment := reader.PeekLine()
	state := p.delegate.Continue(node, reader, pc)
	observation := bodyObservationIn(pc)
	if observation == nil {
		return state
	}
	if index, ok := observation.blocks[node]; ok {
		fact := &observation.codes[index]
		if state&parser.Continue != 0 {
			fact.Span.Stop = segment.Stop
		} else if fact.Kind == CodeFence && state&parser.Close != 0 {
			fact.Closer = Span{Start: segment.Start, Stop: segment.Stop}
			fact.Span.Stop = segment.Stop
		}
	}
	if index, ok := observation.definitions[node]; ok && state&parser.Continue != 0 {
		observation.footnotes[index].Span.Stop = segment.Stop
	}
	return state
}

func (p bodyBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	if observation := bodyObservationIn(pc); observation != nil {
		if index, ok := observation.blocks[node]; ok {
			fact := &observation.codes[index]
			fact.EndOfBody = fact.Closer.Zero() && fact.Span.Stop == len(reader.Source())
		}
	}
	p.delegate.Close(node, reader, pc)
}

type bodyCodeParser struct{ delegate parser.InlineParser }

func (p bodyCodeParser) Trigger() []byte { return p.delegate.Trigger() }
func (p bodyCodeParser) Parse(parent ast.Node, reader text.Reader, pc parser.Context) ast.Node {
	_, start := reader.Position()
	node := p.delegate.Parse(parent, reader, pc)
	if _, ok := node.(*ast.CodeSpan); ok {
		if observation := bodyObservationIn(pc); observation != nil {
			_, end := reader.Position()
			observation.codes = append(observation.codes, CodeFact{Kind: CodeInline, Span: Span{Start: start.Start, Stop: end.Start}})
		}
	}
	return node
}

type bodyInlineFootnote struct{ ast.BaseInline }

var kindBodyInlineFootnote = ast.NewNodeKind("InlineFootnote")

func (*bodyInlineFootnote) Kind() ast.NodeKind { return kindBodyInlineFootnote }
func (n *bodyInlineFootnote) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type bodyInlineFootnoteParser struct{}

func (bodyInlineFootnoteParser) Trigger() []byte { return []byte{'^'} }
func (bodyInlineFootnoteParser) Parse(_ ast.Node, reader text.Reader, pc parser.Context) ast.Node {
	line, start := reader.PeekLine()
	if !strings.HasPrefix(string(line), "^[") {
		return nil
	}
	reader.Advance(2)
	content, found := reader.FindClosure('[', ']', text.FindClosureOptions{
		CodeSpan: true, Nesting: true, Newline: true, Advance: true,
	})
	if !found || util.IsBlank(content.Value(reader.Source())) {
		return nil
	}
	_, end := reader.Position()
	if observation := bodyObservationIn(pc); observation != nil {
		observation.inlineNotes = append(observation.inlineNotes, InlineFootnoteFact{
			Span: Span{Start: start.Start, Stop: end.Start}, Content: string(reader.Source()[start.Start+2 : end.Start-1]),
		})
		for i := range content.Len() {
			observation.inlineSegments[start.Start] = append(observation.inlineSegments[start.Start], content.At(i))
		}
	}
	return &bodyInlineFootnote{}
}
