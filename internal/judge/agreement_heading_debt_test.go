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

// Heading declarations name their base through the canonical source-word
// reader. Repetition explains a suffix only when no authored name claims it.
func agreementDeclaredHeadingCounts(body string) map[string]int {
	if strings.Contains(body, "%%") || strings.Contains(body, "<!--") || strings.Contains(body, "[!") {
		return nil
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	counts := make(map[string]int)
	unclaimed := true
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		heading, ok := node.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		raw := string(heading.Lines().Value(source))
		base := graph.SectionID(render.HeadingWords(raw))
		if _, suffix, found := strings.CutLast(base, "-"); found {
			if _, err := strconv.Atoi(suffix); err == nil {
				unclaimed = false
				return ast.WalkStop, nil
			}
		}
		counts[base]++
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if !unclaimed {
		return nil
	}
	return counts
}

func agreementDuplicateHeadingDifference(c agreementCase, f *agreementFailure, counts map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P4" || f.Identity != "literal-heading-id" || f.Tuple != (agreementCitation{}) || f.Direction != "page-only" || f.Multiplicity != 1 || !f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Cut != "" {
		return "", "", ""
	}
	base, suffix, found := strings.CutLast(f.Fragment, "-")
	if !found {
		return "", "", ""
	}
	ordinal, err := strconv.Atoi(suffix)
	if err != nil || strconv.Itoa(ordinal) != suffix || ordinal < 2 || ordinal > counts[base] {
		return "", "", ""
	}
	return "debt", "#1011 stage 8", "judge"
}

func TestAgreementDuplicateHeadingDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "duplicate atx", body: "## A\n## A\n", want: map[string]int{"a": 2}},
		{name: "unrelated inline html", body: "text <em>outside</em>\n\n## A\n## A\n", want: map[string]int{"a": 2}},
		{name: "raw html owns apparent headings", body: "<div>\n## A\n## A\n</div>\n", want: map[string]int{}},
		{name: "unrelated escaped prose", body: "\\[[A]]\n\n## A\n## A\n", want: map[string]int{"a": 2}},
		{name: "escaped heading marker", body: "\\## A\n", want: map[string]int{}},
		{name: "escaped heading words", body: "## \\A\n## A\n", want: map[string]int{"a": 2}},
		{name: "three declarations", body: "## A\n## A\n## A\n", want: map[string]int{"a": 3}},
		{name: "setext and atx", body: "A\n===\n\n## A\n", want: map[string]int{"a": 2}},
		{name: "different declarations", body: "## A\n## B\n", want: map[string]int{"a": 1, "b": 1}},
		{name: "quoted declarations", body: "> ## A\n> ## A\n", want: map[string]int{"a": 2}},
		{name: "inline code is not a declaration", body: "## A\n\n`## A`\n", want: map[string]int{"a": 1}},
		{name: "fenced code is not a declaration", body: "## A\n\n```\n## A\n```\n", want: map[string]int{"a": 1}},
		{name: "authored suffix collision", body: "## A\n## A\n## A-2\n"},
		{name: "multiword suffix collision", body: "## Alpha beta\n## Alpha beta\n## Alpha beta-2\n"},
		{name: "formatted words share base", body: "## A\n## *A*\n", want: map[string]int{"a": 2}},
		{name: "alias names share base", body: "## [[A|alias]]\n## alias\n", want: map[string]int{"alias": 2}},
		{name: "admitted formatting shares base", body: "## <mark>A</mark>\n## A\n", want: map[string]int{"a": 2}},
		{name: "inert authored tag keeps its words", body: "## <em>A</em>\n## A\n", want: map[string]int{"em-a-em": 1, "a": 1}},
		{name: "empty words use fallback", body: "## !\n## !\n", want: map[string]int{"section": 2}},
		{name: "formatted authored suffix collision", body: "## A\n## A\n## *A-2*\n"},
		{name: "comment role needs ownership", body: "%%\n## A\n%%\n## A\n"},
		{name: "callout layout needs ownership", body: "> [!note] t\n> ## A\n> ## A\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			counts := agreementDeclaredHeadingCounts(tc.body)
			if diff := cmp.Diff(tc.want, counts); diff != "" {
				t.Fatalf("caught: heading declaration inventory (-want +got):\n%s", diff)
			}
			c := agreementCase{Body: tc.body}
			bases := []string{"unrelated"}
			for base := range tc.want {
				bases = append(bases, base)
			}
			slices.Sort(bases)
			for _, base := range slices.Compact(bases) {
				for ordinal := 1; ordinal <= 4; ordinal++ {
					f := agreementFailure{Property: "P4", Identity: "literal-heading-id", Fragment: base + "-" + strconv.Itoa(ordinal), Direction: "page-only", Multiplicity: 1, PagePresent: true}
					kind, authority, wrong := agreementDuplicateHeadingDifference(c, &f, counts)
					want := ordinal >= 2 && ordinal <= tc.want[base]
					if want && (kind != "debt" || authority != "#1011 stage 8" || wrong != "judge") || !want && kind != "" {
						t.Fatalf("caught: duplicate declaration ownership ordinal=%d counts=%v kind=%q authority=%q wrong=%q", ordinal, counts, kind, authority, wrong)
					}
					f.Multiplicity = 2
					if kind, _, _ := agreementDuplicateHeadingDifference(c, &f, counts); kind != "" {
						t.Fatalf("caught: non-unit heading delta admitted: %+v", f)
					}
				}
			}
		})
	}
}
