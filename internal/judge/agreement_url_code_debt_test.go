package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A URL elsewhere cannot give a wrapped source declaration's literal fields
// live ownership. Every actual code tuple for the target must match the source.
func TestAgreementURLCodeDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
		failures   int
	}{
		{name: "wrapped code beside URL", body: "`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n", want: map[string]int{"A": 1}, failures: 2},
		{name: "recorded punctuation beside URL", body: "`open\n[[A]]\nclose`## A\nhttps://example.invalid/`[[A]]` ~~~~\n\ue0020\ue003", want: map[string]int{"A": 1}, failures: 2},
		{name: "whole source inventory beside URL", body: "`open\n[[A]] [[A]] [[B]]\nclose`\n\nhttps://example.invalid/words\n", want: map[string]int{"A": 2, "B": 1}, failures: 4},
		{name: "quoted fence beside URL", body: "> ```\n> [[A]]\n> ```\n\nhttps://example.invalid/words\n", want: map[string]int{"A": 1}, failures: 2},
		{name: "ordinary URL span stays separate", body: "https://example.invalid/`[[A]]`\n", want: map[string]int{}},
		{name: "ordinary root fence stays separate", body: "```\n[[A]]\n```\n\nhttps://example.invalid/words\n", want: map[string]int{}},
		{name: "body without URL stays separate", body: "`open\n[[A]]\nclose`\n"},
		{name: "comment role stays separate", body: "%%\n`open\n[[A]]\nclose`\nhttps://example.invalid/words\n%%\n"},
		{name: "callout role stays separate", body: "> [!note] title\n> `open\n> [[A]]\n> close`\n\nhttps://example.invalid/words\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := agreementURLCodeDebtTargets(tc.body)
			if diff := cmp.Diff(tc.want, targets); diff != "" {
				t.Fatalf("caught: URL-separated code inventory (-want +got):\n%s", diff)
			}
			if tc.failures == 0 {
				return
			}
			if agreementCodeDebtTargets(tc.body) != nil {
				t.Fatal("caught: old code scope admitted URL")
			}
			c := agreementCase{Body: tc.body}
			r, a := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &a)
			if len(failures) != tc.failures {
				t.Fatalf("caught: URL code public delta count=%d want=%d %+v", len(failures), tc.failures, failures)
			}
			for i := range failures {
				f := &failures[i]
				kind, authority, wrong := agreementCodeCarrierDifference(c, f, targets, &a)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: URL code public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				absent := a
				absent.CodeCitations = nil
				if kind, _, _ := agreementCodeCarrierDifference(c, f, targets, &absent); kind != "" {
					t.Fatal("caught: URL code borrowed absent carriers")
				}
				drift := a
				drift.CodeCitations = append([]agreementCitation{}, a.CodeCitations...)
				for i := range drift.CodeCitations {
					if drift.CodeCitations[i].Target == f.Tuple.Target {
						drift.CodeCitations[i].Section = "unowned"
					}
				}
				if kind, _, _ := agreementCodeCarrierDifference(c, f, targets, &drift); kind != "" {
					t.Fatal("caught: URL code borrowed same-cardinality tuple drift")
				}
				excess := a
				excess.CodeCitations = append([]agreementCitation{}, a.CodeCitations...)
				excess.CodeCitations = append(excess.CodeCitations, agreementCitation{Target: f.Tuple.Target, State: "wikilink-broken"})
				if kind, _, _ := agreementCodeCarrierDifference(c, f, targets, &excess); kind != "" {
					t.Fatal("caught: URL code borrowed changed carrier count")
				}
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementCodeCarrierDifference(other, f, targets, &a); kind != "" {
						t.Fatal("caught: URL code borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "judge-only" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementCodeCarrierDifference(c, &changed, targets, &a); kind != "" {
						t.Fatalf("caught: URL code borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
		})
	}
}
