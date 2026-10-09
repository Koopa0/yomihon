package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

type agreementTextSuffixFields struct {
	Selected, All agreementStandaloneTargets
}

// A widget can cross adjacent text nodes, but it cannot cross a code, markup
// or metadata child. The original contiguous bytes own the entire field.
func agreementProseTextRuns(body string, grammar goldmark.Markdown) []graph.Span {
	source := []byte(body)
	ctx := parser.NewContext()
	ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
	var zones []graph.Span
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node.(type) {
		case *ast.Paragraph, *ast.TextBlock:
		default:
			return ast.WalkContinue, nil
		}
		var run graph.Span
		active := false
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if field, ok := child.(*ast.Text); ok {
				if active && field.Segment.Start == run.Stop {
					run.Stop = field.Segment.Stop
					continue
				}
				if active {
					zones = append(zones, run)
				}
				run = graph.Span{Start: field.Segment.Start, Stop: field.Segment.Stop}
				active = true
				continue
			}
			if active {
				zones = append(zones, run)
				active = false
			}
		}
		if active {
			zones = append(zones, run)
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return zones
}

func agreementTextSuffixFieldBudget(body string) agreementTextSuffixFields {
	plain := agreementProseTextRuns(body, agreementFootnoteGrammar)
	gfm := agreementProseTextRuns(body, agreementExclusiveCodeGrammar)
	allRaw, allCheck := agreementDiscardedFieldBudgets(body, plain, gfm)
	selected := func(field string, check bool) string {
		inner, _ := strings.CutPrefix(field, "[[")
		inner, _ = strings.CutSuffix(inner, "]]")
		link, cites := graph.ParseWikilink(inner)
		targets := judge.LinkTargets(field)
		if !cites || !strings.HasSuffix(link.Target, "\\") || len(targets) != 1 || targets[0] == link.Target {
			return ""
		}
		if check {
			return targets[0]
		}
		return link.Target
	}
	raw := agreementExclusiveTargetFieldBudget(body, plain, gfm, func(field string) string { return selected(field, false) })
	check := agreementExclusiveTargetFieldBudget(body, plain, gfm, func(field string) string { return selected(field, true) })
	counts := func(tuples map[agreementCitation]int) map[string]int {
		var result map[string]int
		for tuple, count := range tuples {
			if result == nil {
				result = make(map[string]int)
			}
			result[tuple.Target] += count
		}
		return result
	}
	if len(raw) == 0 || len(check) == 0 {
		return agreementTextSuffixFields{}
	}
	return agreementTextSuffixFields{
		Selected: agreementStandaloneTargets{Page: counts(raw), Judge: counts(check)},
		All:      agreementStandaloneTargets{Page: counts(allRaw), Judge: allCheck},
	}
}

// Source counts must account for all actual carriers and check occurrences of
// every selected spelling, including ordinary names beside suffix widgets.
func agreementTextSuffixFieldDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementTextSuffixFields) (kind, authority, wrong string) {
	kind, authority, wrong = agreementStandaloneTargetDifference(c, f, budget.Selected)
	if kind == "" {
		return "", "", ""
	}
	page, check := make(map[string]int), make(map[string]int)
	for _, tuple := range actual.Citations {
		page[tuple.Target]++
	}
	for _, target := range judge.LinkTargets(c.Body) {
		check[target]++
	}
	for target, count := range budget.Selected.Page {
		if count != budget.All.Page[target] || page[target] != budget.All.Page[target] {
			return "", "", ""
		}
	}
	for target := range budget.Selected.Judge {
		if check[target] != budget.All.Judge[target] || page[target] != budget.All.Page[target] {
			return "", "", ""
		}
	}
	return kind, authority, wrong
}

func TestAgreementTextSuffixFields(t *testing.T) {
	t.Parallel()
	one := agreementStandaloneTargets{Page: map[string]int{"A\\": 1}, Judge: map[string]int{"A": 1}}
	two := agreementStandaloneTargets{Page: map[string]int{"A\\": 1, "B\\": 1}, Judge: map[string]int{"A": 1, "B": 1}}
	repeated := agreementStandaloneTargets{Page: map[string]int{"A\\": 2}, Judge: map[string]int{"A": 2}}
	onlyB := agreementStandaloneTargets{Page: map[string]int{"B\\": 1}, Judge: map[string]int{"B": 1}}
	// The native source runs are complete declarations. Adjacent siblings can
	// join, while a line gap or another inline owner keeps the runs separate.
	for _, tc := range []struct {
		body string
		want []graph.Span
	}{
		{body: "[[A\\]] *words* [[B\\]]\n", want: []graph.Span{{Start: 0, Stop: 7}, {Start: 14, Stop: 21}}},
		{body: "[[A\\]]\n[[B\\]]\n", want: []graph.Span{{Start: 0, Stop: 6}, {Start: 7, Stop: 13}}},
		{body: "`code` words [[A\\]]\n", want: []graph.Span{{Start: 6, Stop: 19}}},
		{body: "[[A`x`\\]]\n", want: []graph.Span{{Start: 0, Stop: 3}, {Start: 6, Stop: 9}}},
	} {
		for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
			if diff := cmp.Diff(tc.want, agreementProseTextRuns(tc.body, grammar)); diff != "" {
				t.Fatalf("caught: complete declared text source runs body=%q (-want +got):\n%s", tc.body, diff)
			}
		}
	}
	for _, tc := range []struct {
		name, body string
		want       agreementTextSuffixFields
		public     agreementStandaloneTargets
		oldRefuses bool
	}{
		{name: "all text runs beside emphasis", body: "[[A\\]] *words* [[B\\]]\n", want: agreementTextSuffixFields{Selected: two, All: two}, public: two, oldRefuses: true},
		{name: "code words cannot own the field", body: "`code` words [[A\\]]\n", want: agreementTextSuffixFields{Selected: one, All: one}, public: one, oldRefuses: true},
		{name: "ordinary and suffix counts stay distinct", body: "[[A]] [[A\\]] [[B\\]]\n", want: agreementTextSuffixFields{Selected: two, All: agreementStandaloneTargets{Page: map[string]int{"A": 1, "A\\": 1, "B\\": 1}, Judge: map[string]int{"A": 2, "B": 1}}}, public: two},
		{name: "all ordinary sections retain their counts", body: "[[A#one]] [[A#two]] [[A\\]]\n", want: agreementTextSuffixFields{Selected: one, All: agreementStandaloneTargets{Page: map[string]int{"A": 2, "A\\": 1}, Judge: map[string]int{"A": 3}}}, public: one},
		{name: "repeated complete fields", body: "[[A\\]] [[A\\]]\n", want: agreementTextSuffixFields{Selected: repeated, All: repeated}, public: repeated},
		{name: "all physical source lines", body: "[[A\\]]\n[[B\\]]\n", want: agreementTextSuffixFields{Selected: two, All: two}, public: two},
		{name: "independent opener target", body: "> [!note] [[B]]\n\nordinary [[A\\]]\n", want: agreementTextSuffixFields{Selected: one, All: agreementStandaloneTargets{Page: map[string]int{"A\\": 1, "B": 1}, Judge: map[string]int{"A": 1, "B": 1}}}, public: one, oldRefuses: true},
		{name: "independent percent target", body: "%%[[A]]%%\n\n[[B\\]]\n", want: agreementTextSuffixFields{Selected: onlyB, All: agreementStandaloneTargets{Page: map[string]int{"A": 1, "B\\": 1}, Judge: map[string]int{"A": 1, "B": 1}}}, public: onlyB, oldRefuses: true},
		{name: "independent linkify words", body: "https://example.invalid/words\n\n[[A\\]]\n", want: agreementTextSuffixFields{Selected: one, All: one}, public: one, oldRefuses: true},
		{name: "used footnote retains native text", body: "ref[^n]\n\n[^n]: [[A\\]]\n", want: agreementTextSuffixFields{Selected: one, All: one}, public: one, oldRefuses: true},
		{name: "common grammar alone retains text", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A\\]]\n"},
		{name: "page grammar alone retains text", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A\\]]\n"},
		{name: "escaped source cannot borrow page receipts", body: "\\[[A\\]] ![[B\\]] [[#C\\]]\n", want: agreementTextSuffixFields{Selected: two, All: two}},
		{name: "comment owns apparent text", body: "<!-- comment -->[[A\\]]\n"},
		{name: "raw HTML owns apparent text", body: "<div>\n[[A\\]]\n</div>\n"},
		{name: "table owns fields in page grammar", body: "| a | b |\n|---|---|\n| [[A\\]] | words |\n"},
		{name: "link words beside the field", body: "[r](target) [[A\\]]\n", want: agreementTextSuffixFields{Selected: one, All: one}, public: one, oldRefuses: true},
		{name: "link label owns the field", body: "[r [[A\\]]](target)\n"},
		{name: "complete field belongs to link label", body: "[label [[A\\\\]] rest](target)\n"},
		{name: "code divides the field", body: "[[A`x`\\]]\n"},
		{name: "emphasis divides the field", body: "[[*A*\\]]\n"},
		{name: "code span owns the whole field", body: "`[[A\\]]`\n"},
		{name: "fence owns the whole field", body: "```\n[[A\\]]\n```\n"},
		{name: "outside normalized spelling refuses the receipt", body: "[[A\\]]\n\n`[[A]]`\n", want: agreementTextSuffixFields{Selected: one, All: agreementStandaloneTargets{Page: map[string]int{"A\\": 1}}}},
		{name: "incomplete field refuses inventory", body: "[[A\\]] [[B\n"},
		{name: "block field refuses inventory", body: "[[A\\#^a]] [[B\\]]\n"},
		{name: "heading owns its fields", body: "# [[A\\]]\n"},
		{name: "ordinary field has no suffix difference", body: "words [[A]]\n"},
		{name: "aliased field already agrees", body: "[[A\\|alias]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementTextSuffixFieldBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete text suffix field inventory (-want +got):\n%s", diff)
			}
			if tc.oldRefuses && len(agreementTextTargetDebt(tc.body).Page) != 0 {
				t.Fatal("caught: earlier text reader borrowed another inline owner")
			}
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			found := agreementStandaloneTargets{}
			for _, f := range agreementPageFailures(tc.body, &r, &actual) {
				if f.Property != "P1" {
					continue
				}
				expected := tc.public.Judge[f.Tuple.Target]
				if f.Direction == "page-only" {
					expected = tc.public.Page[f.Tuple.Target]
				}
				kind, authority, wrong := agreementTextSuffixFieldDifference(c, &f, &actual, budget)
				if expected == 0 {
					if kind != "" {
						t.Fatal("caught: text suffix field borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 3" || wrong != "page" {
					t.Fatalf("caught: text suffix field public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found.Page == nil {
					found = agreementStandaloneTargets{Page: make(map[string]int), Judge: make(map[string]int)}
				}
				if f.Direction == "page-only" {
					found.Page[f.Tuple.Target] = f.Multiplicity
				} else {
					found.Judge[f.Tuple.Target] = f.Multiplicity
				}
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementTextSuffixFieldDifference(other, &f, &actual, budget); kind != "" {
						t.Fatal("caught: text suffix field borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementTextSuffixFieldDifference(c, &changed, &actual, budget); kind != "" {
						t.Fatalf("caught: text suffix field borrowed signature %s", agreementSignature(&changed))
					}
				}
				// Keep cardinality while moving a selected carrier to another spelling.
				changed := actual
				changed.Citations = append([]agreementCitation(nil), actual.Citations...)
				moved := false
				for i := range changed.Citations {
					if budget.Selected.Page[changed.Citations[i].Target] > 0 {
						changed.Citations[i].Target += "-unowned"
						moved = true
						break
					}
				}
				if !moved {
					t.Fatal("caught: selected text suffix carrier absent")
				}
				if kind, _, _ := agreementTextSuffixFieldDifference(c, &f, &changed, budget); kind != "" {
					t.Fatal("caught: text suffix field borrowed same-cardinality page drift")
				}
				for target := range budget.Selected.Judge {
					changed := actual
					changed.Citations = append(append([]agreementCitation(nil), actual.Citations...), agreementCitation{Target: target})
					if kind, _, _ := agreementTextSuffixFieldDifference(c, &f, &changed, budget); kind != "" {
						t.Fatal("caught: text suffix field borrowed ordinary page count")
					}
				}
				if f.Direction == "judge-only" {
					changed := budget
					changed.Selected.Page = make(map[string]int)
					for target, n := range budget.Selected.Page {
						changed.Selected.Page[target] = n + 1
					}
					if kind, _, _ := agreementTextSuffixFieldDifference(c, &f, &actual, changed); kind != "" {
						t.Fatal("caught: text suffix field borrowed stale source count")
					}
				}
			}
			if diff := cmp.Diff(tc.public, found); diff != "" {
				t.Fatalf("caught: complete text suffix field public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
