package judge_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// Literal widget delimiters separate words even beside a live alias. Shield
// only declared literal fields with equal-width punctuation, keeping the native
// heading lines and code owners intact. SectionID treats both delimiters as
// separators. The complete source reading must agree in both grammars and with
// every page heading before it can own a missing check name.
type agreementMixedNamespaceField struct {
	Span graph.Span
	Role string
}
type agreementMixedNamespaceReading struct {
	Bases, Checks []string
	Headings      []graph.Span
	Code          []graph.Span
	Fields        []agreementMixedNamespaceField
	Eligible      bool
}

func agreementMixedNamespaceRead(body string, grammar goldmark.Markdown) agreementMixedNamespaceReading {
	source := []byte(body)
	ctx := parser.NewContext()
	ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
	r := agreementMixedNamespaceReading{Eligible: true}
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := node.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		if h.Lines().Len() == 0 {
			r.Eligible = false
			return ast.WalkStop, nil
		}
		var code []graph.Span
		if err := ast.Walk(h, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if n, ok := node.(*ast.CodeSpan); entering && ok && n.FirstChild() != nil {
				a, okA := n.FirstChild().(*ast.Text)
				b, okB := n.LastChild().(*ast.Text)
				if !okA || !okB {
					r.Eligible = false
					return ast.WalkStop, nil
				}
				code = append(code, graph.Span{Start: a.Segment.Start, Stop: b.Segment.Stop})
				r.Code = append(r.Code, graph.Span{Start: a.Segment.Start, Stop: b.Segment.Stop})
			}
			return ast.WalkContinue, nil
		}); err != nil {
			panic(err)
		}
		start, end := h.Lines().At(0).Start, h.Lines().At(h.Lines().Len()-1).Stop
		r.Headings = append(r.Headings, graph.Span{Start: start, Stop: end})
		masked := slices.Clone(source)
		for off := start; off < end; {
			rel := strings.Index(body[off:end], "[[")
			if rel < 0 {
				break
			}
			open := off + rel
			inner, tail, closed := strings.Cut(body[open+2:end], "]]")
			if !closed || strings.ContainsAny(inner, "[]") {
				r.Eligible = false
				return ast.WalkStop, nil
			}
			off = end - len(tail)
			role := "live"
			switch {
			case agreementFieldContained(code, open, off):
				role = "code"
			case graph.EscapedWikilinkAt(body, open):
				role = "escape"
			case strings.ContainsAny(inner, "\r\n"):
				role = "wrapped"
			case open > 0 && body[open-1] == '!':
				if _, external := graph.ParseWikilink(inner); !external {
					r.Eligible = false
					return ast.WalkStop, nil
				}
				role = "unwritten-embed"
			}
			r.Fields = append(r.Fields, agreementMixedNamespaceField{Span: graph.Span{Start: open, Stop: off}, Role: role})
			if role != "live" {
				copy(masked[open:open+2], "{{")
				copy(masked[off-2:off], "}}")
			}
		}
		r.Bases = append(r.Bases, graph.SectionID(render.HeadingWords(string(h.Lines().Value(masked)))))
		r.Checks = append(r.Checks, graph.SectionID(render.HeadingWords(string(h.Lines().Value(source)))))
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return r
}
func agreementMixedNamespaceAllocate(r *agreementMixedNamespaceReading, actual *agreementHTML) map[string]bool {
	used, authored, selected := make(map[string]bool), make(map[string]bool), make(map[string]bool)
	var expected []string
	for _, check := range r.Checks {
		authored[check] = true
	}
	for _, base := range r.Bases {
		id := base
		for n := 2; used[id]; n++ {
			id = base + "-" + strconv.Itoa(n)
		}
		used[id] = true
		expected = append(expected, id)
		if !authored[id] {
			selected[id] = true
		}
	}
	if len(selected) == 0 || !cmp.Equal(expected, actual.Headings) {
		return nil
	}
	return selected
}

func agreementMixedNamespaceIDs(body string, actual *agreementHTML) map[string]bool {
	plain := agreementMixedNamespaceRead(body, agreementFootnoteGrammar)
	gfm := agreementMixedNamespaceRead(body, agreementExclusiveCodeGrammar)
	if !plain.Eligible || !gfm.Eligible || !cmp.Equal(plain, gfm) {
		return nil
	}
	return agreementMixedNamespaceAllocate(&plain, actual)
}

func TestAgreementMixedNamespaceSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       agreementMixedNamespaceReading
	}{
		{
			name: "code and live alias",
			body: "## `[[B|alias]]` [[A|shown]]\n",
			want: agreementMixedNamespaceReading{
				Bases: []string{"b-alias-shown"}, Checks: []string{"alias-shown"},
				Headings: []graph.Span{{Start: 3, Stop: 28}}, Code: []graph.Span{{Start: 4, Stop: 15}},
				Fields: []agreementMixedNamespaceField{{Span: graph.Span{Start: 4, Stop: 15}, Role: "code"}, {Span: graph.Span{Start: 17, Stop: 28}, Role: "live"}}, Eligible: true,
			},
		},
		{
			name: "nested code keeps its owner",
			body: "## *`[[B|alias]]`* [[A|shown]]\n",
			want: agreementMixedNamespaceReading{
				Bases: []string{"b-alias-shown"}, Checks: []string{"alias-shown"},
				Headings: []graph.Span{{Start: 3, Stop: 30}}, Code: []graph.Span{{Start: 5, Stop: 16}},
				Fields: []agreementMixedNamespaceField{{Span: graph.Span{Start: 5, Stop: 16}, Role: "code"}, {Span: graph.Span{Start: 19, Stop: 30}, Role: "live"}}, Eligible: true,
			},
		},
		{
			name: "escape and live alias",
			body: "## \\[[B|alias]] [[A|shown]]\n",
			want: agreementMixedNamespaceReading{
				Bases: []string{"b-alias-shown"}, Checks: []string{"alias-shown"}, Headings: []graph.Span{{Start: 3, Stop: 27}},
				Fields: []agreementMixedNamespaceField{{Span: graph.Span{Start: 4, Stop: 15}, Role: "escape"}, {Span: graph.Span{Start: 16, Stop: 27}, Role: "live"}}, Eligible: true,
			},
		},
		{
			name: "unwritten embed and live alias",
			body: "## ![[B|alias]] [[A|shown]]\n",
			want: agreementMixedNamespaceReading{
				Bases: []string{"b-alias-shown"}, Checks: []string{"alias-shown"}, Headings: []graph.Span{{Start: 3, Stop: 27}},
				Fields: []agreementMixedNamespaceField{{Span: graph.Span{Start: 4, Stop: 15}, Role: "unwritten-embed"}, {Span: graph.Span{Start: 16, Stop: 27}, Role: "live"}}, Eligible: true,
			},
		},
		{
			name: "wrapped field and live alias",
			body: "[[A\nB]]A [[B|shown]]\n===\n",
			want: agreementMixedNamespaceReading{
				Bases: []string{"a-b-a-shown"}, Checks: []string{"a-ba-shown"}, Headings: []graph.Span{{Start: 0, Stop: 20}},
				Fields: []agreementMixedNamespaceField{{Span: graph.Span{Start: 0, Stop: 7}, Role: "wrapped"}, {Span: graph.Span{Start: 9, Stop: 20}, Role: "live"}}, Eligible: true,
			},
		},
		{
			name: "whole container declaration set",
			body: "> ## `[[B|alias]]`\n> ## [[A|shown]]\n",
			want: agreementMixedNamespaceReading{
				Bases: []string{"b-alias", "shown"}, Checks: []string{"alias", "shown"}, Headings: []graph.Span{{Start: 5, Stop: 18}, {Start: 24, Stop: 35}}, Code: []graph.Span{{Start: 6, Stop: 17}},
				Fields: []agreementMixedNamespaceField{{Span: graph.Span{Start: 6, Stop: 17}, Role: "code"}, {Span: graph.Span{Start: 24, Stop: 35}, Role: "live"}}, Eligible: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				if diff := cmp.Diff(tc.want, agreementMixedNamespaceRead(tc.body, grammar)); diff != "" {
					t.Fatalf("caught: complete mixed namespace source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementMixedNamespace(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"## `[[B|alias]]`\n## [[A\n", "## `[[B|alias]]`\n## ![[#A]]\n", "## `[[B|alias]]`\n## [[A[[B]]]]\n"} {
		actual := agreementHTML{Headings: []string{"b-alias"}}
		if agreementMixedNamespaceIDs(body, &actual) != nil {
			t.Fatal("caught: mixed namespace borrowed incomplete source declarations")
		}
	}
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{name: "code and live alias", body: "## `[[B|alias]]` [[A|shown]]\n", want: []string{"b-alias-shown"}},
		{name: "live companion heading", body: "## `[[B|alias]]`\n## [[A|shown]]\n", want: []string{"b-alias"}},
		{name: "nested code owner", body: "## *`[[B|alias]]`* [[A|shown]]\n", want: []string{"b-alias-shown"}},
		{name: "escape and live alias", body: "## \\[[B|alias]] [[A|shown]]\n", want: []string{"b-alias-shown"}},
		{name: "embed and live alias", body: "## ![[B|alias]] [[A|shown]]\n", want: []string{"b-alias-shown"}},
		{name: "wrapped field and live alias", body: "[[A\nB]]A [[B|shown]]\n===\n", want: []string{"a-b-a-shown"}},
		{name: "authored HTML through public words", body: "## <em>A</em> `[[B|alias]]`\n", want: []string{"em-a-em-b-alias"}},
		{name: "decoded prose through public words", body: "## &amp; `[[B|alias]]`\n", want: []string{"b-alias"}},
		{name: "literal displaces authored name", body: "## `[[B|alias]]`\n## B-Alias\n## [[A|shown]]\n", want: []string{"b-alias-2"}},
		{name: "authored name reserves literal id", body: "## B-Alias\n## `[[B|alias]]`\n## [[A|shown]]\n", want: []string{"b-alias-2"}},
		{name: "authored suffix participates", body: "## `[[B|alias]]`\n## B-Alias-2\n## `[[B|alias]]`\n## [[A|shown]]\n", want: []string{"b-alias", "b-alias-3"}},
		{name: "complete literal repetition", body: "## `[[B|alias]]`\n## `[[B|alias]]`\n## [[A|shown]]\n", want: []string{"b-alias", "b-alias-2"}},
		{name: "canonical source words", body: "## `[[É|reading]]`\n## `[[É|reading]]`\n## [[A|shown]]\n", want: []string{"é-reading", "é-reading-2"}},
		{name: "quoted declarations", body: "> ## `[[B|alias]]`\n> ## [[A|shown]]\n", want: []string{"b-alias"}},
		{name: "used footnote declarations", body: "ref[^n]\n\n[^n]: ## `[[B|alias]]`\n\n    ## [[A|shown]]\n", want: []string{"b-alias"}},
		{name: "independent comment stays outside", body: "## `[[B|alias]]`\n<!--\n## A\n-->\n", want: []string{"b-alias"}},
		{name: "ordinary repetition with live alias", body: "## A\n## A\n## [[B|shown]]\n", want: []string{"a-2"}},
		{name: "both identical field targets", body: "## `[[B|alias]]` [[B|alias]]\n", want: []string{"b-alias-alias"}},
		{name: "embed beside later live alias", body: "![[A]]A\n---\n## [[A|alias]]\n", want: []string{"a-a"}},
		{name: "recorded full embed and live alias body", body: "![[A]]A\n---\n## [[A|alias]]\n  ```\n# A\n## <em>A</em>\n0A\n````\n\n  > [!note] title\n\\\\[[A]]  > [!note] title\nÉ\n", want: []string{"a-a"}},
		{name: "different code ownership refuses whole set", body: "## `[[B|alias]]`\n## https://example.invalid/`[[B|alias]]`\n"},
		{name: "different declaration grammar refuses whole set", body: "| a | b |\n|---|---|\n| row | value |\n---\n## `[[B|alias]]`\n"},
		{name: "local embed refuses whole set", body: "## ![[#A]] `[[B|alias]]`\n"},
		{name: "incomplete field refuses whole set", body: "## `[[B|alias]]` [[A\n"},
		{name: "nested field refuses whole set", body: "## `[[B|alias]]` [[A[[B]]]]\n"},
		{name: "hidden declaration refuses whole set", body: "## `[[B|alias]]`\n%%\n## A\n%%\n"},
		{name: "hidden literal refuses whole set", body: "%%\n## `[[B|alias]]`\n%%\n"},
		{name: "even escapes stay live", body: "## \\\\[[B|alias]] [[A|shown]]\n"},
		{name: "link words have another owner", body: "## [label](target) `[[B|alias]]`\n"},
		{name: "footnote suffix has another owner", body: "## `[[B|alias]]`[^n]\n\n[^n]: words\n"},
		{name: "unused footnote has no declaration", body: "[^n]: ## `[[B|alias]]`\n\n    ## [[A|shown]]\n"},
		{name: "ordinary name stays separate", body: "## A\n"},
		{name: "ordinary live field stays separate", body: "## [[B|alias]]\n"},
		{name: "code block declares no heading", body: "```\n## `[[B|alias]]`\n## [[A|shown]]\n```\n"},
		{name: "unpaired ticks are ordinary source", body: "## `open\n[[B|alias]]\nclose`A\n---\n## [[A|shown]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			_, actual := agreementIsolatedPage(t, c)
			var want map[string]bool
			if tc.want != nil {
				want = make(map[string]bool)
				for _, id := range tc.want {
					want[id] = true
				}
			}
			ids := agreementMixedNamespaceIDs(tc.body, &actual)
			if diff := cmp.Diff(want, ids); diff != "" {
				t.Fatalf("caught: complete mixed namespace public inventory (-want +got):\n%s", diff)
			}
			if want != nil {
				agreementMixedNamespacePageDrift(t, tc.body, &actual)
			}
			var found map[string]bool
			for _, f := range agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})[0] {
				kind, authority, wrong := agreementWrappedHeadingDifference(c, &f, ids)
				if !want[f.Fragment] {
					if kind != "" {
						t.Fatal("caught: mixed namespace borrowed unowned public name")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 8" || wrong != "judge" {
					t.Fatalf("caught: mixed namespace public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[string]bool)
				}
				found[f.Fragment] = true
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
					if kind, _, _ := agreementWrappedHeadingDifference(other, &f, ids); kind != "" {
						t.Fatal("caught: mixed namespace borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.PagePresent = false }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementWrappedHeadingDifference(c, &changed, ids); kind != "" {
						t.Fatalf("caught: mixed namespace borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			if diff := cmp.Diff(want, found); diff != "" {
				t.Fatalf("caught: complete mixed namespace public receipts (-want +got):\n%s", diff)
			}
		})
	}
}

func agreementMixedNamespacePageDrift(t *testing.T, body string, actual *agreementHTML) {
	t.Helper()
	for _, drift := range [][]string{nil, actual.Headings[:len(actual.Headings)-1], append(slices.Clone(actual.Headings), "unowned")} {
		changed := *actual
		changed.Headings = drift
		if agreementMixedNamespaceIDs(body, &changed) != nil {
			t.Fatal("caught: mixed namespace borrowed incomplete or extra page names")
		}
	}
	changed := *actual
	changed.Headings = slices.Clone(actual.Headings)
	changed.Headings[0] += "-unowned"
	if agreementMixedNamespaceIDs(body, &changed) != nil {
		t.Fatal("caught: mixed namespace borrowed same-cardinality page drift")
	}
	if len(actual.Headings) > 1 {
		changed.Headings = slices.Clone(actual.Headings)
		changed.Headings[0], changed.Headings[1] = changed.Headings[1], changed.Headings[0]
		if agreementMixedNamespaceIDs(body, &changed) != nil {
			t.Fatal("caught: mixed namespace borrowed reordered page names")
		}
	}
}
