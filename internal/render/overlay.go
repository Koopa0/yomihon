package render

import (
	"bytes"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// Span is a stretch of the words of a note, as TextContent counts them in bytes,
// that the reader's page draws over: a passage the reader marked. Where the
// stretch runs through more than one piece of text, each piece is wrapped on its
// own, so the markup stays as well formed as it was and a stretch across two
// paragraphs is two wrappers.
type Span struct {
	// Start and End are the stretch, End not included.
	Start, End int
	// First opens the wrapper of the first piece and Rest that of every other;
	// both are elements whose end tag is </mark>. They are given as markup and
	// are not looked at.
	First, Rest string
}

// Note is markup drawn after the block that holds the end of a stretch of words,
// which is where a note in the margin stands for a reader with no room for a
// margin: under the paragraph it is about. At is the offset of the stretch's end,
// as TextContent counts it.
type Note struct {
	At   int
	HTML string
}

// Overlay lays the reader's marks over a rendered fragment: each span's words
// wrapped, and each note after the block its words end in. Every other byte is
// copied as it came, and the words are those TextContent reads, which is why a
// span measured on TextContent of the same fragment is where the reader chose.
// tail is added at the very end, for a note with no place of its own.
//
// A note after a paragraph, a heading, a preformatted block or a table; at the
// end of a list item or a block that holds the words directly; and never inside
// a paragraph, where a block is not allowed and the parser would cut the
// paragraph in two.
func Overlay(fragment string, spans []Span, notes []Note, tail string) string {
	if len(spans) == 0 && len(notes) == 0 && tail == "" {
		return fragment
	}
	o := &overlay{spans: spans, notes: notes}
	o.out.Grow(len(fragment) + 128*(len(spans)+len(notes)))
	tokens := html.NewTokenizer(strings.NewReader(fragment))
	for {
		kind := tokens.Next()
		raw := bytes.Clone(tokens.Raw())
		if kind == html.ErrorToken {
			o.out.Write(raw)
			break
		}
		o.take(kind, tokens.Token(), raw)
	}
	o.out.WriteString(tail)
	return o.out.String()
}

// open is an element the cursor is inside.
type open struct {
	name string
	// notes wait here for the element's end tag.
	notes []string
}

type overlay struct {
	spans []Span
	notes []Note
	words wordReader
	out   strings.Builder
	stack []open
	// seen is how many spans have had a piece wrapped, so a span's first piece is
	// the one that opens it.
	started map[int]bool
}

// voidTag names the elements that have no end tag.
var voidTag = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

func (o *overlay) take(kind html.TokenType, token html.Token, raw []byte) {
	before := o.words.text.Len()
	o.words.take(kind, token)
	switch kind {
	case html.StartTagToken:
		o.out.Write(raw)
		if !voidTag[token.Data] {
			o.stack = append(o.stack, open{name: token.Data})
		}
	case html.EndTagToken:
		o.close(token.Data, raw)
	case html.TextToken:
		after := o.words.text.Len()
		if after == before {
			o.out.Write(raw)
			return
		}
		o.text(token.Data, before, raw)
	case html.SelfClosingTagToken, html.CommentToken, html.DoctypeToken, html.ErrorToken:
		o.out.Write(raw)
	}
}

// close writes an end tag, and the notes that were waiting for it.
func (o *overlay) close(name string, raw []byte) {
	at := -1
	for i := len(o.stack) - 1; i >= 0; i-- {
		if o.stack[i].name == name {
			at = i
			break
		}
	}
	if at < 0 {
		o.out.Write(raw)
		return
	}
	// Anything above the element it closes was left open by the markup, and has
	// nothing to say any more.
	var waiting []string
	for _, e := range o.stack[at:] {
		waiting = append(waiting, e.notes...)
	}
	o.stack = o.stack[:at]
	if len(waiting) == 0 {
		o.out.Write(raw)
		return
	}
	if placedAfter[name] {
		o.out.Write(raw)
		for _, n := range waiting {
			o.out.WriteString(n)
		}
		return
	}
	for _, n := range waiting {
		o.out.WriteString(n)
	}
	o.out.Write(raw)
}

// placedAfter are the carriers a note stands after and not inside.
var placedAfter = map[string]bool{
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"pre": true, "table": true, "dd": true, "dt": true, "figcaption": true, "summary": true,
}

// carriers are the elements a note can wait on: the nearest of them above the
// words, and for a cell of a table the table.
var carriers = map[string]bool{
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"pre": true, "li": true, "dd": true, "dt": true, "figcaption": true, "summary": true,
	"blockquote": true, "div": true, "section": true, "article": true, "aside": true,
	"details": true, "td": true, "th": true,
}

// carrier is the index in the stack of the element a note about words inside the
// innermost open element waits on, or -1.
func (o *overlay) carrier() int {
	for i := len(o.stack) - 1; i >= 0; i-- {
		name := o.stack[i].name
		if !carriers[name] {
			continue
		}
		if name == "td" || name == "th" {
			for j := i - 1; j >= 0; j-- {
				if o.stack[j].name == "table" {
					return j
				}
			}
		}
		return i
	}
	return -1
}

// text writes one piece of the words, wrapped where a span runs through it, and
// hands the notes whose stretch ends in it to the block that holds it. start is
// where the piece begins in TextContent's count.
func (o *overlay) text(data string, start int, raw []byte) {
	end := start + len(data)
	var cuts []int
	var covering []int
	for i, s := range o.spans {
		if s.End <= start || s.Start >= end || s.Start >= s.End {
			continue
		}
		covering = append(covering, i)
		cuts = append(cuts, max(s.Start, start)-start, min(s.End, end)-start)
	}
	for _, n := range o.notes {
		if n.At > start && n.At <= end {
			if i := o.carrier(); i >= 0 {
				o.stack[i].notes = append(o.stack[i].notes, n.HTML)
			}
		}
	}
	if len(covering) == 0 {
		o.out.Write(raw)
		return
	}
	cuts = append(cuts, 0, len(data))
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	for i := range cuts {
		cuts[i] = runeStart(data, cuts[i])
	}
	cuts = slices.Compact(cuts)
	if o.started == nil {
		o.started = make(map[int]bool)
	}
	for k := 0; k+1 < len(cuts); k++ {
		lo, hi := cuts[k], cuts[k+1]
		var inside []int
		for _, i := range covering {
			if o.spans[i].Start-start <= lo && o.spans[i].End-start >= hi {
				inside = append(inside, i)
			}
		}
		for _, i := range inside {
			if o.started[i] {
				o.out.WriteString(o.spans[i].Rest)
			} else {
				o.out.WriteString(o.spans[i].First)
				o.started[i] = true
			}
		}
		o.out.WriteString(html.EscapeString(data[lo:hi]))
		for range inside {
			o.out.WriteString("</mark>")
		}
	}
}

// runeStart moves i back to the start of the character it falls in.
func runeStart(s string, i int) int {
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}
