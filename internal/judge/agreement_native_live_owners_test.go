package judge_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

type agreementNativeLiveSource struct {
	Owned   agreementNativeDiagnosticSource
	Outside []agreementNativeDiagnosticField
	Names   map[string]bool
	Page    map[agreementCitation]int
}

// Keep every outside declaration, including inactive words. A rendered tuple
// must belong to that whole set before an erased declaration can own a deficit.
func agreementNativeLiveReading(body string) agreementNativeLiveSource {
	owned := agreementNativeDiagnosticReading(body)
	if len(owned.Fields) == 0 {
		return agreementNativeLiveSource{}
	}
	reading := agreementNativeLiveSource{Owned: owned, Names: make(map[string]bool), Page: make(map[agreementCitation]int)}
	for _, field := range owned.Check.Fields {
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		if strings.ContainsAny(inner, "[]\r\n`") {
			return agreementNativeLiveSource{}
		}
		link, cites := graph.ParseWikilink(inner)
		if !cites || link.Block != "" {
			return agreementNativeLiveSource{}
		}
		if field.DefinitionOwner >= 0 || field.CodeOwner >= 0 {
			reading.Names[link.Target] = true
			for _, target := range field.Targets {
				reading.Names[target] = true
			}
			continue
		}
		outside := agreementNativeDiagnosticField{Source: field, Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}}
		reading.Outside = append(reading.Outside, outside)
		if !field.Escaped && !graph.In(owned.Page.Comments, field.Span.Start) {
			reading.Page[outside.Tuple]++
		}
	}
	return reading
}

func agreementNativeLiveCarriers(actual *agreementHTML) map[agreementCitation]int {
	carriers := make(map[agreementCitation]int)
	for _, tuple := range actual.Citations {
		carriers[tuple]++
	}
	return carriers
}

type agreementNativeLivePayload struct {
	Native   agreementNativeCheckPayload
	Carriers map[agreementCitation]int
}

// Protect every authored and check-normalized name, including inactive fields.
// The private copy changes target bytes only; owned declarations stay intact.
func agreementNativeLiveProjection(body string, reading *agreementNativeLiveSource) (string, bool) {
	names := maps.Clone(reading.Names)
	for _, field := range reading.Owned.Check.Fields {
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		link, _ := graph.ParseWikilink(inner)
		names[link.Target] = true
		for _, target := range field.Targets {
			names[target] = true
		}
	}
	projected := []byte(body)
	for _, field := range reading.Owned.Check.Fields {
		if field.DefinitionOwner >= 0 || field.CodeOwner >= 0 {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		link, cites := graph.ParseWikilink(inner)
		if !cites || !reading.Names[link.Target] {
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

func agreementNativeLiveBudget(t *testing.T, body string, actual *agreementHTML) agreementNativeLivePayload {
	t.Helper()
	native := agreementNativeCheckBudget(body)
	if len(native.Citation.Targets) == 0 || len(actual.Citations) == 0 {
		return agreementNativeLivePayload{}
	}
	reading := agreementNativeLiveReading(body)
	if len(reading.Owned.Fields) == 0 || !cmp.Equal(reading.Page, agreementNativeLiveCarriers(actual)) {
		return agreementNativeLivePayload{}
	}
	projected, valid := agreementNativeLiveProjection(body, &reading)
	if !valid {
		return agreementNativeLivePayload{}
	}
	after := agreementNativeLiveReading(projected)
	_, witness := agreementIsolatedPage(t, agreementCase{Body: projected})
	return agreementNativeLiveEvidence(&reading, &after, actual, &witness, native)
}

// Same-name live words are renamed in a private copy. Their complete projected
// set must still be visible while every authored and normalized owner name is
// absent. Count equality alone cannot distinguish a borrowed occurrence.
func agreementNativeLiveEvidence(reading, after *agreementNativeLiveSource, actual, witness *agreementHTML, native agreementNativeCheckPayload) agreementNativeLivePayload {
	if !cmp.Equal(reading.Owned.Page, after.Owned.Page) || !cmp.Equal(reading.Owned.Check.Code, after.Owned.Check.Code) || !cmp.Equal(reading.Owned.Check.Comments, after.Owned.Check.Comments) || !cmp.Equal(reading.Owned.Fields, after.Owned.Fields) || !cmp.Equal(reading.Names, after.Names) {
		return agreementNativeLivePayload{}
	}
	if !cmp.Equal(reading.Page, agreementNativeLiveCarriers(actual)) || !cmp.Equal(after.Page, agreementNativeLiveCarriers(witness)) {
		return agreementNativeLivePayload{}
	}
	for _, tuple := range witness.Citations {
		if reading.Names[tuple.Target] {
			return agreementNativeLivePayload{}
		}
	}
	return agreementNativeLivePayload{Native: native, Carriers: agreementNativeLiveCarriers(actual)}
}

func agreementNativeLiveDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementNativeLivePayload) (kind, authority, wrong string) {
	if len(actual.Citations) == 0 || !cmp.Equal(budget.Carriers, agreementNativeLiveCarriers(actual)) {
		return "", "", ""
	}
	kind, _, wrong = agreementSharedUnusedCitationDifference(c, f, actual, budget.Native.Citation)
	if kind == "" || budget.Native.Code[f.Tuple.Target]+budget.Native.Unused[f.Tuple.Target] != f.Multiplicity {
		return "", "", ""
	}
	switch {
	case budget.Native.Code[f.Tuple.Target] > 0 && budget.Native.Unused[f.Tuple.Target] > 0:
		authority = "#1011 stages 4 and 5"
	case budget.Native.Unused[f.Tuple.Target] > 0:
		authority = "#1011 stage 5"
	default:
		authority = "#1011 stage 4"
	}
	return kind, authority, wrong
}

func TestAgreementNativeLiveSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		outside    []agreementNativeDiagnosticField
		names      map[string]bool
		page       map[agreementCitation]int
	}{
		{name: "all outside roles and whole raw normalized namespace", body: "[[A#part|shown]] [[B]] \\[[C]] %%[[D]]%%\n\n``` [[A\\]]\n[[E]]\n```\n", outside: []agreementNativeDiagnosticField{{Source: agreementNativeCheckField{Span: graph.Span{Start: 0, Stop: 16}, Word: "[[A#part|shown]]", Targets: []string{"A"}, Escaped: false, Comment: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 17, Stop: 22}, Word: "[[B]]", Targets: []string{"B"}, Escaped: false, Comment: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "B", Section: "", State: "wikilink-broken"}}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 24, Stop: 29}, Word: "[[C]]", Targets: []string{"C"}, Escaped: true, Comment: false, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "C", Section: "", State: "wikilink-broken"}}, {Source: agreementNativeCheckField{Span: graph.Span{Start: 32, Stop: 37}, Word: "[[D]]", Targets: []string{"D"}, Escaped: false, Comment: true, DefinitionOwner: -1, CodeOwner: -1}, Tuple: agreementCitation{Target: "D", Section: "", State: "wikilink-broken"}}}, names: map[string]bool{"A": true, "A\\": true, "E": true}, page: map[agreementCitation]int{a: 1, b: 1}},

		{name: "every inactive native name remains protected", body: "[^n]: [[A]] `[[B]]` \\[[C]]\n", names: map[string]bool{"A": true, "B": true, "C": true}, page: map[agreementCitation]int{}},
		{name: "metadata source has no outside carrier", body: "``` [[A]]\n```\n", names: map[string]bool{"A": true}, page: map[agreementCitation]int{}},
		{name: "no native field owner", body: "[[A]]\n"},
		{name: "outside local field refuses entire source", body: "[[#A]]\n\n``` [[A]]\n```\n"},
		{name: "outside block field refuses entire source", body: "[[A#^a]]\n\n``` [[A]]\n```\n"},
		{name: "outside nested field refuses entire source", body: "[[B[[A]]]]\n\n``` [[A]]\n```\n"},
		{name: "outside wrapped field refuses entire source", body: "[[A\nB]]\n\n``` [[A]]\n```\n"},
		{name: "outside backtick field refuses entire source", body: "[[A`B]]\n\n``` [[A]]\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agreementNativeLiveReading(tc.body)
			if diff := cmp.Diff(tc.outside, got.Outside); diff != "" {
				t.Fatalf("caught: complete native live outside source (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.names, got.Names); diff != "" {
				t.Fatalf("caught: complete native live owner namespace (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.page, got.Page); diff != "" {
				t.Fatalf("caught: complete native live outside tuple counts (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementNativeLiveProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, want string
		valid            bool
	}{
		{name: "all same name outside declarations", body: "[[A]] [[A#part|shown]] ![[A]]\n\n``` [[A]]\n```\n", want: "[[q]] [[q#part|shown]] ![[q]]\n\n``` [[A]]\n```\n", valid: true},
		{name: "inactive owner still protects its whole name", body: "[[A]] [[C]]\n\n``` [[A]]\n```\n\n[^n]: `[[C]]`\n", want: "[[q]] [[q]]\n\n``` [[A]]\n```\n\n[^n]: `[[C]]`\n", valid: true},
		{name: "normalized owner also excludes fresh name", body: "[[A]]\n\n``` [[A]]\n```\n\n[^n]: [[q\\]]\n", want: "[[z]]\n\n``` [[A]]\n```\n\n[^n]: [[q\\]]\n", valid: true},
		{name: "all outside normalized names exclude fresh name", body: "[[q\\]] [[A]]\n\n``` [[A]]\n```\n", want: "[[q\\]] [[z]]\n\n``` [[A]]\n```\n", valid: true},
		{name: "all raw declarations exclude fresh name", body: "[[q]] [[z]] [[A]]\n\n``` [[A]]\n```\n", want: "[[q]] [[z]] [[v]]\n\n``` [[A]]\n```\n", valid: true},
		{name: "extension alias and fragment stay written", body: "![[image.png#part|shown]]\n\n``` [[image.png]]\n```\n", want: "![[qqqqq.png#part|shown]]\n\n``` [[image.png]]\n```\n", valid: true},
		{name: "Unicode positions retain CRLF", body: "[[章節]]\r\n\r\n``` [[章節]]\r\n```\r\n", want: "[[qqqqqq]]\r\n\r\n``` [[章節]]\r\n```\r\n", valid: true},
		{name: "all fresh names occupied refuses", body: "[[q]] [[z]] [[v]] [[x]] [[w]] [[A]]\n\n``` [[A]]\n```\n"},
		{name: "empty filename base refuses", body: "[[.png]]\n\n``` [[.png]]\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reading := agreementNativeLiveReading(tc.body)
			original := agreementNativeLiveReading(tc.body)
			got, valid := agreementNativeLiveProjection(tc.body, &reading)
			if got != tc.want || valid != tc.valid {
				t.Fatalf("caught: native live projection target bytes %q %t, want %q %t", got, valid, tc.want, tc.valid)
			}
			if diff := cmp.Diff(original, reading); diff != "" {
				t.Fatalf("caught: native live projection changed original source facts:\n%s", diff)
			}
			if valid && len(got) != len(tc.body) {
				t.Fatal("caught: native live projection changed original byte positions")
			}
		})
	}
}

func TestAgreementNativeLiveEvidence(t *testing.T) {
	t.Parallel()
	body := "[[A]] [[A#part]] [[B]]\n\n``` [[A]]\n```\n\n[^n]: [[B]] `[[C]]`\n"
	reading := agreementNativeLiveReading(body)
	projected, valid := agreementNativeLiveProjection(body, &reading)
	if !valid {
		t.Fatal("caught: native live producer fixture has no projection")
	}
	after := agreementNativeLiveReading(projected)
	_, actual := agreementIsolatedPage(t, agreementCase{Body: body})
	_, witness := agreementIsolatedPage(t, agreementCase{Body: projected})
	native := agreementNativeCheckBudget(body)
	baseline := agreementNativeLiveEvidence(&reading, &after, &actual, &witness, native)
	if diff := cmp.Diff(native, baseline.Native); diff != "" {
		t.Fatalf("caught: native live evidence lost whole check partition:\n%s", diff)
	}
	if diff := cmp.Diff(map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "A", Section: "part", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1}, baseline.Carriers); diff != "" {
		t.Fatalf("caught: native live evidence lost original whole page:\n%s", diff)
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementNativeLiveSource)
	}{
		{"whole native definitions", func(s *agreementNativeLiveSource) { s.Owned.Page.Definitions = nil }},
		{"native definition label", func(s *agreementNativeLiveSource) { s.Owned.Page.Definitions[0].Label = "other" }},
		{"native definition span", func(s *agreementNativeLiveSource) { s.Owned.Page.Definitions[0].Span.Stop-- }},
		{"native definition nodes", func(s *agreementNativeLiveSource) { s.Owned.Page.Definitions[0].Nodes = nil }},
		{"whole native code", func(s *agreementNativeLiveSource) { s.Owned.Page.Code = nil }},
		{"native code kind", func(s *agreementNativeLiveSource) { s.Owned.Page.Code[0].Kind = "other" }},
		{"native code parent", func(s *agreementNativeLiveSource) { s.Owned.Page.Code[0].Parent = "other" }},
		{"native code span", func(s *agreementNativeLiveSource) { s.Owned.Page.Code[0].Span.Start++ }},
		{"whole native comments", func(s *agreementNativeLiveSource) { s.Owned.Page.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"whole check code", func(s *agreementNativeLiveSource) { s.Owned.Check.Code = nil }},
		{"whole check comments", func(s *agreementNativeLiveSource) { s.Owned.Check.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"every owned field", func(s *agreementNativeLiveSource) { s.Owned.Fields = s.Owned.Fields[:1] }},
		{"owned raw target", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Tuple.Target = "other" }},
		{"owned section", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Tuple.Section = "other" }},
		{"owned state", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Tuple.State = "other" }},
		{"owned source role", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Tuple.SourceRole = "other" }},
		{"owned raw word", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.Word = "other" }},
		{"owned position", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.Span.Start++ }},
		{"owned normalized names", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.Targets = nil }},
		{"owned code flag", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.Code = true }},
		{"owned comment flag", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.Comment = true }},
		{"owned escape flag", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.Escaped = true }},
		{"owned definition index", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.DefinitionOwner = 0 }},
		{"owned code index", func(s *agreementNativeLiveSource) { s.Owned.Fields[0].Source.CodeOwner = -1 }},
		{"whole protected namespace", func(s *agreementNativeLiveSource) { s.Names = map[string]bool{"unowned": true} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementNativeLiveReading(projected)
			tc.change(&changed)
			if len(agreementNativeLiveEvidence(&reading, &changed, &actual, &witness, native).Native.Citation.Targets) != 0 {
				t.Fatal("caught: native live producer borrowed changed source context")
			}
		})
	}
	for _, owner := range []string{"A", "B", "C"} {
		t.Run("whole namespace "+owner, func(t *testing.T) {
			t.Parallel()
			changed := agreementNativeLiveReading(projected)
			other := witness
			other.Citations = append(append([]agreementCitation{}, witness.Citations...), agreementCitation{Target: owner, State: "wikilink-broken"})
			changed.Page = agreementNativeLiveCarriers(&other)
			if len(agreementNativeLiveEvidence(&reading, &changed, &actual, &other, native).Native.Citation.Targets) != 0 {
				t.Fatal("caught: native live producer borrowed owned page occurrence")
			}
		})
	}
	for _, tuple := range []agreementCitation{{Target: "foreign", State: "wikilink-broken"}, {Target: "A", Section: "other", State: "wikilink-broken"}, {Target: "A", State: "wikilink"}, {SourceRole: agreementOutsideMarkdown, Target: "A", State: "wikilink-broken"}, actual.Citations[0]} {
		for _, side := range []string{"original", "projected"} {
			t.Run(side+" whole carrier "+tuple.Target+tuple.Section+tuple.State+tuple.SourceRole, func(t *testing.T) {
				t.Parallel()
				a, w := actual, witness
				if side == "original" {
					a.Citations = append(append([]agreementCitation{}, actual.Citations...), tuple)
				} else {
					w.Citations = append(append([]agreementCitation{}, witness.Citations...), tuple)
				}
				if len(agreementNativeLiveEvidence(&reading, &after, &a, &w, native).Native.Citation.Targets) != 0 {
					t.Fatal("caught: native live producer borrowed changed whole page")
				}
			})
		}
	}
}

func TestAgreementNativeLiveCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body  string
		want        map[string]int
		authority   string
		authorities map[string]string
	}{
		{name: "same target live and native code metadata", body: "[[A]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authority: "#1011 stage 4"},
		{name: "all live sections and duplicate metadata", body: "[[A]] [[A#part|shown]]\n\n``` [[A]] [[A]]\n```\n", want: map[string]int{"A": 2}, authority: "#1011 stage 4"},
		{name: "different target live and native code metadata", body: "[[B]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authority: "#1011 stage 4"},
		{name: "same target live and whole discarded declaration", body: "[[A]]\n\n[^n]: [[A]]\n", want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "all same target native owners", body: "[[A]]\n\n``` [[A]]\n```\n\n[^n]: [[A]]\n", want: map[string]int{"A": 2}, authority: "#1011 stages 4 and 5"},
		{name: "literal nested fields cannot replace live source", body: "[[A]]\n\n[^n]: [[A]] `[[A]]`\n", want: map[string]int{"A": 1}, authority: "#1011 stage 5"},
		{name: "inactive same name outside owner stays visible", body: "[[A]] [[C]]\n\n``` [[A]]\n```\n\n[^n]: `[[C]]`\n", want: map[string]int{"A": 1, "C": 1}, authorities: map[string]string{"A": "#1011 stage 4", "C": "#1011 stage 5"}},
		{name: "no current live carrier stays in earlier scope", body: "``` [[A]]\n```\n"},
		{name: "reference absent beside live source refuses", body: "[r]: [[A]]\n\n[[B]]\n\n``` [[A]]\n```\n"},
		{name: "same target reference cannot borrow live occurrence", body: "[r]: [[A]]\n\n[[A]]\n\n``` [[A]]\n```\n"},
		{name: "callout title cannot replace outside occurrence", body: "> [!note] [[A]]\n\n[[B]]\n\n``` [[A]]\n```\n"},
		{name: "unowned alias keeps its own source grammar", body: "[[A\\|shown]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authority: "#1011 stage 4"},
		{name: "outside local field refuses", body: "[[B]] [[#A]]\n\n``` [[A]]\n```\n"},
		{name: "outside block field refuses", body: "[[B]] [[A#^a]]\n\n``` [[A]]\n```\n"},
		{name: "outside nested field refuses", body: "[[B]] [[B[[A]]]]\n\n``` [[A]]\n```\n"},
		{name: "full original native live 0", body: "-  ^A\n ^A\n[[A\\|alias]]    ```\n ^a-2\n ^a\n[^unused]: [[A]]\nÉ\n``` [[A]]\n# A\n    ```\n`[[B|alias]]0> ", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 1", body: "\n> - \n\n[[A#A]]## A\n## A\n[[A]] ^é\n[^unused]: [[A]]\n- [ ] [[A]]\n![[A]]~~~~\n``` [[A]]\n- > [!note] title\n\t```\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 2", body: "%%[[A]]%%https://example.invalid/`[[A]]` [[A\\|alias]]> [!note] title\n[^n]: [[A]]\n\n    [[B]]\n``` [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n> [!note] [[A]]\n![[image.png]]- [x] [[B]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 3", body: "```` go [[A]]\ntext  ```\n    B\n\\`## !\n<!--\n[[A]]\n-->- [x] [[B]]\n![[image.png]] ^é\n[[A]]   ```\n%%[[A]]%%- [x] [[B]]\n````\n\\[[A]] %%[[A]]%% ^a\n ^a\n\\\\[[A]]%%0> [!note] one\n> [!note] two\n> [!note] three\n## !\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 4", body: "[^n]: [[A]]\n\n    [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n1. [^n]: [[A]]\n\n    [[B]]\n ```\n## A\n> > ``` [[A]]\n", want: map[string]int{"A": 3}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 5", body: "[^n]: [[A]]\n\n    [[B]]\n- [ ] [[A]]\n<!-- [[A]] -->`^absent-prose ~~~\n0 ^a-2\n``` [[A]]\n<!-- [[A]] -->`~~~~\n`open\n[[A]]\nclose`  ```\n`open\n[[A]]\nclose`<!--` ^é\n- item\n\n      \t```\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 6", body: "A\n\n> [!note] title\n0[[image.png]]B\n ^a\nA\n===\n    ```\n``` [[A]]\n[[image.png]]ref[^n]\n- [x] [[B]]\n  > [!note] title\n<!-- [[A]] -->0 ^é\n01. <!--\n[[A]]\n--><!-- [[A]] -->[[B|alias]]A\n---\n   ```\n\n\n\n<!-- [[A]] --> \\`", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 7", body: "## [[A|alias]]\n- [x] [[B]]\n[[B|alias]]-->## A-2\n## A\n## A\n\t  > [!note] title\n## A-2\n## A\n## A\nA\n===\n[[A]]## A\n## A\n``` [[A]]\nA\n---\n> > | a | b |\n|---|---|\n| [[A]] | ^a |\n> <div>\n[[A]]\n</div>\n ```\nB\n0- [ ] [[A]]\n[^n]: [[A]]\n\n    [[B]]\n<!--É\n``A\n---\n\\[[A]]![[A#A]]## A\n## A\n\n\n章節\n## A-2\n## A\n## A\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 8", body: "## [[A|alias]]\n[^unused]: [[A]]\n> > [!unknown] title\n- item\n\n      - [x] [[B]]\n- item\n\n       ^A\n## !\nA\n---\n![[image.png]]> 01. > [!note] title\n## A-2\n## A\n## A\n\t ^a\n``` [[A]]\n%%[[A]]%%[^n]: [[A]]\n\n    [[B]]\n > [!unknown] title\n章節\n    ## A-2\n## A\n## A\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 9", body: "- [x] [[B]]\n       ```\n  > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n章節\n", want: map[string]int{"A": 1, "B": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5"}},
		{name: "full original native live 10", body: " ^A\n ^é\n ^a\n- > [!note] title\n\\``open\n[[A]]\nclose``-->É\n章節\n```` go [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 11", body: "\\\\[[A]]    ```\n> [!note] title\nA\n===\n%%[[A]]%%## [[A|alias]]\n  - 01. [[B|alias]]1. ```` go [[A]]\n[^n]: [[A]]\n\n    [[B]]\n`open\n[[A]]\nclose`\t```\n", want: map[string]int{"A": 1, "B": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5"}},
		{name: "full original native live 12", body: "> [!note] title\n  - ![[image.png]]> >  ^é\n1. > > <!--https://example.invalid/`[[A]]` A\n---\n``` [[A]]\n0![[image.png]]``^absent-prose ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 13", body: "![[A]][^n]: [[A]]\n\n    [[B]]\n## [[A|alias]]\nA\n--><!--\n[[A]]\n-->```\n## A\n## A\n``` [[A]]\n## A\n## A\n\n## A\n<!--## A\n[[A]] [[A]]![[A]][[A]] [[A]]> [!note] title\n[[B|alias]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 14", body: "``` [[A]]\n``\t`open\n[[A]]\nclose`    ```\n<!-- [[A]] --> ```\n\\[[A]]\\[[A]]```` go [[A]]\n## <em>A</em>\nA\n- item\n\n      <!--\n[[A]]\n-->  ```\n`[[A]]`\t```\n[[A]]## <em>A</em>\n    ```\n## !\n%%- [x] [[B]]\n![[A]]~~~~\n```\n![[A#A]]## !\n%%[[A]]%%", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 15", body: "- ![[A#A]]> [!note] title\nÉ\n```` go [[A]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- item\n\n      > [!note] one\n> [!note] two\n> [!note] three\n- > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 16", body: "0# A\n    \\[[A]]\t[[A#A]]`[[A]]`A\n[[A]] [[A]]```` go [[A]]\n[^unused]: [[A]]\n## [[A|alias]]\n`[[A]]` ^A\n``` [[A]]\n ^a\n-->> [!note] [[A]]\n````\n ^é\n> [!note] title\n> > 1.  ^A\n> <!--\n[[A]]\n-->[[A]]![[A]]> ![[A]]", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 17", body: " ^é\n\t```\n <div>\n[[A]]\n</div>\n````\n0> [!note] one\n> [!note] two\n> [!note] three\n\n ``` [[A]]\n\n\n`open\n[[A]]\nclose``open\n[[A]]\nclose`", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 18", body: "A\n===\n ^é\nA\n===\n`    ```\n ^é\n[[image.png]][^n]: [[A]]\n\n    [[B]]\n## <em>A</em>\n[[A#A]][[A\\|alias]] ^a\n``` [[A]]\n`[[A]]` ```\n   ```\nA\n---\nref[^n]\n![[image.png]]~~~\n## A\n## A\n## A\n<!-- [[A]] --><!--É\n```` go [[A]]\n<!-- [[A]] --> ^a\n ^é\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 19", body: "\t```\n ^a-2\nB\n- > [!note] title\n## !\n[^n]: [[A]]\n\n    [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> > 0> >     ```\n- item\n\n      章節\ntext > <!-- [[A]] -->[[A#A]]É\n![[image.png]]- [x] [[B]]\n\n\n\n ## A-2\n## A\n## A\n<!-- ^a-2\n[^n]: [[A]]\n\n    [[B]]\n~~~~\n[^unused]: [[A]]\n````\n- [ ] [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 20", body: "B\n[^unused]: [[A]]\n<!--\n[[A]]\n-->[[A]] [[A]]``` [[A]]\n``` [[A]]\n<!--章節\n`[[A]]`  - > [!unknown] title\n> > \\\\[[A]]    > > ref[^n]\n## A\n%%[[A]]%%--><!--\n[[A]]\n-->    ```\n[[A\\]]\t```\n# A\n> ", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 21", body: "\t```\n[^n]: [[A]]\n\n    [[B]]\n0 ^A\n> > ## A\n## A\n ^A\nÉ\n| a | b |\n|---|---|\n| [[A]] | ^a |\n%%[[A]]%%\\[[A]]\\`0\t```\n```` go [[A]]\n![[A]]- > [!note] title\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 22", body: "## <em>A</em>\n## A\n## A\n[[A]]A\n---\n  - <!-- [[A]] -->> [!note] one\n> [!note] two\n> [!note] three\n-->0## A\n```` go [[A]]\nB\n^absent-prose <div>\n[[A]]\n</div>\nB\nref[^n]\n0## !\nÉ\n```\n\\`0É\n-  ```\n- > > text ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 23", body: "[[image.png]] ^a\n> [!note] one\n> [!note] two\n> [!note] three\n```` go [[A]]\n  ## A-2\n## A\n## A\n ^a-2\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 24", body: "![[A]][^n]: [[A]]\n\n    [[B]]\n## <em>A</em>\n[[A]] [[A]]``` [[A]]\n- > [!note] title\n\t```\n```` go [[A]]\n> [!note] title\n```` go [[A]]\n~~~\n  - --><!--`[[A]]`    ```\n## A\n-  ^a-2\n%%", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 25", body: "[^unused]: [[A]]\n  > [!note] title\n> [!unknown] title\n``` [[A]]\n## A\n## A\n## A\n`[[A]]`![[image.png]] ```\n\\``[[A]]`ref[^n]\n````\n> [!note] title\n01. <div>\n[[A]]\n</div>\n[[A]]ref[^n]\n# A\n%% ```\n## A\n## A\n`A\n    ```\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 26", body: "\t~~~\n> A\n---\n## !\n[[image.png]]`[[A]]`    ```\n ^a\n# A\n[^n]: [[A]]\n\n    [[B]]\n![[image.png]]``- [ ] [[A]]\n> [!unknown] title\n![[A#A]]章節\n```` go [[A]]\n    1. > [!note] [[A]]\n## <em>A</em>\n\\\\[[A]] `- item\n\n      ![[A#A]]\n\n![[A#A]]> > ", want: map[string]int{"A": 3, "image.png": 1}, authorities: map[string]string{"A": "#1011 stages 4 and 5", "image.png": "#1011 stage 5"}},
		{name: "full original native live 27", body: "> [!note] title\n![[image.png]]# A\n>  > [!unknown] title\n`[[A]]```> > ## !\n> [!unknown] title\nA\n^absent-prose --> ^A\n章節\n``` [[A]]\n\\\\[[A]][[B|alias]]- [x] [[B]]\n- \\\\[[A]]# A\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 28", body: "[[A]] [[A]]- > [!note] title\n[[image.png]]-->~~~\n%%[[A]]%%  > [!note] title\n-->[[image.png]]A\n===\n``` [[A]]\n ^é\n- item\n\n      %%- [x] [[B]]\n\\[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n ^a\n> [!note] title\n<!--\n[[A]]\n--> ^a-2\n> [!note] title\n  - 0![[A#A]]- ## <em>A</em>\nA\n===\n[[image.png]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 29", body: "- | a | b |\n|---|---|\n| [[A]] | ^a |\n\n  > [!note] title\n1. 0## A\n# A\n- \\\\[[A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n``` [[A]]\n^absent-prose ~~~~\n\\\\[[A]]~~~~\n```\n<!--## <em>A</em>\n## !\n[[image.png]]## <em>A</em>\n^absent-prose https://example.invalid/`[[A]]`     %%[[A]]%%[[A#A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 30", body: "[[image.png]][[A\\|alias]]B\nÉ\n[^n]: [[A]]\n\n    [[B]]\n- > [!note] title\nA\n===\n  -  ^é\n[[A#A]]> [!note] title\n```` go [[A]]\n~~~~\n[[B|alias]]- [[A\\]]^absent-prose ", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 31", body: "> [!unknown] title\n1. - [x] [[B]]\nA\n===\n## A\n## A\n> [!unknown] title\n```` go [[A]]\n> [!note] title\n`open\n[[A]]\nclose` ```\n> [!note] title\n`[[A]]`\t```\n   ```\n    ```\n[[B|alias]] ```\n   ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 32", body: "- [x] [[B]]\n  > [!note] title\n## !\n## !\n## A-2\n## A\n## A\n![[image.png]]1. A\n``` [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\\|alias]]## !\n`open\n[[A]]\nclose`B\n````\n```` go [[A]]\n```\n> ## !\n\n\n## [[A|alias]]\n<!--~~~\n![[A]]%%~~~~\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 33", body: "## A\n``` [[A]]\n## A\nA\n---\n\\`text [[A#A]]  > [!note] title\n````\n> [!unknown] title\n- [ ] [[A]]\n\\` ^a-2\n ```\n[^unused]: [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 34", body: "  > [!note] title\n- [x] [[B]]\n```` go [[A]]\nA\n===\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 35", body: "> [!note] title\n\\`\t   ```\n- [x] [[B]]\n![[A#A]] ^é\n[^n]: [[A]]\n\n    [[B]]\n[^unused]: [[A]]\n![[A]]- > [!note] title\n[^unused]: [[A]]\n[^unused]: [[A]]\n## <em>A</em>\n\\\\[[A]]`[[A]]`", want: map[string]int{"A": 5, "B": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5"}},
		{name: "full original native live 36", body: "\t  - [[image.png]]- ref[^n]\n## !\n[[A]] [[A]]`[[A]]` ^é\n- > [!note] title\n\n\n- [x] [[B]]\n``` [[A]]\n## A-2\n## A\n## A\n## [[A|alias]]\n## A\n> [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n> É\n\t```\n> - > [!note] title\n ^a\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 37", body: "- [x] [[B]]\n[^unused]: [[A]]\n    ```\nref[^n]\n<!-- [[A]] -->%%- > [!note] title\n[[A\\|alias]]# A\n[[B|alias]]\\`A\n> [!note] one\n> [!note] two\n> [!note] three\n# A\n[[A#A]]```\n0", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 38", body: "## !\n[^unused]: [[A]]\n\n\n``` [[A]]\n0  ```\n[[A#A]]> > [!note] one\n> [!note] two\n> [!note] three\n```\n  > [!note] title\n[[B|alias]]`` ^é\n0``\\\\[[A]]   ```\n[[A\\|alias]]A\n===\n> [!note] one\n> [!note] two\n> [!note] three\n1. A\n===\n`text -    ```\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]][[image.png]]![[A#A]]", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 39", body: "  - %%[[A]]%% ^é\n  - ![[A#A]]    ```\n## A-2\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n## A\n## A\n``` [[A]]\n[[A]]> [!note] one\n> [!note] two\n> [!note] three\n> [!note] [[A]]\n\t<div>\n[[A]]\n</div>\n`````\n## <em>A</em>\n[[A]]`[[A]]`> [!note] one\n> [!note] two\n> [!note] three\n  > [!note] title\n[[A#A]]## <em>A</em>\n\n\n\t- item\n\n      \\[[A]]", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 40", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n[^n]: [[A]]\n\n    [[B]]\n> [!note] title\n[[A#A]]- [x] [[B]]\n    ```\n``` [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 41", body: "章節\n- ref[^n]\n  -   > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n``` [[A]]\n## <em>A</em>\n[[A\\]]> > [[A\\]][[A]] [[A]]> [!note] one\n> [!note] two\n> [!note] three\n\n  ```\n## !\n0`## A-2\n## A\n## A\n ^A\nref[^n]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 42", body: "--> ^a-2\n`[[A]]`%%[[A]]%%> [!note] [[A]]\n ^a\n```` go [[A]]\n> [!unknown] title\n ^A\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 43", body: "`[[A]]`> [!note] title\n章節\n![[A#A]]`## [[A|alias]]\n``` [[A]]\n ^a\ntext ## A\n> [!note] title\n## A\n## A\n~~~~\n   ```\nA\n===\n[^n]: [[A]]\n\n    [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!unknown] title\n`\t~~~\n# A\n%%[[A]]%%<!--![[A]]É\n## [[A|alias]]\n- item\n\n      É\n> [!note] [[A]]\n   ```\n", want: map[string]int{"A": 3}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 44", body: "A\n===\n## A\n## A\n## A\n1.  ^a\n  > [!note] title\n\n ^é\n   ```\n\n  ^é\n   ```\n[[A]]--> ^A\n- > [!note] title\n- [ ] [[A]]\n  - B\n\n\n- [x] [[B]]\n ^é\n## !\nA\n===\n ^é\n- > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n  - > [!unknown] title\n", want: map[string]int{"A": 1, "B": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5"}},
		{name: "full original native live 45", body: "\n\nA\n---\n章節\n    # A\n![[image.png]]0章節\n\t```\n  > [!note] title\n- item\n\n        ```\n\n## A-2\n## A\n## A\n[[A]] [[A]]1. ```\nÉ\n [[A]] [[A]]- [x] [[B]]\nÉ\n``` [[A]]\n## A\n\\[[A]]%%", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 46", body: "1. ```` go [[A]]\n`[[A]]``- \\` ^é\n ^a-2\n[[A]]^absent-prose ![[A]]## A\n## A\n<!--```\n0", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 47", body: "[^n]: [[A]]\n\n    [[B]]\n\t```\n1. text ````\n| a | b |\n|---|---|\n| [[A]] | ^a |\n    [[A]] [[A]]## A\n\\\\[[A]]  > [!note] title\n[^unused]: [[A]]\n[[A]] [[A]]> [!note] [[A]]\n![[A#A]]## A\n## A\n<!-- [[A]] -->\n\n# A\n## A\n## A\n- > [!note] title\n[[B|alias]]", want: map[string]int{"A": 6}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 48", body: "     ^é\n0## !\n ````\n````\n章節\n  > [!note] title\n- item\n\n      [^n]: [[A]]\n\n    [[B]]\n- > [!note] title\n[^n]: [[A]]\n\n    [[B]]\n[[A\\|alias]]`open\n[[A]]\nclose` ^a\n~~~\n``` [[A]]\n0## A\n[[A]]```` go [[A]]\n![[A#A]]- [ ] [[A]]\n ^é\n> [!note] title\n", want: map[string]int{"A": 2, "B": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5"}},
		{name: "full original native live 49", body: "> [!unknown] title\n`[[A]]`\n## [[A|alias]]\n## A\n\\[[A]]^absent-prose - [x] [[B]]\n  - ````` go [[A]]\n## [[A|alias]]\nA\n===\n# A\n[[A\\|alias]][[A]] [[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 50", body: "[^unused]: [[A]]\n![[A]][[A\\|alias]]````\n![[A]]## A\n## A\n章節\n  - \\\\[[A]]```` go [[A]]\n![[image.png]][[A]]text ## <em>A</em>\n ^a-2\nA\n===\n```` go [[A]]\n[[A#A]]<!-- [[A]] -->- [x] [[B]]\n> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n    ```\n> [!note] one\n> [!note] two\n> [!note] three\n~~~\n", want: map[string]int{"A": 5}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 51", body: "\\`[^unused]: [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n- [x] [[B]]\n[^unused]: [[A]]\n```` go [[A]]\n~~~~\n[[image.png]]É\n``[[B|alias]]章節\n``  - ``\n> [!note] one\n> [!note] two\n> [!note] three\n- - > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 52", body: "1. [[A\\|alias]]> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n<!--> [!unknown] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 53", body: "``![[A#A]][^unused]: [[A]]\nref[^n]\n  - ````\n^absent-prose É\n## A\n## A\n1. 0``` [[A]]\n1. > [!note] [[A]]\n> [!note] title\n[[B|alias]]  - ## A\n## !\n``` [[A]]\n[[A]]  ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 54", body: "[[A]]   ```\n> [!unknown] title\n[^n]: [[A]]\n\n    [[B]]\n  ```\n````\n[^n]: [[A]]\n\n    [[B]]\n[[A]]![[A]]## A-2\n## A\n## A\n章節\n ^é\n\\`````\n## <em>A</em>\n```` go [[A]]\ntext É\nÉ\n  ```\nhttps://example.invalid/`[[A]]` > > ## <em>A</em>\n ^A\n~~~\n1.     ref[^n]\n> [!note] [[A]]\n", want: map[string]int{"A": 5}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 55", body: "[[A]] [[A]]~~~~\n[[A#A]]É\n00> > \t[[B|alias]]> [!note] one\n> [!note] two\n> [!note] three\nB\n## <em>A</em>\n``` [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 56", body: "> > \\` > [!note] title\n![[image.png]]> [!note] title\n```` go [[A]]\n^absent-prose A\n<!--![[A]]\\`0- [x] [[B]]\n[[A\\|alias]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 57", body: "\n\n `[[A]]`\\`<!--章節\nÉ\n<div>\n[[A]]\n</div>\n``<div>\n[[A]]\n</div>\nB\n[^n]: [[A]]\n\n    [[B]]\n`[[A]]```` [[A]]\n<!-- [[A]] -->    ```\n ^é\n- [x] [[B]]\n^absent-prose A\n===\n[^unused]: [[A]]\nhttps://example.invalid/`[[A]]` [[A#A]]## A\n## A\n    https://example.invalid/`[[A]]` ", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 58", body: "  > [!note] title\n     ```\n ^a-2\n\\[[A]]-->[[A#A]]\t[^unused]: [[A]]\n```\n[[A\\]]## !\n\t\n\n`````\n``` [[A]]\n# A\n\t   ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 59", body: "%%%%[[A]]%%![[A#A]]%%[[image.png]]- item\n\n       ^é\n ^a\n> É\n> > ## A\n## A\n## !\n``` [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 60", body: "[[A]] [[A]]<!--[^unused]: [[A]]\n- item\n\n      \n\n<!-- [[A]] -->~~~~\n```` go [[A]]\n ^é\n%%[[A]]%%## A-2\n## A\n## A\n![[A]]\\[[A]]\t```\n> > - item\n\n      ``` [[A]]\n    ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 61", body: "~~~\n%%B\n%%[[A]]%%É\n~~~\n<!-- [[A]] -->[[A\\|alias]][[A]] [[A]]```` go [[A]]\n1.  ``` [[A]]\n- [[image.png]]> [!note] title\nA\n===\n## A\n## A\n   ```\n `[[A]]`", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 62", body: "A\n===\n[[A]] [[A]]%%[[A]]%%ref[^n]\n## A\n1. ```\nref[^n]\nÉ\n > ```\n``` [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 63", body: "[[A\\|alias]]`É\n[[A]] [[A]]``` [[A]]\n# A\n  - ````\n```` go [[A]]\n\n\n<!--\n[[A]]\n-->^absent-prose ^absent-prose \\`## A-2\n## A\n## A\n## <em>A</em>\n- [x] [[B]]\n# A\n````\n[[A#A]]~~~\nref[^n]\n<!--` ^é\n<div>\n[[A]]\n</div>\n![[A]]\\`", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 64", body: "## [[A|alias]]\n> [!note] one\n> [!note] two\n> [!note] three\n ^a-2\n[^n]: [[A]]\n\n    [[B]]\n[^n]: [[A]]\n\n    [[B]]\n^absent-prose [^n]: [[A]]\n\n    [[B]]\n> [!note] title\n![[A]]%%``~~~\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 65", body: "> [!unknown] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n# A\nB\n![[A]]É\n- [ ] [[A]]\n## A\n``` [[A]]\n <!--> [!unknown] title\n[^unused]: [[A]]\n\n ^a\n> [[A\\|alias]]`## [[A|alias]]\n    ```\n- [ ] [[A]]\n- [ ] [[A]]\n[[A]]    ```\n  - ## [[A|alias]]\n> [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 66", body: "0 ^é\n> [!unknown] title\n> \\\\[[A]] ```` go [[A]]\n```` go [[A]]\n[^n]: [[A]]\n\n    [[B]]\n## [[A|alias]]\n> [!note] title\n%%    ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 67", body: "[[image.png]]~~~\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[A\\]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 68", body: "![[A#A]]![[A]] ```\n> > [!unknown] title\n## A\n[[A\\|alias]][[A]][[B|alias]]``` [[A]]\n<!-- [[A]] -->   ```\n  -  ^A\n\t1. ``É\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n- > [!note] title\n1. %%[[A]]%%- [ ] [[A]]\n``` [[A]]\n# A\n- item\n\n      > [!note] title\n> [!note] title\n%%[[A]]%%- > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 69", body: "[^unused]: [[A]]\n ^a\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- [ ] [[A]]\n> [!unknown] title\n ```\n> > ", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 70", body: "0## !\n## A\n## A\n[[A#A]]```\n\n ^é\n- > [!note] title\nA\n===\n- > [!note] title\n`> [!note] one\n> [!note] two\n> [!note] three\n## A\n## A\n[^unused]: [[A]]\n```` go [[A]]\n%%## A\n## A\n^absent-prose \\`B\nref[^n]\n- item\n\n        - ", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 71", body: "``![[A#A]]> \\\\[[A]]\t``- [ ] [[A]]\n  - [^n]: [[A]]\n\n    [[B]]\n`[[A]]`~~~~\n    ```\nÉ\n```` go [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 72", body: "> [!note] title\n\\[[A]][[A#A]]1. %%[[A]]%%\n1. 章節\n``` [[A]]\n[[A]] ^A\nÉ\n- item\n\n      > \\`> [!note] [[A]]\nÉ\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 73", body: "%%## [[A|alias]]\n- > [!note] title\ntext  ```\nhttps://example.invalid/`[[A]]` [[A\\|alias]] ^é\n## A\n## A\n[[A#A]]`open\n[[A]]\nclose`%%## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n`[[A]]`É\n0```\n章節\n-->\\\\[[A]]É\n## A\n## A\nÉ\n[[A#A]]   ```\n[[A]] [[A]]```` go [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 74", body: "> [!unknown] title\n``` [[A]]\n> [!note] title\nA\n---\nÉ\n\\`text ## A-2\n## A\n## A\n[[A\\]]0%%- > [!note] title\n\\`0\t```\n ^é\n%% [[A#A]]![[A]]``` [[A]]\n## A\n<!-- [[A]] -->[[A#A]]## A\n<!--`open\n[[A]]\nclose`A\n===\n````\n![[image.png]]A\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 75", body: "[[A\\|alias]]````\n- item\n\n      ^absent-prose     ## A\n## A\n`\\`> \n- > [!note] title\n\t```\n## A\n- # A\n# A\n章節\nA\n```` go [[A]]\n%%[[A]]%%   ```\nA\n ^a-2\n\n\nA\n===\n<div>\n[[A]]\n</div>\nref[^n]\n<!-- [[A]] -->[[A]] [[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 76", body: "![[image.png]] ```\n## A\n## A\n## !\n ^A\n[[B|alias]]  ```\n[[A]] [[A]]<!--\n[[A]]\n-->  - [[A]] [[A]]%%[[A]]%%[^unused]: [[A]]\n![[A#A]]\n```` go [[A]]\n- [x] [[B]]\nA\n---\n- [ ] [[A]]\n```\n- [x] [[B]]\n[^unused]: [[A]]\n\ttext - [ ] [[A]]\n ^é\n> [!note] title\n[[A\\]][^n]: [[A]]\n\n    [[B]]\n[[A]] [[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 77", body: " ^é\n> [!note] one\n> [!note] two\n> [!note] three\n-->``` [[A]]\n## A\n- [x] [[B]]\nB\n\\[[A]]```\nA\n![[image.png]]![[image.png]]\\[[A]]0- item\n\n      | a | b |\n|---|---|\n| [[A]] | ^a |\n\n\n   ```\n```` go [[A]]\n ^A\n- > [!note] title\n   ```\n``` [[A]]\n ```\n ^A\n ```\n![[image.png]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 78", body: " ^a\n- > [!note] title\n ^a-2\n> [!unknown] title\n[[A]] [[A]]É\n\\`![[A#A]]text text 0\\\\[[A]][[B|alias]]![[A#A]]![[image.png]][[B|alias]]> > \\\\[[A]]\\[[A]]<!-- [[A]] -->## <em>A</em>\n``` [[A]]\n    ```\n ^A\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 79", body: "[[A#A]]> [!unknown] title\n`[[A]]` ^é\n[^n]: [[A]]\n\n    [[B]]\n`~~~\n``` [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 80", body: "[[image.png]][^unused]: [[A]]\n## [[A|alias]]\n## A-2\n## A\n## A\n- > [!unknown] title\nA\n===\n> [!note] one\n> [!note] two\n> [!note] three\n``` [[A]]\n章節\n## A\nref[^n]\n%%text \t```\n# A\n- [x] [[B]]\n![[A]]\n## <em>A</em>\n```\nA\n---\n``%% ^a\n[[B|alias]][[image.png]]%%- item\n\n      - [ ] [[A]]\nA\n===\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 81", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n`\\`  - - > [!note] title\n> [!unknown] title\n[[A\\|alias]]0## A\n## A\n## <em>A</em>\n[^unused]: [[A]]\n\t```\n<div>\n[[A]]\n</div>\n 0[[image.png]]<!--", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 82", body: "  -     text | a | b |\n|---|---|\n| [[A]] | ^a |\n1. [^unused]: [[A]]\n- [x] [[B]]\n``` [[A]]\n[[A]]", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 83", body: "[[A]]    - item\n\n        > [!note] title\n- [[A#A]]````\n<!-- [[A]] --><!--\n[[A]]\n-->^absent-prose ## A\n## A\n<!--<!--\n[[A]]\n-->- > [!note] title\n[[A]]## A\n## A\n1. <!--> [!note] one\n> [!note] two\n> [!note] three\n```` go [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 84", body: "\tA\n ^é\n1. %%[[A]]%%^absent-prose ![[A#A]][^n]: [[A]]\n\n    [[B]]\n```` go [[A]]\nhttps://example.invalid/`[[A]]`  ^é\n ^é\n![[A#A]]É\n- | a | b |\n|---|---|\n| [[A]] | ^a |\n ^A\n## A\n``", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 85", body: "%%[[A]]%% ^é\n## !\n[[image.png]] ^a\n```` go [[A]]\n## !\n## A-2\n## A\n## A\n\\\\[[A]]~~~~\n![[image.png]][^n]: [[A]]\n\n    [[B]]\n ^A\n    - item\n\n      ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 86", body: "    ```\n## !\n ^a-2\n-->-->```\n[[A#A]]~~~\n## A\n0 ^a-2\n<!--\\[[A]]text A\n ^A\nA\n\n\n[[image.png]]É\n[[A]]\n![[image.png]]  ```\n1. <!-- [[A]] -->A\n---\n[^n]: [[A]]\n\n    [[B]]\n```` go [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 87", body: "[[B|alias]][^unused]: [[A]]\n[^n]: [[A]]\n\n    [[B]]\n``````` [[A]]\n# A\n``` [[A]]\n00~~~\n\\`0> %%A\n[[A\\]]> [!unknown] title\ntext   -  ^A\n- > [!note] title\n> [!note] title\n[[A]] [[A]]   ```\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 88", body: "````\n> [!note] [[A]]\nA\n> [!note] title\n  ```\nA\n````\n1. > [!unknown] title\n`[[A]]`<div>\n[[A]]\n</div>\n- item\n\n      [[image.png]]text ``` [[A]]\n - ```` go [[A]]\n- > [!note] title\n章節\n## !\nA\n===\n ^é\n> [!note] one\n> [!note] two\n> [!note] three\n    ```\n- [ ] [[A]]\n\n\n- [x] [[B]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 89", body: "[^n]: [[A]]\n\n    [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- [ ] [[A]]\n- [x] [[B]]\nA\n===\n    ```\n\t```\n``` [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n   ```\n```` go [[A]]\n https://example.invalid/`[[A]]` - [ ] [[A]]\n- > [!note] title\n[[B|alias]]~~~~\n章節\n> - > [!note] title\n", want: map[string]int{"A": 4}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 90", body: "- > [!note] title\n- [x] [[B]]\n[^n]: [[A]]\n\n    [[B]]\n## <em>A</em>\n> [!unknown] title\n![[A#A]]\\[[A]]\\[[A]] ```\nA\n- [x] [[B]]\n[[B|alias]]^absent-prose [[A\\|alias]]\n\n````\n`open\n[[A]]\nclose````[[A]]` ^A\n", want: map[string]int{"A": 1, "B": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5"}},
		{name: "full original native live 91", body: "- [x] [[B]]\n## A-2\n## A\n## A\n## A\n## A\n> [!note] title\n``` [[A]]\n## A-2\n## A\n## A\n[[A#A]][[A]]`[[A]]`[[A\\|alias]]`ref[^n]\n~~~\n``` [[A]]\n  > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 92", body: "\\\\[[A]]É\n> [!unknown] title\n\\[[A]]  - <!-- [[A]] --> ^A\n[[A]][[A]]- [ ] [[A]]\n1. \t[^unused]: [[A]]\n\\[[A]][[A]]", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original native live 93", body: "> > text \\`| a | b |\n|---|---|\n| [[A]] | ^a |\n![[A#A]]É\n> [!note] title\n- [ ] [[A]]\n[[B|alias]]章節\n<!-- [[A]] -->## !\n\t```\n```` go [[A]]\n## A\n%%[[A]]%%> [!unknown] title\nA\nref[^n]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 94", body: "B\n[^n]: [[A]]\n\n    [[B]]\n``` [[A]]\n## [[A|alias]]\n- [x] [[B]]\n<!-- [[A]] --><!--\n[[A]]\n--> ```\nÉ\nB\n ```\n[^n]: [[A]]\n\n    [[B]]\n\\\\[[A]]## A-2\n## A\n## A\n[^n]: [[A]]\n\n    [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A-2\n## A\n## A\n\\[[A]]  -  ^A\n![[A#A]]\n>  ^é\n", want: map[string]int{"A": 6}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 95", body: "<!--\t   ```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n## A\n ^é\n> [!note] [[A]]\n\\`[[A\\]]``` [[A]]\n-->> [!note] one\n> [!note] two\n> [!note] three\n    ```\n## [[A|alias]]\n> >  ^A\n<!--\n[[A]]\n-->A\n===\n```` go [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original native live 96", body: "`É\n\\\\[[A]]# A\n[^unused]: [[A]]\n> [!note] title\n```` go [[A]]\n~~~~\n  - ## <em>A</em>\n- [x] [[B]]\n## !\n>  ^é\n ^a\n ^é\n[[A]] [[A]]## !\n%%-->%%", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original native live 97", body: "[[A#A]]0 ^é\n- > [!note] title\n  - %%[[A]]%%~~~~\n``` [[A]]\n ^A\n1. > [!note] [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementNativeLiveBudget(t, c.Body, &actual)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementNativeLiveDifference(c, &f, &actual, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatalf("caught: native live borrowed unowned public difference %s", agreementSignature(&f))
					}
					continue
				}
				expectedAuthority := tc.authority
				if tc.authorities != nil {
					expectedAuthority = tc.authorities[f.Tuple.Target]
				}
				if kind != "debt" || wrong != "judge" || authority != expectedAuthority {
					t.Fatalf("caught: native live public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementNativeLiveDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete native live public receipts (-want +got):\n%s", diff)
			}
		})
	}
}

func agreementNativeLiveDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementNativeLivePayload) {
	t.Helper()
	agreementSharedUnusedCitationDrift(t, c, f, actual, budget.Native.Citation)
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementNativeLiveDifference(other, f, actual, budget); kind != "" {
			t.Fatal("caught: native live borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementNativeLiveDifference(c, &changed, actual, budget); kind != "" {
			t.Fatalf("caught: native live borrowed signed receipt %s", agreementSignature(&changed))
		}
	}
	for _, extra := range []agreementCitation{{Target: "foreign", State: "wikilink-broken"}, {Target: f.Tuple.Target, Section: "other", State: "wikilink-broken"}, {Target: f.Tuple.Target, State: "wikilink"}, {SourceRole: agreementOutsideMarkdown, Target: f.Tuple.Target, State: "wikilink-broken"}, actual.Citations[0]} {
		changed := *actual
		changed.Citations = append(append([]agreementCitation{}, actual.Citations...), extra)
		if kind, _, _ := agreementNativeLiveDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: native live borrowed changed whole current page")
		}
	}
	for i := range actual.Citations {
		changed := *actual
		changed.Citations = append(append([]agreementCitation{}, actual.Citations[:i]...), actual.Citations[i+1:]...)
		if kind, _, _ := agreementNativeLiveDifference(c, f, &changed, budget); kind != "" {
			t.Fatal("caught: native live borrowed partial current page")
		}
	}
	changed := *actual
	changed.Citations = nil
	if kind, _, _ := agreementNativeLiveDifference(c, f, &changed, budget); kind != "" {
		t.Fatal("caught: native live borrowed empty current page")
	}
	empty := budget
	empty.Carriers = agreementNativeLiveCarriers(&changed)
	if kind, _, _ := agreementNativeLiveDifference(c, f, &changed, empty); kind != "" {
		t.Fatal("caught: native live borrowed empty page binding")
	}
	for _, owner := range []string{"code", "unused"} {
		other := budget
		other.Native.Code = make(map[string]int)
		other.Native.Unused = make(map[string]int)
		maps.Copy(other.Native.Code, budget.Native.Code)
		maps.Copy(other.Native.Unused, budget.Native.Unused)
		if owner == "code" {
			other.Native.Code[f.Tuple.Target]++
		} else {
			other.Native.Unused[f.Tuple.Target]++
		}
		if kind, _, _ := agreementNativeLiveDifference(c, f, actual, other); kind != "" {
			t.Fatal("caught: native live borrowed incomplete owner partition")
		}
	}
}
