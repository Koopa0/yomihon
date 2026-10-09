package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

const agreementUnusedCommentOverlap = "comment-overlap"

// Ordinary words do not change a discarded text field's position. Retain every
// field, including comment overlap, so a hidden name cannot lend a contribution.
func agreementUnusedTextWidgetReading(body string, grammar goldmark.Markdown) []agreementUnusedWidgetField {
	return agreementUnusedPlainWidgetReading(body, grammar, false)
}

func agreementUnusedPlainWidgetReading(body string, grammar goldmark.Markdown, retainEscapes bool) []agreementUnusedWidgetField {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	comments := agreementUnusedWidgetComments(body, doc)
	var fields []agreementUnusedWidgetField
	for _, definition := range unused {
		for node := definition.FirstChild(); node != nil; node = node.NextSibling() {
			switch node.(type) {
			case *ast.Paragraph, *ast.TextBlock:
			default:
				return nil
			}
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if _, plain := child.(*ast.Text); !plain {
					return nil
				}
			}
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				raw := string(source[line.Start:line.Stop])
				offset := 0
				for offset < len(raw) {
					open := strings.Index(raw[offset:], "[[")
					if open < 0 {
						break
					}
					open += offset
					closeAt := strings.Index(raw[open+2:], "]]")
					if closeAt < 0 {
						return nil
					}
					closeAt += open + 2
					inner := raw[open+2 : closeAt]
					if strings.ContainsAny(inner, "[]\r\n`") || !retainEscapes && graph.EscapedWikilinkAt(raw, open) {
						return nil
					}
					link, cites := graph.ParseWikilink(inner)
					if !cites || link.Block != "" {
						return nil
					}
					start := open
					if start > 0 && raw[start-1] == '!' {
						start--
					}
					offset = closeAt + 2
					field := agreementUnusedWidgetField{
						Span:  graph.Span{Start: line.Start + start, Stop: line.Start + offset},
						Word:  raw[start:offset],
						Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"},
					}
					if agreementUnusedWidgetCommentOwned(field.Span, comments) {
						field.Tuple.SourceRole = agreementUnusedCommentOverlap
					}
					if graph.EscapedWikilinkAt(raw, open) {
						field.Tuple.SourceRole = agreementUnusedEscapedField
					}
					fields = append(fields, field)
				}
			}
		}
	}
	return fields
}

func agreementUnusedTextWidgetBudget(body string) agreementUnusedWidgetPayload {
	plain := agreementUnusedTextWidgetReading(body, agreementFootnoteGrammar)
	gfm := agreementUnusedTextWidgetReading(body, agreementExclusiveCodeGrammar)
	if !cmp.Equal(plain, gfm) || len(plain) == 0 {
		return agreementUnusedWidgetPayload{}
	}
	budget := make(map[agreementCitation]int)
	for _, field := range plain {
		budget[field.Tuple]++
	}
	return agreementUnusedWidgetPayload{Body: body, Tuples: budget}
}

func TestAgreementUnusedTextWidgetSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "B", Section: "part", State: "wikilink-broken"}
	rawC := agreementCitation{Target: "C\\", State: "wikilink-broken"}
	commentA := agreementCitation{SourceRole: agreementUnusedCommentOverlap, Target: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       []agreementUnusedWidgetField
	}{
		{name: "all fields paragraphs embeds and raw suffixes", body: "[^n]: words [[A]]after\nnext ![[B#part|shown]] [[A]]\n\n    more [[C\\]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 28, Stop: 45}, Word: "![[B#part|shown]]", Tuple: section}, {Span: graph.Span{Start: 46, Stop: 51}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 62, Stop: 68}, Word: "[[C\\]]", Tuple: rawC}}},
		{name: "raw entity spelling stays authored", body: "[^n]: prose [[A&amp;B|shown]] end\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 12, Stop: 29}, Word: "[[A&amp;B|shown]]", Tuple: agreementCitation{Target: "A&amp;B", State: "wikilink-broken"}}}},
		{name: "fieldless continuation retains later fields", body: "[^n]: [[A]]\nno fields\n[[B]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[B]]", Tuple: b}}},
		{name: "Unicode prefix keeps byte positions", body: "[^n]: 純文字 [[A]] end\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Tuple: a}}},
		{name: "comment fields retain a separate source role", body: "[^n]: prose [[B]]\n%%[[A]]%%\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[B]]", Tuple: b}, {Span: graph.Span{Start: 20, Stop: 25}, Word: "[[A]]", Tuple: commentA}}},
		{name: "partial comment overlap retains whole raw field", body: "[^n]: [[A|%%hidden%%]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 22}, Word: "[[A|%%hidden%%]]", Tuple: commentA}}},
		{name: "used declaration has no unused owner", body: "ref[^n]\n\n[^n]: words [[A]]\n"},
		{name: "ordinary source has no unused owner", body: "words [[A]]\n"},
		{name: "inline code refuses whole source", body: "[^n]: words [[B]] `[[A]]`\n"},
		{name: "formatting refuses whole source", body: "[^n]: words [[B]] *[[A]]*\n"},
		{name: "reference brackets refuse whole source", body: "[A]: /elsewhere\n\n[^n]: words [[B]] [[A]]\n"},
		{name: "later heading refuses earlier fields", body: "[^n]: words [[A]]\n\n    ## [[B]]\n"},
		{name: "later fence refuses earlier fields", body: "[^n]: words [[A]]\n\n    ```\n    [[B]]\n    ```\n"},
		{name: "incomplete field refuses earlier fields", body: "[^n]: words [[A]] [[B\n"},
		{name: "nested field refuses earlier fields", body: "[^n]: words [[A]] [[B[[C]]]]\n"},
		{name: "block target refuses earlier fields", body: "[^n]: words [[A]] [[B#^b]]\n"},
		{name: "local target refuses earlier fields", body: "[^n]: words [[A]] [[#B]]\n"},
		{name: "odd escape refuses whole source", body: "[^n]: words [[B]] \\[[A]]\n"},
		{name: "odd escaped embed refuses whole source", body: "[^n]: words [[B]] \\![[A]]\n"},
		{name: "cross-line field refuses whole source", body: "[^n]: words [[A\nB]] [[C]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				got := agreementUnusedTextWidgetReading(tc.body, grammar)
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Fatalf("caught: complete unused text widget source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementUnusedTextWidgets(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "B", Section: "part", State: "wikilink-broken"}
	commentA := agreementCitation{SourceRole: agreementUnusedCommentOverlap, Target: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		budget     map[agreementCitation]int
		old        map[agreementCitation]int
		want       map[agreementCitation]int
	}{
		{name: "every field beside shared live target", body: "[[A]]\n\n[^n]: words [[A]]after [[A]] [[B#part|shown]] end\n", budget: map[agreementCitation]int{a: 2, section: 1}, want: map[agreementCitation]int{a: 2, section: 1}},
		{name: "all visible occurrences", body: "[[A]] [[A]] [[B]]\n\n[^n]: words [[A]] [[B]] end\n", budget: map[agreementCitation]int{a: 1, b: 1}, want: map[agreementCitation]int{a: 1, b: 1}},
		{name: "every definition and fieldless physical line", body: "[^n]: words [[A]]\nno fields\n[[B]]after\n\n[^m]: more [[A]]\n", budget: map[agreementCitation]int{a: 2, b: 1}, want: map[agreementCitation]int{a: 2, b: 1}},
		{name: "raw entity name reaches public diagnostic", body: "[^n]: prose [[A&amp;B|shown]] end\n", budget: map[agreementCitation]int{{Target: "A&amp;B", State: "wikilink-broken"}: 1}, want: map[agreementCitation]int{{Target: "A&amp;B", State: "wikilink-broken"}: 1}},
		{name: "raw suffix and renamed embed keep fields", body: "[^n]: words [[A\\]] ![[B#part|shown]] end\n", budget: map[agreementCitation]int{{Target: "A\\", State: "wikilink-broken"}: 1, section: 1}, want: map[agreementCitation]int{{Target: "A\\", State: "wikilink-broken"}: 1, section: 1}},
		{name: "comment field cannot lend missing reference diagnostic", body: "[^n]: prose [[B]]\n%%[[A]]%%\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{b: 1, commentA: 1}, want: map[agreementCitation]int{b: 1}},
		{name: "hidden pure definition cannot lend diagnostic", body: "%%\n[^n]: [[A]]\n\n%%\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{commentA: 1}},
		{name: "hidden prose definition cannot lend diagnostic", body: "%%\n[^n]: words [[A]]\n%%\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{commentA: 1}},
		{name: "partially hidden field refuses diagnostic ownership", body: "[^n]: [[A|%%hidden%%]]\n", budget: map[agreementCitation]int{commentA: 1}},
		{name: "whole count refuses another absent owner", body: "[r]: [[A]]\n\n[^n]: words [[A]] end\n", budget: map[agreementCitation]int{a: 1}},
		{name: "page grammar alone uses reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words [[A]]\n"},
		{name: "CommonMark alone uses reference", body: "https://example.invalid/ref[^n]\n\n[^n]: words [[A]]\n"},
		{name: "inline URL owner makes grammars disagree", body: "[^n]: https://example.invalid/[[A]]\n"},
		{name: "ordinary prose stays outside", body: "words [[A]]\n"},
		{name: "older pure field source remains unchanged", body: "[[A]]\n\n[^n]: [[A]]\n", budget: map[agreementCitation]int{a: 1}, old: map[agreementCitation]int{a: 1}, want: map[agreementCitation]int{a: 1}},
		{name: "recorded full original with unclosed comment tail", body: " É\n[^n]: [[A]]\n\n    [[B]]\n%%[[A]]", budget: map[agreementCitation]int{a: 1, b: 1, commentA: 1}, want: map[agreementCitation]int{a: 1, b: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementUnusedTextWidgetBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete unused text widget budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementSharedUnusedWidgetBudget(c.Body).Tuples); diff != "" {
				t.Fatalf("caught: changed earlier unused widget source boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementSharedUnusedDiagnosticDifference(c, &f, &actual, r.Diagnostics, budget)
				if f.Property != "P0" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: unused text widget borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: unused text widget public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementSharedUnusedDiagnosticDrift(t, c, &f, &actual, r.Diagnostics, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete unused text widget public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
