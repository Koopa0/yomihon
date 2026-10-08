package judge_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// Authored suffixes can occupy the names repetition would otherwise create.
// Only generated places absent from the whole authored set belong to this debt.
func agreementHeadingCollisionIDs(body string) map[string]bool {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	var names []string
	authored := make(map[string]bool)
	numeric := false
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		heading, ok := node.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		name := graph.SectionID(render.HeadingWords(string(heading.Lines().Value(source))))
		names = append(names, name)
		authored[name] = true
		if _, suffix, found := strings.CutLast(name, "-"); found {
			if _, err := strconv.Atoi(suffix); err == nil {
				numeric = true
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if !numeric {
		return nil
	}
	used := make(map[string]bool)
	generated := make(map[string]bool)
	for _, name := range names {
		id := name
		for ordinal := 2; used[id]; ordinal++ {
			id = name + "-" + strconv.Itoa(ordinal)
		}
		used[id] = true
		if id != name && !authored[id] {
			generated[id] = true
		}
	}
	return generated
}

func agreementHeadingCollisionDifference(c agreementCase, f *agreementFailure, ids map[string]bool) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P4" || f.Identity != "literal-heading-id" || f.Tuple != (agreementCitation{}) || f.Direction != "page-only" || f.Multiplicity != 1 || !f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Cut != "" || !ids[f.Fragment] {
		return "", "", ""
	}
	return "debt", "#1011 stage 8", "judge"
}

func TestAgreementHeadingCollisionDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]bool
	}{
		{name: "authored suffix follows repetition", body: "## A\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "independent plain opener run", body: "## A\n> [!note] one\n> [!note] two\n> [!note] three\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "independent plain opener", body: "## A\n> [!note] title\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "independent closed comment", body: "<!--%%[!note]-->\n\n## A\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "independent root callout words", body: "show [!note] words\n\n## A\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "independent root percent comment", body: "%%[[Hidden]]%%\n\n## A\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "unrelated literal markers", body: "`%%<!--[!note]`\n\n## A\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "authored suffix precedes repetition", body: "## A-2\n## A\n## A\n", want: map[string]bool{"a-3": true}},
		{name: "multiple prior claims", body: "## A-2\n## A-3\n## A\n## A\n", want: map[string]bool{"a-4": true}},
		{name: "repeated numeric name", body: "## A-2\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "whole collision set", body: "## A\n## A\n## A-2\n## A-2\n", want: map[string]bool{"a-2-2": true, "a-2-3": true}},
		{name: "formatted authored claim", body: "## A\n## A\n## *A-2*\n", want: map[string]bool{"a-2-2": true}},
		{name: "alias authored claim", body: "## [[B|A]]\n## A\n## A-2\n", want: map[string]bool{"a-2-2": true}},
		{name: "numeric literal alone", body: "## A-2\n", want: map[string]bool{}},
		{name: "no numeric declaration", body: "## A\n## A\n"},
		{name: "code owns apparent numeric claim", body: "## A\n## A\n\n```\n## A-2\n```\n"},
		{name: "html owns apparent numeric claim", body: "<div>\n## A-2\n</div>\n\n## A\n## A\n"},
		{name: "comment role needs ownership", body: "%%\n## A-2\n%%\n## A\n## A\n"},
		{name: "callout layout needs ownership", body: "> [!note] t\n> ## A-2\n> ## A\n> ## A\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ids := agreementHeadingCollisionIDs(tc.body)
			if diff := cmp.Diff(tc.want, ids); diff != "" {
				t.Fatalf("caught: generated heading collision inventory (-want +got):\n%s", diff)
			}
			fragments := []string{"a", "a-2", "a-99", "unrelated"}
			for fragment := range tc.want {
				fragments = append(fragments, fragment)
			}
			slices.Sort(fragments)
			for _, fragment := range slices.Compact(fragments) {
				f := agreementFailure{Property: "P4", Identity: "literal-heading-id", Fragment: fragment, Direction: "page-only", Multiplicity: 1, PagePresent: true}
				kind, authority, wrong := agreementHeadingCollisionDifference(agreementCase{Body: tc.body}, &f, ids)
				if tc.want[fragment] && (kind != "debt" || authority != "#1011 stage 8" || wrong != "judge") || !tc.want[fragment] && kind != "" {
					t.Fatalf("caught: generated collision ownership fragment=%q kind=%q authority=%q wrong=%q", fragment, kind, authority, wrong)
				}
				f.Multiplicity++
				if kind, _, _ := agreementHeadingCollisionDifference(agreementCase{Body: tc.body}, &f, ids); kind != "" {
					t.Fatalf("caught: non-unit generated collision admitted: %+v", f)
				}
			}
		})
	}
}
