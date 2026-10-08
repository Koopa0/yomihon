package judge_test

import (
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// Plain heading declarations share their folded base. Repetition explains a
// numeric suffix only when no authored name can itself claim that suffix.
func agreementPlainHeadingCounts(body string) map[string]int {
	if strings.Contains(body, "%%") || strings.Contains(body, "<") || strings.Contains(body, "[!") || strings.Contains(body, "\\") {
		return nil
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	counts := make(map[string]int)
	plain := true
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		heading, ok := node.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		raw := strings.TrimSpace(string(heading.Lines().Value(source)))
		if raw == "" || strings.ContainsFunc(raw, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsSpace(r) && r != '-'
		}) {
			plain = false
			return ast.WalkStop, nil
		}
		base := graph.SectionID(raw)
		if at := strings.LastIndexByte(base, '-'); at >= 0 {
			if _, err := strconv.Atoi(base[at+1:]); err == nil {
				plain = false
				return ast.WalkStop, nil
			}
		}
		counts[base]++
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if !plain {
		return nil
	}
	return counts
}

func agreementDuplicateHeadingDifference(c agreementCase, f *agreementFailure, counts map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P4" || f.Identity != "literal-heading-id" || f.Tuple != (agreementCitation{}) || f.Direction != "page-only" || f.Multiplicity != 1 || !f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Cut != "" {
		return "", "", ""
	}
	at := strings.LastIndexByte(f.Fragment, '-')
	if at < 0 {
		return "", "", ""
	}
	base, suffix := f.Fragment[:at], f.Fragment[at+1:]
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
		{name: "three declarations", body: "## A\n## A\n## A\n", want: map[string]int{"a": 3}},
		{name: "setext and atx", body: "A\n===\n\n## A\n", want: map[string]int{"a": 2}},
		{name: "different declarations", body: "## A\n## B\n", want: map[string]int{"a": 1, "b": 1}},
		{name: "quoted declarations", body: "> ## A\n> ## A\n", want: map[string]int{"a": 2}},
		{name: "inline code is not a declaration", body: "## A\n\n`## A`\n", want: map[string]int{"a": 1}},
		{name: "fenced code is not a declaration", body: "## A\n\n```\n## A\n```\n", want: map[string]int{"a": 1}},
		{name: "authored suffix collision", body: "## A\n## A\n## A-2\n"},
		{name: "multiword suffix collision", body: "## Alpha beta\n## Alpha beta\n## Alpha beta-2\n"},
		{name: "formatted words need ownership", body: "## A\n## *A*\n"},
		{name: "comment role needs ownership", body: "%%\n## A\n%%\n## A\n"},
		{name: "callout layout needs ownership", body: "> [!note] t\n> ## A\n> ## A\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			counts := agreementPlainHeadingCounts(tc.body)
			if diff := cmp.Diff(tc.want, counts); diff != "" {
				t.Fatalf("caught: plain heading declaration inventory (-want +got):\n%s", diff)
			}
			c := agreementCase{Body: tc.body}
			for ordinal := 1; ordinal <= 4; ordinal++ {
				f := agreementFailure{Property: "P4", Identity: "literal-heading-id", Fragment: "a-" + strconv.Itoa(ordinal), Direction: "page-only", Multiplicity: 1, PagePresent: true}
				kind, authority, wrong := agreementDuplicateHeadingDifference(c, &f, counts)
				want := ordinal >= 2 && ordinal <= tc.want["a"]
				if want && (kind != "debt" || authority != "#1011 stage 8" || wrong != "judge") || !want && kind != "" {
					t.Fatalf("caught: duplicate declaration ownership ordinal=%d counts=%v kind=%q authority=%q wrong=%q", ordinal, counts, kind, authority, wrong)
				}
				f.Multiplicity = 2
				if kind, _, _ := agreementDuplicateHeadingDifference(c, &f, counts); kind != "" {
					t.Fatalf("caught: non-unit heading delta admitted: %+v", f)
				}
			}
		})
	}
}
