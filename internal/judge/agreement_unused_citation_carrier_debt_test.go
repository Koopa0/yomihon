package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAgreementUnusedCitationCarriers(t *testing.T) {
	t.Parallel()
	original := "<!--\n[[A]]\n-->\\`ref[^n]\n`[[A]]` ^é\n[[A]]```\n[^n]: [[A]]\n\n    [[B]]\n"
	for _, tc := range []struct{ name, body, target, section string }{
		{"recorded original block role", original, "B", ""},
		{"renamed owned field", strings.Replace(original, "[[B]]", "[[C]]", 1), "C", ""},
		{"owned section field", strings.Replace(original, "[[B]]", "[[B#C]]", 1), "B", "C"},
		{"real reference keeps definition live", "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", "", ""},
		{"grammar-dependent use", "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]]\n\n    [[B]]\n", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementExclusiveUnusedFieldBudget(c.Body)
			var want map[agreementCitation]int
			if tc.target != "" {
				want = map[agreementCitation]int{{Target: tc.target, Section: tc.section, State: "wikilink-broken"}: 1}
			}
			if diff := cmp.Diff(want, budget); diff != "" {
				t.Fatalf("caught: unused citation source inventory (-want +got):\n%s", diff)
			}
			agreementAssertHiddenCitationProof(t, c, budget)
		})
	}
}
