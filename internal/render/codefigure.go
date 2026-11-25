package render

import (
	"html"
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

// A code block stands in a box of its own: a head that names the block's
// language and holds its copy control, over the code itself. The head is the
// box's first child, so the control never covers the code's first line, and the
// slot inside it is drawn by the server for the reason codeCopySlot gives.
//
// The language is the first word of the fence's info string, as the author
// wrote it; anything after it is not the language and is not shown. It is drawn
// from the attribute by the stylesheet rather than written as text, so it is
// neither copied with the page's words nor read as the first words of whatever
// section the block stands under.
//
// data-wide names the reading measures the block's longest line does not fit
// at: wideMeasures says how that is counted. The stylesheet sets a block that
// names the measure the page is reading at to the wide measure instead, where
// the page has room for one. The attribute is the opt-in for any block that
// needs more than the text's width, not only for code: a table or a figure that
// does names every measure.
func codeFigureOpen(lang, wide string) string {
	var b strings.Builder
	b.WriteString(`<div class="y-code"`)
	if wide != "" {
		b.WriteString(` data-wide="`)
		b.WriteString(wide)
		b.WriteString(`"`)
	}
	b.WriteString(`><div class="y-code__head">`)
	if lang != "" {
		b.WriteString(`<span class="y-code__lang" data-text="`)
		b.WriteString(html.EscapeString(lang))
		b.WriteString(`"></span>`)
	}
	b.WriteString(codeCopySlot)
	b.WriteString(`</div>`)
	return b.String()
}

const codeFigureClose = `</div>`

// codeFigure is a whole block: the box, its head and the pre the highlighter or
// the plain fallback wrote.
func codeFigure(lang, source, pre string) string {
	return codeFigureOpen(lang, wideMeasures(source)) + pre + codeFigureClose
}

// A line of code is measured in fifths of an em of the code face, which counts
// every advance the face draws in whole numbers. Geist Mono advances 0.6em for
// every character it draws, three fifths; a character it has no glyph for and
// East Asia sets full width (Han, kana, Hangul, the fullwidth forms) falls back
// to a CJK face at 1em, five fifths. A tab advances to the next stop the
// stylesheet's tab-size of 4 sets, four of the face's characters apart. A
// combining mark, a joiner or a variation selector adds nothing.
const (
	codeNarrowAdvance = 3
	codeWideAdvance   = 5
	codeTabStop       = 4 * codeNarrowAdvance
)

// codeMeasures are the reading measures a code block can stand at, each with
// the longest line, in fifths of an em of the code face, that fits inside it.
// A block stands at the article's measure less the 20px either side the
// stylesheet pads it by, set at --fs-ed-13 of the text size the reader chose.
// The article's measure is --measure-read, 660, 700 and 740px at the three text
// sizes; an article tagged as neither Chinese, Japanese nor Korean takes 31
// reading ems instead (17, 19 and 21px). The code face is 13, 15 and 17px. So
// the room is floor((measure - 40) / (size / 5)):
//
//	m         (660 - 40) / 2.6 = 238.5   l        (700 - 40) / 3.0 = 220
//	xl        (740 - 40) / 3.4 = 205.9   latin-m  (527 - 40) / 2.6 = 187.3
//	latin-l   (589 - 40) / 3.0 = 183     latin-xl (651 - 40) / 3.4 = 179.7
//
// which at the base size is 79 characters of Latin code a line, or 62 in a
// Latin article. The names are the attribute's tokens. The browser probe
// wide-blocks.mjs lays every fixture block out at each measure and holds this
// table to what overflows there.
var codeMeasures = []struct {
	token string
	fits  int
}{
	{"latin-xl", 179},
	{"latin-l", 183},
	{"latin-m", 187},
	{"xl", 205},
	{"l", 220},
	{"m", 238},
}

// wideMeasures names, in the order of codeMeasures, every measure source's
// longest line does not fit inside, or nothing when it fits inside all of them.
func wideMeasures(source string) string {
	longest := 0
	for line := range strings.Lines(source) {
		longest = max(longest, codeLineAdvance(strings.TrimRight(line, "\r\n")))
	}
	var tokens []string
	for _, m := range codeMeasures {
		if longest > m.fits {
			tokens = append(tokens, m.token)
		}
	}
	return strings.Join(tokens, " ")
}

// codeLineAdvance is how far one line of code runs, in fifths of an em.
func codeLineAdvance(line string) int {
	x := 0
	for _, r := range line {
		switch {
		case r == '\t':
			x = (x/codeTabStop + 1) * codeTabStop
		case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) || unicode.Is(unicode.Variation_Selector, r):
		case isFullWidth(r):
			x += codeWideAdvance
		default:
			x += codeNarrowAdvance
		}
	}
	return x
}

func isFullWidth(r rune) bool {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return true
	default:
		return false
	}
}
