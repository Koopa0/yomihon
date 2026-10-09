package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

type agreementOutsideCodeSource struct {
	Namespace           agreementRawNamespaceSource
	Normalized, Omitted map[string]int
}

// A field that the check calls code can remain prose on the page. Retain
// every native owner and outside field; count only identical raw and normalized
// names so a spelling disagreement cannot supply this missing occurrence.
func agreementOutsideCodeReading(body string) agreementOutsideCodeSource {
	r := agreementOutsideCodeSource{Namespace: agreementRawNamespaceReading(body), Normalized: make(map[string]int), Omitted: make(map[string]int)}
	// The public check owns an ordered list even when that list is empty.
	r.Namespace.Ledger.Check.Targets = append([]string{}, r.Namespace.Ledger.Check.Targets...)
	for _, field := range r.Namespace.Ledger.Check.Fields {
		if !field.Code && !field.Comment && !field.Escaped {
			for _, name := range field.Targets {
				r.Normalized[name]++
			}
		}
	}
	for i := range r.Namespace.Ledger.Outside {
		field := &r.Namespace.Ledger.Outside[i]
		if field.Source.Code && !field.Source.Comment && !field.Source.Escaped && !field.Comment && field.Cites && len(field.Source.Targets) == 1 && field.Source.Targets[0] == field.Tuple.Target {
			r.Omitted[field.Tuple.Target]++
		}
	}
	return r
}

type agreementOutsideCodePayload struct {
	Body   string
	Source agreementOutsideCodeSource
}

func agreementOutsideCodeBudget(body string, actual *agreementHTML) agreementOutsideCodePayload {
	r := agreementOutsideCodeReading(body)
	whole := make(map[string]int)
	for _, target := range r.Namespace.Ledger.Check.Targets {
		whole[target]++
	}
	if len(r.Omitted) == 0 || !agreementOwnedCheckLiveLedger(body, &r.Namespace.Ledger) || !cmp.Equal(whole, r.Normalized) || !cmp.Equal(r.Namespace.Ledger.Page, agreementNativeLiveCarriers(actual)) || len(actual.CodeCitations) != 0 || actual.CitationsInCode != 0 {
		return agreementOutsideCodePayload{}
	}
	return agreementOutsideCodePayload{Body: body, Source: r}
}

func agreementOutsideCodeDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget *agreementOutsideCodePayload) (kind, authority, wrong string) {
	if c.Body != budget.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "page-only" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	if !cmp.Equal(budget.Source, agreementOutsideCodeReading(c.Body)) || !cmp.Equal(budget.Source.Namespace.Ledger.Page, agreementNativeLiveCarriers(actual)) || len(actual.CodeCitations) != 0 || actual.CitationsInCode != 0 {
		return "", "", ""
	}
	page := 0
	for tuple, count := range budget.Source.Namespace.Ledger.Page {
		if tuple.Target == f.Tuple.Target {
			page += count
		}
	}
	if budget.Source.Omitted[f.Tuple.Target] != f.Multiplicity || page-budget.Source.Normalized[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 stage 4", "judge"
}

func agreementOutsideCodeDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, budget *agreementOutsideCodePayload) {
	t.Helper()
	zero := *f
	zero.Tuple.Target, zero.Multiplicity = "unowned", 0
	if kind, _, _ := agreementOutsideCodeDifference(c, &zero, actual, budget); kind != "" {
		t.Fatal("caught: outside code borrowed zero occurrence")
	}
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementOutsideCodeDifference(other, f, actual, budget); kind != "" {
			t.Fatal("caught: outside code borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){
		func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
		func(f *agreementFailure) { f.Multiplicity = 0 }, func(f *agreementFailure) { f.Multiplicity = -1 },
	} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementOutsideCodeDifference(c, &changed, actual, budget); kind != "" {
			t.Fatalf("caught: outside code borrowed signature %s", agreementSignature(&changed))
		}
	}
	for _, carriers := range [][]agreementCitation{nil, append(append([]agreementCitation{}, actual.Citations...), agreementCitation{Target: "unowned", State: "wikilink-broken"}), {{Target: f.Tuple.Target, Section: "unowned", State: "wikilink-broken"}}} {
		changed := *actual
		changed.Citations = carriers
		if kind, _, _ := agreementOutsideCodeDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: outside code borrowed changed whole page")
		}
		if agreementOutsideCodeBudget(c.Body, &changed).Body != "" {
			t.Fatal("caught: outside code budget borrowed changed whole page")
		}
	}
	for _, change := range []func(*agreementHTML){func(a *agreementHTML) { a.CitationsInCode++ }, func(a *agreementHTML) {
		a.CodeCitations = append(a.CodeCitations, agreementCitation{Target: "unowned", State: "wikilink-broken"})
	}} {
		changed := *actual
		change(&changed)
		if kind, _, _ := agreementOutsideCodeDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: outside code borrowed code occurrence")
		}
		if agreementOutsideCodeBudget(c.Body, &changed).Body != "" {
			t.Fatal("caught: outside code budget borrowed code occurrence")
		}
	}
}

// A spelling change can cancel a code omission at the same normalized name.
// Keep every namespace counter before admitting its signed receipt.
func TestAgreementOutsideCodeCancellation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body string }{
		{name: "outside spelling", body: "[^n]: [[A]]\n\n    [[B]]\nref[^n]\n[[B\\]]\n"},
		{name: "active unused owner", body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n\n[^u]: [[B]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementOutsideCodeBudget(c.Body, &actual)
			if budget.Body != c.Body {
				t.Fatal("caught: outside code cancellation source is unowned")
			}
			f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Direction: "page-only", Tuple: agreementCitation{Target: "B"}, Multiplicity: 1}
			if kind, _, _ := agreementOutsideCodeDifference(c, &f, &actual, &budget); kind != "" {
				t.Fatal("caught: outside code borrowed a cancelled whole namespace difference")
			}
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				if kind, _, _ := agreementOutsideCodeDifference(c, &f, &actual, &budget); kind != "" {
					t.Fatal("caught: outside code borrowed an unowned spelling receipt")
				}
			}
		})
	}
}

func TestAgreementOutsideCodeSourceDrift(t *testing.T) {
	t.Parallel()
	c := agreementCase{Body: "[^n]: [[A]]\n\n    [[B]] [[B]]\nref[^n]\n[[B]] \\[[C]] %%[[D]]%%\n\n```\n[[E]]\n```\n"}
	_, actual := agreementIsolatedPage(t, c)
	budget := agreementOutsideCodeBudget(c.Body, &actual)
	f := agreementFailure{Property: "P1", Identity: "citation-occurrences", Direction: "page-only", Tuple: agreementCitation{Target: "B"}, Multiplicity: 2}
	if kind, authority, wrong := agreementOutsideCodeDifference(c, &f, &actual, &budget); kind != "debt" || authority != "#1011 stage 4" || wrong != "judge" {
		t.Fatal("caught: outside code source drift control is unowned")
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementOutsideCodeSource)
	}{
		{"native code kind", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Native.Code[0].Kind = "unowned" }},
		{"native code span", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Native.Code[0].Span.Stop++ }},
		{"native comment", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Native.Comments[0].Stop++ }},
		{"native definition", func(s *agreementOutsideCodeSource) {
			s.Namespace.Ledger.Native.Definitions = append(s.Namespace.Ledger.Native.Definitions, agreementNativeOwnerDefinition{Label: "unowned"})
		}},
		{"whole target list", func(s *agreementOutsideCodeSource) {
			s.Namespace.Ledger.Check.Targets = append(s.Namespace.Ledger.Check.Targets, "unowned")
		}},
		{"plain code spans", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Check.Code = nil }},
		{"plain comment spans", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Check.Comments = nil }},
		{"field order", func(s *agreementOutsideCodeSource) {
			s.Namespace.Ledger.Check.Fields[0], s.Namespace.Ledger.Check.Fields[1] = s.Namespace.Ledger.Check.Fields[1], s.Namespace.Ledger.Check.Fields[0]
		}},
		{"field word", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Check.Fields[0].Word = "[[unowned]]" }},
		{"field span", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Check.Fields[0].Span.Start++ }},
		{"field targets", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Check.Fields[0].Targets = []string{"unowned"} }},
		{"owned partition", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Owned = nil }},
		{"outside partition", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Outside = s.Namespace.Ledger.Outside[1:] }},
		{"outside order", func(s *agreementOutsideCodeSource) {
			s.Namespace.Ledger.Outside[0], s.Namespace.Ledger.Outside[1] = s.Namespace.Ledger.Outside[1], s.Namespace.Ledger.Outside[0]
		}},
		{"outside code", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Outside[1].Source.Code = false }},
		{"outside comment", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Outside[1].Comment = true }},
		{"outside block", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Outside[1].Block = "unowned" }},
		{"outside tuple", func(s *agreementOutsideCodeSource) { s.Namespace.Ledger.Outside[1].Tuple.Section = "unowned" }},
		{"whole page", func(s *agreementOutsideCodeSource) {
			s.Namespace.Ledger.Page[agreementCitation{Target: "unowned", State: "wikilink-broken"}]++
		}},
		{"normalized namespace", func(s *agreementOutsideCodeSource) { s.Namespace.Normalized["unowned"]++ }},
		{"whole normalized namespace", func(s *agreementOutsideCodeSource) { s.Normalized["unowned"]++ }},
		{"raw pairs", func(s *agreementOutsideCodeSource) { s.Namespace.Raw["unowned"]++ }},
		{"check pairs", func(s *agreementOutsideCodeSource) { s.Namespace.Check["unowned"]++ }},
		{"omitted namespace", func(s *agreementOutsideCodeSource) { s.Omitted["unowned"]++ }},
		{"omitted count", func(s *agreementOutsideCodeSource) { s.Omitted["B"]++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementOutsideCodePayload{Body: c.Body, Source: agreementOutsideCodeReading(c.Body)}
			tc.change(&changed.Source)
			if kind, _, _ := agreementOutsideCodeDifference(c, &f, &actual, &changed); kind != "" {
				t.Fatal("caught: outside code borrowed changed complete source")
			}
		})
	}
}

func TestAgreementOutsideCodeSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementOutsideCodeSource
	}{{name: "used second paragraph", body: "[^n]: [[A]]\n\n    [[B]]\nref[^n]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 23}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}: 1, {SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1}, Omitted: map[string]int{"B": 1}}}, {name: "reference before definition", body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 26, Stop: 32}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 15, Stop: 20}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 26, Stop: 31}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 15, Stop: 20}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 26, Stop: 31}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}: 1, {SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1}, Omitted: map[string]int{"B": 1}}}, {name: "repeated second paragraph", body: "[^n]: [[A]]\n\n    [[B]] [[B]]\nref[^n]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 29}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 23, Stop: 28}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 23, Stop: 28}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}: 1, {SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}: 2}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1}, Omitted: map[string]int{"B": 2}}}, {name: "ordinary occurrence stays separate", body: "[^n]: [[A]]\n\n    [[B]]\nref[^n]\n[[B]]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 23}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 31, Stop: 36}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A", "B"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 31, Stop: 36}, Word: "[[B]]", Targets: []string{"B"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}: 1, {SourceRole: "", Target: "B", Section: "", State: "wikilink-broken"}: 2}}, Normalized: map[string]int{"A": 1, "B": 1}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1, "B": 1}, Omitted: map[string]int{"B": 1}}}, {name: "unused definition", body: "[^n]: [[A]]\n\n    [[B]]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: []agreementNativeOwnerDefinition{{Label: "n", Parent: "Document", Span: graph.Span{Start: 0, Stop: 23}, Name: graph.Span{Start: 0, Stop: 5}, Nodes: []agreementNativeOwnerNode{{Kind: "Footnote", Depth: 0, Lines: nil, Text: nil}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 6, Stop: 11}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 6, Stop: 7}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 7, Stop: 8}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 8, Stop: 10}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 10, Stop: 11}}}, {Kind: "Paragraph", Depth: 1, Lines: []graph.Span{{Start: 17, Stop: 22}}, Text: nil}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 17, Stop: 18}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 18, Stop: 19}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 19, Stop: 21}}}, {Kind: "Text", Depth: 2, Lines: nil, Text: []graph.Span{{Start: 21, Stop: 22}}}}}}, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 23}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}}, Targets: []string{"A"}}, Owned: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: 0, CodeOwner: -1}}, Outside: nil, Page: map[agreementCitation]int{}}, Normalized: map[string]int{}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1}, Omitted: map[string]int{}}}, {name: "native code in used definition", body: "[^n]: [[A]]\n\n        [[B]]\nref[^n]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: []agreementNativeOwnerCode{{Kind: "CodeBlock", Parent: "Footnote", Span: graph.Span{Start: 21, Stop: 27}}}, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 27}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 21, Stop: 26}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Targets: []string{"A"}}, Owned: []agreementNativeCheckField{{Span: graph.Span{Start: 21, Stop: 26}, Word: "[[B]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: 0}}, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1}, Omitted: map[string]int{}}}, {name: "raw suffix has separate ownership", body: "[^n]: [[A]]\n\n    [[B\\]]\nref[^n]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 24}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 23}, Word: "[[B\\]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 17, Stop: 23}, Word: "[[B\\]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B\\", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}: 1, {SourceRole: "", Target: "B\\", Section: "", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1}, Omitted: map[string]int{}}}, {name: "second paragraph section", body: "[^n]: [[A]]\n\n    [[B#place]]\nref[^n]\n", want: agreementOutsideCodeSource{Namespace: agreementRawNamespaceSource{Ledger: agreementOwnedCheckLiveSource{Native: agreementNativeOwnerSource{Definitions: nil, Code: nil, Comments: nil}, Check: agreementNativeCheckSource{Code: []graph.Span{{Start: 17, Stop: 29}}, Comments: nil, Fields: []agreementNativeCheckField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, {Span: graph.Span{Start: 17, Stop: 28}, Word: "[[B#place]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}}, Targets: []string{"A"}}, Owned: nil, Outside: []agreementDestinationLiveField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, Code: false, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 17, Stop: 28}, Word: "[[B#place]]", Targets: []string{"B"}, Code: true, Comment: false, Escaped: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: "", Target: "B", Section: "place", State: "wikilink-broken"}, Block: "", Cites: true, Comment: false}}, Page: map[agreementCitation]int{{SourceRole: "", Target: "A", Section: "", State: "wikilink-broken"}: 1, {SourceRole: "", Target: "B", Section: "place", State: "wikilink-broken"}: 1}}, Normalized: map[string]int{"A": 1}, Raw: map[string]int{}, Check: map[string]int{}}, Normalized: map[string]int{"A": 1}, Omitted: map[string]int{"B": 1}}}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agreementOutsideCodeReading(tc.body)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("caught: complete outside code source (-want +got):\n%s", diff)
			}
			if !agreementOwnedCheckLiveLedger(tc.body, &got.Namespace.Ledger) {
				t.Fatal("caught: outside code lost complete public field ledger")
			}
		})
	}
}

func TestAgreementOutsideCodePublic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{{name: "used second paragraph", body: "[^n]: [[A]]\n\n    [[B]]\nref[^n]\n", want: map[string]int{"page-only/B": 1}}, {name: "reference before definition", body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"page-only/B": 1}}, {name: "repeated second paragraph", body: "[^n]: [[A]]\n\n    [[B]] [[B]]\nref[^n]\n", want: map[string]int{"page-only/B": 2}}, {name: "ordinary occurrence stays separate", body: "[^n]: [[A]]\n\n    [[B]]\nref[^n]\n[[B]]\n", want: map[string]int{"page-only/B": 1}}, {name: "unused definition", body: "[^n]: [[A]]\n\n    [[B]]\n", want: nil}, {name: "native code in used definition", body: "[^n]: [[A]]\n\n        [[B]]\nref[^n]\n", want: nil}, {name: "raw suffix has separate ownership", body: "[^n]: [[A]]\n\n    [[B\\]]\nref[^n]\n", want: nil}, {name: "second paragraph section", body: "[^n]: [[A]]\n\n    [[B#place]]\nref[^n]\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 1", body: "É\n> [!note] one\n> [!note] two\n> [!note] three\nÉ\n^absent-prose ![[image.png]][^n]: [[A]]\n\n    [[B]]\n> > [!note] one\n> [!note] two\n> [!note] three\n- https://example.invalid/`[[A]]` %%\t1. É\n- [x] [[B]]\n-->```` go [[A]]\n\t## A-2\n## A\n## A\n ^é\n````\n[^n]: [[A]]\n\n    [[B]]\n%% ^a-2\n[[A#^a]]0## A\n", want: map[string]int{"page-only/A": 1}}, {name: "full original 2", body: "[^n]: [[A]]\n\n    [[B]]\nref[^n]\nB\n> [!note] one\n> [!note] two\n> [!note] three\n- [ ] [[A]]\n^absent-prose - [x] [[B]]\n````\n0<!--[[A\\|alias]]\n\n| a | b |\n|---|---|\n| [[A]] | ^a |\n  ```\n\\\\[[A]]", want: map[string]int{"page-only/B": 1}}, {name: "full original 3", body: "> [!note] one\n> [!note] two\n> [!note] three\nref[^n]\nA\n===\n\n\n    ```\n[^n]: [[A]]\n\n    [[B]]\nÉ\n## A\n- item\n\n      # A\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 4", body: "ref[^n]\n ^absent-prose \\[[A]]> [!note] [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n[^n]: [[A]]\n\n    [[B]]\n<!--\n[[A]]\n-->É\n![[A#A]]", want: map[string]int{"page-only/B": 1}}, {name: "full original 5", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n`open\n[[A]]\nclose`![[A#A]]章節\n\t[[#A]] ^a-2\n## A-2\n## A\n## A\n> [!unknown] title\n%%[[A]]%%https://example.invalid/`[[A]]` [^n]: [[A]]\n\n    [[B]]\n ```\n   ```\n`- item\n\n      # A\n| a | b |\n|---|---|\n| [[A]] | ^a |\n~~~~\n<!--", want: map[string]int{"page-only/A": 1}}, {name: "full original 6", body: "`%%[[A]]%%A\n[[A\\|alias]]%%[[A]]%%> [!note] title\n> [!note] title\n        ```\n ^A\n\\\\[[A]]A\n---\n[[A]]## A-2\n## A\n## A\n- > [!note] title\n[[image.png]]  > [!note] title\n## [[A|alias]]\n# A\n[^n]: [[A]]\n\n    [[B]]\nref[^n]\n[[B|alias]]https://example.invalid/`[[A]]` - [ ] [[A]]\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 7", body: "\tA\n [^n]: [[A]]\n\n    [[B]]\n%%[[A]]%%## A\n## A\n ^é\n[[A]]ref[^n]\n```\n\t ^é\nA\n===\n## [[A|alias]]\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 8", body: " ^é\n``https://example.invalid/`[[A]]`   - [[A]] [[A]]<!--[[A\\|alias]]``- > [!note] title\nÉ\n> [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n[^n]: [[A]]\n\n    [[B]]\n    ^absent-prose [[A\\|alias]]ref[^n]\n[[A\\]]\n\nA\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 9", body: "ref[^n]\n ^a-2\n0[[A\\]] ^é\nhttps://example.invalid/`[[A]]`  ```\n[^n]: [[A]]\n\n    [[B]]\n> [!unknown] title\nA\n---\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 10", body: "> [!note] title\n<!-- [[A]] -->0^absent-prose ref[^n]\n    [[A\\|alias]]> > \n ^é\n0## !\n- > [!note] title\nA\n---\n- item\n\n      É\n    https://example.invalid/`[[A]]` ## A\n", want: map[string]int{"page-only/A": 1}}, {name: "full original 11", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n ^a\n![[image.png]]`open\n[[A]]\nclose`\\\\[[A]]  > [!note] title\n%%- `[[A]]`## [[A|alias]]\n`[[A]]`\\`## A-2\n## A\n## A\n> > > A\n---\nA\n---\n ^a-2\n章節\n", want: map[string]int{"page-only/A": 1}}, {name: "full original 12", body: "\\\\[[A]]\\\\[[A]]ref[^n]\n## [[A|alias]]\n[[B|alias]]- item\n\n      > >   > [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n`[[A]]`[^n]: [[A]]\n\n    [[B]]\n[^n]: [[A]]\n\n    [[B]]\n ^é\n`[[A]]````` go [[A]]\n ```\n ^a-2\ntext [[#A]]- [x] [[B]]\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 13", body: "    > [!note] one\n> [!note] two\n> [!note] three\n ^a-2\n> [!note] one\n> [!note] two\n> [!note] three\ntext [[#A]]  ```\n## A\n## A\n![[A#A]]- [x] [[B]]\n    ```\n[^n]: [[A]]\n\n    [[B]]\n章節\n0> [!note] title\n- A\n``- É\n[[#A]]章節\nref[^n]\n> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 14", body: "[^n]: [[A]]\n\n    [[B]]\n  > [!note] title\n## [[A|alias]]\n> [!unknown] title\n[[A\\]]ref[^n]\n[[B|alias]]## !\nÉ\n[[A#A]]> [!note] title\n> <!-- [[A]] -->[[A\\]]```` go [[A]]\n`[[A#A]]A\n===\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A-2\n## A\n## A\n ^é\n## !\n## A\n## A\n ^é\n[[A#A]]- > [!note] title\n^absent-prose ``[[A]]`- > [!note] title\n\n\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 15", body: "[[A]] [[A]][[image.png]]> [!note] [[A]]\n  - ref[^n]\n> [!note] title\n[^n]: [[A]]\n\n    [[B]]\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 16", body: "\\`> > https://example.invalid/`[[A]]` ", want: map[string]int{"page-only/A": 1}}, {name: "full original 17", body: "> > ^absent-prose > \n\n## [[A|alias]]\n[^n]: [[A]]\n\n    [[B]]\n- > [!note] title\n![[A#A]]```` go [[A]]\n ^é\n> [!note] one\n> [!note] two\n> [!note] three\n- item\n\n      É\n[[A]]A\n---\n[[A\\]]   ```\n## !\nref[^n]\n\n``- [ ] [[A]]\n<!-- [[A]] -->0", want: map[string]int{"page-only/B": 1}}, {name: "full original 18", body: "    ```\n%%[[A]]%%## [[A|alias]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n    ```\nB\n[[image.png]]A\n\t```\n\\\\[[A]][[B|alias]]", want: map[string]int{"page-only/image.png": 1}}, {name: "full original 19", body: "> [!note] one\n> [!note] two\n> [!note] three\n ^A\nÉ\n`- > [!note] title\n-  ^a\n> # A\n[^n]: [[A]]\n\n    [[B]]\n[[A\\|alias]]## <em>A</em>\nref[^n]\n````\nref[^n]\n章節\n     ```\n# A\n0   ```\n[^unused]: [[A]]\n ```\n ^a\n## [[A|alias]]\n~~~\n ^é\n", want: map[string]int{"page-only/B": 1}}, {name: "unused owner remains in whole check", body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n\n[^u]: [[C]]\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 20", body: "    ```\n\t  - <!-- [[A]] -->[[A#A]]    ```\n[^n]: [[A]]\n\n    [[B]]\n[[A#A]]^absent-prose %%[[A]]%%> [!unknown] title\n``` [[A]]\n[[A#^a]][[image.png]]`![[image.png]]-->[^n]: [[A]]\n\n    [[B]]\n ^a\nA\n===\n````\n[[B|alias]]ref[^n]\n0    ```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\t## [[A|alias]]\nA\n---\n章節\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 21", body: "## [[A|alias]]\n> [!note] one\n> [!note] two\n> [!note] three\n ^a-2\n[^n]: [[A]]\n\n    [[B]]\n[^n]: [[A]]\n\n    [[B]]\n^absent-prose [^n]: [[A]]\n\n    [[B]]\n> [!note] title\n![[A]]%%``~~~\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 22", body: "[[A]]\t```\n\t- [[A]] [[A]]\\[[A]]text %%[[A]]%%[^n]: [[A]]\n\n    [[B]]\n# A\n章節\n^absent-prose  ```\n[[A\\]][[image.png]][[B|alias]]\\\\[[A]]0É\n[^n]: [[A]]\n\n    [[B]]\n  - # A\n``` [[A]]\n```` go [[A]]\n\t```\n<div>\n[[A]]\n</div>\n  > [!note] title\n    ```\n[[A\\|alias]]  > [!note] title\n", want: map[string]int{"page-only/B": 1}}, {name: "full original 23", body: "- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[A#^a]]![[A]]~~~~\n~~~\n~~~\nref[^n]\n[^unused]: [[A]]\n![[image.png]]https://example.invalid/`[[A]]` --> ```\ntext ~~~~\n章節\n[^n]: [[A]]\n\n    [[B]]\n``", want: map[string]int{"page-only/B": 1}}, {name: "nested field in later paragraph", body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[E[[F]]]]\n", want: nil}, {name: "ordinary names agree", body: "[[A]] [[B]]\n", want: nil}, {name: "native code owns field", body: "[[A]]\n\n```\n[[B]]\n```\n", want: nil}, {name: "escaped continuation", body: "ref[^n]\n\n[^n]: [[A]]\n\n    \\[[B]]\n", want: nil}, {name: "nested outside field", body: "[[E[[F]]]]\n", want: nil}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, a := agreementIsolatedPage(t, c)
			budget := agreementOutsideCodeBudget(c.Body, &a)
			if (len(tc.want) > 0) != (budget.Body == c.Body) {
				t.Fatalf("caught: outside code public budget body=%q, want owned=%t", budget.Body, len(tc.want) > 0)
			}
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &a) {
				key := f.Direction + "/" + f.Tuple.Target
				kind, authority, wrong := agreementOutsideCodeDifference(c, &f, &a, &budget)
				if f.Property != "P1" || tc.want[key] == 0 {
					if kind != "" {
						t.Fatal("caught: outside code borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 4" || wrong != "judge" {
					t.Fatalf("caught: outside code public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[key] = f.Multiplicity
				agreementOutsideCodeDrift(t, c, &f, &a, &budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete outside code public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
