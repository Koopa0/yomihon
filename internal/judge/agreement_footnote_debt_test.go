package judge_test

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

var agreementFootnoteTargetsKey = parser.NewContextKey()
var agreementFootnoteGrammar = goldmark.New(
	goldmark.WithExtensions(extension.Footnote),
	goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(agreementFootnoteDeclarations{}, 998))),
)

type agreementFootnoteDeclarations struct{}

// The declaration remains available before the extension removes unused
// definitions. Its assigned reference index distinguishes used definitions.
func (agreementFootnoteDeclarations) Transform(doc *ast.Document, reader text.Reader, context parser.Context) {
	targets, ok := context.Get(agreementFootnoteTargetsKey).(map[string]int)
	if !ok {
		panic("missing footnote observation context")
	}
	source := reader.Source()
	addresses, addressObservation := context.Get(agreementFootnoteAddressesKey).(map[string]int)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		footnote, ok := node.(*extast.Footnote)
		if !entering || !ok || footnote.Index >= 0 {
			return ast.WalkContinue, nil
		}
		if err := ast.Walk(footnote, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch child.(type) {
			case *ast.Paragraph, *ast.TextBlock:
				var body strings.Builder
				for i := range child.Lines().Len() {
					line := child.Lines().At(i)
					body.Write(line.Value(source))
					if addressObservation && addresses != nil {
						raw := strings.TrimSuffix(strings.TrimSuffix(string(line.Value(source)), "\n"), "\r")
						if address := render.BlockAddress(raw); address != "" {
							addresses[graph.FoldFragment(address)]++
						}
					}
				}
				for _, target := range judge.LinkTargets(body.String()) {
					targets[target]++
				}
			}
			return ast.WalkContinue, nil
		}); err != nil {
			return ast.WalkStop, err
		}
		return ast.WalkSkipChildren, nil
	}); err != nil {
		panic(err)
	}
}

func agreementUnusedFootnoteTargets(body string) map[string]int {
	if !strings.Contains(body, "[^") {
		return nil
	}
	targets := make(map[string]int)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, targets)
	source := []byte(body)
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	return targets
}

func agreementUnusedFootnoteDifference(c agreementCase, f *agreementFailure, targets map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Multiplicity <= 0 || targets[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	if f.Property == "P0" && f.Identity == "diagnostic-html" && f.Direction == "diagnostic-only" && f.Tuple.State == "wikilink-broken" {
		return "debt", "#1011 stage 5", "page-diagnostic"
	}
	if f.Property == "P1" && f.Identity == "citation-occurrences" && f.Direction == "judge-only" && f.Tuple.State == "" {
		return "debt", "#1011 stage 5", "judge"
	}
	return "", "", ""
}

func TestAgreementUnusedFootnoteDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{name: "unused", body: "[^unused]: [[A]]\n", want: 1},
		{name: "independent closed comment", body: "<!--%%[!note]-->\n\n[^unused]: [[A]]\n", want: 1},
		{name: "independent plain opener run", body: "> [!note] one\n> [!note] two\n> [!note] three\n\n[^unused]: [[A]]\n", want: 1},
		{name: "independent plain opener", body: "> [!note] title\n\n[^unused]: [[A]]\n", want: 1},
		{name: "independent root callout words", body: "show [!note] words\n\n[^unused]: [[A]]\n", want: 1},
		{name: "independent root percent comment", body: "%%[[Hidden]]%%\n\n[^unused]: [[A]]\n", want: 1},
		{name: "unrelated literal markers", body: "`%%<!--[!note]`\n\n[^unused]: [[A]]\n", want: 1},
		{name: "two occurrences", body: "[^unused]: [[A]] [[A]]\n", want: 2},
		{name: "used", body: "ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "ordinary prose", body: "[[A]]\n"},
		{name: "unrelated escaped prose", body: "\\[[A]]\n\n[^unused]: [[A]]\n", want: 1},
		{name: "escaped reference leaves definition unused", body: "\\ref\\[^n]\n\n[^n]: [[A]]\n", want: 1},
		{name: "escaped definition marker", body: "\\[^unused]: [[A]]\n"},
		{name: "escaped definition target", body: "[^unused]: \\[[A]]\n"},
		{name: "independent live occurrence", body: "[[A]]\n\n[^unused]: [[A]]\n", want: 1},
		{name: "comment role needs ownership", body: "%%\n[^unused]: [[A]]\n%%\n"},
		{name: "callout layout needs ownership", body: "> [!note] t\n> [^unused]: [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := agreementUnusedFootnoteTargets(tc.body)
			if got := targets["A"]; got != tc.want {
				t.Fatalf("caught: unused definition target budget got=%d want=%d", got, tc.want)
			}
			if len(targets) > 0 && (len(targets) != 1 || tc.want == 0) {
				t.Fatalf("caught: unexpected unused target set: %v", targets)
			}
			if tc.want == 0 {
				return
			}
			c := agreementCase{Body: tc.body}
			for _, f := range []agreementFailure{
				{Property: "P0", Identity: "diagnostic-html", Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, Direction: "diagnostic-only", Multiplicity: tc.want},
				{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: "A"}, Direction: "judge-only", Multiplicity: tc.want},
			} {
				wantWrong := "judge"
				if f.Property == "P0" {
					wantWrong = "page-diagnostic"
				}
				if kind, _, wrong := agreementUnusedFootnoteDifference(c, &f, targets); kind != "debt" || wrong != wantWrong {
					t.Fatalf("caught: unused declaration lacks classified ownership: %+v", f)
				}
				for _, changed := range []agreementFailure{
					{Property: f.Property, Identity: f.Identity, Tuple: f.Tuple, Direction: f.Direction, Multiplicity: f.Multiplicity + 1},
					{Property: f.Property, Identity: f.Identity, Tuple: agreementCitation{Target: "B", State: f.Tuple.State}, Direction: f.Direction, Multiplicity: f.Multiplicity},
					{Property: f.Property, Identity: f.Identity, Tuple: f.Tuple, Direction: "page-only", Multiplicity: f.Multiplicity},
				} {
					if kind, _, _ := agreementUnusedFootnoteDifference(c, &changed, targets); kind != "" {
						t.Fatalf("caught: unrelated unused-definition delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
