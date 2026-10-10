package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// A root quote containing one plain opener line owns only that declaration.
// Its delimiter cannot change source ownership in separate blocks.
func agreementIndependentDeclarationMarkers(source []byte, doc ast.Node) bool {
	return agreementPlainQuoteMarkers(source, doc, false)
}

func agreementPlainOpenerDeclarationMarkers(source []byte, doc ast.Node) bool {
	return agreementIndependentDeclarationMarkers(source, doc) || agreementPlainQuoteMarkers(source, doc, true)
}

func agreementPlainQuoteMarkers(source []byte, doc ast.Node, run bool) bool {
	if agreementDeclarationMarkers(source, doc) {
		return true
	}
	clean := bytes.Clone(source)
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		quote, quoted := node.(*ast.Blockquote)
		if !quoted || quote.FirstChild() != quote.LastChild() {
			continue
		}
		paragraph, prose := quote.FirstChild().(*ast.Paragraph)
		if !prose || !run && paragraph.Lines().Len() != 1 || run && paragraph.Lines().Len() < 2 {
			continue
		}
		plain := true
		for child := paragraph.FirstChild(); child != nil; child = child.NextSibling() {
			if _, textual := child.(*ast.Text); !textual {
				plain = false
			}
		}
		line := paragraph.Lines().At(0)
		physicalStart := bytes.LastIndexByte(source[:line.Start], '\n') + 1
		// An extension can remove a definition without removing its container.
		// Quoted source beside the paragraph prevents an isolated declaration set.
		before := strings.TrimSpace(string(source[:physicalStart]))
		if at := strings.LastIndexByte(before, '\n'); at >= 0 {
			before = strings.TrimSpace(before[at+1:])
		}
		nextStart := paragraph.Lines().At(paragraph.Lines().Len() - 1).Stop
		if nextStart > 0 && source[nextStart-1] != '\n' {
			if nextStart < len(source) && source[nextStart] == '\r' {
				nextStart++
			}
			if nextStart < len(source) && source[nextStart] == '\n' {
				nextStart++
			}
		}
		after := strings.TrimSpace(string(source[nextStart:]))
		if at := strings.IndexByte(after, '\n'); at >= 0 {
			after = strings.TrimSpace(after[:at])
		}
		if strings.HasPrefix(before, ">") || strings.HasPrefix(after, ">") {
			continue
		}
		if nextStart < len(source) && source[nextStart] != '\n' && source[nextStart] != '\r' {
			next, heading := node.NextSibling().(*ast.Heading)
			if !heading || next.Lines().Len() == 0 {
				continue
			}
			first := next.Lines().At(0)
			if bytes.LastIndexByte(source[:first.Start], '\n')+1 != nextStart {
				continue
			}
		}
		var starts []int
		for i := range paragraph.Lines().Len() {
			line := paragraph.Lines().At(i)
			physicalStart := bytes.LastIndexByte(source[:line.Start], '\n') + 1
			raw := string(line.Value(source))
			closeAt := strings.IndexByte(raw, ']')
			if strings.TrimSpace(string(source[physicalStart:line.Start])) != ">" || !plain || !strings.HasPrefix(raw, "[!") || closeAt <= 2 || strings.ContainsAny(raw[2:closeAt], " \t[") || strings.Contains(raw, "[[") || strings.Contains(raw, "[^") || run && strings.Contains(raw, "`") {
				break
			}
			rest := raw[closeAt+1:]
			if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\n") {
				break
			}
			starts = append(starts, line.Start)
		}
		if len(starts) != paragraph.Lines().Len() {
			continue
		}
		for _, start := range starts {
			clean[start], clean[start+1] = ' ', ' '
		}
	}
	return agreementDeclarationMarkers(clean, doc)
}

func TestAgreementIndependentDeclarationContext(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{name: "one plain opener", body: "> [!note] title\n\n## A\n", want: true},
		{name: "unknown quote marker", body: "> [!unknown] title\n\n## A\n", want: true},
		{name: "indented root quote", body: "## A\n  > [!note] title\n## A\n", want: true},
		{name: "empty title", body: "> [!note]\n\n## A\n", want: true},
		{name: "separate root quotes", body: "> [!note] one\n\n## A\n\n> [!note] two\n\n## A\n", want: true},
		{name: "plain root words", body: "show [!note] words\n\n## A\n", want: true},
		{name: "quote continuation needs layout", body: "> [!note] title\n> another line\n\n## A\n"},
		{name: "quote heading needs layout", body: "> [!note] title\n> ## A\n"},
		{name: "nested quote needs layout", body: "> > [!note] title\n\n## A\n"},
		{name: "list quote needs layout", body: "- > [!note] title\n\n## A\n"},
		{name: "code title needs ownership", body: "> [!note] `title`\n\n## A\n"},
		{name: "title link needs ownership", body: "> [!note] [[A]]\n\n## A\n"},
		{name: "title reference needs ownership", body: "> [!note] ref[^n]\n\n[^n]: title\n\n## A\n"},
		{name: "consecutive openers need layout", body: "> [!note] one\n> [!note] two\n\n## A\n"},
		{name: "live percent still needs ownership", body: "> [!note] title\n\n%%\n## A\n%%\n"},
		{name: "discarded quote definition follows opener", body: "> [!note] title\n> [^unused]: [[A]]\n"},
		{name: "discarded quote definition precedes opener", body: "> [^unused]: [[A]]\n> [!note] title\n"},
		{name: "discarded quote definition follows blank", body: "> [!note] title\n\n> [^unused]: [[A]]\n"},
		{name: "discarded quote definition precedes blank", body: "> [^unused]: [[A]]\n\n> [!note] title\n"},
		{name: "lazy prose line needs ownership", body: "> [!note] title\n[^unused]: [[A]]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(tc.body)
			original := bytes.Clone(source)
			context := parser.NewContext()
			context.Set(agreementFootnoteTargetsKey, make(map[string]int))
			doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
			if got := agreementIndependentDeclarationMarkers(source, doc); got != tc.want {
				t.Fatalf("caught: independent declaration context got=%t want=%t", got, tc.want)
			}
			if !bytes.Equal(original, source) {
				t.Fatal("caught: declaration context observer changed source bytes")
			}
		})
	}
}
