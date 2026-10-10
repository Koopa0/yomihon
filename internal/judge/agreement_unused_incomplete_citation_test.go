package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"

	"github.com/koopa0/yomihon/internal/graph"
)

// A physical suffix contributes no guessed target. The check still reads the
// complete original definition, including every label, line and indentation.
func agreementUnusedIncompleteCitationReading(body string, grammar goldmark.Markdown) agreementUnusedCitationSource {
	source := agreementUnusedPlainCitationReading(body, grammar, true)
	incomplete := 0
	for _, field := range source.Fields {
		if field.Tuple.SourceRole == agreementUnusedIncompleteField {
			incomplete++
		}
	}
	if incomplete == 0 {
		return agreementUnusedCitationSource{}
	}
	return source
}

func agreementUnusedIncompleteCitationBudget(body string) agreementUnusedCitationPayload {
	plain := agreementUnusedIncompleteCitationReading(body, agreementFootnoteGrammar)
	gfm := agreementUnusedIncompleteCitationReading(body, agreementExclusiveCodeGrammar)
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

func TestAgreementUnusedIncompleteCitationSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	literal := agreementCitation{SourceRole: agreementUnusedIncompleteField}
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedCitationSource
	}{
		{name: "whole physical source and unfinished field", body: "[^n]: [[A]] [[B\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 12, Stop: 15}, Word: "[[B", Tuple: literal}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 15}, Targets: []string{"A"}}}}},
		{name: "cross-line suffix has no invented target", body: "[^n]: [[A\nB]]\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 10}, Word: "[[A\n", Tuple: literal}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 13}, Targets: []string{}}}}},
		{name: "all declarations paragraphs and physical indentation", body: "[^n]: [[A]] [[C\nD]] [[B]]\n\n    [[A]]\n\n[^m]: [[A]] [[E\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 12, Stop: 16}, Word: "[[C\n", Tuple: literal}, {Span: graph.Span{Start: 20, Stop: 25}, Word: "[[B]]", Tuple: b}, {Span: graph.Span{Start: 31, Stop: 36}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 44, Stop: 49}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 50, Stop: 53}, Word: "[[E", Tuple: literal}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 36}, Targets: []string{"A", "B"}}, {Span: graph.Span{Start: 38, Stop: 53}, Targets: []string{"A"}}}}},
		{name: "ordinary complete source retains earlier scope", body: "[^n]: words [[A]]\n"},
		{name: "escaped source retains separate ownership", body: "[^n]: [[A]] \\[[B]] [[C\n"},
		{name: "comment source cannot lend a target", body: "%%\n[^n]: [[A]] [[B\n%%\n"},
		{name: "other inline owner refuses all earlier fields", body: "[^n]: [[A]] `[[B]]` [[C\n"},
		{name: "used definition has no discarded owner", body: "ref[^n]\n\n[^n]: [[A]] [[B\n"},
		{name: "later block refuses whole original definition", body: "[^n]: [[A]] [[B\n\n    ## [[C]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(tc.want, agreementUnusedIncompleteCitationReading(tc.body, grammar)); diff != "" {
					t.Fatalf("caught: complete unused incomplete citation source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementUnusedIncompleteCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body        string
		budget, old, want map[string]int
	}{
		{name: "all complete fields and shared visible counts", body: "[[A]] [[A]]\n\n[^n]: [[A]] [[A]] [[C\nD]] [[B]]\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "all definitions paragraphs and fieldless lines", body: "[^n]: [[A]] [[C\nD]] [[B]]\nno fields\n\n    [[A]]\n\n[^m]: [[A]] [[E\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "pure literal contributes no target", body: "[^n]: [[A\nB]]\n"},
		{name: "literal cannot lend a missing reference", body: "[^n]: [[A\nB]]\n\n[r]: [[A]]\n"},
		{name: "complete target and separate missing reference", body: "[^n]: [[B]] [[A\n\n[r]: [[A]]\n", budget: map[string]int{"B": 1}, want: map[string]int{"B": 1}},
		{name: "complete target retains whole check equation", body: "[^n]: [[A]] [[B\n\n[r]: [[A]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "other code check count refuses borrowing", body: "[^n]: [[A]] [[B\n\n`open\n[[A]]\nclose`\n", budget: map[string]int{"A": 1}},
		{name: "hidden original source cannot lend a contribution", body: "%%\n[^n]: [[A]] [[B\n%%\n\n[r]: [[A]]\n"},
		{name: "escaped field refuses complete contribution", body: "[^n]: [[A]] \\[[A]] [[B\n"},
		{name: "page grammar alone uses reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]] [[B\n"},
		{name: "CommonMark alone uses reference", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A]] [[B\n"},
		{name: "inline URL makes full readings disagree", body: "[^n]: https://example.invalid/[[A]] [[B\n"},
		{name: "earlier pure source stays unchanged", body: "[^n]: [[A]]\n", old: map[string]int{"A": 1}},
		{name: "earlier prose source stays unchanged", body: "[^n]: words [[A]]\n", old: map[string]int{"A": 1}},
		{name: "full recorded original preserves unrelated counts", body: "[[A#A]]\n> [!note] title\nA\n---\n[^unused]: [[A]]\n[[A\nB]]\n\n- [x] [[B]]\n- [ ] [[A]]\n[[A#^a]]", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementUnusedIncompleteCitationBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Targets); diff != "" {
				t.Fatalf("caught: complete unused incomplete citation budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementUnusedTextCitationBudget(c.Body).Targets); diff != "" {
				t.Fatalf("caught: changed earlier unused text citation boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementSharedUnusedCitationDifference(c, &f, &actual, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: unused incomplete citation borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge" {
					t.Fatalf("caught: unused incomplete citation public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementSharedUnusedCitationDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete unused incomplete citation public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
