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

// RenderedBlockMarkerOpen belongs to markup the renderer plants after reading
// authored comments. A source copy is visible spelling with no marker authority.
const RenderedBlockMarkerOpen = "<!--yomihon-block:"

var bodyReadAloudMarker = regexp.MustCompile(`(?s)^<!--[ \t\r\n]*read-aloud:.*-->$`)

type bodyCommentState struct {
	closing    string
	stop       int
	unclosed   bool
	quoteDepth int
	code       []Span
	extraCode  []Span
	limits     map[int]int
	containers []BodyComment
	sourceLine int
	htmlLine   bool
}

// stripBodyProjection hides private comments exactly once. Both delimiters
// read canonical code ownership before treating authored bytes as private.
func stripBodyProjection(body bodyProjection, observation *bodyObservation, additional []Span) (stripped bodyProjection, comments []Span, unclosed BodyComment, containers []BodyComment) {
	state := bodyCommentState{limits: make(map[int]int), extraCode: additional}
	for _, code := range observation.codes {
		state.code = append(state.code, code.Span)
	}
	for _, limit := range observation.htmlLimits {
		state.limits[limit.Open] = limit.Stop
	}
	state.code = append(state.code, additional...)
	var output bodyProjectionWriter
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
		state.sourceLine = i + 1
		state.htmlLine = state.closing == "-->"
		opened := stripBodyCommentLine(body, at, line, &state, &output, &comments)
		unclosed = state.unclosedComment(unclosed, opened, i+1)
		visible := output.text.String()[before:]
		prefix := bodyCommentQuotePrefix(visible)
		authoredPrefix := bodyCommentQuotePrefix(line)
		if graphBlank := strings.TrimSpace(visible[len(prefix):]) == ""; graphBlank && !state.htmlLine && strings.TrimSpace(line[len(authoredPrefix):]) != "" {
			output.rolePadding("<u></u>")
		}
	}
	return output.roleProjection(), comments, unclosed, state.containers
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
		mark, html := strings.Index(line, "%%"), strings.Index(line, "<!--")
		if html >= 0 && (mark < 0 || html < mark) {
			mark = html
		}
		switch {
		case mark >= 0:
			output.copied(body, at, at+mark)
			open := at + mark
			var stopLine bool
			line, opened, stopLine = stripBodyCommentMark(body, open, offset+originalLength, mark == html, state, output, comments)
			if stopLine {
				return opened
			}
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
	if neutralizeBodyBlockMarker(body, open, lineStop, span, closed, output) {
		return body.text[span.Stop:lineStop], false
	}
	if stop, ok := state.limits[open]; ok && stop < span.Stop {
		span.Stop, closed = stop, false
	}
	if _, block := state.limits[open]; block && closed && span.Stop <= lineStop && !bodyReadAloudMarker.MatchString(body.text[open:span.Stop]) {
		output.rolePadding("<!---->")
	}
	*comments = append(*comments, span)
	if closed && span.Stop <= lineStop {
		copyBodyReadAloudMarker(body, span, output)
		return body.text[span.Stop:lineStop], false
	}
	state.closing, state.stop = "-->", span.Stop
	state.unclosed = !closed && span.Stop == len(body.text)
	if !closed && span.Stop < len(body.text) {
		state.containers = append(state.containers, BodyComment{Line: state.sourceLine, Marker: "<!--"})
	}
	start := strings.LastIndex(body.text[:open], "\n") + 1
	state.quoteDepth = strings.Count(bodyCommentQuotePrefix(body.text[start:open]), ">")
	return "", true
}

func copyBodyReadAloudMarker(body bodyProjection, span Span, output *bodyProjectionWriter) {
	if bodyReadAloudMarker.MatchString(body.text[span.Start:span.Stop]) {
		output.copied(body, span.Start, span.Stop)
	}
}

func stripBodyCommentMark(body bodyProjection, open, lineStop int, html bool, state *bodyCommentState, output *bodyProjectionWriter, comments *[]Span) (remaining string, opened, stopLine bool) {
	if html {
		state.htmlLine = true
		remaining, opened = stripBodyHTMLComment(body, open, lineStop, state, output, comments)
		return remaining, opened, opened
	}
	if In(state.code, open) || In(state.extraCode, open) {
		output.copied(body, open, open+2)
		return body.text[open+2 : lineStop], false, false
	}
	prefix := body.text[offsetLineStart(body.text, open):open]
	prefix = prefix[len(bodyCommentQuotePrefix(prefix)):]
	if strings.TrimSpace(prefix) == "" {
		output.rolePadding("<u></u>")
	}
	state.openPercent(body.text, open, comments)
	return body.text[open+2 : lineStop], true, false
}

func offsetLineStart(body string, offset int) int {
	return strings.LastIndexByte(body[:offset], '\n') + 1
}

func neutralizeBodyBlockMarker(body bodyProjection, open, lineStop int, span Span, closed bool, output *bodyProjectionWriter) bool {
	if strings.HasPrefix(body.text[open:], RenderedBlockMarkerOpen) && closed && span.Stop <= lineStop {
		output.synthetic("&lt;")
		output.copied(body, open+1, span.Stop)
		return true
	}
	return false
}
