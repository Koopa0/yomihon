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

// Literal text and code retain their declared words. Every heading must name
// the same complete namespace on the page before those words can own a missing
// check name, including a later declaration displaced by an earlier literal.
type agreementLiteralHeadingNames struct {
	Bases      []string
	CheckNames map[string]bool
	Eligible   bool
}

func agreementLiteralNamespaceReading(body string, grammar goldmark.Markdown) agreementLiteralHeadingNames {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := grammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var bases []string
	authored := make(map[string]bool)
	eligible, different := true, false
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		heading, ok := node.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		words, literal := agreementLiteralNamespaceWords(heading, source)
		if !literal || !agreementLiteralNamespaceFields(heading, source) {
			eligible = false
			return ast.WalkStop, nil
		}
		base := graph.SectionID(words)
		check := graph.SectionID(render.HeadingWords(string(heading.Lines().Value(source))))
		bases = append(bases, base)
		authored[check] = true
		different = different || base != check
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	if !eligible || !different {
		return agreementLiteralHeadingNames{}
	}
	return agreementLiteralHeadingNames{Bases: bases, CheckNames: authored, Eligible: true}
}

func agreementLiteralNamespaceIDs(body string, actual *agreementHTML) map[string]bool {
	plain := agreementLiteralNamespaceReading(body, agreementFootnoteGrammar)
	gfm := agreementLiteralNamespaceReading(body, agreementExclusiveCodeGrammar)
	if !plain.Eligible || !gfm.Eligible || !cmp.Equal(plain, gfm) {
		return nil
	}
	var expected []string
	used, selected := make(map[string]bool), make(map[string]bool)
	for _, base := range plain.Bases {
		id := base
		for ordinal := 2; used[id]; ordinal++ {
			id = base + "-" + strconv.Itoa(ordinal)
		}
		used[id] = true
		expected = append(expected, id)
		if !plain.CheckNames[id] {
			selected[id] = true
		}
	}
	if len(selected) == 0 || !cmp.Equal(expected, actual.Headings) {
		return nil
	}
	return selected
}

func agreementLiteralNamespaceFields(heading *ast.Heading, source []byte) bool {
	if heading.Lines().Len() == 0 {
		return false
	}
	var code []graph.Span
	for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
		if span, ok := child.(*ast.CodeSpan); ok && span.FirstChild() != nil {
			first, firstOK := span.FirstChild().(*ast.Text)
			last, lastOK := span.LastChild().(*ast.Text)
			if !firstOK || !lastOK {
				return false
			}
			code = append(code, graph.Span{Start: first.Segment.Start, Stop: last.Segment.Stop})
		}
	}
	body := string(source)
	end := heading.Lines().At(heading.Lines().Len() - 1).Stop
	for off := heading.Lines().At(0).Start; off < end; {
		rel := strings.Index(body[off:end], "[[")
		if rel < 0 {
			break
		}
		start := off + rel
		inner, tail, closed := strings.Cut(body[start+2:end], "]]")
		if !closed || strings.ContainsAny(inner, "[]") {
			return false
		}
		off = end - len(tail)
		if agreementFieldContained(code, start, off) || graph.EscapedWikilinkAt(body, start) || strings.ContainsAny(inner, "\r\n") {
			continue
		}
		if start > 0 && body[start-1] == '!' {
			if _, external := graph.ParseWikilink(inner); external {
				continue
			}
		}
		return false
	}
	return true
}

func agreementLiteralNamespaceWords(heading *ast.Heading, source []byte) (string, bool) {
	var words strings.Builder
	for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *ast.Text:
			if strings.ContainsAny(string(node.Value(source)), "&<>") {
				return "", false
			}
			words.Write(node.Value(source))
			if node.SoftLineBreak() || node.HardLineBreak() {
				words.WriteByte('\n')
			}
		case *ast.CodeSpan:
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				literal, ok := child.(*ast.Text)
				if !ok {
					return "", false
				}
				words.WriteString(strings.ReplaceAll(string(literal.Value(source)), "\n", " "))
			}
		default:
			return "", false
		}
	}
	return words.String(), true
}

func TestAgreementLiteralNamespace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		ids        []string
	}{
		{name: "later live field cannot borrow code words", body: "## `[[B|alias]]` [[A|shown]]\n", ids: []string{"b-alias-a-shown"}},
		{name: "other grammar cannot borrow matching page names", body: "| a | b |\n|---|---|\n| row | value |\n---\n\n## `[[B|alias]]`\n", ids: []string{"a-b-row-value", "b-alias"}},
		{name: "local embed cannot borrow literal names", body: "## ![[#A]] `[[B|alias]]`\n", ids: []string{"a-b-alias"}},
		{name: "decoded text cannot borrow raw names", body: "## &amp; `[[B|alias]]`\n", ids: []string{"amp-b-alias"}},
		{name: "live field cannot borrow literal page words", body: "## [[B|alias]]\n", ids: []string{"b-alias"}},
		{name: "page grammar cannot borrow common code", body: "## https://example.invalid/`[[B|alias]]`\n", ids: []string{"https-example-invalid-b-alias"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := agreementHTML{Headings: tc.ids}
			if agreementLiteralNamespaceIDs(tc.body, &actual) != nil {
				t.Fatal("caught: literal namespace borrowed another field owner")
			}
		})
	}

	for _, tc := range []struct {
		name, body string
		want       map[string]bool
	}{
		{name: "code beside plain words", body: "## before `[[B|alias]]` after\n", want: map[string]bool{"before-b-alias-after": true}},
		{name: "all code word owners", body: "## `[[B|alias]]` `[[A|shown]]`\n", want: map[string]bool{"b-alias-a-shown": true}},
		{name: "all literal declarations", body: "## `[[B|alias]]`\n## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true, "b-alias-2": true}},
		{name: "ordinary declaration participates", body: "## A\n## before `[[B|alias]]` after\n", want: map[string]bool{"before-b-alias-after": true}},
		{name: "literal displaces ordinary name", body: "## `[[B|alias]]`\n## B-Alias\n", want: map[string]bool{"b-alias-2": true}},
		{name: "ordinary name reserves literal id", body: "## B-Alias\n## `[[B|alias]]`\n", want: map[string]bool{"b-alias-2": true}},
		{name: "authored suffix and complete collision set", body: "## `[[B|alias]]`\n## B-Alias-2\n## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true, "b-alias-3": true}},
		{name: "code child source whitespace", body: "## before `A` after\n## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true}},
		{name: "literal blank code words", body: "## `   `\n## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true}},
		{name: "independent opener leaves words intact", body: "> [!note] words\n\n## before `[[B|alias]]` after\n", want: map[string]bool{"before-b-alias-after": true}},
		{name: "container source declarations", body: "> ## `[[B|alias]]`\n> ## `[[B|alias]]`\n", want: map[string]bool{"b-alias": true, "b-alias-2": true}},
		{name: "wrapped widget words", body: "[[A\nB]]A\n=\n", want: map[string]bool{"a-b-a": true}},
		{name: "wrapped code source words", body: "`open\n[[A]]\nclose`A\n---\n", want: map[string]bool{"open-a-closea": true}},
		{name: "canonical source names collide", body: "## `[[É|reading]]`\n## `[[É|reading]]`\n", want: map[string]bool{"é-reading": true, "é-reading-2": true}},
		{name: "unwritten embed stays literal", body: "## ![[B|alias]]\n", want: map[string]bool{"b-alias": true}},
		{name: "even escapes do not declare literal fields", body: "## \\\\[[B|alias]]\n"},
		{name: "ordinary words agree", body: "## A\n"},
		{name: "ordinary repetition stays separate", body: "## A\n## A\n"},
		{name: "live alias breaks literal receipt", body: "## [[B|alias]]\n"},
		{name: "other live alias breaks whole receipt", body: "## `[[B|alias]]`\n## [[A|shown]]\n"},
		{name: "mixed alias breaks whole receipt", body: "## `[[B|alias]]` [[A|shown]]\n"},
		{name: "formatting has another word owner", body: "## *A*\n## `[[B|alias]]`\n"},
		{name: "unidentified later heading cannot be omitted", body: "## `[[B|alias]]`\n## <h2>A</h2>\n"},
		{name: "HTML has another word owner", body: "## <mark>A</mark>\n## `[[B|alias]]`\n"},
		{name: "link has another word owner", body: "## [A](x)\n## `[[B|alias]]`\n"},
		{name: "hidden heading breaks entire receipt", body: "%%\n## `[[B|alias]]`\n%%\n"},
		{name: "hidden unsupported declaration still participates", body: "## `[[B|alias]]`\n%%\n## *A*\n%%\n"},
		{name: "recorded escaped widget boundary", body: "\\[[A]]1.  A\n===\n ^a-2\n", want: map[string]bool{"a-1-a": true}},
		{name: "recorded wrapped code and independent markers", body: "`open\n[[A]]\nclose`A\n---\n<!-- [[A]] -->%%![[A#A]]- [ ] [[A]]\nhttps://example.invalid/`[[A]]`  ^é\nA\nref[^n]\n", want: map[string]bool{"open-a-closea": true}},
		{name: "partially hidden namespace breaks receipt", body: "## `[[B|alias]]`\n%%\n## A\n%%\n"},
		{name: "grammar declaration sets differ", body: "| a | b |\n|---|---|\n| row | value |\n---\n\n## `[[B|alias]]`\n"},
		{name: "incomplete plain field cannot borrow code", body: "## [[A `[[B|alias]]`\n"},
		{name: "nested plain field cannot borrow code", body: "## [[A[[B]]]] `[[B|alias]]`\n"},
		{name: "local embed has another owner", body: "## ![[#A]] `[[B|alias]]`\n"},
		{name: "block code declares no heading", body: "```\n## `[[B|alias]]`\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			_, actual := agreementIsolatedPage(t, c)
			ids := agreementLiteralNamespaceIDs(tc.body, &actual)
			if diff := cmp.Diff(tc.want, ids); diff != "" {
				t.Fatalf("caught: complete literal namespace source inventory (-want +got):\n%s", diff)
			}
			if tc.want == nil {
				return
			}
			for _, drift := range [][]string{nil, actual.Headings[:len(actual.Headings)-1], append(slices.Clone(actual.Headings), "unowned")} {
				changed := actual
				changed.Headings = drift
				if agreementLiteralNamespaceIDs(tc.body, &changed) != nil {
					t.Fatal("caught: literal namespace borrowed incomplete or extra page names")
				}
			}
			if len(actual.Headings) > 1 {
				changed := actual
				changed.Headings = slices.Clone(actual.Headings)
				changed.Headings[0], changed.Headings[1] = changed.Headings[1], changed.Headings[0]
				if agreementLiteralNamespaceIDs(tc.body, &changed) != nil {
					t.Fatal("caught: literal namespace borrowed reordered page names")
				}
			}
			found := make(map[string]bool)
			for _, f := range agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{actual})[0] {
				kind, authority, wrong := agreementWrappedHeadingDifference(c, &f, ids)
				if kind != "debt" || authority != "#1011 stage 8" || wrong != "judge" {
					t.Fatalf("caught: literal namespace public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(&f))
				}
				found[f.Fragment] = true
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "words"}}} {
					if kind, _, _ := agreementWrappedHeadingDifference(other, &f, ids); kind != "" {
						t.Fatal("caught: literal namespace borrowed vault context")
					}
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.PagePresent = false }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" },
				} {
					changed := f
					change(&changed)
					if kind, _, _ := agreementWrappedHeadingDifference(c, &changed, ids); kind != "" {
						t.Fatalf("caught: literal namespace borrowed signature %s", agreementSignature(&changed))
					}
				}
				if kind, _, _ := agreementWrappedHeadingDifference(c, &f, nil); kind != "" {
					t.Fatal("caught: literal namespace borrowed absent source")
				}
			}
			if diff := cmp.Diff(tc.want, found); diff != "" {
				t.Fatalf("caught: complete literal namespace public difference (-want +got):\n%s", diff)
			}
		})
	}
}
