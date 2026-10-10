package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

// Text declarations can own punctuation and container prose without owning code
// or metadata. Preserve both readers' targets for every eligible source widget.
func agreementTextTargetDebt(body string) agreementStandaloneTargets {
	if strings.Contains(body, "://") || strings.Contains(body, "%%") || strings.Contains(body, "[^") {
		return agreementStandaloneTargets{}
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return agreementStandaloneTargets{}
	}
	budget := agreementStandaloneTargets{Page: make(map[string]int), Judge: make(map[string]int)}
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node.(type) {
		case *ast.Paragraph, *ast.TextBlock:
		default:
			return ast.WalkContinue, nil
		}
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if _, plain := child.(*ast.Text); !plain {
				return ast.WalkSkipChildren, nil
			}
		}
		for i := range node.Lines().Len() {
			line := node.Lines().At(i)
			raw := string(line.Value(source))
			for {
				opening := strings.Index(raw, "[[")
				if opening < 0 {
					break
				}
				rest := raw[opening+2:]
				inner, after, closed := strings.Cut(rest, "]]")
				if !closed {
					break
				}
				prefix := raw[:opening]
				slashes := len(prefix) - len(strings.TrimRight(prefix, "\\"))
				raw = after
				if slashes%2 != 0 || strings.HasSuffix(prefix, "!") || strings.ContainsAny(inner, "[]\n") {
					continue
				}
				link, cites := graph.ParseWikilink(inner)
				targets := judge.LinkTargets("[[" + inner + "]]")
				if !cites || !strings.HasSuffix(link.Target, "\\") || len(targets) != 1 || targets[0] == link.Target {
					continue
				}
				budget.Page[link.Target]++
				budget.Judge[targets[0]]++
			}
		}
		return ast.WalkSkipChildren, nil
	}); err != nil {
		panic(err)
	}
	if len(budget.Page) == 0 {
		return agreementStandaloneTargets{}
	}
	return budget
}

func TestAgreementTextTargetDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body  string
		page, check map[string]int
	}{
		{name: "unmatched punctuation", body: "[[A\\]]  ```\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "list text", body: "1. [[A\\]]text ", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "quote text", body: "> words [[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "whole shared set", body: "[[A\\]] [[B\\]] [[A\\]]\n", page: map[string]int{"A\\": 2, "B\\": 1}, check: map[string]int{"A": 2, "B": 1}},
		{name: "whole source lines", body: "[[A\\]]\n[[B\\]]\n[[A\\]]\n", page: map[string]int{"A\\": 2, "B\\": 1}, check: map[string]int{"A": 2, "B": 1}},
		{name: "separate containers", body: "- [[A\\]]\n\n> [[B\\]]\n", page: map[string]int{"A\\": 1, "B\\": 1}, check: map[string]int{"A": 1, "B": 1}},
		{name: "ordinary words", body: "before [[A\\]] after\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "ordinary live occurrence", body: "[[A]] [[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "code owner", body: "`[[A\\]]`\n"},
		{name: "another inline owner", body: "`code` [[A\\]]\n"},
		{name: "root fence owner", body: "```\n[[A\\]]\n```\n"},
		{name: "quote fence owner", body: "> ```\n> [[A\\]]\n> ```\n"},
		{name: "reference metadata", body: "[n]: [[A\\]]\n"},
		{name: "escaped opening", body: "\\[[A\\]]\n"},
		{name: "embed widget", body: "![[A\\]]\n"},
		{name: "alias agrees", body: "[[A\\|alias]]\n"},
		{name: "local address", body: "[[#A\\]]\n"},
		{name: "html owner", body: "<div>\n[[A\\]]\n</div>\n"},
		{name: "percent owner", body: "%%[[A\\]]%%\n"},
		{name: "linkify ownership", body: "https://example.invalid/[[A\\]]\n"},
		{name: "relocated footnote owner", body: "ref[^n]\n\n[^n]: [[A\\]]\n"},
		{name: "callout title owner", body: "> [!note] [[A\\]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := agreementStandaloneTargets{Page: tc.page, Judge: tc.check}
			if diff := cmp.Diff(want, agreementTextTargetDebt(tc.body)); diff != "" {
				t.Fatalf("caught: text-owned target inventory (-want +got):\n%s", diff)
			}
			if tc.page == nil {
				return
			}
			c := agreementCase{Body: tc.body}
			result, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &result, &actual)
			if len(failures) != len(tc.page)+len(tc.check) {
				t.Fatalf("caught: text target public delta count=%d want=%d", len(failures), len(tc.page)+len(tc.check))
			}
			for i := range failures {
				failure := &failures[i]
				kind, authority, wrong := agreementStandaloneTargetDifference(c, failure, want)
				if kind != "debt" || authority != "#1011 stage 3" || wrong != "page" {
					t.Fatalf("caught: text target public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(failure))
				}
			}
		})
	}
}
