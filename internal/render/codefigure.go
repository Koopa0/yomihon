package render

import (
	"html"
	"strings"
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
func codeFigureOpen(lang string) string {
	var b strings.Builder
	b.WriteString(`<div class="y-code"><div class="y-code__head">`)
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
func codeFigure(lang, pre string) string {
	return codeFigureOpen(lang) + pre + codeFigureClose
}
