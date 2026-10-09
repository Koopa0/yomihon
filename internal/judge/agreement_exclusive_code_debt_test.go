package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/graph"
)

// Both grammars must declare the same literal ownership. A target also
// written elsewhere cannot lend its code occurrence to another disagreement.
var agreementExclusiveCodeGrammar = goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Footnote), goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(agreementFootnoteDeclarations{}, 998))))

func agreementExclusiveCodeZones(source []byte, grammar goldmark.Markdown) []graph.Span {
	ctx := parser.NewContext()
	ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
	var zones []graph.Span
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.CodeSpan:
			first, ok1 := n.FirstChild().(*ast.Text)
			last, ok2 := n.LastChild().(*ast.Text)
			if ok1 && ok2 && strings.Contains(string(source[first.Segment.Start:last.Segment.Stop]), "\n") {
				zones = append(zones, graph.Span{Start: first.Segment.Start, Stop: last.Segment.Stop})
			}
		case *ast.FencedCodeBlock:
			contained := false
			for parent := node.Parent(); parent != nil; parent = parent.Parent() {
				switch parent.(type) {
				case *ast.ListItem, *ast.Blockquote:
					contained = true
				}
			}
			if contained {
				for i := range node.Lines().Len() {
					line := node.Lines().At(i)
					zones = append(zones, graph.Span{Start: line.Start, Stop: line.Stop})
				}
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return zones
}
func agreementFieldContained(zones []graph.Span, start, stop int) bool {
	for _, zone := range zones {
		if start >= zone.Start && stop <= zone.Stop {
			return true
		}
	}
	return false
}

func agreementExclusiveCodeBudget(body string) map[agreementCitation]int {
	source := []byte(body)
	plain := agreementExclusiveCodeZones(source, agreementFootnoteGrammar)
	gfm := agreementExclusiveCodeZones(source, agreementExclusiveCodeGrammar)
	return agreementExclusiveFieldBudget(body, plain, gfm)
}

func agreementExclusiveFieldBudget(body string, plain, gfm []graph.Span) map[agreementCitation]int {
	all, owned := make(map[string]int), make(map[string]int)
	tuples := make(map[agreementCitation]int)
	for off := 0; off < len(body); {
		rel := strings.Index(body[off:], "[[")
		if rel < 0 {
			break
		}
		start := off + rel
		inner, tail, closed := strings.Cut(body[start+2:], "]]")
		if !closed || strings.ContainsAny(inner, "[]\r\n") {
			return nil
		}
		off = len(body) - len(tail)
		link, cites := graph.ParseWikilink(inner)
		if !cites {
			continue
		}
		if link.Block != "" {
			return nil
		}
		all[link.Target]++
		if agreementFieldContained(plain, start, off) && agreementFieldContained(gfm, start, off) {
			owned[link.Target]++
			tuples[agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}]++
		}
	}
	for tuple := range tuples {
		if all[tuple.Target] != owned[tuple.Target] {
			delete(tuples, tuple)
		}
	}
	if len(tuples) == 0 {
		return nil
	}
	return tuples
}
func agreementExclusiveCodeDifference(c agreementCase, f *agreementFailure, budget map[agreementCitation]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P2" || f.Identity != "wikilink-in-code" || f.Direction != "page-in-code" || f.Tuple.Target == "" || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	if budget[f.Tuple] != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "page"
}

func TestAgreementExclusiveCode(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	raw := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       map[agreementCitation]int
		public     bool
	}{
		{name: "whole code set", body: "`open\n[[A]] [[A]] [[B|alias]]\nclose`\n", want: map[agreementCitation]int{a: 2, b: 1}, public: true},
		{name: "raw and section tuples", body: "`open\n[[A#A]] [[A\\]]\nclose`\n", want: map[agreementCitation]int{section: 1, raw: 1}, public: true},
		{name: "container fence", body: "> ```\n> [[A]] [[A]] [[B]]\n> ```\n", want: map[agreementCitation]int{a: 2, b: 1}, public: true},
		{name: "separate code declarations", body: "`open\n[[A]]\nclose`\n\n`open\n[[B]]\nclose`\n", want: map[agreementCitation]int{a: 1, b: 1}, public: true},
		{name: "literal embed", body: "`open\n![[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}, public: true},
		{name: "remaining target beside live field", body: "`open\n[[A]] [[B]]\nclose`\n\n[[A]]\n", want: map[agreementCitation]int{b: 1}, public: true},
		{name: "separate live target", body: "[[B]]\n\n`open\n[[A]]\nclose`\n", want: map[agreementCitation]int{a: 1}, public: true},
		{name: "recorded live markers elsewhere", body: " `open\n[[A]]\nclose`## !\n``- > [!note] title\n%%", want: map[agreementCitation]int{a: 1}, public: true},
		{name: "shown widget outside code", body: "\\[[A]]\n\n`open\n[[A]]\nclose`\n"},
		{name: "same target other section", body: "[[A#B]]\n\n`open\n[[A#A]]\nclose`\n"},
		{name: "same target reference field", body: "[n]: [[A]]\n\n`open\n[[A]]\nclose`\n"},
		{name: "same target embed outside", body: "![[A]]\n\n`open\n[[A]]\nclose`\n"},
		{name: "plain grammar only", body: "https://example.invalid/` words\n[[A]]\nclose`\n"},
		{name: "page grammar only", body: "https://example.invalid/` words `open\n[[A]]\nclose`\n"},
		{name: "root fence retains scope", body: "```\n[[A]]\n```\n"},
		{name: "single-line span retains scope", body: "`[[A]]`\n"},
		{name: "indented block retains scope", body: "    [[A]]\n"},
		{name: "field crosses code owners", body: "`open\n[[A` outside `B]]\nclose`\n"},
		{name: "nested field refuses inventory", body: "`open\n[[A[[B]]]] [[A]]\nclose`\n"},
		{name: "incomplete field refuses inventory", body: "`open\n[[B]] [[A\nclose`\n"},
		{name: "block field retains scope", body: "`open\n[[A#^a]] [[B]]\nclose`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementExclusiveCodeBudget(tc.body)
			if diff := cmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: exclusive code source inventory (-want +got):\n%s", diff)
			}
			if !tc.public {
				return
			}
			c := agreementCase{Body: tc.body}
			r, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &r, &actual)
			matched := 0
			for i := range failures {
				f := &failures[i]
				if f.Property != "P2" {
					continue
				}
				kind, authority, wrong := agreementExclusiveCodeDifference(c, f, budget)
				if budget[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: exclusive code borrowed unowned public tuple")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: exclusive code public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				matched++
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementExclusiveCodeDifference(other, f, budget); kind != "" {
						t.Fatal("caught: exclusive code borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := *f
					change(&changed)
					if kind, _, _ := agreementExclusiveCodeDifference(c, &changed, budget); kind != "" {
						t.Fatalf("caught: exclusive code borrowed signature %s", agreementSignature(&changed))
					}
				}
			}
			if matched != len(budget) {
				t.Fatalf("caught: exclusive code public set got=%d want=%d", matched, len(budget))
			}
		})
	}
}
