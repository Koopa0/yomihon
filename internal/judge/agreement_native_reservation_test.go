package judge_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementNativeReservationPayload struct {
	Address agreementNativeAddressPayload
	Outside map[string][]int
}

// Keep every outside physical declaration, including literal and comment roles.
func agreementNativeReservationOutside(s *agreementNativeAddressSource) map[string][]int {
	out := make(map[string][]int)
	for i := range s.Original.Fields {
		f := &s.Original.Fields[i]
		if f.Owner == -1 {
			out[f.Folded] = append(out[f.Folded], i)
		}
	}
	return out
}

// Renaming discarded declarations must expose only their same-name outside
// anchors. The whole public namespace keeps its other memberships and cuts.
func agreementNativeReservationEvidence(p *agreementNativeReservationPayload) bool {
	s := &p.Address.Source
	a, w := agreementHTML{Blocks: p.Address.Blocks}, agreementHTML{Blocks: p.Address.WitnessBlocks}
	if !s.Valid || len(s.Changes) == 0 || len(s.Original.Fields) != len(s.Projected.Fields) || !cmp.Equal(p.Outside, agreementNativeReservationOutside(s)) || !cmp.Equal(p.Address.Names, agreementNativeAddressNames(s, &a, &w)) || len(p.Address.Original) != len(p.Address.Names) || len(p.Address.Witness) != len(p.Address.Names) || !cmp.Equal(p.Address.Signatures, agreementNativeAddressSignatures(p.Address.Original)) {
		return false
	}
	selected := make(map[string]bool)
	expected := slices.Clone(p.Address.Blocks)
	for _, c := range s.Changes {
		selected[c.Old], selected[c.New] = true, true
		old, ok := p.Address.Original[c.Old]
		next, found := p.Address.Witness[c.Old]
		renamed, has := p.Address.Witness[c.New]
		if !ok || !found || !has || old.Page || !old.Judge || !old.Found || !next.Page || !next.Judge || !next.Found || renamed.Page || !renamed.Judge || !renamed.Found || p.Address.Original[c.New] != (agreementNativeAddressReceipt{}) {
			return false
		}
		owned, outside := false, false
		for i := range s.Original.Fields {
			f, g := &s.Original.Fields[i], &s.Projected.Fields[i]
			if agreementNativeAddressOwned(f) && f.Folded == c.Old && agreementNativeReservationLine(old.Cut, f.Raw) && agreementNativeReservationLine(renamed.Cut, g.Raw) {
				owned = true
			}
		}
		for _, i := range p.Outside[c.Old] {
			if i < 0 || i >= len(s.Original.Fields) {
				return false
			}
			f, g := &s.Original.Fields[i], &s.Projected.Fields[i]
			if f.Owner == -1 && len(f.Nodes) == 0 && !f.Code && !f.Comment && f.Folded == c.Old && cmp.Equal(f, g) && agreementNativeReservationLine(next.Cut, f.Raw) {
				outside = true
			}
		}
		if !owned || !outside {
			return false
		}
		expected = append(expected, c.Old)
	}
	slices.Sort(expected)
	projected := slices.Clone(p.Address.WitnessBlocks)
	slices.Sort(projected)
	if !cmp.Equal(expected, projected) {
		return false
	}
	for _, name := range p.Address.Names {
		before, ok := p.Address.Original[name]
		after, found := p.Address.Witness[name]
		cut, has := render.Excerpt(p.Address.Body, name)
		wcut, whas := render.Excerpt(s.Body, name)
		if !ok || !found || before.Page != slices.Contains(p.Address.Blocks, name) || after.Page != slices.Contains(p.Address.WitnessBlocks, name) || before.Cut != cut || before.Found != has || after.Cut != wcut || after.Found != whas || (!selected[name] && before != after) {
			return false
		}
	}
	return true
}
func agreementNativeReservationLine(cut, raw string) bool {
	for line := range strings.SplitSeq(cut, "\n") {
		if strings.TrimSuffix(line, "\r") == raw {
			return true
		}
	}
	return false
}
func agreementNativeReservationBudget(t *testing.T, body string, actual *agreementHTML, fs []agreementFailure) agreementNativeReservationPayload {
	t.Helper()
	source := agreementNativeAddressReading(body)
	if !source.Valid {
		return agreementNativeReservationPayload{}
	}
	_, w := agreementIsolatedPage(t, agreementCase{Body: source.Body})
	names := agreementNativeAddressNames(&source, actual, &w)
	original := agreementNativeAddressPublic(t, body, actual, names)
	projected := agreementNativeAddressPublic(t, source.Body, &w, names)
	p := agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: body, Source: source, Names: names, Blocks: slices.Clone(actual.Blocks), WitnessBlocks: slices.Clone(w.Blocks), Original: original, Witness: projected, Signatures: agreementNativeAddressSignatures(original)}, Outside: agreementNativeReservationOutside(&source)}
	if !agreementNativeReservationEvidence(&p) || !cmp.Equal(p.Address.Signatures, agreementNativeAddressCurrent(fs)) {
		return agreementNativeReservationPayload{}
	}
	return p
}
func agreementNativeReservationDifference(c agreementCase, f *agreementFailure, a *agreementHTML, fs []agreementFailure, p *agreementNativeReservationPayload) (kind, authority, wrong string) {
	if c.Body != p.Address.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P3" || f.Identity != "block-three-way" || f.Tuple != (agreementCitation{}) || f.Multiplicity != 1 || f.PagePresent || !f.JudgeAccepted || !f.ExcerptFound || (f.Direction != "judge-only" && f.Direction != "excerpt-only") || f.Cut == "" || p.Address.Signatures[agreementSignature(f)] != 1 {
		return "", "", ""
	}
	if !cmp.Equal(p.Address.Source, agreementNativeAddressReading(c.Body)) || !cmp.Equal(p.Address.Blocks, a.Blocks) || !cmp.Equal(p.Address.Signatures, agreementNativeAddressCurrent(fs)) || !agreementNativeReservationEvidence(p) {
		return "", "", ""
	}
	for _, change := range p.Address.Source.Changes {
		if change.Old == f.Fragment {
			return "debt", "#1011 stage 5", "page"
		}
	}
	return "", "", ""
}

type agreementNativeReservationControl struct {
	Input   agreementNativeAddressControl
	Outside map[string][]int
	Payload agreementNativeReservationPayload
	Owned   map[string]int
}

func agreementNativeReservationControls() []agreementNativeReservationControl {
	base := agreementNativeAddressControls()
	base = append(base, agreementNativeAddressControl{Name: "reservation control 49", Body: "[^n]: words ^a\n\noutside words ^a\n", Source: agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 16}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 14}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 6, Stop: 11}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 11, Stop: 14}}}}}}, Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[^n]: words ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 16, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 16}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 14}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 6, Stop: 11}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 11, Stop: 14}}}}}}, Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[^n]: words ^q", Name: "^q", Folded: "^q", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 16, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "[^n]: words ^q\n\noutside words ^a\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[^n]: words ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 16, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[^n]: words ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 16, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Valid: true}, Original: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "[^n]: words ^a"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}, Witness: map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: true, Found: true, Cut: "outside words ^a"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: true, Cut: "[^n]: words ^q"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}}, agreementNativeAddressControl{Name: "reservation control 50", Body: "[[A]] words ^live\n`open\nliteral ^literal\nclose`\n%%hide ^hidden\n%%\n\n[^n]: [[B]] ^a\n    second ^b\n\noutside words ^a\n\nother words ^b\n", Source: agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 67, Stop: 97}, Name: graph.Span{Start: 67, Stop: 72}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 73, Stop: 82}, {Start: 86, Stop: 95}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 73, Stop: 74}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 74, Stop: 75}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 75, Stop: 78}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 78, Stop: 81}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 86, Stop: 92}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 92, Stop: 95}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 19, Stop: 46}}}, Comments: []graph.Span{{Start: 48, Stop: 65}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 19, Stop: 46}}, Comments: []graph.Span{{Start: 48, Stop: 65}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 73, Stop: 78}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}}, Targets: []string{"A", "B"}}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^literal", Name: "^literal", Folded: "^literal", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 67, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: "[^n]: [[B]] ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 82, Stop: 95}, Token: graph.Span{Start: 93, Stop: 95}, Raw: "    second ^b", Name: "^b", Folded: "^b", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 97, Stop: 113}, Token: graph.Span{Start: 111, Stop: 113}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 115, Stop: 129}, Token: graph.Span{Start: 127, Stop: 129}, Raw: "other words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^b", "^hidden", "^literal", "^live", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 67, Stop: 97}, Name: graph.Span{Start: 67, Stop: 72}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 73, Stop: 82}, {Start: 86, Stop: 95}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 73, Stop: 74}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 74, Stop: 75}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 75, Stop: 78}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 78, Stop: 81}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 86, Stop: 92}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 92, Stop: 95}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 19, Stop: 46}}}, Comments: []graph.Span{{Start: 48, Stop: 65}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 19, Stop: 46}}, Comments: []graph.Span{{Start: 48, Stop: 65}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 73, Stop: 78}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}}, Targets: []string{"A", "B"}}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^literal", Name: "^literal", Folded: "^literal", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 67, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: "[^n]: [[B]] ^q", Name: "^q", Folded: "^q", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 82, Stop: 95}, Token: graph.Span{Start: 93, Stop: 95}, Raw: "    second ^z", Name: "^z", Folded: "^z", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 97, Stop: 113}, Token: graph.Span{Start: 111, Stop: 113}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 115, Stop: 129}, Token: graph.Span{Start: 127, Stop: 129}, Raw: "other words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^b", "^hidden", "^literal", "^live", "^q", "^z", "^é"}}, Body: "[[A]] words ^live\n`open\nliteral ^literal\nclose`\n%%hide ^hidden\n%%\n\n[^n]: [[B]] ^q\n    second ^z\n\noutside words ^a\n\nother words ^b\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}, {Old: "^b", New: "^z"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^literal", Name: "^literal", Folded: "^literal", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 67, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: "[^n]: [[B]] ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 82, Stop: 95}, Token: graph.Span{Start: 93, Stop: 95}, Raw: "    second ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 97, Stop: 113}, Token: graph.Span{Start: 111, Stop: 113}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 115, Stop: 129}, Token: graph.Span{Start: 127, Stop: 129}, Raw: "other words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^literal", Name: "^literal", Folded: "^literal", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 67, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: "[^n]: [[B]] ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 82, Stop: 95}, Token: graph.Span{Start: 93, Stop: 95}, Raw: "    second ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 97, Stop: 113}, Token: graph.Span{Start: 111, Stop: 113}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 115, Stop: 129}, Token: graph.Span{Start: 127, Stop: 129}, Raw: "other words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Valid: true}, Original: map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^a\n    second ^b"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^a\n    second ^b"}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live\n`open\nliteral ^literal\nclose`"}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^z": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}, Witness: map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: true, Found: true, Cut: "outside words ^a"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: true, Judge: true, Found: true, Cut: "other words ^b"}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live\n`open\nliteral ^literal\nclose`"}, "^q": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^q\n    second ^z"}, "^z": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^q\n    second ^z"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}}, agreementNativeAddressControl{Name: "reservation control 51", Body: "outside words ^a\n\n[^n]: words ^a\n", Source: agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 18, Stop: 33}, Name: graph.Span{Start: 18, Stop: 23}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 24, Stop: 32}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 24, Stop: 29}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 29, Stop: 32}}}}}}, Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 16}, Token: graph.Span{Start: 14, Stop: 16}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 18, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "[^n]: words ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 18, Stop: 33}, Name: graph.Span{Start: 18, Stop: 23}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 24, Stop: 32}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 24, Stop: 29}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 29, Stop: 32}}}}}}, Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 16}, Token: graph.Span{Start: 14, Stop: 16}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 18, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "[^n]: words ^q", Name: "^q", Folded: "^q", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "outside words ^a\n\n[^n]: words ^q\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 16}, Token: graph.Span{Start: 14, Stop: 16}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 18, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "[^n]: words ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 16}, Token: graph.Span{Start: 14, Stop: 16}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 18, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "[^n]: words ^?", Name: "?", Folded: "?", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}}, Valid: true}, Original: map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: true, Found: true, Cut: "outside words ^a"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}, Witness: map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: true, Found: true, Cut: "outside words ^a"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: true, Cut: "[^n]: words ^q"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}})
	return []agreementNativeReservationControl{
		{Input: base[0], Outside: map[string][]int{"^a": {1}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "text - [x] [[B]]\n`A\n---\n\\`## !\n![[A#A]]    ```\n`[[A]]`É\n> [!note] one\n> [!note] two\n> [!note] three\n- [x] [[B]]\n[^unused]: [[A]]\n ^A\n# A\n<!-- [[A]] -->> [!note] title\n ^a\n~~~\n`[[image.png]]", Source: base[0].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}, Blocks: []string(nil), WitnessBlocks: []string{"^a"}, Original: base[0].Original, Witness: base[0].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"- [x] [[B]]\\n[^unused]: [[A]]\\n ^A\\n# A\\n> [!note] title\\n ^a\\n~~~\\n`[[image.png]]\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"- [x] [[B]]\\n[^unused]: [[A]]\\n ^A\\n# A\\n> [!note] title\\n ^a\\n~~~\\n`[[image.png]]\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {1}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"- [x] [[B]]\\n[^unused]: [[A]]\\n ^A\\n# A\\n> [!note] title\\n ^a\\n~~~\\n`[[image.png]]\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"- [x] [[B]]\\n[^unused]: [[A]]\\n ^A\\n# A\\n> [!note] title\\n ^a\\n~~~\\n`[[image.png]]\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[1], Outside: map[string][]int{"^a": {1}, "^a-2": {2}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "[^unused]: [[A]]\n``open\n[[A]]\nclose`\ue0000\ue001  > [!note] title\n`open\n[[A]]\nclose````````\n ^A\nÉ\n<div>\n[[A]]\n</div>\nA\n---\n[[A]]## A\n ^A\n   ```\n`~~~~\n ^a-2\n  > [!note] title\nA\n===\n> [!unknown] title\n<!--A\n---\n", Source: base[1].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}, Blocks: []string(nil), WitnessBlocks: []string{"^a"}, Original: base[1].Original, Witness: base[1].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^unused]: [[A]]\\n``open\\n[[A]]\\nclose`\\ue0000\\ue001  > [!note] title\\n`open\\n[[A]]\\nclose````````\\n ^A\\nÉ\\n<div>\\n[[A]]\\n</div>\\nA\\n---\\n[[A]]## A\\n ^A\\n   ```\\n`~~~~\\n ^a-2\\n  > [!note] title\\nA\\n===\\n> [!unknown] title\\n<!--A\\n---\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^unused]: [[A]]\\n``open\\n[[A]]\\nclose`\\ue0000\\ue001  > [!note] title\\n`open\\n[[A]]\\nclose````````\\n ^A\\nÉ\\n<div>\\n[[A]]\\n</div>\\nA\\n---\\n[[A]]## A\\n ^A\\n   ```\\n`~~~~\\n ^a-2\\n  > [!note] title\\nA\\n===\\n> [!unknown] title\\n<!--A\\n---\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {1}, "^a-2": {2}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^unused]: [[A]]\\n``open\\n[[A]]\\nclose`\\ue0000\\ue001  > [!note] title\\n`open\\n[[A]]\\nclose````````\\n ^A\\nÉ\\n<div>\\n[[A]]\\n</div>\\nA\\n---\\n[[A]]## A\\n ^A\\n   ```\\n`~~~~\\n ^a-2\\n  > [!note] title\\nA\\n===\\n> [!unknown] title\\n<!--A\\n---\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^unused]: [[A]]\\n``open\\n[[A]]\\nclose`\\ue0000\\ue001  > [!note] title\\n`open\\n[[A]]\\nclose````````\\n ^A\\nÉ\\n<div>\\n[[A]]\\n</div>\\nA\\n---\\n[[A]]## A\\n ^A\\n   ```\\n`~~~~\\n ^a-2\\n  > [!note] title\\nA\\n===\\n> [!unknown] title\\n<!--A\\n---\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[2], Outside: map[string][]int{"^a": {1}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "[[A]]章節\nÉ\n章節\n[^n]: [[A]]\n\n    [[B]]\n ^A\n  - A\n===\n ^a\n```\n\n ^é\n%%## <em>A</em>\n ```\n![[image.png]]   ```\n%%[[image.png]]## !\n[[#A]]text ", Source: base[2].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}, Blocks: []string(nil), WitnessBlocks: []string{"^a"}, Original: base[2].Original, Witness: base[2].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {1}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[3], Outside: map[string][]int{"^a": {0}, "^a-2": {1}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[4], Outside: map[string][]int{"^a": {0}, "^a-2": {1}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[5], Outside: map[string][]int{"^absent-prose": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[6], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[7], Outside: map[string][]int{"^a": {0}, "^a-2": {1}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[8], Outside: map[string][]int{"^a": {1}, "^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[9], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[10], Outside: map[string][]int{"^a": {1}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "\ue0000\ue001## A\n\t~~~~\n[[B|alias]]## [[A|alias]]\n[^n]: [[A]]\n\n    [[B]]\n[[A]]A\n\\\\[[A]] ^a\n## A-2\n## A\n## A\n ^a\nA\n---\n> [!unknown] title\n## A\n## A\n", Source: base[10].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}, Blocks: []string(nil), WitnessBlocks: []string{"^a"}, Original: base[10].Original, Witness: base[10].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n[[A]]A\\n\\\\\\\\[[A]] ^a\\n## A-2\\n## A\\n## A\\n ^a\\nA\\n---\\n> [!unknown] title\\n## A\\n## A\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n[[A]]A\\n\\\\\\\\[[A]] ^a\\n## A-2\\n## A\\n## A\\n ^a\\nA\\n---\\n> [!unknown] title\\n## A\\n## A\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {1}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n[[A]]A\\n\\\\\\\\[[A]] ^a\\n## A-2\\n## A\\n## A\\n ^a\\nA\\n---\\n> [!unknown] title\\n## A\\n## A\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n[[A]]A\\n\\\\\\\\[[A]] ^a\\n## A-2\\n## A\\n## A\\n ^a\\nA\\n---\\n> [!unknown] title\\n## A\\n## A\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[11], Outside: map[string][]int{"^a": {0, 1}, "^a-2": {2}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[12], Outside: map[string][]int{"^a": {1}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[13], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[14], Outside: map[string][]int{"^a": {0}, "^a-2": {1}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[15], Outside: map[string][]int{"^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[16], Outside: map[string][]int{"^a": {1}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "\t```\n[^n]: [[A]]\n\n    [[B]]\n\ue0020\ue003 ^A\n> > ## A\n## A\n ^A\nÉ\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%[[A]]%%\\[[A]]\\`\ue0020\ue003\t```\n```` go [[A]]\n![[A]]- > [!note] title\n", Source: base[16].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}, Blocks: []string(nil), WitnessBlocks: []string{"^a"}, Original: base[16].Original, Witness: base[16].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n\\ue0020\\ue003 ^A\\n> > ## A\\n## A\\n ^A\\nÉ\\n| a | b |\\n|---|---|\\n| [[A]] | ^a |\\n\\\\[[A]]\\\\`\\ue0020\\ue003\\t```\\n```` go [[A]]\\n![[A]]- > [!note] title\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n\\ue0020\\ue003 ^A\\n> > ## A\\n## A\\n ^A\\nÉ\\n| a | b |\\n|---|---|\\n| [[A]] | ^a |\\n\\\\[[A]]\\\\`\\ue0020\\ue003\\t```\\n```` go [[A]]\\n![[A]]- > [!note] title\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {1}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n\\ue0020\\ue003 ^A\\n> > ## A\\n## A\\n ^A\\nÉ\\n| a | b |\\n|---|---|\\n| [[A]] | ^a |\\n\\\\[[A]]\\\\`\\ue0020\\ue003\\t```\\n```` go [[A]]\\n![[A]]- > [!note] title\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n\\ue0020\\ue003 ^A\\n> > ## A\\n## A\\n ^A\\nÉ\\n| a | b |\\n|---|---|\\n| [[A]] | ^a |\\n\\\\[[A]]\\\\`\\ue0020\\ue003\\t```\\n```` go [[A]]\\n![[A]]- > [!note] title\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[17], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[18], Outside: map[string][]int{"^a": {1}, "^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[19], Outside: map[string][]int{"^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[20], Outside: map[string][]int{"^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[21], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[22], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[23], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[24], Outside: map[string][]int{"^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[25], Outside: map[string][]int{"^a-2": {1}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[26], Outside: map[string][]int{"^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[27], Outside: map[string][]int{"^a": {0, 1, 2}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[28], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[29], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[30], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[31], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[32], Outside: map[string][]int{"^a-2": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[33], Outside: map[string][]int{"^a": {0, 1, 2}, "^a-2": {3}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[34], Outside: map[string][]int{"^a": {1}, "^a-2": {2}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "1. A\n> [!unknown] title\n  > [!note] title\nB\n[^n]: [[A]]\n\n    [[B]]\n ^A\n- [x] [[B]]\n ^a\n ^a-2\n\\[[A]]~~~~\n B\n[^n]: [[A]]\n\n    [[B]]\n<!--> \ue0020\ue003\t```\n[[image.png]]  ```\n `[[A]]`> [!unknown] title\n``` [[A]]\n## [[A|alias]]\n", Source: base[34].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}, Blocks: []string{"^a-2"}, WitnessBlocks: []string{"^a", "^a-2"}, Original: base[34].Original, Witness: base[34].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {1}, "^a-2": {2}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n ^A\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[35], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[36], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[37], Outside: map[string][]int{}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[38], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[39], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[40], Outside: map[string][]int{}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[41], Outside: map[string][]int{}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[42], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[43], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[44], Outside: map[string][]int{"^comment": {2}, "^literal": {1}, "^live": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[45], Outside: map[string][]int{"^q": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[46], Outside: map[string][]int{"^q": {0}, "^v": {2}, "^w": {4}, "^x": {3}, "^z": {1}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[47], Outside: map[string][]int{}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
		{Input: base[48], Outside: map[string][]int{"^a": {1}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "[^n]: words ^a\n\noutside words ^a\n", Source: base[48].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}, Blocks: []string(nil), WitnessBlocks: []string{"^a"}, Original: base[48].Original, Witness: base[48].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^n]: words ^a\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^n]: words ^a\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {1}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^n]: words ^a\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^n]: words ^a\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[49], Outside: map[string][]int{"^a": {5}, "^b": {6}, "^hidden": {2}, "^literal": {1}, "^live": {0}}, Payload: agreementNativeReservationPayload{Address: agreementNativeAddressPayload{Body: "[[A]] words ^live\n`open\nliteral ^literal\nclose`\n%%hide ^hidden\n%%\n\n[^n]: [[B]] ^a\n    second ^b\n\noutside words ^a\n\nother words ^b\n", Source: base[49].Source, Names: []string{"^a", "^a-2", "^agreement-absent-0", "^b", "^hidden", "^literal", "^live", "^q", "^z", "^é"}, Blocks: []string{"^live"}, WitnessBlocks: []string{"^live", "^a", "^b"}, Original: base[49].Original, Witness: base[49].Witness, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=excerpt-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=judge-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true": 1}}, Outside: map[string][]int{"^a": {5}, "^b": {6}, "^hidden": {2}, "^literal": {1}, "^live": {0}}}, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=excerpt-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=judge-only multiplicity=1 cut=\"[^n]: [[B]] ^a\\n    second ^b\" page=false judge=true excerpt=true / debt / #1011 stage 5 / page": 1}},
		{Input: base[50], Outside: map[string][]int{"^a": {0}}, Payload: agreementNativeReservationPayload{}, Owned: map[string]int{}},
	}
}

func TestAgreementNativeReservationOutside(t *testing.T) {
	t.Parallel()
	for _, tc := range agreementNativeReservationControls() {
		t.Run(tc.Input.Name, func(t *testing.T) {
			t.Parallel()
			s := agreementNativeAddressReading(tc.Input.Body)
			if diff := cmp.Diff(tc.Input.Source, s); diff != "" {
				t.Fatalf("caught: complete reservation source (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.Outside, agreementNativeReservationOutside(&s)); diff != "" {
				t.Fatalf("caught: complete reservation outside roster (-want +got):\n%s", diff)
			}
		})
	}
}
func TestAgreementNativeReservationPublic(t *testing.T) {
	t.Parallel()
	for _, tc := range agreementNativeReservationControls() {
		t.Run(tc.Input.Name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.Input.Body}
			_, a := agreementIsolatedPage(t, c)
			fs := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{a})[0]
			p := agreementNativeReservationBudget(t, c.Body, &a, fs)
			if diff := cmp.Diff(tc.Payload, p); diff != "" {
				t.Fatalf("caught: complete reservation public budget (-want +got):\n%s", diff)
			}
			if tc.Input.Source.Valid {
				_, w := agreementIsolatedPage(t, agreementCase{Body: tc.Input.Source.Body})
				names := agreementNativeAddressNames(&tc.Input.Source, &a, &w)
				before := agreementNativeAddressPublic(t, c.Body, &a, names)
				after := agreementNativeAddressPublic(t, tc.Input.Source.Body, &w, names)
				if diff := cmp.Diff(tc.Input.Original, before); diff != "" {
					t.Fatalf("caught: complete reservation original ledger (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.Input.Witness, after); diff != "" {
					t.Fatalf("caught: complete reservation projected ledger (-want +got):\n%s", diff)
				}
			}
			owned := make(map[string]int)
			for _, f := range fs {
				k, authority, wrong := agreementNativeReservationDifference(c, &f, &a, fs, &p)
				if k == "" {
					continue
				}
				owned[agreementSignature(&f)+" / "+k+" / "+authority+" / "+wrong]++
				agreementNativeReservationDrift(t, c, &f, &a, fs, &p)
			}
			if diff := cmp.Diff(tc.Owned, owned); diff != "" {
				t.Fatalf("caught: complete reservation public ownership (-want +got):\n%s", diff)
			}
		})
	}
}
func agreementNativeReservationDrift(t *testing.T, c agreementCase, f *agreementFailure, a *agreementHTML, fs []agreementFailure, p *agreementNativeReservationPayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if k, _, _ := agreementNativeReservationDifference(other, f, a, fs, p); k != "" {
			t.Fatal("caught: reservation borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){
		func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Multiplicity = 0 }, func(f *agreementFailure) { f.Multiplicity = -1 }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = false }, func(f *agreementFailure) { f.ExcerptFound = false }, func(f *agreementFailure) { f.Cut += "unowned" },
	} {
		other := *f
		change(&other)
		if k, _, _ := agreementNativeReservationDifference(c, &other, a, fs, p); k != "" {
			t.Fatalf("caught: reservation borrowed signature %s", agreementSignature(&other))
		}
	}
	other := *a
	other.Blocks = append(slices.Clone(a.Blocks), "^unowned")
	if k, _, _ := agreementNativeReservationDifference(c, f, &other, fs, p); k != "" {
		t.Fatal("caught: reservation borrowed changed whole page set")
	}
	partial := slices.Clone(fs)
	for i := range partial {
		entry := &partial[i]
		if entry.Property == "P3" && agreementSignature(entry) == agreementSignature(f) {
			partial = append(partial[:i], partial[i+1:]...)
			break
		}
	}
	for _, changed := range [][]agreementFailure{nil, partial, append(slices.Clone(fs), *f)} {
		if k, _, _ := agreementNativeReservationDifference(c, f, a, changed, p); k != "" {
			t.Fatal("caught: reservation borrowed changed whole P3 pool")
		}
		if agreementNativeReservationBudget(t, c.Body, a, changed).Address.Body != "" {
			t.Fatal("caught: reservation budget borrowed changed whole P3 pool")
		}
	}
	for _, change := range []func(*agreementNativeReservationPayload){
		func(p *agreementNativeReservationPayload) { p.Outside = nil },
		func(p *agreementNativeReservationPayload) {
			p.Outside = maps.Clone(p.Outside)
			p.Outside["^unowned"] = []int{0}
		},
		func(p *agreementNativeReservationPayload) {
			p.Outside = maps.Clone(p.Outside)
			for name := range p.Outside {
				delete(p.Outside, name)
				break
			}
		},
		func(p *agreementNativeReservationPayload) {
			p.Outside = maps.Clone(p.Outside)
			for name, indices := range p.Outside {
				p.Outside[name] = append(slices.Clone(indices), -1)
				break
			}
		},
	} {
		other := *p
		change(&other)
		if k, _, _ := agreementNativeReservationDifference(c, f, a, fs, &other); k != "" {
			t.Fatal("caught: reservation borrowed changed complete outside roster")
		}
	}

	for _, change := range []func(*agreementNativeReservationPayload){
		func(p *agreementNativeReservationPayload) { p.Address.Body += "unowned" }, func(p *agreementNativeReservationPayload) { p.Address.Names = nil }, func(p *agreementNativeReservationPayload) {
			p.Address.Blocks = append(slices.Clone(p.Address.Blocks), "^unowned")
		}, func(p *agreementNativeReservationPayload) {
			p.Address.WitnessBlocks = append(slices.Clone(p.Address.WitnessBlocks), "^unowned")
		}, func(p *agreementNativeReservationPayload) { p.Address.Original = nil }, func(p *agreementNativeReservationPayload) { p.Address.Witness = nil }, func(p *agreementNativeReservationPayload) { p.Address.Signatures = nil },
		func(p *agreementNativeReservationPayload) {
			p.Address.Original = maps.Clone(p.Address.Original)
			p.Address.Original["^unowned"] = agreementNativeAddressReceipt{}
		}, func(p *agreementNativeReservationPayload) {
			p.Address.Witness = maps.Clone(p.Address.Witness)
			p.Address.Witness["^unowned"] = agreementNativeAddressReceipt{}
		},
		func(p *agreementNativeReservationPayload) {
			p.Address.Original = maps.Clone(p.Address.Original)
			for name, r := range p.Address.Original {
				r.Cut += "unowned"
				p.Address.Original[name] = r
			}
		}, func(p *agreementNativeReservationPayload) {
			p.Address.Witness = maps.Clone(p.Address.Witness)
			for name, r := range p.Address.Witness {
				r.Cut += "unowned"
				p.Address.Witness[name] = r
			}
		},
	} {
		other := *p
		change(&other)
		if k, _, _ := agreementNativeReservationDifference(c, f, a, fs, &other); k != "" {
			t.Fatal("caught: reservation borrowed changed complete public evidence")
		}
	}
}

func TestAgreementNativeReservationSourceDrift(t *testing.T) {
	t.Parallel()
	body := "[[A]] words ^live\n`open\nliteral ^literal\nclose`\n%%hide ^hidden\n%%\n\n[^n]: [[B]] ^a\n    second ^b\n\noutside words ^a\n\nother words ^b\n"
	c := agreementCase{Body: body}
	_, a := agreementIsolatedPage(t, c)
	fs := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{a})[0]
	baseline := agreementNativeReservationBudget(t, body, &a, fs)
	s := &baseline.Address.Source
	if baseline.Address.Body == "" || len(s.Original.Native.Definitions) != 1 || len(s.Original.Native.Code) != 1 || len(s.Original.Native.Comments) != 1 || len(s.Original.Check.Fields) != 2 || len(s.Original.Fields) != 7 || len(s.Projected.Fields) != 7 || len(s.Shapes) != 7 || len(s.AfterShapes) != 7 || len(s.Changes) != 2 || len(s.Original.Native.Definitions[0].Nodes) < 3 || len(s.Original.Native.Definitions[0].Nodes[1].Lines) < 2 || len(s.Original.Native.Definitions[0].Nodes[2].Text) != 1 || len(s.Original.Fields[3].Nodes) != 1 {
		t.Fatal("reservation source drift lacks complete declared shape")
	}
	var failure *agreementFailure
	for _, f := range fs {
		if k, _, _ := agreementNativeReservationDifference(c, &f, &a, fs, &baseline); k != "" {
			owned := f
			failure = &owned
			break
		}
	}
	if failure == nil {
		t.Fatal("reservation source drift control is unowned")
	}
	changes := []struct {
		name   string
		change func(*agreementNativeAddressSource)
	}{
		{"validity", func(s *agreementNativeAddressSource) { s.Valid = false }},
		{"whole original", func(s *agreementNativeAddressSource) { s.Original = agreementNativeAddressPart{} }},
		{"whole projection", func(s *agreementNativeAddressSource) { s.Projected = agreementNativeAddressPart{} }},
		{"original definitions", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions = nil }},
		{"original label", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Label = "unowned" }},
		{"original parent", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Parent = "unowned" }},
		{"original consumed span", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Span.Stop-- }},
		{"original label span", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Name.Start++ }},
		{"original child set", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Nodes = nil }},
		{"original child kind", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Nodes[1].Kind = "unowned" }},
		{"original child depth", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Nodes[1].Depth++ }},
		{"original child lines", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Nodes[1].Lines = nil }},
		{"original child text", func(s *agreementNativeAddressSource) { s.Original.Native.Definitions[0].Nodes[2].Text = nil }},
		{"original code", func(s *agreementNativeAddressSource) { s.Original.Native.Code = nil }},
		{"original code kind", func(s *agreementNativeAddressSource) { s.Original.Native.Code[0].Kind = "unowned" }},
		{"original code parent", func(s *agreementNativeAddressSource) { s.Original.Native.Code[0].Parent = "unowned" }},
		{"original code span", func(s *agreementNativeAddressSource) { s.Original.Native.Code[0].Span.Start++ }},
		{"original comments", func(s *agreementNativeAddressSource) { s.Original.Native.Comments = nil }},
		{"original comment bound", func(s *agreementNativeAddressSource) { s.Original.Native.Comments[0].Stop-- }},
		{"original check fields", func(s *agreementNativeAddressSource) { s.Original.Check.Fields = nil }},
		{"original check word", func(s *agreementNativeAddressSource) { s.Original.Check.Fields[0].Word += "unowned" }},
		{"original check span", func(s *agreementNativeAddressSource) { s.Original.Check.Fields[0].Span.Start++ }},
		{"original check targets", func(s *agreementNativeAddressSource) { s.Original.Check.Targets = nil }},
		{"original check code", func(s *agreementNativeAddressSource) { s.Original.Check.Code = nil }},
		{"original check comments", func(s *agreementNativeAddressSource) { s.Original.Check.Comments = nil }},
		{"original fields", func(s *agreementNativeAddressSource) { s.Original.Fields = nil }},
		{"original raw line", func(s *agreementNativeAddressSource) { s.Original.Fields[3].Raw += "unowned" }},
		{"original physical line", func(s *agreementNativeAddressSource) { s.Original.Fields[3].Line.Start++ }},
		{"original token span", func(s *agreementNativeAddressSource) { s.Original.Fields[3].Token.Stop-- }},
		{"original token spelling", func(s *agreementNativeAddressSource) { s.Original.Fields[3].Name = "^unowned" }},
		{"original folded name", func(s *agreementNativeAddressSource) { s.Original.Fields[3].Folded = "^unowned" }},
		{"original owner", func(s *agreementNativeAddressSource) { s.Original.Fields[3].Owner = -1 }},
		{"original owning nodes", func(s *agreementNativeAddressSource) { s.Original.Fields[3].Nodes = nil }},
		{"original code role", func(s *agreementNativeAddressSource) { s.Original.Fields[1].Code = false }},
		{"original comment role", func(s *agreementNativeAddressSource) { s.Original.Fields[2].Comment = false }},
		{"original namespace", func(s *agreementNativeAddressSource) { s.Original.Names = nil }},
		{"projected definitions", func(s *agreementNativeAddressSource) { s.Projected.Native.Definitions = nil }},
		{"projected code", func(s *agreementNativeAddressSource) { s.Projected.Native.Code = nil }},
		{"projected comments", func(s *agreementNativeAddressSource) { s.Projected.Native.Comments = nil }},
		{"projected check fields", func(s *agreementNativeAddressSource) { s.Projected.Check.Fields = nil }},
		{"projected check targets", func(s *agreementNativeAddressSource) { s.Projected.Check.Targets = nil }},
		{"projected fields", func(s *agreementNativeAddressSource) { s.Projected.Fields = nil }},
		{"projected raw line", func(s *agreementNativeAddressSource) { s.Projected.Fields[3].Raw += "unowned" }},
		{"projected token", func(s *agreementNativeAddressSource) { s.Projected.Fields[3].Token.Start++ }},
		{"projected namespace", func(s *agreementNativeAddressSource) { s.Projected.Names = nil }},
		{"private body", func(s *agreementNativeAddressSource) { s.Body += "unowned" }},
		{"whole changes", func(s *agreementNativeAddressSource) { s.Changes = nil }},
		{"old change name", func(s *agreementNativeAddressSource) { s.Changes[0].Old = "^unowned" }},
		{"new change name", func(s *agreementNativeAddressSource) { s.Changes[0].New = "^unowned" }},
		{"original shapes", func(s *agreementNativeAddressSource) { s.Shapes = nil }},
		{"original shape words", func(s *agreementNativeAddressSource) { s.Shapes[3].Raw += "unowned" }},
		{"original shape order", func(s *agreementNativeAddressSource) { s.Shapes[0], s.Shapes[1] = s.Shapes[1], s.Shapes[0] }},
		{"projected shapes", func(s *agreementNativeAddressSource) { s.AfterShapes = nil }},
		{"projected shape role", func(s *agreementNativeAddressSource) { s.AfterShapes[3].Code = true }},
	}
	for i := range changes {
		change := &changes[i]
		t.Run(change.name, func(t *testing.T) {
			t.Parallel()
			p := baseline
			p.Address.Source = agreementNativeAddressReading(body)
			change.change(&p.Address.Source)
			if k, _, _ := agreementNativeReservationDifference(c, failure, &a, fs, &p); k != "" {
				t.Fatal("caught: reservation borrowed changed complete source")
			}
		})
	}
	otherFamily := append(slices.Clone(fs), agreementFailure{Property: "P4", Identity: "unowned", Fragment: "unowned"})
	if k, _, _ := agreementNativeReservationDifference(c, failure, &a, otherFamily, &baseline); k != "debt" {
		t.Fatal("caught: reservation P3 pool borrowed another property")
	}
	for _, name := range []string{"^literal", "^hidden", "^live"} {
		p := baseline
		p.Outside = maps.Clone(p.Outside)
		delete(p.Outside, name)
		if k, _, _ := agreementNativeReservationDifference(c, failure, &a, fs, &p); k != "" {
			t.Fatal("caught: reservation borrowed omitted inactive or unselected outside member")
		}
	}
}

// Unaddressed prose still belongs to the captured body when declaration facts
// and every public fragment receipt happen to stay the same.
func TestAgreementNativeReservationBody(t *testing.T) {
	t.Parallel()
	rich := agreementNativeReservationControls()[49].Input.Body
	c := agreementCase{Body: "ghost\n\n" + rich}
	other := agreementCase{Body: "shade\n\n" + rich}
	_, a := agreementIsolatedPage(t, c)
	_, w := agreementIsolatedPage(t, other)
	fs := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{a})[0]
	changed := agreementFragmentFailures(t, []agreementCase{other}, []agreementHTML{w})[0]
	p := agreementNativeReservationBudget(t, c.Body, &a, fs)
	q := agreementNativeReservationBudget(t, other.Body, &w, changed)
	if p.Address.Body == "" || q.Address.Body == "" || p.Address.Body == q.Address.Body {
		t.Fatal("reservation body control lacks distinct qualified bodies")
	}
	facts := func(p *agreementNativeReservationPayload) []any {
		return []any{p.Address.Source.Original, p.Address.Source.Projected, p.Address.Source.Changes, p.Address.Source.Shapes, p.Address.Source.AfterShapes, p.Address.Names, p.Address.Blocks, p.Address.WitnessBlocks, p.Address.Original, p.Address.Witness, p.Address.Signatures, p.Outside}
	}
	if !cmp.Equal(facts(&p), facts(&q)) {
		t.Fatal("reservation body control changed declaration facts or public receipts")
	}
	var failure *agreementFailure
	for i := range changed {
		f := &changed[i]
		if k, _, _ := agreementNativeReservationDifference(other, f, &w, changed, &q); k != "" {
			failure = f
			break
		}
	}
	if failure == nil {
		t.Fatal("reservation body control is unowned")
	}
	p.Address.Body = other.Body
	if k, _, _ := agreementNativeReservationDifference(other, failure, &w, changed, &p); k != "" {
		t.Fatal("caught: reservation borrowed different bytes with identical declared facts")
	}
}
