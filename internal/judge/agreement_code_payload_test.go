package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/graph"
)

// A prose occurrence cannot lend its carrier to a literal declaration. Match
// every native code payload to the entire ordered page code set, preserving all
// text around each field. A field either stays literal or occupies exactly its
// declared position as an explained carrier; no other byte can move or vanish.
type agreementNativeCodeWindow struct {
	Kind string
	Span graph.Span
	Text string
}

type agreementCodePart struct {
	Text     string
	Citation agreementCitation
	Field    bool
}

func agreementCodeCarrierPart(n *html.Node) (agreementCodePart, bool) {
	state, shapeErr := agreementCarrierState(n)
	tuple, noticeErr := agreementNotice(agreementAttr(n, "title"))
	if shapeErr != nil || noticeErr != nil || state != "wikilink-broken" {
		return agreementCodePart{}, false
	}
	var display strings.Builder
	explanations := 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			display.WriteString(child.Data)
			continue
		}
		if child.Type != html.ElementNode || child.Data != "span" || agreementAttr(child, "class") != "y-offscreen" {
			return agreementCodePart{}, false
		}
		explanations++
		for word := child.FirstChild; word != nil; word = word.NextSibling {
			if word.Type != html.TextNode {
				return agreementCodePart{}, false
			}
		}
	}
	if explanations != 1 {
		return agreementCodePart{}, false
	}
	return agreementCodePart{Text: display.String(), Citation: tuple, Field: true}, true
}

func agreementCodeParts(root *html.Node) ([]agreementCodePart, bool) {
	var parts []agreementCodePart
	valid := true
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n != root && n.Type == html.ElementNode && n.Data == "code" {
			valid = false
			return
		}
		if n.Type == html.ElementNode && agreementCarrier(n) {
			part, owned := agreementCodeCarrierPart(n)
			valid = valid && owned
			parts = append(parts, part)
			return
		}
		if n.Type == html.TextNode {
			if len(parts) > 0 && !parts[len(parts)-1].Field {
				parts[len(parts)-1].Text += n.Data
			} else {
				parts = append(parts, agreementCodePart{Text: n.Data})
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return parts, valid
}

func agreementCodeWindowParts(raw string) ([][]agreementCodePart, bool) {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return nil, false
	}
	var windows [][]agreementCodePart
	valid := true
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "code" {
			parts, owned := agreementCodeParts(n)
			valid = valid && owned
			windows = append(windows, parts)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return windows, valid
}

func agreementNativeCodeSpan(n ast.Node) (graph.Span, bool) {
	switch node := n.(type) {
	case *ast.CodeSpan:
		first, firstOK := node.FirstChild().(*ast.Text)
		last, lastOK := node.LastChild().(*ast.Text)
		if !firstOK || !lastOK {
			return graph.Span{}, false
		}
		return graph.Span{Start: first.Segment.Start, Stop: last.Segment.Stop}, true
	case *ast.CodeBlock, *ast.FencedCodeBlock:
		if node.Lines().Len() == 0 {
			return graph.Span{}, true
		}
		return graph.Span{Start: node.Lines().At(0).Start, Stop: node.Lines().At(node.Lines().Len() - 1).Stop}, true
	default:
		return graph.Span{}, false
	}
}

func agreementNativeCodeWindows(body string, grammar goldmark.Markdown) ([]agreementNativeCodeWindow, bool) {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var windows []agreementNativeCodeWindow
	valid := true
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		span, code := agreementNativeCodeSpan(node)
		if !code {
			return ast.WalkContinue, nil
		}
		var buf bytes.Buffer
		if err := grammar.Renderer().Render(&buf, source, node); err != nil {
			valid = false
			return ast.WalkStop, err
		}
		parts, owned := agreementCodeWindowParts(buf.String())
		if !owned || len(parts) != 1 {
			valid = false
			return ast.WalkStop, nil
		}
		var literal strings.Builder
		for _, part := range parts[0] {
			if part.Field {
				valid = false
				return ast.WalkStop, nil
			}
			literal.WriteString(part.Text)
		}
		windows = append(windows, agreementNativeCodeWindow{Kind: node.Kind().String(), Span: span, Text: literal.String()})
		return ast.WalkContinue, nil
	}); err != nil {
		return nil, false
	}
	return windows, valid
}

func agreementMatchCodeWindow(literal string, parts []agreementCodePart) (map[agreementCitation]int, bool) {
	budget := make(map[agreementCitation]int)
	offset := 0
	for _, part := range parts {
		if !part.Field {
			if !strings.HasPrefix(literal[offset:], part.Text) {
				return nil, false
			}
			offset += len(part.Text)
			continue
		}
		if !strings.HasPrefix(literal[offset:], "[[") || graph.EscapedWikilinkAt(literal, offset) || offset > 0 && literal[offset-1] == '!' {
			return nil, false
		}
		inner, tail, closed := strings.Cut(literal[offset+2:], "]]")
		if !closed || strings.ContainsAny(inner, "[]\r\n") {
			return nil, false
		}
		link, cites := graph.ParseWikilink(inner)
		if !cites || link.Block != "" || part.Citation != (agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}) || part.Text != link.Display {
			return nil, false
		}
		offset = len(literal) - len(tail)
		budget[part.Citation]++
	}
	return budget, offset == len(literal)
}

type agreementCodePayload struct {
	Body   string
	Tuples map[agreementCitation]int
}

func agreementCodeWindowBudget(body, raw string) agreementCodePayload {
	plain, plainOK := agreementNativeCodeWindows(body, agreementFootnoteGrammar)
	gfm, gfmOK := agreementNativeCodeWindows(body, agreementExclusiveCodeGrammar)
	if !plainOK || !gfmOK || !cmp.Equal(plain, gfm) {
		return agreementCodePayload{}
	}
	actual, actualOK := agreementCodeWindowParts(raw)
	if !actualOK || len(plain) != len(actual) {
		return agreementCodePayload{}
	}
	budget := make(map[agreementCitation]int)
	for i, declared := range plain {
		matched, owned := agreementMatchCodeWindow(declared.Text, actual[i])
		if !owned {
			return agreementCodePayload{}
		}
		for tuple, count := range matched {
			budget[tuple] += count
		}
	}
	if len(budget) == 0 {
		return agreementCodePayload{}
	}
	return agreementCodePayload{Body: body, Tuples: budget}
}

func agreementCodeWindowDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementCodePayload) (kind, authority, wrong string) {
	if c.Body != budget.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P2" || f.Identity != "wikilink-in-code" || f.Direction != "page-in-code" || f.Tuple == (agreementCitation{}) || f.Multiplicity <= 0 || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || budget.Tuples[f.Tuple] != f.Multiplicity {
		return "", "", ""
	}
	observed := make(map[agreementCitation]int)
	for _, tuple := range actual.CodeCitations {
		observed[tuple]++
	}
	if !cmp.Equal(budget.Tuples, observed) || actual.CitationsInCode != len(actual.CodeCitations) {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "page"
}

func TestAgreementCodePayloadSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       []agreementNativeCodeWindow
	}{
		{
			name: "every code kind and payload",
			body: "[[A]]\n\n`open\n[[A#place|shown]]\nclose`\n\n```go\n[[B]]\n```\n\n    [[C]]\n",
			want: []agreementNativeCodeWindow{
				{Kind: "CodeSpan", Span: graph.Span{Start: 8, Stop: 36}, Text: "open [[A#place|shown]] close"},
				{Kind: "FencedCodeBlock", Span: graph.Span{Start: 45, Stop: 51}, Text: "[[B]]\n"},
				{Kind: "CodeBlock", Span: graph.Span{Start: 60, Stop: 66}, Text: "[[C]]\n"},
			},
		},
		{
			name: "all direct heading code children",
			body: "## `one` `[[A]]`\n",
			want: []agreementNativeCodeWindow{
				{Kind: "CodeSpan", Span: graph.Span{Start: 4, Stop: 7}, Text: "one"},
				{Kind: "CodeSpan", Span: graph.Span{Start: 10, Stop: 15}, Text: "[[A]]"},
			},
		},
		{
			name: "container content coordinates",
			body: "> ```go\n> [[A]]\n> ```\n",
			want: []agreementNativeCodeWindow{{Kind: "FencedCodeBlock", Span: graph.Span{Start: 10, Stop: 16}, Text: "[[A]]\n"}},
		},
		{
			name: "used footnote content",
			body: "ref[^n]\n\n[^n]: `open\n    [[A]]\n    close`\n",
			want: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 16, Stop: 40}, Text: "open [[A]] close"}},
		},
		{
			name: "empty fence remains a declaration",
			body: "```go\n```\n",
			want: []agreementNativeCodeWindow{{Kind: "FencedCodeBlock"}},
		},
		{name: "unused definition contributes no code", body: "[^n]: `[[A]]`\n"},
		{name: "ordinary prose contributes no code", body: "[[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				got, valid := agreementNativeCodeWindows(tc.body, grammar)
				if diff := cmp.Diff(tc.want, got); !valid || diff != "" {
					t.Fatalf("caught: complete native code payload inventory valid=%t (-want +got):\n%s", valid, diff)
				}
			}
		})
	}
}

func TestAgreementCodePayloadMatcher(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", Section: "place", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, literal string
		parts         []agreementCodePart
		want          map[agreementCitation]int
		valid         bool
	}{
		{name: "whole field and literal suffix", literal: "before [[A]] after", parts: []agreementCodePart{{Text: "before "}, {Text: "A", Citation: a, Field: true}, {Text: " after"}}, want: map[agreementCitation]int{a: 1}, valid: true},
		{name: "all adjacent field tuples", literal: "[[A]][[B#place|shown]][[A]]", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}, {Text: "shown", Citation: b, Field: true}, {Text: "A", Citation: a, Field: true}}, want: map[agreementCitation]int{a: 2, b: 1}, valid: true},
		{name: "unconverted literal beside a carrier", literal: "[[A]] [[A]]", parts: []agreementCodePart{{Text: "[[A]] "}, {Text: "A", Citation: a, Field: true}}, want: map[agreementCitation]int{a: 1}, valid: true},
		{name: "entirely literal payload", literal: "[[A]]", parts: []agreementCodePart{{Text: "[[A]]"}}, want: map[agreementCitation]int{}, valid: true},
		{name: "prefix drift", literal: "before [[A]] after", parts: []agreementCodePart{{Text: "unownx "}, {Text: "A", Citation: a, Field: true}, {Text: " after"}}},
		{name: "suffix drift", literal: "[[A]] after", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}, {Text: " unowned"}}},
		{name: "omitted final literal bytes", literal: "[[A]] after", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}}, want: map[agreementCitation]int{a: 1}},
		{name: "literal bytes cannot invent a field opening", literal: "AA A]]", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}}},
		{name: "carrier without declared opening", literal: "words", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}}},
		{name: "incomplete field", literal: "[[A", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}}},
		{name: "nested field cannot start at outer opening", literal: "[[A[[B]]]]", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}}},
		{name: "nested syntax cannot invent its own target", literal: "[[A[[B|shown]]]]", parts: []agreementCodePart{{Text: "shown", Citation: agreementCitation{Target: "A[[B", State: "wikilink-broken"}, Field: true}, {Text: "]]"}}},
		{name: "wrapped field cannot lend another target", literal: "[[A\nB]]", parts: []agreementCodePart{{Text: "A", Citation: a, Field: true}}},
		{name: "wrapped syntax cannot invent a complete field", literal: "[[A\nB|shown]]", parts: []agreementCodePart{{Text: "shown", Citation: agreementCitation{Target: "A\nB", State: "wikilink-broken"}, Field: true}}},
		{name: "escaped field remains literal", literal: "\\[[A]]", parts: []agreementCodePart{{Text: "\\"}, {Text: "A", Citation: a, Field: true}}},
		{name: "embed remains another owner", literal: "![[A]]", parts: []agreementCodePart{{Text: "!"}, {Text: "A", Citation: a, Field: true}}},
		{name: "local field remains another owner", literal: "[[#A]]", parts: []agreementCodePart{{Text: "#A", Citation: agreementCitation{Section: "A", State: "wikilink-broken"}, Field: true}}},
		{name: "block field remains another owner", literal: "[[A#^a]]", parts: []agreementCodePart{{Text: "A#^a", Citation: a, Field: true}}},
		{name: "tuple cannot borrow alias display", literal: "[[B#place|shown]]", parts: []agreementCodePart{{Text: "shown", Citation: a, Field: true}}},
		{name: "display cannot borrow target tuple", literal: "[[B#place|shown]]", parts: []agreementCodePart{{Text: "unowned", Citation: b, Field: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, valid := agreementMatchCodeWindow(tc.literal, tc.parts)
			if diff := cmp.Diff(tc.want, got); valid != tc.valid || diff != "" {
				t.Fatalf("caught: complete code payload field positions valid=%t want=%t (-want +got):\n%s", valid, tc.valid, diff)
			}
		})
	}
}

func TestAgreementCodePayloadPublic(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	place := agreementCitation{Target: "A", Section: "place", State: "wikilink-broken"}
	raw := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
	}{
		{name: "ordinary same target cannot lend a carrier", body: "[[A]]\n\n`open\n[[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}},
		{name: "whole adjacent target and section set", body: "[[A#place]]\n\n`open\n[[A#place]] [[B|shown]] [[A#place]]\nclose`\n", want: map[agreementCitation]int{place: 2, b: 1}},
		{name: "raw suffix and alias remain distinct", body: "`open\n[[A\\]] [[A\\|shown]]\nclose`\n[[A]]\n", want: map[agreementCitation]int{raw: 1, a: 1}},
		{name: "all fields and surrounding text", body: "[[A]]\n\n`before [[A]]\n[[A]] after`\n", want: map[agreementCitation]int{a: 2}},
		{name: "quoted fence beside ordinary target", body: "[[A]]\n\n> ```\n> [[A]]\n> ```\n", want: map[agreementCitation]int{a: 1}},
		{name: "used footnote keeps original source", body: "ref[^n]\n\n[^n]: `open\n    [[A]]\n    close`\n", want: map[agreementCitation]int{a: 1}},
		{name: "literal single line code also participates", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\n`[[A]]`\n", want: map[agreementCitation]int{a: 1}},
		{name: "literal indented code also participates", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\n    [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "literal root fence also participates", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\n```\n[[A]]\n```\n", want: map[agreementCitation]int{a: 1}},
		{name: "escaped source field stays literal", body: "[[A]]\n\n`open\n\\[[A]] [[B]]\nclose`\n", want: map[agreementCitation]int{b: 1}},
		{name: "interior field retains its exact position", body: "[[A]]\n\n`open\n[[A[[B]]]]\nclose`\n", want: map[agreementCitation]int{b: 1}},
		{name: "terminal comment cannot borrow code", body: "[[A]]\n\n`open\n[[A]]\nclose`\n<!-- hidden\n", want: map[agreementCitation]int{a: 1}},
		{name: "both declared code payloads", body: "[[A]]\n\n`first\n[[A]]\nend`\n\n`second\n[[B]]\nlast`\n", want: map[agreementCitation]int{a: 1, b: 1}},
		{name: "same tuple in every declared payload", body: "[[A]]\n\n`first\n[[A]]\nend`\n\n`second\n[[A]]\nlast`\n", want: map[agreementCitation]int{a: 2}},
		{name: "recorded full source with several code owners", body: "## A\n## A\n`open\n[[A]]\nclose`~~~\n> [!note] title\nA\n---\n~~~\n> [!unknown] title\n    ```\n%%[[A]]%%É\nÉ\n> [!note] [[A]]\n> [!unknown] title\n\t```\nref[^n]\n# A\n章節\n~~~\n\\``\\`## !\n-->## A-2\n## A\n## A\n- [ ] [[A]]\n```` go [[A]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "different URL code owners refuse whole set", body: "[[A]]\n\n`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n"},
		{name: "hidden code has no page owner", body: "[[A]]\n\n%%\n`open\n[[A]]\nclose`\n%%\n"},
		{name: "embed fields require another owner", body: "[[A]]\n\n`open\n![[A]] [[B]]\nclose`\n"},
		{name: "block fields require another owner", body: "`open\n[[A#^a]]\nclose`\n"},
		{name: "ordinary prose has no code declaration", body: "[[A]]\n"},
		{name: "root fence has no citation", body: "```\n[[A]]\n```\n"},
		{name: "single line code has no citation", body: "`[[A]]`\n"},
		{name: "indented code has no citation", body: "    [[A]]\n"},
		{name: "unused code has no declaration", body: "[^n]: `open\n    [[A]]\n    close`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementCodeWindowBudget(c.Body, r.HTML)
			if diff := cmp.Diff(tc.want, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete code payload public inventory (-want +got):\n%s", diff)
			}
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementCodeWindowDifference(c, &f, &actual, budget)
				if f.Property != "P2" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: code payload borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: code payload public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementCodePayloadDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete code payload public receipts (-want +got):\n%s", diff)
			}
		})
	}
}

func agreementCodePayloadDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementCodePayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementCodeWindowDifference(other, f, actual, budget); kind != "" {
			t.Fatal("caught: code payload borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){
		func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
	} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementCodeWindowDifference(c, &changed, actual, budget); kind != "" {
			t.Fatalf("caught: code payload borrowed signature %s", agreementSignature(&changed))
		}
	}
	for _, carriers := range [][]agreementCitation{nil, append(append([]agreementCitation{}, actual.CodeCitations...), agreementCitation{Target: "unowned", State: "wikilink-broken"}), {{Target: f.Tuple.Target, Section: "unowned", State: "wikilink-broken"}}} {
		changed := *actual
		changed.CodeCitations = carriers
		if kind, _, _ := agreementCodeWindowDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: code payload borrowed absent extra or relocated carriers")
		}
	}
	changed := *actual
	changed.CitationsInCode++
	if kind, _, _ := agreementCodeWindowDifference(c, f, &changed, budget); kind != "" {
		t.Fatal("caught: code payload borrowed another code carrier kind")
	}
}

func TestAgreementCodePayloadHTMLDrift(t *testing.T) {
	t.Parallel()
	const body = "[[A]]\n\n`open\n[[A]]\nclose`\n"
	r, _ := agreementIsolatedPage(t, agreementCase{Body: body})
	for _, tc := range []struct {
		name, old, replacement string
	}{
		{name: "prefix bytes", old: "<code>open", replacement: "<code>unowned"},
		{name: "suffix bytes", old: " close</code>", replacement: " unowned</code>"},
		{name: "missing suffix", old: " close</code>", replacement: "</code>"},
		{name: "missing code owner", old: "<code>", replacement: "<span>"},
		{name: "nested code owner", old: "<code>", replacement: "<code><code>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if strings.Count(r.HTML, tc.old) != 1 {
				t.Fatal("code drift does not identify one declared page site")
			}
			changed := strings.Replace(r.HTML, tc.old, tc.replacement, 1)
			if agreementCodeWindowBudget(body, changed).Tuples != nil {
				t.Fatal("caught: code payload borrowed changed page bytes or owner")
			}
		})
	}
	if agreementCodeWindowBudget(body, r.HTML+"<code>unowned</code>").Tuples != nil {
		t.Fatal("caught: code payload borrowed extra page declaration")
	}
	const otherBody = "`first\n[[A]]\nend`\n\n`second\n[[B]]\nlast`\n"
	other, _ := agreementIsolatedPage(t, agreementCase{Body: otherBody})
	changed := strings.NewReplacer("first", "second", "second", "first", "end", "last", "last", "end").Replace(other.HTML)
	if agreementCodeWindowBudget(otherBody, changed).Tuples != nil {
		t.Fatal("caught: code payload borrowed reordered page declarations")
	}
	const urlBody = "[[A]]\n\n`open\n[[A]]\nclose`\n\nhttps://example.invalid/`[[A]]`\n"
	urlPage, _ := agreementIsolatedPage(t, agreementCase{Body: urlBody})
	if agreementCodeWindowBudget(urlBody, urlPage.HTML+"<code>[[A]]</code>").Tuples != nil {
		t.Fatal("caught: code payload borrowed another grammar's complete code set")
	}
	parts, valid := agreementCodeWindowParts("<code><span>one</span><span>two</span></code>")
	want := [][]agreementCodePart{{{Text: "onetwo"}}}
	if !valid || !cmp.Equal(parts, want) {
		t.Fatal("caught: complete code payload text across formatting nodes")
	}
	if _, valid := agreementCodeWindowParts("<code><code>onetwo</code></code>"); valid {
		t.Fatal("caught: code payload borrowed a nested code owner")
	}
}

func TestAgreementCodePayloadCarrierShape(t *testing.T) {
	t.Parallel()
	r, _ := agreementIsolatedPage(t, agreementCase{Body: "`open\n[[A]]\nclose`\n"})
	for _, tc := range []struct {
		name string
		edit func(*html.Node)
	}{
		{name: "other carrier state", edit: func(n *html.Node) { agreementCodePayloadAttr(n, "class", "wikilink-broken wikilink-title-only") }},
		{name: "unknown notice", edit: func(n *html.Node) { agreementCodePayloadAttr(n, "title", "unowned") }},
		{name: "unowned attribute", edit: func(n *html.Node) { n.Attr = append(n.Attr, html.Attribute{Key: "href", Val: "/unowned"}) }},
		{name: "unowned child element", edit: func(n *html.Node) { n.AppendChild(&html.Node{Type: html.ElementNode, Data: "b"}) }},
		{name: "unowned child class", edit: func(n *html.Node) { agreementCodePayloadAttr(n.LastChild, "class", "unowned") }},
		{name: "missing explanation", edit: func(n *html.Node) { n.RemoveChild(n.LastChild) }},
		{name: "duplicate explanation", edit: func(n *html.Node) {
			n.AppendChild(&html.Node{Type: html.ElementNode, Data: "span", Attr: []html.Attribute{{Key: "class", Val: "y-offscreen"}}})
		}},
		{name: "hidden element cannot contain another owner", edit: func(n *html.Node) { n.LastChild.AppendChild(&html.Node{Type: html.ElementNode, Data: "code"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc, err := html.Parse(strings.NewReader(r.HTML))
			if err != nil {
				t.Fatal(err)
			}
			var carriers []*html.Node
			var walk func(*html.Node)
			walk = func(n *html.Node) {
				if n.Type == html.ElementNode && agreementCarrier(n) {
					carriers = append(carriers, n)
				}
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(doc)
			if len(carriers) != 1 {
				t.Fatal("carrier shape does not identify one declared page site")
			}
			part, valid := agreementCodeCarrierPart(carriers[0])
			want := agreementCodePart{Text: "A", Citation: agreementCitation{Target: "A", State: "wikilink-broken"}, Field: true}
			if !valid || !cmp.Equal(part, want) {
				t.Fatal("caught: complete code payload carrier shape")
			}
			tc.edit(carriers[0])
			if _, valid := agreementCodeCarrierPart(carriers[0]); valid {
				t.Fatal("caught: code payload borrowed malformed carrier shape")
			}
		})
	}
}

func agreementCodePayloadAttr(n *html.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
}
