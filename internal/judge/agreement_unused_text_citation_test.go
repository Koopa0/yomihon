package judge_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

// Preserve the whole physical definition when plain words surround its fields.
// Comment overlap cannot supply an active check contribution.
func agreementUnusedTextCitationReading(body string, grammar goldmark.Markdown) agreementUnusedCitationSource {
	return agreementUnusedPlainCitationReading(body, grammar, false)
}

func agreementUnusedPlainCitationReading(body string, grammar goldmark.Markdown, retainIncomplete bool) agreementUnusedCitationSource {
	fields := agreementUnusedPlainWidgetReading(body, grammar, false, retainIncomplete)
	if len(fields) == 0 {
		return agreementUnusedCitationSource{}
	}
	for _, field := range fields {
		if field.Tuple.SourceRole != "" && (!retainIncomplete || field.Tuple.SourceRole != agreementUnusedIncompleteField) {
			return agreementUnusedCitationSource{}
		}
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var definitions []agreementUnusedCheckDefinition
	for _, definition := range unused {
		start, stop := -1, -1
		for node := definition.FirstChild(); node != nil; node = node.NextSibling() {
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				physical := bytes.LastIndexByte(source[:line.Start], '\n') + 1
				if start < 0 || physical < start {
					start = physical
				}
				stop = max(stop, line.Stop)
			}
		}
		if start < 0 {
			return agreementUnusedCitationSource{}
		}
		definitions = append(definitions, agreementUnusedCheckDefinition{
			Span:    graph.Span{Start: start, Stop: stop},
			Targets: judge.LinkTargets(string(source[start:stop])),
		})
	}
	return agreementUnusedCitationSource{Fields: fields, Definitions: definitions}
}

func agreementUnusedTextCitationBudget(body string) agreementUnusedCitationPayload {
	plain := agreementUnusedTextCitationReading(body, agreementFootnoteGrammar)
	gfm := agreementUnusedTextCitationReading(body, agreementExclusiveCodeGrammar)
	if !cmp.Equal(plain, gfm) {
		return agreementUnusedCitationPayload{}
	}
	var targets map[string]int
	for _, definition := range plain.Definitions {
		for _, target := range definition.Targets {
			if targets == nil {
				targets = make(map[string]int)
			}
			targets[target]++
		}
	}
	if len(targets) == 0 {
		return agreementUnusedCitationPayload{}
	}
	return agreementUnusedCitationPayload{Body: body, Targets: targets}
}

func TestAgreementUnusedTextCitationSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "B", Section: "part", State: "wikilink-broken"}
	rawC := agreementCitation{Target: "C\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedCitationSource
	}{
		{name: "whole prose source keeps indentation and every field", body: "[^n]: words [[A]]after\nnext ![[B#part|shown]] [[A]]\n\n    more [[C\\]]\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 28, Stop: 45}, Word: "![[B#part|shown]]", Tuple: section}, {Span: graph.Span{Start: 46, Stop: 51}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 62, Stop: 68}, Word: "[[C\\]]", Tuple: rawC}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 68}, Targets: []string{"A", "B", "A"}}}}},
		{name: "fieldless physical line retains target order", body: "[^n]: [[A]]\nno fields\n[[B]]\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[B]]", Tuple: b}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 27}, Targets: []string{"A", "B"}}}}},
		{name: "raw entity bytes stay distinct", body: "[^n]: prose [[A&amp;B|shown]] end\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 12, Stop: 29}, Word: "[[A&amp;B|shown]]", Tuple: agreementCitation{Target: "A&amp;B", State: "wikilink-broken"}}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 33}, Targets: []string{"A&amp;B"}}}}},
		{name: "Unicode prefix keeps physical byte bounds", body: "[^n]: 純文字 [[A]] end\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Tuple: a}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 25}, Targets: []string{"A"}}}}},
		{name: "comment overlap refuses the whole active contribution", body: "[^n]: prose [[B]]\n%%[[A]]%%\n"},
		{name: "partial comment overlap refuses active contribution", body: "[^n]: [[A|%%hidden%%]]\n"},
		{name: "used source has no discarded contribution", body: "ref[^n]\n\n[^n]: words [[A]]\n"},
		{name: "ordinary source has no discarded contribution", body: "words [[A]]\n"},
		{name: "other inline owner refuses whole source", body: "[^n]: words [[B]] `[[A]]`\n"},
		{name: "other block refuses earlier source", body: "[^n]: words [[A]]\n\n    ## [[B]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(tc.want, agreementUnusedTextCitationReading(tc.body, grammar)); diff != "" {
					t.Fatalf("caught: complete unused text citation source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementUnusedTextCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body        string
		budget, old, want map[string]int
	}{
		{name: "all plain fields beside shared live target", body: "[[A]]\n\n[^n]: words [[A]]after [[A]] [[B#part|shown]] end\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "all visible counts remain ordinary", body: "[[A]] [[A]] [[B]]\n\n[^n]: words [[A]] [[B]] end\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "all definitions and fieldless physical rows", body: "[^n]: words [[A]]\nno fields\n[[B]]after\n\n[^m]: more [[A]]\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "later paragraphs retain physical indentation", body: "[[A]]\n\n[^n]: words [[A]]\n\n    more [[B]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "raw suffix keeps check normalization", body: "[[A]]\n\n[^n]: words [[A\\]] end\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "sections combine under check target", body: "[[A#part]]\n\n[^n]: words [[A#part]] [[A]] end\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "raw entity check target keeps original spelling", body: "[^n]: prose [[A&amp;B|shown]] end\n", budget: map[string]int{"A&amp;B": 1}, want: map[string]int{"A&amp;B": 1}},
		{name: "used declaration cannot lend contribution", body: "ref[^u]\n\n[^u]: [[A]]\n\n[^n]: words [[A]] end\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "missing reference stays another owner", body: "[r]: [[A]]\n\n[^n]: words [[A]] end\n", budget: map[string]int{"A": 1}},
		{name: "hidden pure field cannot lend missing reference count", body: "%%\n[^n]: [[A]]\n\n%%\n\n[r]: [[A]]\n"},
		{name: "hidden prose field cannot lend missing reference count", body: "%%\n[^n]: words [[A]]\n%%\n\n[r]: [[A]]\n"},
		{name: "mixed active and hidden fields refuse whole contribution", body: "[^n]: prose [[B]]\n%%[[A]]%%\n\n[r]: [[A]]\n"},
		{name: "partial comment overlap refuses", body: "[^n]: [[A|%%hidden%%]]\n"},
		{name: "page grammar alone uses reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words [[A]]\n"},
		{name: "CommonMark alone uses reference", body: "https://example.invalid/ref[^n]\n\n[^n]: words [[A]]\n"},
		{name: "inline URL makes whole grammar inventory disagree", body: "[^n]: https://example.invalid/[[A]]\n"},
		{name: "older pure source remains unchanged", body: "[[A]]\n\n[^n]: [[A]]\n", budget: map[string]int{"A": 1}, old: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "full recorded source retains unrelated public counts", body: "[^n]: [[A]]\n\n    [[B]]\n ^é\n\n\n> [!unknown] title\n- [ ] [[A]]\n![[A]]> [!note] one\n> [!note] two\n> [!note] three\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "ordinary prose remains outside", body: "words [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementUnusedTextCitationBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Targets); diff != "" {
				t.Fatalf("caught: complete unused text citation budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementSharedUnusedCitationBudget(c.Body).Targets); diff != "" {
				t.Fatalf("caught: changed earlier unused citation source boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementSharedUnusedCitationDifference(c, &f, &actual, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: unused text citation borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge" {
					t.Fatalf("caught: unused text citation public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementSharedUnusedCitationDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete unused text citation public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
