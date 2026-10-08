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
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// A widget split between source lines remains text on the page. Its bracket
// boundaries still separate words when the heading's id is stamped.
func agreementWrappedHeadingIDs(body string) map[string]bool {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
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
	id, owned := agreementWrappedHeadingID(sole, source)
	if !owned {
		return nil
	}
	return map[string]bool{id: true}
}

func agreementWrappedHeadingID(sole *ast.Heading, source []byte) (string, bool) {
	if _, root := sole.Parent().(*ast.Document); !root || sole.Lines().Len() < 2 {
		return "", false
	}
	for child := sole.FirstChild(); child != nil; child = child.NextSibling() {
		if _, plain := child.(*ast.Text); !plain {
			return "", false
		}
	}
	raw := strings.TrimSpace(string(sole.Lines().Value(source)))
	inner, opened := strings.CutPrefix(raw, "[[")
	inner, tail, closed := strings.Cut(inner, "]]")
	if !opened || !closed || !strings.Contains(inner, "\n") || strings.ContainsAny(inner, "[]\\<&`") || strings.ContainsFunc(tail, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && !unicode.IsSpace(r)
	}) {
		return "", false
	}
	id := graph.SectionID(raw)
	if id == graph.SectionID(render.HeadingWords(raw)) {
		return "", false
	}
	return id, true
}

func agreementWrappedHeadingDifference(c agreementCase, f *agreementFailure, ids map[string]bool) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P4" || f.Identity != "literal-heading-id" || f.Tuple != (agreementCitation{}) || f.Direction != "page-only" || f.Multiplicity != 1 || !f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Cut != "" || !ids[f.Fragment] {
		return "", "", ""
	}
	return "debt", "#1011 stage 8", "judge"
}

func TestAgreementWrappedHeadingDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]bool
	}{
		{name: "brackets separate trailing words", body: "[[A\nB]]A\n=\n", want: map[string]bool{"a-b-a": true}},
		{name: "alias remains text", body: "[[A\nB|alias]]\n=\n", want: map[string]bool{"a-b-alias": true}},
		{name: "fragment remains text", body: "[[A\nB#place]]After\n=\n", want: map[string]bool{"a-b-place-after": true}},
		{name: "three source lines", body: "[[A\nB\nC]]After\n=\n", want: map[string]bool{"a-b-c-after": true}},
		{name: "independent opener run", body: "> [!note] one\n> [!note] two\n\n[[A\nB]]A\n=\n", want: map[string]bool{"a-b-a": true}},
		{name: "already agreeing words", body: "[[A\nB]]\n=\n"},
		{name: "single line alias is live", body: "## [[A|alias]]\n"},
		{name: "single line widget inside multiline heading", body: "[[A|alias]]\nword\n=\n"},
		{name: "paragraph has no id", body: "[[A\nB]]A\n"},
		{name: "multiple headings own a namespace", body: "## A\n\n[[A\nB]]A\n=\n"},
		{name: "quoted heading owns a container", body: "> [[A\n> B]]A\n> =\n"},
		{name: "emphasis owns words", body: "[[*A*\nB]]A\n=\n"},
		{name: "escaped bracket owns syntax", body: "\\[[A\nB]]A\n=\n"},
		{name: "another widget owns words", body: "[[A\nB]]A [[C|alias]]\n=\n"},
		{name: "nested bracket syntax", body: "[[A\n[B]]]A\n=\n"},
		{name: "unpaired nested bracket", body: "[[A[\nB]]A\n=\n"},
		{name: "live comments own words", body: "%%\n[[A\nB]]A\n=\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ids := agreementWrappedHeadingIDs(tc.body)
			if diff := cmp.Diff(tc.want, ids); diff != "" {
				t.Fatalf("caught: wrapped heading word inventory (-want +got):\n%s", diff)
			}
			if tc.want == nil {
				return
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementFragmentFailures(t, []agreementCase{{Body: tc.body}}, []agreementHTML{observed})[0]
			if len(failures) != len(tc.want) {
				t.Fatalf("caught: wrapped heading public delta count=%d want=%d", len(failures), len(tc.want))
			}
			for _, failure := range failures {
				kind, authority, wrong := agreementWrappedHeadingDifference(agreementCase{Body: tc.body}, &failure, ids)
				if kind != "debt" || authority != "#1011 stage 8" || wrong != "judge" {
					t.Fatalf("caught: wrapped heading public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
				for _, c := range []agreementCase{{Body: tc.body, Title: "A"}, {Body: tc.body, Companions: capturedBodies{"Notes/A.md": "text"}}} {
					if kind, _, _ := agreementWrappedHeadingDifference(c, &failure, ids); kind != "" {
						t.Fatal("caught: wrapped heading context drift admitted")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.PagePresent = false },
					func(f *agreementFailure) { f.JudgeAccepted = true },
					func(f *agreementFailure) { f.Tuple.Target = "unrelated" },
					func(f *agreementFailure) { f.Fragment = "unrelated" },
				} {
					changed := failure
					change(&changed)
					if kind, _, _ := agreementWrappedHeadingDifference(agreementCase{Body: tc.body}, &changed, ids); kind != "" {
						t.Fatalf("caught: unrelated wrapped heading delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
