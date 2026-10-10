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

type agreementRemovedAliasDiagnosticSingle struct {
	Field, Index, Start int
	Body                string
	Diagnostics         []render.Diagnostic
}
type agreementRemovedAliasDiagnosticWitness struct {
	Name, Body, Target, Selected    string
	Diagnostics, PrivateDiagnostics []render.Diagnostic
	Indices                         []int
	Singles                         []agreementRemovedAliasDiagnosticSingle
}

type agreementRemovedAliasDiagnosticReading struct {
	Native          agreementNativeOwnerSource
	Check           agreementNativeCheckSource
	Projected       string
	Positions       []int
	ProjectedNative agreementNativeOwnerSource
	ProjectedCheck  agreementNativeCheckSource
	ProjectedTree   agreementNativeProseAddressTree
	Valid           bool
}
type agreementRemovedAliasDiagnosticOwner struct {
	Field, Definition, Root, Node, Line int
	Label                               string
	Projected                           graph.Span
}

// Retained bytes keep their original positions when declared comment bytes disappear.
func agreementRemovedAliasDiagnosticRead(body string) agreementRemovedAliasDiagnosticReading {
	r := agreementRemovedAliasDiagnosticReading{Native: agreementNativeOwnerReading(body)}
	r.Check = agreementNativeCheckReading(body, r.Native)
	if len(r.Native.Comments) == 0 || !cmp.Equal(r.Native.Comments, r.Check.Comments) {
		return r
	}
	end := 0
	for _, span := range r.Native.Comments {
		if span.Start < end || span.Start < 0 || span.Stop > len(body) || span.Stop <= span.Start {
			return r
		}
		end = span.Stop
	}
	var projected []byte
	for at := range len(body) {
		hidden := false
		for _, span := range r.Native.Comments {
			if at >= span.Start && at < span.Stop {
				hidden = true
			}
		}
		if hidden && body[at] != '\n' && body[at] != '\r' {
			continue
		}
		projected = append(projected, body[at])
		r.Positions = append(r.Positions, at)
	}
	r.Projected = string(projected)
	r.ProjectedNative = agreementNativeOwnerReading(r.Projected)
	r.ProjectedCheck = agreementNativeCheckReading(r.Projected, r.ProjectedNative)
	r.ProjectedTree = agreementNativeProseAddressTreeReading(r.Projected, nil)
	r.Valid = r.ProjectedTree.Valid
	return r
}

// Each outside alias field owns one complete discarded paragraph after comment removal.
func agreementRemovedAliasDiagnosticPhysical(body string, check *agreementNativeCheckSource, target string) ([]agreementRemovedAliasDiagnosticOwner, bool) {
	r := agreementRemovedAliasDiagnosticRead(body)
	if !r.Valid || target == "" || !cmp.Equal(*check, r.Check) {
		return nil, false
	}
	reverse := map[int]int{}
	for i, at := range r.Positions {
		if _, duplicate := reverse[at]; duplicate {
			return nil, false
		}
		reverse[at] = i
	}
	var owners []agreementRemovedAliasDiagnosticOwner
	used := map[graph.Span]bool{}
	for fi, f := range check.Fields {
		if f.Code || f.Comment || f.Escaped || f.DefinitionOwner >= 0 || f.CodeOwner >= 0 || len(f.Targets) != 1 || f.Targets[0] != target || !strings.HasPrefix(f.Word, "[["+target+"|") {
			continue
		}
		if f.Span.Start < 0 || f.Span.Stop > len(body) || f.Span.Stop <= f.Span.Start || body[f.Span.Start:f.Span.Stop] != f.Word {
			return nil, false
		}
		start, sok := reverse[f.Span.Start]
		last, eok := reverse[f.Span.Stop-1]
		stop := last + 1
		if !sok || !eok || stop <= start || stop > len(r.Projected) || r.Projected[start:stop] != f.Word {
			return nil, false
		}
		found := false
		candidate := agreementRemovedAliasDiagnosticOwner{}
		for di, d := range r.ProjectedTree.Definitions {
			if !d.Found || d.Root < 1 || d.Root >= len(r.ProjectedTree.Trees) || d.Node != 0 || d.Label == "" || d.Bounds.Parent == "" || d.Bounds.Span.Start < 0 || d.Bounds.Span.Stop > len(r.Projected) || d.Bounds.Span.Stop <= d.Bounds.Span.Start || d.Bounds.Label.Start < d.Bounds.Span.Start || d.Bounds.Label.Stop > d.Bounds.Span.Stop || d.Bounds.Label.Stop <= d.Bounds.Label.Start || start < d.Bounds.Span.Start || stop > d.Bounds.Span.Stop {
				continue
			}
			nodes := r.ProjectedTree.Trees[d.Root]
			if len(nodes) == 0 || nodes[0].Kind != extast.KindFootnote.String() {
				continue
			}
			for ni, n := range nodes {
				if n.Kind != ast.KindParagraph.String() {
					continue
				}
				for li, line := range n.Lines {
					if line.Start < 0 || line.Stop > len(r.Projected) || line.Stop <= line.Start || start < line.Start || stop > line.Stop {
						continue
					}
					if found {
						return nil, false
					}
					found = true
					candidate = agreementRemovedAliasDiagnosticOwner{Field: fi, Definition: di, Root: d.Root, Node: ni, Line: li, Label: d.Label, Projected: graph.Span{Start: start, Stop: stop}}
				}
			}
		}
		if found {
			if used[f.Span] {
				return nil, false
			}
			used[f.Span] = true
			owners = append(owners, candidate)
		}
	}
	return owners, len(owners) > 0
}

// Only targets in the independently declared physical set receive private names.
func agreementRemovedAliasDiagnosticPrivate(body, target string, owners []agreementRemovedAliasDiagnosticOwner, check *agreementNativeCheckSource) (string, []agreementMixedRename, bool) {
	if target == "" || len(owners) == 0 || strings.Contains(body, "q") {
		return "", nil, false
	}
	physical, ok := agreementRemovedAliasDiagnosticPhysical(body, check, target)
	if !ok {
		return "", nil, false
	}
	private := []byte(body)
	var changes []agreementMixedRename
	used := map[int]bool{}
	for _, owner := range owners {
		if owner.Field < 0 || owner.Field >= len(check.Fields) || used[owner.Field] || !slices.Contains(physical, owner) {
			return "", nil, false
		}
		used[owner.Field] = true
		f := check.Fields[owner.Field]
		start, stop := f.Span.Start+2, f.Span.Start+2+len(target)
		if body[start:stop] != target {
			return "", nil, false
		}
		replacement := strings.Repeat("q", len(target))
		copy(private[start:stop], replacement)
		changes = append(changes, agreementMixedRename{Field: owner.Field, Span: graph.Span{Start: start, Stop: stop}, Old: target, New: replacement})
	}
	return string(private), changes, true
}

func agreementRemovedAliasDiagnosticProducers(t *testing.T, body, target string, native agreementNativeOwnerSource, check *agreementNativeCheckSource) (physicalFields, diagnosticFields []agreementRemovedAliasDiagnosticOwner, targetChanges []agreementMixedRename, projectedBody string, valid bool) {
	t.Helper()
	tree := agreementNativeProseAddressTreeReading(body, nil)
	physical, ok := agreementRemovedAliasDiagnosticPhysical(body, check, target)
	if !ok {
		return nil, nil, nil, "", false
	}
	raw, page := agreementIsolatedPage(t, agreementCase{Body: body})
	parts, pok := agreementCodeWindowParts(raw.HTML)
	if !pok {
		return nil, nil, nil, "", false
	}
	var owners []agreementRemovedAliasDiagnosticOwner
	var changes []agreementMixedRename
	whole := []byte(body)
	used := map[int]bool{}
	for _, owner := range physical {
		private, oneChanges, valid := agreementRemovedAliasDiagnosticPrivate(body, target, []agreementRemovedAliasDiagnosticOwner{owner}, check)
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
		if a.Kind != render.DiagWikilinkBroken || a.Target != target || b.Target != change.New || a.Kind != b.Kind || a.Section != b.Section || a.Block != b.Block || b.Message != strings.ReplaceAll(a.Message, "\""+target+"\"", "\""+change.New+"\"") {
			continue
		}
		used[index] = true
		owners = append(owners, owner)
		changes = append(changes, change)
		copy(whole[change.Span.Start:change.Span.Stop], change.New)
	}
	return physical, owners, changes, string(whole), len(owners) > 0
}

// A discarded diagnostic moves with its physical target while public faces stay fixed.
func agreementRemovedAliasDiagnosticSignedProjection(before, after []agreementFailure, page *agreementHTML, targets []string, selected *agreementFailure, fields int) bool {
	if selected == nil || fields < 1 || fields != selected.Multiplicity || selected.Tuple.Target == "" {
		return false
	}
	target := selected.Tuple.Target
	replacement := strings.Repeat("q", len(target))
	counts := map[string]int{}
	for _, name := range targets {
		counts[name]++
	}
	visible := map[string]int{}
	for _, citation := range page.Citations {
		if citation.SourceRole == "" && citation.Target != "" {
			visible[citation.Target]++
		}
	}
	if counts[target] < fields || counts[replacement] != 0 || visible[replacement] != 0 {
		return false
	}
	signed := func(name string, delta int) string {
		if delta == 0 {
			return ""
		}
		direction := "judge-only"
		if delta < 0 {
			direction = "page-only"
			delta = -delta
		}
		f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: name}, Direction: direction, Multiplicity: delta}
		return agreementSignature(&f)
	}
	oldP1 := map[string]int{}
	for i := range before {
		f := &before[i]
		if f.Property == "P1" && (f.Tuple.Target == target || f.Tuple.Target == replacement) {
			oldP1[agreementSignature(f)]++
		}
	}
	declared := map[string]int{}
	if signature := signed(target, counts[target]-visible[target]); signature != "" {
		declared[signature] = 1
	}
	if !maps.Equal(oldP1, declared) {
		return false
	}
	expected := maps.Clone(agreementMixedCurrent(before))
	signature := agreementSignature(selected)
	if expected[signature] != 1 {
		return false
	}
	delete(expected, signature)
	renamed := *selected
	renamed.Tuple.Target = replacement
	expected[agreementSignature(&renamed)]++
	for key := range oldP1 {
		delete(expected, key)
	}
	if key := signed(target, counts[target]-fields-visible[target]); key != "" {
		expected[key]++
	}
	expected[signed(replacement, fields)]++
	return maps.Equal(expected, agreementMixedCurrent(after))
}

func agreementRemovedAliasDiagnosticSource(body, target string) (*agreementRemovedAliasDiagnosticSourceWitness, *agreementRemovedAliasDiagnosticVariant, bool) {
	witnesses := agreementRemovedAliasDiagnosticSources()
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

// Every changed diagnostic belongs to one declared comment-removed discarded alias field.
func agreementRemovedAliasDiagnosticDifference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	if f == nil || page == nil || c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Multiplicity < 1 || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Tuple.Section != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	records := agreementRemovedAliasDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		if c.Body != r.Body || f.Tuple.Target != r.Target {
			continue
		}
		if agreementSignature(f) != r.Selected || !cmp.Equal(diagnostics, r.Diagnostics) {
			return "", "", ""
		}
		w, v, found := agreementRemovedAliasDiagnosticSource(c.Body, r.Target)
		if !found {
			return "", "", ""
		}
		native := agreementNativeOwnerReading(c.Body)
		check := agreementNativeCheckReading(c.Body, native)
		tree := agreementNativeProseAddressTreeReading(c.Body, nil)
		parts, pok := agreementCodeWindowParts(raw)
		physical, ok := agreementRemovedAliasDiagnosticPhysical(c.Body, &check, r.Target)
		private, changes, valid := agreementRemovedAliasDiagnosticPrivate(c.Body, r.Target, v.Owners, &check)
		if !pok || !ok || !valid || len(v.Owners) != f.Multiplicity || len(r.Indices) != len(v.Owners) || !cmp.Equal(physical, v.Physical) || !cmp.Equal(changes, v.Changes) || private != v.Body || !cmp.Equal(native, w.Native) || !cmp.Equal(check, w.Check) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(agreementRemovedAliasDiagnosticRead(c.Body), w.Removed) || !cmp.Equal(agreementNativeCodePayloadSites(c.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), w.HTML) || !cmp.Equal(judge.LinkTargets(c.Body), w.Targets) || !cmp.Equal(parts, w.Parts) || !cmp.Equal(*page, w.Page) || !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
			return "", "", ""
		}
		return "debt", "#1011 stage 5", "page-diagnostic"
	}
	return "", "", ""
}

type agreementRemovedAliasDiagnosticVariant struct {
	Target, Body     string
	Physical, Owners []agreementRemovedAliasDiagnosticOwner
	Changes          []agreementMixedRename
	Check            agreementNativeCheckSource
	Targets          []string
	Signatures       map[string]int
	Removed          agreementRemovedAliasDiagnosticReading
}
type agreementRemovedAliasDiagnosticSourceWitness struct {
	Name       string
	Body       string
	Native     agreementNativeOwnerSource
	Check      agreementNativeCheckSource
	Sites      []agreementNativeCodePayloadSite
	HTML       []agreementNativeHTMLAddressSite
	Tree       agreementNativeProseAddressTree
	Removed    agreementRemovedAliasDiagnosticReading
	Page       agreementHTML
	Parts      [][]agreementCodePart
	Targets    []string
	Signatures map[string]int
	Selected   map[string]int
	Variants   []agreementRemovedAliasDiagnosticVariant
}

func agreementRemovedAliasDiagnosticSources() []agreementRemovedAliasDiagnosticSourceWitness {
	return []agreementRemovedAliasDiagnosticSourceWitness{
		{Name: "6815c332b79f", Body: "- ![[image.png]][[A]] [[A]][[#A]]``1. > [!note] one\n> [!note] two\n> [!note] three\n    ```\n[[A]] [[A]]- ```` go [[A]]\nÉ\n> [!note] title\nhttps://example.invalid/`[[A]]` <!--\n[[A]]\n-->[^unused]: [[A]]\n[[B|alias]]## <em>A</em>\n![[A#A]]# A\nref[^n]\n`open\n[[A]]\nclose`- item\n\n      0", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 245, Stop: 261}}, {Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 274, Stop: 283}}}, Comments: []graph.Span{{Start: 168, Stop: 182}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 161, Stop: 166}, {Start: 245, Stop: 261}, {Start: 274, Stop: 283}}, Comments: []graph.Span{{Start: 168, Stop: 182}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 16}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 27, Stop: 33}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 90, Stop: 95}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 96, Stop: 101}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 111, Stop: 116}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 161, Stop: 166}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 173, Stop: 178}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 193, Stop: 198}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 199, Stop: 210}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 225, Stop: 232}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 250, Stop: 255}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "B", "A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 61, Parent: 17, Kind: "CodeSpan", ParentKind: "Paragraph", Lines: nil, Text: []text.Segment{{Start: 245, Stop: 250, Padding: 0, ForceNewline: false}, {Start: 250, Stop: 256, Padding: 0, ForceNewline: false}, {Start: 256, Stop: 261, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}, {Root: 0, Node: 67, Parent: 0, Kind: "CodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 274, Stop: 283, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 41, Parent: 17, Kind: "RawHTML", ParentKind: "Paragraph", Type: 0, Lines: []text.Segment{{Start: 168, Stop: 173, Padding: 0, ForceNewline: false}, {Start: 173, Stop: 179, Padding: 0, ForceNewline: false}, {Start: 179, Stop: 182, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}, {Root: 0, Node: 50, Parent: 17, Kind: "RawHTML", ParentKind: "Paragraph", Type: 0, Lines: []text.Segment{{Start: 213, Stop: 217, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}, {Root: 0, Node: 52, Parent: 17, Kind: "RawHTML", ParentKind: "Paragraph", Type: 0, Lines: []text.Segment{{Start: 218, Stop: 223, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: false, HasInfo: false, Raw: false}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "List", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "ListItem", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TextBlock", Depth: 3, Lines: []graph.Span{{Start: 2, Stop: 51}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 2, Stop: 4}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 4, Stop: 5}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 5, Stop: 17}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 23}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 24}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 24, Stop: 28}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 28, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 33}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 33, Stop: 41}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 41, Stop: 47}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 51}}, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 54, Stop: 66}, {Start: 68, Stop: 82}, {Start: 86, Stop: 90}, {Start: 90, Stop: 117}, {Start: 117, Stop: 120}, {Start: 122, Stop: 136}, {Start: 136, Stop: 173}, {Start: 173, Stop: 179}, {Start: 179, Stop: 199}, {Start: 199, Stop: 224}, {Start: 224, Stop: 236}, {Start: 236, Stop: 244}, {Start: 244, Stop: 250}, {Start: 250, Stop: 256}, {Start: 256, Stop: 268}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 54, Stop: 55}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 55, Stop: 61}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 61, Stop: 65}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 68, Stop: 69}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 69, Stop: 75}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 75, Stop: 81}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 86, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 89}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 90, Stop: 91}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 91, Stop: 92}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 92, Stop: 97}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 97, Stop: 98}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 98, Stop: 103}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 103, Stop: 112}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 112, Stop: 113}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 113, Stop: 115}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 115, Stop: 116}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 117, Stop: 119}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 122, Stop: 123}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 123, Stop: 129}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 129, Stop: 135}}, Raw: false, Soft: true, Hard: false}, {Kind: "AutoLink", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 167, Stop: 168}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 168, Stop: 173}, {Start: 173, Stop: 179}, {Start: 179, Stop: 182}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 182, Stop: 183}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 183, Stop: 194}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 194, Stop: 195}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 195, Stop: 197}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 197, Stop: 198}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 199, Stop: 200}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 200, Stop: 201}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 201, Stop: 213}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 213, Stop: 217}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 217, Stop: 218}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 218, Stop: 223}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 223, Stop: 223}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 224, Stop: 226}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 226, Stop: 227}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 227, Stop: 233}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 233, Stop: 235}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 236, Stop: 240}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 240, Stop: 242}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 242, Stop: 243}}, Raw: false, Soft: true, Hard: false}, {Kind: "CodeSpan", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 245, Stop: 250}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 250, Stop: 256}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 256, Stop: 261}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 262, Stop: 263}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 263, Stop: 268}}, Raw: false, Soft: false, Hard: false}, {Kind: "CodeBlock", Depth: 1, Lines: []graph.Span{{Start: 274, Stop: 283}}, Text: nil, Raw: false, Soft: false, Hard: false}}}, Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 245, Stop: 261}}, {Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 274, Stop: 283}}}, Comments: []graph.Span{{Start: 168, Stop: 182}}, Unanchorable: nil, Valid: true}, Removed: agreementRemovedAliasDiagnosticReading{Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 245, Stop: 261}}, {Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 274, Stop: 283}}}, Comments: []graph.Span{{Start: 168, Stop: 182}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 161, Stop: 166}, {Start: 245, Stop: 261}, {Start: 274, Stop: 283}}, Comments: []graph.Span{{Start: 168, Stop: 182}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 16}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 27, Stop: 33}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 90, Stop: 95}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 96, Stop: 101}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 111, Stop: 116}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 161, Stop: 166}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 173, Stop: 178}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 193, Stop: 198}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 199, Stop: 210}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 225, Stop: 232}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 250, Stop: 255}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "B", "A"}}, Projected: "- ![[image.png]][[A]] [[A]][[#A]]``1. > [!note] one\n> [!note] two\n> [!note] three\n    ```\n[[A]] [[A]]- ```` go [[A]]\nÉ\n> [!note] title\nhttps://example.invalid/`[[A]]` \n\n[^unused]: [[A]]\n[[B|alias]]## <em>A</em>\n![[A#A]]# A\nref[^n]\n`open\n[[A]]\nclose`- item\n\n      0", Positions: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164, 165, 166, 167, 172, 178, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255, 256, 257, 258, 259, 260, 261, 262, 263, 264, 265, 266, 267, 268, 269, 270, 271, 272, 273, 274, 275, 276, 277, 278, 279, 280, 281, 282}, ProjectedNative: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "unused", Parent: "Document", Span: graph.Span{Start: 170, Stop: 271}, Name: graph.Span{Start: 170, Stop: 180}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 181, Stop: 187}, {Start: 187, Stop: 212}, {Start: 212, Stop: 224}, {Start: 224, Stop: 232}, {Start: 232, Stop: 238}, {Start: 238, Stop: 244}, {Start: 244, Stop: 256}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 181, Stop: 182}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 182, Stop: 183}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 183, Stop: 185}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 185, Stop: 186}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 187, Stop: 188}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 188, Stop: 189}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 189, Stop: 201}}}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 201, Stop: 205}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 205, Stop: 206}}}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 206, Stop: 211}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 211, Stop: 211}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 212, Stop: 214}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 214, Stop: 215}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 215, Stop: 221}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 221, Stop: 223}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 224, Stop: 228}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 228, Stop: 230}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 230, Stop: 231}}}, {Kind: "CodeSpan", Depth: 2, Lines: nil, Text: nil}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 233, Stop: 238}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 238, Stop: 244}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 244, Stop: 249}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 250, Stop: 251}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 251, Stop: 256}}}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 264, Stop: 271}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 264, Stop: 271}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 233, Stop: 249}}}, Comments: nil}, ProjectedCheck: agreementNativeCheckSource{Code: []graph.Span{{Start: 161, Stop: 166}, {Start: 233, Stop: 249}, {Start: 262, Stop: 271}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 16}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 27, Stop: 33}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 90, Stop: 95}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 96, Stop: 101}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 111, Stop: 116}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 161, Stop: 166}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 181, Stop: 186}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 187, Stop: 198}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 213, Stop: 220}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 238, Stop: 243}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: 0}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "B", "A"}}, ProjectedTree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "List", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "ListItem", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TextBlock", Depth: 3, Lines: []graph.Span{{Start: 2, Stop: 51}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 2, Stop: 4}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 4, Stop: 5}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 5, Stop: 17}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 23}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 24}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 24, Stop: 28}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 28, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 33}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 33, Stop: 41}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 41, Stop: 47}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 51}}, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 54, Stop: 66}, {Start: 68, Stop: 82}, {Start: 86, Stop: 90}, {Start: 90, Stop: 117}, {Start: 117, Stop: 120}, {Start: 122, Stop: 136}, {Start: 136, Stop: 167}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 54, Stop: 55}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 55, Stop: 61}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 61, Stop: 65}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 68, Stop: 69}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 69, Stop: 75}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 75, Stop: 81}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 86, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 89}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 90, Stop: 91}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 91, Stop: 92}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 92, Stop: 97}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 97, Stop: 98}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 98, Stop: 103}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 103, Stop: 112}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 112, Stop: 113}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 113, Stop: 115}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 115, Stop: 116}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 117, Stop: 119}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 122, Stop: 123}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 123, Stop: 129}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 129, Stop: 135}}, Raw: false, Soft: true, Hard: false}, {Kind: "AutoLink", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}, {{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 181, Stop: 187}, {Start: 187, Stop: 212}, {Start: 212, Stop: 224}, {Start: 224, Stop: 232}, {Start: 232, Stop: 238}, {Start: 238, Stop: 244}, {Start: 244, Stop: 256}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 181, Stop: 182}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 182, Stop: 183}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 183, Stop: 185}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 185, Stop: 186}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 187, Stop: 188}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 188, Stop: 189}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 189, Stop: 201}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 201, Stop: 205}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 205, Stop: 206}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 206, Stop: 211}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 211, Stop: 211}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 212, Stop: 214}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 214, Stop: 215}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 215, Stop: 221}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 221, Stop: 223}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 224, Stop: 228}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 228, Stop: 230}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 230, Stop: 231}}, Raw: false, Soft: true, Hard: false}, {Kind: "CodeSpan", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 233, Stop: 238}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 238, Stop: 244}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 244, Stop: 249}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 250, Stop: 251}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 251, Stop: 256}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 264, Stop: 271}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 264, Stop: 271}}, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 1, Node: 0, Label: "unused", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 170, Stop: 271}, Label: graph.Span{Start: 170, Stop: 180}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 233, Stop: 249}}}, Comments: nil, Unanchorable: nil, Valid: true}, Valid: true}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "image.png", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, {SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CitationsInCode: 0, Blocks: nil, Headings: nil, Failures: nil, CodeCitations: nil, CalloutTitles: []string{"two", "three", "title"}}, Parts: [][]agreementCodePart{{{Text: "```\n", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "B", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementRemovedAliasDiagnosticVariant{{Target: "B", Body: "- ![[image.png]][[A]] [[A]][[#A]]``1. > [!note] one\n> [!note] two\n> [!note] three\n    ```\n[[A]] [[A]]- ```` go [[A]]\nÉ\n> [!note] title\nhttps://example.invalid/`[[A]]` <!--\n[[A]]\n-->[^unused]: [[A]]\n[[q|alias]]## <em>A</em>\n![[A#A]]# A\nref[^n]\n`open\n[[A]]\nclose`- item\n\n      0", Physical: []agreementRemovedAliasDiagnosticOwner{{Field: 10, Definition: 0, Root: 1, Node: 1, Line: 1, Label: "unused", Projected: graph.Span{Start: 187, Stop: 198}}}, Owners: []agreementRemovedAliasDiagnosticOwner{{Field: 10, Definition: 0, Root: 1, Node: 1, Line: 1, Label: "unused", Projected: graph.Span{Start: 187, Stop: 198}}}, Changes: []agreementMixedRename{{Field: 10, Span: graph.Span{Start: 201, Stop: 202}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 161, Stop: 166}, {Start: 245, Stop: 261}, {Start: 274, Stop: 283}}, Comments: []graph.Span{{Start: 168, Stop: 182}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 16}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 27, Stop: 33}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 90, Stop: 95}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 96, Stop: 101}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 111, Stop: 116}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 161, Stop: 166}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 173, Stop: 178}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 193, Stop: 198}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 199, Stop: 210}, Word: "[[q|alias]]", Targets: []string{"q"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 225, Stop: 232}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 250, Stop: 255}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "q", "A"}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "q", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:q Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Removed: agreementRemovedAliasDiagnosticReading{Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 245, Stop: 261}}, {Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 274, Stop: 283}}}, Comments: []graph.Span{{Start: 168, Stop: 182}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 161, Stop: 166}, {Start: 245, Stop: 261}, {Start: 274, Stop: 283}}, Comments: []graph.Span{{Start: 168, Stop: 182}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 16}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 27, Stop: 33}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 90, Stop: 95}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 96, Stop: 101}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 111, Stop: 116}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 161, Stop: 166}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 173, Stop: 178}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 193, Stop: 198}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 199, Stop: 210}, Word: "[[q|alias]]", Targets: []string{"q"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 225, Stop: 232}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 250, Stop: 255}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "q", "A"}}, Projected: "- ![[image.png]][[A]] [[A]][[#A]]``1. > [!note] one\n> [!note] two\n> [!note] three\n    ```\n[[A]] [[A]]- ```` go [[A]]\nÉ\n> [!note] title\nhttps://example.invalid/`[[A]]` \n\n[^unused]: [[A]]\n[[q|alias]]## <em>A</em>\n![[A#A]]# A\nref[^n]\n`open\n[[A]]\nclose`- item\n\n      0", Positions: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164, 165, 166, 167, 172, 178, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 240, 241, 242, 243, 244, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255, 256, 257, 258, 259, 260, 261, 262, 263, 264, 265, 266, 267, 268, 269, 270, 271, 272, 273, 274, 275, 276, 277, 278, 279, 280, 281, 282}, ProjectedNative: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "unused", Parent: "Document", Span: graph.Span{Start: 170, Stop: 271}, Name: graph.Span{Start: 170, Stop: 180}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 181, Stop: 187}, {Start: 187, Stop: 212}, {Start: 212, Stop: 224}, {Start: 224, Stop: 232}, {Start: 232, Stop: 238}, {Start: 238, Stop: 244}, {Start: 244, Stop: 256}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 181, Stop: 182}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 182, Stop: 183}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 183, Stop: 185}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 185, Stop: 186}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 187, Stop: 188}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 188, Stop: 189}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 189, Stop: 201}}}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 201, Stop: 205}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 205, Stop: 206}}}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 206, Stop: 211}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 211, Stop: 211}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 212, Stop: 214}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 214, Stop: 215}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 215, Stop: 221}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 221, Stop: 223}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 224, Stop: 228}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 228, Stop: 230}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 230, Stop: 231}}}, {Kind: "CodeSpan", Depth: 2, Lines: nil, Text: nil}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 233, Stop: 238}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 238, Stop: 244}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 244, Stop: 249}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 250, Stop: 251}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 251, Stop: 256}}}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 264, Stop: 271}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 264, Stop: 271}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 233, Stop: 249}}}, Comments: nil}, ProjectedCheck: agreementNativeCheckSource{Code: []graph.Span{{Start: 161, Stop: 166}, {Start: 233, Stop: 249}, {Start: 262, Stop: 271}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 3, Stop: 16}, Word: "[[image.png]]", Targets: []string{"image.png"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 27, Stop: 33}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 90, Stop: 95}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 96, Stop: 101}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 111, Stop: 116}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 161, Stop: 166}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 181, Stop: 186}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 187, Stop: 198}, Word: "[[q|alias]]", Targets: []string{"q"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 213, Stop: 220}, Word: "[[A#A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 238, Stop: 243}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: 0}}, Targets: []string{"image.png", "A", "A", "A", "A", "A", "A", "q", "A"}}, ProjectedTree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "List", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "ListItem", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "TextBlock", Depth: 3, Lines: []graph.Span{{Start: 2, Stop: 51}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 2, Stop: 4}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 4, Stop: 5}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 5, Stop: 17}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 23}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 23, Stop: 24}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 24, Stop: 28}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 28, Stop: 29}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 29, Stop: 33}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 33, Stop: 41}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 41, Stop: 47}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 51}}, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 54, Stop: 66}, {Start: 68, Stop: 82}, {Start: 86, Stop: 90}, {Start: 90, Stop: 117}, {Start: 117, Stop: 120}, {Start: 122, Stop: 136}, {Start: 136, Stop: 167}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 54, Stop: 55}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 55, Stop: 61}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 61, Stop: 65}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 68, Stop: 69}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 69, Stop: 75}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 75, Stop: 81}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 86, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 89}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 90, Stop: 91}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 91, Stop: 92}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 92, Stop: 97}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 97, Stop: 98}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 98, Stop: 103}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 103, Stop: 112}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 112, Stop: 113}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 113, Stop: 115}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 115, Stop: 116}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 117, Stop: 119}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 122, Stop: 123}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 123, Stop: 129}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 129, Stop: 135}}, Raw: false, Soft: true, Hard: false}, {Kind: "AutoLink", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}, {{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 181, Stop: 187}, {Start: 187, Stop: 212}, {Start: 212, Stop: 224}, {Start: 224, Stop: 232}, {Start: 232, Stop: 238}, {Start: 238, Stop: 244}, {Start: 244, Stop: 256}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 181, Stop: 182}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 182, Stop: 183}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 183, Stop: 185}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 185, Stop: 186}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 187, Stop: 188}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 188, Stop: 189}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 189, Stop: 201}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 201, Stop: 205}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 205, Stop: 206}}, Raw: false, Soft: false, Hard: false}, {Kind: "RawHTML", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 206, Stop: 211}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 211, Stop: 211}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 212, Stop: 214}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 214, Stop: 215}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 215, Stop: 221}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 221, Stop: 223}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 224, Stop: 228}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 228, Stop: 230}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 230, Stop: 231}}, Raw: false, Soft: true, Hard: false}, {Kind: "CodeSpan", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 233, Stop: 238}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 238, Stop: 244}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 244, Stop: 249}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 250, Stop: 251}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 251, Stop: 256}}, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 264, Stop: 271}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 264, Stop: 271}}, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 1, Node: 0, Label: "unused", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 170, Stop: 271}, Label: graph.Span{Start: 170, Stop: 180}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 233, Stop: 249}}}, Comments: nil, Unanchorable: nil, Valid: true}, Valid: true}}}},
		{Name: "1dc283f2f9a6", Body: "<!--\n[[A]]\n-->[^unused]: [[A]]\n[[B|alias]]- item\n\n      > [!unknown] title\n`[[A]]`[[#A]][[A#^a]]\\[[A]]", Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 54, Stop: 75}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 76, Stop: 81}}}, Comments: []graph.Span{{Start: 0, Stop: 14}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 54, Stop: 75}, {Start: 76, Stop: 81}}, Comments: []graph.Span{{Start: 0, Stop: 14}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 5, Stop: 10}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 31, Stop: 42}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 76, Stop: 81}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 82, Stop: 88}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 88, Stop: 96}, Word: "[[A#^a]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 97, Stop: 102}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B", "A"}}, Sites: []agreementNativeCodePayloadSite{{Root: 0, Node: 7, Parent: 0, Kind: "CodeBlock", ParentKind: "Document", Lines: []text.Segment{{Start: 54, Stop: 75, Padding: 0, ForceNewline: true}}, Text: nil, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: true}, {Root: 0, Node: 9, Parent: 8, Kind: "CodeSpan", ParentKind: "Paragraph", Lines: nil, Text: []text.Segment{{Start: 76, Stop: 81, Padding: 0, ForceNewline: false}}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasInfo: false, Raw: false}}, HTML: []agreementNativeHTMLAddressSite{{Root: 0, Node: 1, Parent: 0, Kind: "HTMLBlock", ParentKind: "Document", Type: 2, Lines: []text.Segment{{Start: 0, Stop: 5, Padding: 0, ForceNewline: false}, {Start: 5, Stop: 11, Padding: 0, ForceNewline: false}}, Closure: text.Segment{Start: 11, Stop: 31, Padding: 0, ForceNewline: false}, Info: text.Segment{Start: 0, Stop: 0, Padding: 0, ForceNewline: false}, HasClosure: true, HasInfo: false, Raw: true}}, Tree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "HTMLBlock", Depth: 1, Lines: []graph.Span{{Start: 0, Stop: 5}, {Start: 5, Stop: 11}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 31, Stop: 48}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 31, Stop: 32}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 32, Stop: 33}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 33, Stop: 43}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 43, Stop: 48}}, Raw: false, Soft: false, Hard: false}, {Kind: "CodeBlock", Depth: 1, Lines: []graph.Span{{Start: 54, Stop: 75}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 75, Stop: 102}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "CodeSpan", Depth: 2, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 76, Stop: 81}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 82, Stop: 83}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 83, Stop: 84}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 84, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 90}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 90, Stop: 99}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 99, Stop: 101}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 101, Stop: 102}}, Raw: false, Soft: false, Hard: false}}}, Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 54, Stop: 75}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 76, Stop: 81}}}, Comments: []graph.Span{{Start: 0, Stop: 14}}, Unanchorable: nil, Valid: true}, Removed: agreementRemovedAliasDiagnosticReading{Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 54, Stop: 75}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 76, Stop: 81}}}, Comments: []graph.Span{{Start: 0, Stop: 14}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 54, Stop: 75}, {Start: 76, Stop: 81}}, Comments: []graph.Span{{Start: 0, Stop: 14}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 5, Stop: 10}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 31, Stop: 42}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 76, Stop: 81}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 82, Stop: 88}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 88, Stop: 96}, Word: "[[A#^a]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 97, Stop: 102}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B", "A"}}, Projected: "\n\n[^unused]: [[A]]\n[[B|alias]]- item\n\n      > [!unknown] title\n`[[A]]`[[#A]][[A#^a]]\\[[A]]", Positions: []int{4, 10, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101}, ProjectedNative: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "unused", Parent: "Document", Span: graph.Span{Start: 2, Stop: 90}, Name: graph.Span{Start: 2, Stop: 12}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 13, Stop: 19}, {Start: 19, Stop: 36}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 13, Stop: 14}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 14, Stop: 15}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 15, Stop: 17}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 20}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 20, Stop: 21}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 31}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 31, Stop: 36}}}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 46, Stop: 63}, {Start: 63, Stop: 90}}, Text: nil}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 46, Stop: 47}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 56}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 56, Stop: 62}}}, {Kind: "CodeSpan", Depth: 3, Lines: nil, Text: nil}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 64, Stop: 69}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 70, Stop: 71}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 71, Stop: 72}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 72, Stop: 77}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 77, Stop: 78}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 78, Stop: 87}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 87, Stop: 89}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 90}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 64, Stop: 69}}}, Comments: nil}, ProjectedCheck: agreementNativeCheckSource{Code: []graph.Span{{Start: 42, Stop: 63}, {Start: 64, Stop: 69}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 13, Stop: 18}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 19, Stop: 30}, Word: "[[B|alias]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 64, Stop: 69}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: 0}, {Span: graph.Span{Start: 70, Stop: 76}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 76, Stop: 84}, Word: "[[A#^a]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 85, Stop: 90}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: 0, CodeOwner: -1}}, Targets: []string{"A", "B", "A"}}, ProjectedTree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}, {{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 13, Stop: 19}, {Start: 19, Stop: 36}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 13, Stop: 14}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 14, Stop: 15}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 15, Stop: 17}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 20}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 20, Stop: 21}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 31}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 31, Stop: 36}}, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 46, Stop: 63}, {Start: 63, Stop: 90}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 46, Stop: 47}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 56}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 56, Stop: 62}}, Raw: false, Soft: true, Hard: false}, {Kind: "CodeSpan", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 64, Stop: 69}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 70, Stop: 71}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 71, Stop: 72}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 72, Stop: 77}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 77, Stop: 78}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 78, Stop: 87}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 87, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 90}}, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 1, Node: 0, Label: "unused", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 2, Stop: 90}, Label: graph.Span{Start: 2, Stop: 12}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 64, Stop: 69}}}, Comments: nil, Unanchorable: nil, Valid: true}, Valid: true}, Page: agreementHTML{Citations: []agreementCitation{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}}, CitationsInCode: 0, Blocks: nil, Headings: nil, Failures: nil, CodeCitations: nil, CalloutTitles: []string{"title"}}, Parts: [][]agreementCodePart{{{Text: "[[A]]", Citation: agreementCitation{SourceRole: "", Target: "", Section: "", State: ""}, Field: false}}}, Targets: []string{"A", "B", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Selected: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Variants: []agreementRemovedAliasDiagnosticVariant{{Target: "B", Body: "<!--\n[[A]]\n-->[^unused]: [[A]]\n[[q|alias]]- item\n\n      > [!unknown] title\n`[[A]]`[[#A]][[A#^a]]\\[[A]]", Physical: []agreementRemovedAliasDiagnosticOwner{{Field: 2, Definition: 0, Root: 1, Node: 1, Line: 1, Label: "unused", Projected: graph.Span{Start: 19, Stop: 30}}}, Owners: []agreementRemovedAliasDiagnosticOwner{{Field: 2, Definition: 0, Root: 1, Node: 1, Line: 1, Label: "unused", Projected: graph.Span{Start: 19, Stop: 30}}}, Changes: []agreementMixedRename{{Field: 2, Span: graph.Span{Start: 33, Stop: 34}, Old: "B", New: "q"}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 54, Stop: 75}, {Start: 76, Stop: 81}}, Comments: []graph.Span{{Start: 0, Stop: 14}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 5, Stop: 10}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 31, Stop: 42}, Word: "[[q|alias]]", Targets: []string{"q"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 76, Stop: 81}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 82, Stop: 88}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 88, Stop: 96}, Word: "[[A#^a]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 97, Stop: 102}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "q", "A"}}, Targets: []string{"A", "q", "A"}, Signatures: map[string]int{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P0/diagnostic-html tuple={SourceRole: Target:q Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1, "P1/citation-occurrences tuple={SourceRole: Target:q Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false": 1}, Removed: agreementRemovedAliasDiagnosticReading{Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 54, Stop: 75}}, {Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 76, Stop: 81}}}, Comments: []graph.Span{{Start: 0, Stop: 14}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 54, Stop: 75}, {Start: 76, Stop: 81}}, Comments: []graph.Span{{Start: 0, Stop: 14}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 5, Stop: 10}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 30}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 31, Stop: 42}, Word: "[[q|alias]]", Targets: []string{"q"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 76, Stop: 81}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 1}, {Span: graph.Span{Start: 82, Stop: 88}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 88, Stop: 96}, Word: "[[A#^a]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 97, Stop: 102}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "q", "A"}}, Projected: "\n\n[^unused]: [[A]]\n[[q|alias]]- item\n\n      > [!unknown] title\n`[[A]]`[[#A]][[A#^a]]\\[[A]]", Positions: []int{4, 10, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101}, ProjectedNative: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "unused", Parent: "Document", Span: graph.Span{Start: 2, Stop: 90}, Name: graph.Span{Start: 2, Stop: 12}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 13, Stop: 19}, {Start: 19, Stop: 36}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 13, Stop: 14}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 14, Stop: 15}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 15, Stop: 17}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 20}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 20, Stop: 21}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 31}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 31, Stop: 36}}}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 46, Stop: 63}, {Start: 63, Stop: 90}}, Text: nil}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 46, Stop: 47}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 56}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 56, Stop: 62}}}, {Kind: "CodeSpan", Depth: 3, Lines: nil, Text: nil}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 64, Stop: 69}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 70, Stop: 71}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 71, Stop: 72}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 72, Stop: 77}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 77, Stop: 78}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 78, Stop: 87}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 87, Stop: 89}}}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 90}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 64, Stop: 69}}}, Comments: nil}, ProjectedCheck: agreementNativeCheckSource{Code: []graph.Span{{Start: 42, Stop: 63}, {Start: 64, Stop: 69}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 13, Stop: 18}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 19, Stop: 30}, Word: "[[q|alias]]", Targets: []string{"q"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 64, Stop: 69}, Word: "[[A]]", Targets: []string{"A"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: 0}, {Span: graph.Span{Start: 70, Stop: 76}, Word: "[[#A]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 76, Stop: 84}, Word: "[[A#^a]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 85, Stop: 90}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: 0, CodeOwner: -1}}, Targets: []string{"A", "q", "A"}}, ProjectedTree: agreementNativeProseAddressTree{Trees: [][]agreementNativeProseAddressNode{{{Kind: "Document", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}}, {{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 13, Stop: 19}, {Start: 19, Stop: 36}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 13, Stop: 14}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 14, Stop: 15}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 15, Stop: 17}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}, Raw: false, Soft: true, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 20}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 20, Stop: 21}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 31}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 31, Stop: 36}}, Raw: false, Soft: false, Hard: false}, {Kind: "Blockquote", Depth: 1, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 46, Stop: 63}, {Start: 63, Stop: 90}}, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 46, Stop: 47}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 47, Stop: 56}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 56, Stop: 62}}, Raw: false, Soft: true, Hard: false}, {Kind: "CodeSpan", Depth: 3, Lines: nil, Text: nil, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 4, Lines: nil, Text: []graph.Span{{Start: 64, Stop: 69}}, Raw: true, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 70, Stop: 71}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 71, Stop: 72}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 72, Stop: 77}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 77, Stop: 78}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 78, Stop: 87}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 87, Stop: 89}}, Raw: false, Soft: false, Hard: false}, {Kind: "Text", Depth: 3, Lines: nil, Text: []graph.Span{{Start: 89, Stop: 90}}, Raw: false, Soft: false, Hard: false}}}, Definitions: []agreementNativeProseAddressDefinition{{Root: 1, Node: 0, Label: "unused", Bounds: agreementNativeDefinitionSpan{Span: graph.Span{Start: 2, Stop: 90}, Label: graph.Span{Start: 2, Stop: 12}, Parent: "Document"}, Found: true}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 64, Stop: 69}}}, Comments: nil, Unanchorable: nil, Valid: true}, Valid: true}}}},
	}
}
func agreementRemovedAliasDiagnosticWitnesses() []agreementRemovedAliasDiagnosticWitness {
	return []agreementRemovedAliasDiagnosticWitness{
		{Name: "6815c332b79f", Body: "- ![[image.png]][[A]] [[A]][[#A]]``1. > [!note] one\n> [!note] two\n> [!note] three\n    ```\n[[A]] [[A]]- ```` go [[A]]\nÉ\n> [!note] title\nhttps://example.invalid/`[[A]]` <!--\n[[A]]\n-->[^unused]: [[A]]\n[[B|alias]]## <em>A</em>\n![[A#A]]# A\nref[^n]\n`open\n[[A]]\nclose`- item\n\n      0", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "link-section-missing", Target: "", Message: "no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "link-section-missing", Target: "", Message: "no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}, Indices: []int{8}, Singles: []agreementRemovedAliasDiagnosticSingle{{Field: 10, Index: 8, Start: 201, Body: "- ![[image.png]][[A]] [[A]][[#A]]``1. > [!note] one\n> [!note] two\n> [!note] three\n    ```\n[[A]] [[A]]- ```` go [[A]]\nÉ\n> [!note] title\nhttps://example.invalid/`[[A]]` <!--\n[[A]]\n-->[^unused]: [[A]]\n[[q|alias]]## <em>A</em>\n![[A#A]]# A\nref[^n]\n`open\n[[A]]\nclose`- item\n\n      0", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "image.png", Message: "embed target \"image.png\" does not resolve", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "link-section-missing", Target: "", Message: "no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "embed target \"A\" does not resolve", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}}}},
		{Name: "1dc283f2f9a6", Body: "<!--\n[[A]]\n-->[^unused]: [[A]]\n[[B|alias]]- item\n\n      > [!unknown] title\n`[[A]]`[[#A]][[A#^a]]\\[[A]]", Target: "B", Selected: "P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "B", Message: "wikilink \"B\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "unknown-callout", Target: "unknown", Message: "unknown callout type \"unknown\"; rendered as a neutral callout", Section: "", Block: ""}, {Kind: "link-section-missing", Target: "", Message: "no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}, PrivateDiagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "unknown-callout", Target: "unknown", Message: "unknown callout type \"unknown\"; rendered as a neutral callout", Section: "", Block: ""}, {Kind: "link-section-missing", Target: "", Message: "no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}, Indices: []int{1}, Singles: []agreementRemovedAliasDiagnosticSingle{{Field: 2, Index: 1, Start: 33, Body: "<!--\n[[A]]\n-->[^unused]: [[A]]\n[[q|alias]]- item\n\n      > [!unknown] title\n`[[A]]`[[#A]][[A#^a]]\\[[A]]", Diagnostics: []render.Diagnostic{{Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "wikilink-broken", Target: "q", Message: "wikilink \"q\" does not resolve to any note or file", Section: "", Block: ""}, {Kind: "unknown-callout", Target: "unknown", Message: "unknown callout type \"unknown\"; rendered as a neutral callout", Section: "", Block: ""}, {Kind: "link-section-missing", Target: "", Message: "no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note", Section: "A", Block: ""}, {Kind: "wikilink-broken", Target: "A", Message: "wikilink \"A\" does not resolve to any note or file", Section: "", Block: ""}}}}},
	}
}
func TestAgreementRemovedAliasDiagnosticOwnedSource(t *testing.T) {
	t.Parallel()
	records := agreementRemovedAliasDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementRemovedAliasDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			if len(r.Singles) != len(v.Owners) || len(r.Indices) != len(v.Owners) {
				t.Fatal("caught: complete comment-removed discarded alias diagnostic owner set")
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
					t.Fatal("caught: exact single comment-removed discarded alias diagnostic source projection")
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
					t.Fatal("caught: complete unchanged single comment-removed discarded alias diagnostic source facts")
				}
				if len(single.Diagnostics) != len(r.Diagnostics) {
					t.Fatal("caught: complete ordered single comment-removed discarded alias diagnostic ledger")
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
				t.Fatal("caught: whole ordered comment-removed discarded alias diagnostic substitution")
			}
			for _, owner := range v.Owners {
				if !fields[owner.Field] {
					t.Fatal("caught: every declared comment-removed discarded alias source field is represented")
				}
			}
		})
	}
}

func TestAgreementRemovedAliasDiagnosticOwnedPublic(t *testing.T) {
	t.Parallel()
	records := agreementRemovedAliasDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementRemovedAliasDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
			if !cmp.Equal(raw.Diagnostics, r.Diagnostics) || !cmp.Equal(page, w.Page) {
				t.Fatal("caught: complete original comment-removed discarded alias diagnostic and public ledgers")
			}
			private, pp := agreementIsolatedPage(t, agreementCase{Body: v.Body})
			if !cmp.Equal(agreementRemovedAliasDiagnosticRead(r.Body), w.Removed) || !cmp.Equal(agreementRemovedAliasDiagnosticRead(v.Body), v.Removed) {
				t.Fatal("caught: complete before and private comment removal declarations")
			}

			if raw.HTML != private.HTML || !cmp.Equal(private.Diagnostics, r.PrivateDiagnostics) || !cmp.Equal(pp, w.Page) {
				t.Fatal("caught: whole comment-removed discarded alias diagnostic projection leaves complete HTML unchanged")
			}
			for si := range r.Singles {
				single := &r.Singles[si]
				projected, p := agreementIsolatedPage(t, agreementCase{Body: single.Body})
				if projected.HTML != raw.HTML || !cmp.Equal(projected.Diagnostics, single.Diagnostics) || !cmp.Equal(p, w.Page) {
					t.Fatal("caught: complete independent single-field comment-removed discarded alias diagnostic observation")
				}
				parts, valid := agreementCodeWindowParts(projected.HTML)
				if !valid || !cmp.Equal(parts, w.Parts) {
					t.Fatal("caught: every single-field comment-removed discarded alias diagnostic code ledger stays identical")
				}
			}
			c := agreementCase{Body: r.Body}
			failures := agreementPageFailures(r.Body, &raw, &page)
			failures = append(failures, agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{page})[0]...)
			if !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
				t.Fatal("caught: complete original comment-removed discarded alias diagnostic signed pool")
			}
			privateFailures := agreementPageFailures(v.Body, &private, &pp)
			privateFailures = append(privateFailures, agreementFragmentFailures(t, []agreementCase{{Body: v.Body}}, []agreementHTML{pp})[0]...)
			if !maps.Equal(agreementMixedCurrent(privateFailures), v.Signatures) {
				t.Fatal("caught: complete private comment-removed discarded alias diagnostic signed pool")
			}
			seen := 0
			for _, f := range failures {
				if agreementSignature(&f) != r.Selected {
					continue
				}
				seen++
				k, a, b := agreementRemovedAliasDiagnosticDifference(c, &f, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "debt" || a != "#1011 stage 5" || b != "page-diagnostic" {
					t.Fatal("caught: exact comment-removed discarded alias diagnostic P0 debt owner")
				}
			}
			if seen != 1 {
				t.Fatal("diagnostic selected receipt setup")
			}
		})
	}
}

func TestAgreementRemovedAliasDiagnosticOwnedRefusals(t *testing.T) {
	t.Parallel()
	records := agreementRemovedAliasDiagnosticWitnesses()
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
					k, a, b := agreementRemovedAliasDiagnosticDifference(c, &changed, raw.HTML, &page, raw.Diagnostics, failures)
					if k != "" || a != "" || b != "" {
						t.Fatal("caught: comment-removed discarded alias diagnostic borrowed another signed face")
					}
				})
			}
			for _, other := range []agreementCase{{Body: r.Body + "\n"}, {Body: r.Body, Title: "A"}, {Body: r.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
				k, a, b := agreementRemovedAliasDiagnosticDifference(other, &selected, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: comment-removed discarded alias diagnostic borrowed another body or context")
				}
			}
			for i := range failures {
				missing := slices.Delete(slices.Clone(failures), i, i+1)
				k, a, b := agreementRemovedAliasDiagnosticDifference(c, &selected, raw.HTML, &page, raw.Diagnostics, missing)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: comment-removed discarded alias diagnostic borrowed an incomplete whole signed pool")
				}
			}
			for i := range raw.Diagnostics {
				missing := slices.Delete(slices.Clone(raw.Diagnostics), i, i+1)
				k, a, b := agreementRemovedAliasDiagnosticDifference(c, &selected, raw.HTML, &page, missing, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: comment-removed discarded alias diagnostic borrowed an incomplete whole raw ledger")
				}
				altered := slices.Clone(raw.Diagnostics)
				altered[i].Message += "outside"
				k, a, b = agreementRemovedAliasDiagnosticDifference(c, &selected, raw.HTML, &page, altered, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: comment-removed discarded alias diagnostic borrowed another complete raw ledger")
				}
			}
			changed := page
			changed.Headings = append(slices.Clone(page.Headings), "outside")
			k, a, b := agreementRemovedAliasDiagnosticDifference(c, &selected, raw.HTML, &changed, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: comment-removed discarded alias diagnostic borrowed another complete public ledger")
			}
			k, a, b = agreementRemovedAliasDiagnosticDifference(c, &selected, raw.HTML+"<code>outside</code>", &page, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: comment-removed discarded alias diagnostic borrowed changed field-free code text")
			}
		})
	}
}

func TestAgreementRemovedAliasDiagnosticOwnedWholeTargets(t *testing.T) {
	t.Parallel()
	records := agreementRemovedAliasDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementRemovedAliasDiagnosticSource(r.Body, r.Target)
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
				t.Fatal("caught: complete whole comment-removed discarded alias diagnostic physical targets")
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
				t.Fatal("caught: comment-removed discarded alias diagnostic target counts conserve every physical owner")
			}
			delete(before, r.Target)
			delete(before, replacement)
			delete(after, r.Target)
			delete(after, replacement)
			if !maps.Equal(before, after) {
				t.Fatal("caught: every unselected whole comment-removed discarded alias diagnostic target count")
			}
		})
	}
}

func TestAgreementRemovedAliasDiagnosticOwnedWholeSet(t *testing.T) {
	t.Parallel()
	records := agreementRemovedAliasDiagnosticWitnesses()
	witnesses := agreementRemovedAliasDiagnosticSources()
	expected := make(map[[3]string]int)
	for _, r := range records {
		key := [3]string{r.Body, r.Target, r.Selected}
		if _, exists := expected[key]; exists || len(r.Singles) != len(r.Indices) {
			t.Fatal("caught: unique complete comment-removed discarded alias diagnostic declared roster")
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
			physical, owners, changes, private, valid := agreementRemovedAliasDiagnosticProducers(t, w.Body, target, native, &check)
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
			if !agreementRemovedAliasDiagnosticSignedProjection(failures, pf, &page, check.Targets, &f, len(owners)) {
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
				t.Fatal("caught: every eligible comment-removed discarded alias diagnostic roster member once")
			}
			actual[key] = len(owners)
			bodySelected[agreementSignature(&f)]++
			bodyVariants[[2]string{target, agreementSignature(&f)}] = len(owners)
			_, declared, found := agreementRemovedAliasDiagnosticSource(w.Body, target)
			if !found || !cmp.Equal(physical, declared.Physical) || !cmp.Equal(owners, declared.Owners) || !cmp.Equal(changes, declared.Changes) || private != declared.Body {
				t.Fatal("caught: complete native code physical and producer field census")
			}
		}
		declaredVariants := make(map[[2]string]int)
		for _, v := range w.Variants {
			f := agreementFailure{Property: "P0", Identity: "diagnostic-html", Tuple: agreementCitation{Target: v.Target, State: "wikilink-broken"}, Direction: "diagnostic-only", Multiplicity: len(v.Owners)}
			key := [2]string{v.Target, agreementSignature(&f)}
			if _, exists := declaredVariants[key]; exists {
				t.Fatal("caught: unique complete comment-removed discarded alias diagnostic source variants")
			}
			declaredVariants[key] = len(v.Owners)
		}
		if !maps.Equal(bodySelected, w.Selected) || !maps.Equal(bodyVariants, declaredVariants) {
			t.Fatal("caught: whole eligible comment-removed discarded alias diagnostic source declaration set")
		}
	}
	fields := 0
	for _, n := range actual {
		fields += n
	}
	if !maps.Equal(expected, actual) || len(actual) != 2 || fields != 2 || len(witnesses) != 2 {
		t.Fatal("caught: whole declared-body comment-removed discarded alias diagnostic eligibility census")
	}
}

func TestAgreementRemovedAliasDiagnosticOwnedSignedRefusals(t *testing.T) {
	for _, r := range agreementRemovedAliasDiagnosticWitnesses() {
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			w, v, ok := agreementRemovedAliasDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("signed diagnostic source setup")
			}
			raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
			projected, privatePage := agreementIsolatedPage(t, agreementCase{Body: v.Body})
			before := agreementPageFailures(r.Body, &raw, &page)
			before = append(before, agreementFragmentFailures(t, []agreementCase{{Body: r.Body}}, []agreementHTML{page})[0]...)
			after := agreementPageFailures(v.Body, &projected, &privatePage)
			after = append(after, agreementFragmentFailures(t, []agreementCase{{Body: v.Body}}, []agreementHTML{privatePage})[0]...)
			selected := agreementFailure{}
			found := false
			for _, f := range before {
				if agreementSignature(&f) == r.Selected {
					selected = f
					found = true
				}
			}
			if !found || !agreementRemovedAliasDiagnosticSignedProjection(before, after, &page, w.Targets, &selected, len(v.Owners)) {
				t.Fatal("caught: complete paired comment-removed alias diagnostic and check equation")
			}
			for i := range after {
				missing := slices.Delete(slices.Clone(after), i, i+1)
				if agreementRemovedAliasDiagnosticSignedProjection(before, missing, &page, w.Targets, &selected, len(v.Owners)) {
					t.Fatal("caught: paired HTML projection borrowed an incomplete whole signed pool")
				}
			}
			extraBefore := append(slices.Clone(before), selected)
			if agreementRemovedAliasDiagnosticSignedProjection(extraBefore, after, &page, w.Targets, &selected, len(v.Owners)) {
				t.Fatal("caught: paired HTML projection borrowed another original diagnostic count")
			}
			extraAfter := append(slices.Clone(after), selected)
			if agreementRemovedAliasDiagnosticSignedProjection(before, extraAfter, &page, w.Targets, &selected, len(v.Owners)) {
				t.Fatal("caught: paired HTML projection borrowed another projected signed pool")
			}
			forged := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: r.Target}, Direction: "judge-only", Multiplicity: 1}
			forgedBefore := append(slices.Clone(before), forged)
			if agreementRemovedAliasDiagnosticSignedProjection(forgedBefore, after, &page, w.Targets, &selected, len(v.Owners)) {
				t.Fatal("caught: paired HTML projection borrowed another original check deficit")
			}
			otherPage := page
			otherPage.Citations = append(slices.Clone(page.Citations), agreementCitation{Target: r.Target})
			if agreementRemovedAliasDiagnosticSignedProjection(before, after, &otherPage, w.Targets, &selected, len(v.Owners)) {
				t.Fatal("caught: paired HTML projection borrowed another public target count")
			}
			otherTargets := append(slices.Clone(w.Targets), r.Target)
			if agreementRemovedAliasDiagnosticSignedProjection(before, after, &page, otherTargets, &selected, len(v.Owners)) {
				t.Fatal("caught: paired HTML projection borrowed another physical check count")
			}
			if agreementRemovedAliasDiagnosticSignedProjection(before, after, &page, w.Targets, &selected, len(v.Owners)+1) {
				t.Fatal("caught: paired HTML projection borrowed another producer count")
			}
			if agreementRemovedAliasDiagnosticSignedProjection(before, after, &page, w.Targets, nil, len(v.Owners)) {
				t.Fatal("caught: paired HTML projection borrowed an absent selected face")
			}
		})
	}
}
func TestAgreementRemovedAliasDiagnosticOwnedPhysical(t *testing.T) {
	t.Parallel()
	records := agreementRemovedAliasDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementRemovedAliasDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			before := agreementRemovedAliasDiagnosticRead(r.Body)
			after := agreementRemovedAliasDiagnosticRead(v.Body)
			if !cmp.Equal(before, w.Removed) || !cmp.Equal(after, v.Removed) {
				t.Fatal("caught: whole original and private comment removal declarations")
			}
			actual, valid := agreementRemovedAliasDiagnosticPhysical(r.Body, &w.Check, r.Target)
			renamed, privateValid := agreementRemovedAliasDiagnosticPhysical(v.Body, &v.Check, strings.Repeat("q", len(r.Target)))
			if !valid || !privateValid || !cmp.Equal(actual, v.Physical) || !cmp.Equal(renamed, v.Physical) {
				t.Fatal("caught: complete original and private discarded physical ownership")
			}
			for _, owner := range actual {
				field := w.Check.Fields[owner.Field]
				if owner.Projected.Start < 0 || owner.Projected.Stop > len(before.Projected) || before.Projected[owner.Projected.Start:owner.Projected.Stop] != field.Word {
					t.Fatal("caught: original field occupies exactly its projected source span")
				}
			}
		})
	}
}

// Clipped definitions and fields without a complete plain owner stay outside the set.
