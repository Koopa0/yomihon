package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// CoverParagraph returns the first paragraph of a note body, as Markdown, when
// nothing but headings comes before it, and whether anything else follows. A
// list, table, code block or image first means the note has no opening
// paragraph to stand for it. The note's link reference definitions come along
// after it, so a reference link in the paragraph still reaches its address.
// The body must arrive with its frontmatter removed.
func CoverParagraph(body string) (paragraph string, more bool) {
	src := []byte(body)
	context := parser.NewContext()
	doc := plainParser.Parse(text.NewReader(src), parser.WithContext(context))
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
		return strings.TrimSpace(b.String()) + referenceDefinitions(context.References()), followedByContent(p)
	}
	return "", false
}

// referenceDefinitions spells a note's link reference definitions as Markdown,
// each on a line of its own after a blank one.
func referenceDefinitions(refs []parser.Reference) string {
	var b strings.Builder
	for _, ref := range refs {
		b.WriteString("\n\n[" + string(ref.Label()) + "]: <" + string(ref.Destination()) + ">")
		if title := ref.Title(); len(title) > 0 {
			b.WriteString(` "` + strings.ReplaceAll(string(title), `"`, `\"`) + `"`)
		}
	}
	return b.String()
}

// followedByContent reports whether anything is written after the paragraph.
// A link reference definition is not something a reader would open the README
// to see: it is carried to the cover, not shown.
func followedByContent(p ast.Node) bool {
	for next := p.NextSibling(); next != nil; next = next.NextSibling() {
		if next.Kind() != ast.KindLinkReferenceDefinition {
			return true
		}
	}
	return false
}
