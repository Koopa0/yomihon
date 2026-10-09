package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A terminal comment's payload is separate from earlier code declarations.
// All code carriers for a target must match its complete source-owned count.
func agreementCodeTailDifference(c agreementCase, f *agreementFailure, targets map[string]int, actual *agreementHTML) (kind, authority, wrong string) {
	expected := map[agreementCitation]int{{Target: f.Tuple.Target, State: "wikilink-broken"}: targets[f.Tuple.Target]}
	found := make(map[agreementCitation]int)
	for _, tuple := range actual.CodeCitations {
		if tuple.Target == f.Tuple.Target {
			found[tuple]++
		}
	}
	if !cmp.Equal(expected, found) {
		return "", "", ""
	}
	return agreementCodeDebtDifference(c, f, targets)
}

func TestAgreementCodeTailComment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
		failures   int
	}{
		{name: "wrapped code before terminal comment", body: "`open\n[[A]]\nclose`\n<!--", want: map[string]int{"A": 1}, failures: 2},
		{name: "whole code target set", body: "`open\n[[A]] [[A]] [[B]]\nclose`\n<!-- hidden\n", want: map[string]int{"A": 2, "B": 1}, failures: 4},
		{name: "quoted code before terminal comment", body: "> ```\n> [[A]]\n> ```\n\n<!-- hidden\n", want: map[string]int{"A": 1}, failures: 2},
		{name: "recorded code case", body: "``[[A]] [[A]]## !\n[[A\nB]]``\\[[A]]```` go [[A]]\nÉ\n<!--", want: map[string]int{"A": 2}, failures: 2},
		{name: "ordinary live occurrence remains separate", body: "[[A]]\n\n`open\n[[A]]\nclose`\n<!--\n", want: map[string]int{"A": 1}, failures: 2},
		{name: "comment owns apparent code", body: "<!--\n`open\n[[A]]\nclose`\n", want: map[string]int{}},
		{name: "root fence stays separate", body: "```\n[[A]]\n```\n<!--\n", want: map[string]int{}},
		{name: "single-line span stays separate", body: "`[[A]]`\n<!--\n", want: map[string]int{}},
		{name: "closed comment stays separate", body: "`open\n[[A]]\nclose`\n<!--hidden-->\n"},
		{name: "quote owns tail", body: "`open\n[[A]]\nclose`\n\n> <!--\n"},
		{name: "list owns tail", body: "`open\n[[A]]\nclose`\n\n- <!--\n"},
		{name: "raw HTML owns tail", body: "`open\n[[A]]\nclose`\n\n<div>\n<!--\n"},
		{name: "live percent marker remains", body: "%%\n`open\n[[A]]\nclose`\n<!--\n"},
		{name: "linkify needs ownership", body: "https://example.invalid/`open\n[[A]]\nclose`\n<!--\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := agreementCodeDebtTailTargets(tc.body)
			if diff := cmp.Diff(tc.want, targets); diff != "" {
				t.Fatalf("caught: terminal comment code inventory (-want +got):\n%s", diff)
			}
			if tc.failures == 0 {
				return
			}
			if agreementCodeDebtTargets(tc.body) != nil {
				t.Fatal("caught: old code scope admitted terminal comment")
			}
			c := agreementCase{Body: tc.body}
			r, a := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &a)
			if len(failures) != tc.failures {
				t.Fatalf("caught: terminal code public delta count=%d want=%d", len(failures), tc.failures)
			}
			for i := range failures {
				f := &failures[i]
				kind, authority, wrong := agreementCodeTailDifference(c, f, targets, &a)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: terminal code public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				absent := a
				absent.CodeCitations = nil
				if kind, _, _ := agreementCodeTailDifference(c, f, targets, &absent); kind != "" {
					t.Fatal("caught: terminal code borrowed absent carriers")
				}
				extra := a
				extra.CodeCitations = append([]agreementCitation{}, a.CodeCitations...)
				extra.CodeCitations = append(extra.CodeCitations, agreementCitation{Target: f.Tuple.Target, State: "wikilink-broken", Section: "unowned"})
				if kind, _, _ := agreementCodeTailDifference(c, f, targets, &extra); kind != "" {
					t.Fatal("caught: terminal code borrowed another tuple")
				}
				changedTuple := a
				changedTuple.CodeCitations = append([]agreementCitation{}, a.CodeCitations...)
				for i := range changedTuple.CodeCitations {
					if changedTuple.CodeCitations[i].Target == f.Tuple.Target {
						changedTuple.CodeCitations[i].Section = "unowned"
					}
				}
				if kind, _, _ := agreementCodeTailDifference(c, f, targets, &changedTuple); kind != "" {
					t.Fatal("caught: terminal code borrowed same-cardinality different tuples")
				}
				excessCount := a
				excessCount.CodeCitations = append([]agreementCitation{}, a.CodeCitations...)
				excessCount.CodeCitations = append(excessCount.CodeCitations, agreementCitation{Target: f.Tuple.Target, State: "wikilink-broken"})
				if kind, _, _ := agreementCodeTailDifference(c, f, targets, &excessCount); kind != "" {
					t.Fatal("caught: terminal code borrowed changed carrier count")
				}
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementCodeTailDifference(other, f, targets, &a); kind != "" {
						t.Fatal("caught: terminal code borrowed another vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" },
					func(f *agreementFailure) { f.Identity = "unowned" },
					func(f *agreementFailure) { f.Direction = "judge-only" },
					func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementCodeTailDifference(c, &changed, targets, &a); kind != "" {
						t.Fatalf("caught: unrelated terminal code signature accepted: %+v", changed)
					}
				}
			}
		})
	}
}
