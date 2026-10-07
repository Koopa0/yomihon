package render

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	textm "github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// strippedBody is one body with its Obsidian comments off: the text every cut,
// parse and scan downstream reads, beside the line geometry its author wrote.
// The two travel together because the strip empties a line whose every visible
// byte was hidden, and the block-address pass has to widen its run over the
// blank lines the author typed rather than over the ones a strip made.
type strippedBody struct {
	text    string
	address []string
}

// stripBody hides body's comments and records what that cost the line geometry.
// report retains the original comment coordinates beside that text.
func stripBody(body string) (stripped strippedBody, report commentReport) {
	text, report := stripObsidianComments(body)
	return strippedBody{
		text:    text,
		address: BlockAddressLines(strings.Split(body, "\n"), text),
	}, report
}

// commentReport distinguishes container silence from a marker that hides every
// remaining word of the body. Containers retain their source order.
type commentReport struct {
	bodywide   unclosedComment
	containers []unclosedComment
}

// unclosedComment is a comment that never met its closer and so hides the rest
// of its container or body: the original 1-based body line and marker.
// A zero line means no such comment.
type unclosedComment struct {
	line   int
	marker string
}

// stripObsidianComments removes %% and HTML comment regions while preserving the
// delimiters and contents of fenced code blocks. An unclosed comment runs to the
// end of its Markdown container or the body. The report retains each container
// opener and at most one body-wide opener, which swallows any later opener.
func stripObsidianComments(body string) (stripped string, report commentReport) {
	lines := strings.Split(body, "\n")
	state := commentState{htmlCode: htmlCommentCode(body), limits: graph.HTMLCommentLimits(body)}
	offset := 0
	inFence := false
	var fenceByte byte
	var fenceLen int

	for i, line := range lines {
		at := offset
		offset += len(line) + 1
		if state.closing == "-->" && at >= state.stop {
			state.closing = ""
		}
		if inFence {
			if fenceCloses(line, fenceByte, fenceLen) {
				inFence = false
			}
			continue
		}
		// An open comment swallows fence markers the way it swallows
		// everything else. Consulting the fence first would turn tracking
		// on, skip the strip, and leak the fenced block onto the page
		// and into the search corpus.
		if state.closing == "" {
			if marker, n, _, ok := fenceOpen(line); ok {
				inFence = true
				fenceByte = marker
				fenceLen = n
				continue
			}
		}

		var openedHere bool
		state.sourceLine = i + 1
		lines[i], openedHere = stripObsidianCommentLine(line, body, at, &state)
		report.bodywide.follow(&state, i+1, openedHere)
		if state.closing != "" {
			continue
		}
		if marker, n, _, ok := fenceOpen(lines[i]); ok {
			inFence = true
			fenceByte = marker
			fenceLen = n
		}
	}
	report.containers = state.containers
	return strings.Join(lines, "\n"), report
}

// follow updates the record after one line is stripped: a comment opened on it
// that the rest of the body cannot close becomes the record, and a %% that met
// its partner leaves none. A line still inside the recorded comment changes
// nothing.
func (u *unclosedComment) follow(state *commentState, line int, openedHere bool) {
	switch {
	case openedHere && state.closing == "%%":
		*u = unclosedComment{line: line, marker: "%%"}
	case openedHere && state.closing == "-->" && state.unclosed:
		*u = unclosedComment{line: line, marker: "<!--"}
	case state.closing != "%%" && u.marker == "%%":
		*u = unclosedComment{}
	}
}

type commentState struct {
	closing    string
	stop       int
	unclosed   bool
	quoteDepth int
	htmlCode   []graph.Span
	limits     map[int]int
	containers []unclosedComment
	sourceLine int
}

func htmlCommentQuotePrefix(line string) string {
	size := 0
	for {
		prefix := graph.QuotePrefix.FindString(line[size:])
		if prefix == "" {
			return line[:size]
		}
		size += len(prefix)
	}
}

// htmlCommentCode protects comments authored as Markdown code, including
// indented blocks and code spans that cross a line boundary.
func htmlCommentCode(body string) []graph.Span {
	if !strings.Contains(body, "<!--") {
		return nil
	}
	src := []byte(body)
	doc := plainParser.Parse(textm.NewReader(src))
	var zones []graph.Span
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		if !entering {
			return ast.WalkContinue, nil
		}
		if span, ok := htmlCodeSpan(n); ok {
			zones = append(zones, span)
		}

		return ast.WalkContinue, nil
	})
	return zones
}

func htmlCodeSpan(n ast.Node) (graph.Span, bool) {
	switch n.Kind() {
	case ast.KindCodeBlock, ast.KindFencedCodeBlock:
		lines := n.Lines()
		if lines.Len() == 0 {
			return graph.Span{}, false
		}
		return graph.Span{Start: lines.At(0).Start, Stop: lines.At(lines.Len() - 1).Stop}, true
	case ast.KindCodeSpan:
		first, last := -1, 0
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			part, ok := child.(*ast.Text)
			if !ok {
				continue
			}
			if first < 0 {
				first = part.Segment.Start
			}
			last = part.Segment.Stop
		}
		if first >= 0 {
			return graph.Span{Start: first, Stop: last}, true
		}
	}
	return graph.Span{}, false
}

// htmlAt keeps a literal or recognized marker, or consumes a private comment.
// offset is the source coordinate of line's current beginning, not a position
// in the shrinking visible buffer.
func (s *commentState) htmlAt(line, body string, offset, mark int) (kept string, end int, opened bool) {
	open := offset + mark
	span, closed := graph.HTMLCommentSpan(body, open)
	if span.Zero() || graph.In(s.htmlCode, open) {
		return line[mark : mark+4], mark + 4, false
	}
	if stop, ok := s.limits[open]; ok && stop < span.Stop {
		span.Stop, closed = stop, false
	}
	if closed && span.Stop <= offset+len(line) {
		end = span.Stop - offset
		if readAloudMarker.MatchString(line[mark:end]) {
			return line[mark:end], end, false
		}
		return "", end, false
	}
	s.closing, s.stop = "-->", span.Stop
	// A comment its Markdown container ends keeps the words after that
	// container; one that never closes hides every word after it, as %% does.
	s.unclosed = !closed && span.Stop == len(body)
	if !closed && span.Stop < len(body) {
		s.containers = append(s.containers, unclosedComment{
			line:   s.sourceLine,
			marker: "<!--",
		})
	}
	start := strings.LastIndex(body[:open], "\n") + 1
	s.quoteDepth = strings.Count(htmlCommentQuotePrefix(body[start:open]), ">")
	return "", len(line), true
}

// stripObsidianCommentLine removes the hidden parts of one line and reports
// whether a comment left open when it returns was opened here; one carried in from
// an earlier line keeps that line's number. Inside a code span the delimiters
// are displayed text, and spans read as the CommonMark spec reads them: a run of N
// backticks closes on the next run of exactly N, and an unpaired run is ordinary
// text. The line walk protects %% in single-line spans; HTML comments also use
// the parsed multi-line code spans. An open comment hides backticks.
func stripObsidianCommentLine(line, body string, offset int, state *commentState) (visible string, openedHere bool) {
	var b strings.Builder
	originalLength := len(line)
	if state.closing == "-->" && state.quoteDepth > 0 {
		prefix := htmlCommentQuotePrefix(line)
		b.WriteString(prefix)
		line = line[len(prefix):]
	}
	for line != "" {
		if state.closing != "" {
			_, after, found := strings.Cut(line, state.closing)
			if !found {
				return b.String(), openedHere
			}
			state.closing, openedHere = "", false
			line = after
			continue
		}
		tick := strings.IndexByte(line, '`')
		mark, html := strings.Index(line, "%%"), strings.Index(line, "<!--")
		if html >= 0 && (mark < 0 || html < mark) {
			mark = html
		}
		switch {
		case mark >= 0 && (tick < 0 || mark < tick):
			b.WriteString(line[:mark])
			if mark == html {
				kept, end, opened := state.htmlAt(line, body, offset+originalLength-len(line), mark)
				b.WriteString(kept)
				line, openedHere = line[end:], opened
				continue
			}
			state.closing, openedHere = "%%", true
			line = line[mark+2:]
		case tick >= 0:
			end, _ := codeSpanAt(line, tick)
			b.WriteString(line[:end])
			line = line[end:]
		default:
			b.WriteString(line)
			line = ""
		}
	}
	return b.String(), openedHere
}

// unclosedCommentDiagnostic describes a marker that never met its pair, carrying
// the line number beside the marker itself, which is what a reader needs to find
// where their words stopped appearing. Each body is scanned once, where it is
// first read, so no second pass can reopen what the first ruled literal.
func unclosedCommentDiagnostic(unclosed unclosedComment) Diagnostic {
	return Diagnostic{
		Kind:    DiagCommentUnclosed,
		Target:  unclosed.marker,
		Message: fmt.Sprintf("an unclosed %s comment opened at line %d of the note body hides everything after it", unclosed.marker, unclosed.line),
	}
}

// commentDiagnostics converts one scan's original source records for host and
// embedded publication. Only the body-wide kind explains an empty page.
func commentDiagnostics(report commentReport) []Diagnostic {
	var diagnostics []Diagnostic
	for _, container := range report.containers {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagCommentContainerUnclosed,
			Target:  container.marker,
			Message: fmt.Sprintf("an unclosed %s comment opened at line %d of the note body hides the rest of its Markdown container", container.marker, container.line),
		})
	}
	if report.bodywide.line != 0 {
		diagnostics = append(diagnostics, unclosedCommentDiagnostic(report.bodywide))
	}
	return diagnostics
}
