package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementUnusedTextAddresses struct {
	Counts map[string]int
	Lines  map[string][]string
}

// A discarded definition owns only its plain source lines. Unrelated escapes
// do not make those lines live, and every raw claim of an address must be here.
func agreementUnusedTextAddressBudget(body string) agreementUnusedTextAddresses {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return agreementUnusedTextAddresses{}
	}
	addresses := make(map[string]int)
	lines := make(map[string][]string)
	for _, footnote := range unused {
		for node := footnote.FirstChild(); node != nil; node = node.NextSibling() {
			switch node.(type) {
			case *ast.Paragraph, *ast.TextBlock:
			default:
				return agreementUnusedTextAddresses{}
			}
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if _, ok := child.(*ast.Text); !ok {
					return agreementUnusedTextAddresses{}
				}
			}
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

func agreementUnusedTextAddressDifference(c agreementCase, f *agreementFailure, budget agreementUnusedTextAddresses) (kind, authority, wrong string) {
	kind, authority, wrong = agreementUnusedFootnoteAddressDifference(c, f, budget.Counts)
	if kind == "" {
		return "", "", ""
	}
	for line := range strings.SplitSeq(f.Cut, "\n") {
		for _, owned := range budget.Lines[f.Fragment] {
			if strings.TrimSuffix(line, "\r") == owned {
				return kind, authority, wrong
			}
		}
	}
	return "", "", ""
}

func TestAgreementUnusedTextAddresses(t *testing.T) {
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
		{name: "used and unused definition set", body: "ref[^n]\n\n[^n]: words ^a\n\n[^m]: other ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"[^m]: other ^b"}}}, public: true},
		{name: "used definition", body: "ref[^n]\n\n[^n]: words ^a\n"},
		{name: "ordinary address", body: "words ^a\n"},
		{name: "remaining owned set beside shared claim", body: "ordinary ^a\n\n[^n]: first ^a\nsecond ^b\n", want: agreementUnusedTextAddresses{Counts: map[string]int{"^b": 1}, Lines: map[string][]string{"^b": {"second ^b"}}}, public: true},
		{name: "shared live claim", body: "ordinary ^a\n\n[^n]: words ^a\n"},
		{name: "inline code owns text", body: "[^n]: `words` ^a\n"},
		{name: "inline markup owns text", body: "[^n]: *words* ^a\n"},
		{name: "HTML owns text", body: "[^n]: <mark>words</mark> ^a\n"},
		{name: "block code owns text", body: "[^n]: first\n\n    ```\n    words ^a\n    ```\n"},
		{name: "heading owns text", body: "[^n]: first ^a\n\n    ## heading ^b\n"},
		{name: "later nontext refuses whole inventory", body: "[^n]: first ^a\n\n[^m]: *words* ^b\n"},
		{name: "live percent role", body: "%%\n[^n]: words ^a\n%%\n"},
		{name: "live comment role", body: "<!--\n[^n]: words ^a\n-->\n"},
		{name: "empty definition", body: "[^n]: words\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementUnusedTextAddressBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete unused text address inventory (-want +got):\n%s", diff)
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
					t.Fatalf("caught: unused text address public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				matched++
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementUnusedTextAddressDifference(other, f, budget); kind != "" {
						t.Fatal("caught: unused text address borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Cut = "words" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = false }, func(f *agreementFailure) { f.ExcerptFound = false },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementUnusedTextAddressDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: unused text address borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			if matched != 2*len(budget.Counts) {
				t.Fatalf("caught: unused text address public set got=%d want=%d", matched, 2*len(budget.Counts))
			}
		})
	}
}
