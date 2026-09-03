package render

import (
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

// The CommonMark block structure an embed has to read before it can cut a
// section out of a note: which lines are headings, which are inside a fence or
// an HTML block and therefore text, and where one section ends. It is a source
// scan rather than a pass over rendered HTML, because an embed picks its slice
// out of the file's own bytes and hands that to the renderer.
//
// It is here rather than beside the wikilink rendering it serves because it
// answers about the document's shape and knows nothing about links: a heading
// is a heading whether or not anybody points at it.

// atxHeadingLine matches an ATX heading the way goldmark will read it: up to
// three spaces of indent, one to six '#' characters, then whitespace before
// the text. A '#' run glued to text is not a heading in CommonMark and is
// not one here.
var atxHeadingLine = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*)$`)

// The HTML block start conditions of the CommonMark spec, minus the one for a
// bare complete tag alone on its own line. A block opened by any of these
// hands its lines to the reader as written, so a '#' line inside one is text
// rather than a section boundary — the line is quoted markup in a note about
// markup, or a comment, as often as it is anything else.
//
// The omitted condition is the one that cannot interrupt a paragraph, and
// telling those two readings apart needs paragraph state this scan does not
// keep. Leaving it out costs a '#' line inside such a block, and buys never
// mistaking an ordinary sentence that carries a tag for the start of one.
var (
	htmlBlockRawText = regexp.MustCompile(`(?i)^ {0,3}<(script|pre|style|textarea)([ \t>]|$)`)
	htmlBlockRawEnd  = regexp.MustCompile(`(?i)</(script|pre|style|textarea)>`)
	htmlBlockComment = regexp.MustCompile(`^ {0,3}<!--`)
	htmlBlockInstr   = regexp.MustCompile(`^ {0,3}<\?`)
	htmlBlockDecl    = regexp.MustCompile(`^ {0,3}<![A-Za-z]`)
	htmlBlockCDATA   = regexp.MustCompile(`^ {0,3}<!\[CDATA\[`)
	htmlBlockElement = regexp.MustCompile(`(?i)^ {0,3}</?(address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h1|h2|h3|h4|h5|h6|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)([ \t]|/?>|$)`)
)

// htmlBlockOpen reports whether line opens an authored HTML block, and returns
// the test for the line that closes it. The raw-text, comment, instruction,
// declaration, and CDATA blocks close on their own end marker — which may sit
// on the opening line itself — while an element block runs to the next blank
// line.
func htmlBlockOpen(line string) (closes func(string) bool, ok bool) {
	switch {
	case htmlBlockRawText.MatchString(line):
		return htmlBlockRawEnd.MatchString, true
	case htmlBlockComment.MatchString(line):
		return closesOn("-->"), true
	case htmlBlockInstr.MatchString(line):
		return closesOn("?>"), true
	case htmlBlockCDATA.MatchString(line):
		return closesOn("]]>"), true
	case htmlBlockDecl.MatchString(line):
		return closesOn(">"), true
	case htmlBlockElement.MatchString(line):
		return blankLine, true
	}
	return nil, false
}

func closesOn(marker string) func(string) bool {
	return func(line string) bool { return strings.Contains(line, marker) }
}

func blankLine(line string) bool { return strings.TrimSpace(line) == "" }

// blockScan carries the running state a line-by-line section scan needs to
// tell a real heading from a heading-looking line: fenced code and authored
// HTML blocks both hand their contents to the reader as written, so a line
// inside either is content and never a boundary. The zero value starts a scan.
type blockScan struct {
	inFence    bool
	fenceByte  byte
	htmlCloses func(string) bool
}

// skips advances the scan by one line and reports whether that line is inside
// a fenced code block or an authored HTML block — including the line that
// opens or closes one, which belongs to the block rather than to the prose
// around it.
func (s *blockScan) skips(line string) bool {
	switch {
	case s.inFence:
		if fenceCloses(line, s.fenceByte) {
			s.inFence = false
		}
		return true
	case s.htmlCloses != nil:
		if s.htmlCloses(line) {
			s.htmlCloses = nil
		}
		return true
	}
	if marker, _, ok := fenceOpen(line); ok {
		s.inFence, s.fenceByte = true, marker
		return true
	}
	if closes, ok := htmlBlockOpen(line); ok {
		if !closes(line) {
			s.htmlCloses = closes
		}
		return true
	}
	return false
}

// setextUnderline matches the line that underlines a heading written without
// '#' marks, and reports the level it makes: '=' is a level-1 heading, '-' a
// level-2 one.
var setextUnderline = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)

// The line shapes that are not running prose, and therefore cannot be the text
// an underline turns into a heading: a quote, a list item, a break rule, and
// an indented code line. Anything else that is not blank continues a
// paragraph.
var (
	quotedLine       = regexp.MustCompile(`^ {0,3}>`)
	listItemLine     = regexp.MustCompile(`^ {0,3}(?:[-*+]|\d{1,9}[.)])(?:[ \t]|$)`)
	breakRuleLine    = regexp.MustCompile(`^ {0,3}((\*[ \t]*){3,}|(_[ \t]*){3,}|(-[ \t]*){3,})$`)
	indentedCodeLine = regexp.MustCompile(`^ {4,}\S`)
)

// sectionHeading is one heading a scan found: the line its section opens on,
// its level, and the source text its anchor is folded from. An underlined
// heading opens on the first line of the text, not on the underline.
type sectionHeading struct {
	line  int
	level int
	text  string
}

// scanHeadings reports every heading in lines, in document order, reading them
// the way the page that displays them does: '#'-marked and underlined headings
// both count, and a heading-looking line inside fenced code or an authored
// HTML block counts as neither.
//
// An underline only makes a heading of running prose, so the shapes that open
// a different construct — a quote, a list item, a break rule, an indented code
// line — end the run of lines an underline could claim. Where the reading is
// genuinely ambiguous the scan keeps the plainer one, which costs an
// underlined heading and never invents one.
func scanHeadings(lines []string) []sectionHeading {
	var out []sectionHeading
	var scan blockScan
	paragraph := -1
	for i, line := range lines {
		if scan.skips(line) {
			paragraph = -1
			continue
		}
		if m := atxHeadingLine.FindStringSubmatch(line); m != nil {
			out = append(out, sectionHeading{line: i, level: len(m[1]), text: m[2]})
			paragraph = -1
			continue
		}
		switch {
		case paragraph >= 0 && setextUnderline.MatchString(line):
			level := 2
			if strings.HasPrefix(strings.TrimSpace(line), "=") {
				level = 1
			}
			out = append(out, sectionHeading{line: paragraph, level: level, text: strings.Join(lines[paragraph:i], "\n")})
			paragraph = -1
		case blankLine(line), quotedLine.MatchString(line), listItemLine.MatchString(line),
			breakRuleLine.MatchString(line), setextUnderline.MatchString(line),
			paragraph < 0 && indentedCodeLine.MatchString(line):
			paragraph = -1
		case paragraph < 0:
			paragraph = i
		}
	}
	return out
}

// headingSlice returns the section of body that heading names: the first
// heading whose text folds to the same slug as the name, through to the line
// before the next heading of the same or a higher level. Deeper headings stay
// inside the slice, because a section owns its subsections. When the name
// appears twice the first occurrence wins, matching Obsidian's reading view.
//
// The name is folded through slugify, the same function that stamps the
// destination page's anchors, over heading text reduced the same way that pass
// reduces it: a link contributes the words it displays, a ruby reading drops
// out, tags and character references resolve. So the spellings the destination
// page lists in its own table of contents are the spellings an embed of that
// section accepts.
//
// One spelling does not survive the trip, because it is read from source
// rather than from the rendered page: a heading carrying a markdown link keeps
// the address the rendered heading drops. It reports, it does not truncate.
func headingSlice(body, heading string) (slice string, matches int) {
	want := slugify(heading)
	lines := strings.Split(body, "\n")
	headings := scanHeadings(lines)
	for i, h := range headings {
		if slugify(headingSourceText(h.text)) != want {
			continue
		}
		matches++
		if matches > 1 {
			// The later ones are counted, not cut: the excerpt is the first,
			// and what the rest are for is to say how many there were.
			continue
		}
		slice = strings.Join(lines[h.line:], "\n")
		for _, next := range headings[i+1:] {
			if next.level <= h.level {
				slice = strings.Join(lines[h.line:next.line], "\n")
				break
			}
		}
	}
	return slice, matches
}

// headingSourceText reduces a heading's markdown source to the text the page
// stamps its anchor from. A wikilink contributes what it displays, which is
// the target itself unless the author wrote a display alias — the rendered
// heading shows exactly those words, and a reader copying the section's name
// off the page copies them too.
func headingSourceText(raw string) string {
	displayed := wikilinkToken.ReplaceAllStringFunc(raw, func(token string) string {
		inner := strings.TrimPrefix(token, "!")
		_, display, _ := graph.SplitWikilink(inner[2 : len(inner)-2])
		return display
	})
	return headingInnerText(displayed)
}

// blockSlice returns the block carrying the "^name" marker: the run of
// non-blank lines around the first line outside fenced code that ends with the
// marker, stopping at a list item's own line so a marker written on one item
// addresses that item rather than the list it sits in. Where the marked line
// is a continuation, the run reaches back to the line its block opens on — a
// marker written under a multi-line block reaches that block the same way,
// since no blank line separates them.
//
// The address is matched through the fold both fragment kinds share, so
// "^Quote1" and "#^quote1" are one name. Only case and Unicode form fold: the
// rest of an address is an identifier the author chose, and "^quote-1" and
// "^quote1" are two of them.
//
// Nothing external rules on how wide a block reference reaches, so the narrow
// reading is taken: the reader asked for the line the author marked, and
// widening it here would be this renderer choosing an excerpt the author did
// not.
func blockSlice(body, block string) (string, bool) {
	lines := strings.Split(body, "\n")
	at := blockMarkerLine(lines, block)
	if at < 0 {
		return "", false
	}
	start := at
	for start > 0 && !listItemLine.MatchString(lines[start]) && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	end := at + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" && !listItemLine.MatchString(lines[end]) {
		end++
	}
	return strings.Join(lines[start:end], "\n"), true
}

// blockMarkerLine reports which line carries the marker naming block, or -1
// when the note has no such marker. A marker written inside a fenced block is
// code rather than an address, so the scan tracks fences as it walks.
func blockMarkerLine(lines []string, block string) int {
	want := foldFragment("^" + block)
	inFence, fenceByte := false, byte(0)
	for i, line := range lines {
		// A fence is looked for with any quote marker taken off it, because a
		// fence written inside a callout opens one: that body is read on its
		// own with the markers stripped, and a line of code in it is code.
		unquoted := quotePrefix.ReplaceAllString(line, "")
		if inFence {
			if fenceCloses(unquoted, fenceByte) {
				inFence = false
			}
			continue
		}
		if open, _, ok := fenceOpen(unquoted); ok {
			inFence, fenceByte = true, open
			continue
		}
		if unanchorableLine(line) {
			continue
		}
		trimmed := foldFragment(strings.TrimRight(line, " \t"))
		if trimmed == want || strings.HasSuffix(trimmed, " "+want) || strings.HasSuffix(trimmed, "\t"+want) {
			return i
		}
	}
	return -1
}
