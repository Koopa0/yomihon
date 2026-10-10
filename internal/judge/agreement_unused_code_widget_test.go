package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A discarded declaration still separates literal fields from ordinary words.
// Diagnostic counts may use only their own source roles and complete tuples.
func agreementUnusedCodeWidgetBudget(body string) agreementUnusedWidgetPayload {
	plain := agreementUnusedCodeCitationReading(body, agreementFootnoteGrammar)
	gfm := agreementUnusedCodeCitationReading(body, agreementExclusiveCodeGrammar)
	if len(plain.Citation.Fields) == 0 || !cmp.Equal(plain, gfm) {
		return agreementUnusedWidgetPayload{}
	}
	budget := make(map[agreementCitation]int)
	for _, field := range plain.Citation.Fields {
		budget[field.Tuple]++
	}
	return agreementUnusedWidgetPayload{Body: body, Tuples: budget}
}

func TestAgreementUnusedCodeWidgets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body        string
		budget, old, want map[agreementCitation]int
	}{
		{name: "shared live and every literal role", body: "[[A]]\n\n[^n]: words [[A]] `[[B]]` [[A]]\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {SourceRole: "unused-inline-code", Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2}},
		{name: "all visible counts remain ordinary", body: "[[A]] [[A]] [[B]]\n\n[^n]: words [[A]] [[B]] `code`\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1}},
		{name: "all definitions and paragraphs", body: "[^n]: [[A]] `[[B]]`\n\n    more [[A]]\n\n[^m]: `[[B]]` [[A]]\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 3, {SourceRole: "unused-inline-code", Target: "B", State: "wikilink-broken"}: 2}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 3}},
		{name: "code alone cannot lend a diagnostic", body: "[^n]: `[[A]]`\n", budget: map[agreementCitation]int{{SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: nil},
		{name: "literal field cannot lend missing reference diagnostic", body: "[r]: [[A]]\n\n[^n]: `[[A]]`\n", budget: map[agreementCitation]int{{SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: nil},
		{name: "prose count cannot lend missing reference diagnostic", body: "[r]: [[A]]\n\n[^n]: [[A]] `code`\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: nil},
		{name: "later code retains indentation and diagnostic role", body: "[^n]: [[A]]\n\n    `[[B]]` [[A]]\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {SourceRole: "unused-inline-code", Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2}},
		{name: "every wrapped code piece stays literal", body: "[^n]: [[A]] `open\n[[B]]\nclose` [[A]]\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {SourceRole: "unused-inline-code", Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2}},
		{name: "CRLF keeps every source role", body: "[^n]: [[A]] `open\r\n[[B]]\r\nclose` [[A]]\r\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {SourceRole: "unused-inline-code", Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2}},
		{name: "raw aliases sections suffixes and embeds", body: "[^n]: words ![[A#part|shown]] [[A\\]] `[[B]]`\n", budget: map[agreementCitation]int{{Target: "A", Section: "part", State: "wikilink-broken"}: 1, {Target: "A\\", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", Section: "part", State: "wikilink-broken"}: 1, {Target: "A\\", State: "wikilink-broken"}: 1}},
		{name: "raw entity spelling stays authored", body: "[^n]: 純 [[A&amp;B|shown]] `[[B]]`\n", budget: map[agreementCitation]int{{Target: "A&amp;B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A&amp;B", State: "wikilink-broken"}: 1}},
		{name: "all code contexts protect percent markers", body: "`%%`\n\n[^n]: [[A]] `%%`\n[[B]]\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1}},
		{name: "fence body and info retain native code context", body: "``` %%\ntext\n```\n\n[^n]: [[A]] `code`\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}},
		{name: "fieldless code stays without tuples", body: "[^n]: `words`\n", budget: nil, old: nil, want: nil},
		{name: "plain source keeps prior ownership", body: "[^n]: words [[A]]\n", budget: nil, old: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}, want: nil},
		{name: "used source stays visible", body: "ref[^n]\n\n[^n]: [[A]] `[[B]]`\n", budget: nil, old: nil, want: nil},
		{name: "ordinary code has no discarded owner", body: "[[A]] `[[B]]`\n", budget: nil, old: nil, want: nil},
		{name: "CommonMark alone assigns the reference", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A]] `code`\n"},
		{name: "grammar dependent use refuses", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]] `code`\n", budget: nil, old: nil, want: nil},
		{name: "URL owner refuses whole source", body: "[^n]: [[A]] https://example.invalid/[[B]] `code`\n", budget: nil, old: nil, want: nil},
		{name: "other inline owner refuses", body: "[^n]: [[A]] *[[B]]* `code`\n", budget: nil, old: nil, want: nil},
		{name: "other block owner refuses", body: "[^n]: [[A]] `code`\n\n    ## [[B]]\n", budget: nil, old: nil, want: nil},
		{name: "comment overlap refuses all source", body: "[^n]: [[A]] `code` %%[[B]]%%\n", budget: nil, old: nil, want: nil},
		{name: "hidden field cannot lend diagnostic", body: "%%\n[^n]: [[A]] `code`\n%%\n\n[r]: [[A]]\n", budget: nil, old: nil, want: nil},
		{name: "incomplete field refuses all source", body: "[^n]: [[A]] `code` [[B\n", budget: nil, old: nil, want: nil},
		{name: "nested field refuses all source", body: "[^n]: [[A]] `code` [[B[[C]]]]\n", budget: nil, old: nil, want: nil},
		{name: "cross line field refuses all source", body: "[^n]: [[A]] `code` [[B\nC]]\n", budget: nil, old: nil, want: nil},
		{name: "escaped field refuses all source", body: "[^n]: [[A]] `code` \\[[B]]\n", budget: nil, old: nil, want: nil},
		{name: "local field refuses all source", body: "[^n]: [[A]] `code` [[#B]]\n", budget: nil, old: nil, want: nil},
		{name: "block field refuses all source", body: "[^n]: [[A]] `code` [[B#^b]]\n", budget: nil, old: nil, want: nil},
		{name: "full original native unused code diagnostic 0", body: "[^unused]: [[A]]\n``open\n[[A]]\nclose`0  > [!note] title\n`open\n[[A]]\nclose````````\n ^A\nÉ\n<div>\n[[A]]\n</div>\nA\n---\n[[A]]## A\n ^A\n   ```\n`~~~~\n ^a-2\n  > [!note] title\nA\n===\n> [!unknown] title\n<!--A\n---\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 3}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 3}},
		{name: "full original native unused code diagnostic 1", body: "\\\\[[A]]    ```\n> [!note] title\nA\n===\n%%[[A]]%%## [[A|alias]]\n  - 01. [[B|alias]]1. ```` go [[A]]\n[^n]: [[A]]\n\n    [[B]]\n`open\n[[A]]\nclose`\t```\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "B", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 2", body: "%%[[A]]%%# A\n  > [!note] title\n```\n## A\n- [ ] [[A]]\n## !\n\n\\[[A]]ref[^n]\n```\n ^a-2\n [^n]: [[A]]\n\n    [[B]]\n ^a-2\n`[[A]]`| a | b |\n|---|---|\n| [[A]] | ^a |\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 3", body: "[^n]: [[A]]\n\n    [[B]]\n章節\n[^unused]: [[A]]\n-->0  ```\nÉ\n%%[[A]]%%%%[[A]]%%``` [[A]]\n<!--\n[[A]]\n-->- [x] [[B]]\n- item\n\n      [^n]: [[A]]\n\n    [[B]]\n    [[A\\]]- > [!note] title\n[[A\nB]]```` go [[A]]\n## A\n## A\n\\``` ^é\n`open\n[[A]]\nclose`A\n===\n ^A\nA\n- > [!note] title\n     ^a-2\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 3, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 2}, old: nil, want: map[agreementCitation]int{{Target: "B", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 4", body: "\n\n[^n]: [[A]]\n\n    [[B]]\n`[[A]]`![[A]]      > [!note] title\n ^A\n- [ ] [[A]]\n[[A#^a]][[image.png]]`open\n[[A]]\nclose`A\n---\n > [!note] one\n> [!note] two\n> [!note] three\n## A\n\t1. ~~~~\n> %%[[A]]%%https://example.invalid/`[[A]]` 0[[A#A]]`[[A]]`## <em>A</em>\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 5", body: "[^n]: [[A]]\n\n    [[B]]\n`open\n[[A]]\nclose`A\n---\n> [!note] one\n> [!note] two\n> [!note] three\n\\[[A]]![[A#A]]![[image.png]][[A#A]]> > - [x] [[B]]\n> > - [x] [[B]]\n~~~\n%%[[A]]%%%%<!--<!--## [[A|alias]]\n- 1. \\`A\n-->- item\n\n      [[A]] ^é\n## !\n\n\n> [!note] [[A]]\n<!-- ^a-2\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "B", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 6", body: "     ^é\n0## !\n ````\n````\n章節\n  > [!note] title\n- item\n\n      [^n]: [[A]]\n\n    [[B]]\n- > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n[[A\\|alias]]`open\n[[A]]\nclose` ^a\n~~~\n``` [[A]]\n0## A\n[[A]]```` go [[A]]\n![[A#A]]- [ ] [[A]]\n ^é\n> [!note] title\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "B", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 7", body: "章節\n- > [!note] title\n\\`[^unused]: [[A]]\n~~~\n~~~\n## !\n\n ^é\n- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\nA\n`[[A]]`- [x] [[B]]\n[[A]]", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 2, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 2}},
		{name: "full original native unused code diagnostic 8", body: "> [!note] [[A]]\n0  ```\n- [ ] [[A]]\n- [x] [[B]]\n\\`## [[A|alias]]\n[[A#A]]> [!note] title\n## [[A|alias]]\n``# A\n> [!note] [[A]]\n> [!note] [[A]]\nA\n---\n  - É\n\t```\n[^unused]: [[A]]\nÉ\nA\nÉ\n`[[A]]`0\t[^n]: [[A]]\n\n    [[B]]\n  ```\n1. [[A]]![[A#A]]", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 9", body: "%%[[A]]%%1. # A\n\\`| a | b |\n|---|---|\n| [[A]] | ^a |\n![[A]] \n## !\n-->~~~\n[[A]] [[A]]text ``` [[A]]\n[[A#A]][[A\\]]`[[A]]`1.     ```\n[^unused]: [[A]]\n`[[A]]`- item\n\n      A\n---\n## A\n## A\n[[A#A]]- item\n\n      [[A\\|alias]]\t ^a\n\t", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 10", body: "- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]]![[A]]~~~~\n~~~\n~~~\nref[^n]\n[^unused]: [[A]]\n![[image.png]]https://example.invalid/`[[A]]` --> ```\ntext ~~~~\n章節\n[^n]: [[A]]\n\n    [[B]]\n``", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "image.png", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "image.png", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 11", body: "B\n0 ^a-2\n[^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]\\`> [!note] [[A]]\n`[[A]]`  ```\n`[[A]]`[[A#A]]![[image.png]][[B|alias]] ## !\n[^n]: [[A]]\n\n    [[B]]\n0^absent-prose %%", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 4, {Target: "A", Section: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 3, {Target: "image.png", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 2}, old: nil, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 4, {Target: "A", Section: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 3, {Target: "image.png", State: "wikilink-broken"}: 1}},
		{name: "full original native unused code diagnostic 12", body: "![[A#A]]  ```\n ```\n   ```\n  > [!note] title\n> >  ^é\n## !\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->[[B|alias]]  > [!note] title\n[^unused]: [[A]]\n`open\n[[A]]\nclose`## A-2\n## A\n## A\n- > [!note] title\n  - ", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 2, {Target: "B", State: "wikilink-broken"}: 1, {SourceRole: "unused-inline-code", Target: "A", State: "wikilink-broken"}: 1}, old: nil, want: map[agreementCitation]int{{Target: "B", State: "wikilink-broken"}: 1}},
		{name: "same target prose and code retain separate roles", body: "[^n]: [[A]] `[[A]]`\n", budget: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {SourceRole: agreementUnusedInlineCodeField, Target: "A", State: "wikilink-broken"}: 1}, want: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementUnusedCodeWidgetBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete unused code widget budget (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.old, agreementUnusedTextWidgetBudget(c.Body).Tuples); diff != "" {
				t.Fatalf("caught: changed earlier unused text widget boundary (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				changed := f
				changed.Tuple.SourceRole = agreementUnusedInlineCodeField
				if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, &changed, &actual, r.Diagnostics, budget); kind != "" {
					t.Fatal("caught: unused code widget borrowed a literal source role")
				}
				kind, authority, wrong := agreementSharedUnusedDiagnosticDifference(c, &f, &actual, r.Diagnostics, budget)
				if f.Property != "P0" || f.Direction != "diagnostic-only" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: unused code widget borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: unused code widget public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementSharedUnusedDiagnosticDrift(t, c, &f, &actual, r.Diagnostics, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete unused code widget public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
