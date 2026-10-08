package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/wording"
)

// A link's transport spelling must not turn quoted text into a live citation.
func TestAgreementLinkifiedCode(t *testing.T) {
	for _, tc := range []struct {
		body string
		want []string
	}{
		{body: "url `[[A]]` [[B]]\n", want: []string{"B"}},
		{body: "https://example.invalid/`[[A]]`\n"},
		{body: "https://example.invalid/`[[A]]` [[A\\|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n`- item\n\n      > %%[[A]]%%\n\n[[A]]É\n- item\n\n      -->", want: []string{"A", "A", "A"}},
	} {
		body := tc.body
		page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
		result := page.HTML("Notes/Reading.md", "", body, wording.En)
		actual := agreementObserve(t, result.HTML)
		if actual.CitationsInCode != 0 {
			t.Errorf("caught: S5 linkified-code body=%q code-citations=%d html=%q", body, actual.CitationsInCode, result.HTML)
		}
		var targets []string
		for _, citation := range actual.Citations {
			targets = append(targets, citation.Target)
		}
		if diff := cmp.Diff(tc.want, targets); diff != "" {
			t.Errorf("caught: S5 linkified-code complete page citations body=%q (-want +got):\n%s", body, diff)
		}
		if diff := cmp.Diff(tc.want, judge.LinkTargets(body), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("caught: S5 linkified-code complete judge citations body=%q (-want +got):\n%s", body, diff)
		}
		var sequenceTargets []string
		for _, link := range sequence.LiveWikilinks(body) {
			sequenceTargets = append(sequenceTargets, link.Target)
		}
		if diff := cmp.Diff(tc.want, sequenceTargets); diff != "" {
			t.Errorf("caught: S5 linkified-code complete sequence citations body=%q (-want +got):\n%s", body, diff)
		}
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-linkified-code")
}
