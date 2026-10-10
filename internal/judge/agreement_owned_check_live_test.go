package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

type agreementOwnedCheckLiveSource struct {
	Native  agreementNativeOwnerSource
	Check   agreementNativeCheckSource
	Owned   []agreementNativeCheckField
	Outside []agreementDestinationLiveField
	Names   map[string]bool
	Page    map[agreementCitation]int
}

// A check occurrence is owned by its entire consumed declaration, even when
// its raw word is local, block-addressed or malformed. It supplies no page
// diagnostic tuple. Every normalized name remains protected in the witness.
func agreementOwnedCheckLiveReading(body string) agreementOwnedCheckLiveSource {
	native := agreementNativeOwnerReading(body)
	if len(native.Definitions) == 0 && len(native.Code) == 0 {
		return agreementOwnedCheckLiveSource{}
	}
	reading := agreementOwnedCheckLiveSource{Native: native, Check: agreementNativeCheckReading(body, native), Names: make(map[string]bool), Page: make(map[agreementCitation]int)}
	for _, field := range reading.Check.Fields {
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		link, cites := graph.ParseWikilink(inner)
		if field.DefinitionOwner >= 0 || field.CodeOwner >= 0 {
			reading.Owned = append(reading.Owned, field)
			reading.Names[link.Target] = true
			for _, target := range field.Targets {
				reading.Names[target] = true
			}
			continue
		}
		if strings.ContainsAny(inner, "[]\r\n") {
			return agreementOwnedCheckLiveSource{}
		}
		outside := agreementDestinationLiveField{Source: field, Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}, Block: link.Block, Cites: cites, Comment: graph.In(native.Comments, field.Span.Start)}
		reading.Outside = append(reading.Outside, outside)
		if outside.Cites && !outside.Source.Escaped && !outside.Comment {
			reading.Page[outside.Tuple]++
		}
	}
	if len(reading.Owned) == 0 {
		return agreementOwnedCheckLiveSource{}
	}
	return reading
}

// The two partitions retain every ordered field of the whole-body check.
// Bind raw words and normalized lists directly to their public input bytes.
func agreementOwnedCheckLiveLedger(body string, reading *agreementOwnedCheckLiveSource) bool {
	if !cmp.Equal(reading.Check.Targets, judge.LinkTargets(body)) {
		return false
	}
	owned, outside := 0, 0
	for _, field := range reading.Check.Fields {
		start, stop := field.Span.Start, field.Span.Stop
		if start < 0 || stop > len(body) || start > stop || body[start:stop] != field.Word || !cmp.Equal(field.Targets, judge.LinkTargets(field.Word)) {
			return false
		}
		if field.DefinitionOwner >= 0 || field.CodeOwner >= 0 {
			if owned >= len(reading.Owned) || !cmp.Equal(field, reading.Owned[owned]) {
				return false
			}
			owned++
		} else {
			if outside >= len(reading.Outside) || !cmp.Equal(field, reading.Outside[outside].Source) {
				return false
			}
			outside++
		}
	}
	return owned == len(reading.Owned) && outside == len(reading.Outside)
}

func agreementOwnedCheckLiveShapes(body string, reading *agreementOwnedCheckLiveSource) ([]agreementDestinationLiveField, bool) {
	return agreementDestinationLiveShapes(body, &agreementDestinationLiveSource{Outside: reading.Outside})
}

func agreementOwnedCheckLiveProjection(body string, reading *agreementOwnedCheckLiveSource) (string, bool) {
	return agreementDestinationLiveProjection(body, &agreementDestinationLiveSource{Owned: agreementNativeDiagnosticSource{Check: reading.Check}, Outside: reading.Outside, Names: reading.Names})
}

func agreementOwnedCheckLiveBudget(t *testing.T, body string, actual *agreementHTML) agreementNativeLivePayload {
	t.Helper()
	native := agreementNativeCheckBudget(body)
	if len(native.Citation.Targets) == 0 || len(actual.Citations) == 0 {
		return agreementNativeLivePayload{}
	}
	reading := agreementOwnedCheckLiveReading(body)
	if len(reading.Owned) == 0 || !cmp.Equal(reading.Page, agreementNativeLiveCarriers(actual)) {
		return agreementNativeLivePayload{}
	}
	projected, valid := agreementOwnedCheckLiveProjection(body, &reading)
	if !valid {
		return agreementNativeLivePayload{}
	}
	after := agreementOwnedCheckLiveReading(projected)
	_, witness := agreementIsolatedPage(t, agreementCase{Body: projected})
	return agreementOwnedCheckLiveEvidence(body, projected, &reading, &after, actual, &witness, native)
}

func agreementOwnedCheckLiveEvidence(body, projected string, reading, after *agreementOwnedCheckLiveSource, actual, witness *agreementHTML, native agreementNativeCheckPayload) agreementNativeLivePayload {
	if !agreementOwnedCheckLiveLedger(body, reading) || !agreementOwnedCheckLiveLedger(projected, after) {
		return agreementNativeLivePayload{}
	}
	if !cmp.Equal(reading.Native, after.Native) || !cmp.Equal(reading.Check.Code, after.Check.Code) || !cmp.Equal(reading.Check.Comments, after.Check.Comments) || !cmp.Equal(reading.Owned, after.Owned) || !cmp.Equal(reading.Names, after.Names) {
		return agreementNativeLivePayload{}
	}
	beforeShapes, beforeValid := agreementOwnedCheckLiveShapes(body, reading)
	afterShapes, afterValid := agreementOwnedCheckLiveShapes(projected, after)
	if !beforeValid || !afterValid || !cmp.Equal(beforeShapes, afterShapes) {
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

func TestAgreementOwnedCheckLiveSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		owned      []agreementNativeCheckField
		names      map[string]bool
		page       map[agreementCitation]int
	}{
		{name: "all opaque owned words retain whole check fields", body: "[[A]]\n\n[^n]: [[B#^a]] [[#part]] [[C`D]] [[E[[F]]]] \\[[G]]\n\n    [[H\n    I]]\n", owned: []agreementNativeCheckField{{Span: graph.Span{Start: 13, Stop: 21}, Word: "[[B#^a]]", Targets: []string{"B"}, DefinitionOwner: 0, CodeOwner: -1, Escaped: false, Code: false, Comment: false}, {Span: graph.Span{Start: 22, Stop: 31}, Word: "[[#part]]", Targets: []string{}, DefinitionOwner: 0, CodeOwner: -1, Escaped: false, Code: false, Comment: false}, {Span: graph.Span{Start: 32, Stop: 39}, Word: "[[C`D]]", Targets: []string{"C`D"}, DefinitionOwner: 0, CodeOwner: -1, Escaped: false, Code: false, Comment: false}, {Span: graph.Span{Start: 40, Stop: 48}, Word: "[[E[[F]]", Targets: []string{"E[[F"}, DefinitionOwner: 0, CodeOwner: -1, Escaped: false, Code: false, Comment: false}, {Span: graph.Span{Start: 52, Stop: 57}, Word: "[[G]]", Targets: []string{"G"}, DefinitionOwner: 0, CodeOwner: -1, Escaped: true, Code: false, Comment: false}, {Span: graph.Span{Start: 63, Stop: 74}, Word: "[[H\n    I]]", Targets: []string{}, DefinitionOwner: 0, CodeOwner: -1, Escaped: false, Code: true, Comment: false}}, names: map[string]bool{"B": true, "": true, "C`D": true, "E[[F": true, "G": true, "H\n    I": true}, page: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}},
		{name: "all metadata block local and backtick words", body: "[[B]]\n\n~~~ [[A#^a]] [[#part]] [[C`D]]\n~~~\n", owned: []agreementNativeCheckField{{Span: graph.Span{Start: 11, Stop: 19}, Word: "[[A#^a]]", Targets: []string{"A"}, DefinitionOwner: -1, CodeOwner: 0, Escaped: false, Code: false, Comment: false}, {Span: graph.Span{Start: 20, Stop: 29}, Word: "[[#part]]", Targets: []string{}, DefinitionOwner: -1, CodeOwner: 0, Escaped: false, Code: false, Comment: false}, {Span: graph.Span{Start: 30, Stop: 37}, Word: "[[C`D]]", Targets: []string{"C`D"}, DefinitionOwner: -1, CodeOwner: 0, Escaped: false, Code: false, Comment: false}}, names: map[string]bool{"A": true, "": true, "C`D": true}, page: map[agreementCitation]int{{Target: "B", State: "wikilink-broken"}: 1}},
		{name: "inactive names and unique code definition indices", body: "[[A]]\n\n[^n]: [[B]] `[[C]]` \\[[D]]\n", owned: []agreementNativeCheckField{{Span: graph.Span{Start: 13, Stop: 18}, Word: "[[B]]", Targets: []string{"B"}, DefinitionOwner: 0, CodeOwner: -1, Escaped: false, Code: false, Comment: false}, {Span: graph.Span{Start: 20, Stop: 25}, Word: "[[C]]", Targets: []string{"C"}, DefinitionOwner: 0, CodeOwner: 0, Escaped: false, Code: true, Comment: false}, {Span: graph.Span{Start: 28, Stop: 33}, Word: "[[D]]", Targets: []string{"D"}, DefinitionOwner: 0, CodeOwner: -1, Escaped: true, Code: false, Comment: false}}, names: map[string]bool{"B": true, "C": true, "D": true}, page: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}},
		{name: "native comment overlap still refuses", body: "[[A]]\n\n[^n]: [[B]] `[[C]]` \\[[D]] %%[[E]]%%\n"},
		{name: "no native owner", body: "[[A]]\n"},
		{name: "native code without an owned field", body: "`words`\n[[A]]\n"},
		{name: "nested outside still refuses", body: "[[B[[A]]]]\n\n``` [[A#^a]]\n```\n"},
		{name: "wrapped outside still refuses", body: "[[B\nA]]\n\n``` [[A#^a]]\n```\n"},
		{name: "partial native ownership still refuses", body: "[[B]]\n\n``` [[A\nB]]\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agreementOwnedCheckLiveReading(tc.body)
			if diff := cmp.Diff(tc.owned, got.Owned); diff != "" {
				t.Fatalf("caught: complete owned check live fields (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.names, got.Names); diff != "" {
				t.Fatalf("caught: complete owned check live namespace (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.page, got.Page); diff != "" {
				t.Fatalf("caught: complete owned check live outside tuples (-want +got):\n%s", diff)
			}
			if len(tc.owned) > 0 && !agreementOwnedCheckLiveLedger(tc.body, &got) {
				t.Fatal("caught: canonical owned check live ledger lacks public byte binding")
			}
		})
	}
}
func TestAgreementOwnedCheckLiveProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, want string
		valid            bool
	}{
		{name: "local and every block occurrence", body: "[[#q]] [[A#^a|shown]] ![[A#^b]]\n\n``` [[A]]\n```\n", want: "[[#q]] [[q#^a|shown]] ![[q#^b]]\n\n``` [[A]]\n```\n", valid: true},
		{name: "foreign backtick destination stays written", body: "[[B`C]] [[A]]\n\n``` [[A]]\n```\n", want: "[[B`C]] [[q]]\n\n``` [[A]]\n```\n", valid: true},
		{name: "normalized owner excludes fresh destination", body: "[[A#^a]]\n\n``` [[A]]\n```\n\n[^n]: [[q\\]]\n", want: "[[z#^a]]\n\n``` [[A]]\n```\n\n[^n]: [[q\\]]\n", valid: true},
		{name: "normalized outside excludes fresh destination", body: "[[q\\]] [[A#^a]]\n\n``` [[A]]\n```\n", want: "[[q\\]] [[z#^a]]\n\n``` [[A]]\n```\n", valid: true},
		{name: "image extension block and alias stay written", body: "![[image.png#^a|shown]]\n\n``` [[image.png]]\n```\n", want: "![[qqqqq.png#^a|shown]]\n\n``` [[image.png]]\n```\n", valid: true},
		{name: "Unicode CRLF positions stay written", body: "[[#a]] [[章節#^a]]\r\n\r\n``` [[章節]]\r\n```\r\n", want: "[[#a]] [[qqqqqq#^a]]\r\n\r\n``` [[章節]]\r\n```\r\n", valid: true},
		{name: "inactive names remain protected", body: "[[A]] [[C#^a]]\n\n``` [[A]]\n```\n\n[^n]: `[[C]]`\n", want: "[[q]] [[q#^a]]\n\n``` [[A]]\n```\n\n[^n]: `[[C]]`\n", valid: true},
		{name: "all fresh destinations occupied", body: "[[q]] [[z]] [[v]] [[x]] [[w]] [[A#^a]]\n\n``` [[A]]\n```\n"},
		{name: "empty destination base refuses", body: "[[.png#^a]]\n\n``` [[.png]]\n```\n"},
		{name: "owned block metadata remains whole", body: "[[A#^a]]\n\n``` [[A#^b]]\n```\n", want: "[[q#^a]]\n\n``` [[A#^b]]\n```\n", valid: true},
		{name: "owned local metadata remains whole", body: "[[A]]\n\n``` [[A]] [[#part]]\n```\n", want: "[[q]]\n\n``` [[A]] [[#part]]\n```\n", valid: true},
		{name: "owned nested word remains whole", body: "[[A]]\n\n``` [[A]]\n```\n\n[^n]: [[E[[F]]]]\n", want: "[[q]]\n\n``` [[A]]\n```\n\n[^n]: [[E[[F]]]]\n", valid: true},
		{name: "owned wrapped word remains whole", body: "[[A]]\n\n``` [[A]]\n[[B\nC]]\n```\n", want: "[[q]]\n\n``` [[A]]\n[[B\nC]]\n```\n", valid: true},
		{name: "owned backtick metadata remains whole", body: "[[A]]\n\n~~~ [[A]] [[B`C]]\n~~~\n", want: "[[q]]\n\n~~~ [[A]] [[B`C]]\n~~~\n", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reading := agreementOwnedCheckLiveReading(tc.body)
			original := agreementOwnedCheckLiveReading(tc.body)
			got, valid := agreementOwnedCheckLiveProjection(tc.body, &reading)
			if got != tc.want || valid != tc.valid {
				t.Fatalf("caught: owned check live projection target bytes %q %t, want %q %t", got, valid, tc.want, tc.valid)
			}
			if diff := cmp.Diff(original, reading); diff != "" {
				t.Fatalf("caught: owned check live projection changed source facts:\n%s", diff)
			}
			if valid {
				before, bv := agreementOwnedCheckLiveShapes(tc.body, &reading)
				after := agreementOwnedCheckLiveReading(got)
				afterOriginal := agreementOwnedCheckLiveReading(got)
				shapes, av := agreementOwnedCheckLiveShapes(got, &after)
				if !cmp.Equal(original, reading) || !cmp.Equal(afterOriginal, after) {
					t.Fatal("caught: owned check live shapes changed original source facts")
				}
				if !bv || !av || !cmp.Equal(before, shapes) {
					t.Fatal("caught: owned check live projection changed complete outside shape")
				}
				if len(got) != len(tc.body) {
					t.Fatal("caught: owned check live projection changed original byte positions")
				}
			}
		})
	}
}

func TestAgreementOwnedCheckLiveCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body  string
		want        map[string]int
		authorities map[string]string
	}{
		{name: "backtick metadata has only normalized check names", body: "[[A]]\n\n~~~ [[A]] [[B`C]]\n~~~\n", want: map[string]int{"A": 1, "B`C": 1}, authorities: map[string]string{"A": "#1011 stage 4", "B`C": "#1011 stage 4"}},
		{name: "raw suffix and normalized owned names remain separate", body: "[[A]]\n\n``` [[A#^a]]\n```\n\n[^n]: [[q\\]]\n", want: map[string]int{"A": 1, "q": 1}, authorities: map[string]string{"A": "#1011 stage 4", "q": "#1011 stage 5"}},
		{name: "owned block metadata", body: "[[A#^a]]\n\n``` [[A#^b]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "owned local metadata", body: "[[A]]\n\n``` [[A]] [[#part]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "whole discarded block destination", body: "[[A]]\n\n[^n]: [[A#^a]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "nested declaration has only check names", body: "[[A]]\n\n``` [[A]]\n```\n\n[^n]: [[E[[F]]]]\n", want: map[string]int{"A": 1, "E[[F": 1}, authorities: map[string]string{"A": "#1011 stage 4", "E[[F": "#1011 stage 5"}},
		{name: "wrapped literal has no invented occurrence", body: "[[A]]\n\n``` [[A]]\n[[B\nC]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "opaque inactive and contributing names", body: "[[A]] [[C]]\n\n``` [[A]] [[#part]]\n```\n\n[^n]: `[[C]]` [[B#^a]]\n", want: map[string]int{"A": 1, "B": 1}, authorities: map[string]string{"A": "#1011 stage 4", "B": "#1011 stage 5"}},
		{name: "no live carrier", body: "[[#part]]\n\n``` [[A#^a]]\n```\n", want: nil, authorities: nil},
		{name: "used definition retains live ownership", body: "ref[^n]\n\n[[A]]\n\n[^n]: [[A#^a]]\n", want: nil, authorities: nil},
		{name: "nested foreign field still refuses", body: "[[B[[A]]]]\n\n``` [[A#^a]]\n```\n", want: nil, authorities: nil},
		{name: "wrapped foreign field still refuses", body: "[[B\nA]]\n\n``` [[A#^a]]\n```\n", want: nil, authorities: nil},
		{name: "same name reference still refuses", body: "[r]: [[A]]\n\n[[A]]\n\n``` [[A#^a]]\n```\n", want: nil, authorities: nil},
		{name: "full original owned check live 0", body: "## A\n> [!note] title\nB\n-->> > [[B|alias]]- > [!note] title\n[[A]]![[A#A]]`A\n``` [[A]]\nÉ\n> [!unknown] title\n<!--## A\n## A\n## !\n~~~~\n## A-2\n## A\n## A\n![[A#A]]![[image.png]][[A#A]]``` [[A]]\n\n\n> [!unknown] title\n[[#A]][[A\nB]][[A#^a]]- [ ] [[A]]\n\t`", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 1", body: "text [[A]]## A\n## A\n[[A]]\\`## <em>A</em>\n# A\n- [x] [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n> [!note] title\nref[^n]\n## A\n ^a-2\n\t> [!unknown] title\n``` [[A]]\n~~~\n    ```\n[[A#^a]]## !\n-->   > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 2", body: "[[A#^a]]É\n  > [!note] title\n```` go [[A]]\n![[A#A]]0\n ^é\n## A-2\n## A\n## A\n```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[[#A]] ^é\n`open\n[[A]]\nclose`\n``` [[A]]\n- [ ] [[A]]\n\t```\n    A\n![[A#A]]\t\n\n``<!--\n[[A]]\n-->- [[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 3", body: "[[A#A]]https://example.invalid/`[[A]]` [[A#A]]\n\n`[[A]]````` go [[A]]\n## A-2\n## A\n## A\n[[B|alias]]ref[^n]\n## <em>A</em>\n- > [!note] title\n章節\n```` go [[A]]\n%%[[A]]%%[[#A]]> [!unknown] title\n[[A\\|alias]] ^A\n-->[[A]]\t```\n[^unused]: [[A]]\n\\[[A]]1. 章節\nref[^n]\n章節\n`````` go [[A]]\n\\[[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 4", body: "[^n]: [[A]]\n\n    [[B]]\n## A-2\n## A\n## A\n\n\n- [x] [[B]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n0> > ~~~\n````[[B|alias]]\\\\[[A]]  > [!note] title\n`open\n[[A]]\nclose`  > [!note] title\n[[A\nB]]A\n===\n`章節\n- > [!note] title\n[[A\\]][[B|alias]]ref[^n]\n\n\n  -   ```\nÉ\n  - \nA\n`[[A]]`", want: map[string]int{"A": 2, "B": 1}, authorities: map[string]string{"A": "#1011 stages 4 and 5", "B": "#1011 stage 4"}},
		{name: "full original owned check live 5", body: " 0  - A\n``` [[A]]\n[[B|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n ^A\n~~~\n\\[[A]]- > [!note] title\n> [!note] [[A]]\n    ```\n[[A\nB]]ref[^n]\n[^unused]: [[A]]\n```\n> [!note] one\n> [!note] two\n> [!note] three\n ^A\n\\\\[[A]][^n]: [[A]]\n\n    [[B]]\n<!--text ~~~~\n```` go [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 6", body: "A\n===\n[[#A]]<!-- [[A]] -->```` go [[A]]\nA\n---\n```` go [[A]]\n\t> [!unknown] title\n![[A#A]]## !\nref[^n]\n    `[[A]]`[[A\\]][[A\nB]][[A]] [[A]]0> [!unknown] title\ntext     [[B|alias]][[image.png]]## !\n`[[A]]`## A\n## A\n%%[[A]]%%<!-- [[A]] -->   ```\n    ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 7", body: "## !\n- item\n\n      \n>  ^A\n ^A\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/`[[A]]` `[[A]]`[[A\nB]][[A\\]]    ```\nhttps://example.invalid/`[[A]]` > [!unknown] title\n> [!note] title\n<!-- [[A]] -->\\\\[[A]]É\n## !\nA\n ^a\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original owned check live 8", body: "[[A\\|alias]]## A\n ^a\n0`[[A]]`0``` [[A]]\n![[A#A]]## A\n[[A]] [[A]]## <em>A</em>\n-->[[B|alias]]~~~~\nA\n[[A]] [[A]]- > [!note] title\n> [!note] one\n> [!note] two\n> [!note] three\n\\\\[[A]][^unused]: [[A]]\n%%[[A]]%%![[image.png]]> [!unknown] title\n```` go [[A]]\n[[#A]]É\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 9", body: "    ```\n\t  - <!-- [[A]] -->[[A#A]]    ```\n[^n]: [[A]]\n\n    [[B]]\n[[A#A]]^absent-prose %%[[A]]%%> [!unknown] title\n``` [[A]]\n[[A#^a]][[image.png]]`![[image.png]]-->[^n]: [[A]]\n\n    [[B]]\n ^a\nA\n===\n````\n[[B|alias]]ref[^n]\n0    ```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\t## [[A|alias]]\nA\n---\n章節\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 10", body: "``<!--\n[[A]]\n-->0  > [!note] title\n## [[A|alias]]\n```` go [[A]]\n[[A]] [[A]]  ```\n    [[A\nB]]<!-- [[A]] -->\n\nref[^n]\n> [!note] [[A]]\n``> [!note] [[A]]\n- [ ] [[A]]\n[[B|alias]]``\n^absent-prose  ```\nref[^n]\n  - - > [!note] title\n    ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 11", body: "[[A\\|alias]]![[A]]> > [[B|alias]][^unused]: [[A]]\n%%<!-- [[A]] --><!-- [[A]] -->text ````\n%%- \n\n\\[[A]] ^a\n``` [[A]]\n- > [!note] title\n%%[[A]]%%[[A\nB]]A\n---\n## <em>A</em>\n> [!note] title\n[[A]] [[A]]## [[A|alias]]\n1. ## A\n## A\n ^é\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 12", body: "<div>\n[[A]]\n</div>\n> A\n ```\n0- item\n\n      - [ ] [[A]]\n%%[[A]]%%<!-- [[A]] -->\n\n``` [[A]]\n ^A\n![[A#A]]````\n> ## A\n## A\n\n[[A\nB]]> > \t![[image.png]]\\[[A]]-->    ```\n  ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 13", body: "\\`![[A#A]][^unused]: [[A]]\n- [x] [[B]]\n[[A]]## [[A|alias]]\n## <em>A</em>\n> [!note] title\n\\[[A]]  - > [!note] [[A]]\n``` [[A]]\n[[A]]-->- > [!note] title\n- > [!note] title\n![[image.png]]É\n`open\n[[A]]\nclose`![[A]][[B|alias]] ^a\n`[[A]]` ^A\nref[^n]\n- item\n\n      É\nA\n===\n## A-2\n## A\n## A\n<!--[[#A]]https://example.invalid/`[[A]]` ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 14", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n  > [!note] title\n[^n]: [[A]]\n\n    [[B]]\nhttps://example.invalid/`[[A]]`  ## [[A|alias]]\n [[A#^a]]\n\n# A\n ^A\n ^a-2\nA\n---\n", want: map[string]int{"A": 3}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original owned check live 15", body: "[^n]: [[A]]\n\n    [[B]]\n## !\n\\\\[[A]]É\n![[image.png]]É\n ^a\n    ```\n## A-2\n## A\n## A\n![[A#A]]- > [!note] title\nB\n[[A#A]]# A\n````` [[A]]\nB\nhttps://example.invalid/`[[A]]`  ^a-2\n  ```\n\n[[B|alias]]0 ```` go [[A]]\n[[A#^a]]## A\n## A\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 16", body: "[^unused]: [[A]]\n[[A\\|alias]]## [[A|alias]]\n``` [[A]]\n```\nA\n---\n-->`[[A]]````` go [[A]]\n<!-- [[A]] -->0章節\n章節\n  ```\n## A-2\n## A\n## A\n ^é\n-    ```\nA\n===\nA\n---\n## A-2\n## A\n## A\n1. > ref[^n]\nB\n1. \n\n B\n- item\n\n      [[#A]]", want: map[string]int{"A": 4}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 17", body: "\\\\[[A]]--><!--\n[[A]]\n-->## A-2\n## A\n## A\n1. text [[#A]]~~~~\n[[A\\|alias]]| a | b |\n|---|---|\n| [[A]] | ^a |\n[^n]: [[A]]\n\n    [[B]]\n![[image.png]]https://example.invalid/`[[A]]` ```\n[[A\nB]]  > [!note] title\n- > [!note] title\n![[A]]  - \nA\n[[A]] [[A]] ^é\n[[A\\]]A\n===\n\\````` go [[A]]\n\t```\n## A\n", want: map[string]int{"B": 1, "image.png": 1}, authorities: map[string]string{"B": "#1011 stage 5", "image.png": "#1011 stage 5"}},
		{name: "full original owned check live 18", body: "> > - > [!note] title\n  - > [!note] [[A]]\n``` [[A]]\n[[A#^a]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 19", body: "É\n``` [[A]]\n\n\n0[[#A]]- ## !\n[[A#A]]章節\n> >  ^A\n[[A#^a]]É\n   ```\n``0    É\n- item\n\n      [[#A]]^absent-prose     `%%\\`   ```\n[[image.png]]  ```\n[^unused]: [[A]]\n```\n- item\n\n      ", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 20", body: "> [!note] one\n> [!note] two\n> [!note] three\n- > [!note] title\n\t```\n> > [^n]: [[A]]\n\n    [[B]]\ntext ``\n\\[[A]]\\`\t## [[A|alias]]\n## A\nÉ\n``` [[A]]\n`   ```\n[[A#^a]] ^A\n## A\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 21", body: "[^unused]: [[A]]\n\n- - [ ] [[A]]\n ^A\n```` go [[A]]\n1. text `[[A]]`- item\n\n       ```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- [[A]] [[A]]A\n---\n   ```\n[[A\\|alias]]## A\n    [[#A]]`[^n]: [[A]]\n\n    [[B]]\n\t```\nA\n## <em>A</em>\n   ```\n## [[A|alias]]\nhttps://example.invalid/`[[A]]` ``## <em>A</em>\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 22", body: "[[A]] [[A]]0![[image.png]]%%[[A]]%%[[A\\|alias]]~~~~\n ```` go [[A]]\n    <div>\n[[A]]\n</div>\n```\n- [x] [[B]]\n> > \\`%% ^a-2\n`[[A]]````\n> [!unknown] title\n\\\\[[A]][[#A]][[A\nB]]text ## !\nÉ\nB\n```` go [[A]]\n[[A]]- > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 23", body: "``\\\\[[A]]- [x] [[B]]\n> [!note] title\n> ^absent-prose  ^a\n\n\n`[[A]]`0![[image.png]] ```\n- item\n\n      [[A#^a]]  > [!note] title\n``[[A]]  ```\n``` [[A]]\n ^a-2\n0## [[A|alias]]\n[[A\\]] ^é\n<!--\n[[A]]\n-->## [[A|alias]]\n  ```\n ```\n[[B|alias]]\t```\n![[image.png]]    ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 24", body: "    <!-- [[A]] -->````\n[[A#^a]]A\n===\n text text A\n[[A\\|alias]] ^é\nB\n-->    ```\n[[A\nB]]   ```\n\n\nref[^n]\n  > [!note] title\n``` [[A]]\n![[image.png]]```` go [[A]]\n[[A#^a]]    A\n===\n````\n![[image.png]][[image.png]]````\n\\\\[[A]]\t", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 25", body: "  - 0- > [!note] title\nref[^n]\n> - [x] [[B]]\n## A\n## A\n## A\n`````` go [[A]]\n ^a-2\n[^n]: [[A]]\n\n    [[B]]\n```` go [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n[[image.png]]    <div>\n[[A]]\n</div>\n^absent-prose <!-- [[A]] --><div>\n[[A]]\n</div>\n0- É\n> [!note] one\n> [!note] two\n> [!note] three\n## !\n[[A\nB]]- ![[image.png]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 26", body: "| a | b |\n|---|---|\n| [[A]] | ^a |\n[^unused]: [[A]]\n\\[[A]]A\n``` [[A]]\n> [!note] title\n`open\n[[A]]\nclose` ^é\n> [!note] [[A]]\n# A\n  ^é\nref[^n]\n%%[[A]]%%`open\n[[A]]\nclose`[[#A]]![[image.png]]## !\n[[A\\|alias]]## !\n## A\n## A\nA\n---\n\t## <em>A</em>\n ^a-2\n ^é\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 27", body: "章節\n[[image.png]]## A\n## A\n--> ^A\n  > [!note] title\n[[A\\|alias]]ref[^n]\n```` go [[A]]\n ^é\n[[A#^a]] ^é\n\t## A\n## A\n![[A]]%%> >   - \\[[A]]![[A]] <div>\n[[A]]\n</div>\n[^unused]: [[A]]\n   ```\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\n[[#A]]  > [!note] title\n```", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 28", body: "[[image.png]]  - 1. ## [[A|alias]]\n[^unused]: [[A]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n\\[[A]]  - `[[A]]`1. ## !\n  - 章節\n## A-2\n## A\n## A\n> [!note] one\n> [!note] two\n> [!note] three\n ^A\n![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n    ```\n  > [!note] title\n ^A\n   ```\n0``` [[A]]\n    ```\n[[#A]]<!-- [[A]] -->\\[[A]]```\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original owned check live 29", body: "\\`![[A]]## A\n## A\n[^unused]: [[A]]\n## <em>A</em>\n ^A\nA\n===\n## A-2\n## A\n## A\n```` go [[A]]\n[[A#^a]]A\n- item\n\n      [[#A]]- <!--\n[[A]]\n-->- [x] [[B]]\n[[A#A]]ref[^n]\n<!-- [[A]] -->0 ^é\n  ```\n[[A]] [[A]][[B|alias]]   ```\n0`[[A]]`[^n]: [[A]]\n\n    [[B]]\n````\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 30", body: "\t[[A]] [[A]]````\n\\[[A]]A\n- [ ] [[A]]\n章節\n[^n]: [[A]]\n\n    [[B]]\nA\nÉ\n`[[A]]`[[#A]]````\n- [ ] [[A]]\nÉ\n\\\\[[A]]<!-- [[A]] --><!-- [[A]] -->  ```\n- [ ] [[A]]\n- [x] [[B]]\n\\[[A]]![[image.png]]  > [!note] title\n0", want: map[string]int{"A": 1, "B": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5"}},
		{name: "full original owned check live 31", body: "章節\nA\n   ```\n\\`![[A#A]]  - ## [[A|alias]]\n## A-2\n## A\n## A\n0![[A]]<div>\n[[A]]\n</div>\n  ```\n\\[[A]]## [[A|alias]]\n- [ ] [[A]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n[^unused]: [[A]]\n[[#A]]- [ ] [[A]]\n ^a-2\n ^é\n\t## [[A|alias]]\n## A-2\n## A\n## A\n``ref[^n]\n> > ````\n", want: map[string]int{"A": 3}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original owned check live 32", body: "É\n[[B|alias]]``` [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n`````` go [[A]]\n  > [!note] title\n> [!note] title\n> > 00  > [!note] title\n<!--\n[[A]]\n-->[[A#A]]`[[A]]`[^n]: [[A]]\n\n    [[B]]\n[[#A]][[A\\|alias]]## [[A|alias]]\n<!--\n[[A]]\n-->\\\\[[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 33", body: " ^é\n ^é\n\\\\[[A]]## <em>A</em>\n- > [!note] title\n```` go [[A]]\n[[A#^a]][[A#A]]~~~~\n>  ^é\nA\n## A\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 34", body: "A\n---\n| a | b |\n|---|---|\n| [[A]] | ^a |\n- [x] [[B]]\n<!-- [[A]] -->  > [!note] title\n``` [[A]]\n \\\\[[A]][[A\\|alias]] ```\nA\n- [x] [[B]]\n ^é\n\\\\[[A]]0[[A]] [[A]]- [x] [[B]]\ntext É\n[[image.png]][[A\nB]][[A#^a]]- A\n---\n ^é\n%%[[A]]%%- > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 35", body: "É\nA\n---\n# A\nref[^n]\n- [ ] [[A]]\ntext B\n  > [!note] title\n  > [!note] title\n> [!unknown] title\n## <em>A</em>\n- [ ] [[A]]\n ^A\nB\n0> [!unknown] title\n``` [[A]]\n[[#A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 36", body: "## !\n[[A#A]] ^a-2\n## <em>A</em>\n\n\n  > [!note] title\n``` [[A]]\n\n\ntext %%[[A]]%%> [!note] [[A]]\n~~~\n`open\n[[A]]\nclose`- [ ] [[A]]\n ^A\n## A-2\n## A\n## A\n<!-- [[A]] -->`open\n[[A]]\nclose`1. <!-- [[A]] -->A\n===\n> 章節\n`open\n[[A]]\nclose`[[A#^a]] ^a\n%%[[A]]%%", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 37", body: "-->| a | b |\n|---|---|\n| [[A]] | ^a |\n[^unused]: [[A]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n``` [[A]]\nA\n---\n[[A#A]] ^a\n- item\n\n      [[#A]]", want: map[string]int{"A": 3}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original owned check live 38", body: "> [!note] title\n- > [!note] title\n[[#A]] ^A\n\\\\[[A]]-->```` go [[A]]\n``` [[A]]\n[[image.png]]\\[[A]]-  ^é\n ^é\n``<div>\n[[A]]\n</div>\n> > ![[A]]## A-2\n## A\n## A\n[[#A]]\\`^absent-prose [[A#^a]]A\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original owned check live 39", body: "# A\n> [!unknown] title\n![[image.png]] ```\n```` go [[A]]\n`\n[[B|alias]]![[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n  ```\n![[image.png]]text [[A#^a]]\\\\[[A]]\n![[A]]> [!unknown] title\n- > [!note] title\n\\\\[[A]][[A]]- [x] [[B]]\ntext     [[A\\]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementOwnedCheckLiveBudget(t, c.Body, &actual)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementNativeLiveDifference(c, &f, &actual, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatalf("caught: owned check live borrowed unowned public difference %s", agreementSignature(&f))
					}
					continue
				}
				if kind != "debt" || wrong != "judge" || authority != tc.authorities[f.Tuple.Target] {
					t.Fatalf("caught: owned check live public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementNativeLiveDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete owned check live public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
func TestAgreementOwnedCheckLiveEvidence(t *testing.T) {
	t.Parallel()
	body := "[[#place]] [[A#^a|shown]] [[B]]\n\n``` [[A#^b]]\n```\n\n[^n]: [[B]] `[[C]]` [[#local]]\n"
	reading := agreementOwnedCheckLiveReading(body)
	projected, valid := agreementOwnedCheckLiveProjection(body, &reading)
	if !valid {
		t.Fatal("caught: owned check live producer fixture has no projection")
	}
	after := agreementOwnedCheckLiveReading(projected)
	_, actual := agreementIsolatedPage(t, agreementCase{Body: body})
	_, witness := agreementIsolatedPage(t, agreementCase{Body: projected})
	native := agreementNativeCheckBudget(body)
	baseline := agreementOwnedCheckLiveEvidence(body, projected, &reading, &after, &actual, &witness, native)
	if diff := cmp.Diff(native, baseline.Native); diff != "" {
		t.Fatalf("caught: owned check live evidence lost whole check partition:\n%s", diff)
	}
	if diff := cmp.Diff(map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1}, baseline.Carriers); diff != "" {
		t.Fatalf("caught: owned check live evidence lost original whole page:\n%s", diff)
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementOwnedCheckLiveSource)
	}{
		{"whole native definitions", func(s *agreementOwnedCheckLiveSource) { s.Native.Definitions = nil }},
		{"native definition label", func(s *agreementOwnedCheckLiveSource) { s.Native.Definitions[0].Label = "other" }},
		{"native definition span", func(s *agreementOwnedCheckLiveSource) { s.Native.Definitions[0].Span.Stop-- }},
		{"native definition nodes", func(s *agreementOwnedCheckLiveSource) { s.Native.Definitions[0].Nodes = nil }},
		{"whole native code", func(s *agreementOwnedCheckLiveSource) { s.Native.Code = nil }},
		{"native code kind", func(s *agreementOwnedCheckLiveSource) { s.Native.Code[0].Kind = "other" }},
		{"native code parent", func(s *agreementOwnedCheckLiveSource) { s.Native.Code[0].Parent = "other" }},
		{"native code span", func(s *agreementOwnedCheckLiveSource) { s.Native.Code[0].Span.Start++ }},
		{"whole native comments", func(s *agreementOwnedCheckLiveSource) { s.Native.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"whole check code", func(s *agreementOwnedCheckLiveSource) { s.Check.Code = nil }},
		{"whole check comments", func(s *agreementOwnedCheckLiveSource) { s.Check.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"every owned field", func(s *agreementOwnedCheckLiveSource) { s.Owned = s.Owned[:1] }},
		{"owned raw word", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].Word = "other" }},
		{"owned position", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].Span.Start++ }},
		{"owned normalized names", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].Targets = nil }},
		{"owned code flag", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].Code = true }},
		{"owned comment flag", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].Comment = true }},
		{"owned escape flag", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].Escaped = true }},
		{"owned definition index", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].DefinitionOwner = 0 }},
		{"owned code index", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].CodeOwner = -1 }},
		{"coherent owned code flag", func(s *agreementOwnedCheckLiveSource) {
			s.Owned[0].Code = true
			for i := range s.Check.Fields {
				if s.Check.Fields[i].Span == s.Owned[0].Span {
					s.Check.Fields[i] = s.Owned[0]
				}
			}
		}},
		{"coherent owned comment flag", func(s *agreementOwnedCheckLiveSource) {
			s.Owned[0].Comment = true
			for i := range s.Check.Fields {
				if s.Check.Fields[i].Span == s.Owned[0].Span {
					s.Check.Fields[i] = s.Owned[0]
				}
			}
		}},
		{"coherent owned escape flag", func(s *agreementOwnedCheckLiveSource) {
			s.Owned[0].Escaped = true
			for i := range s.Check.Fields {
				if s.Check.Fields[i].Span == s.Owned[0].Span {
					s.Check.Fields[i] = s.Owned[0]
				}
			}
		}},

		{"whole canonical check fields", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields = nil }},
		{"truncated canonical check fields", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields = s.Check.Fields[:1] }},
		{"canonical check word", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields[0].Word = "other" }},
		{"canonical check targets", func(s *agreementOwnedCheckLiveSource) { s.Check.Targets = nil }},
		{"extra owned field", func(s *agreementOwnedCheckLiveSource) { s.Owned = append(s.Owned, s.Owned[0]) }},
		{"extra outside field", func(s *agreementOwnedCheckLiveSource) { s.Outside = append(s.Outside, s.Outside[0]) }},
		{"coherent owned word still needs original bytes", func(s *agreementOwnedCheckLiveSource) {
			s.Owned[0].Word = strings.Replace(s.Owned[0].Word, "A", "Z", 1)
			s.Owned[0].Targets = []string{"Z"}
			for i := range s.Check.Fields {
				if s.Check.Fields[i].Span == s.Owned[0].Span {
					s.Check.Fields[i] = s.Owned[0]
				}
			}
		}},
		{"whole protected namespace", func(s *agreementOwnedCheckLiveSource) { s.Names = map[string]bool{"unowned": true} }},
		{"whole outside set", func(s *agreementOwnedCheckLiveSource) { s.Outside = nil }},
		{"every outside field", func(s *agreementOwnedCheckLiveSource) { s.Outside = s.Outside[:1] }},
		{"ordered outside set", func(s *agreementOwnedCheckLiveSource) { s.Outside[1], s.Outside[2] = s.Outside[2], s.Outside[1] }},
		{"outside source start", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.Span.Start++ }},
		{"outside source stop", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.Span.Stop-- }},
		{"outside raw word", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.Word = "other" }},
		{"outside coherent spelling needs original window", func(s *agreementOwnedCheckLiveSource) {
			s.Outside[1].Source.Word = strings.Replace(s.Outside[1].Source.Word, "q", "z", 1)
			s.Outside[1].Source.Targets = []string{"z"}
			s.Outside[1].Tuple.Target = "z"
		}},
		{"outside normalized names", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.Targets = []string{"other"} }},
		{"outside normalized count", func(s *agreementOwnedCheckLiveSource) {
			s.Outside[1].Source.Targets = append(s.Outside[1].Source.Targets, "other")
		}},
		{"outside check code role", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.Code = true }},
		{"outside check comment role", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.Comment = true }},
		{"outside escape role", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.Escaped = true }},
		{"outside definition owner", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.DefinitionOwner = 0 }},
		{"outside code owner", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Source.CodeOwner = 0 }},
		{"outside tuple target", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Tuple.Target = "other" }},
		{"outside tuple source role", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Tuple.SourceRole = "other" }},
		{"outside tuple section", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Tuple.Section = "other" }},
		{"outside tuple state", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Tuple.State = "other" }},
		{"outside block fragment", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Block = "other" }},
		{"outside note role", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Cites = false }},
		{"outside local role", func(s *agreementOwnedCheckLiveSource) { s.Outside[0].Cites = true }},

		{"coherent outside code role", func(s *agreementOwnedCheckLiveSource) {
			s.Outside[1].Source.Code = true
			for i := range s.Check.Fields {
				if s.Check.Fields[i].Span == s.Outside[1].Source.Span {
					s.Check.Fields[i] = s.Outside[1].Source
				}
			}
		}},
		{"coherent outside comment role", func(s *agreementOwnedCheckLiveSource) {
			s.Outside[1].Source.Comment = true
			for i := range s.Check.Fields {
				if s.Check.Fields[i].Span == s.Outside[1].Source.Span {
					s.Check.Fields[i] = s.Outside[1].Source
				}
			}
		}},
		{"coherent outside escape role", func(s *agreementOwnedCheckLiveSource) {
			s.Outside[1].Source.Escaped = true
			for i := range s.Check.Fields {
				if s.Check.Fields[i].Span == s.Outside[1].Source.Span {
					s.Check.Fields[i] = s.Outside[1].Source
				}
			}
		}},
		{"outside page comment role", func(s *agreementOwnedCheckLiveSource) { s.Outside[1].Comment = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementOwnedCheckLiveReading(projected)
			tc.change(&changed)
			if len(agreementOwnedCheckLiveEvidence(body, projected, &reading, &changed, &actual, &witness, native).Native.Citation.Targets) != 0 {
				t.Fatal("caught: owned check live producer borrowed changed source context")
			}
		})
	}
	for _, owner := range []string{"A", "B", "C"} {
		t.Run("whole namespace "+owner, func(t *testing.T) {
			t.Parallel()
			changed := agreementOwnedCheckLiveReading(projected)
			other := witness
			other.Citations = append(append([]agreementCitation{}, witness.Citations...), agreementCitation{Target: owner, State: "wikilink-broken"})
			changed.Page = agreementNativeLiveCarriers(&other)
			if len(agreementOwnedCheckLiveEvidence(body, projected, &reading, &changed, &actual, &other, native).Native.Citation.Targets) != 0 {
				t.Fatal("caught: owned check live producer borrowed owned page occurrence")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementOwnedCheckLiveSource)
	}{
		{"missing original check fields", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields = nil }},
		{"truncated original check fields", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields = s.Check.Fields[:1] }},
		{"changed original whole check targets", func(s *agreementOwnedCheckLiveSource) { s.Check.Targets = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementOwnedCheckLiveReading(body)
			tc.change(&changed)
			if len(agreementOwnedCheckLiveEvidence(body, projected, &changed, &after, &actual, &witness, native).Native.Citation.Targets) != 0 {
				t.Fatal("caught: owned check live producer borrowed changed original check ledger")
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
				if len(agreementOwnedCheckLiveEvidence(body, projected, &reading, &after, &a, &w, native).Native.Citation.Targets) != 0 {
					t.Fatal("caught: owned check live producer borrowed changed whole page")
				}
			})
		}
	}
}

func TestAgreementOwnedCheckLiveShapes(t *testing.T) {
	t.Parallel()
	body := "[[#part]] [[A#^a|shown]] [[B`C]] \\[[D]] %%[[E]]%%\n\n``` [[A]]\n```\n"
	field := func(start, stop int, word string, targets []string, escaped, comment bool) agreementNativeCheckField {
		return agreementNativeCheckField{Span: graph.Span{Start: start, Stop: stop}, Word: word, Targets: targets, Escaped: escaped, Comment: comment, DefinitionOwner: -1, CodeOwner: -1}
	}
	want := []agreementDestinationLiveField{
		{Source: field(0, 9, "[[#part]]", []string{}, false, false), Tuple: agreementCitation{Section: "part", State: "wikilink-broken"}},
		{Source: field(10, 24, "[[?#^a|shown]]", []string{"?"}, false, false), Tuple: agreementCitation{Target: "?", State: "wikilink-broken"}, Block: "a", Cites: true},
		{Source: field(25, 32, "[[???]]", []string{"?"}, false, false), Tuple: agreementCitation{Target: "???", State: "wikilink-broken"}, Cites: true},
		{Source: field(34, 39, "[[?]]", []string{"?"}, true, false), Tuple: agreementCitation{Target: "?", State: "wikilink-broken"}, Cites: true},
		{Source: field(42, 47, "[[?]]", []string{"?"}, false, true), Tuple: agreementCitation{Target: "?", State: "wikilink-broken"}, Cites: true, Comment: true},
	}
	reading := agreementOwnedCheckLiveReading(body)
	original := agreementOwnedCheckLiveReading(body)
	got, valid := agreementOwnedCheckLiveShapes(body, &reading)
	if !valid {
		t.Fatal("caught: owned check live full shape lacks source binding")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("caught: complete owned check live outside shapes (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original, reading); diff != "" {
		t.Fatalf("caught: owned check live shapes changed source facts:\n%s", diff)
	}
}

func TestAgreementOwnedCheckLiveLedger(t *testing.T) {
	t.Parallel()
	body := "[[A]] [[B]]\n\n``` [[A#^a]] [[#part]]\n```\n\n[^n]: [[B]] `[[C]]`\n"
	reading := agreementOwnedCheckLiveReading(body)
	if !agreementOwnedCheckLiveLedger(body, &reading) {
		t.Fatal("caught: owned check live ledger fixture has no complete public binding")
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementOwnedCheckLiveSource)
	}{
		{"missing check fields", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields = nil }},
		{"truncated check fields", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields = s.Check.Fields[:1] }},
		{"extra check field", func(s *agreementOwnedCheckLiveSource) { s.Check.Fields = append(s.Check.Fields, s.Check.Fields[0]) }},
		{"missing owned fields", func(s *agreementOwnedCheckLiveSource) { s.Owned = nil }},
		{"extra owned field", func(s *agreementOwnedCheckLiveSource) { s.Owned = append(s.Owned, s.Owned[0]) }},
		{"reordered owned fields", func(s *agreementOwnedCheckLiveSource) { s.Owned[0], s.Owned[1] = s.Owned[1], s.Owned[0] }},
		{"missing outside fields", func(s *agreementOwnedCheckLiveSource) { s.Outside = nil }},
		{"extra outside field", func(s *agreementOwnedCheckLiveSource) { s.Outside = append(s.Outside, s.Outside[0]) }},
		{"reordered outside fields", func(s *agreementOwnedCheckLiveSource) { s.Outside[0], s.Outside[1] = s.Outside[1], s.Outside[0] }},
		{"whole public targets", func(s *agreementOwnedCheckLiveSource) { s.Check.Targets = nil }},
		{"public target count", func(s *agreementOwnedCheckLiveSource) { s.Check.Targets = s.Check.Targets[:1] }},
		{"public target member", func(s *agreementOwnedCheckLiveSource) { s.Check.Targets[0] = "foreign" }},
		{"coherent raw word needs body bytes", func(s *agreementOwnedCheckLiveSource) {
			s.Check.Fields[0].Word = "[[Z]]"
			s.Check.Fields[0].Targets = []string{"Z"}
			s.Outside[0].Source = s.Check.Fields[0]
		}},
		{"coherent normalized member needs public word", func(s *agreementOwnedCheckLiveSource) {
			s.Check.Fields[0].Targets = []string{"foreign"}
			s.Outside[0].Source = s.Check.Fields[0]
		}},
		{"coherent normalized count needs public word", func(s *agreementOwnedCheckLiveSource) {
			s.Check.Fields[0].Targets = append(s.Check.Fields[0].Targets, "A")
			s.Outside[0].Source = s.Check.Fields[0]
		}},
		{"owned partition member", func(s *agreementOwnedCheckLiveSource) { s.Owned[0].CodeOwner = -1 }},
		{"outside partition member", func(s *agreementOwnedCheckLiveSource) { s.Outside[0].Source.DefinitionOwner = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementOwnedCheckLiveReading(body)
			tc.change(&changed)
			if agreementOwnedCheckLiveLedger(body, &changed) {
				t.Fatal("caught: owned check live ledger borrowed incomplete or changed public fields")
			}
		})
	}
}

func TestAgreementOwnedCheckLiveKeepsDiagnosticRefusals(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"[[A]]\n\n``` [[A#^a]]\n```\n",
		"[[A]]\n\n``` [[A]] [[#part]]\n```\n",
		"[[A]]\n\n~~~ [[A]] [[B`C]]\n~~~\n",
		"[[A]]\n\n``` [[A]]\n```\n\n[^n]: [[E[[F]]]]\n",
		"[[A]]\n\n``` [[A]]\n[[B\nC]]\n```\n",
	} {
		reading := agreementOwnedCheckLiveReading(body)
		if len(reading.Owned) == 0 {
			t.Fatal("caught: opaque owned check field lost its complete declaration")
		}
		if len(agreementNativeDiagnosticReading(body).Fields) != 0 || len(agreementDestinationLiveReading(body).Owned.Fields) != 0 {
			t.Fatal("caught: owned check live field borrowed diagnostic tuple scope")
		}
	}
}
