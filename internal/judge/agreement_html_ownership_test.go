package judge_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/wording"
)

// Raw HTML owns its lines until the grammar ends the block. Dialect shells
// must not inject a blank inside it and turn later literal backticks into code.
func agreementHTMLOwnership(t *testing.T) {
	t.Helper()
	for _, tc := range []struct {
		body         string
		want         []string
		callout      bool
		sequenceWant []string
		htmlSpans    []graph.Span
	}{
		{body: "</div>\n> [!note] title\n`open\n[[A]]\nclose`", want: []string{"A"}, htmlSpans: []graph.Span{{Start: 0, Stop: 41}}},
		{body: "<custom>\n> [!note] title\n`[[A]]`", want: []string{"A"}, htmlSpans: []graph.Span{{Start: 0, Stop: 32}}},
		{body: "</div>\n```\n[[A]]\n```", want: []string{"A"}, htmlSpans: []graph.Span{{Start: 0, Stop: 20}}},
		{body: "</div>\n\n> [!note] title\n> `[[A]]`\n> [[B]]\n", want: []string{"B"}, callout: true, sequenceWant: []string{"B"}, htmlSpans: []graph.Span{{Start: 0, Stop: 7}}},
	} {
		if diff := cmp.Diff(tc.htmlSpans, slices.Collect(graph.ReadBody(tc.body).HTMLBlocks())); diff != "" {
			t.Errorf("caught: S5 html-ownership complete original HTML spans body=%q (-want +got):\n%s", tc.body, diff)
		}
		page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
		result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
		actual := agreementObserve(t, result.HTML)
		if actual.CitationsInCode != 0 {
			t.Errorf("caught: S5 html-ownership body=%q code-citations=%d html=%q", tc.body, actual.CitationsInCode, result.HTML)
		}
		if got := strings.Contains(result.HTML, `class="callout`); got != tc.callout {
			t.Errorf("caught: S5 html-ownership body=%q callout=%t want=%t html=%q", tc.body, got, tc.callout, result.HTML)
		}
		var targets []string
		for _, citation := range actual.Citations {
			targets = append(targets, citation.Target)
		}
		if diff := cmp.Diff(tc.want, targets); diff != "" {
			t.Errorf("caught: S5 html-ownership complete page citations body=%q (-want +got):\n%s", tc.body, diff)
		}
		if diff := cmp.Diff(tc.want, judge.LinkTargets(tc.body), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("caught: S5 html-ownership complete judge citations body=%q (-want +got):\n%s", tc.body, diff)
		}
		var sequenceTargets []string
		for _, link := range sequence.LiveWikilinks(tc.body) {
			sequenceTargets = append(sequenceTargets, link.Target)
		}
		if diff := cmp.Diff(tc.sequenceWant, sequenceTargets); diff != "" {
			t.Errorf("caught: S5 html-ownership complete sequence citations body=%q (-want +got):\n%s", tc.body, diff)
		}
	}
	if *agreementSource != "" {
		t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%s", agreementMutationSourceDigest(t, *agreementSource))
	}
}

func TestAgreementHTMLOwnership(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-ownership")
}

func TestAgreementHTMLObservation(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-observation")
}

func TestAgreementHTMLProjection(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-projection")
}

func TestAgreementHTMLAccessor(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-accessor")
}

func TestAgreementHTMLPresentation(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-presentation")
}

func TestAgreementHTMLLineOwnership(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-lines")
}

func TestAgreementHTMLFenceOwnership(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-fence")
}

func TestAgreementHTMLCalloutOwnership(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-callout")
}

func TestAgreementHTMLSequenceOwnership(t *testing.T) {
	agreementHTMLOwnership(t)
	t.Log("AGREEMENT-INVOKED S5/stage5-html-sequence")
}
