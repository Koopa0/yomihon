package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/wording"
)

// A definition's unindented paragraph continuation disappears with its owner.
func TestAgreementUnusedFootnoteContinuation(t *testing.T) {
	for _, tc := range []struct {
		body string
		want []string
	}{
		{body: "[^unused]: [[A]]\n[[B]]\n"},
		{body: "[^unused]: [[A]]\n\\\\[[B]]\n"},
		{body: "[^unused]: [[A]]\n\n[[B]]\n", want: []string{"B"}},
		{body: "ref[^used]\n\n[^used]: [[A]]\n[[B]]\n", want: []string{"A", "B"}},
	} {
		page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
		actual := agreementObserve(t, page.HTML("Notes/Reading.md", "", tc.body, wording.En).HTML)
		var targets []string
		for _, citation := range actual.Citations {
			targets = append(targets, citation.Target)
		}
		if diff := cmp.Diff(tc.want, targets); diff != "" {
			t.Errorf("caught: S5 unused-continuation page body=%q (-want +got):\n%s", tc.body, diff)
		}
		if diff := cmp.Diff(tc.want, judge.LinkTargets(tc.body)); diff != "" {
			t.Errorf("caught: S5 unused-continuation check body=%q (-want +got):\n%s", tc.body, diff)
		}
		var sequenceTargets []string
		for _, link := range sequence.LiveWikilinks(tc.body) {
			sequenceTargets = append(sequenceTargets, link.Target)
		}
		if diff := cmp.Diff(tc.want, sequenceTargets); diff != "" {
			t.Errorf("caught: S5 unused-continuation sequence body=%q (-want +got):\n%s", tc.body, diff)
		}
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-unused-continuation")
}
