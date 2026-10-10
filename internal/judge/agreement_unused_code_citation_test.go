package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

// Literal code keeps its own source role inside a discarded declaration.
// Read every physical definition with its label and indentation: the check can
// give those same bytes a different block role. Outside owners cannot lend counts.
const agreementUnusedInlineCodeField = "unused-inline-code"

type agreementUnusedCodeCitationSource struct {
	Citation                       agreementUnusedCitationSource
	Lines, Code, Context, Comments []graph.Span
}

func agreementUnusedCodeCitationReading(body string, grammar goldmark.Markdown) agreementUnusedCodeCitationSource {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var reading agreementUnusedCodeCitationSource
	roots := []ast.Node{doc}
	for _, definition := range unused {
		roots = append(roots, definition)
	}
	for _, root := range roots {
		if err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering {
				if span, code := agreementNativeCodeSpan(node); code && !span.Zero() {
					reading.Context = append(reading.Context, span)
				}
				if fence, fenced := node.(*ast.FencedCodeBlock); fenced && fence.Info != nil {
					reading.Context = append(reading.Context, graph.Span{Start: fence.Info.Segment.Start, Stop: fence.Info.Segment.Stop})
				}
			}
			return ast.WalkContinue, nil
		}); err != nil {
			panic(err)
		}
	}
	for _, definition := range unused {
		for node := definition.FirstChild(); node != nil; node = node.NextSibling() {
			switch node.(type) {
			case *ast.Paragraph, *ast.TextBlock:
			default:
				return agreementUnusedCodeCitationSource{}
			}
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				switch child.(type) {
				case *ast.Text:
				case *ast.CodeSpan:
					for piece := child.FirstChild(); piece != nil; piece = piece.NextSibling() {
						literal, plain := piece.(*ast.Text)
						if !plain {
							return agreementUnusedCodeCitationSource{}
						}
						reading.Code = append(reading.Code, graph.Span{Start: literal.Segment.Start, Stop: literal.Segment.Stop})
					}
				default:
					return agreementUnusedCodeCitationSource{}
				}
			}
		}
	}
	if len(reading.Code) == 0 {
		return agreementUnusedCodeCitationSource{}
	}
	reading.Comments = graph.CommentZones(body, reading.Context)
	for _, definition := range unused {
		start, stop := -1, -1
		for node := definition.FirstChild(); node != nil; node = node.NextSibling() {
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				physical := bytes.LastIndexByte(source[:line.Start], '\n') + 1
				if start < 0 || physical < start {
					if strings.Contains(string(source[physical:line.Start]), "[[") {
						return agreementUnusedCodeCitationSource{}
					}
					start = physical
				}
				stop = max(stop, line.Stop)
				reading.Lines = append(reading.Lines, graph.Span{Start: line.Start, Stop: line.Stop})
				raw := string(source[line.Start:line.Stop])
				offset := 0
				for offset < len(raw) {
					open := strings.Index(raw[offset:], "[[")
					if open < 0 {
						break
					}
					open += offset
					end := strings.Index(raw[open+2:], "]]")
					if end < 0 {
						return agreementUnusedCodeCitationSource{}
					}
					end += open + 2
					inner := raw[open+2 : end]
					if strings.ContainsAny(inner, "[]\r\n`") || graph.EscapedWikilinkAt(raw, open) {
						return agreementUnusedCodeCitationSource{}
					}
					link, cites := graph.ParseWikilink(inner)
					if !cites || link.Block != "" {
						return agreementUnusedCodeCitationSource{}
					}
					fieldStart := open
					if fieldStart > 0 && raw[fieldStart-1] == '!' {
						fieldStart--
					}
					offset = end + 2
					field := agreementUnusedWidgetField{Span: graph.Span{Start: line.Start + fieldStart, Stop: line.Start + offset}, Word: raw[fieldStart:offset], Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}}
					if agreementFieldContained(reading.Code, field.Span.Start, field.Span.Stop) {
						field.Tuple.SourceRole = agreementUnusedInlineCodeField
					} else if agreementUnusedWidgetCommentOwned(field.Span, reading.Code) {
						return agreementUnusedCodeCitationSource{}
					}
					if agreementUnusedWidgetCommentOwned(field.Span, reading.Comments) {
						return agreementUnusedCodeCitationSource{}
					}
					reading.Citation.Fields = append(reading.Citation.Fields, field)
				}
			}
		}
		if start < 0 {
			return agreementUnusedCodeCitationSource{}
		}
		reading.Citation.Definitions = append(reading.Citation.Definitions, agreementUnusedCheckDefinition{Span: graph.Span{Start: start, Stop: stop}, Targets: judge.LinkTargets(string(source[start:stop]))})
	}
	return reading
}

func agreementUnusedCodeCitationBudget(body string) agreementUnusedCitationPayload {
	plain := agreementUnusedCodeCitationReading(body, agreementFootnoteGrammar)
	gfm := agreementUnusedCodeCitationReading(body, agreementExclusiveCodeGrammar)
	if len(plain.Citation.Fields) == 0 || !cmp.Equal(plain, gfm) {
		return agreementUnusedCitationPayload{}
	}
	var targets map[string]int
	for _, definition := range plain.Citation.Definitions {
		for _, target := range definition.Targets {
			if targets == nil {
				targets = make(map[string]int)
			}
			targets[target]++
		}
	}
	return agreementUnusedCitationPayload{Body: body, Targets: targets}
}

func TestAgreementUnusedCodeCitationSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	codeA := a
	codeA.SourceRole = agreementUnusedInlineCodeField
	codeB := b
	codeB.SourceRole = agreementUnusedInlineCodeField
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedCodeCitationSource
		page       *agreementUnusedCodeCitationSource
	}{
		{name: "whole prose every repeated field and literal role", body: "[[A]]\n\n[^n]: words [[A]] `[[B]]` [[A]]\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 19, Stop: 24}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 26, Stop: 31}, Word: "[[B]]", Tuple: codeB}, {Span: graph.Span{Start: 33, Stop: 38}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 7, Stop: 38}, Targets: []string{"A", "A"}}}}, Lines: []graph.Span{{Start: 13, Stop: 38}}, Code: []graph.Span{{Start: 26, Stop: 31}}, Context: []graph.Span{{Start: 26, Stop: 31}}}},
		{name: "first line code becomes check reference syntax", body: "[^n]: `[[A]]`\n\n[^m]: [[B]]\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 7, Stop: 12}, Word: "[[A]]", Tuple: codeA}, {Span: graph.Span{Start: 21, Stop: 26}, Word: "[[B]]", Tuple: b}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 13}, Targets: []string{"A"}}, {Span: graph.Span{Start: 15, Stop: 26}, Targets: []string{"B"}}}}, Lines: []graph.Span{{Start: 6, Stop: 13}, {Start: 21, Stop: 26}}, Code: []graph.Span{{Start: 7, Stop: 12}}, Context: []graph.Span{{Start: 7, Stop: 12}}}},
		{name: "later paragraph retains original indentation", body: "[^n]: [[A]]\n\n    `[[B]]` [[A]]\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 18, Stop: 23}, Word: "[[B]]", Tuple: codeB}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 30}, Targets: []string{"A"}}}}, Lines: []graph.Span{{Start: 6, Stop: 11}, {Start: 17, Stop: 30}}, Code: []graph.Span{{Start: 18, Stop: 23}}, Context: []graph.Span{{Start: 18, Stop: 23}}}},
		{name: "all definitions retain their code pieces", body: "[^n]: [[A]] `[[B]]`\n\n[^m]: `[[B]]` [[A]]\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 13, Stop: 18}, Word: "[[B]]", Tuple: codeB}, {Span: graph.Span{Start: 28, Stop: 33}, Word: "[[B]]", Tuple: codeB}, {Span: graph.Span{Start: 35, Stop: 40}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 19}, Targets: []string{"A"}}, {Span: graph.Span{Start: 21, Stop: 40}, Targets: []string{"A"}}}}, Lines: []graph.Span{{Start: 6, Stop: 19}, {Start: 27, Stop: 40}}, Code: []graph.Span{{Start: 13, Stop: 18}, {Start: 28, Stop: 33}}, Context: []graph.Span{{Start: 13, Stop: 18}, {Start: 28, Stop: 33}}}},
		{name: "all wrapped pieces and physical rows", body: "[^n]: [[A]] `open\n[[B]]\nclose` [[A]]\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 18, Stop: 23}, Word: "[[B]]", Tuple: codeB}, {Span: graph.Span{Start: 31, Stop: 36}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 36}, Targets: []string{"A", "A"}}}}, Lines: []graph.Span{{Start: 6, Stop: 18}, {Start: 18, Stop: 24}, {Start: 24, Stop: 36}}, Code: []graph.Span{{Start: 13, Stop: 18}, {Start: 18, Stop: 24}, {Start: 24, Stop: 29}}, Context: []graph.Span{{Start: 13, Stop: 29}}}},
		{name: "CRLF wrapped pieces keep physical byte coordinates", body: "[^n]: [[A]] `open\r\n[[B]]\r\nclose` [[A]]\r\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 19, Stop: 24}, Word: "[[B]]", Tuple: codeB}, {Span: graph.Span{Start: 33, Stop: 38}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 38}, Targets: []string{"A", "A"}}}}, Lines: []graph.Span{{Start: 6, Stop: 19}, {Start: 19, Stop: 26}, {Start: 26, Stop: 38}}, Code: []graph.Span{{Start: 13, Stop: 19}, {Start: 19, Stop: 26}, {Start: 26, Stop: 31}}, Context: []graph.Span{{Start: 13, Stop: 31}}}},
		{name: "Unicode alias and raw suffix retain original bytes", body: "[^n]: 純 [[A#part|shown]] `[[B\\]]` end\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 10, Stop: 26}, Word: "[[A#part|shown]]", Tuple: agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}}, {Span: graph.Span{Start: 28, Stop: 34}, Word: "[[B\\]]", Tuple: agreementCitation{SourceRole: agreementUnusedInlineCodeField, Target: "B\\", State: "wikilink-broken"}}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 39}, Targets: []string{"A"}}}}, Lines: []graph.Span{{Start: 6, Stop: 39}}, Code: []graph.Span{{Start: 28, Stop: 34}}, Context: []graph.Span{{Start: 28, Stop: 34}}}},
		{name: "embeds retain their entire raw words and code roles", body: "[^n]: ![[A#part|shown]] `![[B\\]]` [[A]]\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 23}, Word: "![[A#part|shown]]", Tuple: agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}}, {Span: graph.Span{Start: 25, Stop: 32}, Word: "![[B\\]]", Tuple: agreementCitation{SourceRole: agreementUnusedInlineCodeField, Target: "B\\", State: "wikilink-broken"}}, {Span: graph.Span{Start: 34, Stop: 39}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 39}, Targets: []string{"A", "A"}}}}, Lines: []graph.Span{{Start: 6, Stop: 39}}, Code: []graph.Span{{Start: 25, Stop: 32}}, Context: []graph.Span{{Start: 25, Stop: 32}}}},
		{name: "fieldless code still retains its declaration", body: "[^n]: `words`\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 13}, Targets: []string{}}}}, Lines: []graph.Span{{Start: 6, Stop: 13}}, Code: []graph.Span{{Start: 7, Stop: 12}}, Context: []graph.Span{{Start: 7, Stop: 12}}}},
		{name: "used and unused code context protects every percent delimiter", body: "`%%`\n\n[^n]: [[A]] `%%`\n[[B]]\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 23, Stop: 28}, Word: "[[B]]", Tuple: b}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 6, Stop: 28}, Targets: []string{"A", "B"}}}}, Lines: []graph.Span{{Start: 12, Stop: 23}, {Start: 23, Stop: 28}}, Code: []graph.Span{{Start: 19, Stop: 21}}, Context: []graph.Span{{Start: 1, Stop: 3}, {Start: 19, Stop: 21}}}},
		{name: "whole native context retains fence body and info", body: "``` %%\ntext\n```\n\n[^n]: [[A]] `code`\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 23, Stop: 28}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 17, Stop: 35}, Targets: []string{"A"}}}}, Lines: []graph.Span{{Start: 23, Stop: 35}}, Code: []graph.Span{{Start: 30, Stop: 34}}, Context: []graph.Span{{Start: 7, Stop: 12}, {Start: 4, Stop: 6}, {Start: 30, Stop: 34}}}},
		{name: "every independent closed comment remains recorded", body: "%%hidden%%\n\n%%other%%\n\n[^n]: [[A]] `code`\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 29, Stop: 34}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 23, Stop: 41}, Targets: []string{"A"}}}}, Lines: []graph.Span{{Start: 29, Stop: 41}}, Code: []graph.Span{{Start: 36, Stop: 40}}, Context: []graph.Span{{Start: 36, Stop: 40}}, Comments: []graph.Span{{Start: 0, Stop: 10}, {Start: 12, Stop: 21}}}},
		{name: "plain source remains prior ownership", body: "[^n]: [[A]]\n"},
		{name: "used source remains visible", body: "ref[^n]\n\n[^n]: [[A]] `[[B]]`\n"},
		{name: "ordinary code has no discarded owner", body: "[[A]] `[[B]]`\n"},
		{name: "heading remains another owner", body: "[^n]: [[A]] `code`\n\n    ## [[B]]\n"},
		{name: "fenced code remains another owner", body: "[^n]: [[A]] `code`\n\n    ```\n    [[B]]\n    ```\n"},
		{name: "emphasis remains another owner", body: "[^n]: [[A]] *[[B]]* `code`\n"},
		{name: "URL retains CommonMark text and GFM owner", body: "[^n]: [[A]] https://example.invalid/[[B]] `code`\n", want: agreementUnusedCodeCitationSource{Citation: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 36, Stop: 41}, Word: "[[B]]", Tuple: b}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 48}, Targets: []string{"A", "B"}}}}, Lines: []graph.Span{{Start: 6, Stop: 48}}, Code: []graph.Span{{Start: 43, Stop: 47}}, Context: []graph.Span{{Start: 43, Stop: 47}}}, page: &agreementUnusedCodeCitationSource{}},
		{name: "reference brackets remain another owner", body: "[A]: /elsewhere\n\n[^n]: [[B]] [[A]] `code`\n"},
		{name: "comment overlap refuses whole source", body: "[^n]: [[A]] `code` %%[[B]]%%\n"},
		{name: "hidden field cannot supply check contribution", body: "%%\n[^n]: [[A]] `code`\n%%\n\n[r]: [[A]]\n"},
		{name: "incomplete field refuses whole set", body: "[^n]: [[A]] `code` [[B\n"},
		{name: "nested field refuses whole set", body: "[^n]: [[A]] `code` [[B[[C]]]]\n"},
		{name: "cross line field refuses whole set", body: "[^n]: [[A]] `code` [[B\nC]]\n"},
		{name: "escaped field refuses whole set", body: "[^n]: [[A]] `code` \\[[B]]\n"},
		{name: "local field refuses whole set", body: "[^n]: [[A]] `code` [[#B]]\n"},
		{name: "block field refuses whole set", body: "[^n]: [[A]] `code` [[B#^b]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for i, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				want := tc.want
				if i == 1 && tc.page != nil {
					want = *tc.page
				}
				if diff := cmp.Diff(want, agreementUnusedCodeCitationReading(tc.body, grammar)); diff != "" {
					t.Fatalf("caught: complete unused code citation source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementUnusedCodeCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body                 string
		budget, old, want, refused map[string]int
	}{
		{name: "shared live fields and literal payload", body: "[[A]]\n\n[^n]: words [[A]] `[[B]]` [[A]]\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "all visible counts remain ordinary", body: "[[A]] [[A]] [[B]]\n\n[^n]: words [[A]] [[B]] `code`\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "all physical definitions", body: "[^n]: [[A]] `[[B]]`\n\n[^m]: `[[B]]` [[A]]\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "first line code has original reference role", body: "[^n]: `[[A]]`\n\n[^m]: [[B]]\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "later code and live fields retain indentation", body: "[^n]: [[A]]\n\n    `[[B]]` [[A]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "wrapped native code retains source count", body: "[^n]: [[A]] `open\n[[B]]\nclose` [[A]]\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "both native code contexts preserve percent markers", body: "`%%`\n\n[^n]: [[A]] `%%`\n[[B]]\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "aliases sections suffixes and embeds use original check", body: "[^n]: words ![[A#part|shown]] [[A\\]] `[[B]]`\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "missing reference remains separate", body: "[r]: [[A]]\n\n[^n]: [[A]] `code`\n", budget: map[string]int{"A": 1}, want: nil},
		{name: "literal reference role cannot borrow missing owner", body: "[r]: [[A]]\n\n[^n]: `[[A]]`\n", budget: map[string]int{"A": 1}, want: nil},
		{name: "hidden source cannot borrow missing owner", body: "%%\n[^n]: [[A]] `code`\n%%\n\n[r]: [[A]]\n", budget: nil, want: nil},
		{name: "comment source refuses contribution", body: "[^n]: [[A]] `code` %%[[B]]%%\n", budget: nil, want: nil},
		{name: "plain old source stays outside", body: "[^n]: [[A]]\n", budget: nil, old: map[string]int{"A": 1}, want: nil},
		{name: "used source stays outside", body: "ref[^n]\n\n[^n]: [[A]] `[[B]]`\n", budget: nil, want: nil},
		{name: "grammar dependent use remains outside", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]] `code`\n", budget: nil, want: nil},
		{name: "other inline owner remains outside", body: "[^n]: [[A]] *[[B]]* `code`\n", budget: nil, want: nil},
		{name: "other block owner remains outside", body: "[^n]: [[A]] `code`\n\n    ## [[B]]\n", budget: nil, want: nil},
		{name: "ordinary source remains outside", body: "[[A]] `[[B]]`\n", budget: nil, want: nil},
		{name: "full original native unused code 0", body: "[^unused]: [[A]]\n``open\n[[A]]\nclose`0  > [!note] title\n`open\n[[A]]\nclose````````\n ^A\nÉ\n<div>\n[[A]]\n</div>\nA\n---\n[[A]]## A\n ^A\n   ```\n`~~~~\n ^a-2\n  > [!note] title\nA\n===\n> [!unknown] title\n<!--A\n---\n", budget: map[string]int{"A": 3}, want: map[string]int{"A": 3}},
		{name: "full original native unused code 1", body: "> [!note] one\n> [!note] two\n> [!note] three\n[[#A]]<!--\n[[A]]\n-->~~~\n[^unused]: [[A]]\n`[[A]]`章節\n````\n\\\\[[A]]<!--\n[[A]]\n-->## [[A|alias]]\n- [x] [[B]]\n<div>\n[[A]]\n</div>\nÉ\n[[B|alias]]É\n| a | b |\n|---|---|\n| [[A]] | ^a |\n1. \t```\n<div>\n[[A]]\n</div>\n![[A#A]]`open\n[[A]]\nclose`\\\\[[A]]# A\n![[A]]<!--\n[[A]]\n-->`\t```\n```\nÉ\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "full original native unused code 2", body: "\\\\[[A]]    ```\n> [!note] title\nA\n===\n%%[[A]]%%## [[A|alias]]\n  - 01. [[B|alias]]1. ```` go [[A]]\n[^n]: [[A]]\n\n    [[B]]\n`open\n[[A]]\nclose`\t```\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "full original native unused code 3", body: "%%[[A]]%%# A\n  > [!note] title\n```\n## A\n- [ ] [[A]]\n## !\n\n\\[[A]]ref[^n]\n```\n ^a-2\n [^n]: [[A]]\n\n    [[B]]\n ^a-2\n`[[A]]`| a | b |\n|---|---|\n| [[A]] | ^a |\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "full original native unused code 4", body: "- 章節\n- [ ] [[A]]\n## A-2\n## A\n## A\n[[A#^a]]`open\n[[A]]\nclose`É\n\n\n![[A]]~~~~\n[[A\\]][[A]]`[[A\nB]][[B|alias]]~~~\n\t```\n%%   ```\n## <em>A</em>\n[^unused]: [[A]]\n[[A#A]][[A\\|alias]]> [!unknown] title\n[[image.png]][[A]]`open\n[[A]]\nclose`B\n\n\n1. ", budget: map[string]int{"A": 4, "image.png": 1}, want: map[string]int{"A": 4, "image.png": 1}},
		{name: "full original native unused code 5", body: "[^n]: [[A]]\n\n    [[B]]\n`open\n[[A]]\nclose`A\n---\n> [!note] one\n> [!note] two\n> [!note] three\n\\[[A]]![[A#A]]![[image.png]][[A#A]]> > - [x] [[B]]\n> > - [x] [[B]]\n~~~\n%%[[A]]%%%%<!--<!--## [[A|alias]]\n- 1. \\`A\n-->- item\n\n      [[A]] ^é\n## !\n\n\n> [!note] [[A]]\n<!-- ^a-2\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "full original native unused code 6", body: "     ^é\n0## !\n ````\n````\n章節\n  > [!note] title\n- item\n\n      [^n]: [[A]]\n\n    [[B]]\n- > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n[[A\\|alias]]`open\n[[A]]\nclose` ^a\n~~~\n``` [[A]]\n0## A\n[[A]]```` go [[A]]\n![[A#A]]- [ ] [[A]]\n ^é\n> [!note] title\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "full original native unused code 7", body: "- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]]![[A]]~~~~\n~~~\n~~~\nref[^n]\n[^unused]: [[A]]\n![[image.png]]https://example.invalid/`[[A]]` --> ```\ntext ~~~~\n章節\n[^n]: [[A]]\n\n    [[B]]\n``", budget: map[string]int{"A": 1, "image.png": 1}, want: map[string]int{"A": 1, "image.png": 1}},
		{name: "full original native unused code 8", body: "![[A#A]]  ```\n ```\n   ```\n  > [!note] title\n> >  ^é\n## !\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->[[B|alias]]  > [!note] title\n[^unused]: [[A]]\n`open\n[[A]]\nclose`## A-2\n## A\n## A\n- > [!note] title\n  - ", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "full original native unused code 9", body: "章節\n- > [!note] title\n\\`[^unused]: [[A]]\n~~~\n~~~\n## !\n\n ^é\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\nA\n`[[A]]`- [x] [[B]]\n[[A]]", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2}, refused: map[string]int{"B": 2}},
		{name: "full original native unused code 10", body: "    ```\n  ```\n\n ```\n[^n]: [[A]]\n\n    [[B]]\n[[A\\|alias]]`open\n[[A]]\nclose`  > [!note] title\n```\n[^unused]: [[A]]\n![[image.png]]    ```\n````\n## [[A|alias]]\n>    ```\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "full original context disagreement refuses", body: "%%## [[A|alias]]\n- > [!note] title\ntext  ```\nhttps://example.invalid/`[[A]]` [[A\\|alias]] ^é\n## A\n## A\n[[A#A]]`open\n[[A]]\nclose`%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n`[[A]]`É\n0```\n章節\n-->\\\\[[A]]É\n## A\n## A\nÉ\n[[A#A]]   ```\n[[A]] [[A]]```` go [[A]]\n", budget: nil, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementUnusedCodeCitationBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Targets); diff != "" {
				t.Fatalf("caught: complete unused code citation budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementUnusedTextCitationBudget(c.Body).Targets); diff != "" {
				t.Fatalf("caught: changed earlier unused text citation boundary (-want +got):\n%s", diff)
			}
			r, a := agreementIsolatedPage(t, c)
			var found, refused map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &a) {
				kind, authority, wrong := agreementSharedUnusedCitationDifference(c, &f, &a, budget)
				if f.Property == "P1" && f.Direction == "judge-only" && tc.refused[f.Tuple.Target] != 0 {
					if f.Multiplicity != tc.refused[f.Tuple.Target] || kind != "" {
						t.Fatalf("caught: unused code citation borrowed another source owner %s", agreementSignature(&f))
					}
					if refused == nil {
						refused = make(map[string]int)
					}
					refused[f.Tuple.Target] = f.Multiplicity
				}
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: unused code citation borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge" {
					t.Fatalf("caught: unused code citation public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementSharedUnusedCitationDrift(t, c, &f, &a, budget)
			}
			if diff := cmp.Diff(tc.refused, refused); diff != "" {
				t.Fatalf("caught: complete unused code citation refused receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete unused code citation public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
