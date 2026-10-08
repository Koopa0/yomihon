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
	inFence := false
	var fenceByte byte
	var fenceLen int
	for i, line := range strings.Split(body.text, "\n") {
		at := offset
		offset += len(line) + 1
		if i > 0 {
			output.copied(body, at-1, at)
		}
		if state.closing == "-->" && at >= state.stop {
			state.closing = ""
		}
		if inFence {
			if FenceCloses(line, fenceByte, fenceLen) {
				inFence = false
			}
			output.copied(body, at, at+len(line))
			continue
		}
		if state.closing == "" {
			if marker, n, ok := FenceOpens(line); ok {
				inFence, fenceByte, fenceLen = true, marker, n
				output.copied(body, at, at+len(line))
				continue
			}
		}
		before := output.text.Len()
		opened := stripBodyCommentLine(body, at, line, &state, &output, &comments)
		switch {
		case opened && state.closing == "%%":
			unclosed = BodyComment{Line: i + 1, Marker: "%%"}
		case opened && state.closing == "-->" && state.unclosed:
			unclosed = BodyComment{Line: i + 1, Marker: "<!--"}
		case state.closing != "%%" && unclosed.Marker == "%%":
			unclosed = BodyComment{}
		}
		if state.closing == "" {
			visible := output.text.String()[before:]
			if marker, n, ok := FenceOpens(visible); ok {
				inFence, fenceByte, fenceLen = true, marker, n
			}
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
			if mark == html {
				span, closed := HTMLCommentSpan(body.text, open)
				if span.Zero() || In(state.code, open) {
					output.copied(body, open, open+4)
					line = line[mark+4:]
					continue
				}
				if stop, ok := state.limits[open]; ok && stop < span.Stop {
					span.Stop, closed = stop, false
				}
				if closed && span.Stop <= offset+originalLength {
					*comments = append(*comments, span)
					if bodyReadAloudMarker.MatchString(body.text[open:span.Stop]) {
						output.copied(body, open, span.Stop)
					}
					line = body.text[span.Stop : offset+originalLength]
					continue
				}
				*comments = append(*comments, span)
				state.closing, state.stop = "-->", span.Stop
				state.unclosed = !closed && span.Stop == len(body.text)
				start := strings.LastIndex(body.text[:open], "\n") + 1
				state.quoteDepth = strings.Count(bodyCommentQuotePrefix(body.text[start:open]), ">")
				return true
			}
			if In(state.extraCode, open) {
				output.copied(body, open, open+2)
				line = line[mark+2:]
				continue
			}
			stop := len(body.text)
			if end := strings.Index(body.text[open+2:], "%%"); end >= 0 {
				stop = open + 2 + end + 2
			}
			*comments = append(*comments, Span{Start: open, Stop: stop})
			state.closing, opened = "%%", true
			line = line[mark+2:]
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
