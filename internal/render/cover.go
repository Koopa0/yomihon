package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// CoverParagraph returns the first paragraph of a note body, as Markdown, when
// nothing but headings comes before it, and whether anything else follows. A
// list, table, code block or image first means the note has no opening
// paragraph to stand for it. The body must arrive with its frontmatter removed.
func CoverParagraph(body string) (paragraph string, more bool) {
	src := []byte(body)
	doc := plainParser.Parse(text.NewReader(src))
	for block := doc.FirstChild(); block != nil; block = block.NextSibling() {
		if _, heading := block.(*ast.Heading); heading {
			continue
		}
		p, ok := block.(*ast.Paragraph)
		if !ok || p.ChildCount() == 0 {
			return "", false
		}
		if _, image := p.FirstChild().(*ast.Image); image && p.ChildCount() == 1 {
			return "", false
		}
		var b strings.Builder
		lines := p.Lines()
		for i := range lines.Len() {
			segment := lines.At(i)
			b.Write(segment.Value(src))
		}
		return strings.TrimSpace(b.String()), p.NextSibling() != nil
	}
	return "", false
}
