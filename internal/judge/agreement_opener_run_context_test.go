package judge_test

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func TestAgreementPlainOpenerRunContext(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{name: "whole plain run", body: "> [!note] one\n> [!note] two\n> [!note] three\n\n## A\n", want: true},
		{name: "mixed marker names", body: "> [!note] one\n> [!unknown] two\n\n## A\n", want: true},
		{name: "run beside root headings", body: "## A\n> [!note] one\n> [!note] two\n## A\n", want: true},
		{name: "one member is prose", body: "> [!note] one\n> another line\n\n## A\n"},
		{name: "one member has another bracket form", body: "> [!note] one\n> [note] two\n\n## A\n"},
		{name: "one member is lazy", body: "> [!note] one\n[!note] two\n\n## A\n"},
		{name: "one member owns a link", body: "> [!note] one\n> [!note] [[A]]\n\n## A\n"},
		{name: "one member owns code", body: "> [!note] one\n> [!note] `title`\n\n## A\n"},
		{name: "one member has unmatched code syntax", body: "> [!note] one\n> [!note] `open\n\n## A\n"},
		{name: "one member owns a reference", body: "> [!note] one\n> [!note] ref[^n]\n\n[^n]: text\n"},
		{name: "one member has nested depth", body: "> [!note] one\n> > [!note] two\n\n## A\n"},
		{name: "run inside a list", body: "- > [!note] one\n  > [!note] two\n\n## A\n"},
		{name: "run shares a quote with heading", body: "> [!note] one\n> [!note] two\n> ## A\n"},
		{name: "removed definition follows run", body: "> [!note] one\n> [!note] two\n\n> [^unused]: [[A]]\n"},
		{name: "removed definition precedes run", body: "> [^unused]: [[A]]\n\n> [!note] one\n> [!note] two\n"},
		{name: "live comment remains", body: "> [!note] one\n> [!note] two\n\n%%\n## A\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(tc.body)
			original := bytes.Clone(source)
			context := parser.NewContext()
			context.Set(agreementFootnoteTargetsKey, make(map[string]int))
			doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
			if got := agreementPlainOpenerDeclarationMarkers(source, doc); got != tc.want {
				t.Fatalf("caught: plain opener run ownership got=%t want=%t", got, tc.want)
			}
			if !bytes.Equal(original, source) {
				t.Fatal("caught: opener run observer changed source bytes")
			}
			if agreementIndependentDeclarationMarkers(source, doc) {
				t.Fatal("caught: single-opener scope admitted a run")
			}
		})
	}
}
