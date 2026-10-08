package judge_test

import (
	"slices"
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
func agreementLinkifiedCode(t *testing.T) {
	t.Helper()
	for _, tc := range []struct {
		body      string
		want      []string
		autolinks []graph.Span
	}{
		{body: "url `[[A]]` [[B]]\n", want: []string{"B"}},
		{body: "https://example.invalid/`[[A]]`\n", autolinks: []graph.Span{{Start: 0, Stop: 31}}},
		{body: "start https://example.invalid/`[[A]]` [[B]]\n", want: []string{"B"}, autolinks: []graph.Span{{Start: 6, Stop: 37}}},
		{body: "<https://example.invalid/`[[A]]`> [[B]]\n", want: []string{"B"}, autolinks: []graph.Span{{Start: 1, Stop: 32}}},
		{body: "www.example.invalid/`[[A]]` [[B]]\n", want: []string{"B"}, autolinks: []graph.Span{{Start: 0, Stop: 27}}},
		{body: "https://example.invalid/`[[A]]` [[A\\|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n`- item\n\n      > %%[[A]]%%\n\n[[A]]É\n- item\n\n      -->", want: []string{"A", "A", "A"}, autolinks: []graph.Span{{Start: 0, Stop: 31}}},
	} {
		body := tc.body
		if diff := cmp.Diff(tc.autolinks, slices.Collect(graph.ReadBody(body).Autolinks())); diff != "" {
			t.Errorf("caught: S5 linkified-code complete original URL spans body=%q (-want +got):\n%s", body, diff)
		}
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
	if *agreementSource != "" {
		t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%s", agreementMutationSourceDigest(t, *agreementSource))
	}
}

func TestAgreementLinkifiedCode(t *testing.T) {
	agreementLinkifiedCode(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-linkified-code")
}

func TestAgreementAutolinkObservation(t *testing.T) {
	agreementLinkifiedCode(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-autolink-observation")
}

func TestAgreementAutolinkProjection(t *testing.T) {
	agreementLinkifiedCode(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-autolink-projection")
}

func TestAgreementAutolinkAccessor(t *testing.T) {
	agreementLinkifiedCode(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-autolink-accessor")
}

func TestAgreementAutolinkCheck(t *testing.T) {
	agreementLinkifiedCode(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-autolink-check")
}

func TestAgreementAutolinkSequence(t *testing.T) {
	agreementLinkifiedCode(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-autolink-sequence")
}
