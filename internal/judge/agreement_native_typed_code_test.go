package judge_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// Every carrier binds one complete native code declaration.
func agreementNativeTypedCodeOwners(windows []agreementNativeCodeWindow, parts [][]agreementCodePart) ([]agreementNativeCodeCarrierOwner, map[agreementCitation]int, bool) {
	var declarations []agreementNativeCodeCarrierOwner
	owners := make(map[agreementCitation]int)
	used := make(map[int]bool)
	for pi, part := range parts {
		fields := false
		for _, word := range part {
			fields = fields || word.Field
		}
		if !fields {
			continue
		}
		candidate := -1
		var fieldsOwned map[agreementCitation]int
		for wi, window := range windows {
			switch window.Kind {
			case ast.KindCodeSpan.String(), ast.KindCodeBlock.String(), ast.KindFencedCodeBlock.String():
			default:
				return nil, nil, false
			}
			matched, valid := agreementMatchCodeWindow(window.Text, part)
			if !valid {
				continue
			}
			if candidate != -1 {
				return nil, nil, false
			}
			candidate = wi
			fieldsOwned = matched
		}
		if candidate < 0 || used[candidate] {
			return nil, nil, false
		}
		used[candidate] = true
		declarations = append(declarations, agreementNativeCodeCarrierOwner{Part: pi, Window: candidate})
		for tuple, count := range fieldsOwned {
			owners[tuple] += count
		}
	}
	return declarations, owners, len(owners) > 0
}

// Private targets replace only words wholly inside declared code content.
func agreementNativeTypedCodeRename(body string, windows []agreementNativeCodeWindow) (string, []agreementNativeCodePayloadTarget, bool) {
	if strings.Contains(body, "q") {
		return "", nil, false
	}
	private := []byte(body)
	var changes []agreementNativeCodePayloadTarget
	for wi, window := range windows {
		switch window.Kind {
		case ast.KindCodeSpan.String(), ast.KindCodeBlock.String(), ast.KindFencedCodeBlock.String():
		default:
			return "", nil, false
		}
		if window.Span.Start < 0 || window.Span.Stop > len(body) || window.Span.Start >= window.Span.Stop {
			return "", nil, false
		}
		for offset := window.Span.Start; offset < window.Span.Stop; {
			relative := strings.Index(body[offset:window.Span.Stop], "[[")
			if relative < 0 {
				break
			}
			open := offset + relative
			closeAt := strings.Index(body[open+2:window.Span.Stop], "]]")
			if closeAt < 0 {
				return "", nil, false
			}
			end := open + 2 + closeAt + 2
			offset = end
			inner := body[open+2 : end-2]
			link, ok := graph.ParseWikilink(inner)
			if !ok || strings.ContainsAny(inner, "[]\r\n`") || link.Target == "" {
				return "", nil, false
			}
			start := open + 2 + strings.Index(inner, link.Target)
			stop := start + len(link.Target)
			if start < open+2 || stop > end-2 || body[start:stop] != link.Target {
				return "", nil, false
			}
			name := strings.Repeat("q", len(link.Target))
			copy(private[start:stop], name)
			changes = append(changes, agreementNativeCodePayloadTarget{Window: wi, Span: graph.Span{Start: start, Stop: stop}, Old: link.Target, New: name})
		}
	}
	return string(private), changes, len(changes) > 0
}

type agreementNativeTypedCodeWitness struct {
	Name, Body, Private     string
	Native                  agreementNativeOwnerSource
	Check, PrivateCheck     agreementNativeCheckSource
	Sites                   []agreementNativeCodePayloadSite
	HTML                    []agreementNativeHTMLAddressSite
	Windows, PrivateWindows []agreementNativeCodeWindow
	Changes                 []agreementNativeCodePayloadTarget
	Parts, PrivateParts     [][]agreementCodePart
	Page, PrivatePage       agreementHTML
	Owners                  []agreementNativeCodeCarrierOwner
	Signatures              map[string]int
}

func agreementNativeTypedCodeWitnesses() []agreementNativeTypedCodeWitness {
	return []agreementNativeTypedCodeWitness{
		{Name: "252fc0790090", Body: "\t ```\n> \t```\n> [[A]]É\n0[[A#A]] ^a-2\nA\n> [!unknown] title\n![[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n- [ ] [[A]]\n\n## A-2\n## A\n## A\n%%> [!note] [[A]]\n```\n``\\`<!-- [[A]] -->``` [[A]]\n- > [!note] title\n\\`## [[A|alias]]\n[^unused]: [[A]]\n> [!note] [[A]]\n", Private: "\t ```\n> \t```\n> [[q]]É\n0[[A#A]] ^a-2\nA\n> [!unknown] title\n![[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n- [ ] [[A]]\n\n## A-2\n## A\n## A\n%%> [!note] [[A]]\n```\n``\\`<!-- [[q]] -->``` [[q]]\n- > [!note] title\n\\`## [[q|alias]]\n[^unused]: [[q]]\n> [!note] [[q]]\n", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 1, Stop: 6}}, {Kind: "FencedCodeBlock", Parent: "Blockquote", Span: graph.Span{Start: 15, Stop: 24}}, {Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 166, Stop: 262}}}, Comments: []graph.Span{{Start: 144, Stop: 262}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 1, Stop: 6}, {Start: 15, Stop: 24}, {Start: 166, Stop: 262}}, Comments: []graph.Span{{Start: 144, Stop: 262}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 15, Stop: 20}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 31, Stop: 38}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 66, Stop: 79}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 101, Stop: 106}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 120, Stop: 125}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 156, Stop: 161}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 175, Stop: 180}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 188, Stop: 193}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 217, Stop: 228}, Word: "[[A|alias]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 240, Stop: 245}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 256, Stop: 261}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}}, Targets: []string{"A", "image.png", "A", "A"}}, PrivateCheck: agreementNativeCheckSource{Code: []graph.Span{{Start: 1, Stop: 6}, {Start: 15, Stop: 24}, {Start: 166, Stop: 262}}, Comments: []graph.Span{{Start: 144, Stop: 262}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 15, Stop: 20}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 31, Stop: 38}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 66, Stop: 79}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 101, Stop: 106}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 120, Stop: 125}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 156, Stop: 161}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 175, Stop: 180}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 188, Stop: 193}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 217, Stop: 228}, Word: "[[q|alias]]", Targets: []string{"q"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 240, Stop: 245}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 256, Stop: 261}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}}, Targets: []string{"A", "image.png", "A", "A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 1, Parent: 0, Kind: "CodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 1, Stop: 6, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}, {Root: 0, Node: 3, Parent: 2, Kind: "FencedCodeBlock", ParentKind: "Blockquote", Lines: []text.Segment{{Start: 15, Stop: 24, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}, {Root: 0, Node: 44, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 166, Stop: 194, Padding: 0, ForceNewline: true}, {Start: 194, Stop: 212, Padding: 0, ForceNewline: true}, {Start: 212, Stop: 229, Padding: 0, ForceNewline: true}, {Start: 229, Stop: 246, Padding: 0, ForceNewline: true}, {Start: 246, Stop: 262, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 3, Parent: 2, Kind: "FencedCodeBlock", ParentKind: "Blockquote", Type: 0, Lines: []text.Segment{{Start: 15, Stop: 24, Padding: 0, ForceNewline: true}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}, {Root: 0, Node: 44, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Type: 0, Lines: []text.Segment{{Start: 166, Stop: 194, Padding: 0, ForceNewline: true}, {Start: 194, Stop: 212, Padding: 0, ForceNewline: true}, {Start: 212, Stop: 229, Padding: 0, ForceNewline: true}, {Start: 229, Stop: 246, Padding: 0, ForceNewline: true}, {Start: 246, Stop: 262, Padding: 0, ForceNewline: true}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Windows: []agreementNativeCodeWindow{{Kind: "CodeBlock", Span: graph.Span{Start: 1, Stop: 6}, Text: " ```\n"}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 15, Stop: 24}, Text: "[[A]]É\n"}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 166, Stop: 262}, Text: "``\\`<!-- [[A]] -->``` [[A]]\n- > [!note] title\n\\`## [[A|alias]]\n[^unused]: [[A]]\n> [!note] [[A]]\n"}}, PrivateWindows: []agreementNativeCodeWindow{{Kind: "CodeBlock", Span: graph.Span{Start: 1, Stop: 6}, Text: " ```\n"}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 15, Stop: 24}, Text: "[[q]]É\n"}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 166, Stop: 262}, Text: "``\\`<!-- [[q]] -->``` [[q]]\n- > [!note] title\n\\`## [[q|alias]]\n[^unused]: [[q]]\n> [!note] [[q]]\n"}}, Changes: []agreementNativeCodePayloadTarget{{Window: 1, Span: graph.Span{Start: 17, Stop: 18}, Old: "A", New: "q"}, {Window: 2, Span: graph.Span{Start: 177, Stop: 178}, Old: "A", New: "q"}, {Window: 2, Span: graph.Span{Start: 190, Stop: 191}, Old: "A", New: "q"}, {Window: 2, Span: graph.Span{Start: 219, Stop: 220}, Old: "A", New: "q"}, {Window: 2, Span: graph.Span{Start: 242, Stop: 243}, Old: "A", New: "q"}, {Window: 2, Span: graph.Span{Start: 258, Stop: 259}, Old: "A", New: "q"}}, Parts: [][]agreementCodePart{{{Text: " ```\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "A", Citation: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Field: true}, {Text: "É\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, PrivateParts: [][]agreementCodePart{{{Text: " ```\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "q", Citation: agreementCitation{SourceRole: "", Target: "q", Section: "", State: "wikilink-broken"}, Field: true}, {Text: "É\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "A", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CitationsInCode: 1, Blocks: []string{"^a-2"}, Headings: []string{"a-2", "a", "a-3"}, Failures: nil, CodeCitations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CalloutTitles: []string{"title"}}, PrivatePage: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "q", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "A", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CitationsInCode: 1, Blocks: []string{"^a-2"}, Headings: []string{"a-2", "a", "a-3"}, Failures: nil, CodeCitations: []agreementCitation{{SourceRole: "", Target: "q", Section: "", State: "wikilink-broken"}}, CalloutTitles: []string{"title"}}, Owners: []agreementNativeCodeCarrierOwner{{Part: 1, Window: 1}}, Signatures: map[string]int{"P2/wikilink-in-code tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=page-in-code multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}},
		{Name: "0e0bfd3baf4d", Body: "[[A]]- [ ] [[A]]\n![[image.png]]\nB\n<!--\n[[A]]\n-->%%[[A]]%%    ```\n## A-2\n## A\n## A\n ^A\n<div>\n[[A]]\n</div>\n\t[[A]] [[A]]É\n\n\n`open\n[[A]]\nclose`> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]]\t```\n[[image.png]]  ```\n````\n![[A#A]]章節\n", Private: "[[A]]- [ ] [[A]]\n![[image.png]]\nB\n<!--\n[[A]]\n-->%%[[A]]%%    ```\n## A-2\n## A\n## A\n ^A\n<div>\n[[A]]\n</div>\n\t[[A]] [[A]]É\n\n\n`open\n[[q]]\nclose`> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]]\t```\n[[qqqqqqqqq]]  ```\n````\n![[q#A]]章節\n", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 124, Stop: 140}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 203, Stop: 217}}, {Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 227, Stop: 242}}}, Comments: []graph.Span{{Start: 34, Stop: 48}, {Start: 48, Stop: 57}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 124, Stop: 140}, {Start: 203, Stop: 217}, {Start: 227, Stop: 242}}, Comments: []graph.Span{{Start: 34, Stop: 48}, {Start: 48, Stop: 57}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 11, Stop: 16}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 18, Stop: 31}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 39, Stop: 44}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 50, Stop: 55}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 92, Stop: 97}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 106, Stop: 111}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 112, Stop: 117}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 129, Stop: 134}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 185, Stop: 198}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 203, Stop: 216}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 228, Stop: 235}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}}, Targets: []string{"A", "A", "image.png", "A", "A", "A", "image.png"}}, PrivateCheck: agreementNativeCheckSource{Code: []graph.Span{{Start: 124, Stop: 140}, {Start: 203, Stop: 217}, {Start: 227, Stop: 242}}, Comments: []graph.Span{{Start: 34, Stop: 48}, {Start: 48, Stop: 57}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 11, Stop: 16}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 18, Stop: 31}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 39, Stop: 44}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 50, Stop: 55}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 92, Stop: 97}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 106, Stop: 111}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 112, Stop: 117}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 129, Stop: 134}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 185, Stop: 198}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 203, Stop: 216}, Word: "[[qqqqqqqqq]]", Targets: []string{"qqqqqqqqq"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 228, Stop: 235}, Word: "[[q#A]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}}, Targets: []string{"A", "A", "image.png", "A", "A", "A", "image.png"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 25, Parent: 24, Kind: "CodeSpan", ParentKind: "Paragraph", Lines: nil, Text: []text.Segment{{Start: 124, Stop: 129, Padding: 0, ForceNewline: false}, {Start: 129, Stop: 135, Padding: 0, ForceNewline: false}, {Start: 135, Stop: 140, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}, {Root: 0, Node: 43, Parent: 33, Kind: "CodeSpan", ParentKind: "Paragraph", Lines: nil, Text: []text.Segment{{Start: 203, Stop: 203, Padding: 0, ForceNewline: false}, {Start: 203, Stop: 217, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}, {Root: 0, Node: 46, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 227, Stop: 242, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 14, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 34, Stop: 39, Padding: 0, ForceNewline: false}, {Start: 39, Stop: 45, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 45, Stop: 65, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}, {Root: 0, Node: 23, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 6, Lines: []text.Segment{{Start: 86, Stop: 92, Padding: 0, ForceNewline: false}, {Start: 92, Stop: 98, Padding: 0, ForceNewline: false}, {Start: 98, Stop: 105, Padding: 0, ForceNewline: false}, {Start: 105, Stop: 121, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}, {Root: 0, Node: 46, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Type: 0, Lines: []text.Segment{{Start: 227, Stop: 242, Padding: 0, ForceNewline: true}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Windows: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 124, Stop: 140}, Text: "open [[A]] close"}, {Kind: "CodeSpan", Span: graph.Span{Start: 203, Stop: 217}, Text: "[[image.png]] "}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 227, Stop: 242}, Text: "![[A#A]]章節\n"}}, PrivateWindows: []agreementNativeCodeWindow{{Kind: "CodeSpan", Span: graph.Span{Start: 124, Stop: 140}, Text: "open [[q]] close"}, {Kind: "CodeSpan", Span: graph.Span{Start: 203, Stop: 217}, Text: "[[qqqqqqqqq]] "}, {Kind: "FencedCodeBlock", Span: graph.Span{Start: 227, Stop: 242}, Text: "![[q#A]]章節\n"}}, Changes: []agreementNativeCodePayloadTarget{{Window: 0, Span: graph.Span{Start: 131, Stop: 132}, Old: "A", New: "q"}, {Window: 1, Span: graph.Span{Start: 205, Stop: 214}, Old: "image.png", New: "qqqqqqqqq"}, {Window: 2, Span: graph.Span{Start: 230, Stop: 231}, Old: "A", New: "q"}}, Parts: [][]agreementCodePart{{{Text: "```\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "open ", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}, {Text: "A", Citation: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Field: true}, {Text: " close", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "image.png", Citation: agreementCitation{SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, Field: true}, {Text: " ", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "![[A#A]]章節\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, PrivateParts: [][]agreementCodePart{{{Text: "```\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "open ", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}, {Text: "q", Citation: agreementCitation{SourceRole: "", Target: "q", Section: "", State: "wikilink-broken"}, Field: true}, {Text: " close", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "qqqqqqqqq", Citation: agreementCitation{SourceRole: "", Target: "qqqqqqqqq", Section: "", State: "wikilink-broken"}, Field: true}, {Text: " ", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, {{Text: "![[q#A]]章節\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}}, CitationsInCode: 2, Blocks: []string{"^a"}, Headings: []string{"a-2", "a", "a-3"}, Failures: nil, CodeCitations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}}, CalloutTitles: []string{"two", "three"}}, PrivatePage: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "q", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "qqqqqqqqq", Section: "", State: "wikilink-broken"}}, CitationsInCode: 2, Blocks: []string{"^a"}, Headings: []string{"a-2", "a", "a-3"}, Failures: nil, CodeCitations: []agreementCitation{{SourceRole: "", Target: "q", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "qqqqqqqqq", Section: "", State: "wikilink-broken"}}, CalloutTitles: []string{"two", "three"}}, Owners: []agreementNativeCodeCarrierOwner{{Part: 1, Window: 0}, {Part: 2, Window: 1}}, Signatures: map[string]int{"P2/wikilink-in-code tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=page-in-code multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P2/wikilink-in-code tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=page-in-code multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}},
	}
}

func agreementNativeTypedCodeDifference(c agreementCase, f *agreementFailure, raw string, actual *agreementHTML, failures []agreementFailure) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P2" || f.Identity != "wikilink-in-code" || f.Direction != "page-in-code" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	witnesses := agreementNativeTypedCodeWitnesses()
	for i := range witnesses {
		w := &witnesses[i]
		if c.Body != w.Body {
			continue
		}
		native := agreementNativeOwnerReading(c.Body)
		check := agreementNativeCheckReading(c.Body, native)
		windows, windowsOK := agreementNativeCodeWindows(c.Body, agreementExclusiveCodeGrammar)
		parts, partsOK := agreementCodeWindowParts(raw)
		private, changes, renameOK := agreementNativeTypedCodeRename(c.Body, windows)
		if !windowsOK || !partsOK || !renameOK || private != w.Private || !cmp.Equal(changes, w.Changes) || !cmp.Equal(native, w.Native) || !cmp.Equal(check, w.Check) || !cmp.Equal(agreementNativeCodePayloadSites(c.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), w.HTML) || !cmp.Equal(windows, w.Windows) || !cmp.Equal(parts, w.Parts) || !cmp.Equal(*actual, w.Page) || !maps.Equal(agreementNativeCodePayloadCurrent(failures), w.Signatures) || w.Signatures[agreementSignature(f)] != 1 {
			return "", "", ""
		}
		declarations, owners, owned := agreementNativeTypedCodeOwners(windows, parts)
		current := make(map[agreementCitation]int)
		for _, tuple := range actual.CodeCitations {
			current[tuple]++
		}
		if !owned || !cmp.Equal(declarations, w.Owners) || !maps.Equal(owners, current) || actual.CitationsInCode != len(actual.CodeCitations) || owners[f.Tuple] != f.Multiplicity {
			return "", "", ""
		}
		return "debt", "#1011 stage 5", "page"
	}
	return "", "", ""
}

func TestAgreementNativeTypedCodeSource(t *testing.T) {
	t.Parallel()
	for _, w := range agreementNativeTypedCodeWitnesses() {
		t.Run(w.Name, func(t *testing.T) {
			t.Parallel()
			for _, body := range []string{w.Body, w.Private} {
				if diff := cmp.Diff(w.Native, agreementNativeOwnerReading(body)); diff != "" {
					t.Fatalf("caught: complete native code source (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(w.Sites, agreementNativeCodePayloadSites(body)); diff != "" {
					t.Fatalf("caught: complete native code declarations (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(w.HTML, agreementNativeHTMLAddressSites(body)); diff != "" {
					t.Fatalf("caught: complete HTML declarations beside code (-want +got):\n%s", diff)
				}
			}
			for _, tc := range []struct {
				Body    string
				Check   agreementNativeCheckSource
				Windows []agreementNativeCodeWindow
			}{{w.Body, w.Check, w.Windows}, {w.Private, w.PrivateCheck, w.PrivateWindows}} {
				if diff := cmp.Diff(tc.Check, agreementNativeCheckReading(tc.Body, w.Native)); diff != "" {
					t.Fatalf("caught: complete check declaration ledger (-want +got):\n%s", diff)
				}
				windows, valid := agreementNativeCodeWindows(tc.Body, agreementExclusiveCodeGrammar)
				if diff := cmp.Diff(tc.Windows, windows); !valid || diff != "" {
					t.Fatalf("caught: complete original and private code windows (-want +got):\n%s", diff)
				}
			}
			private, changes, valid := agreementNativeTypedCodeRename(w.Body, w.Windows)
			if !valid || private != w.Private || !cmp.Equal(changes, w.Changes) {
				t.Fatal("caught: only native code target bytes may change")
			}
		})
	}
}

func TestAgreementNativeTypedCodePublic(t *testing.T) {
	t.Parallel()
	for _, w := range agreementNativeTypedCodeWitnesses() {
		t.Run(w.Name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				Name, Body string
				Parts      [][]agreementCodePart
				Page       agreementHTML
				Windows    []agreementNativeCodeWindow
			}{{"original", w.Body, w.Parts, w.Page, w.Windows}, {"private", w.Private, w.PrivateParts, w.PrivatePage, w.PrivateWindows}} {
				t.Run(tc.Name, func(t *testing.T) {
					rendered, actual := agreementIsolatedPage(t, agreementCase{Body: tc.Body})
					parts, valid := agreementCodeWindowParts(rendered.HTML)
					if diff := cmp.Diff(tc.Parts, parts); !valid || diff != "" {
						t.Fatalf("caught: complete literal page code payloads (-want +got):\n%s", diff)
					}
					if diff := cmp.Diff(tc.Page, actual); diff != "" {
						t.Fatalf("caught: complete original and private public ledgers (-want +got):\n%s", diff)
					}
					declarations, owners, owned := agreementNativeTypedCodeOwners(tc.Windows, parts)
					current := make(map[agreementCitation]int)
					for _, tuple := range actual.CodeCitations {
						current[tuple]++
					}
					if !owned || !cmp.Equal(declarations, w.Owners) || !maps.Equal(owners, current) {
						t.Fatal("caught: every page code carrier must belong to a native code span")
					}
				})
			}
		})
	}
}

func TestAgreementNativeTypedCodeDifferences(t *testing.T) {
	t.Parallel()
	for _, w := range agreementNativeTypedCodeWitnesses() {
		t.Run(w.Name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: w.Body}
			rendered, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &rendered, &actual)
			if !maps.Equal(w.Signatures, agreementNativeCodePayloadCurrent(failures)) {
				t.Fatal("caught: complete literal signed code receipts")
			}
			for _, failure := range failures {
				if failure.Property != "P2" {
					continue
				}
				kind, authority, wrong := agreementNativeTypedCodeDifference(c, &failure, rendered.HTML, &actual, failures)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatal("caught: native code payload lost its named authority")
				}
				for _, alter := range []struct {
					Name   string
					Change func(*agreementFailure)
				}{
					{"property", func(f *agreementFailure) { f.Property = "P1" }}, {"identity", func(f *agreementFailure) { f.Identity = "other" }}, {"direction", func(f *agreementFailure) { f.Direction = "other" }},
					{"target", func(f *agreementFailure) { f.Tuple.Target = "outside" }}, {"source role", func(f *agreementFailure) { f.Tuple.SourceRole = "other" }}, {"section", func(f *agreementFailure) { f.Tuple.Section = "other" }}, {"state", func(f *agreementFailure) { f.Tuple.State = "other" }},
					{"multiplicity", func(f *agreementFailure) { f.Multiplicity++ }}, {"fragment", func(f *agreementFailure) { f.Fragment = "^a" }}, {"cut", func(f *agreementFailure) { f.Cut = "other" }},
					{"page", func(f *agreementFailure) { f.PagePresent = true }}, {"judge", func(f *agreementFailure) { f.JudgeAccepted = true }}, {"excerpt", func(f *agreementFailure) { f.ExcerptFound = true }},
				} {
					t.Run(alter.Name, func(t *testing.T) {
						changed := failure
						alter.Change(&changed)
						gotKind, gotAuthority, gotWrong := agreementNativeTypedCodeDifference(c, &changed, rendered.HTML, &actual, failures)
						if gotKind != "" || gotAuthority != "" || gotWrong != "" {
							t.Fatal("caught: native code payload borrowed another signed receipt")
						}
					})
				}
				for _, other := range []agreementCase{{Body: w.Body + "\n"}, {Body: w.Body, Title: "A"}, {Body: w.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
					gotKind, gotAuthority, gotWrong := agreementNativeTypedCodeDifference(other, &failure, rendered.HTML, &actual, failures)
					if gotKind != "" || gotAuthority != "" || gotWrong != "" {
						t.Fatal("caught: native code payload borrowed another complete body")
					}
				}
				changedPage := actual
				changedPage.Headings = append(slices.Clone(actual.Headings), "outside")
				gotKind, gotAuthority, gotWrong := agreementNativeTypedCodeDifference(c, &failure, rendered.HTML, &changedPage, failures)
				if gotKind != "" || gotAuthority != "" || gotWrong != "" {
					t.Fatal("caught: native code payload borrowed another complete heading ledger")
				}
				changed := actual
				changed.CodeCitations = append(slices.Clone(actual.CodeCitations), agreementCitation{Target: "outside", State: "wikilink-broken"})
				kind, authority, wrong = agreementNativeTypedCodeDifference(c, &failure, rendered.HTML, &changed, failures)
				if kind != "" || authority != "" || wrong != "" {
					t.Fatal("caught: native code payload borrowed another whole page ledger")
				}
				extra := append(slices.Clone(failures), failure)
				kind, authority, wrong = agreementNativeTypedCodeDifference(c, &failure, rendered.HTML, &actual, extra)
				if kind != "" || authority != "" || wrong != "" {
					t.Fatal("caught: native code payload borrowed another whole signed set")
				}
				kind, authority, wrong = agreementNativeTypedCodeDifference(c, &failure, rendered.HTML+"<code>outside</code>", &actual, failures)
				if kind != "" || authority != "" || wrong != "" {
					t.Fatal("caught: native code payload borrowed another complete code set")
				}
			}
		})
	}
}

func TestAgreementNativeTypedCodeOwnershipRefusals(t *testing.T) {
	w := agreementNativeTypedCodeWitnesses()[0]
	declarations, owners, valid := agreementNativeTypedCodeOwners(w.Windows, w.Parts)
	if !valid || len(owners) == 0 || !cmp.Equal(declarations, w.Owners) {
		t.Fatal("native typed carrier setup")
	}
	selected := w.Owners[0].Window
	pi := w.Owners[0].Part
	for _, tc := range []struct {
		Name    string
		Windows []agreementNativeCodeWindow
		Parts   [][]agreementCodePart
	}{
		{"missing", nil, w.Parts}, {"duplicate native code", append(slices.Clone(w.Windows), w.Windows[selected]), w.Parts}, {"reused native code", w.Windows, append(slices.Clone(w.Parts), w.Parts[pi])}, {"field-free", w.Windows, [][]agreementCodePart{{{Text: "outside"}}}},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			if _, _, ok := agreementNativeTypedCodeOwners(tc.Windows, tc.Parts); ok {
				t.Fatal("caught: typed code borrowed absent, ambiguous or reused authority")
			}
		})
	}
	windows := slices.Clone(w.Windows)
	windows[selected].Kind = "RawHTML"
	if _, _, ok := agreementNativeTypedCodeOwners(windows, w.Parts); ok {
		t.Fatal("caught: typed carrier borrowed another native kind")
	}
	if _, _, ok := agreementNativeTypedCodeRename(w.Body, windows); ok {
		t.Fatal("caught: typed projection borrowed another native kind")
	}
	windows = slices.Clone(w.Windows)
	windows[selected].Text += "outside"
	if _, _, ok := agreementNativeTypedCodeOwners(windows, w.Parts); ok {
		t.Fatal("caught: typed carrier borrowed a partial native text")
	}
	for _, body := range []string{"q\n\n`[[A]]`", "`[[A\nB]]`"} {
		windows, ok := agreementNativeCodeWindows(body, agreementExclusiveCodeGrammar)
		if !ok {
			t.Fatal("native typed declaration setup")
		}
		if _, _, ok := agreementNativeTypedCodeRename(body, windows); ok {
			t.Fatal("caught: typed projection borrowed a collision or invalid word")
		}
	}
}

func TestAgreementNativeTypedCodeWholeText(t *testing.T) {
	for _, w := range agreementNativeTypedCodeWitnesses() {
		c := agreementCase{Body: w.Body}
		rendered, actual := agreementIsolatedPage(t, c)
		failures := agreementPageFailures(c.Body, &rendered, &actual)
		cursor := 0
		for _, parts := range w.Parts {
			at := strings.Index(rendered.HTML[cursor:], "<code")
			if at < 0 {
				t.Fatal("code declaration setup")
			}
			open := cursor + at
			start := open + strings.Index(rendered.HTML[open:], ">") + 1
			end := strings.Index(rendered.HTML[start:], "</code>")
			if end < 0 {
				t.Fatal("code closing setup")
			}
			cursor = start + end + len("</code>")
			fields := false
			for _, part := range parts {
				fields = fields || part.Field
			}
			if fields {
				continue
			}
			changed := rendered.HTML[:start] + "outside" + rendered.HTML[start+end:]
			page := agreementObserveKnown(t, changed, nil)
			if diff := cmp.Diff(actual, page); diff != "" {
				t.Fatalf("field-free control changed the public ledger: %s", diff)
			}
			for _, f := range failures {
				if f.Property != "P2" {
					continue
				}
				kind, authority, wrong := agreementNativeTypedCodeDifference(c, &f, changed, &page, failures)
				if kind != "" || authority != "" || wrong != "" {
					t.Fatal("caught: code carrier borrowed changed field-free code text")
				}
			}
			return
		}
	}
	t.Fatal("field-free code control setup")
}
