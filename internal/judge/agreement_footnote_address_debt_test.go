package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

var agreementFootnoteAddressesKey = parser.NewContextKey()

// A discarded definition cannot own a destination on the page. Every raw
// declaration of the same address must belong to that discarded region.
func agreementUnusedFootnoteAddresses(body string) map[string]int {
	if !strings.Contains(body, "[^") || strings.Contains(body, "<") || strings.Contains(body, "\\") {
		return nil
	}
	addresses := make(map[string]int)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	context.Set(agreementFootnoteAddressesKey, addresses)
	source := []byte(body)
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementLiteralMarkers(source, doc) {
		return nil
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
		}
	}
	return addresses
}

func agreementUnusedFootnoteAddressDifference(c agreementCase, f *agreementFailure, addresses map[string]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P3" || f.Identity != "block-three-way" || f.Tuple != (agreementCitation{}) || f.Multiplicity != 1 || f.PagePresent || !f.JudgeAccepted || !f.ExcerptFound || f.Cut == "" || !strings.Contains(c.Body, f.Cut) || addresses[f.Fragment] == 0 {
		return "", "", ""
	}
	if f.Direction != "judge-only" && f.Direction != "excerpt-only" {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "judge+excerpt"
}

func TestAgreementUnusedFootnoteAddressDebt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, cut string
		want            map[string]int
	}{
		{name: "unused first paragraph", body: "[^n]: text ^a\n", cut: "[^n]: text ^a", want: map[string]int{"^a": 1}},
		{name: "unrelated literal markers", body: "`%%[!note]`\n\n[^n]: text ^a\n", cut: "[^n]: text ^a", want: map[string]int{"^a": 1}},
		{name: "unused continuation", body: "[^n]: first\n\n    second ^a\n", cut: "    second ^a", want: map[string]int{"^a": 1}},
		{name: "folded declaration", body: "[^n]: text ^A\n", cut: "[^n]: text ^A", want: map[string]int{"^a": 1}},
		{name: "distinct address set", body: "[^n]: text ^a\n\n[^m]: other ^b\n", cut: "[^n]: text ^a", want: map[string]int{"^a": 1, "^b": 1}},
		{name: "used definition", body: "ref[^n]\n\n[^n]: text ^a\n", want: map[string]int{}},
		{name: "live declaration shares address", body: "ordinary ^a\n\n[^n]: text ^a\n", want: map[string]int{}},
		{name: "ordinary prose", body: "ordinary ^a\n"},
		{name: "comment role needs ownership", body: "%%\nref[^n]\n%%\n\n[^n]: text ^a\n"},
		{name: "callout layout needs ownership", body: "> [!note] ref[^n]\n\n[^n]: text ^a\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addresses := agreementUnusedFootnoteAddresses(tc.body)
			if diff := cmp.Diff(tc.want, addresses); diff != "" {
				t.Fatalf("caught: discarded definition address inventory (-want +got):\n%s", diff)
			}
			c := agreementCase{Body: tc.body}
			for _, direction := range []string{"judge-only", "excerpt-only"} {
				f := agreementFailure{Property: "P3", Identity: "block-three-way", Fragment: "^a", Direction: direction, Multiplicity: 1, JudgeAccepted: true, ExcerptFound: true, Cut: tc.cut}
				kind, authority, wrong := agreementUnusedFootnoteAddressDifference(c, &f, addresses)
				if tc.want["^a"] > 0 && (kind != "debt" || authority != "#1011 stage 5" || wrong != "judge+excerpt") || tc.want["^a"] == 0 && kind != "" {
					t.Fatalf("caught: unused definition address ownership kind=%q authority=%q wrong=%q", kind, authority, wrong)
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.PagePresent = true },
					func(f *agreementFailure) { f.Fragment = "^unrelated" },
					func(f *agreementFailure) { f.Cut = "not authored here" },
					func(f *agreementFailure) { f.Multiplicity++ },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementUnusedFootnoteAddressDifference(c, &changed, addresses); kind != "" {
						t.Fatalf("caught: unrelated discarded-definition address delta admitted: %+v", changed)
					}
				}
			}
		})
	}
}
