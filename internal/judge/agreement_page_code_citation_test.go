package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAgreementPageCodeCitationReceipt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "ordinary target and URL", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n", want: map[string]int{"A": 1}},
		{name: "URL before code", body: "https://example.invalid/`[[A]]`\n\n`open\n[[A]]\nclose`\n", want: map[string]int{"A": 1}},
		{name: "every target and code contribution", body: "https://example.invalid/`[[A]]`\n\n`first\n[[A]] [[B]]\nend`\n\n`second\n[[A]]\nlast`\n", want: map[string]int{"A": 2, "B": 1}},
		{name: "raw and peeled fields", body: "[[A]]\n\n`open\n[[A\\]] [[A\\|shown]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n", want: map[string]int{"A\\": 1, "A": 1}},
		{name: "every section of one target", body: "[[A#ordinary]]\n\n`open\n[[A#one]] [[A#two]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n", want: map[string]int{"A": 2}},
		{name: "literal code remains in full set", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\n`[[A]]`\n\n    [[A]]\n\nhttps://example.invalid/`[[A]]`\n", want: map[string]int{"A": 1}},
		{name: "recorded complete container source", body: "1.  ^a\nhttps://example.invalid/`[[A]]` > [!note] title\n  - ref[^n]\n\t```\n`open\n[[A]]\nclose`[[A]]", want: map[string]int{"A": 1}},
		{name: "same grammar retains older ownership", body: "[[A]]\n\n`open\n[[A]]\nclose`\n"},
		{name: "other excess cannot borrow code", body: "[[A\\]]\n\n`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n"},
		{name: "used footnote counts already agree", body: "ref[^n]\n\n[^n]: `open\n    [[A]]\n    close`\n\nhttps://example.invalid/`[[A]]`\n"},
		{name: "hidden code lacks complete page set", body: "https://example.invalid/`[[A]]`\n\n%%\n`open\n[[A]]\nclose`\n%%\n"},
		{name: "URL alone has no code receipt", body: "https://example.invalid/`[[A]]`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementPageCodeWindowBudget(c.Body, r.HTML)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementCodeCitationDifference(c, &f, &actual, budget)
				if f.Property != "P1" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: page code citation borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: page code citation public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementCodeCitationDrift(t, c, &f, &actual, budget)
				if kind, _, _ := agreementCodeCitationDifference(c, &f, &actual, agreementCodeWindowBudget(c.Body, r.HTML)); kind != "" {
					t.Fatal("caught: earlier code citation borrowed another grammar")
				}
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete page code citation public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
