package judge_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

type agreementNativeHTMLAddressSite struct {
	Root, Node, Parent  int
	Kind, ParentKind    string
	Type                ast.HTMLBlockType
	Lines               []text.Segment
	Closure, Info       text.Segment
	HasClosure, HasInfo bool
	Raw                 bool
}

// HTML closing lines and empty fences are native declarations too. Keep every
// such container, including declarations in unused footnotes, with its complete
// segments rather than borrowing a paragraph's line list.
func agreementNativeHTMLAddressSites(body string) []agreementNativeHTMLAddressSite {
	ctx := parser.NewContext()
	ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	ctx.Set(agreementUnusedFootnoteNodesKey, &unused)
	ctx.Set(agreementNativeDefinitionSpansKey, make(map[ast.Node]agreementNativeDefinitionSpan))
	doc := agreementNativeOwnerGrammar.Parser().Parse(text.NewReader([]byte(body)), parser.WithContext(ctx))
	roots := []ast.Node{doc}
	for _, n := range unused {
		roots = append(roots, n)
	}
	var sites []agreementNativeHTMLAddressSite
	for ri, root := range roots {
		at := 0
		var parents []int
		if err := ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				parents = parents[:len(parents)-1]
				return ast.WalkContinue, nil
			}
			parent := -1
			if len(parents) > 0 {
				parent = parents[len(parents)-1]
			}
			current := at
			at++
			parents = append(parents, current)
			site := agreementNativeHTMLAddressSite{Root: ri, Node: current, Parent: parent, Kind: n.Kind().String(), Raw: n.IsRaw()}
			if n.Parent() != nil {
				site.ParentKind = n.Parent().Kind().String()
			}
			switch h := n.(type) {
			case *ast.HTMLBlock:
				site.Type, site.Closure, site.HasClosure = h.HTMLBlockType, h.ClosureLine, h.HasClosure()
				for i := range h.Lines().Len() {
					site.Lines = append(site.Lines, h.Lines().At(i))
				}
			case *ast.RawHTML:
				for i := range h.Segments.Len() {
					site.Lines = append(site.Lines, h.Segments.At(i))
				}
			case *ast.FencedCodeBlock:
				for i := range h.Lines().Len() {
					site.Lines = append(site.Lines, h.Lines().At(i))
				}
				if h.Info != nil {
					site.Info, site.HasInfo = h.Info.Segment, true
				}
			default:
				return ast.WalkContinue, nil
			}
			sites = append(sites, site)
			return ast.WalkContinue, nil
		}); err != nil {
			panic(err)
		}
	}
	return sites
}

type agreementNativeHTMLAddressWitness struct {
	Name, Body, Repair, Authority, Wrong string
	Sites                                []agreementNativeHTMLAddressSite
	Code                                 []agreementNativeOwnerCode
	Names, Blocks, RepairBlocks          []string
	Original, Repaired                   map[string]agreementNativeAddressReceipt
}

// These complete original bodies name finite disagreements. Literal HTML
// backticks must not manufacture a fence; percent bytes owned by code must not
// hide a later address; a quoted fence ends with its native quote container.
// Literal callout openers in HTML must not create an unused footnote.
func agreementNativeHTMLAddressWitnesses() []agreementNativeHTMLAddressWitness {
	return []agreementNativeHTMLAddressWitness{
		{Name: "literal HTML callout openers manufacture an unused footnote", Body: "\n\n[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   ```\n   ```\n> [!note] one\n> [!note] two\n> [!note] three\n[^unused]: [[A]]\n`open\n[[A]]\nclose` ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t```\n``  > [!note] title\n````\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]", Repair: "\n\n[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   ```\n   ```\n> [!note] one\n> [!note] two\n> [!note] three\nx^unused]: [[A]]\n`open\n[[A]]\nclose` ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t```\n``  > [!note] title\n````\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]", Authority: "#1011 stage 6", Wrong: "page", Sites: []agreementNativeHTMLAddressSite{{Root: 0, Node: 6, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 6,
			Lines: []text.Segment{{Start: 17, Stop: 23, Padding: 0, ForceNewline: false}, {Start: 23, Stop: 29, Padding: 0, ForceNewline: false}, {Start: 29, Stop: 36, Padding: 0, ForceNewline: false}, {Start: 36, Stop: 43, Padding: 0, ForceNewline: false}, {Start: 43, Stop: 50, Padding: 0, ForceNewline: false}, {Start: 50, Stop: 64, Padding: 0, ForceNewline: false}, {Start: 64, Stop: 78, Padding: 0, ForceNewline: false}, {Start: 78, Stop: 94, Padding: 0, ForceNewline: false}, {Start: 94, Stop: 111, Padding: 0, ForceNewline: false}, {Start: 111, Stop: 117, Padding: 0, ForceNewline: false}, {Start: 117, Stop: 123, Padding: 0, ForceNewline: false}, {Start: 123, Stop: 133, Padding: 0, ForceNewline: false}, {Start: 133, Stop: 135, Padding: 0, ForceNewline: false}, {Start: 135, Stop: 139, Padding: 0, ForceNewline: false}, {Start: 139, Stop: 142, Padding: 0, ForceNewline: false}, {Start: 142, Stop: 149, Padding: 0, ForceNewline: false}, {Start: 149, Stop: 154, Padding: 0, ForceNewline: false}, {Start: 154, Stop: 159, Padding: 0, ForceNewline: false}, {Start: 159, Stop: 164, Padding: 0, ForceNewline: false}, {Start: 164, Stop: 184, Padding: 0, ForceNewline: false}, {Start: 184, Stop: 189, Padding: 0, ForceNewline: false}, {Start: 189, Stop: 203, Padding: 0, ForceNewline: false}, {Start: 203, Stop: 217, Padding: 0, ForceNewline: false}, {Start: 217, Stop: 233, Padding: 0, ForceNewline: false}, {Start: 233, Stop: 243, Padding: 0, ForceNewline: false}, {Start: 243, Stop: 276, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Code: nil, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^é"}, Blocks: nil, RepairBlocks: []string{"^a"}, Original: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   ```\n   ```\n> [!note] one\n> [!note] two\n> [!note] three\n[^unused]: [[A]]\n`open\n[[A]]\nclose` ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t```\n``  > [!note] title\n````\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}, Repaired: map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: true, Found: true, Cut: "[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   ```\n   ```\n> [!note] one\n> [!note] two\n> [!note] three\nx^unused]: [[A]]\n`open\n[[A]]\nclose` ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t```\n``  > [!note] title\n````\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}},
		{Name: "literal HTML backticks manufacture a fence", Body: "> [!note] [[A]]\n> <!-- [[A]] -->```` go [[A]]\nref[^n]\n%%[[A]]%%<div>\n[[A]]\n</div>\n1.  ^a-2\n![[image.png]]-  ^é\n[[A\\]][[A\nB]]B\n[[A]] [[A]][[A]]> > \\\\[[A]]![[A]]\n\\\\[[A]]É\n> ~~~\n## <em>A</em>\n<div>\n[[A]]\n</div>\n`` ^é\n", Repair: "> [!note] [[A]]\n> <!-- [[A]] -->x``` go [[A]]\nref[^n]\n%%[[A]]%%<div>\n[[A]]\n</div>\n1.  ^a-2\n![[image.png]]-  ^é\n[[A\\]][[A\nB]]B\n[[A]] [[A]][[A]]> > \\\\[[A]]![[A]]\n\\\\[[A]]É\n> ~~~\n## <em>A</em>\n<div>\n[[A]]\n</div>\n`` ^é\n", Authority: "#1011 stage 5", Wrong: "judge+excerpt", Sites: []agreementNativeHTMLAddressSite{{Root: 0, Node: 8, Parent: 1, Kind: "HTMLBlock", ParentKind: "Blockquote", Type: 2,
			Lines: []text.Segment{{Start: 18, Stop: 46, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true},
			{Root: 0, Node: 16, Parent: 9, Kind: "RawHTML", ParentKind: "Paragraph", Type: 0,
				Lines: []text.Segment{{Start: 63, Stop: 68, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false},
			{Root: 0, Node: 22, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 6,
				Lines: []text.Segment{{Start: 75, Stop: 82, Padding: 0, ForceNewline: false}, {Start: 82, Stop: 91, Padding: 0, ForceNewline: false}, {Start: 91, Stop: 112, Padding: 0, ForceNewline: false}, {Start: 112, Stop: 122, Padding: 0, ForceNewline: false}, {Start: 122, Stop: 127, Padding: 0, ForceNewline: false}, {Start: 127, Stop: 161, Padding: 0, ForceNewline: false}, {Start: 161, Stop: 171, Padding: 0, ForceNewline: false}, {Start: 171, Stop: 177, Padding: 0, ForceNewline: false}, {Start: 177, Stop: 191, Padding: 0, ForceNewline: false}, {Start: 191, Stop: 197, Padding: 0, ForceNewline: false}, {Start: 197, Stop: 203, Padding: 0, ForceNewline: false}, {Start: 203, Stop: 210, Padding: 0, ForceNewline: false}, {Start: 210, Stop: 217, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Code: nil, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^é"}, Blocks: []string{"^a-2"}, RepairBlocks: []string{"^a-2"}, Original: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: true, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}, Repaired: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: true, Judge: true, Found: true, Cut: "1.  ^a-2\n![[image.png]]-  ^é\n[[A\\]][[A\nB]]B\n[[A]] [[A]][[A]]> > \\\\[[A]]![[A]]\n\\\\[[A]]É\n> ~~~\n## <em>A</em>\n<div>\n[[A]]\n</div>\n`` ^é"}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}},
		{Name: "native code percent bytes erase an HTML address", Body: "\thttps://example.invalid/`[[A]]` %%É\n[[B|alias]]B\n<!-- [[A]] --> ^a-2\n````\n ``` [[A]]\n    ```\n\n\\[[A]]A\n> [!note] [[A]]\n    ```\n[[A\nB]] ```\n> [!unknown] title\n\t## [[A|alias]]\n[[A]] [[A]]## [[A|alias]]\n ```\n[[A#A]]\\\\[[A]] ^é\n> ![[A]]", Repair: "\thttps://example.invalid/`[[A]]` xxÉ\n[[B|alias]]B\n<!-- [[A]] --> ^a-2\n````\n ``` [[A]]\n    ```\n\n\\[[A]]A\n> [!note] [[A]]\n    ```\n[[A\nB]] ```\n> [!unknown] title\n\t## [[A|alias]]\n[[A]] [[A]]## [[A|alias]]\n ```\n[[A#A]]\\\\[[A]] ^é\n> ![[A]]", Authority: "#1011 stage 5", Wrong: "page+excerpt", Sites: []agreementNativeHTMLAddressSite{{Root: 0, Node: 7, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2,
			Lines: []text.Segment{{Start: 52, Stop: 72, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true},
			{Root: 0, Node: 8, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Type: 0,
				Lines: []text.Segment{{Start: 77, Stop: 88, Padding: 0, ForceNewline: true}, {Start: 88, Stop: 96, Padding: 0, ForceNewline: true}, {Start: 96, Stop: 97, Padding: 0, ForceNewline: true}, {Start: 97, Stop: 105, Padding: 0, ForceNewline: true}, {Start: 105, Stop: 121, Padding: 0, ForceNewline: true}, {Start: 121, Stop: 129, Padding: 0, ForceNewline: true}, {Start: 129, Stop: 133, Padding: 0, ForceNewline: true}, {Start: 133, Stop: 141, Padding: 0, ForceNewline: true}, {Start: 141, Stop: 160, Padding: 0, ForceNewline: true}, {Start: 160, Stop: 176, Padding: 0, ForceNewline: true}, {Start: 176, Stop: 202, Padding: 0, ForceNewline: true}, {Start: 202, Stop: 207, Padding: 0, ForceNewline: true}, {Start: 207, Stop: 226, Padding: 0, ForceNewline: true}, {Start: 226, Stop: 234, Padding: 0, ForceNewline: true}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 1, Stop: 39}}, {Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 77, Stop: 234}}}, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^é"}, Blocks: nil, RepairBlocks: []string{"^a-2"}, Original: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: true, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}, Repaired: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: true, Judge: true, Found: true, Cut: "\thttps://example.invalid/`[[A]]` xxÉ\n[[B|alias]]B\n ^a-2\n````\n ``` [[A]]\n    ```"}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}},
		{Name: "empty quoted fence outlives its native container", Body: "É\n## A\n> ``` [[A]]\n\n\n> >     > [!note] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]``` [[A]]\n[[A\\|alias]]    ```\n<!-- [[A]] -->text [[A\nB]]ref[^n]\n## A-2\n## A\n## A\n## <em>A</em>\n<!--\n[[A]]\n-->`` ^a-2\n[^n]: [[A]]\n\n    [[B]]\nB\n- [x] [[B]]\n", Repair: "", Authority: "#1011 stage 3", Wrong: "judge+excerpt", Sites: []agreementNativeHTMLAddressSite{{Root: 0, Node: 6, Parent: 5, Kind: "FencedCodeBlock", ParentKind: "Blockquote", Type: 0,
			Lines: nil, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 14, Stop: 19, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: true, Raw: true},
			{Root: 0, Node: 10, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2,
				Lines: []text.Segment{{Start: 106, Stop: 129, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true},
			{Root: 0, Node: 21, Parent: 20, Kind: "RawHTML", ParentKind: "Heading", Type: 0,
				Lines: []text.Segment{{Start: 160, Stop: 164, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false},
			{Root: 0, Node: 23, Parent: 20, Kind: "RawHTML", ParentKind: "Heading", Type: 0,
				Lines: []text.Segment{{Start: 165, Stop: 170, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false},
			{Root: 0, Node: 24, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2,
				Lines: []text.Segment{{Start: 171, Stop: 176, Padding: 0, ForceNewline: false}, {Start: 176, Stop: 182, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 182, Stop: 193, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}}, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlockInfo", Parent: "Blockquote", Span: graph.Span{Start: 14, Stop: 19}}, {Kind: "CodeBlock", Parent: "Blockquote", Span: graph.Span{Start: 30, Stop: 46}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 80, Stop: 101}}}, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^é"}, Blocks: []string{"^a-2"}, RepairBlocks: nil, Original: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: true, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}, Repaired: nil},
	}
}

func agreementNativeHTMLAddressDifference(c agreementCase, failure *agreementFailure, actual *agreementHTML, fragments []agreementFailure) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || failure.Property != "P3" || failure.Identity != "block-three-way" || failure.Tuple != (agreementCitation{}) || failure.Multiplicity != 1 {
		return "", "", ""
	}
	witnesses := agreementNativeHTMLAddressWitnesses()
	for i := range witnesses {
		witness := &witnesses[i]
		if c.Body != witness.Body {
			continue
		}
		declared := agreementNativeAddressPartReading(c.Body)
		tree := agreementNativeProseAddressTreeReading(c.Body, declared.Fields)
		signatures := agreementNativeAddressSignatures(witness.Original)
		names := append(agreementCandidates(c.Body, actual), "^agreement-absent-0")
		slices.Sort(names)
		names = slices.Compact(names)
		if !tree.Valid || !cmp.Equal(tree.Code, witness.Code) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), witness.Sites) || !slices.Equal(names, witness.Names) || !cmp.Equal(actual.Blocks, witness.Blocks) || !cmp.Equal(agreementNativeAddressCurrent(fragments), signatures) || signatures[agreementSignature(failure)] != 1 {
			return "", "", ""
		}
		return "debt", witness.Authority, witness.Wrong
	}
	return "", "", ""
}

func TestAgreementNativeHTMLAddressSource(t *testing.T) {
	t.Parallel()
	for _, witness := range agreementNativeHTMLAddressWitnesses() {
		t.Run(witness.Name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(witness.Sites, agreementNativeHTMLAddressSites(witness.Body)); diff != "" {
				t.Fatalf("caught: complete native HTML and fence declarations (-want +got):\n%s", diff)
			}
			fields := agreementNativeAddressPartReading(witness.Body).Fields
			if diff := cmp.Diff(witness.Code, agreementNativeProseAddressTreeReading(witness.Body, fields).Code); diff != "" {
				t.Fatalf("caught: complete native code declarations (-want +got):\n%s", diff)
			}
		})
	}
	for _, tc := range []struct {
		Name, Body string
		Sites      []agreementNativeHTMLAddressSite
	}{
		{Name: "quoted empty fence with complete HTML closure", Body: "> ``` go\n\n<!--\ninside\n--> ^a\n", Sites: []agreementNativeHTMLAddressSite{{Root: 0, Node: 2, Parent: 1, Kind: "FencedCodeBlock", ParentKind: "Blockquote", Type: 0,
			Lines: nil, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 6, Stop: 8, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: true, Raw: true},
			{Root: 0, Node: 3, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2,
				Lines: []text.Segment{{Start: 10, Stop: 15, Padding: 0, ForceNewline: false}, {Start: 15, Stop: 22, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 22, Stop: 29, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}}},
		{Name: "two unused roots keep separate HTML declarations", Body: "[^one]:\n    <div>\n    first\n    </div>\n\n[^two]:\n    <!--\n    second\n    -->\n", Sites: []agreementNativeHTMLAddressSite{{Root: 1, Node: 1, Parent: 0, Kind: "HTMLBlock", ParentKind: "Footnote", Type: 6,
			Lines: []text.Segment{{Start: 12, Stop: 18, Padding: 0, ForceNewline: false}, {Start: 22, Stop: 28, Padding: 0, ForceNewline: false}, {Start: 32, Stop: 39, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true},
			{Root: 2, Node: 1, Parent: 0, Kind: "HTMLBlock", ParentKind: "Footnote", Type: 2,
				Lines: []text.Segment{{Start: 52, Stop: 57, Padding: 0, ForceNewline: false}, {Start: 61, Stop: 68, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 72, Stop: 76, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}}},
		{Name: "padding stays in the source declaration", Body: "- <div>\n  x\n  </div>\n", Sites: []agreementNativeHTMLAddressSite{{Root: 0, Node: 3, Parent: 2, Kind: "HTMLBlock", ParentKind: "ListItem", Type: 6,
			Lines: []text.Segment{{Start: 2, Stop: 8, Padding: 0, ForceNewline: false}, {Start: 10, Stop: 12, Padding: 0, ForceNewline: false}, {Start: 14, Stop: 21, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.Sites, agreementNativeHTMLAddressSites(tc.Body)); diff != "" {
				t.Fatalf("caught: complete native HTML and fence declarations (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementNativeHTMLAddressReceipts(t *testing.T) {
	t.Parallel()
	for _, witness := range agreementNativeHTMLAddressWitnesses() {
		t.Run(witness.Name, func(t *testing.T) {
			t.Parallel()
			_, actual := agreementIsolatedPage(t, agreementCase{Body: witness.Body})
			if diff := cmp.Diff(witness.Blocks, actual.Blocks); diff != "" {
				t.Fatalf("caught: complete original page address set (-want +got):\n%s", diff)
			}
			original := agreementNativeAddressPublic(t, witness.Body, &actual, witness.Names)
			if diff := cmp.Diff(witness.Original, original); diff != "" {
				t.Fatalf("caught: literal original public address ledger (-want +got):\n%s", diff)
			}
			if witness.Repair == "" {
				return
			}
			originalSource, repairedSource := agreementNativeQuoteAddressReading(witness.Body), agreementNativeQuoteAddressReading(witness.Repair)
			originalFacts := []any{originalSource.Address.Original, originalSource.Original, originalSource.Lists, originalSource.Quotes, agreementNativeHTMLAddressSites(witness.Body)}
			repairedFacts := []any{repairedSource.Address.Original, repairedSource.Original, repairedSource.Lists, repairedSource.Quotes, agreementNativeHTMLAddressSites(witness.Repair)}
			if diff := cmp.Diff(originalFacts, repairedFacts); diff != "" {
				t.Fatalf("caught: repair borrowed a different native container (-want +got):\n%s", diff)
			}
			_, repaired := agreementIsolatedPage(t, agreementCase{Body: witness.Repair})
			if diff := cmp.Diff(witness.RepairBlocks, repaired.Blocks); diff != "" {
				t.Fatalf("caught: complete repaired page address set (-want +got):\n%s", diff)
			}
			got := agreementNativeAddressPublic(t, witness.Repair, &repaired, witness.Names)
			if diff := cmp.Diff(witness.Repaired, got); diff != "" {
				t.Fatalf("caught: literal repaired public address ledger (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementNativeHTMLAddressDifferences(t *testing.T) {
	t.Parallel()
	for _, witness := range agreementNativeHTMLAddressWitnesses() {
		t.Run(witness.Name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: witness.Body}
			_, actual := agreementIsolatedPage(t, c)
			batches := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})
			signatures := agreementNativeAddressSignatures(witness.Original)
			if diff := cmp.Diff(signatures, agreementNativeAddressCurrent(batches[0])); diff != "" {
				t.Fatalf("caught: complete literal signed block receipts (-want +got):\n%s", diff)
			}
			for _, failure := range batches[0] {
				if failure.Property != "P3" {
					continue
				}
				kind, authority, wrong := agreementNativeHTMLAddressDifference(c, &failure, &actual, batches[0])
				if kind != "debt" || authority != witness.Authority || wrong != witness.Wrong {
					t.Fatalf("caught: native HTML block receipt lost its named authority: %q %q %q", kind, authority, wrong)
				}
				for _, alter := range []struct {
					Name   string
					Change func(*agreementFailure)
				}{
					{"different property", func(f *agreementFailure) { f.Property = "P4" }},
					{"different identity", func(f *agreementFailure) { f.Identity = "other" }},
					{"different tuple", func(f *agreementFailure) { f.Tuple.Target = "A" }},
					{"different fragment", func(f *agreementFailure) { f.Fragment = "^unrelated" }},
					{"different direction", func(f *agreementFailure) { f.Direction = "other" }},
					{"different multiplicity", func(f *agreementFailure) { f.Multiplicity++ }},
					{"different cut", func(f *agreementFailure) { f.Cut += "outside" }},
					{"different page", func(f *agreementFailure) { f.PagePresent = !f.PagePresent }},
					{"different judge", func(f *agreementFailure) { f.JudgeAccepted = !f.JudgeAccepted }},
					{"different excerpt", func(f *agreementFailure) { f.ExcerptFound = !f.ExcerptFound }},
				} {
					t.Run(failure.Direction+"/"+alter.Name, func(t *testing.T) {
						changed := failure
						alter.Change(&changed)
						gotKind, gotAuthority, gotWrong := agreementNativeHTMLAddressDifference(c, &changed, &actual, batches[0])
						if gotKind != "" || gotAuthority != "" || gotWrong != "" {
							t.Fatal("caught: native HTML receipt borrowed a different signed difference")
						}
					})
				}
				for _, other := range []agreementCase{{Body: witness.Body + "\n"}, {Body: witness.Body, Title: "A"}, {Body: witness.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
					gotKind, gotAuthority, gotWrong := agreementNativeHTMLAddressDifference(other, &failure, &actual, batches[0])
					if gotKind != "" || gotAuthority != "" || gotWrong != "" {
						t.Fatal("caught: native HTML receipt borrowed another complete body")
					}
				}
				changed := actual
				changed.Blocks = append(slices.Clone(actual.Blocks), "^outside")
				kind, authority, wrong = agreementNativeHTMLAddressDifference(c, &failure, &changed, batches[0])
				if kind != "" || authority != "" || wrong != "" {
					t.Fatal("caught: native HTML receipt borrowed a different complete page set")
				}
				extra := append(slices.Clone(batches[0]), failure)
				kind, authority, wrong = agreementNativeHTMLAddressDifference(c, &failure, &actual, extra)
				if kind != "" || authority != "" || wrong != "" {
					t.Fatal("caught: native HTML receipt borrowed an additional signed difference")
				}
			}
		})
	}
}

func TestAgreementNativeHTMLCalloutControl(t *testing.T) {
	t.Parallel()
	witness := agreementNativeHTMLAddressWitnesses()[0]
	originalSource := agreementNativeQuoteAddressReading(witness.Body)
	if len(originalSource.Address.Original.Native.Definitions) != 0 || len(originalSource.Original.Definitions) != 0 || len(originalSource.Quotes) != 0 || len(originalSource.Original.Code) != 0 {
		t.Fatal("caught: literal HTML acquired a native footnote, quote, or code declaration")
	}
	body := "\n\n[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   ```\n   ```\n  [!note] one\n  [!note] two\n  [!note] three\n[^unused]: [[A]]\n`open\n[[A]]\nclose` ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t```\n``    [!note] title\n````\n  [!note] one\n  [!note] two\n  [!note] three\n``` [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]"
	repairedSource := agreementNativeQuoteAddressReading(body)
	originalFacts := []any{originalSource.Address.Original, originalSource.Original, originalSource.Lists, originalSource.Quotes, agreementNativeHTMLAddressSites(witness.Body)}
	repairedFacts := []any{repairedSource.Address.Original, repairedSource.Original, repairedSource.Lists, repairedSource.Quotes, agreementNativeHTMLAddressSites(body)}
	if diff := cmp.Diff(originalFacts, repairedFacts); diff != "" {
		t.Fatalf("caught: literal callout control changed native declarations (-want +got):\n%s", diff)
	}
	_, actual := agreementIsolatedPage(t, agreementCase{Body: body})
	if diff := cmp.Diff([]string{"^a"}, actual.Blocks); diff != "" {
		t.Fatalf("caught: complete literal callout control page address set (-want +got):\n%s", diff)
	}
	want := map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: true, Found: true, Cut: "[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   ```\n   ```\n  [!note] one\n  [!note] two\n  [!note] three\n[^unused]: [[A]]\n`open\n[[A]]\nclose` ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t```\n``    [!note] title\n````\n  [!note] one\n  [!note] two\n  [!note] three\n``` [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	if diff := cmp.Diff(want, agreementNativeAddressPublic(t, body, &actual, witness.Names)); diff != "" {
		t.Fatalf("caught: literal callout control public address ledger (-want +got):\n%s", diff)
	}
}
