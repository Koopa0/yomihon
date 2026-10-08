package judge_test

import (
	"bytes"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

type agreementWidgetCodeBudget struct {
	Citations     map[agreementCitation]int
	LocalHeadings map[string]int
}

// One plain opener owns its title delimiter, while wrapped code in that quote
// owns its own source segments. Other opener and reference owners stay separate.
func agreementPlainCalloutCodeOwner(source []byte, doc ast.Node) *ast.Blockquote {
	if strings.Contains(string(source), "[^") {
		return nil
	}
	var owner *ast.Blockquote
	start := 0
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		quote, quoted := node.(*ast.Blockquote)
		if !quoted {
			continue
		}
		paragraph, prose := quote.FirstChild().(*ast.Paragraph)
		if !prose || paragraph.Lines().Len() == 0 {
			continue
		}
		line := paragraph.Lines().At(0)
		physicalStart := bytes.LastIndexByte(source[:line.Start], '\n') + 1
		if strings.TrimSpace(string(source[physicalStart:line.Start])) != ">" {
			continue
		}
		raw := string(line.Value(source))
		title := strings.TrimSpace(raw)
		closeAt := strings.IndexByte(title, ']')
		if !strings.HasPrefix(title, "[!") || closeAt <= 2 || strings.ContainsAny(title[2:closeAt], " \t[") {
			continue
		}
		rest := title[closeAt+1:]
		if rest != "" && !strings.HasPrefix(rest, " ") {
			continue
		}
		if strings.ContainsFunc(rest, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && !unicode.IsSpace(r)
		}) {
			continue
		}
		at := line.Start + len(raw) - len(strings.TrimLeft(raw, " \t"))
		stop := line.Start + len(strings.TrimRight(raw, "\r\n"))
		cursor := at
		plainTitle := true
		for child := paragraph.FirstChild(); child != nil; child = child.NextSibling() {
			plain, textual := child.(*ast.Text)
			if !textual {
				if cursor < stop {
					plainTitle = false
				}
				continue
			}
			lo, hi := max(at, plain.Segment.Start), min(stop, plain.Segment.Stop)
			if hi <= lo {
				continue
			}
			if lo != cursor {
				plainTitle = false
			}
			cursor = hi
		}
		if !plainTitle || cursor != stop {
			continue
		}
		if owner != nil {
			return nil
		}
		owner, start = quote, at
	}
	if owner == nil {
		return nil
	}
	clean := bytes.Clone(source)
	clean[start], clean[start+1] = ' ', ' '
	if !agreementDeclarationMarkers(clean, doc) {
		return nil
	}
	return owner
}

func agreementCalloutWrappedCode(body string) agreementWidgetCodeBudget {
	return agreementDeclaredWidgetCode(body, true)
}

func agreementDeclaredWrappedCode(body string) agreementWidgetCodeBudget {
	return agreementDeclaredWidgetCode(body, false)
}

func agreementDeclaredWidgetCode(body string, callout bool) agreementWidgetCodeBudget {
	if strings.Contains(body, "://") {
		return agreementWidgetCodeBudget{}
	}
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	owner := doc
	if callout {
		quote := agreementPlainCalloutCodeOwner(source, doc)
		if quote == nil {
			return agreementWidgetCodeBudget{}
		}
		owner = quote
	} else if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return agreementWidgetCodeBudget{}
	}
	budget := agreementWidgetCodeBudget{Citations: make(map[agreementCitation]int), LocalHeadings: make(map[string]int)}
	if err := ast.Walk(owner, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var literals []string
		switch node := node.(type) {
		case *ast.CodeSpan:
			first, firstOK := node.FirstChild().(*ast.Text)
			last, lastOK := node.LastChild().(*ast.Text)
			if !firstOK || !lastOK || !strings.Contains(string(source[first.Segment.Start:last.Segment.Stop]), "\n") {
				return ast.WalkContinue, nil
			}
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if raw, textual := child.(*ast.Text); textual {
					literals = append(literals, string(raw.Value(source)))
				}
			}
		case *ast.FencedCodeBlock:
			if callout {
				return ast.WalkContinue, nil
			}
			for parent := node.Parent(); parent != nil; parent = parent.Parent() {
				_, list := parent.(*ast.ListItem)
				_, quote := parent.(*ast.Blockquote)
				if list || quote {
					literals = append(literals, string(node.Lines().Value(source)))
					break
				}
			}
		}
		for _, literal := range literals {
			for line := range strings.SplitSeq(literal, "\n") {
				field := strings.TrimSpace(line)
				inner, opened := strings.CutPrefix(field, "[[")
				inner, closed := strings.CutSuffix(inner, "]]")
				if !opened || !closed || strings.ContainsAny(inner, "[]\n") {
					continue
				}
				link, cites := graph.ParseWikilink(inner)
				if cites {
					budget.Citations[agreementCitation{Target: link.Target, Section: link.Heading, State: "wikilink-broken"}]++
				} else if link.Heading != "" && link.Block == "" {
					budget.LocalHeadings[link.Heading]++
				}
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if len(budget.Citations) == 0 && len(budget.LocalHeadings) == 0 {
		return agreementWidgetCodeBudget{}
	}
	return budget
}

func agreementWidgetCodeDifference(c agreementCase, f *agreementFailure, budget agreementWidgetCodeBudget, observed *agreementHTML) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Identity != "citation-occurrences" && f.Identity != "wikilink-in-code" || f.Fragment != "" || f.Cut != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	count := 0
	switch f.Property {
	case "P2":
		if f.Identity != "wikilink-in-code" || f.Direction != "page-in-code" {
			return "", "", ""
		}
		if f.Tuple == (agreementCitation{}) {
			for _, n := range budget.LocalHeadings {
				count += n
			}
		} else {
			count = budget.Citations[f.Tuple]
		}
	case "P1":
		if f.Identity != "citation-occurrences" || f.Direction != "page-only" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" {
			return "", "", ""
		}
		expected, actual := make(map[agreementCitation]int), make(map[agreementCitation]int)
		for tuple, n := range budget.Citations {
			if tuple.Target == f.Tuple.Target {
				expected[tuple] = n
				count += n
			}
		}
		for _, tuple := range observed.CodeCitations {
			if tuple.Target == f.Tuple.Target {
				actual[tuple]++
			}
		}
		if !cmp.Equal(expected, actual) {
			return "", "", ""
		}
	default:
		return "", "", ""
	}
	if count != f.Multiplicity {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "page"
}

func TestAgreementCalloutWrappedCode(t *testing.T) {
	t.Parallel()
	a := agreementCitation{Target: "A", State: "wikilink-broken"}
	b := agreementCitation{Target: "B", State: "wikilink-broken"}
	heading := agreementCitation{Target: "A", Section: "A", State: "wikilink-broken"}
	suffix := agreementCitation{Target: "A\\", State: "wikilink-broken"}
	for _, tc := range []struct {
		name, body string
		citations  map[agreementCitation]int
		local      map[string]int
		failures   int
	}{
		{name: "wrapped body", body: "> [!note] title\n> `open\n> [[A]]\n> close`\n", citations: map[agreementCitation]int{a: 1}, failures: 2},
		{name: "whole source set", body: "> [!note] title\n> `open\n> [[A]]\n> [[B]]\n> [[A]]\n> close`\n", citations: map[agreementCitation]int{a: 2, b: 1}, failures: 4},
		{name: "section belongs to raw widget", body: "> [!note] title\n> `open\n> [[A#A]]\n> close`\n", citations: map[agreementCitation]int{heading: 1}, failures: 2},
		{name: "suffix belongs to raw widget", body: "> [!note] title\n> `open\n> [[A\\]]\n> close`\n", citations: map[agreementCitation]int{suffix: 1}, failures: 2},
		{name: "local address remains code", body: "# A\n\n> [!note] title\n> `open\n> [[#A]]\n> close`\n", local: map[string]int{"A": 1}, failures: 1},
		{name: "whole local address set", body: "# A\n\n# B\n\n> [!note] title\n> `open\n> [[#A]]\n> [[#B]]\n> [[#A]]\n> close`\n", local: map[string]int{"A": 2, "B": 1}, failures: 1},
		{name: "fenced body already agrees", body: "> [!note] title\n> ```\n> [[A]]\n> ```\n"},
		{name: "single-line code already agrees", body: "> [!note] title\n> `[[A]]`\n"},
		{name: "ordinary prose is a different owner", body: "> [!note] title\n> [[A]]\n"},
		{name: "root code has a separate owner", body: "> [!note] title\n> words\n\n`open\n[[A]]\nclose`\n"},
		{name: "title link is separate", body: "> [!note] [[A]]\n> `open\n> [[A]]\n> close`\n"},
		{name: "title code is separate", body: "> [!note] `title`\n> `open\n> [[A]]\n> close`\n"},
		{name: "punctuated title stays separate", body: "> [!note] title / path\n> `open\n> [[A]]\n> close`\n"},
		{name: "reference owns opener text", body: "> [!note] title\n> `open\n> [[A]]\n> close`\n\n[!note]: /local\n"},
		{name: "nested opener is separate", body: "> > [!note] title\n> > `open\n> > [[A]]\n> > close`\n"},
		{name: "multiple live openers are separate", body: "> [!note] one\n> [!note] two\n> `open\n> [[A]]\n> close`\n"},
		{name: "reference relocation is separate", body: "> [^unused]: [[A]]\n> [!note] title\n> `open\n> [[A]]\n> close`\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := agreementCalloutWrappedCode(tc.body)
			want := agreementWidgetCodeBudget{Citations: tc.citations, LocalHeadings: tc.local}
			if len(tc.citations) > 0 && tc.local == nil {
				want.LocalHeadings = map[string]int{}
			}
			if len(tc.local) > 0 && tc.citations == nil {
				want.Citations = map[agreementCitation]int{}
			}
			if diff := cmp.Diff(want, budget); diff != "" {
				t.Fatalf("caught: plain opener code inventory (-want +got):\n%s", diff)
			}
			source := []byte(tc.body)
			original := bytes.Clone(source)
			context := parser.NewContext()
			context.Set(agreementFootnoteTargetsKey, make(map[string]int))
			doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
			owner := agreementPlainCalloutCodeOwner(source, doc)
			if tc.failures > 0 && owner == nil {
				t.Fatal("caught: plain opener code owner missing")
			}
			if !bytes.Equal(source, original) {
				t.Fatal("caught: plain opener code observer changed source bytes")
			}
			if tc.failures == 0 {
				return
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementPageFailures(tc.body, &result, &observed)
			if len(failures) != tc.failures {
				t.Fatalf("caught: plain opener public delta count=%d want=%d", len(failures), tc.failures)
			}
			for _, failure := range failures {
				for _, altered := range []agreementCase{
					{Body: tc.body, Title: "Reading"},
					{Body: tc.body, Companions: capturedBodies{"A": "body"}},
				} {
					if kind, _, _ := agreementWidgetCodeDifference(altered, &failure, budget, &observed); kind != "" {
						t.Fatal("caught: widget code delta borrowed a different note context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.Tuple.Target = "unowned" },
					func(f *agreementFailure) { f.Direction = "judge-only" },
					func(f *agreementFailure) { f.PagePresent = true },
					func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" },
					func(f *agreementFailure) { f.Tuple.Section = "unowned" },
					func(f *agreementFailure) { f.Tuple.State = "unowned" },
					func(f *agreementFailure) { f.Identity = "unowned" },
					func(f *agreementFailure) { f.Property = "unowned" },
					func(f *agreementFailure) { f.Fragment = "unowned" },
					func(f *agreementFailure) { f.Cut = "unowned" },
					func(f *agreementFailure) { f.JudgeAccepted = true },
					func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := failure
					change(&changed)
					if kind, _, _ := agreementWidgetCodeDifference(agreementCase{Body: tc.body}, &changed, budget, &observed); kind != "" {
						t.Fatalf("caught: unrelated opener code delta admitted: %+v", changed)
					}
				}
				kind, authority, wrong := agreementWidgetCodeDifference(agreementCase{Body: tc.body}, &failure, budget, &observed)
				if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
					t.Fatalf("caught: plain opener public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
				if failure.Property == "P1" {
					empty := observed
					empty.CodeCitations = nil
					if kind, _, _ := agreementWidgetCodeDifference(agreementCase{Body: tc.body}, &failure, budget, &empty); kind != "" {
						t.Fatal("caught: citation delta borrowed an absent code carrier")
					}
				}
			}
		})
	}
}
