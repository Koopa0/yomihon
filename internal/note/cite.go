package note

import (
	"cmp"
	"fmt"
	"path"
	"strings"
	"unicode"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// Citation is one place the reader marked, and what they said of it, as it is
// handed out of yomihon: the words they chose, where in the vault they stood,
// and the reader's own words about them.
type Citation struct {
	// Title is the title of the note, as its link is written. When it has none
	// the link is written with the file's name.
	Title string
	// Path is the note's path from the vault's root, written with slashes.
	Path string
	// Section is the name of the section the words are in. It is empty for a
	// mark before the first heading, which has no section to name.
	Section string
	// Quote is the words the reader chose; empty for a mark on a whole section.
	Quote string
	// Code says Quote is a block of code, which is quoted whole, as it is
	// written, whatever a page would make of it. Language is the language the
	// block was marked with, written after the fence when it is a word that
	// can stand there.
	Code     bool
	Language string
	// Words is what the reader wrote about it, in any of the states of a mark.
	Words string
}

// Cite is the one way a mark is written out: copied from a page, or handed over
// by the command line. Nothing else makes this text, so what a reader pastes
// and what an export carries are one shape:
//
//	> the words the reader chose
//
//	— [Title](path/from/the/vault/root.md), Section
//
//	the reader's own words
//
// The quote and the reader's words are left out when there are none; a mark on
// a whole section is its source line alone.
//
// A quote is what the reader saw, so it is written in a quotation block only
// when that block, drawn by the page's own renderer, shows those words again,
// ignoring the spaces between them. Otherwise it is put in a fence inside the
// block, which draws exactly what is in it, and the fence is longer than any
// run of backticks in the quote so that nothing in the quote can end it.
// htmlOf is that renderer: it turns Markdown into the HTML the page would draw.
//
// Whatever the quote, the title, the path and the section hold, they cannot
// leave the shape above: the text of a line is written so that it stays one
// line of the one link, the path is written as the destination of the link, and
// no line of the quote can end the quotation. The reader's words are Markdown
// of their own and are written as they are, after a blank line.
func Cite(c *Citation, lang wording.Lang, htmlOf func(markdown string) string) string {
	if htmlOf == nil {
		panic("note: Cite needs the renderer the page draws with")
	}
	var b strings.Builder
	if quote := trimBlank(splitLines(c.Quote)); len(quote) > 0 {
		b.WriteString(quotation(quote, c, htmlOf))
		b.WriteString("\n\n")
	}
	b.WriteString(sourceLine(c, lang))
	if words := trimBlank(splitLines(c.Words)); len(words) > 0 {
		b.WriteString("\n\n")
		b.WriteString(strings.Join(words, "\n"))
	}
	return b.String()
}

// splitLines is the lines of text, whichever way each one ends: a bare carriage
// return ends a line to Markdown as a line feed does.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

// trimBlank drops the lines that hold nothing from either end.
func trimBlank(lines []string) []string {
	blank := func(l string) bool { return strings.TrimSpace(l) == "" }
	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func quotation(lines []string, c *Citation, htmlOf func(string) string) string {
	if !c.Code {
		plain := prefixed(lines)
		if drawnAsWritten(plain, lines, htmlOf) {
			return plain
		}
	}
	longest := 0
	for _, l := range lines {
		run := 0
		for _, r := range l {
			if r != '`' {
				run = 0
				continue
			}
			run++
			longest = max(longest, run)
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	out := make([]string, 0, len(lines)+2)
	out = append(out, fence+infoString(c.Language))
	out = append(out, lines...)
	out = append(out, fence)
	return prefixed(out)
}

// prefixed is lines inside a quotation block.
func prefixed(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if l == "" {
			b.WriteByte('>')
			continue
		}
		b.WriteString("> ")
		b.WriteString(l)
	}
	return b.String()
}

// drawnAsWritten says the page draws block with the words of lines and no
// others. Spaces are not words: a line that wraps is a space on one page and
// nothing between two Chinese characters on another, and a reader selecting it
// gets either.
func drawnAsWritten(block string, lines []string, htmlOf func(string) string) bool {
	return withoutSpace(render.TextContent(htmlOf(block))) == withoutSpace(strings.Join(lines, " "))
}

func withoutSpace(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, text)
}

// infoString is the language a fence says, when it is one word of the kind a
// language is named with, and nothing else: the line of a fence cannot hold a
// backtick, and a block marked with anything odd is fenced all the same.
func infoString(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return ""
	}
	for _, r := range language {
		if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_+#.-", r)) {
			return ""
		}
	}
	return language
}

func sourceLine(c *Citation, lang wording.Lang) string {
	title := cmp.Or(oneLine(c.Title), oneLine(strings.TrimSuffix(path.Base(c.Path), path.Ext(c.Path))))
	line := "— [" + escapeInline(title) + "](" + destination(c.Path) + ")"
	if section := oneLine(c.Section); section != "" {
		line += wording.CiteSectionSep.In(lang) + escapeInline(section)
	}
	return line
}

// oneLine is text on one line: every run of spaces, line ends and controls is
// one space.
func oneLine(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
}

// inlineMarkup are the characters of a line that Markdown, or the page's own
// additions to it, may read as the start of something. Any ASCII punctuation
// can be written after a backslash and means itself.
const inlineMarkup = "\\`*_[]<>&~=%^|$"

func escapeInline(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r <= unicode.MaxASCII && strings.ContainsRune(inlineMarkup, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// destination is a path as the destination of a link: written between angle
// brackets when it holds a space or a parenthesis, which end one without
// them, and so is anything else that is only plain between them.
func destination(rel string) string {
	var b strings.Builder
	for _, r := range rel {
		if unicode.IsControl(r) {
			for _, by := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", by)
			}
			continue
		}
		b.WriteRune(r)
	}
	plain := b.String()
	if plain != "" && !strings.ContainsAny(plain, " \t()<>\\&") {
		return plain
	}
	var angled strings.Builder
	angled.WriteByte('<')
	for _, r := range plain {
		if r == '\\' || r == '<' || r == '>' || r == '&' {
			angled.WriteByte('\\')
		}
		angled.WriteRune(r)
	}
	angled.WriteByte('>')
	return angled.String()
}
