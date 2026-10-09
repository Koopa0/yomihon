package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"

	"github.com/koopa0/yomihon/internal/graph"
)

const agreementUnusedEscapedField = "unused-escaped"

// Keep quoted fields in the source inventory without lending their counts to an
// active diagnostic. Unquoted fields still belong to the discarded definition.
func agreementUnusedEscapedWidgetBudget(body string) agreementUnusedWidgetPayload {
	plain := agreementUnusedPlainWidgetReading(body, agreementFootnoteGrammar, true)
	gfm := agreementUnusedPlainWidgetReading(body, agreementExclusiveCodeGrammar, true)
	if !cmp.Equal(plain, gfm) || len(plain) == 0 {
		return agreementUnusedWidgetPayload{}
	}
	budget := make(map[agreementCitation]int)
	escapedCount := 0
	for _, field := range plain {
		budget[field.Tuple]++
		if field.Tuple.SourceRole == agreementUnusedEscapedField {
			escapedCount++
		}
	}
	if escapedCount == 0 {
		return agreementUnusedWidgetPayload{}
	}
	return agreementUnusedWidgetPayload{Body: body, Tuples: budget}
}

func TestAgreementUnusedEscapedWidgetSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", Section: "part", State: "wikilink-broken"}
	escapedA := agreementCitation{SourceRole: agreementUnusedEscapedField, Target: "A", State: "wikilink-broken"}
	escapedB := agreementCitation{SourceRole: agreementUnusedEscapedField, Target: "B", Section: "part", State: "wikilink-broken"}
	commentC := agreementCitation{SourceRole: agreementUnusedCommentOverlap, Target: "C", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       []agreementUnusedWidgetField
	}{
		{name: "active escaped and embedded fields stay distinct", body: "[^n]: [[A]] \\[[A]] ![[B#part|shown]] \\![[B#part|shown]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 13, Stop: 18}, Word: "[[A]]", Tuple: escapedA}, {Span: graph.Span{Start: 19, Stop: 36}, Word: "![[B#part|shown]]", Tuple: b}, {Span: graph.Span{Start: 38, Stop: 55}, Word: "![[B#part|shown]]", Tuple: escapedB}}},
		{name: "even and odd backslashes have different roles", body: "[^n]: \\\\[[A]] \\[[A]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 8, Stop: 13}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 15, Stop: 20}, Word: "[[A]]", Tuple: escapedA}}},
		{name: "comment overlap stays inactive beside escape", body: "[^n]: [[B]] \\[[A]]\n%%[[C]]%%\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[B]]", Tuple: agreementCitation{Target: "B", State: "wikilink-broken"}}, {Span: graph.Span{Start: 13, Stop: 18}, Word: "[[A]]", Tuple: escapedA}, {Span: graph.Span{Start: 21, Stop: 26}, Word: "[[C]]", Tuple: commentC}}},
		{name: "escape inside comment remains inactive", body: "%%\n[^n]: \\[[A]] [[A]]\n%%\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 10, Stop: 15}, Word: "[[A]]", Tuple: escapedA}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Tuple: agreementCitation{SourceRole: agreementUnusedCommentOverlap, Target: "A", State: "wikilink-broken"}}}},
		{name: "raw suffix keeps authored target", body: "[^n]: [[A\\]] \\[[A\\]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 12}, Word: "[[A\\]]", Tuple: agreementCitation{Target: "A\\", State: "wikilink-broken"}}, {Span: graph.Span{Start: 14, Stop: 20}, Word: "[[A\\]]", Tuple: agreementCitation{SourceRole: agreementUnusedEscapedField, Target: "A\\", State: "wikilink-broken"}}}},
		{name: "used source stays outside", body: "ref[^n]\n\n[^n]: [[B]] \\[[A]]\n"},
		{name: "ordinary source stays outside", body: "[[B]] \\[[A]]\n"},
		{name: "another inline owner refuses all fields", body: "[^n]: [[B]] \\[[A]] `[[C]]`\n"},
		{name: "later block refuses earlier fields", body: "[^n]: [[B]] \\[[A]]\n\n    ## [[C]]\n"},
		{name: "malformed escaped field refuses whole source", body: "[^n]: [[B]] \\[[A\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(tc.want, agreementUnusedPlainWidgetReading(tc.body, grammar, true)); diff != "" {
					t.Fatalf("caught: complete unused escaped widget source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementUnusedEscapedWidgets(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "B", Section: "part", State: "wikilink-broken"}
	escapedA := agreementCitation{SourceRole: agreementUnusedEscapedField, Target: "A", State: "wikilink-broken"}
	escapedB := agreementCitation{SourceRole: agreementUnusedEscapedField, Target: "B", Section: "part", State: "wikilink-broken"}
	commentA := agreementCitation{SourceRole: agreementUnusedCommentOverlap, Target: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body        string
		budget, old, want map[agreementCitation]int
	}{
		{name: "same target active escape and shared live counts", body: "[[A]] [[A]]\n\n[^n]: [[A]] \\[[A]] [[A]] \\[[A]]\n", budget: map[agreementCitation]int{a: 2, escapedA: 2}, want: map[agreementCitation]int{a: 2}},
		{name: "all definitions paragraphs and sections", body: "[^n]: [[A]] \\[[A]]\n\n    more ![[B#part|shown]] \\![[B#part|shown]]\n\n[^m]: [[A]] \\[[A]]\n", budget: map[agreementCitation]int{a: 2, escapedA: 2, section: 1, escapedB: 1}, want: map[agreementCitation]int{a: 2, section: 1}},
		{name: "even prefix retains active diagnostic", body: "[^n]: \\\\[[A]] \\[[A]]\n", budget: map[agreementCitation]int{a: 1, escapedA: 1}, want: map[agreementCitation]int{a: 1}},
		{name: "escaped only cannot lend missing reference diagnostic", body: "[^n]: words \\[[A]]\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{escapedA: 1}},
		{name: "escaped target cannot lend to another missing owner", body: "[^n]: [[B]] \\[[A]]\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{b: 1, escapedA: 1}, want: map[agreementCitation]int{b: 1}},
		{name: "hidden and escaped fields cannot lend diagnostic", body: "%%\n[^n]: \\[[A]] [[A]]\n%%\n\n[r]: [[A]]\n", budget: map[agreementCitation]int{escapedA: 1, commentA: 1}},
		{name: "whole count refuses another absent owner", body: "[r]: [[A]]\n\n[^n]: [[A]] \\[[A]]\n", budget: map[agreementCitation]int{a: 1, escapedA: 1}},
		{name: "grammar use disagreement refuses source", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A]] \\[[A]]\n"},
		{name: "inline URL grammar disagreement refuses source", body: "[^n]: https://example.invalid/[[A]] \\[[A]]\n"},
		{name: "used definition has no unused source", body: "ref[^n]\n\n[^n]: [[A]] \\[[A]]\n"},
		{name: "older source keeps ownership without escapes", body: "[[A]]\n\n[^n]: words [[A]]\n", old: map[agreementCitation]int{a: 1}},
		{name: "complete recorded original keeps every public tuple", body: "[^unused]: [[A]]\n[[B|alias]][[A\\]]## !\nB\n\\[[A]]![[image.png]]", budget: map[agreementCitation]int{a: 1, b: 1, {Target: "A\\", State: "wikilink-broken"}: 1, {Target: "image.png", State: "wikilink-broken"}: 1, escapedA: 1}, want: map[agreementCitation]int{a: 1, b: 1, {Target: "A\\", State: "wikilink-broken"}: 1, {Target: "image.png", State: "wikilink-broken"}: 1}},
		{name: "ordinary prose remains outside", body: "[[A]] \\[[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementUnusedEscapedWidgetBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete unused escaped widget budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementUnusedTextWidgetBudget(c.Body).Tuples); diff != "" {
				t.Fatalf("caught: changed earlier unused text source boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementSharedUnusedDiagnosticDifference(c, &f, &actual, r.Diagnostics, budget)
				if f.Property != "P0" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: unused escaped widget borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: unused escaped widget public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementSharedUnusedDiagnosticDrift(t, c, &f, &actual, r.Diagnostics, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete unused escaped widget public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
