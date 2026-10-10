package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

// A block address retains its authored display even though the broken carrier
// reports the target without that address. Keep every ordinary field and every
// literal byte at its declared position in the page grammar's whole code set.
func agreementMatchBlockCodeWindow(literal string, parts []agreementCodePart) (budget map[agreementCitation]int, blocks int, valid bool) {
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
		if !strings.HasPrefix(literal[offset:], "[[") || graph.EscapedWikilinkAt(literal, offset) || offset > 0 && literal[offset-1] == '!' {
			return nil, 0, false
		}
		inner, tail, closed := strings.Cut(literal[offset+2:], "]]")
		if !closed || strings.ContainsAny(inner, "[]\r\n") {
			return nil, 0, false
		}
		link, cites := graph.ParseWikilink(inner)
		if !cites || part.Citation != (agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}) || part.Text != link.Display {
			return nil, 0, false
		}
		offset = len(literal) - len(tail)
		budget[part.Citation]++
		if link.Block != "" {
			blocks++
		}
	}
	return budget, blocks, offset == len(literal)
}

func agreementBlockCodeWindowBudget(body, raw string) agreementCodePayload {
	declared, valid := agreementNativeCodeWindows(body, agreementExclusiveCodeGrammar)
	actual, owned := agreementCodeWindowParts(raw)
	if !valid || !owned || len(declared) != len(actual) {
		return agreementCodePayload{}
	}
	budget := make(map[agreementCitation]int)
	blocks := 0
	for i, window := range declared {
		matched, count, valid := agreementMatchBlockCodeWindow(window.Text, actual[i])
		if !valid {
			return agreementCodePayload{}
		}
		blocks += count
		for tuple, count := range matched {
			budget[tuple] += count
		}
	}
	if blocks == 0 {
		return agreementCodePayload{}
	}
	return agreementCodePayload{Body: body, Tuples: budget}
}

func TestAgreementBlockCodeMatcher(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", Section: "place", State: "wikilink-broken"}
	field := func(text string, tuple agreementCitation) agreementCodePart {
		return agreementCodePart{Text: text, Citation: tuple, Field: true}
	}
	for _, tc := range []struct {
		name, literal string
		parts         []agreementCodePart
		want          map[agreementCitation]int
		blocks        int
		valid         bool
	}{
		{name: "whole block and literal bytes", literal: "before [[A#^a]] after", parts: []agreementCodePart{{Text: "before "}, field("A#^a", a), {Text: " after"}}, want: map[agreementCitation]int{a: 1}, blocks: 1, valid: true},
		{name: "every adjacent field and repeated tuple", literal: "[[A#^a]][[B#place|shown]][[A#^b]]", parts: []agreementCodePart{field("A#^a", a), field("shown", b), field("A#^b", a)}, want: map[agreementCitation]int{a: 2, b: 1}, blocks: 2, valid: true},
		{name: "block alias stays authored", literal: "[[A#^a|shown]]", parts: []agreementCodePart{field("shown", a)}, want: map[agreementCitation]int{a: 1}, blocks: 1, valid: true},
		{name: "literal block beside converted block", literal: "[[A#^a]] [[A#^b]]", parts: []agreementCodePart{{Text: "[[A#^a]] "}, field("A#^b", a)}, want: map[agreementCitation]int{a: 1}, blocks: 1, valid: true},
		{name: "ordinary field retains its role", literal: "[[A]]", parts: []agreementCodePart{field("A", a)}, want: map[agreementCitation]int{a: 1}, valid: true},
		{name: "literal block supplies no contribution", literal: "[[A#^a]]", parts: []agreementCodePart{{Text: "[[A#^a]]"}}, want: map[agreementCitation]int{}, valid: true},
		{name: "prefix drift", literal: "before [[A#^a]]", parts: []agreementCodePart{{Text: "unownx "}, field("A#^a", a)}},
		{name: "suffix drift", literal: "[[A#^a]] after", parts: []agreementCodePart{field("A#^a", a), {Text: " unowned"}}},
		{name: "missing final bytes", literal: "[[A#^a]] after", parts: []agreementCodePart{field("A#^a", a)}, want: map[agreementCitation]int{a: 1}, blocks: 1},
		{name: "absent field opening", literal: "AA A#^a]]", parts: []agreementCodePart{field("A#^a", a)}},
		{name: "incomplete field", literal: "[[A#^a", parts: []agreementCodePart{field("A#^a", a)}},
		{name: "nested opening stays unowned", literal: "[[A[[B#^b]]]]", parts: []agreementCodePart{field("A[[B#^b", agreementCitation{Target: "A[[B", State: "wikilink-broken"}), {Text: "]]"}}},
		{name: "wrapped field stays unowned", literal: "[[A\nB#^a]]", parts: []agreementCodePart{field("A\nB#^a", agreementCitation{Target: "A\nB", State: "wikilink-broken"})}},
		{name: "escaped block stays literal", literal: "\\[[A#^a]]", parts: []agreementCodePart{{Text: "\\"}, field("A#^a", a)}},
		{name: "embed stays another owner", literal: "![[A#^a]]", parts: []agreementCodePart{{Text: "!"}, field("A#^a", a)}},
		{name: "local block stays another owner", literal: "[[#^a]]", parts: []agreementCodePart{field("#^a", agreementCitation{State: "wikilink-broken"})}},
		{name: "target drift", literal: "[[A#^a]]", parts: []agreementCodePart{field("A#^a", b)}},
		{name: "section cannot borrow block", literal: "[[A#^a]]", parts: []agreementCodePart{field("A#^a", agreementCitation{Target: "A", Section: "^a", State: "wikilink-broken"})}},
		{name: "state drift", literal: "[[A#^a]]", parts: []agreementCodePart{field("A#^a", agreementCitation{Target: "A", State: "wikilink"})}},
		{name: "role drift", literal: "[[A#^a]]", parts: []agreementCodePart{field("A#^a", agreementCitation{SourceRole: "unowned", Target: "A", State: "wikilink-broken"})}},
		{name: "block display cannot lose address", literal: "[[A#^a]]", parts: []agreementCodePart{field("A", a)}},
		{name: "alias cannot borrow raw display", literal: "[[A#^a|shown]]", parts: []agreementCodePart{field("A#^a", a)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, blocks, valid := agreementMatchBlockCodeWindow(tc.literal, tc.parts)
			if diff := cmp.Diff(tc.want, got); valid != tc.valid || blocks != tc.blocks || diff != "" {
				t.Fatalf("caught: complete block code field positions valid=%t blocks=%d, want %t %d (-want +got):\n%s", valid, blocks, tc.valid, tc.blocks, diff)
			}
		})
	}
}

func TestAgreementBlockCodeHTMLDrift(t *testing.T) {
	t.Parallel()
	const body = "[[A]]\n\n`open\n[[A#^a]]\nclose`\n\n`words`\n"
	r, _ := agreementIsolatedPage(t, agreementCase{Body: body})
	if agreementBlockCodeWindowBudget(body, r.HTML).Tuples == nil {
		t.Fatal("caught: block code drift lacks an owned baseline")
	}
	for _, tc := range []struct {
		name, old, replacement string
	}{
		{name: "prefix bytes", old: "<code>open", replacement: "<code>unowned"},
		{name: "suffix bytes", old: " close</code>", replacement: " unowned</code>"},
		{name: "missing suffix", old: " close</code>", replacement: "</code>"},
		{name: "missing code owner", old: "<code>open", replacement: "<span>open"},
		{name: "nested code owner", old: "<code>open", replacement: "<code><code>open"},
		{name: "block address display", old: ">A#^a<span", replacement: ">A<span"},
		{name: "unrelated literal bytes", old: "<code>words</code>", replacement: "<code>other</code>"},
		{name: "missing unrelated declaration", old: "<code>words</code>", replacement: "words"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if strings.Count(r.HTML, tc.old) != 1 {
				t.Fatal("block code drift does not identify one declared page site")
			}
			changed := strings.Replace(r.HTML, tc.old, tc.replacement, 1)
			if agreementBlockCodeWindowBudget(body, changed).Tuples != nil {
				t.Fatal("caught: block code payload borrowed changed page bytes or owner")
			}
		})
	}
	if agreementBlockCodeWindowBudget(body, r.HTML+"<code>unowned</code>").Tuples != nil {
		t.Fatal("caught: block code payload borrowed extra page declaration")
	}
	emptyBody := body + "\n```\n```\n"
	empty, _ := agreementIsolatedPage(t, agreementCase{Body: emptyBody})
	if agreementBlockCodeWindowBudget(emptyBody, empty.HTML).Tuples == nil || strings.Count(empty.HTML, "<code></code>") != 1 {
		t.Fatalf("caught: block code empty declaration lacks an owned baseline: %q", empty.HTML)
	}
	nested := strings.Replace(empty.HTML, "<code></code>", "<code><code></code></code>", 1)
	if agreementBlockCodeWindowBudget(emptyBody, nested).Tuples != nil {
		t.Fatal("caught: block code payload borrowed invalid empty code owner")
	}
}

func TestAgreementBlockCodeParts(t *testing.T) {
	t.Parallel()
	const body = "`open\n[[A#^a]]\nclose`\n"
	r, _ := agreementIsolatedPage(t, agreementCase{Body: body})
	_, tail, found := strings.Cut(r.HTML, "<code>open")
	inside, _, closed := strings.Cut(tail, "</code>")
	if !found || !closed {
		t.Fatal("block code parts do not identify a declared page owner")
	}
	raw := "<code>open" + inside + "</code>"
	want := [][]agreementCodePart{{{Text: "open "}, {Text: "A#^a", Citation: agreementCitation{Target: "A", State: "wikilink-broken"}, Field: true}, {Text: " close"}}}
	got, valid := agreementCodeWindowParts(raw)
	if diff := cmp.Diff(want, got); !valid || diff != "" {
		t.Fatalf("caught: complete block code page parts valid=%t (-want +got):\n%s", valid, diff)
	}
	_, words, explanation := strings.Cut(raw, "<span class=\"y-offscreen\">")
	word, _, end := strings.Cut(words, "</span>")
	if !explanation || !end {
		t.Fatal("block code parts lack a declared explanation")
	}
	full := "<span class=\"y-offscreen\">" + word + "</span>"
	degraded := strings.Replace(raw, "<span class=\"wikilink-broken\"", "<a href=\"/notes/A.md\" class=\"wikilink wikilink-degraded\"", 1)
	degraded = strings.Replace(degraded, "</span> close</code>", "</a> close</code>", 1)
	if _, valid := agreementCodeWindowParts(degraded); valid {
		t.Fatal("caught: block code parts borrowed another declared carrier state")
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{name: "missing explanation", old: full, replacement: ""},
		{name: "extra explanation", old: full, replacement: full + full},
		{name: "resolved state", old: "class=\"wikilink-broken\"", replacement: "class=\"wikilink\""},
		{name: "nested owner", old: "<code>open", replacement: "<code><code></code>open"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if strings.Count(raw, tc.old) != 1 {
				t.Fatal("block code parts drift does not identify one declared site")
			}
			changed := strings.Replace(raw, tc.old, tc.replacement, 1)
			if _, valid := agreementCodeWindowParts(changed); valid {
				t.Fatal("caught: block code parts borrowed changed carrier or owner")
			}
		})
	}
}

func TestAgreementBlockCodeSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       []agreementNativeCodeWindow
	}{
		{name: "full original block code 0", body: "[[A\nB]] ^é\n\t```\n\\[[A]][[A#^a]]https://example.invalid/`[[A]]` ``` [[A]]\n<!--[[A#A]]É\n0[[A]] [[A]]1. | a | b |\n|---|---|\n| [[A]] | ^a |\n``", want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 18, Stop: 63}, Text: "\\[[A]][[A#^a]]https://example.invalid/`[[A]]`"}}},
		{name: "full original block code 1", body: "B\n0 ^a\n[[A#^a]]- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n- > [!note] title\n![[A#A]][[A]]`[[A#^a]]É\n`[[A]]`    ```\ntext   > [!note] title\n`[[A]]`<!--\n[[A]]\n-->[[#A]]1. ", want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 109, Stop: 121}, Text: "[[A#^a]]É "}, {Kind: "CodeSpan", Span: graph.Span{Start: 129, Stop: 158}, Text: "   ``` text   > [!note] title"}}},
		{name: "full original block code 2", body: "[[A#^a]]\\`## A\n## A\n![[A]]A\n---\n[[A]] ^a-2\n[[A\\|alias]]https://example.invalid/`[[A]]` 1. ~~~~\n ^A\n![[A]]> [!note] [[A]]\nÉ\n`[[A]]`  - 0 \t```\n[[A#^a]]   ```\n  > [!note] title\n[[A#^a]]text <!--\n[[A]]\n-->0", want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 80, Stop: 85}, Text: "[[A]]"}, {Kind: "CodeSpan", Span: graph.Span{Start: 125, Stop: 130}, Text: "[[A]]"}, {Kind: "CodeSpan", Span: graph.Span{Start: 148, Stop: 158}, Text: "[[A#^a]]  "}}},
		{name: "full original block code 3", body: "ref[^n]\n\\\\[[A]][[A\nB]]  - text [[B|alias]]  > [!note] title\n\\`[[A#A]]```` go [[A]]\n[[A\\|alias]]\t```\n[[A#^a]]   ```\n<div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n   ```\nA\n---\n%%![[A]]- > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## !\n# A\n  - ``| a | b |\n|---|---|\n| [[A]] | ^a |\n", want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 100, Stop: 110}, Text: "[[A#^a]]  "}, {Kind: "CodeBlock", Span: graph.Span{Start: 151, Stop: 157}, Text: "[[B]]\n"}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 164, Stop: 281}, Text: "A\n---\n%%![[A]]- > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## !\n# A\n- ``| a | b |\n|---|---|\n| [[A]] | ^a |\n"}}},
		{name: "full original block code 4", body: "[[B|alias]]   ```\n[[A#^a]]> >   > [!note] title\n\t```\n    ```\nÉ\n## [[A|alias]]\n[[A\nB]][[A#^a]]``> [!note] title\n[[A#A]]## A-2\n## A\n## A\n~~~\n![[image.png]]", want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 18, Stop: 47}, Text: "[[A#^a]]> >   > [!note] title"}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 140, Stop: 154}, Text: "![[image.png]]\n"}}},
		{name: "full original block code 5", body: "[[A#^a]]1. `[[A]]```  > [!note] title\n[[A#^a]]text    ```\n[[B|alias]]<!-- [[A]] -->ref[^n]\n[[image.png]]<!-- [[A]] --># A\n- [ ] [[A]]\n[[A\\]] ^é\n````\n- item\n\n      `[[A]]`![[image.png]]  > [!note] title\n   ```\n1. > > B\n\\` ^a\n[[A#A]]<!-- [[A]] -->1. ", want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 21, Stop: 53}, Text: " > [!note] title [[A#^a]]text   "}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 150, Stop: 249}, Text: "- item\n\n      `[[A]]`![[image.png]]  > [!note] title\n   ```\n1. > > B\n\\` ^a\n[[A#A]]<!-- [[A]] -->1. \n"}}},
		{name: "full original block code 6", body: "A\n1.  ^é\nhttps://example.invalid/`[[A]]`    ```\nA\n===\n[[A\\|alias]][[A#^a]]   ```\nref[^n]\n[[A\nB]]![[A#A]]> [!note] one\n> [!note] two\n> [!note] three\n  ```\n\\[[A]]", want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 49, Stop: 77}, Text: "A === [[A\\|alias]][[A#^a]]  "}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 155, Stop: 161}, Text: "\\[[A]]\n"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, valid := agreementNativeCodeWindows(tc.body, agreementExclusiveCodeGrammar)
			if diff := cmp.Diff(tc.want, got); !valid || diff != "" {
				t.Fatalf("caught: complete block code native declarations valid=%t (-want +got):\n%s", valid, diff)
			}
		})
	}
}
func TestAgreementBlockCodePublic(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", Section: "place", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
	}{
		{name: "whole wrapped block", body: "[[A]]\n\n`open\n[[A#^a]]\nclose`\n", want: map[agreementCitation]int{a: 1}},
		{name: "whole block alias", body: "`open\n[[A#^a|shown]]\nclose`\n", want: map[agreementCitation]int{a: 1}},
		{name: "all block and section fields", body: "`open\n[[A#^a]] [[B#place|shown]] [[A#^b]]\nclose`\n", want: map[agreementCitation]int{a: 2, b: 1}},
		{name: "all declared code sets", body: "`open\n[[A#^a]]\nclose`\n\n`words`\n\n```\n[[A#^b]]\n```\n\n    [[A#^c]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "every contributing code window", body: "`first\n[[A#^a]]\nend`\n\n`second\n[[B#place]] [[A#^b]]\nlast`\n", want: map[agreementCitation]int{a: 2, b: 1}},
		{name: "page URL grammar retains its own code set", body: "https://example.invalid/`[[A]]`\n\n`open\n[[A#^a]] [[B#place|shown]]\nclose`\n", want: map[agreementCitation]int{a: 1, b: 1}},
		{name: "used footnote keeps source code", body: "ref[^n]\n\n[^n]: `open\n    [[A#^a]]\n    close`\n", want: map[agreementCitation]int{a: 1}},
		{name: "escaped field stays literal beside block", body: "`open\n\\[[A#^b]] [[A#^a]]\nclose`\n", want: map[agreementCitation]int{a: 1}},
		{name: "CRLF retains whole block text", body: "`open\r\n[[A#^a]]\r\nclose`\r\n", want: map[agreementCitation]int{a: 1}},
		{name: "ordinary fields keep earlier ownership", body: "`open\n[[A]]\nclose`\n", want: nil},
		{name: "ordinary prose supplies no code", body: "[[A#^a]]\n", want: nil},
		{name: "root fence stays literal", body: "```\n[[A#^a]]\n```\n", want: nil},
		{name: "indented code stays literal", body: "    [[A#^a]]\n", want: nil},
		{name: "single line code stays literal", body: "`[[A#^a]]`\n", want: nil},
		{name: "unused definition supplies no code", body: "[^n]: `open\n    [[A#^a]]\n    close`\n", want: nil},
		{name: "comment has no code carrier", body: "%%\n`open\n[[A#^a]]\nclose`\n%%\n", want: nil},
		{name: "embed still refuses whole set", body: "`open\n![[A#^a]] [[A#^b]]\nclose`\n", want: nil},
		{name: "local block still refuses whole set", body: "`open\n[[#^a]] [[A#^b]]\nclose`\n", want: nil},
		{name: "full original block code 0", body: "[[A\nB]] ^é\n\t```\n\\[[A]][[A#^a]]https://example.invalid/`[[A]]` ``` [[A]]\n<!--[[A#A]]É\n0[[A]] [[A]]1. | a | b |\n|---|---|\n| [[A]] | ^a |\n``", want: map[agreementCitation]int{a: 1}},
		{name: "full original block code 1", body: "B\n0 ^a\n[[A#^a]]- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n- > [!note] title\n![[A#A]][[A]]`[[A#^a]]É\n`[[A]]`    ```\ntext   > [!note] title\n`[[A]]`<!--\n[[A]]\n-->[[#A]]1. ", want: map[agreementCitation]int{a: 1}},
		{name: "full original block code 2", body: "[[A#^a]]\\`## A\n## A\n![[A]]A\n---\n[[A]] ^a-2\n[[A\\|alias]]https://example.invalid/`[[A]]` 1. ~~~~\n ^A\n![[A]]> [!note] [[A]]\nÉ\n`[[A]]`  - 0 \t```\n[[A#^a]]   ```\n  > [!note] title\n[[A#^a]]text <!--\n[[A]]\n-->0", want: map[agreementCitation]int{a: 1}},
		{name: "full original block code 3", body: "ref[^n]\n\\\\[[A]][[A\nB]]  - text [[B|alias]]  > [!note] title\n\\`[[A#A]]```` go [[A]]\n[[A\\|alias]]\t```\n[[A#^a]]   ```\n<div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n   ```\nA\n---\n%%![[A]]- > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## !\n# A\n  - ``| a | b |\n|---|---|\n| [[A]] | ^a |\n", want: map[agreementCitation]int{a: 1}},
		{name: "full original block code 4", body: "[[B|alias]]   ```\n[[A#^a]]> >   > [!note] title\n\t```\n    ```\nÉ\n## [[A|alias]]\n[[A\nB]][[A#^a]]``> [!note] title\n[[A#A]]## A-2\n## A\n## A\n~~~\n![[image.png]]", want: map[agreementCitation]int{a: 1}},
		{name: "full original block code 5", body: "[[A#^a]]1. `[[A]]```  > [!note] title\n[[A#^a]]text    ```\n[[B|alias]]<!-- [[A]] -->ref[^n]\n[[image.png]]<!-- [[A]] --># A\n- [ ] [[A]]\n[[A\\]] ^é\n````\n- item\n\n      `[[A]]`![[image.png]]  > [!note] title\n   ```\n1. > > B\n\\` ^a\n[[A#A]]<!-- [[A]] -->1. ", want: map[agreementCitation]int{a: 1}},
		{name: "full original block code 6", body: "A\n1.  ^é\nhttps://example.invalid/`[[A]]`    ```\nA\n===\n[[A\\|alias]][[A#^a]]   ```\nref[^n]\n[[A\nB]]![[A#A]]> [!note] one\n> [!note] two\n> [!note] three\n  ```\n\\[[A]]", want: map[agreementCitation]int{a: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementBlockCodeWindowBudget(c.Body, r.HTML)
			want := agreementCodePayload{}
			if len(tc.want) != 0 {
				want = agreementCodePayload{Body: c.Body, Tuples: tc.want}
			}
			if diff := cmp.Diff(want, budget); diff != "" {
				t.Fatalf("caught: complete block code public inventory (-want +got):\n%s", diff)
			}
			if len(tc.want) > 0 && (agreementCodeWindowBudget(c.Body, r.HTML).Tuples != nil || agreementPageCodeWindowBudget(c.Body, r.HTML).Tuples != nil) {
				t.Fatal("caught: earlier code payload borrowed block address ownership")
			}
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementCodeWindowDifference(c, &f, &actual, budget)
				if f.Property != "P2" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: block code payload borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: block code public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementCodePayloadDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete block code public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
