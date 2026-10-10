package judge_test

import (
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// An unwritten embed shows its literal widget, including the alias separator.
// Every other heading still reserves its name in the same declaration order.
func agreementUnwrittenEmbedHeadingIDs(body string) map[string]bool {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	var headings []*ast.Heading
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := node.(*ast.Heading); entering && ok {
			headings = append(headings, heading)
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	used, selected := make(map[string]bool), make(map[string]bool)
	for _, heading := range headings {
		if _, root := heading.Parent().(*ast.Document); !root || heading.Lines().Len() != 1 {
			return nil
		}
		for child := heading.FirstChild(); child != nil; child = child.NextSibling() {
			if _, plain := child.(*ast.Text); !plain {
				return nil
			}
		}
		raw := strings.TrimSpace(string(heading.Lines().Value(source)))
		rest, embedded := raw, false
		for {
			before, after, found := strings.Cut(rest, "![[")
			if !agreementEmbedHeadingWords(before) {
				return nil
			}
			if !found {
				break
			}
			inner, tail, closed := strings.Cut(after, "]]")
			if !closed || strings.ContainsFunc(inner, func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && r != ' ' && !strings.ContainsRune(".|#^/-", r)
			}) {
				return nil
			}
			if _, cites := graph.ParseWikilink(inner); !cites {
				return nil
			}
			embedded, rest = true, tail
		}
		base := graph.SectionID(raw)
		id := base
		for ordinal := 2; used[id]; ordinal++ {
			id = base + "-" + strconv.Itoa(ordinal)
		}
		used[id] = true
		if embedded && base != graph.SectionID(render.HeadingWords(raw)) {
			selected[id] = true
		}
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func agreementEmbedHeadingWords(words string) bool {
	return !strings.ContainsFunc(words, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && r != ' ' && r != '\t' && r != '-'
	})
}

func TestAgreementUnwrittenEmbedHeading(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]bool
	}{
		{name: "literal widget beside words", body: "![[A]]A\n---\n", want: map[string]bool{"a-a": true}},
		{name: "literal alias separator", body: "## A![[B|alias]]C\n", want: map[string]bool{"a-b-alias-c": true}},
		{name: "literal heading separator", body: "## ![[A#A]]B\n", want: map[string]bool{"a-a-b": true}},
		{name: "literal block separator", body: "## ![[A#^a]]B\n", want: map[string]bool{"a-a-b": true}},
		{name: "literal file name", body: "## ![[image.png]]A\n", want: map[string]bool{"image-png-a": true}},
		{name: "whole embedded word set", body: "## ![[A]]A![[B|alias]]C\n", want: map[string]bool{"a-a-b-alias-c": true}},
		{name: "ordinary heading before embed", body: "## A\n\n![[A]]A\n---\n", want: map[string]bool{"a-a": true}},
		{name: "ordinary heading after embed", body: "![[A]]A\n---\n\n## A\n", want: map[string]bool{"a-a": true}},
		{name: "ordinary name reserves id", body: "## A-A\n\n![[A]]A\n---\n", want: map[string]bool{"a-a-2": true}},
		{name: "authored suffix reserves id", body: "## A-A\n## A-A-2\n\n![[A]]A\n---\n", want: map[string]bool{"a-a-3": true}},
		{name: "whole embed namespace", body: "![[A]]A\n---\n\n![[A]]A\n---\n", want: map[string]bool{"a-a": true, "a-a-2": true}},
		{name: "independent plain opener", body: "> [!note] title\n\n![[A]]A\n---\n", want: map[string]bool{"a-a": true}},
		{name: "already matching words", body: "## ![[A]]\n"},
		{name: "ordinary heading", body: "## A\n"},
		{name: "paragraph is a different owner", body: "![[A]]A\n"},
		{name: "ordinary link owns its display", body: "## [[A]]A\n"},
		{name: "mixed widgets need separate ownership", body: "## ![[A]]A[[B|alias]]\n"},
		{name: "local address is separate", body: "## ![[#A]]B\n"},
		{name: "escaped widget is separate", body: "## \\![[A]]A\n"},
		{name: "code owns heading words", body: "## `![[A]]A`\n"},
		{name: "emphasis owns other words", body: "## *A*![[B|alias]]C\n"},
		{name: "quote owns heading", body: "> ## ![[A]]A\n"},
		{name: "list owns heading", body: "- ## ![[A]]A\n"},
		{name: "other heading owns code", body: "## `A`\n\n![[A]]A\n---\n"},
		{name: "native reference owns widget words", body: "## ![[A]]A\n\n[A]: /local\n"},
		{name: "other heading owns punctuation", body: "## a/b\n\n![[A]]A\n---\n"},
		{name: "other heading owns alias", body: "## [[B|alias]]\n\n![[A]]A\n---\n"},
		{name: "wrapped widget is separate", body: "![[A\nB]]A\n---\n"},
		{name: "comment context is separate", body: "%%\n![[A]]A\n---\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ids := agreementUnwrittenEmbedHeadingIDs(tc.body)
			if diff := cmp.Diff(tc.want, ids); diff != "" {
				t.Fatalf("caught: unwritten embed heading inventory (-want +got):\n%s", diff)
			}
			if tc.want == nil {
				return
			}
			c := agreementCase{Body: tc.body}
			_, observed := agreementIsolatedPage(t, c)
			failures := agreementFragmentFailures(t, []agreementCase{c}, []agreementHTML{observed})[0]
			if len(failures) != len(tc.want) {
				t.Fatalf("caught: embed heading public delta count=%d want=%d", len(failures), len(tc.want))
			}
			for _, failure := range failures {
				kind, authority, wrong := agreementWrappedHeadingDifference(c, &failure, ids)
				if kind != "debt" || authority != "#1011 stage 8" || wrong != "judge" {
					t.Fatalf("caught: embed heading public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
				for _, drift := range []agreementCase{
					{Body: tc.body, Title: "Reading"},
					{Body: tc.body, Companions: capturedBodies{"A.md": "body"}},
				} {
					if kind, _, _ := agreementWrappedHeadingDifference(drift, &failure, ids); kind != "" {
						t.Fatal("caught: embed heading borrowed a resolved or reserved name")
					}
				}
			}
		})
	}
}
