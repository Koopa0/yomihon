package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Linkify can own backticks that CommonMark reads as code. The page grammar's
// entire original code set must still match every actual page payload; a code
// declaration from the other reading cannot fill a missing page declaration.
func agreementPageCodeWindowBudget(body, raw string) agreementCodePayload {
	plain, plainOK := agreementNativeCodeWindows(body, agreementFootnoteGrammar)
	gfm, gfmOK := agreementNativeCodeWindows(body, agreementExclusiveCodeGrammar)
	if !plainOK || !gfmOK || cmp.Equal(plain, gfm) {
		return agreementCodePayload{}
	}
	return agreementDeclaredCodePayload(body, raw, gfm)
}

func TestAgreementPageCodePayload(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
	}{
		{name: "different URL code set", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n", want: map[agreementCitation]int{a: 1}},
		{name: "URL preceding complete code set", body: "https://example.invalid/`[[A]]`\n\n`open\n[[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}},
		{name: "every code contribution", body: "https://example.invalid/`[[A]]`\n\n`first\n[[A]] [[B]]\nend`\n\n`second\n[[A]]\nlast`\n", want: map[agreementCitation]int{a: 2, b: 1}},
		{name: "ordinary literal owners remain complete", body: "https://example.invalid/`[[A]]`\n\n`open\n[[A]]\nclose`\n\n`[[A]]`\n\n    [[B]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "recorded full source", body: "- [x] [[B]]\n ^a\n`[[A\\|alias]]--> ^é\n`https://example.invalid/`[[A]]`  ^é\n%%[[#A]]", want: map[agreementCitation]int{a: 1}},
		{name: "same readings keep earlier ownership", body: "[[A]]\n\n`open\n[[A]]\nclose`\n"},
		{name: "URL alone contributes no page code", body: "https://example.invalid/`[[A]]`\n"},
		{name: "different set without a code citation", body: "`words`\n\nhttps://example.invalid/`[[A]]`\n"},
		{name: "hidden declaration has no page owner", body: "https://example.invalid/`[[A]]`\n\n%%\n`open\n[[A]]\nclose`\n%%\n"},
		{name: "embed retains earlier refusal", body: "https://example.invalid/`[[A]]`\n\n`open\n![[A]]\nclose`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementPageCodeWindowBudget(c.Body, r.HTML)
			want := agreementCodePayload{}
			if len(tc.want) != 0 {
				want = agreementCodePayload{Body: c.Body, Tuples: tc.want}
			}
			if diff := cmp.Diff(want, budget); diff != "" {
				t.Fatalf("caught: complete page grammar code payload inventory (-want +got):\n%s", diff)
			}
			if len(tc.want) != 0 && agreementCodeWindowBudget(c.Body, r.HTML).Tuples != nil {
				t.Fatal("caught: earlier code payload borrowed another grammar")
			}
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				if f.Property != "P2" {
					continue
				}
				kind, authority, wrong := agreementCodeWindowDifference(c, &f, &actual, budget)
				if tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: page code payload borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: page code payload public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementCodePayloadDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete page grammar code public receipts (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementPageCodePayloadDrift(t *testing.T) {
	t.Parallel()
	const body = "[[A]]\n\n`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n"
	r, _ := agreementIsolatedPage(t, agreementCase{Body: body})
	if agreementPageCodeWindowBudget(body, r.HTML).Tuples == nil {
		t.Fatal("caught: page code payload drift lacks an owned baseline")
	}
	if strings.Count(r.HTML, " close</code>") != 1 || strings.Count(r.HTML, "<code>") != 1 {
		t.Fatal("page code drift does not identify one declared site")
	}
	for _, changed := range []string{
		r.HTML + "<code>[[A]]</code>",
		r.HTML + "<code>unowned</code>",
		strings.Replace(r.HTML, " close</code>", " missing</code>", 1),
		strings.Replace(r.HTML, " close</code>", "</code>", 1),
		strings.Replace(r.HTML, "<code>", "<span>", 1),
	} {
		if agreementPageCodeWindowBudget(body, changed).Tuples != nil {
			t.Fatal("caught: page code payload borrowed incomplete or extra page declaration")
		}
	}
}
