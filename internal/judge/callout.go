package judge

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// calloutTitle is one recognised callout's opening-line title, with the
// 1-based file line the opening sits on.
type calloutTitle struct {
	title string
	line  int
}

// extractCalloutTitlesFrom reads the title of every recognised callout in
// body whose opening line starts outside the comment zones, a fence and an
// indented code block, each with its file line.
func extractCalloutTitlesFrom(body string, bodyStartLine int, comments []byteRange) []calloutTitle {
	var out []calloutTitle
	var fence fenceTracker
	offset := 0
	for i, line := range strings.Split(body, "\n") {
		lineStart := offset
		offset += len(line) + 1
		if graph.In(comments, lineStart) {
			continue
		}
		if fence.fenced(line) {
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

// recognisedCalloutTitle reports the title of a syntactically valid callout's
// opening line, including unknown types whose neutral shell escapes the title.
// Opening syntax is independent of the renderer's block-address refusal.
func recognisedCalloutTitle(line string) (title string, ok bool) {
	if !render.IsCalloutOpening(line) {
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
		RuleID:          RuleCalloutTitleMarkup,
		Severity:        SeverityInfo,
		Path:            n.path,
		Line:            new(title.line),
		Message:         "this callout title carries markup the page escapes as visible text",
		Evidence:        "the title is written as " + title.title + " on a recognised callout's opening line",
		SuggestedAction: "move the markup into the callout body, or write the title as plain text",
		SourceRule:      sourceYomihon,
		Target:          new(title.title),
		Fingerprint:     fingerprint(RuleCalloutTitleMarkup, n.path, title.title),
	}
}

// titleCarriesMarkup reports whether a callout title holds a wikilink, an
// HTML tag, emphasis, or a code span — the markup calloutShell escapes.
func titleCarriesMarkup(title string) bool {
	if titleCarriesWikilink(title) {
		return true
	}
	found := false
	walkNodes(mdParser.Parse(text.NewReader([]byte(title))), func(n ast.Node) {
		switch n.(type) {
		case *ast.Emphasis, *ast.CodeSpan, *ast.RawHTML, *ast.HTMLBlock, *ast.Link, *ast.AutoLink, *ast.Image:
			found = true
		}
	})
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
