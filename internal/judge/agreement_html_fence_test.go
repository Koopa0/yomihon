package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

// A fence-shaped line remains literal inside an authored HTML element block.
// Retain every native line and raw field before selecting the fields the line
// walk leaves unlinked. Other source owners cannot lend their target counts.
type agreementHTMLFenceField struct {
	Span         graph.Span
	Word, Target string
	Fenced       bool
}

type agreementHTMLFenceSource struct {
	Owner  graph.Span
	Lines  []graph.Span
	Fields []agreementHTMLFenceField
}

func agreementHTMLFenceReading(body string, grammar goldmark.Markdown) agreementHTMLFenceSource {
	source := []byte(body)
	ctx := parser.NewContext()
	ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
	var block *ast.HTMLBlock
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if candidate, ok := n.(*ast.HTMLBlock); ok && candidate.HTMLBlockType == ast.HTMLBlockType6 {
			if block != nil {
				return agreementHTMLFenceSource{}
			}
			block = candidate
		}
	}
	if block == nil || block.Lines().Len() == 0 || block.HasClosure() {
		return agreementHTMLFenceSource{}
	}
	start := block.Lines().At(0).Start
	stop := block.Lines().At(block.Lines().Len() - 1).Stop
	if start < 0 || stop <= start || stop > len(body) {
		return agreementHTMLFenceSource{}
	}
	for line := range strings.SplitSeq(body[:start], "\n") {
		if _, _, opens := graph.FenceOpens(line); opens {
			return agreementHTMLFenceSource{}
		}
	}
	if strings.Contains(body[start:stop], "[!") || strings.Contains(body[start:stop], "[^") || !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return agreementHTMLFenceSource{}
	}
	comments := agreementUnusedWidgetComments(body, doc)
	if agreementUnusedWidgetCommentOwned(graph.Span{Start: start, Stop: stop}, comments) {
		return agreementHTMLFenceSource{}
	}
	reading := agreementHTMLFenceSource{Owner: graph.Span{Start: start, Stop: stop}}
	var marker byte
	var size int
	inFence := false
	for i := range block.Lines().Len() {
		line := block.Lines().At(i)
		reading.Lines = append(reading.Lines, graph.Span{Start: line.Start, Stop: line.Stop})
		raw := body[line.Start:line.Stop]
		owned := inFence
		if inFence {
			if graph.FenceCloses(raw, marker, size) {
				inFence = false
				owned = false
			}
		} else if m, n, opens := graph.FenceOpens(raw); opens {
			info := strings.TrimSpace(strings.TrimLeft(raw, " \t")[n:])
			if strings.EqualFold(info, "mermaid") {
				return agreementHTMLFenceSource{}
			}
			marker, size, inFence, owned = m, n, true, true
		}
		offset := 0
		for offset < len(raw) {
			open := strings.Index(raw[offset:], "[[")
			if open < 0 {
				break
			}
			open += offset
			end := strings.Index(raw[open+2:], "]]")
			if end < 0 {
				return agreementHTMLFenceSource{}
			}
			end += open + 2
			inner := raw[open+2 : end]
			if strings.ContainsAny(inner, "[]\r\n`") || graph.EscapedWikilinkAt(raw, open) || open > 0 && raw[open-1] == '!' || strings.ContainsAny(raw, "<>") {
				return agreementHTMLFenceSource{}
			}
			link, cites := graph.ParseWikilink(inner)
			if !cites || link.Block != "" {
				return agreementHTMLFenceSource{}
			}
			word := raw[open : end+2]
			targets := judge.LinkTargets(word)
			if len(targets) != 1 {
				return agreementHTMLFenceSource{}
			}
			reading.Fields = append(reading.Fields, agreementHTMLFenceField{Span: graph.Span{Start: line.Start + open, Stop: line.Start + end + 2}, Word: word, Target: targets[0], Fenced: owned})
			offset = end + 2
		}
	}
	return reading
}

func agreementHTMLFenceBudget(body string) agreementUnusedCitationPayload {
	p := agreementHTMLFenceReading(body, agreementFootnoteGrammar)
	g := agreementHTMLFenceReading(body, agreementExclusiveCodeGrammar)
	if len(p.Fields) == 0 || !cmp.Equal(p, g) {
		return agreementUnusedCitationPayload{}
	}
	all := make(map[string]int)
	check := make(map[string]int)
	for _, f := range p.Fields {
		all[f.Target]++
	}
	for _, t := range judge.LinkTargets(body) {
		check[t]++
	}
	var targets map[string]int
	for _, f := range p.Fields {
		if !f.Fenced || all[f.Target] != check[f.Target] {
			continue
		}
		if targets == nil {
			targets = make(map[string]int)
		}
		targets[f.Target]++
	}
	return agreementUnusedCitationPayload{Body: body, Targets: targets}
}

func agreementHTMLFenceDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget agreementUnusedCitationPayload) (kind, authority, wrong string) {
	kind, authority, wrong = agreementSharedUnusedCitationDifference(c, f, actual, budget)
	if kind != "" {
		wrong = "page"
	}
	return kind, authority, wrong
}

func TestAgreementHTMLFenceSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementHTMLFenceSource
	}{
		{name: "every physical row and selected field", body: "<div>\n[[A]]\n```\n[[B]]\n```\n[[A]]\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 39}, Lines: []graph.Span{{Start: 0, Stop: 6}, {Start: 6, Stop: 12}, {Start: 12, Stop: 16}, {Start: 16, Stop: 22}, {Start: 22, Stop: 26}, {Start: 26, Stop: 32}, {Start: 32, Stop: 39}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Target: "A"}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[B]]", Target: "B", Fenced: true}, {Span: graph.Span{Start: 26, Stop: 31}, Word: "[[A]]", Target: "A"}}}},
		{name: "opening info and all repeated payload fields", body: "<div>\n``` [[A]]\n[[B]] [[B]]\n```\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 39}, Lines: []graph.Span{{Start: 0, Stop: 6}, {Start: 6, Stop: 16}, {Start: 16, Stop: 28}, {Start: 28, Stop: 32}, {Start: 32, Stop: 39}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 10, Stop: 15}, Word: "[[A]]", Target: "A", Fenced: true}, {Span: graph.Span{Start: 16, Stop: 21}, Word: "[[B]]", Target: "B", Fenced: true}, {Span: graph.Span{Start: 22, Stop: 27}, Word: "[[B]]", Target: "B", Fenced: true}}}},
		{name: "ordinary HTML retains an unselected field", body: "<div>\n[[A]]\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 19}, Lines: []graph.Span{{Start: 0, Stop: 6}, {Start: 6, Stop: 12}, {Start: 12, Stop: 19}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Target: "A"}}}},
		{name: "unclosed apparent fence stays inside native HTML", body: "<div>\n```\n[[A]]\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 23}, Lines: []graph.Span{{Start: 0, Stop: 6}, {Start: 6, Stop: 10}, {Start: 10, Stop: 16}, {Start: 16, Stop: 23}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 10, Stop: 15}, Word: "[[A]]", Target: "A", Fenced: true}}}},
		{name: "root HTML without fields still owns its declaration", body: "<div>\n\n```\n[[A]]\n```\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 6}, Lines: []graph.Span{{Start: 0, Stop: 6}}}},
		{name: "physical CRLF coordinates remain original", body: "<div>\r\n~~~\r\n[[A]]\r\n~~~\r\n</div>\r\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 32}, Lines: []graph.Span{{Start: 0, Stop: 7}, {Start: 7, Stop: 12}, {Start: 12, Stop: 19}, {Start: 19, Stop: 24}, {Start: 24, Stop: 32}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[A]]", Target: "A", Fenced: true}}}},
		{name: "native opening indentation remains original", body: "  <div>\n```\n[[A]]\n```\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 29}, Lines: []graph.Span{{Start: 0, Stop: 8}, {Start: 8, Stop: 12}, {Start: 12, Stop: 18}, {Start: 18, Stop: 22}, {Start: 22, Stop: 29}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[A]]", Target: "A", Fenced: true}}}},
		{name: "Unicode prefix alias section and raw suffix retain bytes", body: "<div>\n~~~\n純 !\n[[A#part|shown]] [[B\\]]\n~~~\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 51}, Lines: []graph.Span{{Start: 0, Stop: 6}, {Start: 6, Stop: 10}, {Start: 10, Stop: 16}, {Start: 16, Stop: 40}, {Start: 40, Stop: 44}, {Start: 44, Stop: 51}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 16, Stop: 32}, Word: "[[A#part|shown]]", Target: "A", Fenced: true}, {Span: graph.Span{Start: 33, Stop: 39}, Word: "[[B\\]]", Target: "B", Fenced: true}}}},
		{name: "field indentation and trailing space keep physical coordinates", body: "<div>\n```\n  [[A]]  \n```\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 31}, Lines: []graph.Span{{Start: 0, Stop: 6}, {Start: 6, Stop: 10}, {Start: 10, Stop: 20}, {Start: 20, Stop: 24}, {Start: 24, Stop: 31}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 12, Stop: 17}, Word: "[[A]]", Target: "A", Fenced: true}}}},
		{name: "short and mixed closers retain each original field role", body: "<div>\n````\n[[A]]\n```\n[[B]]\n~~~~\n[[A]]\n````\n[[B]]\n</div>\n", want: agreementHTMLFenceSource{Owner: graph.Span{Start: 0, Stop: 56}, Lines: []graph.Span{{Start: 0, Stop: 6}, {Start: 6, Stop: 11}, {Start: 11, Stop: 17}, {Start: 17, Stop: 21}, {Start: 21, Stop: 27}, {Start: 27, Stop: 32}, {Start: 32, Stop: 38}, {Start: 38, Stop: 43}, {Start: 43, Stop: 49}, {Start: 49, Stop: 56}}, Fields: []agreementHTMLFenceField{{Span: graph.Span{Start: 11, Stop: 16}, Word: "[[A]]", Target: "A", Fenced: true}, {Span: graph.Span{Start: 21, Stop: 26}, Word: "[[B]]", Target: "B", Fenced: true}, {Span: graph.Span{Start: 32, Stop: 37}, Word: "[[A]]", Target: "A", Fenced: true}, {Span: graph.Span{Start: 43, Stop: 48}, Word: "[[B]]", Target: "B"}}}},
		{name: "different raw element kind refuses", body: "<pre>\n```\n[[A]]\n```\n</pre>\n"},
		{name: "native quote has another owner", body: "> <div>\n> ```\n> [[A]]\n> ```\n> </div>\n"},
		{name: "real root fence is not HTML", body: "```\n[[A]]\n```\n"},
		{name: "whole native declaration set cannot select last owner", body: "<div>\nplain\n</div>\n\n<div>\n```\n[[A]]\n```\n</div>\n"},
		{name: "earlier fence retains outside context", body: "```\n[[B]]\n```\n<div>\n```\n[[A]]\n```\n</div>\n"},
		{name: "hidden HTML source refuses", body: "%%\n<div>\n```\n[[A]]\n```\n</div>\n%%\n"},
		{name: "callout inside HTML needs another owner", body: "<div>\n> [!note] words\n```\n[[A]]\n```\n</div>\n"},
		{name: "reference inside HTML needs another owner", body: "<div>\n[^n]: [[B]]\n```\n[[A]]\n```\n</div>\n"},
		{name: "mermaid is another rendering", body: "<div>\n```mermaid\n[[A]]\n```\n</div>\n"},
		{name: "unfinished field refuses the full set", body: "<div>\n```\n[[A]] [[B\n```\n</div>\n"},
		{name: "nested field refuses the full set", body: "<div>\n```\n[[A]] [[B[[C]]]]\n```\n</div>\n"},
		{name: "escaped field refuses the full set", body: "<div>\n```\n[[A]] \\[[B]]\n```\n</div>\n"},
		{name: "embed has another source role", body: "<div>\n```\n[[A]] ![[B]]\n```\n</div>\n"},
		{name: "local target has another role", body: "<div>\n```\n[[A]] [[#B]]\n```\n</div>\n"},
		{name: "block target has another role", body: "<div>\n```\n[[A]] [[B#^b]]\n```\n</div>\n"},
		{name: "tag on a field line needs another reading", body: "<div>\n```\n<em>[[A]]</em>\n```\n</div>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(tc.want, agreementHTMLFenceReading(tc.body, grammar)); diff != "" {
					t.Fatalf("caught: complete HTML fence source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementHTMLFenceCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body   string
		budget, want map[string]int
	}{
		{name: "opening info and every repeated field", body: "<div>\n``` [[A]]\n[[B]] [[B]]\n```\n</div>\n", budget: map[string]int{"A": 1, "B": 2}, want: map[string]int{"A": 1, "B": 2}},
		{name: "all visible counts within the native owner", body: "<div>\n[[A]] [[A]]\n```\n[[A]] [[A]] [[B]]\n```\n[[B]]\n</div>\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "every independent apparent fence", body: "<div>\n~~~\n[[A]]\n~~~\n```\n[[B]]\n```\n</div>\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "shorter and mixed closers retain ownership", body: "<div>\n````\n[[A]]\n```\n[[B]]\n~~~~\n[[A]]\n````\n[[B]]\n</div>\n", budget: map[string]int{"A": 2, "B": 1}, want: map[string]int{"A": 2, "B": 1}},
		{name: "unclosed apparent fence reaches native owner end", body: "<div>\n```\n[[A]]\n</div>\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "raw suffix alias and section keep check normalization", body: "<div>\n~~~\n[[A#part|shown]] [[B\\]]\n~~~\n</div>\n", budget: map[string]int{"A": 1, "B": 1}, want: map[string]int{"A": 1, "B": 1}},
		{name: "ordinary HTML has no fence difference", body: "<div>\n[[A]]\n</div>\n"},
		{name: "same target outside HTML refuses exclusive contribution", body: "[[A]]\n\n<div>\n```\n[[A]]\n```\n</div>\n"},
		{name: "outside missing reference cannot lend count", body: "<div>\n```\n[[A]]\n```\n\n[r]: [[A]]\n"},
		{name: "different outside missing target stays separate", body: "<div>\n```\n[[A]]\n```\n\n[r]: [[B]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "different outside visible target stays separate", body: "[[B]]\n\n<div>\n```\n[[A]]\n```\n</div>\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "inline reading inside HTML cannot lend absent count", body: "<div>\n```\n[[A]]\n```\n`[[A]]`\n</div>\n", budget: map[string]int{"A": 1}},
		{name: "real fence after blank remains outside", body: "<div>\n\n```\n[[A]]\n```\n"},
		{name: "hidden declaration cannot lend missing reference", body: "%%\n<div>\n```\n[[A]]\n```\n</div>\n%%\n\n[r]: [[A]]\n"},
		{name: "mermaid refuses full contribution", body: "<div>\n```mermaid\n[[A]]\n```\n</div>\n"},
		{name: "escaped field retains another role", body: "<div>\n```\n[[A]] \\[[B]]\n```\n</div>\n"},
		{name: "full original native HTML 0", body: "<div>\n[[A]]\n</div>\n[[A#A]]~~~\n`open\n[[A]]\nclose`É\n````\n<div>\n[[A]]\n</div>\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "full original native HTML 1", body: "[[#A]] ^é\n\\`   ```\n<!-- [[A]] -->## A\n<div>\n[[A]]\n</div>\n````\n## [[A|alias]]\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "full original native HTML 2", body: "<div>\n[[A]]\n</div>\n```` go [[A]]\n> ", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "full original native HTML 3", body: "<div>\n[[A]]\n</div>\n ^A\n`open\n[[A]]\nclose`<div>\n[[A]]\n</div>\n[[image.png]]章節\n   ```\n<div>\n[[A]]\n</div>\n## A\n", budget: map[string]int{"A": 1}, want: map[string]int{"A": 1}},
		{name: "ordinary root code remains earlier ownership", body: "```\n[[A]]\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementHTMLFenceBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Targets); diff != "" {
				t.Fatalf("caught: complete HTML fence source budget (-want +got):\n%s", diff)
			}
			r, a := agreementIsolatedPage(t, c)
			var found map[string]int
			for _, f := range agreementPageFailures(c.Body, &r, &a) {
				kind, authority, wrong := agreementHTMLFenceDifference(c, &f, &a, budget)
				if f.Property != "P1" || f.Direction != "judge-only" || tc.want[f.Tuple.Target] == 0 {
					if kind != "" {
						t.Fatal("caught: HTML fence borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: HTML fence public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]int)
				}
				found[f.Tuple.Target] = f.Multiplicity
				agreementSharedUnusedCitationDrift(t, c, &f, &a, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete HTML fence public receipts (-want +got):\n%s", diff)
			}
		})
	}
}
