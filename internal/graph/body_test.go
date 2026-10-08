package graph_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestBodyFactsFenceSource(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want graph.CodeFact
	}{
		{
			name: "info and closer", body: "``` [[Info]]\n[[Body]]\n```\n",
			want: graph.CodeFact{Kind: graph.CodeFence, Span: graph.Span{Start: 0, Stop: 26}, Opener: graph.Span{Start: 0, Stop: 13}, Info: graph.Span{Start: 4, Stop: 12}, Closer: graph.Span{Start: 22, Stop: 26}},
		},
		{
			name: "empty", body: "```\n```\n",
			want: graph.CodeFact{Kind: graph.CodeFence, Span: graph.Span{Start: 0, Stop: 8}, Opener: graph.Span{Start: 0, Stop: 4}, Closer: graph.Span{Start: 4, Stop: 8}},
		},
		{
			name: "EOF", body: "~~~ lang\nbody",
			want: graph.CodeFact{Kind: graph.CodeFence, Span: graph.Span{Start: 0, Stop: 13}, Opener: graph.Span{Start: 0, Stop: 9}, Info: graph.Span{Start: 4, Stop: 8}, EndOfBody: true},
		},
		{
			name: "quote", body: "> ``` x\n> quoted\n> ```\nafter\n",
			want: graph.CodeFact{Kind: graph.CodeFence, Span: graph.Span{Start: 2, Stop: 23}, Opener: graph.Span{Start: 2, Stop: 8}, Info: graph.Span{Start: 6, Stop: 7}, Closer: graph.Span{Start: 19, Stop: 23}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			facts := graph.ReadBody(tt.body)
			if facts.Source() != tt.body {
				t.Fatal("ReadBody changed source bytes")
			}
			if diff := cmp.Diff([]graph.CodeFact{tt.want}, slices.Collect(facts.Codes())); diff != "" {
				t.Errorf("caught: body fence provenance differs (-want +got):\n%s", diff)
			}
			if !facts.CodeAt(tt.want.Opener.Start) || !facts.CodeAt(tt.want.Span.Stop-1) || facts.CodeAt(tt.want.Span.Stop) {
				t.Error("caught: body fence boundaries differ")
			}
		})
	}
}

func TestBodyFactsMultilineCodeAndComments(t *testing.T) {
	const body = "`one\n[[Two]]` %% hidden %%\n"
	facts := graph.ReadBody(body)
	want := []graph.CodeFact{{Kind: graph.CodeInline, Span: graph.Span{Start: 0, Stop: 13}}}
	if diff := cmp.Diff(want, slices.Collect(facts.Codes())); diff != "" {
		t.Errorf("caught: multiline code provenance differs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]graph.CodeLiteral{{Span: graph.Span{Start: 0, Stop: 13}, Text: "one [[Two]]"}}, slices.Collect(facts.CodeLiterals())); diff != "" {
		t.Errorf("caught: multiline code words differ (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]graph.Span{{Start: 14, Stop: 26}}, slices.Collect(facts.Comments())); diff != "" {
		t.Errorf("caught: original comment provenance differs (-want +got):\n%s", diff)
	}
	if !facts.CodeAt(5) || facts.CommentAt(5) || facts.EmittedAt(15) || !facts.EmittedAt(5) {
		t.Error("caught: code and comment emission differ")
	}
}

func TestBodyFactsUnusedFootnoteProvenance(t *testing.T) {
	const body = "[^unused]: [[Hidden]]\n\nvisible\n"
	facts := graph.ReadBody(body)
	definitions := slices.Collect(facts.Footnotes())
	if len(definitions) != 1 || definitions[0].Label != "unused" || definitions[0].Emitted {
		t.Fatalf("caught: unused footnote provenance = %+v", definitions)
	}
	if facts.EmittedAt(11) || !facts.EmittedAt(24) {
		t.Error("caught: unused definition gained page emission")
	}
	const referenced = "ref[^used]\n\n[^used]: [[Shown]]\n"
	used := graph.ReadBody(referenced)
	definitions = slices.Collect(used.Footnotes())
	if len(definitions) != 1 || definitions[0].Label != "used" || !definitions[0].Emitted || !used.EmittedAt(20) {
		t.Errorf("caught: referenced definition emission = %+v", definitions)
	}
}

func TestBodyFactsValueIsolation(t *testing.T) {
	const body = "```\nx\n```\n"
	facts := graph.ReadBody(body)
	copied := slices.Collect(facts.Codes())
	copied[0].Span = graph.Span{}
	for code := range facts.Codes() {
		code.Span.Start = 99
	}
	if !facts.CodeAt(0) || facts.Source() != body {
		t.Error("caught: returned code values changed owned facts")
	}
	count := 0
	for range facts.Codes() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("code iterator yielded %d values before stop, want 1", count)
	}
}

func TestBodyFactsFootnoteReferenceVisibility(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []bool
	}{
		{name: "hidden reference", body: "%% [^used] %%\n\n[^used]: [[Hidden]]\n", want: []bool{false}},
		{name: "visible reference", body: "%% [^used] %%\nvisible[^used]\n\n[^used]: [[Shown]]\n", want: []bool{true}},
		{name: "unused reference chain", body: "[^first]: second[^second]\n\n[^second]: [[Hidden]]\n", want: []bool{false, true}},
		{name: "visible reference chain", body: "visible[^first]\n\n[^first]: second[^second]\n\n[^second]: [[Shown]]\n", want: []bool{true, true}},
		{name: "inline reference", body: "^[ref[^used]]\n\n[^used]: [[Shown]]\n", want: []bool{true}},
		{name: "duplicate definition", body: "ref[^x]\n\n[^x]: first\n\n[^x]: [[Hidden]]\n", want: []bool{true, false}},
		{name: "hidden first duplicate", body: "%%\n[^x]: hidden\n%%\nvisible[^x]\n\n[^x]: [[Shown]]\n", want: []bool{false, true}},
		{name: "reference in unselected duplicate", body: "ref[^x]\n\n[^x]: first\n\n[^x]: ref[^y]\n\n[^y]: shown\n", want: []bool{true, false, true}},
		{name: "ordinary cycle", body: "[^a]: ref[^b]\n\n[^b]: ref[^a]\n", want: []bool{true, true}},
		{name: "inline token in unused definition", body: "[^unused]: ^[ref[^used]]\n\n[^used]: [[Shown]]\n", want: []bool{false, true}},
		{name: "comment creates reference", body: "[^%% hide %%x]\n\n[^x]: [[Shown]]\n", want: []bool{true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			facts := graph.ReadBody(tt.body)
			var emitted []bool
			for definition := range facts.Footnotes() {
				emitted = append(emitted, definition.Emitted)
			}
			if diff := cmp.Diff(tt.want, emitted); diff != "" {
				t.Errorf("caught: visible footnote reachability differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBodyFactsInlineFootnoteSource(t *testing.T) {
	const body = "note ^[one\n two] end\n"
	facts := graph.ReadBody(body)
	want := []graph.InlineFootnoteFact{{Span: graph.Span{Start: 5, Stop: 16}, Content: "one\n two"}}
	if diff := cmp.Diff(want, slices.Collect(facts.InlineFootnotes())); diff != "" {
		t.Errorf("caught: original inline note provenance differs (-want +got):\n%s", diff)
	}
	for _, literal := range []string{"`^[literal]`", "\\^[escaped]", "<a title=\"^[attribute]\">link</a>"} {
		if notes := slices.Collect(graph.ReadBody(literal).InlineFootnotes()); len(notes) != 0 {
			t.Errorf("caught: literal inline note %q was recognized: %+v", literal, notes)
		}
	}
}

func TestBodyFactsInlineFootnoteContent(t *testing.T) {
	const body = "^[literal `[[Missing]]` and [link](Target.md)]"
	facts := graph.ReadBody(body)
	wantCode := []graph.CodeFact{{Kind: graph.CodeInline, Span: graph.Span{Start: 10, Stop: 23}}}
	if diff := cmp.Diff(wantCode, slices.Collect(facts.Codes())); diff != "" {
		t.Errorf("caught: inline note code provenance differs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]graph.CodeLiteral{{Span: graph.Span{Start: 10, Stop: 23}, Text: "[[Missing]]"}}, slices.Collect(facts.CodeLiterals())); diff != "" {
		t.Errorf("caught: inline note code literal differs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]graph.BodyDestination{{Offset: 28, Target: "Target.md"}}, slices.Collect(facts.Destinations())); diff != "" {
		t.Errorf("caught: inline note target provenance differs (-want +got):\n%s", diff)
	}
	headings := slices.Collect(graph.ReadBody("## Heading^[words and [link](Target.md)] after\n").Headings())
	if len(headings) != 1 || headings[0].Text != "Heading after" {
		t.Errorf("caught: inline note content changed heading prose: %+v", headings)
	}
}

func TestBodyFactsInlineFootnoteOriginalContent(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{name: "comment inside", body: "^[one %%hidden%% two]", want: "one %%hidden%% two"},
		{name: "comment joins opener", body: "^%%hidden%%[words]", want: "words"},
		{name: "quote prefix", body: "> ^[one\n> two]", want: "one\n> two"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			notes := slices.Collect(graph.ReadBody(tt.body).InlineFootnotes())
			if len(notes) != 1 || notes[0].Content != tt.want {
				t.Errorf("caught: inline content lost original delimiter provenance: got %+v, want %q", notes, tt.want)
			}
		})
	}
}

func TestBodyFactsHiddenDuplicateSource(t *testing.T) {
	const body = "%%\n[^x]: hidden\n%%\nvisible[^x]\n\n[^x]: [[Shown]]\n"
	facts := graph.ReadBody(body)
	if facts.Source() != body || !facts.EmittedAt(38) || facts.EmittedAt(9) {
		t.Errorf("caught: surviving duplicate lost original Shown identity: source=%q shown=%v hidden=%v", facts.Source(), facts.EmittedAt(38), facts.EmittedAt(9))
	}
}

func TestBodyFactsInlineContainerMapping(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "quote", body: "> note ^[one\n> `code` [link](Target.md)]\n"},
		{name: "list", body: "- note ^[one\n  `code` [link](Target.md)]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			facts := graph.ReadBody(tt.body)
			wantCode := []graph.CodeFact{{Kind: graph.CodeInline, Span: graph.Span{Start: 15, Stop: 21}}}
			if diff := cmp.Diff(wantCode, slices.Collect(facts.Codes())); diff != "" {
				t.Errorf("caught: container code mapping differs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]graph.BodyDestination{{Offset: 22, Target: "Target.md"}}, slices.Collect(facts.Destinations())); diff != "" {
				t.Errorf("caught: container destination mapping differs (-want +got):\n%s", diff)
			}
			if facts.Source() != tt.body || strings.Count(facts.Source()[:22], "\n") != 1 {
				t.Error("caught: mapped content lost its original second line")
			}
		})
	}
}

func TestBodyFactsInlineHostIsolation(t *testing.T) {
	const body = "[host ^[note]](Host.md)"
	if diff := cmp.Diff([]graph.BodyDestination{{Offset: 0, Target: "Host.md"}}, slices.Collect(graph.ReadBody(body).Destinations())); diff != "" {
		t.Errorf("caught: opaque note consumed its host link closer (-want +got):\n%s", diff)
	}
	for _, source := range []string{"==before ^[inside==] after", "*before ^[inside*] after"} {
		expanded := graph.ExpandInlineFootnotes(source)
		doc := graph.NewBodyMarkdown(nil).Parser().Parse(text.NewReader([]byte(expanded)))
		paired := 0
		if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering && (node.Kind() == graph.KindHighlight || node.Kind() == ast.KindEmphasis) {
				paired++
			}
			return ast.WalkContinue, nil
		}); err != nil {
			t.Fatal(err)
		}
		if paired != 0 {
			t.Errorf("caught: host and note delimiter stacks paired across their boundary: %q, pairs=%d", source, paired)
		}
	}
}

func TestBodyFactsSingleInlineAdmission(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{name: "nested literal", body: "^[outer ^[inner]]", want: "[^yomihon-inline-footnote-1]\n[^yomihon-inline-footnote-1]: outer ^[inner]\n\n"},
		{name: "newly selected definition", body: "^[ref[^used]]\n\n[^used]: ^[inner]\n", want: "[^yomihon-inline-footnote-1]\n\n[^used]: ^[inner]\n[^yomihon-inline-footnote-1]: ref[^used]\n\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expanded := graph.ExpandInlineFootnotes(tt.body)
			if expanded != tt.want {
				t.Fatalf("caught: inline admission recursed or changed placement: got %q, want %q", expanded, tt.want)
			}
			var rendered bytes.Buffer
			if err := graph.NewBodyMarkdown(nil).Convert([]byte(expanded), &rendered); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rendered.String(), "^[inner]") {
				t.Errorf("caught: final grammar changed nested literal: %q", rendered.String())
			}
			if notes := slices.Collect(graph.AuthoredInlineNotes(tt.body)); len(notes) != 1 {
				t.Errorf("caught: original single-pass admission = %+v", notes)
			}
		})
	}
	facts := graph.ReadBody("^[ref[^used]]\n\n[^used]: ^[inner]\n")
	if !facts.EmittedAt(24) {
		t.Error("caught: newly selected definition lost emission despite unexpanded inline token")
	}
}

func TestBodyFactsOneCommentStrip(t *testing.T) {
	const body = "``%%``%%%%`"
	facts := graph.ReadBody(body)
	if facts.CommentFree() != "``%%```" || facts.UnclosedComment() != (graph.BodyComment{}) {
		t.Errorf("caught: original comments were stripped again: text=%q unclosed=%+v", facts.CommentFree(), facts.UnclosedComment())
	}
	if diff := cmp.Diff([]graph.Span{{Start: 6, Stop: 10}}, slices.Collect(facts.Comments())); diff != "" {
		t.Errorf("caught: second strip changed original comment geometry (-want +got):\n%s", diff)
	}
	const joined = "`one`%%hidden%%`two`"
	joinedFacts := graph.ReadBody(joined)
	if diff := cmp.Diff([]graph.CodeLiteral{{Span: graph.Span{Start: 0, Stop: 20}, Text: "one``two"}}, slices.Collect(joinedFacts.CodeLiterals())); diff != "" {
		t.Errorf("caught: joined code authority kept bootstrap coordinates (-want +got):\n%s", diff)
	}
	if joinedFacts.EmittedAt(7) || !joinedFacts.EmittedAt(16) {
		t.Error("caught: joined authority lost original comment visibility")
	}
}

func TestBodyFactsUnchangedPresentation(t *testing.T) {
	const body = "## Plain\n\n[link](Target.md)\n"
	facts := graph.ReadBody(body)
	if facts.Source() != body || facts.CommentFree() != body || graph.ExpandInlineFootnotes(body) != body {
		t.Error("caught: untouched source was transformed")
	}
	if diff := cmp.Diff([]graph.BodyDestination{{Offset: 10, Target: "Target.md"}}, slices.Collect(facts.Destinations())); diff != "" {
		t.Errorf("caught: unchanged destination authority differs (-want +got):\n%s", diff)
	}
}

func TestBodyFactsRetainedInstructionIsNotProse(t *testing.T) {
	const body = "<!-- read-aloud: [[Ghost]] -->"
	facts := graph.ReadBody(body)
	if facts.CommentFree() != body || facts.EmittedAt(17) {
		t.Errorf("caught: retained instruction became prose: text=%q emitted=%v", facts.CommentFree(), facts.EmittedAt(17))
	}
	if diff := cmp.Diff([]graph.Span{{Start: 0, Stop: 30}}, slices.Collect(facts.Comments())); diff != "" {
		t.Errorf("caught: retained instruction lost hidden-source identity (-want +got):\n%s", diff)
	}
}

func FuzzBodyFactsOriginalCoordinates(f *testing.F) {
	for _, body := range []string{
		"plain text", "``` x\n[[Literal]]\n```\n", "> ```\n> code\n\noutside\n",
		"%%\n[^x]: hidden\n%%\nvisible[^x]\n\n[^x]: shown\n",
		"^[outer ^[inner]]", "> note ^[one\n> `code` [link](Target.md)]\n",
		"[host ^[note]](Host.md)", "``%%``%%%%`", "[^a]: ref[^b]\n\n[^b]: ref[^a]\n",
	} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		facts := graph.ReadBody(body)
		if facts.Source() != body {
			t.Fatal("caught: body construction replaced original source")
		}
		for code := range facts.Codes() {
			for _, span := range []graph.Span{code.Span, code.Opener, code.Info, code.Closer} {
				if span.Start < 0 || span.Stop < span.Start || span.Stop > len(body) {
					t.Fatalf("caught: code provenance escaped original bytes: %+v in %q", span, body)
				}
			}
		}
		for destination := range facts.Destinations() {
			if destination.Offset < 0 || destination.Offset >= len(body) {
				t.Fatalf("caught: destination provenance escaped original bytes: %+v in %q", destination, body)
			}
		}
		for definition := range facts.Footnotes() {
			if definition.Span.Start < 0 || definition.Span.Stop < definition.Span.Start || definition.Span.Stop > len(body) {
				t.Fatalf("caught: definition provenance escaped original bytes: %+v in %q", definition, body)
			}
		}
	})
}

func TestBodyFactsContainerEnds(t *testing.T) {
	const body = "> ```\n> code\n\noutside [[Visible]]\n"
	facts := graph.ReadBody(body)
	codes := slices.Collect(facts.Codes())
	if len(codes) != 1 || codes[0].Kind != graph.CodeFence || !codes[0].Closer.Zero() || codes[0].EndOfBody {
		t.Fatalf("caught: container-bounded fence = %+v", codes)
	}
	if facts.CodeAt(15) {
		t.Error("caught: quote fence consumed following outside prose")
	}
	const comment = "> <!-- unclosed\n\noutside\n"
	hidden := graph.ReadBody(comment)
	if !hidden.CommentAt(4) || hidden.CommentAt(19) || !hidden.EmittedAt(19) {
		t.Error("caught: HTML comment crossed its quote container")
	}
}

func TestBodyFactsHeadingsAndDestinations(t *testing.T) {
	const body = "## Actual `quoted`\n\n[link](Target.md)\n\n`code\nliteral ## Literal`\n"
	facts := graph.ReadBody(body)
	headings := slices.Collect(facts.Headings())
	if diff := cmp.Diff([]graph.BodyHeading{{Span: graph.Span{Start: 3, Stop: 18}, Level: 2, Text: "Actual "}}, headings); diff != "" {
		t.Errorf("caught: heading recognition differs (-want +got):\n%s", diff)
	}
	destinations := slices.Collect(facts.Destinations())
	if diff := cmp.Diff([]graph.BodyDestination{{Offset: 20, Target: "Target.md"}}, destinations); diff != "" {
		t.Errorf("caught: destination source attribution differs (-want +got):\n%s", diff)
	}
	if !facts.CodeAt(44) {
		t.Error("caught: multiline span lost its heading-like code")
	}
	if diff := cmp.Diff([]graph.CodeFact{
		{Kind: graph.CodeInline, Span: graph.Span{Start: 10, Stop: 18}},
		{Kind: graph.CodeInline, Span: graph.Span{Start: 39, Stop: 64}},
	}, slices.Collect(facts.Codes())); diff != "" {
		t.Errorf("caught: multiline code lost its consumed delimiters (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]graph.CodeLiteral{
		{Span: graph.Span{Start: 10, Stop: 18}, Text: "quoted"},
		{Span: graph.Span{Start: 39, Stop: 64}, Text: "code literal ## Literal"},
	}, slices.Collect(facts.CodeLiterals())); diff != "" {
		t.Errorf("caught: multiline code words changed (-want +got):\n%s", diff)
	}
}

func TestBodyFactsATXInterruptsUnclosedCode(t *testing.T) {
	const body = "## Actual `quoted`\n\n[link](Target.md)\n\n`code\n## Literal`\n"
	facts := graph.ReadBody(body)
	if diff := cmp.Diff([]graph.BodyHeading{
		{Span: graph.Span{Start: 3, Stop: 18}, Level: 2, Text: "Actual "},
		{Span: graph.Span{Start: 48, Stop: 56}, Level: 2, Text: "Literal`"},
	}, slices.Collect(facts.Headings())); diff != "" {
		t.Errorf("caught: ATX heading did not interrupt its paragraph (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]graph.CodeFact{{Kind: graph.CodeInline, Span: graph.Span{Start: 10, Stop: 18}}}, slices.Collect(facts.Codes())); diff != "" {
		t.Errorf("caught: interrupted backticks manufactured multiline code (-want +got):\n%s", diff)
	}
	if facts.CodeAt(39) || facts.CodeAt(48) {
		t.Error("caught: interrupted paragraph or heading became code")
	}
}
