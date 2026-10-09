package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementNativeDiagnosticField struct {
	Source agreementNativeCheckField
	Tuple  agreementCitation
}

type agreementNativeDiagnosticSource struct {
	Page   agreementNativeOwnerSource
	Check  agreementNativeCheckSource
	Fields []agreementNativeDiagnosticField
}

// Keep authored tuples separate from check's normalized names. Literal fields
// inside discarded definitions cannot supply ordinary diagnostic occurrences.
func agreementNativeDiagnosticReading(body string) agreementNativeDiagnosticSource {
	page := agreementNativeOwnerReading(body)
	if len(page.Definitions) == 0 && len(page.Code) == 0 {
		return agreementNativeDiagnosticSource{}
	}
	reading := agreementNativeDiagnosticSource{Page: page, Check: agreementNativeCheckReading(body, page)}
	for _, source := range reading.Check.Fields {
		if source.DefinitionOwner < 0 && source.CodeOwner < 0 {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(source.Word, "[["), "]]")
		if strings.ContainsAny(inner, "[]\r\n`") {
			return agreementNativeDiagnosticSource{}
		}
		link, cites := graph.ParseWikilink(inner)
		if !cites || link.Block != "" {
			return agreementNativeDiagnosticSource{}
		}
		field := agreementNativeDiagnosticField{Source: source, Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}}
		switch {
		case source.DefinitionOwner >= 0 && source.CodeOwner >= 0:
			field.Tuple.SourceRole = agreementUnusedInlineCodeField
		case source.Escaped:
			field.Tuple.SourceRole = agreementUnusedEscapedField
		}
		reading.Fields = append(reading.Fields, field)
	}
	return reading
}

// Rename only unowned targets that share an authored name. Keeping delimiter,
// alias, extension and byte positions lets a second public render establish
// which diagnostics the owned fields actually produced.
func agreementNativeDiagnosticProjection(body string, reading *agreementNativeDiagnosticSource) (string, bool) {
	owned := make(map[string]bool)
	names := make(map[string]bool)
	for i := range reading.Fields {
		field := &reading.Fields[i]
		if field.Tuple.SourceRole == "" {
			owned[field.Tuple.Target] = true
		}
	}
	for _, field := range reading.Check.Fields {
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		link, _ := graph.ParseWikilink(inner)
		names[link.Target] = true
	}
	projected := []byte(body)
	for _, field := range reading.Check.Fields {
		if field.DefinitionOwner >= 0 || field.CodeOwner >= 0 {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		if strings.ContainsAny(inner, "[]\r\n`") {
			return "", false
		}
		link, cites := graph.ParseWikilink(inner)
		if !cites || !owned[link.Target] {
			continue
		}
		start := field.Span.Start + 2 + strings.Index(inner, link.Target)
		stop := start + len(link.Target)
		if start < field.Span.Start+2 || stop > field.Span.Stop-2 || body[start:stop] != link.Target {
			return "", false
		}
		base, extension := link.Target, ""
		if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
			base, extension = base[:dot], base[dot:]
		}
		if base == "" {
			return "", false
		}
		name := ""
		for _, letter := range []string{"q", "z", "v", "x", "w"} {
			candidate := strings.Repeat(letter, len(base)) + extension
			if !names[candidate] {
				name = candidate
				break
			}
		}
		if name == "" {
			return "", false
		}
		copy(projected[start:stop], name)
	}
	return string(projected), true
}

func agreementNativeDiagnosticBudget(t *testing.T, body string) agreementUnusedWidgetPayload {
	t.Helper()
	reading := agreementNativeDiagnosticReading(body)
	if len(reading.Fields) == 0 {
		return agreementUnusedWidgetPayload{}
	}
	// A literal field with the same name could otherwise lend its occurrence
	// to an ordinary field whose own diagnostic was suppressed.
	ordinary := make(map[string]bool)
	for i := range reading.Fields {
		if reading.Fields[i].Tuple.SourceRole == "" {
			ordinary[reading.Fields[i].Tuple.Target] = true
		}
	}
	for i := range reading.Fields {
		if reading.Fields[i].Tuple.SourceRole != "" && ordinary[reading.Fields[i].Tuple.Target] {
			return agreementUnusedWidgetPayload{}
		}
	}
	projected, valid := agreementNativeDiagnosticProjection(body, &reading)
	if !valid {
		return agreementUnusedWidgetPayload{}
	}
	projectedReading := agreementNativeDiagnosticReading(projected)
	witness, _ := agreementIsolatedPage(t, agreementCase{Body: projected})
	return agreementNativeDiagnosticEvidence(body, &reading, &projectedReading, witness.Diagnostics)
}

func agreementNativeDiagnosticEvidence(body string, reading, projectedReading *agreementNativeDiagnosticSource, diagnostics []render.Diagnostic) agreementUnusedWidgetPayload {
	if !cmp.Equal(reading.Page, projectedReading.Page) || !cmp.Equal(reading.Check.Code, projectedReading.Check.Code) || !cmp.Equal(reading.Check.Comments, projectedReading.Check.Comments) || !cmp.Equal(reading.Fields, projectedReading.Fields) {
		return agreementUnusedWidgetPayload{}
	}
	budget := make(map[agreementCitation]int)
	for i := range reading.Fields {
		budget[reading.Fields[i].Tuple]++
	}
	produced := make(map[agreementCitation]int)
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == render.DiagWikilinkBroken {
			if diagnostic.Block != "" {
				return agreementUnusedWidgetPayload{}
			}
			produced[agreementCitation{Target: diagnostic.Target, Section: diagnostic.Section, State: "wikilink-broken"}]++
		}
	}
	for tuple, count := range budget {
		if tuple.SourceRole == "" && produced[tuple] != count {
			return agreementUnusedWidgetPayload{}
		}
	}
	return agreementUnusedWidgetPayload{Body: body, Tuples: budget}
}

func agreementNativeDiagnosticDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, diagnostics []render.Diagnostic, budget agreementUnusedWidgetPayload) (kind, authority, wrong string) {
	if len(actual.Citations) != 0 {
		return "", "", ""
	}
	return agreementSharedUnusedDiagnosticDifference(c, f, actual, diagnostics, budget)
}

func TestAgreementNativeDiagnosticSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       []agreementNativeDiagnosticField
	}{
		{name: "whole authored ordinary literal and escaped fields", body: "[^n]: [[A]] `[[B]]` \\[[C]]\n", want: []agreementNativeDiagnosticField{
			{Source: agreementNativeCheckField{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Targets: []string{"A"}, DefinitionOwner: 0, CodeOwner: -1}, Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}},
			{Source: agreementNativeCheckField{Span: graph.Span{Start: 13, Stop: 18}, Word: "[[B]]", Targets: []string{"B"}, Code: true, DefinitionOwner: 0, CodeOwner: 0}, Tuple: agreementCitation{SourceRole: agreementUnusedInlineCodeField, Target: "B", State: "wikilink-broken"}},
			{Source: agreementNativeCheckField{Span: graph.Span{Start: 21, Stop: 26}, Word: "[[C]]", Targets: []string{"C"}, Escaped: true, DefinitionOwner: 0, CodeOwner: -1}, Tuple: agreementCitation{SourceRole: agreementUnusedEscapedField, Target: "C", State: "wikilink-broken"}},
		}},
		{name: "root fence metadata and body are potential producers", body: "``` [[A]]\n[[B]]\n```\n", want: []agreementNativeDiagnosticField{
			{Source: agreementNativeCheckField{Span: graph.Span{Start: 4, Stop: 9}, Word: "[[A]]", Targets: []string{"A"}, DefinitionOwner: -1, CodeOwner: 1}, Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}},
			{Source: agreementNativeCheckField{Span: graph.Span{Start: 10, Stop: 15}, Word: "[[B]]", Targets: []string{"B"}, Code: true, DefinitionOwner: -1, CodeOwner: 0}, Tuple: agreementCitation{Target: "B", State: "wikilink-broken"}},
		}},
		{name: "heading keeps raw section and alias", body: "[^n]: ## [[A#part|shown]]\n", want: []agreementNativeDiagnosticField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 9, Stop: 25}, Word: "[[A#part|shown]]", Targets: []string{"A"}, DefinitionOwner: 0, CodeOwner: -1}, Tuple: agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}}}},
		{name: "no native owner", body: "[[A]]\n"},
		{name: "used definition", body: "ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "nested field refuses earlier owned field", body: "[^n]: [[A]] [[B[[C]]]]\n"},
		{name: "cross line field refuses earlier owned field", body: "[^n]: [[A]] [[B\nC]]\n"},
		{name: "backtick field refuses earlier owned field", body: "[^n]: [[A]] [[B`C]]\n"},
		{name: "block field refuses earlier owned field", body: "[^n]: [[A]] [[B#^b]]\n"},
		{name: "local field refuses earlier owned field", body: "[^n]: [[A]] [[#B]]\n"},
		{name: "comment overlap refuses every owner", body: "[^n]: [[A]] %%[[B]]%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agreementNativeDiagnosticReading(tc.body)
			if diff := cmp.Diff(tc.want, got.Fields); diff != "" {
				t.Fatalf("caught: complete native diagnostic field source (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementNativeDiagnosticProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, want string
		valid            bool
	}{
		{name: "every same target outside owner", body: "[r]: [[A]]\n\n[[A#part|shown]] ![[A]]\n\n[^n]: [[A]]\n", want: "[r]: [[q]]\n\n[[q#part|shown]] ![[q]]\n\n[^n]: [[A]]\n", valid: true},
		{name: "all declared names protect fresh target", body: "[[q]] [[z]] [[A]]\n\n[^n]: [[A]]\n", want: "[[q]] [[z]] [[v]]\n\n[^n]: [[A]]\n", valid: true},
		{name: "image extension remains authored", body: "[r]: ![[image.png#part|shown]]\n\n[^n]: ![[image.png]]\n", want: "[r]: ![[qqqqq.png#part|shown]]\n\n[^n]: ![[image.png]]\n", valid: true},
		{name: "Unicode byte positions and CRLF", body: "[[章節#part|shown]]\r\n\r\n[^n]: [[章節]]\r\n", want: "[[qqqqqq#part|shown]]\r\n\r\n[^n]: [[章節]]\r\n", valid: true},
		{name: "escaped alias delimiter keeps another target", body: "[[A\\|shown]]\n\n[^n]: [[A\\]]\n", want: "[[A\\|shown]]\n\n[^n]: [[A\\]]\n", valid: true},
		{name: "raw escape remains target byte", body: "[[A\\]]\n\n[^n]: [[A\\]]\n", want: "[[qq]]\n\n[^n]: [[A\\]]\n", valid: true},
		{name: "only inactive owner cannot rename ordinary field", body: "[[A]]\n\n[^n]: `[[A]]`\n", want: "[[A]]\n\n[^n]: `[[A]]`\n", valid: true},
		{name: "all other targets remain unchanged", body: "[[B]]\n\n[^n]: [[A]]\n", want: "[[B]]\n\n[^n]: [[A]]\n", valid: true},
		{name: "all available names occupied refuses", body: "[[q]] [[z]] [[v]] [[x]] [[w]] [[A]]\n\n[^n]: [[A]]\n"},
		{name: "empty filename base refuses", body: "[[.png]]\n\n[^n]: [[.png]]\n"},
		{name: "unowned nested field could hide owned diagnostic", body: "[r]: [[B[[A]]]]\n\n[^n]: [[A]]\n"},
		{name: "unowned cross line field refuses", body: "[[B\nC]]\n\n[^n]: [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reading := agreementNativeDiagnosticReading(tc.body)
			original := agreementNativeDiagnosticReading(tc.body)
			got, valid := agreementNativeDiagnosticProjection(tc.body, &reading)
			if got != tc.want || valid != tc.valid {
				t.Fatalf("caught: diagnostic projection target bytes %q %t, want %q %t", got, valid, tc.want, tc.valid)
			}
			if diff := cmp.Diff(original, reading); diff != "" {
				t.Fatalf("caught: diagnostic projection changed original source facts:\n%s", diff)
			}
			if valid && len(got) != len(tc.body) {
				t.Fatal("caught: diagnostic projection changed original byte positions")
			}
		})
	}
}

func TestAgreementNativeDiagnosticEvidence(t *testing.T) {
	t.Parallel()
	body := "[^n]: [[A]] [[A]] `[[B]]` \\[[C]]\n"
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	want := map[agreementCitation]int{a: 2, {SourceRole: agreementUnusedInlineCodeField, Target: "B", State: "wikilink-broken"}: 1, {SourceRole: agreementUnusedEscapedField, Target: "C", State: "wikilink-broken"}: 1}
	diagnostics := []render.Diagnostic{{Kind: render.DiagWikilinkBroken, Target: "A"}, {Kind: render.DiagWikilinkBroken, Target: "A"}}
	reading := agreementNativeDiagnosticReading(body)
	if diff := cmp.Diff(want, agreementNativeDiagnosticEvidence(body, &reading, &reading, diagnostics).Tuples); diff != "" {
		t.Fatalf("caught: native diagnostic complete producer counts:\n%s", diff)
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementNativeDiagnosticSource)
	}{
		{"whole native definitions", func(s *agreementNativeDiagnosticSource) { s.Page.Definitions = nil }},
		{"native definition span", func(s *agreementNativeDiagnosticSource) { s.Page.Definitions[0].Span.Stop-- }},
		{"native definition label", func(s *agreementNativeDiagnosticSource) { s.Page.Definitions[0].Label = "other" }},
		{"native definition nodes", func(s *agreementNativeDiagnosticSource) { s.Page.Definitions[0].Nodes = nil }},
		{"whole native code", func(s *agreementNativeDiagnosticSource) { s.Page.Code = nil }},
		{"native code kind", func(s *agreementNativeDiagnosticSource) { s.Page.Code[0].Kind = "other" }},
		{"native code parent", func(s *agreementNativeDiagnosticSource) { s.Page.Code[0].Parent = "other" }},
		{"native comments", func(s *agreementNativeDiagnosticSource) { s.Page.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"whole check code", func(s *agreementNativeDiagnosticSource) { s.Check.Code = nil }},
		{"whole check comments", func(s *agreementNativeDiagnosticSource) { s.Check.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"every owned field", func(s *agreementNativeDiagnosticSource) { s.Fields = s.Fields[:1] }},
		{"authored target", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Tuple.Target = "other" }},
		{"authored section", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Tuple.Section = "other" }},
		{"authored state", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Tuple.State = "other" }},
		{"authored source role", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Tuple.SourceRole = "other" }},
		{"owned raw word", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.Word = "other" }},
		{"owned physical span", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.Span.Start++ }},
		{"owned normalized targets", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.Targets = nil }},
		{"owned check code flag", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.Code = true }},
		{"owned check comment flag", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.Comment = true }},
		{"owned check escape flag", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.Escaped = true }},
		{"owned definition index", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.DefinitionOwner = -1 }},
		{"owned code index", func(s *agreementNativeDiagnosticSource) { s.Fields[0].Source.CodeOwner = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementNativeDiagnosticReading(body)
			tc.change(&changed)
			if len(agreementNativeDiagnosticEvidence(body, &reading, &changed, diagnostics).Tuples) != 0 {
				t.Fatal("caught: native diagnostic producer borrowed changed source context")
			}
		})
	}
	for _, changed := range [][]render.Diagnostic{nil, diagnostics[:1], append(append([]render.Diagnostic{}, diagnostics...), diagnostics[0]), {{Kind: render.DiagMarkdownBroken, Target: "A"}, diagnostics[0]}, {{Kind: render.DiagWikilinkBroken, Target: "A", Section: "other"}, diagnostics[0]}, {{Kind: render.DiagWikilinkBroken, Target: "A", Block: "a"}, diagnostics[0]}} {
		if len(agreementNativeDiagnosticEvidence(body, &reading, &reading, changed).Tuples) != 0 {
			t.Fatal("caught: native diagnostic producer borrowed incomplete public count")
		}
	}
}

func TestAgreementNativeDiagnostics(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
	}{
		{name: "native heading in unused definition", body: "[^n]: ## [[A#part|shown]]\n", want: map[agreementCitation]int{{Target: "A", Section: "part", State: "wikilink-broken"}: 1}},
		{name: "all discarded roles keep producer counts", body: "[^n]: [[A]] `[[B]]` \\[[C]]\n", want: map[agreementCitation]int{a: 1}},
		{name: "complete discarded multiplicity", body: "[^n]: [[A]] [[A]]\n\n[^m]: [[A]]\n", want: map[agreementCitation]int{a: 3}},
		{name: "native info differs from physical fence opener", body: "  - ```` go [[A]]\n\\[[A]]text\n", want: map[agreementCitation]int{a: 1}},
		{name: "root native code skips every producer", body: "``` [[A]]\n[[B]]\n```\n"},
		{name: "root metadata cannot borrow reference diagnostic", body: "[r]: [[A]]\n\n``` [[A]]\n```\n"},
		{name: "inactive code cannot borrow reference diagnostic", body: "[r]: [[A]]\n\n[^n]: `[[A]]`\n"},
		{name: "discarded prose cannot borrow another absent producer", body: "[r]: [[A]]\n\n[^n]: [[A]]\n"},
		{name: "whole producer set refuses inactive other target", body: "``` [[A]]\n```\n\n[^n]: [[B]]\n"},
		{name: "every visible carrier stays outside new scope", body: "[[B]]\n\n[^n]: [[A]]\n"},
		{name: "used native definition remains visible", body: "ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "nested foreign source cannot lend inner target", body: "[r]: [[B[[A]]]]\n\n``` [[A]]\n```\n"},
		{name: "blocks retain separate authority", body: "[^n]: [[A#^a]]\n"},
		{name: "only literal discarded field has no producer", body: "[^n]: `[[A]]`\n"},
		{name: "literal same target cannot lend an ordinary producer", body: "[^n]: [[A]] `[[A]]`\n"},
		{name: "escaped same target cannot lend an ordinary producer", body: "[^n]: [[A]] \\[[A]]\n"},
		{name: "full original native producer 0", body: "0<!-- [[A]] -->\n\n1. ```` go [[A]]\n%%![[A]]text  ^é\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## [[A|alias]]\nA\n\t```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n ^é\n![[A]]text [[#A]]`open\n[[A]]\nclose`", want: map[agreementCitation]int{{Target: "A", Section: "", State: "wikilink-broken"}: 1}},
		{name: "full original native producer 1", body: "## A-2\n## A\n## A\n  - ```` go [[A]]\n\\[[A]]text  ^a\n", want: map[agreementCitation]int{{Target: "A", Section: "", State: "wikilink-broken"}: 1}},
		{name: "full original native producer 2", body: "B\n    ```\n## A\n[^unused]: [[A]]\n[[A#A]]## <em>A</em>\n[[A#A]]- > [!note] title\n## A\n## A\n<!--[[A]][[A#^a]]## A\n## A\n## !\n", want: map[agreementCitation]int{{Target: "A", Section: "", State: "wikilink-broken"}: 1, {Target: "A", Section: "A", State: "wikilink-broken"}: 2}},
		{name: "full original native producer 3", body: "> [!note] [[A]]\n> > ````` go [[A]]\n> [!note] [[A]]\n%%``00- [ ] [[A]]\n`## !\n\t```\n~~~~\n", want: map[agreementCitation]int{{Target: "A", Section: "", State: "wikilink-broken"}: 1}},
		{name: "full original native producer 4", body: " > [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\n\t```\n<!-- [[A]] -->> [!note] title\n<!--[^n]: [[A]]\n\n    [[B]]\n[^n]: [[A]]\n\n    [[B]]\n    ```\n> > [!unknown] title\n ^é\n> 章節\n<div>\n[[A]]\n</div>\n<div>\n[[A]]\n</div>\n ^a\n ^A\n    ```\n\\[[A]]- [ ] [[A]]\n``` [[A]]\n<!--\n[[A]]\n-->", want: map[agreementCitation]int{{Target: "A", Section: "", State: "wikilink-broken"}: 1, {Target: "B", Section: "", State: "wikilink-broken"}: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementNativeDiagnosticBudget(t, c.Body)
			r, actual := agreementIsolatedPage(t, c)
			var found map[agreementCitation]int
			failures := agreementPageFailures(c.Body, &r, &actual)
			for i := range failures {
				f := &failures[i]
				kind, authority, wrong := agreementNativeDiagnosticDifference(c, f, &actual, r.Diagnostics, budget)
				if f.Property != "P0" || f.Direction != "diagnostic-only" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatalf("caught: native diagnostic borrowed unowned public difference %s", agreementSignature(f))
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: native diagnostic public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementSharedUnusedDiagnosticDrift(t, c, f, &actual, r.Diagnostics, budget)
				for _, other := range []agreementCitation{b, {SourceRole: agreementOutsideMarkdown, Target: f.Tuple.Target, Section: f.Tuple.Section, State: "wikilink-broken"}} {
					changed := actual
					changed.Citations = append(append([]agreementCitation{}, actual.Citations...), other)
					if k, _, _ := agreementNativeDiagnosticDifference(c, f, &changed, r.Diagnostics, budget); k != "" {
						t.Fatal("caught: native diagnostic borrowed any current page carrier")
					}
				}
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete native diagnostic public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
