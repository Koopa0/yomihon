package render

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// A table is set to the wide measure where the text's width would crowd it:
// where its columns, each at the width it reads comfortably at, add up to more
// than the measure. A column reads comfortably at its longest line, but never
// narrower than its longest unbroken word and never wider than a cap past which
// a column of sentences only wraps more evenly, not more readably. Its header
// is set on one line, so the whole header is a word it cannot break.
//
// The widths are counted from the table's own faces. The body is the sans at
// --fs-ed-14 and the header the sans at --fs-ed-13 in its semibold, tracked
// out by 0.02em, each at the text size the reader chose. A character the sans
// draws takes its advance from sansAdvance, measured from Geist; one East Asia
// sets full width falls back to a CJK face at a whole em; a combining mark, a
// joiner or a variation selector adds nothing. Code inside a cell is the mono
// face at 0.82em with 5px either side. Every column but the last ends in 16px
// of space before the next, which is the stylesheet's.
//
// It is an estimate, as a table's own layout is: the browser probe
// wide-blocks.mjs lays the fixture tables out in the browser and holds the
// estimate to what the browser draws, on tables that sit well clear of the
// boundary.
const (
	tableColumnGap = 16
	// tableColumnCap is the widest, in ems of the body face, a column is
	// counted at: about fifteen Han characters or thirty Latin letters.
	tableColumnCap = 15.0
	// tableHeaderWeight is how much wider the semibold header draws than the
	// regular weight it is counted in, and tableHeaderTracking the space the
	// stylesheet sets after each of its characters, in ems of its face.
	tableHeaderWeight   = 1.045
	tableHeaderTracking = 0.02
	// tableCodeAdvance and tableCodePadding are a code span's character and
	// its padding either side together, in ems of the cell: 5px a side is
	// about a third of an em at every size.
	tableCodeAdvance = 0.6 * 0.82
	tableCodePadding = 0.7
)

// tableBodySize and tableHeaderSize are the body's and the header's pixel
// sizes at each text size: --fs-ed-14 and --fs-ed-13.
var (
	tableBodySize   = [...]float64{textM: 14, textL: 16, textXL: 18}
	tableHeaderSize = [...]float64{textM: 13, textL: 15, textXL: 17}
)

// tableWrapOpen is the opening renderWrappedTable writes, which is how the pass
// below finds a table: an author's own markup reaches the page escaped, so
// nothing else writes it.
const tableWrapOpen = `<div class="` + tableWrapClass + `"><table>`

// widenTables names, on every table's wrapper, each measure the table does not
// fit at.
func widenTables(htmlOut string) string {
	if !strings.Contains(htmlOut, tableWrapOpen) {
		return htmlOut
	}
	var b strings.Builder
	b.Grow(len(htmlOut) + 64)
	rest := htmlOut
	for {
		at := strings.Index(rest, tableWrapOpen)
		if at < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:at])
		rest = rest[at+len(tableWrapOpen):]
		b.WriteString(`<div class="` + tableWrapClass + `"`)
		b.WriteString(wideAttribute(tableWide(tableColumns(rest))))
		b.WriteString(`><table>`)
	}
}

// tableColumn is what one column holds, in ems of the face each part is set in:
// the body's longest line and longest unbroken word, and its header's width.
type tableColumn struct {
	line, word, header float64
}

// tableWide names every measure a table with these columns is crowded at.
func tableWide(columns []tableColumn) string {
	if len(columns) == 0 {
		return ""
	}
	return wideFor(func(m readingMeasure) bool {
		body, header := tableBodySize[m.size], tableHeaderSize[m.size]
		need := float64(tableColumnGap * (len(columns) - 1))
		for _, c := range columns {
			floor := max(c.word*body, c.header*header*tableHeaderWeight)
			need += max(floor, min(c.line*body, tableColumnCap*body))
		}
		return need > float64(m.width)
	})
}

// tableColumns reads the columns of the table whose markup starts htmlOut, up
// to its own closing tag. It reads what the renderer wrote, after every link
// and embed in it has been substituted, so a cell is counted by the words a
// reader sees: a reading over a word (an rt or rp) and a note only a screen
// reader hears are not.
func tableColumns(htmlOut string) []tableColumn {
	z := html.NewTokenizer(strings.NewReader(htmlOut))
	r := tableReader{depth: 1}
	for !r.done {
		switch z.Next() {
		case html.ErrorToken:
			return r.columns
		case html.StartTagToken:
			r.start(z)
		case html.SelfClosingTagToken:
			if tag, _ := z.TagName(); r.cell != nil && atom.Lookup(tag) == atom.Br {
				r.cell.breakLine()
			}
		case html.EndTagToken:
			tag, _ := z.TagName()
			r.end(atom.Lookup(tag))
		case html.TextToken:
			if r.cell != nil && !r.unseen() {
				r.cell.text(string(z.Text()), r.code > 0)
			}
		default:
		}
	}
	return r.columns
}

// tableReader is where tableColumns stands in a table: the cell it is in, the
// column that cell is, and the elements open inside it.
type tableReader struct {
	columns []tableColumn
	cell    *cellCount
	column  int
	// quiet says, for each element open inside the cell, whether what it holds
	// is unseen; code counts the code spans open.
	quiet []bool
	code  int
	// depth is how many tables deep the reader is: a table inside a cell is
	// part of that cell.
	depth int
	done  bool
}

func (r *tableReader) unseen() bool {
	return len(r.quiet) > 0 && r.quiet[len(r.quiet)-1]
}

func (r *tableReader) start(z *html.Tokenizer) {
	tag, hasAttr := z.TagName()
	name := atom.Lookup(tag)
	if name == atom.Table {
		r.depth++
	}
	if r.depth == 1 && name == atom.Tr {
		r.column = 0
	}
	if r.depth == 1 && (name == atom.Th || name == atom.Td) {
		r.cell = &cellCount{header: name == atom.Th}
		r.quiet = r.quiet[:0]
		return
	}
	if r.cell != nil {
		r.open(name, hasAttr && hasClass(z, offscreenNoteClass))
	}
}

// open is an element starting inside a cell; offscreen says it is a note only
// a screen reader hears.
func (r *tableReader) open(name atom.Atom, offscreen bool) {
	if name == atom.Br {
		r.cell.breakLine()
		return
	}
	if voidElement(name) {
		return
	}
	unseen := r.unseen() || offscreen || name == atom.Rt || name == atom.Rp
	r.quiet = append(r.quiet, unseen)
	if name == atom.Code {
		r.code++
		if !unseen {
			r.cell.codeSpan()
		}
	}
}

func (r *tableReader) end(name atom.Atom) {
	if name == atom.Table {
		r.depth--
		r.done = r.depth == 0
	}
	if r.depth == 1 && r.cell != nil && (name == atom.Th || name == atom.Td) {
		for len(r.columns) <= r.column {
			r.columns = append(r.columns, tableColumn{})
		}
		r.cell.into(&r.columns[r.column])
		r.cell = nil
		r.column++
		return
	}
	if r.cell == nil || len(r.quiet) == 0 {
		return
	}
	r.quiet = r.quiet[:len(r.quiet)-1]
	if name == atom.Code {
		r.code = max(0, r.code-1)
	}
}

// voidElement reports whether an element has no end tag to pop it.
func voidElement(name atom.Atom) bool {
	return name == atom.Img || name == atom.Input || name == atom.Wbr || name == atom.Hr || name == atom.Br
}

// hasClass reports whether the tag the tokenizer stands on carries class.
func hasClass(z *html.Tokenizer, class string) bool {
	for {
		key, value, more := z.TagAttr()
		if string(key) == "class" && strings.Contains(" "+string(value)+" ", " "+class+" ") {
			return true
		}
		if !more {
			return false
		}
	}
}

// cellCount measures one cell as its text arrives: its longest line and its
// longest unbroken word, in ems.
type cellCount struct {
	header     bool
	line, word float64
	longest    float64
	widest     float64
}

func (c *cellCount) breakLine() {
	c.longest = max(c.longest, c.line)
	c.widest = max(c.widest, c.word)
	c.line, c.word = 0, 0
}

// codeSpan counts the padding a code span is drawn with, which sits on its
// line and inside whatever word it is part of.
func (c *cellCount) codeSpan() {
	c.line += tableCodePadding
	c.word += tableCodePadding
}

func (c *cellCount) text(s string, code bool) {
	for _, r := range s {
		advance := sansAdvance(r)
		if c.header {
			advance += tableHeaderTracking
		}
		if code {
			advance = tableCodeAdvance
			if isFullWidth(r) {
				advance = 0.82
			}
		}
		switch {
		case r == '\n':
			advance = sansAdvance(' ')
			fallthrough
		case unicode.IsSpace(r):
			// A space ends a word; a line breaks there unless it is a header.
			c.widest = max(c.widest, c.word)
			c.word = 0
			c.line += advance
		case isFullWidth(r):
			// A Han, kana or Hangul character may end a line on either side.
			c.widest = max(c.widest, c.word)
			c.word = 0
			c.line += advance
			c.widest = max(c.widest, advance)
		default:
			c.line += advance
			c.word += advance
		}
	}
}

// into records the cell in its column.
func (c *cellCount) into(col *tableColumn) {
	c.breakLine()
	if c.header {
		col.header = max(col.header, c.longest)
		return
	}
	col.line = max(col.line, c.longest)
	col.word = max(col.word, c.widest)
}

// sansAdvance is how far the sans face moves for one character, in ems: Geist's
// own advances for the printable ASCII range, in thousandths, a whole em for a
// character East Asia sets full width, nothing for a mark that joins the one
// before it, and an average lowercase letter for anything else.
func sansAdvance(r rune) float64 {
	switch {
	case r >= ' ' && r <= '~':
		return float64(geistAdvance[r-' ']) / 1000
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) || unicode.Is(unicode.Variation_Selector, r):
		return 0
	case isFullWidth(r):
		return 1
	default:
		return 0.56
	}
}

// geistAdvance is Geist's regular advance, in thousandths of an em, for each
// character from the space to the tilde, with figures tabular as the table sets
// them.
var geistAdvance = [...]int{
	250, 213, 346, 478, 600, 802, 620, 178, 274, 274, 430, 558, 201, 419, 201, 385, // space to /
	600, 600, 600, 600, 600, 600, 600, 600, 600, 600, 297, 297, 544, 540, 544, 559, // 0 to ?
	906, 668, 673, 703, 709, 594, 572, 700, 695, 252, 570, 622, 553, 859, 725, 739, // @ to O
	632, 733, 672, 640, 561, 689, 658, 936, 606, 594, 544, 347, 385, 347, 540, 557, // P to _
	300, 551, 586, 537, 586, 552, 319, 585, 576, 226, 251, 545, 258, 859, 563, 564, // ` to o
	586, 586, 343, 511, 331, 575, 536, 819, 549, 537, 523, 347, 264, 347, 540, // p to ~
}
