package judge_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// An outer paragraph keeps its prose address when it ends a quoted fence.
// A quoted literal with the same caret belongs to code instead.
type agreementQuoteAddresses struct {
	Outer, Literal map[string]int
}

func agreementQuoteAddressOwnership(body string) agreementQuoteAddresses {
	if strings.Contains(body, "<") || strings.Contains(body, "[^") || strings.Contains(body, "\\") {
		return agreementQuoteAddresses{}
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementLiteralMarkers(source, doc) {
		return agreementQuoteAddresses{}
	}
	quotedFence := false
	literal := make(map[string]int)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := node.(*ast.FencedCodeBlock); !entering || !ok {
			return ast.WalkContinue, nil
		}
		for parent := node.Parent(); parent != nil; parent = parent.Parent() {
			if _, ok := parent.(*ast.Blockquote); ok {
				quotedFence = true
				for i := range node.Lines().Len() {
					line := node.Lines().At(i)
					if address := render.BlockAddress(strings.TrimSuffix(strings.TrimSuffix(string(line.Value(source)), "\n"), "\r")); address != "" {
						literal[graph.FoldFragment(address)]++
					}
				}
				break
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if !quotedFence {
		return agreementQuoteAddresses{}
	}
	addresses := make(map[string]int)
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if _, ok := node.(*ast.Paragraph); !ok {
			continue
		}
		for i := range node.Lines().Len() {
			line := node.Lines().At(i)
			if address := render.BlockAddress(strings.TrimSuffix(strings.TrimSuffix(string(line.Value(source)), "\n"), "\r")); address != "" {
				addresses[graph.FoldFragment(address)]++
			}
		}
	}
	all := make(map[string]int)
	for line := range strings.SplitSeq(body, "\n") {
		if address := render.BlockAddress(strings.TrimSuffix(line, "\r")); address != "" {
			all[graph.FoldFragment(address)]++
		}
	}
	for _, owned := range []map[string]int{addresses, literal} {
		for address, count := range owned {
			if all[address] != count {
				delete(owned, address)
			}
		}
	}
	return agreementQuoteAddresses{Outer: addresses, Literal: literal}
}

func agreementQuoteAddressDifference(c agreementCase, f *agreementFailure, addresses agreementQuoteAddresses) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P3" || f.Identity != "block-three-way" || f.Tuple != (agreementCitation{}) || f.Multiplicity != 1 || !f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Cut != "" {
		return "", "", ""
	}
	if f.Direction != "page-not-judge" && f.Direction != "page-not-excerpt" {
		return "", "", ""
	}
	if addresses.Outer[f.Fragment] > 0 {
		return "debt", "#1011 stage 3", "judge+excerpt"
	}
	if addresses.Literal[f.Fragment] > 0 {
		return "debt", "#1011 stage 5", "page"
	}
	return "", "", ""
}

func TestAgreementQuoteAddressDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
		literal    map[string]int
	}{
		{name: "outer prose ends empty quoted fence", body: "> ```\n^a\n", want: map[string]int{"^a": 1}},
		{name: "unrelated literal markers", body: "> ```\n> %%[!note]\n\n^a\n", want: map[string]int{"^a": 1}},
		{name: "outer prose follows quoted content", body: "> ```\n> code\n\n^a\n", want: map[string]int{"^a": 1}},
		{name: "closed quoted fence", body: "> ```\n> code\n> ```\n\n^a\n", want: map[string]int{"^a": 1}},
		{name: "folded outer address", body: "> ```\n^A\n", want: map[string]int{"^a": 1}},
		{name: "whole outer set", body: "> ```\n^a\n\nsecond ^b\n", want: map[string]int{"^a": 1, "^b": 1}},
		{name: "quoted literal owns address", body: "> ```\n> ^a\n", want: map[string]int{}, literal: map[string]int{"^a": 1}},
		{name: "closed quoted literal", body: "> ```\n> ^a\n> ```\n", want: map[string]int{}, literal: map[string]int{"^a": 1}},
		{name: "folded literal address", body: "> ```\n> ^A\n", want: map[string]int{}, literal: map[string]int{"^a": 1}},
		{name: "literal shares outer address", body: "> ```\n> ^a\n\n^a\n", want: map[string]int{}},
		{name: "ordinary prose", body: "^a\n"},
		{name: "ordinary fence", body: "```\n^a\n```\n"},
		{name: "comment role needs ownership", body: "%%\n> ```\n^a\n%%\n"},
		{name: "callout layout needs ownership", body: "> [!note] t\n> ```\n^a\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addresses := agreementQuoteAddressOwnership(tc.body)
			expected := agreementQuoteAddresses{Outer: tc.want}
			if tc.want != nil {
				expected.Literal = map[string]int{}
			}
			if tc.literal != nil {
				expected.Literal = tc.literal
			}
			if diff := cmp.Diff(expected, addresses); diff != "" {
				t.Fatalf("caught: quoted-fence address ownership inventory (-want +got):\n%s", diff)
			}
			fragments := []string{"^unrelated"}
			for fragment := range tc.want {
				fragments = append(fragments, fragment)
			}
			for fragment := range tc.literal {
				fragments = append(fragments, fragment)
			}
			slices.Sort(fragments)
			for _, fragment := range slices.Compact(fragments) {
				for _, direction := range []string{"page-not-judge", "page-not-excerpt"} {
					f := agreementFailure{Property: "P3", Identity: "block-three-way", Fragment: fragment, Direction: direction, Multiplicity: 1, PagePresent: true}
					kind, authority, wrong := agreementQuoteAddressDifference(agreementCase{Body: tc.body}, &f, addresses)
					wantAuthority, wantWrong := "", ""
					if tc.want[fragment] > 0 {
						wantAuthority, wantWrong = "#1011 stage 3", "judge+excerpt"
					} else if tc.literal[fragment] > 0 {
						wantAuthority, wantWrong = "#1011 stage 5", "page"
					}
					if wantWrong != "" && (kind != "debt" || authority != wantAuthority || wrong != wantWrong) || wantWrong == "" && kind != "" {
						t.Fatalf("caught: quoted-fence ownership kind=%q authority=%q wrong=%q", kind, authority, wrong)
					}
					for _, change := range []func(*agreementFailure){
						func(f *agreementFailure) { f.PagePresent = false },
						func(f *agreementFailure) { f.Fragment = "^unrelated" },
						func(f *agreementFailure) { f.Cut = "quoted ^a" },
						func(f *agreementFailure) { f.Multiplicity++ },
					} {
						changed := f
						change(&changed)
						if kind, _, _ := agreementQuoteAddressDifference(agreementCase{Body: tc.body}, &changed, addresses); kind != "" {
							t.Fatalf("caught: unrelated outer paragraph address delta admitted: %+v", changed)
						}
					}
				}
			}
		})
	}
}
