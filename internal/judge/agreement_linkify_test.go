package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// A link's transport spelling must not turn quoted text into a live citation.
func TestAgreementLinkifiedCode(t *testing.T) {
	for _, body := range []string{
		"url `[[A]]` [[B]]\n",
		"https://example.invalid/`[[A]]`\n",
		"https://example.invalid/`[[A]]` [[A\\|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n`- item\n\n      > %%[[A]]%%\n\n[[A]]É\n- item\n\n      -->",
	} {
		page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
		result := page.HTML("Notes/Reading.md", "", body, wording.En)
		actual := agreementObserve(t, result.HTML)
		if actual.CitationsInCode != 0 {
			t.Errorf("caught: S5 linkified-code body=%q code-citations=%d html=%q", body, actual.CitationsInCode, result.HTML)
		}
		if body == "url `[[A]]` [[B]]\n" {
			want := []agreementCitation{{Target: "B", State: "wikilink-broken"}}
			if diff := cmp.Diff(want, actual.Citations); diff != "" {
				t.Errorf("caught: S5 linkified-code live control (-want +got):\n%s", diff)
			}
		}
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-linkified-code")
}
