package judge_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

type agreementDestinationLiveField struct {
	Source         agreementNativeCheckField
	Tuple          agreementCitation
	Block          string
	Cites, Comment bool
}

type agreementDestinationLiveSource struct {
	Owned   agreementNativeDiagnosticSource
	Outside []agreementDestinationLiveField
	Names   map[string]bool
	Page    map[agreementCitation]int
}

// Local and block fragments keep their declarations even when they contribute
// no note name. Only a complete public outside set can separate live words
// from code metadata and discarded declarations with the same destination.
func agreementDestinationLiveReading(body string) agreementDestinationLiveSource {
	owned := agreementNativeDiagnosticReading(body)
	if len(owned.Fields) == 0 {
		return agreementDestinationLiveSource{}
	}
	reading := agreementDestinationLiveSource{Owned: owned, Names: make(map[string]bool), Page: make(map[agreementCitation]int)}
	for _, field := range owned.Check.Fields {
		inner := strings.TrimSuffix(strings.TrimPrefix(field.Word, "[["), "]]")
		link, cites := graph.ParseWikilink(inner)
		if field.DefinitionOwner >= 0 || field.CodeOwner >= 0 {
			reading.Names[link.Target] = true
			for _, target := range field.Targets {
				reading.Names[target] = true
			}
			continue
		}
		if strings.ContainsAny(inner, "[]\r\n") {
			return agreementDestinationLiveSource{}
		}
		outside := agreementDestinationLiveField{Source: field, Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}, Block: link.Block, Cites: cites, Comment: graph.In(owned.Page.Comments, field.Span.Start)}
		reading.Outside = append(reading.Outside, outside)
		if outside.Cites && !outside.Source.Escaped && !outside.Comment {
			reading.Page[outside.Tuple]++
		}
	}
	return reading
}

// The target is the only changing slot. Preserve every other byte and role,
// and bind each normalized list to the public check of its original raw word.
func agreementDestinationLiveShapes(body string, reading *agreementDestinationLiveSource) ([]agreementDestinationLiveField, bool) {
	shapes := make([]agreementDestinationLiveField, 0, len(reading.Outside))
	for index := range reading.Outside {
		field := reading.Outside[index]
		start, stop := field.Source.Span.Start, field.Source.Span.Stop
		if start < 0 || stop > len(body) || start > stop || body[start:stop] != field.Source.Word || !cmp.Equal(field.Source.Targets, judge.LinkTargets(field.Source.Word)) {
			return nil, false
		}
		if field.Cites {
			inner := field.Source.Word[2 : len(field.Source.Word)-2]
			target := field.Tuple.Target
			first := 2 + strings.Index(inner, target)
			last := first + len(target)
			if target == "" || first < 2 || last > len(field.Source.Word)-2 {
				return nil, false
			}
			masked := strings.Repeat("?", len(target))
			field.Source.Word = field.Source.Word[:first] + masked + field.Source.Word[last:]
			field.Tuple.Target = masked
		}
		field.Source.Targets = make([]string, len(field.Source.Targets))
		for i := range field.Source.Targets {
			field.Source.Targets[i] = "?"
		}
		shapes = append(shapes, field)
	}
	return shapes, true
}

func agreementDestinationLiveProjection(body string, reading *agreementDestinationLiveSource) (string, bool) {
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
	for index := range reading.Outside {
		field := &reading.Outside[index]
		target := field.Tuple.Target
		if !field.Cites || !reading.Names[target] {
			continue
		}
		inner := field.Source.Word[2 : len(field.Source.Word)-2]
		start := field.Source.Span.Start + 2 + strings.Index(inner, target)
		stop := start + len(target)
		if start < field.Source.Span.Start+2 || stop > field.Source.Span.Stop-2 || body[start:stop] != target {
			return "", false
		}
		base, extension := target, ""
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

func agreementDestinationLiveBudget(t *testing.T, body string, actual *agreementHTML) agreementNativeLivePayload {
	t.Helper()
	native := agreementNativeCheckBudget(body)
	if len(native.Citation.Targets) == 0 || len(actual.Citations) == 0 {
		return agreementNativeLivePayload{}
	}
	reading := agreementDestinationLiveReading(body)
	if len(reading.Owned.Fields) == 0 || !cmp.Equal(reading.Page, agreementNativeLiveCarriers(actual)) {
		return agreementNativeLivePayload{}
	}
	projected, valid := agreementDestinationLiveProjection(body, &reading)
	if !valid {
		return agreementNativeLivePayload{}
	}
	after := agreementDestinationLiveReading(projected)
	_, witness := agreementIsolatedPage(t, agreementCase{Body: projected})
	return agreementDestinationLiveEvidence(body, projected, &reading, &after, actual, &witness, native)
}

func agreementDestinationLiveEvidence(body, projected string, reading, after *agreementDestinationLiveSource, actual, witness *agreementHTML, native agreementNativeCheckPayload) agreementNativeLivePayload {
	if !cmp.Equal(reading.Owned.Page, after.Owned.Page) || !cmp.Equal(reading.Owned.Check.Code, after.Owned.Check.Code) || !cmp.Equal(reading.Owned.Check.Comments, after.Owned.Check.Comments) || !cmp.Equal(reading.Owned.Fields, after.Owned.Fields) || !cmp.Equal(reading.Names, after.Names) {
		return agreementNativeLivePayload{}
	}
	beforeShapes, beforeValid := agreementDestinationLiveShapes(body, reading)
	afterShapes, afterValid := agreementDestinationLiveShapes(projected, after)
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

func TestAgreementDestinationLiveSource(t *testing.T) {
	t.Parallel()
	field := func(start, stop int, word string, targets []string, escaped, comment bool) agreementNativeCheckField {
		return agreementNativeCheckField{Span: graph.Span{Start: start, Stop: stop}, Word: word, Targets: targets, Escaped: escaped, Comment: comment, DefinitionOwner: -1, CodeOwner: -1}
	}
	for _, tc := range []struct {
		name, body string
		outside    []agreementDestinationLiveField
		names      map[string]bool
		page       map[agreementCitation]int
	}{
		{name: "complete local block literal and inactive source", body: "[[#part]] [[A#^a|shown]] [[B`C]] \\[[D]] %%[[E]]%%\n\n``` [[A]]\n```\n", outside: []agreementDestinationLiveField{
			{Source: field(0, 9, "[[#part]]", []string{}, false, false), Tuple: agreementCitation{Section: "part", State: "wikilink-broken"}},
			{Source: field(10, 24, "[[A#^a|shown]]", []string{"A"}, false, false), Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, Block: "a", Cites: true},
			{Source: field(25, 32, "[[B`C]]", []string{"B`C"}, false, false), Tuple: agreementCitation{Target: "B`C", State: "wikilink-broken"}, Cites: true},
			{Source: field(34, 39, "[[D]]", []string{"D"}, true, false), Tuple: agreementCitation{Target: "D", State: "wikilink-broken"}, Cites: true},
			{Source: field(42, 47, "[[E]]", []string{"E"}, false, true), Tuple: agreementCitation{Target: "E", State: "wikilink-broken"}, Cites: true, Comment: true},
		}, names: map[string]bool{"A": true}, page: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B`C", State: "wikilink-broken"}: 1}},
		{name: "all raw normalized inactive owner names", body: "[[A#^a]]\n\n``` [[A\\]]\n```\n\n[^n]: `[[C]]`\n", outside: []agreementDestinationLiveField{{Source: field(0, 8, "[[A#^a]]", []string{"A"}, false, false), Tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, Block: "a", Cites: true}}, names: map[string]bool{"A": true, "A\\": true, "C": true}, page: map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1}},
		{name: "no owner", body: "[[A#^a]]\n"},
		{name: "nested outside frame refuses", body: "[[B[[A]]]]\n\n``` [[A]]\n```\n"},
		{name: "wrapped outside frame refuses", body: "[[B\nA]]\n\n``` [[A]]\n```\n"},
		{name: "local owner remains refused", body: "[[B]]\n\n[^n]: [[#A]]\n"},
		{name: "block owner remains refused", body: "[[B]]\n\n[^n]: [[A#^a]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := agreementDestinationLiveReading(tc.body)
			if diff := cmp.Diff(tc.outside, got.Outside); diff != "" {
				t.Fatalf("caught: complete destination live outside source (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.names, got.Names); diff != "" {
				t.Fatalf("caught: complete destination live owner namespace (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.page, got.Page); diff != "" {
				t.Fatalf("caught: complete destination live outside tuples (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAgreementDestinationLiveProjection(t *testing.T) {
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reading := agreementDestinationLiveReading(tc.body)
			original := agreementDestinationLiveReading(tc.body)
			got, valid := agreementDestinationLiveProjection(tc.body, &reading)
			if got != tc.want || valid != tc.valid {
				t.Fatalf("caught: destination live projection target bytes %q %t, want %q %t", got, valid, tc.want, tc.valid)
			}
			if diff := cmp.Diff(original, reading); diff != "" {
				t.Fatalf("caught: destination live projection changed source facts:\n%s", diff)
			}
			if valid {
				before, bv := agreementDestinationLiveShapes(tc.body, &reading)
				after := agreementDestinationLiveReading(got)
				afterOriginal := agreementDestinationLiveReading(got)
				shapes, av := agreementDestinationLiveShapes(got, &after)
				if !cmp.Equal(original, reading) || !cmp.Equal(afterOriginal, after) {
					t.Fatal("caught: destination live shapes changed original source facts")
				}
				if !bv || !av || !cmp.Equal(before, shapes) {
					t.Fatal("caught: destination live projection changed complete outside shape")
				}
				if len(got) != len(tc.body) {
					t.Fatal("caught: destination live projection changed original byte positions")
				}
			}
		})
	}
}

func TestAgreementDestinationLiveCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body  string
		want        map[string]int
		authorities map[string]string
	}{
		{name: "local outside retains complete page", body: "[[#part]] [[A]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "same block destination metadata", body: "[[A#^a]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "every duplicate outside block occurrence", body: "[[A#^a]] [[A#^a]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "foreign block destination metadata", body: "[[B#^a]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "unmatched backtick outside remains live", body: "[[B`C]] [[A]]\n\n``` [[A]]\n```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "block beside whole discarded declaration", body: "[[A#^a]]\n\n[^n]: [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "local beside both native owners", body: "[[#part]] [[A]]\n\n``` [[A]]\n```\n\n[^n]: [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "inactive block name stays visible", body: "[[A]] [[C#^a]]\n\n``` [[A]]\n```\n\n[^n]: `[[C]]`\n", want: map[string]int{"A": 1, "C": 1}, authorities: map[string]string{"A": "#1011 stage 4", "C": "#1011 stage 5"}},
		{name: "same-name reference refuses borrowing", body: "[r]: [[A]]\n\n[[#part]] [[A]]\n\n``` [[A]]\n```\n", want: nil, authorities: nil},
		{name: "nested outside still refuses", body: "[[B[[A]]]]\n\n``` [[A]]\n```\n", want: nil, authorities: nil},
		{name: "wrapped outside still refuses", body: "[[B\nA]]\n\n``` [[A]]\n```\n", want: nil, authorities: nil},
		{name: "discarded block owner stays refused", body: "[[B#^a]]\n\n[^n]: [[A#^a]]\n", want: nil, authorities: nil},
		{name: "no live page carrier stays earlier", body: "[[#part]]\n\n``` [[A]]\n```\n", want: nil, authorities: nil},
		{name: "full original destination live 0", body: "<!-- [[A]] --> \\`> [!note] title\n%%[[A]]%%![[A]][[#A]]    ```\n`# A\n ^a\n%%[[A]]%% ^é\n ^é\n``` [[A]]\n## [[A|alias]]\nA\n===\n > [!note] title\n## <em>A</em>\n> > ## A\n## A\n``A\n---\n%%", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 1", body: "> [!unknown] title\n  - - [[A#^a]]## A\n\t ``` [[A]]\ntext   > [!note] title\n> [!note] title\nA\n---\n ^A\n   ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 2", body: "![[A]]> B\n\t ```\n[[image.png]][[A#^a]] ^A\n ^a-2\n-   - %%[[A]]%%- [x] [[B]]\nA\n---\n[[A]]> [!note] [[A]]\n ^é\nÉ\n    \t```\n## <em>A</em>\n``` [[A]]\n0<div>\n[[A]]\n</div>\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 3", body: "[[B|alias]] ^é\n> [^n]: [[A]]\n\n    [[B]]\n> >   ```\n> [!note] title\n[[image.png]]1. ## A\n[[A]] [[A]]![[A]]- item\n\n      1. > [!note] title\n ^é\n\n\n[[#A]][[B|alias]] ^a\n``` [[A]]\n ^a\n`[[A]]`\t```\n--><div>\n[[A]]\n</div>\n## !\n\t É\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original destination live 4", body: "A\n> > > [!note] title\n![[A#A]]https://example.invalid/`[[A]]` - [x] [[B]]\n[[image.png]][[A#^a]]A\n    ```\n[[image.png]]````\n\t- > [!note] title\n[^unused]: [[A]]\n[[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n\\[[A]]## [[A|alias]]\n-->## !\n", want: map[string]int{"A": 3, "image.png": 1}, authorities: map[string]string{"A": "#1011 stage 5", "image.png": "#1011 stage 5"}},
		{name: "full original destination live 5", body: "~~~\n> [!note] [[A]]\n> > A\n===\n## A\n~~~\n ^é\n1. > [!note] [[A]]\n[[image.png]]## <em>A</em>\n> [!note] one\n> [!note] two\n> [!note] three\nA\n> > -->[[A#^a]] ```\n```` go [[A]]\nÉ\n## A-2\n## A\n## A\n`![[image.png]] ^é\n\t```\nÉ\n\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 6", body: "  - B\n[[image.png]][[A#A]][[A#^a]]A\n^absent-prose \\[[A]][[A#^a]]   ```\n## A\n## A\n``` [[A]]\n````\n0[[image.png]]<!--\n[[A]]\n-->%%| a | b |\n|---|---|\n| [[A]] | ^a |\nB\n    ```\n## [[A|alias]]\n`open\n[[A]]\nclose````\n- [ ] [[A]]\n  ```\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 7", body: "  - ## !\n``--> ^é\n<!-- ^A\n--> ^a-2\n[[A#^a]]  ```\n> [!unknown] title\n``` [[A]]\n- > [!note] title\n## A\n## A\n> [!unknown] title\n> > ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 8", body: "1.  ^é\n[^unused]: [[A]]\n ^A\n    ```\nA\n  ## A\n## A\n![[image.png]]0> > [[A]] [[A]]<!--\t```\n-->[[#A]]> > ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original destination live 9", body: "- ```` go [[A]]\n  > [!note] title\n![[A#A]]| a | b |\n|---|---|\n| [[A]] | ^a |\n%% ^a\n%%[[A]]%%````\nB\nhttps://example.invalid/`[[A]]` [[#A]]- item\n\n      \n\n[[image.png]]![[image.png]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 10", body: "<!-- [[A]] -->[[A#^a]]> [!note] title\n\n\n1. [[image.png]]- item\n\n       É\n```` go [[A]]\n> [!unknown] title\n- > [!note] title\n\t![[image.png]]ref[^n]\n^absent-prose ~~~\n- [x] [[B]]\n\\`\t```\n ^a\nÉ\n<!-- [[A]] -->[[A\\]]`     ^a\n[[A#A]]-->", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 11", body: "![[image.png]]章節\n> [!unknown] title\n[[A#^a]]![[A#A]]    ```\n`\n\n```` go [[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 12", body: "- > [!note] title\n-->> [!note] title\n ^A\n[[A#^a]] ^é\n[^n]: [[A]]\n\n    [[B]]\n![[A]]-->![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n    ```\n> [!note] title\n章節\nÉ\n## A\n    \t```\n- item\n\n      A\n===\n  > [!note] title\n## [[A|alias]]\n![[image.png]] ^é\n> [!note] one\n> [!note] two\n> [!note] three\n", want: map[string]int{"A": 2, "B": 1, "image.png": 1}, authorities: map[string]string{"A": "#1011 stage 5", "B": "#1011 stage 5", "image.png": "#1011 stage 5"}},
		{name: "full original destination live 13", body: "A\n> > \t````\n-->- [x] [[B]]\n## A-2\n## A\n## A\nA\n---\n[[B|alias]]\t```\n> [!note] one\n> [!note] two\n> [!note] three\nA\n===\n``` [[A]]\n\n~~~~\n````\n0```\nref[^n]\n[[image.png]][[#A]]``- [x] [[B]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 14", body: "%%[[A]]%%\\[[A]]## !\n## [[A|alias]]\nA\n===\n[[#A]][[A\\|alias]][[A]] ^é\n\n\n`[[A]]`  - B\ntext  > [!note] title\n``` [[A]]\n ^é\n[[A\\|alias]]\n\n%%[[A]]%%--> ^a-2\n\t```\n![[image.png]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 15", body: "``[[A#^a]][^n]: [[A]]\n\n    [[B]]\n  > [!note] title\n[^unused]: [[A]]\n    1. ``` [[A]]\n`` ^é\n0\\\\[[A]]```` go [[A]]\n\\`[[image.png]]%%## A\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original destination live 16", body: "[[A\\|alias]]> \\[[A]]~~~~\n![[A#A]]\t```\n<!-- [[A]] -->- [ ] [[A]]\n<!-- [[A]] --> ^é\n\n\n[[#A]][[A\\|alias]][[A\\|alias]]``` [[A]]\n[[#A]]> ## A-2\n## A\n## A\n^absent-prose \t```\n ^é\n[^n]: [[A]]\n\n    [[B]]\n\t- [ ] [[A]]\n    ```\n ^A\n1. ``", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original destination live 17", body: "<!--\n[[A]]\n-->`open\n[[A]]\nclose`A\n[[A#^a]]ref[^n]\n- item\n\n      ```\n    ```\n    ```\n> [!note] one\n> [!note] two\n> [!note] three\n  - ```` go [[A]]\n> [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 18", body: "%%[[A]]%%> [!note] one\n> [!note] two\n> [!note] three\n`\\\\[[A]]ref[^n]\n  - 1. > > <!--[[A#^a]]![[image.png]][[A#A]]~~~\n## A\n## A\nÉ\n``` [[A]]\n0## !\n  ^é\n[^n]: [[A]]\n\n    [[B]]\n\\`- > [!note] title\n| a | b |\n|---|---|\n| [[A]] | ^a |\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 19", body: "  - - > [!note] title\n``` [[A]]\n   ```\nA\n---\n-   ```\n`A\n---\n- > [!note] title\n[[A#^a]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 20", body: "<!-- [[A]] -->[[A#A]] - [ ] [[A]]\n<!-- [[A]] -->[[#A]]A\n---\n`  - É\n``` [[A]]\n````\n- [[A]] ^a\n- item\n\n      `> [!note] [[A]]\n章節\nA\n---\n0%%\t[[image.png]] ^a\n[[A\\|alias]][[A#^a]]## A\n## A\n## !\nÉ\n\\[[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 21", body: " ^a\n[[A#^a]][[B|alias]]A\n---\n\n\n[^unused]: [[A]]\n``` [[A]]\n# A\n  > [!note] title\n## A\n## A\n- [x] [[B]]\n![[image.png]] ```\n<!-- [[A]] -->[[image.png]]1. [[A]][[image.png]] ^a-2\nÉ\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original destination live 22", body: "[[A\\|alias]]- > [!note] title\n\t```\n[^n]: [[A]]\n\n    [[B]]\n[[A]] [[A]]<div>\n[[A]]\n</div>\n- [x] [[B]]\n<div>\n[[A]]\n</div>\n## A-2\n## A\n## A\n![[A]]-->      ```\n> [!unknown] title\n- > [!note] title\n[[A#^a]]- [[A]]É\n- item\n\n      ", want: map[string]int{"A": 4}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original destination live 23", body: "# A\n[[A]] [[A]]~~~~\nB\nref[^n]\n[[A\\|alias]][[#A]]> [!unknown] title\n  -    ```\n## [[A|alias]]\n[[image.png]]## A\nA\nA\n===\n![[image.png]]> [!note] one\n> [!note] two\n> [!note] three\n`~~~\n%%[[A]]%%\n| a | b |\n|---|---|\n| [[A]] | ^a |\n| a | b |\n|---|---|\n| [[A]] | ^a |\n| a | b |\n|---|---|\n| [[A]] | ^a |\n```` go [[A]]\n ## <em>A</em>\n[[A\\|alias]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 24", body: " ^A\n[[A\\|alias]]- [ ] [[A]]\n ^é\n0- > [!note] title\n- \\`[[#A]]- [x] [[B]]\n0https://example.invalid/`[[A]]`  ^é\n    ```\n```` go [[A]]\n[[A]]É\n    - > [!note] one\n> [!note] two\n> [!note] three\n[[A]]\nÉ\ntext ^absent-prose [[A]] [[A]]0> [!note] title\n````\ntext %%[[A]]%%", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 25", body: "``\t%%[[A]]%%1. \n  > [!note] title\n![[A]]  - -   ```\n## A\n## A\n    ```\n[[A#^a]]B\n``` [[A]]\n[[A\\|alias]]## A-2\n## A\n## A\n- > [!note] title\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 26", body: "0\\\\[[A]][[#A]]> [!note] one\n> [!note] two\n> [!note] three\nA\n[[A]]- [ ] [[A]]\nB\n## A\n## A\nB\n  - ## A-2\n## A\n## A\nB\n## <em>A</em>\n- [x] [[B]]\n   ```\n-  ^a\n```` go [[A]]\n -->- ref[^n]\n<!--  > [!note] title\n-->- > [!note] title\n<!-- [[A]] -->```\nA\n---\n%%", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 27", body: " ^a\n ^é\n- > [!note] title\n[^unused]: [[A]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\n`open\n[[A]]\nclose`> [!note] [[A]]\n> [!note] title\n[[A\\|alias]]```\n\t ``` [[A]]\n ^é\n\\\\[[A]]--> ^A\n![[A#A]]\n\n[[A#^a]]", want: map[string]int{"A": 3}, authorities: map[string]string{"A": "#1011 stage 5"}},
		{name: "full original destination live 28", body: "B\nÉ\n\n ^é\nref[^n]\n# A\n[[image.png]]\t%%[[A]]%%[[#A]]text \t```\n- > [!note] title\n ^a\n``` [[A]]\n``> [!note] title\n\\\\[[A]]\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 29", body: "É\n[^unused]: [[A]]\n## A\n-->-->[[#A]]``` [[A]]\n```` go [[A]]\n> [!note] one\n> [!note] two\n> [!note] three\n## <em>A</em>\n~~~\n<!--## !\n0`[[A]]`  - %%[[A]]%%> `[[A]]`  > [!note] title\n- item\n\n          ```\n    [[A\\|alias]]<div>\n[[A]]\n</div>\n- [ ] [[A]]\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original destination live 30", body: "[[#A]][[A#^a]]> > [!note] one\n> [!note] two\n> [!note] three\n\\[[A]]   ```\n[[A]]<!-- [[A]] -->  > [!note] title\n## <em>A</em>\nA\n---\n ^é\n```` go [[A]]\n````\n## A\n## A\n> - > [!note] title\n[^unused]: [[A]]\n## A\n## A\n ^é\n\t[[A#A]] ^é\n~~~\n`open\n[[A]]\nclose`~~~~\n章節\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
		{name: "full original destination live 31", body: "``` [[A]]\n  ```\nA\n===\n> >  ^a-2\nref[^n]\ntext - [ ] [[A]]\n[[A]] [[A]]## [[A|alias]]\n| a | b |\n|---|---|\n| [[A]] | ^a |\nA\n===\n[[A]] [[A]]<div>\n[[A]]\n</div>\n\\\\[[A]]- > [!note] title\n    ```\n    > [!unknown] title\n%%[[A]] [[A]]text A\n---\n<!--\n[[A]]\n-->[[A#A]]\\\\[[A]][[#A]]ref[^n]\nhttps://example.invalid/`[[A]]` [[A#^a]]A\n===\n", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 32", body: "> > <!--- [ ] [[A]]\n## A-2\n## A\n## A\n``![[A]]0[^unused]: [[A]]\n ^a\n  - [[#A]]\n``` [[A]]\n[^n]: [[A]]\n\n    [[B]]\n- [ ] [[A]]\n- item\n\n      <!--\n[[A]]\n-->\t  > [!note] title\n    ```\n## <em>A</em>\n> [!note] title\n    -  ^a-2\n~~~~\n\t```\nref[^n]\nÉ\n## A\n> > ", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 33", body: "> [!note] one\n> [!note] two\n> [!note] three\n- [x] [[B]]\n  - 0- [x] [[B]]\n[[#A]]^absent-prose > - > [!note] title\n```` go [[A]]\nhttps://example.invalid/`[[A]]` ## A\n\n\n-->\\\\[[A]]", want: map[string]int{"A": 1}, authorities: map[string]string{"A": "#1011 stage 4"}},
		{name: "full original destination live 34", body: "`<!--\n[[A]]\n-->## !\n0  -  ^a\n章節\n[^n]: [[A]]\n\n    [[B]]\n> > ```\n[[A#^a]]- [ ] [[A]]\n## !\n[[A]] ^é\n\\`  - ## A\n## A\n\\\\[[A]]![[image.png]]| a | b |\n|---|---|\n| [[A]] | ^a |\n```` go [[A]]\n[[A]]É\n- > [!note] title\n- [ ] [[A]]\n![[image.png]]  > [!note] title\n", want: map[string]int{"A": 2}, authorities: map[string]string{"A": "#1011 stages 4 and 5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			budget := agreementDestinationLiveBudget(t, c.Body, &actual)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementNativeLiveDifference(c, &f, &actual, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatalf("caught: destination live borrowed unowned public difference %s", agreementSignature(&f))
					}
					continue
				}
				if kind != "debt" || wrong != "judge" || authority != tc.authorities[f.Tuple.Target] {
					t.Fatalf("caught: destination live public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementNativeLiveDrift(t, c, &f, &actual, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete destination live public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
func TestAgreementDestinationLiveEvidence(t *testing.T) {
	t.Parallel()
	body := "[[#place]] [[A#^a|shown]] [[B]]\n\n``` [[A]]\n```\n\n[^n]: [[B]] `[[C]]`\n"
	reading := agreementDestinationLiveReading(body)
	projected, valid := agreementDestinationLiveProjection(body, &reading)
	if !valid {
		t.Fatal("caught: destination live producer fixture has no projection")
	}
	after := agreementDestinationLiveReading(projected)
	_, actual := agreementIsolatedPage(t, agreementCase{Body: body})
	_, witness := agreementIsolatedPage(t, agreementCase{Body: projected})
	native := agreementNativeCheckBudget(body)
	baseline := agreementDestinationLiveEvidence(body, projected, &reading, &after, &actual, &witness, native)
	if diff := cmp.Diff(native, baseline.Native); diff != "" {
		t.Fatalf("caught: destination live evidence lost whole check partition:\n%s", diff)
	}
	if diff := cmp.Diff(map[agreementCitation]int{{Target: "A", State: "wikilink-broken"}: 1, {Target: "B", State: "wikilink-broken"}: 1}, baseline.Carriers); diff != "" {
		t.Fatalf("caught: destination live evidence lost original whole page:\n%s", diff)
	}
	for _, tc := range []struct {
		name   string
		change func(*agreementDestinationLiveSource)
	}{
		{"whole native definitions", func(s *agreementDestinationLiveSource) { s.Owned.Page.Definitions = nil }},
		{"native definition label", func(s *agreementDestinationLiveSource) { s.Owned.Page.Definitions[0].Label = "other" }},
		{"native definition span", func(s *agreementDestinationLiveSource) { s.Owned.Page.Definitions[0].Span.Stop-- }},
		{"native definition nodes", func(s *agreementDestinationLiveSource) { s.Owned.Page.Definitions[0].Nodes = nil }},
		{"whole native code", func(s *agreementDestinationLiveSource) { s.Owned.Page.Code = nil }},
		{"native code kind", func(s *agreementDestinationLiveSource) { s.Owned.Page.Code[0].Kind = "other" }},
		{"native code parent", func(s *agreementDestinationLiveSource) { s.Owned.Page.Code[0].Parent = "other" }},
		{"native code span", func(s *agreementDestinationLiveSource) { s.Owned.Page.Code[0].Span.Start++ }},
		{"whole native comments", func(s *agreementDestinationLiveSource) { s.Owned.Page.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"whole check code", func(s *agreementDestinationLiveSource) { s.Owned.Check.Code = nil }},
		{"whole check comments", func(s *agreementDestinationLiveSource) { s.Owned.Check.Comments = []graph.Span{{Start: 0, Stop: 1}} }},
		{"every owned field", func(s *agreementDestinationLiveSource) { s.Owned.Fields = s.Owned.Fields[:1] }},
		{"owned raw target", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Tuple.Target = "other" }},
		{"owned section", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Tuple.Section = "other" }},
		{"owned state", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Tuple.State = "other" }},
		{"owned source role", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Tuple.SourceRole = "other" }},
		{"owned raw word", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.Word = "other" }},
		{"owned position", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.Span.Start++ }},
		{"owned normalized names", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.Targets = nil }},
		{"owned code flag", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.Code = true }},
		{"owned comment flag", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.Comment = true }},
		{"owned escape flag", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.Escaped = true }},
		{"owned definition index", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.DefinitionOwner = 0 }},
		{"owned code index", func(s *agreementDestinationLiveSource) { s.Owned.Fields[0].Source.CodeOwner = -1 }},
		{"whole protected namespace", func(s *agreementDestinationLiveSource) { s.Names = map[string]bool{"unowned": true} }},
		{"whole outside set", func(s *agreementDestinationLiveSource) { s.Outside = nil }},
		{"every outside field", func(s *agreementDestinationLiveSource) { s.Outside = s.Outside[:1] }},
		{"ordered outside set", func(s *agreementDestinationLiveSource) { s.Outside[1], s.Outside[2] = s.Outside[2], s.Outside[1] }},
		{"outside source start", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.Span.Start++ }},
		{"outside source stop", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.Span.Stop-- }},
		{"outside raw word", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.Word = "other" }},
		{"outside coherent spelling needs original window", func(s *agreementDestinationLiveSource) {
			s.Outside[1].Source.Word = strings.Replace(s.Outside[1].Source.Word, "q", "z", 1)
			s.Outside[1].Source.Targets = []string{"z"}
			s.Outside[1].Tuple.Target = "z"
		}},
		{"outside normalized names", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.Targets = []string{"other"} }},
		{"outside normalized count", func(s *agreementDestinationLiveSource) {
			s.Outside[1].Source.Targets = append(s.Outside[1].Source.Targets, "other")
		}},
		{"outside check code role", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.Code = true }},
		{"outside check comment role", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.Comment = true }},
		{"outside escape role", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.Escaped = true }},
		{"outside definition owner", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.DefinitionOwner = 0 }},
		{"outside code owner", func(s *agreementDestinationLiveSource) { s.Outside[1].Source.CodeOwner = 0 }},
		{"outside tuple target", func(s *agreementDestinationLiveSource) { s.Outside[1].Tuple.Target = "other" }},
		{"outside tuple source role", func(s *agreementDestinationLiveSource) { s.Outside[1].Tuple.SourceRole = "other" }},
		{"outside tuple section", func(s *agreementDestinationLiveSource) { s.Outside[1].Tuple.Section = "other" }},
		{"outside tuple state", func(s *agreementDestinationLiveSource) { s.Outside[1].Tuple.State = "other" }},
		{"outside block fragment", func(s *agreementDestinationLiveSource) { s.Outside[1].Block = "other" }},
		{"outside note role", func(s *agreementDestinationLiveSource) { s.Outside[1].Cites = false }},
		{"outside local role", func(s *agreementDestinationLiveSource) { s.Outside[0].Cites = true }},
		{"outside page comment role", func(s *agreementDestinationLiveSource) { s.Outside[1].Comment = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changed := agreementDestinationLiveReading(projected)
			tc.change(&changed)
			if len(agreementDestinationLiveEvidence(body, projected, &reading, &changed, &actual, &witness, native).Native.Citation.Targets) != 0 {
				t.Fatal("caught: destination live producer borrowed changed source context")
			}
		})
	}
	for _, owner := range []string{"A", "B", "C"} {
		t.Run("whole namespace "+owner, func(t *testing.T) {
			t.Parallel()
			changed := agreementDestinationLiveReading(projected)
			other := witness
			other.Citations = append(append([]agreementCitation{}, witness.Citations...), agreementCitation{Target: owner, State: "wikilink-broken"})
			changed.Page = agreementNativeLiveCarriers(&other)
			if len(agreementDestinationLiveEvidence(body, projected, &reading, &changed, &actual, &other, native).Native.Citation.Targets) != 0 {
				t.Fatal("caught: destination live producer borrowed owned page occurrence")
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
				if len(agreementDestinationLiveEvidence(body, projected, &reading, &after, &a, &w, native).Native.Citation.Targets) != 0 {
					t.Fatal("caught: destination live producer borrowed changed whole page")
				}
			})
		}
	}
}

func TestAgreementDestinationLiveShapes(t *testing.T) {
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
	reading := agreementDestinationLiveReading(body)
	original := agreementDestinationLiveReading(body)
	got, valid := agreementDestinationLiveShapes(body, &reading)
	if !valid {
		t.Fatal("caught: destination live full shape lacks source binding")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("caught: complete destination live outside shapes (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original, reading); diff != "" {
		t.Fatalf("caught: destination live shapes changed source facts:\n%s", diff)
	}
}
