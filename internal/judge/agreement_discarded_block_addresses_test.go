package judge_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
)

// Outside markup cannot supply an address whose every raw declaration belongs
// to discarded blocks. Both grammars must discard the entire owned set.
func agreementDiscardedBlockAddressReading(body string, grammar goldmark.Markdown) agreementUnusedTextAddresses {
	source, _, nodes := agreementUnusedBlockDeclarations(body, grammar)
	return agreementUnusedBlockAddressSources(body, source, nodes)
}

func agreementDiscardedBlockAddressBudget(body string) agreementUnusedTextAddresses {
	plain := agreementDiscardedBlockAddressReading(body, agreementFootnoteGrammar)
	gfm := agreementDiscardedBlockAddressReading(body, agreementExclusiveCodeGrammar)
	if !cmp.Equal(plain, gfm) {
		return agreementUnusedTextAddresses{}
	}
	return plain
}

func TestAgreementDiscardedBlockAddresses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedTextAddresses
		public     map[string][]string
	}{
		{name: "outside live opener", body: "> [!note] [[A]]\n\n[^n]: words ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}},
		{name: "outside percent comment", body: "%%words%%\n\n[^n]: words ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}},
		{name: "outside open comment", body: "[^n]: words ^a\n\n<!--\nwords\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}},
		{name: "all blocks and declarations", body: "> [!note] [[A]]\n\n[^n]: first ^A\nsecond ^a\n\n    ## heading ^b\n\n    ```\n    words ^c\n    ```\n\n[^m]: other ^d\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 2, "^b": 1, "^c": 1, "^d": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^A", "second ^a"}, "^b": {"    ## heading ^b"}, "^c": {"    words ^c"}, "^d": {"[^m]: other ^d"}}}},
		{name: "indented code declaration", body: "> [!note] [[A]]\n\n[^n]: words ^a\n\n        code ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1, "^b": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}, "^b": {"        code ^b"}}}},
		{name: "nested list and quote declarations", body: "> [!note] [[A]]\n\n[^n]: words ^a\n\n    - words ^b\n\n    > words ^c\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1, "^b": 1, "^c": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}, "^b": {"    - words ^b"}, "^c": {"    > words ^c"}}}},
		{name: "used and unused definitions", body: "> [!note] [[A]]\n\nref[^u]\n\n[^u]: words ^a\n\n[^n]: other ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"[^n]: other ^b"}}}, public: map[string][]string{"^b": {"excerpt-only", "judge-only"}}},
		{name: "all case-folded addresses", body: "> [!note] [[A]]\n\n[^n]: first ^A\nsecond ^a\n\n[^m]: other ^B\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 2, "^b": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^A", "second ^a"}, "^b": {"[^m]: other ^B"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}, "^b": {"excerpt-only", "judge-only"}}},
		{name: "whole target removed beside live claim", body: "> [!note] words ^a\n\n[^n]: words ^a\nnext ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"next ^b"}}}, public: map[string][]string{"^b": {"excerpt-only", "judge-only"}}},
		{name: "used reference cannot borrow outside context", body: "> [!note] ref[^n]\n\n[^n]: words ^a\n"},
		{name: "hidden reference still activates native owner", body: "%%ref[^n]%%\n\n[^n]: words ^a\n"},
		{name: "common reference not used by page grammar", body: "https://example.invalid/ref[^n]\n\n[^n]: words ^a\n"},
		{name: "page reference not used by common grammar", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words ^a\n"},
		{name: "outside address is not discarded", body: "> [!note] words ^a\n\nother ^b\n"},
		{name: "shared address is not exclusive", body: "ordinary ^a\n\n[^n]: words ^a\n"},
		{name: "physical carriage returns retained", body: "> [!note] [[A]]\r\n\r\n[^n]: words ^a\r\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: map[string][]string{}},
		{name: "recorded discarded address beside outside opener", body: "\t[[image.png]]``` [[A]]\n- > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n ^A\n\\`É\n## [[A|alias]]\n[[A\\]]A\n---\n`章節\n ^é\n`[[A]]`", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {" ^A"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}},
		{name: "recorded discarded address beside outside quote", body: "[^unused]: [[A]]\n ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- [ ] [[A]]\n> [!unknown] title\n ```\n> > ", public: map[string][]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementDiscardedBlockAddressBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete discarded block address inventory (-want +got):\n%s", diff)
			}
			if tc.public == nil {
				return
			}
			c := agreementCase{Body: tc.body}
			_, actual := agreementIsolatedPage(t, c)
			found := make(map[string][]string)
			for _, f := range agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})[0] {
				if f.Property != "P3" {
					continue
				}
				if budget.Counts[f.Fragment] == 0 {
					if kind, _, _ := agreementUnusedTextAddressDifference(c, &f, budget); kind != "" {
						t.Fatal("caught: discarded block address borrowed an unowned public fragment")
					}
					continue
				}
				kind, authority, wrong := agreementUnusedTextAddressDifference(c, &f, budget)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge+excerpt" {
					t.Fatalf("caught: discarded block address public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				found[f.Fragment] = append(found[f.Fragment], f.Direction)
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
					if kind, _, _ := agreementUnusedTextAddressDifference(other, &f, budget); kind != "" {
						t.Fatal("caught: discarded block address borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Cut = "words" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = false }, func(f *agreementFailure) { f.ExcerptFound = false },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementUnusedTextAddressDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: discarded block address borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			for fragment := range found {
				slices.Sort(found[fragment])
			}
			if diff := cmp.Diff(tc.public, found); diff != "" {
				t.Fatalf("caught: complete discarded block address public set (-want +got):\n%s", diff)
			}
		})
	}
}
