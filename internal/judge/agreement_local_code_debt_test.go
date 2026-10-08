package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// A same-note heading widget cites no file, but a live link inside code still
// violates literal ownership. Keep every declared local address in its budget.
func agreementLocalCodeHeadings(body string) map[string]int {
	if strings.Contains(body, "://") {
		return nil
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	headings := make(map[string]int)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		literal := ""
		switch node := node.(type) {
		case *ast.CodeSpan:
			first, firstOK := node.FirstChild().(*ast.Text)
			last, lastOK := node.LastChild().(*ast.Text)
			if firstOK && lastOK {
				content := string(source[first.Segment.Start:last.Segment.Stop])
				if strings.Contains(content, "\n") {
					literal = content
				}
			}
		case *ast.FencedCodeBlock:
			for parent := node.Parent(); parent != nil; parent = parent.Parent() {
				_, list := parent.(*ast.ListItem)
				_, quote := parent.(*ast.Blockquote)
				if list || quote {
					literal = string(node.Lines().Value(source))
					break
				}
			}
		}
		for line := range strings.SplitSeq(literal, "\n") {
			field := strings.TrimSpace(line)
			inner, opened := strings.CutPrefix(field, "[[")
			inner, closed := strings.CutSuffix(inner, "]]")
			if !opened || !closed || strings.ContainsAny(inner, "[]\n") {
				continue
			}
			link, cites := graph.ParseWikilink(inner)
			if cites || link.Target != "" || link.Heading == "" || link.Block != "" {
				continue
			}
			headings[link.Heading]++
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return headings
}

func agreementLocalCodeDifference(c agreementCase, f *agreementFailure, headings map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P2" || f.Identity != "wikilink-in-code" || f.Tuple != (agreementCitation{}) || f.Direction != "page-in-code" || f.Multiplicity <= 0 || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	count := 0
	for _, occurrences := range headings {
		count += occurrences
	}
	if count != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "page"
}

func TestAgreementLocalCodeDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "wrapped heading widget", body: "# A\n\n`open\n[[#A]]\nclose`\n", want: map[string]int{"A": 1}},
		{name: "missing heading remains a carrier", body: "`open\n[[#Missing]]\nclose`\n", want: map[string]int{"Missing": 1}},
		{name: "whole local address set", body: "# A\n# B\n\n`open\n[[#A]]\n[[#B]]\n[[#A]]\nclose`\n", want: map[string]int{"A": 2, "B": 1}},
		{name: "alias owns display only", body: "# A\n\n`open\n[[#A|label]]\nclose`\n", want: map[string]int{"A": 1}},
		{name: "quoted fence", body: "# A\n\n> ```\n> [[#A]]\n> ```\n", want: map[string]int{"A": 1}},
		{name: "list fence", body: "# A\n\n- item\n\n    ```\n    [[#A]]\n    ```\n", want: map[string]int{"A": 1}},
		{name: "info and outer addresses stay separate", body: "# A\n# B\n\n> ``` [[#B]]\n> [[#A]]\n> ```\n\n[[#B]]\n", want: map[string]int{"A": 1}},
		{name: "ordinary prose", body: "# A\n\n[[#A]]\n", want: map[string]int{}},
		{name: "single-line code", body: "# A\n\n`[[#A]]`\n", want: map[string]int{}},
		{name: "ordinary root fence", body: "# A\n\n```\n[[#A]]\n```\n", want: map[string]int{}},
		{name: "cross-file target has a different tuple", body: "`open\n[[A#A]]\nclose`\n", want: map[string]int{}},
		{name: "local block is shown text", body: "`open\n[[#^a]]\nclose`\n", want: map[string]int{}},
		{name: "block with heading is shown text", body: "`open\n[[^a#A]]\nclose`\n", want: map[string]int{}},
		{name: "empty heading is shown text", body: "`open\n[[#]]\nclose`\n", want: map[string]int{}},
		{name: "local embed is shown text", body: "`open\n![[#A]]\nclose`\n", want: map[string]int{}},
		{name: "escaped widget is shown text", body: "`open\n\\[[#A]]\nclose`\n", want: map[string]int{}},
		{name: "mixed source line needs occurrence ownership", body: "`open\n[[#A]] [[#B]]\nclose`\n", want: map[string]int{}},
		{name: "live comment needs ownership", body: "%%\n`open\n[[#A]]\nclose`\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			headings := agreementLocalCodeHeadings(tc.body)
			if diff := cmp.Diff(tc.want, headings); diff != "" {
				t.Fatalf("caught: local code heading inventory (-want +got):\n%s", diff)
			}
			if len(tc.want) == 0 {
				return
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementPageFailures(tc.body, &result, &observed)
			if len(failures) != 1 {
				t.Fatalf("caught: local code public delta count=%d want=1", len(failures))
			}
			failure := failures[0]
			kind, authority, wrong := agreementLocalCodeDifference(agreementCase{Body: tc.body}, &failure, headings)
			if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
				t.Fatalf("caught: local code public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
			}
			for _, change := range []func(*agreementFailure){
				func(f *agreementFailure) { f.Multiplicity++ },
				func(f *agreementFailure) { f.Tuple.Target = "A" },
				func(f *agreementFailure) { f.Tuple.State = "wikilink" },
				func(f *agreementFailure) { f.Direction = "page-only" },
				func(f *agreementFailure) { f.PagePresent = true },
			} {
				changed := failure
				change(&changed)
				if kind, _, _ := agreementLocalCodeDifference(agreementCase{Body: tc.body}, &changed, headings); kind != "" {
					t.Fatalf("caught: unrelated local code delta admitted: %+v", changed)
				}
			}
		})
	}
}
