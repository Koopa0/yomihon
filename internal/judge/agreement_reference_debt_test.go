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

type agreementReferenceDestinations struct {
	Diagnostics map[agreementCitation]int
	Targets     map[string]int
}

// A reference declaration stores a destination; it does not display a
// wikilink there. Keep every declaration, including duplicate labels.
func agreementReferenceDestinationBudget(body string) agreementReferenceDestinations {
	return agreementReferenceFieldBudget(body, false)
}

func agreementCompoundReferenceBudget(body string) agreementReferenceDestinations {
	return agreementReferenceFieldBudget(body, true)
}

func agreementReferenceFieldBudget(body string, compound bool) agreementReferenceDestinations {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return agreementReferenceDestinations{}
	}
	budget := agreementReferenceDestinations{Diagnostics: make(map[agreementCitation]int), Targets: make(map[string]int)}
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		ref, declaration := node.(*ast.LinkReferenceDefinition)
		if !declaration {
			continue
		}
		destination := string(ref.Destination)
		if strings.Contains(string(ref.Label), "[[") || strings.Contains(string(ref.Title), "[[") {
			continue
		}
		var links []graph.Wikilink
		if compound {
			links = agreementCompoundReferenceLinks(destination)
		} else {
			inner, open := strings.CutPrefix(destination, "[[")
			inner, closed := strings.CutSuffix(inner, "]]")
			if !open || !closed || strings.ContainsAny(inner, "]\n") {
				continue
			}
			link, cites := graph.ParseWikilink(inner)
			if !cites {
				continue
			}
			links = []graph.Wikilink{link}
		}
		if len(links) == 0 {
			continue
		}
		for _, link := range links {
			budget.Diagnostics[agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}]++
		}
		for _, target := range judge.LinkTargets(destination) {
			budget.Targets[target]++
		}
	}
	if len(budget.Diagnostics) == 0 && len(budget.Targets) == 0 {
		return agreementReferenceDestinations{}
	}
	return budget
}

// The complete destination must consist of adjacent literal widgets. No
// surrounding URL bytes or nested brackets enter this set.
func agreementCompoundReferenceLinks(destination string) []graph.Wikilink {
	var links []graph.Wikilink
	for remaining := destination; remaining != ""; {
		inner, open := strings.CutPrefix(remaining, "[[")
		inner, rest, closed := strings.Cut(inner, "]]")
		if !open || !closed || strings.ContainsAny(inner, "[]\n`") {
			return nil
		}
		link, cites := graph.ParseWikilink(inner)
		if !cites {
			return nil
		}
		links = append(links, link)
		remaining = rest
	}
	if len(links) < 2 {
		return nil
	}
	return links
}

func agreementReferenceDestinationDifference(c agreementCase, f *agreementFailure, budget agreementReferenceDestinations) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	if f.Property == "P0" && f.Identity == "diagnostic-html" && f.Direction == "diagnostic-only" && budget.Diagnostics[f.Tuple] == f.Multiplicity {
		return "debt", "#1011 unrendered reference declarations", "page-diagnostic"
	}
	if f.Property == "P1" && f.Identity == "citation-occurrences" && f.Direction == "judge-only" && f.Tuple.SourceRole == "" && f.Tuple.Section == "" && f.Tuple.State == "" && budget.Targets[f.Tuple.Target] == f.Multiplicity {
		return "debt", "#1011 unrendered reference declarations", "judge"
	}
	return "", "", ""
}

func TestAgreementReferenceDestinationDebt(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	escaped := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body  string
		diagnostics map[agreementCitation]int
		targets     map[string]int
	}{
		{name: "declared destination", body: "[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "duplicate labels retain every declaration", body: "[n]: [[A]]\n[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 2}, targets: map[string]int{"A": 2}},
		{name: "whole destination set", body: "[n]: [[A]]\n[m]: [[B]]\n", diagnostics: map[agreementCitation]int{a: 1, b: 1}, targets: map[string]int{"A": 1, "B": 1}},
		{name: "heading destination field", body: "[n]: [[A#A]]\n", diagnostics: map[agreementCitation]int{section: 1}, targets: map[string]int{"A": 1}},
		{name: "alias keeps target field", body: "[n]: [[A|alias]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "backslash distinguishes producer fields", body: "[n]: [[A\\]]\n", diagnostics: map[agreementCitation]int{escaped: 1}, targets: map[string]int{"A": 1}},
		{name: "angle destination", body: "[n]: <[[A]]>\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "reference use does not display destination", body: "[read][n]\n\n[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "independent live target", body: "[[A]]\n\n[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "independent literal markers", body: "`%%<!--[!note]`\n\n[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "independent plain opener run", body: "> [!note] one\n> [!note] two\n> [!note] three\n\n[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "independent plain opener", body: "> [!note] title\n\n[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "independent prose words", body: "show [!note] words\n\n[n]: [[A]]\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "ordinary wikilink", body: "[[A]]\n"},
		{name: "ordinary destination", body: "[n]: Other.md\n"},
		{name: "escaped declaration", body: "\\[n]: [[A]]\n"},
		{name: "escaped destination", body: "[n]: \\[[A]]\n"},
		{name: "code owns declaration", body: "```\n[n]: [[A]]\n```\n"},
		{name: "compound destination needs field ownership", body: "[n]: [[A]][[B]]\n"},
		{name: "ordinary title keeps destination", body: "[n]: [[A]] \"ordinary\"\n", diagnostics: map[agreementCitation]int{a: 1}, targets: map[string]int{"A": 1}},
		{name: "destination beside title target needs ownership", body: "[n]: [[A]] \"[[B]]\"\n"},
		{name: "title target needs field ownership", body: "[n]: Other.md \"[[A]]\"\n"},
		{name: "quoted declaration needs container ownership", body: "> [n]: [[A]]\n"},
		{name: "live comment needs declaration ownership", body: "%%\n[n]: [[A]]\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementReferenceDestinationBudget(tc.body)
			want := agreementReferenceDestinations{Diagnostics: tc.diagnostics, Targets: tc.targets}
			if diff := cmp.Diff(want, budget); diff != "" {
				t.Fatalf("caught: reference destination inventory (-want +got):\n%s", diff)
			}
			failures := []agreementFailure{
				{Property: "P0", Identity: "diagnostic-html", Tuple: agreementCitation{Target: "unrelated", State: "wikilink-broken"}, Direction: "diagnostic-only", Multiplicity: 1},
				{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: "unrelated"}, Direction: "judge-only", Multiplicity: 1},
			}
			for citation, count := range tc.diagnostics {
				failures = append(failures, agreementFailure{Property: "P0", Identity: "diagnostic-html", Tuple: citation, Direction: "diagnostic-only", Multiplicity: count})
			}
			for target, count := range tc.targets {
				failures = append(failures, agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: target}, Direction: "judge-only", Multiplicity: count})
			}
			for _, f := range failures {
				kind, authority, wrong := agreementReferenceDestinationDifference(agreementCase{Body: tc.body}, &f, budget)
				owned, wantWrong := tc.diagnostics[f.Tuple] > 0, "page-diagnostic"
				if f.Property == "P1" {
					owned, wantWrong = tc.targets[f.Tuple.Target] > 0, "judge"
				}
				if owned && (kind != "debt" || authority != "#1011 unrendered reference declarations" || wrong != wantWrong) || !owned && kind != "" {
					t.Fatalf("caught: reference destination ownership kind=%q authority=%q wrong=%q", kind, authority, wrong)
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.PagePresent = true },
					func(f *agreementFailure) { f.Tuple.SourceRole = "unrelated" },
					func(f *agreementFailure) { f.Direction = "page-only" },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementReferenceDestinationDifference(agreementCase{Body: tc.body}, &changed, budget); kind != "" {
						t.Fatalf("caught: unrelated reference delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
