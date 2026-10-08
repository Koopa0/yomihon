package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/judge"
)

// A fence's info field is part of its code declaration. Its target-like text
// does not become a citation in the note's prose.
func agreementFenceInfoTargets(body string) map[string]int {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementDeclarationMarkers(source, doc) {
		return nil
	}
	targets := make(map[string]int)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		fence, ok := node.(*ast.FencedCodeBlock)
		if !entering || !ok || fence.Info == nil {
			return ast.WalkContinue, nil
		}
		for _, target := range judge.LinkTargets(string(fence.Info.Segment.Value(source))) {
			targets[target]++
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return targets
}

func agreementFenceInfoDifference(c agreementCase, f *agreementFailure, targets map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "judge-only" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 || targets[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 stage 4", "judge"
}

func TestAgreementFenceInfoDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "info occurrence", body: "``` [[A]]\n```\n", want: map[string]int{"A": 1}},
		{name: "independent closed comment", body: "<!--%%[!note]-->\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}},
		{name: "independent root percent comment", body: "%%[[Hidden]]%%\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}},
		{name: "unrelated literal markers", body: "`%%<!--[!note]`\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}},
		{name: "unrelated inline html", body: "text <em>outside</em>\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}},
		{name: "raw html owns apparent opener", body: "<div>\n``` [[A]]\n```\n</div>\n", want: map[string]int{}},
		{name: "repeated info occurrences", body: "``` [[A]] [[A]]\n```\n", want: map[string]int{"A": 2}},
		{name: "two targets", body: "``` [[A]] [[B]]\n```\n", want: map[string]int{"A": 1, "B": 1}},
		{name: "unclosed fence", body: "``` [[A]]\n", want: map[string]int{"A": 1}},
		{name: "independent live occurrence", body: "[[A]]\n\n``` [[A]]\n[[B]]\n```\n", want: map[string]int{"A": 1}},
		{name: "unrelated escaped prose", body: "\\[[B]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}},
		{name: "escaped info target", body: "``` \\[[A]]\n```\n", want: map[string]int{}},
		{name: "ordinary prose", body: "[[A]]\n", want: map[string]int{}},
		{name: "fence content", body: "```\n[[A]]\n```\n", want: map[string]int{}},
		{name: "indented literal opener", body: "    ``` [[A]]\n", want: map[string]int{}},
		{name: "comment role needs ownership", body: "%%\n``` [[A]]\n```\n%%\n"},
		{name: "callout layout needs ownership", body: "> [!note] t\n> ``` [[A]]\n> ```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := agreementFenceInfoTargets(tc.body)
			if diff := cmp.Diff(tc.want, targets); diff != "" {
				t.Fatalf("caught: fence info target inventory (-want +got):\n%s", diff)
			}
			c := agreementCase{Body: tc.body}
			for _, target := range []string{"A", "B", "Unrelated"} {
				f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: target}, Direction: "judge-only", Multiplicity: max(1, tc.want[target])}
				kind, authority, wrong := agreementFenceInfoDifference(c, &f, targets)
				if tc.want[target] > 0 && (kind != "debt" || authority != "#1011 stage 4" || wrong != "judge") || tc.want[target] == 0 && kind != "" {
					t.Fatalf("caught: fence info ownership target=%q kind=%q authority=%q wrong=%q", target, kind, authority, wrong)
				}
				f.Multiplicity++
				if kind, _, _ := agreementFenceInfoDifference(c, &f, targets); kind != "" {
					t.Fatalf("caught: fence info delta exceeds source occurrence count: %+v", f)
				}
			}
		})
	}
}
