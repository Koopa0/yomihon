package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/judge"
)

// An assigned reference keeps the definition alive. Its later paragraphs
// remain prose even though their authored indentation resembles code.
func agreementFootnoteContinuationTargets(body string) map[string]int {
	if !strings.Contains(body, "[^") {
		return nil
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	targets := make(map[string]int)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		footnote, ok := node.(*extast.Footnote)
		if !entering || !ok || footnote.Index < 0 {
			return ast.WalkContinue, nil
		}
		first := true
		for child := footnote.FirstChild(); child != nil; child = child.NextSibling() {
			if _, ok := child.(*ast.Paragraph); !ok {
				continue
			}
			if first {
				first = false
				continue
			}
			for _, target := range judge.LinkTargets(string(child.Lines().Value(source))) {
				targets[target]++
			}
		}
		return ast.WalkSkipChildren, nil
	}); err != nil {
		panic(err)
	}
	return targets
}

func agreementFootnoteContinuationDifference(c agreementCase, f *agreementFailure, targets map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "page-only" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 || targets[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 stage 4", "judge"
}

func TestAgreementFootnoteContinuationDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "used second paragraph", body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "independent closed comment", body: "<!--%%[!note]-->\n\nref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "comment closing line owns reference text", body: "<!--%%[!note]--> ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{}},
		{name: "independent plain opener run", body: "> [!note] one\n> [!note] two\n> [!note] three\n\nref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "independent plain opener", body: "> [!note] title\n\nref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "independent root callout words", body: "show [!note] words\n\nref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "independent root percent comment", body: "%%[[Hidden]]%%\n\nref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "unrelated literal markers", body: "`%%<!--[!note]` ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "unrelated inline html", body: "text <em>outside</em> ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "raw html owns apparent definition", body: "ref[^n]\n\n<div>\n[^n]: [[A]]\n\n    [[B]]\n</div>\n", want: map[string]int{}},
		{name: "unrelated escaped prose", body: "\\[[B]] ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "escaped reference discards continuation", body: "ref\\[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{}},
		{name: "escaped continuation target", body: "ref[^n]\n\n[^n]: [[A]]\n\n    \\[[B]]\n", want: map[string]int{}},
		{name: "two continuation occurrences", body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]] [[B]]\n", want: map[string]int{"B": 2}},
		{name: "independent live occurrence", body: "[[B]] ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"B": 1}},
		{name: "used first paragraph", body: "ref[^n]\n\n[^n]: [[A]]\n", want: map[string]int{}},
		{name: "unused definition", body: "[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{}},
		{name: "ordinary indented block", body: "[[A]]\n\n    [[B]]\n"},
		{name: "quoted continuation code", body: "ref[^n]\n\n[^n]: [[A]]\n\n    `[[B]]`\n", want: map[string]int{}},
		{name: "comment role needs ownership", body: "%%\nref[^n]\n%%\n\n[^n]: [[A]]\n\n    [[B]]\n"},
		{name: "callout layout needs ownership", body: "> [!note] ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := agreementFootnoteContinuationTargets(tc.body)
			if diff := cmp.Diff(tc.want, targets); diff != "" {
				t.Fatalf("caught: used continuation target inventory (-want +got):\n%s", diff)
			}
			c := agreementCase{Body: tc.body}
			f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: "B"}, Direction: "page-only", Multiplicity: max(1, tc.want["B"])}
			kind, authority, wrong := agreementFootnoteContinuationDifference(c, &f, targets)
			if tc.want["B"] > 0 && (kind != "debt" || authority != "#1011 stage 4" || wrong != "judge") || tc.want["B"] == 0 && kind != "" {
				t.Fatalf("caught: footnote continuation ownership kind=%q authority=%q wrong=%q", kind, authority, wrong)
			}
			for _, change := range []func(*agreementFailure){
				func(f *agreementFailure) { f.Multiplicity++ },
				func(f *agreementFailure) { f.Tuple.Target = "A" },
				func(f *agreementFailure) { f.Direction = "judge-only" },
			} {
				changed := f
				change(&changed)
				if kind, _, _ := agreementFootnoteContinuationDifference(c, &changed, targets); kind != "" {
					t.Fatalf("caught: unrelated continuation delta admitted: %+v", changed)
				}
			}
		})
	}
}
