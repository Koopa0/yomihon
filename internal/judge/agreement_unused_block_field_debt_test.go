package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

// Every raw field of a selected target is discarded in both grammars.
// Check spelling is tracked separately so escaped names cannot borrow a live one.
func agreementUnusedBlockFieldZones(body string, grammar goldmark.Markdown) []graph.Span {
	source, doc, nodes := agreementUnusedBlockDeclarations(body, grammar)
	if !agreementUnusedBlockMarkerContext(source, doc, nodes) {
		return nil
	}
	var spans []graph.Span
	for _, node := range nodes {
		for i := range node.Lines().Len() {
			line := node.Lines().At(i)
			spans = append(spans, graph.Span{Start: line.Start, Stop: line.Stop})
		}
	}
	return spans
}
func agreementUnusedBlockFieldBudgets(body string) (raw map[agreementCitation]int, check map[string]int) {
	plain := agreementUnusedBlockFieldZones(body, agreementFootnoteGrammar)
	gfm := agreementUnusedBlockFieldZones(body, agreementExclusiveCodeGrammar)
	raw = agreementExclusiveFieldBudget(body, plain, gfm)
	normalized := agreementExclusiveTargetFieldBudget(body, plain, gfm, func(field string) string {
		targets := judge.LinkTargets(field)
		if len(targets) != 1 {
			return ""
		}
		return targets[0]
	})
	for tuple, n := range normalized {
		if check == nil {
			check = make(map[string]int)
		}
		check[tuple.Target] += n
	}
	return raw, check
}
func agreementUnusedBlockCheckDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "judge-only" || f.Tuple.Target == "" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 || budget[f.Tuple.Target] < f.Multiplicity {
		return "", "", ""
	}
	for _, tuple := range actual.Citations {
		if tuple.Target == f.Tuple.Target {
			return "", "", ""
		}
	}
	n := 0
	for _, target := range judge.LinkTargets(c.Body) {
		if target == f.Tuple.Target {
			n++
		}
	}
	if n != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "judge"
}

func TestAgreementUnusedBlockFields(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	rawA := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		raw        map[agreementCitation]int
		check      map[string]int
		public     bool
		p0         map[agreementCitation]int
		p1         map[string]int
		page       map[string]int
	}{
		{name: "field-shaped label is not a definition", body: "[^[[A]]n]: [[B]]\n\n    ## heading\n"},
		{name: "field across table cells stays outside", body: "[^n]: words\n\n    | a | b |\n    |---|---|\n    | [[A|alias]] | [[B]] |\n"},
		{name: "recorded comment-hidden reference", body: "A\n---\n\t[[image.png]] ^A\n    ```\n![[image.png]]  > [!note] title\n<!-- [[A]] -->[[A#A]][[image.png]]ref[^n]\n[^unused]: [[A]]\nA\n===\n0> [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\n[[A\\]][[image.png]]0- > [!unknown] title\n![[A#A]]https://example.invalid/`[[A]]` ## <em>A</em>\n", raw: map[agreementCitation]int{rawA: 1, b: 1}, check: map[string]int{"B": 1}, public: true, page: map[string]int{"A\\": 1, "B": 1}},
		{name: "real reference activates recorded definition", body: "ref[^n]\n\nA\n---\n\t[[image.png]] ^A\n    ```\n![[image.png]]  > [!note] title\n<!-- [[A]] -->[[A#A]][[image.png]]ref[^n]\n[^unused]: [[A]]\nA\n===\n0> [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\n[[A\\]][[image.png]]0- > [!unknown] title\n![[A#A]]https://example.invalid/`[[A]]` ## <em>A</em>\n"},
		{name: "heading beside whole fields", body: "[^n]: [[A]] [[A]]\n\n    ## heading\n", raw: map[agreementCitation]int{a: 2}, check: map[string]int{"A": 2}, public: true, p0: map[agreementCitation]int{a: 2}, p1: map[string]int{"A": 2}},
		{name: "raw suffix and check spelling", body: "[^n]: [[A\\]]\n\n    ## heading\n", raw: map[agreementCitation]int{rawA: 1}, check: map[string]int{"A": 1}, public: true, p0: map[agreementCitation]int{rawA: 1}, p1: map[string]int{"A": 1}},
		{name: "normalized outside target refuses check owner", body: "[[A]]\n\n[^n]: [[A\\]] [[B]]\n\n    ## heading\n", raw: map[agreementCitation]int{rawA: 1, b: 1}, check: map[string]int{"B": 1}, public: true, p0: map[agreementCitation]int{rawA: 1, b: 1}, p1: map[string]int{"B": 1}},
		{name: "partial check of discarded fields", body: "[^n]: [[A]]\n\n    ```\n    [[A]] [[A]]\n    ```\n", raw: map[agreementCitation]int{a: 3}, check: map[string]int{"A": 3}, public: true, p0: map[agreementCitation]int{a: 3}, p1: map[string]int{"A": 1}},
		{name: "all blocks and sections", body: "[^n]: [[A]] [[A#A]]\n\n    ## [[B]]\n\n    ```\n    [[A]]\n    ```\n\n[^m]: [[B]]\n", raw: map[agreementCitation]int{a: 2, section: 1, b: 2}, check: map[string]int{"A": 3, "B": 2}},
		{name: "literal alias delimiter", body: "[^n]: [[A\\|alias]]\n\n    ## heading\n", raw: map[agreementCitation]int{a: 1}, check: map[string]int{"A": 1}},
		{name: "discarded opener text", body: "[^n]: [!note] [[A]]\n\n    ## heading\n", raw: map[agreementCitation]int{a: 1}, check: map[string]int{"A": 1}, public: true, p0: map[agreementCitation]int{a: 1}, p1: map[string]int{"A": 1}},
		{name: "table beside whole fields", body: "[^n]: [[A]]\n\n    | a | b |\n    |---|---|\n    | [[B]] | text |\n", raw: map[agreementCitation]int{a: 1, b: 1}, check: map[string]int{"A": 1, "B": 1}},
		{name: "used and unused set", body: "ref[^u]\n\n[^u]: [[A]]\n\n    ## heading\n\n[^n]: [[B]]\n\n    ## heading\n", raw: map[agreementCitation]int{b: 1}, check: map[string]int{"B": 1}, public: true, p0: map[agreementCitation]int{b: 1}, p1: map[string]int{"B": 1}},
		{name: "used definition", body: "ref[^n]\n\n[^n]: [[A]]\n\n    ## heading\n"},
		{name: "outside source", body: "[[A]]\n\n## [[B]]\n"},
		{name: "shared raw target", body: "[[A]]\n\n[^n]: [[A#A]]\n\n    ## heading\n"},
		{name: "grammar use differs", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]]\n\n    ## heading\n"},
		{name: "outside callout", body: "> [!note] [[A]]\n\n[^n]: [[B]]\n\n    ## heading\n"},
		{name: "incomplete field", body: "[^n]: [[A]] [[B\n\n    ## heading\n"},
		{name: "nested field", body: "[^n]: [[A[[B]]]]\n\n    ## heading\n"},
		{name: "block field", body: "[^n]: [[A#^a]] [[B]]\n\n    ## heading\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, check := agreementUnusedBlockFieldBudgets(tc.body)
			if diff := cmp.Diff(tc.raw, raw); diff != "" {
				t.Fatalf("caught: complete unused block raw fields (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.check, check); diff != "" {
				t.Fatalf("caught: complete unused block check fields (-want +got):\n%s", diff)
			}
			if !tc.public {
				return
			}
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &actual)
			found0 := make(map[agreementCitation]int)
			found1 := make(map[string]int)
			foundPage := make(map[string]int)
			for i := range failures {
				f := &failures[i]
				if kind, authority, wrong := agreementExclusiveUnusedDiagnosticDifference(c, f, &actual, raw); kind != "" {
					if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
						t.Fatal("caught: unused block diagnostic ownership")
					}
					found0[f.Tuple] = f.Multiplicity
				}
				if kind, authority, wrong := agreementExclusiveHiddenCitationDifference(c, f, &actual, raw); kind != "" {
					if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
						t.Fatal("caught: unused block page ownership")
					}
					foundPage[f.Tuple.Target] = f.Multiplicity
				}
				kind, authority, wrong := agreementUnusedBlockCheckDifference(c, f, &actual, check)
				if kind == "" {
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge" {
					t.Fatal("caught: unused block check ownership")
				}
				found1[f.Tuple.Target] = f.Multiplicity
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementUnusedBlockCheckDifference(other, f, &actual, check); kind != "" {
						t.Fatal("caught: unused block check borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "words" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementUnusedBlockCheckDifference(c, &changed, &actual, check); kind != "" {
						t.Fatalf("caught: unused block check borrowed signature %s", agreementSignature(&changed))
					}
				}
				if kind, _, _ := agreementUnusedBlockCheckDifference(c, f, &actual, nil); kind != "" {
					t.Fatal("caught: unused block check borrowed absent source")
				}
				extra := actual
				extra.Citations = append(append([]agreementCitation(nil), actual.Citations...), agreementCitation{Target: f.Tuple.Target, State: "wikilink-broken"})
				if kind, _, _ := agreementUnusedBlockCheckDifference(c, f, &extra, check); kind != "" {
					t.Fatal("caught: unused block check borrowed page carrier")
				}
			}
			if len(found0) == 0 {
				found0 = nil
			}
			if len(found1) == 0 {
				found1 = nil
			}
			if len(foundPage) == 0 {
				foundPage = nil
			}
			if diff := cmp.Diff(tc.page, foundPage); diff != "" {
				t.Fatalf("caught: complete unused block page receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.p0, found0); diff != "" {
				t.Fatalf("caught: complete unused block diagnostic receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.p1, found1); diff != "" {
				t.Fatalf("caught: complete unused block check receipts (-want +got):\n%s", diff)
			}
		})
	}
}
