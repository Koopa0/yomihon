package judge_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

// Reserved markup delimiters disappear before the page is parsed. Permit only
// those literal bytes to disappear from each native code payload. Fields keep
// their original spelling and position; removing a delimiter cannot create a
// field, change its escape or move its bang into another display role.
const agreementCodeMarkerRunes = "\ue000\ue001\ue002\ue003"

func agreementCodeMarkerText(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(agreementCodeMarkerRunes, r) {
			return -1
		}
		return r
	}, s)
}

func agreementMatchMarkerCodeWindow(literal string, parts []agreementCodePart) (budget map[agreementCitation]int, removed int, valid bool) {
	budget = make(map[agreementCitation]int)
	offset := 0
	skip := func() {
		for offset < len(literal) {
			r, width := utf8.DecodeRuneInString(literal[offset:])
			if !strings.ContainsRune(agreementCodeMarkerRunes, r) {
				break
			}
			offset += width
			removed++
		}
	}
	for _, part := range parts {
		skip()
		if !part.Field {
			at := 0
			for at < len(part.Text) {
				skip()
				if offset >= len(literal) {
					return nil, 0, false
				}
				_, width := utf8.DecodeRuneInString(literal[offset:])
				if !strings.HasPrefix(part.Text[at:], literal[offset:offset+width]) {
					return nil, 0, false
				}
				at += width
				offset += width
			}
			continue
		}
		open := offset
		embed := strings.HasPrefix(literal[offset:], "![[")
		if embed {
			open++
		}
		prefix := agreementCodeMarkerText(literal[:open])
		if !strings.HasPrefix(literal[open:], "[[") || graph.EscapedWikilinkAt(prefix+"[[", len(prefix)) || !embed && strings.HasSuffix(prefix, "!") {
			return nil, 0, false
		}
		_, tail, closed := strings.Cut(literal[open+2:], "]]")
		if !closed {
			return nil, 0, false
		}
		end := len(literal) - len(tail)
		field := literal[offset:end]
		if strings.ContainsAny(field, agreementCodeMarkerRunes) {
			return nil, 0, false
		}
		matched, _, valid := agreementMatchEmbedCodeWindow(field, []agreementCodePart{part})
		if !valid {
			return nil, 0, false
		}
		for tuple, count := range matched {
			budget[tuple] += count
		}
		offset = end
	}
	skip()
	return budget, removed, offset == len(literal)
}

func agreementMarkerCodeWindowBudget(body, raw string) agreementCodePayload {
	declared, valid := agreementNativeCodeWindows(body, agreementExclusiveCodeGrammar)
	actual, owned := agreementCodeWindowParts(raw)
	if !valid || !owned || len(declared) != len(actual) {
		return agreementCodePayload{}
	}
	budget := make(map[agreementCitation]int)
	removed := 0
	for i, window := range declared {
		matched, count, valid := agreementMatchMarkerCodeWindow(window.Text, actual[i])
		if !valid {
			return agreementCodePayload{}
		}
		removed += count
		for tuple, count := range matched {
			budget[tuple] += count
		}
	}
	if removed == 0 || len(budget) == 0 {
		return agreementCodePayload{}
	}
	return agreementCodePayload{Body: body, Tuples: budget}
}

func TestAgreementMarkerCodeMatcher(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	field := func(text string) agreementCodePart { return agreementCodePart{Text: text, Citation: a, Field: true} }
	for _, tc := range []struct {
		name, literal string
		parts         []agreementCodePart
		want          map[agreementCitation]int
		removed       int
		valid         bool
	}{
		{name: "all reserved delimiters and Unicode bytes", literal: "章\ue0000\ue001[[A]]\ue0020\ue003節", parts: []agreementCodePart{{Text: "章0"}, field("A"), {Text: "0節"}}, want: map[agreementCitation]int{a: 1}, removed: 4, valid: true},
		{name: "marker before first and after final field", literal: "\ue000[[A]]\ue001", parts: []agreementCodePart{field("A")}, want: map[agreementCitation]int{a: 1}, removed: 2, valid: true},
		{name: "all adjacent field contributions", literal: "\ue002[[A]][[A]]![[A]]\ue003", parts: []agreementCodePart{field("A"), field("A"), field("![[A]]")}, want: map[agreementCitation]int{a: 3}, removed: 2, valid: true},
		{name: "other private characters survive", literal: "\ue004[[A]]", parts: []agreementCodePart{{Text: "\ue004"}, field("A")}, want: map[agreementCitation]int{a: 1}, valid: true},
		{name: "ordinary field retains earlier ownership", literal: "[[A]]", parts: []agreementCodePart{field("A")}, want: map[agreementCitation]int{a: 1}, valid: true},
		{name: "literal bytes without a field", literal: "\ue0000\ue001", parts: []agreementCodePart{{Text: "0"}}, want: map[agreementCitation]int{}, removed: 2, valid: true},
		{name: "ordinary byte loss", literal: "\ue0000[[A]]", parts: []agreementCodePart{field("A")}},
		{name: "Unicode byte drift", literal: "章\ue0000[[A]]", parts: []agreementCodePart{{Text: "節0"}, field("A")}},
		{name: "extra actual bytes", literal: "\ue0000[[A]]", parts: []agreementCodePart{{Text: "0long"}, field("A")}},
		{name: "remaining ordinary bytes", literal: "\ue000[[A]] after", parts: []agreementCodePart{field("A")}, want: map[agreementCitation]int{a: 1}, removed: 1},
		{name: "missing opening", literal: "\ue000AA A]]", parts: []agreementCodePart{field("A")}},
		{name: "incomplete field", literal: "\ue000[[A", parts: []agreementCodePart{field("A")}},
		{name: "marker cannot create opening", literal: "[\ue000[A]]", parts: []agreementCodePart{field("A")}},
		{name: "marker inside field cannot change target", literal: "[[A\ue000]]", parts: []agreementCodePart{field("A")}},
		{name: "unchanged marked field still refuses ownership", literal: "[[A|sh\ue000own]]", parts: []agreementCodePart{field("sh\ue000own")}},
		{name: "marker inside alias cannot change display", literal: "[[A|sh\ue000own]]", parts: []agreementCodePart{field("shown")}},
		{name: "original escaped field", literal: "\\[[A]]", parts: []agreementCodePart{{Text: "\\"}, field("A")}},
		{name: "marker cannot change escaped ownership", literal: "\\\ue000[[A]]", parts: []agreementCodePart{{Text: "\\"}, field("A")}},
		{name: "marker cannot lend literal bang", literal: "!\ue000[[A]]", parts: []agreementCodePart{{Text: "!"}, field("A")}},
		{name: "wrong field tuple", literal: "\ue000[[B]]", parts: []agreementCodePart{field("B")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, removed, valid := agreementMatchMarkerCodeWindow(tc.literal, tc.parts)
			if diff := cmp.Diff(tc.want, got); valid != tc.valid || removed != tc.removed || diff != "" {
				t.Fatalf("caught: complete marker code literal positions valid=%t removed=%d (-want +got):\n%s", valid, removed, diff)
			}
		})
	}
}
func TestAgreementMarkerCodePayload(t *testing.T) {
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
		{name: "all four literal delimiters", body: "[[A]]\n\n`open\n0[[A]]0\nclose`\n", want: map[agreementCitation]int{a: 1}, p1: map[string]int{"A": 1}},
		{name: "every tuple and section", body: "`open\n[[A#place]] 0![[A]] [[B]]\nclose`\n", want: map[agreementCitation]int{place: 1, a: 1, b: 1}, p1: map[string]int{"A": 2, "B": 1}},
		{name: "every code contribution", body: "[[A]]\n\n`first\n0[[A]]\nend`\n\n`second\n[[A]] [[B]]0\nlast`\n", want: map[agreementCitation]int{a: 2, b: 1}, p1: map[string]int{"A": 2, "B": 1}},
		{name: "literal fence also participates", body: "`first\n[[A]]\nend`\n\n~~~\n0words\n~~~\n", want: map[agreementCitation]int{a: 1}, p1: map[string]int{"A": 1}},
		{name: "other private characters remain literal", body: "`open\n0[[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}, p1: map[string]int{"A": 1}},
		{name: "URL owns another grammar reading", body: "https://example.invalid/`[[A]]`\n\n`open\n0[[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}, p1: map[string]int{"A": 1}},
		{name: "used footnote occurrences already agree", body: "ref[^n]\n\n[^n]: `open\n    0[[A]]\n    close`\n", want: map[agreementCitation]int{a: 1}},
		{name: "other excess cannot borrow code", body: "[[A\\]]\n\n`open\n0[[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}},
		{name: "recorded complete source", body: "A\n---\nref[^n]\n\\\\[[A]]  ```\nref[^n]\n0`open\n[[A]]\nclose` ^é\n![[A]] ```\n[[image.png]][[#A]] ```\n", want: map[agreementCitation]int{a: 2}, p1: map[string]int{"A": 2}},
		{name: "markers outside code supply no ownership", body: "0`open\n[[A]]\nclose`\n", earlier: map[agreementCitation]int{a: 1}},
		{name: "no markers retain older ownership", body: "`open\n[[A]]\nclose`\n", earlier: map[agreementCitation]int{a: 1}},
		{name: "hidden declaration has no page owner", body: "%%\n`open\n0[[A]]\nclose`\n%%\n"},
		{name: "literal code has no carrier", body: "`0[[A]]`\n"},
		{name: "empty citation budget refuses ownership", body: "`0words`\n"},
		{name: "unused code has no declaration", body: "[^n]: `open\n    0[[A]]\n    close`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementMarkerCodeWindowBudget(c.Body, r.HTML)
			if diff := cmp.Diff(tc.want, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete marker code inventory (-want +got):\n%s", diff)
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
						t.Fatal("caught: marker code borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: marker code public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
			}
			if diff := cmp.Diff(tc.want, p2); diff != "" {
				t.Fatalf("caught: complete marker code public P2 receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.p1, p1); diff != "" {
				t.Fatalf("caught: complete marker code public P1 receipts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.earlier, agreementCodeWindowBudget(c.Body, r.HTML).Tuples); diff != "" {
				t.Fatalf("caught: marker code changed earlier source ownership (-want +got):\n%s", diff)
			}
			if agreementPageCodeWindowBudget(c.Body, r.HTML).Tuples != nil {
				t.Fatal("caught: marker code changed earlier page grammar ownership")
			}
		})
	}
}

func TestAgreementMarkerCodePayloadDrift(t *testing.T) {
	t.Parallel()
	const body = "[[A]]\n\n`first\n[[A]]\nend`\n\n`second \ue0000\ue001 last`\n"
	r, _ := agreementIsolatedPage(t, agreementCase{Body: body})
	if agreementMarkerCodeWindowBudget(body, r.HTML).Tuples == nil {
		t.Fatal("caught: marker code drift lacks owned baseline")
	}
	for _, tc := range []struct{ old, replacement string }{
		{old: "<code>first", replacement: "<code>unown"},
		{old: " end</code>", replacement: "</code>"},
		{old: "<code>second", replacement: "<span>second"},
		{old: "second 0 last", replacement: "second last"},
		{old: "second 0 last", replacement: "second \ue0040 last"},
	} {
		if strings.Count(r.HTML, tc.old) != 1 {
			t.Fatal("marker code drift does not identify one declared site")
		}
		if agreementMarkerCodeWindowBudget(body, strings.Replace(r.HTML, tc.old, tc.replacement, 1)).Tuples != nil {
			t.Fatal("caught: marker code borrowed changed literal bytes or owner")
		}
	}
	for _, changed := range []string{r.HTML + "<code>unowned</code>", r.HTML + "<code>[[A]]</code>", strings.NewReplacer("first", "second", "second", "first", "end", "last", "last", "end").Replace(r.HTML)} {
		if agreementMarkerCodeWindowBudget(body, changed).Tuples != nil {
			t.Fatal("caught: marker code borrowed extra or reordered page declaration")
		}
	}
}
