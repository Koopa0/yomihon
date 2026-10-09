package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/judge"
)

// Wrapped spans and fences inside containers own their literal bytes where a
// line-by-line replacement currently treats those bytes as prose.
func agreementCodeDebtTargets(body string) map[string]int {
	return agreementCodeDebtTargetProfile(body, false)
}

func agreementCodeDebtTailTargets(body string) map[string]int {
	return agreementCodeDebtTargetProfile(body, true)
}

func agreementCodeDebtTargetProfile(body string, tail bool) map[string]int {
	if strings.Contains(body, "://") {
		return nil
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	markers := agreementPlainOpenerDeclarationMarkers(source, doc)
	if tail {
		markers = agreementTailHTMLCommentMarkers(source, doc)
	}
	if !markers {
		return nil
	}
	targets := make(map[string]int)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		literal := ""
		switch node := node.(type) {
		case *ast.CodeSpan:
			first, ok := node.FirstChild().(*ast.Text)
			last, lastOK := node.LastChild().(*ast.Text)
			if !ok || !lastOK {
				return ast.WalkContinue, nil
			}
			content := string(source[first.Segment.Start:last.Segment.Stop])
			if strings.Contains(content, "\n") {
				literal = content
			}
		case *ast.FencedCodeBlock:
			for parent := node.Parent(); parent != nil; parent = parent.Parent() {
				if _, list := parent.(*ast.ListItem); list {
					literal = string(node.Lines().Value(source))
					break
				}
				if _, quote := parent.(*ast.Blockquote); quote {
					literal = string(node.Lines().Value(source))
					break
				}
			}
		}
		if literal == "" {
			return ast.WalkContinue, nil
		}
		for _, target := range judge.LinkTargets(literal) {
			targets[target]++
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return targets
}

func agreementCodeDebtDifference(c agreementCase, f *agreementFailure, targets map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Multiplicity <= 0 || targets[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	if f.Property == "P1" && f.Identity == "citation-occurrences" && f.Direction == "page-only" && f.Tuple.State == "" {
		return "debt", "#1011 stage 5", "page"
	}
	if f.Property == "P2" && f.Identity == "wikilink-in-code" && f.Direction == "page-in-code" && f.Tuple.State == "wikilink-broken" {
		return "debt", "#1011 stage 5", "page"
	}
	return "", "", ""
}

func TestAgreementCodeDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{name: "wrapped span", body: "`open\n[[A]]\nclose`\n", want: 1},
		{name: "independent closed comment", body: "<!--%%[!note]-->\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "independent plain opener run", body: "> [!note] one\n> [!note] two\n> [!note] three\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "independent plain opener", body: "> [!note] title\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "independent root callout words", body: "show [!note] words\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "independent root percent comment", body: "%%[[Hidden]]%%\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "unrelated literal markers", body: "`%%<!--[!note]`\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "unrelated inline html", body: "text <em>outside</em>\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "raw html owns apparent literal", body: "<div>\n`open\n[[A]]\nclose`\n</div>\n"},
		{name: "two wrapped occurrences", body: "`open\n[[A]] [[A]]\nclose`\n", want: 2},
		{name: "independent live occurrence", body: "[[A]]\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "list fence", body: "- item\n\n    ```\n    [[A]]\n    ```\n", want: 1},
		{name: "quoted fence", body: "> ```\n> [[A]]\n> ```\n", want: 1},
		{name: "unclosed quoted fence", body: "> ```\n> [[A]]\n", want: 1},
		{name: "nested quoted fence", body: "> > ```\n> > [[A]]\n> > ```\n", want: 1},
		{name: "quoted fence whole occurrences", body: "> ```\n> [[A]] [[A]]\n> ```\n", want: 2},
		{name: "quoted fence excludes info target", body: "> ``` [[B]]\n> [[A]]\n> ```\n", want: 1},
		{name: "quoted fence excludes distinct outer target", body: "> ```\n> [[A]]\n> ```\n\n[[B]]\n", want: 1},
		{name: "quoted fence excludes outer target", body: "> ```\n> [[A]]\n> ```\n\n[[A]]\n", want: 1},
		{name: "quoted fence ends at root paragraph", body: "> ```\n> [[A]]\n\n[[A]]\n", want: 1},
		{name: "quote without fence", body: "> [[A]]\n"},
		{name: "ordinary prose", body: "[[A]]\n"},
		{name: "single line span", body: "`[[A]]`\n"},
		{name: "ordinary fence", body: "```\n[[A]]\n```\n"},
		{name: "indented block", body: "    [[A]]\n"},
		{name: "comment role needs ownership", body: "%%\n`open\n[[A]]\nclose`\n%%\n"},
		{name: "callout layout needs ownership", body: "> [!note] t\n> `open\n> [[A]]\n> close`\n"},
		{name: "unrelated escaped prose", body: "\\[[A]]\n\n`open\n[[A]]\nclose`\n", want: 1},
		{name: "escaped opening delimiter", body: "\\`open\n[[A]]\nclose`\n"},
		{name: "escaped target within literal", body: "`open\n\\[[A]]\nclose`\n"},
		{name: "linkify needs ownership", body: "https://example.invalid/`open\n[[A]]\nclose`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := agreementCodeDebtTargets(tc.body)
			want := map[string]int{}
			if tc.want > 0 {
				want["A"] = tc.want
			}
			if len(targets) != 0 || len(want) != 0 {
				if diff := cmp.Diff(want, targets); diff != "" {
					t.Fatalf("caught: wrapped/list code target inventory (-want +got):\n%s", diff)
				}
			}
			c := agreementCase{Body: tc.body}
			for _, f := range []agreementFailure{
				{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: "A"}, Direction: "page-only", Multiplicity: max(1, tc.want)},
				{Property: "P2", Identity: "wikilink-in-code", Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, Direction: "page-in-code", Multiplicity: max(1, tc.want)},
			} {
				kind, authority, wrong := agreementCodeDebtDifference(c, &f, targets)
				if tc.want > 0 && (kind != "debt" || authority != "#1011 stage 5" || wrong != "page") || tc.want == 0 && kind != "" {
					t.Fatalf("caught: code node ownership budget=%d kind=%q authority=%q wrong=%q", tc.want, kind, authority, wrong)
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.Tuple.Target = "B" },
					func(f *agreementFailure) { f.Direction = "judge-only" },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementCodeDebtDifference(c, &changed, targets); kind != "" {
						t.Fatalf("caught: unrelated code delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
