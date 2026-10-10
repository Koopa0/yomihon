package judge_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// An address is declared at the physical line's tail. A cell's apparent tail
// cannot add another declaration, and both grammars must discard each raw claim.
func agreementDiscardedPhysicalAddressBudget(body string) agreementUnusedTextAddresses {
	_, _, plainNodes := agreementUnusedBlockDeclarations(body, agreementFootnoteGrammar)
	_, _, gfmNodes := agreementUnusedBlockDeclarations(body, agreementExclusiveCodeGrammar)
	plain, gfm := agreementUnusedBlockSourceLines(plainNodes), agreementUnusedBlockSourceLines(gfmNodes)
	all, counts := make(map[string]int), make(map[string]int)
	lines := make(map[string][]string)
	for off := 0; off < len(body); {
		line, _, _ := strings.Cut(body[off:], "\n")
		raw := strings.TrimSuffix(line, "\r")
		address := render.BlockAddress(raw)
		if address != "" {
			key := graph.FoldFragment(address)
			all[key]++
			at := off + strings.LastIndex(raw, address)
			if agreementFieldContained(plain, at, at+len(address)) && agreementFieldContained(gfm, at, at+len(address)) {
				counts[key]++
				lines[key] = append(lines[key], raw)
			}
		}
		off += len(line) + 1
	}
	for address, count := range counts {
		if all[address] != count {
			delete(counts, address)
			delete(lines, address)
		}
	}
	if len(counts) == 0 {
		return agreementUnusedTextAddresses{}
	}
	return agreementUnusedTextAddresses{Counts: counts, Lines: lines}
}

// The public cut may omit an independent comment. Its exact receipt still
// has to show an original physical line belonging to the discarded address.
func agreementPhysicalDiscardedAddressDifference(c agreementCase, f *agreementFailure, budget agreementUnusedTextAddresses) (kind, authority, wrong string) {
	if !agreementUnusedFootnoteAddressSignature(c, f, budget.Counts) {
		return "", "", ""
	}
	cut, found := render.Excerpt(c.Body, f.Fragment)
	if !found || cut != f.Cut {
		return "", "", ""
	}
	for line := range strings.SplitSeq(f.Cut, "\n") {
		for _, owned := range budget.Lines[f.Fragment] {
			if strings.TrimSuffix(line, "\r") == owned {
				return "debt", "#1011 stage 5", "judge+excerpt"
			}
		}
	}
	return "", "", ""
}

func TestAgreementPhysicalDiscardedAddresses(t *testing.T) {
	t.Parallel()
	t.Run("rewritten owned line cannot borrow raw receipt", func(t *testing.T) {
		t.Parallel()
		c := agreementCase{Body: "[^n]: <!--hidden--> words ^a\n"}
		budget := agreementDiscardedPhysicalAddressBudget(c.Body)
		want := agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: <!--hidden--> words ^a"}}}
		if diff := cmp.Diff(want, budget); diff != "" {
			t.Fatalf("caught: rewritten physical source inventory (-want +got):\n%s", diff)
		}
		_, actual := agreementIsolatedPage(t, c)
		found := make(map[string][]string)
		for _, f := range agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})[0] {
			if f.Property != "P3" {
				continue
			}
			if !agreementUnusedFootnoteAddressSignature(c, &f, budget.Counts) || f.Cut != "[^n]:  words ^a" {
				t.Fatalf("caught: rewritten physical public receipt %s", agreementSignature(&f))
			}
			if kind, _, _ := agreementPhysicalDiscardedAddressDifference(c, &f, budget); kind != "" {
				t.Fatal("caught: physical discarded address borrowed rewritten owned line")
			}
			found[f.Fragment] = append(found[f.Fragment], f.Direction)
		}
		for fragment := range found {
			slices.Sort(found[fragment])
		}
		if diff := cmp.Diff(map[string][]string{"^a": {"excerpt-only", "judge-only"}}, found); diff != "" {
			t.Fatalf("caught: complete rewritten physical public inventory (-want +got):\n%s", diff)
		}
	})

	for _, tc := range []struct {
		name, body    string
		want          agreementUnusedTextAddresses
		public        map[string][]string
		oldRefuses    bool
		oldCutRefuses bool
	}{
		{name: "recorded exact noncontiguous public cut", body: "1.  ^é\n[^unused]: [[A]]\n ^A\n    ```\nA\n  ## A\n## A\n![[image.png]]0> > [[A]] [[A]]<!--\t```\n-->[[#A]]> > ", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {" ^A"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}, oldCutRefuses: true},
		{name: "recorded physical address beside table", body: "[^unused]: [[A]]\n ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- [ ] [[A]]\n> [!unknown] title\n ```\n> > ", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {" ^a"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}, oldRefuses: true},
		{name: "cell tail is no physical declaration", body: "[^n]: words\n\n    | a | b |\n    |---|---|\n    | row | ^a |\n", public: map[string][]string{}},
		{name: "table gap cannot borrow source ownership", body: "[^n]: first ^a\n\n    | a | b |\n    |---|---|\n    | row | value | ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^a"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}},
		{name: "whole token fits a final cell", body: "[^n]: first ^a\n\n    | a | b |\n    |---|---|\n    | row | ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1, "^b": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^a"}, "^b": {"    | row | ^b"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}},
		{name: "all physical rows and definitions", body: "[^n]: first ^A\nsecond ^a\n\n    ## heading ^b\n\n    ```\n    words ^c\n    ```\n\n[^m]: other ^d\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 2, "^b": 1, "^c": 1, "^d": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^A", "second ^a"}, "^b": {"    ## heading ^b"}, "^c": {"    words ^c"}, "^d": {"[^m]: other ^d"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}, "^b": {"excerpt-only", "judge-only"}, "^d": {"excerpt-only", "judge-only"}}},
		{name: "nested quote physical prefixes", body: "[^n]: first ^a\n\n    > words ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1, "^b": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^a"}, "^b": {"    > words ^b"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}, "^b": {"excerpt-only", "judge-only"}}},

		{name: "token starts the owned segment", body: "[^n]: ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: ^a"}}}},
		{name: "tail differs from definition label", body: "[^a]: words ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^a]: words ^a"}}}},
		{name: "later live claim removes all folded ownership", body: "[^n]: first ^a\nnext ^b\n\nordinary ^A\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"next ^b"}}}, public: map[string][]string{"^b": {"excerpt-only", "judge-only"}}},
		{name: "source EOF without line ending", body: "[^n]: words ^a", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}},
		{name: "spaces follow the raw token", body: "[^n]: words ^a  \n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a  "}}}},
		{name: "carriage return preserves physical inventory", body: "> [!note] [[A]]\r\n\r\n[^n]: words ^a\r\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: map[string][]string{}},
		{name: "outside markup cannot claim another address", body: "> [!note] [[A]]\n\n[^n]: words ^a\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: map[string][]string{"^a": {"excerpt-only", "judge-only"}}},
		{name: "live raw claim removes the complete target", body: "ordinary ^a\n\n[^n]: first ^A\nsecond ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"second ^b"}}}, public: map[string][]string{"^b": {"excerpt-only", "judge-only"}}},
		{name: "used and unused definitions", body: "ref[^u]\n\n[^u]: words ^a\n\n[^n]: other ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"[^n]: other ^b"}}}, public: map[string][]string{"^b": {"excerpt-only", "judge-only"}}},
		{name: "common grammar uses the definition", body: "https://example.invalid/ref[^n]\n\n[^n]: words ^a\n", public: map[string][]string{}},
		{name: "page grammar uses the definition", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words ^a\n", public: map[string][]string{}},
		{name: "used address has no discarded owner", body: "ref[^n]\n\n[^n]: words ^a\n", public: map[string][]string{}},
		{name: "ordinary address has no discarded owner", body: "words ^a\n", public: map[string][]string{}},
		{name: "unsupported physical token cannot borrow valid ones", body: "[^n]: words ^a\nnext ^é\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementDiscardedPhysicalAddressBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete physical discarded address inventory (-want +got):\n%s", diff)
			}
			if tc.oldRefuses && agreementDiscardedBlockAddressBudget(tc.body).Counts != nil {
				t.Fatal("caught: earlier discarded block reader borrowed physical scope")
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
					if kind, _, _ := agreementPhysicalDiscardedAddressDifference(c, &f, budget); kind != "" {
						t.Fatal("caught: physical discarded address borrowed an unowned public fragment")
					}
					continue
				}
				kind, authority, wrong := agreementPhysicalDiscardedAddressDifference(c, &f, budget)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "judge+excerpt" {
					t.Fatalf("caught: physical discarded address public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				found[f.Fragment] = append(found[f.Fragment], f.Direction)
				if tc.oldCutRefuses {
					if strings.Contains(c.Body, f.Cut) {
						t.Fatal("caught: noncontiguous control became contiguous")
					}
					if kind, _, _ := agreementUnusedTextAddressDifference(c, &f, budget); kind != "" {
						t.Fatal("caught: earlier unused receipt borrowed noncontiguous cut")
					}
				}
				changedCut := f
				changedCut.Cut += "\n" + budget.Lines[f.Fragment][0]
				if kind, _, _ := agreementPhysicalDiscardedAddressDifference(c, &changedCut, budget); kind != "" {
					t.Fatal("caught: physical discarded address borrowed changed public cut")
				}

				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
					if kind, _, _ := agreementPhysicalDiscardedAddressDifference(other, &f, budget); kind != "" {
						t.Fatal("caught: physical discarded address borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Cut = "words" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = false }, func(f *agreementFailure) { f.ExcerptFound = false },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementPhysicalDiscardedAddressDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: physical discarded address borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			for fragment := range found {
				slices.Sort(found[fragment])
			}
			if diff := cmp.Diff(tc.public, found); diff != "" {
				t.Fatalf("caught: complete physical discarded address public set (-want +got):\n%s", diff)
			}
		})
	}
}
