package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"

	"github.com/koopa0/yomihon/internal/graph"
)

// A hidden field cannot lend its contribution to a missing reference with the
// same name. Comments retain their original ranges and every native code owner.
func agreementUnusedWidgetComments(body string, doc ast.Node) []graph.Span {
	var code []graph.Span
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if span, literal := agreementNativeCodeSpan(node); literal && !span.Zero() {
			code = append(code, span)
		}
		if fence, fenced := node.(*ast.FencedCodeBlock); fenced && fence.Info != nil {
			code = append(code, graph.Span{Start: fence.Info.Segment.Start, Stop: fence.Info.Segment.Stop})
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return graph.CommentZones(body, code)
}

func agreementUnusedWidgetCommentOwned(field graph.Span, comments []graph.Span) bool {
	for _, comment := range comments {
		if field.Start < comment.Stop && comment.Start < field.Stop {
			return true
		}
	}
	return false
}

func TestAgreementUnusedWidgetCommentRoles(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
	}{
		{name: "comment owns full discarded declaration", body: "%%\n[^n]: [[A]]\n\n%%\n", want: nil},
		{name: "comment owns part of field", body: "[^n]: [[A|%%hidden%%]]\n", want: nil},
		{name: "later comment owns discarded field", body: "%%unrelated%%\n\n%%\n[^n]: [[A]]\n\n%%\n", want: nil},
		{name: "inline code owns percent delimiter", body: "`%%`\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "wrapped code owns percent delimiter", body: "`open\n%%\nclose`\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "root fence owns percent delimiter", body: "```\n%%\n```\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "every code owner retains its delimiter", body: "`%%`\n\n```\n%%\n```\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "root fence info owns percent delimiter", body: "``` %%\n```\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "quote fence owns percent delimiter", body: "> ```\n> %%\n> ```\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "indented code owns percent delimiter", body: "    %%\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "closed unrelated comment retains field", body: "%%hidden%%\n\n[^n]: [[A]]\n", want: map[agreementCitation]int{a: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementSharedUnusedWidgetBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete unused widget comment roles (-want +got):\n%s", diff)
			}
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				fields := agreementSharedUnusedWidgetReading(tc.body, grammar)
				var tuples map[agreementCitation]int
				for _, field := range fields {
					if tuples == nil {
						tuples = make(map[agreementCitation]int)
					}
					tuples[field.Tuple]++
				}
				if diff := cmp.Diff(tc.want, tuples); diff != "" {
					t.Fatalf("caught: complete native unused widget comment ownership (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementUnusedWidgetCommentCannotBorrowMissingOwner(t *testing.T) {
	t.Parallel()
	c := agreementCase{Body: "%%\n[^n]: [[A]]\n\n%%\n\n[r]: [[A]]\n"}
	widgets := agreementSharedUnusedWidgetBudget(c.Body)
	citations := agreementSharedUnusedCitationBudget(c.Body)
	if len(widgets.Tuples) != 0 || len(citations.Targets) != 0 {
		t.Fatalf("caught: hidden unused fields lent a diagnostic or check contribution: %+v %+v", widgets.Tuples, citations.Targets)
	}
	r, actual := agreementIsolatedPage(t, c)
	var failures map[string]int
	for _, f := range agreementPageFailures(c.Body, &r, &actual) {
		if f.Property != "P0" && f.Property != "P1" {
			continue
		}
		if failures == nil {
			failures = make(map[string]int)
		}
		failures[agreementSignature(&f)] = f.Multiplicity
		if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, &f, &actual, r.Diagnostics, widgets); kind != "" {
			t.Fatal("caught: hidden unused field borrowed another diagnostic owner")
		}
		if kind, _, _ := agreementSharedUnusedCitationDifference(c, &f, &actual, citations); kind != "" {
			t.Fatal("caught: hidden unused field borrowed another check owner")
		}
	}
	want := map[string]int{
		agreementSignature(&agreementFailure{Property: "P0", Identity: "diagnostic-html", Direction: "diagnostic-only", Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, Multiplicity: 1}): 1,
		agreementSignature(&agreementFailure{Property: "P1", Identity: "citation-occurrences", Direction: "judge-only", Tuple: agreementCitation{Target: "A"}, Multiplicity: 1}):                           1,
	}
	if diff := cmp.Diff(want, failures); diff != "" {
		t.Fatalf("caught: complete hidden-field missing-owner public receipts (-want +got):\n%s", diff)
	}
}
