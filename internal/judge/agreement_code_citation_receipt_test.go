package judge_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
)

// The complete code payload owns its extra citations even beside ordinary
// citations of the same file. Removing exactly that owned contribution must
// leave the page's occurrence count equal to the public check's count.
func agreementCodeCitationDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementCodePayload) (kind, authority, wrong string) {
	if c.Body != budget.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "page-only" || f.Tuple.Target == "" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	code := make(map[agreementCitation]int)
	for _, tuple := range actual.CodeCitations {
		code[tuple]++
	}
	if !cmp.Equal(budget.Tuples, code) || actual.CitationsInCode != len(actual.CodeCitations) {
		return "", "", ""
	}
	owned := 0
	for tuple, count := range budget.Tuples {
		if tuple.Target == f.Tuple.Target {
			owned += count
		}
	}
	if owned != f.Multiplicity {
		return "", "", ""
	}
	pageCount, checkCount := 0, 0
	for _, tuple := range actual.Citations {
		if tuple.SourceRole == "" && tuple.Target == f.Tuple.Target {
			pageCount++
		}
	}
	for _, target := range judge.LinkTargets(c.Body) {
		if target == f.Tuple.Target {
			checkCount++
		}
	}
	if pageCount-owned != checkCount {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "page"
}

func TestAgreementCodeCitationReceipt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "ordinary same target", body: "[[A]]\n\n`open\n[[A]]\nclose`\n", want: map[string]int{"A": 1}},
		{name: "all code sections and ordinary occurrences", body: "[[A#one]] [[A#two]]\n\n`open\n[[A#one]] [[A#two]] [[B|shown]]\nclose`\n", want: map[string]int{"A": 2, "B": 1}},
		{name: "every code payload", body: "[[A]]\n\n`first\n[[A]]\nend`\n\n`second\n[[A]] [[B]]\nlast`\n", want: map[string]int{"A": 2, "B": 1}},
		{name: "raw and peeled targets remain distinct", body: "[[A]]\n\n`open\n[[A\\]] [[A\\|shown]]\nclose`\n", want: map[string]int{"A\\": 1, "A": 1}},
		{name: "quoted fence", body: "[[A]]\n\n> ```\n> [[A]]\n> ```\n", want: map[string]int{"A": 1}},
		{name: "used footnote citation counts already agree", body: "ref[^n]\n\n[^n]: `open\n    [[A]]\n    close`\n"},
		{name: "literal code stays in the complete set", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\n`[[A]]`\n\n    [[A]]\n", want: map[string]int{"A": 1}},
		{name: "other excess cannot borrow code", body: "[[A\\]]\n\n`open\n[[A]]\nclose`\n"},
		{name: "grammar disagreement", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n"},
		{name: "hidden code", body: "[[A]]\n\n%%\n`open\n[[A]]\nclose`\n%%\n"},
		{name: "ordinary page agrees", body: "[[A]]\n"},
		{name: "literal page agrees", body: "`[[A]]`\n"},
		{name: "recorded original punctuation", body: "> [!unknown] title\n\\`[[image.png]]`open\n[[A]]\nclose`\t<!-- [[A]] -->", want: map[string]int{"A": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementCodeWindowBudget(c.Body, r.HTML)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementCodeCitationDifference(c, &f, &actual, budget)
				if f.Property != "P1" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: code citation borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: code citation public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementCodeCitationDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete code citation public receipts (-want +got):\n%s", diff)
			}
		})
	}
}

func agreementCodeCitationDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementCodePayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "words\n"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementCodeCitationDifference(other, f, actual, budget); kind != "" {
			t.Fatal("caught: code citation borrowed source or vault context")
		}
	}
	live := "[[" + f.Tuple.Target + "]]\n"
	if slices.Contains(judge.LinkTargets(live), f.Tuple.Target) {
		other := c
		other.Body += "\n" + live
		stale := budget
		stale.Body = other.Body
		if kind, _, _ := agreementCodeCitationDifference(other, f, actual, stale); kind != "" {
			t.Fatal("caught: code citation borrowed changed public check count")
		}
	}
	for _, change := range []func(*agreementFailure){
		func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
	} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementCodeCitationDifference(c, &changed, actual, budget); kind != "" {
			t.Fatalf("caught: code citation borrowed signature %s", agreementSignature(&changed))
		}
	}
	for _, carriers := range [][]agreementCitation{nil, append(append([]agreementCitation{}, actual.CodeCitations...), agreementCitation{Target: "unowned", State: "wikilink-broken"}), {{Target: f.Tuple.Target, Section: "unowned", State: "wikilink-broken"}}} {
		changed := *actual
		changed.CodeCitations = carriers
		if kind, _, _ := agreementCodeCitationDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: code citation borrowed incomplete code receipts")
		}
	}
	changed := *actual
	changed.CitationsInCode++
	if kind, _, _ := agreementCodeCitationDifference(c, f, &changed, budget); kind != "" {
		t.Fatal("caught: code citation borrowed another code carrier kind")
	}
	changed = *actual
	changed.Citations = append(append([]agreementCitation{}, actual.Citations...), agreementCitation{SourceRole: agreementOutsideMarkdown, Target: f.Tuple.Target, State: "wikilink-broken"})
	if kind, _, _ := agreementCodeCitationDifference(c, f, &changed, budget); kind != "debt" {
		t.Fatal("caught: code citation counted another page carrier kind")
	}
	for _, carriers := range [][]agreementCitation{nil, append(append([]agreementCitation{}, actual.Citations...), agreementCitation{Target: f.Tuple.Target, State: "wikilink-broken"})} {
		changed := *actual
		changed.Citations = carriers
		if kind, _, _ := agreementCodeCitationDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: code citation borrowed whole page count")
		}
	}
}
