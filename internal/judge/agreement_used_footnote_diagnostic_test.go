package judge_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementUsedFootnoteDiagnosticSingle struct {
	Field, Index, Start int
	Body                string
	Diagnostics         []render.Diagnostic
}
type agreementUsedFootnoteDiagnosticWitness struct {
	Name, Body, Target, Selected    string
	Diagnostics, PrivateDiagnostics []render.Diagnostic
	Indices                         []int
	Singles                         []agreementUsedFootnoteDiagnosticSingle
}

// Every renamed field is a later paragraph of one declared used definition.
func agreementUsedFootnoteDiagnosticPrivate(body string, check *agreementNativeCheckSource, notes []agreementUsedFootnoteDeclaration) (string, []agreementMixedRename, bool) {
	private, owners, valid := agreementUsedFootnotePrivate(body, check, notes)
	if !valid {
		return "", nil, false
	}
	var changes []agreementMixedRename
	for _, owner := range owners {
		f := check.Fields[owner.Field]
		changes = append(changes, agreementMixedRename{Field: owner.Field, Span: graph.Span{Start: f.Span.Start + 2, Stop: f.Span.Start + 3}, Old: "B", New: "q"})
	}
	return private, changes, true
}

func agreementUsedFootnoteDiagnosticSource(body, target string) (*agreementUsedFootnoteDiagnosticSourceWitness, *agreementUsedFootnoteDiagnosticVariant, bool) {
	witnesses := agreementUsedFootnoteDiagnosticSources()
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
func agreementUsedFootnoteDiagnosticDifference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	if f == nil || page == nil || c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Multiplicity < 1 || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Tuple.Section != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	records := agreementUsedFootnoteDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		if c.Body != r.Body || f.Tuple.Target != r.Target {
			continue
		}
		if agreementSignature(f) != r.Selected || !cmp.Equal(diagnostics, r.Diagnostics) {
			return "", "", ""
		}
		w, v, found := agreementUsedFootnoteDiagnosticSource(c.Body, r.Target)
		if !found {
			return "", "", ""
		}
		native := agreementNativeOwnerReading(c.Body)
		check := agreementNativeCheckReading(c.Body, native)
		tree := agreementNativeProseAddressTreeReading(c.Body, nil)
		notes := agreementUsedFootnoteReading(c.Body)
		parts, pok := agreementCodeWindowParts(raw)
		owners, ok := agreementUsedFootnoteOwners(&check, notes)
		private, changes, valid := agreementUsedFootnoteDiagnosticPrivate(c.Body, &check, notes)
		if !pok || !ok || !valid || len(owners) != f.Multiplicity || len(r.Indices) != len(owners) || !cmp.Equal(owners, v.Owners) || !cmp.Equal(changes, v.Changes) || private != v.Body || !cmp.Equal(native, w.Native) || !cmp.Equal(check, w.Check) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(notes, w.Notes) || !cmp.Equal(agreementNativeCodePayloadSites(c.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), w.HTML) || !cmp.Equal(judge.LinkTargets(c.Body), w.Targets) || !cmp.Equal(parts, w.Parts) || !cmp.Equal(*page, w.Page) || !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
			return "", "", ""
		}
		return "debt", "#1011 stage 5", "page-diagnostic"
	}
	return "", "", ""
}

type agreementUsedFootnoteDiagnosticVariant struct {
	Target, Body string
	Owners       []agreementUsedFootnoteOwner
	Changes      []agreementMixedRename
	Check        agreementNativeCheckSource
	Targets      []string
	Signatures   map[string]int
}
type agreementUsedFootnoteDiagnosticSourceWitness struct {
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
	Variants   []agreementUsedFootnoteDiagnosticVariant
}

func agreementUsedFootnoteDiagnosticSources() []agreementUsedFootnoteDiagnosticSourceWitness {
	return []agreementUsedFootnoteDiagnosticSourceWitness{
		{Name: "b69109464c1a", Body: "![[A#A]]É\n[^n]: [[A]]\n\n    [[B]]\n    ```\n`## <em>A</em>\n\t```\ntext <!--# A\n ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\nref[^n]\n![[A#A]]![[A#A]]-->[[A\\|alias]]", Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: []graph.Span{{Start: 67, Stop: 141}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 28, Stop: 42}}, Comments: []graph.Span{{Start: 67, Stop: 141}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 1, Stop: 8}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 28, Stop: 33}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 101, Stop: 106}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 123, Stop: 130}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 131, Stop: 138}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 141, Stop: 153}, Word: "[[A\\|alias]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "A", "A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 59, Parent: 48, Kind: "FencedCodeBlock", ParentKind: "Footnote", Lines: nil, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 8, Parent: 6, Kind: "RawHTML", ParentKind: "Paragraph", Type: 0, Lines: []text.Segment{{Start: 46, Stop: 50, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}, {Root: 0, Node: 10, Parent: 6, Kind: "RawHTML", ParentKind: "Paragraph", Type: 0, Lines: []text.Segment{{Start: 51, Stop: 56, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}, {Root: 0, Node: 59, Parent: 48, Kind: "FencedCodeBlock", ParentKind: "Footnote", Type: 0, Lines: nil, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 0, Stop: 10}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 0, Stop: 2}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 2, Stop: 3}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 3, Stop: 7}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 7, Stop: 10}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 42, Stop: 57}, {Start: 58, Stop: 62}, {Start: 62, Stop: 75}, {Start: 76, Stop: 78}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 42, Stop: 46}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 46, Stop: 50}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 50, Stop: 51}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 51, Stop: 56}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 56, Stop: 56}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 58, Stop: 61}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 61, Stop: 61}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 62, Stop: 72}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 72, Stop: 74}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 76, Stop: 78}}, Raw: false, Soft: false, Hard: false}, {Kind: "Table", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TableHeader", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: []graph.Span{{Start: 81, Stop: 82}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 81, Stop: 82}}, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: []graph.Span{{Start: 85, Stop: 86}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 85, Stop: 86}}, Raw: false, Soft: false, Hard: false}, {Kind: "TableRow", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: []graph.Span{{Start: 101, Stop: 106}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 101, Stop: 102}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 102, Stop: 103}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 103, Stop: 105}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 105, Stop: 106}}, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: []graph.Span{{Start: 109, Stop: 111}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 109, Stop: 111}}, Raw: false, Soft: false, Hard: false}, {Kind: "TableRow", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: []graph.Span{{Start: 114, Stop: 121}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 114, Stop: 117}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteLink", Depth: 4, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TableRow", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: []graph.Span{{Start: 122, Stop: 153}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 122, Stop: 124}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 124, Stop: 125}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 125, Stop: 132}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 132, Stop: 133}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 133, Stop: 142}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 142, Stop: 143}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 143, Stop: 152}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 152, Stop: 153}}, Raw: false, Soft: false, Hard: false}, {Kind: "TableCell", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteList", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Footnote", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 17, Stop: 22}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 19}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 21}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 22}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 28, Stop: 33}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 28, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 30}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 30, Stop: 32}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 32, Stop: 33}}, Raw: false, Soft: false, Hard: false}, {Kind: "FencedCodeBlock", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteBacklink", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 0, Node: 48, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 11, Stop: 42}, Label: graph.Span{Start: 11, Stop: 16}, Parent: "Document"}, Found: true}}, Code: nil, Comments: []graph.Span{{Start: 67, Stop: 141}}, Unanchorable: nil, Valid: true}, Notes: []agreementUsedFootnoteDeclaration{{Node: 48, Label: "n", Parent: "FootnoteList", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 11, Stop: 42}, Label: graph.Span{Start: 11, Stop: 16}, Parent: "Document"}, Found: true, Paragraphs: []agreementUsedFootnoteParagraph{{Ordinal: 0, Kind: "Paragraph", Lines: []text.Segment{{Start: 17, Stop: 22, Padding: 0, ForceNewline: false}}}, {Ordinal: 1, Kind: "Paragraph", Lines: []text.Segment{{Start: 28, Stop: 33, Padding: 0, ForceNewline: false}}}}}}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "A", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CitationsInCode: 0, Blocks: nil, Headings: nil, Failures: nil, CodeCitations: nil, CalloutTitles: nil}, Parts: nil, Targets: []string{"A", "A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementUsedFootnoteDiagnosticVariant{{Target: "B", Body: "![[A#A]]É\n[^n]: [[A]]\n\n    [[q]]\n    ```\n`## <em>A</em>\n\t```\ntext <!--# A\n ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\nref[^n]\n![[A#A]]![[A#A]]-->[[A\\|alias]]", Owners: []agreementUsedFootnoteOwner{{Field: 2, Note: 0, Node: 48, Paragraph: 1, Ordinal: 1, Line: 0, Label: "n"}}, Changes: []agreementMixedRename{{Field: 2, Span: graph.Span{Start: 30, Stop: 31}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 28, Stop: 42}}, Comments: []graph.Span{{Start: 67, Stop: 141}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 1, Stop: 8}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 28, Stop: 33}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 101, Stop: 106}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 123, Stop: 130}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 131, Stop: 138}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 141, Stop: 153}, Word: "[[A\\|alias]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "A", "A"}}, Targets: []string{"A", "A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}}}},
		{Name: "399c86bb410f", Body: "## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->~~~~\n> > [[image.png]]> [!note] title\n[[A\nB]]# A\n\\[[A]]ref[^n]\nÉ\n<!-- [[A]] --><!-- [[A]] -->   ```\nhttps://example.invalid/`[[A]]` 章節\n~~~~\n章節\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%- ~~~~\n> [!unknown] title\n", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 202, Stop: 272}}}, Comments: []graph.Span{{Start: 42, Stop: 56}, {Start: 123, Stop: 137}, {Start: 137, Stop: 151}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 36, Stop: 42}, {Start: 183, Stop: 188}, {Start: 202, Stop: 272}}, Comments: []graph.Span{{Start: 42, Stop: 56}, {Start: 123, Stop: 137}, {Start: 137, Stop: 151}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 14}, Word: "[[A|alias]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 36, Stop: 41}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 47, Stop: 52}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 65, Stop: 78}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 94, Stop: 101}, Word: "[[A\nB]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 106, Stop: 111}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 128, Stop: 133}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 142, Stop: 147}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 183, Stop: 188}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 231, Stop: 236}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"A", "A", "image.png"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 31, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 202, Stop: 209, Padding: 0, ForceNewline: true}, {Start: 209, Stop: 219, Padding: 0, ForceNewline: true}, {Start: 219, Stop: 229, Padding: 0, ForceNewline: true}, {Start: 229, Stop: 244, Padding: 0, ForceNewline: true}, {Start: 244, Stop: 253, Padding: 0, ForceNewline: true}, {Start: 253, Stop: 272, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 8, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 42, Stop: 47, Padding: 0, ForceNewline: false}, {Start: 47, Stop: 53, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 53, Stop: 61, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}, {Root: 0, Node: 27, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 123, Stop: 158, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: -1, Stop: -1, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}, {Root: 0, Node: 31, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Type: 0, Lines: []text.Segment{{Start: 202, Stop: 209, Padding: 0, ForceNewline: true}, {Start: 209, Stop: 219, Padding: 0, ForceNewline: true}, {Start: 219, Stop: 229, Padding: 0, ForceNewline: true}, {Start: 229, Stop: 244, Padding: 0, ForceNewline: true}, {Start: 244, Stop: 253, Padding: 0, ForceNewline: true}, {Start: 253, Stop: 272, Padding: 0, ForceNewline: true}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 3, Stop: 14}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 3, Stop: 4}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 4, Stop: 5}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 5, Stop: 13}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 13, Stop: 14}}, Raw: false, Soft: false, Hard: false}, {Kind: "Heading", Depth: 1, Lines: []graph.Span{{Start: 17, Stop: 18}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "HTMLBlock", Depth: 1, Lines: []graph.Span{{Start: 42, Stop: 47}, {Start: 47, Stop: 53}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 65, Stop: 94}, {Start: 94, Stop: 98}, {Start: 98, Stop: 105}, {Start: 105, Stop: 119}, {Start: 119, Stop: 122}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 65, Stop: 66}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 66, Stop: 67}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 67, Stop: 81}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 81, Stop: 87}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 87, Stop: 93}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 94, Stop: 95}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 95, Stop: 96}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 96, Stop: 97}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 98, Stop: 102}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 102, Stop: 104}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 105, Stop: 108}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 108, Stop: 114}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteLink", Depth: 4, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 118, Stop: 118}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 119, Stop: 122}}, Raw: false, Soft: false, Hard: false}, {Kind: "HTMLBlock", Depth: 1, Lines: []graph.Span{{Start: 123, Stop: 158}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 158, Stop: 196}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "AutoLink", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 189, Stop: 196}}, Raw: false, Soft: false, Hard: false}, {Kind: "FencedCodeBlock", Depth: 1, Lines: []graph.Span{{Start: 202, Stop: 209}, {Start: 209, Stop: 219}, {Start: 219, Stop: 229}, {Start: 229, Stop: 244}, {Start: 244, Stop: 253}, {Start: 253, Stop: 272}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteList", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Footnote", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 25, Stop: 30}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 25, Stop: 26}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 26, Stop: 27}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 27, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 30}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 3, Lines: []graph.Span{{Start: 36, Stop: 41}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 36, Stop: 37}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 37, Stop: 38}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 38, Stop: 40}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 40, Stop: 41}}, Raw: false, Soft: false, Hard: false}, {Kind: "FootnoteBacklink", Depth: 4, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 0, Node: 33, Label: "n", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 19, Stop: 42}, Label: graph.Span{Start: 19, Stop: 24}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 202, Stop: 272}}}, Comments: []graph.Span{{Start: 42, Stop: 56}, {Start: 123, Stop: 137}, {Start: 137, Stop: 151}}, Unanchorable: nil, Valid: true}, Notes: []agreementUsedFootnoteDeclaration{{Node: 33, Label: "n", Parent: "FootnoteList", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 19, Stop: 42}, Label: graph.Span{Start: 19, Stop: 24}, Parent: "Document"}, Found: true, Paragraphs: []agreementUsedFootnoteParagraph{{Ordinal: 0, Kind: "Paragraph", Lines: []text.Segment{{Start: 25, Stop: 30, Padding: 0, ForceNewline: false}}}, {Ordinal: 1, Kind: "Paragraph", Lines: []text.Segment{{Start: 36, Stop: 41, Padding: 0, ForceNewline: false}}}}}}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CitationsInCode: 0, Blocks: nil, Headings: []string{"alias", "a"}, Failures: nil, CodeCitations: nil, CalloutTitles: nil}, Parts: [][]agreementCodePart{{{Text: "> > [[image.png]]> [!note] title\n[[A\nB]]# A\n\\[[A]]ref[^n]\nÉ\n<!-- [[A]] --><!-- [[A]] -->   ```\nhttps://example.invalid/`[[A]]` 章節\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, Targets: []string{"A", "A", "image.png"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementUsedFootnoteDiagnosticVariant{{Target: "B", Body: "## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[q]]\n<!--\n[[A]]\n-->~~~~\n> > [[image.png]]> [!note] title\n[[A\nB]]# A\n\\[[A]]ref[^n]\nÉ\n<!-- [[A]] --><!-- [[A]] -->   ```\nhttps://example.invalid/`[[A]]` 章節\n~~~~\n章節\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%- ~~~~\n> [!unknown] title\n", Owners: []agreementUsedFootnoteOwner{{Field: 2, Note: 0, Node: 33, Paragraph: 1, Ordinal: 1, Line: 0, Label: "n"}}, Changes: []agreementMixedRename{{Field: 2, Span: graph.Span{Start: 38, Stop: 39}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 36, Stop: 42}, {Start: 183, Stop: 188}, {Start: 202, Stop: 272}}, Comments: []graph.Span{{Start: 42, Stop: 56}, {Start: 123, Stop: 137}, {Start: 137, Stop: 151}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 14}, Word: "[[A|alias]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 36, Stop: 41}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 47, Stop: 52}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 65, Stop: 78}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 94, Stop: 101}, Word: "[[A\nB]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 106, Stop: 111}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 128, Stop: 133}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 142, Stop: 147}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 183, Stop: 188}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 231, Stop: 236}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"A", "A", "image.png"}}, Targets: []string{"A", "A", "image.png"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}}}},
	}
}
func agreementUsedFootnoteDiagnosticWitnesses() []agreementUsedFootnoteDiagnosticWitness {
	return []agreementUsedFootnoteDiagnosticWitness{
		{Name: "b69109464c1a", Body: "![[A#A]]É\n[^n]: [[A]]\n\n    [[B]]\n    ```\n`## <em>A</em>\n\t```\ntext <!--# A\n ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\nref[^n]\n![[A#A]]![[A#A]]-->[[A\\|alias]]", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}, Indices: []int{2}, Singles: []agreementUsedFootnoteDiagnosticSingle{{Field: 2, Index: 2, Start: 30, Body: "![[A#A]]É\n[^n]: [[A]]\n\n    [[q]]\n    ```\n`## <em>A</em>\n\t```\ntext <!--# A\n ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\nref[^n]\n![[A#A]]![[A#A]]-->[[A\\|alias]]", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}}}},
		{Name: "399c86bb410f", Body: "## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->~~~~\n> > [[image.png]]> [!note] title\n[[A\nB]]# A\n\\[[A]]ref[^n]\nÉ\n<!-- [[A]] --><!-- [[A]] -->   ```\nhttps://example.invalid/`[[A]]` 章節\n~~~~\n章節\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%- ~~~~\n> [!unknown] title\n", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "risky-fence", Target: "", Message: "wikilink/callout/table syntax found inside a fenced code block; left untouched", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 21 of the note body hides everything after it", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "risky-fence", Target: "", Message: "wikilink/callout/table syntax found inside a fenced code block; left untouched", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 21 of the note body hides everything after it", Section: "", Block: ""}}, Indices: []int{2}, Singles: []agreementUsedFootnoteDiagnosticSingle{{Field: 2, Index: 2, Start: 38, Body: "## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[q]]\n<!--\n[[A]]\n-->~~~~\n> > [[image.png]]> [!note] title\n[[A\nB]]# A\n\\[[A]]ref[^n]\nÉ\n<!-- [[A]] --><!-- [[A]] -->   ```\nhttps://example.invalid/`[[A]]` 章節\n~~~~\n章節\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%- ~~~~\n> [!unknown] title\n", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "risky-fence", Target: "", Message: "wikilink/callout/table syntax found inside a fenced code block; left untouched", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "comment-unclosed", Target: "%%", Message: "an unclosed %% comment opened at line 21 of the note body hides everything after it", Section: "", Block: ""}}}}},
	}
}
func TestAgreementUsedFootnoteDiagnosticOwnedSource(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnoteDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementUsedFootnoteDiagnosticSource(r.Body, r.Target)
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

func TestAgreementUsedFootnoteDiagnosticOwnedPublic(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnoteDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementUsedFootnoteDiagnosticSource(r.Body, r.Target)
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
				k, a, b := agreementUsedFootnoteDiagnosticDifference(c, &f, raw.HTML, &page, raw.Diagnostics, failures)
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

func TestAgreementUsedFootnoteDiagnosticOwnedRefusals(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnoteDiagnosticWitnesses()
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
					k, a, b := agreementUsedFootnoteDiagnosticDifference(c, &changed, raw.HTML, &page, raw.Diagnostics, failures)
					if k != "" || a != "" || b != "" {
						t.Fatal("caught: discarded diagnostic borrowed another signed face")
					}
				})
			}
			for _, other := range []agreementCase{{Body: r.Body + "\n"}, {Body: r.Body, Title: "A"}, {Body: r.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
				k, a, b := agreementUsedFootnoteDiagnosticDifference(other, &selected, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed another body or context")
				}
			}
			for i := range failures {
				missing := slices.Delete(slices.Clone(failures), i, i+1)
				k, a, b := agreementUsedFootnoteDiagnosticDifference(c, &selected, raw.HTML, &page, raw.Diagnostics, missing)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed an incomplete whole signed pool")
				}
			}
			for i := range raw.Diagnostics {
				missing := slices.Delete(slices.Clone(raw.Diagnostics), i, i+1)
				k, a, b := agreementUsedFootnoteDiagnosticDifference(c, &selected, raw.HTML, &page, missing, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed an incomplete whole raw ledger")
				}
				altered := slices.Clone(raw.Diagnostics)
				altered[i].Message += "outside"
				k, a, b = agreementUsedFootnoteDiagnosticDifference(c, &selected, raw.HTML, &page, altered, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: discarded diagnostic borrowed another complete raw ledger")
				}
			}
			changed := page
			changed.Headings = append(slices.Clone(page.Headings), "outside")
			k, a, b := agreementUsedFootnoteDiagnosticDifference(c, &selected, raw.HTML, &changed, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: discarded diagnostic borrowed another complete public ledger")
			}
			k, a, b = agreementUsedFootnoteDiagnosticDifference(c, &selected, raw.HTML+"<code>outside</code>", &page, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: discarded diagnostic borrowed changed field-free code text")
			}
		})
	}
}

func TestAgreementUsedFootnoteDiagnosticOwnedWholeTargets(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnoteDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementUsedFootnoteDiagnosticSource(r.Body, r.Target)
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

func TestAgreementUsedFootnoteDiagnosticOwnedWholeSet(t *testing.T) {
	t.Parallel()
	records := agreementUsedFootnoteDiagnosticWitnesses()
	witnesses := agreementUsedFootnoteDiagnosticSources()
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
			owners, ok := agreementUsedFootnoteOwners(&check, notes)
			private, changes, valid := agreementUsedFootnoteDiagnosticPrivate(w.Body, &check, notes)
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
					if (x.Property == "P0" || x.Property == "P1") && (x.Tuple.Target == target || x.Tuple.Target == changes[0].New) {
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
	if !maps.Equal(expected, actual) || len(actual) != 2 || fields != 2 || len(witnesses) != 2 {
		t.Fatal("caught: whole declared-body discarded diagnostic eligibility census")
	}
}
