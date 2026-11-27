package graph

import (
	"regexp"
	"slices"
	"strings"
)

// The line shapes the page and the check both read when they walk a note:
// which lines are headings, which cannot be the prose an underline turns into
// one, and which open an authored HTML block whose contents reach the reader
// as written. Every face asks them here, so no two faces walk a note apart.

var (
	// ATXHeading matches an ATX heading the way goldmark reads it: up to three
	// spaces of indent, one to six '#' characters, then whitespace. A '#' run
	// glued to text is not a heading in CommonMark and is not one here. The
	// second group is the heading's words, which stop before a closing run of
	// '#': CommonMark reads a trailing run preceded by whitespace as part of
	// the marks, and the rendered heading shows neither the run nor the space
	// before it. A run with no whitespace before it is text, and stays in the
	// words.
	ATXHeading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$`)

	// SetextUnderline matches the line that underlines a heading written
	// without '#' marks.
	SetextUnderline = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)

	// QuotedLine matches a line that opens or continues a block quote at the
	// indent CommonMark allows.
	QuotedLine = regexp.MustCompile(`^ {0,3}>`)

	// ListItemLine matches a bullet or ordered list marker at the indent
	// CommonMark allows.
	ListItemLine = regexp.MustCompile(`^ {0,3}(?:[-*+]|\d{1,9}[.)])(?:[ \t]|$)`)

	// BreakRuleLine matches a thematic break: three or more '*', '_', or '-'
	// markers, spaces and tabs between them allowed.
	BreakRuleLine = regexp.MustCompile(`^ {0,3}((\*[ \t]*){3,}|(_[ \t]*){3,}|(-[ \t]*){3,})$`)

	// IndentedCodeLine matches a line that opens an indented code block when
	// it is not continuing a paragraph.
	IndentedCodeLine = regexp.MustCompile(`^ {4,}\S`)

	// htmlBlockElement matches a CommonMark type-6 HTML block opening: a
	// known tag at up to three spaces of indent. A block opened by one of
	// these hands its lines over as written, so a '#' line inside one is
	// text and not a heading. The type-7 complete-tag condition is omitted
	// because it cannot interrupt a paragraph, and telling those readings
	// apart needs paragraph state a line scan does not keep.
	htmlBlockElement = regexp.MustCompile(`(?i)^ {0,3}</?(address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h1|h2|h3|h4|h5|h6|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)([ \t]|/?>|$)`)

	// QuotePrefix matches the single leading quote marker a line scan peels
	// before asking whether a fence opened or a row begins with a pipe.
	// QuotedLine is the CommonMark indent and does not consume the optional
	// space after `>`.
	QuotePrefix = regexp.MustCompile(`^\s*>\s?`)
)

// SetextLevel is the level an underline makes, for a line the caller has
// already recognized as one: '=' underlines a level-1 heading, '-' a level-2
// one. A course branch is a heading from level 2 to 6, so a declaration
// written on an underlined title is not a branch the course parser opens.
func SetextLevel(line string) int {
	if strings.HasPrefix(strings.TrimSpace(line), "=") {
		return 1
	}
	return 2
}

// BlankLine reports whether line is empty or only whitespace.
func BlankLine(line string) bool { return strings.TrimSpace(line) == "" }

// FenceOpens reports whether a line opens a fenced code block, with which
// marker byte and how many of that marker the opener used. The count is what
// a closer has to meet: CommonMark will not close a longer fence with a
// shorter run of the same mark.
func FenceOpens(line string) (marker byte, n int, ok bool) {
	t := strings.TrimLeft(line, " \t")
	// CommonMark allows at most three spaces before an opening fence; deeper
	// than that the line is indented code, so a fence an author wrote as an
	// example of one opens nothing. A tab is four columns and is already too
	// deep.
	if indent := line[:len(line)-len(t)]; len(indent) > 3 || strings.Contains(indent, "\t") {
		return 0, 0, false
	}
	switch {
	case strings.HasPrefix(t, "```"):
		return '`', fenceMarkerRun(t, '`'), true
	case strings.HasPrefix(t, "~~~"):
		return '~', fenceMarkerRun(t, '~'), true
	default:
		return 0, 0, false
	}
}

func fenceMarkerRun(s string, marker byte) int {
	n := 0
	for n < len(s) && s[n] == marker {
		n++
	}
	return n
}

// FenceCloses reports whether a line closes the open fence: at most three
// spaces of indent, then all of the fence marker, and at least as long as the
// opener. A shorter all-marker line is content, not a close, and so is a
// deeper one — CommonMark keeps a marker written four spaces in inside the
// block, which is how a note shows what a fence looks like without ending the
// fence it is showing it in. This is the rule the opening side already keeps,
// and a tab is four columns and already too deep for either.
func FenceCloses(line string, marker byte, openerLen int) bool {
	t := strings.TrimLeft(line, " \t")
	if indent := line[:len(line)-len(t)]; len(indent) > 3 || strings.Contains(indent, "\t") {
		return false
	}
	t = strings.TrimSpace(t)
	if openerLen < 3 {
		openerLen = 3
	}
	return len(t) >= openerLen && strings.Count(t, string(marker)) == len(t)
}

// HTMLBlockKind is one of the HTML blocks CommonMark ends at a marker rather
// than at a blank line.
type HTMLBlockKind struct {
	// Opens matches the line that opens the block.
	Opens *regexp.Regexp
	// Ends matches the text that closes it, which may stand on the opening
	// line itself. The four raw-text elements share one: a block opened by any
	// of them is closed by any of their closing tags, as the Markdown parser
	// closes them.
	Ends *regexp.Regexp
	// Closing is the kind's own end, the text a scan stopped inside one writes
	// to close it.
	Closing string
}

// rawTextEnd closes any of the four raw-text HTML elements.
var rawTextEnd = regexp.MustCompile(`(?i)</(script|pre|style|textarea)>`)

// htmlBlockKinds are the HTML blocks a blank line does not end, in the order a
// line is asked: CDATA before a declaration, since both begin "<!". Each
// raw-text opening also takes its self-closing form (`<pre/>`): CommonMark does
// not close a raw-text block on "/>", so the lines after it are raw text until
// an end tag, and a heading-shaped one of them is not a heading.
var htmlBlockKinds = []HTMLBlockKind{
	{regexp.MustCompile(`(?i)^ {0,3}<pre(?:[ \t>]|/>|$)`), rawTextEnd, "</pre>"},
	{regexp.MustCompile(`(?i)^ {0,3}<script(?:[ \t>]|/>|$)`), rawTextEnd, "</script>"},
	{regexp.MustCompile(`(?i)^ {0,3}<style(?:[ \t>]|/>|$)`), rawTextEnd, "</style>"},
	{regexp.MustCompile(`(?i)^ {0,3}<textarea(?:[ \t>]|/>|$)`), rawTextEnd, "</textarea>"},
	{regexp.MustCompile(`^ {0,3}<!--`), regexp.MustCompile(`-->`), "-->"},
	{regexp.MustCompile(`^ {0,3}<\?`), regexp.MustCompile(`\?>`), "?>"},
	{regexp.MustCompile(`^ {0,3}<!\[CDATA\[`), regexp.MustCompile(`\]\]>`), "]]>"},
	{regexp.MustCompile(`^ {0,3}<![A-Za-z]`), regexp.MustCompile(`>`), ">"},
}

// HTMLBlockKinds are the HTML blocks a blank line does not end, in the order a
// line is asked. The slice is the caller's own; the patterns in it are shared
// and safe to use from any goroutine.
func HTMLBlockKinds() []HTMLBlockKind { return slices.Clone(htmlBlockKinds) }

// lineScan carries the running state a line-by-line walk needs to tell a
// heading from a heading-shaped line inside fenced code or an authored HTML
// block, whose contents reach the reader as written. The zero value starts a
// scan.
type lineScan struct {
	inFence    bool
	fenceByte  byte
	fenceLen   int
	htmlCloses func(string) bool
}

// skip advances the scan by one line and reports whether that line belongs to
// a fenced code block or an authored HTML block, the lines that open and close
// one included.
func (s *lineScan) skip(line string) bool {
	switch {
	case s.inFence:
		if FenceCloses(line, s.fenceByte, s.fenceLen) {
			s.inFence = false
		}
		return true
	case s.htmlCloses != nil:
		if s.htmlCloses(line) {
			s.htmlCloses = nil
		}
		return true
	}
	if marker, n, ok := FenceOpens(line); ok {
		s.inFence, s.fenceByte, s.fenceLen = true, marker, n
		return true
	}
	if closes, ok := htmlBlockOpens(line); ok {
		if !closes(line) {
			s.htmlCloses = closes
		}
		return true
	}
	return false
}

// LineSkipZones are the body ranges a line walk hides: fenced code and authored
// HTML blocks, the lines that open and close one included. The map's link
// scan reads them so a bracket sitting where a heading walk already refused
// cannot stay live.
func LineSkipZones(body string) []Span {
	if body == "" {
		return nil
	}
	var scan lineScan
	var zones []Span
	var start int
	in := false
	offset := 0
	for line := range strings.SplitSeq(body, "\n") {
		if scan.skip(line) {
			if !in {
				in, start = true, offset
			}
		} else if in {
			zones = append(zones, Span{Start: start, Stop: offset})
			in = false
		}
		offset += len(line) + 1
	}
	if in {
		zones = append(zones, Span{Start: start, Stop: len(body)})
	}
	return zones
}

// Heading is one heading a body's line walk found: where the heading opens,
// in both coordinates its readers cut by, the level its marks or its
// underline give it, and its source words. An underlined heading opens on
// the first line of its words rather than on the underline, so a reader that
// cuts from here keeps the words the heading is read by.
type Heading struct {
	// Line is the zero-based index of the opening line among the body's
	// lines; Start is the byte offset of that same line. One place, in the
	// two coordinates the readers of a body already cut with.
	Line  int
	Start int
	Level int
	// Text is the heading's source, folded by nothing: for a marked heading
	// the words between the marks and any closing run the author balanced
	// them with, and for an underlined one the lines of prose the underline
	// titles, joined by newline as the body wrote them.
	Text string
}

// Headings reports every heading in body, in document order, read the way the
// page that displays them does: marked with a run of one to six '#' at the
// indent CommonMark allows, or underlined beneath a run of prose. A line
// inside zones is not a heading — the caller passes the ranges its own face
// hides, so a face that hides more than a line walk can see hides it here
// too. An underline titles running prose alone: a blank line, a quote, a list
// item, a break rule, another underline, or an indented code line opening the
// run ends what it could claim. Where the reading is ambiguous the plainer
// one is kept, which never invents a heading.
func Headings(body string, zones []Span) []Heading {
	lines := strings.Split(body, "\n")
	var out []Heading
	paragraph, paragraphStart, offset := -1, 0, 0
	for i, line := range lines {
		start := offset
		offset = start + len(line) + 1
		// A CRLF file's carriage return is a line end, not part of a line's
		// shape or of a heading's words; offsets still count it.
		line = strings.TrimSuffix(line, "\r")
		lines[i] = line
		if In(zones, start) {
			paragraph = -1
			continue
		}
		if m := ATXHeading.FindStringSubmatch(line); m != nil {
			out = append(out, Heading{Line: i, Start: start, Level: len(m[1]), Text: m[2]})
			paragraph = -1
			continue
		}
		switch {
		case paragraph >= 0 && SetextUnderline.MatchString(line):
			out = append(out, Heading{
				Line:  paragraph,
				Start: paragraphStart,
				Level: SetextLevel(line),
				Text:  strings.Join(lines[paragraph:i], "\n"),
			})
			paragraph = -1
		case BlankLine(line), QuotedLine.MatchString(line), ListItemLine.MatchString(line),
			BreakRuleLine.MatchString(line), SetextUnderline.MatchString(line),
			paragraph < 0 && IndentedCodeLine.MatchString(line):
			paragraph = -1
		case paragraph < 0:
			paragraph, paragraphStart = i, start
		}
	}
	return out
}

// htmlBlockOpens reports whether a line opens an authored HTML block, and
// returns the test for the line that closes it. A kind in htmlBlockKinds closes
// on its own end marker, which may sit on the opening line itself; an element
// block runs to the next blank line.
func htmlBlockOpens(line string) (closes func(string) bool, ok bool) {
	for i := range htmlBlockKinds {
		if htmlBlockKinds[i].Opens.MatchString(line) {
			return htmlBlockKinds[i].Ends.MatchString, true
		}
	}
	if htmlBlockElement.MatchString(line) {
		return BlankLine, true
	}
	return nil, false
}
