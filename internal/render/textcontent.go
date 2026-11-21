package render

import (
	"strings"

	"golang.org/x/net/html"
)

// TextContent is the words of an HTML fragment as a reader selects them: the
// text nodes in document order, with a space where a block ends so two
// paragraphs never run into one word. It is made from the sanitised HTML the
// page is served as, never from the Markdown, because a selection is made from
// the page. PlainText is the search index's text and differs at code spans,
// math, ruby and the markers a footnote adds.
//
// A script, a style and a template are not words, and neither is the bracket a
// browser hides around a ruby reading; the reading itself is selected with its
// base and so is in the text.
func TextContent(fragment string) string {
	var words wordReader
	tokens := html.NewTokenizer(strings.NewReader(fragment))
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			return words.text.String()
		}
		words.take(kind, tokens.Token())
	}
}

// wordReader collects the words of the tokens it is given.
type wordReader struct {
	text strings.Builder
	// skipping is the name of the element whose contents are not words, until
	// its end tag; depth counts a nested element of the same name inside it.
	skipping string
	depth    int
}

func (r *wordReader) take(kind html.TokenType, token html.Token) {
	switch kind {
	case html.StartTagToken, html.SelfClosingTagToken:
		r.opens(&token, kind == html.StartTagToken)
	case html.EndTagToken:
		r.closes(&token)
	case html.TextToken:
		if r.skipping == "" {
			r.text.WriteString(token.Data)
		}
	case html.CommentToken, html.DoctypeToken, html.ErrorToken:
	}
}

// opens reads a start tag: it begins a stretch that is not words, ends a line,
// or is nothing to a reader. A self-closing tag holds nothing to skip.
func (r *wordReader) opens(token *html.Token, hasContents bool) {
	switch {
	case r.skipping != "":
		if token.Data == r.skipping && hasContents {
			r.depth++
		}
	case hidesItsWords[token.Data] && hasContents:
		r.skipping, r.depth = token.Data, 1
	case endsALine[token.Data]:
		r.text.WriteByte(' ')
	}
}

// closes reads an end tag: it ends the stretch that is not words, or a line.
func (r *wordReader) closes(token *html.Token) {
	switch {
	case r.skipping == "":
		if endsALine[token.Data] {
			r.text.WriteByte(' ')
		}
	case token.Data == r.skipping:
		r.depth--
		if r.depth == 0 {
			r.skipping = ""
		}
	}
}

// hidesItsWords are the elements whose contents a browser does not show as
// text: code a page runs, rules it is styled by, markup it only stamps out, and
// the parentheses that stand around a ruby reading for a browser that draws no
// ruby.
var hidesItsWords = map[string]bool{"script": true, "style": true, "template": true, "rp": true}

// endsALine are the elements a browser puts on a line of their own, or between
// which it breaks one.
var endsALine = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true, "br": true,
	"dd": true, "details": true, "div": true, "dl": true, "dt": true,
	"figcaption": true, "figure": true, "footer": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "header": true, "hr": true,
	"li": true, "main": true, "nav": true, "ol": true, "p": true, "pre": true,
	"section": true, "summary": true, "table": true, "tbody": true, "td": true,
	"tfoot": true, "th": true, "thead": true, "tr": true, "ul": true,
}
