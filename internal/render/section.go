package render

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/wording"
)

// The CommonMark block structure an embed reads before it can cut a section out
// of a note: which lines are headings, which are inside a fence or an HTML block
// and therefore text, and where one section ends. It scans source rather than
// rendered HTML, because an embed slices the file's own bytes.

// atxHeadingLine matches an ATX heading the way goldmark reads it: up to three
// spaces of indent, one to six '#' characters, then whitespace. A '#' run glued
// to text is not a heading in CommonMark and is not one here. The second group
// is the heading's words, which stop before a closing run of '#': CommonMark
// reads a trailing run preceded by whitespace as part of the marks, and the
// rendered heading shows neither the run nor the space before it. A run with no
// whitespace before it is text, and stays in the words.
var atxHeadingLine = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$`)

// The HTML block start conditions of the CommonMark spec, minus the one for a
// bare complete tag alone on its line. A block opened by any of these hands its
// lines over as written, so a '#' line inside one is text and not a boundary.
// The omitted condition cannot interrupt a paragraph, and telling those readings
// apart needs paragraph state this scan does not keep.
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
// the test for the line closing it. Raw-text, comment, instruction, declaration
// and CDATA blocks close on their own end marker, which may sit on the opening
// line; an element block runs to the next blank line.
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

// blockScan carries the running state a line-by-line section scan needs to tell a
// real heading from a heading-looking line: a line inside fenced code or an
// authored HTML block is content, never a boundary. The zero value starts a scan.
type blockScan struct {
	inFence    bool
	fenceByte  byte
	htmlCloses func(string) bool
}

// skips advances the scan by one line and reports whether that line is inside a
// fenced code block or an authored HTML block, the lines opening and closing one
// included.
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

// setextLevel is the level an underline makes, for a line the caller has
// already recognized as one.
func setextLevel(line string) int {
	if strings.HasPrefix(strings.TrimSpace(line), "=") {
		return 1
	}
	return 2
}

// The line shapes that are not running prose, and so cannot be the text an
// underline turns into a heading: a quote, a list item, a break rule, an indented
// code line. Anything else that is not blank continues a paragraph.
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
// the way the page that displays them does: '#'-marked and underlined both count,
// and a heading-looking line inside fenced code or an authored HTML block counts
// as neither. An underline only makes a heading of running prose, and where the
// reading is ambiguous the scan keeps the plainer one, which never invents a heading.
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
			out = append(out, sectionHeading{
				line:  paragraph,
				level: setextLevel(line),
				text:  strings.Join(lines[paragraph:i], "\n"),
			})
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

// headingSlice returns the section of body that heading names: the first heading
// whose text folds to the same slug, through to the line before the next heading
// of the same or a higher level, deeper ones included. A repeated name takes the
// first, as Obsidian's reading view does. The name folds through slugify over
// heading text reduced the way the anchor pass reduces it, so the destination's
// own table of contents lists the spellings an embed accepts.
func headingSlice(body, heading string) (slice string, matches int) {
	want := slugify(heading)
	lines := strings.Split(body, "\n")
	headings := scanHeadings(lines)
	for i, h := range headings {
		if slugify(headingSourceText(h.text, h.level)) != want {
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

// Excerpt is the part of body that a link's own fragment addresses: a caret
// opens a block address, anything else names a section, and an empty fragment
// asks for the note itself. Obsidian's %% comments come off before any edge is
// chosen, so a marker cannot span the cut and arrive visible in the excerpt.
//
// An address the note does not answer to comes back not found, and the caller
// says so. Widening to the whole note would answer a question nobody asked —
// the reader named one place, and being shown a different one without being
// told reads as the place they named.
//
// The fragment is the one an anchor already carries, already folded by the pass
// that wrote it; nothing here folds a name a second time.
func Excerpt(body, fragment string) (slice string, found bool) {
	stripped, _ := stripObsidianComments(body)
	slice, matches := excerptOf(stripped, fragment)
	return slice, matches > 0
}

// excerptOf is the one cut every excerpt is made with, over a body whose
// comments are already off: it returns the part of it that fragment addresses,
// spelled as Excerpt reads it, and counts the places that answered — one for a
// block or for the note itself, and for a section every heading folding to the
// name, of which the first is cut. Zero is an address the note does not answer
// to, and then nothing is cut: there is no narrower answer than the one asked
// for, and a wider one would be this renderer's rather than the author's.
func excerptOf(stripped, fragment string) (slice string, matches int) {
	switch {
	case strings.HasPrefix(fragment, "^"):
		cut, ok := blockSlice(stripped, strings.TrimPrefix(fragment, "^"))
		if !ok {
			return "", 0
		}
		return cut, 1
	case fragment != "":
		return headingSlice(stripped, fragment)
	}
	return stripped, 1
}

// fragmentOf is the address an embed carries, in the spelling Excerpt reads. A
// block wins when the author wrote both a block and a section, which is the
// order a link's address resolves that conflict in too.
func fragmentOf(link graph.Wikilink) string {
	if link.Block != "" {
		return "^" + link.Block
	}
	return link.Heading
}

// ExcerptWithheld is what a surface says where an excerpt it could not cut
// would have stood: the address the note does not answer to, spelled the way
// the author's own link spells it, and the note it was asked of. The reading
// page says it inside the block an embed leaves behind and the hover card says
// it under the note's name, from this one sentence, so the two report one fact
// in one voice. fragment is the one Excerpt reads.
func ExcerptWithheld(relPath, fragment string, lang wording.Lang) string {
	return fmt.Sprintf(wording.ExcerptWithheldFmt.In(lang), "#"+fragment, noteName(relPath))
}

// noteName is the name a note is cited by: its file's own, without the
// extension. A provenance line and a withheld notice both call a note this,
// because it is the name a citation resolves by.
func noteName(relPath string) string {
	return strings.TrimSuffix(path.Base(relPath), ".md")
}

// ExcerptHeading is the words a reader sees in the heading an excerpt opens on,
// or empty when it does not open on one — a block excerpt, or a note read from
// its first line. It reduces the heading the way the anchor pass does, so a
// reading that names a section names it as its own table of contents does.
func ExcerptHeading(slice string) string {
	lines := strings.SplitN(slice, "\n", 3)
	if len(lines) == 0 {
		return ""
	}
	if m := atxHeadingLine.FindStringSubmatch(lines[0]); m != nil {
		return headingSourceText(m[2], len(m[1]))
	}
	// A heading written under its own underline opens on the line of text, so
	// the line below it is what says the text was a heading at all.
	if len(lines) > 1 && setextUnderline.MatchString(lines[1]) && !blankLine(lines[0]) {
		return headingSourceText(lines[0], setextLevel(lines[1]))
	}
	return ""
}

// headingSourceText reduces a heading's markdown source to the text the page
// stamps its anchor from. A wikilink contributes what it displays, which is what
// the rendered heading shows and what a reader copies off the page. A course
// branch declares its part in the order at the end of the heading that opens
// it, and that declaration is grammar rather than words, so the level decides
// what the heading is called for the same reason it does on the page.
func headingSourceText(raw string, level int) string {
	displayed := wikilinkToken.ReplaceAllStringFunc(sequence.HeadingName(raw, level), func(token string) string {
		inner := strings.TrimPrefix(token, "!")
		_, display, _ := graph.SplitWikilink(inner[2 : len(inner)-2])
		return display
	})
	return headingInnerText(displayed)
}

// blockSlice returns the block carrying the "^name" marker: the run of non-blank
// lines around the first line outside fenced code ending with the marker, stopping
// at a list item's own line, and reaching back to the line its block opens on when
// the marked line is a continuation. The address matches through the fold both
// fragment kinds share, so "^quote-1" and "^quote1" stay two names. Nothing rules
// how wide a block reference reaches, so the narrow reading is taken.
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
