package judge

import (
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

const calloutTitleMarkupRule RuleID = "callout.title_markup"

// calloutTitle is one recognised callout's opening-line title, with the
// 1-based file line the opening sits on.
type calloutTitle struct {
	title string
	line  int
}

// extractCalloutTitles walks a note body the way the page classifies a
// callout: fences and comments are not titles, an unknown type is a
// blockquote, and a recognised opening's title is the text after `[!type]`.
func extractCalloutTitles(body string, bodyStartLine int) []calloutTitle {
	codeZones, _ := structure(body, nil)
	skip := graph.CommentZones(body, codeZones)
	var out []calloutTitle
	inFence, fenceByte, fenceLen := false, byte(0), 0
	offset := 0
	for i, line := range strings.Split(body, "\n") {
		lineStart := offset
		offset += len(line) + 1
		if graph.In(skip, lineStart) {
			continue
		}
		unquoted := graph.QuotePrefix.ReplaceAllString(line, "")
		if inFence {
			if graph.FenceCloses(unquoted, fenceByte, fenceLen) {
				inFence = false
			}
			continue
		}
		if marker, n, ok := graph.FenceOpens(unquoted); ok {
			inFence, fenceByte, fenceLen = true, marker, n
			continue
		}
		if graph.IndentedCodeLine.MatchString(line) {
			continue
		}
		title, ok := recognisedCalloutTitle(line)
		if !ok || title == "" {
			continue
		}
		out = append(out, calloutTitle{title: title, line: bodyStartLine + i})
	}
	return out
}

// recognisedCalloutTitle reports the title of a recognised callout's opening
// line. Recognition is render.UnanchorableLine's, so a type the page does not
// answer to stays a blockquote here too. A table row is also unanchorable and
// is not a callout.
func recognisedCalloutTitle(line string) (title string, ok bool) {
	if !render.UnanchorableLine(line) {
		return "", false
	}
	unquoted := graph.QuotePrefix.ReplaceAllString(line, "")
	if strings.HasPrefix(strings.TrimLeft(unquoted, " \t"), "|") {
		return "", false
	}
	open := strings.Index(line, "[!")
	if open < 0 {
		return "", false
	}
	end := strings.IndexByte(line[open:], ']')
	if end < 0 {
		return "", false
	}
	rest := line[open+end+1:]
	if rest != "" && (rest[0] == '-' || rest[0] == '+') {
		rest = rest[1:]
	}
	return strings.TrimSpace(rest), true
}

func calloutTitleFindings(notes []note) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		for _, title := range n.calloutTitles {
			if !titleCarriesMarkup(title.title) {
				continue
			}
			out = append(out, calloutTitleMarkupFinding(n, title))
		}
	}
	return out
}

func calloutTitleMarkupFinding(n *note, title calloutTitle) Finding {
	return Finding{
		RuleID:          calloutTitleMarkupRule,
		Severity:        SeverityInfo,
		Path:            n.path,
		Line:            new(title.line),
		Message:         "this callout title carries markup the page escapes as visible text",
		Evidence:        "the title is written as " + title.title + " on a recognised callout's opening line",
		SuggestedAction: "move the markup into the callout body, or write the title as plain text",
		SourceRule:      sourceYomihon,
		Target:          new(title.title),
		Fingerprint:     fingerprint(calloutTitleMarkupRule, n.path, strconv.Itoa(title.line)+"\x1f"+title.title),
	}
}

// titleCarriesMarkup reports whether a callout title holds a wikilink, an
// HTML tag, emphasis, or a code span — the markup calloutShell escapes.
func titleCarriesMarkup(title string) bool {
	if titleCarriesWikilink(title) {
		return true
	}
	doc := mdParser.Parse(text.NewReader([]byte(title)))
	found := false
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Emphasis, *ast.CodeSpan, *ast.RawHTML, *ast.HTMLBlock, *ast.Link, *ast.AutoLink, *ast.Image:
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	}); err != nil {
		return false
	}
	return found
}

func titleCarriesWikilink(title string) bool {
	for i := 0; i+1 < len(title); i++ {
		if title[i] == '[' && title[i+1] == '[' && !graph.EscapedWikilinkAt(title, i) {
			return true
		}
	}
	return false
}
