package judge_test

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// Delimiters inside a declared literal cannot change the surrounding prose's
// roles. Every occurrence must be owned; one live delimiter keeps the refusal.
func agreementLiteralMarkers(source []byte, doc ast.Node) bool {
	var zones []graph.Span
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *ast.CodeSpan:
			first, firstOK := node.FirstChild().(*ast.Text)
			last, lastOK := node.LastChild().(*ast.Text)
			if firstOK && lastOK {
				zones = append(zones, graph.Span{Start: first.Segment.Start, Stop: last.Segment.Stop})
			}
		case *ast.FencedCodeBlock:
			if node.Info != nil {
				zones = append(zones, graph.Span{Start: node.Info.Segment.Start, Stop: node.Info.Segment.Stop})
			}
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				zones = append(zones, graph.Span{Start: line.Start, Stop: line.Stop})
			}
		case *ast.CodeBlock:
			for i := range node.Lines().Len() {
				line := node.Lines().At(i)
				zones = append(zones, graph.Span{Start: line.Start, Stop: line.Stop})
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	body := string(source)
	for _, marker := range []string{"%%", "<!--", "[!"} {
		for off := 0; off < len(body); {
			rel := strings.Index(body[off:], marker)
			if rel < 0 {
				break
			}
			at := off + rel
			if !graph.In(zones, at) {
				return false
			}
			off = at + len(marker)
		}
	}
	return true
}

func TestAgreementLiteralMarkers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{name: "ordinary prose", body: "plain", want: true},
		{name: "inline literal", body: "`%%<!--[!note]`", want: true},
		{name: "wrapped literal", body: "`open\n%%<!--[!note]\nclose`", want: true},
		{name: "fence content", body: "```\n%%<!--[!note]\n```\n", want: true},
		{name: "fence declaration", body: "``` %%<!--[!note]\n", want: true},
		{name: "indented literal", body: "    %%<!--[!note]\n", want: true},
		{name: "quoted literal", body: "> ```\n> %%<!--[!note]\n", want: true},
		{name: "live percent comment", body: "%%text%%"},
		{name: "live html comment", body: "<!--text-->"},
		{name: "callout opener", body: "> [!note] text\n"},
		{name: "mixed percent owners", body: "`%%` %%text%%"},
		{name: "mixed html owners", body: "`<!--` <!--text-->"},
		{name: "mixed callout owners", body: "`[!note]`\n\n> [!note] text\n"},
		{name: "escaped delimiter needs ownership", body: "\\<!--text-->"},
		{name: "escaped code opener", body: "\\`%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(tc.body)
			context := parser.NewContext()
			context.Set(agreementFootnoteTargetsKey, make(map[string]int))
			doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
			if got := agreementLiteralMarkers(source, doc); got != tc.want {
				t.Fatalf("caught: literal delimiter ownership got=%t want=%t", got, tc.want)
			}
		})
	}
}
