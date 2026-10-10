package judge_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

type agreementUnusedCheckDefinition struct {
	Span    graph.Span
	Targets []string
}

type agreementUnusedCitationSource struct {
	Fields      []agreementUnusedWidgetField
	Definitions []agreementUnusedCheckDefinition
}

// The check reads the original definition, including its label and indentation.
// Reading each field alone would invent citations from later indented paragraphs.
func agreementSharedUnusedCitationReading(body string, grammar goldmark.Markdown) agreementUnusedCitationSource {
	fields := agreementSharedUnusedWidgetReading(body, grammar)
	if len(fields) == 0 {
		return agreementUnusedCitationSource{}
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var definitions []agreementUnusedCheckDefinition
	for _, definition := range unused {
		start, stop := -1, -1
		for node := definition.FirstChild(); node != nil; node = node.NextSibling() {
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				physical := bytes.LastIndexByte(source[:line.Start], '\n') + 1
				if start < 0 || physical < start {
					start = physical
				}
				stop = max(stop, line.Stop)
			}
		}
		if start < 0 {
			return agreementUnusedCitationSource{}
		}
		definitions = append(definitions, agreementUnusedCheckDefinition{
			Span:    graph.Span{Start: start, Stop: stop},
			Targets: judge.LinkTargets(string(source[start:stop])),
		})
	}
	return agreementUnusedCitationSource{Fields: fields, Definitions: definitions}
}

type agreementUnusedCitationPayload struct {
	Body    string
	Targets map[string]int
}

func agreementSharedUnusedCitationBudget(body string) agreementUnusedCitationPayload {
	plain := agreementSharedUnusedCitationReading(body, agreementFootnoteGrammar)
	gfm := agreementSharedUnusedCitationReading(body, agreementExclusiveCodeGrammar)
	if !cmp.Equal(plain, gfm) {
		return agreementUnusedCitationPayload{}
	}
	var targets map[string]int
	for _, definition := range plain.Definitions {
		for _, target := range definition.Targets {
			if targets == nil {
				targets = make(map[string]int)
			}
			targets[target]++
		}
	}
	if len(targets) == 0 {
		return agreementUnusedCitationPayload{}
	}
	return agreementUnusedCitationPayload{Body: body, Targets: targets}
}

func agreementSharedUnusedCitationDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementUnusedCitationPayload) (kind, authority, wrong string) {
	if c.Body != budget.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "judge-only" || f.Tuple.Target == "" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 || budget.Targets[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	checkCount, pageCount := 0, 0
	for _, target := range judge.LinkTargets(c.Body) {
		if target == f.Tuple.Target {
			checkCount++
		}
	}
	for _, tuple := range actual.Citations {
		if tuple.SourceRole == "" && tuple.Target == f.Tuple.Target {
			pageCount++
		}
	}
	if checkCount-budget.Targets[f.Tuple.Target] != pageCount {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "judge"
}

func TestAgreementSharedUnusedCitationSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	rawB := agreementCitation{Target: "B\\", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedCitationSource
	}{
		{name: "original label and whole first line", body: "[[A]]\n\n[^n]: [[A]] [[A#part|shown]]\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 13, Stop: 18}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 19, Stop: 35}, Word: "[[A#part|shown]]", Tuple: section}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 7, Stop: 35}, Targets: []string{"A", "A"}}}}},
		{name: "all definitions retain later paragraph indentation", body: "[^n]: [[A]]\n\n    [[B]]\n\n[^m]: [[B\\]]\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Tuple: b}, {Span: graph.Span{Start: 30, Stop: 36}, Word: "[[B\\]]", Tuple: rawB}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 22}, Targets: []string{"A"}}, {Span: graph.Span{Start: 24, Stop: 36}, Targets: []string{"B"}}}}},
		{name: "all physical rows retain check order", body: "[^n]: [[A]]\n[[B]]\n", want: agreementUnusedCitationSource{Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 12, Stop: 17}, Word: "[[B]]", Tuple: b}}, Definitions: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 17}, Targets: []string{"A", "B"}}}}},
		{name: "used declaration has no unused contribution", body: "ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "ordinary source has no definition", body: "[[A]]\n"},
		{name: "unsupported field refuses whole reading", body: "[^n]: [[A]] words\n"},
		{name: "unsupported later block refuses whole reading", body: "[^n]: [[A]]\n\n    ## [[B]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				got := agreementSharedUnusedCitationReading(tc.body, grammar)
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Fatalf("caught: complete shared unused citation source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementSharedUnusedCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		budget     map[string]int
		want       map[string]int
	}{
		{name: "all targets and occurrences beside shared live target", body: "[[A]]\n\n[^n]: [[A]] [[A]] [[B#part]]\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "every visible target occurrence", body: "[[A]] [[A]] [[B]]\n\n[^n]: [[A]] [[B]]\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "all unused definitions", body: "[[A]]\n\n[^n]: [[A]]\n\n[^m]: [[A]] [[B]]\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "later indented paragraph stays outside check contribution", body: "[[A]]\n\n[^n]: [[A]]\n\n    [[B]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "all diagnostic sections share check target", body: "[[A#part]]\n\n[^n]: [[A#part]] [[A]]\n", budget: map[string]int{"A": 2}, want: map[string]int{"A": 2}},
		{name: "embed retains original check source", body: "[[A]]\n\n[^n]: ![[A#part|shown]] [[B]]\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "raw suffix retains check normalization", body: "[[A]]\n\n[^n]: [[A\\]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "used declaration cannot lend its contribution", body: "ref[^u]\n\n[^u]: [[A]]\n\n[^n]: [[A]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "whole count refuses another absent owner", body: "[r]: [[A]]\n\n[^n]: [[A]]\n", budget: map[string]int{"A": 1}},
		{name: "callout title needs separate check owner", body: "[^unused]: [[A]]\n> [!note] [[A]]\n章節\n", budget: map[string]int{"A": 1}},
		{name: "page grammar alone uses the reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]]\n"},
		{name: "CommonMark alone uses the reference", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "comment continuation refuses whole source", body: "%%\n[^n]: [[A]]\n%%\n"},
		{name: "recorded full original container source", body: "A\n===\n- ```\n%%[[A]]%%\t```\n[^n]: [[A]]\n\n    [[B]]\n  ```\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "ordinary prose stays outside", body: "[[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementSharedUnusedCitationBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Targets); diff != "" {
				t.Fatalf("caught: complete shared unused citation budget (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementSharedUnusedCitationDifference(c, &f, &actual, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: shared unused citation borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge" {
					t.Fatalf("caught: shared unused citation ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementSharedUnusedCitationDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete shared unused citation public receipts (-want +got):\n%s", diff)
			}
		})
	}
}

func agreementSharedUnusedCitationDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementUnusedCitationPayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementSharedUnusedCitationDifference(other, f, actual, budget); kind != "" {
			t.Fatal("caught: shared unused citation borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementSharedUnusedCitationDifference(c, &changed, actual, budget); kind != "" {
			t.Fatalf("caught: shared unused citation borrowed signature %s", agreementSignature(&changed))
		}
	}
	for _, extra := range []agreementCitation{{Target: f.Tuple.Target, State: "wikilink-broken"}, {Target: f.Tuple.Target, Section: "other", State: "wikilink"}} {
		changed := *actual
		changed.Citations = append(append([]agreementCitation{}, actual.Citations...), extra)
		if kind, _, _ := agreementSharedUnusedCitationDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: shared unused citation borrowed current page count")
		}
	}
	var surviving []agreementCitation
	removed := false
	for _, tuple := range actual.Citations {
		if tuple.SourceRole == "" && tuple.Target == f.Tuple.Target && !removed {
			removed = true
			continue
		}
		surviving = append(surviving, tuple)
	}
	changed := *actual
	changed.Citations = surviving
	kind, _, _ := agreementSharedUnusedCitationDifference(c, f, &changed, budget)
	if removed && kind != "" || !removed && kind != "debt" {
		t.Fatal("caught: shared unused citation lost selected carrier count scope")
	}
	changed = *actual
	changed.Citations = append(append([]agreementCitation{}, actual.Citations...), agreementCitation{SourceRole: agreementOutsideMarkdown, Target: f.Tuple.Target, State: "wikilink-broken"}, agreementCitation{Target: "other", State: "wikilink-broken"})
	if kind, _, _ := agreementSharedUnusedCitationDifference(c, f, &changed, budget); kind != "debt" {
		t.Fatal("caught: shared unused citation counted another carrier target or role")
	}
}
