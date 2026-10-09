package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// The unused definition is discarded with every block it owns. Native block
// lines retain the source boundaries without borrowing live address claims.
func agreementUnusedBlockDeclarations(body string, grammar goldmark.Markdown) (source []byte, doc ast.Node, nodes []ast.Node) {
	source = []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	doc = grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))

	for _, footnote := range unused {
		if err := ast.Walk(footnote, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering && node.Type() == ast.TypeBlock && node.Lines().Len() > 0 {
				nodes = append(nodes, node)
			}
			return ast.WalkContinue, nil
		}); err != nil {
			panic(err)
		}
	}
	return source, doc, nodes
}

func agreementUnusedBlockMarkerContext(source []byte, doc ast.Node, nodes []ast.Node) bool {
	contextSource := bytes.Clone(source)
	// Discarded blocks cannot supply a live opener. Context checks retain the
	// original tree and offsets for every marker outside those blocks.
	for _, node := range nodes {
		for i := range node.Lines().Len() {
			line := node.Lines().At(i)
			for at := line.Start; at+1 < line.Stop; at++ {
				if source[at] == '[' && source[at+1] == '!' {
					contextSource[at], contextSource[at+1] = ' ', ' '
				}
			}
		}
	}

	return agreementPlainOpenerDeclarationMarkers(contextSource, doc)
}

func agreementUnusedBlockAddressReading(body string, grammar goldmark.Markdown) agreementUnusedTextAddresses {
	source, doc, nodes := agreementUnusedBlockDeclarations(body, grammar)
	addresses := make(map[string]int)
	lines := make(map[string][]string)
	for _, node := range nodes {
		for i := range node.Lines().Len() {
			line := node.Lines().At(i)
			raw := strings.TrimSuffix(strings.TrimSuffix(string(line.Value(source)), "\n"), "\r")
			if address := render.BlockAddress(raw); address != "" {
				key := graph.FoldFragment(address)
				addresses[key]++
				start := bytes.LastIndexByte(source[:line.Start], '\n') + 1
				physical := strings.TrimSuffix(strings.TrimSuffix(string(source[start:line.Stop]), "\n"), "\r")
				lines[key] = append(lines[key], physical)
			}
		}
	}

	if !agreementUnusedBlockMarkerContext(source, doc, nodes) {
		return agreementUnusedTextAddresses{}
	}
	all := make(map[string]int)
	for line := range strings.SplitSeq(body, "\n") {
		if address := render.BlockAddress(strings.TrimSuffix(line, "\r")); address != "" {
			all[graph.FoldFragment(address)]++
		}
	}
	for address, count := range addresses {
		if all[address] != count {
			delete(addresses, address)
			delete(lines, address)
		}
	}
	if len(addresses) == 0 {
		return agreementUnusedTextAddresses{}
	}
	return agreementUnusedTextAddresses{Counts: addresses, Lines: lines}
}

func agreementUnusedBlockAddressBudget(body string) agreementUnusedTextAddresses {
	plain := agreementUnusedBlockAddressReading(body, agreementFootnoteGrammar)
	gfm := agreementUnusedBlockAddressReading(body, agreementExclusiveCodeGrammar)
	if !cmp.Equal(plain, gfm) {
		return agreementUnusedTextAddresses{}
	}
	return plain
}

func TestAgreementUnusedBlockAddresses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementUnusedTextAddresses
		public     bool
	}{
		{name: "recorded discarded block set", body: "## A\n> > ````\n[[A]] [[A]][[A\\|alias]]## [[A|alias]]\n[[image.png]][[A#^a]]A\n===\n0~~~\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[^unused]: [[A]]\n[[image.png]] ^A\n\t```\n    ![[A]]`[[A]]`    ```\n[[A#^a]]    ```\n[^unused]: [[A]]\nhttps://example.invalid/`[[A]]` > > ", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[[image.png]] ^A"}}}, public: true},
		{name: "marker at block start", body: "[^n]: [!note] words ^a\n\n    ## heading\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: [!note] words ^a"}}}, public: true},
		{name: "marker at block end", body: "[^n]: words ^a\n\n    ## heading\n\n    [!", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: true},
		{name: "heading beside discarded address", body: "[^n]: words ^a\n\n    ## heading\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: true},
		{name: "table beside discarded address", body: "[^n]: words ^a\n\n    | a | b |\n    |---|---|\n    | [[A]] | text |\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: true},
		{name: "fence beside discarded address", body: "[^n]: words ^a\n\n    ```\n    [[A]]\n    ```\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}}}, public: true},
		{name: "complete blocks and definitions", body: "[^n]: first ^A\nsecond ^a\n\n    ## heading ^b\n\n    ```\n    words ^c\n    ```\n\n[^m]: other ^d\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 2, "^b": 1, "^c": 1, "^d": 1}, Lines: map[string][]string{"^a": {"[^n]: first ^A", "second ^a"}, "^b": {"    ## heading ^b"}, "^c": {"    words ^c"}, "^d": {"[^m]: other ^d"}}}},
		{name: "indented block source", body: "[^n]: words ^a\n\n        code ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1, "^b": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}, "^b": {"        code ^b"}}}},
		{name: "nested list and quote source", body: "[^n]: words ^a\n\n    - words ^b\n\n    > words ^c\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1, "^b": 1, "^c": 1}, Lines: map[string][]string{"^a": {"[^n]: words ^a"}, "^b": {"    - words ^b"}, "^c": {"    > words ^c"}}}},
		{name: "discarded marker and later block", body: "[^n]: words [!note] ^a\n\n    ## heading\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^a": 1}, Lines: map[string][]string{"^a": {"[^n]: words [!note] ^a"}}}, public: true},
		{name: "used and unused blocks", body: "ref[^u]\n\n[^u]: words ^a\n\n    ## heading\n\n[^n]: other ^b\n\n    ```\n    [[A]]\n    ```\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"[^n]: other ^b"}}}, public: true},
		{name: "remaining address beside shared claim", body: "ordinary ^a\n\n[^n]: words ^a\n\n    ## heading ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"    ## heading ^b"}}}},
		{name: "used blocks", body: "ref[^n]\n\n[^n]: words ^a\n\n    ## heading ^b\n"},
		{name: "outside blocks", body: "words ^a\n\n## heading ^b\n"},
		{name: "grammars disagree on use", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: words ^a\n\n    ## heading\n"},
		{name: "live marker outside", body: "> [!note] [[A]]\n\n[^n]: words ^a\n\n    ## heading\n"},
		{name: "comment role remains outside", body: "%%\n[^n]: words ^a\n\n    ## heading\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementUnusedBlockAddressBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete unused block address inventory (-want +got):\n%s", diff)
			}
			if tc.want.Counts != nil && agreementUnusedOpenerAddressBudget(tc.body).Counts != nil {
				t.Fatal("caught: paragraph profile borrowed other block scope")
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
					t.Fatalf("caught: unused block address public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				matched++
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementUnusedTextAddressDifference(other, f, budget); kind != "" {
						t.Fatal("caught: unused block address borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Cut = "words" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = false }, func(f *agreementFailure) { f.ExcerptFound = false },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementUnusedTextAddressDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: unused block address borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			if matched != 2*len(budget.Counts) {
				t.Fatalf("caught: unused block address public set got=%d want=%d", matched, 2*len(budget.Counts))
			}
		})
	}
}
