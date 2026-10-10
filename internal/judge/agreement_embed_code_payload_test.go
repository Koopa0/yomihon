package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

// An unresolved embed keeps its entire source spelling as the carrier's
// display. Match that spelling at its declared code position, alongside every
// ordinary field and literal byte in the page grammar's complete code set.
func agreementMatchEmbedCodeWindow(literal string, parts []agreementCodePart) (budget map[agreementCitation]int, embeds int, valid bool) {
	budget = make(map[agreementCitation]int)
	offset := 0
	for _, part := range parts {
		if !part.Field {
			if !strings.HasPrefix(literal[offset:], part.Text) {
				return nil, 0, false
			}
			offset += len(part.Text)
			continue
		}
		open := offset
		embed := strings.HasPrefix(literal[offset:], "![[")
		if embed {
			open++
		}
		if !strings.HasPrefix(literal[open:], "[[") || graph.EscapedWikilinkAt(literal, open) || !embed && open > 0 && literal[open-1] == '!' {
			return nil, 0, false
		}
		inner, tail, closed := strings.Cut(literal[open+2:], "]]")
		if !closed || strings.ContainsAny(inner, "[]\r\n") {
			return nil, 0, false
		}
		link, cites := graph.ParseWikilink(inner)
		end := len(literal) - len(tail)
		display := link.Display
		if embed {
			display = literal[offset:end]
		}
		if !cites || link.Block != "" || part.Citation != (agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}) || part.Text != display {
			return nil, 0, false
		}
		offset = end
		budget[part.Citation]++
		if embed {
			embeds++
		}
	}
	return budget, embeds, offset == len(literal)
}

func agreementEmbedCodeWindowBudget(body, raw string) agreementCodePayload {
	declared, valid := agreementNativeCodeWindows(body, agreementExclusiveCodeGrammar)
	actual, owned := agreementCodeWindowParts(raw)
	if !valid || !owned || len(declared) != len(actual) {
		return agreementCodePayload{}
	}
	budget := make(map[agreementCitation]int)
	embeds := 0
	for i, window := range declared {
		matched, count, valid := agreementMatchEmbedCodeWindow(window.Text, actual[i])
		if !valid {
			return agreementCodePayload{}
		}
		embeds += count
		for tuple, count := range matched {
			budget[tuple] += count
		}
	}
	if embeds == 0 {
		return agreementCodePayload{}
	}
	return agreementCodePayload{Body: body, Tuples: budget}
}

func TestAgreementEmbedCodeMatcher(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	place := agreementCitation{Target: "A", Section: "place", State: "wikilink-broken"}
	field := func(text string, tuple agreementCitation) agreementCodePart {
		return agreementCodePart{Text: text, Citation: tuple, Field: true}
	}
	for _, tc := range []struct {
		name, literal string
		parts         []agreementCodePart
		want          map[agreementCitation]int
		embeds        int
		valid         bool
	}{
		{name: "all field roles and literal bytes", literal: "before ![[A#place|shown]] [[A]] ![[A]] after", parts: []agreementCodePart{{Text: "before "}, field("![[A#place|shown]]", place), {Text: " "}, field("A", a), {Text: " "}, field("![[A]]", a), {Text: " after"}}, want: map[agreementCitation]int{place: 1, a: 2}, embeds: 2, valid: true},
		{name: "literal embed beside converted one", literal: "![[A]] ![[A]]", parts: []agreementCodePart{{Text: "![[A]] "}, field("![[A]]", a)}, want: map[agreementCitation]int{a: 1}, embeds: 1, valid: true},
		{name: "ordinary field retains earlier role", literal: "[[A]]", parts: []agreementCodePart{field("A", a)}, want: map[agreementCitation]int{a: 1}, valid: true},
		{name: "all literal payload", literal: "![[A]]", parts: []agreementCodePart{{Text: "![[A]]"}}, want: map[agreementCitation]int{}, valid: true},
		{name: "prefix drift", literal: "before ![[A]]", parts: []agreementCodePart{{Text: "unownx "}, field("![[A]]", a)}},
		{name: "suffix drift", literal: "![[A]] after", parts: []agreementCodePart{field("![[A]]", a), {Text: " unowned"}}},
		{name: "missing final bytes", literal: "![[A]] after", parts: []agreementCodePart{field("![[A]]", a)}, want: map[agreementCitation]int{a: 1}, embeds: 1},
		{name: "missing opening", literal: "AA A]]", parts: []agreementCodePart{field("A", a)}},
		{name: "incomplete field", literal: "![[A", parts: []agreementCodePart{field("![[A", a)}},
		{name: "nested opening", literal: "![[A[[B|shown]]]]", parts: []agreementCodePart{field("![[A[[B|shown]]", agreementCitation{Target: "A[[B", State: "wikilink-broken"}), {Text: "]]"}}},
		{name: "wrapped syntax", literal: "![[A\nB|shown]]", parts: []agreementCodePart{field("![[A\nB|shown]]", agreementCitation{Target: "A\nB", State: "wikilink-broken"})}},
		{name: "escaped ordinary field", literal: "\\[[A]]", parts: []agreementCodePart{{Text: "\\"}, field("A", a)}},
		{name: "bang cannot belong to earlier literal", literal: "![[A]]", parts: []agreementCodePart{{Text: "!"}, field("A", a)}},
		{name: "local embed", literal: "![[#A]]", parts: []agreementCodePart{field("![[#A]]", agreementCitation{Section: "A", State: "wikilink-broken"})}},
		{name: "block embed", literal: "![[A#^a]]", parts: []agreementCodePart{field("![[A#^a]]", a)}},
		{name: "tuple drift", literal: "![[A#place|shown]]", parts: []agreementCodePart{field("![[A#place|shown]]", a)}},
		{name: "alias cannot replace full embed spelling", literal: "![[A#place|shown]]", parts: []agreementCodePart{field("shown", place)}},
		{name: "embed spelling cannot replace ordinary display", literal: "[[A]]", parts: []agreementCodePart{field("[[A]]", a)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, embeds, valid := agreementMatchEmbedCodeWindow(tc.literal, tc.parts)
			if diff := cmp.Diff(tc.want, got); valid != tc.valid || embeds != tc.embeds || diff != "" {
				t.Fatalf("caught: complete embed code field roles valid=%t embeds=%d (-want +got):\n%s", valid, embeds, diff)
			}
		})
	}
}

func TestAgreementEmbedCodePayload(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	place := agreementCitation{Target: "A", Section: "place", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
		p1         map[string]int
		earlier    map[agreementCitation]int
	}{
		{name: "ordinary and embedded same target", body: "[[A]]\n\n`open\n![[A]] [[B]]\nclose`\n", want: map[agreementCitation]int{a: 1, b: 1}, p1: map[string]int{"A": 1, "B": 1}},
		{name: "full embed alias and section", body: "`open\n![[A#place|shown]] ![[B]]\nclose`\n", want: map[agreementCitation]int{place: 1, b: 1}, p1: map[string]int{"A": 1, "B": 1}},
		{name: "all sections of same target", body: "`open\n![[A#place|shown]] [[A]]\nclose`\n", want: map[agreementCitation]int{place: 1, a: 1}, p1: map[string]int{"A": 2}},
		{name: "all counts across code declarations", body: "[[A]]\n\n`first\n![[A]] [[A]]\nend`\n\n`second\n![[A]] [[B]]\nlast`\n", want: map[agreementCitation]int{a: 3, b: 1}, p1: map[string]int{"A": 3, "B": 1}},
		{name: "page grammar around URL", body: "https://example.invalid/`[[A]]`\n\n`open\n![[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}, p1: map[string]int{"A": 1}},
		{name: "quoted fence", body: "[[A]]\n\n> ```\n> ![[A]] [[B]]\n> ```\n", want: map[agreementCitation]int{a: 1, b: 1}, p1: map[string]int{"A": 1, "B": 1}},
		{name: "used footnote counts agree", body: "ref[^n]\n\n[^n]: `open\n    ![[A]]\n    close`\n", want: map[agreementCitation]int{a: 1}},
		{name: "literal code participates in complete set", body: "`open\n![[A]]\nclose`\n\n`![[B]]`\n\n    ![[A]]\n", want: map[agreementCitation]int{a: 1}, p1: map[string]int{"A": 1}},
		{name: "recorded complete source", body: "> ^absent-prose ![[A]]``<!--\n[[A]]\n-->![[A]]>  ``\ue0000\ue001- item\n\n      ", want: map[agreementCitation]int{a: 2}, p1: map[string]int{"A": 2}},
		{name: "unrelated excess refuses occurrence receipt", body: "[[A\\]]\n\n`open\n![[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}},
		{name: "no converted embed keeps earlier ownership", body: "`open\n[[A]]\nclose`\n", earlier: map[agreementCitation]int{a: 1}},
		{name: "hidden declaration has no page owner", body: "%%\n`open\n![[A]]\nclose`\n%%\n"},
		{name: "ordinary prose has no code owner", body: "![[A]]\n"},
		{name: "literal embed has no converted owner", body: "`![[A]]`\n"},
		{name: "unused code has no native declaration", body: "[^n]: `open\n    ![[A]]\n    close`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementEmbedCodeWindowBudget(c.Body, r.HTML)
			if diff := cmp.Diff(tc.want, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete embed code inventory (-want +got):\n%s", diff)
			}
			var p1 map[string]int
			var p2 map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementCodeWindowDifference(c, &f, &actual, budget)
				if f.Property == "P1" {
					kind, authority, wrong = agreementCodeCitationDifference(c, &f, &actual, budget)
				}
				switch {
				case f.Property == "P2" && tc.want[f.Tuple] != 0:
					if p2 == nil {
						p2 = make(map[agreementCitation]int)
					}
					p2[f.Tuple] = f.Multiplicity
					agreementCodePayloadDrift(t, c, &f, &actual, budget)
				case f.Property == "P1" && tc.p1[f.Tuple.Target] != 0:
					if p1 == nil {
						p1 = make(map[string]int)
					}
					p1[f.Tuple.Target] = f.Multiplicity
					agreementCodeCitationDrift(t, c, &f, &actual, budget)
				default:
					if kind != "" {
						t.Fatal("caught: embed code borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: embed code public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
			}
			if diff := cmp.Diff(tc.want, p2); diff != "" {
				t.Fatalf("caught: complete embed code public P2 receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.p1, p1); diff != "" {
				t.Fatalf("caught: complete embed code public P1 receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.earlier, agreementCodeWindowBudget(c.Body, r.HTML).Tuples); diff != "" {
				t.Fatalf("caught: embed code changed earlier source ownership (-want +got):\n%s", diff)
			}
			if agreementPageCodeWindowBudget(c.Body, r.HTML).Tuples != nil {
				t.Fatal("caught: embed code changed earlier page grammar ownership")
			}
		})
	}
}

func TestAgreementEmbedCodePayloadDrift(t *testing.T) {
	t.Parallel()
	const body = "[[A]]\n\n`first\n![[A]]\nend`\n\n`second\n[[B]]\nlast`\n"
	r, _ := agreementIsolatedPage(t, agreementCase{Body: body})
	if agreementEmbedCodeWindowBudget(body, r.HTML).Tuples == nil {
		t.Fatal("caught: embed code drift lacks owned baseline")
	}
	for _, tc := range []struct{ old, replacement string }{
		{old: "<code>first", replacement: "<code>unown"},
		{old: " end</code>", replacement: "</code>"},
		{old: "<code>second", replacement: "<span>second"},
		{old: "![[A]]", replacement: "A"},
	} {
		if strings.Count(r.HTML, tc.old) != 1 {
			t.Fatal("embed code drift does not identify one declared page site")
		}
		if agreementEmbedCodeWindowBudget(body, strings.Replace(r.HTML, tc.old, tc.replacement, 1)).Tuples != nil {
			t.Fatal("caught: embed code borrowed changed payload or field role")
		}
	}
	for _, changed := range []string{
		r.HTML + "<code>unowned</code>",
		r.HTML + "<code>![[A]]</code>",
		strings.NewReplacer("first", "second", "second", "first", "end", "last", "last", "end").Replace(r.HTML),
	} {
		if agreementEmbedCodeWindowBudget(body, changed).Tuples != nil {
			t.Fatal("caught: embed code borrowed extra or reordered code declaration")
		}
	}
}
