package graph

import (
	"regexp"
	"strings"
)

// The line shapes the page and the check both read when they walk a note:
// which lines are headings, which cannot be the prose an underline turns into
// one, and which open an authored HTML block whose contents reach the reader
// as written. They lived twice, once on each face, and then they were not
// the same scan; every face that asks these questions reads them from here.

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

	// HTMLBlockElement matches a CommonMark type-6 HTML block opening: a
	// known tag at up to three spaces of indent. A block opened by one of
	// these hands its lines over as written, so a '#' line inside one is
	// text and not a heading. The type-7 complete-tag condition is omitted
	// because it cannot interrupt a paragraph, and telling those readings
	// apart needs paragraph state a line scan does not keep.
	HTMLBlockElement = regexp.MustCompile(`(?i)^ {0,3}</?(address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h1|h2|h3|h4|h5|h6|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)([ \t]|/?>|$)`)

	// HTMLBlockRawEnd closes any of the four raw-text HTML elements.
	HTMLBlockRawEnd = regexp.MustCompile(`(?i)</(script|pre|style|textarea)>`)

	// The four raw-text HTML openings, each recognising a self-closing form
	// (`<pre/>`) as well as the ordinary tag. A self-closing pre is still an
	// opening: CommonMark does not close a type-1 block on `/>`, so the lines
	// after it are raw text until a matching end tag, and a heading-shaped
	// one of them is not a heading.
	HTMLBlockRawPre      = regexp.MustCompile(`(?i)^ {0,3}<pre(?:[ \t>]|/>|$)`)
	HTMLBlockRawScript   = regexp.MustCompile(`(?i)^ {0,3}<script(?:[ \t>]|/>|$)`)
	HTMLBlockRawStyle    = regexp.MustCompile(`(?i)^ {0,3}<style(?:[ \t>]|/>|$)`)
	HTMLBlockRawTextarea = regexp.MustCompile(`(?i)^ {0,3}<textarea(?:[ \t>]|/>|$)`)

	HTMLBlockComment = regexp.MustCompile(`^ {0,3}<!--`)
	HTMLBlockInstr   = regexp.MustCompile(`^ {0,3}<\?`)
	HTMLBlockCDATA   = regexp.MustCompile(`^ {0,3}<!\[CDATA\[`)
	HTMLBlockDecl    = regexp.MustCompile(`^ {0,3}<![A-Za-z]`)

	// quoteMarker is the single leading quote the unanchorable-line test
	// peels before asking whether a row opens with a pipe. It is the same
	// shape the page peels; QuotedLine is the CommonMark indent and does
	// not consume the optional space after `>`.
	quoteMarker = regexp.MustCompile(`^\s*>\s?`)

	// calloutOpening matches an Obsidian callout's first line. The type
	// list below is the closed set the page recognises; a type outside it
	// is a blockquote, and a blockquote can carry an address.
	calloutOpening = regexp.MustCompile(`^\s*>\s*\[!([A-Za-z]+)\]([+-]?)\s?(.*)$`)
)

// recognisedCalloutTypes is every callout type the page answers to, written
// here because the page's own list lives in render/callout.go, which this
// package must not import. A type added there and not here would let a title
// carry an address the page would not stamp; a type added here and not there
// would refuse an address the page stamps. Unknown types stay off this set
// on purpose: they are not callouts.
var recognisedCalloutTypes = map[string]bool{
	"info": true, "note": true, "tip": true, "hint": true, "abstract": true, "summary": true, "todo": true,
	"question": true, "help": true, "faq": true,
	"example": true,
	"quote":   true, "cite": true,
	"warning": true, "caution": true, "attention": true,
	"danger": true, "error": true, "bug": true, "fail": true, "failure": true, "missing": true,
}

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

// FenceOpens reports whether a line opens a fenced code block, and with which
// marker byte.
func FenceOpens(line string) (byte, bool) {
	t := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(t, "```"):
		return '`', true
	case strings.HasPrefix(t, "~~~"):
		return '~', true
	default:
		return 0, false
	}
}

// FenceCloses reports whether a line closes the open fence: trimmed, at least
// three characters, all of them the fence marker.
func FenceCloses(line string, marker byte) bool {
	t := strings.TrimSpace(line)
	return len(t) >= 3 && strings.Count(t, string(marker)) == len(t)
}

// LineScan carries the running state a line-by-line walk needs to tell a
// heading from a heading-shaped line inside fenced code or an authored HTML
// block, whose contents reach the reader as written. The zero value starts a
// scan.
type LineScan struct {
	inFence    bool
	fenceByte  byte
	htmlCloses func(string) bool
}

// Skip advances the scan by one line and reports whether that line belongs to
// a fenced code block or an authored HTML block, the lines that open and close
// one included.
func (s *LineScan) Skip(line string) bool {
	switch {
	case s.inFence:
		if FenceCloses(line, s.fenceByte) {
			s.inFence = false
		}
		return true
	case s.htmlCloses != nil:
		if s.htmlCloses(line) {
			s.htmlCloses = nil
		}
		return true
	}
	if marker, ok := FenceOpens(line); ok {
		s.inFence, s.fenceByte = true, marker
		return true
	}
	if closes, ok := HTMLBlockOpens(line); ok {
		if !closes(line) {
			s.htmlCloses = closes
		}
		return true
	}
	return false
}

// HTMLBlockOpens reports whether a line opens an authored HTML block, and
// returns the test for the line that closes it. The raw-text, comment,
// instruction, CDATA, and declaration blocks close on their own end marker,
// which may sit on the opening line itself; an element block runs to the next
// blank line. CDATA is asked before a declaration because both begin `<!`.
func HTMLBlockOpens(line string) (closes func(string) bool, ok bool) {
	switch {
	case HTMLBlockRawPre.MatchString(line),
		HTMLBlockRawScript.MatchString(line),
		HTMLBlockRawStyle.MatchString(line),
		HTMLBlockRawTextarea.MatchString(line):
		return HTMLBlockRawEnd.MatchString, true
	case HTMLBlockComment.MatchString(line):
		return lineContains("-->"), true
	case HTMLBlockInstr.MatchString(line):
		return lineContains("?>"), true
	case HTMLBlockCDATA.MatchString(line):
		return lineContains("]]>"), true
	case HTMLBlockDecl.MatchString(line):
		return lineContains(">"), true
	case HTMLBlockElement.MatchString(line):
		return BlankLine, true
	}
	return nil, false
}

func lineContains(marker string) func(string) bool {
	return func(line string) bool { return strings.Contains(line, marker) }
}

// UnanchorableLine reports whether a line is one no block address can survive
// on. Both entries are lines something downstream takes apart: a recognised
// callout's opening line, which is consumed as the block's title, and a table
// row, which is cut into cells against its header's column count and drops
// whatever follows the last. An unknown callout type is a blockquote, not a
// callout, and can carry an address.
func UnanchorableLine(line string) bool {
	if m := calloutOpening.FindStringSubmatch(line); m != nil {
		if recognisedCalloutTypes[strings.ToLower(m[1])] {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimLeft(quoteMarker.ReplaceAllString(line, ""), " \t"), "|")
}
