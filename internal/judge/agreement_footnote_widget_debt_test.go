package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

var agreementUnusedFootnoteNodesKey = parser.NewContextKey()

// Retained unused declarations own their fields before the extension removes
// them. Diagnostic sections and check targets are different inventories.
func agreementUnusedFootnoteWidgetBudget(body string) agreementReferenceDestinations {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var declarations []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &declarations)
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return agreementReferenceDestinations{}
	}
	budget := agreementReferenceDestinations{Diagnostics: make(map[agreementCitation]int), Targets: make(map[string]int)}
	for _, definition := range declarations {
		start, stop := -1, -1
		for child := definition.FirstChild(); child != nil; child = child.NextSibling() {
			switch child.(type) {
			case *ast.Paragraph, *ast.TextBlock:
			default:
				return agreementReferenceDestinations{}
			}
			for inline := child.FirstChild(); inline != nil; inline = inline.NextSibling() {
				if _, plain := inline.(*ast.Text); !plain {
					return agreementReferenceDestinations{}
				}
			}
			for i := range child.Lines().Len() {
				line := child.Lines().At(i)
				if start < 0 {
					start = bytes.LastIndexByte(source[:line.Start], '\n') + 1
				}
				stop = max(stop, line.Stop)
				fields := strings.Fields(string(line.Value(source)))
				if len(fields) == 0 {
					return agreementReferenceDestinations{}
				}
				for _, field := range fields {
					inner, open := strings.CutPrefix(field, "[[")
					inner, closed := strings.CutSuffix(inner, "]]")
					if !open || !closed || strings.ContainsAny(inner, "[]\n`") {
						return agreementReferenceDestinations{}
					}
					link, cites := graph.ParseWikilink(inner)
					targets := judge.LinkTargets(field)
					if !cites || link.Block != "" || len(targets) != 1 {
						return agreementReferenceDestinations{}
					}
					budget.Diagnostics[agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}]++
				}
			}
		}
		if start >= 0 {
			// Authored indentation keeps later definition paragraphs out of the
			// current check inventory, while all their fields can raise diagnostics.
			for _, target := range judge.LinkTargets(string(source[start:stop])) {
				budget.Targets[target]++
			}
		}
	}
	if len(budget.Diagnostics) == 0 {
		return agreementReferenceDestinations{}
	}
	return budget
}

func agreementUnusedFootnoteWidgetDifference(c agreementCase, f *agreementFailure, budget agreementReferenceDestinations) (kind, authority, wrong string) {
	kind, _, wrong = agreementReferenceDestinationDifference(c, f, budget)
	if kind != "" {
		authority = "#1011 stage 5"
	}
	return kind, authority, wrong
}

func TestAgreementUnusedFootnoteWidgets(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", Section: "B", State: "wikilink-broken"}
	raw := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       agreementReferenceDestinations
		failures   int
	}{
		{name: "complete source lines", body: "[^unused]: [[A#A]]\n    [[B#B]]\n", want: agreementReferenceDestinations{Diagnostics: map[agreementCitation]int{section: 1, b: 1}, Targets: map[string]int{"A": 1, "B": 1}}, failures: 4},
		{name: "recorded lazy raw suffix", body: "[[A\nB]]- item\n\n      ## A\n[^unused]: [[A]]\n[[A\\]]", want: agreementReferenceDestinations{Diagnostics: map[agreementCitation]int{a: 1, raw: 1}, Targets: map[string]int{"A": 2}}, failures: 3},
		{name: "section field", body: "[^unused]: [[A#A]]\n", want: agreementReferenceDestinations{Diagnostics: map[agreementCitation]int{section: 1}, Targets: map[string]int{"A": 1}}, failures: 2},
		{name: "complete mixed sections", body: "[^one]: [[A]] [[A#A]]\n\n    [[B#B]] [[A#A]]\n\n[^two]: [[B#B]]\n", want: agreementReferenceDestinations{Diagnostics: map[agreementCitation]int{a: 1, section: 2, b: 2}, Targets: map[string]int{"A": 2, "B": 1}}, failures: 5},
		{name: "raw suffix target", body: "[^unused]: [[A\\]]\n", want: agreementReferenceDestinations{Diagnostics: map[agreementCitation]int{raw: 1}, Targets: map[string]int{"A": 1}}, failures: 2},
		{name: "alias keeps section", body: "[^unused]: [[A#A|alias]]\n", want: agreementReferenceDestinations{Diagnostics: map[agreementCitation]int{section: 1}, Targets: map[string]int{"A": 1}}, failures: 2},
		{name: "live target stays separate", body: "[[A#A]]\n\n[^unused]: [[A#A]]\n", want: agreementReferenceDestinations{Diagnostics: map[agreementCitation]int{section: 1}, Targets: map[string]int{"A": 1}}, failures: 2},
		{name: "used definition", body: "ref[^n]\n\n[^n]: [[A#A]]\n"},
		{name: "code owns definition", body: "```\n[^unused]: [[A#A]]\n```\n"},
		{name: "ordinary prose", body: "[[A#A]]\n"},
		{name: "emphasis owns field bytes", body: "[^unused]: [[A*B*C]]\n"},
		{name: "inline code owner", body: "[^unused]: `[[A#A]]`\n"},
		{name: "mixed fields", body: "[^unused]: [[A#A]] words\n"},
		{name: "mixed continuation", body: "[^unused]: [[A#A]]\n\n    words [[B]]\n"},
		{name: "heading needs ownership", body: "[^unused]: [[A]]\n\n    # [[B]]\n"},
		{name: "block needs ownership", body: "[^unused]: [[A#^a]]\n"},
		{name: "local target stays separate", body: "[^unused]: [[#A]]\n"},
		{name: "nested field", body: "[^unused]: [[A[[B]]]]\n"},
		{name: "linkify owns field", body: "[^unused]: https://example.invalid/[[A#A]]\n"},
		{name: "live comment needs ownership", body: "%%\n[^unused]: [[A#A]]\n%%\n"},
		{name: "callout needs ownership", body: "> [!note] title\n> [^unused]: [[A#A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementUnusedFootnoteWidgetBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: unused footnote field inventory (-want +got):\n%s", diff)
			}
			if tc.failures == 0 {
				return
			}
			c := agreementCase{Body: tc.body}
			r, a := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &a)
			if len(failures) != tc.failures {
				t.Fatalf("caught: unused field public deltas=%d want=%d %+v", len(failures), tc.failures, failures)
			}
			for i := range failures {
				f := &failures[i]
				kind, authority, wrong := agreementUnusedFootnoteWidgetDifference(c, f, budget)
				wantWrong := "judge"
				if f.Property == "P0" {
					wantWrong = "page-diagnostic"
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != wantWrong {
					t.Fatalf("caught: unused field public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementUnusedFootnoteWidgetDifference(other, f, budget); kind != "" {
						t.Fatal("caught: unused field borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "page-only" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementUnusedFootnoteWidgetDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: unused field borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
		})
	}
}
