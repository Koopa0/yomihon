package graph

import (
	"regexp"
	"strings"
)

// BodyComment is the authored line and delimiter of an unclosed private comment.
type BodyComment struct {
	Line   int
	Marker string
}

var bodyReadAloudMarker = regexp.MustCompile(`(?s)^<!--[ \t\r\n]*read-aloud:.*-->$`)

type bodyCommentState struct {
	closing    string
	stop       int
	unclosed   bool
	quoteDepth int
	code       []Span
	extraCode  []Span
	limits     map[int]int
}

// stripBodyProjection applies the existing line-role strip exactly once.
// Original code facts protect HTML comments. Percent quotation retains its
// line-local presentation policy, independently of canonical multiline facts.
func stripBodyProjection(body bodyProjection, observation *bodyObservation, additional []Span) (bodyProjection, []Span, BodyComment) {
	state := bodyCommentState{limits: make(map[int]int), extraCode: additional}
	for _, code := range observation.codes {
		state.code = append(state.code, code.Span)
	}
	for _, limit := range observation.htmlLimits {
		state.limits[limit.Open] = limit.Stop
	}
	state.code = append(state.code, additional...)
	var output bodyProjectionWriter
	var comments []Span
	var unclosed BodyComment
	offset := 0
	fence := bodyCommentFence{}
	for i, line := range strings.Split(body.text, "\n") {
		at := offset
		offset += len(line) + 1
		if i > 0 {
			output.copied(body, at-1, at)
		}
		if state.closing == "-->" && at >= state.stop {
			state.closing = ""
		}
		if fence.protects(line, state.closing) {
			output.copied(body, at, at+len(line))
			continue
		}
		before := output.text.Len()
		opened := stripBodyCommentLine(body, at, line, &state, &output, &comments)
		unclosed = state.unclosedComment(unclosed, opened, i+1)
		if state.closing == "" {
			fence.opens(output.text.String()[before:])
		}
	}
	return output.projection(), comments, unclosed
}

func bodyCommentQuotePrefix(line string) string {
	size := 0
	for {
		prefix := QuotePrefix.FindString(line[size:])
		if prefix == "" {
			return line[:size]
		}
		size += len(prefix)
	}
}

func stripBodyCommentLine(body bodyProjection, offset int, line string, state *bodyCommentState, output *bodyProjectionWriter, comments *[]Span) bool {
	opened := false
	originalLength := len(line)
	if state.closing == "-->" && state.quoteDepth > 0 {
		prefix := bodyCommentQuotePrefix(line)
		output.copied(body, offset, offset+len(prefix))
		line = line[len(prefix):]
	}
	for line != "" {
		at := offset + originalLength - len(line)
		if state.closing != "" {
			index := strings.Index(line, state.closing)
			if index < 0 {
				return opened
			}
			end := index + len(state.closing)
			state.closing, opened = "", false
			line = line[end:]
			continue
		}
		tick := strings.IndexByte(line, '`')
		mark, html := strings.Index(line, "%%"), strings.Index(line, "<!--")
		if html >= 0 && (mark < 0 || html < mark) {
			mark = html
		}
		switch {
		case mark >= 0 && (tick < 0 || mark < tick):
			output.copied(body, at, at+mark)
			open := at + mark
			var stopLine bool
			line, opened, stopLine = stripBodyCommentMark(body, open, offset+originalLength, mark == html, state, output, comments)
			if stopLine {
				return opened
			}
		case tick >= 0:
			end, _ := CodeSpanAt(line, tick)
			output.copied(body, at, at+end)
			line = line[end:]
		default:
			output.copied(body, at, at+len(line))
			line = ""
		}
	}
	return opened
}

// bodyCommentFence preserves the strip's line-local fence policy.
type bodyCommentFence struct {
	active bool
	marker byte
	width  int
}

func (f *bodyCommentFence) protects(line, closing string) bool {
	if f.active {
		if FenceCloses(line, f.marker, f.width) {
			f.active = false
		}
		return true
	}
	if closing != "" {
		return false
	}
	return f.opens(line)
}

func (f *bodyCommentFence) opens(line string) bool {
	marker, width, ok := FenceOpens(line)
	if ok {
		f.active, f.marker, f.width = true, marker, width
	}
	return ok
}

func (s *bodyCommentState) unclosedComment(previous BodyComment, opened bool, line int) BodyComment {
	switch {
	case opened && s.closing == "%%":
		return BodyComment{Line: line, Marker: "%%"}
	case opened && s.closing == "-->" && s.unclosed:
		return BodyComment{Line: line, Marker: "<!--"}
	case s.closing != "%%" && previous.Marker == "%%":
		return BodyComment{}
	default:
		return previous
	}
}

func (s *bodyCommentState) openPercent(body string, open int, comments *[]Span) {
	stop := len(body)
	if end := strings.Index(body[open+2:], "%%"); end >= 0 {
		stop = open + 2 + end + 2
	}
	*comments = append(*comments, Span{Start: open, Stop: stop})
	s.closing = "%%"
}

// stripBodyHTMLComment returns the remaining physical line, or records a
// multiline opener and stops this line. Protected and read-aloud bytes retain
// their original copied-piece attribution.
func stripBodyHTMLComment(body bodyProjection, open, lineStop int, state *bodyCommentState, output *bodyProjectionWriter, comments *[]Span) (string, bool) {
	span, closed := HTMLCommentSpan(body.text, open)
	if span.Zero() || In(state.code, open) {
		output.copied(body, open, open+4)
		return body.text[open+4 : lineStop], false
	}
	if stop, ok := state.limits[open]; ok && stop < span.Stop {
		span.Stop, closed = stop, false
	}
	*comments = append(*comments, span)
	if closed && span.Stop <= lineStop {
		if bodyReadAloudMarker.MatchString(body.text[open:span.Stop]) {
			output.copied(body, open, span.Stop)
		}
		return body.text[span.Stop:lineStop], false
	}
	state.closing, state.stop = "-->", span.Stop
	state.unclosed = !closed && span.Stop == len(body.text)
	start := strings.LastIndex(body.text[:open], "\n") + 1
	state.quoteDepth = strings.Count(bodyCommentQuotePrefix(body.text[start:open]), ">")
	return "", true
}

func stripBodyCommentMark(body bodyProjection, open, lineStop int, html bool, state *bodyCommentState, output *bodyProjectionWriter, comments *[]Span) (remaining string, opened, stopLine bool) {
	if html {
		remaining, opened = stripBodyHTMLComment(body, open, lineStop, state, output, comments)
		return remaining, opened, opened
	}
	if In(state.extraCode, open) {
		output.copied(body, open, open+2)
		return body.text[open+2:lineStop], false, false
	}
	state.openPercent(body.text, open, comments)
	return body.text[open+2:lineStop], true, false
}
