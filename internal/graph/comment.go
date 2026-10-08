package graph

import "strings"

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
	observation := observeBody(body, bodyAuthoredMarkdown.Parser())
	limits := make(map[int]int, len(observation.htmlLimits))
	for _, limit := range observation.htmlLimits {
		limits[limit.Open] = limit.Stop
	}
	return limits
}

// CommentZones are the %% and HTML comment spans in body, in source order.
// An opener in code is ignored. Each comment consumes its own closer, so a
// different delimiter inside it cannot change pairing. An unclosed comment
// owns the rest of its Markdown container, or the body for a %% comment.
func CommentZones(body string, code []Span) []Span {
	original := originalBody(body)
	observation := observeBody(body, bodyAuthoredMarkdown.Parser())
	protectBodyInline(original, observation)
	_, comments, _, _ := stripBodyProjection(original, observation, code)
	return comments
}
