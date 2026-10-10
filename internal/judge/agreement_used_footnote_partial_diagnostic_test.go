package judge_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementUsedFootnotePartialDiagnosticSingle struct {
	Field, Index, Start int
	Body                string
	Diagnostics         []render.Diagnostic
}
type agreementUsedFootnotePartialDiagnosticWitness struct {
	Name, Body, Target, Selected    string
	Diagnostics, PrivateDiagnostics []render.Diagnostic
	Indices                         []int
	Singles                         []agreementUsedFootnotePartialDiagnosticSingle
}

// Each field has one later paragraph owner; outside names retain their own roles.
func agreementUsedFootnotePartialDiagnosticOwners(check *agreementNativeCheckSource, notes []agreementUsedFootnoteDeclaration) ([]agreementUsedFootnoteOwner, bool) {
	var owners []agreementUsedFootnoteOwner
	used := make(map[graph.Span]bool)
	for fi, field := range check.Fields {
		if !slices.Contains(field.Targets, "B") || field.Comment || field.Escaped {
			continue
		}
		candidate := agreementUsedFootnoteOwner{}
		found := false
		for ni := range notes {
			note := &notes[ni]
			if !note.Found || note.Parent != extast.KindFootnoteList.String() || note.Label == "" || note.Bounds.Parent == "" || note.Bounds.Span.Start < 0 || note.Bounds.Span.Stop <= note.Bounds.Span.Start || note.Bounds.Label.Start < note.Bounds.Span.Start || note.Bounds.Label.Stop > note.Bounds.Span.Stop || note.Bounds.Label.Stop <= note.Bounds.Label.Start || field.Span.Start < note.Bounds.Span.Start || field.Span.Stop > note.Bounds.Span.Stop {
				continue
			}
			for pi := range note.Paragraphs {
				paragraph := &note.Paragraphs[pi]
				if paragraph.Ordinal < 1 || paragraph.Kind != ast.KindParagraph.String() {
					continue
				}
				for li, line := range paragraph.Lines {
					if field.Span.Start < line.Start || field.Span.Stop > line.Stop {
						continue
					}
					if found {
						return nil, false
					}
					found = true
					candidate = agreementUsedFootnoteOwner{Field: fi, Note: ni, Node: note.Node, Label: note.Label, Paragraph: pi, Ordinal: paragraph.Ordinal, Line: li}
				}
			}
		}
		if !found {
			continue
		}
		if !field.Code || field.Comment || field.Escaped || field.DefinitionOwner >= 0 || field.CodeOwner >= 0 || len(field.Targets) != 1 || field.Word != "[[B]]" || field.Span.Start < 0 || field.Span.Stop-field.Span.Start != 5 || used[field.Span] {
			return nil, false
		}

		used[field.Span] = true
		owners = append(owners, candidate)
	}
	return owners, len(owners) > 0
}

// Only fields in later used-definition paragraphs are renamed.
func agreementUsedFootnotePartialDiagnosticPrivate(body string, check *agreementNativeCheckSource, notes []agreementUsedFootnoteDeclaration) (string, []agreementMixedRename, bool) {
	if strings.Contains(body, "q") {
		return "", nil, false
	}
	owners, valid := agreementUsedFootnotePartialDiagnosticOwners(check, notes)
	if !valid {
		return "", nil, false
	}
	private := []byte(body)
	var changes []agreementMixedRename
	for _, owner := range owners {
		field := check.Fields[owner.Field]
		note := notes[owner.Note]
		if field.Span.Stop > len(body) || note.Bounds.Span.Stop > len(body) || body[field.Span.Start:field.Span.Stop] != field.Word {
			return "", nil, false
		}
		private[field.Span.Start+2] = 'q'
		changes = append(changes, agreementMixedRename{Field: owner.Field, Span: graph.Span{Start: field.Span.Start + 2, Stop: field.Span.Start + 3}, Old: "B", New: "q"})
	}
	return string(private), changes, true
}

func agreementUsedFootnotePartialDiagnosticSource(body, target string) (*agreementUsedFootnotePartialDiagnosticSourceWitness, *agreementUsedFootnotePartialDiagnosticVariant, bool) {
	witnesses := agreementUsedFootnotePartialDiagnosticSources()
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
func agreementUsedFootnotePartialDiagnosticDifference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	if f == nil || page == nil || c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Multiplicity < 1 || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Tuple.Section != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	records := agreementUsedFootnotePartialDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		if c.Body != r.Body || f.Tuple.Target != r.Target {
			continue
		}
		if agreementSignature(f) != r.Selected || !cmp.Equal(diagnostics, r.Diagnostics) {
			return "", "", ""
		}
		w, v, found := agreementUsedFootnotePartialDiagnosticSource(c.Body, r.Target)
		if !found {
			return "", "", ""
		}
		native := agreementNativeOwnerReading(c.Body)
		check := agreementNativeCheckReading(c.Body, native)
		tree := agreementNativeProseAddressTreeReading(c.Body, nil)
		notes := agreementUsedFootnoteReading(c.Body)
		parts, pok := agreementCodeWindowParts(raw)
		owners, ok := agreementUsedFootnotePartialDiagnosticOwners(&check, notes)
		private, changes, valid := agreementUsedFootnotePartialDiagnosticPrivate(c.Body, &check, notes)
		if !pok || !ok || !valid || len(owners) != f.Multiplicity || len(r.Indices) != len(owners) || !cmp.Equal(owners, v.Owners) || !cmp.Equal(changes, v.Changes) || private != v.Body || !cmp.Equal(native, w.Native) || !cmp.Equal(check, w.Check) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(notes, w.Notes) || !cmp.Equal(agreementNativeCodePayloadSites(c.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), w.HTML) || !cmp.Equal(judge.LinkTargets(c.Body), w.Targets) || !cmp.Equal(parts, w.Parts) || !cmp.Equal(*page, w.Page) || !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
			return "", "", ""
		}
		return "debt", "#1011 stage 5", "page-diagnostic"
	}
	return "", "", ""
}

type agreementUsedFootnotePartialDiagnosticVariant struct {
	Target, Body string
	Owners       []agreementUsedFootnoteOwner
	Changes      []agreementMixedRename
	Check        agreementNativeCheckSource
	Targets      []string
	Signatures   map[string]int
}
type agreementUsedFootnotePartialDiagnosticSourceWitness struct {
	Name       string
	Body       string
	Native     agreementNativeOwnerSource
	Check      agreementNativeCheckSource
	Sites      []agreementNativeCodePayloadSite
	HTML       []agreementNativeHTMLAddressSite
	Tree       agreementNativeProseAddressTree
	Notes      []agreementUsedFootnoteDeclaration
	Page       agreementHTML
	Parts      [][]agreementCodePart
	Targets    []string
	Signatures map[string]int
	Selected   map[string]int
	Variants   []agreementUsedFootnotePartialDiagnosticVariant
}

func agreementUsedFootnotePartialDiagnosticSources() []agreementUsedFootnotePartialDiagnosticSourceWitness {
	return []agreementUsedFootnotePartialDiagnosticSourceWitness{
		{Name: "8f534fbe487b", Body: "## A\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n%%   ```\n[[B|alias]]## !\n<!-- [[A]] -->  ```\n ^A\n[[image.png]]\\\\[[A]]>   - [^n]: [[A]]\n\n    [[B]]\n\t[^unused]: [[A]]\nA\n===\n<!--\n[[A]]\n--> [[A]] [[A]]  > [!note] title\n    ", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 130, Stop: 154}}}, Comments: []graph.Span{{Start: 38, Stop: 208}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 32, Stop: 38}, {Start: 130, Stop: 154}}, Comments: []graph.Span{{Start: 38, Stop: 208}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 21, Stop: 26}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 32, Stop: 37}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 47, Stop: 58}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 68, Stop: 73}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 87, Stop: 100}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 102, Stop: 107}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 119, Stop: 124}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 130, Stop: 135}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 148, Stop: 153}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 165, Stop: 170}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 175, Stop: 180}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 181, Stop: 186}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 20, Parent: 0, Kind: "CodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 130, Stop: 136, Padding: 0, ForceNewline: true}, {Start: 137, Stop: 154, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 7, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 63, Stop: 83, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}, {Root: 0, Node: 23, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 160, Stop: 165, Padding: 0, ForceNewline: false}, {Start: 165, Stop: 171, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 171, Stop: 204, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 3, Stop: 4}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 3, Stop: 4}}, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 8, Stop: 9}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 8, Stop: 9}}, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 13, Stop: 14}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 13, Stop: 14}}, Raw: false, Soft: false, Hard: false}, {Kind: "HTMLBlock", Depth: 1, Lines: []graph.Span{{Start: 63, Stop: 83}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 84, Stop: 87}, {Start: 87, Stop: 124}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 84, Stop: 86}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 87, Stop: 88}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 88, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 103}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 103, Stop: 104}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 104, Stop: 113}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteLink", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 117, Stop: 120}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 120, Stop: 121}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 121, Stop: 123}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 123, Stop: 124}}, Raw: false, Soft: false, Hard: false}, {Kind: "CodeBlock", Depth: 1, Lines: []graph.Span{{Start: 130, Stop: 136}, {Start: 137, Stop: 154}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 154, Stop: 155}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 154, Stop: 155}}, Raw: false, Soft: false, Hard: false}, {Kind: "HTMLBlock", Depth: 1, Lines: []graph.Span{{Start: 160, Stop: 165}, {Start: 165, Stop: 171}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteList", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Footnote", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 21, Stop: 26}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 22}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 22, Stop: 23}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 25}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 25, Stop: 26}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 32, Stop: 38}, {Start: 38, Stop: 47}, {Start: 47, Stop: 62}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 32, Stop: 33}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 33, Stop: 34}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 34, Stop: 36}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 36, Stop: 37}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 38, Stop: 43}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 43, Stop: 46}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 46, Stop: 46}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 48}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 48, Stop: 49}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 49, Stop: 61}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 61, Stop: 62}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteBacklink", Depth: 4, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 0, Node: 25, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 15, Stop: 63}, Label: graph.Span{Start: 15, Stop: 20}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 130, Stop: 154}}}, Comments: []graph.Span{{Start: 38, Stop: 208}}, Unanchorable: nil, Valid: true}, Notes: []agreementUsedFootnoteDeclaration{{Node: 25, Label: "n", Parent: "FootnoteList", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 15, Stop: 63}, Label: graph.Span{Start: 15, Stop: 20}, Parent: "Document"}, Found: true, Paragraphs: []agreementUsedFootnoteParagraph{{Ordinal: 0, Kind: "Paragraph", Lines: []text.Segment{{Start: 21, Stop: 26, Padding: 0, ForceNewline: false}}}, {Ordinal: 1, Kind: "Paragraph", Lines: []text.Segment{{Start: 32, Stop: 38, Padding: 0, ForceNewline: false}, {Start: 38, Stop: 47, Padding: 0, ForceNewline: false}, {Start: 47, Stop: 62, Padding: 0, ForceNewline: false}}}}}}, Page: agreementHTML{Citations: nil, CitationsInCode: 0, Blocks: nil, Headings: []string{"a", "a-2", "a-3"}, Failures: nil, CodeCitations: nil, CalloutTitles: nil}, Parts: nil, Targets: []string{"A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementUsedFootnotePartialDiagnosticVariant{{Target: "B", Body: "## A\n## A\n## A\n[^n]: [[A]]\n\n    [[q]]\n%%   ```\n[[B|alias]]## !\n<!-- [[A]] -->  ```\n ^A\n[[image.png]]\\\\[[A]]>   - [^n]: [[A]]\n\n    [[B]]\n\t[^unused]: [[A]]\nA\n===\n<!--\n[[A]]\n--> [[A]] [[A]]  > [!note] title\n    ", Owners: []agreementUsedFootnoteOwner{{Field: 1, Note: 0, Node: 25, Paragraph: 1, Ordinal: 1, Line: 0, Label: "n"}}, Changes: []agreementMixedRename{{Field: 1, Span: graph.Span{Start: 34, Stop: 35}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 32, Stop: 38}, {Start: 130, Stop: 154}}, Comments: []graph.Span{{Start: 38, Stop: 208}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 21, Stop: 26}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 32, Stop: 37}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 47, Stop: 58}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 68, Stop: 73}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 87, Stop: 100}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 102, Stop: 107}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 119, Stop: 124}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 130, Stop: 135}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 148, Stop: 153}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 165, Stop: 170}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 175, Stop: 180}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 181, Stop: 186}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Targets: []string{"A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}}}},
		{Name: "c7055c8f8ca2", Body: "## A\n[^n]: [[A]]\n\n    [[B]]\nA\n---\n    - [ ] [[A]]\n\\\\[[A]]%%<!-- [[A]] -->A\n===\n\n<!--\n[[A]]\n--> ^é\n- item\n\n      \\`https://example.invalid/`[[A]]` text [[B|alias]]- [ ] [[A]]\nA\n> [!note] [[A]]\n<!--\n[[A]]\n-->![[A#A]][[A#A]]1. \\\\[[A]]> [!note] title\nref[^n]\n", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 38, Stop: 50}}, {Kind: "CodeBlock", Parent: "ListItem", Span: graph.Span{Start: 113, Stop: 175}}}, Comments: []graph.Span{{Start: 57, Stop: 256}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 22, Stop: 28}, {Start: 38, Stop: 50}, {Start: 113, Stop: 175}}, Comments: []graph.Span{{Start: 57, Stop: 256}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 11, Stop: 16}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 44, Stop: 49}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 52, Stop: 57}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 64, Stop: 69}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 85, Stop: 90}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 140, Stop: 145}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 152, Stop: 163}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 169, Stop: 174}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 187, Stop: 192}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 198, Stop: 203}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 208, Stop: 215}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 215, Stop: 222}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 227, Stop: 232}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 4, Parent: 0, Kind: "CodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 38, Stop: 50, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}, {Root: 0, Node: 16, Parent: 13, Kind: "CodeBlock", ParentKind: "ListItem", Lines: []text.Segment{{Start: 113, Stop: 175, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 9, Parent: 5, Kind: "RawHTML", ParentKind: "Heading", Type: 0, Lines: []text.Segment{{Start: 59, Stop: 73, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}, {Root: 0, Node: 11, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 80, Stop: 85, Padding: 0, ForceNewline: false}, {Start: 85, Stop: 91, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 91, Stop: 99, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}, {Root: 0, Node: 26, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 193, Stop: 198, Padding: 0, ForceNewline: false}, {Start: 198, Stop: 204, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 204, Stop: 248, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 3, Stop: 4}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 3, Stop: 4}}, Raw: false, Soft: false, Hard: false}, {Kind: "ThematicBreak", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "CodeBlock", Depth: 1, Lines: []graph.Span{{Start: 38, Stop: 50}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 50, Stop: 74}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 50, Stop: 53}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 53, Stop: 54}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 54, Stop: 59}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 59, Stop: 73}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 73, Stop: 74}}, Raw: false, Soft: false, Hard: false}, {Kind: "HTMLBlock", Depth: 1, Lines: []graph.Span{{Start: 80, Stop: 85}, {Start: 85, Stop: 91}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "List", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "ListItem", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 101, Stop: 105}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 101, Stop: 105}}, Raw: false, Soft: false, Hard: false}, {Kind: "CodeBlock", Depth: 3, Lines: []graph.Span{{Start: 113, Stop: 175}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 175, Stop: 176}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 175, Stop: 176}}, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 179, Stop: 192}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 179, Stop: 180}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 180, Stop: 188}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 188, Stop: 189}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 189, Stop: 191}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 191, Stop: 192}}, Raw: false, Soft: false, Hard: false}, {Kind: "HTMLBlock", Depth: 1, Lines: []graph.Span{{Start: 193, Stop: 198}, {Start: 198, Stop: 204}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 248, Stop: 255}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 248, Stop: 251}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteLink", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteList", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Footnote", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 11, Stop: 16}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 11, Stop: 12}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 12, Stop: 13}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 13, Stop: 15}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 15, Stop: 16}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 22, Stop: 28}, {Start: 28, Stop: 29}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 22, Stop: 23}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 24}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 24, Stop: 26}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 26, Stop: 27}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 28, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteBacklink", Depth: 4, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 0, Node: 31, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 5, Stop: 30}, Label: graph.Span{Start: 5, Stop: 10}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 38, Stop: 50}}, {Kind: "CodeBlock", Parent: "ListItem", Span: graph.Span{Start: 113, Stop: 175}}}, Comments: []graph.Span{{Start: 57, Stop: 256}}, Unanchorable: nil, Valid: true}, Notes: []agreementUsedFootnoteDeclaration{{Node: 31, Label: "n", Parent: "FootnoteList", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 5, Stop: 30}, Label: graph.Span{Start: 5, Stop: 10}, Parent: "Document"}, Found: true, Paragraphs: []agreementUsedFootnoteParagraph{{Ordinal: 0, Kind: "Paragraph", Lines: []text.Segment{{Start: 11, Stop: 16, Padding: 0, ForceNewline: false}}}, {Ordinal: 1, Kind: "Paragraph", Lines: []text.Segment{{Start: 22, Stop: 28, Padding: 0, ForceNewline: false}, {Start: 28, Stop: 29, Padding: 0, ForceNewline: false}}}}}}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CitationsInCode: 0, Blocks: nil, Headings: []string{"a"}, Failures: nil, CodeCitations: nil, CalloutTitles: nil}, Parts: [][]agreementCodePart{{{Text: "- [ ] [[A]]\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, Targets: []string{"A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementUsedFootnotePartialDiagnosticVariant{{Target: "B", Body: "## A\n[^n]: [[A]]\n\n    [[q]]\nA\n---\n    - [ ] [[A]]\n\\\\[[A]]%%<!-- [[A]] -->A\n===\n\n<!--\n[[A]]\n--> ^é\n- item\n\n      \\`https://example.invalid/`[[A]]` text [[B|alias]]- [ ] [[A]]\nA\n> [!note] [[A]]\n<!--\n[[A]]\n-->![[A#A]][[A#A]]1. \\\\[[A]]> [!note] title\nref[^n]\n", Owners: []agreementUsedFootnoteOwner{{Field: 1, Note: 0, Node: 31, Paragraph: 1, Ordinal: 1, Line: 0, Label: "n"}}, Changes: []agreementMixedRename{{Field: 1, Span: graph.Span{Start: 24, Stop: 25}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 22, Stop: 28}, {Start: 38, Stop: 50}, {Start: 113, Stop: 175}}, Comments: []graph.Span{{Start: 57, Stop: 256}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 11, Stop: 16}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 44, Stop: 49}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 52, Stop: 57}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 64, Stop: 69}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 85, Stop: 90}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 140, Stop: 145}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 152, Stop: 163}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 169, Stop: 174}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 187, Stop: 192}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 198, Stop: 203}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 208, Stop: 215}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 215, Stop: 222}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 227, Stop: 232}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "A"}}, Targets: []string{"A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}}}},
		{Name: "0f243ab846ac", Body: "%%[[A]]%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n    ```\n`0<!-- ^é\nref[^n]\n## <em>A</em>\n```` go [[A]]\n[[A]]- item\n\n      text > [!note] title\n> A\n===\n[^n]: [[A]]\n\n    [[B]]\n> > [[A\\|alias]]~~~\n     ```\n- [x] [[B]]\n## A\n## A\n[[A]] [[A]]\\` ^A\n%%[[A]]%%\\` ^A\n~~~~\n", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 104, Stop: 263}}, {Kind: "FencedCodeBlockInfo", Parent: "Document", Span: graph.Span{Start: 95, Stop: 103}}}, Comments: []graph.Span{{Start: 0, Stop: 9}, {Start: 58, Stop: 263}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 36, Stop: 50}, {Start: 104, Stop: 263}}, Comments: []graph.Span{{Start: 0, Stop: 9}, {Start: 58, Stop: 263}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 2, Stop: 7}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 36, Stop: 41}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 98, Stop: 103}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 104, Stop: 109}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 158, Stop: 163}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 169, Stop: 174}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 179, Stop: 191}, Word: "[[A\\|alias]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 210, Stop: 215}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 226, Stop: 231}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 232, Stop: 237}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 245, Stop: 250}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 17, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 104, Stop: 116, Padding: 0, ForceNewline: true}, {Start: 116, Stop: 117, Padding: 0, ForceNewline: true}, {Start: 117, Stop: 144, Padding: 0, ForceNewline: true}, {Start: 144, Stop: 148, Padding: 0, ForceNewline: true}, {Start: 148, Stop: 152, Padding: 0, ForceNewline: true}, {Start: 152, Stop: 164, Padding: 0, ForceNewline: true}, {Start: 164, Stop: 165, Padding: 0, ForceNewline: true}, {Start: 165, Stop: 175, Padding: 0, ForceNewline: true}, {Start: 175, Stop: 195, Padding: 0, ForceNewline: true}, {Start: 195, Stop: 204, Padding: 0, ForceNewline: true}, {Start: 204, Stop: 216, Padding: 0, ForceNewline: true}, {Start: 216, Stop: 221, Padding: 0, ForceNewline: true}, {Start: 221, Stop: 226, Padding: 0, ForceNewline: true}, {Start: 226, Stop: 243, Padding: 0, ForceNewline: true}, {Start: 243, Stop: 258, Padding: 0, ForceNewline: true}, {Start: 258, Stop: 263, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 95, Stop: 103, Padding: 0, ForceNewline: false}, HasInfo: true, Raw: true}, {Root: 0, Node: 30, Parent: 19, Kind: "FencedCodeBlock", ParentKind: "Footnote", Lines: nil, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 14, Parent: 13, Kind: "RawHTML", ParentKind: "Heading", Type: 0, Lines: []text.Segment{{Start: 79, Stop: 83, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}, {Root: 0, Node: 16, Parent: 13, Kind: "RawHTML", ParentKind: "Heading", Type: 0, Lines: []text.Segment{{Start: 84, Stop: 89, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}, {Root: 0, Node: 17, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Type: 0, Lines: []text.Segment{{Start: 104, Stop: 116, Padding: 0, ForceNewline: true}, {Start: 116, Stop: 117, Padding: 0, ForceNewline: true}, {Start: 117, Stop: 144, Padding: 0, ForceNewline: true}, {Start: 144, Stop: 148, Padding: 0, ForceNewline: true}, {Start: 148, Stop: 152, Padding: 0, ForceNewline: true}, {Start: 152, Stop: 164, Padding: 0, ForceNewline: true}, {Start: 164, Stop: 165, Padding: 0, ForceNewline: true}, {Start: 165, Stop: 175, Padding: 0, ForceNewline: true}, {Start: 175, Stop: 195, Padding: 0, ForceNewline: true}, {Start: 195, Stop: 204, Padding: 0, ForceNewline: true}, {Start: 204, Stop: 216, Padding: 0, ForceNewline: true}, {Start: 216, Stop: 221, Padding: 0, ForceNewline: true}, {Start: 221, Stop: 226, Padding: 0, ForceNewline: true}, {Start: 226, Stop: 243, Padding: 0, ForceNewline: true}, {Start: 243, Stop: 258, Padding: 0, ForceNewline: true}, {Start: 258, Stop: 263, Padding: 0, ForceNewline: true}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 95, Stop: 103, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: true, Raw: true}, {Root: 0, Node: 30, Parent: 19, Kind: "FencedCodeBlock", ParentKind: "Footnote", Type: 0, Lines: nil, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 0, Stop: 13}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 0, Stop: 3}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 3, Stop: 4}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 4, Stop: 11}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 11, Stop: 13}}, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 17, Stop: 18}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 50, Stop: 68}, {Start: 68, Stop: 75}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 50, Stop: 62}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 62, Stop: 67}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 68, Stop: 71}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteLink", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 79, Stop: 89}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 79, Stop: 83}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 83, Stop: 84}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 84, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "FencedCodeBlock", Depth: 1, Lines: []graph.Span{{Start: 104, Stop: 116}, {Start: 116, Stop: 117}, {Start: 117, Stop: 144}, {Start: 144, Stop: 148}, {Start: 148, Stop: 152}, {Start: 152, Stop: 164}, {Start: 164, Stop: 165}, {Start: 165, Stop: 175}, {Start: 175, Stop: 195}, {Start: 195, Stop: 204}, {Start: 204, Stop: 216}, {Start: 216, Stop: 221}, {Start: 221, Stop: 226}, {Start: 226, Stop: 243}, {Start: 243, Stop: 258}, {Start: 258, Stop: 263}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteList", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Footnote", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 25, Stop: 30}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 25, Stop: 26}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 26, Stop: 27}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 27, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 30}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 36, Stop: 41}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 36, Stop: 37}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 37, Stop: 38}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 38, Stop: 40}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 40, Stop: 41}}, Raw: false, Soft: false, Hard: false}, {Kind: "FencedCodeBlock", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteBacklink", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 0, Node: 19, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 19, Stop: 50}, Label: graph.Span{Start: 19, Stop: 24}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 104, Stop: 263}}, {Kind: "FencedCodeBlockInfo", Parent: "Document", Span: graph.Span{Start: 95, Stop: 103}}}, Comments: []graph.Span{{Start: 0, Stop: 9}, {Start: 58, Stop: 263}}, Unanchorable: nil, Valid: true}, Notes: []agreementUsedFootnoteDeclaration{{Node: 19, Label: "n", Parent: "FootnoteList", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 19, Stop: 50}, Label: graph.Span{Start: 19, Stop: 24}, Parent: "Document"}, Found: true, Paragraphs: []agreementUsedFootnoteParagraph{{Ordinal: 0, Kind: "Paragraph", Lines: []text.Segment{{Start: 25, Stop: 30, Padding: 0, ForceNewline: false}}}, {Ordinal: 1, Kind: "Paragraph", Lines: []text.Segment{{Start: 36, Stop: 41, Padding: 0, ForceNewline: false}}}}}}, Page: agreementHTML{Citations: nil, CitationsInCode: 0, Blocks: nil, Headings: []string{"a", "a-2"}, Failures: nil, CodeCitations: nil, CalloutTitles: nil}, Parts: nil, Targets: []string{"A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementUsedFootnotePartialDiagnosticVariant{{Target: "B", Body: "%%[[A]]%%## A\n## A\n[^n]: [[A]]\n\n    [[q]]\n    ```\n`0<!-- ^é\nref[^n]\n## <em>A</em>\n```` go [[A]]\n[[A]]- item\n\n      text > [!note] title\n> A\n===\n[^n]: [[A]]\n\n    [[B]]\n> > [[A\\|alias]]~~~\n     ```\n- [x] [[B]]\n## A\n## A\n[[A]] [[A]]\\` ^A\n%%[[A]]%%\\` ^A\n~~~~\n", Owners: []agreementUsedFootnoteOwner{{Field: 2, Note: 0, Node: 19, Paragraph: 1, Ordinal: 1, Line: 0, Label: "n"}}, Changes: []agreementMixedRename{{Field: 2, Span: graph.Span{Start: 38, Stop: 39}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 36, Stop: 50}, {Start: 104, Stop: 263}}, Comments: []graph.Span{{Start: 0, Stop: 9}, {Start: 58, Stop: 263}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 2, Stop: 7}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 36, Stop: 41}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 98, Stop: 103}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 104, Stop: 109}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 158, Stop: 163}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 169, Stop: 174}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 179, Stop: 191}, Word: "[[A\\|alias]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 210, Stop: 215}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 226, Stop: 231}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 232, Stop: 237}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 245, Stop: 250}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"A"}}, Targets: []string{"A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}}}},
		{Name: "168171f654f5", Body: "[^n]: [[A]]\n\n    [[B]]\n%%[[A]]%%A\n^absent-prose ^absent-prose >   > [!note] title\n%%[[B|alias]]`B\n\t> [!note] title\n> [!note] [[A]]\n\\`[[A\\]]1. ```` go [[A]]\n^absent-prose 1. ````\n\tref[^n]\n> [!note] [[A]]\n", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 147, Stop: 172}}}, Comments: []graph.Span{{Start: 23, Stop: 32}, {Start: 82, Stop: 203}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 23}, {Start: 147, Stop: 172}}, Comments: []graph.Span{{Start: 23, Stop: 32}, {Start: 82, Stop: 203}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 84, Stop: 95}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 125, Stop: 130}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 133, Stop: 139}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 150, Stop: 155}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 197, Stop: 202}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 12, Parent: 2, Kind: "CodeSpan", ParentKind: "Paragraph", Lines: nil, Text: []text.Segment{{Start: 147, Stop: 156, Padding: 0, ForceNewline: false}, {Start: 156, Stop: 172, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}}, HTML: nil, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 117, Stop: 131}, {Start: 131, Stop: 156}, {Start: 156, Stop: 178}, {Start: 179, Stop: 187}, {Start: 189, Stop: 202}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 117, Stop: 118}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 118, Stop: 126}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 126, Stop: 127}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 127, Stop: 129}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 129, Stop: 130}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 131, Stop: 133}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 133, Stop: 134}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 134, Stop: 135}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 135, Stop: 142}}, Raw: false, Soft: false, Hard: false}, {Kind: "CodeSpan", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 147, Stop: 156}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 156, Stop: 172}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 177, Stop: 177}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 179, Stop: 182}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteLink", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 186, Stop: 186}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 189, Stop: 190}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 190, Stop: 198}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 198, Stop: 199}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 199, Stop: 201}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 201, Stop: 202}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteList", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Footnote", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 6, Stop: 11}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 6, Stop: 7}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 7, Stop: 8}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 8, Stop: 10}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 10, Stop: 11}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 17, Stop: 23}, {Start: 23, Stop: 34}, {Start: 34, Stop: 82}, {Start: 82, Stop: 97}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 19}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 21}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 22}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 26}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 26, Stop: 27}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 27, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 33}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 34, Stop: 69}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 69, Stop: 75}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 75, Stop: 81}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 82, Stop: 85}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 85, Stop: 86}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 86, Stop: 95}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 95, Stop: 96}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 96, Stop: 97}}, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 4, Lines: []graph.Span{{Start: 101, Stop: 114}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 5, Lines: nil, Text: []graph.Span{{Start: 101, Stop: 102}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 5, Lines: nil, Text: []graph.Span{{Start: 102, Stop: 108}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 5, Lines: nil, Text: []graph.Span{{Start: 108, Stop: 114}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteBacklink", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 0, Node: 25, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 0, Stop: 115}, Label: graph.Span{Start: 0, Stop: 5}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 147, Stop: 172}}}, Comments: []graph.Span{{Start: 23, Stop: 32}, {Start: 82, Stop: 203}}, Unanchorable: nil, Valid: true}, Notes: []agreementUsedFootnoteDeclaration{{Node: 25, Label: "n", Parent: "FootnoteList", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 0, Stop: 115}, Label: graph.Span{Start: 0, Stop: 5}, Parent: "Document"}, Found: true, Paragraphs: []agreementUsedFootnoteParagraph{{Ordinal: 0, Kind: "Paragraph", Lines: []text.Segment{{Start: 6, Stop: 11, Padding: 0, ForceNewline: false}}}, {Ordinal: 1, Kind: "Paragraph", Lines: []text.Segment{{Start: 17, Stop: 23, Padding: 0, ForceNewline: false}, {Start: 23, Stop: 34, Padding: 0, ForceNewline: false}, {Start: 34, Stop: 82, Padding: 0, ForceNewline: false}, {Start: 82, Stop: 97, Padding: 0, ForceNewline: false}}}}}}, Page: agreementHTML{Citations: nil, CitationsInCode: 0, Blocks: nil, Headings: nil, Failures: nil, CodeCitations: nil, CalloutTitles: nil}, Parts: nil, Targets: []string{"A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementUsedFootnotePartialDiagnosticVariant{{Target: "B", Body: "[^n]: [[A]]\n\n    [[q]]\n%%[[A]]%%A\n^absent-prose ^absent-prose >   > [!note] title\n%%[[B|alias]]`B\n\t> [!note] title\n> [!note] [[A]]\n\\`[[A\\]]1. ```` go [[A]]\n^absent-prose 1. ````\n\tref[^n]\n> [!note] [[A]]\n", Owners: []agreementUsedFootnoteOwner{{Field: 1, Note: 0, Node: 25, Paragraph: 1, Ordinal: 1, Line: 0, Label: "n"}}, Changes: []agreementMixedRename{{Field: 1, Span: graph.Span{Start: 19, Stop: 20}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 23}, {Start: 147, Stop: 172}}, Comments: []graph.Span{{Start: 23, Stop: 32}, {Start: 82, Stop: 203}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 84, Stop: 95}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 125, Stop: 130}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 133, Stop: 139}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 150, Stop: 155}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 197, Stop: 202}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Targets: []string{"A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}}}},
	}
}
func agreementUsedFootnotePartialDiagnosticWitnesses() []agreementUsedFootnotePartialDiagnosticWitness {
	return []agreementUsedFootnotePartialDiagnosticWitness{
		{Name: "8f534fbe487b", Body: "## A\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n%%   ```\n[[B|alias]]## !\n<!-- [[A]] -->  ```\n ^A\n[[image.png]]\\\\[[A]]>   - [^n]: [[A]]\n\n    [[B]]\n\t[^unused]: [[A]]\nA\n===\n<!--\n[[A]]\n--> [[A]] [[A]]  > [!note] title\n    ", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 7 of the note body hides everything after it", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 7 of the note body hides everything after it", Section: "", Block: ""}}, Indices: []int{1}, Singles: []agreementUsedFootnotePartialDiagnosticSingle{{Field: 1, Index: 1, Start: 34, Body: "## A\n## A\n## A\n[^n]: [[A]]\n\n    [[q]]\n%%   ```\n[[B|alias]]## !\n<!-- [[A]] -->  ```\n ^A\n[[image.png]]\\\\[[A]]>   - [^n]: [[A]]\n\n    [[B]]\n\t[^unused]: [[A]]\nA\n===\n<!--\n[[A]]\n--> [[A]] [[A]]  > [!note] title\n    ", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 7 of the note body hides everything after it", Section: "", Block: ""}}}}},
		{Name: "c7055c8f8ca2", Body: "## A\n[^n]: [[A]]\n\n    [[B]]\nA\n---\n    - [ ] [[A]]\n\\\\[[A]]%%<!-- [[A]] -->A\n===\n\n<!--\n[[A]]\n--> ^é\n- item\n\n      \\`https://example.invalid/`[[A]]` text [[B|alias]]- [ ] [[A]]\nA\n> [!note] [[A]]\n<!--\n[[A]]\n-->![[A#A]][[A#A]]1. \\\\[[A]]> [!note] title\nref[^n]\n", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 8 of the note body hides everything after it", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 8 of the note body hides everything after it", Section: "", Block: ""}}, Indices: []int{1}, Singles: []agreementUsedFootnotePartialDiagnosticSingle{{Field: 1, Index: 1, Start: 24, Body: "## A\n[^n]: [[A]]\n\n    [[q]]\nA\n---\n    - [ ] [[A]]\n\\\\[[A]]%%<!-- [[A]] -->A\n===\n\n<!--\n[[A]]\n--> ^é\n- item\n\n      \\`https://example.invalid/`[[A]]` text [[B|alias]]- [ ] [[A]]\nA\n> [!note] [[A]]\n<!--\n[[A]]\n-->![[A#A]][[A#A]]1. \\\\[[A]]> [!note] title\nref[^n]\n", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 8 of the note body hides everything after it", Section: "", Block: ""}}}}},
		{Name: "0f243ab846ac", Body: "%%[[A]]%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n    ```\n`0<!-- ^é\nref[^n]\n## <em>A</em>\n```` go [[A]]\n[[A]]- item\n\n      text > [!note] title\n> A\n===\n[^n]: [[A]]\n\n    [[B]]\n> > [[A\\|alias]]~~~\n     ```\n- [x] [[B]]\n## A\n## A\n[[A]] [[A]]\\` ^A\n%%[[A]]%%\\` ^A\n~~~~\n", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "<!--", Message: "an unclosed <!-- comment opened at line 7 of the note body hides everything after it", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "<!--", Message: "an unclosed <!-- comment opened at line 7 of the note body hides everything after it", Section: "", Block: ""}}, Indices: []int{1}, Singles: []agreementUsedFootnotePartialDiagnosticSingle{{Field: 2, Index: 1, Start: 38, Body: "%%[[A]]%%## A\n## A\n[^n]: [[A]]\n\n    [[q]]\n    ```\n`0<!-- ^é\nref[^n]\n## <em>A</em>\n```` go [[A]]\n[[A]]- item\n\n      text > [!note] title\n> A\n===\n[^n]: [[A]]\n\n    [[B]]\n> > [[A\\|alias]]~~~\n     ```\n- [x] [[B]]\n## A\n## A\n[[A]] [[A]]\\` ^A\n%%[[A]]%%\\` ^A\n~~~~\n", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "<!--", Message: "an unclosed <!-- comment opened at line 7 of the note body hides everything after it", Section: "", Block: ""}}}}},
		{Name: "168171f654f5", Body: "[^n]: [[A]]\n\n    [[B]]\n%%[[A]]%%A\n^absent-prose ^absent-prose >   > [!note] title\n%%[[B|alias]]`B\n\t> [!note] title\n> [!note] [[A]]\n\\`[[A\\]]1. ```` go [[A]]\n^absent-prose 1. ````\n\tref[^n]\n> [!note] [[A]]\n", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 6 of the note body hides everything after it", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 6 of the note body hides everything after it", Section: "", Block: ""}}, Indices: []int{1}, Singles: []agreementUsedFootnotePartialDiagnosticSingle{{Field: 1, Index: 1, Start: 19, Body: "[^n]: [[A]]\n\n    [[q]]\n%%[[A]]%%A\n^absent-prose ^absent-prose >   > [!note] title\n%%[[B|alias]]`B\n\t> [!note] title\n> [!note] [[A]]\n\\`[[A\\]]1. ```` go [[A]]\n^absent-prose 1. ````\n\tref[^n]\n> [!note] [[A]]\n", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 6 of the note body hides everything after it", Section: "", Block: ""}}}}},
	}
}
func TestAgreementUsedFootnotePartialDiagnosticOwnedSource(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnotePartialDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementUsedFootnotePartialDiagnosticSource(r.Body, r.Target)
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
				if !cmp.Equal(native, w.Native) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(agreementUsedFootnoteReading(single.Body), w.Notes) || !cmp.Equal(check, original) || !cmp.Equal(agreementNativeCodePayloadSites(single.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(single.Body), w.HTML) {
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
					if diag.Kind != render.DiagWikilinkBroken || diag.Target != r.Target || private.Target != change.New || private.Kind != diag.Kind || private.Section != diag.Section || private.Block != diag.Block || private.Message != strings.ReplaceAll(diag.Message, "\""+r.Target+"\"", "\""+change.New+"\"") {
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

func TestAgreementUsedFootnotePartialDiagnosticOwnedPublic(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnotePartialDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementUsedFootnotePartialDiagnosticSource(r.Body, r.Target)
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
				k, a, b := agreementUsedFootnotePartialDiagnosticDifference(c, &f, raw.HTML, &page, raw.Diagnostics, failures)
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

func TestAgreementUsedFootnotePartialDiagnosticOwnedRefusals(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnotePartialDiagnosticWitnesses()
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
					k, a, b := agreementUsedFootnotePartialDiagnosticDifference(c, &changed, raw.HTML, &page, raw.Diagnostics, failures)
					if k != "" || a != "" || b != "" {
						t.Fatal("caught: discarded diagnostic borrowed another signed face")
					}
				})
			}
			for _, other := range []agreementCase{{Body: r.Body + "\n"}, {Body: r.Body, Title: "A"}, {Body: r.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
				k, a, b := agreementUsedFootnotePartialDiagnosticDifference(other, &selected, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed another body or context")
				}
			}
			for i := range failures {
				missing := slices.Delete(slices.Clone(failures), i, i+1)
				k, a, b := agreementUsedFootnotePartialDiagnosticDifference(c, &selected, raw.HTML, &page, raw.Diagnostics, missing)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed an incomplete whole signed pool")
				}
			}
			for i := range raw.Diagnostics {
				missing := slices.Delete(slices.Clone(raw.Diagnostics), i, i+1)
				k, a, b := agreementUsedFootnotePartialDiagnosticDifference(c, &selected, raw.HTML, &page, missing, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed an incomplete whole raw ledger")
				}
				altered := slices.Clone(raw.Diagnostics)
				altered[i].Message += "outside"
				k, a, b = agreementUsedFootnotePartialDiagnosticDifference(c, &selected, raw.HTML, &page, altered, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed another complete raw ledger")
				}
			}
			changed := page
			changed.Headings = append(slices.Clone(page.Headings), "outside")
			k, a, b := agreementUsedFootnotePartialDiagnosticDifference(c, &selected, raw.HTML, &changed, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: discarded diagnostic borrowed another complete public ledger")
			}
			k, a, b = agreementUsedFootnotePartialDiagnosticDifference(c, &selected, raw.HTML+"<code>outside</code>", &page, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: discarded diagnostic borrowed changed field-free code text")
			}
		})
	}
}

func TestAgreementUsedFootnotePartialDiagnosticOwnedWholeTargets(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnotePartialDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementUsedFootnotePartialDiagnosticSource(r.Body, r.Target)
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
			replacement := strings.Repeat("q", len(r.Target))
			n := 0
			for _, owner := range v.Owners {
				field := w.Check.Fields[owner.Field]
				if !field.Code && !field.Comment && !field.Escaped {
					n++
				}
			}
			if before[r.Target]-after[r.Target] != n || after[replacement]-before[replacement] != n {
				t.Fatal("caught: discarded diagnostic target counts conserve every physical owner")
			}
			delete(before, r.Target)
			delete(before, replacement)
			delete(after, r.Target)
			delete(after, replacement)
			if !maps.Equal(before, after) {
				t.Fatal("caught: every unselected whole discarded diagnostic target count")
			}
		})
	}
}

func TestAgreementUsedFootnotePartialDiagnosticOwnedWholeSet(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnotePartialDiagnosticWitnesses()
	witnesses := agreementUsedFootnotePartialDiagnosticSources()
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
		notes := agreementUsedFootnoteReading(w.Body)
		failures := agreementPageFailures(w.Body, &raw, &page)
		failures = append(failures, agreementFragmentFailures(t, []agreementCase{{Body: w.Body}}, []agreementHTML{page})[0]...)
		bodySelected := make(map[string]int)
		bodyVariants := make(map[[2]string]int)
		for _, f := range failures {
			if f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.Target != "B" || f.Tuple.State != "wikilink-broken" {
				continue
			}
			target := f.Tuple.Target
			owners, ok := agreementUsedFootnotePartialDiagnosticOwners(&check, notes)
			private, changes, valid := agreementUsedFootnotePartialDiagnosticPrivate(w.Body, &check, notes)
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
					if x.Property == "P0" && (x.Tuple.Target == target || x.Tuple.Target == changes[0].New) {
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
				if diag.Kind != render.DiagWikilinkBroken || diag.Target != target || renamed.Target != change.New || renamed.Kind != diag.Kind || renamed.Section != diag.Section || renamed.Block != diag.Block || renamed.Message != strings.ReplaceAll(diag.Message, "\""+target+"\"", "\""+change.New+"\"") {
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
	if !maps.Equal(expected, actual) || len(actual) != 4 || fields != 4 || len(witnesses) != 4 {
		t.Fatal("caught: whole declared-body discarded diagnostic eligibility census")
	}
}
