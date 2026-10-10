package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

// Several fields can share one literal code declaration. Keep all targets,
// sections and occurrences; ambiguous fields cannot leave a partial inventory.
func agreementLiteralCodeFields(literal string) ([]graph.Wikilink, bool) {
	var fields []graph.Wikilink
	for off := 0; off < len(literal); {
		relative := strings.Index(literal[off:], "[[")
		if relative < 0 {
			break
		}
		open := off + relative
		rest := literal[open+2:]
		inner, tail, closed := strings.Cut(rest, "]]")
		if !closed || strings.ContainsAny(inner, "[]\r\n") {
			return nil, false
		}
		off = len(literal) - len(tail)
		if graph.EscapedWikilinkAt(literal, open) {
			continue
		}
		if open > 0 && literal[open-1] == '!' {
			return nil, false
		}
		link, cites := graph.ParseWikilink(inner)
		if !cites && (link.Heading == "" || link.Block != "") {
			return nil, false
		}
		fields = append(fields, link)
	}
	return fields, true
}

func TestAgreementCodeFields(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", Section: "B", State: "wikilink-broken"}
	raw := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	plain := agreementCitation{Target: "B", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       agreementWidgetCodeBudget
		failures   int
	}{
		{name: "multiple section fields", body: "`open\n[[A#A]] [[A#A]] [[B#B]]\nclose`\n", want: agreementWidgetCodeBudget{Citations: map[agreementCitation]int{a: 2, b: 1}, LocalHeadings: map[string]int{}}, failures: 4},
		{name: "raw fields beside literal words", body: "`open\nbefore [[A\\]] after [[B|alias]]\nclose`\n", want: agreementWidgetCodeBudget{Citations: map[agreementCitation]int{raw: 1, plain: 1}, LocalHeadings: map[string]int{}}, failures: 4},
		{name: "adjacent section fields", body: "`open\n[[A#A]][[B#B]]\nclose`\n", want: agreementWidgetCodeBudget{Citations: map[agreementCitation]int{a: 1, b: 1}, LocalHeadings: map[string]int{}}, failures: 4},
		{name: "quoted fields beside URL", body: "> ```\n> [[A#A]] [[B#B]]\n> ```\n\nhttps://example.invalid/words\n", want: agreementWidgetCodeBudget{Citations: map[agreementCitation]int{a: 1, b: 1}, LocalHeadings: map[string]int{}}, failures: 4},
		{name: "whole local address set", body: "`open\n[[#A]] [[#A]] [[#B]]\nclose`\n", want: agreementWidgetCodeBudget{Citations: map[agreementCitation]int{}, LocalHeadings: map[string]int{"A": 2, "B": 1}}, failures: 1},
		{name: "escaped field stays separate", body: "`open\n\\[[A#A]]\nclose`\n"},
		{name: "embed needs ownership", body: "`open\n![[A#A]] [[B#B]]\nclose`\n"},
		{name: "nested field refuses whole set", body: "`open\n[[A[[B]]]] [[B#B]]\nclose`\n"},
		{name: "terminal open field refuses whole set", body: "`open\n[[B#B]]\nclose [[A`\n"},
		{name: "open field refuses whole set", body: "`open\n[[B#B]] [[A\nclose`\n"},
		{name: "local block refuses whole set", body: "`open\n[[#^a]] [[B#B]]\nclose`\n"},
		{name: "single-line span stays separate", body: "`[[A#A]] [[B#B]]`\n"},
		{name: "root fence stays separate", body: "```\n[[A#A]] [[B#B]]\n```\n"},
		{name: "live marker stays separate", body: "%%\n`open\n[[A#A]] [[B#B]]\nclose`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementCodeFieldBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete literal code field inventory (-want +got):\n%s", diff)
			}
			if tc.failures == 0 {
				return
			}
			c := agreementCase{Body: tc.body}
			r, a := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &a)
			if len(failures) != tc.failures {
				t.Fatalf("caught: code field public delta count=%d want=%d %+v", len(failures), tc.failures, failures)
			}
			for i := range failures {
				f := &failures[i]
				kind, authority, wrong := agreementWidgetCodeDifference(c, f, budget, &a)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: code field public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				if f.Property == "P1" {
					absent := a
					absent.CodeCitations = nil
					if kind, _, _ := agreementWidgetCodeDifference(c, f, budget, &absent); kind != "" {
						t.Fatal("caught: code fields borrowed absent carriers")
					}
					drift := a
					drift.CodeCitations = append([]agreementCitation{}, a.CodeCitations...)
					for i := range drift.CodeCitations {
						if drift.CodeCitations[i].Target == f.Tuple.Target {
							drift.CodeCitations[i].Section = "unowned"
						}
					}
					if kind, _, _ := agreementWidgetCodeDifference(c, f, budget, &drift); kind != "" {
						t.Fatal("caught: code fields borrowed same-cardinality tuple drift")
					}
				}
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementWidgetCodeDifference(other, f, budget, &a); kind != "" {
						t.Fatal("caught: code fields borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementWidgetCodeDifference(c, &changed, budget, &a); kind != "" {
						t.Fatalf("caught: code fields borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
		})
	}
}
