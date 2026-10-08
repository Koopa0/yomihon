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
)

// A heading consisting of one code span names literal bytes. Replacing a
// wikilink's target with its alias before reading code changes those words.
func agreementLiteralHeadingIDs(body string) map[string]bool {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementDeclarationMarkers(source, doc) {
		return nil
	}
	var sole *ast.Heading
	count := 0
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := node.(*ast.Heading); entering && ok {
			sole = heading
			count++
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if count != 1 {
		return nil
	}
	if _, root := sole.Parent().(*ast.Document); !root || sole.FirstChild() != sole.LastChild() {
		return nil
	}
	span, literal := sole.FirstChild().(*ast.CodeSpan)
	if !literal || span.FirstChild() != span.LastChild() {
		return nil
	}
	words, plain := span.FirstChild().(*ast.Text)
	if !plain || strings.Contains(string(words.Value(source)), "\n") {
		return nil
	}
	id := graph.SectionID(string(words.Value(source)))
	if id == graph.SectionID(render.HeadingWords(string(sole.Lines().Value(source)))) {
		return nil
	}
	return map[string]bool{id: true}
}

func agreementLiteralHeadingDifference(c agreementCase, f *agreementFailure, ids map[string]bool) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P4" || f.Identity != "literal-heading-id" || f.Tuple != (agreementCitation{}) || f.Direction != "page-only" || f.Multiplicity != 1 || !f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Cut != "" || !ids[f.Fragment] {
		return "", "", ""
	}
	return "debt", "#1011 stage 8", "judge"
}

func TestAgreementLiteralHeadingDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]bool
	}{
		{name: "alias remains literal", body: "## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true}},
		{name: "whole literal words", body: "## `[[B|alias]] [[A]]`\n", want: map[string]bool{"b-alias-a": true}},
		{name: "literal markers", body: "## `[[B|alias]] %%[!note]<!--`\n", want: map[string]bool{"b-alias-note": true}},
		{name: "independent prose", body: "show [!note] words\n\n## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true}},
		{name: "independent closed comment", body: "<!--hidden-->\n\n## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true}},
		{name: "plain code agrees", body: "## `[[A]]`\n"},
		{name: "ordinary alias is live", body: "## [[B|alias]]\n"},
		{name: "mixed prose needs word ownership", body: "## before `[[B|alias]]`\n"},
		{name: "trailing prose needs word ownership", body: "## `[[B|alias]]` after\n"},
		{name: "wrapped span needs page ownership", body: "## `open\n[[B|alias]]\nclose`\n"},
		{name: "multiple headings need namespace ownership", body: "## `[[B|alias]]`\n## A\n"},
		{name: "literal heading follows another declaration", body: "## A\n## `[[B|alias]]`\n"},
		{name: "quoted heading needs container ownership", body: "> ## `[[B|alias]]`\n"},
		{name: "code block is not a heading", body: "```\n## `[[B|alias]]`\n```\n"},
		{name: "live comment needs ownership", body: "%%\n## `[[B|alias]]`\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ids := agreementLiteralHeadingIDs(tc.body)
			if diff := cmp.Diff(tc.want, ids); diff != "" {
				t.Fatalf("caught: literal heading word inventory (-want +got):\n%s", diff)
			}
			fragments := []string{"alias", "b-alias-2", "unrelated"}
			for id := range tc.want {
				fragments = append(fragments, id)
			}
			for _, id := range fragments {
				f := agreementFailure{Property: "P4", Identity: "literal-heading-id", Fragment: id, Direction: "page-only", Multiplicity: 1, PagePresent: true}
				kind, authority, wrong := agreementLiteralHeadingDifference(agreementCase{Body: tc.body}, &f, ids)
				if tc.want[id] && (kind != "debt" || authority != "#1011 stage 8" || wrong != "judge") || !tc.want[id] && kind != "" {
					t.Fatalf("caught: literal heading ownership id=%q kind=%q authority=%q wrong=%q", id, kind, authority, wrong)
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.Direction = "judge-only" },
					func(f *agreementFailure) { f.PagePresent = false },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementLiteralHeadingDifference(agreementCase{Body: tc.body}, &changed, ids); kind != "" {
						t.Fatalf("caught: unrelated literal heading delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
