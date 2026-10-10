package judge_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// Plain words beside a single source-line widget do not own its target.
// The paragraph's text declarations exclude code, HTML and other inline owners.
func agreementProseTargetDebt(body string) agreementStandaloneTargets {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return agreementStandaloneTargets{}
	}
	budget := agreementStandaloneTargets{Page: make(map[string]int), Judge: make(map[string]int)}
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		paragraph, prose := node.(*ast.Paragraph)
		if !prose {
			continue
		}
		plain := true
		for child := paragraph.FirstChild(); child != nil; child = child.NextSibling() {
			if _, textual := child.(*ast.Text); !textual {
				plain = false
			}
		}
		if !plain {
			continue
		}
		for i := range paragraph.Lines().Len() {
			line := paragraph.Lines().At(i)
			raw := string(line.Value(source))
			before, rest, opened := strings.Cut(raw, "[[")
			inner, after, closed := strings.Cut(rest, "]]")
			if !opened || !closed || strings.ContainsAny(inner, "[]\n") || strings.ContainsFunc(before+after, func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && !unicode.IsSpace(r)
			}) {
				continue
			}
			field := "[[" + inner + "]]"
			link, cites := graph.ParseWikilink(inner)
			targets := judge.LinkTargets(field)
			if !cites || !strings.HasSuffix(link.Target, "\\") || len(targets) != 1 || targets[0] == link.Target {
				continue
			}
			budget.Page[link.Target]++
			budget.Judge[targets[0]]++
		}
	}
	if len(budget.Page) == 0 {
		return agreementStandaloneTargets{}
	}
	return budget
}

func TestAgreementProseTargetDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body  string
		page, check map[string]int
	}{
		{name: "separate source line", body: "É\n[[A\\]]", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "adjacent following word", body: "[[A\\]]text ", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "words on both sides", body: "before [[A\\]] after\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "words on other lines", body: "before\n[[A\\]]\nafter\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "whole target set", body: "one [[A\\]]\ntwo [[B\\]]\nthree [[A\\]]\n", page: map[string]int{"A\\": 2, "B\\": 1}, check: map[string]int{"A": 2, "B": 1}},
		{name: "independent live occurrence", body: "[[A]]\nwords [[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "independent opener run", body: "> [!note] one\n> [!note] two\n\nwords [[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "ordinary target agrees", body: "words [[A]]\n"},
		{name: "alias agrees", body: "words [[A\\|alias]]\n"},
		{name: "backslash before heading agrees", body: "words [[A\\#A]]\n"},
		{name: "escaped opener owns syntax", body: "words \\[[A\\]]\n"},
		{name: "code owns syntax", body: "words `[[A\\]]`\n"},
		{name: "paragraph contains another inline owner", body: "`code` words [[A\\]]\n"},
		{name: "another source line owns code", body: "`code`\nwords [[A\\]]\n"},
		{name: "quoted paragraph owns a container", body: "> words [[A\\]]\n"},
		{name: "list paragraph owns a container", body: "- words [[A\\]]\n"},
		{name: "reference destination is metadata", body: "[n]: [[A\\]]\n"},
		{name: "two widgets need combined ownership", body: "[[A\\]] [[A]]\n"},
		{name: "live comment owns syntax", body: "%%\nwords [[A\\]]\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementProseTargetDebt(tc.body)
			if diff := cmp.Diff(agreementStandaloneTargets{Page: tc.page, Judge: tc.check}, budget); diff != "" {
				t.Fatalf("caught: prose target inventory (-want +got):\n%s", diff)
			}
			if tc.page == nil {
				return
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementPageFailures(tc.body, &result, &observed)
			if len(failures) != len(tc.page)+len(tc.check) {
				t.Fatalf("caught: prose public delta count=%d want=%d", len(failures), len(tc.page)+len(tc.check))
			}
			for _, failure := range failures {
				kind, authority, wrong := agreementStandaloneTargetDifference(agreementCase{Body: tc.body}, &failure, budget)
				if kind != "debt" || authority != "#1011 stage 3" || wrong != "page" {
					t.Fatalf("caught: prose public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
			}
		})
	}
}
