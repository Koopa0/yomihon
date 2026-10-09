package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Outside markup cannot supply a field for a target whose every candidate
// belongs to discarded definitions. Both grammars must discard each owner;
// the signed receipts still read the actual page and the entire check result.
func agreementDiscardedTargetFields(body string) (raw map[agreementCitation]int, check map[string]int) {
	_, _, plainNodes := agreementUnusedBlockDeclarations(body, agreementFootnoteGrammar)
	_, _, gfmNodes := agreementUnusedBlockDeclarations(body, agreementExclusiveCodeGrammar)
	return agreementDiscardedFieldBudgets(body, agreementUnusedBlockSourceLines(plainNodes), agreementUnusedBlockSourceLines(gfmNodes))
}

func TestAgreementDiscardedTargetFields(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	rawA := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	section := agreementCitation{Target: "B", Section: "part", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body  string
		raw         map[agreementCitation]int
		check       map[string]int
		public      bool
		p0          map[agreementCitation]int
		judge, page map[string]int
	}{
		{name: "independent opener", body: "> [!note] words\n\n[^n]: [[B]] [[B]]\n", raw: map[agreementCitation]int{b: 2}, check: map[string]int{"B": 2}, public: true, p0: map[agreementCitation]int{b: 2}, judge: map[string]int{"B": 2}},
		{name: "selected target beside live opener field", body: "> [!note] [[A]]\n\n[^n]: [[B]] [[B#part]]\n", raw: map[agreementCitation]int{b: 1, section: 1}, check: map[string]int{"B": 2}, public: true, p0: map[agreementCitation]int{b: 1, section: 1}, judge: map[string]int{"B": 2}},
		{name: "raw and normalized fields stay separate", body: "> [!note] [[A]]\n\n[^n]: [[A\\]] [[B]]\n", raw: map[agreementCitation]int{rawA: 1, b: 1}, check: map[string]int{"B": 1}, public: true, p0: map[agreementCitation]int{rawA: 1, b: 1}, judge: map[string]int{"B": 1}},
		{name: "used and unused beside live markers", body: "> [!note] [[A]]\n\nref[^u]\n\n[^u]: [[A]]\n\n[^n]: [[B]]\n", raw: map[agreementCitation]int{b: 1}, check: map[string]int{"B": 1}, public: true, p0: map[agreementCitation]int{b: 1}, judge: map[string]int{"B": 1}},
		{name: "all blocks and lines", body: "> [!note] words\n\n[^n]: [[A]] [[A]]\nnext [[B]]\n\n    ## [[B#part]]\n\n    ```\n    [[B]]\n    ```\n\n[^m]: [[B]]\n", raw: map[agreementCitation]int{a: 2, b: 3, section: 1}, check: map[string]int{"A": 2, "B": 4}},
		{name: "outside occurrence removes whole target", body: "> [!note] [[A]]\n\n[^n]: [[A]] [[B]]\n", raw: map[agreementCitation]int{b: 1}, check: map[string]int{"B": 1}, public: true, p0: map[agreementCitation]int{b: 1}, judge: map[string]int{"B": 1}},
		{name: "incomplete field refuses set", body: "> [!note] words\n\n[^n]: [[A]] [[B\n"},
		{name: "nested field refuses set", body: "> [!note] words\n\n[^n]: [[A[[B]]]] [[A]]\n"},
		{name: "block field refuses set", body: "> [!note] words\n\n[^n]: [[A#^a]] [[B]]\n"},
		{name: "used definition stays outside", body: "> [!note] words\n\nref[^n]\n\n[^n]: [[A]]\n"},
		{name: "linkify hides a reference only in page grammar", body: "https://example.invalid/ref[^n]\n\n[^n]: [[B]]\n"},
		{name: "recorded discarded field beside outside opener", body: "\t[[image.png]]``` [[A]]\n- > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n ^A\n\\`É\n## [[A|alias]]\n[[A\\]]A\n---\n`章節\n ^é\n`[[A]]`", raw: map[agreementCitation]int{b: 1}, check: map[string]int{"B": 1}, public: true, p0: map[agreementCitation]int{b: 1}, judge: map[string]int{"B": 1}},
		{name: "grammar disagreement stays outside", body: "> [!note] words\n\nhttps://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]]\n"},
		{name: "outside declaration stays outside", body: "> [!note] [[A]]\n\n## [[B]]\n"},
		{name: "field across table cells stays outside", body: "> [!note] words\n\n[^n]: words\n\n    | a | b |\n    |---|---|\n    | [[A|alias]] | [[B]] |\n"},
		{name: "recorded hidden reference beside outside comment", body: "A\n---\nÉ\n[^n]: [[A]]\n\n    [[B]]\n> > <!--\n[[A]]\n-->  ^a\n<!-- [[A]] -->ref[^n]\n    ```\n[[A]] [[A]][[A\\]]É\n ^é\n", raw: map[agreementCitation]int{b: 1}, check: map[string]int{"B": 1}, public: true, page: map[string]int{"B": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, check := agreementDiscardedTargetFields(tc.body)
			if diff := cmp.Diff(tc.raw, raw); diff != "" {
				t.Fatalf("caught: complete discarded target raw inventory (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.check, check); diff != "" {
				t.Fatalf("caught: complete discarded target check inventory (-want +got):\n%s", diff)
			}
			if !tc.public {
				return
			}
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			found0 := make(map[agreementCitation]int)
			foundJudge, foundPage := make(map[string]int), make(map[string]int)
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				if kind, authority, wrong := agreementExclusiveUnusedDiagnosticDifference(c, &f, &actual, raw); kind != "" {
					if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
						t.Fatal("caught: discarded target diagnostic ownership")
					}
					found0[f.Tuple] = f.Multiplicity
				}
				if kind, authority, wrong := agreementExclusiveHiddenCitationDifference(c, &f, &actual, raw); kind != "" {
					if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
						t.Fatal("caught: discarded target page ownership")
					}
					foundPage[f.Tuple.Target] = f.Multiplicity
				}
				if kind, authority, wrong := agreementUnusedBlockCheckDifference(c, &f, &actual, check); kind != "" {
					if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge" {
						t.Fatal("caught: discarded target check ownership")
					}
					foundJudge[f.Tuple.Target] = f.Multiplicity
				}
			}
			if len(found0) == 0 {
				found0 = nil
			}
			if len(foundJudge) == 0 {
				foundJudge = nil
			}
			if len(foundPage) == 0 {
				foundPage = nil
			}
			if diff := cmp.Diff(tc.p0, found0); diff != "" {
				t.Fatalf("caught: complete discarded target diagnostic receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.judge, foundJudge); diff != "" {
				t.Fatalf("caught: complete discarded target judge receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.page, foundPage); diff != "" {
				t.Fatalf("caught: complete discarded target page receipts (-want +got):\n%s", diff)
			}
		})
	}
}
