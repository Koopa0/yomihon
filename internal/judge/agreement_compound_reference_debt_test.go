package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAgreementCompoundReferenceDebt(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	h := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	raw := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body  string
		diagnostics map[agreementCitation]int
		targets     map[string]int
	}{
		{name: "complete compound field", body: "[n]: [[A]][[B]]\n", diagnostics: map[agreementCitation]int{a: 1, b: 1}, targets: map[string]int{"A": 1, "B": 1}},
		{name: "duplicate occurrences", body: "[n]: [[A]][[A]]\n", diagnostics: map[agreementCitation]int{a: 2}, targets: map[string]int{"A": 2}},
		{name: "duplicate label declarations", body: "[n]: [[A]][[B]]\n[n]: [[B]][[A]]\n", diagnostics: map[agreementCitation]int{a: 2, b: 2}, targets: map[string]int{"A": 2, "B": 2}},
		{name: "heading field", body: "[n]: [[A#A]][[B]]\n", diagnostics: map[agreementCitation]int{h: 1, b: 1}, targets: map[string]int{"A": 1, "B": 1}},
		{name: "raw target field", body: "[n]: [[A\\]][[B]]\n", diagnostics: map[agreementCitation]int{raw: 1, b: 1}, targets: map[string]int{"A": 1, "B": 1}},
		{name: "angle field", body: "[n]: <[[A]][[B]]>\n", diagnostics: map[agreementCitation]int{a: 1, b: 1}, targets: map[string]int{"A": 1, "B": 1}},
		{name: "independent plain opener", body: "> [!note] title\n\n[n]: [[A]][[B]]\n", diagnostics: map[agreementCitation]int{a: 1, b: 1}, targets: map[string]int{"A": 1, "B": 1}},
		{name: "single field stays separate", body: "[n]: [[A]]\n"},
		{name: "ordinary prose stays separate", body: "[[A]][[B]]\n"},
		{name: "quoted field needs container ownership", body: "> [n]: [[A]][[B]]\n"},
		{name: "code field is displayed", body: "```\n[n]: [[A]][[B]]\n```\n"},
		{name: "prefix URL bytes need ownership", body: "[n]: before[[A]][[B]]\n"},
		{name: "suffix URL bytes need ownership", body: "[n]: [[A]][[B]]after\n"},
		{name: "nested brackets need ownership", body: "[n]: [[[A]]][[B]]\n"},
		{name: "embed fields need ownership", body: "[n]: ![[A]][[B]]\n"},
		{name: "empty local field needs ownership", body: "[n]: [[#A]][[B]]\n"},
		{name: "title field needs ownership", body: "[n]: [[A]][[B]] \"[[A]]\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementCompoundReferenceBudget(tc.body)
			want := agreementReferenceDestinations{Diagnostics: tc.diagnostics, Targets: tc.targets}
			if diff := cmp.Diff(want, budget); diff != "" {
				t.Fatalf("caught: compound reference destination inventory (-want +got):\n%s", diff)
			}
			for tuple, count := range tc.diagnostics {
				f := agreementFailure{Property: "P0", Identity: "diagnostic-html", Tuple: tuple, Direction: "diagnostic-only", Multiplicity: count}
				kind, _, wrong := agreementReferenceDestinationDifference(agreementCase{Body: tc.body}, &f, budget)
				if kind != "debt" || wrong != "page-diagnostic" {
					t.Fatalf("caught: compound reference diagnostic ownership kind=%q wrong=%q", kind, wrong)
				}
			}
			for target, count := range tc.targets {
				f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: target}, Direction: "judge-only", Multiplicity: count}
				kind, _, wrong := agreementReferenceDestinationDifference(agreementCase{Body: tc.body}, &f, budget)
				if kind != "debt" || wrong != "judge" {
					t.Fatalf("caught: compound reference citation ownership kind=%q wrong=%q", kind, wrong)
				}
			}
		})
	}
}
