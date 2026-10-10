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

type agreementNativeCodePlainDiagnosticSingle struct {
	Field, Index, Start int
	Body                string
	Diagnostics         []render.Diagnostic
}
type agreementNativeCodePlainDiagnosticWitness struct {
	Name, Body, Target, Selected    string
	Diagnostics, PrivateDiagnostics []render.Diagnostic
	Indices                         []int
	Singles                         []agreementNativeCodePlainDiagnosticSingle
}

type agreementNativeCodePlainDiagnosticOwner = agreementNativeCodeDiagnosticOwner

// Each producer changes one diagnostic while the complete rendered page stays fixed.
func agreementNativeCodePlainDiagnosticProducers(t *testing.T, body, target string, native agreementNativeOwnerSource, check *agreementNativeCheckSource) (physicalFields, diagnosticFields []agreementNativeCodePlainDiagnosticOwner, targetChanges []agreementMixedRename, projectedBody string, valid bool) {
	t.Helper()
	physical, ok := agreementNativeCodeDiagnosticPhysical(native, check, target)
	if !ok {
		return nil, nil, nil, "", false
	}
	raw, page := agreementIsolatedPage(t, agreementCase{Body: body})
	tree := agreementNativeProseAddressTreeReading(body, nil)
	parts, pok := agreementCodeWindowParts(raw.HTML)
	if !pok {
		return nil, nil, nil, "", false
	}
	var owners []agreementNativeCodePlainDiagnosticOwner
	var changes []agreementMixedRename
	whole := []byte(body)
	used := map[int]bool{}
	for _, owner := range physical {
		private, oneChanges, valid := agreementNativeCodeDiagnosticPrivate(body, target, []agreementNativeCodePlainDiagnosticOwner{owner}, check)
		if !valid {
			continue
		}
		pr, pp := agreementIsolatedPage(t, agreementCase{Body: private})
		privateParts, validParts := agreementCodeWindowParts(pr.HTML)
		if !validParts || !cmp.Equal(native, agreementNativeOwnerReading(private)) || !cmp.Equal(tree, agreementNativeProseAddressTreeReading(private, nil)) || raw.HTML != pr.HTML || !cmp.Equal(page, pp) || !cmp.Equal(parts, privateParts) || len(raw.Diagnostics) != len(pr.Diagnostics) {
			continue
		}
		index := -1
		unique := true
		for di := range raw.Diagnostics {
			if cmp.Equal(raw.Diagnostics[di], pr.Diagnostics[di]) {
				continue
			}
			if index >= 0 {
				unique = false
				break
			}
			index = di
		}
		if !unique || index < 0 || used[index] {
			continue
		}
		a, b := raw.Diagnostics[index], pr.Diagnostics[index]
		change := oneChanges[0]
		if a.Kind != render.DiagWikilinkBroken || a.Section != "" || a.Target != target || b.Target != change.New || a.Kind != b.Kind || a.Section != b.Section || a.Block != b.Block || b.Message != strings.ReplaceAll(a.Message, "\""+target+"\"", "\""+change.New+"\"") {
			continue
		}
		used[index] = true
		owners = append(owners, owner)
		changes = append(changes, change)
		copy(whole[change.Span.Start:change.Span.Stop], change.New)
	}
	return physical, owners, changes, string(whole), len(owners) > 0
}

func agreementNativeCodePlainDiagnosticSource(body, target string) (*agreementNativeCodePlainDiagnosticSourceWitness, *agreementNativeCodePlainDiagnosticVariant, bool) {
	witnesses := agreementNativeCodePlainDiagnosticSources()
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

// Every changed diagnostic belongs to one declared native code field.
func agreementNativeCodePlainDiagnosticDifference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	if f == nil || page == nil || c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Multiplicity < 1 || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Tuple.Section != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	records := agreementNativeCodePlainDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		if c.Body != r.Body || f.Tuple.Target != r.Target {
			continue
		}
		if agreementSignature(f) != r.Selected || !cmp.Equal(diagnostics, r.Diagnostics) {
			return "", "", ""
		}
		w, v, found := agreementNativeCodePlainDiagnosticSource(c.Body, r.Target)
		if !found {
			return "", "", ""
		}
		native := agreementNativeOwnerReading(c.Body)
		check := agreementNativeCheckReading(c.Body, native)
		tree := agreementNativeProseAddressTreeReading(c.Body, nil)
		parts, pok := agreementCodeWindowParts(raw)
		physical, ok := agreementNativeCodeDiagnosticPhysical(native, &check, r.Target)
		private, changes, valid := agreementNativeCodeDiagnosticPrivate(c.Body, r.Target, v.Owners, &check)
		if !pok || !ok || !valid || len(v.Owners) != f.Multiplicity || len(r.Indices) != len(v.Owners) || !cmp.Equal(physical, v.Physical) || !cmp.Equal(changes, v.Changes) || private != v.Body || !cmp.Equal(native, w.Native) || !cmp.Equal(check, w.Check) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(agreementNativeCodePayloadSites(c.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), w.HTML) || !cmp.Equal(judge.LinkTargets(c.Body), w.Targets) || !cmp.Equal(parts, w.Parts) || !cmp.Equal(*page, w.Page) || !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
			return "", "", ""
		}
		return "debt", "#1011 stage 5", "page-diagnostic"
	}
	return "", "", ""
}

type agreementNativeCodePlainDiagnosticVariant struct {
	Target, Body     string
	Physical, Owners []agreementNativeCodePlainDiagnosticOwner
	Changes          []agreementMixedRename
	Check            agreementNativeCheckSource
	Targets          []string
	Signatures       map[string]int
}
type agreementNativeCodePlainDiagnosticSourceWitness struct {
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
	Variants   []agreementNativeCodePlainDiagnosticVariant
}

func agreementNativeCodePlainDiagnosticSources() []agreementNativeCodePlainDiagnosticSourceWitness {
	return []agreementNativeCodePlainDiagnosticSourceWitness{{Name: "8e2cc3b85c06", Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[A]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[image.png]]<!--\\\\[[A]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 12, Stop: 28}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 34, Stop: 86}}, {Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 96, Stop: 225}}}, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 12, Stop: 28}, {Start: 34, Stop: 86}, {Start: 96, Stop: 225}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 35, Stop: 48}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 53, Stop: 59}, Word: "[[#A]]", Targets: []string{}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 79, Stop: 84}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 124, Stop: 129}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 130, Stop: 137}, Word: "[[A\nB]]", Targets: []string{}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 139, Stop: 144}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 152, Stop: 157}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 157, Stop: 170}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 176, Stop: 181}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 185, Stop: 192}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 199, Stop: 206}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 207, Stop: 220}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}}, Targets: []string{"A", "A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 7, Parent: 1, Kind: "CodeSpan", ParentKind: "Paragraph", Lines: nil, Text: []text.Segment{{Start: 12, Stop: 17, Padding: 0, ForceNewline: false}, {Start: 17, Stop: 23, Padding: 0, ForceNewline: false}, {Start: 23, Stop: 28, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}, {Root: 0, Node: 12, Parent: 1, Kind: "CodeSpan", ParentKind: "Paragraph", Lines: nil, Text: []text.Segment{{Start: 34, Stop: 34, Padding: 0, ForceNewline: false}, {Start: 34, Stop: 53, Padding: 0, ForceNewline: false}, {Start: 53, Stop: 77, Padding: 0, ForceNewline: false}, {Start: 77, Stop: 86, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}, {Root: 0, Node: 17, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 96, Stop: 103, Padding: 0, ForceNewline: true}, {Start: 103, Stop: 108, Padding: 0, ForceNewline: true}, {Start: 108, Stop: 113, Padding: 0, ForceNewline: true}, {Start: 113, Stop: 130, Padding: 0, ForceNewline: true}, {Start: 130, Stop: 134, Padding: 0, ForceNewline: true}, {Start: 134, Stop: 151, Padding: 0, ForceNewline: true}, {Start: 151, Stop: 197, Padding: 0, ForceNewline: true}, {Start: 197, Stop: 225, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 17, Parent: 0, Kind: "FencedCodeBlock", ParentKind: "Document", Type: 0, Lines: []text.Segment{{Start: 96, Stop: 103, Padding: 0, ForceNewline: true}, {Start: 103, Stop: 108, Padding: 0, ForceNewline: true}, {Start: 108, Stop: 113, Padding: 0, ForceNewline: true}, {Start: 113, Stop: 130, Padding: 0, ForceNewline: true}, {Start: 130, Stop: 134, Padding: 0, ForceNewline: true}, {Start: 134, Stop: 151, Padding: 0, ForceNewline: true}, {Start: 151, Stop: 197, Padding: 0, ForceNewline: true}, {Start: 197, Stop: 225, Padding: 0, ForceNewline: true}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: true}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 0, Stop: 17}, {Start: 17, Stop: 23}, {Start: 23, Stop: 34}, {Start: 34, Stop: 53}, {Start: 53, Stop: 77}, {Start: 77, Stop: 90}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 0, Stop: 1}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 1, Stop: 2}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 2, Stop: 7}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 7, Stop: 8}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 8, Stop: 11}}, Raw: false, Soft: false, Hard: false}, {Kind: "CodeSpan", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 12, Stop: 17}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 23}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 28}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 30}}, Raw: false, Soft: false, Hard: false}, {Kind: "CodeSpan", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 34, Stop: 34}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 34, Stop: 53}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 53, Stop: 77}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 77, Stop: 86}}, Raw: true, Soft: false, Hard: false}, {Kind: "FencedCodeBlock", Depth: 1, Lines: []graph.Span{{Start: 96, Stop: 103}, {Start: 103, Stop: 108}, {Start: 108, Stop: 113}, {Start: 113, Stop: 130}, {Start: 130, Stop: 134}, {Start: 134, Stop: 151}, {Start: 151, Stop: 197}, {Start: 197, Stop: 225}}, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 12, Stop: 28}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 34, Stop: 86}}, {Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 96, Stop: 225}}}, Comments: nil, Unanchorable: nil, Valid: true}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "A", State: "wikilink-broken"}, {SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}}, CitationsInCode: 1, Blocks: nil, Headings: []string{"a-2", "a", "a-3"}, Failures: nil, CodeCitations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CalloutTitles: nil}, Parts: [][]agreementCodePart{{{Text: "open ", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}, {Text: "A", Citation: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Field: true}, {Text: " close", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}, nil}, Targets: []string{"A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=page-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=page-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P2/wikilink-in-code tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=page-in-code multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementNativeCodePlainDiagnosticVariant{{Target: "A", Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[q]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[image.png]]<!--\\\\[[q]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Physical: []agreementNativeCodePlainDiagnosticOwner{{Field: 2, Code: 0}, {Field: 5, Code: 1}, {Field: 6, Code: 2}, {Field: 8, Code: 2}, {Field: 11, Code: 2}, {Field: 12, Code: 2}, {Field: 13, Code: 2}}, Owners: []agreementNativeCodePlainDiagnosticOwner{{Field: 6, Code: 2}, {Field: 11, Code: 2}}, Changes: []agreementMixedRename{{Field: 6, Span: graph.Span{Start: 126, Stop: 127}, Old: "A", New: "q"}, {Field: 11, Span: graph.Span{Start: 178, Stop: 179}, Old: "A", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 12, Stop: 28}, {Start: 34, Stop: 86}, {Start: 96, Stop: 225}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 35, Stop: 48}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 53, Stop: 59}, Word: "[[#A]]", Targets: []string{}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 79, Stop: 84}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 124, Stop: 129}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 130, Stop: 137}, Word: "[[A\nB]]", Targets: []string{}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 139, Stop: 144}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 152, Stop: 157}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 157, Stop: 170}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 176, Stop: 181}, Word: "[[q]]", Targets: []string{"q"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 185, Stop: 192}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 199, Stop: 206}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 207, Stop: 220}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}}, Targets: []string{"A", "A"}}, Targets: []string{"A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=page-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=page-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P2/wikilink-in-code tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=page-in-code multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}}, {Target: "image.png", Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[A]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[qqqqqqqqq]]<!--\\\\[[A]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Physical: []agreementNativeCodePlainDiagnosticOwner{{Field: 3, Code: 1}, {Field: 10, Code: 2}, {Field: 14, Code: 2}}, Owners: []agreementNativeCodePlainDiagnosticOwner{{Field: 10, Code: 2}}, Changes: []agreementMixedRename{{Field: 10, Span: graph.Span{Start: 159, Stop: 168}, Old: "image.png", New: "qqqqqqqqq"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 12, Stop: 28}, {Start: 34, Stop: 86}, {Start: 96, Stop: 225}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}, {Span: graph.Span{Start: 35, Stop: 48}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 53, Stop: 59}, Word: "[[#A]]", Targets: []string{}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 79, Stop: 84}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 124, Stop: 129}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 130, Stop: 137}, Word: "[[A\nB]]", Targets: []string{}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 139, Stop: 144}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 152, Stop: 157}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 157, Stop: 170}, Word: "[[qqqqqqqqq]]", Targets: []string{"qqqqqqqqq"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 176, Stop: 181}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 185, Stop: 192}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 199, Stop: 206}, Word: "[[A#A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}, {Span: graph.Span{Start: 207, Stop: 220}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 2}}, Targets: []string{"A", "A"}}, Targets: []string{"A", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:qqqqqqqqq Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=page-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=page-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P2/wikilink-in-code tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=page-in-code multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"a-3\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}}}}}
}
func agreementNativeCodePlainDiagnosticWitnesses() []agreementNativeCodePlainDiagnosticWitness {
	return []agreementNativeCodePlainDiagnosticWitness{{Name: "8e2cc3b85c06", Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[A]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[image.png]]<!--\\\\[[A]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Target: "A", Selected: "P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}, Indices: []int{4, 6}, Singles: []agreementNativeCodePlainDiagnosticSingle{{Field: 6, Index: 4, Start: 126, Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[q]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[image.png]]<!--\\\\[[A]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}}, {Field: 11, Index: 6, Start: 178, Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[A]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[image.png]]<!--\\\\[[q]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}}}}, {Name: "8e2cc3b85c06", Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[A]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[image.png]]<!--\\\\[[A]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Target: "image.png", Selected: "P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "qqqqqqqqq", Message: "wikilink \"qqqqqqqqq\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}, Indices: []int{5}, Singles: []agreementNativeCodePlainDiagnosticSingle{{Field: 10, Index: 5, Start: 159, Body: "[[A]] [[A]]`open\n[[A]]\nclose` ```\n![[image.png]]~~~~\n[[#A]]^absent-prose ~~~\n%%[[A]]%% ```\n````\n## A-2\n## A\n## A\n[^unused]: [[A]]\n[[A\nB]]%%[[A]]%%````\n\\[[A]][[qqqqqqqqq]]<!--\\\\[[A]]-->![[A#A]]````\n> [[A#A]]\t[[image.png]]## !\n", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "qqqqqqqqq", Message: "wikilink \"qqqqqqqqq\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "image.png", Message: "wikilink \"image.png\" does not resolve to any note or file", Section: "", Block: ""}}}}}}
}
func TestAgreementNativeCodePlainDiagnosticOwnedSource(t *testing.T) {
	t.Parallel()
	records := agreementNativeCodePlainDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementNativeCodePlainDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			if len(r.Singles) != len(v.Owners) || len(r.Indices) != len(v.Owners) {
				t.Fatal("caught: complete native code diagnostic owner set")
			}
			changed := make(map[int]bool)
			fields := make(map[int]bool)
			expected := slices.Clone(r.Diagnostics)
			for si := range r.Singles {
				single := &r.Singles[si]
				owner := v.Owners[si]
				change := v.Changes[si]
				if single.Field != owner.Field || single.Start != change.Span.Start || single.Index != r.Indices[si] || single.Index < 0 || single.Index >= len(expected) || changed[single.Index] || fields[single.Field] {
					t.Fatal("caught: unique complete native code field-to-diagnostic map")
				}
				changed[single.Index] = true
				fields[single.Field] = true
				body := []byte(r.Body)
				copy(body[change.Span.Start:change.Span.Stop], change.New)
				if string(body) != single.Body {
					t.Fatal("caught: exact single native code diagnostic source projection")
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
					t.Fatal("caught: complete unchanged single native code diagnostic source facts")
				}
				if len(single.Diagnostics) != len(r.Diagnostics) {
					t.Fatal("caught: complete ordered single native code diagnostic ledger")
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
				t.Fatal("caught: whole ordered native code diagnostic substitution")
			}
			for _, owner := range v.Owners {
				if !fields[owner.Field] {
					t.Fatal("caught: every declared native code source field is represented")
				}
			}
		})
	}
}

func TestAgreementNativeCodePlainDiagnosticOwnedPublic(t *testing.T) {
	t.Parallel()
	records := agreementNativeCodePlainDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementNativeCodePlainDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
			if !cmp.Equal(raw.Diagnostics, r.Diagnostics) || !cmp.Equal(page, w.Page) {
				t.Fatal("caught: complete original native code diagnostic and public ledgers")
			}
			private, pp := agreementIsolatedPage(t, agreementCase{Body: v.Body})
			if raw.HTML != private.HTML || !cmp.Equal(private.Diagnostics, r.PrivateDiagnostics) || !cmp.Equal(pp, w.Page) {
				t.Fatal("caught: whole native code diagnostic projection leaves complete HTML unchanged")
			}
			for si := range r.Singles {
				single := &r.Singles[si]
				projected, p := agreementIsolatedPage(t, agreementCase{Body: single.Body})
				if projected.HTML != raw.HTML || !cmp.Equal(projected.Diagnostics, single.Diagnostics) || !cmp.Equal(p, w.Page) {
					t.Fatal("caught: complete independent single-field native code diagnostic observation")
				}
				parts, valid := agreementCodeWindowParts(projected.HTML)
				if !valid || !cmp.Equal(parts, w.Parts) {
					t.Fatal("caught: every single-field native code diagnostic code ledger stays identical")
				}
			}
			c := agreementCase{Body: r.Body}
			failures := agreementPageFailures(r.Body, &raw, &page)
			failures = append(failures, agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{page})[0]...)
			if !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
				t.Fatal("caught: complete original native code diagnostic signed pool")
			}
			privateFailures := agreementPageFailures(v.Body, &private, &pp)
			privateFailures = append(privateFailures, agreementFragmentFailures(t, []agreementCase{{Body: v.Body}}, []agreementHTML{pp})[0]...)
			if !maps.Equal(agreementMixedCurrent(privateFailures), v.Signatures) {
				t.Fatal("caught: complete private native code diagnostic signed pool")
			}
			seen := 0
			for _, f := range failures {
				if agreementSignature(&f) != r.Selected {
					continue
				}
				seen++
				k, a, b := agreementNativeCodePlainDiagnosticDifference(c, &f, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "debt" || a != "#1011 stage 5" || b != "page-diagnostic" {
					t.Fatal("caught: exact native code diagnostic P0 debt owner")
				}
			}
			if seen != 1 {
				t.Fatal("diagnostic selected receipt setup")
			}
		})
	}
}

func TestAgreementNativeCodePlainDiagnosticOwnedRefusals(t *testing.T) {
	t.Parallel()
	records := agreementNativeCodePlainDiagnosticWitnesses()
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
					k, a, b := agreementNativeCodePlainDiagnosticDifference(c, &changed, raw.HTML, &page, raw.Diagnostics, failures)
					if k != "" || a != "" || b != "" {
						t.Fatal("caught: native code diagnostic borrowed another signed face")
					}
				})
			}
			for _, other := range []agreementCase{{Body: r.Body + "\n"}, {Body: r.Body, Title: "A"}, {Body: r.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
				k, a, b := agreementNativeCodePlainDiagnosticDifference(other, &selected, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: native code diagnostic borrowed another body or context")
				}
			}
			for i := range failures {
				missing := slices.Delete(slices.Clone(failures), i, i+1)
				k, a, b := agreementNativeCodePlainDiagnosticDifference(c, &selected, raw.HTML, &page, raw.Diagnostics, missing)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: native code diagnostic borrowed an incomplete whole signed pool")
				}
			}
			for i := range raw.Diagnostics {
				missing := slices.Delete(slices.Clone(raw.Diagnostics), i, i+1)
				k, a, b := agreementNativeCodePlainDiagnosticDifference(c, &selected, raw.HTML, &page, missing, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: native code diagnostic borrowed an incomplete whole raw ledger")
				}
				altered := slices.Clone(raw.Diagnostics)
				altered[i].Message += "outside"
				k, a, b = agreementNativeCodePlainDiagnosticDifference(c, &selected, raw.HTML, &page, altered, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: native code diagnostic borrowed another complete raw ledger")
				}
			}
			changed := page
			changed.Headings = append(slices.Clone(page.Headings), "outside")
			k, a, b := agreementNativeCodePlainDiagnosticDifference(c, &selected, raw.HTML, &changed, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: native code diagnostic borrowed another complete public ledger")
			}
			k, a, b = agreementNativeCodePlainDiagnosticDifference(c, &selected, raw.HTML+"<code>outside</code>", &page, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: native code diagnostic borrowed changed field-free code text")
			}
		})
	}
}

func TestAgreementNativeCodePlainDiagnosticOwnedWholeTargets(t *testing.T) {
	t.Parallel()
	records := agreementNativeCodePlainDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementNativeCodePlainDiagnosticSource(r.Body, r.Target)
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
			if !cmp.Equal(expected, v.Check) || !cmp.Equal(agreementNativeCheckReading(v.Body, native), expected) || !cmp.Equal(judge.LinkTargets(r.Body), w.Targets) || !cmp.Equal(judge.LinkTargets(v.Body), v.Targets) || !slices.Equal(expected.Targets, v.Targets) {
				t.Fatal("caught: complete whole native code diagnostic physical targets")
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
				t.Fatal("caught: native code diagnostic target counts conserve every physical owner")
			}
			delete(before, r.Target)
			delete(before, replacement)
			delete(after, r.Target)
			delete(after, replacement)
			if !maps.Equal(before, after) {
				t.Fatal("caught: every unselected whole native code diagnostic target count")
			}
		})
	}
}

func TestAgreementNativeCodePlainDiagnosticOwnedWholeSet(t *testing.T) {
	t.Parallel()
	records := agreementNativeCodePlainDiagnosticWitnesses()
	witnesses := agreementNativeCodePlainDiagnosticSources()
	expected := make(map[[3]string]int)
	for _, r := range records {
		key := [3]string{r.Body, r.Target, r.Selected}
		if _, exists := expected[key]; exists || len(r.Singles) != len(r.Indices) {
			t.Fatal("caught: unique complete native code diagnostic declared roster")
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
			physical, owners, changes, private, valid := agreementNativeCodePlainDiagnosticProducers(t, w.Body, target, native, &check)
			ok := len(physical) > 0
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
				t.Fatal("caught: every eligible native code diagnostic roster member once")
			}
			actual[key] = len(owners)
			bodySelected[agreementSignature(&f)]++
			bodyVariants[[2]string{target, agreementSignature(&f)}] = len(owners)
			_, declared, found := agreementNativeCodePlainDiagnosticSource(w.Body, target)
			if !found || !cmp.Equal(physical, declared.Physical) || !cmp.Equal(owners, declared.Owners) || !cmp.Equal(changes, declared.Changes) || private != declared.Body {
				t.Fatal("caught: complete native code physical and producer field census")
			}
		}
		declaredVariants := make(map[[2]string]int)
		for _, v := range w.Variants {
			f := agreementFailure{Property: "P0", Identity: "diagnostic-html", Tuple: agreementCitation{Target: v.Target, State: "wikilink-broken"}, Direction: "diagnostic-only", Multiplicity: len(v.Owners)}
			key := [2]string{v.Target, agreementSignature(&f)}
			if _, exists := declaredVariants[key]; exists {
				t.Fatal("caught: unique complete native code diagnostic source variants")
			}
			declaredVariants[key] = len(v.Owners)
		}
		if !maps.Equal(bodySelected, w.Selected) || !maps.Equal(bodyVariants, declaredVariants) {
			t.Fatal("caught: whole eligible native code diagnostic source declaration set")
		}
	}
	fields := 0
	for _, n := range actual {
		fields += n
	}
	if !maps.Equal(expected, actual) || len(actual) != 2 || fields != 3 || len(witnesses) != 1 {
		t.Fatal("caught: whole declared-body native code diagnostic eligibility census")
	}
}
