package graph

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var commentParser = goldmark.New().Parser()

// Span is a half-open byte range [Start, Stop) into a body. Comment pairing,
// the course's skip zones, and a row's source identity all use this one type
// so a parked mark cannot be a comment on one face and prose on another.
type Span struct{ Start, Stop int }

// Contains reports whether off falls in the range.
func (s Span) Contains(off int) bool {
	return off >= s.Start && off < s.Stop
}

// Zero reports whether this span identifies nothing — a heading group's
// anchor, or an orphan's.
func (s Span) Zero() bool { return s == Span{} }

// In reports whether off falls in any of the ranges.
func In(zones []Span, off int) bool {
	for _, z := range zones {
		if z.Contains(off) {
			return true
		}
	}
	return false
}

// HTMLCommentSpan locates an unescaped HTML comment beginning at open. A
// comment closes at its first -->, including CommonMark's short empty forms.
// An unclosed one owns the rest of the body; closed distinguishes that case.
func HTMLCommentSpan(body string, open int) (span Span, closed bool) {
	if open < 0 || open > len(body) || !strings.HasPrefix(body[open:], "<!--") {
		return Span{}, false
	}
	escapes := 0
	for i := open - 1; i >= 0 && body[i] == '\\'; i-- {
		escapes++
	}
	if escapes%2 == 1 {
		return Span{}, false
	}
	rest := body[open+4:]
	switch {
	case strings.HasPrefix(rest, ">"):
		return Span{Start: open, Stop: open + 5}, true
	case strings.HasPrefix(rest, "->"):
		return Span{Start: open, Stop: open + 6}, true
	}
	if end := strings.Index(rest, "-->"); end >= 0 {
		return Span{Start: open, Stop: open + 4 + end + 3}, true
	}
	return Span{Start: open, Stop: len(body)}, false
}

// HTMLCommentLimits reports where a comment's Markdown container ends.
// An unclosed HTML block inside a list item or quote cannot consume prose
// outside that container, even when a later line carries a closing marker.
func HTMLCommentLimits(body string) map[int]int {
	if !strings.Contains(body, "<!--") {
		return nil
	}
	src := []byte(body)
	doc := commentParser.Parse(text.NewReader(src))
	limits := make(map[int]int)
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		block, ok := n.(*ast.HTMLBlock)
		if !entering || !ok || block.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}
		first := block.Lines().At(0)
		raw := body[first.Start:first.Stop]
		trimmed := strings.TrimLeft(raw, " \t")
		if !strings.HasPrefix(trimmed, "<!--") {
			return ast.WalkContinue, nil
		}
		open := first.Start + len(raw) - len(trimmed)
		stop := block.Lines().At(block.Lines().Len() - 1).Stop
		if block.HasClosure() {
			stop = block.ClosureLine.Stop
		}
		limits[open] = stop
		return ast.WalkContinue, nil
	})
	return limits
}

// CommentZones are the %% and HTML comment spans in body, in source order.
// An opener in code is ignored. Each comment consumes its own closer, so a
// different delimiter inside it cannot change pairing. An unclosed comment
// owns the rest of its Markdown container, or the body for a %% comment.
func CommentZones(body string, code []Span) []Span {
	var zones []Span
	limits := HTMLCommentLimits(body)
	for off := 0; off < len(body); {
		percent, html := strings.Index(body[off:], "%%"), strings.Index(body[off:], "<!--")
		start, closer, size := percent, "%%", 2
		if html >= 0 && (percent < 0 || html < percent) {
			start, closer, size = html, "-->", 4
		}
		if start < 0 {
			break
		}
		at := off + start
		off = at + size
		if In(code, at) {
			continue
		}
		if closer == "-->" {
			span, _ := HTMLCommentSpan(body, at)
			if stop, ok := limits[at]; ok {
				span.Stop = min(span.Stop, stop)
			}
			if span.Zero() {
				continue
			}
			zones = append(zones, span)
			off = span.Stop
			continue
		}
		stop := len(body)
		if end := strings.Index(body[off:], closer); end >= 0 {
			stop = off + end + len(closer)
		}
		zones = append(zones, Span{Start: at, Stop: stop})
		off = stop
	}
	return zones
}
