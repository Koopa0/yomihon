package graph

import (
	"regexp"
	"strings"

	cjkfriendly "github.com/tats-u/goldmark-cjk-friendly/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// headingInline reads a heading's source with the page's own inline grammar, so
// the marks the page hides around bold, code, links and footnote references
// are not in the words.
var headingInline = goldmark.New(goldmark.WithExtensions(extension.Strikethrough, extension.Footnote, cjkfriendly.CJKFriendlyEmphasis)).Parser()

var (
	trailingHashRun = regexp.MustCompile(`(^|[ \t])(#+)$`)

	// A ruby reading is spoken over the base text and is not part of the words,
	// as the page's own heading reduction treats it.
	rubyReadingInHeading = regexp.MustCompile(`(?is)<(rt|rp)>.*?</(rt|rp)>`)

	// The inert formatting tags the page keeps as markup. Any other tag is shown
	// as the text it was typed, so it stays in the words.
	inertTagInHeading = regexp.MustCompile(`(?i)^</?(b|i|u|s|em|strong|mark|sub|sup|small|kbd|code|del|ins|span|ruby|br)(\s[^>]*)?/?>$`)
)

// headingWords is the words a reader sees in a heading's source: emphasis and
// code marks, link and wikilink brackets, footnotes and a block address are
// gone and what they wrapped stays, with white space folded. A title shown in
// a list or over a page is these words, never the source. definitions are the
// footnoteDefinitions of the body the heading stands in.
func headingWords(raw, definitions string) string {
	source := HeadingSource(raw)
	var displayed strings.Builder
	last := 0
	for _, token := range WikilinkTokens(source) {
		displayed.WriteString(source[last:token.Start])
		_, display, _ := SplitWikilink(token.Inner)
		displayed.WriteString(display)
		last = token.End
	}
	displayed.WriteString(source[last:])
	source = rubyReadingInHeading.ReplaceAllString(displayed.String(), "")
	// The words are read as the inside of the heading they came from, so a tag
	// opening them is inline markup and not an HTML block of its own. An
	// underlined heading's words run over several lines and take their
	// underline back.
	if strings.Contains(source, "\n") {
		source += "\n==="
	} else {
		// The line scan has already taken off the one closing run a heading
		// may carry; one left at the end is words, and read as a heading again
		// it would be taken off a second time.
		source = "# " + trailingHashRun.ReplaceAllString(source, `$1\$2`)
	}
	src := []byte(source + definitions)
	doc := headingInline.Parse(text.NewReader(src))
	var b strings.Builder
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		if n.Kind() == east.KindFootnoteList {
			return ast.WalkSkipChildren, nil
		}
		if entering {
			writeHeadingNode(&b, n, src)
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(b.String()), " ")
}

// writeHeadingNode writes the words one inline node contributes to a heading.
func writeHeadingNode(b *strings.Builder, n ast.Node, src []byte) {
	switch n := n.(type) {
	case *ast.Text:
		value := n.Segment.Value(src)
		// Code is literal: an entity or a backslash in it is what was typed.
		if n.Parent().Kind() != ast.KindCodeSpan {
			value = util.ResolveEntityNames(util.UnescapePunctuations(value))
		}
		b.Write(value)
		if n.SoftLineBreak() || n.HardLineBreak() {
			b.WriteByte(' ')
		}
	case *ast.String:
		b.Write(n.Value)
	case *ast.AutoLink:
		b.Write(n.Label(src))
	case *ast.RawHTML:
		for i := range n.Segments.Len() {
			segment := n.Segments.At(i)
			if tag := segment.Value(src); !inertTagInHeading.Match(tag) {
				b.Write(tag)
			}
		}
	}
}
