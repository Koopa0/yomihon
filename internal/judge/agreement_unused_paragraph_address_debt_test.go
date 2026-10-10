package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Formatting does not make a discarded definition live. Both grammars must
// agree on every owned source line and on which definitions are unused.
func agreementUnusedParagraphAddressBudget(body string) agreementUnusedTextAddresses {
	plain := agreementUnusedAddressProfile(body, agreementFootnoteGrammar, false)
	gfm := agreementUnusedAddressProfile(body, agreementExclusiveCodeGrammar, false)
	if !cmp.Equal(plain, gfm) {
		return agreementUnusedTextAddresses{}
	}
	return plain
}
func TestAgreementUnusedParagraphAddresses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedTextAddresses
		public     bool
	}{
		{name: "unrelated escape", body: "\\[[A]]\n\n[^unused]: words ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^unused]: words ^a"}}}, public: true},
		{name: "complete lines and definitions", body: "[^n]: first ^A\nnext ^a\n\n    second ^b\n\n[^m]: third ^c\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 2, "^b": 1, "^c": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^A", "next ^a"}, "^b": {"    second ^b"}, "^c": {"[^m]: third ^c"}}}, public: true},
		{name: "unrelated HTML words", body: "## <em>A</em>\n\n[^unused]: words ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^unused]: words ^a"}}}, public: true},
		{name: "recorded lazy continuation", body: "## A\n[^n]: [[A]]\n\n    [[B]]\n ^a-2\n\\[[A]]- [ ] [[A]]\n## A\n## A\n-->B\n``` [[A]]\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a-2": 1}, Lines: map[string][]string{"^a-2": {" ^a-2"}}}, public: true},
		{name: "recorded formatted lazy paragraph", body: "[^unused]: [[A]]\n章節\nA\n[[B|alias]]`[[A]]`[[A\nB]]# A\n0  ^a\n  - ## <em>A</em>\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"0  ^a"}}}, public: true},
		{name: "used and unused definition set", body: "ref[^n]\n\n[^n]: words ^a\n\n[^m]: other ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"[^m]: other ^b"}}}, public: true},
		{name: "used definition", body: "ref[^n]\n\n[^n]: words ^a\n"},
		{name: "ordinary address", body: "words ^a\n"},
		{name: "remaining owned set beside shared claim", body: "ordinary ^a\n\n[^n]: first ^a\nsecond ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"second ^b"}}}, public: true},
		{name: "shared live claim", body: "ordinary ^a\n\n[^n]: words ^a\n"},
		{name: "inline code owns text", body: "[^n]: `words` ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: `words` ^a"}}}, public: true},
		{name: "inline markup owns text", body: "[^n]: *words* ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: *words* ^a"}}}, public: true},
		{name: "HTML owns text", body: "[^n]: <mark>words</mark> ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: <mark>words</mark> ^a"}}}, public: true},
		{name: "block code owns text", body: "[^n]: first\n\n    ```\n    words ^a\n    ```\n"},
		{name: "heading owns text", body: "[^n]: first ^a\n\n    ## heading ^b\n"},
		{name: "complete mixed inline inventory", body: "[^n]: first ^A\nsecond `words` ^a\n\n    *next* ^b\n\n[^m]: <mark>words</mark> ^c\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 2, "^b": 1, "^c": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^A", "second `words` ^a"}, "^b": {"    *next* ^b"}, "^c": {"[^m]: <mark>words</mark> ^c"}}}, public: true},
		{name: "used and unused markup", body: "ref[^n]\n\n[^n]: `used` ^a\n\n[^m]: *unused* ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"[^m]: *unused* ^b"}}}, public: true},
		{name: "grammars disagree on use", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: *words* ^a\n"},
		{name: "live percent role", body: "%%\n[^n]: words ^a\n%%\n"},
		{name: "live comment role", body: "<!--\n[^n]: words ^a\n-->\n"},
		{name: "empty definition", body: "[^n]: words\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementUnusedParagraphAddressBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete unused paragraph address inventory (-want +got):\n%s", diff)
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
					t.Fatalf("caught: unused paragraph address public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				matched++
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementUnusedTextAddressDifference(other, f, budget); kind != "" {
						t.Fatal("caught: unused paragraph address borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Cut = "words" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = false }, func(f *agreementFailure) { f.ExcerptFound = false },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementUnusedTextAddressDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: unused paragraph address borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			if matched != 2*len(budget.Counts) {
				t.Fatalf("caught: unused paragraph address public set got=%d want=%d", matched, 2*len(budget.Counts))
			}
		})
	}
}
