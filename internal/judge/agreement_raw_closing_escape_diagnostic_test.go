package judge_test

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementRawClosingEscapeDiagnosticSingle struct {
	Field, Index, Start int
	Body                string
	Diagnostics         []render.Diagnostic
}
type agreementRawClosingEscapeDiagnosticWitness struct {
	Name, Body, Target, Selected    string
	Diagnostics, PrivateDiagnostics []render.Diagnostic
	Indices                         []int
	Singles                         []agreementRawClosingEscapeDiagnosticSingle
}

type agreementRawClosingEscapeDiagnosticOwner struct {
	Field, Declaration, Root, Node int
	Label                          string
	Bounds                         agreementNativeDefinitionSpan
}

func agreementRawClosingEscapeDiagnosticOwners(native agreementNativeOwnerSource, check *agreementNativeCheckSource, tree *agreementNativeProseAddressTree, target string) ([]agreementRawClosingEscapeDiagnosticOwner, bool) {
	if !tree.Valid || !strings.HasSuffix(target, "\\") {
		return nil, false
	}
	name := strings.TrimSuffix(target, "\\")
	if name == "" || strings.ContainsAny(name, "[]\\\r\n") {
		return nil, false
	}
	var owners []agreementRawClosingEscapeDiagnosticOwner
	used := map[graph.Span]bool{}
	for fi, f := range check.Fields {
		if f.Word != "[["+target+"]]" {
			continue
		}
		if !f.Code || f.Comment || f.Escaped || f.DefinitionOwner < 0 || f.DefinitionOwner >= len(native.Definitions) || f.CodeOwner >= 0 || !slices.Equal(f.Targets, []string{name}) || used[f.Span] {
			return nil, false
		}
		definition := native.Definitions[f.DefinitionOwner]
		at := -1
		for di, d := range tree.Definitions {
			if !d.Found || d.Node != 0 || d.Root < 1 || d.Root >= len(tree.Trees) || len(tree.Trees[d.Root]) == 0 || tree.Trees[d.Root][0].Kind != extast.KindFootnote.String() || d.Label == "" || d.Label != definition.Label || d.Bounds.Parent != definition.Parent || d.Bounds.Span != definition.Span || d.Bounds.Label != definition.Name || d.Bounds.Span.Start < 0 || d.Bounds.Span.Stop <= d.Bounds.Span.Start || d.Bounds.Label.Start < d.Bounds.Span.Start || d.Bounds.Label.Stop > d.Bounds.Span.Stop || d.Bounds.Label.Stop <= d.Bounds.Label.Start || f.Span.Start < d.Bounds.Span.Start || f.Span.Stop > d.Bounds.Span.Stop || f.Span.Stop <= f.Span.Start {
				continue
			}
			if at >= 0 {
				return nil, false
			}
			at = di
		}
		if at < 0 {
			return nil, false
		}
		d := tree.Definitions[at]
		used[f.Span] = true
		owners = append(owners, agreementRawClosingEscapeDiagnosticOwner{fi, at, d.Root, d.Node, d.Label, d.Bounds})
	}
	return owners, len(owners) > 0
}

func agreementRawClosingEscapeDiagnosticPrivate(body, target string, native agreementNativeOwnerSource, check *agreementNativeCheckSource, tree *agreementNativeProseAddressTree) (string, []agreementMixedRename, bool) {
	owners, ok := agreementRawClosingEscapeDiagnosticOwners(native, check, tree, target)
	if !ok || strings.Contains(body, "q") {
		return "", nil, false
	}
	name := strings.TrimSuffix(target, "\\")
	private := []byte(body)
	var changes []agreementMixedRename
	for _, owner := range owners {
		f := check.Fields[owner.Field]
		if f.Span.Start < 0 || f.Span.Stop > len(body) || owner.Bounds.Span.Stop > len(body) || body[f.Span.Start:f.Span.Stop] != f.Word {
			return "", nil, false
		}
		start := f.Span.Start + 2
		stop := start + len(name)
		if stop+1 != f.Span.Stop-2 || body[start:stop] != name || body[stop] != '\\' {
			return "", nil, false
		}
		replacement := strings.Repeat("q", len(name))
		copy(private[start:stop], replacement)
		changes = append(changes, agreementMixedRename{Field: owner.Field, Span: graph.Span{Start: start, Stop: stop}, Old: name, New: replacement})
	}
	return string(private), changes, true
}

func agreementRawClosingEscapeDiagnosticSource(body, target string) (*agreementRawClosingEscapeDiagnosticSourceWitness, *agreementRawClosingEscapeDiagnosticVariant, bool) {
	witnesses := agreementRawClosingEscapeDiagnosticSources()
	for wi := range witnesses {
		w := &witnesses[wi]
		if w.Body != body {
			continue
		}
		for vi := range w.Variants {
			v := &w.Variants[vi]
			if v.Target == target {
				return w, v, true
			}
		}
		return nil, nil, false
	}
	return nil, nil, false
}

// Every changed diagnostic belongs to one declared discarded field.
func agreementRawClosingEscapeDiagnosticDifference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	if f == nil || page == nil || c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Multiplicity < 1 || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Tuple.Section != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	records := agreementRawClosingEscapeDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		if c.Body != r.Body || f.Tuple.Target != r.Target {
			continue
		}
		if agreementSignature(f) != r.Selected || !cmp.Equal(diagnostics, r.Diagnostics) {
			return "", "", ""
		}
		w, v, found := agreementRawClosingEscapeDiagnosticSource(c.Body, r.Target)
		if !found {
			return "", "", ""
		}
		native := agreementNativeOwnerReading(c.Body)
		check := agreementNativeCheckReading(c.Body, native)
		tree := agreementNativeProseAddressTreeReading(c.Body, nil)
		parts, pok := agreementCodeWindowParts(raw)
		owners, ok := agreementRawClosingEscapeDiagnosticOwners(native, &check, &tree, r.Target)
		private, changes, valid := agreementRawClosingEscapeDiagnosticPrivate(c.Body, r.Target, native, &check, &tree)
		if !pok || !ok || !valid || len(owners) != f.Multiplicity || len(r.Indices) != len(owners) || !cmp.Equal(owners, v.Owners) || !cmp.Equal(changes, v.Changes) || private != v.Body || !cmp.Equal(native, w.Native) || !cmp.Equal(check, w.Check) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(agreementNativeCodePayloadSites(c.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), w.HTML) || !cmp.Equal(judge.LinkTargets(c.Body), w.Targets) || !cmp.Equal(parts, w.Parts) || !cmp.Equal(*page, w.Page) || !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
			return "", "", ""
		}
		return "debt", "#1011 stage 5", "page-diagnostic"
	}
	return "", "", ""
}

type agreementRawClosingEscapeDiagnosticVariant struct {
	Target, Body string
	Owners       []agreementRawClosingEscapeDiagnosticOwner
	Changes      []agreementMixedRename
	Check        agreementNativeCheckSource
	Targets      []string
	Signatures   map[string]int
}
type agreementRawClosingEscapeDiagnosticSourceWitness struct {
	Name       string
	Body       string
	Native     agreementNativeOwnerSource
	Check      agreementNativeCheckSource
	Sites      []agreementNativeCodePayloadSite
	HTML       []agreementNativeHTMLAddressSite
	Tree       agreementNativeProseAddressTree
	Page       agreementHTML
	Parts      [][]agreementCodePart
	Targets    []string
	Signatures map[string]int
	Selected   map[string]int
	Variants   []agreementRawClosingEscapeDiagnosticVariant
}

func agreementRawClosingEscapeDiagnosticSources() []agreementRawClosingEscapeDiagnosticSourceWitness {
	return []agreementRawClosingEscapeDiagnosticSourceWitness{{Name: "e879635b51dc", Body: "[^n]: [[A]]\n\n    [[B]]\n![[A]][[A\nB]]\n\n    - ^absent-prose ![[A#A]][[A\\]][[image.png]]É\n ^é\nB\n`[[A]]`", Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 103}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 11}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 6, Stop: 7}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 7, Stop: 8}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 8, Stop: 10}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 10, Stop: 11}}}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 17, Stop: 23}, {Start: 23, Stop: 33}, {Start: 33, Stop: 36}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 19}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 21}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 22}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 25}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 25, Stop: 26}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 26, Stop: 30}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 30, Stop: 31}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 31, Stop: 32}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 33, Stop: 35}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 35, Stop: 36}}}, {Kind: "List", Depth: 1, Lines: nil, Text: nil}, {Kind: "ListItem", Depth: 2, Lines: nil, Text: nil}, {Kind: "TextBlock", Depth: 3, Lines: []graph.Span{{Start: 44, Stop: 89}, {Start: 90, Stop: 94}, {Start: 94, Stop: 96}, {Start: 96, Stop: 103}}, Text: nil}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 44, Stop: 60}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 60, Stop: 61}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 61, Stop: 66}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 66, Stop: 67}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 67, Stop: 68}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 68, Stop: 73}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 73, Stop: 74}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 74, Stop: 84}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 84, Stop: 88}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 90, Stop: 93}}}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 94, Stop: 95}}}, {Kind: "CodeSpan", Depth: 4, Lines: nil, Text: nil}, {Kind: "Text", Depth: 5, Lines: nil, Text: []graph.Span{{Start: 97, Stop: 102}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "TextBlock", Span: graph.Span{Start: 97, Stop: 102}}}, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 23}, {Start: 42, Stop: 89}, {Start: 97, Stop: 102}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 24, Stop: 29}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 29, Stop: 36}, Word: "[[A\nB]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 59, Stop: 66}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 66, Stop: 72}, Word: "[[A\\]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 72, Stop: 85}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 97, Stop: 102}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: 0}}, Targets: []string{"A", "A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 1, Node: 32, Parent: 20, Kind: "CodeSpan", ParentKind: "TextBlock", Lines: nil, Text: []text.Segment{{Start: 97, Stop: 102, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}}, HTML: nil, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}, {{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 11}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 6, Stop: 7}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 7, Stop: 8}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 8, Stop: 10}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 10, Stop: 11}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 17, Stop: 23}, {Start: 23, Stop: 33}, {Start: 33, Stop: 36}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 19}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 21}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 22}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 25}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 25, Stop: 26}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 26, Stop: 30}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 30, Stop: 31}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 31, Stop: 32}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 33, Stop: 35}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 35, Stop: 36}}, Raw: false, Soft: false, Hard: false}, {Kind: "List", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "ListItem", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TextBlock", Depth: 3, Lines: []graph.Span{{Start: 44, Stop: 89}, {Start: 90, Stop: 94}, {Start: 94, Stop: 96}, {Start: 96, Stop: 103}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 44, Stop: 60}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 60, Stop: 61}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 61, Stop: 66}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 66, Stop: 67}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 67, Stop: 68}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 68, Stop: 73}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 73, Stop: 74}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 74, Stop: 84}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 84, Stop: 88}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 90, Stop: 93}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 94, Stop: 95}}, Raw: false, Soft: true, Hard: false}, {Kind: "CodeSpan", Depth: 4, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 5, Lines: nil, Text: []graph.Span{{Start: 97, Stop: 102}}, Raw: true, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 1, Node: 0, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 0, Stop: 103}, Label: graph.Span{Start: 0, Stop: 5}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "TextBlock", Span: graph.Span{Start: 97, Stop: 102}}}, Comments: nil, Unanchorable: nil, Valid: true}, Page: agreementHTML{Citations: nil, CitationsInCode: 0, Blocks: nil, Headings: nil, Failures: nil, CodeCitations: nil, CalloutTitles: nil}, Parts: nil, Targets: []string{"A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementRawClosingEscapeDiagnosticVariant{{Target: "A\\", Body: "[^n]: [[A]]\n\n    [[B]]\n![[A]][[A\nB]]\n\n    - ^absent-prose ![[A#A]][[q\\]][[image.png]]É\n ^é\nB\n`[[A]]`", Owners: []agreementRawClosingEscapeDiagnosticOwner{{Field: 5, Declaration: 0, Root: 1, Node: 0, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 0, Stop: 103}, Label: graph.Span{Start: 0, Stop: 5}, Parent: "Document"}}}, Changes: []agreementMixedRename{{Field: 5, Span: graph.Span{Start: 68, Stop: 69}, Old: "A", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 23}, {Start: 42, Stop: 89}, {Start: 97, Stop: 102}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 24, Stop: 29}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 29, Stop: 36}, Word: "[[A\nB]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 59, Stop: 66}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 66, Stop: 72}, Word: "[[q\\]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 72, Stop: 85}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 97, Stop: 102}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: 0}}, Targets: []string{"A", "A"}}, Targets: []string{"A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1}}}}}
}
func agreementRawClosingEscapeDiagnosticWitnesses() []agreementRawClosingEscapeDiagnosticWitness {
	return []agreementRawClosingEscapeDiagnosticWitness{{Name: "e879635b51dc", Body: "[^n]: [[A]]\n\n    [[B]]\n![[A]][[A\nB]]\n\n    - ^absent-prose ![[A#A]][[A\\]][[image.png]]É\n ^é\nB\n`[[A]]`", Target: "A\\", Selected: "P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A\\", Message: "wikilink \"A\\\\\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "q\\", Message: "wikilink \"q\\\\\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}, Indices: []int{4}, Singles: []agreementRawClosingEscapeDiagnosticSingle{{Field: 5, Index: 4, Start: 68, Body: "[^n]: [[A]]\n\n    [[B]]\n![[A]][[A\nB]]\n\n    - ^absent-prose ![[A#A]][[q\\]][[image.png]]É\n ^é\nB\n`[[A]]`", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "q\\", Message: "wikilink \"q\\\\\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}}}}}
}
func TestAgreementRawClosingEscapeDiagnosticOwnedSource(t *testing.T) {
	t.Parallel()
	records := agreementRawClosingEscapeDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementRawClosingEscapeDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			if len(r.Singles) != len(v.Owners) || len(r.Indices) != len(v.Owners) {
				t.Fatal("caught: complete discarded diagnostic owner set")
			}
			changed := make(map[int]bool)
			fields := make(map[int]bool)
			expected := slices.Clone(r.Diagnostics)
			for si := range r.Singles {
				single := &r.Singles[si]
				owner := v.Owners[si]
				change := v.Changes[si]
				if single.Field != owner.Field || single.Start != change.Span.Start || single.Index != r.Indices[si] || single.Index < 0 || single.Index >= len(expected) || changed[single.Index] || fields[single.Field] {
					t.Fatal("caught: unique complete discarded field-to-diagnostic map")
				}
				changed[single.Index] = true
				fields[single.Field] = true
				body := []byte(r.Body)
				copy(body[change.Span.Start:change.Span.Stop], change.New)
				if string(body) != single.Body {
					t.Fatal("caught: exact single discarded diagnostic source projection")
				}
				native := agreementNativeOwnerReading(single.Body)
				tree := agreementNativeProseAddressTreeReading(single.Body, nil)
				check := agreementNativeCheckReading(single.Body, native)
				original := w.Check
				original.Fields = slices.Clone(original.Fields)
				field := &original.Fields[single.Field]
				start, stop := change.Span.Start-field.Span.Start, change.Span.Stop-field.Span.Start
				field.Word = field.Word[:start] + change.New + field.Word[stop:]
				field.Targets = []string{change.New}
				original.Targets = nil
				for _, f := range original.Fields {
					if !f.Code && !f.Comment && !f.Escaped {
						original.Targets = append(original.Targets, f.Targets...)
					}
				}
				if !cmp.Equal(native, w.Native) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(check, original) || !cmp.Equal(agreementNativeCodePayloadSites(single.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(single.Body), w.HTML) {
					t.Fatal("caught: complete unchanged single discarded diagnostic source facts")
				}
				if len(single.Diagnostics) != len(r.Diagnostics) {
					t.Fatal("caught: complete ordered single discarded diagnostic ledger")
				}
				for i, diag := range r.Diagnostics {
					private := single.Diagnostics[i]
					if i != single.Index {
						if !cmp.Equal(diag, private) {
							t.Fatal("caught: every unselected raw diagnostic stays identical")
						}
						continue
					}
					if diag.Kind != render.DiagWikilinkBroken || diag.Target != r.Target || private.Target != change.New+"\\" || private.Kind != diag.Kind || private.Section != diag.Section || private.Block != diag.Block || private.Message != strings.ReplaceAll(diag.Message, strconv.Quote(r.Target), strconv.Quote(change.New+"\\")) {
						t.Fatal("caught: one owned diagnostic target and message change per source field")
					}
					expected[i] = private
				}
			}
			if !cmp.Equal(expected, r.PrivateDiagnostics) {
				t.Fatal("caught: whole ordered discarded diagnostic substitution")
			}
			for _, owner := range v.Owners {
				if !fields[owner.Field] {
					t.Fatal("caught: every declared discarded source field is represented")
				}
			}
		})
	}
}

func TestAgreementRawClosingEscapeDiagnosticOwnedPublic(t *testing.T) {
	t.Parallel()
	records := agreementRawClosingEscapeDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementRawClosingEscapeDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
			if !cmp.Equal(raw.Diagnostics, r.Diagnostics) || !cmp.Equal(page, w.Page) {
				t.Fatal("caught: complete original discarded diagnostic and public ledgers")
			}
			private, pp := agreementIsolatedPage(t, agreementCase{Body: v.Body})
			if raw.HTML != private.HTML || !cmp.Equal(private.Diagnostics, r.PrivateDiagnostics) || !cmp.Equal(pp, w.Page) {
				t.Fatal("caught: whole discarded diagnostic projection leaves complete HTML unchanged")
			}
			for si := range r.Singles {
				single := &r.Singles[si]
				projected, p := agreementIsolatedPage(t, agreementCase{Body: single.Body})
				if projected.HTML != raw.HTML || !cmp.Equal(projected.Diagnostics, single.Diagnostics) || !cmp.Equal(p, w.Page) {
					t.Fatal("caught: complete independent single-field discarded diagnostic observation")
				}
				parts, valid := agreementCodeWindowParts(projected.HTML)
				if !valid || !cmp.Equal(parts, w.Parts) {
					t.Fatal("caught: every single-field discarded diagnostic code ledger stays identical")
				}
			}
			c := agreementCase{Body: r.Body}
			failures := agreementPageFailures(r.Body, &raw, &page)
			failures = append(failures, agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{page})[0]...)
			if !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
				t.Fatal("caught: complete original discarded diagnostic signed pool")
			}
			privateFailures := agreementPageFailures(v.Body, &private, &pp)
			privateFailures = append(privateFailures, agreementFragmentFailures(t, []agreementCase{{Body: v.Body}}, []agreementHTML{pp})[0]...)
			if !maps.Equal(agreementMixedCurrent(privateFailures), v.Signatures) {
				t.Fatal("caught: complete private discarded diagnostic signed pool")
			}
			seen := 0
			for _, f := range failures {
				if agreementSignature(&f) != r.Selected {
					continue
				}
				seen++
				k, a, b := agreementRawClosingEscapeDiagnosticDifference(c, &f, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "debt" || a != "#1011 stage 5" || b != "page-diagnostic" {
					t.Fatal("caught: exact discarded diagnostic P0 debt owner")
				}
			}
			if seen != 1 {
				t.Fatal("diagnostic selected receipt setup")
			}
		})
	}
}

func TestAgreementRawClosingEscapeDiagnosticOwnedRefusals(t *testing.T) {
	t.Parallel()
	records := agreementRawClosingEscapeDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: r.Body}
			raw, page := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(r.Body, &raw, &page)
			failures = append(failures, agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{page})[0]...)
			var selected agreementFailure
			seen := 0
			for _, f := range failures {
				if agreementSignature(&f) == r.Selected {
					selected = f
					seen++
				}
			}
			if seen != 1 {
				t.Fatal("diagnostic selected receipt setup")
			}
			for _, tc := range []struct {
				Name   string
				Change func(*agreementFailure)
			}{
				{"property", func(f *agreementFailure) { f.Property = "P1" }},
				{"identity", func(f *agreementFailure) { f.Identity = "other" }},
				{"direction", func(f *agreementFailure) { f.Direction = "page-only" }},
				{"role", func(f *agreementFailure) { f.Tuple.SourceRole = "other" }},
				{"state", func(f *agreementFailure) { f.Tuple.State = "other" }},
				{"section", func(f *agreementFailure) { f.Tuple.Section = "other" }},
				{"target", func(f *agreementFailure) { f.Tuple.Target = "outside" }},
				{"count", func(f *agreementFailure) { f.Multiplicity++ }},
				{"fragment", func(f *agreementFailure) { f.Fragment = "outside" }},
				{"cut", func(f *agreementFailure) { f.Cut = "outside" }},
				{"page", func(f *agreementFailure) { f.PagePresent = true }},
				{"judge", func(f *agreementFailure) { f.JudgeAccepted = true }},
				{"excerpt", func(f *agreementFailure) { f.ExcerptFound = true }},
			} {
				t.Run(tc.Name, func(t *testing.T) {
					changed := selected
					tc.Change(&changed)
					k, a, b := agreementRawClosingEscapeDiagnosticDifference(c, &changed, raw.HTML, &page, raw.Diagnostics, failures)
					if k != "" || a != "" || b != "" {
						t.Fatal("caught: discarded diagnostic borrowed another signed face")
					}
				})
			}
			for _, other := range []agreementCase{{Body: r.Body + "\n"}, {Body: r.Body, Title: "A"}, {Body: r.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
				k, a, b := agreementRawClosingEscapeDiagnosticDifference(other, &selected, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed another body or context")
				}
			}
			for i := range failures {
				missing := slices.Delete(slices.Clone(failures), i, i+1)
				k, a, b := agreementRawClosingEscapeDiagnosticDifference(c, &selected, raw.HTML, &page, raw.Diagnostics, missing)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed an incomplete whole signed pool")
				}
			}
			for i := range raw.Diagnostics {
				missing := slices.Delete(slices.Clone(raw.Diagnostics), i, i+1)
				k, a, b := agreementRawClosingEscapeDiagnosticDifference(c, &selected, raw.HTML, &page, missing, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed an incomplete whole raw ledger")
				}
				altered := slices.Clone(raw.Diagnostics)
				altered[i].Message += "outside"
				k, a, b = agreementRawClosingEscapeDiagnosticDifference(c, &selected, raw.HTML, &page, altered, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed another complete raw ledger")
				}
			}
			changed := page
			changed.Headings = append(slices.Clone(page.Headings), "outside")
			k, a, b := agreementRawClosingEscapeDiagnosticDifference(c, &selected, raw.HTML, &changed, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: discarded diagnostic borrowed another complete public ledger")
			}
			k, a, b = agreementRawClosingEscapeDiagnosticDifference(c, &selected, raw.HTML+"<code>outside</code>", &page, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: discarded diagnostic borrowed changed field-free code text")
			}
		})
	}
}

func TestAgreementRawClosingEscapeDiagnosticOwnedWholeTargets(t *testing.T) {
	t.Parallel()
	records := agreementRawClosingEscapeDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementRawClosingEscapeDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("partial diagnostic source setup")
			}
			expected := w.Check
			expected.Fields = slices.Clone(expected.Fields)
			for _, change := range v.Changes {
				field := &expected.Fields[change.Field]
				start, stop := change.Span.Start-field.Span.Start, change.Span.Stop-field.Span.Start
				field.Word = field.Word[:start] + change.New + field.Word[stop:]
				field.Targets = []string{change.New}
			}
			expected.Targets = nil
			for _, field := range expected.Fields {
				if !field.Code && !field.Comment && !field.Escaped {
					expected.Targets = append(expected.Targets, field.Targets...)
				}
			}
			native := agreementNativeOwnerReading(v.Body)
			if !cmp.Equal(expected, v.Check) || !cmp.Equal(agreementNativeCheckReading(v.Body, native), expected) || !cmp.Equal(judge.LinkTargets(r.Body), w.Targets) || !cmp.Equal(judge.LinkTargets(v.Body), v.Targets) || !cmp.Equal(expected.Targets, v.Targets) {
				t.Fatal("caught: complete whole discarded diagnostic physical targets")
			}
			counts := func(targets []string) map[string]int {
				m := make(map[string]int)
				for _, target := range targets {
					m[target]++
				}
				return m
			}
			before, after := counts(w.Targets), counts(v.Targets)
			normalized := strings.TrimSuffix(r.Target, "\\")
			replacement := strings.Repeat("q", len(normalized))
			n := 0
			for _, owner := range v.Owners {
				field := w.Check.Fields[owner.Field]
				if !field.Code && !field.Comment && !field.Escaped {
					n++
				}
			}
			if before[normalized]-after[normalized] != n || after[replacement]-before[replacement] != n {
				t.Fatal("caught: discarded diagnostic target counts conserve every physical owner")
			}
			delete(before, normalized)
			delete(before, replacement)
			delete(after, normalized)
			delete(after, replacement)
			if !maps.Equal(before, after) {
				t.Fatal("caught: every unselected whole discarded diagnostic target count")
			}
		})
	}
}

func TestAgreementRawClosingEscapeDiagnosticOwnedWholeSet(t *testing.T) {
	t.Parallel()
	records := agreementRawClosingEscapeDiagnosticWitnesses()
	witnesses := agreementRawClosingEscapeDiagnosticSources()
	expected := make(map[[3]string]int)
	for _, r := range records {
		key := [3]string{r.Body, r.Target, r.Selected}
		if _, exists := expected[key]; exists || len(r.Singles) != len(r.Indices) {
			t.Fatal("caught: unique complete discarded diagnostic declared roster")
		}
		expected[key] = len(r.Singles)
	}
	actual := make(map[[3]string]int)
	for wi := range witnesses {
		w := &witnesses[wi]
		raw, page := agreementIsolatedPage(t, agreementCase{Body: w.Body})
		native := agreementNativeOwnerReading(w.Body)
		check := agreementNativeCheckReading(w.Body, native)
		tree := agreementNativeProseAddressTreeReading(w.Body, nil)
		failures := agreementPageFailures(w.Body, &raw, &page)
		failures = append(failures, agreementFragmentFailures(t, []agreementCase{{Body: w.Body}}, []agreementHTML{page})[0]...)
		bodySelected := make(map[string]int)
		bodyVariants := make(map[[2]string]int)
		for _, f := range failures {
			if f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "wikilink-broken" {
				continue
			}
			target := f.Tuple.Target
			owners, ok := agreementRawClosingEscapeDiagnosticOwners(native, &check, &tree, target)
			private, changes, valid := agreementRawClosingEscapeDiagnosticPrivate(w.Body, target, native, &check, &tree)
			if !ok || !valid || len(owners) != f.Multiplicity {
				continue
			}
			pr, pp := agreementIsolatedPage(t, agreementCase{Body: private})
			if raw.HTML != pr.HTML || !cmp.Equal(page, pp) || !cmp.Equal(native, agreementNativeOwnerReading(private)) || !cmp.Equal(tree, agreementNativeProseAddressTreeReading(private, nil)) {
				continue
			}
			pf := agreementPageFailures(private, &pr, &pp)
			pf = append(pf, agreementFragmentFailures(t, []agreementCase{{Body: private}}, []agreementHTML{pp})[0]...)
			unselected := func(pool []agreementFailure) map[string]int {
				result := make(map[string]int)
				for _, x := range pool {
					if x.Property == "P0" && (x.Tuple.Target == target || x.Tuple.Target == changes[0].New+"\\") {
						continue
					}
					result[agreementSignature(&x)]++
				}
				return result
			}
			if !maps.Equal(unselected(failures), unselected(pf)) {
				continue
			}
			producing := true
			usedIndices := make(map[int]bool)
			expectedDiagnostics := slices.Clone(raw.Diagnostics)
			for _, change := range changes {
				oneBody := []byte(w.Body)
				copy(oneBody[change.Span.Start:change.Span.Stop], change.New)
				one, onePage := agreementIsolatedPage(t, agreementCase{Body: string(oneBody)})
				if one.HTML != raw.HTML || !cmp.Equal(onePage, page) || len(one.Diagnostics) != len(raw.Diagnostics) {
					producing = false
					break
				}
				index := -1
				for di := range raw.Diagnostics {
					if cmp.Equal(raw.Diagnostics[di], one.Diagnostics[di]) {
						continue
					}
					if index >= 0 {
						producing = false
						break
					}
					index = di
				}
				if !producing || index < 0 || usedIndices[index] {
					producing = false
					break
				}
				diag, renamed := raw.Diagnostics[index], one.Diagnostics[index]
				if diag.Kind != render.DiagWikilinkBroken || diag.Target != target || renamed.Target != change.New+"\\" || renamed.Kind != diag.Kind || renamed.Section != diag.Section || renamed.Block != diag.Block || renamed.Message != strings.ReplaceAll(diag.Message, strconv.Quote(target), strconv.Quote(change.New+"\\")) {
					producing = false
					break
				}
				usedIndices[index] = true
				expectedDiagnostics[index] = renamed
			}
			if !producing || !cmp.Equal(expectedDiagnostics, pr.Diagnostics) {
				continue
			}
			key := [3]string{w.Body, target, agreementSignature(&f)}
			if _, exists := actual[key]; exists {
				t.Fatal("caught: every eligible discarded diagnostic roster member once")
			}
			actual[key] = len(owners)
			bodySelected[agreementSignature(&f)]++
			bodyVariants[[2]string{target, agreementSignature(&f)}] = len(owners)
		}
		declaredVariants := make(map[[2]string]int)
		for _, v := range w.Variants {
			f := agreementFailure{Property: "P0", Identity: "diagnostic-html", Tuple: agreementCitation{Target: v.Target, State: "wikilink-broken"}, Direction: "diagnostic-only", Multiplicity: len(v.Owners)}
			key := [2]string{v.Target, agreementSignature(&f)}
			if _, exists := declaredVariants[key]; exists {
				t.Fatal("caught: unique complete discarded diagnostic source variants")
			}
			declaredVariants[key] = len(v.Owners)
		}
		if !maps.Equal(bodySelected, w.Selected) || !maps.Equal(bodyVariants, declaredVariants) {
			t.Fatal("caught: whole eligible discarded diagnostic source declaration set")
		}
	}
	fields := 0
	for _, n := range actual {
		fields += n
	}
	if !maps.Equal(expected, actual) || len(actual) != 1 || fields != 1 || len(witnesses) != 1 {
		t.Fatal("caught: whole declared-body discarded diagnostic eligibility census")
	}
}
