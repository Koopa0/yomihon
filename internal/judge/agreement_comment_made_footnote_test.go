package judge_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
)

// Removing a comment prefix must not turn its literal tail into a definition.
// The original HTML declaration, rather than a discarded footnote, owns it.
func agreementCommentMadeFootnoteTargets(body string) map[string]int {
	source := []byte(body)
	context := parser.NewContext()
	context.Set(agreementFootnoteTargetsKey, make(map[string]int))
	doc := agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	if !agreementPlainOpenerDeclarationMarkers(source, doc) {
		return nil
	}
	targets := make(map[string]int)
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		block, ok := node.(*ast.HTMLBlock)
		if !ok || block.HTMLBlockType != ast.HTMLBlockType2 || block.Lines().Len() != 1 {
			continue
		}
		line := block.Lines().At(0)
		raw := string(line.Value(source))
		if !strings.HasPrefix(raw, "<!--") {
			continue
		}
		comment, closed := graph.HTMLCommentSpan(body, line.Start)
		if !closed || comment.Stop > line.Stop {
			continue
		}
		tail := strings.TrimRight(body[comment.Stop:line.Stop], "\r\n")
		if !strings.HasPrefix(tail, "[^") {
			continue
		}
		label, field, defined := strings.Cut(tail, "]:")
		label = strings.TrimPrefix(label, "[^")
		if !defined || label == "" || strings.ContainsFunc(label, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && r != '-'
		}) || strings.Count(body, "[^"+label+"]") != 1 {
			return nil
		}
		fields := strings.Fields(field)
		if len(fields) == 0 {
			return nil
		}
		for _, widget := range fields {
			inner, opened := strings.CutPrefix(widget, "[[")
			inner, ended := strings.CutSuffix(inner, "]]")
			if !opened || !ended || inner == "" || strings.ContainsFunc(inner, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) }) {
				return nil
			}
			found := judge.LinkTargets(widget)
			if len(found) != 1 {
				return nil
			}
			targets[found[0]]++
		}
	}
	if len(targets) == 0 {
		return nil
	}
	return targets
}

func agreementCommentMadeFootnoteDifference(c agreementCase, f *agreementFailure, targets map[string]int) (kind, authority, wrong string) {
	kind, authority, wrong = agreementUnusedFootnoteDifference(c, f, targets)
	if kind != "" && f.Property == "P1" {
		wrong = "page"
	}
	return kind, authority, wrong
}

func TestAgreementCommentMadeFootnotes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
	}{
		{name: "closed prefix", body: "<!--hidden-->[^unused]: [[A]]\n", want: map[string]int{"A": 1}},
		{name: "preceding prose", body: "A\n<!-- [[A]] -->[^unused]: [[A]]\n", want: map[string]int{"A": 1}},
		{name: "whole target set", body: "<!--hidden-->[^unused]: [[A]] [[B]] [[A]]\n", want: map[string]int{"A": 2, "B": 1}},
		{name: "every declaration", body: "<!--hidden-->[^one]: [[A]]\n<!--hidden-->[^two]: [[B]]\n", want: map[string]int{"A": 1, "B": 1}},
		{name: "real definition stays separate", body: "[^unused]: [[A]]\n"},
		{name: "used manufactured definition", body: "<!--hidden-->[^n]: [[A]]\nref[^n]\n"},
		{name: "duplicated label needs assignment", body: "<!--hidden-->[^n]: [[A]]\n<!--hidden-->[^n]: [[B]]\n"},
		{name: "unclosed comment", body: "<!--hidden[^unused]: [[A]]\n"},
		{name: "code owner", body: "```\n<!--hidden-->[^unused]: [[A]]\n```\n"},
		{name: "quote owner", body: "> <!--hidden-->[^unused]: [[A]]\n"},
		{name: "ordinary tail", body: "<!--hidden-->words [[A]]\n"},
		{name: "space retains another role", body: "<!--hidden--> [^unused]: [[A]]\n"},
		{name: "raw HTML container", body: "<div>\n<!--hidden-->[^unused]: [[A]]\n</div>\n"},
		{name: "invalid label syntax", body: "<!--hidden-->[^two words]: [[A]]\n"},
		{name: "mixed field needs ownership", body: "<!--hidden-->[^unused]: words [[A]]\n"},
		{name: "alias needs target ownership", body: "<!--hidden-->[^unused]: [[B|alias]]\n"},
		{name: "comment owns target", body: "<!--[[A]]-->[^unused]: words\n"},
		{name: "percent region", body: "%%\n<!--hidden-->[^unused]: [[A]]\n%%\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			targets := agreementCommentMadeFootnoteTargets(tc.body)
			if diff := cmp.Diff(tc.want, targets); diff != "" {
				t.Fatalf("caught: comment-made definition inventory (-want +got):\n%s", diff)
			}
			if tc.want == nil {
				return
			}
			c := agreementCase{Body: tc.body}
			result, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &result, &actual)
			if len(failures) != 2*len(tc.want) {
				t.Fatalf("caught: comment-made public delta count=%d want=%d", len(failures), 2*len(tc.want))
			}
			for i := range failures {
				f := &failures[i]
				kind, authority, wrong := agreementCommentMadeFootnoteDifference(c, f, targets)
				expectedWrong := "page"
				if f.Property == "P0" {
					expectedWrong = "page-diagnostic"
				}
				if kind != "debt" || authority != "#1011 stage 5" || wrong != expectedWrong {
					t.Fatalf("caught: comment-made public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "P2" },
					func(f *agreementFailure) { f.Identity = "unowned" },
					func(f *agreementFailure) { f.Direction = "page-only" },
					func(f *agreementFailure) { f.Tuple.State = "unowned" },
					func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					drift := *f
					change(&drift)
					if kind, _, _ := agreementCommentMadeFootnoteDifference(c, &drift, targets); kind != "" {
						t.Fatalf("caught: unrelated comment-made signature accepted: %+v", drift)
					}
				}
				for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
					if kind, _, _ := agreementCommentMadeFootnoteDifference(other, f, targets); kind != "" {
						t.Fatal("caught: comment-made definition borrowed another context")
					}
				}
			}
			source := []byte(c.Body)
			ctx := parser.NewContext()
			ctx.Set(agreementFootnoteTargetsKey, make(map[string]int))
			agreementFootnoteGrammar.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
			if original, ok := ctx.Get(agreementFootnoteTargetsKey).(map[string]int); !ok || len(original) != 0 {
				t.Fatalf("caught: original source declared discarded targets: %v", original)
			}
		})
	}
}
