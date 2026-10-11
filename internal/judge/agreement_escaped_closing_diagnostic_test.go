package judge_test

import (
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementEscapedClosingDiagnosticSingle struct {
	Field, Index, Start int
	Body                string
	Diagnostics         []render.Diagnostic
}
type agreementEscapedClosingDiagnosticWitness struct {
	Name, Body, Target, RawTarget, Selected string
	Diagnostics, PrivateDiagnostics         []render.Diagnostic
	Indices                                 []int
	Singles                                 []agreementEscapedClosingDiagnosticSingle
}

type agreementEscapedClosingDiagnosticOwner struct {
	Field     int
	Family    string
	Code      agreementNativeCodeDiagnosticOwner
	Native    agreementMixedUnusedOwner
	HTML      agreementHTMLDiagnosticOwner
	Info      agreementNativeInfoOwner
	Paragraph agreementUnusedParagraphDiagnosticOwner
}

// Native discarded fields keep one footnote declaration even when both readers name it.
func agreementEscapedClosingDiagnosticPhysical(body string, check *agreementNativeCheckSource, tree *agreementNativeProseAddressTree, target string) ([]agreementEscapedClosingDiagnosticOwner, bool) {
	if target == "" || !tree.Valid {
		return nil, false
	}
	native := agreementNativeOwnerReading(body)
	code, cok := agreementNativeCodeDiagnosticPhysical(native, check, target)
	discarded, dok := agreementInfoCombinedDiagnosticNativeOwners(native, check, tree, target)
	html, hok := agreementHTMLDiagnosticPhysical(body, check, agreementNativeHTMLAddressSites(body), tree, target)
	info, iok := agreementInfoCombinedDiagnosticInfoOwners(native, check, agreementNativeCodePayloadSites(body), target)
	if !cok && !dok && !hok && !iok {
		if _, ok := agreementConsumedWordDiagnosticInactiveOwners(body, check, tree, target); !ok {
			return nil, false
		}
	}
	byField := map[int]agreementEscapedClosingDiagnosticOwner{}
	if cok {
		for _, o := range code {
			byField[o.Field] = agreementEscapedClosingDiagnosticOwner{Field: o.Field, Family: "native-code", Code: o}
		}
	}
	if dok {
		for _, o := range discarded {
			if _, seen := byField[o.Field]; seen {
				return nil, false
			}
			byField[o.Field] = agreementEscapedClosingDiagnosticOwner{Field: o.Field, Family: "native-discarded", Native: o}
		}
	}
	if hok {
		for _, o := range html {
			if _, seen := byField[o.Field]; seen {
				return nil, false
			}
			byField[o.Field] = agreementEscapedClosingDiagnosticOwner{Field: o.Field, Family: "html", HTML: o}
		}
	}
	if iok {
		for _, o := range info {
			if _, seen := byField[o.Field]; seen {
				return nil, false
			}
			byField[o.Field] = agreementEscapedClosingDiagnosticOwner{Field: o.Field, Family: "native-info", Info: o}
		}
	}

	paragraph, pok := agreementConsumedWordDiagnosticInactiveOwners(body, check, tree, target)
	if !pok {
		paragraph = nil
	}
	for oi := range paragraph {
		o := &paragraph[oi]
		if prior, seen := byField[o.Field]; seen {
			if prior.Family != "native-discarded" || prior.Native.Root != o.Root || prior.Native.Definition < 0 || prior.Native.Definition >= len(native.Definitions) || o.Definition < 0 || o.Definition >= len(tree.Definitions) || tree.Definitions[o.Definition].Bounds.Span != native.Definitions[prior.Native.Definition].Span || tree.Definitions[o.Definition].Bounds.Label != native.Definitions[prior.Native.Definition].Name {
				return nil, false
			}
			prior.Family = "native-discarded-paragraph"
			prior.Paragraph = *o
			byField[o.Field] = prior
		} else {
			byField[o.Field] = agreementEscapedClosingDiagnosticOwner{Field: o.Field, Family: "consumed-paragraph-word", Paragraph: *o}
		}
	}
	var owners []agreementEscapedClosingDiagnosticOwner
	for fi := range check.Fields {
		if o, ok := byField[fi]; ok {
			f := check.Fields[fi]
			if len(f.Targets) != 1 || f.Targets[0] != target || f.Comment || f.Escaped {
				return nil, false
			}
			owners = append(owners, o)
		}
	}
	return owners, len(owners) > 0
}

// A private rename retains every unselected physical field.
func agreementEscapedClosingDiagnosticPrivateSingle(body, rawTarget, target string, owner *agreementEscapedClosingDiagnosticOwner, check *agreementNativeCheckSource) (string, []agreementMixedRename, bool) {
	if rawTarget != target+"\\" || target == "" || strings.Contains(body, "q") || owner.Field < 0 || owner.Field >= len(check.Fields) {
		return "", nil, false
	}
	f := check.Fields[owner.Field]
	if f.Word != "[["+rawTarget+"]]" || f.Span.Start < 0 || f.Span.Stop > len(body) || f.Span.Stop-f.Span.Start != len(f.Word) || body[f.Span.Start:f.Span.Stop] != f.Word || len(f.Targets) != 1 || f.Targets[0] != target || f.Comment || f.Escaped {
		return "", nil, false
	}
	start := f.Span.Start + 2
	stop := start + len(target)
	if body[start:stop] != target {
		return "", nil, false
	}
	replacement := strings.Repeat("q", len(target))
	b := []byte(body)
	copy(b[start:stop], replacement)
	return string(b), []agreementMixedRename{{Field: owner.Field, Span: graph.Span{Start: start, Stop: stop}, Old: target, New: replacement}}, true
}
func agreementEscapedClosingDiagnosticPrivate(body, target string, owners []agreementEscapedClosingDiagnosticOwner, check *agreementNativeCheckSource) (string, []agreementMixedRename, bool) {
	if target == "" || len(owners) == 0 {
		return "", nil, false
	}
	tree := agreementNativeProseAddressTreeReading(body, nil)
	physical, ok := agreementEscapedClosingDiagnosticPhysical(body, check, &tree, target)
	if !ok {
		return "", nil, false
	}
	seen := map[int]bool{}
	b := []byte(body)
	changes := []agreementMixedRename{}
	for oi := range owners {
		o := owners[oi]
		if seen[o.Field] || !slices.Contains(physical, o) {
			return "", nil, false
		}
		seen[o.Field] = true
		_, one, valid := agreementEscapedClosingDiagnosticPrivateSingle(body, target+"\\", target, &o, check)
		if !valid || len(one) != 1 {
			return "", nil, false
		}
		c := one[0]
		copy(b[c.Span.Start:c.Span.Stop], c.New)
		changes = append(changes, c)
	}
	return string(b), changes, true
}

func agreementEscapedClosingDiagnosticActive(owners []agreementEscapedClosingDiagnosticOwner, check *agreementNativeCheckSource) int {
	count := 0
	for oi := range owners {
		o := &owners[oi]
		if o.Field < 0 || o.Field >= len(check.Fields) {
			return -1
		}
		if !check.Fields[o.Field].Code {
			count++
		}
	}
	return count
}

func agreementEscapedClosingDiagnosticProducers(t *testing.T, body, target string, native agreementNativeOwnerSource, check *agreementNativeCheckSource) (physicalFields, diagnosticFields []agreementEscapedClosingDiagnosticOwner, targetChanges []agreementMixedRename, projectedBody string, valid bool) {
	t.Helper()
	rawTarget := target + "\\"
	tree := agreementNativeProseAddressTreeReading(body, nil)
	physical, ok := agreementEscapedClosingDiagnosticPhysical(body, check, &tree, target)
	if !ok {
		return nil, nil, nil, "", false
	}
	raw, page := agreementIsolatedPage(t, agreementCase{Body: body})
	parts, pok := agreementCodeWindowParts(raw.HTML)
	if !pok {
		return nil, nil, nil, "", false
	}
	var owners []agreementEscapedClosingDiagnosticOwner
	var changes []agreementMixedRename
	whole := []byte(body)
	used := map[int]bool{}
	for oi := range physical {
		owner := &physical[oi]
		private, oneChanges, valid := agreementEscapedClosingDiagnosticPrivateSingle(body, rawTarget, target, owner, check)
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
		if a.Section != "" || a.Kind != render.DiagWikilinkBroken || a.Target != rawTarget || b.Target != change.New+"\\" || a.Kind != b.Kind || a.Section != b.Section || a.Block != b.Block || b.Message != strings.ReplaceAll(a.Message, strconv.Quote(rawTarget), strconv.Quote(change.New+"\\")) {
			continue
		}
		used[index] = true
		owners = append(owners, *owner)
		changes = append(changes, change)
		copy(whole[change.Span.Start:change.Span.Stop], change.New)
	}
	return physical, owners, changes, string(whole), len(owners) > 0
}

// Target-only edits retain public page facts and rebind excerpt payloads within the same source intervals.
func agreementEscapedClosingDiagnosticSignedProjection(before, after []agreementFailure, page *agreementHTML, targets []string, selected *agreementFailure, fields, active int, body, private string, changes []agreementMixedRename) bool {
	if !agreementEscapedClosingDiagnosticCutProjection(body, private, changes, before, after) {
		return false
	}
	cleanBefore := []agreementFailure{}
	cleanAfter := []agreementFailure{}
	for fi := range before {
		f := before[fi]
		if f.Property != "P3" {
			cleanBefore = append(cleanBefore, f)
		}
	}
	for fi := range after {
		f := after[fi]
		if f.Property != "P3" {
			cleanAfter = append(cleanAfter, f)
		}
	}
	before = cleanBefore
	after = cleanAfter
	if selected == nil || fields < 1 || active < 0 || active > fields || fields != selected.Multiplicity || selected.Tuple.Target == "" {
		return false
	}
	target := strings.TrimSuffix(selected.Tuple.Target, "\\")
	if target == selected.Tuple.Target || target == "" || selected.Tuple.Target != target+"\\" {
		return false
	}
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
	if counts[target] < active || counts[replacement] != 0 || visible[replacement] != 0 {
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
	renamed.Tuple.Target = replacement + "\\"
	expected[agreementSignature(&renamed)]++
	for key := range oldP1 {
		delete(expected, key)
	}
	if key := signed(target, counts[target]-active-visible[target]); key != "" {
		expected[key]++
	}
	if key := signed(replacement, active); key != "" {
		expected[key]++
	}
	return maps.Equal(expected, agreementMixedCurrent(after))
}

func agreementEscapedClosingDiagnosticCutProjection(body, private string, changes []agreementMixedRename, before, after []agreementFailure) bool {
	if len(body) != len(private) || len(changes) == 0 {
		return false
	}
	projected := []byte(body)
	last := 0
	seenFields := map[int]bool{}
	for _, change := range changes {
		a, b := change.Span.Start, change.Span.Stop
		if a < last || a < 0 || b > len(body) || b <= a || len(change.Old) != len(change.New) || body[a:b] != change.Old || seenFields[change.Field] {
			return false
		}
		seenFields[change.Field] = true
		last = b
		copy(projected[a:b], change.New)
	}
	if string(projected) != private {
		return false
	}
	expected := map[string]int{}
	for fi := range before {
		f := before[fi]
		if f.Property != "P3" {
			continue
		}
		if f.Cut == "" {
			expected[agreementSignature(&f)]++
			continue
		}
		if strings.Count(body, f.Cut) != 1 {
			return false
		}
		start := strings.Index(body, f.Cut)
		stop := start + len(f.Cut)
		cut := []byte(f.Cut)
		for _, change := range changes {
			a, b := change.Span.Start, change.Span.Stop
			if a < 0 || b > len(body) || b <= a || body[a:b] != change.Old || private[a:b] != change.New || len(change.Old) != len(change.New) {
				return false
			}
			if a >= stop || b <= start {
				continue
			}
			if a < start || b > stop {
				return false
			}
			copy(cut[a-start:b-start], change.New)
		}
		f.Cut = string(cut)
		expected[agreementSignature(&f)]++
	}
	actual := map[string]int{}
	for fi := range after {
		f := after[fi]
		if f.Property == "P3" {
			actual[agreementSignature(&f)]++
		}
	}
	return maps.Equal(expected, actual)
}

func agreementEscapedClosingDiagnosticSource(body, target string) (*agreementEscapedClosingDiagnosticSourceWitness, *agreementEscapedClosingDiagnosticVariant, bool) {
	witnesses := agreementEscapedClosingDiagnosticSources()
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

// Every diagnostic binds a complete physical field while excerpt cuts retain their original source intervals.
func agreementEscapedClosingDiagnosticDifference(c agreementCase, f *agreementFailure, raw string, page *agreementHTML, diagnostics []render.Diagnostic, failures []agreementFailure) (kind, authority, wrong string) {
	if f == nil || page == nil || c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Multiplicity < 1 || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Tuple.Section != "" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound {
		return "", "", ""
	}
	records := agreementEscapedClosingDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		if c.Body != r.Body || f.Tuple.Target != r.RawTarget {
			continue
		}
		if agreementSignature(f) != r.Selected || !cmp.Equal(diagnostics, r.Diagnostics) {
			return "", "", ""
		}
		w, v, found := agreementEscapedClosingDiagnosticSource(c.Body, r.Target)
		if !found {
			return "", "", ""
		}
		native := agreementNativeOwnerReading(c.Body)
		check := agreementNativeCheckReading(c.Body, native)
		tree := agreementNativeProseAddressTreeReading(c.Body, nil)
		parts, pok := agreementCodeWindowParts(raw)
		physical, ok := agreementEscapedClosingDiagnosticPhysical(c.Body, &check, &tree, r.Target)
		private, changes, valid := agreementEscapedClosingDiagnosticPrivate(c.Body, r.Target, v.Owners, &check)
		if !pok || !ok || !valid || len(v.Owners) != f.Multiplicity || len(r.Indices) != len(v.Owners) || !cmp.Equal(physical, v.Physical) || !cmp.Equal(changes, v.Changes) || private != v.Body || !cmp.Equal(native, w.Native) || !cmp.Equal(check, w.Check) || !cmp.Equal(tree, w.Tree) || !cmp.Equal(agreementRemovedCommentDiagnosticRead(c.Body), w.Removed) || !cmp.Equal(agreementNativeCodePayloadSites(c.Body), w.Sites) || !cmp.Equal(agreementNativeHTMLAddressSites(c.Body), w.HTML) || !cmp.Equal(judge.LinkTargets(c.Body), w.Targets) || !cmp.Equal(parts, w.Parts) || !cmp.Equal(*page, w.Page) || !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
			return "", "", ""
		}
		return "debt", "#1011 stage 5", "page-diagnostic"
	}
	return "", "", ""
}

type agreementEscapedClosingDiagnosticVariant struct {
	Target, Body     string
	Physical, Owners []agreementEscapedClosingDiagnosticOwner
	Changes          []agreementMixedRename
	Check            agreementNativeCheckSource
	Targets          []string
	Signatures       map[string]int
	Removed          agreementRemovedCommentDiagnosticReading
}
type agreementEscapedClosingDiagnosticSourceWitness struct {
	Name       string
	Body       string
	Native     agreementNativeOwnerSource
	Check      agreementNativeCheckSource
	Sites      []agreementNativeCodePayloadSite
	HTML       []agreementNativeHTMLAddressSite
	Tree       agreementNativeProseAddressTree
	Removed    agreementRemovedCommentDiagnosticReading
	Page       agreementHTML
	Parts      [][]agreementCodePart
	Targets    []string
	Signatures map[string]int
	Selected   map[string]int
	Variants   []agreementEscapedClosingDiagnosticVariant
}

func agreementEscapedClosingDiagnosticSources() []agreementEscapedClosingDiagnosticSourceWitness {
	const frozen = `[{"Name":"c41d987b2fa8","Body":"[^unused]: [[A]]\nB\n[[A\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~\n","Native":{"Definitions":[{"Label":"unused","Parent":"Document","Span":{"Start":0,"Stop":184},"Name":{"Start":0,"Stop":10},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":11,"Stop":17},{"Start":17,"Stop":19},{"Start":19,"Stop":29},{"Start":29,"Stop":61},{"Start":61,"Stop":91},{"Start":91,"Stop":116},{"Start":116,"Stop":120},{"Start":120,"Stop":133},{"Start":133,"Stop":147},{"Start":147,"Stop":153},{"Start":153,"Stop":164},{"Start":164,"Stop":183}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":11,"Stop":12}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":12,"Stop":13}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":13,"Stop":15}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":15,"Stop":16}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":20}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":20,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":25}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":25,"Stop":28}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":29,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":54}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":54,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":63}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":63,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":87}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":91}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":111}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":115,"Stop":115}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":116,"Stop":117}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":117,"Stop":118}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":118,"Stop":119}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":120,"Stop":123}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":123,"Stop":128}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":128,"Stop":129}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":129,"Stop":131}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":131,"Stop":132}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":133,"Stop":134}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":134,"Stop":135}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":135,"Stop":141}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":142,"Stop":147}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":147,"Stop":153}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":153,"Stop":158}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":159,"Stop":163}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":164,"Stop":167}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":167,"Stop":168}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":168,"Stop":174}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":174,"Stop":175}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":175,"Stop":182}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":182,"Stop":183}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":91,"Stop":111}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":142,"Stop":158}}],"Comments":null},"Check":{"Code":[{"Start":91,"Stop":111},{"Start":142,"Stop":158}],"Comments":null,"Fields":[{"Span":{"Start":11,"Stop":16},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":19,"Stop":25},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":62,"Stop":75},"Word":"[[image.png]]","Targets":["image.png"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":83},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":91,"Stop":103},"Word":"[[A\\|alias]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":103,"Stop":109},"Word":"[[#A]]","Targets":[],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":116,"Stop":123},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":127,"Stop":132},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":133,"Stop":141},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":147,"Stop":152},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":166,"Stop":173},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":173,"Stop":180},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1}],"Targets":["A","A","image.png","A","A","A","A","A"]},"Sites":[{"Root":1,"Node":19,"Parent":1,"Kind":"CodeSpan","ParentKind":"Paragraph","Lines":null,"Text":[{"Start":91,"Stop":91,"Padding":0,"ForceNewline":false},{"Start":91,"Stop":111,"Padding":0,"ForceNewline":false}],"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasInfo":false,"Raw":false},{"Root":1,"Node":34,"Parent":1,"Kind":"CodeSpan","ParentKind":"Paragraph","Lines":null,"Text":[{"Start":142,"Stop":147,"Padding":0,"ForceNewline":false},{"Start":147,"Stop":153,"Padding":0,"ForceNewline":false},{"Start":153,"Stop":158,"Padding":0,"ForceNewline":false}],"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasInfo":false,"Raw":false}],"HTML":null,"Tree":{"Trees":[[{"Kind":"Document","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false}],[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":11,"Stop":17},{"Start":17,"Stop":19},{"Start":19,"Stop":29},{"Start":29,"Stop":61},{"Start":61,"Stop":91},{"Start":91,"Stop":116},{"Start":116,"Stop":120},{"Start":120,"Stop":133},{"Start":133,"Stop":147},{"Start":147,"Stop":153},{"Start":153,"Stop":164},{"Start":164,"Stop":183}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":11,"Stop":12}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":12,"Stop":13}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":13,"Stop":15}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":15,"Stop":16}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":20}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":20,"Stop":21}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":25}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":25,"Stop":28}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":29,"Stop":48}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":54}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":54,"Stop":60}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":63}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":63,"Stop":64}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":77}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":87}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":91}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":111}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":115,"Stop":115}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":116,"Stop":117}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":117,"Stop":118}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":118,"Stop":119}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":120,"Stop":123}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":123,"Stop":128}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":128,"Stop":129}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":129,"Stop":131}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":131,"Stop":132}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":133,"Stop":134}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":134,"Stop":135}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":135,"Stop":141}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":142,"Stop":147}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":147,"Stop":153}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":153,"Stop":158}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":159,"Stop":163}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":164,"Stop":167}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":167,"Stop":168}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":168,"Stop":174}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":174,"Stop":175}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":175,"Stop":182}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":182,"Stop":183}],"Raw":false,"Soft":false,"Hard":false}]],"Definitions":[{"Root":1,"Node":0,"Label":"unused","Bounds":{"Span":{"Start":0,"Stop":184},"Label":{"Start":0,"Stop":10},"Parent":"Document"},"Found":true}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":91,"Stop":111}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":142,"Stop":158}}],"Comments":null,"Unanchorable":null,"Valid":true},"Removed":{"Native":{"Definitions":[{"Label":"unused","Parent":"Document","Span":{"Start":0,"Stop":184},"Name":{"Start":0,"Stop":10},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":11,"Stop":17},{"Start":17,"Stop":19},{"Start":19,"Stop":29},{"Start":29,"Stop":61},{"Start":61,"Stop":91},{"Start":91,"Stop":116},{"Start":116,"Stop":120},{"Start":120,"Stop":133},{"Start":133,"Stop":147},{"Start":147,"Stop":153},{"Start":153,"Stop":164},{"Start":164,"Stop":183}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":11,"Stop":12}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":12,"Stop":13}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":13,"Stop":15}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":15,"Stop":16}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":20}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":20,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":25}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":25,"Stop":28}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":29,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":54}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":54,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":63}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":63,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":87}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":91}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":111}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":115,"Stop":115}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":116,"Stop":117}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":117,"Stop":118}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":118,"Stop":119}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":120,"Stop":123}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":123,"Stop":128}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":128,"Stop":129}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":129,"Stop":131}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":131,"Stop":132}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":133,"Stop":134}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":134,"Stop":135}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":135,"Stop":141}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":142,"Stop":147}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":147,"Stop":153}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":153,"Stop":158}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":159,"Stop":163}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":164,"Stop":167}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":167,"Stop":168}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":168,"Stop":174}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":174,"Stop":175}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":175,"Stop":182}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":182,"Stop":183}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":91,"Stop":111}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":142,"Stop":158}}],"Comments":null},"Check":{"Code":[{"Start":91,"Stop":111},{"Start":142,"Stop":158}],"Comments":null,"Fields":[{"Span":{"Start":11,"Stop":16},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":19,"Stop":25},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":62,"Stop":75},"Word":"[[image.png]]","Targets":["image.png"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":83},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":91,"Stop":103},"Word":"[[A\\|alias]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":103,"Stop":109},"Word":"[[#A]]","Targets":[],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":116,"Stop":123},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":127,"Stop":132},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":133,"Stop":141},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":147,"Stop":152},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":166,"Stop":173},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":173,"Stop":180},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1}],"Targets":["A","A","image.png","A","A","A","A","A"]},"Projected":"","Positions":null,"ProjectedNative":{"Definitions":null,"Code":null,"Comments":null},"ProjectedCheck":{"Code":null,"Comments":null,"Fields":null,"Targets":null},"ProjectedTree":{"Trees":null,"Definitions":null,"Code":null,"Comments":null,"Unanchorable":null,"Valid":false},"Valid":false},"Page":{"Citations":null,"CitationsInCode":0,"Blocks":null,"Headings":null,"Failures":null,"CodeCitations":null,"CalloutTitles":null},"Parts":null,"Targets":["A","A","image.png","A","A","A","A","A"],"Signatures":{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=5 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=3 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=7 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^unused]: [[A]]\\nB\\n[[A\\\\]] ^A\\n^absent-prose - > [!note] title\\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\\n[[A\\\\|alias]][[#A]]   \u0060\u0060\u0060\\n[[A\\nB]]\u0060\u0060\u0060 [[A]]\\n[[A#^a]]\u0060open\\n[[A]]\\nclose\u0060 ^é\\n\u0060![[A#A]][[A#A]]~~~\" page=false judge=true excerpt=true":1,"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^unused]: [[A]]\\nB\\n[[A\\\\]] ^A\\n^absent-prose - > [!note] title\\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\\n[[A\\\\|alias]][[#A]]   \u0060\u0060\u0060\\n[[A\\nB]]\u0060\u0060\u0060 [[A]]\\n[[A#^a]]\u0060open\\n[[A]]\\nclose\u0060 ^é\\n\u0060![[A#A]][[A#A]]~~~\" page=false judge=true excerpt=true":1},"Selected":{"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1},"Variants":[{"Target":"A","Body":"[^unused]: [[A]]\nB\n[[q\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~\n","Physical":[{"Field":0,"Family":"native-discarded-paragraph","Code":{"Field":0,"Code":0},"Native":{"Field":0,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":1,"Node":1,"Line":0,"Label":"unused"}},{"Field":1,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":1,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":3,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":3,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":4,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":4,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":7,"Family":"native-discarded-paragraph","Code":{"Field":0,"Code":0},"Native":{"Field":7,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":7,"Definition":0,"Root":1,"Node":1,"Line":7,"Label":"unused"}},{"Field":8,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":8,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":9,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":9,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":10,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":10,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":11,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":11,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}}],"Owners":[{"Field":1,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":1,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}}],"Changes":[{"Field":1,"Span":{"Start":21,"Stop":22},"Old":"A","New":"q"}],"Check":{"Code":[{"Start":91,"Stop":111},{"Start":142,"Stop":158}],"Comments":null,"Fields":[{"Span":{"Start":11,"Stop":16},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":19,"Stop":25},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":62,"Stop":75},"Word":"[[image.png]]","Targets":["image.png"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":83},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":91,"Stop":103},"Word":"[[A\\|alias]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":103,"Stop":109},"Word":"[[#A]]","Targets":[],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":116,"Stop":123},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":127,"Stop":132},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":133,"Stop":141},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":147,"Stop":152},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":166,"Stop":173},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":173,"Stop":180},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1}],"Targets":["A","q","image.png","A","A","A","A","A"]},"Targets":["A","q","image.png","A","A","A","A","A"],"Signatures":{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=5 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:A Section:A State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=3 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:image.png Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:q\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=6 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:image.png Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:q Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^unused]: [[A]]\\nB\\n[[q\\\\]] ^A\\n^absent-prose - > [!note] title\\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\\n[[A\\\\|alias]][[#A]]   \u0060\u0060\u0060\\n[[A\\nB]]\u0060\u0060\u0060 [[A]]\\n[[A#^a]]\u0060open\\n[[A]]\\nclose\u0060 ^é\\n\u0060![[A#A]][[A#A]]~~~\" page=false judge=true excerpt=true":1,"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^unused]: [[A]]\\nB\\n[[q\\\\]] ^A\\n^absent-prose - > [!note] title\\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\\n[[A\\\\|alias]][[#A]]   \u0060\u0060\u0060\\n[[A\\nB]]\u0060\u0060\u0060 [[A]]\\n[[A#^a]]\u0060open\\n[[A]]\\nclose\u0060 ^é\\n\u0060![[A#A]][[A#A]]~~~\" page=false judge=true excerpt=true":1},"Removed":{"Native":{"Definitions":[{"Label":"unused","Parent":"Document","Span":{"Start":0,"Stop":184},"Name":{"Start":0,"Stop":10},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":11,"Stop":17},{"Start":17,"Stop":19},{"Start":19,"Stop":29},{"Start":29,"Stop":61},{"Start":61,"Stop":91},{"Start":91,"Stop":116},{"Start":116,"Stop":120},{"Start":120,"Stop":133},{"Start":133,"Stop":147},{"Start":147,"Stop":153},{"Start":153,"Stop":164},{"Start":164,"Stop":183}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":11,"Stop":12}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":12,"Stop":13}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":13,"Stop":15}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":15,"Stop":16}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":20}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":20,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":25}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":25,"Stop":28}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":29,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":54}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":54,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":63}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":63,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":87}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":91}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":111}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":115,"Stop":115}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":116,"Stop":117}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":117,"Stop":118}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":118,"Stop":119}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":120,"Stop":123}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":123,"Stop":128}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":128,"Stop":129}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":129,"Stop":131}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":131,"Stop":132}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":133,"Stop":134}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":134,"Stop":135}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":135,"Stop":141}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":142,"Stop":147}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":147,"Stop":153}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":153,"Stop":158}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":159,"Stop":163}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":164,"Stop":167}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":167,"Stop":168}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":168,"Stop":174}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":174,"Stop":175}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":175,"Stop":182}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":182,"Stop":183}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":91,"Stop":111}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":142,"Stop":158}}],"Comments":null},"Check":{"Code":[{"Start":91,"Stop":111},{"Start":142,"Stop":158}],"Comments":null,"Fields":[{"Span":{"Start":11,"Stop":16},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":19,"Stop":25},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":62,"Stop":75},"Word":"[[image.png]]","Targets":["image.png"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":83},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":91,"Stop":103},"Word":"[[A\\|alias]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":103,"Stop":109},"Word":"[[#A]]","Targets":[],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":116,"Stop":123},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":127,"Stop":132},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":133,"Stop":141},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":147,"Stop":152},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":166,"Stop":173},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":173,"Stop":180},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1}],"Targets":["A","q","image.png","A","A","A","A","A"]},"Projected":"","Positions":null,"ProjectedNative":{"Definitions":null,"Code":null,"Comments":null},"ProjectedCheck":{"Code":null,"Comments":null,"Fields":null,"Targets":null},"ProjectedTree":{"Trees":null,"Definitions":null,"Code":null,"Comments":null,"Unanchorable":null,"Valid":false},"Valid":false}}]},{"Name":"0af16098b14c","Body":"[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[A\\]]\u0060[[A]]\u0060   \u0060\u0060\u0060\n[[A#^a]][[B|alias]]章節\n\u0060\u0060\u0060\u0060\n\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":0,"Stop":94},"Name":{"Start":0,"Stop":5},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":6,"Stop":11}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":6,"Stop":7}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":7,"Stop":8}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":8,"Stop":10}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":10,"Stop":11}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":17,"Stop":23},{"Start":27,"Stop":48},{"Start":48,"Stop":68},{"Start":68,"Stop":93}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":18,"Stop":19}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":22}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":27,"Stop":35}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":35,"Stop":41}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":41,"Stop":47}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":50}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":50,"Stop":54}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":55,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":67,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":68,"Stop":69}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":69,"Stop":70}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":70,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":86}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":86,"Stop":93}]}]}],"Code":[{"Kind":"FencedCodeBlock","Parent":"Document","Span":{"Start":99,"Stop":172}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":55,"Stop":60}}],"Comments":null},"Check":{"Code":[{"Start":17,"Stop":48},{"Start":55,"Stop":60},{"Start":99,"Stop":172}],"Comments":null,"Fields":[{"Span":{"Start":6,"Stop":11},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":17,"Stop":22},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":48,"Stop":54},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":55,"Stop":60},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":68,"Stop":76},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":87},"Word":"[[B|alias]]","Targets":["B"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":101,"Stop":106},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":137,"Stop":142},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":155,"Stop":160},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0}],"Targets":["A","A","A","B"]},"Sites":[{"Root":0,"Node":1,"Parent":0,"Kind":"FencedCodeBlock","ParentKind":"Document","Lines":[{"Start":99,"Stop":121,"Padding":0,"ForceNewline":true},{"Start":121,"Stop":129,"Padding":0,"ForceNewline":true},{"Start":129,"Stop":143,"Padding":0,"ForceNewline":true},{"Start":143,"Stop":145,"Padding":0,"ForceNewline":true},{"Start":145,"Stop":146,"Padding":0,"ForceNewline":true},{"Start":146,"Stop":147,"Padding":0,"ForceNewline":true},{"Start":147,"Stop":155,"Padding":0,"ForceNewline":true},{"Start":155,"Stop":161,"Padding":0,"ForceNewline":true},{"Start":161,"Stop":172,"Padding":0,"ForceNewline":true}],"Text":null,"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasInfo":false,"Raw":true},{"Root":1,"Node":17,"Parent":6,"Kind":"CodeSpan","ParentKind":"Paragraph","Lines":null,"Text":[{"Start":55,"Stop":60,"Padding":0,"ForceNewline":false}],"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasInfo":false,"Raw":false}],"HTML":[{"Root":0,"Node":1,"Parent":0,"Kind":"FencedCodeBlock","ParentKind":"Document","Type":0,"Lines":[{"Start":99,"Stop":121,"Padding":0,"ForceNewline":true},{"Start":121,"Stop":129,"Padding":0,"ForceNewline":true},{"Start":129,"Stop":143,"Padding":0,"ForceNewline":true},{"Start":143,"Stop":145,"Padding":0,"ForceNewline":true},{"Start":145,"Stop":146,"Padding":0,"ForceNewline":true},{"Start":146,"Stop":147,"Padding":0,"ForceNewline":true},{"Start":147,"Stop":155,"Padding":0,"ForceNewline":true},{"Start":155,"Stop":161,"Padding":0,"ForceNewline":true},{"Start":161,"Stop":172,"Padding":0,"ForceNewline":true}],"Closure":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasClosure":false,"HasInfo":false,"Raw":true}],"Tree":{"Trees":[[{"Kind":"Document","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"FencedCodeBlock","Depth":1,"Lines":[{"Start":99,"Stop":121},{"Start":121,"Stop":129},{"Start":129,"Stop":143},{"Start":143,"Stop":145},{"Start":145,"Stop":146},{"Start":146,"Stop":147},{"Start":147,"Stop":155},{"Start":155,"Stop":161},{"Start":161,"Stop":172}],"Text":null,"Raw":false,"Soft":false,"Hard":false}],[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":6,"Stop":11}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":6,"Stop":7}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":7,"Stop":8}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":8,"Stop":10}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":10,"Stop":11}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":17,"Stop":23},{"Start":27,"Stop":48},{"Start":48,"Stop":68},{"Start":68,"Stop":93}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":18,"Stop":19}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":21}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":22}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":27,"Stop":35}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":35,"Stop":41}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":41,"Stop":47}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":50}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":50,"Stop":54}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":55,"Stop":60}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":64}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":67}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":67,"Stop":67}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":68,"Stop":69}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":69,"Stop":70}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":70,"Stop":77}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":86}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":86,"Stop":93}],"Raw":false,"Soft":false,"Hard":false}]],"Definitions":[{"Root":1,"Node":0,"Label":"n","Bounds":{"Span":{"Start":0,"Stop":94},"Label":{"Start":0,"Stop":5},"Parent":"Document"},"Found":true}],"Code":[{"Kind":"FencedCodeBlock","Parent":"Document","Span":{"Start":99,"Stop":172}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":55,"Stop":60}}],"Comments":null,"Unanchorable":null,"Valid":true},"Removed":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":0,"Stop":94},"Name":{"Start":0,"Stop":5},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":6,"Stop":11}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":6,"Stop":7}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":7,"Stop":8}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":8,"Stop":10}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":10,"Stop":11}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":17,"Stop":23},{"Start":27,"Stop":48},{"Start":48,"Stop":68},{"Start":68,"Stop":93}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":18,"Stop":19}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":22}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":27,"Stop":35}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":35,"Stop":41}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":41,"Stop":47}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":50}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":50,"Stop":54}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":55,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":67,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":68,"Stop":69}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":69,"Stop":70}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":70,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":86}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":86,"Stop":93}]}]}],"Code":[{"Kind":"FencedCodeBlock","Parent":"Document","Span":{"Start":99,"Stop":172}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":55,"Stop":60}}],"Comments":null},"Check":{"Code":[{"Start":17,"Stop":48},{"Start":55,"Stop":60},{"Start":99,"Stop":172}],"Comments":null,"Fields":[{"Span":{"Start":6,"Stop":11},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":17,"Stop":22},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":48,"Stop":54},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":55,"Stop":60},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":68,"Stop":76},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":87},"Word":"[[B|alias]]","Targets":["B"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":101,"Stop":106},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":137,"Stop":142},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":155,"Stop":160},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0}],"Targets":["A","A","A","B"]},"Projected":"","Positions":null,"ProjectedNative":{"Definitions":null,"Code":null,"Comments":null},"ProjectedCheck":{"Code":null,"Comments":null,"Fields":null,"Targets":null},"ProjectedTree":{"Trees":null,"Definitions":null,"Code":null,"Comments":null,"Unanchorable":null,"Valid":false},"Valid":false},"Page":{"Citations":null,"CitationsInCode":0,"Blocks":null,"Headings":null,"Failures":null,"CodeCitations":null,"CalloutTitles":null},"Parts":[[{"Text":"\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","Citation":{"SourceRole":"","Target":"","Section":"","State":""},"Field":false}]],"Targets":["A","A","A","B"],"Signatures":{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=3 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1},"Selected":{"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1},"Variants":[{"Target":"A","Body":"[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[q\\]]\u0060[[A]]\u0060   \u0060\u0060\u0060\n[[A#^a]][[B|alias]]章節\n\u0060\u0060\u0060\u0060\n\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","Physical":[{"Field":0,"Family":"native-discarded-paragraph","Code":{"Field":0,"Code":0},"Native":{"Field":0,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":1,"Node":1,"Line":0,"Label":"n"}},{"Field":2,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":2,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":3,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":3,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":4,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":4,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":6,"Family":"native-code","Code":{"Field":6,"Code":0},"Native":{"Field":0,"Definition":0,"Root":0,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":7,"Family":"native-code","Code":{"Field":7,"Code":0},"Native":{"Field":0,"Definition":0,"Root":0,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":8,"Family":"native-code","Code":{"Field":8,"Code":0},"Native":{"Field":0,"Definition":0,"Root":0,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}}],"Owners":[{"Field":2,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":2,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}}],"Changes":[{"Field":2,"Span":{"Start":50,"Stop":51},"Old":"A","New":"q"}],"Check":{"Code":[{"Start":17,"Stop":48},{"Start":55,"Stop":60},{"Start":99,"Stop":172}],"Comments":null,"Fields":[{"Span":{"Start":6,"Stop":11},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":17,"Stop":22},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":48,"Stop":54},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":55,"Stop":60},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":68,"Stop":76},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":87},"Word":"[[B|alias]]","Targets":["B"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":101,"Stop":106},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":137,"Stop":142},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":155,"Stop":160},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0}],"Targets":["A","q","A","B"]},"Targets":["A","q","A","B"],"Signatures":{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:q\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:B Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:q Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1},"Removed":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":0,"Stop":94},"Name":{"Start":0,"Stop":5},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":6,"Stop":11}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":6,"Stop":7}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":7,"Stop":8}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":8,"Stop":10}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":10,"Stop":11}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":17,"Stop":23},{"Start":27,"Stop":48},{"Start":48,"Stop":68},{"Start":68,"Stop":93}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":18,"Stop":19}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":22}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":27,"Stop":35}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":35,"Stop":41}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":41,"Stop":47}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":50}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":50,"Stop":54}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":55,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":67,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":68,"Stop":69}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":69,"Stop":70}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":70,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":86}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":86,"Stop":93}]}]}],"Code":[{"Kind":"FencedCodeBlock","Parent":"Document","Span":{"Start":99,"Stop":172}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":55,"Stop":60}}],"Comments":null},"Check":{"Code":[{"Start":17,"Stop":48},{"Start":55,"Stop":60},{"Start":99,"Stop":172}],"Comments":null,"Fields":[{"Span":{"Start":6,"Stop":11},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":17,"Stop":22},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":48,"Stop":54},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":55,"Stop":60},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":68,"Stop":76},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":87},"Word":"[[B|alias]]","Targets":["B"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":101,"Stop":106},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":137,"Stop":142},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":155,"Stop":160},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0}],"Targets":["A","q","A","B"]},"Projected":"","Positions":null,"ProjectedNative":{"Definitions":null,"Code":null,"Comments":null},"ProjectedCheck":{"Code":null,"Comments":null,"Fields":null,"Targets":null},"ProjectedTree":{"Trees":null,"Definitions":null,"Code":null,"Comments":null,"Unanchorable":null,"Valid":false},"Valid":false}}]},{"Name":"1206e9dd8d0e","Body":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[A\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n","Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":30,"Stop":164},"Name":{"Start":30,"Stop":35},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}]},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":[{"Start":180,"Stop":194}]},"Check":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":[{"Start":180,"Stop":194}],"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":185,"Stop":190},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":true,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1},{"Span":{"Start":196,"Stop":201},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","A","A"]},"Sites":[{"Root":1,"Node":13,"Parent":6,"Kind":"CodeSpan","ParentKind":"Paragraph","Lines":null,"Text":[{"Start":86,"Stop":91,"Padding":0,"ForceNewline":false}],"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasInfo":false,"Raw":false},{"Root":1,"Node":25,"Parent":6,"Kind":"CodeSpan","ParentKind":"Paragraph","Lines":null,"Text":[{"Start":138,"Stop":143,"Padding":0,"ForceNewline":false}],"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasInfo":false,"Raw":false}],"HTML":[{"Root":0,"Node":16,"Parent":0,"Kind":"HTMLBlock","ParentKind":"Document","Type":2,"Lines":[{"Start":180,"Stop":205,"Padding":0,"ForceNewline":false}],"Closure":{"Start":-1,"Stop":-1,"Padding":0,"ForceNewline":false},"Info":{"Start":0,"Stop":0,"Padding":0,"ForceNewline":false},"HasClosure":false,"HasInfo":false,"Raw":true}],"Tree":{"Trees":[[{"Kind":"Document","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Heading","Depth":1,"Lines":[{"Start":3,"Stop":4}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":3,"Stop":4}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"List","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"ListItem","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"TextBlock","Depth":3,"Lines":[{"Start":7,"Stop":11}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":4,"Lines":null,"Text":[{"Start":7,"Stop":11}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Blockquote","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":2,"Lines":[{"Start":23,"Stop":26},{"Start":27,"Stop":29}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":23,"Stop":25}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":27,"Stop":29}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Blockquote","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":2,"Lines":[{"Start":166,"Stop":179}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":166,"Stop":167}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":167,"Stop":173}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":173,"Stop":179}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"HTMLBlock","Depth":1,"Lines":[{"Start":180,"Stop":205}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Heading","Depth":1,"Lines":[{"Start":208,"Stop":209}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":208,"Stop":209}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":210,"Stop":212},{"Start":213,"Stop":215}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":210,"Stop":211}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":213,"Stop":215}],"Raw":false,"Soft":false,"Hard":false}],[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}],"Raw":false,"Soft":false,"Hard":false}]],"Definitions":[{"Root":1,"Node":0,"Label":"n","Bounds":{"Span":{"Start":30,"Stop":164},"Label":{"Start":30,"Stop":35},"Parent":"Document"},"Found":true}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":[{"Start":180,"Stop":194}],"Unanchorable":null,"Valid":true},"Removed":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":30,"Stop":164},"Name":{"Start":30,"Stop":35},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}]},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":[{"Start":180,"Stop":194}]},"Check":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":[{"Start":180,"Stop":194}],"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":185,"Stop":190},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":true,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1},{"Span":{"Start":196,"Stop":201},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","A","A"]},"Projected":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[A\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n\\\\[[A]]É\n## !\nA\n ^a\n","Positions":[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,77,78,79,80,81,82,83,84,85,86,87,88,89,90,91,92,93,94,95,96,97,98,99,100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,120,121,122,123,124,125,126,127,128,129,130,131,132,133,134,135,136,137,138,139,140,141,142,143,144,145,146,147,148,149,150,151,152,153,154,155,156,157,158,159,160,161,162,163,164,165,166,167,168,169,170,171,172,173,174,175,176,177,178,179,194,195,196,197,198,199,200,201,202,203,204,205,206,207,208,209,210,211,212,213,214,215],"ProjectedNative":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":30,"Stop":164},"Name":{"Start":30,"Stop":35},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}]},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":null},"ProjectedCheck":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":null,"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":182,"Stop":187},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","A","A"]},"ProjectedTree":{"Trees":[[{"Kind":"Document","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Heading","Depth":1,"Lines":[{"Start":3,"Stop":4}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":3,"Stop":4}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"List","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"ListItem","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"TextBlock","Depth":3,"Lines":[{"Start":7,"Stop":11}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":4,"Lines":null,"Text":[{"Start":7,"Stop":11}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Blockquote","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":2,"Lines":[{"Start":23,"Stop":26},{"Start":27,"Stop":29}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":23,"Stop":25}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":27,"Stop":29}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Blockquote","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":2,"Lines":[{"Start":166,"Stop":180},{"Start":180,"Stop":190}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":166,"Stop":167}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":167,"Stop":173}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":173,"Stop":179}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":180,"Stop":183}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":183,"Stop":184}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":184,"Stop":186}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":186,"Stop":190}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Heading","Depth":1,"Lines":[{"Start":194,"Stop":195}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":194,"Stop":195}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":196,"Stop":198},{"Start":199,"Stop":201}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":196,"Stop":197}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":199,"Stop":201}],"Raw":false,"Soft":false,"Hard":false}],[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}],"Raw":false,"Soft":false,"Hard":false}]],"Definitions":[{"Root":1,"Node":0,"Label":"n","Bounds":{"Span":{"Start":30,"Stop":164},"Label":{"Start":30,"Stop":35},"Parent":"Document"},"Found":true}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":null,"Unanchorable":null,"Valid":true},"Valid":true},"Page":{"Citations":[{"SourceRole":"","Target":"A","Section":"","State":"wikilink-broken"}],"CitationsInCode":0,"Blocks":["^a"],"Headings":["section","section-2"],"Failures":null,"CodeCitations":null,"CalloutTitles":["title"]},"Parts":null,"Targets":["A","A","A"],"Signatures":{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false":1,"P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false":1},"Selected":{"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1},"Variants":[{"Target":"A","Body":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[q\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n","Physical":[{"Field":0,"Family":"native-discarded-paragraph","Code":{"Field":0,"Code":0},"Native":{"Field":0,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":1,"Node":1,"Line":0,"Label":"n"}},{"Field":2,"Family":"native-discarded-paragraph","Code":{"Field":0,"Code":0},"Native":{"Field":2,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":2,"Definition":0,"Root":1,"Node":6,"Line":1,"Label":"n"}},{"Field":3,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":3,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":5,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":5,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":6,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":6,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}},{"Field":8,"Family":"html","Code":{"Field":0,"Code":0},"Native":{"Field":0,"Definition":0,"Root":0,"Node":0},"HTML":{"Field":8,"Site":0,"Line":0,"Root":0,"Node":16,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}}],"Owners":[{"Field":5,"Family":"native-discarded","Code":{"Field":0,"Code":0},"Native":{"Field":5,"Definition":0,"Root":1,"Node":0},"HTML":{"Field":0,"Site":0,"Line":0,"Root":0,"Node":0,"Closure":false},"Info":{"Field":0,"Code":0,"Site":0,"Root":0,"Node":0},"Paragraph":{"Field":0,"Definition":0,"Root":0,"Node":0,"Line":0,"Label":""}}],"Changes":[{"Field":5,"Span":{"Start":101,"Stop":102},"Old":"A","New":"q"}],"Check":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":[{"Start":180,"Stop":194}],"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":185,"Stop":190},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":true,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1},{"Span":{"Start":196,"Stop":201},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","q","A"]},"Targets":["A","q","A"],"Signatures":{"P0/diagnostic-html tuple={SourceRole: Target:A Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:B Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P0/diagnostic-html tuple={SourceRole: Target:q\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:A Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P1/citation-occurrences tuple={SourceRole: Target:q Section: State:} fragment=\"\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false":1,"P4/literal-heading-id tuple={SourceRole: Target: Section: State:} fragment=\"section-2\" direction=page-only multiplicity=1 cut=\"\" page=true judge=false excerpt=false":1},"Removed":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":30,"Stop":164},"Name":{"Start":30,"Stop":35},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}]},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":[{"Start":180,"Stop":194}]},"Check":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":[{"Start":180,"Stop":194}],"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":185,"Stop":190},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":true,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1},{"Span":{"Start":196,"Stop":201},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","q","A"]},"Projected":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[q\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n\\\\[[A]]É\n## !\nA\n ^a\n","Positions":[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,77,78,79,80,81,82,83,84,85,86,87,88,89,90,91,92,93,94,95,96,97,98,99,100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,120,121,122,123,124,125,126,127,128,129,130,131,132,133,134,135,136,137,138,139,140,141,142,143,144,145,146,147,148,149,150,151,152,153,154,155,156,157,158,159,160,161,162,163,164,165,166,167,168,169,170,171,172,173,174,175,176,177,178,179,194,195,196,197,198,199,200,201,202,203,204,205,206,207,208,209,210,211,212,213,214,215],"ProjectedNative":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":30,"Stop":164},"Name":{"Start":30,"Stop":35},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}]},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":null},"ProjectedCheck":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":null,"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":182,"Stop":187},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","q","A"]},"ProjectedTree":{"Trees":[[{"Kind":"Document","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Heading","Depth":1,"Lines":[{"Start":3,"Stop":4}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":3,"Stop":4}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"List","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"ListItem","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"TextBlock","Depth":3,"Lines":[{"Start":7,"Stop":11}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":4,"Lines":null,"Text":[{"Start":7,"Stop":11}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Blockquote","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":2,"Lines":[{"Start":23,"Stop":26},{"Start":27,"Stop":29}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":23,"Stop":25}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":27,"Stop":29}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Blockquote","Depth":1,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":2,"Lines":[{"Start":166,"Stop":180},{"Start":180,"Stop":190}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":166,"Stop":167}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":167,"Stop":173}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":173,"Stop":179}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":180,"Stop":183}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":183,"Stop":184}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":184,"Stop":186}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":186,"Stop":190}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Heading","Depth":1,"Lines":[{"Start":194,"Stop":195}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":194,"Stop":195}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":196,"Stop":198},{"Start":199,"Stop":201}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":196,"Stop":197}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":199,"Stop":201}],"Raw":false,"Soft":false,"Hard":false}],[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}],"Raw":false,"Soft":true,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null,"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}],"Raw":true,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}],"Raw":false,"Soft":false,"Hard":false},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}],"Raw":false,"Soft":false,"Hard":false}]],"Definitions":[{"Root":1,"Node":0,"Label":"n","Bounds":{"Span":{"Start":30,"Stop":164},"Label":{"Start":30,"Stop":35},"Parent":"Document"},"Found":true}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":null,"Unanchorable":null,"Valid":true},"Valid":true}}]}]`
	var records []agreementEscapedClosingDiagnosticSourceWitness
	if json.Unmarshal([]byte(frozen), &records) != nil {
		return nil
	}
	return records
}

func agreementEscapedClosingDiagnosticWitnesses() []agreementEscapedClosingDiagnosticWitness {
	const frozen = `[{"Name":"c41d987b2fa8","Body":"[^unused]: [[A]]\nB\n[[A\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~\n","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false","Diagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A\\","Message":"wikilink \"A\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"image.png","Message":"embed target \"image.png\" does not resolve","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"embed target \"A\" does not resolve","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"link-section-missing","Target":"","Message":"no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"embed target \"A\" does not resolve","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"A","Block":""}],"PrivateDiagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"q\\","Message":"wikilink \"q\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"image.png","Message":"embed target \"image.png\" does not resolve","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"embed target \"A\" does not resolve","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"link-section-missing","Target":"","Message":"no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"embed target \"A\" does not resolve","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"A","Block":""}],"Indices":[1],"Singles":[{"Field":1,"Index":1,"Start":21,"Body":"[^unused]: [[A]]\nB\n[[q\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~\n","Diagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"q\\","Message":"wikilink \"q\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"image.png","Message":"embed target \"image.png\" does not resolve","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"embed target \"A\" does not resolve","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"link-section-missing","Target":"","Message":"no heading in \"Notes/Reading.md\" matched \"A\"; the address is left as written and may land at the top of the note","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"embed target \"A\" does not resolve","Section":"A","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"A","Block":""}]}]},{"Name":"0af16098b14c","Body":"[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[A\\]]\u0060[[A]]\u0060   \u0060\u0060\u0060\n[[A#^a]][[B|alias]]章節\n\u0060\u0060\u0060\u0060\n\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false","Diagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A\\","Message":"wikilink \"A\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"risky-fence","Target":"","Message":"wikilink/callout/table syntax found inside a fenced code block; left untouched","Section":"","Block":""}],"PrivateDiagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"q\\","Message":"wikilink \"q\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"risky-fence","Target":"","Message":"wikilink/callout/table syntax found inside a fenced code block; left untouched","Section":"","Block":""}],"Indices":[2],"Singles":[{"Field":2,"Index":2,"Start":50,"Body":"[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[q\\]]\u0060[[A]]\u0060   \u0060\u0060\u0060\n[[A#^a]][[B|alias]]章節\n\u0060\u0060\u0060\u0060\n\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","Diagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"q\\","Message":"wikilink \"q\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"risky-fence","Target":"","Message":"wikilink/callout/table syntax found inside a fenced code block; left untouched","Section":"","Block":""}]}]},{"Name":"1206e9dd8d0e","Body":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[A\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false","Diagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A\\","Message":"wikilink \"A\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""}],"PrivateDiagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"q\\","Message":"wikilink \"q\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""}],"Indices":[2],"Singles":[{"Field":5,"Index":2,"Start":101,"Body":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[q\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n","Diagnostics":[{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"B","Message":"wikilink \"B\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"q\\","Message":"wikilink \"q\\\\\" does not resolve to any note or file","Section":"","Block":""},{"Kind":"wikilink-broken","Target":"A","Message":"wikilink \"A\" does not resolve to any note or file","Section":"","Block":""}]}]}]`
	var records []agreementEscapedClosingDiagnosticWitness
	if json.Unmarshal([]byte(frozen), &records) != nil {
		return nil
	}
	return records
}

func TestAgreementEscapedClosingDiagnosticOwnedSource(t *testing.T) {
	t.Parallel()
	records := agreementEscapedClosingDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementEscapedClosingDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			if len(r.Singles) != len(v.Owners) || len(r.Indices) != len(v.Owners) {
				t.Fatal("caught: complete combined discarded diagnostic owner set")
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
					t.Fatal("caught: exact single combined discarded diagnostic source projection")
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
					t.Fatal("caught: complete unchanged single combined discarded diagnostic source facts")
				}
				if len(single.Diagnostics) != len(r.Diagnostics) {
					t.Fatal("caught: complete ordered single combined discarded diagnostic ledger")
				}
				for i, diag := range r.Diagnostics {
					private := single.Diagnostics[i]
					if i != single.Index {
						if !cmp.Equal(diag, private) {
							t.Fatal("caught: every unselected raw diagnostic stays identical")
						}
						continue
					}
					if diag.Kind != render.DiagWikilinkBroken || diag.Target != r.RawTarget || private.Target != change.New+"\\" || private.Kind != diag.Kind || private.Section != diag.Section || private.Block != diag.Block || private.Message != strings.ReplaceAll(diag.Message, strconv.Quote(r.RawTarget), strconv.Quote(change.New+"\\")) {
						t.Fatal("caught: one owned diagnostic target and message change per source field")
					}
					expected[i] = private
				}
			}
			if !cmp.Equal(expected, r.PrivateDiagnostics) {
				t.Fatal("caught: whole ordered combined discarded diagnostic substitution")
			}
			for oi := range v.Owners {
				owner := &v.Owners[oi]
				if !fields[owner.Field] {
					t.Fatal("caught: every declared combined discarded source field is represented")
				}
			}
		})
	}
}

func TestAgreementEscapedClosingDiagnosticOwnedPublic(t *testing.T) {
	t.Parallel()
	records := agreementEscapedClosingDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementEscapedClosingDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
			if !cmp.Equal(raw.Diagnostics, r.Diagnostics) || !cmp.Equal(page, w.Page) {
				t.Fatal("caught: complete original combined discarded diagnostic and public ledgers")
			}
			private, pp := agreementIsolatedPage(t, agreementCase{Body: v.Body})
			if !cmp.Equal(agreementRemovedCommentDiagnosticRead(r.Body), w.Removed) || !cmp.Equal(agreementRemovedCommentDiagnosticRead(v.Body), v.Removed) {
				t.Fatal("caught: whole original and private combined discarded comment declarations")
			}

			if raw.HTML != private.HTML || !cmp.Equal(private.Diagnostics, r.PrivateDiagnostics) || !cmp.Equal(pp, w.Page) {
				t.Fatal("caught: whole combined discarded diagnostic projection leaves complete HTML unchanged")
			}
			for si := range r.Singles {
				single := &r.Singles[si]
				projected, p := agreementIsolatedPage(t, agreementCase{Body: single.Body})
				if projected.HTML != raw.HTML || !cmp.Equal(projected.Diagnostics, single.Diagnostics) || !cmp.Equal(p, w.Page) {
					t.Fatal("caught: complete independent single-field combined discarded diagnostic observation")
				}
				parts, valid := agreementCodeWindowParts(projected.HTML)
				if !valid || !cmp.Equal(parts, w.Parts) {
					t.Fatal("caught: every single-field combined discarded diagnostic code ledger stays identical")
				}
			}
			c := agreementCase{Body: r.Body}
			failures := agreementPageFailures(r.Body, &raw, &page)
			failures = append(failures, agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{page})[0]...)
			if !maps.Equal(agreementMixedCurrent(failures), w.Signatures) {
				t.Fatal("caught: complete original combined discarded diagnostic signed pool")
			}
			privateFailures := agreementPageFailures(v.Body, &private, &pp)
			privateFailures = append(privateFailures, agreementFragmentFailures(t, []agreementCase{{Body: v.Body}}, []agreementHTML{pp})[0]...)
			if !maps.Equal(agreementMixedCurrent(privateFailures), v.Signatures) {
				t.Fatal("caught: complete private combined discarded diagnostic signed pool")
			}
			seen := 0
			for _, f := range failures {
				if agreementSignature(&f) != r.Selected {
					continue
				}
				seen++
				k, a, b := agreementEscapedClosingDiagnosticDifference(c, &f, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "debt" || a != "#1011 stage 5" || b != "page-diagnostic" {
					t.Fatal("caught: exact combined discarded diagnostic P0 debt owner")
				}
			}
			if seen != 1 {
				t.Fatal("diagnostic selected receipt setup")
			}
		})
	}
}

func TestAgreementEscapedClosingDiagnosticOwnedRefusals(t *testing.T) {
	t.Parallel()
	records := agreementEscapedClosingDiagnosticWitnesses()
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
				{"decoded target", func(f *agreementFailure) { f.Tuple.Target = r.Target }},
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
					k, a, b := agreementEscapedClosingDiagnosticDifference(c, &changed, raw.HTML, &page, raw.Diagnostics, failures)
					if k != "" || a != "" || b != "" {
						t.Fatal("caught: combined discarded diagnostic borrowed another signed face")
					}
				})
			}
			for _, other := range []agreementCase{{Body: r.Body + "\n"}, {Body: r.Body, Title: "A"}, {Body: r.Body, Companions: capturedBodies{"Notes/Other.md": "outside"}}} {
				k, a, b := agreementEscapedClosingDiagnosticDifference(other, &selected, raw.HTML, &page, raw.Diagnostics, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: combined discarded diagnostic borrowed another body or context")
				}
			}
			for i := range failures {
				missing := slices.Delete(slices.Clone(failures), i, i+1)
				k, a, b := agreementEscapedClosingDiagnosticDifference(c, &selected, raw.HTML, &page, raw.Diagnostics, missing)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: combined discarded diagnostic borrowed an incomplete whole signed pool")
				}
			}
			for i := range raw.Diagnostics {
				missing := slices.Delete(slices.Clone(raw.Diagnostics), i, i+1)
				k, a, b := agreementEscapedClosingDiagnosticDifference(c, &selected, raw.HTML, &page, missing, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: combined discarded diagnostic borrowed an incomplete whole raw ledger")
				}
				altered := slices.Clone(raw.Diagnostics)
				altered[i].Message += "outside"
				k, a, b = agreementEscapedClosingDiagnosticDifference(c, &selected, raw.HTML, &page, altered, failures)
				if k != "" || a != "" || b != "" {
					t.Fatal("caught: combined discarded diagnostic borrowed another complete raw ledger")
				}
			}
			changed := page
			changed.Headings = append(slices.Clone(page.Headings), "outside")
			k, a, b := agreementEscapedClosingDiagnosticDifference(c, &selected, raw.HTML, &changed, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: combined discarded diagnostic borrowed another complete public ledger")
			}
			k, a, b = agreementEscapedClosingDiagnosticDifference(c, &selected, raw.HTML+"<code>outside</code>", &page, raw.Diagnostics, failures)
			if k != "" || a != "" || b != "" {
				t.Fatal("caught: combined discarded diagnostic borrowed changed field-free code text")
			}
		})
	}
}

func TestAgreementEscapedClosingDiagnosticOwnedWholeTargets(t *testing.T) {
	t.Parallel()
	records := agreementEscapedClosingDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementEscapedClosingDiagnosticSource(r.Body, r.Target)
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
				t.Fatal("caught: complete whole combined discarded diagnostic physical targets")
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
			for oi := range v.Owners {
				owner := &v.Owners[oi]
				field := w.Check.Fields[owner.Field]
				if !field.Code && !field.Comment && !field.Escaped {
					n++
				}
			}
			if before[r.Target]-after[r.Target] != n || after[replacement]-before[replacement] != n {
				t.Fatal("caught: combined discarded diagnostic target counts conserve every physical owner")
			}
			delete(before, r.Target)
			delete(before, replacement)
			delete(after, r.Target)
			delete(after, replacement)
			if !maps.Equal(before, after) {
				t.Fatal("caught: every unselected whole combined discarded diagnostic target count")
			}
		})
	}
}

func TestAgreementEscapedClosingDiagnosticOwnedWholeSet(t *testing.T) {
	t.Parallel()
	records := agreementEscapedClosingDiagnosticWitnesses()
	witnesses := agreementEscapedClosingDiagnosticSources()
	expected := make(map[[3]string]int)
	for _, r := range records {
		key := [3]string{r.Body, r.Target, r.Selected}
		if _, exists := expected[key]; exists || len(r.Singles) != len(r.Indices) {
			t.Fatal("caught: unique complete combined discarded diagnostic declared roster")
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
			if !strings.HasSuffix(f.Tuple.Target, "\\") {
				continue
			}
			target := strings.TrimSuffix(f.Tuple.Target, "\\")
			physical, owners, changes, private, valid := agreementEscapedClosingDiagnosticProducers(t, w.Body, target, native, &check)
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
			if !agreementEscapedClosingDiagnosticSignedProjection(failures, pf, &page, check.Targets, &f, len(owners), agreementEscapedClosingDiagnosticActive(owners, &check), w.Body, private, changes) {
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
				if diag.Kind != render.DiagWikilinkBroken || diag.Target != target+"\\" || renamed.Target != change.New+"\\" || renamed.Kind != diag.Kind || renamed.Section != diag.Section || renamed.Block != diag.Block || renamed.Message != strings.ReplaceAll(diag.Message, strconv.Quote(target+"\\"), strconv.Quote(change.New+"\\")) {
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
				t.Fatal("caught: every eligible combined discarded diagnostic roster member once")
			}
			actual[key] = len(owners)
			bodySelected[agreementSignature(&f)]++
			bodyVariants[[2]string{target, agreementSignature(&f)}] = len(owners)
			_, declared, found := agreementEscapedClosingDiagnosticSource(w.Body, target)
			if !found || !cmp.Equal(physical, declared.Physical) || !cmp.Equal(owners, declared.Owners) || !cmp.Equal(changes, declared.Changes) || private != declared.Body {
				t.Fatal("caught: complete native code physical and producer field census")
			}
		}
		declaredVariants := make(map[[2]string]int)
		for _, v := range w.Variants {
			f := agreementFailure{Property: "P0", Identity: "diagnostic-html", Tuple: agreementCitation{Target: v.Target + "\\", State: "wikilink-broken"}, Direction: "diagnostic-only", Multiplicity: len(v.Owners)}
			key := [2]string{v.Target, agreementSignature(&f)}
			if _, exists := declaredVariants[key]; exists {
				t.Fatal("caught: unique complete combined discarded diagnostic source variants")
			}
			declaredVariants[key] = len(v.Owners)
		}
		if !maps.Equal(bodySelected, w.Selected) || !maps.Equal(bodyVariants, declaredVariants) {
			t.Fatal("caught: whole eligible combined discarded diagnostic source declaration set")
		}
	}
	fields := 0
	for _, n := range actual {
		fields += n
	}
	if !maps.Equal(expected, actual) || len(actual) != 3 || fields != 3 || len(witnesses) != 3 {
		t.Fatal("caught: whole declared-body combined discarded diagnostic eligibility census")
	}
}

func TestAgreementEscapedClosingDiagnosticOwnedSignedRefusals(t *testing.T) {
	for _, r := range agreementEscapedClosingDiagnosticWitnesses() {
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			w, v, ok := agreementEscapedClosingDiagnosticSource(r.Body, r.Target)
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
			for fi := range before {
				f := before[fi]
				if agreementSignature(&f) == r.Selected {
					selected = f
					found = true
				}
			}
			if !found || !agreementEscapedClosingDiagnosticSignedProjection(before, after, &page, w.Targets, &selected, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: complete paired combined discarded diagnostic and check equation")
			}
			for i := range after {
				missing := slices.Delete(slices.Clone(after), i, i+1)
				if agreementEscapedClosingDiagnosticSignedProjection(before, missing, &page, w.Targets, &selected, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
					t.Fatal("caught: paired HTML projection borrowed an incomplete whole signed pool")
				}
			}
			extraBefore := append(slices.Clone(before), selected)
			if agreementEscapedClosingDiagnosticSignedProjection(extraBefore, after, &page, w.Targets, &selected, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: paired HTML projection borrowed another original diagnostic count")
			}
			extraAfter := append(slices.Clone(after), selected)
			if agreementEscapedClosingDiagnosticSignedProjection(before, extraAfter, &page, w.Targets, &selected, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: paired HTML projection borrowed another projected signed pool")
			}
			forged := agreementFailure{Property: "P1", Identity: "citation-occurrences", Tuple: agreementCitation{Target: r.Target}, Direction: "judge-only", Multiplicity: 1}
			forgedBefore := append(slices.Clone(before), forged)
			if agreementEscapedClosingDiagnosticSignedProjection(forgedBefore, after, &page, w.Targets, &selected, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: paired HTML projection borrowed another original check deficit")
			}
			otherPage := page
			otherPage.Citations = append(slices.Clone(page.Citations), agreementCitation{Target: r.Target})
			if agreementEscapedClosingDiagnosticSignedProjection(before, after, &otherPage, w.Targets, &selected, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: paired HTML projection borrowed another public target count")
			}
			otherTargets := append(slices.Clone(w.Targets), r.Target)
			if agreementEscapedClosingDiagnosticSignedProjection(before, after, &page, otherTargets, &selected, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: paired HTML projection borrowed another physical check count")
			}
			if agreementEscapedClosingDiagnosticSignedProjection(before, after, &page, w.Targets, &selected, len(v.Owners)+1, agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: paired HTML projection borrowed another producer count")
			}
			if agreementEscapedClosingDiagnosticSignedProjection(before, after, &page, w.Targets, nil, len(v.Owners), agreementEscapedClosingDiagnosticActive(v.Owners, &w.Check), r.Body, v.Body, v.Changes) {
				t.Fatal("caught: paired HTML projection borrowed an absent selected face")
			}
		})
	}
}

// Every field remains attached to its own declaration through the target projection.
func TestAgreementEscapedClosingDiagnosticOwnedPhysical(t *testing.T) {
	t.Parallel()
	records := agreementEscapedClosingDiagnosticWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Name+"/"+r.Target, func(t *testing.T) {
			t.Parallel()
			w, v, ok := agreementEscapedClosingDiagnosticSource(r.Body, r.Target)
			if !ok {
				t.Fatal("diagnostic source setup")
			}
			tree := agreementNativeProseAddressTreeReading(r.Body, nil)
			actual, valid := agreementEscapedClosingDiagnosticPhysical(r.Body, &w.Check, &tree, r.Target)
			privateTree := agreementNativeProseAddressTreeReading(v.Body, nil)
			renamed, privateValid := agreementEscapedClosingDiagnosticPhysical(v.Body, &v.Check, &privateTree, strings.Repeat("q", len(r.Target)))
			if !valid || !privateValid || !cmp.Equal(actual, v.Physical) || !cmp.Equal(renamed, v.Owners) {
				t.Fatal("caught: whole original and private combined discarded physical declaration sets")
			}
			if !cmp.Equal(agreementRemovedCommentDiagnosticRead(r.Body), w.Removed) || !cmp.Equal(agreementRemovedCommentDiagnosticRead(v.Body), v.Removed) {
				t.Fatal("caught: whole original and private combined discarded projection declarations")
			}
		})
	}
}

type agreementEscapedClosingDiagnosticAddressWitness struct {
	Body, Private, Target               string
	Names                               []string
	Original, Projected                 agreementNativeAddressPart
	OriginalAddresses, PrivateAddresses map[string]agreementNativeAddressReceipt
}

func agreementEscapedClosingDiagnosticAddressWitnesses() []agreementEscapedClosingDiagnosticAddressWitness {
	const frozen = `[{"Body":"[^unused]: [[A]]\nB\n[[A\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~\n","Names":["^a","^a-2","^agreement-absent-0","^é"],"Original":{"Native":{"Definitions":[{"Label":"unused","Parent":"Document","Span":{"Start":0,"Stop":184},"Name":{"Start":0,"Stop":10},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":11,"Stop":17},{"Start":17,"Stop":19},{"Start":19,"Stop":29},{"Start":29,"Stop":61},{"Start":61,"Stop":91},{"Start":91,"Stop":116},{"Start":116,"Stop":120},{"Start":120,"Stop":133},{"Start":133,"Stop":147},{"Start":147,"Stop":153},{"Start":153,"Stop":164},{"Start":164,"Stop":183}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":11,"Stop":12}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":12,"Stop":13}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":13,"Stop":15}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":15,"Stop":16}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":20}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":20,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":25}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":25,"Stop":28}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":29,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":54}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":54,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":63}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":63,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":87}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":91}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":111}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":115,"Stop":115}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":116,"Stop":117}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":117,"Stop":118}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":118,"Stop":119}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":120,"Stop":123}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":123,"Stop":128}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":128,"Stop":129}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":129,"Stop":131}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":131,"Stop":132}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":133,"Stop":134}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":134,"Stop":135}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":135,"Stop":141}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":142,"Stop":147}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":147,"Stop":153}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":153,"Stop":158}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":159,"Stop":163}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":164,"Stop":167}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":167,"Stop":168}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":168,"Stop":174}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":174,"Stop":175}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":175,"Stop":182}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":182,"Stop":183}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":91,"Stop":111}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":142,"Stop":158}}],"Comments":null},"Check":{"Code":[{"Start":91,"Stop":111},{"Start":142,"Stop":158}],"Comments":null,"Fields":[{"Span":{"Start":11,"Stop":16},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":19,"Stop":25},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":62,"Stop":75},"Word":"[[image.png]]","Targets":["image.png"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":83},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":91,"Stop":103},"Word":"[[A\\|alias]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":103,"Stop":109},"Word":"[[#A]]","Targets":[],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":116,"Stop":123},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":127,"Stop":132},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":133,"Stop":141},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":147,"Stop":152},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":166,"Stop":173},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":173,"Stop":180},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1}],"Targets":["A","A","image.png","A","A","A","A","A"]},"Fields":[{"Line":{"Start":19,"Stop":28},"Token":{"Start":26,"Stop":28},"Raw":"[[A\\]] ^A","Name":"^A","Folded":"^a","Owner":0,"Nodes":[1],"Code":false,"Comment":false}],"Names":["^a","^a-2","^é"]},"OriginalAddresses":{"^a":{"Page":false,"Judge":true,"Found":true,"Cut":"[^unused]: [[A]]\nB\n[[A\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~"},"^a-2":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^agreement-absent-0":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^é":{"Page":false,"Judge":false,"Found":false,"Cut":""}},"Private":"[^unused]: [[A]]\nB\n[[q\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~\n","PrivateAddresses":{"^a":{"Page":false,"Judge":true,"Found":true,"Cut":"[^unused]: [[A]]\nB\n[[q\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~"},"^a-2":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^agreement-absent-0":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^é":{"Page":false,"Judge":false,"Found":false,"Cut":""}},"Projected":{"Native":{"Definitions":[{"Label":"unused","Parent":"Document","Span":{"Start":0,"Stop":184},"Name":{"Start":0,"Stop":10},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":11,"Stop":17},{"Start":17,"Stop":19},{"Start":19,"Stop":29},{"Start":29,"Stop":61},{"Start":61,"Stop":91},{"Start":91,"Stop":116},{"Start":116,"Stop":120},{"Start":120,"Stop":133},{"Start":133,"Stop":147},{"Start":147,"Stop":153},{"Start":153,"Stop":164},{"Start":164,"Stop":183}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":11,"Stop":12}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":12,"Stop":13}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":13,"Stop":15}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":15,"Stop":16}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":20}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":20,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":25}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":25,"Stop":28}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":29,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":54}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":54,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":63}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":63,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":87}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":91}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":91,"Stop":111}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":115,"Stop":115}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":116,"Stop":117}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":117,"Stop":118}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":118,"Stop":119}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":120,"Stop":123}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":123,"Stop":128}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":128,"Stop":129}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":129,"Stop":131}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":131,"Stop":132}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":133,"Stop":134}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":134,"Stop":135}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":135,"Stop":141}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":142,"Stop":147}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":147,"Stop":153}]},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":153,"Stop":158}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":159,"Stop":163}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":164,"Stop":167}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":167,"Stop":168}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":168,"Stop":174}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":174,"Stop":175}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":175,"Stop":182}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":182,"Stop":183}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":91,"Stop":111}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":142,"Stop":158}}],"Comments":null},"Check":{"Code":[{"Start":91,"Stop":111},{"Start":142,"Stop":158}],"Comments":null,"Fields":[{"Span":{"Start":11,"Stop":16},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":19,"Stop":25},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":62,"Stop":75},"Word":"[[image.png]]","Targets":["image.png"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":83},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":91,"Stop":103},"Word":"[[A\\|alias]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":103,"Stop":109},"Word":"[[#A]]","Targets":[],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":116,"Stop":123},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":127,"Stop":132},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":133,"Stop":141},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":147,"Stop":152},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":166,"Stop":173},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":173,"Stop":180},"Word":"[[A#A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1}],"Targets":["A","q","image.png","A","A","A","A","A"]},"Fields":[{"Line":{"Start":19,"Stop":28},"Token":{"Start":26,"Stop":28},"Raw":"[[q\\]] ^A","Name":"^A","Folded":"^a","Owner":0,"Nodes":[1],"Code":false,"Comment":false}],"Names":["^a","^a-2","^é"]},"Target":"A"},{"Body":"[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[A\\]]\u0060[[A]]\u0060   \u0060\u0060\u0060\n[[A#^a]][[B|alias]]章節\n\u0060\u0060\u0060\u0060\n\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","Names":["^a","^a-2","^agreement-absent-0","^é"],"Original":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":0,"Stop":94},"Name":{"Start":0,"Stop":5},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":6,"Stop":11}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":6,"Stop":7}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":7,"Stop":8}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":8,"Stop":10}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":10,"Stop":11}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":17,"Stop":23},{"Start":27,"Stop":48},{"Start":48,"Stop":68},{"Start":68,"Stop":93}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":18,"Stop":19}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":22}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":27,"Stop":35}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":35,"Stop":41}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":41,"Stop":47}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":50}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":50,"Stop":54}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":55,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":67,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":68,"Stop":69}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":69,"Stop":70}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":70,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":86}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":86,"Stop":93}]}]}],"Code":[{"Kind":"FencedCodeBlock","Parent":"Document","Span":{"Start":99,"Stop":172}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":55,"Stop":60}}],"Comments":null},"Check":{"Code":[{"Start":17,"Stop":48},{"Start":55,"Stop":60},{"Start":99,"Stop":172}],"Comments":null,"Fields":[{"Span":{"Start":6,"Stop":11},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":17,"Stop":22},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":48,"Stop":54},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":55,"Stop":60},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":68,"Stop":76},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":87},"Word":"[[B|alias]]","Targets":["B"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":101,"Stop":106},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":137,"Stop":142},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":155,"Stop":160},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0}],"Targets":["A","A","A","B"]},"Fields":null,"Names":["^a","^a-2","^é"]},"OriginalAddresses":{"^a":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^a-2":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^agreement-absent-0":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^é":{"Page":false,"Judge":false,"Found":false,"Cut":""}},"Private":"[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[q\\]]\u0060[[A]]\u0060   \u0060\u0060\u0060\n[[A#^a]][[B|alias]]章節\n\u0060\u0060\u0060\u0060\n\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","PrivateAddresses":{"^a":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^a-2":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^agreement-absent-0":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^é":{"Page":false,"Judge":false,"Found":false,"Cut":""}},"Projected":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":0,"Stop":94},"Name":{"Start":0,"Stop":5},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":6,"Stop":11}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":6,"Stop":7}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":7,"Stop":8}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":8,"Stop":10}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":10,"Stop":11}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":17,"Stop":23},{"Start":27,"Stop":48},{"Start":48,"Stop":68},{"Start":68,"Stop":93}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":17,"Stop":18}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":18,"Stop":19}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":19,"Stop":21}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":21,"Stop":22}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":27,"Stop":35}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":35,"Stop":41}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":41,"Stop":47}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":50}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":50,"Stop":54}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":55,"Stop":60}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":61,"Stop":64}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":64,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":67,"Stop":67}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":68,"Stop":69}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":69,"Stop":70}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":70,"Stop":77}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":77,"Stop":78}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":78,"Stop":86}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":86,"Stop":93}]}]}],"Code":[{"Kind":"FencedCodeBlock","Parent":"Document","Span":{"Start":99,"Stop":172}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":55,"Stop":60}}],"Comments":null},"Check":{"Code":[{"Start":17,"Stop":48},{"Start":55,"Stop":60},{"Start":99,"Stop":172}],"Comments":null,"Fields":[{"Span":{"Start":6,"Stop":11},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":17,"Stop":22},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":48,"Stop":54},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":55,"Stop":60},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":68,"Stop":76},"Word":"[[A#^a]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":76,"Stop":87},"Word":"[[B|alias]]","Targets":["B"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":101,"Stop":106},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":137,"Stop":142},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0},{"Span":{"Start":155,"Stop":160},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":0}],"Targets":["A","q","A","B"]},"Fields":null,"Names":["^a","^a-2","^é"]},"Target":"A"},{"Body":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[A\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n","Names":["^a","^a-2","^agreement-absent-0","^é"],"Original":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":30,"Stop":164},"Name":{"Start":30,"Stop":35},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}]},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":[{"Start":180,"Stop":194}]},"Check":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":[{"Start":180,"Stop":194}],"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[A\\]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":185,"Stop":190},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":true,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1},{"Span":{"Start":196,"Stop":201},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","A","A"]},"Fields":[{"Line":{"Start":20,"Stop":25},"Token":{"Start":23,"Stop":25},"Raw":">  ^A","Name":"^A","Folded":"^a","Owner":-1,"Nodes":null,"Code":false,"Comment":false},{"Line":{"Start":26,"Stop":29},"Token":{"Start":27,"Stop":29},"Raw":" ^A","Name":"^A","Folded":"^a","Owner":-1,"Nodes":null,"Code":false,"Comment":false},{"Line":{"Start":212,"Stop":215},"Token":{"Start":213,"Stop":215},"Raw":" ^a","Name":"^a","Folded":"^a","Owner":-1,"Nodes":null,"Code":false,"Comment":false}],"Names":["^a","^a-2","^é"]},"OriginalAddresses":{"^a":{"Page":true,"Judge":true,"Found":true,"Cut":">  ^A\n ^A\n[^n]: [[A]]"},"^a-2":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^agreement-absent-0":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^é":{"Page":false,"Judge":false,"Found":false,"Cut":""}},"Private":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[q\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n","PrivateAddresses":{"^a":{"Page":true,"Judge":true,"Found":true,"Cut":">  ^A\n ^A\n[^n]: [[A]]"},"^a-2":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^agreement-absent-0":{"Page":false,"Judge":false,"Found":false,"Cut":""},"^é":{"Page":false,"Judge":false,"Found":false,"Cut":""}},"Projected":{"Native":{"Definitions":[{"Label":"n","Parent":"Document","Span":{"Start":30,"Stop":164},"Name":{"Start":30,"Stop":35},"Nodes":[{"Kind":"Footnote","Depth":0,"Lines":null,"Text":null},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":36,"Stop":41}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":36,"Stop":37}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":37,"Stop":38}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":38,"Stop":40}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":40,"Stop":41}]},{"Kind":"Paragraph","Depth":1,"Lines":[{"Start":47,"Stop":53},{"Start":53,"Stop":96},{"Start":96,"Stop":113},{"Start":113,"Stop":163}],"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":47,"Stop":48}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":48,"Stop":49}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":49,"Stop":51}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":51,"Stop":52}]},{"Kind":"AutoLink","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":84,"Stop":85}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":86,"Stop":91}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":92,"Stop":93}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":93,"Stop":94}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":94,"Stop":95}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":96,"Stop":99}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":99,"Stop":100}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":100,"Stop":101}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":101,"Stop":109}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":109,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":112,"Stop":112}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":113,"Stop":137}]},{"Kind":"CodeSpan","Depth":2,"Lines":null,"Text":null},{"Kind":"Text","Depth":3,"Lines":null,"Text":[{"Start":138,"Stop":143}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":144,"Stop":148}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":148,"Stop":157}]},{"Kind":"Text","Depth":2,"Lines":null,"Text":[{"Start":157,"Stop":163}]}]}],"Code":[{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":86,"Stop":91}},{"Kind":"CodeSpan","Parent":"Paragraph","Span":{"Start":138,"Stop":143}}],"Comments":[{"Start":180,"Stop":194}]},"Check":{"Code":[{"Start":47,"Stop":53},{"Start":78,"Stop":83},{"Start":86,"Stop":91},{"Start":138,"Stop":143}],"Comments":[{"Start":180,"Stop":194}],"Fields":[{"Span":{"Start":36,"Stop":41},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":47,"Stop":52},"Word":"[[B]]","Targets":["B"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":78,"Stop":83},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":86,"Stop":91},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":0},{"Span":{"Start":92,"Stop":99},"Word":"[[A\nB]]","Targets":[],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":99,"Stop":105},"Word":"[[q\\]]","Targets":["q"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":-1},{"Span":{"Start":138,"Stop":143},"Word":"[[A]]","Targets":["A"],"Code":true,"Comment":false,"Escaped":false,"DefinitionOwner":0,"CodeOwner":1},{"Span":{"Start":185,"Stop":190},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":true,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1},{"Span":{"Start":196,"Stop":201},"Word":"[[A]]","Targets":["A"],"Code":false,"Comment":false,"Escaped":false,"DefinitionOwner":-1,"CodeOwner":-1}],"Targets":["A","q","A"]},"Fields":[{"Line":{"Start":20,"Stop":25},"Token":{"Start":23,"Stop":25},"Raw":">  ^A","Name":"^A","Folded":"^a","Owner":-1,"Nodes":null,"Code":false,"Comment":false},{"Line":{"Start":26,"Stop":29},"Token":{"Start":27,"Stop":29},"Raw":" ^A","Name":"^A","Folded":"^a","Owner":-1,"Nodes":null,"Code":false,"Comment":false},{"Line":{"Start":212,"Stop":215},"Token":{"Start":213,"Stop":215},"Raw":" ^a","Name":"^a","Folded":"^a","Owner":-1,"Nodes":null,"Code":false,"Comment":false}],"Names":["^a","^a-2","^é"]},"Target":"A"}]`
	var records []agreementEscapedClosingDiagnosticAddressWitness
	if json.Unmarshal([]byte(frozen), &records) != nil {
		return nil
	}
	return records
}

func TestAgreementEscapedClosingDiagnosticOwnedCutReceipts(t *testing.T) {
	t.Parallel()
	records := agreementEscapedClosingDiagnosticAddressWitnesses()
	for ri := range records {
		r := &records[ri]
		t.Run(r.Target+"/"+r.Body, func(t *testing.T) {
			t.Parallel()
			raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
			pr, pp := agreementIsolatedPage(t, agreementCase{Body: r.Private})
			a := agreementNativeAddressPartReading(r.Body)
			b := agreementNativeAddressPartReading(r.Private)
			if !cmp.Equal(a, r.Original) || !cmp.Equal(b, r.Projected) {
				t.Fatal("caught: whole original and private diagnostic address declarations")
			}
			names := slices.Clone(a.Names)
			names = append(names, b.Names...)
			names = append(names, page.Blocks...)
			names = append(names, pp.Blocks...)
			absent := "^agreement-absent-0"
			for strings.Contains(r.Body, absent) || strings.Contains(r.Private, absent) || slices.Contains(names, absent) {
				absent += "x"
			}
			names = append(names, absent)
			slices.Sort(names)
			names = slices.Compact(names)
			if !slices.Equal(names, r.Names) {
				t.Fatal("caught: whole diagnostic address namespace including absent control")
			}
			if !cmp.Equal(agreementNativeAddressPublic(t, r.Body, &page, names), r.OriginalAddresses) || !cmp.Equal(agreementNativeAddressPublic(t, r.Private, &pp, names), r.PrivateAddresses) {
				t.Fatal("caught: whole original and private diagnostic address public receipts")
			}
			before := agreementPageFailures(r.Body, &raw, &page)
			before = append(before, agreementFragmentFailures(t, []agreementCase{{Body: r.Body}}, []agreementHTML{page})[0]...)
			after := agreementPageFailures(r.Private, &pr, &pp)
			after = append(after, agreementFragmentFailures(t, []agreementCase{{Body: r.Private}}, []agreementHTML{pp})[0]...)
			_, v, ok := agreementEscapedClosingDiagnosticSource(r.Body, r.Target)
			if !ok || v.Body != r.Private || !agreementEscapedClosingDiagnosticCutProjection(r.Body, r.Private, v.Changes, before, after) {
				t.Fatal("caught: complete source-bound diagnostic excerpt cut projection")
			}
			for i := range after {
				if after[i].Property != "P3" {
					continue
				}
				missing := slices.Delete(slices.Clone(after), i, i+1)
				if agreementEscapedClosingDiagnosticCutProjection(r.Body, r.Private, v.Changes, before, missing) {
					t.Fatal("caught: every projected address cut receipt retained")
				}
				changed := slices.Clone(after)
				changed[i].Cut += "!"
				if agreementEscapedClosingDiagnosticCutProjection(r.Body, r.Private, v.Changes, before, changed) {
					t.Fatal("caught: projected address cut follows exact owned source intervals")
				}
				changed = slices.Clone(after)
				changed[i].PagePresent = !changed[i].PagePresent
				if agreementEscapedClosingDiagnosticCutProjection(r.Body, r.Private, v.Changes, before, changed) {
					t.Fatal("caught: address faces cannot change with diagnostic target bytes")
				}
			}
			for i := range before {
				if before[i].Property != "P3" {
					continue
				}
				missing := slices.Delete(slices.Clone(before), i, i+1)
				if agreementEscapedClosingDiagnosticCutProjection(r.Body, r.Private, v.Changes, missing, after) {
					t.Fatal("caught: every original address cut receipt retained")
				}
			}
			for i := range v.Changes {
				duplicate := slices.Clone(v.Changes)
				duplicate = append(duplicate, duplicate[i])
				if agreementEscapedClosingDiagnosticCutProjection(r.Body, r.Private, duplicate, before, after) {
					t.Fatal("caught: every target source interval named exactly once")
				}
				omitted := slices.Delete(slices.Clone(v.Changes), i, i+1)
				if agreementEscapedClosingDiagnosticCutProjection(r.Body, r.Private, omitted, before, after) {
					t.Fatal("caught: whole private body follows the complete owned interval set")
				}
			}
		})
	}
}

func TestAgreementEscapedClosingDiagnosticOwnedCensus(t *testing.T) {
	t.Parallel()
	const frozen = `[{"Body":"[^unused]: [[A]]\nB\n[[A\\]] ^A\n^absent-prose - > [!note] title\n![[image.png]]![[A#A]]    \u0060\u0060\u0060\n[[A\\|alias]][[#A]]   \u0060\u0060\u0060\n[[A\nB]]\u0060\u0060\u0060 [[A]]\n[[A#^a]]\u0060open\n[[A]]\nclose\u0060 ^é\n\u0060![[A#A]][[A#A]]~~~\n","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false","Eligible":true},{"Body":"%%[[A]]%%[^n]: [[A]]\n\n    [[B]]\n[[A\\]]\u0060[[A]]\u00600~~~~\n## A\n  \u0060\u0060\u0060\n![[image.png]]A\n===\n[[A#^a]] ^é\n~~~\n[[A\nB]]%%[[A]]%%\\\u0060[[B|alias]]","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false","Eligible":false},{"Body":"[^n]: [[A]]\n\n    [[B]]\n    -->  > [!note] title\n[[A\\]]\u0060[[A]]\u0060   \u0060\u0060\u0060\n[[A#^a]][[B|alias]]章節\n\u0060\u0060\u0060\u0060\n\\\\[[A]]^absent-prose \nref[^n]\n\u0060\u0060\u0060\u0060 go [[A]]\nA\n\n\n1. <!--\n[[A]]\n-->  - # A\n","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false","Eligible":true},{"Body":"## A\n## A\n\tA\n===\nÉ\n\n[^n]: [[A]]\n\n    [[B]]\n[^n]: [[A]]\n\n    [[B]]\n![[A#A]][[A\\]]<!-- [[A]] -->> >   - [[B|alias]][[A\\]]章節\n\\\u0060^absent-prose text [^n]: [[A]]\n\n    [[B]]\n\u0060\u0060\u0060\u0060\n[[A#^a]][^n]: [[A]]\n\n    [[B]]\n  > [!note] title\ntext ","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=2 cut=\"\" page=false judge=false excerpt=false","Eligible":false},{"Body":"## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/\u0060[[A]]\u0060 \u0060[[A]]\u0060[[A\nB]][[A\\]]    \u0060\u0060\u0060\nhttps://example.invalid/\u0060[[A]]\u0060 > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n","Target":"A","RawTarget":"A\\","Selected":"P0/diagnostic-html tuple={SourceRole: Target:A\\ Section: State:wikilink-broken} fragment=\"\" direction=diagnostic-only multiplicity=1 cut=\"\" page=false judge=false excerpt=false","Eligible":true}]`
	var rows []struct {
		Body, Target, RawTarget, Selected string
		Eligible                          bool
	}
	if json.Unmarshal([]byte(frozen), &rows) != nil || len(rows) != 5 {
		t.Fatal("caught: complete raw closing diagnostic prospect set")
	}
	selected := map[[3]string]bool{}
	for _, r := range rows {
		native := agreementNativeOwnerReading(r.Body)
		check := agreementNativeCheckReading(r.Body, native)
		_, owners, changes, private, valid := agreementEscapedClosingDiagnosticProducers(t, r.Body, r.Target, native, &check)
		raw, page := agreementIsolatedPage(t, agreementCase{Body: r.Body})
		before := agreementPageFailures(r.Body, &raw, &page)
		before = append(before, agreementFragmentFailures(t, []agreementCase{{Body: r.Body}}, []agreementHTML{page})[0]...)
		pr, pp := agreementIsolatedPage(t, agreementCase{Body: private})
		after := agreementPageFailures(private, &pr, &pp)
		after = append(after, agreementFragmentFailures(t, []agreementCase{{Body: private}}, []agreementHTML{pp})[0]...)
		var f *agreementFailure
		for i := range before {
			if agreementSignature(&before[i]) == r.Selected {
				f = &before[i]
			}
		}
		active := agreementEscapedClosingDiagnosticActive(owners, &check)
		eligible := valid && f != nil && f.Tuple.Target == r.RawTarget && active >= 0 && len(owners) == f.Multiplicity && agreementEscapedClosingDiagnosticSignedProjection(before, after, &page, judge.LinkTargets(r.Body), f, len(owners), active, r.Body, private, changes)
		if eligible != r.Eligible {
			t.Fatal("caught: whole raw closing diagnostic eligibility and refusal set")
		}
		if eligible {
			selected[[3]string{r.Body, r.Target, r.Selected}] = true
		}
	}
	records := agreementEscapedClosingDiagnosticWitnesses()
	if len(records) != 3 || len(selected) != 3 {
		t.Fatal("caught: complete raw closing diagnostic source set")
	}
	for _, r := range records {
		if !selected[[3]string{r.Body, r.Target, r.Selected}] {
			t.Fatal("caught: every eligible raw closing diagnostic declaration")
		}
	}
}
