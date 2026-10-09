package judge_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

type agreementCommentFootnoteCitationSource struct {
	Original agreementCommentFootnoteSource
	Check    agreementUnusedCitationSource
}

// The check reads physical source, including labels and later indentation.
// Every discarded definition must originate at an owned comment tail; a real
// unused definition has a different owner even when its target is the same.
func agreementCommentFootnoteCitationReading(body string, grammar goldmark.Markdown) agreementCommentFootnoteCitationSource {
	original := agreementCommentFootnoteReading(body, grammar)
	if len(original.Fields) == 0 {
		return agreementCommentFootnoteCitationSource{}
	}
	check := agreementUnusedPlainCitationReading(original.Projection.Body, grammar, false)
	if len(check.Fields) == 0 || len(check.Definitions) != len(original.Definitions) {
		return agreementCommentFootnoteCitationSource{}
	}
	for i, field := range check.Fields {
		if field.Span.Start < 0 || field.Span.Stop <= field.Span.Start || field.Span.Stop > len(original.Projection.Positions) {
			return agreementCommentFootnoteCitationSource{}
		}
		start := original.Projection.Positions[field.Span.Start]
		stop := original.Projection.Positions[field.Span.Stop-1] + 1
		if body[start:stop] != field.Word {
			return agreementCommentFootnoteCitationSource{}
		}
		check.Fields[i].Span = graph.Span{Start: start, Stop: stop}
	}
	if !cmp.Equal(original.Fields, check.Fields) {
		return agreementCommentFootnoteCitationSource{}
	}
	for i, definition := range check.Definitions {
		if definition.Span.Start < 0 || definition.Span.Stop <= definition.Span.Start || definition.Span.Stop > len(original.Projection.Positions) {
			return agreementCommentFootnoteCitationSource{}
		}
		start := original.Projection.Positions[definition.Span.Start]
		stop := original.Projection.Positions[definition.Span.Stop-1] + 1
		physical := bytes.LastIndexByte([]byte(body[:start]), '\n') + 1
		owned := false
		for _, native := range original.Definitions {
			if start >= native.Owner.Start && start < native.Owner.Stop && start <= native.Tail.Start && native.Tail.Start >= physical && native.Tail.Start < stop {
				owned = true
			}
		}
		if !owned {
			return agreementCommentFootnoteCitationSource{}
		}
		targets := judge.LinkTargets(body[physical:stop])
		if !cmp.Equal(targets, definition.Targets) {
			return agreementCommentFootnoteCitationSource{}
		}
		check.Definitions[i] = agreementUnusedCheckDefinition{Span: graph.Span{Start: physical, Stop: stop}, Targets: targets}
	}
	return agreementCommentFootnoteCitationSource{Original: original, Check: check}
}

func agreementCommentFootnoteCitationBudget(body string) agreementUnusedCitationPayload {
	plain := agreementCommentFootnoteCitationReading(body, agreementFootnoteGrammar)
	gfm := agreementCommentFootnoteCitationReading(body, agreementExclusiveCodeGrammar)
	if len(plain.Check.Fields) == 0 || !cmp.Equal(plain, gfm) {
		return agreementUnusedCitationPayload{}
	}
	var targets map[string]int
	for _, definition := range plain.Check.Definitions {
		for _, target := range definition.Targets {
			if targets == nil {
				targets = make(map[string]int)
			}
			targets[target]++
		}
	}
	return agreementUnusedCitationPayload{Body: body, Targets: targets}
}

func agreementCommentFootnoteCitationDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementUnusedCitationPayload) (kind, authority, wrong string) {
	kind, authority, wrong = agreementSharedUnusedCitationDifference(c, f, actual, budget)
	if kind != "" {
		wrong = "page"
	}
	return kind, authority, wrong
}

func TestAgreementCommentFootnoteCitationSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		original   agreementCommentFootnoteSource
		check      []agreementUnusedCheckDefinition
	}{
		{name: "all original physical rows and target order", body: "<!--h-->[^n]: [[A]] [[A#part|shown]]\nnext [[B]]\n\n    [[C\\]]\n", original: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 37}, Comment: graph.Span{Start: 0, Stop: 8}, Tail: graph.Span{Start: 8, Stop: 36}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "[^n]: [[A]] [[A#part|shown]]\nnext [[B]]\n\n    [[C\\]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 8, Stop: 60})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 14, Stop: 19}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 20, Stop: 36}, Word: "[[A#part|shown]]", Tuple: section}, {Span: graph.Span{Start: 42, Stop: 47}, Word: "[[B]]", Tuple: b}, {Span: graph.Span{Start: 53, Stop: 59}, Word: "[[C\\]]", Tuple: agreementCitation{Target: "C\\", State: "wikilink-broken"}}}}, check: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 59}, Targets: []string{"A", "A", "B"}}}},
		{name: "multiline closure retains its original physical prefix", body: "<!--\n[[A]]\n-->[^n]: [[A]]\n\n    [[B]]\n", original: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 26}, Comment: graph.Span{Start: 0, Stop: 14}, Tail: graph.Span{Start: 14, Stop: 25}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "\n\n[^n]: [[A]]\n\n    [[B]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 4, Stop: 5}, graph.Span{Start: 10, Stop: 11}, graph.Span{Start: 14, Stop: 37})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 20, Stop: 25}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 31, Stop: 36}, Word: "[[B]]", Tuple: b}}}, check: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 11, Stop: 36}, Targets: []string{"A"}}}},
		{name: "native leading indentation keeps original start", body: "  <!--h-->[^n]: [[A]]\n", original: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 22}, Comment: graph.Span{Start: 2, Stop: 10}, Tail: graph.Span{Start: 10, Stop: 21}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "  [^n]: [[A]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 0, Stop: 2}, graph.Span{Start: 10, Stop: 22})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Tuple: a}}}, check: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 21}, Targets: []string{"A"}}}},
		{name: "every original declaration retains ordered repeated targets", body: "<!--h-->[^a]: [[A]]\n<!--h-->[^b]: [[B]] [[A]]\n", original: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 20}, Comment: graph.Span{Start: 0, Stop: 8}, Tail: graph.Span{Start: 8, Stop: 19}, Label: "a"}, {Owner: graph.Span{Start: 20, Stop: 46}, Comment: graph.Span{Start: 20, Stop: 28}, Tail: graph.Span{Start: 28, Stop: 45}, Label: "b"}}, Projection: agreementCommentFootnoteProjection{Body: "[^a]: [[A]]\n[^b]: [[B]] [[A]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 8, Stop: 20}, graph.Span{Start: 28, Stop: 46})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 14, Stop: 19}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 34, Stop: 39}, Word: "[[B]]", Tuple: b}, {Span: graph.Span{Start: 40, Stop: 45}, Word: "[[A]]", Tuple: a}}}, check: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 19}, Targets: []string{"A"}}, {Span: graph.Span{Start: 20, Stop: 45}, Targets: []string{"B", "A"}}}},
		{name: "CRLF retains physical closure and carriage return", body: "<!--\r\nhidden\r\n-->[^n]: [[A]]\r\n", original: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 30}, Comment: graph.Span{Start: 0, Stop: 17}, Tail: graph.Span{Start: 17, Stop: 28}, Label: "n"}}, Projection: agreementCommentFootnoteProjection{Body: "\r\n\r\n[^n]: [[A]]\r\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 4, Stop: 6}, graph.Span{Start: 12, Stop: 14}, graph.Span{Start: 17, Stop: 30})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 23, Stop: 28}, Word: "[[A]]", Tuple: a}}}, check: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 14, Stop: 28}, Targets: []string{"A"}}}},
		{name: "Unicode label and raw embed bytes", body: "<!--h-->[^一]: ![[A#part|shown]] [[B\\]]\n", original: agreementCommentFootnoteSource{Definitions: []agreementCommentFootnoteDefinition{{Owner: graph.Span{Start: 0, Stop: 41}, Comment: graph.Span{Start: 0, Stop: 8}, Tail: graph.Span{Start: 8, Stop: 40}, Label: "一"}}, Projection: agreementCommentFootnoteProjection{Body: "[^一]: ![[A#part|shown]] [[B\\]]\n", Positions: agreementCommentExpectedPositions(graph.Span{Start: 8, Stop: 41})}, Fields: []agreementUnusedWidgetField{{Span: graph.Span{Start: 16, Stop: 33}, Word: "![[A#part|shown]]", Tuple: section}, {Span: graph.Span{Start: 34, Stop: 40}, Word: "[[B\\]]", Tuple: agreementCitation{Target: "B\\", State: "wikilink-broken"}}}}, check: []agreementUnusedCheckDefinition{{Span: graph.Span{Start: 0, Stop: 40}, Targets: []string{"A", "B"}}}},
		{name: "real unused definition has another owner", body: "[^m]: [[B]]\n\n<!--h-->[^n]: [[A]]\n"},
		{name: "same-target real definition cannot lend manufactured ownership", body: "[^m]: [[A]]\n\n<!--h-->[^n]: [[A]]\n"},
		{name: "inactive overlap refuses complete source", body: "<!--h-->[^n]: [[B]]\n%%[[A]]%%\n\n[r]: [[A]]\n"},
		{name: "odd escape refuses complete source", body: "<!--h-->[^n]: [[A]] \\[[B]]\n"},
		{name: "unfinished field refuses complete source", body: "<!--h-->[^n]: [[A]] [[B\n"},
		{name: "inline code has another source owner", body: "<!--h-->[^n]: [[A]] `[[B]]`\n"},
		{name: "later block has another source owner", body: "<!--h-->[^n]: [[A]]\n\n    ## [[B]]\n"},
		{name: "outside comment refuses fabricated owner", body: "%%\n<!--h-->[^n]: [[A]]\n%%\n"},
		{name: "used manufactured source is not discarded", body: "<!--h-->[^n]: [[A]]\nref[^n]\n"},
		{name: "ordinary source retains prior ownership", body: "[^n]: [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := agreementCommentFootnoteCitationSource{Original: tc.original, Check: agreementUnusedCitationSource{Fields: tc.original.Fields, Definitions: tc.check}}
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(want, agreementCommentFootnoteCitationReading(tc.body, grammar)); diff != "" {
					t.Fatalf("caught: complete comment footnote citation source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementCommentFootnoteCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body        string
		budget, old, want map[string]int
	}{
		{name: "one original native declaration", body: "<!--h-->[^n]: [[A]]\n", budget: map[string]int{"A": 1}, old: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "every original physical row retains target order", body: "<!--h-->[^n]: [[A]] [[A#part|shown]]\nnext [[B]]\n\n    [[C\\]]\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "later indentation is not a check contribution", body: "<!--\n[[A]]\n-->[^n]: [[A]]\n\n    [[B]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "all repeated and shared live targets", body: "[[A]] [[A]] [[B]]\n\n<!--h-->[^n]: [[A]] [[A]] [[B]]\n", budget: map[string]int{"A": 2, "B": 1}, old: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "all original root declarations", body: "<!--h-->[^a]: [[A]]\n<!--h-->[^b]: [[B]] [[A]]\n", budget: map[string]int{"A": 2, "B": 1}, old: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "native indentation retains original declaration", body: "  <!--h-->[^n]: [[A]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "embed alias and section keep check target", body: "<!--h-->[^n]: ![[A#part|shown]] [[B\\]]\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "missing reference cannot lend whole count", body: "<!--h-->[^n]: [[A]]\n\n[r]: [[A]]\n", budget: map[string]int{"A": 1}, old: map[string]int{"A": 1}},
		{name: "missing reference retains separate target", body: "<!--h-->[^n]: [[A]]\n\n    [[B]]\n\n[r]: [[B]]\n", budget: map[string]int{"A": 1}, old: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "real definition retains different wrong side", body: "[^m]: [[B]]\n\n<!--h-->[^n]: [[A]]\n", old: map[string]int{"A": 1}},
		{name: "same-target real definition refuses mixed ownership", body: "[^m]: [[A]]\n\n<!--h-->[^n]: [[A]]\n", old: map[string]int{"A": 1}},
		{name: "inactive field cannot borrow missing reference", body: "<!--h-->[^n]: [[B]]\n%%[[A]]%%\n\n[r]: [[A]]\n", old: map[string]int{"B": 1}},
		{name: "odd escape retains different source role", body: "<!--h-->[^n]: [[A]] \\[[B]]\n"},
		{name: "unfinished literal cannot invent target", body: "<!--h-->[^n]: [[A]] [[B\n"},
		{name: "used definition is not discarded", body: "<!--h-->[^n]: [[A]]\nref[^n]\n"},
		{name: "inline code retains its owner", body: "<!--h-->[^n]: [[A]] `[[B]]`\n"},
		{name: "CommonMark alone assigns reference", body: "https://example.invalid/ref[^n]\n\n<!--h-->[^n]: [[A]]\n"},
		{name: "page grammar alone assigns reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n<!--h-->[^n]: [[A]]\n"},
		{name: "URL makes complete field reading disagree", body: "<!--h-->[^n]: https://example.invalid/[[A]]\n"},
		{name: "full original retains all continuation occurrences", body: "<!--\n[[A]]\n-->[^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]\\\\[[A]]\n\n ^é\n", budget: map[string]int{"A": 3}, want: map[string]int{"A": 3}},
		{name: "full original retains literal comment and every later owner", body: " ^a\n<!--   ```\n`\\\\[[A]]^absent-prose 1.  ^é\n%%- item\n\n      ## <em>A</em>\n> - item\n\n       ^A\n``  ```\n~~~~\n[[#A]]A\n---\n\n<div>\n[[A]]\n</div>\n-->[^unused]: [[A]]\n````\n## !\n- > [!note] title\n[[A]]`https://example.invalid/`[[A]]` ", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "ordinary definition stays with earlier owner", body: "[^n]: [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementCommentFootnoteCitationBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Targets); diff != "" {
				t.Fatalf("caught: complete comment footnote citation budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementCommentMadeFootnoteTargets(c.Body)); diff != "" {
				t.Fatalf("caught: changed earlier comment made citation boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementCommentFootnoteCitationDifference(c, &f, &actual, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: comment footnote citation borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: comment footnote citation public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementSharedUnusedCitationDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete comment footnote citation public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
