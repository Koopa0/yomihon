package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

type agreementRawNamespaceSource struct {
	Ledger                 agreementOwnedCheckLiveSource
	Normalized, Raw, Check map[string]int
}

// A suffix belongs to its own raw field, even beside other prose or inside a
// container. Keep the whole original check ledger and page namespace separate;
// another grammar disagreement cannot lend its count to this spelling change.
func agreementRawNamespaceReading(body string) agreementRawNamespaceSource {
	native := agreementNativeOwnerReading(body)
	r := agreementRawNamespaceSource{
		Ledger:     agreementOwnedCheckLiveSource{Native: native, Check: agreementNativeCheckReading(body, native), Page: make(map[agreementCitation]int)},
		Normalized: make(map[string]int), Raw: make(map[string]int), Check: make(map[string]int),
	}
	for _, field := range r.Ledger.Check.Fields {
		if field.DefinitionOwner >= 0 || field.CodeOwner >= 0 {
			r.Ledger.Owned = append(r.Ledger.Owned, field)
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		if strings.ContainsAny(inner, "[]\r\n") {
			return agreementRawNamespaceSource{}
		}
		link, cites := graph.ParseWikilink(inner)
		outside := agreementDestinationLiveField{Source: field, Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}, Block: link.Block, Cites: cites, Comment: graph.In(native.Comments, field.Span.Start)}
		r.Ledger.Outside = append(r.Ledger.Outside, outside)
		active := !field.Code && !field.Comment && !field.Escaped
		if active {
			for _, name := range field.Targets {
				r.Normalized[name]++
			}
		}
		if !cites || field.Escaped || outside.Comment {
			continue
		}
		r.Ledger.Page[outside.Tuple]++
		if active && strings.HasSuffix(link.Target, "\\") && len(field.Targets) == 1 && field.Targets[0] != link.Target {
			r.Raw[link.Target]++
			r.Check[field.Targets[0]]++
		}
	}
	return r
}

type agreementRawNamespacePayload struct {
	Body   string
	Source agreementRawNamespaceSource
}

func agreementRawNamespaceBudget(body string, actual *agreementHTML) agreementRawNamespacePayload {
	r := agreementRawNamespaceReading(body)
	whole := make(map[string]int)
	for _, target := range r.Ledger.Check.Targets {
		whole[target]++
	}
	if len(r.Raw) == 0 || !agreementOwnedCheckLiveLedger(body, &r.Ledger) || !cmp.Equal(whole, r.Normalized) || !cmp.Equal(r.Ledger.Page, agreementNativeLiveCarriers(actual)) || len(actual.CodeCitations) != 0 || actual.CitationsInCode != 0 {
		return agreementRawNamespacePayload{}
	}
	return agreementRawNamespacePayload{Body: body, Source: r}
}

func agreementRawNamespaceDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget *agreementRawNamespacePayload) (kind, authority, wrong string) {
	if c.Body != budget.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	if !cmp.Equal(budget.Source, agreementRawNamespaceReading(c.Body)) || !cmp.Equal(budget.Source.Ledger.Page, agreementNativeLiveCarriers(actual)) || len(actual.CodeCitations) != 0 || actual.CitationsInCode != 0 {
		return "", "", ""
	}
	page := 0
	for tuple, count := range budget.Source.Ledger.Page {
		if tuple.Target == f.Tuple.Target {
			page += count
		}
	}
	delta := budget.Source.Normalized[f.Tuple.Target] - page
	paired := budget.Source.Check[f.Tuple.Target] - budget.Source.Raw[f.Tuple.Target]
	if paired == 0 || paired != delta {
		return "", "", ""
	}
	if f.Direction == "judge-only" && delta == f.Multiplicity || f.Direction == "page-only" && -delta == f.Multiplicity {
		return "debt", "#1011 stage 3", "page"
	}
	return "", "", ""
}

func agreementRawNamespaceDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, budget *agreementRawNamespacePayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementRawNamespaceDifference(other, f, actual, budget); kind != "" {
			t.Fatal("caught: raw namespace borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){
		func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
		func(f *agreementFailure) { f.Multiplicity = 0 }, func(f *agreementFailure) { f.Multiplicity = -1 },
	} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementRawNamespaceDifference(c, &changed, actual, budget); kind != "" {
			t.Fatalf("caught: raw namespace borrowed signature %s", agreementSignature(&changed))
		}
	}
	for _, carriers := range [][]agreementCitation{nil, append(append([]agreementCitation{}, actual.Citations...), agreementCitation{Target: "unowned", State: "wikilink-broken"}), {{Target: f.Tuple.Target, Section: "unowned", State: "wikilink-broken"}}} {
		changed := *actual
		changed.Citations = carriers
		if kind, _, _ := agreementRawNamespaceDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: raw namespace borrowed changed whole page")
		}
		if agreementRawNamespaceBudget(c.Body, &changed).Body != "" {
			t.Fatal("caught: raw namespace budget borrowed changed whole page")
		}
	}
	for _, change := range []func(*agreementHTML){func(a *agreementHTML) { a.CitationsInCode++ }, func(a *agreementHTML) {
		a.CodeCitations = append(a.CodeCitations, agreementCitation{Target: "unowned", State: "wikilink-broken"})
	}} {
		changed := *actual
		change(&changed)
		if kind, _, _ := agreementRawNamespaceDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: raw namespace borrowed code occurrence")
		}
		if agreementRawNamespaceBudget(c.Body, &changed).Body != "" {
			t.Fatal("caught: raw namespace budget borrowed code occurrence")
		}
	}
}

func TestAgreementRawNamespaceSourceDrift(t *testing.T) {
	t.Parallel()
	c := agreementCase{Body: "[[A\\]] [[A]] [[B#place]] [[A\\]] \\[[C]] %%[[D]]%%\n\n```\n[[E]]\n```\n"}
	_, actual := agreementIsolatedPage(t, c)
	budget := agreementRawNamespaceBudget(c.Body, &actual)
	f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Direction: "judge-only", Tuple: agreementCitation{Target: "A"}, Multiplicity: 2}
	if kind, authority, wrong := agreementRawNamespaceDifference(c, &f, &actual, &budget); kind != "debt" || authority != "#1011 stage 3" || wrong != "page" {
		t.Fatal("caught: raw namespace source drift control is unowned")
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementRawNamespaceSource)
	}{
		{"native code kind", func(s *agreementRawNamespaceSource) { s.Ledger.Native.Code[0].Kind = "unowned" }},
		{"native code span", func(s *agreementRawNamespaceSource) { s.Ledger.Native.Code[0].Span.Start++ }},
		{"native comment span", func(s *agreementRawNamespaceSource) { s.Ledger.Native.Comments[0].Stop++ }},
		{"native definition", func(s *agreementRawNamespaceSource) {
			s.Ledger.Native.Definitions = append(s.Ledger.Native.Definitions, agreementNativeOwnerDefinition{Label: "unowned"})
		}},
		{"whole check names", func(s *agreementRawNamespaceSource) {
			s.Ledger.Check.Targets = append(s.Ledger.Check.Targets, "unowned")
		}},
		{"whole check code", func(s *agreementRawNamespaceSource) { s.Ledger.Check.Code = nil }},
		{"whole check comments", func(s *agreementRawNamespaceSource) { s.Ledger.Check.Comments = nil }},
		{"whole check order", func(s *agreementRawNamespaceSource) {
			s.Ledger.Check.Fields[0], s.Ledger.Check.Fields[1] = s.Ledger.Check.Fields[1], s.Ledger.Check.Fields[0]
		}},
		{"check field word", func(s *agreementRawNamespaceSource) { s.Ledger.Check.Fields[0].Word = "[[unowned]]" }},
		{"check field span", func(s *agreementRawNamespaceSource) { s.Ledger.Check.Fields[0].Span.Start++ }},
		{"check normalized field", func(s *agreementRawNamespaceSource) { s.Ledger.Check.Fields[0].Targets = []string{"unowned"} }},
		{"owned partition", func(s *agreementRawNamespaceSource) { s.Ledger.Owned = nil }},
		{"owned index", func(s *agreementRawNamespaceSource) { s.Ledger.Owned[0].CodeOwner++ }},
		{"outside partition", func(s *agreementRawNamespaceSource) { s.Ledger.Outside = s.Ledger.Outside[1:] }},
		{"outside order", func(s *agreementRawNamespaceSource) {
			s.Ledger.Outside[0], s.Ledger.Outside[1] = s.Ledger.Outside[1], s.Ledger.Outside[0]
		}},
		{"outside role", func(s *agreementRawNamespaceSource) { s.Ledger.Outside[0].Cites = false }},
		{"outside comment", func(s *agreementRawNamespaceSource) { s.Ledger.Outside[0].Comment = true }},
		{"outside block", func(s *agreementRawNamespaceSource) { s.Ledger.Outside[0].Block = "unowned" }},
		{"outside tuple", func(s *agreementRawNamespaceSource) { s.Ledger.Outside[0].Tuple.Section = "unowned" }},
		{"outside field flags", func(s *agreementRawNamespaceSource) { s.Ledger.Outside[0].Source.Escaped = true }},
		{"whole page namespace", func(s *agreementRawNamespaceSource) {
			s.Ledger.Page[agreementCitation{Target: "unowned", State: "wikilink-broken"}]++
		}},
		{"page tuple section", func(s *agreementRawNamespaceSource) {
			s.Ledger.Page[agreementCitation{Target: "A", Section: "unowned", State: "wikilink-broken"}]++
		}},
		{"normalized namespace", func(s *agreementRawNamespaceSource) { s.Normalized["B"]++ }},
		{"raw pair namespace", func(s *agreementRawNamespaceSource) { s.Raw["unowned"]++ }},
		{"check pair namespace", func(s *agreementRawNamespaceSource) { s.Check["unowned"]++ }},
		{"paired difference", func(s *agreementRawNamespaceSource) { s.Check["A"]++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementRawNamespacePayload{Body: c.Body, Source: agreementRawNamespaceReading(c.Body)}
			tc.change(&changed.Source)
			if kind, _, _ := agreementRawNamespaceDifference(c, &f, &actual, &changed); kind != "" {
				t.Fatal("caught: raw namespace borrowed changed complete source")
			}
		})
	}
}

func TestAgreementRawNamespaceSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementRawNamespaceSource
	}{
		{name: "mixed repeated names", body: "[[A\\]] [[A]] [[B]] [[A\\]]\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: nil, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 7, Stop: 12}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 13, Stop: 18}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 19, Stop: 25}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "A", "B", "A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 7, Stop: 12}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 13, Stop: 18}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 19, Stop: 25}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 2, {Target: "A", Section: "", State: "wikilink-broken"}: 1, {Target: "B", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 3, "B": 1}, Raw: map[string]int{"A\\": 2}, Check: map[string]int{"A": 2}}},
		{name: "quoted and list fields", body: "> [[A\\]] [[B]]\n\n- [[A\\]]\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: nil, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 2, Stop: 8}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 9, Stop: 14}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 18, Stop: 24}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B", "A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 2, Stop: 8}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 9, Stop: 14}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 18, Stop: 24}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 2, {Target: "B", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 2, "B": 1}, Raw: map[string]int{"A\\": 2}, Check: map[string]int{"A": 2}}},
		{name: "ordinary inline owners", body: "**words** [[A\\]] `words`\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeSpan", Parent: "Paragraph", Span: graph.Span{Start: 18, Stop: 23}}}, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 18, Stop: 23}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 10, Stop: 16}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 10, Stop: 16}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{"A\\": 1}, Check: map[string]int{"A": 1}}},
		{name: "literal code remains separate", body: "[[A\\]]\n\n```\n[[B]]\n```\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "FencedCodeBlock", Parent: "Document", Span: graph.Span{Start: 12, Stop: 18}}}, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 12, Stop: 18}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 12, Stop: 17}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"A"}}, Owned: []agreementNativeCheckField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{"A\\": 1}, Check: map[string]int{"A": 1}}},
		{name: "unused definition supplies active check names", body: "[[A\\]]\n\n[^n]: [[B]]\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 8, Stop: 20}, Name: graph.Span{Start: 8, Stop: 13}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 14, Stop: 19}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 14, Stop: 15}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 15, Stop: 16}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 16, Stop: 18}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 19}}}}}}, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: nil, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 14, Stop: 19}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}}, Targets: []string{"A", "B"}}, Owned: []agreementNativeCheckField{{Span: graph.Span{Start: 14, Stop: 19}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}}, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{"A\\": 1}, Check: map[string]int{"A": 1}}},
		{name: "used definition stays in original body", body: "ref[^n]\n\n[^n]: [[A\\]] [[B]]\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: nil, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 15, Stop: 21}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 15, Stop: 21}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 22, Stop: 27}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 1, {Target: "B", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1, "B": 1}, Raw: map[string]int{"A\\": 1}, Check: map[string]int{"A": 1}}},
		{name: "escaped comment and alias names", body: "[[A\\]] \\[[B\\]] %%[[C]]%% [[D\\|shown]]\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: []graph.Span{{Start: 15, Stop: 24}}}, Check: agreementNativeCheckSource{Code: nil, Comments: []graph.Span{{Start: 15, Stop: 24}}, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 8, Stop: 14}, Word: "[[B\\]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[C]]", Targets: []string{"C"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 25, Stop: 37}, Word: "[[D\\|shown]]", Targets: []string{"D"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "D"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 8, Stop: 14}, Word: "[[B\\]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: true, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "B\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 17, Stop: 22}, Word: "[[C]]", Targets: []string{"C"}, Code: false, Comment: true, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "C", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: true}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 25, Stop: 37}, Word: "[[D\\|shown]]", Targets: []string{"D"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "D", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 1, {Target: "D", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1, "D": 1}, Raw: map[string]int{"A\\": 1}, Check: map[string]int{"A": 1}}},
		{name: "blocks headings and local fields", body: "[[A\\]] [[B#^a]] [[C#place|shown]] [[#local]]\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: nil, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 7, Stop: 15}, Word: "[[B#^a]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 16, Stop: 33}, Word: "[[C#place|shown]]", Targets: []string{"C"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 34, Stop: 44}, Word: "[[#local]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B", "C"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 0, Stop: 6}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 7, Stop: 15}, Word: "[[B#^a]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "B", Section: "", State: "wikilink-broken"}, Block: "a", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 16, Stop: 33}, Word: "[[C#place|shown]]", Targets: []string{"C"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "C", Section: "place", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 34, Stop: 44}, Word: "[[#local]]", Targets: []string{}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "", Section: "local", State: "wikilink-broken"}, Block: "", Cites: false, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 1, {Target: "B", Section: "", State: "wikilink-broken"}: 1, {Target: "C", Section: "place", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1, "B": 1, "C": 1}, Raw: map[string]int{"A\\": 1}, Check: map[string]int{"A": 1}}},
		{name: "Unicode and CRLF widths", body: "章節 [[A\\]] [[B]]\r\n", want: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: nil, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 7, Stop: 13}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 14, Stop: 19}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 7, Stop: 13}, Word: "[[A\\]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 14, Stop: 19}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{Target: "A\\", Section: "", State: "wikilink-broken"}: 1, {Target: "B", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1, "B": 1}, Raw: map[string]int{"A\\": 1}, Check: map[string]int{"A": 1}}},
		{name: "nested outside word", body: "[[E[[F]]]] [[A\\]]\n", want: agreementRawNamespaceSource{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agreementRawNamespaceReading(tc.body)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("caught: complete raw namespace source (-want +got):\n%s", diff)
			}
			if len(tc.want.Raw) > 0 && !agreementOwnedCheckLiveLedger(tc.body, &got.Ledger) {
				t.Fatal("caught: raw namespace source lacks complete public byte ledger")
			}
		})
	}
}
func TestAgreementRawNamespacePublic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "mixed repeated names", body: "[[A\\]] [[A]] [[B]] [[A\\]]\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "quoted and list fields", body: "> [[A\\]] [[B]]\n\n- [[A\\]]\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "ordinary inline owners", body: "**words** [[A\\]] `words`\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "literal code remains separate", body: "[[A\\]]\n\n```\n[[B]]\n```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "unused definition supplies active check names", body: "[[A\\]]\n\n[^n]: [[B]]\n", want: nil},
		{name: "used definition stays in original body", body: "ref[^n]\n\n[^n]: [[A\\]] [[B]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "escaped comment and alias names", body: "[[A\\]] \\[[B\\]] %%[[C]]%% [[D\\|shown]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "blocks headings and local fields", body: "[[A\\]] [[B#^a]] [[C#place|shown]] [[#local]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "Unicode and CRLF widths", body: "章節 [[A\\]] [[B]]\r\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "nested outside word", body: "[[E[[F]]]] [[A\\]]\n", want: nil},
		{name: "ordinary target already agrees", body: "[[A]] [[B]]\n", want: nil},
		{name: "alias already agrees", body: "[[A\\|shown]] [[B]]\n", want: nil},
		{name: "escaped suffix stays literal", body: "\\[[A\\]]\n", want: nil},
		{name: "wrapped outside refuses", body: "[[A\nB]] [[A\\]]\n", want: nil},
		{name: "converted code refuses", body: "`open\n[[A]]\nclose`\n\n[[A\\]]\n", want: nil},
		{name: "metadata active names refuse", body: "``` [[B]]\n```\n\n[[A\\]]\n", want: nil},
		{name: "full original raw namespace 0", body: " ^A\n%%[[A]]%%[[A\\]]%%[[A]]%%  ```\n```\n[[B|alias]]- [x] [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- [x] [[B]]\n```open\n[[A]]\nclose`# A\n\\\\[[A]]\n\n## A\n1. > > ![[image.png]]# A\n  ```\n\\[[A]][[image.png]] ^é\n[[A]] [[A]][[#A]]<div>\n[[A]]\n</div>\n[[A]][^unused]: [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "raw pairs refuse an ordinary outside cancellation", body: " ^é\n``https://example.invalid/`[[A]]`   - [[A]] [[A]]<!--[[A\\|alias]]``- > [!note] title\nÉ\n> [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n[^n]: [[A]]\n\n    [[B]]\n    ^absent-prose [[A\\|alias]]ref[^n]\n[[A\\]]\n\nA\n[[A\\]]\n", want: map[string]int{"page-only/A\\": 2}},
		{name: "full original raw namespace 1", body: "A\n===\n ^A\n> - [ ] [[A]]\n- [ ] [[A]]\n0[[A\\]][[A]]%%[[A]]%%\\[[A]] ^a-2\n# A\nÉ\n`\\\\[[A]]> > [[A\\|alias]]0-  ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 2", body: "   ```\n-->\t```\n<!-- [[A]] -->0> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n\t## A\n## A\n<div>\n[[A]]\n</div>\n\\` ^a-2\nref[^n]\n ```\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]\\\\[[A]]## A\n- item\n\n      ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 3", body: "[[A\\]]  - ## A-2\n## A\n## A\n  ```\n ^a\n\\`0\\[[A]]\n章節\n[[#A]][[A]][[A#^a]]- item\n\n       `~~~\n## A\nÉ\nA\n---\n<!-- [[A]] -->[^unused]: [[A]]\n\t0%%\t>  ^A\n[[#A]]\\\\[[A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 4", body: "> [!unknown] title\n> [!unknown] title\n[[A\\]]`[[A]]`[[A\\]][[B|alias]]\n```\n    ```\n> [!note] [[A]]\n1. ## A-2\n## A\n## A\n`\\`[[A]]", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 5", body: "![[image.png]]## <em>A</em>\n[[A\\]] ^é\n^absent-prose [[A\\|alias]][[A]]![[image.png]]É\n<div>\n[[A]]\n</div>\n-->[[A#^a]]ref[^n]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 6", body: "<!--\n[[A]]\n--><!--\n[[A]]\n-->![[A]]0     ```\n    ```\n> [!unknown] title\n[[A\\]]> [!note] title\n![[image.png]][[A]] ^é\n~~~\n>  ```\n- > [!note] title\n- ![[A]]  - ## A-2\n## A\n## A\n%%[[A]]%%- item\n\n      \n> [!note] [[A]]\n\\\\[[A]][[A]]https://example.invalid/`[[A]]` ## !\n  - ````\n    ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 7", body: "> [!note] one\n> [!note] two\n> [!note] three\nÉ\n     ^é\n<!--     ^a-2\n\n[[A#A]][[A#A]]<!--`open\n[[A]]\nclose` ^a-2\n ^A\n-->> [!note] title\n[[A\\]]%%[[A]]%%[[B|alias]]    É\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 8", body: "0 ```\n^absent-prose - item\n\n      [^n]: [[A]]\n\n    [[B]]\n![[A#A]]~~~~\n[[A\\]]![[image.png]]\tref[^n]\n[[#A]]- item\n\n      [[A#A]] ^é\n~~~~\n````\n````\n<!--\n[[A]]\n-->~~~\n1. - B\n# A\n<!--\n[[A]]\n--> ^é\n[[A]]`[[A]]`%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 9", body: "# A\nA\n===\n\\\\[[A]][[A\\|alias]][[A\\]]<!--\n[[A]]\n--> ^é\n<!--\n[[A]]\n-->^absent-prose - [ ] [[A]]\n``- ![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n [[image.png]]ref[^n]\n-->```\ntext   > [!note] title\n```\n[[A\\]]É\n%%[[A]]%%[[A#A]]## !\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 10", body: "`[[A]]`## !\nÉ\n[[A\\]]  - \\[[A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 11", body: "![[image.png]]<!-- [[A]] -->[[A\\]]> [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 12", body: "- item\n\n      [[B|alias]]text \t```\n\t> > - item\n\n      > [!note] title\n ^é\nA\n===\n<div>\n[[A]]\n</div>\n[[A#A]] ^a-2\nref[^n]\n[[A\\]]  ```\n\t## !\n ref[^n]\n`open\n[[A]]\nclose`É\n1. ~~~\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 13", body: "1. ref[^n]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A\\]][[A]] [[A]][[A]] [[A]]~~~\n[[A\\|alias]]```` go [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 14", body: "## <em>A</em>\n- [x] [[B]]\n\t<div>\n[[A]]\n</div>\nÉ\n ^é\n\\\\[[A]]> - [ ] [[A]]\nÉ\n[[A\\]]0", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 15", body: "[[A\\]][^unused]: [[A]]\n\\[[A]][[A\\]] ```\n## [[A|alias]]\ntext - [ ] [[A]]\nB\n- [ ] [[A]]\n~~~~\nA\n===\n   ```\nB\n\n\n```` go [[A]]\n> [!note] title\nÉ\n~~~~\nA\n===\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 16", body: "0- [^unused]: [[A]]\n[[image.png]]    | a | b |\n|---|---|\n| [[A]] | ^a |\n\n ^é\n| a | b |\n|---|---|\n| [[A]] | ^a |\n^absent-prose [[A\\]][[A#A]][[A]] ^a-2\n\n\n ^a-2\n- > [!note] title\n-->- item\n\n       ^A\n- > [!note] title\n[[A\\]]``` [[A]]\nA\n![[image.png]]~~~~\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 17", body: "[[#A]]A\n===\n[[A\\]]![[A#A]]```` go [[A]]\n` ```\n## [[A|alias]]\n\n- > [!note] title\nB\n0%%  ```\n# A\n# A\n````\nÉ\n   ```\n`[[A]]` \\`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 18", body: "[[A\\]]<!--[^unused]: [[A]]\n## !\n| a | b |\n|---|---|\n| [[A]] | ^a |\n1. ## <em>A</em>\n ```\nA\n---\nÉ\n> ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 19", body: "- item\n\n      ## <em>A</em>\n![[A]][[A\\]]%%0  > [!note] title\n> [!unknown] title\n<!-- [[A]] -->> [!note] one\n> [!note] two\n> [!note] three\n# A\ntext %%[[A]]%%[[A]] [[A]]`## A\n<!--\n[[A]]\n-->> [!note] title\n  - - [ ] [[A]]\n`open\n[[A]]\nclose`[[A\\|alias]]- [ ] [[A]]\n\n\n ^é\n  -  ^a\n%%<!--\n[[A]]\n-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 20", body: "B\n[[A\\]]    ```\n   ```\n[[A#A]]![[image.png]] ^a-2\n![[A#A]]^absent-prose ~~~~\n## !\n> > [[A\\]]```` go [[A]]\n[^n]: [[A]]\n\n    [[B]]\n[[A#A]]A\n- > [!note] title\n\n~~~\n\\`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 21", body: "text > [!note] title\n````\n````\n\n\n`[[A]]`![[image.png]][[image.png]]text `    %%[[A]]%%## !\n[[A\\]][[A\\]][[A#^a]]%%", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 22", body: "> [!note] title\n   ```\nA\nA\n---\n   ```\n> [!note] one\n> [!note] two\n> [!note] three\n  - \t```\n[[A\\]]## A\n## A\n[[A#^a]]^absent-prose \\\\[[A]]A\n===\n## [[A|alias]]\n<!-- [[A]] -->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 23", body: "[[A\\]]`[[A]]`\t```\n  ```\nref[^n]\n   ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 24", body: "[[A\\]]É\n~~~\n## A-2\n## A\n## A\n[[image.png]]\t```\n> [!note] title\n  - - ## <em>A</em>\n[^n]: [[A]]\n\n    [[B]]\nB\n- [ ] [[A]]\nA\n===\n~~~\n## A-2\n## A\n## A\n  > [!note] title\nA\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 25", body: "\n\n- [ ] [[A]]\n[[#A]]  > [!note] title\n\n\n[[A\\]]  ```\nÉ\n\t```\n[[A#^a]]```\n[[B|alias]] ^a-2\n- > [!note] title\n`[[A]]`\t```\n\\[[A]]``[^n]: [[A]]\n\n    [[B]]\n\nÉ\n- > [!note] title\n> [!unknown] title\n> > > [!note] title\n\\[[A]]    ```\n  - 1.  ^A\n    ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 26", body: "## A\n1. [[A\\|alias]]A\n\t## A\n## A\n- [ ] [[A]]\nÉ\n[[B|alias]]- [ ] [[A]]\ntext [[A\\]][[#A]]A\n---\n  ```\n## A\n> [!note] title\nA\n===\n<div>\n[[A]]\n</div>\nÉ\n 0text [[A#A]] ```\n<!--\n[[A]]\n--><!--text [^n]: [[A]]\n\n    [[B]]\n  > [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 27", body: "> [!unknown] title\n ![[A#A]]-->[^unused]: [[A]]\n[[A]]![[A#A]]# A\nA\n---\n\t## A-2\n## A\n## A\n  -  ^é\n0[[A#^a]]A\nA\n\\[[A]][[A\\]]1. ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 28", body: "\n0 ```\n> [!note] title\n0[^unused]: [[A]]\n00[^n]: [[A]]\n\n    [[B]]\n[[A\\]]## A\n## A\n[[A#A]]A\n===\nA\n---\n^absent-prose - [ ] [[A]]\n> [!unknown] title\n ^a-2\n## A\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 29", body: "## A\n  > [!note] title\n## A-2\n## A\n## A\n## [[A|alias]]\n-->[[A\\]]<!-- [[A]] -->[[A#A]]<!-- [[A]] -->- [x] [[B]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 30", body: "## A-2\n## A\n## A\n> > A\n![[A#A]]<!-- [[A]] -->B\n> [!note] title\n\\\\[[A]][[A\\]] ^é\n   ```\n  - text > > ## A\n## A\n  - %%%%A\n``", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 31", body: "[[A\\|alias]][[A\\]]A\n---\n`[[A]]`# A\n ^é\ntext  ^a\n\\\\[[A]]1. É\n> > > [!note] one\n> [!note] two\n> [!note] three\n## [[A|alias]]\n## A\n## A\nref[^n]\n# A\n\t```\n<div>\n[[A]]\n</div>\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 32", body: "[[A#^a]]    ```\n[[A\\]]ref[^n]\n- [[A]]text %%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 33", body: "[[A\\]] - [x] [[B]]\n    ```\n![[image.png]]\t <!-- [[A]] -->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 34", body: "0 ^é\n> [!unknown] title\n[[A\\]]> A\n---\n- [ ] [[A]]\n  > [!note] title\n````\n- [ ] [[A]]\n\\`[[A\\|alias]]``0![[A#A]]<!--\n[[A]]\n-->^absent-prose > [!note] [[A]]\n![[A]]## !\n- [ ] [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 35", body: "[[A\\]] ^é\n![[image.png]]<!-- [[A]] -->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 36", body: "text ``open\n[[A]]\nclose`[[A\\|alias]]\\[[A]]`open\n[[A]]\nclose`A\n===\n[[A\\]] ^A\n## A-2\n## A\n## A\n<div>\n[[A]]\n</div>\n\n\n> [!unknown] title\n\n\n[[A#A]]É\n\\\\[[A]]> ```\n- item\n\n      [^unused]: [[A]]\nref[^n]\n  -  ^é\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 37", body: "## [[A|alias]]\n\t```\n\n\n%%[[A]]%% ^é\n![[A#A]][[A\\]]## !\n![[image.png]]<!--````` go [[A]]\nref[^n]\n> [!note] [[A]]\n ^a\nÉ\n> [!unknown] title\n[[A\\]]    ```\n- [ ] [[A]]\n  ```\n```\n> [!note] title\n[[A]][[A\\|alias]]ref[^n]\n[[A]]> ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 38", body: "# A\n\\`\n\n ^é\n[[B|alias]]> [!unknown] title\n<div>\n[[A]]\n</div>\n## A\n- > [!note] title\n[[A#^a]]> [!note] [[A]]\n[[A\\]] ^é\n[[image.png]]\\\\[[A]][[A#A]]-->\\[[A]]- [ ] [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 39", body: "章節\n- \t[[A\\|alias]]  ^A\n# A\nA\n---\nA\n---\n> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n[[A\\]]> > ^absent-prose [[A]]A\n---\n    https://example.invalid/`[[A]]` > > > [!note] title\n   ```\n# A\n## [[A|alias]]\n  ```\n> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 40", body: "## A\n## A\n<!-- [[A]] -->``[[A\\]]^absent-prose ## <em>A</em>\n  ```\n[[B|alias]]## A\n## A\n> [!note] [[A]]\n[[A\\]][[A\nB]]```` go [[A]]\n> [!note] [[A]]\n> [!unknown] title\n0[[A#^a]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 41", body: "[[A#^a]]  ```\n![[image.png]]> > ~~~\n ^é\n[[A\\]] ^a-2\n## A-2\n## A\n## A\n````\n<div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n%%[[A]]%% ^A\n> >    ```\n~~~~\n章節\n-->## A-2\n## A\n## A\n%%[[A]]%%<!-- [[A]] -->## [[A|alias]]\n![[A]]- > [!note] title\nA\n---\nÉ\n[[image.png]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 42", body: "\\[[A]]\\[[A]]ref[^n]\n  > [!note] title\n- item\n\n      > > [[A#A]]  > [!note] title\n\t```\n\n\n[[A\\]][[A\\|alias]]> ````\n## A\n## A\n- item\n\n       ^a\n\t```\n ^a\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 43", body: "<div>\n[[A]]\n</div>\n--> ^é\n ^é\n[[A\\]]## A\n> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 44", body: " ^a\nÉ\n``![[A]]章節\nÉ\n ^a\n<!-- [[A]] -->[[A\\]]章節\n[[A#A]][[#A]]0## !\n> [!note] title\n- > [!note] title\n![[A#A]]## A\n## A\n- [ ] [[A]]\nÉ\n0--><!-- [[A]] -->- > [!note] title\n> 1.   > [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 45", body: "- [[A\\]]\n\n%%0<!--\n[[A]]\n-->^absent-prose ## [[A|alias]]\n> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 46", body: "<div>\n[[A]]\n</div>\n[[A\\]]0   ```\n# A\n ^é\n[[A#^a]]A\n`open\n[[A]]\nclose`![[A#A]]É\n[[A]] [[A]]> [!note] [[A]]\n ^A\n`open\n[[A]]\nclose`  ```\n- [x] [[B]]\n    ```\n`open\n[[A]]\nclose`A\n<!--\t```\n%%> [!note] title\n![[A]]## !\n    ```\nA\n===\nA\n---\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 47", body: "A\n ^a-2\n## !\n![[image.png]][[A\\]]- item\n\n      [^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 48", body: "![[A#A]]````\nA\n-->A\n---\n## A-2\n## A\n## A\n``[[A\\]]| a | b |\n|---|---|\n| [[A]] | ^a |\n`[[A\\|alias]]<!-- [[A]] -->- item\n\n      B\n%%[^unused]: [[A]]\n- > [!note] title\n`    ```\nB\n章節\n0", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 49", body: "- item\n\n      ![[A]]- [ ] [[A]]\n<!---  ^A\n\\[[A]]<!-- [[A]] -->## !\n[[A#^a]]É\n[[A#^a]][^unused]: [[A]]\n[[A\\]]   ```\n\t```\n[[B|alias]]## !\n%%ref[^n]\n    ```\n```` go [[A]]\n <!--`0`open\n[[A]]\nclose`## A\n## A\n  > [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 50", body: "0B\n ^é\n    ```\n## A\n## A\n## <em>A</em>\n[[A\\]]B\n  ```\n1. [[A\nB]]## <em>A</em>\n A\n---\n\n![[image.png]][[#A]][[image.png]] ```\n\n> > -->text - \n\n```` go [[A]]\n<div>\n[[A]]\n</div>\n   ```\n%%<!--\n[[A]]\n-->> [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 51", body: "> > [[A#A]]%%[[A]]%%## A\n## A\n ^é\n[[A]] [[A]]\\`[[#A]][[A\\|alias]]## !\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#^a]]![[image.png]] ^A\nA\n[[A\\]]  - ## <em>A</em>\n> > ## !\n## [[A|alias]]\n  ```\n```` go [[A]]\n\n0-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 52", body: "> [!note] title\n> [!note] title\nA\nref[^n]\n\\[[A]][[A\\]]~~~\n> [!note] title\n`[[A]]```` [[A]]\n> ## !\n0-->0[[A#A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 53", body: "\n![[image.png]]- [x] [[B]]\n## A\n  > [!note] title\nA\n===\n-  ^é\n> [!unknown] title\n![[A]]  - ## A\n<div>\n[[A]]\n</div>\n> [!unknown] title\n[[A#A]]\\\\[[A]] ^é\n> > - item\n\n      ![[A]]1. # A\n> > -->[[A\\]]> [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n    ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 54", body: "^absent-prose - > [!note] title\n    ![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]``` [[A]]\n<!-- [[A]] -->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 55", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n````\n ^é\n\\\\[[A]]A\n---\n ^a\n\n0````\n``` [[A]]\n````\n\\\\[[A]]ref[^n]\n[[A\\]]A\n---\n- [x] [[B]]\n ^é\n> [!note] one\n> [!note] two\n> [!note] three\n## A-2\n## A\n## A\n[[A\\]]\n\n``~~~~\n\\[[A]]章節\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 56", body: "\\[[A]]%%[[A]]%%> [!note] title\n[[A\\]][[A]] [[A]] ^a\n%%````\n章節\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 57", body: "  > [!note] title\n## [[A|alias]]\n## [[A|alias]]\n> ^absent-prose `[[A]]`^absent-prose ## [[A|alias]]\n## !\n1. ## A\n## A\n^absent-prose > > <!-- [[A]] -->[[A\\]]## A\n## A\n> <!--`open\n[[A]]\nclose`![[image.png]]  - \n\n# A\nB\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 58", body: "%%[[A]]%%> [!unknown] title\n1.  ^a-2\n[[B|alias]]<!-- [[A]] -->> [!note] [[A]]\n[[A\\]][^n]: [[A]]\n\n    [[B]]\n<!--[[A#^a]] ^A\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 59", body: "ref[^n]\nB\n[[A\\]]> [!note] title\n``0<!--[[A\\]]``\n| a | b |\n|---|---|\n| [[A]] | ^a |\n [[#A]]B\nref[^n]\n- [ ] [[A]]\n%%[[A]]<div>\n[[A]]\n</div>\n<!--\n^absent-prose <div>\n[[A]]\n</div>\n<div>\n[[A]]\n</div>\n>    ```\n\\[[A]]> [!note] one\n> [!note] two\n> [!note] three\n   ```\nhttps://example.invalid/`[[A]]`  ^a-2\n`open\n[[A]]\nclose`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 60", body: "%%[[A]]%%## A\n## A\n ^é\nA\n---\n ^a-2\n章節\n## A\n[[A\\]]# A\n\\`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 61", body: "> [!note] title\n<!--\n[[A]]\n-->[[image.png]][[A\\]][[A]]  ```\n%%[[A]]%%\t```\nÉ\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 62", body: "``[[A\\|alias]]<!--A\n---\n[[B|alias]]```\n## !\n-->[[B|alias]] ^a\n    ```\n```\n   ```\n[[A\\]]~~~~\n> > text [^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 63", body: " ^é\n[[A\\|alias]][[A\\]]## [[A|alias]]\n[[A#^a]]  - [[B|alias]]![[A]]\\[[A]]   ```\n0    ```\n<!--\n[[A]]\n--><!-- ^A\n    ```\n`open\n[[A]]\nclose```", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 64", body: "## A\n[[A#A]]~~~~\n[[A\\]][[A\\|alias]]\t```\n`[[A]]`text ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 65", body: "[[A]] [[A]] ^A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A\\]][^n]: [[A]]\n\n    [[B]]\n<!--`open\n[[A]]\nclose`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 66", body: "<!--\n[[A]]\n-->[[A\\]]A\n\\`[^n]: [[A]]\n\n    [[B]]\n<!--~~~\n^absent-prose  ^é\n    \t[[image.png]][[A]]~~~\n    - [x] [[B]]\n ^a-2\n%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 67", body: " ^é\n``https://example.invalid/`[[A]]`   - [[A]] [[A]]<!--[[A\\|alias]]``- > [!note] title\nÉ\n> [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n[^n]: [[A]]\n\n    [[B]]\n    ^absent-prose [[A\\|alias]]ref[^n]\n[[A\\]]\n\nA\n", want: map[string]int{"page-only/A\\": 1}},
		{name: "full original raw namespace 68", body: "--><div>\n[[A]]\n</div>\n[[A]]- > [!note] title\n[[A\\]]<!--\n[[A]]\n-->> [!unknown] title\n## <em>A</em>\n  > [!note] title\n## A\n`[[A#A]]> [!note] [[A]]\nA\n  - \\[[A]]A\n[[A#^a]][^n]: [[A]]\n\n    [[B]]\n- > [!note] title\n-->`<!--\n[[A]]\n-->0[^n]: [[A]]\n\n    [[B]]\n[[A#A]]0A\n---\n~~~\n![[image.png]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 69", body: "`[[A]]` ^A\n[[A\\]]~~~~\n![[A#A]]B\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 70", body: "\t```\nA\n===\n[[A\\]]章節\nÉ\n    <!--\n[[A]]\n-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 71", body: "[[A\\|alias]][[A]]A\n===\nB\n^absent-prose [^unused]: [[A]]\n`[[A]]`[[A]] [[A]]\\[[A]]```` go [[A]]\n    ```\nref[^n]\n[[A\\]]![[A#A]]É\n## A\n## A\n<!--[[#A]]\t```\n``` [[A]]\n\\[[A]] ^a-2\n~~~\n ```\n\\`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 72", body: "0> [!note] [[A]]\n[[A]] [[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n ^é\n[[A\\]]\\[[A]]<!--\n[[A]]\n-->%% ^é\n<!--\n[[A]]\n-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 73", body: "![[A#A]]<!--\n[[A]]\n-->[[A\\]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 74", body: "> [[A\\]]  > [!note] title\n- > [!note] title\n> > [!note] title\n1. ## [[A|alias]]\n  > [!note] title\nA\n---\n%%[[A]]%%[[B|alias]]0<!--![[image.png]][[#A]][[A\\|alias]] ^a\n![[A#A]][[#A]]``## A\n> [!note] [[A]]\n## A-2\n## A\n## A\n````\nÉ\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 75", body: "[[A#A]][[A]]B\n^absent-prose [[A\\]] ^é\n> [!unknown] title\n> [!note] one\n> [!note] two\n> [!note] three\n    ```\n  > [!note] title\n%%[[A]]%%## A\n## A\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 76", body: "ref[^n]\n ^a-2\n0[[A\\]] ^é\nhttps://example.invalid/`[[A]]`  ```\n[^n]: [[A]]\n\n    [[B]]\n> [!unknown] title\nA\n---\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 77", body: "[[A\\]] ^A\n\\`# A\n`[[A]]`B\n0%%![[A#A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 78", body: " ^a-2\n[[A\\]]1.   ```\n<div>\n[[A]]\n</div>\n\\`<!-- [[A]] --><!--\n[[A]]\n--># A\n  - ## <em>A</em>\n## !\n- [x] [[B]]\n[[A]]^absent-prose É\n- > [!note] title\n- > [!note] title\n<div>\n[[A]]\n</div>\n    ```\n![[A]][[A\\|alias]][[A#A]]text ```` go [[A]]\n## A\n## A\n- ## <em>A</em>\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 79", body: "-->[[A\\]]A\n---\n## [[A|alias]]\n```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[^unused]: [[A]]\n[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n%%````\n    ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 80", body: "## !\nA\n---\n![[A#A]]\\\\[[A]]<div>\n[[A]]\n</div>\n\t```\n> > A\n===\n## A-2\n## A\n## A\n ^A\n[[image.png]]1. [[A\\]] ```\n# A\n<!-- [[A]] -->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 81", body: "## A-2\n## A\n## A\n[[A\\]]- item\n\n      -->````\n````\n[[A\nB]][[A\\|alias]]<!-- [[A]] -->\n\nÉ\n ^é\nA\n---\n`open\n[[A]]\nclose`## A\n## A\n-->[^n]: [[A]]\n\n    [[B]]\n^absent-prose ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 82", body: "[[A\\]][[A]] [[A]]- [x] [[B]]\nB\n ^a\n> [!note] one\n> [!note] two\n> [!note] three\n<!--\n[[A]]\n-->[[A\\|alias]]\tÉ\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 83", body: "[[A\\]] ^é\n0## <em>A</em>\n[[B|alias]]````\n ^é\n`%%[[A]]%%> [!unknown] title\n`open\n[[A]]\nclose`> [!note] title\n    ```\n ^a\n\t%%[[A]]%%## A-2\n## A\n## A\n> [!unknown] title\n--><!--\n[[A]]\n-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 84", body: "%%[[A]]%%> [!unknown] title\n## <em>A</em>\n![[A#A]]![[A#A]]- [x] [[B]]\n![[A#A]]~~~~\n- item\n\n      - [x] [[B]]\n## A\n![[A#A]]1. -->[[A\\]]\\\\[[A]]0 ^é\n~~~~\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A\n- [ ] [[A]]\n[^unused]: [[A]]\n%%[[A]]%% ^é\n[^unused]: [[A]]\n``` [[A]]\n> \t```\n\t```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 85", body: "  > [!note] title\n0[[A\\]]1. [[A#A]]~~~~\nB\n%%[[A]]%%- [x] [[B]]\n~~~\n[[A\\]]<!-- [[A]] -->````\n``-->>  ^a-2\n<!-->  ^é\nB\n ^A\n- item\n\n      ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 86", body: "\\`^absent-prose 章節\n## A-2\n## A\n## A\n0<div>\n[[A]]\n</div>\n\t```\n ^A\n[[A\\]]> >  | a | b |\n|---|---|\n| [[A]] | ^a |\n`![[A]]É\n> [!unknown] title\n- [ ] [[A]]\n`- item\n\n      [[A#^a]] ^a-2\n- item\n\n      - [ ] [[A]]\n![[image.png]]É\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 87", body: "B\n> [!note] title\n## A\n## A\n\n\n[[A\\]]\t``` [[A]]\n^absent-prose A\n---\n ^A\n[[A#A]]> [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n> [!note] title\n\t```\n```\n[[A]] [[A]][[#A]] ^a-2\n  > [!note] title\nref[^n]\n> [!note] one\n> [!note] two\n> [!note] three\n%%> [!unknown] title\n%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 88", body: "## !\n\n[[A\\]]## A-2\n## A\n## A\n`> ~~~~\n<!--\\`> [!note] one\n> [!note] two\n> [!note] three\nA\n ![[A]]![[A#A]][^n]: [[A]]\n\n    [[B]]\n^absent-prose ``É\n章節\n- item\n\n      ```\n[[image.png]]  > [!note] title\n ^é\n\\[[A]]<!-- [[A]] -->[[A]]```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 89", body: "[[A\\]]ref[^n]\n![[A]]```open\n[[A]]\nclose`![[A]]## [[A|alias]]\n\n  - %%```` go [[A]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n## A\n## A\n> [!unknown] title\n > ![[image.png]] ^a\nÉ\n ^a\n[[A#^a]]`[[A]]` ^a-2\n%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 90", body: "1. \t   ```\n## A\n## A\n[[A]] [[A]]É\n    É\n## !\n ^é\n# A\n`\\[[A]]\\`[[A\\]] ## A-2\n## A\n## A\n[[A#^a]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 91", body: "%%[[A]]%%[[A\\]]````\nÉ\n1. ## A-2\n## A\n## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n![[A]]\\[[A]]~~~~\n## A\n## A\n![[A#A]]> [!unknown] title\n[[A\\]]", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 92", body: "\n\n[[A\\]]A\n---\n章節\n-->## !\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 93", body: " `[[A]]`[[#A]]- [[A\\]]> [!unknown] title\nB\n0<!--\n[[A]]\n-->É\n`[[A]]`\n0   ```\n\\````\n%%[[A]]%%> <!--[[#A]]- [ ] [[A]]\nA\nref[^n]\n> [!note] title\nA\ntext [[A#A]]<!--\n[[A]]\n-->[[A#^a]]É\n ^é\n0", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 94", body: "0- [x] [[B]]\n- [x] [[B]]\n1. É\n[[A\\]][^n]: [[A]]\n\n    [[B]]\n ^a\n- > [!note] title\n[[A#A]][[#A]]<!-- [[A]] --> ```\n- [x] [[B]]\n\\[[A]][[B|alias]]``` [[A]]\n\t```\n ^é\n[[B|alias]]> [!unknown] title\n## [[A|alias]]\n\\[[A]]\\\\[[A]]```` go [[A]]\n## A\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 95", body: "## A\n## A\n[[A#^a]]B\n`[[A]]`> [[A\\]]É\n[[A]][[A]] [[A]]- [x] [[B]]\n## A-2\n## A\n## A\n章節\n\n\n> [!note] title\n# A\n ^a-2\n ^a-2\nref[^n]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 96", body: "## A\n## [[A|alias]]\nÉ\nA\n===\n## <em>A</em>\n ^é\n章節\n[[A\\]]ref[^n]\n- [x] [[B]]\nref[^n]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 97", body: "![[A]]https://example.invalid/`[[A]]` ``` [[A]]\n<!--\n[[A]]\n-->[[A\\]]  ```\n[[A#^a]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 98", body: "> [!note] title\n\\\\[[A]]É\nA\n-->  > [!note] title\n\n\n## A\n## <em>A</em>\nref[^n]\n[[#A]]## [[A|alias]]\n[[A#^a]][[A\\]]| a | b |\n|---|---|\n| [[A]] | ^a |\n\\\\[[A]]%%[[A]]%%![[A]]- [ ] [[A]]\n> > ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 99", body: "<!-- [[A]] -->![[A#A]]![[image.png]]## [[A|alias]]\n    ```\n\n\n\\`\t```\n![[image.png]][[#A]] > A\n===\n- - ## [[A|alias]]\n^absent-prose ````\n[[A\\]]%%[[A#^a]]A\n===\n0<div>\n[[A]]\n</div>\n ^é\n`[[A]]`> <!-- [[A]] -->`[[A]]`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 100", body: "  - [[A\\]]## A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n    \\\\[[A]] ```\n[[A#A]]https://example.invalid/`[[A]]` ``` [[A]]\n>     \n\n| a | b |\n|---|---|\n| [[A]] | ^a |\n  - [[B|alias]]```` go [[A]]\n%%^absent-prose ^absent-prose [[A\\]]`[[A]]`^absent-prose ``` [[A]]\n ^é\n![[A#A]]%%[[A]]%%    ```\n## A\n## A\n[[A#^a]]", want: map[string]int{"page-only/A\\": 1}},
		{name: "full original raw namespace 101", body: "~~~\n## !\n````\n# A\n[[A]]   ```\nA\n---\n~~~~\n  > [!note] title\n[[A#^a]]\\`## !\n%%[[A]]%%\n## A-2\n## A\n## A\n    ```\n ^A\n\n\n[[A\\]][^unused]: [[A]]\n> [!unknown] title\n0A\n\\[[A]]É\n[[A#^a]]- item\n\n      text - ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 102", body: "[[A\\]]- item\n\n       ^é\n![[A]][[A\\]]\t```\n ```\n[[A#A]]A\n===\n\\\\[[A]]> [!unknown] title\n<!--[[A]] [[A]]1. text  ```\n^absent-prose [[B|alias]]https://example.invalid/`[[A]]` [[A\\|alias]] ^A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n ^a-2\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 103", body: "[[A\\]]    ```\n\t```\n-->```\n- > [!note] title\n\t```\nÉ\n> [!unknown] title\n<!--## !\n```\n[[A\\]]章節\n[[image.png]]0A\nA\n===\n![[image.png]]\\` ^a\n~~~\nA\n- [x] [[B]]\n## [[A|alias]]\n   ```\nA\n===\nA\n- [x] [[B]]\n ^é\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 104", body: "[[A#A]]text %%[[A]]%%\n\n%%[[A]]%%[[B|alias]][[A\\]]``` [[A]]\n[[A]] [[A]]\n\n  ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 105", body: " > > \\[[A]]text ```\n> [!note] one\n> [!note] two\n> [!note] three\n> [!note] [[A]]\n    ```\n%%[[A]]%%É\n![[image.png]]A\n\t``` [[A]]\n## !\n[[A\\]]--> ````\n    https://example.invalid/`[[A]]` - item\n\n      章節\n章節\n    ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 106", body: "[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\n## [[A|alias]]\n> [!unknown] title\n[[A\\]]ref[^n]\n[[B|alias]]## !\nÉ\n[[A#A]]> [!note] title\n> <!-- [[A]] -->[[A\\]]```` go [[A]]\n`[[A#A]]A\n===\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A-2\n## A\n## A\n ^é\n## !\n## A\n## A\n ^é\n[[A#A]]- > [!note] title\n^absent-prose ``[[A]]`- > [!note] title\n\n\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 107", body: "\\[[A]]> [!note] [[A]]\nB\n[[A\\|alias]][[A\\]]> [!unknown] title\n  ```\n ^é\n- [x] [[B]]\n## A\n## A\n[[#A]]https://example.invalid/`[[A]]` [[image.png]]# A\n-->[[A\nB]]  > [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 108", body: "\\[[A]]<!-- [[A]] -->- [ ] [[A]]\nÉ\n[[A\\]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 109", body: "> > 1. 0> [!note] [[A]]\n## [[A|alias]]\n[[B|alias]]0[[A\\]] ^é\n%%[[#A]]## !\n[[A#A]]A\n## <em>A</em>\n> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]]0-->- > [!note] title\n- item\n\n      ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 110", body: "\\[[A]]- [x] [[B]]\n[[A\\]]## !\n`[[A]]`- > [!note] title\n> [!note] title\n[[A]] [[A]]0[[A#^a]]## A\n## !\n^absent-prose - [x] [[B]]\n     ```\n1. ## [[A|alias]]\n ^é\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 111", body: "[[A]] [[A]][^unused]: [[A]]\n``> > ^absent-prose > <div>\n[[A]]\n</div>\n- item\n\n      - item\n\n      \n\nÉ\nB\ntext     [[A\\]]<!--\n[[A]]\n-->[[A#^a]]https://example.invalid/`[[A]]` ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 112", body: "[[A\\]]\\`[^n]: [[A]]\n\n    [[B]]\n> [!unknown] title\n[[A\\|alias]] text ~~~\n ^é\n%%[[A]]%%章節\n ^é\n[[A\\]]", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 113", body: "- [ ] [[A]]\nref[^n]\n[[A#A]]%%[[A]]%%[[A\\]] ^é\ntext > > <!-- [[A]] -->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 114", body: "[[A]] [[A]][[A\\]][[A]] [[A]] \t[[B|alias]][^unused]: [[A]]\n[[A\\|alias]]> [!note] one\n> [!note] two\n> [!note] three\n  ```\n[[A]] [[A]]``` [[A]]\n[[B|alias]] ^a-2\n[[A\nB]]```` go [[A]]\n0%%[[A]]%%- [x] [[B]]\n[[#A]][^n]: [[A]]\n\n    [[B]]\n~~~\n- item\n\n      ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 115", body: "A\n[[A\\]]  > [!note] title\n ^é\n\\[[A]]- item\n\n      > [!note] title\n[[image.png]]<!-- [[A]] -->`A\n---\n ```\n```` go [[A]]\n![[image.png]]`É\nÉ\n  - ## A\n## A\n0%%[[A]]%%- > [!note] title\n1. 0# A\nref[^n]\n[[A\\|alias]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 116", body: "![[A#A]][[A]]> [!note] [[A]]\nÉ\n[[A\\]]![[A]]- [x] [[B]]\n![[image.png]][[A]]\t```\n\n\n[[B|alias]][[A\\]]\\`[^unused]: [[A]]\n``<!--\n[[A]]\n--> ^é\n>   - ~~~\n0", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 117", body: "[[A\\]]## A-2\n## A\n## A\n````\n[[A\\|alias]][[A#A]][[image.png]] ^A\n\n<!-- [[A]] -->É\n ^A\n-->[^n]: [[A]]\n\n    [[B]]\n[[A]][[A\\|alias]]`# A\n\\\\[[A]]<!--https://example.invalid/`[[A]]` ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 118", body: "\t    ```\n0\n\n<!--\n[[A]]\n-->- [x] [[B]]\n![[A#A]]> [!note] title\n![[A#A]][[A\\]]```\n\n```\n~~~\n0A\n===\n%%~~~~\n~~~~\n[^unused]: [[A]]\n\\`![[A]]> [!note] one\n> [!note] two\n> [!note] three\n# A\n[[image.png]]> [!unknown] title\n\\[[A]]<!--\n[[A]]\n--> ^é\n^absent-prose  ^a-2\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 119", body: "[[A\\]][[A]][[#A]][[B|alias]]É\n ^A\n- item\n\n      [^n]: [[A]]\n\n    [[B]]\nÉ\n[[A\\]] \\`\n\n0", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 120", body: "- > [!note] title\n[[A\\]][^n]: [[A]]\n\n    [[B]]\n\n\nB\n\t```\n[[image.png]]~~~~\n[[A#^a]] ^a-2\n<!--- item\n\n      ref[^n]\n ^a\n%%# A\n[[A#^a]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 121", body: " ^a-2\n[[A\\]]    ```\n![[A]]https://example.invalid/`[[A]]` \n\n--># A\n-->[[image.png]]\n\n    ```\n    > [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 122", body: "![[image.png]]%%[[A]]%%\\\\[[A]]~~~~\n\t``\n\n\\\\[[A]][[A#^a]]```\n[[B|alias]]章節\nA\n---\n[[A\\]]``[[A]] [[A]]  > [!note] title\ntext > [!note] title\n<!-- [[A]] -->| a | b |\n|---|---|\n| [[A]] | ^a |\n## A\n ^é\nÉ\n- - item\n\n       ^a-2\n## <em>A</em>\n  ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 123", body: "  > [!note] title\n-->~~~\n%%[[A]]%%<!--\n[[A]]\n-->``[[A\\]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 124", body: "> >   > [!note] title\n0- > >  ref[^n]\n[[A\\]][[A]] [[A]]    ```\n## A-2\n## A\n## A\n1. \\[[A]][[#A]][[image.png]]``` [[A]]\n ^a\n<!--\n[[A]]\n-->É\n0- [ ] [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 125", body: "[[A\\]]<!--\n[[A]]\n--> ^é\n^absent-prose A\n-->[[A\\|alias]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 126", body: "[[A\\|alias]][[A\\]]<!--\n[[A]]\n-->章節\n\\````\n> [!unknown] title\n1. - > [!note] title\nref[^n]\n```\n<!--\n[[A]]\n-->  > [!note] title\n[[A\\]]## A\n## A\n章節\n# A\n\n\n    ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 127", body: "A\n> \n\n\\`- [ ] [[A]]\nA\n1. <!--`[[A]]` ^a\n[[B|alias]]- - > [!note] title\n`[[A]]`- item\n\n      0 ^é\n- \\\\[[A]]0ref[^n]\n ![[A#A]]  - ~~~~\n`[^n]: [[A]]\n\n    [[B]]\n[[A#A]][[A\\]]\n\n\\[[A]]\\[[A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 128", body: "[[A\\]][[A\\]]<!--\n[[A]]\n-->![[A]]![[image.png]] ^é\n# A\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 129", body: "![[A]]https://example.invalid/`[[A]]` ## A\n## A\n<!-- [[A]] -->[[A\\]]    [[A]]<!--> [!note] one\n> [!note] two\n> [!note] three\n![[A#A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 130", body: "- [ ] [[A]]\n[[A\\|alias]][[A#A]]> [[A\\]]    > [!note] title\n[[#A]][[A\\]][[A#A]][[A#A]]\\[[A]]%%[[A]]%%\nÉ\n[[A#^a]]- [x] [[B]]\n    ```\n\t```\n> > - [ ] [[A]]\n<!-- [[A]] -->- É\n# A\nÉ\n\n   ```\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 131", body: "<!-- [[A]] -->> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]\t```\n[[A\\]]https://example.invalid/`[[A]]` - item\n\n      ![[A]]https://example.invalid/`[[A]]` B\n## !\n~~~~\n\n\n-->- item\n\n      ![[A#A]]# A\n[[A\\]]^absent-prose ## A\n## A\n- [ ] [[A]]\n[[A\\]][[A]]    [[A#^a]]0![[A#A]]`[[A]]`", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 132", body: "```\n ^A\n<!--\n[[A]]\n-->A\n===\n- > [!note] title\n ## A\n## A\n%%> ````\nB\n```\n[[A\\]]   ```\n## <em>A</em>\n[[A]]````\n`[[A]]`````\n> > - > [!note] title\nÉ\n  ```\n[[B|alias]][[B|alias]][[A#A]]É\n章節\n> [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 133", body: "B\n[[A\\]]A\n===\n- > [!note] title\n` ^a\n[[A#A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 134", body: "[[A]]ref[^n]\n[[A\\]]\t```\n ```\n ^A\n## A\n[[#A]]`> [!note] [[A]]\n> > ^absent-prose \t## A-2\n## A\n## A\n<!--\n[[A]]\n-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 135", body: "^absent-prose [[A#^a]]```\n## A\n## A\n- [[A\\]]![[A]]````\n ^a\n- [x] [[B]]\n> [!unknown] title\n- [x] [[B]]\n[[#A]]   ```\n- - [ ] [[A]]\nB\n> > ## A\n## A\n````\n0[^unused]: [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 136", body: "- item\n\n      [[A]] [[A]][[#A]] text > <!--![[A]]ref[^n]\n## A\n\\`\t text [[A\\]]<!-- [[A]] --> ^a-2\n ```\n`[[A]]`", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 137", body: "\\`-  ^A\n[[A\\]]ref[^n]\n\n[[#A]]``` [[A]]\n%%> [!unknown] title\n ^a\n~~~~\n## A\n## A\n`open\n[[A]]\nclose`A\n---\n   ```\nref[^n]\n[[B|alias]]## !\n ^A\n````\n ```\n````\nÉ\n[[B|alias]]0", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 138", body: "É\n## A\n ^a\ntext <!--\n[[A]]\n-->[[A]]  - \n\n`[[A]]`B\n ^a-2\n> [!note] title\n- [x] [[B]]\nÉ\n## A-2\n## A\n## A\n## !\n[[A#^a]]A\n===\n[[A\\]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 139", body: "``open\n[[A]]\nclose`> [!note] title\n章節\n[[A#A]]É\n[[A\\]]## A-2\n## A\n## A\n[[A]] [[A]]> ![[A]]https://example.invalid/`[[A]]` 0```` go [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 140", body: "[[A]]``> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n<!--\n[[A]]\n-->[[A\\]]<!--\n[[A]]\n--><!--<!--  ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 141", body: "> [!note] title\n  - [[A]] [[A]] [[A\\]]\n\t```\n\n\n^absent-prose    ```\n~~~\n  > [!note] title\n## [[A|alias]]\n%%   ```\n~~~\n![[A]]> [!note] one\n> [!note] two\n> [!note] three\n> [!unknown] title\nÉ\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 142", body: "[[B|alias]]    A\n---\n![[image.png]][^n]: [[A]]\n\n    [[B]]\n`[[A]]`~~~~\n[[A\\]]^absent-prose ````\n[[A#^a]]## A-2\n## A\n## A\n%%[[A]]%%   ```\n ^é\n ^é\n\n\n-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 143", body: "[[A#^a]][[A\\]]# A\n1. ![[image.png]][[#A]] ^a-2\n\\[[A]][[image.png]]A\n===\n    [[A\\]]> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]]É\nB\n  ```\n\\[[A]][[A]] [[A]][[#A]]\n\n\n> É\n[[A#A]]> > ## <em>A</em>\n[[B|alias]]``` [[A]]\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 144", body: "# A\n## [[A|alias]]\n## A-2\n## A\n## A\n[[A\\]] > `[[A]]`B\n[[A\\|alias]]````\n\t```\n\\\\[[A]][^n]: [[A]]\n\n    [[B]]\n<div>\n[[A]]\n</div>\n    B\n0A\n===\n\t```\n    ```\n%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 145", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A#A]][[A#A]]\n\t[[A\\]] ```\n-->[[B|alias]]![[A#A]]```` go [[A]]\n0- [^unused]: [[A]]\n[[A]][[A\\]]-->\n## A\n## A\n## <em>A</em>\n  > [!note] title\n![[A#A]]`[[A]]`## A-2\n## A\n## A\n章節\nÉ\nB\n[^unused]: [[A]]\n ^A\n````\n- <div>\n[[A]]\n</div>\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 146", body: "É\n## A-2\n## A\n## A\n[[A\\]][[B|alias]]B\n## A-2\n## A\n## A\n   ```\nref[^n]\n1. | a | b |\n|---|---|\n| [[A]] | ^a |\n- [x] [[B]]\n\\`\\`[[A#A]]    \t `[[A]]`[[A]][^unused]: [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 147", body: "\t````\n[[image.png]]```[[A]]`[[A\\]]章節\nÉ\n## <em>A</em>\n\t1. [[A\\]]## A\n## A\n> [!unknown] title\n章節\nA\n---\n- > [!note] title\n ```\n\n\nA\n> %%0> [!note] [[A]]\n0A\n## A-2\n## A\n## A\n> > ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 148", body: "    ```\n\\`[[A\\]]> > > [!note] one\n> [!note] two\n> [!note] three\n  ```\n<div>\n[[A]]\n</div>\n^absent-prose \\[[A]]É\nÉ\n1. [[A]] [[A]]ref[^n]\n-   ```\n[[image.png]]A\n===\n[[B|alias]]https://example.invalid/`[[A]]` # A\n[[A\\]] ^a-2\n^absent-prose [^unused]: [[A]]\n`[^unused]: [[A]]\n<!--<!--\n[[A]]\n-->%%[[A]]%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 149", body: "- [[A]][[#A]][[A]]## !\n## !\n    ```\n ^a-2\n\t- > [!note] title\n- ![[A#A]]\t^absent-prose \n\n[[A\\]]0<div>\n[[A]]\n</div>\n[^n]: [[A]]\n\n    [[B]]\n  ```\n- > [!note] title\n\\[[A]]>  ^a-2\n<!-- [[A]] -->- > [!note] title\n<!--[[A]]~~~\n`[[A]]`[[A\\]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 150", body: "- > [!note] title\n\\` ^A\n%%[[A]]%%章節\n- 0  > [!note] title\nref[^n]\n\n\n[[A#A]]> >      ^é\n[[A\\]]## [[A|alias]]\n[[B|alias]]~~~~\n```\n# A\n ^a\nB\n```` go [[A]]\n章節\n- item\n\n      É\n<!-- [[A]] -->``` [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n- item\n\n      ```\n[[#A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 151", body: "## A\n- ``> [!note] [[A]]\nÉ\n\n\n## [[A|alias]]\n ^a-2\n[[image.png]]\n# A\n\\`[[A]] [[A]]## A\n\\\\[[A]][[#A]]A\n# A\n ^a-2\n[[A\\]]- > [!note] title\n[[#A]]\n## A-2\n## A\n## A\n[[A#^a]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 152", body: "> > ^absent-prose > \n\n## [[A|alias]]\n[^n]: [[A]]\n\n    [[B]]\n- > [!note] title\n![[A#A]]```` go [[A]]\n ^é\n> [!note] one\n> [!note] two\n> [!note] three\n- item\n\n      É\n[[A]]A\n---\n[[A\\]]   ```\n## !\nref[^n]\n\n``- [ ] [[A]]\n<!-- [[A]] -->0", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 153", body: "## A-2\n## A\n## A\n    [^n]: [[A]]\n\n    [[B]]\n1.   - - item\n\n      [^n]: [[A]]\n\n    [[B]]\n# A\nref[^n]\n[[A\\]]É\n\t```\n- > [!note] title\nÉ\n[[image.png]]> > É\n<div>\n[[A]]\n</div>\n````\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 154", body: "![[A]]`[[A]]`[[B|alias]]![[image.png]]## <em>A</em>\n[[A\\]] ^é\n- item\n\n      ```` go [[A]]\n[[#A]][[B|alias]]> ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 155", body: "> [!note] one\n> [!note] two\n> [!note] three\n ^a-2\n[[A\\]]ref[^n]\n[[A]][^n]: [[A]]\n\n    [[B]]\n  ```\n\\`[[A\\]][[A\nB]]`[[A]]` ^a\n`[[A]]`![[image.png]]> [!note] [[A]]\n- [ ] [[A]]\n> ## A-2\n## A\n## A\n- [x] [[B]]\nB\n章節\nA\n---\n![[A]]~~~~\n%%## A\n## A\n[[A\nB]]  ```\n`## !\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 156", body: "<!--<!-- [[A]] -->[[A\\|alias]][[A\\]]É\n# A\n\n\t- > [!note] title\n> [!unknown] title\nÉ\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 157", body: "[[A\\]]`[[A]]````\n~~~~\nÉ\n%%`[[A]]````\n%%[[A]]%%    ```\n\t```\n```` go [[A]]\n> [!unknown] title\n<!--B\n[^unused]: [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 158", body: "  ```\n%%- > [!note] title\n## A-2\n## A\n## A\n%%``0A\n ```\n   ```\n## !\n- [x] [[B]]\n[[A\\]]```\nB\n  ```\n[[A#A]][[A\\]]\\`[[A#^a]]- \n\n\n\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 159", body: "[[A\\]]%%  ```\n[[A\\]]https://example.invalid/`[[A]]`   > [!note] title\n# A\n> [!note] title\n## <em>A</em>\n[[image.png]]  > [!note] title\n%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 160", body: "> > ## [[A|alias]]\n[[A#A]][[A]]É\n ^é\n[[A#^a]] ^é\n[[A\\]]    <!-- [[A]] -->[[B|alias]]> [!note] one\n> [!note] two\n> [!note] three\n  ```\n[[image.png]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 161", body: "[[A\\]]- [ ] [[A]]\n%%[[A#^a]]0\t```\nA\n===\n\n\n## A-2\n## A\n## A\n![[A#A]]\\\\[[A]]![[A]] ^é\n\\[[A]]B\n\n ^A\n[[A\\]]\n\n> [[A#^a]]> [!unknown] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 162", body: "## A-2\n## A\n## A\n[[A]] [[A]][[#A]] ```\n[[A\\]]A\n===\n\t    ```\n## A\n\n\t- item\n\n      - [ ] [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 163", body: "text | a | b |\n|---|---|\n| [[A]] | ^a |\n[[A\\]]## A-2\n## A\n## A\n0    ```\n- [x] [[B]]\n ^é\n[[A]] [[A]] ^a\n\\[[A]]- > [!note] title\n[[A#A]]É\n%%%%  ```\n```\n ^é\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 164", body: "> [!note] title\n[[A]] [[A]]- > [!note] title\n[[A\\]][^unused]: [[A]]\n\n\n%% ^a-2\n> > [!note] [[A]]\n[[A#^a]]<!--[^unused]: [[A]]\n## !\n> > ## <em>A</em>\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 165", body: "<!--\n[[A]]\n-->| a | b |\n|---|---|\n| [[A]] | ^a |\n00## !\n## A\n- [ ] [[A]]\n%%[[A]]%%A\n---\n## <em>A</em>\n章節\n ^a-2\n[[A]] [[A]][[B|alias]]## !\n\t[[A\\]] ^é\n    ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 166", body: "\\`[[A\\]]``` [[A]]\n ^é\n        ![[A#A]] ^a\n```\n``- > [!note] title\n- [x] [[B]]\n\t```\n  - > > É\n`[[A]]``open\n[[A]]\nclose`\\[[A]] ^é\n%%- item\n\n      ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 167", body: "![[A]]\\\\[[A]][[A\\]]![[image.png]]A\n---\n## A\n## A\n<!--\n[[A]]\n-->   ```\n> [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 168", body: "[[A\\]]^absent-prose    ```\n   ```\n[[A#^a]]01.  ```\ntext [[A#^a]]%%^absent-prose A\nB\n\n^absent-prose B\n![[image.png]]- [x] [[B]]\nA\n[[A]]%%[[A]]%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 169", body: "\\`> > <!--\n[[A]]\n-->^absent-prose [[A\\|alias]] ^a-2\n ^A\n\\`[[A\\]]\t```\n    <!-- [[A]] --># A\nref[^n]\n\n0[^unused]: [[A]]\n``B\n ^é\n> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 170", body: "[[A\\]]A\nhttps://example.invalid/`[[A]]` B\n ^é\n> [!note] one\n> [!note] two\n> [!note] three\n\\`[[A#^a]]章節\n1. - [x] [[B]]\n## A-2\n## A\n## A\n%%[[A]]%%\t```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 171", body: "<!--\n[[A]]\n-->[[A\\]]  > [!note] title\n\n > > A\n\\`0> [!unknown] title\n\n<!--^absent-prose ![[A#A]]  > [!note] title\nB\n[[A\\|alias]]<div>\n[[A]]\n</div>\n[[A#^a]]^absent-prose 0`open\n[[A]]\nclose``open\n[[A]]\nclose`````\n%%-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 172", body: "[[A\\]]~~~~\n\n\nA\n===\n0[[A#^a]]![[A#A]]\t```\n  ```\n  ```\n<!--\n[[A]]\n-->> > %%\\`   ```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 173", body: " ^é\n    ```\n ^é\n    - [x] [[B]]\n%%[[A]]%%## A-2\n## A\n## A\n ^a-2\n[[A\\]][[B|alias]]<!--\n[[A]]\n-->", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 174", body: "[[A\\]]`[[A]]` - [x] [[B]]\n[[A\\]]<div>\n[[A]]\n</div>\nA\n---\n<!--\n[[A]]\n-->0", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 175", body: "![[image.png]][^n]: [[A]]\n\n    [[B]]\n[[A\\]]%%> [!note] title\n## [[A|alias]]\n<div>\n[[A]]\n</div>\n[[A]]> [!unknown] title\n^absent-prose ", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 176", body: "[[A\\]][[#A]]> >  ^é\n`[[A]]`  - ``A\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n[[A#^a]][[A#A]]\\[[A]]> [!note] title\n[[A\\|alias]]## A\n## A\nA\n===\n~~~\n[[A]] [[A]]ref[^n]\nB\n%%[[#A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 177", body: "[[A\\]]\t- item\n\n      - ## !\n[[A]] [[A]]<!--A\n---\n``` [[A]]\n0> [!note] title\n- item\n\n      ![[image.png]]0`  - \\`\t ^é\n## A\n## A\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 178", body: "`[[A]]`<div>\n[[A]]\n</div>\n1. ## A\n[[A\\]]B\n## A\n## A\n`> [!note] [[A]]\n\t```\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 179", body: "> > >  ^é\n\n\n[[A\\]]  > [!note] title\nÉ\n<!-- ```\n ^A\n ## A-2\n## A\n## A\nhttps://example.invalid/`[[A]]` ``https://example.invalid/`[[A]]` -->\\`<!--\n[[A]]\n-->````\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 180", body: "0`[[A]]`[[A\\]]É\n[[A#A]]- > [!note] title\n````\n```` go [[A]]\n\\\\[[A]][[A#^a]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 181", body: "%%[[A]]%%\n\n[[A\\]]<div>\n[[A]]\n</div>\n``[[A]] [[A]]- [x] [[B]]\nA\n---\n- item\n\n      ^absent-prose \t[^unused]: [[A]]\n# A\n> [!unknown] title\n1. ![[A#A]]![[A]]B\n[[A\\]] ^A\n## !\n[[A]] [[A]][[A#A]]<!--## A-2\n## A\n## A\n ```\n\\[[A]]-   ```\nA\n---\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 182", body: "## [[A|alias]]\n[[A\\|alias]]- item\n\n       ```\n[[A\\]][[#A]][[A\\]]  > [!note] title\n## [[A|alias]]\n[[A]] [[A]] ^a-2\n-->> [!note] one\n> [!note] two\n> [!note] three\n- [x] [[B]]\n[[A#A]][[A\\]]> [!note] one\n> [!note] two\n> [!note] three\n - [x] [[B]]\n[[A\\]]> [!note] title\n[[A#A]]> ![[image.png]]", want: map[string]int{"judge-only/A": 4, "page-only/A\\": 4}},
		{name: "full original raw namespace 183", body: "## A-2\n## A\n## A\nA\n---\ntext É\n  - [[A\\]]É\n    [[A#^a]]## [[A|alias]]\nref[^n]\n## <em>A</em>\n  - ~~~~\n\n\n[[image.png]]```\n\n\n ```\n\n\n![[A#A]][[A\\]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 184", body: "[[A\\]]ref[^n]\ntext ```\n  ```\n``` [[A]]\n[[A#^a]] ## !\n^absent-prose [[A\\|alias]][[A#^a]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 185", body: "![[A#A]][[image.png]] ^é\n![[A]]- [ ] [[A]]\nÉ\n[[image.png]]## A-2\n## A\n## A\n[[A#A]]A\n===\n\\`\n[[#A]]~~~\nB\n- item\n\n      ~~~~\n-->[[A\\|alias]] ^A\n[[A]] [[A]]章節\n\\[[A]]- [x] [[B]]\n  > [!note] title\n[[A\\]]1. ```\n[[A\\|alias]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 186", body: "\t`[[A]]`[[A#^a]]É\nA\n===\n\n\n%%[[A]]%%## A\n## A\n## <em>A</em>\n0 ^é\n[[A\\|alias]]> [[A\\]] ^a\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 187", body: "`## A\n## A\n[[B|alias]]^absent-prose `[[A]]`    ```\n\t```\n ^é\n [[A\\]]`[[A]]````` go [[A]]\n章節\n[[A\\]]      > [!note] title\n> > \\`\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 188", body: "\n\\`![[image.png]]  -  ^a\n## A\n## A\n ^é\n0![[A]]B\n> [!note] title\n    ```\n## A-2\n## A\n## A\n[[A]]   ```\n ^é\n## A\n## A\n[[A\\]] %%[[A]]%%   ```\nA\n ^a-2\n    ```\nA\n---\n![[image.png]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 189", body: " ^A\n- [[A\\]]```\n## A-2\n## A\n## A\n> > [[A]] [[A]]\t```\nA\n---\n> [!note] one\n> [!note] two\n> [!note] three\n![[A]][[A#A]][[A#^a]]A\n===\n[[A\\|alias]]~~~~\n ^a\n<!-- ^é\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 190", body: "A\n<!--^absent-prose \\\\[[A]] ^A\n> [!unknown] title\n ```\n ```\n- > [!note] title\n[[B|alias]]0B\n[^n]: [[A]]\n\n    [[B]]\n-->[[A\\]]É\n ^é\n[[A]] [[A]][[A\\]] ``  ```\n<div>\n[[A]]\n</div>\n![[A]]## A\n`open\n[[A]]\nclose`A\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 191", body: "0  > [!note] title\n> [!note] title\n[[A]]A\n===\n[[A\\]]- > [!note] title\n  ```\n> [!note] title\n%%[[A]]%% ^a\n[[image.png]]https://example.invalid/`[[A]]` https://example.invalid/`[[A]]` [^n]: [[A]]\n\n    [[B]]\n# A\n[[A\\]][[#A]]  > [!note] title\n- > [!note] title\n^absent-prose [[image.png]]## A-2\n## A\n## A\n> [!note] title\n ^A\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 192", body: "-->![[A#A]]\n\n ^a-2\n<!-- [[A]] -->[[A\\]][[A#^a]] ^é\n ^é\n![[A#A]]B\n- [ ] [[A]]\n ^A\n ^a\n# A\n\\\\[[A]]```` go [[A]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 193", body: "[[A\\]]> [!note] title\n%%> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]# A\n``` [[A]]\nA\n===\n> [!unknown] title\n- \\` > > ````\n ^A\n<!--\n[[A]]\n-->A\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A\\]][[A#^a]]```\n![[A]]A\n- item\n\n      ![[image.png]][^unused]: [[A]]\n~~~~\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 194", body: "## <em>A</em>\n\\[[A]]<!-- [[A]] -->   ```\n ^é\n[[A#^a]]| a | b |\n|---|---|\n| [[A]] | ^a |\n> `\\`   ```\n1.     ref[^n]\n## !\n0\t[[A]][[A\\]]`## <em>A</em>\n~~~~\n<!--\n[[A]]\n-->B\n ~~~\nÉ\n  > [!note] title\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 195", body: "- [x] [[B]]\n\\`| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n  - > [!note] [[A]]\n[[A\\]] ^A\nA\n- [ ] [[A]]\n```\n- > [!note] title\n%%<!--\n[[A]]\n-->章節\n## A\n[[A]]![[A#A]]````\n章節\n ^a\n\t ^a-2\n[[A]] [[A]]\nB\n| a | b |\n|---|---|\n| [[A]] | ^a |\n ^A\nA\n---\n```\n%%[[A]]%%", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 196", body: "章節\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]## A\n![[A#A]]`[[A]]`[[A]]## [[A|alias]]\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 197", body: "`  > [!note] title\n## A-2\n## A\n## A\n![[A#A]][^n]: [[A]]\n\n    [[B]]\n## <em>A</em>\n```\n ^A\n> [!note] one\n> [!note] two\n> [!note] three\n ^a-2\nA\n===\n````\n> [!note] title\n[[A]] [[A]]![[A]] ^A\n\\[[A]] ^A\n## A\n## A\n<!-- [[A]] -->0``` [[A]]\n[[A\\]][[A#^a]]1. ``\n\n ^é\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 198", body: "## A-2\n## A\n## A\n\\[[A]]<!--\n[[A]]\n-->[[image.png]]A\n===\n<!-- [[A]] -->\\\\[[A]][^unused]: [[A]]\n- [x] [[B]]\n[[A\\]]\\[[A]]```\n\n\n> [!note] one\n> [!note] two\n> [!note] three\n[[B|alias]]É\n0![[image.png]]## <em>A</em>\n-->~~~~\n ^a-2\n> [!unknown] title\n[[A\\]]B\n\t[[B|alias]]# A\n[[A]]> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 199", body: "   ```\n``[[A\\]]B\n[[image.png]]ref[^n]\n[[A\nB]]\t```\n[[A\\|alias]]0~~~\n\n\n[[A#^a]] ^A\n  - `![[image.png]] ^A\n## A-2\n## A\n## A\n ^A\n````\n[[A\\]] ^a\n## !\n1. É\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 200", body: "[[A\\]]## A\nA\n---\n> >   > [!note] title\n  > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[A]] [[A]]^absent-prose - item\n\n          - [[A#^a]]  - ``` [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n## !\n> ^absent-prose   - > [!unknown] title\n# A\n ^é\n<!--\n[[A]]\n-->[[A\\|alias]]0~~~~\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 201", body: "> [!note] title\n\\\\[[A]]![[A]]É\n^absent-prose 1. 章節\n[[A\\]][[A#A]]> > # A\n[[A]] [[A]]## !\n ^é\n## <em>A</em>\n```\n[[A]]", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 202", body: "## A-2\n## A\n## A\nB\n[[A\\]]\\[[A]]1. | a | b |\n|---|---|\n| [[A]] | ^a |\n1.     ```\n> [!unknown] title\n```\n[[image.png]]  - <!-- [[A]] -->   ```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n~~~\n ^A\n- %%  -   ```\n```` go [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n ```\n[[A\\]]B\n- [x] [[B]]\n[[A#^a]]%%[[A#^a]]", want: map[string]int{"judge-only/A": 2, "page-only/A\\": 2}},
		{name: "full original raw namespace 203", body: "[[A\\]]\t```\n~~~\n## !\n- > [!note] title\n https://example.invalid/`[[A]]` [[A]] [[A]]``` [[A]]\n0> [!note] title\n\t```\n-->%%[[A]]%%~~~\n0", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
		{name: "full original raw namespace 204", body: "A\n---\n-->![[A]]A\n===\n- item\n\n      > > https://example.invalid/`[[A]]` [[A#^a]][[B|alias]][[A]]> >  ```\n    `[[A\\]]<!--- > [!note] title\n[[A]]%%[[A]]%%- [ ] [[A]]\n> > É\n ^A\n", want: map[string]int{"judge-only/A": 1, "page-only/A\\": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, a := agreementIsolatedPage(t, c)
			budget := agreementRawNamespaceBudget(c.Body, &a)
			if (len(tc.want) > 0) != (budget.Body == c.Body) {
				t.Fatalf("caught: raw namespace public budget body=%q, want owned=%t", budget.Body, len(tc.want) > 0)
			}
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &a) {
				key := f.Direction + "/" + f.Tuple.Target
				kind, authority, wrong := agreementRawNamespaceDifference(c, &f, &a, &budget)
				if f.Property != "P1" || tc.want[key] == 0 {
					if kind != "" {
						t.Fatal("caught: raw namespace borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 3" || wrong != "page" {
					t.Fatalf("caught: raw namespace public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[key] = f.Multiplicity
				agreementRawNamespaceDrift(t, c, &f, &a, &budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete raw namespace public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
