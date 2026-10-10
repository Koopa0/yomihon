package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestAgreementDeclaredWidgetCode(t *testing.T) {
	t.Parallel()
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	suffix := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		citations  map[agreementCitation]int
		local      map[string]int
		failures   int
	}{
		{name: "heading field belongs to literal", body: "`open\n[[A#A]]\nclose`\n", citations: map[agreementCitation]int{section: 1}, failures: 2},
		{name: "raw suffix belongs to literal", body: "`open\n[[A\\]]\nclose`\n", citations: map[agreementCitation]int{suffix: 1}, failures: 2},
		{name: "whole tuple set", body: "`open\n[[A#A]]\n[[A\\]]\n[[A#A]]\nclose`\n", citations: map[agreementCitation]int{section: 2, suffix: 1}, failures: 4},
		{name: "quote span owns peeled segments", body: "> `open\n> [[A#A]]\n> close`\n", citations: map[agreementCitation]int{section: 1}, failures: 2},
		{name: "quote fence owns section", body: "> ```\n> [[A#A]]\n> ```\n", citations: map[agreementCitation]int{section: 1}, failures: 2},
		{name: "list fence owns suffix", body: "- item\n\n    ```\n    [[A\\]]\n    ```\n", citations: map[agreementCitation]int{suffix: 1}, failures: 2},
		{name: "info and outside words stay separate", body: "> ``` [[B]]\n> [[A#A]]\n> ```\n\n[[A]]\n", citations: map[agreementCitation]int{section: 1}},
		{name: "single-line code already agrees", body: "`[[A#A]]`\n"},
		{name: "root fence already agrees", body: "```\n[[A#A]]\n```\n"},
		{name: "inline words need occurrence ownership", body: "`open\nwords [[A#A]]\nclose`\n"},
		{name: "mixed widgets need occurrence ownership", body: "`open\n[[A#A]] [[A]]\nclose`\n"},
		{name: "callout profile stays separate", body: "> [!note] title\n> `open\n> [[A]]\n> close`\n"},
		{name: "live comment stays separate", body: "%%\n`open\n[[A]]\nclose`\n%%\n"},
		{name: "plain wrapped source", body: "`open\n[[A]]\nclose`\n", citations: map[agreementCitation]int{a: 1}, failures: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementDeclaredWrappedCode(tc.body)
			want := agreementWidgetCodeBudget{Citations: tc.citations, LocalHeadings: tc.local}
			if len(tc.citations) > 0 && tc.local == nil {
				want.LocalHeadings = map[string]int{}
			}
			if len(tc.local) > 0 && tc.citations == nil {
				want.Citations = map[agreementCitation]int{}
			}
			if diff := cmp.Diff(want, budget); diff != "" {
				t.Fatalf("caught: declared widget code inventory (-want +got):\n%s", diff)
			}
			if tc.failures == 0 {
				return
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementPageFailures(tc.body, &result, &observed)
			if len(failures) != tc.failures {
				t.Fatalf("caught: declared widget public delta count=%d want=%d", len(failures), tc.failures)
			}
			for _, failure := range failures {
				for _, altered := range []agreementCase{
					{Body: tc.body, Title: "Reading"},
					{Body: tc.body, Companions: capturedBodies{"A": "body"}},
				} {
					if kind, _, _ := agreementWidgetCodeDifference(altered, &failure, budget, &observed); kind != "" {
						t.Fatal("caught: widget code delta borrowed a different note context")
					}
				}
				kind, authority, wrong := agreementWidgetCodeDifference(agreementCase{Body: tc.body}, &failure, budget, &observed)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: declared widget public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.Tuple.Target = "unowned" },
					func(f *agreementFailure) { f.Direction = "judge-only" },
					func(f *agreementFailure) { f.PagePresent = true },
					func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" },
					func(f *agreementFailure) { f.Tuple.Section = "unowned" },
					func(f *agreementFailure) { f.Tuple.State = "unowned" },
					func(f *agreementFailure) { f.Identity = "unowned" },
					func(f *agreementFailure) { f.Property = "unowned" },
					func(f *agreementFailure) { f.Fragment = "unowned" },
					func(f *agreementFailure) { f.Cut = "unowned" },
					func(f *agreementFailure) { f.JudgeAccepted = true },
					func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := failure
					change(&changed)
					if kind, _, _ := agreementWidgetCodeDifference(agreementCase{Body: tc.body}, &changed, budget, &observed); kind != "" {
						t.Fatalf("caught: unrelated widget code delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
