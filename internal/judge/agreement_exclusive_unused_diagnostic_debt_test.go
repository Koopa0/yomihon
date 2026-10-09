package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// Every spelling of a selected target must belong to unused paragraphs in
// both grammars. Live or differently owned fields cannot lend a ghost count.
func agreementUnusedParagraphZones(body string, grammar goldmark.Markdown) []graph.Span {
	ctx := parser.NewContext()
	ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	ctx.Set(agreementUnusedFootnoteNodesKey, &unused)
	src := []byte(body)
	doc := grammar.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))
	if !agreementPlainOpenerDeclarationMarkers(src, doc) {
		return nil
	}
	var spans []graph.Span
	for _, def := range unused {
		for node := def.FirstChild(); node != nil; node = node.NextSibling() {
			switch node.(type) {
			case *ast.Paragraph, *ast.TextBlock:
			default:
				return nil
			}
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				spans = append(spans, graph.Span{Start: line.Start, Stop: line.Stop})
			}
		}
	}
	return spans
}

func agreementExclusiveUnusedDiagnostics(body string) map[agreementCitation]int {
	if !strings.Contains(body, "[^") {
		return nil
	}
	plain := agreementUnusedParagraphZones(body, agreementFootnoteGrammar)
	gfm := agreementUnusedParagraphZones(body, agreementExclusiveCodeGrammar)
	return agreementExclusiveFieldBudget(body, plain, gfm)
}
func agreementExclusiveUnusedDiagnosticDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget map[agreementCitation]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Tuple.Target == "" || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 || budget[f.Tuple] != f.Multiplicity {
		return "", "", ""
	}
	for _, tuple := range actual.Citations {
		if tuple.Target == f.Tuple.Target {
			return "", "", ""
		}
	}
	return "debt", "#1011 stage 5", "page-diagnostic"
}
func TestAgreementExclusiveUnusedDiagnostics(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	raw := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
		public     bool
	}{
		{name: "complete formatted inventory", body: "[^n]: *words* [[A]] [[A#A]] [[A]] [[B|alias]]\n", want: map[agreementCitation]int{a: 2, section: 1, b: 1}, public: true},
		{name: "recorded complete raw inventory", body: "[^unused]: [[A]]\n[[A]] [[A]][[A\\]]````\n> \n\n\ttext ", want: map[agreementCitation]int{a: 3, raw: 1}, public: true},
		{name: "code words beside fields", body: "[^n]: `words` [[A]] [[B]]\n", want: map[agreementCitation]int{a: 1, b: 1}, public: true},
		{name: "HTML words beside fields", body: "[^n]: <mark>words</mark> [[A]]\n", want: map[agreementCitation]int{a: 1}, public: true},
		{name: "all paragraphs and declarations", body: "[^n]: words [[A]]\nnext [[A]]\n\n    *words* [[B]]\n\n[^m]: words [[A#A]]\n", want: map[agreementCitation]int{a: 2, b: 1, section: 1}, public: true},
		{name: "raw target", body: "[^n]: words [[A\\]]\n", want: map[agreementCitation]int{raw: 1}, public: true},
		{name: "escaped alias delimiter", body: "[^n]: words [[A\\|alias]]\n", want: map[agreementCitation]int{a: 1}, public: true},
		{name: "remaining owned target", body: "[[A]]\n\n[^n]: words [[A]] [[B]]\n", want: map[agreementCitation]int{b: 1}, public: true},
		{name: "used and unused set", body: "ref[^n]\n\n[^n]: *used* [[A]]\n\n[^m]: *unused* [[B]]\n", want: map[agreementCitation]int{b: 1}, public: true},
		{name: "literal code field", body: "[^n]: `[[A]]`\n", want: map[agreementCitation]int{a: 1}},
		{name: "ordinary live field", body: "words [[A]]\n"},
		{name: "used definition", body: "ref[^n]\n\n[^n]: words [[A]]\n"},
		{name: "live field after definition", body: "[^n]: words [[A]]\n\nordinary [[A]]\n"},
		{name: "other live section", body: "[[A#B]]\n\n[^n]: words [[A#A]]\n"},
		{name: "shown field outside", body: "\\[[A]]\n\n[^n]: words [[A]]\n"},
		{name: "embed field outside", body: "![[A]]\n\n[^n]: words [[A]]\n"},
		{name: "reference field outside", body: "[r]: [[A]]\n\n[^n]: words [[A]]\n"},
		{name: "code owns definition", body: "```\n[^n]: words [[A]]\n```\n"},
		{name: "later heading refuses inventory", body: "[^n]: words [[A]]\n\n    ## [[B]]\n"},
		{name: "later code block refuses inventory", body: "[^n]: words [[A]]\n\n    ```\n    [[B]]\n    ```\n"},
		{name: "grammars disagree on use", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words [[A]]\n"},
		{name: "live percent context", body: "%%\n[^n]: words [[A]]\n%%\n"},
		{name: "nested field", body: "[^n]: words [[A[[B]]]]\n"},
		{name: "incomplete field", body: "[^n]: words [[A]] [[B\n"},
		{name: "block field", body: "[^n]: words [[A#^a]] [[B]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementExclusiveUnusedDiagnostics(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: exclusive unused diagnostic inventory (-want +got):\n%s", diff)
			}
			if !tc.public {
				return
			}
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &actual)
			matched := 0
			for i := range failures {
				f := &failures[i]
				if f.Property != "P0" {
					continue
				}
				kind, authority, wrong := agreementExclusiveUnusedDiagnosticDifference(c, f, &actual, budget)
				if budget[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: exclusive unused diagnostic borrowed tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: exclusive unused diagnostic public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				matched++
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementExclusiveUnusedDiagnosticDifference(other, f, &actual, budget); kind != "" {
						t.Fatal("caught: exclusive unused diagnostic borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementExclusiveUnusedDiagnosticDifference(c, &changed, &actual, budget); kind != "" {
						t.Fatalf("caught: exclusive unused diagnostic borrowed signature %s", agreementSignature(&changed))
					}
				}
				changed := actual
				changed.Citations = append(append([]agreementCitation(nil), actual.Citations...), f.Tuple)
				if kind, _, _ := agreementExclusiveUnusedDiagnosticDifference(c, f, &changed, budget); kind != "" {
					t.Fatal("caught: exclusive unused diagnostic borrowed page carrier")
				}
			}
			if matched != len(budget) {
				t.Fatalf("caught: exclusive unused diagnostic public set got=%d want=%d", matched, len(budget))
			}
		})
	}
}
