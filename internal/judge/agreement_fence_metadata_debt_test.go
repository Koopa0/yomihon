package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// A fence's info field is a declaration, not displayed prose. Container
// prefixes can hide that role from the page's source-line replacement.
func agreementContainerFenceDiagnostics(body string) map[agreementCitation]int {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementDeclarationMarkers(source, doc) {
		return nil
	}
	diagnostics := make(map[agreementCitation]int)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		fence, isFence := node.(*ast.FencedCodeBlock)
		if !entering || !isFence || fence.Info == nil {
			return ast.WalkContinue, nil
		}
		container := false
		for parent := fence.Parent(); parent != nil; parent = parent.Parent() {
			switch parent.(type) {
			case *ast.ListItem, *ast.Blockquote:
				container = true
			}
		}
		if !container {
			return ast.WalkContinue, nil
		}
		field := string(fence.Info.Segment.Value(source))
		inner, open := strings.CutPrefix(field, "[[")
		inner, closed := strings.CutSuffix(inner, "]]")
		if !open || !closed || strings.ContainsAny(inner, "]\n") {
			return ast.WalkContinue, nil
		}
		link, cites := graph.ParseWikilink(inner)
		if cites {
			diagnostics[agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}]++
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return diagnostics
}

func agreementContainerFenceDiagnosticDifference(c agreementCase, f *agreementFailure, diagnostics map[agreementCitation]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 || diagnostics[f.Tuple] != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 fence declaration ownership", "page-diagnostic"
}

func TestAgreementContainerFenceMetadataDebt(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
	}{
		{name: "quoted info", body: "> ```[[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "list info", body: "1. ```[[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "nested quoted info", body: "> > ```[[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "whole info set", body: "> ```[[A]]\n> ```\n\n1. ```[[B]]\n", want: map[agreementCitation]int{a: 1, b: 1}},
		{name: "duplicate info fields", body: "> ```[[A]]\n> ```\n\n> ```[[A]]\n", want: map[agreementCitation]int{a: 2}},
		{name: "heading info field", body: "> ```[[A#A]]\n", want: map[agreementCitation]int{section: 1}},
		{name: "content is another field", body: "> ```[[A]]\n> [[B]]\n> ```\n", want: map[agreementCitation]int{a: 1}},
		{name: "literal marker payload", body: "> ```[[A]]\n> %%<!--[!note]\n", want: map[agreementCitation]int{a: 1}},
		{name: "root info is outside container debt", body: "```[[A]]\n", want: map[agreementCitation]int{}},
		{name: "plain quoted fence", body: "> ```\n> [[A]]\n", want: map[agreementCitation]int{}},
		{name: "quoted prose is not info", body: "> [[A]]\n", want: map[agreementCitation]int{}},
		{name: "prefixed info needs token ownership", body: "> ```go [[A]]\n", want: map[agreementCitation]int{}},
		{name: "compound info needs token ownership", body: "> ```[[A]][[B]]\n", want: map[agreementCitation]int{}},
		{name: "indented example is not fence info", body: "    > ```[[A]]\n", want: map[agreementCitation]int{}},
		{name: "live comment needs info ownership", body: "%%\n> ```[[A]]\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			diagnostics := agreementContainerFenceDiagnostics(tc.body)
			if diff := cmp.Diff(tc.want, diagnostics); diff != "" {
				t.Fatalf("caught: container fence diagnostic inventory (-want +got):\n%s", diff)
			}
			tuples := []agreementCitation{{Target: "unrelated", State: "wikilink-broken"}}
			for tuple := range tc.want {
				tuples = append(tuples, tuple)
			}
			for _, tuple := range tuples {
				f := agreementFailure{Property: "P0", Identity: "diagnostic-html", Tuple: tuple, Direction: "diagnostic-only", Multiplicity: max(1, tc.want[tuple])}
				kind, authority, wrong := agreementContainerFenceDiagnosticDifference(agreementCase{Body: tc.body}, &f, diagnostics)
				if tc.want[tuple] > 0 && (kind != "debt" || authority != "#1011 fence declaration ownership" || wrong != "page-diagnostic") || tc.want[tuple] == 0 && kind != "" {
					t.Fatalf("caught: fence info diagnostic ownership kind=%q authority=%q wrong=%q", kind, authority, wrong)
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.PagePresent = true },
					func(f *agreementFailure) { f.Tuple.SourceRole = "unrelated" },
					func(f *agreementFailure) { f.Direction = "page-only" },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementContainerFenceDiagnosticDifference(agreementCase{Body: tc.body}, &changed, diagnostics); kind != "" {
						t.Fatalf("caught: unrelated fence diagnostic delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
