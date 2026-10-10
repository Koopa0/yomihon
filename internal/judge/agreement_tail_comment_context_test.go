package judge_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// A terminal root HTML comment owns its entire unfinished payload. Definitions
// before its source line remain separate; masking never joins their labels.
func agreementTailHTMLCommentMarkers(source []byte, doc ast.Node) bool {
	block, comment := doc.LastChild().(*ast.HTMLBlock)
	if !comment || block.HTMLBlockType != ast.HTMLBlockType2 || block.HasClosure() || block.Lines().Len() == 0 {
		return false
	}
	first := block.Lines().At(0)
	last := block.Lines().At(block.Lines().Len() - 1)
	if last.Stop != len(source) {
		return false
	}
	body := string(source)
	raw := string(first.Value(source))
	start := first.Start + len(raw) - len(strings.TrimLeft(raw, " \t"))
	span, closed := graph.HTMLCommentSpan(body, start)
	if closed || span.Start != start || span.Stop != last.Stop {
		return false
	}
	clean := bytes.Clone(source)
	for i := span.Start; i < span.Stop; i++ {
		if clean[i] != '\n' && clean[i] != '\r' {
			clean[i] = ' '
		}
	}
	return agreementPlainOpenerDeclarationMarkers(clean, doc)
}

func TestAgreementUnusedFootnoteTailComment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		context    bool
		want       map[string]int
		failures   int
	}{
		{name: "terminal comment", body: "[^n]: [[A]]\n\n    [[B]]\n<!--", context: true, want: map[string]int{"A": 1, "B": 1}, failures: 3},
		{name: "payload owns reference and markers", body: "[^n]: [[A]]\n\n<!-- %% [!note] ref[^n] [[C]]\n", context: true, want: map[string]int{"A": 1}, failures: 2},
		{name: "whole unused inventory", body: "[^n]: [[A]] [[A]] [[B]]\n\n<!--\n", context: true, want: map[string]int{"A": 2, "B": 1}, failures: 4},
		{name: "crlf preserves boundaries", body: "[^n]: [[A]]\r\n\r\n  <!--\r\nhidden\r\n", context: true, want: map[string]int{"A": 1}, failures: 2},
		{name: "independent live occurrence", body: "[[A]]\n\n[^n]: [[A]]\n\n<!--\n", context: true, want: map[string]int{"A": 1}, failures: 2},
		{name: "comment owns apparent definition", body: "<!--\n[^n]: [[A]]\n", context: true},
		{name: "closed scope stays separate", body: "[^n]: [[A]]\n\n<!--hidden-->\n"},
		{name: "used scope stays separate", body: "ref[^n]\n\n[^n]: [[A]]\n\n<!--\n"},
		{name: "inline comment can join reference", body: "ref[^<!-- hidden -->n]\n\n[^n]: [[A]]\n\n<!--\n"},
		{name: "quote limits comment", body: "[^n]: [[A]]\n\n> <!--\n"},
		{name: "list limits comment", body: "[^n]: [[A]]\n\n- <!--\n"},
		{name: "raw container owns comment", body: "[^n]: [[A]]\n\n<div>\n<!--\n"},
		{name: "live percent comment remains", body: "%%\n[^n]: [[A]]\n\n<!--\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(tc.body)
			original := bytes.Clone(source)
			context := parser.NewContext()
			context.Set(agreementFootnoteTargetsKey, make(map[string]int))
			doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
			if got := agreementTailHTMLCommentMarkers(source, doc); got != tc.context {
				t.Fatalf("caught: terminal HTML comment ownership got=%t want=%t", got, tc.context)
			}
			if !bytes.Equal(source, original) {
				t.Fatal("caught: terminal comment observer changed source bytes")
			}
			targets := agreementUnusedFootnoteTailTargets(tc.body)
			if diff := cmp.Diff(tc.want, targets); diff != "" {
				t.Fatalf("caught: terminal comment unused inventory (-want +got):\n%s", diff)
			}
			if tc.want == nil {
				return
			}
			if agreementUnusedFootnoteTargets(tc.body) != nil {
				t.Fatal("caught: ordinary comment scope admitted a terminal comment")
			}
			page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, nil)
			failures := agreementPageFailures(tc.body, &result, &observed)
			if len(failures) != tc.failures {
				t.Fatalf("caught: terminal comment public delta count=%d want=%d", len(failures), tc.failures)
			}
			for _, failure := range failures {
				kind, authority, wrong := agreementUnusedFootnoteDifference(agreementCase{Body: tc.body}, &failure, targets)
				wantWrong := "judge"
				if failure.Property == "P0" {
					wantWrong = "page-diagnostic"
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != wantWrong {
					t.Fatalf("caught: terminal comment public ownership kind=%q wrong=%q signature=%s", kind, wrong, agreementSignature(&failure))
				}
			}
		})
	}
}
