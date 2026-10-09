package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

var agreementNativeDefinitionSpansKey = parser.NewContextKey()

type agreementNativeDefinitionSpan struct {
	Span, Label graph.Span
	Parent      string
}

type agreementNativeDefinitionParser struct{ parser.BlockParser }

// Observe the native parser's whole consumed definition. Block lines alone
// omit reference metadata, closing fences and lazy paragraph margins.
func (p agreementNativeDefinitionParser) Open(parent ast.Node, reader text.Reader, context parser.Context) (ast.Node, parser.State) {
	_, line := reader.PeekLine()
	node, state := p.BlockParser.Open(parent, reader, context)
	if node != nil {
		spans, observed := context.Get(agreementNativeDefinitionSpansKey).(map[ast.Node]agreementNativeDefinitionSpan)
		if !observed {
			panic("missing native definition observation context")
		}
		start := bytes.LastIndexByte(reader.Source()[:line.Start], '\n') + 1
		footnote, native := node.(*extast.Footnote)
		if !native {
			panic("native definition parser returned another block kind")
		}
		labelStart := line.Start + context.BlockOffset() - line.Padding
		spans[node] = agreementNativeDefinitionSpan{
			Span: graph.Span{Start: start, Stop: line.Stop}, Label: graph.Span{Start: labelStart, Stop: labelStart + len(footnote.Ref) + 4}, Parent: parent.Kind().String(),
		}
	}
	return node, state
}

func (p agreementNativeDefinitionParser) Close(node ast.Node, reader text.Reader, context parser.Context) {
	_, position := reader.Position()
	stop := len(reader.Source())
	if position.Start < len(reader.Source()) {
		stop = bytes.LastIndexByte(reader.Source()[:position.Start], '\n') + 1
	}
	spans, observed := context.Get(agreementNativeDefinitionSpansKey).(map[ast.Node]agreementNativeDefinitionSpan)
	if !observed {
		panic("missing native definition observation context")
	}
	declaration := spans[node]
	declaration.Span.Stop = stop
	spans[node] = declaration
	p.BlockParser.Close(node, reader, context)
}

var agreementNativeOwnerGrammar = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(
		parser.WithBlockParsers(util.Prioritized(agreementNativeDefinitionParser{extension.NewFootnoteBlockParser()}, 998)),
		parser.WithASTTransformers(util.Prioritized(agreementFootnoteDeclarations{}, 998)),
	),
)

type agreementNativeOwnerNode struct {
	Kind        string
	Depth       int
	Lines, Text []graph.Span
}

type agreementNativeOwnerDefinition struct {
	Label, Parent string
	Span, Name    graph.Span
	Nodes         []agreementNativeOwnerNode
}

type agreementNativeOwnerCode struct {
	Kind, Parent string
	Span         graph.Span
}

type agreementNativeOwnerSource struct {
	Definitions []agreementNativeOwnerDefinition
	Code        []agreementNativeOwnerCode
	Comments    []graph.Span
}

func agreementNativeOwnerReading(body string) agreementNativeOwnerSource {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	spans := make(map[ast.Node]agreementNativeDefinitionSpan)
	context.Set(agreementNativeDefinitionSpansKey, spans)
	doc := agreementNativeOwnerGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var reading agreementNativeOwnerSource
	roots := []ast.Node{doc}
	for _, definition := range unused {
		roots = append(roots, definition)
	}
	var code []graph.Span
	for _, root := range roots {
		if err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering {
				if span, literal := agreementNativeCodeSpan(node); literal && !span.Zero() {
					reading.Code = append(reading.Code, agreementNativeOwnerCode{Kind: node.Kind().String(), Parent: node.Parent().Kind().String(), Span: span})
					code = append(code, span)
				}
				if fence, fenced := node.(*ast.FencedCodeBlock); fenced && fence.Info != nil {
					span := graph.Span{Start: fence.Info.Segment.Start, Stop: fence.Info.Segment.Stop}
					reading.Code = append(reading.Code, agreementNativeOwnerCode{Kind: "FencedCodeBlockInfo", Parent: fence.Parent().Kind().String(), Span: span})
					code = append(code, span)
				}
			}
			return ast.WalkContinue, nil
		}); err != nil {
			panic(err)
		}
	}
	reading.Comments = graph.CommentZones(body, code)
	for _, definition := range unused {
		declaration, found := spans[definition]
		span := declaration.Span
		if !found || span.Start < 0 || span.Stop <= span.Start || span.Stop > len(body) || declaration.Label.Start < span.Start || declaration.Label.Stop > span.Stop {
			return agreementNativeOwnerSource{}
		}
		if agreementUnusedWidgetCommentOwned(span, reading.Comments) {
			return agreementNativeOwnerSource{}
		}
		record := agreementNativeOwnerDefinition{Label: string(definition.Ref), Parent: declaration.Parent, Span: span, Name: declaration.Label}
		depth := 0
		if err := ast.Walk(definition, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				depth--
				return ast.WalkContinue, nil
			}
			child := agreementNativeOwnerNode{Kind: node.Kind().String(), Depth: depth}
			depth++
			if node.Type() == ast.TypeBlock {
				for i := range node.Lines().Len() {
					line := node.Lines().At(i)
					child.Lines = append(child.Lines, graph.Span{Start: line.Start, Stop: line.Stop})
				}
			}
			if word, textNode := node.(*ast.Text); textNode {
				child.Text = append(child.Text, graph.Span{Start: word.Segment.Start, Stop: word.Segment.Stop})
			}
			if html, raw := node.(*ast.RawHTML); raw {
				for i := range html.Segments.Len() {
					segment := html.Segments.At(i)
					child.Text = append(child.Text, graph.Span{Start: segment.Start, Stop: segment.Stop})
				}
			}
			record.Nodes = append(record.Nodes, child)
			return ast.WalkContinue, nil
		}); err != nil {
			panic(err)
		}
		reading.Definitions = append(reading.Definitions, record)
	}
	return reading
}

type agreementNativeCheckField struct {
	Span                       graph.Span
	Word                       string
	Targets                    []string
	Code, Comment, Escaped     bool
	DefinitionOwner, CodeOwner int
}

type agreementNativeCheckSource struct {
	Code, Comments []graph.Span
	Fields         []agreementNativeCheckField
	Targets        []string
}

var agreementNativeCheckGrammar = goldmark.New()

// Read the check grammar over the original whole body. An isolated definition
// parse can assign different code and comment roles to these same bytes.
func agreementNativeCheckReading(body string, page agreementNativeOwnerSource) agreementNativeCheckSource {
	doc := agreementNativeCheckGrammar.Parser().Parse(text.NewReader([]byte(body)))
	var reading agreementNativeCheckSource
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if span, code := agreementNativeCodeSpan(node); code && !span.Zero() {
				reading.Code = append(reading.Code, span)
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	reading.Comments = graph.CommentZones(body, reading.Code)
	for offset := 0; offset < len(body); {
		relative := strings.Index(body[offset:], "[[")
		if relative < 0 {
			break
		}
		start := offset + relative
		closeAt := strings.Index(body[start+2:], "]]")
		if closeAt < 0 {
			break
		}
		stop := start + 2 + closeAt + 2
		offset = stop
		field := agreementNativeCheckField{Span: graph.Span{Start: start, Stop: stop}, Word: body[start:stop], Targets: judge.LinkTargets(body[start:stop]), Code: graph.In(reading.Code, start), Comment: graph.In(reading.Comments, start), Escaped: graph.EscapedWikilinkAt(body, start), DefinitionOwner: -1, CodeOwner: -1}
		for i, definition := range page.Definitions {
			if start >= definition.Span.Start && stop <= definition.Span.Stop {
				if field.DefinitionOwner >= 0 {
					return agreementNativeCheckSource{}
				}
				field.DefinitionOwner = i
			} else if start < definition.Span.Stop && stop > definition.Span.Start {
				return agreementNativeCheckSource{}
			}
		}
		for i, code := range page.Code {
			if start >= code.Span.Start && stop <= code.Span.Stop {
				if field.CodeOwner >= 0 {
					return agreementNativeCheckSource{}
				}
				field.CodeOwner = i
			} else if start < code.Span.Stop && stop > code.Span.Start {
				return agreementNativeCheckSource{}
			}
		}
		if !field.Code && !field.Comment && !field.Escaped {
			reading.Targets = append(reading.Targets, field.Targets...)
		}
		reading.Fields = append(reading.Fields, field)
	}
	return reading
}

type agreementNativeCheckPayload struct {
	Citation     agreementUnusedCitationPayload
	Code, Unused map[string]int
}

func agreementNativeCheckBudget(body string) agreementNativeCheckPayload {
	page := agreementNativeOwnerReading(body)
	if len(page.Definitions) == 0 && len(page.Code) == 0 {
		return agreementNativeCheckPayload{}
	}
	check := agreementNativeCheckReading(body, page)
	if !cmp.Equal(check.Targets, judge.LinkTargets(body)) {
		return agreementNativeCheckPayload{}
	}
	budget := agreementNativeCheckPayload{Citation: agreementUnusedCitationPayload{Body: body}}
	for _, field := range check.Fields {
		if field.Code || field.Comment || field.Escaped || field.DefinitionOwner < 0 && field.CodeOwner < 0 {
			continue
		}
		for _, target := range field.Targets {
			if budget.Citation.Targets == nil {
				budget.Citation.Targets = make(map[string]int)
			}
			budget.Citation.Targets[target]++
			if field.DefinitionOwner >= 0 {
				if budget.Unused == nil {
					budget.Unused = make(map[string]int)
				}
				budget.Unused[target]++
			} else {
				if budget.Code == nil {
					budget.Code = make(map[string]int)
				}
				budget.Code[target]++
			}
		}
	}
	return budget
}

func agreementNativeCheckDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementNativeCheckPayload) (kind, authority, wrong string) {
	if len(actual.Citations) != 0 {
		return "", "", ""
	}
	kind, _, wrong = agreementSharedUnusedCitationDifference(c, f, actual, budget.Citation)
	if kind == "" || budget.Code[f.Tuple.Target]+budget.Unused[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	switch {
	case budget.Code[f.Tuple.Target] > 0 && budget.Unused[f.Tuple.Target] > 0:
		authority = "#1011 stages 4 and 5"
	case budget.Unused[f.Tuple.Target] > 0:
		authority = "#1011 stage 5"
	default:
		authority = "#1011 stage 4"
	}
	return kind, authority, wrong
}

func TestAgreementNativeOwnerSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementNativeOwnerSource
	}{
		{name: "whole label and native text segments", body: "[^n]: [[A]]\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 12}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 11}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 6, Stop: 7}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 7, Stop: 8}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 8, Stop: 10}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 10, Stop: 11}}}}}}}},
		{name: "reference metadata and heading belong to one consumed block", body: "[^n]: [r]: [[A]]\n\n    ## heading\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 33}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "LinkReferenceDefinition", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 16}}}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 25, Stop: 32}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 25, Stop: 32}}}}}}}},
		{name: "lazy continuation closes before unrelated paragraph", body: "[^n]: [[A]]\n[[B]]\n\nordinary [[C]]\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 19}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 12}, {Start: 12, Stop: 17}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 6, Stop: 7}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 7, Stop: 8}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 8, Stop: 10}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 10, Stop: 11}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 12, Stop: 13}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 13, Stop: 14}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 14, Stop: 16}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 16, Stop: 17}}}}}}}},
		{name: "closing fence remains in physical definition", body: "[^n]: [[A]]\n\n    ``` [[B]]\n    [[C]]\n    ```\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 45}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 11}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 6, Stop: 7}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 7, Stop: 8}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 8, Stop: 10}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 10, Stop: 11}}}, {Kind: "FencedCodeBlock", Depth: 1, Lines: []graph.Span{{Start: 31, Stop: 37}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Footnote", Span: graph.Span{Start: 31, Stop: 37}}, {Kind: "FencedCodeBlockInfo", Parent: "Footnote", Span: graph.Span{Start: 21, Stop: 26}}}}},
		{name: "fieldless definition retains its declaration", body: "[^n]:\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 6}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}}}}}},
		{name: "used definition has only literal code owner", body: "ref[^n]\n\n[^n]: [[A]] `code`\n", want: agreementNativeOwnerSource{Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 22, Stop: 26}}}}},
		{name: "hidden declaration cannot supply active ownership", body: "%%\n[^n]: [[A]]\n%%\n"},
		{name: "quote container retains source label and every paragraph", body: "> [^n]: [[A]]\n>\n>     [[B]]\n\nordinary [[C]]\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Blockquote", Span: graph.Span{Start: 0, Stop: 28}, Name: graph.Span{Start: 2, Stop: 7}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 8, Stop: 13}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 8, Stop: 9}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 9, Stop: 10}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 10, Stop: 12}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 12, Stop: 13}}}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 22, Stop: 27}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 22, Stop: 23}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 23, Stop: 24}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 24, Stop: 26}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 26, Stop: 27}}}}}}}},
		{name: "Unicode label and CRLF keep byte coordinates", body: "> [^é]:\r\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "é", Parent: "Blockquote", Span: graph.Span{Start: 0, Stop: 10}, Name: graph.Span{Start: 2, Stop: 8}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}}}}}},
		{name: "raw HTML segments retain their source coordinates", body: "[^n]: <em>[[A]]</em>\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 21}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 20}}}, {Kind: "RawHTML", Depth: 2, Text: []graph.Span{{Start: 6, Stop: 10}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 10, Stop: 11}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 11, Stop: 12}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 12, Stop: 15}}}, {Kind: "RawHTML", Depth: 2, Text: []graph.Span{{Start: 15, Stop: 20}}}}}}}},
		{name: "indented code retains its declared kind", body: "    [[A]]\n", want: agreementNativeOwnerSource{Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 10}}}}},
		{name: "indented label keeps its original byte offset", body: "   [^n]:\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 9}, Name: graph.Span{Start: 3, Stop: 8}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}}}}}},
		{name: "every discarded definition retains its code owner", body: "[^n]: `[[A]]`\n\n[^m]: `[[B]]`\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 15}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 13}}}, {Kind: "CodeSpan", Depth: 2}, {Kind: "Text", Depth: 3, Text: []graph.Span{{Start: 7, Stop: 12}}}}}, {Label: "m", Parent: "Document", Span: graph.Span{Start: 15, Stop: 29}, Name: graph.Span{Start: 15, Stop: 20}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 21, Stop: 28}}}, {Kind: "CodeSpan", Depth: 2}, {Kind: "Text", Depth: 3, Text: []graph.Span{{Start: 22, Stop: 27}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 7, Stop: 12}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 22, Stop: 27}}}}},
		{name: "native code protects a literal comment delimiter", body: "`%%`\n\n[^n]: [[A]]\n", want: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 6, Stop: 18}, Name: graph.Span{Start: 6, Stop: 11}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote"}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 12, Stop: 17}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 12, Stop: 13}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 13, Stop: 14}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 14, Stop: 16}}}, {Kind: "Text", Depth: 2, Text: []graph.Span{{Start: 16, Stop: 17}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 1, Stop: 3}}}}},
		{name: "ordinary words have no literal or unused owner", body: "words [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.want, agreementNativeOwnerReading(tc.body)); diff != "" {
				t.Fatalf("caught: complete native owner source (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementNativeCheckSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		page       agreementNativeOwnerSource
		want       agreementNativeCheckSource
	}{
		{name: "whole context keeps inactive and active fields in order", body: "`[[A]]` [[B]] \\[[C]] %%[[D]]%% [[E#part|shown]]\n", page: agreementNativeOwnerSource{Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Span: graph.Span{Start: 1, Stop: 6}}}}, want: agreementNativeCheckSource{Code: []graph.Span{{Start: 1, Stop: 6}}, Comments: []graph.Span{{Start: 21, Stop: 30}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 1, Stop: 6}, Word: "[[A]]", Targets: []string{"A"}, Code: true, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 8, Stop: 13}, Word: "[[B]]", Targets: []string{"B"}, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 15, Stop: 20}, Word: "[[C]]", Targets: []string{"C"}, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 23, Stop: 28}, Word: "[[D]]", Targets: []string{"D"}, Comment: true, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 31, Stop: 47}, Word: "[[E#part|shown]]", Targets: []string{"E"}, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"B", "E"}}},
		{name: "raw check normalization keeps every spelling", body: "![[A\\]][[#part]][[A\nB]][[B&amp;C|shown]][[D]][[", want: agreementNativeCheckSource{Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 1, Stop: 7}, Word: "[[A\\]]", Targets: []string{"A"}, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 7, Stop: 16}, Word: "[[#part]]", Targets: []string{}, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 23}, Word: "[[A\nB]]", Targets: []string{}, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 23, Stop: 40}, Word: "[[B&amp;C|shown]]", Targets: []string{"B&amp;C"}, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 40, Stop: 45}, Word: "[[D]]", Targets: []string{"D"}, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B&amp;C", "D"}}},
		{name: "disjoint definitions keep explicit owner indexes", body: "[^n]: [[A]]\n\n[^m]: [[B]]\n", page: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Span: graph.Span{Start: 0, Stop: 13}}, {Span: graph.Span{Start: 13, Stop: 25}}}}, want: agreementNativeCheckSource{Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 19, Stop: 24}, Word: "[[B]]", Targets: []string{"B"}, DefinitionOwner: 1, CodeOwner: -1}}, Targets: []string{"A", "B"}}},
		{name: "disjoint code owners retain every original index", body: "[[A]] [[B]]", page: agreementNativeOwnerSource{Code: []agreementNativeOwnerCode{{Span: graph.Span{Start: 0, Stop: 5}}, {Span: graph.Span{Start: 6, Stop: 11}}}}, want: agreementNativeCheckSource{Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 6, Stop: 11}, Word: "[[B]]", Targets: []string{"B"}, DefinitionOwner: -1, CodeOwner: 1}}, Targets: []string{"A", "B"}}},
		{name: "duplicate definition ownership refuses", body: "[[A]]", page: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Span: graph.Span{Start: 0, Stop: 5}}, {Span: graph.Span{Start: 0, Stop: 5}}}}},
		{name: "partial definition ownership refuses", body: "[[A]]", page: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Span: graph.Span{Start: 1, Stop: 5}}}}},
		{name: "duplicate code ownership refuses", body: "[[A]]", page: agreementNativeOwnerSource{Code: []agreementNativeOwnerCode{{Span: graph.Span{Start: 0, Stop: 5}}, {Span: graph.Span{Start: 0, Stop: 5}}}}},
		{name: "partial code ownership refuses", body: "[[A]]", page: agreementNativeOwnerSource{Code: []agreementNativeOwnerCode{{Span: graph.Span{Start: 0, Stop: 4}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.want, agreementNativeCheckReading(tc.body, tc.page)); diff != "" {
				t.Fatalf("caught: complete whole-body check ownership ledger (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementNativeCheckOwners(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body         string
		code, unused, want map[string]int
		authority          string
	}{
		{name: "repeated code info fields retain occurrence counts", body: "``` [[A]] [[A]]\n```\n", code: map[string]int{"A": 2}, want: map[string]int{"A": 2}, authority: "#1011 stage 4"},
		{name: "whole check comments keep info fields inactive", body: "``` %%[[A]]%% [[B]]\n```\n", code: map[string]int{"B": 1}, want: map[string]int{"B": 1}, authority: "#1011 stage 4"},
		{name: "fence info is a declared literal owner", body: "``` [[A]]\n```\n", code: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 4"},
		{name: "native metadata belongs to discarded definition", body: "[^n]: [r]: [[A]]\n\n    ## heading\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "different owners combine the same check name exactly once", body: "[^n]: [[A]]\n\n``` [[A]]\n```\n", code: map[string]int{"A": 1}, unused: map[string]int{"A": 1}, want: map[string]int{"A": 2}, authority: "#1011 stages 4 and 5"},
		{name: "discarded inline code has definition priority", body: "[^n]: `[[A]]`\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "all definitions repeated targets and inactive physical indent", body: "[^n]: [[A]] [[A#part|shown]]\n\n    [[B]]\n\n[^m]: [[B]]\n", unused: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}, authority: "#1011 stage 5"},
		{name: "check code body cannot lend an active contribution", body: "``` [[A]]\n[[B]]\n```\n", code: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 4"},
		{name: "escaped field cannot lend active contribution", body: "[^n]: \\[[B]] [[A]]\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "comment overlap refuses entire native definition", body: "[^n]: [[A]] %%[[B]]%%\n"},
		{name: "missing reference of same name cannot borrow code budget", body: "[r]: [[A]]\n\n``` [[A]]\n```\n", code: map[string]int{"A": 1}},
		{name: "different live page target still refuses entire scope", body: "[[B]]\n\n``` [[A]]\n```\n", code: map[string]int{"A": 1}},
		{name: "live reference uses its definition", body: "ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "physical check indentation cannot borrow definition target", body: "[^n]:\n\n    ## [[A]]\n"},
		{name: "raw suffix target remains check spelling", body: "[^n]: [[A\\]]\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "Unicode and CRLF preserve physical ownership", body: "[^é]: words [[A]]\r\n\r\n``` [[B#part|shown]]\r\n```\r\n", code: map[string]int{"B": 1}, unused: map[string]int{"A": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "unclosed raw field carries no guessed target", body: "[^n]: [[A]] [[unfinished\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "quote definition owns original physical source", body: "> [^n]: [[A]]\n>\n>     [[B]]\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "list definition retains its container owner", body: "- [^n]: [[A]]\n\n      [[B]]\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "duplicate labels retain both physical declarations", body: "[^n]: [[A]]\n\n[^n]: [[B]]\n", unused: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}, authority: "#1011 stage 5"},
		{name: "nested emphasis and quote blocks retain whole discarded scope", body: "[^n]: words *[[A]]*\n\n    > [[B]]\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "page grammar alone discards URL reference", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A]]\n", unused: map[string]int{"A": 1}, want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "ordinary prose has no native owner", body: "words [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementNativeCheckBudget(tc.body)
			if diff := cmp.Diff(tc.code, budget.Code); diff != "" {
				t.Fatalf("caught: complete native code contribution budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.unused, budget.Unused); diff != "" {
				t.Fatalf("caught: complete native unused contribution budget (-want +got):\n%s", diff)
			}
			agreementNativeCheckReceipts(t, tc.body, budget, tc.want, tc.authority)
		})
	}
}

func agreementNativeCheckReceipts(t *testing.T, body string, budget agreementNativeCheckPayload, want map[string]int, authority string) {
	t.Helper()
	c := agreementCase{Body: body}
	r, actual := agreementIsolatedPage(t, c)
	var found map[string]int
	failures := agreementPageFailures(c.Body, &r, &actual)
	for i := range failures {
		f := &failures[i]
		kind, ruled, wrong := agreementNativeCheckDifference(c, f, &actual, budget)
		if f.Property != "P1" || f.Direction != "judge-only" || want[f.Tuple.Target] == 0 {
			if kind != "" {
				t.Fatalf("caught: native owner borrowed unowned public difference %s", agreementSignature(f))
			}
			continue
		}
		if kind != "debt" || wrong != "judge" || authority != "" && ruled != authority || ruled != "#1011 stage 4" && ruled != "#1011 stage 5" && ruled != "#1011 stages 4 and 5" {
			t.Fatalf("caught: native check public ownership %q %q %q %s", kind, ruled, wrong, agreementSignature(f))
		}
		if found == nil {
			found = make(map[string]int)
		}
		found[f.Tuple.Target] = f.Multiplicity
		agreementNativeCheckDrift(t, c, f, &actual, budget)
	}
	if diff := cmp.Diff(want, found); diff != "" {
		t.Fatalf("caught: complete native check public receipts (-want +got):\n%s", diff)
	}
}

func agreementNativeCheckDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementNativeCheckPayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementNativeCheckDifference(other, f, actual, budget); kind != "" {
			t.Fatal("caught: native check borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementNativeCheckDifference(c, &changed, actual, budget); kind != "" {
			t.Fatalf("caught: native check borrowed signature %s", agreementSignature(&changed))
		}
	}
	for _, extra := range []agreementCitation{{Target: f.Tuple.Target, State: "wikilink-broken"}, {Target: "other", State: "wikilink"}, {Target: f.Tuple.Target, SourceRole: agreementOutsideMarkdown, State: "wikilink-broken"}} {
		changed := *actual
		changed.Citations = append(append([]agreementCitation{}, actual.Citations...), extra)
		if kind, _, _ := agreementNativeCheckDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: native check borrowed live page carrier")
		}
	}
	changed := budget
	changed.Citation.Targets = map[string]int{f.Tuple.Target: f.Multiplicity + 1}
	if kind, _, _ := agreementNativeCheckDifference(c, f, actual, changed); kind != "" {
		t.Fatal("caught: native check borrowed complete contribution count")
	}
	changed = budget
	changed.Code = nil
	changed.Unused = nil
	if kind, _, _ := agreementNativeCheckDifference(c, f, actual, changed); kind != "" {
		t.Fatal("caught: native check borrowed owner partition")
	}
	changed = budget
	changed.Citation.Body = "unowned"
	if kind, _, _ := agreementNativeCheckDifference(c, f, actual, changed); kind != "" {
		t.Fatal("caught: native check borrowed source budget binding")
	}
}

func TestAgreementNativeCheckOriginals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		body string
		want map[string]int
	}{
		{body: "> [!note] title\n```` go [[A]]\n`[[A#A]][[A\nB]]", want: map[string]int{"A": 1}},
		{body: " ^a\n- item\n\n      A\n---\nA\n===\n> [!note] title\n```` go [[A]]\n1. # A\n# A\n ^A\n ```\n ^é\n   ```\n\n`open\n[[A]]\nclose`![[A]]## <em>A</em>\n ^A\n## !\n[[A#A]]`open\n[[A]]\nclose``open\n[[A]]\nclose`[^n]: [[A]]\n\n    [[B]]\n ```\n> >  ^A\n```` go [[A]]\n``> ", want: map[string]int{"A": 1}},
		{body: "1. ## A-2\n## A\n## A\n- > [!note] title\n\t```\n```` go [[A]]\n> [!note] title\n## A-2\n## A\n## A\n```\n\n\n![[A]]- [ ] [[A]]\ntext   - ", want: map[string]int{"A": 1}},
		{body: "[^unused]: [[A]]\n```` go [[A]]\n`> [!note] one\n> [!note] two\n> [!note] three\n- item\n\n      [[A\nB]][[A]] [[A]]`open\n[[A]]\nclose`- [x] [[B]]\n`[[A]]` ```\n    ``\\\\[[A]]<!-- [[A]] -->É\nÉ\n## A\n\n ^A\nref[^n]\n``` [[A]]\n[^n]: [[A]]\n\n    [[B]]\n## A\n## A\n", want: map[string]int{"A": 2}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n> https://example.invalid/`[[A]]` - \t```\n```` go [[A]]\nB\n## A\nref[^n]\n[^unused]: [[A]]\n0<div>\n[[A]]\n</div>\n\t> > ", want: map[string]int{"A": 2}},
		{body: "A\n===\n  ```\n> [!note] one\n> [!note] two\n> [!note] three\n[[A]]\\\\[[A]]`[[A]]`ref[^n]\n^absent-prose <!--\n[[A]]\n-->## A-2\n## A\n## A\n\t`[[A]]`\n\n ^é\n\\\\[[A]][[A]] [[A]]<div>\n[[A]]\n</div>\n- item\n\n      - [ ] [[A]]\n ```\n> [!note] one\n> [!note] two\n> [!note] three\n- \t```\n``` [[A]]\n^absent-prose <!--[[A#^a]]", want: map[string]int{"A": 1}},
		{body: "> [!note] title\n``` [[A]]\n ^A\n``` [[A]]\n[[A]] [[A]]``<!--\n\n  - ~~~~\nA\n> - [x] [[B]]\n![[image.png]]## <em>A</em>\n0\n\n[[A#^a]]章節\n## <em>A</em>\n章節\n     [[A\\|alias]]", want: map[string]int{"A": 1}},
		{body: "[^unused]: [[A]]\nB\n[[A\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    ```\n[[A\\|alias]][[#A]]   ```\n[[A\nB]]``` [[A]]\n[[A#^a]]`open\n[[A]]\nclose` ^é\n`![[A#A]][[A#A]]~~~\n", want: map[string]int{"A": 7, "image.png": 1}},
		{body: "[[A\nB]]ref[^n]\n[^unused]: [[A]]\n```` go [[A]]\nref[^n]\n> [!unknown] title\n~~~~\n[[A\\]]> [!note] title\n# A\n````\nÉ\n\n\n", want: map[string]int{"A": 2}},
		{body: "%%[[B|alias]]  > [!note] title\nref[^n]\n\\\\[[A]]    ```\n ^é\n`open\n[[A]]\nclose`<!--%%> [!unknown] title\n`````` go [[A]]\n````\n", want: map[string]int{"A": 1}},
		{body: "1. > [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n# A\n# A\n<!-- [[A]] -->[[A\\|alias]]```` go [[A]]\n## A\n## A\nB\n\\[[A]]![[image.png]]\t```\n\t", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n`> [!note] [[A]]\n ^a-2\n[^n]: [[A]]\n\n    [[B]]\n``` [[A]]\n## A\n![[A#A]][[A\nB]]A\n===\n## A\n## A\n\\`\\[[A]]`", want: map[string]int{"A": 4}},
		{body: "# A\n[^unused]: [[A]]\n[[image.png]][[A\\]]## <em>A</em>\n```` go [[A]]\nA\n---\n\t ^a-2\nA\n---\n ^é\n![[image.png]]\t[[#A]]> [!note] title\n[[A\\|alias]]## A\n## A\n<!--\n[[A]]\n-->## <em>A</em>\nÉ\nA\n===\nA\n---\n", want: map[string]int{"A": 3, "image.png": 1}},
		{body: "\n ^a-2\n> [!note] one\n> [!note] two\n> [!note] three\n```` go [[A]]\n ```\n ^é\n ^é\n    ```\n`open\n[[A]]\nclose`<div>\n[[A]]\n</div>\n## !\n~~~\nhttps://example.invalid/`[[A]]` -->````\n%%[[A]]%%[[A\\|alias]]> [!note] [[A]]\n``` [[A]]\n[[A\\]]| a | b |\n|---|---|\n| [[A]] | ^a |\n-->[[A\\|alias]]\\[[A]]", want: map[string]int{"A": 1}},
		{body: "    ```\n\n ^A\n[^n]: [[A]]\n\n    [[B]]\n```` go [[A]]\n\\```` [[A]]\n<div>\n[[A]]\n</div>\n[[A\\]]`- item\n\n       ^a-2\n ```\n\\[[A]]````\n[[A#A]]1. \t```\n1. [[A]]0\n\n[[A#A]]", want: map[string]int{"A": 2}},
		{body: "\\[[A]][[#A]]<!--\n[[A]]\n-->É\nA\n>  ^a\n章節\n- ## <em>A</em>\n[^n]: [[A]]\n\n    [[B]]\n章節\nB\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n", want: map[string]int{"A": 2}},
		{body: " A\n---\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n```\n ~~~\n1.      [[B|alias]]``` [[A]]\n", want: map[string]int{"A": 1}},
		{body: "> [!note] one\n> [!note] two\n> [!note] three\n1. https://example.invalid/`[[A]]`   -  ^é\n``` [[A]]\n ^a\nB\n## A\n## A\n`open\n[[A]]\nclose`[[A\nB]]", want: map[string]int{"A": 1}},
		{body: "- > [!note] title\n ``` [[A]]\n", want: map[string]int{"A": 1}},
		{body: "## A\n## A\n  > [!note] title\n<!-- [[A]] -->## <em>A</em>\n``` [[A]]\n ^a-2\n![[A]]- [x] [[B]]\nhttps://example.invalid/`[[A]]`     ```\n# A\n<!-- [[A]] -->```\n[[A\\]]", want: map[string]int{"A": 1}},
		{body: "> [!unknown] title\n``` [[A]]\nÉ\nref[^n]\n ^a\n ^a-2\n| a | b |\n|---|---|\n| [[A]] | ^a |\n ^A\n[[image.png]]%%>  ^A\n\ntext  ^a\n~~~~\nref[^n]\n    [[B|alias]]## [[A|alias]]\n", want: map[string]int{"A": 1}},
		{body: "> [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\n```` go [[A]]\n## A-2\n## A\n## A\n`open\n[[A]]\nclose`1. [[A\\|alias]]<!--\n[[A]]\n--><!-- [[A]] --><!-- [[A]] -->[[A\\]]%%ref[^n]\n[[A]] [[A]]## A\nhttps://example.invalid/`[[A]]` ## A\n## A\n0[[A\nB]]```\nhttps://example.invalid/`[[A]]`    ```\n    \\[[A]] ^A\n", want: map[string]int{"A": 2}},
		{body: "\\[[A]]- > [!note] title\n- item\n\n      > [!note] one\n> [!note] two\n> [!note] three\n- > [!note] title\n```` go [[A]]\n  - - [x] [[B]]\n## A\n## A\n## <em>A</em>\n[[A#A]]A\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[B|alias]]   ```\n![[A]]<!--\n[[A]]\n-->[[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n    <!-- [[A]] -->- [ ] [[A]]\nA\n---\n``` [[A]]\n```` go [[A]]\n[[image.png]]\\\\[[A]]", want: map[string]int{"A": 1}},
		{body: "> [!unknown] title\nA\n===\n1.  ^a\n``` [[A]]\n![[image.png]]\nref[^n]\n^absent-prose ## A\n## A\nA\n---\n> > text \t<!--\n[[A]]\n-->[[A\nB]] ^a-2\nhttps://example.invalid/`[[A]]` 0[[image.png]]`[[A]]`\n   ```\n<!--", want: map[string]int{"A": 1}},
		{body: "\t```\n[^unused]: [[A]]\n\\`\n``` [[A]]\n~~~~\n \t``# A\n[[A]] [[A]]> text -->`open\n[[A]]\nclose`<!-- [[A]] -->-->章節\n", want: map[string]int{"A": 2}},
		{body: "B\n    ```\n## A\n[^unused]: [[A]]\n[[A#A]]## <em>A</em>\n[[A#A]]- > [!note] title\n## A\n## A\n<!--[[A]][[A#^a]]## A\n## A\n## !\n", want: map[string]int{"A": 3}},
		{body: "  > [!note] title\n``` [[A]]\n## A-2\n## A\n## A\n> [!note] title\n```` go [[A]]\n``` [[A]]\n1. `[[A]]````\n| a | b |\n|---|---|\n| [[A]] | ^a |\n章節\n![[image.png]] ^A\n", want: map[string]int{"A": 1}},
		{body: "> [!note] one\n> [!note] two\n> [!note] three\n[^n]: [[A]]\n\n    [[B]]\n[^unused]: [[A]]\n`-->[[A#^a]]  ```\n[[B|alias]]", want: map[string]int{"A": 3, "B": 1}},
		{body: "- item\n\n       ^a\n  > [!note] title\n ^a-2\n  > [!note] title\n章節\n```` [[A]]\n0\n\n  - > > > \t[[image.png]]  ```\n0```` go [[A]]\n[[#A]]https://example.invalid/`[[A]]` - > [!note] title\n- [ ] [[A]]\n## <em>A</em>\n ```\nhttps://example.invalid/`[[A]]`  ^A\n[^unused]: [[A]]\n", want: map[string]int{"A": 1}},
		{body: "[^unused]: [[A]]\n![[A]]## [[A|alias]]\n\\[[A]]`open\n[[A]]\nclose````` go [[A]]\n- > [!note] title\n``` [[A]]\n[[A\nB]]%%> > > [!unknown] title\n\n\n~~~~\nA\n---\n[[B|alias]]> [!note] one\n> [!note] two\n> [!note] three\n> > 0[[A]] 0https://example.invalid/`[[A]]`  ^é\n## A\n## A\n", want: map[string]int{"A": 6}},
		{body: "> [!note] one\n> [!note] two\n> [!note] three\n ^a-2\n``` [[A]]\n- [x] [[B]]\nA\n===\n0    ^absent-prose ## <em>A</em>\n0\\\\[[A]]<!--\n[[A]]\n-->````\n[^n]: [[A]]\n\n    [[B]]\nÉ\n[[image.png]]~~~~\n[[A#^a]] ^A\n![[image.png]]- [x] [[B]]\n  ```\n", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n``` [[A]]\n  - <div>\n[[A]]\n</div>\n## A\n## A\ntext [[A\nB]]    ```\n^absent-prose [^unused]: [[A]]\n![[A#A]]  - [^n]: [[A]]\n\n    [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n<!-- [[A]] --> ^A\n>  0- [x] [[B]]\n> > ```\n`open\n[[A]]\nclose`\\`", want: map[string]int{"A": 2}},
		{body: "[^unused]: [[A]]\n```` go [[A]]\n%%text ## [[A|alias]]\n", want: map[string]int{"A": 2}},
		{body: "--> ```\n> ``` [[A]]\n ^a\n ^a-2\n%%", want: map[string]int{"A": 1}},
		{body: "  > [!note] title\n```` go [[A]]\n1. ## A\n## A\n``\n- item\n\n          ```\n ^é\n- [ ] [[A]]\n  - \\`[[A\\|alias]]- > [!note] title\n- <div>\n[[A]]\n</div>\n\n\n", want: map[string]int{"A": 1}},
		{body: "## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n ^é\n    ```\n > [!note] one\n> [!note] two\n> [!note] three\n~~~~\n[[A\nB]]`[[A]]`\\`\\`> [!unknown] title\n[[A\\]]- [[A\nB]]`open\n[[A]]\nclose`[[A\\]]", want: map[string]int{"A": 1}},
		{body: "> [!unknown] title\nÉ\n  > [!note] title\n``` [[A]]\n## !\n`[[A]]`\t```\n   ```\n<!--  ```\n", want: map[string]int{"A": 1}},
		{body: "> [!unknown] title\n\t```\n```` go [[A]]\n ^a-2\nA\n`open\n[[A]]\nclose`- [x] [[B]]\n ^é\n[[A]] [[A]]> [!note] [[A]]\n<!--\n[[A]]\n-->  ```\n\t```\nA\n===\n[[A#^a]]![[A#A]][[#A]]", want: map[string]int{"A": 1}},
		{body: " ^é\n``` [[A]]\n- > [!note] title\n# A\n> [!unknown] title\n`[[A]]`[[A]] [[A]]## !\n^absent-prose > [!unknown] title\n[[A\\]]^absent-prose   ```\n> [!note] one\n> [!note] two\n> [!note] three\n- [ ] [[A]]\n[[A]] [[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n> ````\ntext   ```\nB\n![[A#A]]A\n---\n![[A]]ref[^n]\n章節\nÉ\n   ```\n<!--", want: map[string]int{"A": 1}},
		{body: "[[#A]]É\n%%[[A\\|alias]]\n\n`# A\n> [!note] title\n> > É\n# A\n> [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n%%\\`## A-2\n## A\n## A\n## !\n```` go [[A]]\n## A\n## A\n", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n## !\n```` go [[A]]\n![[image.png]] [[A\\|alias]]", want: map[string]int{"A": 2}},
		{body: "```` go [[A]]\n    B\n\n\n`[[A]]`- item\n\n      https://example.invalid/`[[A]]`     ```\n# A\ntext   > [!note] title\n<div>\n[[A]]\n</div>\n````\n  ```\n1.  ^é\nA\n---\n![[A#A]] 01. É\n章節\n```\n[^unused]: [[A]]\n ^é\n[[A#^a]]", want: map[string]int{"A": 3}},
		{body: "## A\n[^n]: [[A]]\n\n    [[B]]\n ^a-2\n\\[[A]]- [ ] [[A]]\n## A\n## A\n-->B\n``` [[A]]\n", want: map[string]int{"A": 3}},
		{body: "A\n===\n0É\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]][[A\nB]] ^é\n> [!note] title\nÉ\n", want: map[string]int{"A": 1}},
		{body: "0  > [!note] title\n  - [^unused]: [[A]]\nref[^n]\n```` go [[A]]\n[[A#A]]\\\\[[A]]> > - [x] [[B]]\n  ```\n`open\n[[A]]\nclose`0A\n===\n| a | b |\n|---|---|\n| [[A]] | ^a |\nÉ\nhttps://example.invalid/`[[A]]`  ^é\nÉ\n> [!note] one\n> [!note] two\n> [!note] three\n\\`[[A\nB]]\n\n\\\\[[A]]", want: map[string]int{"A": 2}},
		{body: "- > [!note] title\n  > [!note] title\n``` [[A]]\n## A-2\n## A\n## A\nA\n---\n\\\\[[A]][[A\\|alias]]A\n===\n\t```\n", want: map[string]int{"A": 1}},
		{body: "`[[A]]`## A-2\n## A\n## A\n```\n ^a-2\n[[A#^a]]É\n<!--\n[[A]]\n-->0É\n~~~~\n\t~~~\n   ```\n  ```\nref[^n]\n\n  ```\n章節\nB\n- item\n\n      [[A\\|alias]]```\n> [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\nA\n===\nB\n## A-2\n## A\n## A\n``` [[A]]\n`open\n[[A]]\nclose`~~~~\n[[A\\|alias]]", want: map[string]int{"A": 2}},
		{body: "## <em>A</em>\n- > [!note] title\n[[A\nB]]  > [!note] title\n## A\n## A\n## A\n## A\n```` go [[A]]\n ```\n## [[A|alias]]\n> > ^absent-prose - item\n\n      %%[[A]]%%", want: map[string]int{"A": 1}},
		{body: "```` go [[A]]\n````\n-   - 1. text \n\n ^a-2\n%%\t ^a-2\n ^a-2\n\\`<div>\n[[A]]\n</div>\n<div>\n[[A]]\n</div>\n## <em>A</em>\nÉ\n[[A\\|alias]]> [!unknown] title\n ```\n~~~\n``` [[A]]\n", want: map[string]int{"A": 1}},
		{body: "- > [!note] title\n``` [[A]]\n\\`[[A]] [[A]][[A]]1. %%  - ref[^n]\n1. <!--\n[[A]]\n-->~~~~\n## <em>A</em>\n", want: map[string]int{"A": 1}},
		{body: "0<!-- [[A]] -->\n\n1. ```` go [[A]]\n%%![[A]]text  ^é\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## [[A|alias]]\nA\n\t```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n ^é\n![[A]]text [[#A]]`open\n[[A]]\nclose`", want: map[string]int{"A": 1}},
		{body: "<!--\n[[A]]\n-->%%[[A]]%%  - > [!note] one\n> [!note] two\n> [!note] three\nÉ\n``` [[A]]\n> \\[[A]]A\n[[#A]]`[[A]]` ^é\n", want: map[string]int{"A": 1}},
		{body: "> [!note] title\n\t0<!--<!-- [[A]] -->## !\n``` [[A]]\n## A-2\n## A\n## A\n ^a\n![[image.png]][^n]: [[A]]\n\n    [[B]]\n> [!note] title\n", want: map[string]int{"A": 1}},
		{body: "> [!note] one\n> [!note] two\n> [!note] three\n\t## <em>A</em>\n\n```` go [[A]]\n\n\n  - ## A\n## A\n## [[A|alias]]\n# A\n ^a\n## [[A|alias]]\n ^é\n## A\n[[A]] [[A]]## <em>A</em>\n`open\n[[A]]\nclose`B\n[[A\\|alias]]", want: map[string]int{"A": 1}},
		{body: "章節\n> ````\nref[^n]\n# A\n- item\n\n      https://example.invalid/`[[A]]` > [!note] one\n> [!note] two\n> [!note] three\nA\n---\n``` [[A]]\n0`[[A]]`- ```` go [[A]]\nref[^n]\n`[[A]]`## <em>A</em>\n   ```\n\t```\n> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n", want: map[string]int{"A": 1}},
		{body: "> [!unknown] title\n```` go [[A]]\n[[A\nB]]`open\n[[A]]\nclose`-     ```\n章節\n  > [!note] title\n`open\n[[A]]\nclose`~~~~\n`open\n[[A]]\nclose`[[A#A]]", want: map[string]int{"A": 1}},
		{body: "``` [[A]]\n```\n> [!note] one\n> [!note] two\n> [!note] three\n%%![[A#A]]<div>\n[[A]]\n</div>\n`[[A]]````\n![[image.png]]~~~\n``0\n\nÉ\n0- > [!note] title\n ![[image.png]] [[A#^a]][^unused]: [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[B|alias]]![[image.png]]\\`- item\n\n      ![[A#A]] ```\n`[[A]]`[[#A]][[A#A]]``", want: map[string]int{"A": 1}},
		{body: "- > [!note] title\n``` [[A]]\nhttps://example.invalid/`[[A]]` \\\\[[A]]", want: map[string]int{"A": 1}},
		{body: "`[[A]]`## A\n- > [!note] title\nref[^n]\n## A-2\n## A\n## A\n```` go [[A]]\n[^unused]: [[A]]\n0![[A#A]][[A#A]]text - [ ] [[A]]\n- item\n\n      \t```\n``[^n]: [[A]]\n\n    [[B]]\n  ```\n``` [[A]]\n`# A\n", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n![[A]][[A\nB]]\n\n    - ^absent-prose ![[A#A]][[A\\]][[image.png]]É\n ^é\nB\n`[[A]]`", want: map[string]int{"A": 2}},
		{body: "1. > [!unknown] title\n```` go [[A]]\n- [x] [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#^a]]\\\\[[A]]    ```\n[[A#^a]]A\n<!--- item\n\n      text ", want: map[string]int{"A": 1}},
		{body: "## !\n ## <em>A</em>\nhttps://example.invalid/`[[A]]` > [!note] title\n## A\n\\[[A]]B\n\\`> [!note] one\n> [!note] two\n> [!note] three\n````\n> [!unknown] title\n[[A#^a]]\t```\n[[image.png]]# A\n%%[^unused]: [[A]]\n> [!note] title\n\n## A\n## A\n## <em>A</em>\n````\n     ^A\n[[#A]] ^é\n```` go [[A]]\n  - `open\n[[A]]\nclose`![[A#A]]> [!note] title\n", want: map[string]int{"A": 1}},
		{body: "   ```\n~~~~\n ^a\n1. [[A\nB]]## A\n## A\n[[A\\]]    ```\n[^unused]: [[A]]\ntext  ^a-2\n  ```\n- > [!note] title\n1. ``` [[A]]\n  - ", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n[[#A]][[A\nB]][[A]] [[A]]\n`[[A]] [[A]]> [!note] title\n[[A]] [[A]]`[[A]]`text É\n![[A]] ^A\n ```\n> [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#A]]0ref[^n]\n[[A]][[A]] [[A]]<div>\n[[A]]\n</div>\n[[A\\]]-->\t```\nA\n===\n", want: map[string]int{"A": 5}},
		{body: "# A\n> [!note] one\n> [!note] two\n> [!note] three\n`````` go [[A]]\n  ```\n![[A#A]]    A\n---\n ^a\n> [!note] one\n> [!note] two\n> [!note] three\n ^A\n-->^absent-prose     ## <em>A</em>\n<!--\n[[A]]\n-->[[A]]\n0``> > <!-- [[A]] -->> [[image.png]]- item\n\n      \n\n  - ``` [[A]]\n## A\n## A\n[[A\nB]]", want: map[string]int{"A": 1}},
		{body: "# A\nB\n```text 0[^n]: [[A]]\n\n    [[B]]\n ```\n%%", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\nB\n[[A#A]]   ```\ntext \\[[A]]>   > [!note] title\n ```\n> [!note] title\n[[A#A]]<!--> [!unknown] title\n[[A#A]]\\\\[[A]]- [[A\\|alias]]É\n[[A\\]]0> [!note] one\n> [!note] two\n> [!note] three\n- [[image.png]][[A#A]]", want: map[string]int{"A": 2}},
		{body: "[^unused]: [[A]]\n[[A]]`open\n[[A]]\nclose`<div>\n[[A]]\n</div>\n- > [!note] title\n<!-- [[A]] -->````\n\\[[A]]## A\ntext  ```\n", want: map[string]int{"A": 3}},
		{body: "``` [[A]]\n[[A#^a]] ## <em>A</em>\n~~~~\nÉ\n```\n[^n]: [[A]]\n\n    [[B]]\n![[A]]", want: map[string]int{"A": 3}},
		{body: "ref[^n]\n  > [!note] title\n```` go [[A]]\n-->\\\\[[A]]- > [!note] title\n`[[A]]`> [!note] one\n> [!note] two\n> [!note] three\n# A\n", want: map[string]int{"A": 1}},
		{body: "- item\n\n      ````\n[^n]: [[A]]\n\n    [[B]]\n```` go [[A]]\n", want: map[string]int{"A": 2}},
		{body: "    - > [!note] title\n  > [!note] title\n<!--\n[[A]]\n-->  ```\nA\n```` go [[A]]\n![[A#A]]\\\\[[A]]", want: map[string]int{"A": 1}},
		{body: "> [!note] title\n``` [[A]]\n%%| a | b |\n|---|---|\n| [[A]] | ^a |\n[^unused]: [[A]]\n> > - item\n\n      https://example.invalid/`[[A]]`  ^é\n[[A#^a]]- > [!note] title\n`open\n[[A]]\nclose`\n\n--> `[[A]]`> [!note] title\n[[A\\]]   ```\n> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"A": 1}},
		{body: " > [!note] one\n> [!note] two\n> [!note] three\n```` go [[A]]\nÉ\n~~~~\n 0 ```\n1. %%A\nhttps://example.invalid/`[[A]]` <div>\n[[A]]\n</div>\n0", want: map[string]int{"A": 1}},
		{body: "B\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/`[[A]]`   > [!note] title\n    ## A\n## A\n ^a\n```\nB\n```` go [[A]]\n## A\n## A\n<!--## A\n- item\n\n      1. ## !\n# A\n- | a | b |\n|---|---|\n| [[A]] | ^a |\n\n\n^absent-prose ", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n\t0- [x] [[B]]\n\n> > B\n```` go [[A]]\n\t```\n`open\n[[A]]\nclose`> [!unknown] title\n[[image.png]]^absent-prose ", want: map[string]int{"A": 2}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[A\\]]`[[A]]`   ```\n[[A#^a]][[B|alias]]章節\n````\n\\\\[[A]]^absent-prose \nref[^n]\n```` go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n", want: map[string]int{"A": 3, "B": 1}},
		{body: "%%[[A]]%%    # A\n``` [[A]]\n ^a\n\\[[A]]ref[^n]\n[[A\\]]> <!--\n[[A]]\n-->", want: map[string]int{"A": 1}},
		{body: "-->    ```\n ^é\n- > [!note] title\n```\n~~~\n[^n]: [[A]]\n\n    [[B]]\nA\n===\n\\`A\n> [!unknown] title\nhttps://example.invalid/`[[A]]` https://example.invalid/`[[A]]` [[A#^a]]~~~\n   ```\n\n\n[^unused]: [[A]]\n```` go [[A]]\n`open\n[[A]]\nclose`[[A\\|alias]]- [ ] [[A]]\n ```\n![[A]]ref[^n]\ntext \n\n  ```\n![[A]]- [ ] [[A]]\n", want: map[string]int{"A": 2}},
		{body: "- > [!note] title\n``` [[A]]\n\n[[A]] [[A]]    %%\\`ref[^n]\n`[[A]]`> [!note] [[A]]\n^absent-prose ![[A#A]]-->    https://example.invalid/`[[A]]` 1. ## !\nref[^n]\n## <em>A</em>\nref[^n]\n  -  ^é\n![[A#A]]![[image.png]]## A-2\n## A\n## A\n![[A]]```` go [[A]]\n", want: map[string]int{"A": 1}},
		{body: "> [!note] title\n```` go [[A]]\n## A-2\n## A\n## A\nÉ\n ^a-2\n- [ ] [[A]]\n ^A\n ^a\n   ```\n![[A]]- \t```\nÉ\nhttps://example.invalid/`[[A]]` [[#A]]```\n![[A#A]] ^a\n```` go [[A]]\n<div>\n[[A]]\n</div>\n```` go [[A]]\n    - > [!note] title\n`````` go [[A]]\n ^a\n## A-2\n## A\n## A\n[[B|alias]]", want: map[string]int{"A": 1}},
		{body: "[^n]: [[A]]\n\n    [[B]]\n[[A\\|alias]]   ```\n```` go [[A]]\n  ```\n    > [!note] one\n> [!note] two\n> [!note] three\n- [x] [[B]]\n`[[A]]`## [[A|alias]]\n```` go [[A]]\n- 0- [x] [[B]]\n\t```\n ^a-2\n- item\n\n      ", want: map[string]int{"A": 3}},
		{body: "> [!note] title\n- ``` [[A]]\n\n\n    ```\n\\[[A]]\n\n[^n]: [[A]]\n\n    [[B]]\n## A\n章節\n  > [!note] title\n## A\n## A\n`[[B|alias]]`## !\n~~~~\nÉ\n ^é\n", want: map[string]int{"A": 2}},
		{body: "- > [!note] title\n`[[A]]`    ```\n```` go [[A]]\nB\n>     [[A#A]] ## <em>A</em>\n![[A#A]]# A\nB\n> [!unknown] title\n\n## A\n> [!unknown] title\nhttps://example.invalid/`[[A]]` <!-- [[A]] -->A\n---\n<div>\n[[A]]\n</div>\n ^a-2\n", want: map[string]int{"A": 1}},
		{body: " > [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\n\t```\n<!-- [[A]] -->> [!note] title\n<!--[^n]: [[A]]\n\n    [[B]]\n[^n]: [[A]]\n\n    [[B]]\n    ```\n> > [!unknown] title\n ^é\n> 章節\n<div>\n[[A]]\n</div>\n<div>\n[[A]]\n</div>\n ^a\n ^A\n    ```\n\\[[A]]- [ ] [[A]]\n``` [[A]]\n<!--\n[[A]]\n-->", want: map[string]int{"A": 1}},
	} {
		t.Run(tc.body, func(t *testing.T) {
			t.Parallel()
			agreementNativeCheckReceipts(t, tc.body, agreementNativeCheckBudget(tc.body), tc.want, "")
		})
	}
}

func TestAgreementNativeCheckCannotBorrowOtherCheckOwner(t *testing.T) {
	t.Parallel()
	c := agreementCase{Body: "[r]: [[A]]\n\n``` [[A]]\n```\n"}
	budget := agreementNativeCheckBudget(c.Body)
	r, actual := agreementIsolatedPage(t, c)
	if len(actual.Citations) != 0 {
		t.Fatal("caught: native check counterexample lost empty page scope")
	}
	failures := agreementPageFailures(c.Body, &r, &actual)
	for i := range failures {
		f := failures[i]
		if f.Property != "P1" || f.Tuple.Target != "A" {
			continue
		}
		if f.Multiplicity != 2 {
			t.Fatal("caught: native check counterexample lost whole check multiplicity")
		}
		f.Multiplicity = 1
		if kind, _, _ := agreementNativeCheckDifference(c, &f, &actual, budget); kind != "" {
			t.Fatal("caught: native check borrowed another physical check owner")
		}
		return
	}
	t.Fatal("caught: native check counterexample lost public difference")
}
