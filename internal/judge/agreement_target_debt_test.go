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

type agreementStandaloneTargets struct {
	Page, Judge map[string]int
}

// A standalone prose wikilink has no enclosing literal or destination field.
// The frozen check reading removes a target's unaliased trailing backslashes.
func agreementStandaloneTargetDebt(body string) agreementStandaloneTargets {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementDeclarationMarkers(source, doc) {
		return agreementStandaloneTargets{}
	}
	budget := agreementStandaloneTargets{Page: make(map[string]int), Judge: make(map[string]int)}
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		paragraph, prose := node.(*ast.Paragraph)
		if !prose || paragraph.Lines().Len() != 1 {
			continue
		}
		plain := true
		for child := paragraph.FirstChild(); child != nil; child = child.NextSibling() {
			if _, textual := child.(*ast.Text); !textual {
				plain = false
			}
		}
		if !plain {
			continue
		}
		line := paragraph.Lines().At(0)
		field := strings.TrimSpace(string(line.Value(source)))
		inner, open := strings.CutPrefix(field, "[[")
		inner, closed := strings.CutSuffix(inner, "]]")
		if !open || !closed || strings.ContainsAny(inner, "]\n") {
			continue
		}
		link, cites := graph.ParseWikilink(inner)
		targets := judge.LinkTargets(field)
		if !cites || !strings.HasSuffix(link.Target, "\\") || len(targets) != 1 || targets[0] == link.Target {
			continue
		}
		budget.Page[link.Target]++
		budget.Judge[targets[0]]++
	}
	if len(budget.Page) == 0 {
		return agreementStandaloneTargets{}
	}
	return budget
}

func agreementStandaloneTargetDifference(c agreementCase, f *agreementFailure, budget agreementStandaloneTargets) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	if f.Direction == "page-only" && budget.Page[f.Tuple.Target] == f.Multiplicity || f.Direction == "judge-only" && budget.Judge[f.Tuple.Target] == f.Multiplicity {
		return "debt", "#1011 stage 3", "page"
	}
	return "", "", ""
}

func TestAgreementStandaloneTargetDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body  string
		page, check map[string]int
	}{
		{name: "standalone suffix", body: "[[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "whole suffix set", body: "[[A\\]]\n\n[[B\\]]\n", page: map[string]int{"A\\": 1, "B\\": 1}, check: map[string]int{"A": 1, "B": 1}},
		{name: "duplicate standalone suffixes", body: "[[A\\]]\n\n[[A\\]]\n", page: map[string]int{"A\\": 2}, check: map[string]int{"A": 2}},
		{name: "multiple trailing slashes", body: "[[A\\\\]]\n", page: map[string]int{"A\\\\": 1}, check: map[string]int{"A": 1}},
		{name: "backslash before heading already agrees", body: "[[A\\#A]]\n"},
		{name: "unrelated prose", body: "ordinary words\n\n[[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "independent live target", body: "[[A]]\n\n[[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "literal markers are independent", body: "`%%<!--[!note]`\n\n[[A\\]]\n", page: map[string]int{"A\\": 1}, check: map[string]int{"A": 1}},
		{name: "ordinary target agrees", body: "[[A]]\n"},
		{name: "alias already removes suffix", body: "[[A\\|alias]]\n"},
		{name: "escaped opener is shown", body: "\\[[A\\]]\n"},
		{name: "inline code is shown", body: "`[[A\\]]`\n"},
		{name: "quoted target needs container ownership", body: "> [[A\\]]\n"},
		{name: "destination is metadata", body: "[n]: [[A\\]]\n"},
		{name: "mixed fields need occurrence ownership", body: "[[A\\]] [[A]]\n"},
		{name: "compound suffix needs occurrence ownership", body: "[[A]] [[B\\]]\n"},
		{name: "multiline paragraph needs occurrence ownership", body: "before\n[[A\\]]\n"},
		{name: "live comments need ownership", body: "%%\n[[A\\]]\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementStandaloneTargetDebt(tc.body)
			if diff := cmp.Diff(agreementStandaloneTargets{Page: tc.page, Judge: tc.check}, budget); diff != "" {
				t.Fatalf("caught: standalone target inventory (-want +got):\n%s", diff)
			}
			for direction, targets := range map[string]map[string]int{"page-only": tc.page, "judge-only": tc.check} {
				failures := []agreementFailure{{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: "unrelated"}, Direction: direction, Multiplicity: 1}}
				for target, count := range targets {
					failures = append(failures, agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: target}, Direction: direction, Multiplicity: count})
				}
				for _, f := range failures {
					kind, authority, wrong := agreementStandaloneTargetDifference(agreementCase{Body: tc.body}, &f, budget)
					if targets[f.Tuple.Target] > 0 && (kind != "debt" || authority != "#1011 stage 3" || wrong != "page") || targets[f.Tuple.Target] == 0 && kind != "" {
						t.Fatalf("caught: standalone target ownership kind=%q authority=%q wrong=%q", kind, authority, wrong)
					}
					for _, change := range []func(*agreementFailure){
						func(f *agreementFailure) { f.Multiplicity++ },
						func(f *agreementFailure) { f.Tuple.Section = "unrelated" },
						func(f *agreementFailure) { f.Tuple.SourceRole = "unrelated" },
						func(f *agreementFailure) { f.Property = "P0" },
					} {
						changed := f
						change(&changed)
						if kind, _, _ := agreementStandaloneTargetDifference(agreementCase{Body: tc.body}, &changed, budget); kind != "" {
							t.Fatalf("caught: unrelated standalone target delta admitted: %+v", changed)
						}
					}
				}
			}
		})
	}
}
