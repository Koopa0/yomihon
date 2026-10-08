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
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// Other plain heading declarations reserve names before a wrapped widget.
// Every declaration participates in allocation, including ordinary names.
func agreementWrappedHeadingNamespaceIDs(body string) map[string]bool {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	var headings []*ast.Heading
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := node.(*ast.Heading); entering && ok {
			headings = append(headings, heading)
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if len(headings) < 2 {
		return nil
	}
	used, selected := make(map[string]bool), make(map[string]bool)
	for _, heading := range headings {
		base, wrapped := agreementWrappedHeadingID(heading, source)
		if !wrapped {
			if _, root := heading.Parent().(*ast.Document); !root {
				return nil
			}
			for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
				if _, plain := child.(*ast.Text); !plain {
					return nil
				}
			}
			raw := strings.TrimSpace(string(heading.Lines().Value(source)))
			if strings.ContainsFunc(raw, func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && !unicode.IsSpace(r) && r != '-'
			}) {
				return nil
			}
			base = graph.SectionID(raw)
		}
		id := base
		for ordinal := 2; used[id]; ordinal++ {
			id = base + "-" + strconv.Itoa(ordinal)
		}
		used[id] = true
		if wrapped {
			selected[id] = true
		}
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func TestAgreementWrappedHeadingNamespace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]bool
	}{
		{name: "ordinary declaration precedes widget", body: "A\n---\n[[A\nB]]A\n===\n", want: map[string]bool{"a-b-a": true}},
		{name: "ordinary declaration follows widget", body: "[[A\nB]]A\n=\n\n## A\n", want: map[string]bool{"a-b-a": true}},
		{name: "ordinary name reserves id", body: "## A-B-A\n\n[[A\nB]]A\n=\n", want: map[string]bool{"a-b-a-2": true}},
		{name: "whole wrapped declaration set", body: "[[A\nB]]A\n=\n\n[[A\nB]]A\n=\n", want: map[string]bool{"a-b-a": true, "a-b-a-2": true}},
		{name: "authored suffix participates", body: "## A-B-A\n## A-B-A-2\n\n[[A\nB]]A\n=\n", want: map[string]bool{"a-b-a-3": true}},
		{name: "single scope stays separate", body: "[[A\nB]]A\n=\n"},
		{name: "ordinary namespace stays separate", body: "## A\n## A\n"},
		{name: "alias owns other words", body: "## [[A|alias]]\n\n[[A\nB]]A\n=\n"},
		{name: "code owns other words", body: "## `A`\n\n[[A\nB]]A\n=\n"},
		{name: "quote owns other declaration", body: "> ## A\n\n[[A\nB]]A\n=\n"},
		{name: "quote owns widget", body: "## A\n\n> [[A\n> B]]A\n> =\n"},
		{name: "live comment owns context", body: "%%\n## A\n%%\n\n[[A\nB]]A\n=\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ids := agreementWrappedHeadingNamespaceIDs(tc.body)
			if diff := cmp.Diff(tc.want, ids); diff != "" {
				t.Fatalf("caught: wrapped heading namespace inventory (-want +got):\n%s", diff)
			}
			if tc.want == nil {
				return
			}
			if agreementWrappedHeadingIDs(tc.body) != nil {
				t.Fatal("caught: single heading scope admitted a namespace")
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementFragmentFailures(t, []agreementCase{{Body: tc.body}}, []agreementHTML{observed})[0]
			if len(failures) != len(tc.want) {
				t.Fatalf("caught: wrapped namespace public delta count=%d want=%d", len(failures), len(tc.want))
			}
			for _, failure := range failures {
				kind, authority, wrong := agreementWrappedHeadingDifference(agreementCase{Body: tc.body}, &failure, ids)
				if kind != "debt" || authority != "#1011 stage 8" || wrong != "judge" {
					t.Fatalf("caught: wrapped namespace public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
			}
		})
	}
}
