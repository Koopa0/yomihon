package judge_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

type agreementCommentFootnoteDefinition struct {
	Owner, Comment, Tail graph.Span
	Label                string
}

type agreementCommentFootnoteProjection struct {
	Body      string
	Positions []int
}

type agreementCommentFootnoteSource struct {
	Definitions []agreementCommentFootnoteDefinition
	Projection  agreementCommentFootnoteProjection
	Fields      []agreementUnusedWidgetField
}

// A root HTML comment can expose a definition when its bytes disappear. Retain
// the original declaration and label before observing any discarded paragraphs.
func agreementCommentFootnoteDeclarations(body string, grammar goldmark.Markdown) []agreementCommentFootnoteDefinition {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	var definitions []agreementCommentFootnoteDefinition
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		block, ok := node.(*ast.HTMLBlock)
		if !ok || block.HTMLBlockType != ast.HTMLBlockType2 || block.Lines().Len() == 0 {
			continue
		}
		start := block.Lines().At(0).Start
		stop := block.Lines().At(block.Lines().Len() - 1).Stop
		if block.HasClosure() {
			stop = max(stop, block.ClosureLine.Stop)
		}
		raw := body[start:stop]
		commentStart := start + len(raw) - len(strings.TrimLeft(raw, " \t"))
		comment, closed := graph.HTMLCommentSpan(body, commentStart)
		if !closed || comment.Stop > stop || !strings.HasPrefix(body[commentStart:], "<!--") {
			continue
		}
		tail := strings.TrimRight(body[comment.Stop:stop], "\r\n")
		if !strings.HasPrefix(tail, "[^") {
			continue
		}
		label, _, defined := strings.Cut(tail, "]:")
		label = strings.TrimPrefix(label, "[^")
		if !defined || label == "" || strings.ContainsFunc(label, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && r != '-'
		}) || strings.Count(body, "[^"+label+"]") != 1 {
			return nil
		}
		definitions = append(definitions, agreementCommentFootnoteDefinition{
			Owner:   graph.Span{Start: start, Stop: stop},
			Comment: comment,
			Tail:    graph.Span{Start: comment.Stop, Stop: comment.Stop + len(tail)},
			Label:   label,
		})
	}
	return definitions
}

// Only owned comment bytes disappear. Every retained byte and physical newline
// keeps its original coordinate, including indentation outside the comment.
func agreementProjectCommentFootnotes(body string, definitions []agreementCommentFootnoteDefinition) agreementCommentFootnoteProjection {
	var projected []byte
	var positions []int
	for i := range len(body) {
		hidden := false
		for _, definition := range definitions {
			if i >= definition.Comment.Start && i < definition.Comment.Stop {
				hidden = true
			}
		}
		if hidden && body[i] != '\n' && body[i] != '\r' {
			continue
		}
		projected = append(projected, body[i])
		positions = append(positions, i)
	}
	return agreementCommentFootnoteProjection{Body: string(projected), Positions: positions}
}

func agreementCommentFootnoteReading(body string, grammar goldmark.Markdown) agreementCommentFootnoteSource {
	definitions := agreementCommentFootnoteDeclarations(body, grammar)
	if len(definitions) == 0 {
		return agreementCommentFootnoteSource{}
	}
	projection := agreementProjectCommentFootnotes(body, definitions)
	fields := agreementUnusedPlainWidgetReading(projection.Body, grammar, false, false)
	if len(fields) == 0 {
		return agreementCommentFootnoteSource{}
	}
	for i, field := range fields {
		if field.Span.Start < 0 || field.Span.Stop <= field.Span.Start || field.Span.Stop > len(projection.Positions) {
			return agreementCommentFootnoteSource{}
		}
		start := projection.Positions[field.Span.Start]
		stop := projection.Positions[field.Span.Stop-1] + 1
		if start < 0 || stop <= start || stop > len(body) || body[start:stop] != field.Word {
			return agreementCommentFootnoteSource{}
		}
		fields[i].Span = graph.Span{Start: start, Stop: stop}
	}
	return agreementCommentFootnoteSource{Definitions: definitions, Projection: projection, Fields: fields}
}

func agreementCommentFootnoteBudget(body string) agreementUnusedWidgetPayload {
	plain := agreementCommentFootnoteReading(body, agreementFootnoteGrammar)
	gfm := agreementCommentFootnoteReading(body, agreementExclusiveCodeGrammar)
	if !cmp.Equal(plain, gfm) || len(plain.Fields) == 0 {
		return agreementUnusedWidgetPayload{}
	}
	budget := make(map[agreementCitation]int)
	for _, field := range plain.Fields {
		budget[field.Tuple]++
	}
	return agreementUnusedWidgetPayload{Body: body, Tuples: budget}
}

func agreementCommentExpectedPositions(ranges ...graph.Span) []int {
	var positions []int
	for _, span := range ranges {
		for i := span.Start; i < span.Stop; i++ {
			positions = append(positions, i)
		}
	}
	return positions
}

func TestAgreementCommentFootnoteSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}
	rawB := agreementCitation{Target: "B\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       agreementCommentFootnoteSource
	}{
		{name: "complete original root and later paragraph", body: "<!--hidden-->[^n]: [[A]]\n\n    [[B]]\n ", want: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 25}, Comment: graph.Span{Start: 0, Stop: 13}, Tail: graph.Span{Start: 13, Stop: 24}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "[^n]: [[A]]\n\n    [[B]]\n ", Positions: agreementCommentExpectedPositions(graph.Span{Start: 13, Stop: 37})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 19, Stop: 24}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 30, Stop: 35}, Word: "[[B]]", Tuple: b}}}},
		{name: "multiline native closure preserves physical newlines", body: "<!--\n[[A]]\n-->[^n]: [[A]]\n\n    [[B]]\n", want: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 26}, Comment: graph.Span{Start: 0, Stop: 14}, Tail: graph.Span{Start: 14, Stop: 25}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "\n\n[^n]: [[A]]\n\n    [[B]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 4, Stop: 5}, graph.Span{Start: 10, Stop: 11}, graph.Span{Start: 14, Stop: 37})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 20, Stop: 25}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 31, Stop: 36}, Word: "[[B]]", Tuple: b}}}},
		{name: "Unicode label raw bytes section and embed", body: "<!--h-->[^一]: ![[A#part|shown]] [[B\\]]\n", want: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 41}, Comment: graph.Span{Start: 0, Stop: 8}, Tail: graph.Span{Start: 8, Stop: 40}, Label: "一"}}, Projection: agreementCommentFootnoteProjection{Body: "[^一]: ![[A#part|shown]] [[B\\]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 8, Stop: 41})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 16, Stop: 33}, Word: "![[A#part|shown]]", Tuple: section}, {Span: graph.Span{Start: 34, Stop: 40}, Word: "[[B\\]]", Tuple: rawB}}}},
		{name: "every original root declaration and field", body: "<!--h-->[^a]: [[A]]\n<!--h-->[^b]: [[B]] [[A]]\n", want: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 20}, Comment: graph.Span{Start: 0, Stop: 8}, Tail: graph.Span{Start: 8, Stop: 19}, Label: "a"}, {Owner: graph.Span{Start: 20, Stop: 46}, Comment: graph.Span{Start: 20, Stop: 28}, Tail: graph.Span{Start: 28, Stop: 45}, Label: "b"}}, Projection: agreementCommentFootnoteProjection{Body: "[^a]: [[A]]\n[^b]: [[B]] [[A]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 8, Stop: 20}, graph.Span{Start: 28, Stop: 46})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 14, Stop: 19}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 34, Stop: 39}, Word: "[[B]]", Tuple: b}, {Span: graph.Span{Start: 40, Stop: 45}, Word: "[[A]]", Tuple: a}}}},
		{name: "native indentation retains all original prefix bytes", body: "  <!--h-->[^n]: [[A]]\n", want: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 22}, Comment: graph.Span{Start: 2, Stop: 10}, Tail: graph.Span{Start: 10, Stop: 21}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "  [^n]: [[A]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 0, Stop: 2}, graph.Span{Start: 10, Stop: 22})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Tuple: a}}}},
		{name: "CRLF bytes remain separate original coordinates", body: "<!--\r\nhidden\r\n-->[^n]: [[A]]\r\n", want: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 30}, Comment: graph.Span{Start: 0, Stop: 17}, Tail: graph.Span{Start: 17, Stop: 28}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "\r\n\r\n[^n]: [[A]]\r\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 4, Stop: 6}, graph.Span{Start: 12, Stop: 14}, graph.Span{Start: 17, Stop: 30})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 23, Stop: 28}, Word: "[[A]]", Tuple: a}}}},
		{name: "unfinished field refuses all earlier complete fields", body: "<!--h-->[^n]: [[A]] [[B\n"},
		{name: "nested field refuses whole original inventory", body: "<!--h-->[^n]: [[A]] [[B[[C]]]]\n"},
		{name: "local target has another source role", body: "<!--h-->[^n]: [[A]] [[#A]]\n"},
		{name: "block target has another source role", body: "<!--h-->[^n]: [[A]] [[B#^b]]\n"},
		{name: "native quote remains outside root ownership", body: "> <!--h-->[^n]: [[A]]\n"},
		{name: "native fence remains outside root ownership", body: "```\n<!--h-->[^n]: [[A]]\n```\n"},
		{name: "raw HTML container remains another owner", body: "<div>\n<!--h-->[^n]: [[A]]\n</div>\n"},
		{name: "space before label retains prose ownership", body: "<!--h--> [^n]: [[A]]\n"},
		{name: "used label refuses original declaration", body: "<!--h-->[^n]: [[A]]\nref[^n]\n"},
		{name: "duplicated label refuses complete set", body: "<!--h-->[^n]: [[A]]\n<!--h-->[^n]: [[B]]\n"},
		{name: "invalid label retains separate ownership", body: "<!--h-->[^two words]: [[A]]\n"},
		{name: "unclosed comment has no manufactured declaration", body: "<!--h[^n]: [[A]]\n"},
		{name: "later inline code refuses all earlier fields", body: "<!--h-->[^n]: [[A]]\n\n    `[[B]]`\n"},
		{name: "later heading refuses all earlier fields", body: "<!--h-->[^n]: [[A]]\n\n    ## [[B]]\n"},
		{name: "outside percent ownership refuses manufactured source", body: "%%\n<!--h-->[^n]: [[A]]\n%%\n"},
		{name: "ordinary footnote retains original ownership", body: "[^n]: [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(tc.want, agreementCommentFootnoteReading(tc.body, grammar)); diff != "" {
					t.Fatalf("caught: complete comment footnote original source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementCommentFootnoteDiagnostics(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body   string
		budget, want map[agreementCitation]int
		old          map[string]int
	}{
		{name: "all native continuation fields", body: "<!-- [[A]] -->[^n]: [[A]]\n\n    [[B]]\n ", budget: map[agreementCitation]int{a: 1, b: 1}, want: map[agreementCitation]int{a: 1, b: 1}, old: map[string]int{"A": 1}},
		{name: "all native multiline fields", body: "<!--\n[[A]]\n-->[^n]: [[A]]\n\n    [[B]]\n", budget: map[agreementCitation]int{a: 1, b: 1}, want: map[agreementCitation]int{a: 1, b: 1}},
		{name: "shared visible targets retain all counts", body: "[[A]] [[A]]\n\n<!--h-->[^n]: [[A]] [[A]] [[B]]\n", budget: map[agreementCitation]int{a: 2, b: 1}, want: map[agreementCitation]int{a: 2, b: 1}, old: map[string]int{"A": 2, "B": 1}},
		{name: "whole declaration set and every paragraph", body: "<!--h-->[^a]: [[A]]\n\n    [[B]]\n\n<!--h-->[^b]: [[A]] [[B]]\n", budget: map[agreementCitation]int{a: 2, b: 2}, want: map[agreementCitation]int{a: 2, b: 2}, old: map[string]int{"A": 2, "B": 1}},
		{name: "alias sections and embeds retain typed targets", body: "<!--h-->[^n]: ![[A#part|shown]] [[A]]\n", budget: map[agreementCitation]int{section: 1, a: 1}, want: map[agreementCitation]int{section: 1, a: 1}},
		{name: "inactive field cannot lend missing reference role", body: "<!--h-->[^n]: [[B]]\n%%[[A]]%%\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{b: 1, {SourceRole: agreementUnusedCommentOverlap, Target: "A", State: "wikilink-broken"}: 1}, want: map[agreementCitation]int{b: 1}, old: map[string]int{"B": 1}},
		{name: "missing reference cannot borrow whole count", body: "<!--h-->[^n]: [[A]]\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{a: 1}, old: map[string]int{"A": 1}},
		{name: "missing reference retains separate target", body: "<!--h-->[^n]: [[A]]\n\n    [[B]]\n\n[r]: [[B]]\n", budget: map[agreementCitation]int{a: 1, b: 1}, want: map[agreementCitation]int{a: 1}, old: map[string]int{"A": 1}},
		{name: "existing real unused definition stays in full source", body: "[^m]: [[B]]\n\n<!--h-->[^n]: [[A]]\n", budget: map[agreementCitation]int{a: 1, b: 1}, want: map[agreementCitation]int{a: 1, b: 1}, old: map[string]int{"A": 1}},
		{name: "earlier one-line owner stays unchanged", body: "<!--h-->[^n]: [[A]]\n", budget: map[agreementCitation]int{a: 1}, want: map[agreementCitation]int{a: 1}, old: map[string]int{"A": 1}},
		{name: "odd escape refuses original full source", body: "<!--h-->[^n]: [[A]] \\[[B]]\n"},
		{name: "used manufactured declaration stays outside", body: "<!--h-->[^n]: [[A]]\nref[^n]\n"},
		{name: "other inline source owner stays outside", body: "<!--h-->[^n]: `[[A]]`\n"},
		{name: "CommonMark alone assigns the reference", body: "https://example.invalid/ref[^n]\n\n<!--h-->[^n]: [[A]]\n"},
		{name: "page grammar alone assigns the reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n<!--h-->[^n]: [[A]]\n"},
		{name: "URL makes the manufactured field readings disagree", body: "<!--h-->[^n]: https://example.invalid/[[A]]\n"},
		{name: "recorded full original retains every continuation field", body: "<!--\n[[A]]\n-->[^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]\\\\[[A]]\n\n ^é\n", budget: map[agreementCitation]int{a: 3, b: 1}, want: map[agreementCitation]int{a: 3, b: 1}},
		{name: "outside hidden region cannot lend missing target", body: "%%\n<!--h-->[^n]: [[A]]\n%%\n\n[r]: [[A]]\n"},
		{name: "ordinary source keeps earlier owner", body: "[^n]: [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementCommentFootnoteBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete comment footnote source budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementCommentMadeFootnoteTargets(c.Body)); diff != "" {
				t.Fatalf("caught: changed earlier comment footnote boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				if f.Property == "P0" {
					for tuple, count := range budget.Tuples {
						if tuple.SourceRole == "" || tuple.Target != f.Tuple.Target || tuple.Section != f.Tuple.Section || count != f.Multiplicity {
							continue
						}
						drift := f
						drift.Tuple = tuple
						if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, &drift, &actual, r.Diagnostics, budget); kind != "" {
							t.Fatal("caught: inactive comment footnote field borrowed public diagnostic")
						}
					}
				}
				kind, authority, wrong := agreementSharedUnusedDiagnosticDifference(c, &f, &actual, r.Diagnostics, budget)
				if f.Property != "P0" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: comment footnote borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: comment footnote public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementSharedUnusedDiagnosticDrift(t, c, &f, &actual, r.Diagnostics, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete comment footnote public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
