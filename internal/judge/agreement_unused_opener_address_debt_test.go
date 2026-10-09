package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func agreementUnusedOpenerAddressBudget(body string) agreementUnusedTextAddresses {
	plain := agreementUnusedAddressReading(body, agreementFootnoteGrammar, false, true)
	gfm := agreementUnusedAddressReading(body, agreementExclusiveCodeGrammar, false, true)
	if !cmp.Equal(plain, gfm) {
		return agreementUnusedTextAddresses{}
	}
	return plain
}

func TestAgreementUnusedOpenerAddresses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedTextAddresses
		public     bool
	}{
		{name: "marker at source start", body: "[^n]: [!note] words ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: [!note] words ^a"}}}, public: true},
		{name: "marker at source end", body: "[^n]: words ^a\n[!", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: true},
		{name: "discarded opener words", body: "[^n]: words ^a\nnext > [!note] words\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: true},
		{name: "recorded literal marker", body: "[^unused]: [[A]]\n ^a\n![[A#A]]``> [!unknown] title\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {" ^a"}}}, public: true},
		{name: "all lines and definitions", body: "[^n]: first ^A\nsecond [!note] ^a\n\n    third [!warning] ^b\n\n[^m]: fourth [!unknown] ^c\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 2, "^b": 1, "^c": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^A", "second [!note] ^a"}, "^b": {"    third [!warning] ^b"}, "^c": {"[^m]: fourth [!unknown] ^c"}}}, public: true},
		{name: "markup beside marker", body: "[^n]: *words* [!note] ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: *words* [!note] ^a"}}}, public: true},
		{name: "live shared address", body: "ordinary ^a\n\n[^n]: words [!note] ^a\nnext [!warning] ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"next [!warning] ^b"}}}, public: true},
		{name: "used and unused set", body: "ref[^u]\n\n[^u]: used ^a\n\n[^n]: unused [!note] ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"[^n]: unused [!note] ^b"}}}, public: true},
		{name: "live callout outside", body: "> [!note] [[A]]\n\n[^n]: words [!warning] ^a\n"},
		{name: "used marked definition", body: "ref[^n]\n\n[^n]: words [!note] ^a\n"},
		{name: "used marker beside unused definition", body: "ref[^u]\n\n[^u]: words [!note]\n\n[^n]: words [!warning] ^a\n"},
		{name: "grammars disagree on use", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words [!note] ^a\n"},
		{name: "other live percent marker", body: "%%\n[^n]: words [!note] ^a\n%%\n"},
		{name: "other live HTML marker", body: "<!--\n[^n]: words [!note] ^a\n-->\n"},
		{name: "discarded percent is not this proof", body: "[^n]: words %% [!note] ^a\n"},
		{name: "discarded HTML is not this proof", body: "[^n]: words <!-- [!note] ^a\n"},
		{name: "later code block refuses set", body: "[^n]: words [!note] ^a\n\n    ```\n    words ^b\n    ```\n"},
		{name: "later heading refuses set", body: "[^n]: words [!note] ^a\n\n    ## heading ^b\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementUnusedOpenerAddressBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete unused opener address inventory (-want +got):\n%s", diff)
			}
			if !tc.public {
				return
			}
			c := agreementCase{Body: tc.body}
			r, a := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &a)
			fragments := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{a})
			failures = append(failures, fragments[0]...)
			matched := 0
			for i := range failures {
				f := &failures[i]
				if f.Property != "P3" || budget.Counts[f.Fragment] == 0 {
					continue
				}
				kind, authority, wrong := agreementUnusedTextAddressDifference(c, f, budget)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge+excerpt" {
					t.Fatalf("caught: unused opener address public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				matched++
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementUnusedTextAddressDifference(other, f, budget); kind != "" {
						t.Fatal("caught: unused opener address borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Cut = "words" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = false }, func(f *agreementFailure) { f.ExcerptFound = false },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementUnusedTextAddressDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: unused opener address borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			if matched != 2*len(budget.Counts) {
				t.Fatalf("caught: unused opener address public set got=%d want=%d", matched, 2*len(budget.Counts))
			}
		})
	}
}
