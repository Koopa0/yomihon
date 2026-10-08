package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// A closed HTML comment or a complete one-line root comment owns its payload.
// Ignoring its delimiters preserves the other declarations and coordinates.
func agreementDeclarationMarkers(source []byte, doc ast.Node) bool {
	if agreementLiteralMarkers(source, doc) {
		return true
	}
	clean := bytes.Clone(source)
	body := string(source)
	if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		start, limit := -1, -1
		switch node := node.(type) {
		case *ast.Paragraph:
			if _, root := node.Parent().(*ast.Document); !root || node.Lines().Len() != 1 {
				return ast.WalkContinue, nil
			}
			line := node.Lines().At(0)
			raw := strings.TrimSpace(string(line.Value(source)))
			if !strings.HasPrefix(raw, "%%") || !strings.HasSuffix(raw, "%%") || strings.Count(raw, "%%") != 2 || strings.Contains(raw, "[^") {
				return ast.WalkContinue, nil
			}
			for i := line.Start; i < line.Stop; i++ {
				if clean[i] != '\n' && clean[i] != '\r' {
					clean[i] = ' '
				}
			}
			return ast.WalkContinue, nil
		case *ast.RawHTML:
			// Removing an inline comment can join a footnote reference label.
			// Its reference assignment needs separate ownership.
			if strings.Contains(body, "[^") {
				return ast.WalkContinue, nil
			}
			if node.Segments.Len() > 0 {
				start = node.Segments.At(0).Start
				limit = node.Segments.At(node.Segments.Len() - 1).Stop
			}
		case *ast.HTMLBlock:
			if node.HTMLBlockType != ast.HTMLBlockType2 {
				return ast.WalkContinue, nil
			}
			if node.Lines().Len() > 0 {
				start = node.Lines().At(0).Start
				limit = node.Lines().At(node.Lines().Len() - 1).Stop
			}
			if node.HasClosure() {
				if start < 0 {
					start = node.ClosureLine.Start
				}
				limit = max(limit, node.ClosureLine.Stop)
			}
		}
		if start < 0 || limit < start || limit > len(source) {
			return ast.WalkContinue, nil
		}
		start += len(body[start:limit]) - len(strings.TrimLeft(body[start:limit], " \t"))
		span, closed := graph.HTMLCommentSpan(body, start)
		if !closed || span.Stop > limit {
			return ast.WalkContinue, nil
		}
		for i := span.Start; i < span.Stop; i++ {
			if clean[i] != '\n' && clean[i] != '\r' {
				clean[i] = ' '
			}
		}
		return ast.WalkContinue, nil
	}); err != nil {
		panic(err)
	}
	return agreementLiteralMarkers(clean, doc)
}

func TestAgreementDeclarationMarkers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{name: "closed block comment", body: "<!--hidden-->\n", want: true},
		{name: "complete root percent comment", body: "%%[[A]]%%\n", want: true},
		{name: "percent payload owns other markers", body: "%%<!--[!note]%%\n", want: true},
		{name: "percent payload can assign reference", body: "%%ref[^n]%%\n\n[^n]: [[A]]\n"},
		{name: "percent comment shares prose", body: "before %%[[A]]%% after\n"},
		{name: "multiple percent regions", body: "%%a%% b %%c%%\n"},
		{name: "percent comment has container", body: "> %%[[A]]%%\n"},
		{name: "closed inline comment", body: "text <!--hidden--> more\n", want: true},
		{name: "short empty comment", body: "<!-->\n", want: true},
		{name: "quoted comment", body: "> <!--hidden-->\n", want: true},
		{name: "payload owns all markers", body: "<!--%%[!note]-->\n", want: true},
		{name: "literal and comment", body: "`%%` <!--[!note]-->\n", want: true},
		{name: "unclosed comment", body: "<!--hidden\n"},
		{name: "closer outside declared container", body: "> <!--hidden\n\noutside -->\n"},
		{name: "live percent beside comment", body: "<!--hidden--> %%live%%"},
		{name: "live callout beside comment", body: "<!--hidden-->\n\n> [!note] text\n"},
		{name: "escaped comment opener", body: "\\<!--hidden-->"},
		{name: "inline comment can join reference label", body: "ref[^n<!--hidden-->]\n\n[^n]: [[A]]\n"},
		{name: "raw html container needs ownership", body: "<div>\n<!--hidden-->\n</div>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(tc.body)
			original := bytes.Clone(source)
			context := parser.NewContext()
			context.Set(agreementFootnoteTargetsKey, make(map[string]int))
			doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
			if got := agreementDeclarationMarkers(source, doc); got != tc.want {
				t.Fatalf("caught: declared delimiter ownership got=%t want=%t", got, tc.want)
			}
			if !bytes.Equal(original, source) {
				t.Fatal("caught: delimiter observer changed source bytes")
			}
		})
	}
}
