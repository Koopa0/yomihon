package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"

	"github.com/koopa0/yomihon/internal/graph"
)

const agreementUnusedIncompleteField = "unused-incomplete"

// An unfinished field has source bytes but no target. Preserve it alongside the
// complete fields so a missing reference cannot borrow an invented contribution.
func agreementUnusedIncompleteWidgetBudget(body string) agreementUnusedWidgetPayload {
	plain := agreementUnusedPlainWidgetReading(body, agreementFootnoteGrammar, true, true)
	gfm := agreementUnusedPlainWidgetReading(body, agreementExclusiveCodeGrammar, true, true)
	if !cmp.Equal(plain, gfm) || len(plain) == 0 {
		return agreementUnusedWidgetPayload{}
	}
	budget := make(map[agreementCitation]int)
	incompleteCount := 0
	for _, field := range plain {
		budget[field.Tuple]++
		if field.Tuple.SourceRole == agreementUnusedIncompleteField {
			incompleteCount++
		}
	}
	if incompleteCount == 0 {
		return agreementUnusedWidgetPayload{}
	}
	return agreementUnusedWidgetPayload{Body: body, Tuples: budget}
}

func TestAgreementUnusedIncompleteWidgetSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	d := agreementCitation{Target: "D", State: "wikilink-broken"}
	incomplete := agreementCitation{SourceRole: agreementUnusedIncompleteField}
	for _, tc := range []struct {
		name, body string
		want       []agreementUnusedWidgetField
	}{
		{name: "cross-line literal retains fields on both sides", body: "[^n]: [[A]] [[B\nC]] [[D]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 12, Stop: 16}, Word: "[[B\n", Tuple: incomplete}, {Span: graph.Span{Start: 20, Stop: 25}, Word: "[[D]]", Tuple: d}}},
		{name: "each unfinished physical line has a record", body: "[^n]: [[A\n[[B\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 10}, Word: "[[A\n", Tuple: incomplete}, {Span: graph.Span{Start: 10, Stop: 13}, Word: "[[B", Tuple: incomplete}}},
		{name: "unfinished embed retains the bang and physical end", body: "[^n]: ![[A\nnext [[B]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "![[A\n", Tuple: incomplete}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[B]]", Tuple: b}}},
		{name: "Unicode prefix keeps byte coordinates", body: "[^n]: 純文字 [[A]] [[B\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 22, Stop: 25}, Word: "[[B", Tuple: incomplete}}},
		{name: "used definition remains outside", body: "ref[^n]\n\n[^n]: [[A]] [[B\n"},
		{name: "ordinary source remains outside", body: "[[A]] [[B\n"},
		{name: "other inline owner refuses whole source", body: "[^n]: `[[A]]` [[B\n"},
		{name: "later block refuses earlier literal", body: "[^n]: [[A]] [[B\n\n    ## [[C]]\n"},
		{name: "nested closed field stays outside", body: "[^n]: [[A]] [[B[[C]]]]\n"},
		{name: "block field stays outside", body: "[^n]: [[A]] [[B#^b]] [[C\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(tc.want, agreementUnusedPlainWidgetReading(tc.body, grammar, true, true)); diff != "" {
					t.Fatalf("caught: complete unused incomplete widget source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementUnusedIncompleteWidgets(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	d := agreementCitation{Target: "D", State: "wikilink-broken"}
	incomplete := agreementCitation{SourceRole: agreementUnusedIncompleteField}
	escapedA := agreementCitation{SourceRole: agreementUnusedEscapedField, Target: "A", State: "wikilink-broken"}
	commentA := agreementCitation{SourceRole: agreementUnusedCommentOverlap, Target: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body                        string
		budget, oldText, oldEscaped, want map[agreementCitation]int
	}{
		{name: "all complete fields and shared visible counts", body: "[[A]] [[A]]\n\n[^n]: [[A]] [[A]] [[B\nC]] [[D]]\n", budget: map[agreementCitation]int{a: 2, d: 1, incomplete: 1}, want: map[agreementCitation]int{a: 2, d: 1}},
		{name: "every unused declaration and paragraph", body: "[^n]: [[A]] [[B\nC]] [[B]]\n\n    [[A]]\n\n[^m]: [[A]] [[C\n", budget: map[agreementCitation]int{a: 3, b: 1, incomplete: 2}, want: map[agreementCitation]int{a: 3, b: 1}},
		{name: "each literal line retains a separate record", body: "[^n]: [[A\n[[B\n", budget: map[agreementCitation]int{incomplete: 2}},
		{name: "unfinished field cannot lend missing reference target", body: "[^n]: [[A\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{incomplete: 1}},
		{name: "valid field stays separate from missing reference", body: "[^n]: [[B]] [[A\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{b: 1, incomplete: 1}, want: map[agreementCitation]int{b: 1}},
		{name: "whole count refuses extra missing owner", body: "[^n]: [[A]] [[A\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{a: 1, incomplete: 1}},
		{name: "hidden fields cannot lend reference diagnostic", body: "%%\n[^n]: [[A]] [[B\n%%\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{commentA: 1, incomplete: 1}},
		{name: "escaped fields remain inactive beside unfinished field", body: "[^n]: [[A]] \\[[A]] [[B\n", budget: map[agreementCitation]int{a: 1, escapedA: 1, incomplete: 1}, want: map[agreementCitation]int{a: 1}},
		{name: "CommonMark alone uses reference", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A]] [[B\n"},
		{name: "page grammar alone uses reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]] [[B\n"},
		{name: "other URL inline owner makes readings disagree", body: "[^n]: https://example.invalid/[[A]] [[B\n"},
		{name: "used definition has no discarded owner", body: "ref[^n]\n\n[^n]: [[A]] [[B\n"},
		{name: "older unescaped scope stays unchanged", body: "[^n]: words [[A]]\n", oldText: map[agreementCitation]int{a: 1}},
		{name: "older escaped scope stays unchanged", body: "[^n]: [[A]] \\[[A]]\n", oldEscaped: map[agreementCitation]int{a: 1, escapedA: 1}},
		{name: "full recorded original retains all active and literal roles", body: "A\n===\n\ue0020\ue003É\n[^n]: [[A]]\n\n    [[B]]\n\\[[A]][[A\nB]] ^é\n> [!note] title\nÉ\n", budget: map[agreementCitation]int{a: 1, b: 1, escapedA: 1, incomplete: 1}, want: map[agreementCitation]int{a: 1, b: 1}},
		{name: "ordinary source remains outside", body: "[[A]] [[B\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementUnusedIncompleteWidgetBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete unused incomplete widget budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.oldText, agreementUnusedTextWidgetBudget(c.Body).Tuples); diff != "" {
				t.Fatalf("caught: changed earlier unused text boundary (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.oldEscaped, agreementUnusedEscapedWidgetBudget(c.Body).Tuples); diff != "" {
				t.Fatalf("caught: changed earlier unused escaped boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementSharedUnusedDiagnosticDifference(c, &f, &actual, r.Diagnostics, budget)
				if f.Property != "P0" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: unused incomplete widget borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: unused incomplete widget public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementSharedUnusedDiagnosticDrift(t, c, &f, &actual, r.Diagnostics, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete unused incomplete widget public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
