package judge_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

type agreementUnusedWidgetField struct {
	Span  graph.Span
	Word  string
	Tuple agreementCitation
}

// A discarded declaration can share a target with visible prose. Retain every
// plain widget and its original position in both grammars; subtracting that
// complete contribution must leave the actual diagnostic and carrier counts
// equal. A missing reference declaration cannot borrow this contribution.
func agreementSharedUnusedWidgetReading(body string, grammar goldmark.Markdown) []agreementUnusedWidgetField {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	var unused []*extast.Footnote
	context.Set(agreementUnusedFootnoteNodesKey, &unused)
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	comments := agreementUnusedWidgetComments(body, doc)
	var fields []agreementUnusedWidgetField
	for _, definition := range unused {
		for node := definition.FirstChild(); node != nil; node = node.NextSibling() {
			switch node.(type) {
			case *ast.Paragraph, *ast.TextBlock:
			default:
				return nil
			}
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if _, plain := child.(*ast.Text); !plain {
					return nil
				}
			}
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				raw := string(source[line.Start:line.Stop])
				words := strings.Fields(raw)
				if len(words) == 0 {
					return nil
				}
				offset := 0
				for _, word := range words {
					start := offset + strings.Index(raw[offset:], word)
					offset = start + len(word)
					widget := strings.TrimPrefix(word, "!")
					inner, open := strings.CutPrefix(widget, "[[")
					inner, closed := strings.CutSuffix(inner, "]]")
					if !open || !closed || strings.ContainsAny(inner, "[]\r\n`") {
						return nil
					}
					link, cites := graph.ParseWikilink(inner)
					if !cites || link.Block != "" {
						return nil
					}
					field := agreementUnusedWidgetField{
						Span:  graph.Span{Start: line.Start + start, Stop: line.Start + offset},
						Word:  word,
						Tuple: agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"},
					}
					if agreementUnusedWidgetCommentOwned(field.Span, comments) {
						return nil
					}
					fields = append(fields, field)
				}
			}
		}
	}
	return fields
}

type agreementUnusedWidgetPayload struct {
	Body   string
	Tuples map[agreementCitation]int
}

func agreementSharedUnusedWidgetBudget(body string) agreementUnusedWidgetPayload {
	plain := agreementSharedUnusedWidgetReading(body, agreementFootnoteGrammar)
	gfm := agreementSharedUnusedWidgetReading(body, agreementExclusiveCodeGrammar)
	if !cmp.Equal(plain, gfm) || len(plain) == 0 {
		return agreementUnusedWidgetPayload{}
	}
	budget := make(map[agreementCitation]int)
	for _, field := range plain {
		budget[field.Tuple]++
	}
	return agreementUnusedWidgetPayload{Body: body, Tuples: budget}
}

func agreementSharedUnusedDiagnosticDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, diagnostics []render.Diagnostic, budget agreementUnusedWidgetPayload) (kind, authority, wrong string) {
	if c.Body != budget.Body || c.Title != "" || len(c.Companions) != 0 || f.Property != "P0" || f.Identity != "diagnostic-html" || f.Direction != "diagnostic-only" || f.Tuple.Target == "" || f.Tuple.SourceRole != "" || f.Tuple.State != "wikilink-broken" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 || budget.Tuples[f.Tuple] != f.Multiplicity {
		return "", "", ""
	}
	diagnosticCount, pageCount := 0, 0
	for _, d := range diagnostics {
		if d.Kind == render.DiagWikilinkBroken && d.Target == f.Tuple.Target && d.Section == f.Tuple.Section {
			if d.Block != "" {
				return "", "", ""
			}
			diagnosticCount++
		}
	}
	for _, tuple := range actual.Citations {
		if tuple == f.Tuple {
			pageCount++
		}
	}
	if diagnosticCount-budget.Tuples[f.Tuple] != pageCount {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "page-diagnostic"
}

func TestAgreementSharedUnusedWidgetSource(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}
	raw := agreementCitation{Target: "B\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		want       []agreementUnusedWidgetField
	}{
		{name: "shared prose keeps complete field positions", body: "[[A]]\n\n[^n]: [[A]] [[A#part|shown]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 13, Stop: 18}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 19, Stop: 35}, Word: "[[A#part|shown]]", Tuple: section}}},
		{name: "all definitions paragraphs raw words and embeds", body: "[^n]: ![[A#part|shown]] [[B]]\n\n    [[A]]\n\n[^m]: [[B\\]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 23}, Word: "![[A#part|shown]]", Tuple: section}, {Span: graph.Span{Start: 24, Stop: 29}, Word: "[[B]]", Tuple: b}, {Span: graph.Span{Start: 35, Stop: 40}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 48, Stop: 54}, Word: "[[B\\]]", Tuple: raw}}},
		{name: "all physical rows of one paragraph", body: "[^n]: [[A]]\n[[B]]\n", want: []agreementUnusedWidgetField{{Span: graph.Span{Start: 6, Stop: 11}, Word: "[[A]]", Tuple: a}, {Span: graph.Span{Start: 12, Stop: 17}, Word: "[[B]]", Tuple: b}}},
		{name: "used declaration retains its field", body: "ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "ordinary field has no discarded owner", body: "[[A]]\n"},
		{name: "prose words refuse entire declaration", body: "[^n]: words [[A]]\n"},
		{name: "formatted words refuse entire declaration", body: "[^n]: *[[A]]*\n"},
		{name: "inline code refuses entire declaration", body: "[^n]: `[[A]]`\n"},
		{name: "reference-owned brackets refuse entire declaration", body: "[A]: /elsewhere\n\n[^n]: [[A]]\n"},
		{name: "HTML comment refuses entire declaration", body: "[^n]: <!-- [[A]] -->\n"},
		{name: "heading refuses every earlier field", body: "[^n]: [[A]]\n\n    ## [[B]]\n"},
		{name: "fence refuses every earlier field", body: "[^n]: [[A]]\n\n    ```\n    [[B]]\n    ```\n"},
		{name: "ordinary suffix refuses entire line", body: "[^n]: [[A]]after\n"},
		{name: "incomplete field refuses earlier fields", body: "[^n]: [[A]] [[B\n"},
		{name: "nested field refuses earlier fields", body: "[^n]: [[A]] [[B[[C]]]]\n"},
		{name: "local field has another owner", body: "[^n]: [[#A]]\n"},
		{name: "block field has another owner", body: "[^n]: [[A#^a]]\n"},
		{name: "empty target has no widget", body: "[^n]: [[]]\n"},
		{name: "escaped field has another owner", body: "[^n]: \\[[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, grammar := range []goldmark.Markdown{agreementFootnoteGrammar, agreementExclusiveCodeGrammar} {
				got := agreementSharedUnusedWidgetReading(tc.body, grammar)
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Fatalf("caught: complete shared unused widget source inventory (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestAgreementSharedUnusedDiagnostics(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	section := agreementCitation{Target: "A", Section: "part", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		budget     map[agreementCitation]int
		want       map[agreementCitation]int
	}{
		{name: "shared live same target", body: "[[A]]\n\n[^n]: [[A]] [[A]] [[B]]\n", budget: map[agreementCitation]int{a: 2, b: 1}, want: map[agreementCitation]int{a: 2, b: 1}},
		{name: "every visible occurrence", body: "[[A]] [[A]] [[B]]\n\n[^n]: [[A]] [[B]]\n", budget: map[agreementCitation]int{a: 1, b: 1}, want: map[agreementCitation]int{a: 1, b: 1}},
		{name: "every source definition", body: "[[A]] [[B]]\n\n[^n]: [[A]]\n\n[^m]: [[A]] [[B]]\n", budget: map[agreementCitation]int{a: 2, b: 1}, want: map[agreementCitation]int{a: 2, b: 1}},
		{name: "all diagnostic sections", body: "[[A#part]]\n\n[^n]: [[A#part]] [[A]]\n", budget: map[agreementCitation]int{section: 1, a: 1}, want: map[agreementCitation]int{section: 1, a: 1}},
		{name: "embeds retain raw alias spelling", body: "[[A]]\n\n[^n]: ![[A#part|shown]] [[B]]\n", budget: map[agreementCitation]int{section: 1, b: 1}, want: map[agreementCitation]int{section: 1, b: 1}},
		{name: "used and unused share a live target", body: "ref[^u]\n\n[^u]: [[A]]\n\n[^n]: [[A]]\n", budget: map[agreementCitation]int{a: 1}, want: map[agreementCitation]int{a: 1}},
		{name: "whole count equation refuses other absence", body: "[r]: [[A]]\n\n[^n]: [[A]]\n", budget: map[agreementCitation]int{a: 1}},
		{name: "page grammar alone retains the reference", body: "https://example.invalid/` words ref[^n]\nclose`\n\n[^n]: [[A]]\n"},
		{name: "grammar disagrees on use", body: "https://example.invalid/ref[^n]\n\n[^n]: [[A]]\n"},
		{name: "hidden source produces no diagnostic", body: "%%\n[^n]: [[A]]\n%%\n"},
		{name: "recorded full original source", body: "[^unused]: [[A]]\n> [!note] [[A]]\n章節\n", budget: map[agreementCitation]int{a: 1}, want: map[agreementCitation]int{a: 1}},
		{name: "ordinary prose remains outside", body: "[[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementSharedUnusedWidgetBudget(c.Body)
			if diff := cmp.Diff(tc.budget, budget.Tuples); diff != "" {
				t.Fatalf("caught: complete shared unused widget budget (-want +got):\n%s", diff)
			}
			r, actual := agreementIsolatedPage(t, c)
			var found map[agreementCitation]int
			for _, f := range agreementPageFailures(c.Body, &r, &actual) {
				kind, authority, wrong := agreementSharedUnusedDiagnosticDifference(c, &f, &actual, r.Diagnostics, budget)
				if f.Property != "P0" || tc.want[f.Tuple] == 0 {
					if kind != "" {
						t.Fatal("caught: shared unused diagnostic borrowed unowned public difference")
					}
					continue
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page-diagnostic" {
					t.Fatalf("caught: shared unused diagnostic ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				if found == nil {
					found = make(map[agreementCitation]int)
				}
				found[f.Tuple] = f.Multiplicity
				agreementSharedUnusedDiagnosticDrift(t, c, &f, &actual, r.Diagnostics, budget)
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete shared unused diagnostic public receipts (-want +got):\n%s", diff)
			}
		})
	}
}

func agreementSharedUnusedDiagnosticDrift(t *testing.T, c agreementCase, f *agreementFailure, actual *agreementHTML, diagnostics []render.Diagnostic, budget agreementUnusedWidgetPayload) {
	t.Helper()
	for _, other := range []agreementCase{{Body: c.Body + "unowned"}, {Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
		if kind, _, _ := agreementSharedUnusedDiagnosticDifference(other, f, actual, diagnostics, budget); kind != "" {
			t.Fatal("caught: shared unused diagnostic borrowed source or vault context")
		}
	}
	for _, change := range []func(*agreementFailure){func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }} {
		changed := *f
		change(&changed)
		if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, &changed, actual, diagnostics, budget); kind != "" {
			t.Fatalf("caught: shared unused diagnostic borrowed signature %s", agreementSignature(&changed))
		}
	}
	changed := *actual
	changed.Citations = append(append([]agreementCitation{}, actual.Citations...), f.Tuple)
	if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, f, &changed, diagnostics, budget); kind != "" {
		t.Fatal("caught: shared unused diagnostic borrowed current page count")
	}
	var surviving []agreementCitation
	removed := false
	for _, tuple := range actual.Citations {
		if tuple == f.Tuple && !removed {
			removed = true
			continue
		}
		surviving = append(surviving, tuple)
	}
	changed.Citations = surviving
	kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, f, &changed, diagnostics, budget)
	if removed && kind != "" || !removed && kind != "debt" {
		t.Fatal("caught: shared unused diagnostic lost selected carrier count scope")
	}
	changed = *actual
	changed.Citations = append(append([]agreementCitation{}, actual.Citations...), agreementCitation{SourceRole: agreementOutsideMarkdown, Target: f.Tuple.Target, Section: f.Tuple.Section, State: "wikilink-broken"}, agreementCitation{Target: f.Tuple.Target, Section: "unowned", State: "wikilink-broken"}, agreementCitation{Target: f.Tuple.Target, Section: f.Tuple.Section, State: "wikilink"})
	if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, f, &changed, diagnostics, budget); kind != "debt" {
		t.Fatal("caught: shared unused diagnostic counted another carrier tuple")
	}
	for _, records := range [][]render.Diagnostic{nil, append(append([]render.Diagnostic{}, diagnostics...), render.Diagnostic{Kind: render.DiagWikilinkBroken, Target: f.Tuple.Target, Section: f.Tuple.Section})} {
		if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, f, actual, records, budget); kind != "" {
			t.Fatal("caught: shared unused diagnostic borrowed current diagnostic count")
		}
	}
	for _, extra := range []render.Diagnostic{{Kind: render.DiagMarkdownBroken, Target: f.Tuple.Target, Section: f.Tuple.Section}, {Kind: render.DiagWikilinkBroken, Target: "unowned", Section: f.Tuple.Section}, {Kind: render.DiagWikilinkBroken, Target: f.Tuple.Target, Section: "unowned"}} {
		changed := append(append([]render.Diagnostic{}, diagnostics...), extra)
		if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, f, actual, changed, budget); kind != "debt" {
			t.Fatal("caught: shared unused diagnostic counted another diagnostic tuple")
		}
	}
	changedRecords := append([]render.Diagnostic{}, diagnostics...)
	selected := false
	for i, d := range changedRecords {
		if d.Kind == render.DiagWikilinkBroken && d.Target == f.Tuple.Target && d.Section == f.Tuple.Section {
			changedRecords[i].Block = "a"
			selected = true
			break
		}
	}
	if !selected {
		t.Fatal("caught: shared unused diagnostic drift lacks an owned diagnostic")
	}
	if kind, _, _ := agreementSharedUnusedDiagnosticDifference(c, f, actual, changedRecords, budget); kind != "" {
		t.Fatal("caught: shared unused diagnostic borrowed a block diagnostic")
	}
}
