package judge_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementNativeCodeAddressOwner struct{ Field, Code int }

// A spelling is literal only when every physical occurrence belongs wholly to
// one native code body. Info strings and shared prose cannot supply that proof.
func agreementNativeCodeAddressOwners(s *agreementNativeAddressPart) map[string][]agreementNativeCodeAddressOwner {
	fields := make(map[string][]int)
	for i := range s.Fields {
		f := &s.Fields[i]
		fields[f.Folded] = append(fields[f.Folded], i)
	}
	out := make(map[string][]agreementNativeCodeAddressOwner)
	for name, indices := range fields {
		var owners []agreementNativeCodeAddressOwner
		valid := true
		for _, i := range indices {
			f := &s.Fields[i]
			owner := -1
			for c, code := range s.Native.Code {
				if code.Span.Start > f.Token.Start || f.Token.Stop > code.Span.Stop {
					continue
				}
				if owner >= 0 || (code.Kind != "CodeSpan" && code.Kind != "CodeBlock" && code.Kind != "FencedCodeBlock") {
					valid = false
				}
				owner = c
			}
			if owner < 0 || f.Owner != -1 || len(f.Nodes) != 0 || !f.Code || f.Comment {
				valid = false
			}
			owners = append(owners, agreementNativeCodeAddressOwner{Field: i, Code: owner})
		}
		if valid {
			out[name] = owners
		}
	}
	return out
}
func agreementNativeCodeAddressShapes(s *agreementNativeAddressPart) []agreementNativeAddressField {
	owners := agreementNativeCodeAddressOwners(s)
	out := slices.Clone(s.Fields)
	for i := range out {
		f := &out[i]
		if len(owners[f.Folded]) == 0 {
			continue
		}
		raw := []byte(f.Raw)
		for at := f.Token.Start - f.Line.Start + 1; at < f.Token.Stop-f.Line.Start; at++ {
			raw[at] = '?'
		}
		f.Raw, f.Name, f.Folded = string(raw), "?", "?"
	}
	return out
}

// Changing only literal tail names preserves all native/check declarations and
// every other source byte. Both full namespaces still include inactive names.
func agreementNativeCodeAddressReading(body string) agreementNativeAddressSource {
	r := agreementNativeAddressSource{Original: agreementNativeAddressPartReading(body)}
	owners := agreementNativeCodeAddressOwners(&r.Original)
	occupied := make(map[string]bool)
	for _, name := range r.Original.Names {
		if strings.ContainsAny(name, "|#]\r\n") || strings.HasSuffix(name, "\\") {
			return r
		}
		occupied[name] = true
	}
	changed := make(map[string]string)
	private := []byte(body)
	for i := range r.Original.Fields {
		f := &r.Original.Fields[i]
		if len(owners[f.Folded]) == 0 {
			continue
		}
		name := changed[f.Folded]
		if name == "" {
			for _, ch := range []string{"q", "z", "v", "x", "w"} {
				candidate := "^" + strings.Repeat(ch, len(f.Name)-1)
				if !occupied[candidate] {
					name = candidate
					break
				}
			}
			if name == "" {
				return r
			}
			changed[f.Folded], occupied[name] = name, true
			r.Changes = append(r.Changes, agreementNativeAddressChange{Old: f.Folded, New: name})
		}
		if len(name) != f.Token.Stop-f.Token.Start {
			return r
		}
		copy(private[f.Token.Start:f.Token.Stop], name)
	}
	if len(r.Changes) == 0 {
		return r
	}
	r.Body = string(private)
	r.Projected = agreementNativeAddressPartReading(r.Body)
	r.Shapes, r.AfterShapes = agreementNativeCodeAddressShapes(&r.Original), agreementNativeCodeAddressShapes(&r.Projected)
	r.Valid = cmp.Equal(r.Shapes, r.AfterShapes) && cmp.Equal(r.Original.Native, r.Projected.Native) && cmp.Equal(r.Original.Check, r.Projected.Check) && slices.Equal(r.Original.Check.Targets, judge.LinkTargets(body)) && slices.Equal(r.Projected.Check.Targets, judge.LinkTargets(r.Body))
	return r
}

type agreementNativeCodeAddressPayload struct {
	Address agreementNativeAddressPayload
	Owners  map[string][]agreementNativeCodeAddressOwner
}

// Code names have no address authority. A private name must reproduce every
// observed face while removing the old name from the whole public namespace.
func agreementNativeCodeAddressEvidence(p *agreementNativeCodeAddressPayload) bool {
	s := &p.Address.Source
	a, w := agreementHTML{Blocks: p.Address.Blocks}, agreementHTML{Blocks: p.Address.WitnessBlocks}
	if !s.Valid || len(s.Changes) == 0 || len(s.Original.Fields) != len(s.Projected.Fields) || !cmp.Equal(p.Owners, agreementNativeCodeAddressOwners(&s.Original)) || len(p.Owners) != len(s.Changes) || !cmp.Equal(p.Address.Names, agreementNativeAddressNames(s, &a, &w)) || len(p.Address.Original) != len(p.Address.Names) || len(p.Address.Witness) != len(p.Address.Names) || !cmp.Equal(p.Address.Signatures, agreementNativeAddressSignatures(p.Address.Original)) {
		return false
	}
	selected := make(map[string]string)
	projectedOwners := agreementNativeCodeAddressOwners(&s.Projected)
	if len(projectedOwners) != len(p.Owners) {
		return false
	}
	visible := false
	for _, c := range s.Changes {
		old, ok := p.Address.Original[c.Old]
		next, has := p.Address.Witness[c.New]
		if !ok || !has || len(p.Owners[c.Old]) == 0 || !cmp.Equal(p.Owners[c.Old], projectedOwners[c.New]) || p.Address.Original[c.New] != (agreementNativeAddressReceipt{}) || p.Address.Witness[c.Old] != (agreementNativeAddressReceipt{}) || old.Page != next.Page || old.Judge != next.Judge || old.Found != next.Found {
			return false
		}
		selected[c.Old], selected[c.New] = c.New, c.New
		if old.Page != old.Judge || old.Page != old.Found {
			visible = true
		}
		if old.Found {
			paired := false
			for _, owner := range p.Owners[c.Old] {
				f, g := &s.Original.Fields[owner.Field], &s.Projected.Fields[owner.Field]
				if agreementNativeReservationLine(old.Cut, f.Raw) && agreementNativeReservationLine(next.Cut, g.Raw) {
					paired = true
				}
			}
			if !paired {
				return false
			}
		}
	}
	if !visible {
		return false
	}
	expected := slices.Clone(p.Address.Blocks)
	for i, name := range expected {
		if renamed := selected[name]; renamed != "" {
			expected[i] = renamed
		}
	}
	if !cmp.Equal(expected, p.Address.WitnessBlocks) {
		return false
	}
	for _, name := range p.Address.Names {
		before, ok := p.Address.Original[name]
		after, has := p.Address.Witness[name]
		cut, found := render.Excerpt(p.Address.Body, name)
		wcut, wfound := render.Excerpt(s.Body, name)
		if !ok || !has || before.Page != slices.Contains(p.Address.Blocks, name) || after.Page != slices.Contains(p.Address.WitnessBlocks, name) || before.Cut != cut || before.Found != found || after.Cut != wcut || after.Found != wfound || (selected[name] == "" && before != after) {
			return false
		}
	}
	return true
}
func agreementNativeCodeAddressBudget(t *testing.T, body string, a *agreementHTML, fs []agreementFailure) agreementNativeCodeAddressPayload {
	t.Helper()
	s := agreementNativeCodeAddressReading(body)
	if !s.Valid {
		return agreementNativeCodeAddressPayload{}
	}
	_, w := agreementIsolatedPage(t, agreementCase{Body: s.Body})
	names := agreementNativeAddressNames(&s, a, &w)
	original := agreementNativeAddressPublic(t, body, a, names)
	projected := agreementNativeAddressPublic(t, s.Body, &w, names)
	p := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: body, Source: s, Names: names, Blocks: slices.Clone(a.Blocks), WitnessBlocks: slices.Clone(w.Blocks), Original: original, Witness: projected, Signatures: agreementNativeAddressSignatures(original)}, Owners: agreementNativeCodeAddressOwners(&s.Original)}
	if !agreementNativeCodeAddressEvidence(&p) || !cmp.Equal(p.Address.Signatures, agreementNativeAddressCurrent(fs)) {
		return agreementNativeCodeAddressPayload{}
	}
	return p
}
func agreementNativeCodeAddressDifference(c agreementCase, f *agreementFailure, a *agreementHTML, fs []agreementFailure, p *agreementNativeCodeAddressPayload) (kind, authority, wrong string) {
	if c.Body != p.Address.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P3" || f.Identity != "block-three-way" || f.Tuple != (agreementCitation{}) || f.Multiplicity != 1 || p.Address.Signatures[agreementSignature(f)] != 1 || len(p.Owners[f.Fragment]) == 0 {
		return "", "", ""
	}
	if !cmp.Equal(p.Address.Source, agreementNativeCodeAddressReading(c.Body)) || !cmp.Equal(p.Address.Blocks, a.Blocks) || !cmp.Equal(p.Address.Signatures, agreementNativeAddressCurrent(fs)) || !agreementNativeCodeAddressEvidence(p) {
		return "", "", ""
	}
	r := p.Address.Original[f.Fragment]
	var faces []string
	if r.Page {
		faces = append(faces, "page")
	}
	if r.Judge {
		faces = append(faces, "judge")
	}
	if r.Found {
		faces = append(faces, "excerpt")
	}
	if len(faces) == 0 {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", strings.Join(faces, "+")
}

type agreementNativeCodeAddressControl struct {
	Name, Body        string
	Source            agreementNativeAddressSource
	Owners            map[string][]agreementNativeCodeAddressOwner
	Names             []string
	Original, Witness map[string]agreementNativeAddressReceipt
	Payload           agreementNativeCodeAddressPayload
	Owned             map[string]int
}

func agreementNativeCodeAddressControls() []agreementNativeCodeAddressControl {
	base := agreementNativeReservationControls()
	source0 := agreementNativeAddressSource{Original: base[0].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners0 := map[string][]agreementNativeCodeAddressOwner{}
	names0 := []string(nil)
	original0 := map[string]agreementNativeAddressReceipt(nil)
	witness0 := map[string]agreementNativeAddressReceipt(nil)
	payload0 := agreementNativeCodeAddressPayload{}
	source1 := agreementNativeAddressSource{Original: base[1].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners1 := map[string][]agreementNativeCodeAddressOwner{}
	names1 := []string(nil)
	original1 := map[string]agreementNativeAddressReceipt(nil)
	witness1 := map[string]agreementNativeAddressReceipt(nil)
	payload1 := agreementNativeCodeAddressPayload{}
	source2 := agreementNativeAddressSource{Original: base[2].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners2 := map[string][]agreementNativeCodeAddressOwner{}
	names2 := []string(nil)
	original2 := map[string]agreementNativeAddressReceipt(nil)
	witness2 := map[string]agreementNativeAddressReceipt(nil)
	payload2 := agreementNativeCodeAddressPayload{}
	source3 := agreementNativeAddressSource{Original: base[3].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners3 := map[string][]agreementNativeCodeAddressOwner{}
	names3 := []string(nil)
	original3 := map[string]agreementNativeAddressReceipt(nil)
	witness3 := map[string]agreementNativeAddressReceipt(nil)
	payload3 := agreementNativeCodeAddressPayload{}
	source4 := agreementNativeAddressSource{Original: base[4].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners4 := map[string][]agreementNativeCodeAddressOwner{}
	names4 := []string(nil)
	original4 := map[string]agreementNativeAddressReceipt(nil)
	witness4 := map[string]agreementNativeAddressReceipt(nil)
	payload4 := agreementNativeCodeAddressPayload{}
	source5 := agreementNativeAddressSource{Original: base[5].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners5 := map[string][]agreementNativeCodeAddressOwner{}
	names5 := []string(nil)
	original5 := map[string]agreementNativeAddressReceipt(nil)
	witness5 := map[string]agreementNativeAddressReceipt(nil)
	payload5 := agreementNativeCodeAddressPayload{}
	source6 := agreementNativeAddressSource{Original: base[6].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners6 := map[string][]agreementNativeCodeAddressOwner{}
	names6 := []string(nil)
	original6 := map[string]agreementNativeAddressReceipt(nil)
	witness6 := map[string]agreementNativeAddressReceipt(nil)
	payload6 := agreementNativeCodeAddressPayload{}
	source7 := agreementNativeAddressSource{Original: base[7].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[7].Input.Source.Original.Native, Check: base[7].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 5}, Token: graph.Span{Start: 3, Stop: 5}, Raw: "\\` ^A", Name: "^A", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 59, Stop: 71}, Token: graph.Span{Start: 67, Stop: 71}, Raw: "\ue0000\ue001 ^qqq", Name: "^qqq", Folded: "^qqq", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^qqq", "^é"}}, Body: "\\` ^A\n\t```\n%%![[image.png]]  ```\n`[[A]]`## <em>A</em>\n~~~~\n\ue0000\ue001 ^qqq\n ```\n   ```\n1. ``` [[A]]\n`    ", Changes: []agreementNativeAddressChange{{Old: "^a-2", New: "^qqq"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 5}, Token: graph.Span{Start: 3, Stop: 5}, Raw: "\\` ^A", Name: "^A", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 59, Stop: 71}, Token: graph.Span{Start: 67, Stop: 71}, Raw: "\ue0000\ue001 ^???", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 5}, Token: graph.Span{Start: 3, Stop: 5}, Raw: "\\` ^A", Name: "^A", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 59, Stop: 71}, Token: graph.Span{Start: 67, Stop: 71}, Raw: "\ue0000\ue001 ^???", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners7 := map[string][]agreementNativeCodeAddressOwner{"^a-2": {{Field: 1, Code: 2}}}
	names7 := []string{"^a", "^a-2", "^agreement-absent-0", "^qqq", "^é"}
	original7 := map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: false, Found: true, Cut: "\\` ^A\n\t```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^qqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness7 := map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: false, Found: true, Cut: "\\` ^A\n\t```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^qqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload7 := agreementNativeCodeAddressPayload{}
	source8 := agreementNativeAddressSource{Original: base[8].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners8 := map[string][]agreementNativeCodeAddressOwner{}
	names8 := []string(nil)
	original8 := map[string]agreementNativeAddressReceipt(nil)
	witness8 := map[string]agreementNativeAddressReceipt(nil)
	payload8 := agreementNativeCodeAddressPayload{}
	source9 := agreementNativeAddressSource{Original: base[9].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[9].Input.Source.Original.Native, Check: base[9].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 209, Stop: 218}, Token: graph.Span{Start: 216, Stop: 218}, Raw: "       ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "[[A]] [[A]]\t```\n1.  ^é\n[[A\\|alias]] ```\n    ```\n\n## A\n   ```\nA\n===\n\\[[A]]- > [!note] title\n    ```\n## [[A|alias]]\n## <em>A</em>\n- item\n\n      [[A#^a]] ```\n- [ ] [[A]]\n ```\n>   ```\n## !\n<!--\n[[A]]\n-->- item\n\n       ^q\n``![[A#A]]", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 209, Stop: 218}, Token: graph.Span{Start: 216, Stop: 218}, Raw: "       ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 209, Stop: 218}, Token: graph.Span{Start: 216, Stop: 218}, Raw: "       ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners9 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 2}}}
	names9 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original9 := map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness9 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: true, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload9 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "[[A]] [[A]]\t```\n1.  ^é\n[[A\\|alias]] ```\n    ```\n\n## A\n   ```\nA\n===\n\\[[A]]- > [!note] title\n    ```\n## [[A|alias]]\n## <em>A</em>\n- item\n\n      [[A#^a]] ```\n- [ ] [[A]]\n ```\n>   ```\n## !\n<!--\n[[A]]\n-->- item\n\n       ^a\n``![[A#A]]", Source: source9, Names: names9, Blocks: []string{"^a"}, WitnessBlocks: []string{"^q"}, Original: original9, Witness: witness9, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=page-not-excerpt multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=page-not-judge multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1}}, Owners: owners9}
	source10 := agreementNativeAddressSource{Original: base[10].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners10 := map[string][]agreementNativeCodeAddressOwner{}
	names10 := []string(nil)
	original10 := map[string]agreementNativeAddressReceipt(nil)
	witness10 := map[string]agreementNativeAddressReceipt(nil)
	payload10 := agreementNativeCodeAddressPayload{}
	source11 := agreementNativeAddressSource{Original: base[11].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[11].Input.Source.Original.Native, Check: base[11].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 60, Stop: 76}, Token: graph.Span{Start: 74, Stop: 76}, Raw: "[[image.png]] ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 170, Stop: 178}, Token: graph.Span{Start: 176, Stop: 178}, Raw: "\t     ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 179, Stop: 184}, Token: graph.Span{Start: 180, Stop: 184}, Raw: " ^qqq", Name: "^qqq", Folded: "^qqq", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^qqq", "^é"}}, Body: ">   ```\n```\n\\\\[[A]][^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]\\[[A]]~~~~\n[[image.png]] ^q\n## A-2\n## A\n## A\n[[A\\]]## <em>A</em>\nB\n> [!unknown] title\n\n\n    ```\n<!--\n[[A]]\n--># A\n\n> ~~~\n\t     ^q\n ^qqq\n\\`## A-2\n## A\n## A\n\t- [x] [[B]]\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}, {Old: "^a-2", New: "^qqq"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 60, Stop: 76}, Token: graph.Span{Start: 74, Stop: 76}, Raw: "[[image.png]] ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 170, Stop: 178}, Token: graph.Span{Start: 176, Stop: 178}, Raw: "\t     ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 179, Stop: 184}, Token: graph.Span{Start: 180, Stop: 184}, Raw: " ^???", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 60, Stop: 76}, Token: graph.Span{Start: 74, Stop: 76}, Raw: "[[image.png]] ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 170, Stop: 178}, Token: graph.Span{Start: 176, Stop: 178}, Raw: "\t     ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 179, Stop: 184}, Token: graph.Span{Start: 180, Stop: 184}, Raw: " ^???", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners11 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}, {Field: 1, Code: 0}}, "^a-2": {{Field: 2, Code: 0}}}
	names11 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^qqq", "^é"}
	original11 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "    [[B]]\n\\\\[[A]]\\[[A]]~~~~\n[[image.png]] ^A\n## A-2\n## A\n## A\n[[A\\]]## <em>A</em>\nB\n> [!unknown] title"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^qqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness11 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: true, Cut: "    [[B]]\n\\\\[[A]]\\[[A]]~~~~\n[[image.png]] ^q\n## A-2\n## A\n## A\n[[A\\]]## <em>A</em>\nB\n> [!unknown] title"}, "^qqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload11 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: ">   ```\n```\n\\\\[[A]][^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]\\[[A]]~~~~\n[[image.png]] ^A\n## A-2\n## A\n## A\n[[A\\]]## <em>A</em>\nB\n> [!unknown] title\n\n\n    ```\n<!--\n[[A]]\n--># A\n\n> ~~~\n\t     ^A\n ^a-2\n\\`## A-2\n## A\n## A\n\t- [x] [[B]]\n", Source: source11, Names: names11, Blocks: []string(nil), WitnessBlocks: []string(nil), Original: original11, Witness: witness11, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n\\\\\\\\[[A]]\\\\[[A]]~~~~\\n[[image.png]] ^A\\n## A-2\\n## A\\n## A\\n[[A\\\\]]## <em>A</em>\\nB\\n> [!unknown] title\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n\\\\\\\\[[A]]\\\\[[A]]~~~~\\n[[image.png]] ^A\\n## A-2\\n## A\\n## A\\n[[A\\\\]]## <em>A</em>\\nB\\n> [!unknown] title\" page=false judge=true excerpt=true": 1}}, Owners: owners11}
	source12 := agreementNativeAddressSource{Original: base[12].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners12 := map[string][]agreementNativeCodeAddressOwner{}
	names12 := []string(nil)
	original12 := map[string]agreementNativeAddressReceipt(nil)
	witness12 := map[string]agreementNativeAddressReceipt(nil)
	payload12 := agreementNativeCodeAddressPayload{}
	source13 := agreementNativeAddressSource{Original: base[13].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners13 := map[string][]agreementNativeCodeAddressOwner{}
	names13 := []string(nil)
	original13 := map[string]agreementNativeAddressReceipt(nil)
	witness13 := map[string]agreementNativeAddressReceipt(nil)
	payload13 := agreementNativeCodeAddressPayload{}
	source14 := agreementNativeAddressSource{Original: base[14].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners14 := map[string][]agreementNativeCodeAddressOwner{}
	names14 := []string(nil)
	original14 := map[string]agreementNativeAddressReceipt(nil)
	witness14 := map[string]agreementNativeAddressReceipt(nil)
	payload14 := agreementNativeCodeAddressPayload{}
	source15 := agreementNativeAddressSource{Original: base[15].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners15 := map[string][]agreementNativeCodeAddressOwner{}
	names15 := []string(nil)
	original15 := map[string]agreementNativeAddressReceipt(nil)
	witness15 := map[string]agreementNativeAddressReceipt(nil)
	payload15 := agreementNativeCodeAddressPayload{}
	source16 := agreementNativeAddressSource{Original: base[16].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners16 := map[string][]agreementNativeCodeAddressOwner{}
	names16 := []string(nil)
	original16 := map[string]agreementNativeAddressReceipt(nil)
	witness16 := map[string]agreementNativeAddressReceipt(nil)
	payload16 := agreementNativeCodeAddressPayload{}
	source17 := agreementNativeAddressSource{Original: base[17].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[17].Input.Source.Original.Native, Check: base[17].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 78, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: " ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "```\nÉ\n> ````\n\\[[A]]A\n===\n^absent-prose \\\\[[A]]\n\ue0020\ue003[[A#^a]]## [[A|alias]]\n ^q\n```\n- item\n\n      > [!note] [[A]]\n%%[[A]]%%\\\\[[A]]<div>\n[[A]]\n</div>\n\ue0020\ue003text  ", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 78, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: " ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 78, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: " ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners17 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}}
	names17 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original17 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "```\nÉ\n> ````\n\\[[A]]A\n===\n^absent-prose \\\\[[A]]\n\ue0020\ue003[[A#^a]]## [[A|alias]]\n ^A\n```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness17 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: true, Cut: "```\nÉ\n> ````\n\\[[A]]A\n===\n^absent-prose \\\\[[A]]\n\ue0020\ue003[[A#^a]]## [[A|alias]]\n ^q\n```"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload17 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "```\nÉ\n> ````\n\\[[A]]A\n===\n^absent-prose \\\\[[A]]\n\ue0020\ue003[[A#^a]]## [[A|alias]]\n ^A\n```\n- item\n\n      > [!note] [[A]]\n%%[[A]]%%\\\\[[A]]<div>\n[[A]]\n</div>\n\ue0020\ue003text  ", Source: source17, Names: names17, Blocks: []string(nil), WitnessBlocks: []string(nil), Original: original17, Witness: witness17, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nÉ\\n> ````\\n\\\\[[A]]A\\n===\\n^absent-prose \\\\\\\\[[A]]\\n\\ue0020\\ue003[[A#^a]]## [[A|alias]]\\n ^A\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nÉ\\n> ````\\n\\\\[[A]]A\\n===\\n^absent-prose \\\\\\\\[[A]]\\n\\ue0020\\ue003[[A#^a]]## [[A|alias]]\\n ^A\\n```\" page=false judge=true excerpt=true": 1}}, Owners: owners17}
	source18 := agreementNativeAddressSource{Original: base[18].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners18 := map[string][]agreementNativeCodeAddressOwner{}
	names18 := []string(nil)
	original18 := map[string]agreementNativeAddressReceipt(nil)
	witness18 := map[string]agreementNativeAddressReceipt(nil)
	payload18 := agreementNativeCodeAddressPayload{}
	source19 := agreementNativeAddressSource{Original: base[19].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners19 := map[string][]agreementNativeCodeAddressOwner{}
	names19 := []string(nil)
	original19 := map[string]agreementNativeAddressReceipt(nil)
	witness19 := map[string]agreementNativeAddressReceipt(nil)
	payload19 := agreementNativeCodeAddressPayload{}
	source20 := agreementNativeAddressSource{Original: base[20].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners20 := map[string][]agreementNativeCodeAddressOwner{}
	names20 := []string(nil)
	original20 := map[string]agreementNativeAddressReceipt(nil)
	witness20 := map[string]agreementNativeAddressReceipt(nil)
	payload20 := agreementNativeCodeAddressPayload{}
	source21 := agreementNativeAddressSource{Original: base[21].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners21 := map[string][]agreementNativeCodeAddressOwner{}
	names21 := []string(nil)
	original21 := map[string]agreementNativeAddressReceipt(nil)
	witness21 := map[string]agreementNativeAddressReceipt(nil)
	payload21 := agreementNativeCodeAddressPayload{}
	source22 := agreementNativeAddressSource{Original: base[22].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners22 := map[string][]agreementNativeCodeAddressOwner{}
	names22 := []string(nil)
	original22 := map[string]agreementNativeAddressReceipt(nil)
	witness22 := map[string]agreementNativeAddressReceipt(nil)
	payload22 := agreementNativeCodeAddressPayload{}
	source23 := agreementNativeAddressSource{Original: base[23].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners23 := map[string][]agreementNativeCodeAddressOwner{}
	names23 := []string(nil)
	original23 := map[string]agreementNativeAddressReceipt(nil)
	witness23 := map[string]agreementNativeAddressReceipt(nil)
	payload23 := agreementNativeCodeAddressPayload{}
	source24 := agreementNativeAddressSource{Original: base[24].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners24 := map[string][]agreementNativeCodeAddressOwner{}
	names24 := []string(nil)
	original24 := map[string]agreementNativeAddressReceipt(nil)
	witness24 := map[string]agreementNativeAddressReceipt(nil)
	payload24 := agreementNativeCodeAddressPayload{}
	source25 := agreementNativeAddressSource{Original: base[25].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners25 := map[string][]agreementNativeCodeAddressOwner{}
	names25 := []string(nil)
	original25 := map[string]agreementNativeAddressReceipt(nil)
	witness25 := map[string]agreementNativeAddressReceipt(nil)
	payload25 := agreementNativeCodeAddressPayload{}
	source26 := agreementNativeAddressSource{Original: base[26].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners26 := map[string][]agreementNativeCodeAddressOwner{}
	names26 := []string(nil)
	original26 := map[string]agreementNativeAddressReceipt(nil)
	witness26 := map[string]agreementNativeAddressReceipt(nil)
	payload26 := agreementNativeCodeAddressPayload{}
	source27 := agreementNativeAddressSource{Original: base[27].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners27 := map[string][]agreementNativeCodeAddressOwner{}
	names27 := []string(nil)
	original27 := map[string]agreementNativeAddressReceipt(nil)
	witness27 := map[string]agreementNativeAddressReceipt(nil)
	payload27 := agreementNativeCodeAddressPayload{}
	source28 := agreementNativeAddressSource{Original: base[28].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[28].Input.Source.Original.Native, Check: base[28].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 112, Stop: 125}, Token: graph.Span{Start: 123, Stop: 125}, Raw: "%%![[A#A]] ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "- > [!note] title\n## !\n  -  ^é\n\\`![[image.png]]<!-- [[A]] -->`[[A#^a]][[A\\|alias]][[A#^a]]<!--\n[[A]]\n-->-  ```\n%%![[A#A]] ^q\nref[^n]\n`\t[[A]]ref[^n]\n<!--\n[[A]]\n-->``` [[A]]\n\t# A\n\n^absent-prose É\n1. É\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 112, Stop: 125}, Token: graph.Span{Start: 123, Stop: 125}, Raw: "%%![[A#A]] ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 112, Stop: 125}, Token: graph.Span{Start: 123, Stop: 125}, Raw: "%%![[A#A]] ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners28 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}}
	names28 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original28 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness28 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload28 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "- > [!note] title\n## !\n  -  ^é\n\\`![[image.png]]<!-- [[A]] -->`[[A#^a]][[A\\|alias]][[A#^a]]<!--\n[[A]]\n-->-  ```\n%%![[A#A]] ^a\nref[^n]\n`\t[[A]]ref[^n]\n<!--\n[[A]]\n-->``` [[A]]\n\t# A\n\n^absent-prose É\n1. É\n", Source: source28, Names: names28, Blocks: []string(nil), WitnessBlocks: []string(nil), Original: original28, Witness: witness28, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=true excerpt=false": 1}}, Owners: owners28}
	source29 := agreementNativeAddressSource{Original: base[29].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners29 := map[string][]agreementNativeCodeAddressOwner{}
	names29 := []string(nil)
	original29 := map[string]agreementNativeAddressReceipt(nil)
	witness29 := map[string]agreementNativeAddressReceipt(nil)
	payload29 := agreementNativeCodeAddressPayload{}
	source30 := agreementNativeAddressSource{Original: base[30].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[30].Input.Source.Original.Native, Check: base[30].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 94, Stop: 103}, Token: graph.Span{Start: 101, Stop: 103}, Raw: "B]]1.  ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "\\`É\n章節\n%%text `open\n[[A]]\nclose`## A\n```\n%%[[A]]%%\\\\[[A]]# A\n[[B|alias]]  - \\` ^é\n[[A\nB]]1.  ^q\n- [ ] [[A]]\n^absent-prose > [[A]] [[A]]B\n![[image.png]]-->\ue0020\ue003```\n~~~~\n<div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 94, Stop: 103}, Token: graph.Span{Start: 101, Stop: 103}, Raw: "B]]1.  ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 94, Stop: 103}, Token: graph.Span{Start: 101, Stop: 103}, Raw: "B]]1.  ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners30 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 1}}}
	names30 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original30 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness30 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload30 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "\\`É\n章節\n%%text `open\n[[A]]\nclose`## A\n```\n%%[[A]]%%\\\\[[A]]# A\n[[B|alias]]  - \\` ^é\n[[A\nB]]1.  ^A\n- [ ] [[A]]\n^absent-prose > [[A]] [[A]]B\n![[image.png]]-->\ue0020\ue003```\n~~~~\n<div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n", Source: source30, Names: names30, Blocks: []string(nil), WitnessBlocks: []string(nil), Original: original30, Witness: witness30, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=true excerpt=false": 1}}, Owners: owners30}
	source31 := agreementNativeAddressSource{Original: base[31].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners31 := map[string][]agreementNativeCodeAddressOwner{}
	names31 := []string(nil)
	original31 := map[string]agreementNativeAddressReceipt(nil)
	witness31 := map[string]agreementNativeAddressReceipt(nil)
	payload31 := agreementNativeCodeAddressPayload{}
	source32 := agreementNativeAddressSource{Original: base[32].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners32 := map[string][]agreementNativeCodeAddressOwner{}
	names32 := []string(nil)
	original32 := map[string]agreementNativeAddressReceipt(nil)
	witness32 := map[string]agreementNativeAddressReceipt(nil)
	payload32 := agreementNativeCodeAddressPayload{}
	source33 := agreementNativeAddressSource{Original: base[33].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners33 := map[string][]agreementNativeCodeAddressOwner{}
	names33 := []string(nil)
	original33 := map[string]agreementNativeAddressReceipt(nil)
	witness33 := map[string]agreementNativeAddressReceipt(nil)
	payload33 := agreementNativeCodeAddressPayload{}
	source34 := agreementNativeAddressSource{Original: base[34].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners34 := map[string][]agreementNativeCodeAddressOwner{}
	names34 := []string(nil)
	original34 := map[string]agreementNativeAddressReceipt(nil)
	witness34 := map[string]agreementNativeAddressReceipt(nil)
	payload34 := agreementNativeCodeAddressPayload{}
	source35 := agreementNativeAddressSource{Original: base[35].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners35 := map[string][]agreementNativeCodeAddressOwner{}
	names35 := []string(nil)
	original35 := map[string]agreementNativeAddressReceipt(nil)
	witness35 := map[string]agreementNativeAddressReceipt(nil)
	payload35 := agreementNativeCodeAddressPayload{}
	source36 := agreementNativeAddressSource{Original: base[36].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners36 := map[string][]agreementNativeCodeAddressOwner{}
	names36 := []string(nil)
	original36 := map[string]agreementNativeAddressReceipt(nil)
	witness36 := map[string]agreementNativeAddressReceipt(nil)
	payload36 := agreementNativeCodeAddressPayload{}
	source37 := agreementNativeAddressSource{Original: base[37].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners37 := map[string][]agreementNativeCodeAddressOwner{}
	names37 := []string(nil)
	original37 := map[string]agreementNativeAddressReceipt(nil)
	witness37 := map[string]agreementNativeAddressReceipt(nil)
	payload37 := agreementNativeCodeAddressPayload{}
	source38 := agreementNativeAddressSource{Original: base[38].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners38 := map[string][]agreementNativeCodeAddressOwner{}
	names38 := []string(nil)
	original38 := map[string]agreementNativeAddressReceipt(nil)
	witness38 := map[string]agreementNativeAddressReceipt(nil)
	payload38 := agreementNativeCodeAddressPayload{}
	source39 := agreementNativeAddressSource{Original: base[39].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners39 := map[string][]agreementNativeCodeAddressOwner{}
	names39 := []string(nil)
	original39 := map[string]agreementNativeAddressReceipt(nil)
	witness39 := map[string]agreementNativeAddressReceipt(nil)
	payload39 := agreementNativeCodeAddressPayload{}
	source40 := agreementNativeAddressSource{Original: base[40].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners40 := map[string][]agreementNativeCodeAddressOwner{}
	names40 := []string(nil)
	original40 := map[string]agreementNativeAddressReceipt(nil)
	witness40 := map[string]agreementNativeAddressReceipt(nil)
	payload40 := agreementNativeCodeAddressPayload{}
	source41 := agreementNativeAddressSource{Original: base[41].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners41 := map[string][]agreementNativeCodeAddressOwner{}
	names41 := []string(nil)
	original41 := map[string]agreementNativeAddressReceipt(nil)
	witness41 := map[string]agreementNativeAddressReceipt(nil)
	payload41 := agreementNativeCodeAddressPayload{}
	source42 := agreementNativeAddressSource{Original: base[42].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners42 := map[string][]agreementNativeCodeAddressOwner{}
	names42 := []string(nil)
	original42 := map[string]agreementNativeAddressReceipt(nil)
	witness42 := map[string]agreementNativeAddressReceipt(nil)
	payload42 := agreementNativeCodeAddressPayload{}
	source43 := agreementNativeAddressSource{Original: base[43].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners43 := map[string][]agreementNativeCodeAddressOwner{}
	names43 := []string(nil)
	original43 := map[string]agreementNativeAddressReceipt(nil)
	witness43 := map[string]agreementNativeAddressReceipt(nil)
	payload43 := agreementNativeCodeAddressPayload{}
	source44 := agreementNativeAddressSource{Original: base[44].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[44].Input.Source.Original.Native, Check: base[44].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 23}, Token: graph.Span{Start: 18, Stop: 23}, Raw: "ghost [[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 30, Stop: 44}, Token: graph.Span{Start: 36, Stop: 44}, Raw: "words ^qqqqqqq", Name: "^qqqqqqq", Folded: "^qqqqqqq", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 52, Stop: 69}, Token: graph.Span{Start: 61, Stop: 69}, Raw: "%%hidden ^comment", Name: "^comment", Folded: "^comment", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 74, Stop: 94}, Token: graph.Span{Start: 92, Stop: 94}, Raw: "[^n]: [[B]] words ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 95, Stop: 108}, Token: graph.Span{Start: 106, Stop: 108}, Raw: "    second ^b", Name: "^b", Folded: "^b", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 110, Stop: 127}, Token: graph.Span{Start: 125, Stop: 127}, Raw: "    ## heading ^d", Name: "^d", Folded: "^d", Owner: 0, Nodes: []int{8}, Code: false, Comment: false}, {Line: graph.Span{Start: 129, Stop: 143}, Token: graph.Span{Start: 141, Stop: 143}, Raw: "    > words ^e", Name: "^e", Folded: "^e", Owner: 0, Nodes: []int{12}, Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^b", "^comment", "^d", "^e", "^live", "^qqqqqqq", "^é"}}, Body: "ghost [[A]] words ^live\n`open\nwords ^qqqqqqq\nclose`\n%%hidden ^comment\n%%\n\n[^n]: [[B]] words ^a\n    second ^b\n\n    ## heading ^d\n\n    > words ^e\n", Changes: []agreementNativeAddressChange{{Old: "^literal", New: "^qqqqqqq"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 23}, Token: graph.Span{Start: 18, Stop: 23}, Raw: "ghost [[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 30, Stop: 44}, Token: graph.Span{Start: 36, Stop: 44}, Raw: "words ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 52, Stop: 69}, Token: graph.Span{Start: 61, Stop: 69}, Raw: "%%hidden ^comment", Name: "^comment", Folded: "^comment", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 74, Stop: 94}, Token: graph.Span{Start: 92, Stop: 94}, Raw: "[^n]: [[B]] words ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 95, Stop: 108}, Token: graph.Span{Start: 106, Stop: 108}, Raw: "    second ^b", Name: "^b", Folded: "^b", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 110, Stop: 127}, Token: graph.Span{Start: 125, Stop: 127}, Raw: "    ## heading ^d", Name: "^d", Folded: "^d", Owner: 0, Nodes: []int{8}, Code: false, Comment: false}, {Line: graph.Span{Start: 129, Stop: 143}, Token: graph.Span{Start: 141, Stop: 143}, Raw: "    > words ^e", Name: "^e", Folded: "^e", Owner: 0, Nodes: []int{12}, Code: false, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 23}, Token: graph.Span{Start: 18, Stop: 23}, Raw: "ghost [[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 30, Stop: 44}, Token: graph.Span{Start: 36, Stop: 44}, Raw: "words ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 52, Stop: 69}, Token: graph.Span{Start: 61, Stop: 69}, Raw: "%%hidden ^comment", Name: "^comment", Folded: "^comment", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 74, Stop: 94}, Token: graph.Span{Start: 92, Stop: 94}, Raw: "[^n]: [[B]] words ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 95, Stop: 108}, Token: graph.Span{Start: 106, Stop: 108}, Raw: "    second ^b", Name: "^b", Folded: "^b", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 110, Stop: 127}, Token: graph.Span{Start: 125, Stop: 127}, Raw: "    ## heading ^d", Name: "^d", Folded: "^d", Owner: 0, Nodes: []int{8}, Code: false, Comment: false}, {Line: graph.Span{Start: 129, Stop: 143}, Token: graph.Span{Start: 141, Stop: 143}, Raw: "    > words ^e", Name: "^e", Folded: "^e", Owner: 0, Nodes: []int{12}, Code: false, Comment: false}}, Valid: true}
	owners44 := map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 0}}}
	names44 := []string{"^a", "^a-2", "^agreement-absent-0", "^b", "^comment", "^d", "^e", "^literal", "^live", "^qqqqqqq", "^é"}
	original44 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] words ^a\n    second ^b"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] words ^a\n    second ^b"}, "^comment": {Page: false, Judge: false, Found: false, Cut: ""}, "^d": {Page: false, Judge: true, Found: true, Cut: "    ## heading ^d"}, "^e": {Page: false, Judge: true, Found: true, Cut: "    > words ^e"}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "ghost [[A]] words ^live\n`open\nwords ^literal\nclose`"}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness44 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] words ^a\n    second ^b"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] words ^a\n    second ^b"}, "^comment": {Page: false, Judge: false, Found: false, Cut: ""}, "^d": {Page: false, Judge: true, Found: true, Cut: "    ## heading ^d"}, "^e": {Page: false, Judge: true, Found: true, Cut: "    > words ^e"}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "ghost [[A]] words ^live\n`open\nwords ^qqqqqqq\nclose`"}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload44 := agreementNativeCodeAddressPayload{}
	source45 := agreementNativeAddressSource{Original: base[45].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners45 := map[string][]agreementNativeCodeAddressOwner{}
	names45 := []string(nil)
	original45 := map[string]agreementNativeAddressReceipt(nil)
	witness45 := map[string]agreementNativeAddressReceipt(nil)
	payload45 := agreementNativeCodeAddressPayload{}
	source46 := agreementNativeAddressSource{Original: base[46].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners46 := map[string][]agreementNativeCodeAddressOwner{}
	names46 := []string(nil)
	original46 := map[string]agreementNativeAddressReceipt(nil)
	witness46 := map[string]agreementNativeAddressReceipt(nil)
	payload46 := agreementNativeCodeAddressPayload{}
	source47 := agreementNativeAddressSource{Original: base[47].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners47 := map[string][]agreementNativeCodeAddressOwner{}
	names47 := []string(nil)
	original47 := map[string]agreementNativeAddressReceipt(nil)
	witness47 := map[string]agreementNativeAddressReceipt(nil)
	payload47 := agreementNativeCodeAddressPayload{}
	source48 := agreementNativeAddressSource{Original: base[48].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners48 := map[string][]agreementNativeCodeAddressOwner{}
	names48 := []string(nil)
	original48 := map[string]agreementNativeAddressReceipt(nil)
	witness48 := map[string]agreementNativeAddressReceipt(nil)
	payload48 := agreementNativeCodeAddressPayload{}
	source49 := agreementNativeAddressSource{Original: base[49].Input.Source.Original, Projected: agreementNativeAddressPart{Native: base[49].Input.Source.Original.Native, Check: base[49].Input.Source.Original.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^qqqqqqq", Name: "^qqqqqqq", Folded: "^qqqqqqq", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 67, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: "[^n]: [[B]] ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 82, Stop: 95}, Token: graph.Span{Start: 93, Stop: 95}, Raw: "    second ^b", Name: "^b", Folded: "^b", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 97, Stop: 113}, Token: graph.Span{Start: 111, Stop: 113}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 115, Stop: 129}, Token: graph.Span{Start: 127, Stop: 129}, Raw: "other words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^b", "^hidden", "^live", "^qqqqqqq", "^é"}}, Body: "[[A]] words ^live\n`open\nliteral ^qqqqqqq\nclose`\n%%hide ^hidden\n%%\n\n[^n]: [[B]] ^a\n    second ^b\n\noutside words ^a\n\nother words ^b\n", Changes: []agreementNativeAddressChange{{Old: "^literal", New: "^qqqqqqq"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 67, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: "[^n]: [[B]] ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 82, Stop: 95}, Token: graph.Span{Start: 93, Stop: 95}, Raw: "    second ^b", Name: "^b", Folded: "^b", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 97, Stop: 113}, Token: graph.Span{Start: 111, Stop: 113}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 115, Stop: 129}, Token: graph.Span{Start: 127, Stop: 129}, Raw: "other words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 67, Stop: 81}, Token: graph.Span{Start: 79, Stop: 81}, Raw: "[^n]: [[B]] ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 82, Stop: 95}, Token: graph.Span{Start: 93, Stop: 95}, Raw: "    second ^b", Name: "^b", Folded: "^b", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 97, Stop: 113}, Token: graph.Span{Start: 111, Stop: 113}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 115, Stop: 129}, Token: graph.Span{Start: 127, Stop: 129}, Raw: "other words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Valid: true}
	owners49 := map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 0}}}
	names49 := []string{"^a", "^a-2", "^agreement-absent-0", "^b", "^hidden", "^literal", "^live", "^qqqqqqq", "^é"}
	original49 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^a\n    second ^b"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^a\n    second ^b"}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live\n`open\nliteral ^literal\nclose`"}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness49 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^a\n    second ^b"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] ^a\n    second ^b"}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live\n`open\nliteral ^qqqqqqq\nclose`"}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload49 := agreementNativeCodeAddressPayload{}
	source50 := agreementNativeAddressSource{Original: base[50].Input.Source.Original, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners50 := map[string][]agreementNativeCodeAddressOwner{}
	names50 := []string(nil)
	original50 := map[string]agreementNativeAddressReceipt(nil)
	witness50 := map[string]agreementNativeAddressReceipt(nil)
	payload50 := agreementNativeCodeAddressPayload{}
	sourceOriginal51 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 13}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 4, Stop: 13}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 4, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}
	source51 := agreementNativeAddressSource{Original: sourceOriginal51, Projected: agreementNativeAddressPart{Native: sourceOriginal51.Native, Check: sourceOriginal51.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 4, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "```\nwords ^q\n```\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 4, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 4, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners51 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}}
	names51 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original51 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness51 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload51 := agreementNativeCodeAddressPayload{}
	sourceOriginal52 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 24}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 4, Stop: 24}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}
	source52 := agreementNativeAddressSource{Original: sourceOriginal52, Projected: agreementNativeAddressPart{Native: sourceOriginal52.Native, Check: sourceOriginal52.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "```\nword\n> ```\nwords ^q\n```\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners52 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}}
	names52 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original52 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^a\n```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness52 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^q\n```"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload52 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "```\nword\n> ```\nwords ^a\n```\n", Source: source52, Names: names52, Blocks: []string(nil), WitnessBlocks: []string(nil), Original: original52, Witness: witness52, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true": 1}}, Owners: owners52}
	source53 := agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 24}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 4, Stop: 24}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 29, Stop: 45}, Token: graph.Span{Start: 43, Stop: 45}, Raw: "outside words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners53 := map[string][]agreementNativeCodeAddressOwner{}
	names53 := []string(nil)
	original53 := map[string]agreementNativeAddressReceipt(nil)
	witness53 := map[string]agreementNativeAddressReceipt(nil)
	payload53 := agreementNativeCodeAddressPayload{}
	sourceOriginal54 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 13}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 4, Stop: 13}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "    words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}
	source54 := agreementNativeAddressSource{Original: sourceOriginal54, Projected: agreementNativeAddressPart{Native: sourceOriginal54.Native, Check: sourceOriginal54.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "    words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "    words ^q\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "    words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 12}, Token: graph.Span{Start: 10, Stop: 12}, Raw: "    words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners54 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}}
	names54 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original54 := map[string]agreementNativeAddressReceipt{"^a": {Page: true, Judge: true, Found: true, Cut: "    words ^a"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness54 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: true, Judge: true, Found: true, Cut: "    words ^q"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload54 := agreementNativeCodeAddressPayload{}
	sourceOriginal55 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 1, Stop: 20}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 1, Stop: 20}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 6, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}
	source55 := agreementNativeAddressSource{Original: sourceOriginal55, Projected: agreementNativeAddressPart{Native: sourceOriginal55.Native, Check: sourceOriginal55.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 6, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "`open\nwords ^q\nclose`\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 6, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 6, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners55 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}}
	names55 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original55 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness55 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload55 := agreementNativeCodeAddressPayload{}
	source56 := agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlockInfo", Parent: "Blockquote", Span: graph.Span{Start: 6, Stop: 10}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 11, Stop: 19}, Token: graph.Span{Start: 17, Stop: 19}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners56 := map[string][]agreementNativeCodeAddressOwner{}
	names56 := []string(nil)
	original56 := map[string]agreementNativeAddressReceipt(nil)
	witness56 := map[string]agreementNativeAddressReceipt(nil)
	payload56 := agreementNativeCodeAddressPayload{}
	sourceOriginal57 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 33}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 4, Stop: 33}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^A", Name: "^A", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}
	source57 := agreementNativeAddressSource{Original: sourceOriginal57, Projected: agreementNativeAddressPart{Native: sourceOriginal57.Native, Check: sourceOriginal57.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "```\nword\n> ```\nwords ^q\nwords ^q\n```\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 32}, Token: graph.Span{Start: 30, Stop: 32}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners57 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}, {Field: 1, Code: 0}}}
	names57 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original57 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^A\nwords ^a\n```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness57 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^q\nwords ^q\n```"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload57 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "```\nword\n> ```\nwords ^A\nwords ^a\n```\n", Source: source57, Names: names57, Blocks: []string(nil), WitnessBlocks: []string(nil), Original: original57, Witness: witness57, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^A\\nwords ^a\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^A\\nwords ^a\\n```\" page=false judge=true excerpt=true": 1}}, Owners: owners57}
	sourceOriginal58 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 5, Stop: 28}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 5, Stop: 28}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 18, Stop: 26}, Token: graph.Span{Start: 24, Stop: 26}, Raw: "words ^A", Name: "^A", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}
	source58 := agreementNativeAddressSource{Original: sourceOriginal58, Projected: agreementNativeAddressPart{Native: sourceOriginal58.Native, Check: sourceOriginal58.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 18, Stop: 26}, Token: graph.Span{Start: 24, Stop: 26}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}, Body: "```\r\nword\r\n> ```\r\nwords ^q\r\n```\r\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 18, Stop: 26}, Token: graph.Span{Start: 24, Stop: 26}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 18, Stop: 26}, Token: graph.Span{Start: 24, Stop: 26}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners58 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}}
	names58 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^é"}
	original58 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness58 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload58 := agreementNativeCodeAddressPayload{}
	sourceOriginal59 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 19, Stop: 46}}, {Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 71, Stop: 100}}}, Comments: []graph.Span{{Start: 48, Stop: 65}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 19, Stop: 46}, {Start: 71, Stop: 100}}, Comments: []graph.Span{{Start: 48, Stop: 65}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^literal", Name: "^literal", Folded: "^literal", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 82, Stop: 90}, Token: graph.Span{Start: 88, Stop: 90}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 91, Stop: 99}, Token: graph.Span{Start: 97, Stop: 99}, Raw: "words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^b", "^hidden", "^literal", "^live", "^é"}}
	source59 := agreementNativeAddressSource{Original: sourceOriginal59, Projected: agreementNativeAddressPart{Native: sourceOriginal59.Native, Check: sourceOriginal59.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^qqqqqqq", Name: "^qqqqqqq", Folded: "^qqqqqqq", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 82, Stop: 90}, Token: graph.Span{Start: 88, Stop: 90}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 91, Stop: 99}, Token: graph.Span{Start: 97, Stop: 99}, Raw: "words ^z", Name: "^z", Folded: "^z", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^hidden", "^live", "^q", "^qqqqqqq", "^z", "^é"}}, Body: "[[A]] words ^live\n`open\nliteral ^qqqqqqq\nclose`\n%%hide ^hidden\n%%\n\n```\nword\n> ```\nwords ^q\nwords ^z\n```\n", Changes: []agreementNativeAddressChange{{Old: "^literal", New: "^qqqqqqq"}, {Old: "^a", New: "^q"}, {Old: "^b", New: "^z"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 82, Stop: 90}, Token: graph.Span{Start: 88, Stop: 90}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 91, Stop: 99}, Token: graph.Span{Start: 97, Stop: 99}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 24, Stop: 40}, Token: graph.Span{Start: 32, Stop: 40}, Raw: "literal ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 48, Stop: 62}, Token: graph.Span{Start: 55, Stop: 62}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 82, Stop: 90}, Token: graph.Span{Start: 88, Stop: 90}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 91, Stop: 99}, Token: graph.Span{Start: 97, Stop: 99}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners59 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 3, Code: 1}}, "^b": {{Field: 4, Code: 1}}, "^literal": {{Field: 1, Code: 0}}}
	names59 := []string{"^a", "^a-2", "^agreement-absent-0", "^b", "^hidden", "^literal", "^live", "^q", "^qqqqqqq", "^z", "^é"}
	original59 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^a\nwords ^b\n```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^a\nwords ^b\n```"}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live\n`open\nliteral ^literal\nclose`"}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^z": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness59 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: false, Found: false, Cut: ""}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live\n`open\nliteral ^qqqqqqq\nclose`"}, "^q": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^q\nwords ^z\n```"}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^z": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^q\nwords ^z\n```"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload59 := agreementNativeCodeAddressPayload{}
	sourceOriginal60 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 20, Stop: 40}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 20, Stop: 40}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[[A]] words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 31, Stop: 39}, Token: graph.Span{Start: 37, Stop: 39}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^é"}}
	source60 := agreementNativeAddressSource{Original: sourceOriginal60, Projected: agreementNativeAddressPart{Native: sourceOriginal60.Native, Check: sourceOriginal60.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[[A]] words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 31, Stop: 39}, Token: graph.Span{Start: 37, Stop: 39}, Raw: "words ^z", Name: "^z", Folded: "^z", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^z", "^é"}}, Body: "[[A]] words ^q\n\n```\nword\n> ```\nwords ^z\n```\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^z"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[[A]] words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 31, Stop: 39}, Token: graph.Span{Start: 37, Stop: 39}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 14}, Token: graph.Span{Start: 12, Stop: 14}, Raw: "[[A]] words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 31, Stop: 39}, Token: graph.Span{Start: 37, Stop: 39}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners60 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 1, Code: 0}}}
	names60 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^z", "^é"}
	original60 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^a\n```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^q"}, "^z": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness60 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^q"}, "^z": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^z\n```"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload60 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "[[A]] words ^q\n\n```\nword\n> ```\nwords ^a\n```\n", Source: source60, Names: names60, Blocks: []string{"^q"}, WitnessBlocks: []string{"^q"}, Original: original60, Witness: witness60, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true": 1}}, Owners: owners60}
	source61 := agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 50, Stop: 70}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 50, Stop: 70}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 8}, Token: graph.Span{Start: 6, Stop: 8}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 9, Stop: 17}, Token: graph.Span{Start: 15, Stop: 17}, Raw: "words ^z", Name: "^z", Folded: "^z", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 18, Stop: 26}, Token: graph.Span{Start: 24, Stop: 26}, Raw: "words ^v", Name: "^v", Folded: "^v", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 27, Stop: 35}, Token: graph.Span{Start: 33, Stop: 35}, Raw: "words ^x", Name: "^x", Folded: "^x", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 36, Stop: 44}, Token: graph.Span{Start: 42, Stop: 44}, Raw: "words ^w", Name: "^w", Folded: "^w", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 61, Stop: 69}, Token: graph.Span{Start: 67, Stop: 69}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^v", "^w", "^x", "^z", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners61 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 5, Code: 0}}}
	names61 := []string(nil)
	original61 := map[string]agreementNativeAddressReceipt(nil)
	witness61 := map[string]agreementNativeAddressReceipt(nil)
	payload61 := agreementNativeCodeAddressPayload{}
	source62 := agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 61}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 11}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 6, Stop: 11}}}, {Kind: "FencedCodeBlock", Depth: 1, Lines: []graph.Span{{Start: 25, Stop: 30}, {Start: 34, Stop: 40}, {Start: 44, Stop: 53}}, Text: []graph.Span(nil)}}}}, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Footnote", Span: graph.Span{Start: 25, Stop: 53}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 61}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 40, Stop: 52}, Token: graph.Span{Start: 50, Stop: 52}, Raw: "    words ^a", Name: "^a", Folded: "^a", Owner: 0, Nodes: []int{3}, Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners62 := map[string][]agreementNativeCodeAddressOwner{}
	names62 := []string(nil)
	original62 := map[string]agreementNativeAddressReceipt(nil)
	witness62 := map[string]agreementNativeAddressReceipt(nil)
	payload62 := agreementNativeCodeAddressPayload{}
	source63 := agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span{{Start: 0, Stop: 20}}}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span{{Start: 0, Stop: 20}}, Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 9, Stop: 17}, Token: graph.Span{Start: 15, Stop: 17}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}}, Names: []string{"^a", "^a-2", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners63 := map[string][]agreementNativeCodeAddressOwner{}
	names63 := []string(nil)
	original63 := map[string]agreementNativeAddressReceipt(nil)
	witness63 := map[string]agreementNativeAddressReceipt(nil)
	payload63 := agreementNativeCodeAddressPayload{}
	source64 := agreementNativeAddressSource{Original: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 14}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 4, Stop: 14}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string{"^a", "^a-2", "^é"}}, Projected: agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode(nil), Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span(nil), Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField(nil), Names: []string(nil)}, Body: "", Changes: []agreementNativeAddressChange(nil), Shapes: []agreementNativeAddressField(nil), AfterShapes: []agreementNativeAddressField(nil), Valid: false}
	owners64 := map[string][]agreementNativeCodeAddressOwner{}
	names64 := []string(nil)
	original64 := map[string]agreementNativeAddressReceipt(nil)
	witness64 := map[string]agreementNativeAddressReceipt(nil)
	payload64 := agreementNativeCodeAddressPayload{}
	sourceOriginal65 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition(nil), Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 4, Stop: 35}}}, Comments: []graph.Span(nil)}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 4, Stop: 35}}, Comments: []graph.Span(nil), Fields: []agreementNativeCheckField(nil), Targets: []string(nil)}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 34}, Token: graph.Span{Start: 30, Stop: 34}, Raw: "words ^a-2", Name: "^a-2", Folded: "^a-2", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^é"}}
	source65 := agreementNativeAddressSource{Original: sourceOriginal65, Projected: agreementNativeAddressPart{Native: sourceOriginal65.Native, Check: sourceOriginal65.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 34}, Token: graph.Span{Start: 30, Stop: 34}, Raw: "words ^qqq", Name: "^qqq", Folded: "^qqq", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Names: []string{"^a", "^a-2", "^q", "^qqq", "^é"}}, Body: "```\nword\n> ```\nwords ^q\nwords ^qqq\n```\n", Changes: []agreementNativeAddressChange{{Old: "^a", New: "^q"}, {Old: "^a-2", New: "^qqq"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 34}, Token: graph.Span{Start: 30, Stop: 34}, Raw: "words ^???", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 15, Stop: 23}, Token: graph.Span{Start: 21, Stop: 23}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 24, Stop: 34}, Token: graph.Span{Start: 30, Stop: 34}, Raw: "words ^???", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}}, Valid: true}
	owners65 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 0, Code: 0}}, "^a-2": {{Field: 1, Code: 0}}}
	names65 := []string{"^a", "^a-2", "^agreement-absent-0", "^q", "^qqq", "^é"}
	original65 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^a\nwords ^a-2\n```"}, "^a-2": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^a\nwords ^a-2\n```"}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^qqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness65 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^q": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^q\nwords ^qqq\n```"}, "^qqq": {Page: false, Judge: true, Found: true, Cut: "```\nword\n> ```\nwords ^q\nwords ^qqq\n```"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload65 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "```\nword\n> ```\nwords ^a\nwords ^a-2\n```\n", Source: source65, Names: names65, Blocks: []string(nil), WitnessBlocks: []string(nil), Original: original65, Witness: witness65, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a-2\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a-2\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true": 1}}, Owners: owners65}
	sourceOriginal66 := agreementNativeAddressPart{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 81, Stop: 137}, Name: graph.Span{Start: 81, Stop: 86}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 87, Stop: 106}, {Start: 110, Stop: 116}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 87, Stop: 88}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 88, Stop: 89}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 89, Stop: 98}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 98, Stop: 105}}}, {Kind: "Text", Depth: 2, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 110, Stop: 116}}}, {Kind: "Blockquote", Depth: 1, Lines: []graph.Span(nil), Text: []graph.Span(nil)}, {Kind: "Paragraph", Depth: 2, Lines: []graph.Span{{Start: 123, Stop: 135}}, Text: []graph.Span(nil)}, {Kind: "Text", Depth: 3, Lines: []graph.Span(nil), Text: []graph.Span{{Start: 123, Stop: 135}}}}}}, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 32, Stop: 59}}, {Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 149, Stop: 178}}}, Comments: []graph.Span{{Start: 62, Stop: 79}}}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 32, Stop: 59}, {Start: 149, Stop: 178}}, Comments: []graph.Span{{Start: 62, Stop: 79}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 5}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 87, Stop: 92}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}}, Targets: []string{"A", "B"}}, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 37, Stop: 53}, Token: graph.Span{Start: 45, Stop: 53}, Raw: "literal ^literal", Name: "^literal", Folded: "^literal", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 62, Stop: 76}, Token: graph.Span{Start: 69, Stop: 76}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 81, Stop: 105}, Token: graph.Span{Start: 99, Stop: 105}, Raw: "[^n]: [[B]] words ^owned", Name: "^owned", Folded: "^owned", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 160, Stop: 168}, Token: graph.Span{Start: 166, Stop: 168}, Raw: "words ^a", Name: "^a", Folded: "^a", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 169, Stop: 177}, Token: graph.Span{Start: 175, Stop: 177}, Raw: "words ^b", Name: "^b", Folded: "^b", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 192, Stop: 214}, Token: graph.Span{Start: 206, Stop: 214}, Raw: "outside words ^outside", Name: "^outside", Folded: "^outside", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^b", "^hidden", "^literal", "^live", "^outside", "^owned", "^é"}}
	source66 := agreementNativeAddressSource{Original: sourceOriginal66, Projected: agreementNativeAddressPart{Native: sourceOriginal66.Native, Check: sourceOriginal66.Check, Fields: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 37, Stop: 53}, Token: graph.Span{Start: 45, Stop: 53}, Raw: "literal ^qqqqqqq", Name: "^qqqqqqq", Folded: "^qqqqqqq", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 62, Stop: 76}, Token: graph.Span{Start: 69, Stop: 76}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 81, Stop: 105}, Token: graph.Span{Start: 99, Stop: 105}, Raw: "[^n]: [[B]] words ^owned", Name: "^owned", Folded: "^owned", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 160, Stop: 168}, Token: graph.Span{Start: 166, Stop: 168}, Raw: "words ^q", Name: "^q", Folded: "^q", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 169, Stop: 177}, Token: graph.Span{Start: 175, Stop: 177}, Raw: "words ^z", Name: "^z", Folded: "^z", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 192, Stop: 214}, Token: graph.Span{Start: 206, Stop: 214}, Raw: "outside words ^outside", Name: "^outside", Folded: "^outside", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Names: []string{"^a", "^a-2", "^hidden", "^live", "^outside", "^owned", "^q", "^qqqqqqq", "^z", "^é"}}, Body: "[[A]] words ^live\n\n## separate\n`open\nliteral ^qqqqqqq\nclose`\n\n%%hide ^hidden\n%%\n\n[^n]: [[B]] words ^owned\n    second\n    > continuation\n\n## code\n```\nword\n> ```\nwords ^q\nwords ^z\n```\n\n## after\noutside words ^outside\n", Changes: []agreementNativeAddressChange{{Old: "^literal", New: "^qqqqqqq"}, {Old: "^a", New: "^q"}, {Old: "^b", New: "^z"}}, Shapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 37, Stop: 53}, Token: graph.Span{Start: 45, Stop: 53}, Raw: "literal ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 62, Stop: 76}, Token: graph.Span{Start: 69, Stop: 76}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 81, Stop: 105}, Token: graph.Span{Start: 99, Stop: 105}, Raw: "[^n]: [[B]] words ^owned", Name: "^owned", Folded: "^owned", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 160, Stop: 168}, Token: graph.Span{Start: 166, Stop: 168}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 169, Stop: 177}, Token: graph.Span{Start: 175, Stop: 177}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 192, Stop: 214}, Token: graph.Span{Start: 206, Stop: 214}, Raw: "outside words ^outside", Name: "^outside", Folded: "^outside", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, AfterShapes: []agreementNativeAddressField{{Line: graph.Span{Start: 0, Stop: 17}, Token: graph.Span{Start: 12, Stop: 17}, Raw: "[[A]] words ^live", Name: "^live", Folded: "^live", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}, {Line: graph.Span{Start: 37, Stop: 53}, Token: graph.Span{Start: 45, Stop: 53}, Raw: "literal ^???????", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 62, Stop: 76}, Token: graph.Span{Start: 69, Stop: 76}, Raw: "%%hide ^hidden", Name: "^hidden", Folded: "^hidden", Owner: -1, Nodes: []int(nil), Code: false, Comment: true}, {Line: graph.Span{Start: 81, Stop: 105}, Token: graph.Span{Start: 99, Stop: 105}, Raw: "[^n]: [[B]] words ^owned", Name: "^owned", Folded: "^owned", Owner: 0, Nodes: []int{1}, Code: false, Comment: false}, {Line: graph.Span{Start: 160, Stop: 168}, Token: graph.Span{Start: 166, Stop: 168}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 169, Stop: 177}, Token: graph.Span{Start: 175, Stop: 177}, Raw: "words ^?", Name: "?", Folded: "?", Owner: -1, Nodes: []int(nil), Code: true, Comment: false}, {Line: graph.Span{Start: 192, Stop: 214}, Token: graph.Span{Start: 206, Stop: 214}, Raw: "outside words ^outside", Name: "^outside", Folded: "^outside", Owner: -1, Nodes: []int(nil), Code: false, Comment: false}}, Valid: true}
	owners66 := map[string][]agreementNativeCodeAddressOwner{"^a": {{Field: 4, Code: 1}}, "^b": {{Field: 5, Code: 1}}, "^literal": {{Field: 1, Code: 0}}}
	names66 := []string{"^a", "^a-2", "^agreement-absent-0", "^b", "^hidden", "^literal", "^live", "^outside", "^owned", "^q", "^qqqqqqq", "^z", "^é"}
	original66 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: true, Found: true, Cut: "## code\n```\nword\n> ```\nwords ^a\nwords ^b\n```"}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: true, Found: true, Cut: "## code\n```\nword\n> ```\nwords ^a\nwords ^b\n```"}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live"}, "^outside": {Page: true, Judge: false, Found: false, Cut: ""}, "^owned": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] words ^owned\n    second\n    > continuation"}, "^q": {Page: false, Judge: false, Found: false, Cut: ""}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^z": {Page: false, Judge: false, Found: false, Cut: ""}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	witness66 := map[string]agreementNativeAddressReceipt{"^a": {Page: false, Judge: false, Found: false, Cut: ""}, "^a-2": {Page: false, Judge: false, Found: false, Cut: ""}, "^agreement-absent-0": {Page: false, Judge: false, Found: false, Cut: ""}, "^b": {Page: false, Judge: false, Found: false, Cut: ""}, "^hidden": {Page: false, Judge: false, Found: false, Cut: ""}, "^literal": {Page: false, Judge: false, Found: false, Cut: ""}, "^live": {Page: true, Judge: true, Found: true, Cut: "[[A]] words ^live"}, "^outside": {Page: true, Judge: false, Found: false, Cut: ""}, "^owned": {Page: false, Judge: true, Found: true, Cut: "[^n]: [[B]] words ^owned\n    second\n    > continuation"}, "^q": {Page: false, Judge: true, Found: true, Cut: "## code\n```\nword\n> ```\nwords ^q\nwords ^z\n```"}, "^qqqqqqq": {Page: false, Judge: false, Found: false, Cut: ""}, "^z": {Page: false, Judge: true, Found: true, Cut: "## code\n```\nword\n> ```\nwords ^q\nwords ^z\n```"}, "^é": {Page: false, Judge: false, Found: false, Cut: ""}}
	payload66 := agreementNativeCodeAddressPayload{Address: agreementNativeAddressPayload{Body: "[[A]] words ^live\n\n## separate\n`open\nliteral ^literal\nclose`\n\n%%hide ^hidden\n%%\n\n[^n]: [[B]] words ^owned\n    second\n    > continuation\n\n## code\n```\nword\n> ```\nwords ^a\nwords ^b\n```\n\n## after\noutside words ^outside\n", Source: source66, Names: names66, Blocks: []string{"^live", "^outside"}, WitnessBlocks: []string{"^live", "^outside"}, Original: original66, Witness: witness66, Signatures: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=excerpt-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=judge-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^outside\" direction=page-not-excerpt multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^outside\" direction=page-not-judge multiplicity=1 cut=\"\" page=true judge=false excerpt=false": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^owned\" direction=excerpt-only multiplicity=1 cut=\"[^n]: [[B]] words ^owned\\n    second\\n    > continuation\" page=false judge=true excerpt=true": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^owned\" direction=judge-only multiplicity=1 cut=\"[^n]: [[B]] words ^owned\\n    second\\n    > continuation\" page=false judge=true excerpt=true": 1}}, Owners: owners66}
	return []agreementNativeCodeAddressControl{
		{Name: "complete original/control 1", Body: "text - [x] [[B]]\n`A\n---\n\\`## !\n![[A#A]]    ```\n`[[A]]`É\n> [!note] one\n> [!note] two\n> [!note] three\n- [x] [[B]]\n[^unused]: [[A]]\n ^A\n# A\n<!-- [[A]] -->> [!note] title\n ^a\n~~~\n`[[image.png]]", Source: source0, Owners: owners0, Names: names0, Original: original0, Witness: witness0, Payload: payload0, Owned: map[string]int{}},
		{Name: "complete original/control 2", Body: "[^unused]: [[A]]\n``open\n[[A]]\nclose`0  > [!note] title\n`open\n[[A]]\nclose````````\n ^A\nÉ\n<div>\n[[A]]\n</div>\nA\n---\n[[A]]## A\n ^A\n   ```\n`~~~~\n ^a-2\n  > [!note] title\nA\n===\n> [!unknown] title\n<!--A\n---\n", Source: source1, Owners: owners1, Names: names1, Original: original1, Witness: witness1, Payload: payload1, Owned: map[string]int{}},
		{Name: "complete original/control 3", Body: "[[A]]章節\nÉ\n章節\n[^n]: [[A]]\n\n    [[B]]\n ^A\n  - A\n===\n ^a\n```\n\n ^é\n%%## <em>A</em>\n ```\n![[image.png]]   ```\n%%[[image.png]]## !\n[[#A]]text ", Source: source2, Owners: owners2, Names: names2, Original: original2, Witness: witness2, Payload: payload2, Owned: map[string]int{}},
		{Name: "complete original/control 4", Body: "%%[[A]]%%\\[[A]][^n]: [[A]]\n\n    [[B]]\n[^unused]: [[A]]\n![[A]]## <em>A</em>\n-->- item\n\n      %%[[A]]%% ^A\n章節\n<!--\n[[A]]\n-->  ```\n``[[A#A]]![[A#A]]> [!note] [[A]]\n ```\n ^a-2\n    %%[[A]]%%[[A]] [[A]][[image.png]]\\`![[A]]\\`%%[[B|alias]]<!--# A\n", Source: source3, Owners: owners3, Names: names3, Original: original3, Witness: witness3, Payload: payload3, Owned: map[string]int{}},
		{Name: "complete original/control 5", Body: "[^n]: [[A]]\n\n    [[B]]\n章節\n[^unused]: [[A]]\n-->0  ```\nÉ\n%%[[A]]%%%%[[A]]%%``` [[A]]\n<!--\n[[A]]\n-->- [x] [[B]]\n- item\n\n      [^n]: [[A]]\n\n    [[B]]\n    [[A\\]]- > [!note] title\n[[A\nB]]```` go [[A]]\n## A\n## A\n\\``` ^é\n`open\n[[A]]\nclose`A\n===\n ^A\nA\n- > [!note] title\n     ^a-2\n", Source: source4, Owners: owners4, Names: names4, Original: original4, Witness: witness4, Payload: payload4, Owned: map[string]int{}},
		{Name: "complete original/control 6", Body: "A\n===\nB\n`[[A]]` <!-- [[A]] --># A\n[[A]] [[A]] ```\n>   - ## A\n    %%- [x] [[B]]\n\tA\n---\n> [!note] one\n> [!note] two\n> [!note] three\n- [ ] [[A]]\n[[image.png]]~~~\n^absent-prose ", Source: source5, Owners: owners5, Names: names5, Original: original5, Witness: witness5, Payload: payload5, Owned: map[string]int{}},
		{Name: "complete original/control 7", Body: "\t```\n[[#A]][[A]] [[A]]> > \\[[A]]1.     ```\n%%[[A]]%%%%    ```\n`[[A]]`## [[A|alias]]\n[[B|alias]]~~~\nÉ\n## !\nA\n===\n ^é\n ^a\n[[B|alias]]https://example.invalid/`[[A]]` ## A\n## A\n## A\n## A\n", Source: source6, Owners: owners6, Names: names6, Original: original6, Witness: witness6, Payload: payload6, Owned: map[string]int{}},
		{Name: "complete original/control 8", Body: "\\` ^A\n\t```\n%%![[image.png]]  ```\n`[[A]]`## <em>A</em>\n~~~~\n0 ^a-2\n ```\n   ```\n1. ``` [[A]]\n`    ", Source: source7, Owners: owners7, Names: names7, Original: original7, Witness: witness7, Payload: payload7, Owned: map[string]int{}},
		{Name: "complete original/control 9", Body: "text \t## A\n## A\n\n\n## A\n ^a-2\nA\n===\n`open\n[[A]]\nclose`\\[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n0```` go [[A]]\n[[A]]\t```\n%%````\n![[A#A]] ^a\nÉ\n<div>\n[[A]]\n</div>\n<!--## A-2\n## A\n## A\n   ```\n1. ", Source: source8, Owners: owners8, Names: names8, Original: original8, Witness: witness8, Payload: payload8, Owned: map[string]int{}},
		{Name: "complete original/control 10", Body: "[[A]] [[A]]\t```\n1.  ^é\n[[A\\|alias]] ```\n    ```\n\n## A\n   ```\nA\n===\n\\[[A]]- > [!note] title\n    ```\n## [[A|alias]]\n## <em>A</em>\n- item\n\n      [[A#^a]] ```\n- [ ] [[A]]\n ```\n>   ```\n## !\n<!--\n[[A]]\n-->- item\n\n       ^a\n``![[A#A]]", Source: source9, Owners: owners9, Names: names9, Original: original9, Witness: witness9, Payload: payload9, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=page-not-excerpt multiplicity=1 cut=\"\" page=true judge=false excerpt=false / debt / #1011 stage 5 / page": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=page-not-judge multiplicity=1 cut=\"\" page=true judge=false excerpt=false / debt / #1011 stage 5 / page": 1}},
		{Name: "complete original/control 11", Body: "0## A\n\t~~~~\n[[B|alias]]## [[A|alias]]\n[^n]: [[A]]\n\n    [[B]]\n[[A]]A\n\\\\[[A]] ^a\n## A-2\n## A\n## A\n ^a\nA\n---\n> [!unknown] title\n## A\n## A\n", Source: source10, Owners: owners10, Names: names10, Original: original10, Witness: witness10, Payload: payload10, Owned: map[string]int{}},
		{Name: "complete original/control 12", Body: ">   ```\n```\n\\\\[[A]][^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]\\[[A]]~~~~\n[[image.png]] ^A\n## A-2\n## A\n## A\n[[A\\]]## <em>A</em>\nB\n> [!unknown] title\n\n\n    ```\n<!--\n[[A]]\n--># A\n\n> ~~~\n\t     ^A\n ^a-2\n\\`## A-2\n## A\n## A\n\t- [x] [[B]]\n", Source: source11, Owners: owners11, Names: names11, Original: original11, Witness: witness11, Payload: payload11, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"    [[B]]\\n\\\\\\\\[[A]]\\\\[[A]]~~~~\\n[[image.png]] ^A\\n## A-2\\n## A\\n## A\\n[[A\\\\]]## <em>A</em>\\nB\\n> [!unknown] title\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"    [[B]]\\n\\\\\\\\[[A]]\\\\[[A]]~~~~\\n[[image.png]] ^A\\n## A-2\\n## A\\n## A\\n[[A\\\\]]## <em>A</em>\\nB\\n> [!unknown] title\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1}},
		{Name: "complete original/control 13", Body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n\n\n## A\n## A\n[^unused]: [[A]]\n ^A\n ^é\n````\n- [x] [[B]]\n![[image.png]]\\\\[[A]]   ```\n<!--text  ```\n[[B|alias]]0A\n---\n ```\n``` [[A]]\nhttps://example.invalid/`[[A]]` ```\n ^A\n  ```\n`  ```\n ````\n  > [!note] title\n    ```\n`[[A]]`", Source: source12, Owners: owners12, Names: names12, Original: original12, Witness: witness12, Payload: payload12, Owned: map[string]int{}},
		{Name: "complete original/control 14", Body: "https://example.invalid/`[[A]]` \n  - [[A\\|alias]] ^é\n<!--\n[[A]]\n-->\t```\n    ```\n- [ ] [[A]]\n`open\n[[A]]\nclose`<!--\n[[A]]\n-->[^unused]: [[A]]\n ^a\n\t```\n", Source: source13, Owners: owners13, Names: names13, Original: original13, Witness: witness13, Payload: payload13, Owned: map[string]int{}},
		{Name: "complete original/control 15", Body: "--> ```\n> ``` [[A]]\n ^a\n ^a-2\n%%", Source: source14, Owners: owners14, Names: names14, Original: original14, Witness: witness14, Payload: payload14, Owned: map[string]int{}},
		{Name: "complete original/control 16", Body: "É\n## A\n> ``` [[A]]\n\n\n> >     > [!note] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]``` [[A]]\n[[A\\|alias]]    ```\n<!-- [[A]] -->text [[A\nB]]ref[^n]\n## A-2\n## A\n## A\n## <em>A</em>\n<!--\n[[A]]\n-->`` ^a-2\n[^n]: [[A]]\n\n    [[B]]\nB\n- [x] [[B]]\n", Source: source15, Owners: owners15, Names: names15, Original: original15, Witness: witness15, Payload: payload15, Owned: map[string]int{}},
		{Name: "complete original/control 17", Body: "\t```\n[^n]: [[A]]\n\n    [[B]]\n0 ^A\n> > ## A\n## A\n ^A\nÉ\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%[[A]]%%\\[[A]]\\`0\t```\n```` go [[A]]\n![[A]]- > [!note] title\n", Source: source16, Owners: owners16, Names: names16, Original: original16, Witness: witness16, Payload: payload16, Owned: map[string]int{}},
		{Name: "complete original/control 18", Body: "```\nÉ\n> ````\n\\[[A]]A\n===\n^absent-prose \\\\[[A]]\n0[[A#^a]]## [[A|alias]]\n ^A\n```\n- item\n\n      > [!note] [[A]]\n%%[[A]]%%\\\\[[A]]<div>\n[[A]]\n</div>\n0text  ", Source: source17, Owners: owners17, Names: names17, Original: original17, Witness: witness17, Payload: payload17, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nÉ\\n> ````\\n\\\\[[A]]A\\n===\\n^absent-prose \\\\\\\\[[A]]\\n\\ue0020\\ue003[[A#^a]]## [[A|alias]]\\n ^A\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nÉ\\n> ````\\n\\\\[[A]]A\\n===\\n^absent-prose \\\\\\\\[[A]]\\n\\ue0020\\ue003[[A#^a]]## [[A|alias]]\\n ^A\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1}},
		{Name: "complete original/control 19", Body: "- > [!note] title\n^absent-prose 0> [!note] [[A]]\n<!-- [[A]] -->## [[A|alias]]\n> - <!--![[A#A]]<!-- [[A]] -->   ```\n[^unused]: [[A]]\n%%[[A]]%% ^a-2\n[[#A]]## A-2\n## A\n## A\n`````` go [[A]]\n- [ ] [[A]]\n## !\n[[A#^a]]\\\\[[A]]> > --> ^A\n![[A]]", Source: source18, Owners: owners18, Names: names18, Original: original18, Witness: witness18, Payload: payload18, Owned: map[string]int{}},
		{Name: "complete original/control 20", Body: "## A\n[[A]]章節\n> [!note] one\n> [!note] two\n> [!note] three\nhttps://example.invalid/`[[A]]` [[A\\]]- \t[[A#A]]\\`    ## [[A|alias]]\n> ```\n ^é\n ^a-2\n## <em>A</em>\n\\````\ntext [[A\\|alias]]A\n---\n", Source: source19, Owners: owners19, Names: names19, Original: original19, Witness: witness19, Payload: payload19, Owned: map[string]int{}},
		{Name: "complete original/control 21", Body: ">  ^é\n````\n<!-- [[A]] -->![[A#A]]````\n ^a-2\n- [ ] [[A]]\n[[A]]\t%%[[A]]%%\n\n- [ ] [[A]]\n章節\n````\n> > [^unused]: [[A]]\n[[A]] [[A]]ref[^n]\n0 ```\n[[A]]  > [!note] title\n ^a-2\n![[A]]^absent-prose   > [!note] title\n> [!note] [[A]]\n\n## !\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n", Source: source20, Owners: owners20, Names: names20, Original: original20, Witness: witness20, Payload: payload20, Owned: map[string]int{}},
		{Name: "complete original/control 22", Body: "\n\n[[B|alias]]~~~\n<div>\n[[A]]\n</div>\n   ```\n   ```\n> [!note] one\n> [!note] two\n> [!note] three\n[^unused]: [[A]]\n`open\n[[A]]\nclose` ^a\nA\n---\nÉ\n## A-2\n## A\n## A\n\t```\n``  > [!note] title\n````\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n\\[[A]]<!-- [[A]] -->[[image.png]]", Source: source21, Owners: owners21, Names: names21, Original: original21, Witness: witness21, Payload: payload21, Owned: map[string]int{}},
		{Name: "complete original/control 23", Body: "\t%%\t```\n    ```\n## A-2\n## A\n## A\n    ```\n`open\n[[A]]\nclose`章節\n ^a\n> [!note] one\n> [!note] two\n> [!note] three\n\t[[A\\]][^n]: [[A]]\n\n    [[B]]\nÉ\n> [!note] one\n> [!note] two\n> [!note] three\n[[A]] [[A]]  > [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n", Source: source22, Owners: owners22, Names: names22, Original: original22, Witness: witness22, Payload: payload22, Owned: map[string]int{}},
		{Name: "complete original/control 24", Body: "> ``` [[A]]\n<div>\n[[A]]\n</div>\n\n\t~~~~\n- item\n\n      ~~~\n`| a | b |\n|---|---|\n| [[A]] | ^a |\n ^a\n ^é\n- [x] [[B]]\n~~~\n\\\\[[A]]1. %%A\n---\n`open\n[[A]]\nclose`# A\n[[A\\|alias]]1.      ^é\n ^é\nÉ\n## A\nÉ\nÉ\n", Source: source23, Owners: owners23, Names: names23, Original: original23, Witness: witness23, Payload: payload23, Owned: map[string]int{}},
		{Name: "complete original/control 25", Body: "A\n---\n- [x] [[B]]\n\t[[A]][[A\nB]]`# A\nA\n---\n`[[A]]`  -  [[A\\|alias]]  ```\n    ^absent-prose --> ^é\n\tA\n===\n%%[[A]]%%[^n]: [[A]]\n\n    [[B]]\n[[#A]] ^a-2\nB\n", Source: source24, Owners: owners24, Names: names24, Original: original24, Witness: witness24, Payload: payload24, Owned: map[string]int{}},
		{Name: "complete original/control 26", Body: " ^é\n\\\\[[A]]A\n===\n[^n]: [[A]]\n\n    [[B]]\n ^a-2\n<!-- [[A]] -->- > [!note] title\ntext  ```\n> [!note] [[A]]\n ```\n- > [!note] title\n```` go [[A]]\n[[A\\|alias]]![[A#A]]![[A]]``  ^a-2\n ^é\n[[A#A]][[B|alias]]章節\n[[A#A]] | a | b |\n|---|---|\n| [[A]] | ^a |\n## A\nhttps://example.invalid/`[[A]]` ", Source: source25, Owners: owners25, Names: names25, Original: original25, Witness: witness25, Payload: payload25, Owned: map[string]int{}},
		{Name: "complete original/control 27", Body: "> [!note] [[A]]\n> <!-- [[A]] -->```` go [[A]]\nref[^n]\n%%[[A]]%%<div>\n[[A]]\n</div>\n1.  ^a-2\n![[image.png]]-  ^é\n[[A\\]][[A\nB]]B\n[[A]] [[A]][[A]]> > \\\\[[A]]![[A]]\n\\\\[[A]]É\n> ~~~\n## <em>A</em>\n<div>\n[[A]]\n</div>\n`` ^é\n", Source: source26, Owners: owners26, Names: names26, Original: original26, Witness: witness26, Payload: payload26, Owned: map[string]int{}},
		{Name: "complete original/control 28", Body: "[^unused]: [[A]]\n%%[[A]]%%0`\\`[^n]: [[A]]\n\n    [[B]]\n ^A\n```\n## A-2\n## A\n## A\n0 ^é\n- item\n\n       B\n章節\n<div>\n[[A]]\n</div>\n![[A#A]]  - <!-- [[A]] -->  ^é\nhttps://example.invalid/`[[A]]`  ^a\n0    ```\n``[[A\\]][[A#^a]] ^A\n[^unused]: [[A]]\n", Source: source27, Owners: owners27, Names: names27, Original: original27, Witness: witness27, Payload: payload27, Owned: map[string]int{}},
		{Name: "complete original/control 29", Body: "- > [!note] title\n## !\n  -  ^é\n\\`![[image.png]]<!-- [[A]] -->`[[A#^a]][[A\\|alias]][[A#^a]]<!--\n[[A]]\n-->-  ```\n%%![[A#A]] ^a\nref[^n]\n`\t[[A]]ref[^n]\n<!--\n[[A]]\n-->``` [[A]]\n\t# A\n\n^absent-prose É\n1. É\n", Source: source28, Owners: owners28, Names: names28, Original: original28, Witness: witness28, Payload: payload28, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=true excerpt=false / debt / #1011 stage 5 / judge": 1}},
		{Name: "complete original/control 30", Body: "[[A\\|alias]]## [[A|alias]]\nA\n===\n[[A\\]]A\n===\n[[#A]]https://example.invalid/`[[A]]` \\`0[[A\nB]][[image.png]]```open\n[[A]]\nclose`^absent-prose > [!note] title\n[[image.png]]`[[A]]`\t%%\\` ^a\n\n\nref[^n]\n```` go [[A]]\n%%[[A]]%%`", Source: source29, Owners: owners29, Names: names29, Original: original29, Witness: witness29, Payload: payload29, Owned: map[string]int{}},
		{Name: "complete original/control 31", Body: "\\`É\n章節\n%%text `open\n[[A]]\nclose`## A\n```\n%%[[A]]%%\\\\[[A]]# A\n[[B|alias]]  - \\` ^é\n[[A\nB]]1.  ^A\n- [ ] [[A]]\n^absent-prose > [[A]] [[A]]B\n![[image.png]]-->0```\n~~~~\n<div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n", Source: source30, Owners: owners30, Names: names30, Original: original30, Witness: witness30, Payload: payload30, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"\" page=false judge=true excerpt=false / debt / #1011 stage 5 / judge": 1}},
		{Name: "complete original/control 32", Body: "> [!note] title\n``  - ![[image.png]]   ```\n[[image.png]]    %%```\ntext [[A#A]]> [!note] title\nA\n---\n  -  ^a\n\n\n> [!note] [[A]]\n-->章節\n> [!unknown] title\n", Source: source31, Owners: owners31, Names: names31, Original: original31, Witness: witness31, Payload: payload31, Owned: map[string]int{}},
		{Name: "complete original/control 33", Body: "\thttps://example.invalid/`[[A]]` %%É\n[[B|alias]]B\n<!-- [[A]] --> ^a-2\n````\n ``` [[A]]\n    ```\n\n\\[[A]]A\n> [!note] [[A]]\n    ```\n[[A\nB]] ```\n> [!unknown] title\n\t## [[A|alias]]\n[[A]] [[A]]## [[A|alias]]\n ```\n[[A#A]]\\\\[[A]] ^é\n> ![[A]]", Source: source32, Owners: owners32, Names: names32, Original: original32, Witness: witness32, Payload: payload32, Owned: map[string]int{}},
		{Name: "complete original/control 34", Body: "\t```\n`## !\n ^A\n    %%0\n ^A\n[[A\\]]<!--\n[[A]]\n-->`open\n[[A]]\nclose`- > [!note] title\n\n ^é\n[[A]]> [!note] [[A]]\n ^A\n  - \nref[^n]\n> [!note] title\n ^a-2\n-  ^é\n\t`open\n[[A]]\nclose`A\n---\n", Source: source33, Owners: owners33, Names: names33, Original: original33, Witness: witness33, Payload: payload33, Owned: map[string]int{}},
		{Name: "complete original/control 35", Body: "1. A\n> [!unknown] title\n  > [!note] title\nB\n[^n]: [[A]]\n\n    [[B]]\n ^A\n- [x] [[B]]\n ^a\n ^a-2\n\\[[A]]~~~~\n B\n[^n]: [[A]]\n\n    [[B]]\n<!--> 0\t```\n[[image.png]]  ```\n `[[A]]`> [!unknown] title\n``` [[A]]\n## [[A|alias]]\n", Source: source34, Owners: owners34, Names: names34, Original: original34, Witness: witness34, Payload: payload34, Owned: map[string]int{}},
		{Name: "complete original/control 36", Body: "- item\n\n      %%![[A#A]]`open\n[[A]]\nclose` ^A\n<div>\n[[A]]\n</div>\n```\nref[^n]\n", Source: source35, Owners: owners35, Names: names35, Original: original35, Witness: witness35, Payload: payload35, Owned: map[string]int{}},
		{Name: "complete original/control 37", Body: "[^n]: <!--hidden--> words ^a\n", Source: source36, Owners: owners36, Names: names36, Original: original36, Witness: witness36, Payload: payload36, Owned: map[string]int{}},
		{Name: "complete original/control 38", Body: "[^n]: words ^a\n", Source: source37, Owners: owners37, Names: names37, Original: original37, Witness: witness37, Payload: payload37, Owned: map[string]int{}},
		{Name: "complete original/control 39", Body: "ref[^n]\n\n[^n]: words ^a\n", Source: source38, Owners: owners38, Names: names38, Original: original38, Witness: witness38, Payload: payload38, Owned: map[string]int{}},
		{Name: "complete original/control 40", Body: "ordinary ^a\n\n[^n]: words ^a\n", Source: source39, Owners: owners39, Names: names39, Original: original39, Witness: witness39, Payload: payload39, Owned: map[string]int{}},
		{Name: "complete original/control 41", Body: "[^n]: `words ^a`\n", Source: source40, Owners: owners40, Names: names40, Original: original40, Witness: witness40, Payload: payload40, Owned: map[string]int{}},
		{Name: "complete original/control 42", Body: "[^n]: words\n\n    ```\n    words ^a\n    ```\n", Source: source41, Owners: owners41, Names: names41, Original: original41, Witness: witness41, Payload: payload41, Owned: map[string]int{}},
		{Name: "complete original/control 43", Body: "%%\n[^n]: words ^a\n%%\n", Source: source42, Owners: owners42, Names: names42, Original: original42, Witness: witness42, Payload: payload42, Owned: map[string]int{}},
		{Name: "complete original/control 44", Body: "ghost\r\n\r\n[^n]: <!--hidden--> words ^a\r\n", Source: source43, Owners: owners43, Names: names43, Original: original43, Witness: witness43, Payload: payload43, Owned: map[string]int{}},
		{Name: "complete original/control 45", Body: "ghost [[A]] words ^live\n`open\nwords ^literal\nclose`\n%%hidden ^comment\n%%\n\n[^n]: [[B]] words ^a\n    second ^b\n\n    ## heading ^d\n\n    > words ^e\n", Source: source44, Owners: owners44, Names: names44, Original: original44, Witness: witness44, Payload: payload44, Owned: map[string]int{}},
		{Name: "complete original/control 46", Body: "[[A]] ^q\n\n[^n]: [[B]] ^a\n", Source: source45, Owners: owners45, Names: names45, Original: original45, Witness: witness45, Payload: payload45, Owned: map[string]int{}},
		{Name: "complete original/control 47", Body: "[[A]] ^q\n[[A]] ^z\n[[A]] ^v\n[[A]] ^x\n[[A]] ^w\n\n[^n]: [[B]] ^a\n", Source: source46, Owners: owners46, Names: names46, Original: original46, Witness: witness46, Payload: payload46, Owned: map[string]int{}},
		{Name: "complete original/control 48", Body: "[[A]]\r\n\r\n[^n]: [[B]] ^a\r\n", Source: source47, Owners: owners47, Names: names47, Original: original47, Witness: witness47, Payload: payload47, Owned: map[string]int{}},
		{Name: "complete original/control 49", Body: "[^n]: words ^a\n\noutside words ^a\n", Source: source48, Owners: owners48, Names: names48, Original: original48, Witness: witness48, Payload: payload48, Owned: map[string]int{}},
		{Name: "complete original/control 50", Body: "[[A]] words ^live\n`open\nliteral ^literal\nclose`\n%%hide ^hidden\n%%\n\n[^n]: [[B]] ^a\n    second ^b\n\noutside words ^a\n\nother words ^b\n", Source: source49, Owners: owners49, Names: names49, Original: original49, Witness: witness49, Payload: payload49, Owned: map[string]int{}},
		{Name: "complete original/control 51", Body: "outside words ^a\n\n[^n]: words ^a\n", Source: source50, Owners: owners50, Names: names50, Original: original50, Witness: witness50, Payload: payload50, Owned: map[string]int{}},
		{Name: "complete original/control 52", Body: "```\nwords ^a\n```\n", Source: source51, Owners: owners51, Names: names51, Original: original51, Witness: witness51, Payload: payload51, Owned: map[string]int{}},
		{Name: "complete original/control 53", Body: "```\nword\n> ```\nwords ^a\n```\n", Source: source52, Owners: owners52, Names: names52, Original: original52, Witness: witness52, Payload: payload52, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1}},
		{Name: "complete original/control 54", Body: "```\nword\n> ```\nwords ^a\n```\n\noutside words ^a\n", Source: source53, Owners: owners53, Names: names53, Original: original53, Witness: witness53, Payload: payload53, Owned: map[string]int{}},
		{Name: "complete original/control 55", Body: "    words ^a\n", Source: source54, Owners: owners54, Names: names54, Original: original54, Witness: witness54, Payload: payload54, Owned: map[string]int{}},
		{Name: "complete original/control 56", Body: "`open\nwords ^a\nclose`\n", Source: source55, Owners: owners55, Names: names55, Original: original55, Witness: witness55, Payload: payload55, Owned: map[string]int{}},
		{Name: "complete original/control 57", Body: "> ``` info\nwords ^a\n", Source: source56, Owners: owners56, Names: names56, Original: original56, Witness: witness56, Payload: payload56, Owned: map[string]int{}},
		{Name: "complete original/control 58", Body: "```\nword\n> ```\nwords ^A\nwords ^a\n```\n", Source: source57, Owners: owners57, Names: names57, Original: original57, Witness: witness57, Payload: payload57, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^A\\nwords ^a\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^A\\nwords ^a\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1}},
		{Name: "complete original/control 59", Body: "```\r\nword\r\n> ```\r\nwords ^A\r\n```\r\n", Source: source58, Owners: owners58, Names: names58, Original: original58, Witness: witness58, Payload: payload58, Owned: map[string]int{}},
		{Name: "complete original/control 60", Body: "[[A]] words ^live\n`open\nliteral ^literal\nclose`\n%%hide ^hidden\n%%\n\n```\nword\n> ```\nwords ^a\nwords ^b\n```\n", Source: source59, Owners: owners59, Names: names59, Original: original59, Witness: witness59, Payload: payload59, Owned: map[string]int{}},
		{Name: "complete original/control 61", Body: "[[A]] words ^q\n\n```\nword\n> ```\nwords ^a\n```\n", Source: source60, Owners: owners60, Names: names60, Original: original60, Witness: witness60, Payload: payload60, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1}},
		{Name: "complete original/control 62", Body: "words ^q\nwords ^z\nwords ^v\nwords ^x\nwords ^w\n\n```\nword\n> ```\nwords ^a\n```\n", Source: source61, Owners: owners61, Names: names61, Original: original61, Witness: witness61, Payload: payload61, Owned: map[string]int{}},
		{Name: "complete original/control 63", Body: "[^n]: words\n\n    ```\n    word\n    > ```\n    words ^a\n    ```\n", Source: source62, Owners: owners62, Names: names62, Original: original62, Witness: witness62, Payload: payload62, Owned: map[string]int{}},
		{Name: "complete original/control 64", Body: "%%hidden\nwords ^a\n%%\n", Source: source63, Owners: owners63, Names: names63, Original: original63, Witness: witness63, Payload: payload63, Owned: map[string]int{}},
		{Name: "complete original/control 65", Body: "```\nwords ^é\n```\n", Source: source64, Owners: owners64, Names: names64, Original: original64, Witness: witness64, Payload: payload64, Owned: map[string]int{}},
		{Name: "complete original/control 66", Body: "```\nword\n> ```\nwords ^a\nwords ^a-2\n```\n", Source: source65, Owners: owners65, Names: names65, Original: original65, Witness: witness65, Payload: payload65, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a-2\" direction=excerpt-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a-2\" direction=judge-only multiplicity=1 cut=\"```\\nword\\n> ```\\nwords ^a\\nwords ^a-2\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1}},
		{Name: "complete original/control 67", Body: "[[A]] words ^live\n\n## separate\n`open\nliteral ^literal\nclose`\n\n%%hide ^hidden\n%%\n\n[^n]: [[B]] words ^owned\n    second\n    > continuation\n\n## code\n```\nword\n> ```\nwords ^a\nwords ^b\n```\n\n## after\noutside words ^outside\n", Source: source66, Owners: owners66, Names: names66, Original: original66, Witness: witness66, Payload: payload66, Owned: map[string]int{"P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=excerpt-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^a\" direction=judge-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=excerpt-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1, "P3/block-three-way tuple={SourceRole: Target: Section: State:} fragment=\"^b\" direction=judge-only multiplicity=1 cut=\"## code\\n```\\nword\\n> ```\\nwords ^a\\nwords ^b\\n```\" page=false judge=true excerpt=true / debt / #1011 stage 5 / judge+excerpt": 1}},
	}
}
func TestAgreementNativeCodeAddressSource(t *testing.T) {
	t.Parallel()
	for _, tc := range agreementNativeCodeAddressControls() {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := agreementNativeCodeAddressReading(tc.Body)
			if diff := cmp.Diff(tc.Source, s); diff != "" {
				t.Fatalf("caught: complete code address source (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.Owners, agreementNativeCodeAddressOwners(&s.Original)); diff != "" {
				t.Fatalf("caught: complete code address owner roster (-want +got):\n%s", diff)
			}
		})
	}
}
func TestAgreementNativeCodeAddressPublic(t *testing.T) {
	t.Parallel()
	for _, tc := range agreementNativeCodeAddressControls() {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.Body}
			_, a := agreementIsolatedPage(t, c)
			fs := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{a})[0]
			p := agreementNativeCodeAddressBudget(t, c.Body, &a, fs)
			if diff := cmp.Diff(tc.Payload, p); diff != "" {
				t.Fatalf("caught: complete code address public budget (-want +got):\n%s", diff)
			}
			if tc.Source.Valid {
				_, w := agreementIsolatedPage(t, agreementCase{Body: tc.Source.Body})
				names := agreementNativeAddressNames(&tc.Source, &a, &w)
				if diff := cmp.Diff(tc.Names, names); diff != "" {
					t.Fatalf("caught: complete code address public namespace (-want +got):\n%s", diff)
				}
				before := agreementNativeAddressPublic(t, c.Body, &a, names)
				after := agreementNativeAddressPublic(t, tc.Source.Body, &w, names)
				if diff := cmp.Diff(tc.Original, before); diff != "" {
					t.Fatalf("caught: complete code address original ledger (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.Witness, after); diff != "" {
					t.Fatalf("caught: complete code address projected ledger (-want +got):\n%s", diff)
				}
			}
			owned := make(map[string]int)
			for _, f := range fs {
				k, authority, wrong := agreementNativeCodeAddressDifference(c, &f, &a, fs, &p)
				if k == "" {
					continue
				}
				owned[agreementSignature(&f)+" / "+k+" / "+authority+" / "+wrong]++
				agreementNativeCodeAddressDrift(t, c, &f, &a, fs, &p)
			}
			if diff := cmp.Diff(tc.Owned, owned); diff != "" {
				t.Fatalf("caught: complete code address public ownership (-want +got):\n%s", diff)
			}
		})
	}
}
func agreementNativeCodeAddressDrift(t *testing.T, c agreementCase, f *agreementFailure, a *agreementHTML, fs []agreementFailure, p *agreementNativeCodeAddressPayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if k, _, _ := agreementNativeCodeAddressDifference(other, f, a, fs, p); k != "" {
			t.Fatal("caught: code address borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){
		func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Fragment = "^unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Multiplicity = 0 }, func(f *agreementFailure) { f.Multiplicity = -1 }, func(f *agreementFailure) { f.PagePresent = !f.PagePresent }, func(f *agreementFailure) { f.JudgeAccepted = !f.JudgeAccepted }, func(f *agreementFailure) { f.ExcerptFound = !f.ExcerptFound }, func(f *agreementFailure) { f.Cut += "unowned" },
	} {
		other := *f
		change(&other)
		if k, _, _ := agreementNativeCodeAddressDifference(c, &other, a, fs, p); k != "" {
			t.Fatalf("caught: code address borrowed signature %s", agreementSignature(&other))
		}
	}
	other := *a
	other.Blocks = append(slices.Clone(a.Blocks), "^unowned")
	if k, _, _ := agreementNativeCodeAddressDifference(c, f, &other, fs, p); k != "" {
		t.Fatal("caught: code address borrowed changed whole page set")
	}
	partial := slices.Clone(fs)
	for i := range partial {
		if partial[i].Property == "P3" && agreementSignature(&partial[i]) == agreementSignature(f) {
			partial = append(partial[:i], partial[i+1:]...)
			break
		}
	}
	for _, changed := range [][]agreementFailure{nil, partial, append(slices.Clone(fs), *f)} {
		if k, _, _ := agreementNativeCodeAddressDifference(c, f, a, changed, p); k != "" {
			t.Fatal("caught: code address borrowed changed whole P3 pool")
		}
		if agreementNativeCodeAddressBudget(t, c.Body, a, changed).Address.Body != "" {
			t.Fatal("caught: code address budget borrowed changed whole P3 pool")
		}
	}
	for _, change := range []func(*agreementNativeCodeAddressPayload){
		func(p *agreementNativeCodeAddressPayload) { p.Owners = nil },
		func(p *agreementNativeCodeAddressPayload) {
			p.Owners = maps.Clone(p.Owners)
			p.Owners["^unowned"] = []agreementNativeCodeAddressOwner{{Field: 0, Code: 0}}
		},
		func(p *agreementNativeCodeAddressPayload) {
			p.Owners = maps.Clone(p.Owners)
			for name := range p.Owners {
				delete(p.Owners, name)
				break
			}
		},
		func(p *agreementNativeCodeAddressPayload) {
			p.Owners = maps.Clone(p.Owners)
			for name, owners := range p.Owners {
				p.Owners[name] = append(slices.Clone(owners), agreementNativeCodeAddressOwner{Field: 0, Code: 0})
				break
			}
		},
		func(p *agreementNativeCodeAddressPayload) {
			p.Owners = maps.Clone(p.Owners)
			for name, owners := range p.Owners {
				changed := slices.Clone(owners)
				changed[0].Field++
				p.Owners[name] = changed
				break
			}
		},
		func(p *agreementNativeCodeAddressPayload) {
			p.Owners = maps.Clone(p.Owners)
			for name, owners := range p.Owners {
				changed := slices.Clone(owners)
				changed[0].Code++
				p.Owners[name] = changed
				break
			}
		},
	} {
		other := *p
		change(&other)
		if k, _, _ := agreementNativeCodeAddressDifference(c, f, a, fs, &other); k != "" {
			t.Fatal("caught: code address borrowed changed complete owner roster")
		}
	}
	for _, change := range []func(*agreementNativeCodeAddressPayload){
		func(p *agreementNativeCodeAddressPayload) { p.Address.Body += "unowned" }, func(p *agreementNativeCodeAddressPayload) { p.Address.Names = nil }, func(p *agreementNativeCodeAddressPayload) {
			p.Address.Blocks = append(slices.Clone(p.Address.Blocks), "^unowned")
		}, func(p *agreementNativeCodeAddressPayload) {
			p.Address.WitnessBlocks = append(slices.Clone(p.Address.WitnessBlocks), "^unowned")
		}, func(p *agreementNativeCodeAddressPayload) { p.Address.Original = nil }, func(p *agreementNativeCodeAddressPayload) { p.Address.Witness = nil }, func(p *agreementNativeCodeAddressPayload) { p.Address.Signatures = nil },
		func(p *agreementNativeCodeAddressPayload) {
			p.Address.Original = maps.Clone(p.Address.Original)
			p.Address.Original["^unowned"] = agreementNativeAddressReceipt{}
		}, func(p *agreementNativeCodeAddressPayload) {
			p.Address.Witness = maps.Clone(p.Address.Witness)
			p.Address.Witness["^unowned"] = agreementNativeAddressReceipt{}
		},
		func(p *agreementNativeCodeAddressPayload) {
			p.Address.Original = maps.Clone(p.Address.Original)
			for name, r := range p.Address.Original {
				r.Cut += "unowned"
				p.Address.Original[name] = r
			}
		}, func(p *agreementNativeCodeAddressPayload) {
			p.Address.Witness = maps.Clone(p.Address.Witness)
			for name, r := range p.Address.Witness {
				r.Cut += "unowned"
				p.Address.Witness[name] = r
			}
		},
	} {
		other := *p
		change(&other)
		if k, _, _ := agreementNativeCodeAddressDifference(c, f, a, fs, &other); k != "" {
			t.Fatal("caught: code address borrowed changed complete public evidence")
		}
	}
	for ledgerIndex, ledger := range []map[string]agreementNativeAddressReceipt{p.Address.Original, p.Address.Witness} {
		for name := range ledger {
			for _, change := range []func(*agreementNativeAddressReceipt){func(r *agreementNativeAddressReceipt) { r.Page = !r.Page }, func(r *agreementNativeAddressReceipt) { r.Judge = !r.Judge }, func(r *agreementNativeAddressReceipt) { r.Found = !r.Found }} {
				other := *p
				if ledgerIndex == 0 {
					other.Address.Original = maps.Clone(p.Address.Original)
					r := other.Address.Original[name]
					change(&r)
					other.Address.Original[name] = r
				} else {
					other.Address.Witness = maps.Clone(p.Address.Witness)
					r := other.Address.Witness[name]
					change(&r)
					other.Address.Witness[name] = r
				}
				if k, _, _ := agreementNativeCodeAddressDifference(c, f, a, fs, &other); k != "" {
					t.Fatal("caught: code address borrowed changed complete public roles")
				}
			}
		}
	}
}

func TestAgreementNativeCodeAddressSourceDrift(t *testing.T) {
	t.Parallel()
	body := "[[A]] words ^live\n\n## separate\n`open\nliteral ^literal\nclose`\n\n%%hide ^hidden\n%%\n\n[^n]: [[B]] words ^owned\n    second\n    > continuation\n\n## code\n```\nword\n> ```\nwords ^a\nwords ^b\n```\n\n## after\noutside words ^outside\n"
	c := agreementCase{Body: body}
	_, a := agreementIsolatedPage(t, c)
	fs := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{a})[0]
	baseline := agreementNativeCodeAddressBudget(t, body, &a, fs)
	s := &baseline.Address.Source
	if baseline.Address.Body == "" || len(s.Original.Native.Definitions) != 1 || len(s.Original.Native.Code) != 2 || len(s.Original.Native.Comments) != 1 || len(s.Original.Check.Fields) != 2 || len(s.Original.Fields) != 7 || len(s.Projected.Fields) != 7 || len(s.Shapes) != 7 || len(s.AfterShapes) != 7 || len(s.Changes) != 3 || len(s.Original.Native.Definitions[0].Nodes) < 3 || len(s.Original.Native.Definitions[0].Nodes[1].Lines) < 2 || len(s.Original.Native.Definitions[0].Nodes[2].Text) != 1 || len(s.Original.Fields[3].Nodes) != 1 {
		t.Fatal("code address source drift lacks complete declared shape")
	}
	var failure *agreementFailure
	for _, f := range fs {
		if k, _, _ := agreementNativeCodeAddressDifference(c, &f, &a, fs, &baseline); k != "" {
			owned := f
			failure = &owned
			break
		}
	}
	if failure == nil {
		t.Fatal("code address source drift control is unowned")
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
	changes = append(changes,
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"original code order", func(s *agreementNativeAddressSource) {
			s.Original.Native.Code[0], s.Original.Native.Code[1] = s.Original.Native.Code[1], s.Original.Native.Code[0]
		}},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"original code token role", func(s *agreementNativeAddressSource) { s.Original.Fields[4].Code = false }},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"original code token comment", func(s *agreementNativeAddressSource) { s.Original.Fields[4].Comment = true }},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"original partial token containment", func(s *agreementNativeAddressSource) {
			s.Original.Native.Code[1].Span.Start = s.Original.Fields[4].Token.Start + 1
		}},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"original check field targets", func(s *agreementNativeAddressSource) { s.Original.Check.Fields[0].Targets = nil }},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"original check owner", func(s *agreementNativeAddressSource) { s.Original.Check.Fields[1].DefinitionOwner = -1 }},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"projected code kind", func(s *agreementNativeAddressSource) { s.Projected.Native.Code[1].Kind = "unowned" }},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"projected code bound", func(s *agreementNativeAddressSource) { s.Projected.Native.Code[1].Span.Stop-- }},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"projected code token role", func(s *agreementNativeAddressSource) { s.Projected.Fields[4].Code = false }},
		struct {
			name   string
			change func(*agreementNativeAddressSource)
		}{"projected owner", func(s *agreementNativeAddressSource) { s.Projected.Fields[4].Owner = 0 }},
	)
	for i := range changes {
		change := &changes[i]
		t.Run(change.name, func(t *testing.T) {
			t.Parallel()
			p := baseline
			p.Address.Source = agreementNativeCodeAddressReading(body)
			change.change(&p.Address.Source)
			if k, _, _ := agreementNativeCodeAddressDifference(c, failure, &a, fs, &p); k != "" {
				t.Fatal("caught: code address borrowed changed complete source")
			}
		})
	}
	otherFamily := append(slices.Clone(fs), agreementFailure{Property: "P4", Identity: "unowned", Fragment: "unowned"})
	if k, _, _ := agreementNativeCodeAddressDifference(c, failure, &a, otherFamily, &baseline); k != "debt" {
		t.Fatal("caught: code address P3 pool borrowed another property")
	}
	for _, name := range []string{"^literal", "^a", "^b"} {
		p := baseline
		p.Owners = maps.Clone(p.Owners)
		delete(p.Owners, name)
		if k, _, _ := agreementNativeCodeAddressDifference(c, failure, &a, fs, &p); k != "" {
			t.Fatal("caught: code address borrowed omitted inactive or unselected code member")
		}
	}
}

// Unaddressed prose still belongs to the captured body when declaration facts
// and every public fragment receipt happen to stay the same.
func TestAgreementNativeCodeAddressBody(t *testing.T) {
	t.Parallel()
	rich := agreementNativeCodeAddressControls()[66].Body
	c := agreementCase{Body: "ghost\n\n" + rich}
	other := agreementCase{Body: "shade\n\n" + rich}
	_, a := agreementIsolatedPage(t, c)
	_, w := agreementIsolatedPage(t, other)
	fs := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{a})[0]
	changed := agreementFragmentFailures(t, []agreementCase{other}, []agreementHTML{w})[0]
	p := agreementNativeCodeAddressBudget(t, c.Body, &a, fs)
	q := agreementNativeCodeAddressBudget(t, other.Body, &w, changed)
	if p.Address.Body == "" || q.Address.Body == "" || p.Address.Body == q.Address.Body {
		t.Fatal("code address body control lacks distinct qualified bodies")
	}
	facts := func(p *agreementNativeCodeAddressPayload) []any {
		return []any{p.Address.Source.Original, p.Address.Source.Projected, p.Address.Source.Changes, p.Address.Source.Shapes, p.Address.Source.AfterShapes, p.Address.Names, p.Address.Blocks, p.Address.WitnessBlocks, p.Address.Original, p.Address.Witness, p.Address.Signatures, p.Owners}
	}
	if !cmp.Equal(facts(&p), facts(&q)) {
		t.Fatal("code address body control changed declaration facts or public receipts")
	}
	var failure *agreementFailure
	for i := range changed {
		f := &changed[i]
		if k, _, _ := agreementNativeCodeAddressDifference(other, f, &w, changed, &q); k != "" {
			failure = f
			break
		}
	}
	if failure == nil {
		t.Fatal("code address body control is unowned")
	}
	p.Address.Body = other.Body
	if k, _, _ := agreementNativeCodeAddressDifference(other, failure, &w, changed, &p); k != "" {
		t.Fatal("caught: code address borrowed different bytes with identical declared facts")
	}
}

func TestAgreementNativeCodeAddressOwnerDrift(t *testing.T) {
	t.Parallel()
	all := map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 0}}, "^a": {{Field: 4, Code: 1}}, "^b": {{Field: 5, Code: 1}}}
	literal := map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 0}}}
	withoutA := map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 0}}, "^b": {{Field: 5, Code: 1}}}
	withoutB := map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 0}}, "^a": {{Field: 4, Code: 1}}}
	tests := []struct {
		name   string
		change func(*agreementNativeAddressPart)
		want   map[string][]agreementNativeCodeAddressOwner
	}{
		{"whole roster", func(*agreementNativeAddressPart) {}, all},
		{"unrecognized code kind", func(s *agreementNativeAddressPart) { s.Native.Code[1].Kind = "unowned" }, literal},
		{"info string refusal", func(s *agreementNativeAddressPart) { s.Native.Code[1].Kind = "FencedCodeBlockInfo" }, literal},
		{"partial token start", func(s *agreementNativeAddressPart) { s.Native.Code[1].Span.Start = s.Fields[4].Token.Start + 1 }, withoutA},
		{"partial token stop", func(s *agreementNativeAddressPart) { s.Native.Code[1].Span.Stop = s.Fields[5].Token.Stop - 1 }, withoutB},
		{"missing code body", func(s *agreementNativeAddressPart) { s.Native.Code = s.Native.Code[:1] }, literal},
		{"ambiguous code bodies", func(s *agreementNativeAddressPart) { s.Native.Code = append(s.Native.Code, s.Native.Code[1]) }, literal},
		{"ordered code owner indices", func(s *agreementNativeAddressPart) {
			s.Native.Code[0], s.Native.Code[1] = s.Native.Code[1], s.Native.Code[0]
		}, map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 1}}, "^a": {{Field: 4, Code: 0}}, "^b": {{Field: 5, Code: 0}}}},
		{"nonliteral token", func(s *agreementNativeAddressPart) { s.Fields[4].Code = false }, withoutA},
		{"comment token", func(s *agreementNativeAddressPart) { s.Fields[4].Comment = true }, withoutA},
		{"discarded token", func(s *agreementNativeAddressPart) { s.Fields[4].Owner = 0 }, withoutA},
		{"native child token", func(s *agreementNativeAddressPart) { s.Fields[4].Nodes = []int{1} }, withoutA},
		{"same-name prose sharing", func(s *agreementNativeAddressPart) {
			f := s.Fields[6]
			f.Name, f.Folded = "^a", "^a"
			s.Fields = append(s.Fields, f)
		}, withoutA},
		{"same-name comment sharing", func(s *agreementNativeAddressPart) {
			f := s.Fields[2]
			f.Name, f.Folded = "^a", "^a"
			s.Fields = append(s.Fields, f)
		}, withoutA},
		{"complete literal multiplicity", func(s *agreementNativeAddressPart) { s.Fields = append(s.Fields, s.Fields[4]) }, map[string][]agreementNativeCodeAddressOwner{"^literal": {{Field: 1, Code: 0}}, "^a": {{Field: 4, Code: 1}, {Field: 7, Code: 1}}, "^b": {{Field: 5, Code: 1}}}},
		{"folded spelling", func(s *agreementNativeAddressPart) { s.Fields[4].Name = "^A" }, all},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := agreementNativeAddressPartReading(agreementNativeCodeAddressControls()[66].Body)
			if len(s.Fields) != 7 || len(s.Native.Code) != 2 {
				t.Fatal("code owner drift lacks complete declared shape")
			}
			tc.change(&s)
			if diff := cmp.Diff(tc.want, agreementNativeCodeAddressOwners(&s)); diff != "" {
				t.Fatalf("caught: complete code owner eligibility (-want +got):\n%s", diff)
			}
		})
	}
}
