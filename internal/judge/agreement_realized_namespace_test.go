package judge_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// Repetition owns a generated name only when every declaration agrees with
// the complete page namespace. Hidden or rewritten headings break that receipt.
func agreementRealizedNamespaceIDs(body string, actual *agreementHTML) map[string]bool {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var names []string
	authored := make(map[string]bool)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := node.(*ast.Heading); entering && ok {
			base := graph.SectionID(render.HeadingWords(string(heading.Lines().Value(source))))
			names = append(names, base)
			authored[base] = true
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	used, selected := make(map[string]bool), make(map[string]bool)
	var expected []string
	for _, base := range names {
		id := base
		for ordinal := 2; used[id]; ordinal++ {
			id = base + "-" + strconv.Itoa(ordinal)
		}
		used[id] = true
		expected = append(expected, id)
		if id != base && !authored[id] {
			selected[id] = true
		}
	}
	if len(selected) == 0 || !cmp.Equal(expected, actual.Headings) {
		return nil
	}
	return selected
}

func TestAgreementRealizedHeadingNamespace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]bool
	}{
		{name: "plain repetition", body: "## A\n## A\n", want: map[string]bool{"a-2": true}},
		{name: "whole generated set", body: "## A\n## A\n## A\n", want: map[string]bool{"a-2": true, "a-3": true}},
		{name: "every base participates", body: "## A\n## B\n## B\n", want: map[string]bool{"b-2": true}},
		{name: "canonical alias words", body: "## [[B|A]]\n## A\n", want: map[string]bool{"a-2": true}},
		{name: "other headings reserve names", body: "## A-2\n## A\n## A\n", want: map[string]bool{"a-3": true}},
		{name: "authored names stay separate", body: "## A\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "callout prose leaves namespace intact", body: "> [!note] title\n> words\n\n## A\n## A\n", want: map[string]bool{"a-2": true}},
		{name: "callout contains declarations", body: "> [!note] title\n> ## A\n> ## A\n", want: map[string]bool{"a-2": true}},
		{name: "title link leaves namespace intact", body: "> [!note] [[B|alias]]\n> ## A\n> ## A\n", want: map[string]bool{"a-2": true}},
		{name: "independent percent passage", body: "%%\nhidden\n%%\n## A\n## A\n", want: map[string]bool{"a-2": true}},
		{name: "terminal comment after declarations", body: "## A\n## A\n<!-- hidden\n", want: map[string]bool{"a-2": true}},
		{name: "used footnote leaves namespace intact", body: "## A\n## A\nref[^n]\n\n[^n]: words\n", want: map[string]bool{"a-2": true}},
		{name: "ordinary name stays separate", body: "## A\n"},
		{name: "code is not a declaration", body: "```\n## A\n## A\n```\n"},
		{name: "hidden declarations break receipt", body: "%%\n## A\n## A\n%%\n"},
		{name: "partially hidden namespace breaks receipt", body: "## A\n%%\n## A\n%%\n"},
		{name: "literal alias words break receipt", body: "## `[[B|alias]]`\n## `[[B|alias]]`\n"},
		{name: "unwritten embed words break receipt", body: "## ![[B|alias]]\n## ![[B|alias]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			_, actual := agreementIsolatedPage(t, c)
			ids := agreementRealizedNamespaceIDs(tc.body, &actual)
			if diff := cmp.Diff(tc.want, ids); diff != "" {
				t.Fatalf("caught: realized heading namespace inventory (-want +got):\n%s", diff)
			}
			if tc.want == nil {
				return
			}
			for _, drift := range [][]string{
				nil,
				actual.Headings[:len(actual.Headings)-1],
				append(slices.Clone(actual.Headings), "unowned"),
			} {
				changed := actual
				changed.Headings = drift
				if agreementRealizedNamespaceIDs(tc.body, &changed) != nil {
					t.Fatal("caught: incomplete or extra page namespace borrowed declaration ownership")
				}
			}
			changed := actual
			changed.Headings = slices.Clone(actual.Headings)
			changed.Headings[0], changed.Headings[1] = changed.Headings[1], changed.Headings[0]
			if agreementRealizedNamespaceIDs(tc.body, &changed) != nil {
				t.Fatal("caught: reordered page namespace borrowed declaration ownership")
			}
			failures := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})[0]
			if len(failures) != len(tc.want) {
				t.Fatalf("caught: realized namespace public delta count=%d want=%d", len(failures), len(tc.want))
			}
			for _, failure := range failures {
				kind, authority, wrong := agreementWrappedHeadingDifference(c, &failure, ids)
				if kind != "debt" || authority != "#1011 stage 8" || wrong != "judge" {
					t.Fatalf("caught: realized namespace public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
			}
		})
	}
}
