package judge_test

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// Distinct reference fields can cite the same name. Their hidden occurrences
// share one signed delta, so both complete declaration inventories contribute.
func agreementSharedReferenceBudget(scalar, compound agreementReferenceDestinations) agreementReferenceDestinations {
	if len(scalar.Diagnostics) == 0 || len(compound.Diagnostics) == 0 {
		return agreementReferenceDestinations{}
	}
	budget := agreementReferenceDestinations{Diagnostics: make(map[agreementCitation]int), Targets: make(map[string]int)}
	maps.Copy(budget.Diagnostics, scalar.Diagnostics)
	maps.Copy(budget.Targets, scalar.Targets)
	for tuple, count := range compound.Diagnostics {
		budget.Diagnostics[tuple] += count
	}
	for target, count := range compound.Targets {
		budget.Targets[target] += count
	}
	return budget
}

func TestAgreementSharedReferenceDebt(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body  string
		diagnostics map[agreementCitation]int
		targets     map[string]int
	}{
		{name: "shared name", body: "[n]: [[A]]\n[m]: [[A]][[B]]\n", diagnostics: map[agreementCitation]int{a: 2, b: 1}, targets: map[string]int{"A": 2, "B": 1}},
		{name: "duplicate label whole set", body: "[n]: [[A]]\n[n]: [[A]][[B]]\n[n]: [[B]]\n", diagnostics: map[agreementCitation]int{a: 2, b: 2}, targets: map[string]int{"A": 2, "B": 2}},
		{name: "distinct names", body: "[n]: [[A]]\n[m]: [[B]][[B]]\n", diagnostics: map[agreementCitation]int{a: 1, b: 2}, targets: map[string]int{"A": 1, "B": 2}},
		{name: "scalar scope stays separate", body: "[n]: [[A]]\n"},
		{name: "compound scope stays separate", body: "[n]: [[A]][[B]]\n"},
		{name: "live scope stays separate", body: "[[A]] [[B]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scalar := agreementReferenceDestinationBudget(tc.body)
			compound := agreementCompoundReferenceBudget(tc.body)
			original := agreementReferenceDestinations{Diagnostics: maps.Clone(scalar.Diagnostics), Targets: maps.Clone(scalar.Targets)}
			budget := agreementSharedReferenceBudget(scalar, compound)
			if diff := cmp.Diff(agreementReferenceDestinations{Diagnostics: tc.diagnostics, Targets: tc.targets}, budget); diff != "" {
				t.Fatalf("caught: shared reference declaration inventory (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(original, scalar); diff != "" {
				t.Fatalf("caught: shared reference observer changed scalar inventory:\n%s", diff)
			}
			if tc.diagnostics == nil {
				return
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementPageFailures(tc.body, &result, &observed)
			if len(failures) != len(tc.diagnostics)+len(tc.targets) {
				t.Fatalf("caught: shared reference public delta count=%d want=%d", len(failures), len(tc.diagnostics)+len(tc.targets))
			}
			for _, failure := range failures {
				kind, authority, wrong := agreementReferenceDestinationDifference(agreementCase{Body: tc.body}, &failure, budget)
				wantWrong := "page-diagnostic"
				if failure.Property == "P1" {
					wantWrong = "judge"
				}
				if kind != "debt" || authority != "#1011 unrendered reference declarations" || wrong != wantWrong {
					t.Fatalf("caught: shared reference public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
			}
		})
	}
}
